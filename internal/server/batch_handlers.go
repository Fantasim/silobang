package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"silobang/internal/audit"
	"silobang/internal/auth"
	"silobang/internal/constants"
	"silobang/internal/database"
	"silobang/internal/queries"
)

// =============================================================================
// Batch Metadata Handlers
// =============================================================================

// BatchMetadataRequest represents the request body for POST /api/metadata/batch
type BatchMetadataRequest struct {
	Operations       []BatchOperationInput `json:"operations"`
	Processor        string                `json:"processor"`
	ProcessorVersion string                `json:"processor_version"`
}

// BatchOperationInput represents a single operation in the batch request
type BatchOperationInput struct {
	Hash  string      `json:"hash"`
	Op    string      `json:"op"` // "set" | "delete"
	Key   string      `json:"key"`
	Value interface{} `json:"value,omitempty"`
}

// BatchMetadataResponse represents the response for batch operations
type BatchMetadataResponse struct {
	Success   bool                           `json:"success"`
	Total     int                            `json:"total"`
	Succeeded int                            `json:"succeeded"`
	Failed    int                            `json:"failed"`
	Results   []database.BatchOperationResult `json:"results"`
}

// ApplyMetadataRequest represents the request body for POST /api/metadata/apply
type ApplyMetadataRequest struct {
	QueryPreset      string                 `json:"query_preset"`
	QueryParams      map[string]interface{} `json:"query_params,omitempty"`
	Topics           []string               `json:"topics,omitempty"`
	Op               string                 `json:"op"`
	Key              string                 `json:"key"`
	Value            interface{}            `json:"value,omitempty"`
	Processor        string                 `json:"processor"`
	ProcessorVersion string                 `json:"processor_version"`
}

// handleBatchMetadata handles POST /api/metadata/batch
func (s *Server) handleBatchMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	identity := s.requireAuth(w, r)
	if identity == nil {
		return
	}

	if !s.authorize(w, identity, &auth.ActionContext{Action: constants.AuthActionMetadata}) {
		return
	}

	// Check if configured
	if s.app.Config.WorkingDirectory == "" {
		WriteError(w, http.StatusBadRequest, "Working directory not configured", constants.ErrCodeNotConfigured)
		return
	}

	// Parse request
	var req BatchMetadataRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid JSON", constants.ErrCodeInvalidRequest)
		return
	}

	// Validate request
	if len(req.Operations) == 0 {
		WriteError(w, http.StatusBadRequest, "No operations provided", constants.ErrCodeInvalidRequest)
		return
	}

	if len(req.Operations) > s.app.Config.Batch.MaxOperations {
		WriteError(w, http.StatusBadRequest, "Too many operations", constants.ErrCodeBatchTooManyOperations)
		return
	}

	if req.Processor == "" {
		req.Processor = constants.ProcessorAPI
	}
	if req.ProcessorVersion == "" {
		req.ProcessorVersion = "1.0"
	}

	// Convert input operations to database operations
	dbOperations := make([]database.BatchOperation, len(req.Operations))
	for i, op := range req.Operations {
		// Validate hash format
		if len(op.Hash) != constants.HashLength {
			WriteError(w, http.StatusBadRequest, "Invalid hash: "+op.Hash, constants.ErrCodeInvalidHash)
			return
		}

		// Convert value to string
		valueStr := ""
		if op.Value != nil {
			switch v := op.Value.(type) {
			case string:
				valueStr = v
			case float64:
				valueStr = formatFloat(v)
			case bool:
				if v {
					valueStr = "true"
				} else {
					valueStr = "false"
				}
			default:
				// JSON encode other types
				bytes, _ := json.Marshal(v)
				valueStr = string(bytes)
			}
		}

		dbOperations[i] = database.BatchOperation{
			Hash:             op.Hash,
			Op:               op.Op,
			Key:              op.Key,
			Value:            valueStr,
			Processor:        req.Processor,
			ProcessorVersion: req.ProcessorVersion,
		}
	}

	// Group operations by topic
	grouped, notFound := database.GroupOperationsByTopic(s.app.OrchestratorDB, dbOperations)

	s.logger.Info("Batch metadata: %d operations across %d topics, %d not found", len(dbOperations), len(grouped), len(notFound))

	// Execute operations per topic atomically
	allResults := make([]database.BatchOperationResult, 0, len(dbOperations))
	allResults = append(allResults, notFound...)

	for _, group := range grouped {
		s.logger.Debug("Processing batch for topic %s: %d operations", group.Topic, len(group.Operations))

		topicDB, err := s.app.GetTopicDB(group.Topic)
		if err != nil {
			s.logger.Warn("Topic DB unavailable for batch: %s", group.Topic)
			// Topic DB unavailable, mark all as failed
			for _, op := range group.Operations {
				allResults = append(allResults, database.BatchOperationResult{
					Hash:    op.Hash,
					Success: false,
					Error:   "topic database unavailable",
				})
			}
			continue
		}

		// Begin transaction for atomic execution
		tx, err := topicDB.Begin()
		if err != nil {
			s.logger.Error("Failed to begin transaction for topic %s: %v", group.Topic, err)
			for _, op := range group.Operations {
				allResults = append(allResults, database.BatchOperationResult{
					Hash:    op.Hash,
					Success: false,
					Error:   "failed to begin transaction",
				})
			}
			continue
		}

		results, err := database.ExecuteBatchMetadataTx(tx, group.Operations, s.app.Config.Metadata.MaxValueBytes)
		if err != nil {
			tx.Rollback()
			s.logger.Error("Batch execution failed for topic %s: %v", group.Topic, err)
			for _, op := range group.Operations {
				allResults = append(allResults, database.BatchOperationResult{
					Hash:    op.Hash,
					Success: false,
					Error:   "batch execution failed: " + err.Error(),
				})
			}
			continue
		}

		// Commit transaction
		if err := tx.Commit(); err != nil {
			s.logger.Error("Commit failed for topic %s: %v", group.Topic, err)
			for _, op := range group.Operations {
				allResults = append(allResults, database.BatchOperationResult{
					Hash:    op.Hash,
					Success: false,
					Error:   "commit failed: " + err.Error(),
				})
			}
			continue
		}

		s.logger.Debug("Batch completed for topic %s: %d operations", group.Topic, len(results))
		allResults = append(allResults, results...)
	}

	// Count successes and failures
	succeeded := 0
	failed := 0
	for _, r := range allResults {
		if r.Success {
			succeeded++
		} else {
			failed++
		}
	}

	s.logger.Info("Batch metadata complete: %d succeeded, %d failed", succeeded, failed)

	// Audit batch metadata operation
	if s.app.AuditLogger != nil {
		s.app.AuditLogger.Log(constants.AuditActionMetadataBatch, getClientIP(r), getAuditUsername(identity), audit.MetadataBatchDetails{
			OperationCount: len(req.Operations),
			Succeeded:      succeeded,
			Failed:         failed,
			Processor:      req.Processor,
		})
	}

	response := BatchMetadataResponse{
		Success:   failed == 0,
		Total:     len(allResults),
		Succeeded: succeeded,
		Failed:    failed,
		Results:   allResults,
	}

	WriteSuccess(w, response)
}

