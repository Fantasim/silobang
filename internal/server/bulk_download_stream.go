package server

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"net/http"

	"meshbank/internal/audit"
	"meshbank/internal/auth"
	"meshbank/internal/constants"
	"meshbank/internal/services"
)

// POST /api/download/bulk - Bulk download assets as ZIP
func (s *Server) handleBulkDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
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

	// Check if configured
	if s.app.Config.WorkingDirectory == "" {
		WriteError(w, http.StatusBadRequest, "Working directory not configured", constants.ErrCodeNotConfigured)
		return
	}

	// Parse request body
	var req BulkDownloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body: "+err.Error(), constants.ErrCodeInvalidRequest)
		return
	}

	// Convert to service request
	serviceReq := &services.BulkResolveRequest{
		Mode:           req.Mode,
		Preset:         req.Preset,
		Params:         req.Params,
		Topics:         req.Topics,
		AssetIDs:       req.AssetIDs,
		FilenameFormat: req.FilenameFormat,
	}

	// Validate request via service
	if err := s.app.Services.Bulk.ValidateRequest(serviceReq); err != nil {
		s.handleServiceError(w, err)
		return
	}

	// Resolve assets via service
	assets, err := s.app.Services.Bulk.ResolveAssets(serviceReq)
	if err != nil {
		s.handleServiceError(w, err)
		return
	}

	// Validate asset count via service
	if err := s.app.Services.Bulk.ValidateAssetCount(len(assets)); err != nil {
		s.handleServiceError(w, err)
		return
	}

	// Use validated filename format from service (may have been set to default)
	req.FilenameFormat = serviceReq.FilenameFormat

	// Stream ZIP response
	s.streamZIPArchive(w, assets, req, getClientIP(r), getAuditUsername(identity))
}

func (s *Server) streamZIPArchive(w http.ResponseWriter, assets []*services.ResolvedAsset, req BulkDownloadRequest, clientIP string, username string) {
	// Set response headers for streaming
	w.Header().Set(constants.HeaderContentType, constants.MimeTypeZIP)
	w.Header().Set(constants.HeaderContentDisposition, fmt.Sprintf(constants.ContentDispositionFormat, constants.BulkDownloadZipFilename))
	w.Header().Set(constants.HeaderTransferEncoding, constants.TransferEncodingChunked)

	// Create zip writer
	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	// Delegate to shared ZIP building logic
	result := s.buildZIPArchive(zipWriter, assets, req, nil)

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
