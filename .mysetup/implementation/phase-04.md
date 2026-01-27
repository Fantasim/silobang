# Phase 4: Core API

## Objective
Implement the HTTP server with core API endpoints: configuration, topics, single asset upload, asset download, and metadata operations.

**Key constraints:**
- Low memory environment (< 2GB RAM, 4 cores max)
- Must stream file uploads (support files up to max_dat_size, even 100GB)
- No CORS needed (embedded frontend, same origin)
- Simple graceful shutdown on SIGINT/SIGTERM

---

## Prerequisites
- Phase 1 completed (project structure, constants, config, logger)
- Phase 2 completed (database layer, schema, connections, pipeline)
- Phase 3 completed (blob format, .dat file operations)

---

## Task 1: Error Codes (`internal/constants/errors.go`)

```go
package constants

// API Error Codes
const (
    ErrCodeTopicNotFound      = "TOPIC_NOT_FOUND"
    ErrCodeTopicAlreadyExists = "TOPIC_ALREADY_EXISTS"
    ErrCodeTopicUnhealthy     = "TOPIC_UNHEALTHY"
    ErrCodeInvalidTopicName   = "INVALID_TOPIC_NAME"
    ErrCodeAssetNotFound      = "ASSET_NOT_FOUND"
    ErrCodeAssetTooLarge      = "ASSET_TOO_LARGE"
    ErrCodeAssetDuplicate     = "ASSET_DUPLICATE"
    ErrCodeParentNotFound     = "PARENT_NOT_FOUND"
    ErrCodeInvalidRequest     = "INVALID_REQUEST"
    ErrCodeInternalError      = "INTERNAL_ERROR"
    ErrCodeNotConfigured      = "NOT_CONFIGURED"
    ErrCodeInvalidHash        = "INVALID_HASH"
    ErrCodeMetadataError      = "METADATA_ERROR"
)
```

---

## Task 2: Application State (`internal/server/app.go`)

Central application struct holding all shared state.

```go
package server

import (
    "database/sql"
    "sync"

    "meshbank/internal/config"
    "meshbank/internal/logger"
)

// App holds all application state and dependencies
type App struct {
    Config        *config.Config
    Logger        *logger.Logger
    OrchestratorDB *sql.DB

    // Topic databases - lazily opened, keyed by topic name
    topicDBs      map[string]*sql.DB
    topicDBsMu    sync.RWMutex

    // Topic health status - keyed by topic name
    topicHealth   map[string]*TopicHealth
    topicHealthMu sync.RWMutex
}

// TopicHealth tracks the health status of a topic
type TopicHealth struct {
    Healthy bool
    Error   string // empty if healthy
}

// NewApp creates a new application instance
func NewApp(cfg *config.Config, log *logger.Logger) *App {
    return &App{
        Config:      cfg,
        Logger:      log,
        topicDBs:    make(map[string]*sql.DB),
        topicHealth: make(map[string]*TopicHealth),
    }
}

// GetTopicDB returns the database connection for a topic, opening it lazily if needed
// Returns nil and error if topic doesn't exist or is unhealthy
func (a *App) GetTopicDB(topicName string) (*sql.DB, error) {
    // Check health first
    a.topicHealthMu.RLock()
    health, exists := a.topicHealth[topicName]
    a.topicHealthMu.RUnlock()

    if !exists {
        return nil, fmt.Errorf("topic not found: %s", topicName)
    }
    if !health.Healthy {
        return nil, fmt.Errorf("topic unhealthy: %s - %s", topicName, health.Error)
    }

    // Check if already open
    a.topicDBsMu.RLock()
    db, exists := a.topicDBs[topicName]
    a.topicDBsMu.RUnlock()

    if exists {
        return db, nil
    }

    // Open lazily
    a.topicDBsMu.Lock()
    defer a.topicDBsMu.Unlock()

    // Double-check after acquiring write lock
    if db, exists := a.topicDBs[topicName]; exists {
        return db, nil
    }

    // Open the database
    dbPath := filepath.Join(a.Config.WorkingDirectory, topicName, constants.InternalDir, topicName+".db")
    db, err := database.OpenDatabase(dbPath)
    if err != nil {
        return nil, fmt.Errorf("failed to open topic database: %w", err)
    }

    a.topicDBs[topicName] = db
    return db, nil
}

// GetTopicPath returns the filesystem path for a topic
func (a *App) GetTopicPath(topicName string) string {
    return filepath.Join(a.Config.WorkingDirectory, topicName)
}

// RegisterTopic adds a topic to the health registry
func (a *App) RegisterTopic(name string, healthy bool, errMsg string) {
    a.topicHealthMu.Lock()
    defer a.topicHealthMu.Unlock()
    a.topicHealth[name] = &TopicHealth{Healthy: healthy, Error: errMsg}
}

// CloseAllTopicDBs closes all open topic database connections
func (a *App) CloseAllTopicDBs() {
    a.topicDBsMu.Lock()
    defer a.topicDBsMu.Unlock()

    for name, db := range a.topicDBs {
        if err := db.Close(); err != nil {
            a.Logger.Error("Failed to close topic DB %s: %v", name, err)
        }
    }
    a.topicDBs = make(map[string]*sql.DB)
}

// ClearTopicRegistry clears all topic health entries
func (a *App) ClearTopicRegistry() {
    a.topicHealthMu.Lock()
    defer a.topicHealthMu.Unlock()
    a.topicHealth = make(map[string]*TopicHealth)
}

// IsTopicHealthy checks if a topic is healthy
func (a *App) IsTopicHealthy(topicName string) (bool, string) {
    a.topicHealthMu.RLock()
    defer a.topicHealthMu.RUnlock()

    health, exists := a.topicHealth[topicName]
    if !exists {
        return false, "topic not found"
    }
    return health.Healthy, health.Error
}

// ListTopics returns all registered topic names
func (a *App) ListTopics() []string {
    a.topicHealthMu.RLock()
    defer a.topicHealthMu.RUnlock()

    names := make([]string, 0, len(a.topicHealth))
    for name := range a.topicHealth {
        names = append(names, name)
    }
    sort.Strings(names)
    return names
}
```

