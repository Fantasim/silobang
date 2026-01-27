package services

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zeebo/blake3"

	"silobang/internal/constants"
	"silobang/internal/database"
	"silobang/internal/logger"
	"silobang/internal/sanitize"
	"silobang/internal/storage"
)

// UploadResult contains the result of an asset upload operation.
type UploadResult struct {
	Hash          string `json:"hash"`
	Size          int64  `json:"size"`
	BlobName      string `json:"blob"`
	Skipped       bool   `json:"skipped"`
	ExistingTopic string `json:"existing_topic,omitempty"`
}

// AssetInfo contains information about an asset for download.
type AssetInfo struct {
	Hash        string
	Size        int64
	OriginName  string
	Extension   string
	ContentType string
	TopicName   string
}

// AssetReader wraps a file reader with asset metadata.
type AssetReader struct {
	io.ReadCloser
	Info *AssetInfo
}

// AssetService handles asset upload, download, and management operations.
type AssetService struct {
	app    AppState
	logger *logger.Logger
}

// NewAssetService creates a new asset service instance.
func NewAssetService(app AppState, log *logger.Logger) *AssetService {
	return &AssetService{
		app:    app,
		logger: log,
	}
}

// Upload handles the complete upload workflow for an asset.
// It streams the file to disk while computing the hash, checks for duplicates,
// and atomically writes to the DAT file and database.
func (s *AssetService) Upload(ctx context.Context, topicName string, reader io.Reader, filename string, parentID *string) (*UploadResult, error) {
	// Get max size from config
	maxSize := s.app.GetConfig().MaxDatSize
	if maxSize == 0 {
		maxSize = constants.DefaultMaxDatSize
	}

	// Validate parent if provided
	if parentID != nil && *parentID != "" {
		exists, _, _, err := database.CheckHashExists(s.app.GetOrchestratorDB(), *parentID)
		if err != nil {
			return nil, WrapInternalError(err)
		}
		if !exists {
			return nil, NewServiceError(constants.ErrCodeParentNotFound, "parent asset not found")
		}
	}

	// Sanitize filename to prevent path traversal, header injection, and control character attacks
	cleanFilename := sanitize.Filename(filename)
	ext := ""
	originName := ""
	if idx := strings.LastIndex(cleanFilename, "."); idx != -1 {
		ext = sanitize.Extension(cleanFilename[idx+1:])
		originName = sanitize.OriginName(cleanFilename[:idx])
	} else {
		originName = sanitize.OriginName(cleanFilename)
	}
	s.logger.Debug("Sanitized upload filename: original=%q sanitized=%q originName=%q ext=%q",
		filename, cleanFilename, originName, ext)

	// Stream file to temp file while computing hash (outside lock - I/O intensive and safe)
	tempFile, hash, size, err := s.streamToTempWithHash(reader, maxSize)
	if err != nil {
		if err.Error() == "file too large" {
			return nil, ErrAssetTooLarge
		}
		return nil, WrapInternalError(err)
	}
	defer os.Remove(tempFile)

	// Acquire per-topic write mutex for the critical section:
	// duplicate check + dat file write + DB commit must be serialized
	// to prevent byte offset collisions and duplicate detection races
	topicMu := s.app.GetTopicWriteMu(topicName)
	topicMu.Lock()
	defer topicMu.Unlock()

	s.logger.Debug("Acquired write lock for topic %s, hash %s", topicName, hash)

	// Check for duplicate (inside lock to prevent race)
	exists, existingTopic, _, err := database.CheckHashExists(s.app.GetOrchestratorDB(), hash)
	if err != nil {
		return nil, WrapInternalError(err)
	}
	if exists {
		s.logger.Debug("Duplicate detected for hash %s in topic %s, skipping", hash, existingTopic)
		return &UploadResult{
			Hash:          hash,
			Skipped:       true,
			ExistingTopic: existingTopic,
			Size:          size,
		}, nil
	}

	// Get topic database
	topicDB, err := s.app.GetTopicDB(topicName)
	if err != nil {
		return nil, s.wrapTopicError(topicName, err)
	}

	topicPath := s.app.GetTopicPath(topicName)

	// Write asset using pipeline (inside lock - dat file write + DB commit)
	asset, err := s.writeAssetFromTempFile(topicDB, topicName, topicPath, tempFile, hash, size, ext, originName, parentID)
	if err != nil {
		return nil, WrapInternalError(err)
	}

	s.logger.Debug("Uploaded asset %s to topic %s", hash, topicName)

	return &UploadResult{
		Hash:     asset.AssetID,
		Size:     asset.AssetSize,
		BlobName: asset.BlobName,
		Skipped:  false,
	}, nil
}