// handleApplyMetadata handles POST /api/metadata/apply
func (s *Server) handleApplyMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	identity := s.requireAuth(w, r)
	if identity == nil {
		return
	}

	if !s.authorize(w, identity, &auth.ActionContext{Action: constants.AuthActionMetadata}) {
		return
	}

	// Check if configured
	if s.app.Config.WorkingDirectory == "" {
		WriteError(w, http.StatusBadRequest, "Working directory not configured", constants.ErrCodeNotConfigured)
		return
	}

	// Parse request
	var req ApplyMetadataRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid JSON", constants.ErrCodeInvalidRequest)
		return
	}

	// Validate request
	if req.QueryPreset == "" {
		WriteError(w, http.StatusBadRequest, "query_preset is required", constants.ErrCodeInvalidRequest)
		return
	}
	if req.Key == "" {
		WriteError(w, http.StatusBadRequest, "key is required", constants.ErrCodeInvalidRequest)
		return
	}
	if len(req.Key) > constants.MaxMetadataKeyLength {
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("key exceeds maximum length of %d characters", constants.MaxMetadataKeyLength), constants.ErrCodeMetadataKeyTooLong)
		return
	}
	if req.Op != constants.BatchMetadataOpSet && req.Op != constants.BatchMetadataOpDelete {
		WriteError(w, http.StatusBadRequest, "op must be 'set' or 'delete'", constants.ErrCodeBatchInvalidOperation)
		return
	}

	if req.Processor == "" {
		req.Processor = constants.ProcessorAPI
	}
	if req.ProcessorVersion == "" {
		req.ProcessorVersion = "1.0"
	}

	// Load query preset
	preset, err := s.app.QueriesConfig.GetPreset(req.QueryPreset)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid query preset: "+req.QueryPreset, constants.ErrCodePresetNotFound)
		return
	}

	// Convert params
	params := queries.ParamsToStrings(req.QueryParams)
	validatedParams, err := queries.ValidateParams(preset, params)
	if err != nil {
		WriteError(w, http.StatusBadRequest, err.Error(), constants.ErrCodeMissingParam)
		return
	}

	// Get topics to query
	topicNames := req.Topics
	if len(topicNames) == 0 {
		// Use all registered topics
		topicNames = s.app.ListTopics()
	}

	// Collect all topic DBs
	topicDBs, validNames, err := s.app.GetTopicDBsForQuery(topicNames)
	if err != nil {
		// Log warning but continue with available topics
		s.logger.Warn("Some topics unavailable: %v", err)
		// Try to get whatever topics we can
		topicDBs = make(map[string]*sql.DB)
		validNames = []string{}
		for _, name := range topicNames {
			db, err := s.app.GetTopicDB(name)
			if err == nil {
				topicDBs[name] = db
				validNames = append(validNames, name)
			}
		}
	}

	// Execute query to get asset hashes
	result, err := queries.ExecuteCrossTopicQuery(preset, validatedParams, topicDBs, validNames)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "Query execution failed: "+err.Error(), constants.ErrCodeQueryError)
		return
	}

	// Find hash column index
	hashColIdx := -1
	for i, col := range result.Columns {
		if col == "hash" || col == "asset_id" {
			hashColIdx = i
			break
		}
	}

	if hashColIdx == -1 {
		WriteError(w, http.StatusBadRequest, "Query must return a 'hash' or 'asset_id' column", constants.ErrCodeInvalidRequest)
		return
	}

	// Note: _topic column is added by ExecuteCrossTopicQuery but we don't need it
	// since we re-lookup topics via orchestrator for correctness

	// Convert value to string
	valueStr := ""
	if req.Value != nil {
		switch v := req.Value.(type) {
		case string:
			valueStr = v
		case float64:
			valueStr = formatFloat(v)
		case bool:
			if v {
				valueStr = "true"
			} else {
				valueStr = "false"
			}
		default:
			bytes, _ := json.Marshal(v)
			valueStr = string(bytes)
		}
		maxValueBytes := s.app.Config.Metadata.MaxValueBytes
		if len(valueStr) > maxValueBytes {
			WriteError(w, http.StatusBadRequest, fmt.Sprintf("value exceeds maximum size of %d bytes", maxValueBytes), constants.ErrCodeMetadataValueTooLong)
			return
		}
	}

	// Build operations from query results
	operations := make([]database.BatchOperation, 0, len(result.Rows))
	for _, row := range result.Rows {
		if hashColIdx >= len(row) {
			continue
		}
		hash, ok := row[hashColIdx].(string)
		if !ok {
			continue
		}

		operations = append(operations, database.BatchOperation{
			Hash:             hash,
			Op:               req.Op,
			Key:              req.Key,
			Value:            valueStr,
			Processor:        req.Processor,
			ProcessorVersion: req.ProcessorVersion,
		})
	}

	if len(operations) == 0 {
		WriteSuccess(w, BatchMetadataResponse{
			Success:   true,
			Total:     0,
			Succeeded: 0,
			Failed:    0,
			Results:   []database.BatchOperationResult{},
		})
		return
	}

	// Group operations by topic (re-lookup to ensure correctness)
	grouped, notFound := database.GroupOperationsByTopic(s.app.OrchestratorDB, operations)

	s.logger.Info("Apply metadata: preset=%s, key=%s, %d operations across %d topics", req.QueryPreset, req.Key, len(operations), len(grouped))

	// Execute operations per topic atomically
	allResults := make([]database.BatchOperationResult, 0, len(operations))
	allResults = append(allResults, notFound...)

	for _, group := range grouped {
		topicDB, exists := topicDBs[group.Topic]
		if !exists {
			for _, op := range group.Operations {
				allResults = append(allResults, database.BatchOperationResult{
					Hash:    op.Hash,
					Success: false,
					Error:   "topic database unavailable",
				})
			}
			continue
		}

		tx, err := topicDB.Begin()
		if err != nil {
			for _, op := range group.Operations {
				allResults = append(allResults, database.BatchOperationResult{
					Hash:    op.Hash,
					Success: false,
					Error:   "failed to begin transaction",
				})
			}
			continue
		}

		results, err := database.ExecuteBatchMetadataTx(tx, group.Operations, s.app.Config.Metadata.MaxValueBytes)
		if err != nil {
			tx.Rollback()
			for _, op := range group.Operations {
				allResults = append(allResults, database.BatchOperationResult{
					Hash:    op.Hash,
					Success: false,
					Error:   "batch execution failed: " + err.Error(),
				})
			}
			continue
		}

		if err := tx.Commit(); err != nil {
			for _, op := range group.Operations {
				allResults = append(allResults, database.BatchOperationResult{
					Hash:    op.Hash,
					Success: false,
					Error:   "commit failed: " + err.Error(),
				})
			}
			continue
		}

		allResults = append(allResults, results...)
	}

	// Count successes and failures
	succeeded := 0
	failed := 0
	for _, r := range allResults {
		if r.Success {
			succeeded++
		} else {
			failed++
		}
	}

	s.logger.Info("Apply metadata complete: %d succeeded, %d failed", succeeded, failed)

	// Audit apply metadata operation
	if s.app.AuditLogger != nil {
		s.app.AuditLogger.Log(constants.AuditActionMetadataApply, getClientIP(r), getAuditUsername(identity), audit.MetadataApplyDetails{
			QueryPreset:    req.QueryPreset,
			Op:             req.Op,
			Key:            req.Key,
			OperationCount: len(operations),
			Succeeded:      succeeded,
			Failed:         failed,
			Processor:      req.Processor,
		})
	}

	response := BatchMetadataResponse{
		Success:   failed == 0,
		Total:     len(allResults),
		Succeeded: succeeded,
		Failed:    failed,
		Results:   allResults,
	}

	WriteSuccess(w, response)
}

// formatFloat converts a float64 to string, preserving integer format when possible
func formatFloat(f float64) string {
	if f == float64(int64(f)) {
		return fmt.Sprintf("%d", int64(f))
	}
	return fmt.Sprintf("%g", f)
}