**Required imports for app.go:**
```go
import (
    "database/sql"
    "fmt"
    "path/filepath"
    "sort"
    "sync"

    "meshbank/internal/config"
    "meshbank/internal/constants"
    "meshbank/internal/database"
    "meshbank/internal/logger"
)
```

---

## Task 3: HTTP Response Helpers (`internal/server/response.go`)

```go
package server

import (
    "encoding/json"
    "net/http"
)

// APIError represents a standard error response
type APIError struct {
    Error   bool   `json:"error"`
    Message string `json:"message"`
    Code    string `json:"code"`
}

// WriteJSON writes a JSON response with the given status code
func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(data)
}

// WriteError writes a standard error response
func WriteError(w http.ResponseWriter, status int, message string, code string) {
    WriteJSON(w, status, APIError{
        Error:   true,
        Message: message,
        Code:    code,
    })
}

// WriteSuccess writes a simple success response
func WriteSuccess(w http.ResponseWriter, data interface{}) {
    WriteJSON(w, http.StatusOK, data)
}
```

---

## Task 4: HTTP Server Setup (`internal/server/server.go`)

Using standard `net/http` with manual routing (minimal code, no dependencies).

```go
package server

import (
    "context"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "meshbank/internal/logger"
)

// Server wraps the HTTP server with graceful shutdown
type Server struct {
    httpServer *http.Server
    app        *App
    logger     *logger.Logger
}

// NewServer creates a new HTTP server
func NewServer(app *App, addr string) *Server {
    mux := http.NewServeMux()

    s := &Server{
        httpServer: &http.Server{
            Addr:         addr,
            Handler:      mux,
            ReadTimeout:  0, // No timeout for streaming uploads
            WriteTimeout: 0, // No timeout for streaming downloads
            IdleTimeout:  120 * time.Second,
        },
        app:    app,
        logger: app.Logger,
    }

    // Register routes
    s.registerRoutes(mux)

    return s
}

// registerRoutes sets up all API routes
func (s *Server) registerRoutes(mux *http.ServeMux) {
    // API routes
    mux.HandleFunc("/api/config", s.handleConfig)
    mux.HandleFunc("/api/topics", s.handleTopics)
    mux.HandleFunc("/api/topics/", s.handleTopicRoutes)
    mux.HandleFunc("/api/assets/", s.handleAssetRoutes)
    mux.HandleFunc("/api/logs", s.handleLogs)

    // Static files (frontend) - to be implemented in Phase 7
    // mux.Handle("/", http.FileServer(http.FS(webFS)))
}

// Start runs the server and blocks until shutdown signal
func (s *Server) Start() error {
    // Channel for shutdown signals
    stop := make(chan os.Signal, 1)
    signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

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
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    if err := s.httpServer.Shutdown(ctx); err != nil {
        s.logger.Error("Shutdown error: %v", err)
    }

    // Close all database connections
    s.app.CloseAllTopicDBs()
    if s.app.OrchestratorDB != nil {
        s.app.OrchestratorDB.Close()
    }

    s.logger.Info("Server stopped")
    return nil
}
```

---

## Task 5: Config Handler (`internal/server/handlers.go`)

