package services

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"

	"meshbank/internal/constants"
	"meshbank/internal/database"
	"meshbank/internal/logger"
	"meshbank/internal/storage"
)

// VerifyService handles verification of DAT files and index consistency.
type VerifyService struct {
	app    AppState
	logger *logger.Logger
}

// NewVerifyService creates a new verify service instance.
func NewVerifyService(app AppState, log *logger.Logger) *VerifyService {
	return &VerifyService{
		app:    app,
		logger: log,
	}
}

// DatFileResult contains the result of verifying a single DAT file.
type DatFileResult struct {
	DatFile    string
	Valid      bool
	EntryCount int
	Error      string
}

// TopicResult contains the result of verifying a topic.
type TopicResult struct {
	TopicName       string
	Valid           bool
	DatFilesChecked int
	Errors          []string
	DatResults      []DatFileResult
}

// IndexIssue represents an issue found during index verification.
type IndexIssue struct {
	Type    string // "orphan", "missing", "mismatch"
	Hash    string
	Topic   string
	DatFile string
	Detail  string
}

// IndexResult contains the result of index verification.
type IndexResult struct {
	Valid         bool
	OrphanCount   int
	MissingCount  int
	MismatchCount int
	Issues        []IndexIssue
}

// DatProgressCallback is called during DAT file verification with progress updates.
type DatProgressCallback func(entriesProcessed, totalEntries int) error

// TopicProgressCallback is called when verification of a topic starts or a DAT file completes.
type TopicProgressCallback func(event string, data interface{}) error

// ListDatFiles returns the list of DAT files for a topic.
func (s *VerifyService) ListDatFiles(topicName string) ([]string, error) {
	topicPath := s.app.GetTopicPath(topicName)
	return storage.ListDatFiles(topicPath)
}

// VerifyDatFile verifies a single DAT file and returns the result.
// The progressCallback is called periodically to report progress (can be nil).
func (s *VerifyService) VerifyDatFile(
	ctx context.Context,
	topicName string,
	datFile string,
	progressInterval int,
	progressCallback DatProgressCallback,
) (*DatFileResult, error) {
	topicPath := s.app.GetTopicPath(topicName)

	// Get topic database
	topicDB, err := s.app.GetTopicDB(topicName)
	if err != nil {
		return &DatFileResult{
			DatFile: datFile,
			Valid:   false,
			Error:   fmt.Sprintf("failed to open database: %v", err),
		}, nil
	}

	// Get stored hash from database
	storedHash, storedCount, err := database.GetDatHash(topicDB, datFile)
	if err != nil {
		return &DatFileResult{
			DatFile: datFile,
			Valid:   false,
			Error:   fmt.Sprintf("failed to get stored hash: %v", err),
		}, nil
	}
	if storedHash == "" {
		return &DatFileResult{
			DatFile: datFile,
			Valid:   false,
			Error:   "no hash record found",
		}, nil
	}

	datPath := filepath.Join(topicPath, datFile)

	// Create internal progress callback that wraps the external one
	var internalCallback func(entriesProcessed int) error
	if progressCallback != nil {
		internalCallback = func(entriesProcessed int) error {
			// Check for cancellation
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			return progressCallback(entriesProcessed, int(storedCount))
		}
	}

	// Replay hash chain with progress
	computedHash, computedCount, err := storage.ReplayRunningHashWithProgress(datPath, progressInterval, internalCallback)
	if err != nil {
		if err == context.Canceled {
			return &DatFileResult{
				DatFile: datFile,
				Valid:   false,
				Error:   "cancelled",
			}, nil
		}
		return &DatFileResult{
			DatFile: datFile,
			Valid:   false,
			Error:   fmt.Sprintf("failed to replay hash: %v", err),
		}, nil
	}

	// Compare
	if computedHash != storedHash {
		s.logger.Warn("Hash mismatch in %s/%s: stored=%s computed=%s", topicName, datFile, storedHash[:16]+"...", computedHash[:16]+"...")
		return &DatFileResult{
			DatFile:    datFile,
			Valid:      false,
			EntryCount: computedCount,
			Error:      "hash mismatch",
		}, nil
	}
	if computedCount != int(storedCount) {
		s.logger.Warn("Entry count mismatch in %s/%s: expected %d, got %d", topicName, datFile, storedCount, computedCount)
		return &DatFileResult{
			DatFile:    datFile,
			Valid:      false,
			EntryCount: computedCount,
			Error:      fmt.Sprintf("entry count mismatch: expected %d, got %d", storedCount, computedCount),
		}, nil
	}

	return &DatFileResult{
		DatFile:    datFile,
		Valid:      true,
		EntryCount: computedCount,
	}, nil
}

