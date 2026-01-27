package services

import (
	"database/sql"
	"fmt"

	"silobang/internal/constants"
	"silobang/internal/database"
	"silobang/internal/logger"
	"silobang/internal/queries"
)

// BulkService handles bulk download asset resolution and validation.
type BulkService struct {
	app    AppState
	logger *logger.Logger
}

// NewBulkService creates a new bulk service instance.
func NewBulkService(app AppState, log *logger.Logger) *BulkService {
	return &BulkService{
		app:    app,
		logger: log,
	}
}

// ResolvedAsset contains all info needed to download an asset.
type ResolvedAsset struct {
	Hash      string
	Topic     string
	Asset     *database.Asset
	TopicPath string
	TopicDB   *sql.DB
}

// BulkResolveRequest contains parameters for resolving assets.
type BulkResolveRequest struct {
	Mode           string                 // "query" | "ids"
	Preset         string                 // for mode="query"
	Params         map[string]interface{} // for mode="query"
	Topics         []string               // for mode="query", optional
	AssetIDs       []string               // for mode="ids"
	FilenameFormat string                 // "hash" | "original" | "hash_original"
}

// ValidateRequest validates a bulk download request.
func (s *BulkService) ValidateRequest(req *BulkResolveRequest) error {
	// Check filename format
	if req.FilenameFormat == "" {
		req.FilenameFormat = constants.DefaultFilenameFormat
	}

	if !s.isValidFilenameFormat(req.FilenameFormat) {
		return NewServiceError(constants.ErrCodeInvalidFilenameFormat,
			"invalid filename_format: must be hash, original, or hash_original")
	}

	// Validate mode
	switch req.Mode {
	case "query":
		if req.Preset == "" {
			return NewServiceError(constants.ErrCodeInvalidRequest, "preset is required for mode=query")
		}
	case "ids":
		if len(req.AssetIDs) == 0 {
			return NewServiceError(constants.ErrCodeInvalidRequest, "asset_ids is required for mode=ids")
		}
	default:
		return NewServiceError(constants.ErrCodeInvalidDownloadMode, "invalid mode: must be query or ids")
	}

	return nil
}

// ResolveAssets resolves assets based on the request mode.
// Returns the resolved assets and any error.
func (s *BulkService) ResolveAssets(req *BulkResolveRequest) ([]*ResolvedAsset, error) {
	switch req.Mode {
	case "query":
		return s.resolveFromQuery(req)
	case "ids":
		return s.resolveFromIDs(req.AssetIDs)
	default:
		return nil, NewServiceError(constants.ErrCodeInvalidDownloadMode, "invalid mode: must be query or ids")
	}
}

// resolveFromQuery resolves assets from a query preset.
func (s *BulkService) resolveFromQuery(req *BulkResolveRequest) ([]*ResolvedAsset, error) {
	qc := s.app.GetQueriesConfig()
	if qc == nil {
		return nil, NewServiceError(constants.ErrCodeNotConfigured, "queries config not loaded")
	}

	preset, err := qc.GetPreset(req.Preset)
	if err != nil {
		// Return INVALID_REQUEST for backward compatibility with original bulk download behavior
		return nil, NewServiceError(constants.ErrCodeInvalidRequest,
			fmt.Sprintf("preset not found: %s", req.Preset))
	}

	// Convert params and validate
	stringParams := queries.ParamsToStrings(req.Params)
	params, err := queries.ValidateParams(preset, stringParams)
	if err != nil {
		return nil, NewServiceError(constants.ErrCodeInvalidRequest, err.Error())
	}

	// Get topic databases
	topicDBs, topicNames, err := s.app.GetTopicDBsForQuery(req.Topics)
	if err != nil {
		return nil, WrapInternalError(err)
	}

	if len(topicNames) == 0 {
		return nil, nil
	}

	// Execute query
	result, err := queries.ExecuteCrossTopicQuery(preset, params, topicDBs, topicNames)
	if err != nil {
		return nil, WrapInternalError(fmt.Errorf("query execution failed: %w", err))
	}

	// Find asset_id column index
	assetIDIdx := -1
	topicIdx := -1
	for i, col := range result.Columns {
		if col == "asset_id" {
			assetIDIdx = i
		}
		if col == "_topic" {
			topicIdx = i
		}
	}

	if assetIDIdx == -1 {
		return nil, NewServiceError(constants.ErrCodeInvalidRequest, "query must return asset_id column")
	}

	// Resolve each asset
	var assets []*ResolvedAsset
	for _, row := range result.Rows {
		hash, ok := row[assetIDIdx].(string)
		if !ok {
			continue
		}

		var topic string
		if topicIdx != -1 {
			topic, _ = row[topicIdx].(string)
		}

		resolved, err := s.resolveAsset(hash, topic)
		if err != nil {
			s.logger.Debug("Skipping asset %s: %v", hash, err)
			continue // Skip failed assets
		}
		assets = append(assets, resolved)
	}

	return assets, nil
}