```go
package server

import (
    "encoding/json"
    "net/http"
    "regexp"

    "meshbank/internal/config"
    "meshbank/internal/constants"
    "meshbank/internal/database"
)

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
    cfg := s.app.Config

    response := map[string]interface{}{
        "configured":        cfg.WorkingDirectory != "",
        "working_directory": cfg.WorkingDirectory,
        "port":              cfg.Port,
        "max_dat_size":      cfg.MaxDatSize,
    }

    WriteSuccess(w, response)
}

func (s *Server) postConfig(w http.ResponseWriter, r *http.Request) {
    var req struct {
        WorkingDirectory string `json:"working_directory"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        WriteError(w, http.StatusBadRequest, "Invalid JSON", constants.ErrCodeInvalidRequest)
        return
    }

    if req.WorkingDirectory == "" {
        WriteError(w, http.StatusBadRequest, "working_directory is required", constants.ErrCodeInvalidRequest)
        return
    }

    // Validate directory exists
    if err := config.ValidateWorkingDirectory(req.WorkingDirectory); err != nil {
        WriteError(w, http.StatusBadRequest, err.Error(), constants.ErrCodeInvalidRequest)
        return
    }

    // Close existing connections (project restart behavior)
    s.app.CloseAllTopicDBs()
    s.app.ClearTopicRegistry()
    if s.app.OrchestratorDB != nil {
        s.app.OrchestratorDB.Close()
        s.app.OrchestratorDB = nil
    }

    // Initialize new working directory
    if err := config.InitializeWorkingDirectory(req.WorkingDirectory); err != nil {
        WriteError(w, http.StatusInternalServerError, err.Error(), constants.ErrCodeInternalError)
        return
    }

    // Update and save config
    s.app.Config.WorkingDirectory = req.WorkingDirectory
    if err := config.SaveConfig(s.app.Config); err != nil {
        WriteError(w, http.StatusInternalServerError, "Failed to save config", constants.ErrCodeInternalError)
        return
    }

    // Open orchestrator DB
    orchPath := filepath.Join(req.WorkingDirectory, constants.InternalDir, constants.OrchestratorDB)
    orchDB, err := database.InitOrchestratorDB(orchPath)
    if err != nil {
        WriteError(w, http.StatusInternalServerError, "Failed to open orchestrator database", constants.ErrCodeInternalError)
        return
    }
    s.app.OrchestratorDB = orchDB

    // Discover and register topics
    topics, err := config.DiscoverTopics(req.WorkingDirectory)
    if err != nil {
        s.logger.Warn("Topic discovery error: %v", err)
    }

    for _, topic := range topics {
        s.app.RegisterTopic(topic.Name, topic.Healthy, topic.Error)
        if topic.Healthy {
            // Index to orchestrator
            if err := database.IndexTopicToOrchestrator(topic.Path, topic.Name, s.app.OrchestratorDB); err != nil {
                s.logger.Warn("Failed to index topic %s: %v", topic.Name, err)
            }
        }
    }

    s.logger.Info("Working directory changed to: %s, discovered %d topics", req.WorkingDirectory, len(topics))

    WriteSuccess(w, map[string]bool{"success": true})
}
```

**Add to imports:**
```go
import (
    "path/filepath"
)
```

---

## Task 6: Topics Handler (`internal/server/handlers.go`)

```go
var topicNameRegex = regexp.MustCompile(constants.TopicNameRegex)

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
    type TopicResponse struct {
        Name    string                 `json:"name"`
        Stats   map[string]interface{} `json:"stats,omitempty"`
        Healthy bool                   `json:"healthy"`
        Error   string                 `json:"error,omitempty"`
    }

    topicNames := s.app.ListTopics()
    topics := make([]TopicResponse, 0, len(topicNames))

    for _, name := range topicNames {
        healthy, errMsg := s.app.IsTopicHealthy(name)

        tr := TopicResponse{
            Name:    name,
            Healthy: healthy,
        }

        if !healthy {
            tr.Error = errMsg
        } else {
            // Get stats for healthy topics
            stats, err := s.getTopicStats(name)
            if err != nil {
                s.logger.Warn("Failed to get stats for topic %s: %v", name, err)
            } else {
                tr.Stats = stats
            }
        }

        topics = append(topics, tr)
    }

    WriteSuccess(w, map[string]interface{}{"topics": topics})
}

func (s *Server) getTopicStats(topicName string) (map[string]interface{}, error) {
    db, err := s.app.GetTopicDB(topicName)
    if err != nil {
        return nil, err
    }

    topicPath := s.app.GetTopicPath(topicName)
    stats := make(map[string]interface{})

    // Total size from assets
    var totalSize sql.NullInt64
    db.QueryRow("SELECT SUM(asset_size) FROM assets").Scan(&totalSize)
    stats["total_size"] = totalSize.Int64

    // File count
    var fileCount int64
    db.QueryRow("SELECT COUNT(*) FROM assets").Scan(&fileCount)
    stats["file_count"] = fileCount

    // Average size
    var avgSize sql.NullFloat64
    db.QueryRow("SELECT AVG(asset_size) FROM assets").Scan(&avgSize)
    stats["avg_size"] = avgSize.Float64

    // Last added
    var lastAdded sql.NullInt64
    db.QueryRow("SELECT MAX(created_at) FROM assets").Scan(&lastAdded)
    stats["last_added"] = lastAdded.Int64

    // Last hash
    var lastHash sql.NullString
    db.QueryRow("SELECT asset_id FROM assets ORDER BY created_at DESC LIMIT 1").Scan(&lastHash)
    stats["last_hash"] = lastHash.String

    // DB size (file size)
    dbPath := filepath.Join(topicPath, constants.InternalDir, topicName+".db")
    if info, err := os.Stat(dbPath); err == nil {
        stats["db_size"] = info.Size()
    }

    // DAT total size
    datSize, err := storage.GetTotalDatSize(topicPath)
    if err == nil {
        stats["dat_size"] = datSize
    }

    return stats, nil
}

