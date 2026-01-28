package services

import (
	"fmt"
	"strconv"
	"time"

	"silobang/internal/constants"
	"silobang/internal/database"
	"silobang/internal/logger"
)

// MetadataService handles metadata operations for assets.
type MetadataService struct {
	app    AppState
	logger *logger.Logger
}

// NewMetadataService creates a new metadata service instance.
func NewMetadataService(app AppState, log *logger.Logger) *MetadataService {
	return &MetadataService{
		app:    app,
		logger: log,
	}
}

// AssetMetadata contains asset info and its metadata.
type AssetMetadata struct {
	Asset struct {
		OriginName string  `json:"origin_name"`
		Extension  string  `json:"extension"`
		Size       int64   `json:"size"`
		CreatedAt  int64   `json:"created_at"`
		ParentID   *string `json:"parent_id"`
	} `json:"asset"`
	ComputedMetadata      map[string]interface{}           `json:"computed_metadata"`
	MetadataWithProcessor []database.MetadataWithProcessor `json:"metadata_with_processor"`
}

// MetadataSetRequest represents a request to set or delete metadata.
type MetadataSetRequest struct {
	Op               string      `json:"op"`
	Key              string      `json:"key"`
	Value            interface{} `json:"value"`
	Processor        string      `json:"processor"`
	ProcessorVersion string      `json:"processor_version"`
}

// MetadataSetResult contains the result of a metadata set operation.
type MetadataSetResult struct {
	LogID            int64                  `json:"log_id"`
	ComputedMetadata map[string]interface{} `json:"computed_metadata"`
	TopicName        string                 `json:"topic_name"`
}

// Get retrieves asset info and metadata for a hash.
func (s *MetadataService) Get(hash string) (*AssetMetadata, error) {
	// Validate hash
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

	// Get topic database
	topicDB, err := s.app.GetTopicDB(topicName)
	if err != nil {
		return nil, WrapInternalError(err)
	}

	// Get asset details
	asset, err := database.GetAsset(topicDB, hash)
	if err != nil {
		return nil, WrapInternalError(err)
	}
	if asset == nil {
		return nil, ErrAssetNotFoundWithHash(hash)
	}

	// Get computed metadata
	computed, err := database.GetMetadataComputed(topicDB, hash)
	if err != nil {
		s.logger.Warn("Failed to get computed metadata: %v", err)
		computed = make(map[string]interface{})
	}

	// Get metadata with processor info
	withProcessor, err := database.GetMetadataWithProcessor(topicDB, hash)
	if err != nil {
		s.logger.Warn("Failed to get metadata with processor: %v", err)
		withProcessor = []database.MetadataWithProcessor{}
	}

	result := &AssetMetadata{
		ComputedMetadata:      computed,
		MetadataWithProcessor: withProcessor,
	}
	result.Asset.OriginName = asset.OriginName
	result.Asset.Extension = asset.Extension
	result.Asset.Size = asset.AssetSize
	result.Asset.CreatedAt = asset.CreatedAt
	result.Asset.ParentID = asset.ParentID

	return result, nil
}

// Set sets or deletes metadata for an asset.
func (s *MetadataService) Set(hash string, req *MetadataSetRequest) (*MetadataSetResult, error) {
	// Validate hash
	if len(hash) != constants.HashLength {
		return nil, ErrInvalidHash
	}

	// Validate request
	if err := s.validateMetadataRequest(req); err != nil {
		return nil, err
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

	// Convert value to string
	valueStr, err := s.convertValueToString(req.Op, req.Value)
	if err != nil {
		return nil, err
	}

	// Get topic database
	topicDB, err := s.app.GetTopicDB(topicName)
	if err != nil {
		return nil, WrapInternalError(err)
	}

	// Create metadata log entry
	entry := database.MetadataLogEntry{
		AssetID:          hash,
		Op:               req.Op,
		Key:              req.Key,
		Value:            valueStr,
		Processor:        req.Processor,
		ProcessorVersion: req.ProcessorVersion,
		Timestamp:        time.Now().Unix(),
	}

	logID, err := database.InsertMetadataLog(topicDB, entry)
	if err != nil {
		return nil, WrapMetadataError(err)
	}

	// Get updated computed metadata
	computed, err := database.GetMetadataComputed(topicDB, hash)
	if err != nil {
		s.logger.Warn("Failed to get computed metadata: %v", err)
		computed = make(map[string]interface{})
	}

	return &MetadataSetResult{
		LogID:            logID,
		ComputedMetadata: computed,
		TopicName:        topicName,
	}, nil

}

// validateMetadataRequest validates a metadata set request.
func (s *MetadataService) validateMetadataRequest(req *MetadataSetRequest) error {
	if req.Op != constants.BatchMetadataOpSet && req.Op != constants.BatchMetadataOpDelete {
		return NewServiceError(constants.ErrCodeInvalidRequest, "op must be 'set' or 'delete'")
	}
	if req.Key == "" {
		return NewServiceError(constants.ErrCodeInvalidRequest, "key is required")
	}
	if len(req.Key) > constants.MaxMetadataKeyLength {
		return ErrMetadataKeyTooLong
	}
	if req.Processor == "" {
		return NewServiceError(constants.ErrCodeInvalidRequest, "processor is required")
	}
	if req.ProcessorVersion == "" {
		return NewServiceError(constants.ErrCodeInvalidRequest, "processor_version is required")
	}
	return nil
}

// convertValueToString converts a metadata value to string.
func (s *MetadataService) convertValueToString(op string, value interface{}) (string, error) {
	if op != constants.BatchMetadataOpSet {
		return "", nil
	}

	if value == nil {
		return "", NewServiceError(constants.ErrCodeInvalidRequest, "value is required for set operation")
	}

	var valueStr string
	switch v := value.(type) {
	case string:
		valueStr = v
	case float64:
		valueStr = strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		valueStr = strconv.FormatBool(v)
	default:
		return "", NewServiceError(constants.ErrCodeInvalidRequest, "value must be string, number, or boolean")
	}

	if len(valueStr) > s.app.GetConfig().Metadata.MaxValueBytes {
		return "", ErrMetadataValueTooLong
	}

	return valueStr, nil
}

// GetTopicForHash returns the topic name for a given hash.
// This is a helper for batch operations.
func (s *MetadataService) GetTopicForHash(hash string) (string, error) {
	exists, topicName, _, err := database.CheckHashExists(s.app.GetOrchestratorDB(), hash)
	if err != nil {
		return "", WrapInternalError(err)
	}
	if !exists {
		return "", ErrAssetNotFoundWithHash(hash)
	}
	return topicName, nil
}

// ValidateValueForBatch validates and converts a value for batch operations.
func (s *MetadataService) ValidateValueForBatch(op string, value interface{}) (string, error) {
	return s.convertValueToString(op, value)
}

// ValidateKeyLength validates metadata key length.
func (s *MetadataService) ValidateKeyLength(key string) error {
	if len(key) > constants.MaxMetadataKeyLength {
		return fmt.Errorf("key '%s' exceeds maximum length of %d characters", key, constants.MaxMetadataKeyLength)
	}
	return nil
}