// GetReader returns a reader for downloading an asset by hash.
// The caller is responsible for closing the returned reader.
func (s *AssetService) GetReader(hash string) (*AssetReader, error) {
	// Validate hash format
	if len(hash) != constants.HashLength {
		return nil, ErrInvalidHash
	}

	// Look up in orchestrator to find topic
	exists, topicName, _, err := database.CheckHashExists(s.app.GetOrchestratorDB(), hash)
	if err != nil {
		return nil, WrapInternalError(err)
	}
	if !exists {
		return nil, ErrAssetNotFoundWithHash(hash)
	}

	// Check topic health
	healthy, errMsg := s.app.IsTopicHealthy(topicName)
	if !healthy {
		return nil, ErrTopicUnhealthyWithReason(topicName, errMsg)
	}

	// Get asset details from topic DB
	topicDB, err := s.app.GetTopicDB(topicName)
	if err != nil {
		return nil, WrapInternalError(err)
	}

	asset, err := database.GetAsset(topicDB, hash)
	if err != nil {
		return nil, WrapInternalError(err)
	}
	if asset == nil {
		return nil, ErrAssetNotFoundWithHash(hash)
	}

	// Determine content type
	contentType := constants.DefaultMimeType
	if mimeType, ok := constants.ExtensionMimeTypes[asset.Extension]; ok {
		contentType = mimeType
	}

	// Open the DAT file
	topicPath := s.app.GetTopicPath(topicName)
	datPath := filepath.Join(topicPath, asset.BlobName)

	f, err := os.Open(datPath)
	if err != nil {
		return nil, WrapInternalError(fmt.Errorf("failed to open data file: %w", err))
	}

	// Seek to data start (skip header)
	dataStart := asset.ByteOffset + int64(constants.HeaderSize)
	if _, err := f.Seek(dataStart, io.SeekStart); err != nil {
		f.Close()
		return nil, WrapInternalError(fmt.Errorf("failed to seek in data file: %w", err))
	}

	// Create limited reader that only reads the asset data
	limitedReader := io.LimitReader(f, asset.AssetSize)

	return &AssetReader{
		ReadCloser: &assetFileReader{
			Reader: limitedReader,
			Closer: f,
		},
		Info: &AssetInfo{
			Hash:        hash,
			Size:        asset.AssetSize,
			OriginName:  asset.OriginName,
			Extension:   asset.Extension,
			ContentType: contentType,
			TopicName:   topicName,
		},
	}, nil
}

// GetInfo returns information about an asset without streaming data.
func (s *AssetService) GetInfo(hash string) (*AssetInfo, error) {
	// Validate hash format
	if len(hash) != constants.HashLength {
		return nil, ErrInvalidHash
	}

	// Look up in orchestrator to find topic
	exists, topicName, _, err := database.CheckHashExists(s.app.GetOrchestratorDB(), hash)
	if err != nil {
		return nil, WrapInternalError(err)
	}
	if !exists {
		return nil, ErrAssetNotFoundWithHash(hash)
	}

	// Check topic health
	healthy, errMsg := s.app.IsTopicHealthy(topicName)
	if !healthy {
		return nil, ErrTopicUnhealthyWithReason(topicName, errMsg)
	}

	// Get asset details from topic DB
	topicDB, err := s.app.GetTopicDB(topicName)
	if err != nil {
		return nil, WrapInternalError(err)
	}

	asset, err := database.GetAsset(topicDB, hash)
	if err != nil {
		return nil, WrapInternalError(err)
	}
	if asset == nil {
		return nil, ErrAssetNotFoundWithHash(hash)
	}

	// Determine content type
	contentType := constants.DefaultMimeType
	if mimeType, ok := constants.ExtensionMimeTypes[asset.Extension]; ok {
		contentType = mimeType
	}

	return &AssetInfo{
		Hash:        hash,
		Size:        asset.AssetSize,
		OriginName:  asset.OriginName,
		Extension:   asset.Extension,
		ContentType: contentType,
		TopicName:   topicName,
	}, nil
}

// streamToTempWithHash streams data to a temp file while computing BLAKE3 hash.
// Returns temp file path, hash, size, or error.
func (s *AssetService) streamToTempWithHash(r io.Reader, maxSize int64) (tempPath string, hash string, size int64, err error) {
	// Create temp file
	tempFile, err := os.CreateTemp("", "silobang-upload-*")
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath = tempFile.Name()

	// Setup hash writer
	hasher := blake3.New()

	// Create a multi-writer to write to both temp file and hasher
	multiWriter := io.MultiWriter(tempFile, hasher)

	// Copy with size limit
	limitReader := io.LimitReader(r, maxSize+1) // +1 to detect overflow
	size, err = io.Copy(multiWriter, limitReader)

	tempFile.Close() // Close before potential cleanup

	if err != nil {
		os.Remove(tempPath)
		return "", "", 0, fmt.Errorf("failed to write temp file: %w", err)
	}

	if size > maxSize-int64(constants.HeaderSize) {
		os.Remove(tempPath)
		return "", "", 0, fmt.Errorf("file too large")
	}

	// Get hash
	hashBytes := hasher.Sum(nil)
	hash = hex.EncodeToString(hashBytes)

	return tempPath, hash, size, nil
}