// VerifyTopic verifies all DAT files in a topic.
func (s *VerifyService) VerifyTopic(
	ctx context.Context,
	topicName string,
	progressInterval int,
	datProgressCallback DatProgressCallback,
) (*TopicResult, error) {
	topicPath := s.app.GetTopicPath(topicName)

	// List .dat files
	datFiles, err := storage.ListDatFiles(topicPath)
	if err != nil {
		return nil, WrapInternalError(fmt.Errorf("failed to list dat files: %w", err))
	}

	result := &TopicResult{
		TopicName:  topicName,
		Valid:      true,
		DatResults: make([]DatFileResult, 0, len(datFiles)),
	}

	// Verify each .dat file
	for _, datFile := range datFiles {
		// Check for cancellation
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		datResult, err := s.VerifyDatFile(ctx, topicName, datFile, progressInterval, datProgressCallback)
		if err != nil {
			return result, err
		}

		result.DatResults = append(result.DatResults, *datResult)
		result.DatFilesChecked++

		if !datResult.Valid {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", datFile, datResult.Error))
		}
	}

	return result, nil
}

// VerifyIndex verifies the orchestrator index consistency.
func (s *VerifyService) VerifyIndex(ctx context.Context, topics []string) (*IndexResult, error) {
	orchDB := s.app.GetOrchestratorDB()
	if orchDB == nil {
		return nil, NewServiceError(constants.ErrCodeNotConfigured, "orchestrator database not available")
	}

	result := &IndexResult{
		Valid:  true,
		Issues: make([]IndexIssue, 0),
	}

	// Build set of valid topics for quick lookup
	topicSet := make(map[string]bool)
	for _, t := range topics {
		topicSet[t] = true
	}

	// Check each orchestrator entry
	rows, err := orchDB.Query("SELECT hash, topic, dat_file FROM asset_index")
	if err != nil {
		return nil, WrapInternalError(fmt.Errorf("failed to query index: %w", err))
	}
	defer rows.Close()

	for rows.Next() {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		var hash, topic, datFile string
		if err := rows.Scan(&hash, &topic, &datFile); err != nil {
			continue
		}

		// Skip topics not in verification scope
		if !topicSet[topic] {
			continue
		}

		// Check if topic exists and is healthy
		healthy, _ := s.app.IsTopicHealthy(topic)
		if !healthy {
			result.OrphanCount++
			result.Issues = append(result.Issues, IndexIssue{
				Type:   "orphan",
				Hash:   hash,
				Topic:  topic,
				Detail: "topic unhealthy or missing",
			})
			continue
		}

		// Verify asset exists in topic database
		topicDB, err := s.app.GetTopicDB(topic)
		if err != nil {
			continue
		}

		asset, err := database.GetAsset(topicDB, hash)
		if err != nil || asset == nil {
			result.MissingCount++
			result.Issues = append(result.Issues, IndexIssue{
				Type:   "missing",
				Hash:   hash,
				Topic:  topic,
				Detail: "not found in topic database",
			})
			continue
		}

		// Verify dat_file matches
		if asset.BlobName != datFile {
			result.MismatchCount++
			result.Issues = append(result.Issues, IndexIssue{
				Type:    "mismatch",
				Hash:    hash,
				Topic:   topic,
				DatFile: datFile,
				Detail:  fmt.Sprintf("orchestrator says %s, topic says %s", datFile, asset.BlobName),
			})
		}
	}

	// Limit issues in response
	if len(result.Issues) > constants.MaxVerifyIssuesInResponse {
		result.Issues = result.Issues[:constants.MaxVerifyIssuesInResponse]
	}

	result.Valid = result.OrphanCount == 0 && result.MissingCount == 0 && result.MismatchCount == 0

	return result, nil
}

// GetTotalIndexEntries returns the total number of entries in the index.
func (s *VerifyService) GetTotalIndexEntries() (int, error) {
	orchDB := s.app.GetOrchestratorDB()
	if orchDB == nil {
		return 0, NewServiceError(constants.ErrCodeNotConfigured, "orchestrator database not available")
	}

	var count int
	err := orchDB.QueryRow("SELECT COUNT(*) FROM asset_index").Scan(&count)
	if err != nil {
		return 0, WrapInternalError(err)
	}
	return count, nil
}

// GetTopicDB returns the database for a topic (helper for handlers).
func (s *VerifyService) GetTopicDB(topicName string) (*sql.DB, error) {
	return s.app.GetTopicDB(topicName)
}
