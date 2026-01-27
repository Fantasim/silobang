package server

import (
	"archive/zip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"silobang/internal/audit"
	"silobang/internal/auth"
	"silobang/internal/constants"
	"silobang/internal/services"
)

// BulkDownloadEvent represents an SSE event for bulk download progress
type BulkDownloadEvent struct {
	Type      string      `json:"type"`
	Timestamp int64       `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// Event payload structures for bulk download

type DownloadStartData struct {
	DownloadID  string `json:"download_id"`
	TotalAssets int    `json:"total_assets"`
	TotalBytes  int64  `json:"total_bytes"`
	Mode        string `json:"mode"`
}

type AssetProgressData struct {
	DownloadID  string `json:"download_id"`
	AssetIndex  int    `json:"asset_index"`
	TotalAssets int    `json:"total_assets"`
	Hash        string `json:"hash"`
	Topic       string `json:"topic"`
	Size        int64  `json:"size"`
	Filename    string `json:"filename"`
}

type ZipProgressData struct {
	DownloadID      string `json:"download_id"`
	BytesWritten    int64  `json:"bytes_written"`
	TotalBytes      int64  `json:"total_bytes"`
	PercentComplete int    `json:"percent_complete"`
}

type DownloadCompleteData struct {
	DownloadID   string `json:"download_id"`
	DownloadURL  string `json:"download_url"`
	TotalAssets  int    `json:"total_assets"`
	TotalSize    int64  `json:"total_size"`
	FailedAssets int    `json:"failed_assets"`
	DurationMs   int    `json:"duration_ms"`
	ExpiresAt    int64  `json:"expires_at"`
}

type DownloadErrorData struct {
	DownloadID string `json:"download_id"`
	Message    string `json:"message"`
	Code       string `json:"code"`
}

// BulkDownloadSession tracks an in-progress or completed download
type BulkDownloadSession struct {
	ID          string
	Status      string // "pending", "processing", "complete", "error"
	CreatedAt   time.Time
	CompletedAt *time.Time
	ZIPPath     string
	ZIPSize     int64
	Error       string

	// Progress tracking
	TotalAssets     int
	ProcessedAssets int
	TotalBytes      int64
	ProcessedBytes  int64
	FailedAssets    int
}

// DownloadSessionManager manages active download sessions with cleanup
type DownloadSessionManager struct {
	sessions   map[string]*BulkDownloadSession
	mu         sync.RWMutex
	tempDir    string
	stopClean  chan struct{}
	sessionTTL time.Duration
}

// NewDownloadSessionManager creates a new session manager with cleanup goroutine
func NewDownloadSessionManager(workingDir string, sessionTTLMins int) *DownloadSessionManager {
	// Clean up leftover files from previous runs
	CleanBulkDownloadDirectory(workingDir)

	tempDir := filepath.Join(workingDir, constants.InternalDir, constants.BulkDownloadTempDir)

	// Ensure temp directory exists
	os.MkdirAll(tempDir, constants.DirPermissions)

	m := &DownloadSessionManager{
		sessions:   make(map[string]*BulkDownloadSession),
		tempDir:    tempDir,
		stopClean:  make(chan struct{}),
		sessionTTL: time.Duration(sessionTTLMins) * time.Minute,
	}

	// Start cleanup goroutine
	go m.cleanupLoop()

	return m
}

// Stop stops the cleanup goroutine
func (m *DownloadSessionManager) Stop() {
	close(m.stopClean)
}

// cleanupLoop periodically removes expired sessions
func (m *DownloadSessionManager) cleanupLoop() {
	ticker := time.NewTicker(time.Duration(constants.BulkDownloadCleanupMins) * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopClean:
			return
		case <-ticker.C:
			m.cleanupExpiredSessions()
		}
	}
}

// cleanupExpiredSessions removes sessions older than TTL
func (m *DownloadSessionManager) cleanupExpiredSessions() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	for id, session := range m.sessions {
		if now.Sub(session.CreatedAt) > m.sessionTTL {
			// Remove ZIP file if exists
			if session.ZIPPath != "" {
				os.Remove(session.ZIPPath)
			}
			delete(m.sessions, id)
		}
	}
}

// CreateSession creates a new download session with unique ID
func (m *DownloadSessionManager) CreateSession() (*BulkDownloadSession, error) {
	id, err := generateDownloadID()
	if err != nil {
		return nil, err
	}

	session := &BulkDownloadSession{
		ID:        id,
		Status:    "pending",
		CreatedAt: time.Now(),
	}

	m.mu.Lock()
	m.sessions[id] = session
	m.mu.Unlock()

	return session, nil
}

// GetSession retrieves a session by ID
func (m *DownloadSessionManager) GetSession(id string) *BulkDownloadSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[id]
}

// UpdateSession updates session fields
func (m *DownloadSessionManager) UpdateSession(id string, updater func(*BulkDownloadSession)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if session, exists := m.sessions[id]; exists {
		updater(session)
	}
}

// RemoveSession removes a session and its ZIP file
func (m *DownloadSessionManager) RemoveSession(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, exists := m.sessions[id]; exists {
		if session.ZIPPath != "" {
			os.Remove(session.ZIPPath)
		}
		delete(m.sessions, id)
	}
}

// GetTempDir returns the temporary directory path
func (m *DownloadSessionManager) GetTempDir() string {
	return m.tempDir
}

// generateDownloadID creates a random download ID
func generateDownloadID() (string, error) {
	bytes := make([]byte, constants.BulkDownloadIDLength/2)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// CleanBulkDownloadDirectory removes all ZIP files from the downloads directory.
// Called during initialization to clean up leftover files from previous runs.
func CleanBulkDownloadDirectory(workingDir string) error {
	if workingDir == "" {
		return nil
	}

	downloadDir := filepath.Join(workingDir, constants.InternalDir, constants.BulkDownloadTempDir)

	if _, err := os.Stat(downloadDir); os.IsNotExist(err) {
		return nil
	}

	pattern := filepath.Join(downloadDir, constants.BulkDownloadFilePattern)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	for _, match := range matches {
		os.Remove(match)
	}

	return nil
}

// BulkDownloadSSEWriter handles SSE for bulk downloads
type BulkDownloadSSEWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

// NewBulkDownloadSSEWriter creates a new SSE writer for bulk downloads
func NewBulkDownloadSSEWriter(w http.ResponseWriter) (*BulkDownloadSSEWriter, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported")
	}

	// Set SSE headers
	w.Header().Set(constants.HeaderContentType, constants.ContentTypeSSE)
	w.Header().Set(constants.HeaderCacheControl, constants.SSECacheControl)
	w.Header().Set(constants.HeaderConnection, constants.SSEConnection)
	w.Header().Set(constants.HeaderXAccelBuffering, constants.SSEXAccelBuffering) // Disable nginx buffering

	return &BulkDownloadSSEWriter{w: w, flusher: flusher}, nil
}

// Send sends an SSE event with the given type and data
func (s *BulkDownloadSSEWriter) Send(eventType string, data interface{}) error {
	event := BulkDownloadEvent{
		Type:      eventType,
		Timestamp: time.Now().Unix(),
		Data:      data,
	}

	jsonData, err := json.Marshal(event)
	if err != nil {
		return err
	}

	// SSE format: "data: {json}\n\n"
	_, err = fmt.Fprintf(s.w, "data: %s\n\n", jsonData)
	if err != nil {
		return err
	}

	s.flusher.Flush()
	return nil
}

// handleBulkDownloadSSE handles GET /api/download/bulk/start with SSE streaming
func (s *Server) handleBulkDownloadSSE(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	identity := s.requireAuth(w, r)
	if identity == nil {
		return
	}

	if !s.authorize(w, identity, &auth.ActionContext{Action: constants.AuthActionBulkDownload}) {
		return
	}

	// Set up SSE writer FIRST so all errors go through SSE format
	// This ensures EventSource receives proper SSE events, not JSON
	sse, err := NewBulkDownloadSSEWriter(w)
	if err != nil {
		// Only case where we can't use SSE - streaming not supported
		WriteError(w, http.StatusInternalServerError, "Streaming not supported", constants.ErrCodeStreamingError)
		return
	}

	// Helper to send error and return
	sendError := func(message, code string) {
		sse.Send("error", DownloadErrorData{
			Message: message,
			Code:    code,
		})
	}

	// Check if configured
	if s.app.Config.WorkingDirectory == "" {
		sendError("Working directory not configured", constants.ErrCodeNotConfigured)
		return
	}

	// Ensure download manager is initialized
	if s.downloadManager == nil {
		s.downloadManager = NewDownloadSessionManager(s.app.Config.WorkingDirectory, s.app.Config.BulkDownload.SessionTTLMins)
	}

	// Parse request from query params
	req, err := s.parseBulkDownloadSSEParams(r)
	if err != nil {
		sendError(err.Error(), constants.ErrCodeInvalidRequest)
		return
	}

	// Convert to service request and validate
	serviceReq := &services.BulkResolveRequest{
		Mode:           req.Mode,
		Preset:         req.Preset,
		Params:         req.Params,
		Topics:         req.Topics,
		AssetIDs:       req.AssetIDs,
		FilenameFormat: req.FilenameFormat,
	}

	// Validate via service
	if err := s.app.Services.Bulk.ValidateRequest(serviceReq); err != nil {
		if svcErr, ok := err.(*services.ServiceError); ok {
			sendError(svcErr.Message, svcErr.Code)
		} else {
			sendError(err.Error(), constants.ErrCodeInvalidRequest)
		}
		return
	}

	// Resolve assets via service
	assets, err := s.app.Services.Bulk.ResolveAssets(serviceReq)
	if err != nil {
		if svcErr, ok := err.(*services.ServiceError); ok {
			sendError(svcErr.Message, svcErr.Code)
		} else {
			sendError(err.Error(), constants.ErrCodeInvalidRequest)
		}
		return
	}

	// Validate asset count via service
	if err := s.app.Services.Bulk.ValidateAssetCount(len(assets)); err != nil {
		if svcErr, ok := err.(*services.ServiceError); ok {
			sendError(svcErr.Message, svcErr.Code)
		} else {
			sendError(err.Error(), constants.ErrCodeInvalidRequest)
		}
		return
	}

	// Calculate total size
	var totalBytes int64
	for _, asset := range assets {
		totalBytes += asset.Asset.AssetSize
	}

	// Create session
	session, err := s.downloadManager.CreateSession()
	if err != nil {
		sendError("Failed to create download session", constants.ErrCodeInternalError)
		return
	}

	// Update session with initial info
	s.downloadManager.UpdateSession(session.ID, func(sess *BulkDownloadSession) {
		sess.Status = "processing"
		sess.TotalAssets = len(assets)
		sess.TotalBytes = totalBytes
	})

	// Run ZIP generation with progress events
	s.generateZIPWithProgress(r.Context(), sse, session, assets, req, getClientIP(r), getAuditUsername(identity))
}

// parseBulkDownloadSSEParams parses query parameters for SSE bulk download
func (s *Server) parseBulkDownloadSSEParams(r *http.Request) (BulkDownloadRequest, error) {
	q := r.URL.Query()

	req := BulkDownloadRequest{
		Mode:            q.Get("mode"),
		Preset:          q.Get("preset"),
		FilenameFormat:  q.Get("filename_format"),
		IncludeMetadata: q.Get("include_metadata") == "true",
	}

	// Parse topics
	if topics := q.Get("topics"); topics != "" {
		req.Topics = strings.Split(topics, ",")
		for i := range req.Topics {
			req.Topics[i] = strings.TrimSpace(req.Topics[i])
		}
	}

	// Parse asset_ids
	if assetIDs := q.Get("asset_ids"); assetIDs != "" {
		req.AssetIDs = strings.Split(assetIDs, ",")
		for i := range req.AssetIDs {
			req.AssetIDs[i] = strings.TrimSpace(req.AssetIDs[i])
		}
	}

	// Parse params (JSON-encoded)
	if params := q.Get("params"); params != "" {
		if err := json.Unmarshal([]byte(params), &req.Params); err != nil {
			return req, fmt.Errorf("invalid params JSON: %w", err)
		}
	}

	// Validate mode
	if req.Mode == "" {
		return req, fmt.Errorf("mode is required")
	}

	return req, nil
}

// generateZIPWithProgress creates ZIP file with progress events
func (s *Server) generateZIPWithProgress(
	ctx context.Context,
	sse *BulkDownloadSSEWriter,
	session *BulkDownloadSession,
	assets []*services.ResolvedAsset,
	req BulkDownloadRequest,
	clientIP string,
	username string,
) {
	startTime := time.Now()

	s.logger.Info("Bulk download started: id=%s, assets=%d, bytes=%d, mode=%s", session.ID, len(assets), session.TotalBytes, req.Mode)

	// Send download_start event
	sse.Send("download_start", DownloadStartData{
		DownloadID:  session.ID,
		TotalAssets: len(assets),
		TotalBytes:  session.TotalBytes,
		Mode:        req.Mode,
	})

	// Create temp ZIP file
	zipPath := filepath.Join(s.downloadManager.GetTempDir(), session.ID+".zip")
	zipFile, err := os.Create(zipPath)
	if err != nil {
		s.sendDownloadError(sse, session.ID, "Failed to create ZIP file", constants.ErrCodeInternalError)
		s.downloadManager.UpdateSession(session.ID, func(sess *BulkDownloadSession) {
			sess.Status = "error"
			sess.Error = err.Error()
		})
		return
	}

	zipWriter := zip.NewWriter(zipFile)

	// Build ZIP with progress callbacks for SSE events
	result := s.buildZIPArchive(zipWriter, assets, req, &ZIPBuildCallbacks{
		OnAssetProcessed: func(index int, asset *services.ResolvedAsset, filename string, processedBytes int64) {
			// Send asset progress event every N assets
			if index%constants.BulkDownloadProgressInterval == 0 || index == len(assets)-1 {
				sse.Send("asset_progress", AssetProgressData{
					DownloadID:  session.ID,
					AssetIndex:  index + 1,
					TotalAssets: len(assets),
					Hash:        asset.Hash,
					Topic:       asset.Topic,
					Size:        asset.Asset.AssetSize,
					Filename:    filename,
				})
			}

			// Update session progress
			s.downloadManager.UpdateSession(session.ID, func(sess *BulkDownloadSession) {
				sess.ProcessedAssets = index + 1
				sess.ProcessedBytes = processedBytes
			})

			// Send ZIP progress periodically
			if index%constants.BulkDownloadProgressInterval == 0 && session.TotalBytes > 0 {
				percent := int((processedBytes * 100) / session.TotalBytes)
				sse.Send("zip_progress", ZipProgressData{
					DownloadID:      session.ID,
					BytesWritten:    processedBytes,
					TotalBytes:      session.TotalBytes,
					PercentComplete: percent,
				})
			}
		},
		CheckCancelled: func() bool {
			select {
			case <-ctx.Done():
				return true
			default:
				return false
			}
		},
	})

	// Handle cancellation
	if result.Cancelled {
		zipWriter.Close()
		zipFile.Close()
		os.Remove(zipPath)
		s.sendDownloadError(sse, session.ID, "Download cancelled", constants.ErrCodeStreamingError)
		s.downloadManager.UpdateSession(session.ID, func(sess *BulkDownloadSession) {
			sess.Status = "error"
			sess.Error = "cancelled"
		})
		return
	}

	// Close ZIP
	if err := zipWriter.Close(); err != nil {
		s.logger.Error("Failed to close ZIP writer: %v", err)
	}

	// Get final file size
	fileInfo, _ := zipFile.Stat()
	var zipSize int64
	if fileInfo != nil {
		zipSize = fileInfo.Size()
	}
	zipFile.Close()

	duration := time.Since(startTime)
	expiresAt := time.Now().Add(s.downloadManager.sessionTTL)

	// Update session as complete
	completedAt := time.Now()
	s.downloadManager.UpdateSession(session.ID, func(sess *BulkDownloadSession) {
		sess.Status = "complete"
		sess.CompletedAt = &completedAt
		sess.ZIPPath = zipPath
		sess.ZIPSize = zipSize
		sess.FailedAssets = result.FailedCount
	})

	s.logger.Info("Bulk download complete: id=%s, assets=%d, size=%d, failed=%d, duration=%dms", session.ID, result.Manifest.AssetCount, result.TotalSize, result.FailedCount, int(duration.Milliseconds()))

	// Send complete event
	sse.Send("complete", DownloadCompleteData{
		DownloadID:   session.ID,
		DownloadURL:  "/api/download/bulk/" + session.ID,
		TotalAssets:  result.Manifest.AssetCount,
		TotalSize:    result.TotalSize,
		FailedAssets: result.FailedCount,
		DurationMs:   int(duration.Milliseconds()),
		ExpiresAt:    expiresAt.Unix(),
	})

	// Audit log for bulk download
	if s.app.AuditLogger != nil {
		s.app.AuditLogger.Log(constants.AuditActionDownloadedBulk, clientIP, username, audit.DownloadedBulkDetails{
			Mode:       req.Mode,
			AssetCount: result.Manifest.AssetCount,
			TotalSize:  result.TotalSize,
			Topics:     result.Topics,
			Preset:     req.Preset,
		})
	}
}

// sendDownloadError sends an error event
func (s *Server) sendDownloadError(sse *BulkDownloadSSEWriter, downloadID, message, code string) {
	sse.Send("error", DownloadErrorData{
		DownloadID: downloadID,
		Message:    message,
		Code:       code,
	})
}

// handleBulkDownloadFetch handles GET /api/download/bulk/{id}
func (s *Server) handleBulkDownloadFetch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	identity := s.requireAuth(w, r)
	if identity == nil {
		return
	}

	if !s.authorize(w, identity, &auth.ActionContext{Action: constants.AuthActionBulkDownload}) {
		return
	}

	// Extract download ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/download/bulk/")
	downloadID := strings.TrimSuffix(path, "/")

	if downloadID == "" {
		WriteError(w, http.StatusBadRequest, "Download ID is required", constants.ErrCodeInvalidRequest)
		return
	}

	// Check if download manager exists
	if s.downloadManager == nil {
		WriteError(w, http.StatusNotFound, "Download session not found", constants.ErrCodeDownloadSessionNotFound)
		return
	}

	// Get session
	session := s.downloadManager.GetSession(downloadID)
	if session == nil {
		WriteError(w, http.StatusNotFound, "Download session not found", constants.ErrCodeDownloadSessionNotFound)
		return
	}

	// Check session status
	switch session.Status {
	case "pending", "processing":
		WriteError(w, http.StatusBadRequest, "Download is still in progress", constants.ErrCodeDownloadInProgress)
		return
	case "error":
		WriteError(w, http.StatusInternalServerError, "Download failed: "+session.Error, constants.ErrCodeDownloadSessionNotFound)
		return
	}

	// Check if expired
	if time.Since(session.CreatedAt) > s.downloadManager.sessionTTL {
		s.downloadManager.RemoveSession(downloadID)
		WriteError(w, http.StatusGone, "Download session has expired", constants.ErrCodeDownloadSessionExpired)
		return
	}

	// Check if ZIP file exists
	if _, err := os.Stat(session.ZIPPath); os.IsNotExist(err) {
		s.downloadManager.RemoveSession(downloadID)
		WriteError(w, http.StatusGone, "Download file no longer available", constants.ErrCodeDownloadSessionExpired)
		return
	}

	// Open ZIP file
	zipFile, err := os.Open(session.ZIPPath)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "Failed to open download file", constants.ErrCodeInternalError)
		return
	}
	defer zipFile.Close()

	// Set response headers
	w.Header().Set(constants.HeaderContentType, constants.MimeTypeZIP)
	w.Header().Set(constants.HeaderContentDisposition, fmt.Sprintf(constants.ContentDispositionFormat, constants.BulkDownloadZipFilename))
	if session.ZIPSize > 0 {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", session.ZIPSize))
	}

	// Track whether download completed successfully
	var downloadSuccessful bool
	defer func() {
		if downloadSuccessful {
			s.downloadManager.RemoveSession(downloadID)
		}
	}()

	// Stream file to response
	bytesWritten, err := io.Copy(w, zipFile)
	if err == nil && bytesWritten == session.ZIPSize {
		downloadSuccessful = true
	}
}