// writeAssetFromTempFile writes an asset from a temp file using the pipeline.
func (s *AssetService) writeAssetFromTempFile(
	topicDB *sql.DB,
	topicName string,
	topicPath string,
	tempFile string,
	hash string,
	size int64,
	extension string,
	originName string,
	parentID *string,
) (*database.Asset, error) {
	maxDatSize := s.app.GetConfig().MaxDatSize
	if maxDatSize == 0 {
		maxDatSize = constants.DefaultMaxDatSize
	}

	// Determine target .dat file
	entrySize := int64(constants.HeaderSize) + size
	datFile, _, err := storage.DetermineTargetDatFile(topicPath, entrySize, maxDatSize)
	if err != nil {
		return nil, fmt.Errorf("failed to determine dat file: %w", err)
	}

	datPath := filepath.Join(topicPath, datFile)

	// Begin transactions
	txTopic, err := topicDB.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin topic transaction: %w", err)
	}
	defer txTopic.Rollback()

	txOrch, err := s.app.GetOrchestratorDB().Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin orchestrator transaction: %w", err)
	}
	defer txOrch.Rollback()

	// Append to .dat file by streaming from temp file
	byteOffset, err := s.appendFromTempFile(datPath, hash, tempFile, size)
	if err != nil {
		return nil, fmt.Errorf("failed to append to dat file: %w", err)
	}

	// Create asset record
	asset := database.Asset{
		AssetID:    hash,
		AssetSize:  size,
		OriginName: originName,
		ParentID:   parentID,
		Extension:  extension,
		BlobName:   datFile,
		ByteOffset: byteOffset,
		CreatedAt:  time.Now().Unix(),
	}

	if err := database.InsertAsset(txTopic, asset); err != nil {
		return nil, fmt.Errorf("failed to insert asset: %w", err)
	}

	if err := database.InsertAssetIndex(txOrch, hash, topicName, datFile); err != nil {
		return nil, fmt.Errorf("failed to insert asset index: %w", err)
	}

	// Compute new running hash - O(1) operation
	prevHash, entryCount, err := database.GetDatHashTx(txTopic, datFile)
	if err != nil {
		return nil, fmt.Errorf("failed to get dat hash: %w", err)
	}
	if prevHash == "" {
		// New .dat file - use genesis hash
		prevHash = storage.GenesisHash(datFile)
		entryCount = 0
	}

	newRunningHash, err := storage.ComputeRunningHash(prevHash, hash, byteOffset, size)
	if err != nil {
		return nil, fmt.Errorf("failed to compute running hash: %w", err)
	}

	if err := database.UpdateDatHash(txTopic, datFile, newRunningHash, entryCount+1); err != nil {
		return nil, fmt.Errorf("failed to update dat hash: %w", err)
	}

	// Commit transactions
	if err := txTopic.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit topic transaction: %w", err)
	}

	if err := txOrch.Commit(); err != nil {
		s.logger.Warn("Orchestrator commit failed (will recover on restart): %v", err)
	}

	return &asset, nil
}

// appendFromTempFile appends data from temp file to .dat file.
func (s *AssetService) appendFromTempFile(datPath string, hash string, tempFile string, size int64) (byteOffset int64, err error) {
	// Serialize header
	header, err := storage.SerializeHeader(hash, uint64(size))
	if err != nil {
		return 0, err
	}

	// Open .dat file for appending
	datFile, err := os.OpenFile(datPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return 0, fmt.Errorf("failed to open dat file: %w", err)
	}
	defer datFile.Close()

	// Get current offset
	stat, err := datFile.Stat()
	if err != nil {
		return 0, fmt.Errorf("failed to stat dat file: %w", err)
	}
	byteOffset = stat.Size()

	// Write header
	if _, err := datFile.Write(header); err != nil {
		return 0, fmt.Errorf("failed to write header: %w", err)
	}

	// Stream data from temp file
	srcFile, err := os.Open(tempFile)
	if err != nil {
		return 0, fmt.Errorf("failed to open temp file: %w", err)
	}
	defer srcFile.Close()

	if _, err := io.Copy(datFile, srcFile); err != nil {
		return 0, fmt.Errorf("failed to copy data: %w", err)
	}

	// Sync to ensure durability
	if err := datFile.Sync(); err != nil {
		return 0, fmt.Errorf("failed to sync dat file: %w", err)
	}

	return byteOffset, nil
}

// wrapTopicError wraps topic-related errors with appropriate service errors.
func (s *AssetService) wrapTopicError(topicName string, err error) *ServiceError {
	errStr := err.Error()
	if strings.Contains(errStr, "topic not found") {
		return ErrTopicNotFoundWithName(topicName)
	}
	if strings.Contains(errStr, "topic unhealthy") {
		return ErrTopicUnhealthyWithReason(topicName, errStr)
	}
	return WrapInternalError(err)
}

// assetFileReader combines a limited reader with a file closer.
type assetFileReader struct {
	io.Reader
	Closer io.Closer
}

func (r *assetFileReader) Close() error {
	return r.Closer.Close()
}
