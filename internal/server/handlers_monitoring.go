package server

import (
	"net/http"
	"strings"

	"meshbank/internal/auth"
	"meshbank/internal/constants"
)

// =============================================================================
// Monitoring Handlers
// =============================================================================

// GET /api/monitoring - System monitoring info
func (s *Server) handleMonitoring(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	identity := s.requireAuth(w, r)
	if identity == nil {
		return
	}

	if !s.authorize(w, identity, &auth.ActionContext{Action: constants.AuthActionManageConfig}) {
		return
	}

	// Check if configured
	if s.app.Config.WorkingDirectory == "" {
		WriteError(w, http.StatusBadRequest, "Working directory not configured", constants.ErrCodeNotConfigured)
		return
	}

	s.logger.Debug("Monitoring: serving monitoring info request")

	info, err := s.app.Services.Monitoring.GetMonitoringInfo()
	if err != nil {
		s.handleServiceError(w, err)
		return
	}

	WriteSuccess(w, info)
}

// GET /api/monitoring/logs/:level/:filename - Read log file content
func (s *Server) handleMonitoringLogFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	identity := s.requireAuth(w, r)
	if identity == nil {
		return
	}

	if !s.authorize(w, identity, &auth.ActionContext{Action: constants.AuthActionManageConfig}) {
		return
	}

	// Check if configured
	if s.app.Config.WorkingDirectory == "" {
		WriteError(w, http.StatusBadRequest, "Working directory not configured", constants.ErrCodeNotConfigured)
		return
	}

	// Parse path: /api/monitoring/logs/:level/:filename
	path := r.URL.Path
	prefix := "/api/monitoring/logs/"

	if !strings.HasPrefix(path, prefix) {
		http.NotFound(w, r)
		return
	}

	remaining := path[len(prefix):]
	parts := strings.SplitN(remaining, "/", 2)

	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		WriteError(w, http.StatusBadRequest, "Level and filename required", constants.ErrCodeInvalidRequest)
		return
	}

	level := parts[0]
	filename := parts[1]

	s.logger.Info("Monitoring: log file request level=%s filename=%s", level, filename)

	content, err := s.app.Services.Monitoring.GetLogFileContent(level, filename)
	if err != nil {
		s.handleServiceError(w, err)
		return
	}

	// Return as plain text
	w.Header().Set(constants.HeaderContentType, constants.ContentTypeText)
	w.Write(content)
}
