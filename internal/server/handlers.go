package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	"silobang/internal/audit"
	"silobang/internal/auth"
	"silobang/internal/constants"
	"silobang/internal/sanitize"
	"silobang/internal/services"
)

var topicNameRegex = regexp.MustCompile(constants.TopicNameRegex)

// =============================================================================
// Config Handlers
// =============================================================================

// GET /api/config - Get current configuration status
// POST /api/config - Update configuration
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.getConfig(w, r)
	case http.MethodPost:
		s.postConfig(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) getConfig(w http.ResponseWriter, r *http.Request) {
	// Auth check: manage_config required (skip if auth not available — initial setup)
	if s.isAuthAvailable() {
		identity := s.requireAuth(w, r)
		if identity == nil {
			return
		}
		if !s.authorize(w, identity, &auth.ActionContext{Action: constants.AuthActionManageConfig}) {
			return
		}
	}

	status := s.app.Services.Config.GetStatus()
	WriteSuccess(w, status)
}

func (s *Server) postConfig(w http.ResponseWriter, r *http.Request) {
	// Auth check: manage_config required (skip if auth not available — initial setup)
	if s.isAuthAvailable() {
		identity := s.requireAuth(w, r)
		if identity == nil {
			return
		}
		if !s.authorize(w, identity, &auth.ActionContext{Action: constants.AuthActionManageConfig}) {
			return
		}
	}

	var req struct {
		WorkingDirectory string `json:"working_directory"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid JSON", constants.ErrCodeInvalidRequest)
		return
	}

	// Call service
	if err := s.app.Services.Config.SetWorkingDirectory(req.WorkingDirectory, s.app.Config.Port); err != nil {
		s.handleServiceError(w, err)
		return
	}

	// Initialize audit logger (needs to be done in handler as it's server-specific)
	s.app.AuditLogger = s.app.Services.Config.SetAuditLogger()

	// Re-initialize services so AuthService picks up the new orchestrator DB
	s.app.ReinitServices()

	// Bootstrap auth if this is first-time setup (no users yet)
	response := map[string]interface{}{"success": true}
	isBootstrap := false
	if s.app.Services.Auth != nil {
		bootstrapResult, err := auth.Bootstrap(s.app.Services.Auth.GetStore(), s.logger)
		if err != nil {
			s.logger.Error("Auth bootstrap failed during config: %v", err)
		} else if bootstrapResult != nil {
			isBootstrap = true
			s.logger.Info("Auth: bootstrap completed via config API — admin user created")
			response["bootstrap"] = map[string]interface{}{
				"username": bootstrapResult.Username,
				"password": bootstrapResult.Password,
				"api_key":  bootstrapResult.APIKey,
			}
		}
	}

	// Audit config change (audit logger was just initialized above)
	if s.app.AuditLogger != nil {
		auditUsername := ""
		if s.isAuthAvailable() {
			if identity, ok := auth.RequireAuth(r); ok {
				auditUsername = getAuditUsername(identity)
			}
		}
		s.app.AuditLogger.Log(constants.AuditActionConfigChanged, getClientIP(r), auditUsername, audit.ConfigChangedDetails{
			WorkingDirectory: req.WorkingDirectory,
			IsBootstrap:      isBootstrap,
		})
	}

	WriteSuccess(w, response)
}

// =============================================================================
// Topics Handlers
// =============================================================================

// GET /api/topics - List all topics with stats
// POST /api/topics - Create new topic
func (s *Server) handleTopics(w http.ResponseWriter, r *http.Request) {
	// Check if configured
	if s.app.Config.WorkingDirectory == "" {
		WriteError(w, http.StatusBadRequest, "Working directory not configured", constants.ErrCodeNotConfigured)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.listTopics(w, r)
	case http.MethodPost:
		s.createTopic(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) listTopics(w http.ResponseWriter, r *http.Request) {
	// Auth: any authenticated user can list topics (no specific action required)
	identity := s.requireAuth(w, r)
	if identity == nil {
		return
	}

	// Get topics from service
	result, err := s.app.Services.Config.ListTopics()
	if err != nil {
		s.handleServiceError(w, err)
		return
	}

	// Compute service info using server's method (needs access to file system)
	serviceInfo, err := s.getServiceInfo(result.AllStats)
	if err != nil {
		s.logger.Warn("Failed to get service info: %v", err)
	}

	WriteSuccess(w, map[string]interface{}{
		"topics":  result.Topics,
		"service": serviceInfo,
	})
}

func (s *Server) createTopic(w http.ResponseWriter, r *http.Request) {
	identity := s.requireAuth(w, r)
	if identity == nil {
		return
	}

	var req struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid JSON", constants.ErrCodeInvalidRequest)
		return
	}

	// Authorize: manage_topics with create sub-action
	if !s.authorize(w, identity, &auth.ActionContext{
		Action:    constants.AuthActionManageTopics,
		SubAction: "create",
		TopicName: req.Name,
	}) {
		return
	}

	// Check disk usage limit before creating topic
	if !s.checkDiskLimit(w, r, identity, "create_topic") {
		return
	}

	// Call service
	if err := s.app.Services.Config.CreateTopic(req.Name); err != nil {
		s.handleServiceError(w, err)
		return
	}

	// Audit log
	if s.app.AuditLogger != nil {
		s.app.AuditLogger.Log(constants.AuditActionAddingTopic, getClientIP(r), getAuditUsername(identity), audit.AddingTopicDetails{
			TopicName: req.Name,
		})
	}

	WriteSuccess(w, map[string]interface{}{
		"success": true,
		"name":    req.Name,
	})
}

// =============================================================================
// Topic Sub-Routes Handler
// =============================================================================

// /api/topics/:name/... routes
func (s *Server) handleTopicRoutes(w http.ResponseWriter, r *http.Request) {
	// Check if configured
	if s.app.Config.WorkingDirectory == "" {
		WriteError(w, http.StatusBadRequest, "Working directory not configured", constants.ErrCodeNotConfigured)
		return
	}

	// Parse path: /api/topics/:name/assets
	path := r.URL.Path
	prefix := "/api/topics/"

	if !strings.HasPrefix(path, prefix) {
		http.NotFound(w, r)
		return
	}

	remaining := path[len(prefix):]
	parts := strings.SplitN(remaining, "/", 2)

	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}

	topicName := parts[0]

	// Check topic exists and is healthy
	healthy, errMsg := s.app.IsTopicHealthy(topicName)
	if errMsg == "topic not found" {
		WriteError(w, http.StatusNotFound, "Topic not found", constants.ErrCodeTopicNotFound)
		return
	}
	if !healthy {
		WriteError(w, http.StatusServiceUnavailable, "Topic is unhealthy: "+errMsg, constants.ErrCodeTopicUnhealthy)
		return
	}

	// Route to sub-handler
	if len(parts) == 1 {
		// /api/topics/:name - topic detail (future)
		http.NotFound(w, r)
		return
	}

	subPath := parts[1]

	switch {
	case subPath == "assets" && r.Method == http.MethodPost:
		s.uploadAsset(w, r, topicName)
	default:
		http.NotFound(w, r)
	}
}

// =============================================================================
// Asset Upload Handler
// =============================================================================

// POST /api/topics/:name/assets - Upload single asset
func (s *Server) uploadAsset(w http.ResponseWriter, r *http.Request, topicName string) {
	identity := s.requireAuth(w, r)
	if identity == nil {
		return
	}

	// Parse multipart form with streaming
	// MaxMemory = 0 means all files go to disk (no memory buffering)
	if err := r.ParseMultipartForm(0); err != nil {
		WriteError(w, http.StatusBadRequest, "Failed to parse multipart form", constants.ErrCodeInvalidRequest)
		return
	}
	defer r.MultipartForm.RemoveAll()

	// Get the file
	file, header, err := r.FormFile(constants.FormFieldFile)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "No file provided", constants.ErrCodeInvalidRequest)
		return
	}
	defer file.Close()

	// Extract extension for auth constraint check
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(header.Filename)), ".")

	// Authorize: upload action with extension, size, and topic constraints
	if !s.authorize(w, identity, &auth.ActionContext{
		Action:    constants.AuthActionUpload,
		TopicName: topicName,
		Extension: ext,
		FileSize:  header.Size,
	}) {
		return
	}

	// Check file size against max_dat_size (early rejection)
	maxSize := s.app.Config.MaxDatSize
	if maxSize == 0 {
		maxSize = constants.DefaultMaxDatSize
	}
	if header.Size > maxSize-int64(constants.HeaderSize) {
		WriteError(w, http.StatusRequestEntityTooLarge, "File exceeds maximum size", constants.ErrCodeAssetTooLarge)
		return
	}

	// Check disk usage limit before writing
	if !s.checkDiskLimit(w, r, identity, "upload") {
		return
	}

	// Get optional parent_id
	var parentID *string
	if pid := r.FormValue(constants.FormFieldParentID); pid != "" {
		parentID = &pid
	}

	// Call service
	result, err := s.app.Services.Asset.Upload(r.Context(), topicName, file, header.Filename, parentID)
	if err != nil {
		s.handleServiceError(w, err)
		return
	}

	// Increment quota after successful upload
	if s.app.Services.Auth != nil {
		s.app.Services.Auth.GetEvaluator().IncrementQuota(identity.User.ID, constants.AuthActionUpload, result.Size)
	}

	// Audit log
	if !result.Skipped && s.app.AuditLogger != nil {
		s.app.AuditLogger.Log(constants.AuditActionAddingFile, getClientIP(r), getAuditUsername(identity), audit.AddingFileDetails{
			Hash:      result.Hash,
			TopicName: topicName,
			Filename:  header.Filename,
			Size:      result.Size,
			Skipped:   result.Skipped,
		})
	}

	// Format response
	response := map[string]interface{}{
		"success": true,
		"hash":    result.Hash,
		"skipped": result.Skipped,
	}
	if result.Skipped {
		response["existing_topic"] = result.ExistingTopic
	} else {
		response["size"] = result.Size
		response["blob"] = result.BlobName
	}
	WriteSuccess(w, response)
}

// =============================================================================
// Asset Routes Handler
// =============================================================================

// /api/assets/:hash/... routes
func (s *Server) handleAssetRoutes(w http.ResponseWriter, r *http.Request) {
	// Check if configured
	if s.app.Config.WorkingDirectory == "" {
		WriteError(w, http.StatusBadRequest, "Working directory not configured", constants.ErrCodeNotConfigured)
		return
	}

	// Parse path: /api/assets/:hash/download or /api/assets/:hash/metadata
	path := r.URL.Path
	prefix := "/api/assets/"

	if !strings.HasPrefix(path, prefix) {
		http.NotFound(w, r)
		return
	}

	remaining := path[len(prefix):]
	parts := strings.SplitN(remaining, "/", 2)

	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}

	hash := parts[0]

	// Validate hash format
	if len(hash) != constants.HashLength {
		WriteError(w, http.StatusBadRequest, "Invalid hash format", constants.ErrCodeInvalidHash)
		return
	}

	if len(parts) == 1 {
		http.NotFound(w, r)
		return
	}

	action := parts[1]

	switch {
	case action == "download" && r.Method == http.MethodGet:
		s.downloadAsset(w, r, hash)
	case action == "metadata" && r.Method == http.MethodGet:
		s.getMetadata(w, r, hash)
	case action == "metadata" && r.Method == http.MethodPost:
		s.postMetadata(w, r, hash)
	default:
		http.NotFound(w, r)
	}
}

// =============================================================================
// Asset Download Handler
// =============================================================================

// GET /api/assets/:hash/download - Download asset
func (s *Server) downloadAsset(w http.ResponseWriter, r *http.Request, hash string) {
	identity := s.requireAuth(w, r)
	if identity == nil {
		return
	}

	// Call service to get reader (need info for auth context)
	reader, err := s.app.Services.Asset.GetReader(hash)
	if err != nil {
		s.handleServiceError(w, err)
		return
	}
	defer reader.Close()

	info := reader.Info

	// Authorize: download with topic constraint
	if !s.authorize(w, identity, &auth.ActionContext{
		Action:      constants.AuthActionDownload,
		TopicName:   info.TopicName,
		VolumeBytes: info.Size,
	}) {
		return
	}

	// Set response headers
	w.Header().Set(constants.HeaderContentType, info.ContentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size))

	// Build filename for Content-Disposition (defense-in-depth: sanitize at output
	// even though input is sanitized at upload, in case of pre-existing data)
	filename := hash
	if info.OriginName != "" {
		filename = info.OriginName
	}
	if info.Extension != "" {
		filename = filename + "." + info.Extension
	}
	safeFilename := sanitize.ContentDispositionFilename(filename)
	if safeFilename == "" {
		safeFilename = hash
	}
	w.Header().Set(constants.HeaderContentDisposition, fmt.Sprintf(constants.ContentDispositionFormat, safeFilename))

	// Stream data
	io.Copy(w, reader)

	// Increment quota after successful download
	if s.app.Services.Auth != nil {
		s.app.Services.Auth.GetEvaluator().IncrementQuota(identity.User.ID, constants.AuthActionDownload, info.Size)
	}

	// Audit log for download
	if s.app.AuditLogger != nil {
		s.app.AuditLogger.Log(constants.AuditActionDownloaded, getClientIP(r), getAuditUsername(identity), audit.DownloadedDetails{
			Hash:     hash,
			Topic:    info.TopicName,
			Filename: filename,
			Size:     info.Size,
		})
	}
}

// =============================================================================
// Metadata Handler
// =============================================================================

// GET /api/assets/:hash/metadata - Get asset info and computed metadata
func (s *Server) getMetadata(w http.ResponseWriter, r *http.Request, hash string) {
	identity := s.requireAuth(w, r)
	if identity == nil {
		return
	}

	if !s.authorize(w, identity, &auth.ActionContext{Action: constants.AuthActionMetadata}) {
		return
	}

	result, err := s.app.Services.Metadata.Get(hash)
	if err != nil {
		s.handleServiceError(w, err)
		return
	}

	// Increment quota
	if s.app.Services.Auth != nil {
		s.app.Services.Auth.GetEvaluator().IncrementQuota(identity.User.ID, constants.AuthActionMetadata, 0)
	}

	WriteSuccess(w, map[string]interface{}{
		"asset":                   result.Asset,
		"computed_metadata":       result.ComputedMetadata,
		"metadata_with_processor": result.MetadataWithProcessor,
	})
}

// POST /api/assets/:hash/metadata - Add/delete metadata
func (s *Server) postMetadata(w http.ResponseWriter, r *http.Request, hash string) {
	identity := s.requireAuth(w, r)
	if identity == nil {
		return
	}

	if !s.authorize(w, identity, &auth.ActionContext{Action: constants.AuthActionMetadata}) {
		return
	}

	var req services.MetadataSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid JSON", constants.ErrCodeInvalidRequest)
		return
	}

	// Check disk usage limit before writing metadata (set operations grow SQLite)
	if req.Op == constants.BatchMetadataOpSet {
		if !s.checkDiskLimit(w, r, identity, "metadata_set") {
			return
		}
	}

	result, err := s.app.Services.Metadata.Set(hash, &req)
	if err != nil {
		s.handleServiceError(w, err)
		return
	}

	// Audit metadata set
	if s.app.AuditLogger != nil {
		s.app.AuditLogger.Log(constants.AuditActionMetadataSet, getClientIP(r), getAuditUsername(identity), audit.MetadataSetDetails{
			Hash: hash,
			Op:   req.Op,
			Key:  req.Key,
		})
	}

	// Increment quota
	if s.app.Services.Auth != nil {
		s.app.Services.Auth.GetEvaluator().IncrementQuota(identity.User.ID, constants.AuthActionMetadata, 0)
	}

	WriteSuccess(w, map[string]interface{}{
		"success":           true,
		"log_id":            result.LogID,
		"computed_metadata": result.ComputedMetadata,
	})
}

// =============================================================================
// Query Handlers
// =============================================================================

// GET /api/queries - List available query presets
func (s *Server) handleQueries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	identity := s.requireAuth(w, r)
	if identity == nil {
		return
	}

	if !s.authorize(w, identity, &auth.ActionContext{Action: constants.AuthActionQuery}) {
		return
	}

	presets, err := s.app.Services.Query.ListPresets()
	if err != nil {
		s.handleServiceError(w, err)
		return
	}

	WriteSuccess(w, map[string]interface{}{
		"presets": presets,
	})
}

// POST /api/query/:preset - Run a preset query
func (s *Server) handleQueryExecution(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	identity := s.requireAuth(w, r)
	if identity == nil {
		return
	}

	// Parse preset name from path: /api/query/:preset
	path := r.URL.Path
	prefix := "/api/query/"

	if !strings.HasPrefix(path, prefix) {
		http.NotFound(w, r)
		return
	}

	presetName := path[len(prefix):]
	if presetName == "" {
		WriteError(w, http.StatusBadRequest, "Preset name is required", constants.ErrCodeInvalidRequest)
		return
	}

	// Authorize: query action with preset constraint
	if !s.authorize(w, identity, &auth.ActionContext{
		Action:     constants.AuthActionQuery,
		PresetName: presetName,
	}) {
		return
	}

	// Parse request body
	var req services.QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Empty body is OK - use defaults
		req = services.QueryRequest{}
	}

	// Execute query via service
	result, topicNames, err := s.app.Services.Query.Execute(presetName, &req)
	if err != nil {
		s.handleServiceError(w, err)
		return
	}

	// Increment quota
	if s.app.Services.Auth != nil {
		s.app.Services.Auth.GetEvaluator().IncrementQuota(identity.User.ID, constants.AuthActionQuery, 0)
	}

	// Audit log for query
	if s.app.AuditLogger != nil {
		s.app.AuditLogger.Log(constants.AuditActionQuerying, getClientIP(r), getAuditUsername(identity), audit.QueryingDetails{
			Preset:   presetName,
			Topics:   topicNames,
			RowCount: result.RowCount,
		})
	}

	WriteSuccess(w, result)
}