// resolveFromIDs resolves assets from a list of asset IDs.
func (s *BulkService) resolveFromIDs(assetIDs []string) ([]*ResolvedAsset, error) {
	if len(assetIDs) == 0 {
		return nil, NewServiceError(constants.ErrCodeInvalidRequest, "asset_ids is required for mode=ids")
	}

	var assets []*ResolvedAsset
	for _, hash := range assetIDs {
		resolved, err := s.resolveAsset(hash, "")
		if err != nil {
			s.logger.Debug("Skipping asset %s: %v", hash, err)
			continue // Skip failed assets
		}
		assets = append(assets, resolved)
	}

	return assets, nil
}

// resolveAsset resolves a single asset by hash.
func (s *BulkService) resolveAsset(hash, knownTopic string) (*ResolvedAsset, error) {
	var topicName string

	if knownTopic != "" {
		topicName = knownTopic
	} else {
		// Look up in orchestrator to find topic
		exists, topic, _, err := database.CheckHashExists(s.app.GetOrchestratorDB(), hash)
		if err != nil {
			return nil, WrapInternalError(fmt.Errorf("failed to lookup asset: %w", err))
		}
		if !exists {
			return nil, ErrAssetNotFoundWithHash(hash)
		}
		topicName = topic
	}

	// Check topic health
	healthy, errMsg := s.app.IsTopicHealthy(topicName)
	if !healthy {
		return nil, ErrTopicUnhealthyWithReason(topicName, errMsg)
	}

	// Get topic DB and path
	topicDB, err := s.app.GetTopicDB(topicName)
	if err != nil {
		return nil, WrapInternalError(fmt.Errorf("failed to access topic: %w", err))
	}

	topicPath := s.app.GetTopicPath(topicName)

	// Get asset details
	asset, err := database.GetAsset(topicDB, hash)
	if err != nil {
		return nil, WrapInternalError(fmt.Errorf("failed to get asset: %w", err))
	}
	if asset == nil {
		return nil, ErrAssetNotFoundWithHash(hash)
	}

	return &ResolvedAsset{
		Hash:      hash,
		Topic:     topicName,
		Asset:     asset,
		TopicPath: topicPath,
		TopicDB:   topicDB,
	}, nil
}

// ValidateAssetCount checks if the number of assets is within limits.
func (s *BulkService) ValidateAssetCount(count int) error {
	if count == 0 {
		return NewServiceError(constants.ErrCodeBulkDownloadEmpty, "no assets found matching the request")
	}
	maxAssets := s.app.GetConfig().BulkDownload.MaxAssets
	if count > maxAssets {
		return NewServiceError(constants.ErrCodeBulkDownloadTooLarge,
			fmt.Sprintf("too many assets: %d (max: %d)", count, maxAssets))
	}
	return nil
}

// isValidFilenameFormat checks if the filename format is valid.
func (s *BulkService) isValidFilenameFormat(format string) bool {
	return format == constants.FilenameFormatHash ||
		format == constants.FilenameFormatOriginal ||
		format == constants.FilenameFormatHashOriginal
}
