package server

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"meshbank/internal/constants"
	"meshbank/internal/database"
	"meshbank/internal/sanitize"
	"meshbank/internal/services"
)

// BulkDownloadRequest represents the request body for bulk downloads
type BulkDownloadRequest struct {
	Mode            string                 `json:"mode"`             // "query" | "ids"
	Preset          string                 `json:"preset"`           // for mode="query"
	Params          map[string]interface{} `json:"params"`           // for mode="query"
	Topics          []string               `json:"topics"`           // for mode="query", optional
	AssetIDs        []string               `json:"asset_ids"`        // for mode="ids"
	IncludeMetadata bool                   `json:"include_metadata"` // include metadata files
	FilenameFormat  string                 `json:"filename_format"`  // "hash" | "original" | "hash_original"
}

// ManifestAsset represents an asset entry in the manifest
type ManifestAsset struct {
	Hash       string `json:"hash"`
	Filename   string `json:"filename"`
	Size       int64  `json:"size"`
	Extension  string `json:"extension"`
	OriginName string `json:"origin_name"`
	Topic      string `json:"topic"`
}

// FailedAsset represents a failed asset in the manifest
type FailedAsset struct {
	Hash  string `json:"hash"`
	Error string `json:"error"`
	Topic string `json:"topic,omitempty"`
}

// BulkDownloadManifest represents the manifest.json content
type BulkDownloadManifest struct {
	CreatedAt       int64           `json:"created_at"`
	AssetCount      int             `json:"asset_count"`
	TotalSize       int64           `json:"total_size"`
	IncludeMetadata bool            `json:"include_metadata"`
	Assets          []ManifestAsset `json:"assets"`
	FailedAssets    []FailedAsset   `json:"failed_assets,omitempty"`
}

// AssetMetadataFile represents the per-asset metadata JSON file content
type AssetMetadataFile struct {
	Asset            BulkAssetInfo          `json:"asset"`
	ComputedMetadata map[string]interface{} `json:"computed_metadata"`
}

// BulkAssetInfo contains asset information for metadata files
type BulkAssetInfo struct {
	Hash       string  `json:"hash"`
	Size       int64   `json:"size"`
	Extension  string  `json:"extension"`
	OriginName string  `json:"origin_name"`
	ParentID   *string `json:"parent_id,omitempty"`
	CreatedAt  int64   `json:"created_at"`
	Topic      string  `json:"topic"`
	BlobName   string  `json:"blob_name"`
}

// ZIPBuildCallbacks provides optional hooks for progress tracking and cancellation
// during ZIP archive generation. Both fields are optional — pass nil for the entire
// struct when no callbacks are needed (e.g., direct streaming).
type ZIPBuildCallbacks struct {
	// OnAssetProcessed is called after each asset is written (or fails).
	// index is 0-based, filename is the resolved filename in the ZIP.
	OnAssetProcessed func(index int, asset *services.ResolvedAsset, filename string, processedBytes int64)
	// CheckCancelled returns true if the operation should abort.
	CheckCancelled func() bool
}

// ZIPBuildResult contains the output of a buildZIPArchive operation.
type ZIPBuildResult struct {
	Manifest    BulkDownloadManifest
	FailedCount int
	Topics      []string
	TotalSize   int64
	Cancelled   bool
}