func (s *Server) createTopic(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Name string `json:"name"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        WriteError(w, http.StatusBadRequest, "Invalid JSON", constants.ErrCodeInvalidRequest)
        return
    }

    // Validate topic name
    if req.Name == "" {
        WriteError(w, http.StatusBadRequest, "Topic name is required", constants.ErrCodeInvalidRequest)
        return
    }

    if len(req.Name) < constants.MinTopicNameLen || len(req.Name) > constants.MaxTopicNameLen {
        WriteError(w, http.StatusBadRequest, "Topic name length invalid", constants.ErrCodeInvalidTopicName)
        return
    }

    if !topicNameRegex.MatchString(req.Name) {
        WriteError(w, http.StatusBadRequest, "Topic name must contain only lowercase letters, numbers, hyphens, and underscores", constants.ErrCodeInvalidTopicName)
        return
    }

    // Check if already exists
    if _, exists := s.app.topicHealth[req.Name]; exists {
        WriteError(w, http.StatusConflict, "Topic already exists", constants.ErrCodeTopicAlreadyExists)
        return
    }

    topicPath := s.app.GetTopicPath(req.Name)

    // Check if folder already exists on disk
    if _, err := os.Stat(topicPath); err == nil {
        WriteError(w, http.StatusConflict, "Topic folder already exists", constants.ErrCodeTopicAlreadyExists)
        return
    }

    // Create topic folder structure
    // 1. Create topic folder
    if err := os.MkdirAll(topicPath, 0755); err != nil {
        WriteError(w, http.StatusInternalServerError, "Failed to create topic folder", constants.ErrCodeInternalError)
        return
    }

    // 2. Create .internal folder
    internalPath := filepath.Join(topicPath, constants.InternalDir)
    if err := os.MkdirAll(internalPath, 0755); err != nil {
        os.RemoveAll(topicPath) // Cleanup on failure
        WriteError(w, http.StatusInternalServerError, "Failed to create internal folder", constants.ErrCodeInternalError)
        return
    }

    // 3. Create topic database with schema
    dbPath := filepath.Join(internalPath, req.Name+".db")
    topicDB, err := database.InitTopicDB(dbPath)
    if err != nil {
        os.RemoveAll(topicPath) // Cleanup on failure
        WriteError(w, http.StatusInternalServerError, "Failed to create topic database", constants.ErrCodeInternalError)
        return
    }

    // Store the DB connection
    s.app.topicDBsMu.Lock()
    s.app.topicDBs[req.Name] = topicDB
    s.app.topicDBsMu.Unlock()

    // Register topic as healthy
    s.app.RegisterTopic(req.Name, true, "")

    s.logger.Info("Created topic: %s", req.Name)

    WriteSuccess(w, map[string]interface{}{
        "success": true,
        "name":    req.Name,
    })
}
```

**Add to imports:**
```go
import (
    "database/sql"
    "os"

    "meshbank/internal/storage"
)
```

---

## Task 7: Topic Sub-Routes Handler (`internal/server/handlers.go`)

```go
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
```

**Add to imports:**
```go
import (
    "strings"
)
```

---

## Task 8: Single Asset Upload (`internal/server/handlers.go`)

**Critical: Stream upload to temp file, then process. Never load entire file into memory.**

```go
// POST /api/topics/:name/assets - Upload single asset
func (s *Server) uploadAsset(w http.ResponseWriter, r *http.Request, topicName string) {
    // Parse multipart form with streaming
    // MaxMemory = 0 means all files go to disk (no memory buffering)
    if err := r.ParseMultipartForm(0); err != nil {
        WriteError(w, http.StatusBadRequest, "Failed to parse multipart form", constants.ErrCodeInvalidRequest)
        return
    }
    defer r.MultipartForm.RemoveAll() // Clean up temp files

    // Get the file
    file, header, err := r.FormFile("file")
    if err != nil {
        WriteError(w, http.StatusBadRequest, "No file provided", constants.ErrCodeInvalidRequest)
        return
    }
    defer file.Close()

    // Check file size against max_dat_size
    maxSize := s.app.Config.MaxDatSize
    if maxSize == 0 {
        maxSize = constants.DefaultMaxDatSize
    }

    // header.Size might not be accurate for streams, we'll check during hash computation
    if header.Size > maxSize-int64(constants.HeaderSize) {
        WriteError(w, http.StatusRequestEntityTooLarge, "File exceeds maximum size", constants.ErrCodeAssetTooLarge)
        return
    }

    // Get optional parent_id
    var parentID *string
    if pid := r.FormValue("parent_id"); pid != "" {
        // Validate parent exists in orchestrator
        exists, _, _, err := database.CheckHashExists(s.app.OrchestratorDB, pid)
        if err != nil {
            WriteError(w, http.StatusInternalServerError, "Failed to check parent", constants.ErrCodeInternalError)
            return
        }
        if !exists {
            WriteError(w, http.StatusBadRequest, "Parent asset not found", constants.ErrCodeParentNotFound)
            return
        }
        parentID = &pid
    }

    // Extract extension and origin name from filename
    filename := header.Filename
    ext := ""
    originName := ""

    if idx := strings.LastIndex(filename, "."); idx != -1 {
        ext = strings.ToLower(filename[idx+1:])
        originName = filename[:idx]
    } else {
        originName = filename
    }

    // Stream file to temp file while computing hash
    tempFile, hash, size, err := s.streamToTempWithHash(file, maxSize)
    if err != nil {
        if err.Error() == "file too large" {
            WriteError(w, http.StatusRequestEntityTooLarge, "File exceeds maximum size", constants.ErrCodeAssetTooLarge)
            return
        }
        WriteError(w, http.StatusInternalServerError, "Failed to process file", constants.ErrCodeInternalError)
        return
    }
    defer os.Remove(tempFile) // Clean up temp file

    // Check for duplicate
    exists, existingTopic, _, err := database.CheckHashExists(s.app.OrchestratorDB, hash)
    if err != nil {
        WriteError(w, http.StatusInternalServerError, "Failed to check duplicate", constants.ErrCodeInternalError)
        return
    }
    if exists {
        WriteSuccess(w, map[string]interface{}{
            "success":        true,
            "hash":           hash,
            "skipped":        true,
            "existing_topic": existingTopic,
        })
        return
    }

    // Get topic database
    topicDB, err := s.app.GetTopicDB(topicName)
    if err != nil {
        WriteError(w, http.StatusInternalServerError, "Failed to access topic", constants.ErrCodeInternalError)
        return
    }

    topicPath := s.app.GetTopicPath(topicName)

    // Write asset using pipeline (streams from temp file)
    asset, err := s.writeAssetFromTempFile(topicDB, topicName, topicPath, tempFile, hash, size, ext, originName, parentID)
    if err != nil {
        WriteError(w, http.StatusInternalServerError, "Failed to write asset: "+err.Error(), constants.ErrCodeInternalError)
        return
    }

    s.logger.Debug("Uploaded asset %s to topic %s", hash, topicName)

    WriteSuccess(w, map[string]interface{}{
        "success": true,
        "hash":    asset.AssetID,
        "size":    asset.AssetSize,
        "blob":    asset.BlobName,
        "skipped": false,
    })
}

// streamToTempWithHash streams data to a temp file while computing BLAKE3 hash
// Returns temp file path, hash, size, or error
func (s *Server) streamToTempWithHash(r io.Reader, maxSize int64) (tempPath string, hash string, size int64, err error) {
    // Create temp file
    tempFile, err := os.CreateTemp("", "meshbank-upload-*")
    if err != nil {
        return "", "", 0, fmt.Errorf("failed to create temp file: %w", err)
    }
    tempPath = tempFile.Name()

    // Setup hash writer
    hasher := blake3.New()

    // Create a multi-writer to write to both temp file and hasher
    multiWriter := io.MultiWriter(tempFile, hasher)

    // Copy with size limit
    limitReader := io.LimitReader(r, maxSize+1) // +1 to detect overflow
    size, err = io.Copy(multiWriter, limitReader)

    tempFile.Close() // Close before potential cleanup

    if err != nil {
        os.Remove(tempPath)
        return "", "", 0, fmt.Errorf("failed to write temp file: %w", err)
    }

    if size > maxSize-int64(constants.HeaderSize) {
        os.Remove(tempPath)
        return "", "", 0, fmt.Errorf("file too large")
    }

    // Get hash
    hashBytes := hasher.Sum(nil)
    hash = hex.EncodeToString(hashBytes)

    return tempPath, hash, size, nil
}

// writeAssetFromTempFile writes an asset from a temp file using the pipeline
func (s *Server) writeAssetFromTempFile(
    topicDB *sql.DB,
    topicName string,
    topicPath string,
    tempFile string,
    hash string,
    size int64,
    extension string,
    originName string,
    parentID *string,
) (*database.Asset, error) {

    maxDatSize := s.app.Config.MaxDatSize
    if maxDatSize == 0 {
        maxDatSize = constants.DefaultMaxDatSize
    }

    // Determine target .dat file
    entrySize := int64(constants.HeaderSize) + size
    datFile, _, err := storage.DetermineTargetDatFile(topicPath, entrySize, maxDatSize)
    if err != nil {
        return nil, fmt.Errorf("failed to determine dat file: %w", err)
    }

    datPath := filepath.Join(topicPath, datFile)

    // Begin transactions
    txTopic, err := topicDB.Begin()
    if err != nil {
        return nil, fmt.Errorf("failed to begin topic transaction: %w", err)
    }
    defer txTopic.Rollback()

    txOrch, err := s.app.OrchestratorDB.Begin()
    if err != nil {
        return nil, fmt.Errorf("failed to begin orchestrator transaction: %w", err)
    }
    defer txOrch.Rollback()

    // Append to .dat file by streaming from temp file
    byteOffset, err := s.appendFromTempFile(datPath, hash, tempFile, size)
    if err != nil {
        return nil, fmt.Errorf("failed to append to dat file: %w", err)
    }

    // Create asset record
    asset := database.Asset{
        AssetID:    hash,
        AssetSize:  size,
        OriginName: originName,
        ParentID:   parentID,
        Extension:  extension,
        BlobName:   datFile,
        ByteOffset: byteOffset,
        CreatedAt:  time.Now().Unix(),
    }

    if err := database.InsertAsset(txTopic, asset); err != nil {
        return nil, fmt.Errorf("failed to insert asset: %w", err)
    }

    if err := database.InsertAssetIndex(txOrch, hash, topicName, datFile); err != nil {
        return nil, fmt.Errorf("failed to insert asset index: %w", err)
    }

    // Compute new .dat hash
    newDatHash, err := storage.ComputeFileBlake3Hex(datPath)
    if err != nil {
        return nil, fmt.Errorf("failed to compute dat hash: %w", err)
    }

    if err := database.UpdateDatHash(txTopic, datFile, newDatHash); err != nil {
        return nil, fmt.Errorf("failed to update dat hash: %w", err)
    }

    // Commit transactions
    if err := txTopic.Commit(); err != nil {
        return nil, fmt.Errorf("failed to commit topic transaction: %w", err)
    }

    if err := txOrch.Commit(); err != nil {
        s.logger.Warn("Orchestrator commit failed (will recover on restart): %v", err)
    }

    return &asset, nil
}

// appendFromTempFile appends data from temp file to .dat file
func (s *Server) appendFromTempFile(datPath string, hash string, tempFile string, size int64) (byteOffset int64, err error) {
    // Serialize header
    header, err := storage.SerializeHeader(hash, uint64(size))
    if err != nil {
        return 0, err
    }

    // Open .dat file for appending
    datFile, err := os.OpenFile(datPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        return 0, fmt.Errorf("failed to open dat file: %w", err)
    }
    defer datFile.Close()

    // Get current offset
    stat, err := datFile.Stat()
    if err != nil {
        return 0, fmt.Errorf("failed to stat dat file: %w", err)
    }
    byteOffset = stat.Size()

    // Write header
    if _, err := datFile.Write(header); err != nil {
        return 0, fmt.Errorf("failed to write header: %w", err)
    }

    // Stream data from temp file
    srcFile, err := os.Open(tempFile)
    if err != nil {
        return 0, fmt.Errorf("failed to open temp file: %w", err)
    }
    defer srcFile.Close()

    if _, err := io.Copy(datFile, srcFile); err != nil {
        return 0, fmt.Errorf("failed to copy data: %w", err)
    }

    // Sync to ensure durability
    if err := datFile.Sync(); err != nil {
        return 0, fmt.Errorf("failed to sync dat file: %w", err)
    }

    return byteOffset, nil
}
```

**Add to imports:**
```go
import (
    "encoding/hex"
    "io"
    "time"

    "github.com/zeebo/blake3"
)
```

---

## Task 9: Asset Download (`internal/server/handlers.go`)

```go
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
    case action == "metadata" && r.Method == http.MethodPost:
        s.postMetadata(w, r, hash)
    default:
        http.NotFound(w, r)
    }
}

// GET /api/assets/:hash/download - Download asset
func (s *Server) downloadAsset(w http.ResponseWriter, r *http.Request, hash string) {
    // Look up in orchestrator to find topic
    exists, topicName, _, err := database.CheckHashExists(s.app.OrchestratorDB, hash)
    if err != nil {
        WriteError(w, http.StatusInternalServerError, "Failed to lookup asset", constants.ErrCodeInternalError)
        return
    }
    if !exists {
        WriteError(w, http.StatusNotFound, "Asset not found", constants.ErrCodeAssetNotFound)
        return
    }

    // Check topic health
    healthy, errMsg := s.app.IsTopicHealthy(topicName)
    if !healthy {
        WriteError(w, http.StatusServiceUnavailable, "Topic is unhealthy: "+errMsg, constants.ErrCodeTopicUnhealthy)
        return
    }

    // Get asset details from topic DB
    topicDB, err := s.app.GetTopicDB(topicName)
    if err != nil {
        WriteError(w, http.StatusInternalServerError, "Failed to access topic", constants.ErrCodeInternalError)
        return
    }

    asset, err := database.GetAsset(topicDB, hash)
    if err != nil {
        WriteError(w, http.StatusInternalServerError, "Failed to get asset details", constants.ErrCodeInternalError)
        return
    }
    if asset == nil {
        WriteError(w, http.StatusNotFound, "Asset not found in topic", constants.ErrCodeAssetNotFound)
        return
    }

    // Set Content-Type based on extension
    contentType := constants.DefaultMimeType
    if mimeType, ok := constants.ExtensionMimeTypes[asset.Extension]; ok {
        contentType = mimeType
    }
    w.Header().Set("Content-Type", contentType)

    // Set Content-Disposition for download
    filename := hash
    if asset.OriginName != "" {
        filename = asset.OriginName
    }
    if asset.Extension != "" {
        filename = filename + "." + asset.Extension
    }
    w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

    // Set Content-Length
    w.Header().Set("Content-Length", fmt.Sprintf("%d", asset.AssetSize))

    // Stream the file
    topicPath := s.app.GetTopicPath(topicName)
    datPath := filepath.Join(topicPath, asset.BlobName)

    f, err := os.Open(datPath)
    if err != nil {
        WriteError(w, http.StatusInternalServerError, "Failed to open data file", constants.ErrCodeInternalError)
        return
    }
    defer f.Close()

    // Seek to data start (skip header)
    dataStart := asset.ByteOffset + int64(constants.HeaderSize)
    if _, err := f.Seek(dataStart, io.SeekStart); err != nil {
        WriteError(w, http.StatusInternalServerError, "Failed to seek in data file", constants.ErrCodeInternalError)
        return
    }

    // Stream data
    io.CopyN(w, f, asset.AssetSize)
}
```

---

## Task 10: Metadata Handler (`internal/server/handlers.go`)

```go
// POST /api/assets/:hash/metadata - Add/delete metadata
func (s *Server) postMetadata(w http.ResponseWriter, r *http.Request, hash string) {
    // Look up in orchestrator to find topic
    exists, topicName, _, err := database.CheckHashExists(s.app.OrchestratorDB, hash)
    if err != nil {
        WriteError(w, http.StatusInternalServerError, "Failed to lookup asset", constants.ErrCodeInternalError)
        return
    }
    if !exists {
        WriteError(w, http.StatusNotFound, "Asset not found", constants.ErrCodeAssetNotFound)
        return
    }

    // Check topic health
    healthy, errMsg := s.app.IsTopicHealthy(topicName)
    if !healthy {
        WriteError(w, http.StatusServiceUnavailable, "Topic is unhealthy: "+errMsg, constants.ErrCodeTopicUnhealthy)
        return
    }

    // Parse request
    var req struct {
        Op               string      `json:"op"`
        Key              string      `json:"key"`
        Value            interface{} `json:"value"`
        Processor        string      `json:"processor"`
        ProcessorVersion string      `json:"processor_version"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        WriteError(w, http.StatusBadRequest, "Invalid JSON", constants.ErrCodeInvalidRequest)
        return
    }

    // Validate request
    if req.Op != "set" && req.Op != "delete" {
        WriteError(w, http.StatusBadRequest, "op must be 'set' or 'delete'", constants.ErrCodeInvalidRequest)
        return
    }
    if req.Key == "" {
        WriteError(w, http.StatusBadRequest, "key is required", constants.ErrCodeInvalidRequest)
        return
    }
    if req.Processor == "" {
        WriteError(w, http.StatusBadRequest, "processor is required", constants.ErrCodeInvalidRequest)
        return
    }
    if req.ProcessorVersion == "" {
        WriteError(w, http.StatusBadRequest, "processor_version is required", constants.ErrCodeInvalidRequest)
        return
    }

    // Convert value to string
    valueStr := ""
    if req.Op == "set" {
        if req.Value == nil {
            WriteError(w, http.StatusBadRequest, "value is required for set operation", constants.ErrCodeInvalidRequest)
            return
        }
        switch v := req.Value.(type) {
        case string:
            valueStr = v
        case float64:
            // JSON numbers are float64
            valueStr = strconv.FormatFloat(v, 'f', -1, 64)
        case bool:
            valueStr = strconv.FormatBool(v)
        default:
            WriteError(w, http.StatusBadRequest, "value must be string, number, or boolean", constants.ErrCodeInvalidRequest)
            return
        }
    }

    // Get topic database
    topicDB, err := s.app.GetTopicDB(topicName)
    if err != nil {
        WriteError(w, http.StatusInternalServerError, "Failed to access topic", constants.ErrCodeInternalError)
        return
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
        WriteError(w, http.StatusInternalServerError, "Failed to insert metadata: "+err.Error(), constants.ErrCodeMetadataError)
        return
    }

    // Get updated computed metadata
    computed, err := database.GetMetadataComputed(topicDB, hash)
    if err != nil {
        s.logger.Warn("Failed to get computed metadata: %v", err)
        computed = make(map[string]interface{})
    }

    WriteSuccess(w, map[string]interface{}{
        "success":           true,
        "log_id":            logID,
        "computed_metadata": computed,
    })
}
```

**Add to imports:**
```go
import (
    "strconv"
)
```

---

## Task 11: Logs Handler (Placeholder) (`internal/server/handlers.go`)

```go
// GET /api/logs - Get system error logs
func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // Placeholder - full implementation in Phase 8
    // For now, return empty logs
    WriteSuccess(w, map[string]interface{}{
        "logs": []interface{}{},
    })
}
```

---

## Task 12: Update Main Entry Point (`cmd/meshbank/main.go`)

Update main.go to use the new server:

```go
package main

