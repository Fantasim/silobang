package server

import (
	"context"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"silobang/internal/auth"
	"silobang/internal/constants"
	"silobang/internal/logger"
)

// Server wraps the HTTP server with graceful shutdown
type Server struct {
	httpServer      *http.Server
	app             *App
	logger          *logger.Logger
	webFS           fs.FS
	downloadManager *DownloadSessionManager

	// Pre-computed caches for immutable endpoints (schema, prompts list).
	// Populated lazily on first successful request, never invalidated.
	schemaCacheMu  sync.RWMutex
	schemaCache    *cachedResponse
	promptsCacheMu sync.RWMutex
	promptsCache   *cachedResponse
}

// NewServer creates a new HTTP server
func NewServer(app *App, addr string, webFS fs.FS) *Server {
	mux := http.NewServeMux()

	s := &Server{
		app:    app,
		logger: app.Logger,
		webFS:  webFS,
	}

	// Register routes
	s.registerRoutes(mux)

	// Build middleware chain: RequestID → SecurityHeaders → GzipCompress → Authenticate → handler
	// Auth middleware uses a dynamic store provider so it adapts when the auth
	// system is initialised after server start (e.g. POST /api/config).
	authMW := auth.NewMiddleware(func() *auth.Store {
		if app.Services.Auth != nil {
			return app.Services.Auth.GetStore()
		}
		return nil
	}, app.Logger)
	handler := Chain(mux, RequestID, SecurityHeaders, GzipCompress, authMW.Authenticate)

	// Start periodic reconciliation to detect manually-removed topic folders
	if app.Services.Reconcile != nil {
		app.Services.Reconcile.Start(time.Duration(constants.ReconcileIntervalMins) * time.Minute)
	}

	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  0, // No timeout for streaming uploads
		WriteTimeout: 0, // No timeout for streaming downloads
		IdleTimeout:  constants.HTTPIdleTimeout,
	}

	return s
}

// registerRoutes sets up all API routes
func (s *Server) registerRoutes(mux *http.ServeMux) {
	// API routes
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/topics", s.handleTopics)
	mux.HandleFunc("/api/topics/", s.handleTopicRoutes)
	mux.HandleFunc("/api/assets/", s.handleAssetRoutes)
	mux.HandleFunc("/api/queries", s.handleQueries)
	mux.HandleFunc("/api/query/", s.handleQueryExecution)
	mux.HandleFunc("/api/verify", s.handleVerify)
	mux.HandleFunc("/api/download/bulk", s.handleBulkDownload)
	mux.HandleFunc("/api/download/bulk/start", s.handleBulkDownloadSSE)
	mux.HandleFunc("/api/download/bulk/", s.handleBulkDownloadFetch)

	// Audit log routes
	mux.HandleFunc("/api/audit", s.handleAuditQuery)
	mux.HandleFunc("/api/audit/stream", s.handleAuditStream)
	mux.HandleFunc("/api/audit/actions", s.handleAuditActions)

	// Batch metadata routes
	mux.HandleFunc("/api/metadata/batch", s.handleBatchMetadata)
	mux.HandleFunc("/api/metadata/apply", s.handleApplyMetadata)

	// API schema and prompts routes
	mux.HandleFunc("/api/schema", s.handleSchema)
	mux.HandleFunc("/api/prompts", s.handlePrompts)
	mux.HandleFunc("/api/prompts/", s.handlePrompts)

	// Auth routes
	mux.HandleFunc("/api/auth/", s.handleAuthRoutes)

	// Monitoring routes
	mux.HandleFunc("/api/monitoring", s.handleMonitoring)
	mux.HandleFunc("/api/monitoring/logs/", s.handleMonitoringLogFile)

	// Static files (frontend) with pre-compressed asset support.
	// Serves brotli (.br) or gzip (.gz) variants when available and accepted by the client.
	if s.webFS != nil {
		mux.HandleFunc("/", s.serveStaticWithCompression)
	}
}

// Start runs the server and blocks until shutdown signal
func (s *Server) Start() error {
	// Channel for shutdown signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, shutdownSignals...)

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		s.logger.Info("Server listening on %s", s.httpServer.Addr)
		if err := s.httpServer.ListenAndServe(); err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait for shutdown signal or error
	select {
	case err := <-errChan:
		return err
	case sig := <-stop:
		s.logger.Info("Received signal %v, shutting down...", sig)
	}

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(constants.ShutdownTimeoutSecs)*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		s.logger.Error("Shutdown error: %v", err)
	}

	// Stop auth service cleanup goroutine
	if s.app.Services.Auth != nil {
		s.app.Services.Auth.Stop()
	}

	// Stop download manager cleanup goroutine
	if s.downloadManager != nil {
		s.downloadManager.Stop()
	}

	// Stop reconciliation goroutine
	if s.app.Services.Reconcile != nil {
		s.app.Services.Reconcile.Stop()
	}

	// Stop audit logger cleanup goroutine
	if s.app.AuditLogger != nil {
		s.app.AuditLogger.Stop()
	}

	// Close all database connections
	s.app.CloseAllTopicDBs()
	if s.app.OrchestratorDB != nil {
		s.app.OrchestratorDB.Close()
	}

	s.logger.Info("Server stopped")
	return nil
}

// Handler returns the HTTP handler for testing purposes
func (s *Server) Handler() http.Handler {
	return s.httpServer.Handler
}