// buildZIPArchive writes assets into a ZIP archive with manifest and optional metadata.
// The caller is responsible for creating and closing the zip.Writer.
// Progress and cancellation are handled via optional callbacks.
func (s *Server) buildZIPArchive(
	zipWriter *zip.Writer,
	assets []*services.ResolvedAsset,
	req BulkDownloadRequest,
	callbacks *ZIPBuildCallbacks,
) ZIPBuildResult {
	// Initialize manifest
	manifest := BulkDownloadManifest{
		CreatedAt:       time.Now().Unix(),
		IncludeMetadata: req.IncludeMetadata,
		Assets:          make([]ManifestAsset, 0, len(assets)),
		FailedAssets:    make([]FailedAsset, 0),
	}

	// Track used filenames for collision handling
	usedNames := make(map[string]int)

	// Collect unique topics
	topicSet := make(map[string]struct{})

	var processedBytes int64
	failedCount := 0

	// Write each asset
	for i, resolved := range assets {
		// Check cancellation
		if callbacks != nil && callbacks.CheckCancelled != nil && callbacks.CheckCancelled() {
			return ZIPBuildResult{
				Manifest:    manifest,
				FailedCount: failedCount,
				Topics:      collectTopics(topicSet),
				TotalSize:   manifest.TotalSize,
				Cancelled:   true,
			}
		}

		filename := buildFilename(resolved.Asset, req.FilenameFormat, usedNames)
		fullPath := constants.BulkDownloadAssetsDir + "/" + filename

		// Write asset file
		err := s.writeAssetToZip(zipWriter, resolved, fullPath)
		if err != nil {
			manifest.FailedAssets = append(manifest.FailedAssets, FailedAsset{
				Hash:  resolved.Hash,
				Error: err.Error(),
				Topic: resolved.Topic,
			})
			failedCount++
			s.logger.Error("Failed to write asset %s to ZIP: %v", resolved.Hash, err)

			// Notify progress even for failed assets
			if callbacks != nil && callbacks.OnAssetProcessed != nil {
				callbacks.OnAssetProcessed(i, resolved, filename, processedBytes)
			}
			continue
		}

		// Track in manifest
		manifest.Assets = append(manifest.Assets, ManifestAsset{
			Hash:       resolved.Hash,
			Filename:   fullPath,
			Size:       resolved.Asset.AssetSize,
			Extension:  resolved.Asset.Extension,
			OriginName: resolved.Asset.OriginName,
			Topic:      resolved.Topic,
		})
		manifest.TotalSize += resolved.Asset.AssetSize
		processedBytes += resolved.Asset.AssetSize
		topicSet[resolved.Topic] = struct{}{}

		// Write metadata file if requested (filename mirrors asset filename with .json extension)
		if req.IncludeMetadata {
			metadataBaseName := filename
			if cleanExt := sanitize.Extension(resolved.Asset.Extension); cleanExt != "" {
				metadataBaseName = strings.TrimSuffix(filename, "."+cleanExt)
			}
			metadataPath := constants.BulkDownloadMetadataDir + "/" + metadataBaseName + ".json"
			if err := s.writeMetadataToZip(zipWriter, resolved, metadataPath); err != nil {
				s.logger.Error("Failed to write metadata for %s: %v", resolved.Hash, err)
			}
		}

		// Notify progress
		if callbacks != nil && callbacks.OnAssetProcessed != nil {
			callbacks.OnAssetProcessed(i, resolved, filename, processedBytes)
		}
	}

	// Update manifest asset count
	manifest.AssetCount = len(manifest.Assets)

	// Write manifest
	if err := writeManifestToZip(zipWriter, manifest); err != nil {
		s.logger.Error("Failed to write manifest: %v", err)
	}

	return ZIPBuildResult{
		Manifest:    manifest,
		FailedCount: failedCount,
		Topics:      collectTopics(topicSet),
		TotalSize:   manifest.TotalSize,
	}
}

// collectTopics converts a topic set into a sorted slice
func collectTopics(topicSet map[string]struct{}) []string {
	topics := make([]string, 0, len(topicSet))
	for t := range topicSet {
		topics = append(topics, t)
	}
	return topics
}