import (
    "fmt"
    "os"
    "path/filepath"

    "meshbank/internal/config"
    "meshbank/internal/constants"
    "meshbank/internal/database"
    "meshbank/internal/logger"
    "meshbank/internal/server"
)

func main() {
    // 1. Initialize debug logger
    log := logger.NewLogger(constants.DefaultLogLevel)

    // 2. Load or create config
    log.Info("Loading configuration...")
    cfg, err := config.LoadConfig()
    if err != nil {
        log.Error("Failed to load config: %v", err)
        os.Exit(1)
    }
    log.Debug("Config directory: %s", config.GetConfigDir())

    // 3. Ensure queries.yaml exists with defaults
    if err := config.EnsureQueriesFile(); err != nil {
        log.Error("Failed to create queries.yaml: %v", err)
        os.Exit(1)
    }
    log.Debug("Queries file ready: %s", config.GetQueriesPath())

    // 4. Create application instance
    app := server.NewApp(cfg, log)

    // 5. If working_directory is set and valid, initialize it
    if cfg.WorkingDirectory != "" {
        log.Info("Initializing working directory: %s", cfg.WorkingDirectory)
        if err := config.InitializeWorkingDirectory(cfg.WorkingDirectory); err != nil {
            log.Error("Failed to initialize working directory: %v", err)
            cfg.WorkingDirectory = "" // Clear invalid path
        } else {
            // Open orchestrator DB
            orchPath := filepath.Join(cfg.WorkingDirectory, constants.InternalDir, constants.OrchestratorDB)
            orchDB, err := database.InitOrchestratorDB(orchPath)
            if err != nil {
                log.Error("Failed to open orchestrator database: %v", err)
                os.Exit(1)
            }
            app.OrchestratorDB = orchDB

            // Discover existing topics
            topics, err := config.DiscoverTopics(cfg.WorkingDirectory)
            if err != nil {
                log.Warn("Topic discovery failed: %v", err)
            } else {
                log.Info("Discovered %d topic(s)", len(topics))
                for _, t := range topics {
                    app.RegisterTopic(t.Name, t.Healthy, t.Error)
                    if t.Healthy {
                        log.Debug("  - %s (healthy)", t.Name)
                        // Index to orchestrator
                        if err := database.IndexTopicToOrchestrator(t.Path, t.Name, app.OrchestratorDB); err != nil {
                            log.Warn("Failed to index topic %s: %v", t.Name, err)
                        }
                    } else {
                        log.Warn("  - %s (unhealthy: %s)", t.Name, t.Error)
                    }
                }
            }
        }
    } else {
        log.Warn("Working directory not set - configure via dashboard")
    }

    // 6. Start HTTP server
    port := cfg.Port
    if port == 0 {
        port = constants.DefaultPort
    }

    addr := fmt.Sprintf(":%d", port)
    srv := server.NewServer(app, addr)

    log.Info("Starting MeshBank server on port %d", port)
    if err := srv.Start(); err != nil {
        log.Error("Server error: %v", err)
        os.Exit(1)
    }
}
```

---

## Verification Checklist

After completing Phase 4, verify:

1. **Server starts:**
   - Run `go build ./cmd/meshbank && ./meshbank`
   - Server starts on port 2369
   - Ctrl+C gracefully shuts down

2. **GET /api/config:**
   - Returns `configured: false` if no working_directory
   - Returns full config after setting working_directory

3. **POST /api/config:**
   - Set working_directory to valid path
   - Creates `.internal/` and `orchestrator.db`
   - Rescans topics

4. **POST /api/topics:**
   - Create topic with valid name → success
   - Create duplicate → 409 conflict
   - Invalid name (uppercase, special chars) → 400 validation error

5. **GET /api/topics:**
   - Lists all topics with stats
   - Shows healthy/unhealthy status

6. **POST /api/topics/:name/assets:**
   - Upload small file → success with hash
   - Upload duplicate → skipped: true
   - Upload with parent_id → validates parent exists
   - Upload to unhealthy topic → 503 error
   - Upload file > max_dat_size → 413 error

7. **GET /api/assets/:hash/download:**
   - Download by hash → correct file content
   - Correct Content-Type header
   - Correct Content-Disposition with filename
   - Asset in unhealthy topic → 503 error

8. **POST /api/assets/:hash/metadata:**
   - Set metadata → success with computed metadata
   - Delete metadata → removes from computed
   - Invalid op → 400 error

9. **Memory efficiency:**
   - Upload 100MB+ file with < 2GB system memory
   - Monitor memory usage during upload (should not spike)

---

## Files to Create/Update

| File | Action |
|------|--------|
| `internal/constants/errors.go` | Create |
| `internal/server/app.go` | Create |
| `internal/server/response.go` | Create |
| `internal/server/server.go` | Create |
| `internal/server/handlers.go` | Create |
| `cmd/meshbank/main.go` | Update |

---

## Notes for Agent

- **Streaming is critical:** Never load entire upload into memory. Use temp file approach.
- **Use `ParseMultipartForm(0)`:** Setting maxMemory to 0 forces all files to disk.
- **Always defer cleanup:** `r.MultipartForm.RemoveAll()` and `os.Remove(tempFile)`
- **Transaction ordering:** File I/O first, then DB commits
- **Cross-topic parent:** Use orchestrator.db to validate parent_id across all topics
- **No CORS headers:** Same-origin only
- **Graceful shutdown:** Close all DB connections on SIGINT/SIGTERM
- **Error format:** Always use `WriteError()` with proper error code
- **Standard net/http:** No external router dependencies
- **Topic health check:** Check before any topic operation