func buildFilename(asset *database.Asset, format string, usedNames map[string]int) string {
	// Defense-in-depth: sanitize origin name and extension at output even though
	// input is sanitized at upload, in case of pre-existing unsanitized data
	cleanOrigin := sanitize.OriginName(asset.OriginName)
	cleanExt := sanitize.Extension(asset.Extension)

	var baseName string

	switch format {
	case constants.FilenameFormatHash:
		baseName = asset.AssetID
	case constants.FilenameFormatOriginal:
		baseName = cleanOrigin
		if baseName == "" {
			baseName = asset.AssetID
		}
	case constants.FilenameFormatHashOriginal:
		if cleanOrigin != "" {
			baseName = asset.AssetID + "_" + cleanOrigin
		} else {
			baseName = asset.AssetID
		}
	default:
		baseName = asset.AssetID
	}

	// Build full filename
	filename := baseName
	if cleanExt != "" {
		filename = baseName + "." + cleanExt
	}

	// Handle collisions for original format
	if format == constants.FilenameFormatOriginal {
		originalFilename := filename
		count := usedNames[originalFilename]
		if count > 0 {
			if cleanExt != "" {
				filename = fmt.Sprintf("%s_%d.%s", baseName, count+1, cleanExt)
			} else {
				filename = fmt.Sprintf("%s_%d", baseName, count+1)
			}
		}
		usedNames[originalFilename]++
	}

	return filename
}

func (s *Server) writeAssetToZip(zipWriter *zip.Writer, resolved *services.ResolvedAsset, path string) error {
	// Create ZIP entry header with Store method (no compression for streaming)
	header := &zip.FileHeader{
		Name:   path,
		Method: zip.Store,
	}
	header.SetModTime(time.Unix(resolved.Asset.CreatedAt, 0))

	// Create writer for this entry
	entryWriter, err := zipWriter.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("failed to create zip entry: %w", err)
	}

	// Open .dat file
	datPath := filepath.Join(resolved.TopicPath, resolved.Asset.BlobName)
	f, err := os.Open(datPath)
	if err != nil {
		return fmt.Errorf("failed to open data file: %w", err)
	}
	defer f.Close()

	// Seek to data start (skip header)
	dataStart := resolved.Asset.ByteOffset + int64(constants.HeaderSize)
	if _, err := f.Seek(dataStart, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek in data file: %w", err)
	}

	// Stream data to zip entry
	_, err = io.CopyN(entryWriter, f, resolved.Asset.AssetSize)
	if err != nil {
		return fmt.Errorf("failed to stream data: %w", err)
	}

	return nil
}

func (s *Server) writeMetadataToZip(zipWriter *zip.Writer, resolved *services.ResolvedAsset, path string) error {
	// Get computed metadata
	computedMetadata, err := database.GetMetadataComputed(resolved.TopicDB, resolved.Hash)
	if err != nil {
		return fmt.Errorf("failed to get computed metadata: %w", err)
	}
	if computedMetadata == nil {
		computedMetadata = make(map[string]interface{})
	}

	// Build metadata file content
	metadataFile := AssetMetadataFile{
		Asset: BulkAssetInfo{
			Hash:       resolved.Hash,
			Size:       resolved.Asset.AssetSize,
			Extension:  resolved.Asset.Extension,
			OriginName: resolved.Asset.OriginName,
			ParentID:   resolved.Asset.ParentID,
			CreatedAt:  resolved.Asset.CreatedAt,
			Topic:      resolved.Topic,
			BlobName:   resolved.Asset.BlobName,
		},
		ComputedMetadata: computedMetadata,
	}

	// Serialize to JSON
	jsonBytes, err := json.MarshalIndent(metadataFile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize metadata: %w", err)
	}

	// Create ZIP entry
	header := &zip.FileHeader{
		Name:   path,
		Method: zip.Store,
	}
	header.SetModTime(time.Now())

	entryWriter, err := zipWriter.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("failed to create metadata zip entry: %w", err)
	}

	_, err = entryWriter.Write(jsonBytes)
	return err
}

func writeManifestToZip(zipWriter *zip.Writer, manifest BulkDownloadManifest) error {
	jsonBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize manifest: %w", err)
	}

	header := &zip.FileHeader{
		Name:   constants.ManifestFilename,
		Method: zip.Store,
	}
	header.SetModTime(time.Now())

	entryWriter, err := zipWriter.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("failed to create manifest zip entry: %w", err)
	}

	_, err = entryWriter.Write(jsonBytes)
	return err
}
