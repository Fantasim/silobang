package server

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"meshbank/internal/audit"
	"meshbank/internal/config"
	"meshbank/internal/constants"
	"meshbank/internal/database"
	"meshbank/internal/logger"
	"meshbank/internal/prompts"
	"meshbank/internal/queries"
	"meshbank/internal/services"
)

// App holds all application state and dependencies
type App struct {
	Config         *config.Config
	Logger         *logger.Logger
	OrchestratorDB *sql.DB
	QueriesConfig  *queries.QueriesConfig
	PromptsManager *prompts.Manager
	AuditLogger    *audit.Logger
	StartedAt      time.Time

	// Services layer for business logic
	Services *services.Services

	// Topic databases - lazily opened, keyed by topic name
	topicDBs   map[string]*sql.DB
	topicDBsMu sync.RWMutex

	// Topic health status - keyed by topic name
	topicHealth   map[string]*TopicHealth
	topicHealthMu sync.RWMutex

	// Per-topic write mutex - serializes uploads to prevent byte offset
	// collisions and duplicate detection races within the same topic
	topicWriteMu   map[string]*sync.Mutex
	topicWriteMuMu sync.Mutex

	// Global topic creation mutex - serializes topic creation to prevent
	// filesystem races when concurrent requests create the same topic
	topicCreateMu sync.Mutex
}

// TopicHealth tracks the health status of a topic
type TopicHealth struct {
	Healthy bool
	Error   string // empty if healthy
}

// NewApp creates a new application instance
func NewApp(cfg *config.Config, log *logger.Logger) *App {
	app := &App{
		Config:      cfg,
		Logger:      log,
		StartedAt:   time.Now(),
		topicDBs:     make(map[string]*sql.DB),
		topicHealth:  make(map[string]*TopicHealth),
		topicWriteMu: make(map[string]*sync.Mutex),
	}

	// Initialize services layer
	app.Services = services.NewServices(app, log)

	return app
}

// ReinitServices re-initializes the services layer.
// Call this after the orchestrator DB becomes available so that
// DB-dependent services (like AuthService) can be created.
func (a *App) ReinitServices() {
	a.Services = services.NewServices(a, a.Logger)
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

// UnregisterTopic removes a topic from the health registry and closes its DB
func (a *App) UnregisterTopic(name string) {
	// Close DB if open
	a.topicDBsMu.Lock()
	if db, exists := a.topicDBs[name]; exists {
		db.Close()
		delete(a.topicDBs, name)
	}
	a.topicDBsMu.Unlock()

	// Remove from health registry
	a.topicHealthMu.Lock()
	delete(a.topicHealth, name)
	a.topicHealthMu.Unlock()

	// Remove write mutex
	a.topicWriteMuMu.Lock()
	delete(a.topicWriteMu, name)
	a.topicWriteMuMu.Unlock()
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

// ClearTopicRegistry clears all topic health entries and write mutexes
func (a *App) ClearTopicRegistry() {
	a.topicHealthMu.Lock()
	defer a.topicHealthMu.Unlock()
	a.topicHealth = make(map[string]*TopicHealth)

	a.topicWriteMuMu.Lock()
	defer a.topicWriteMuMu.Unlock()
	a.topicWriteMu = make(map[string]*sync.Mutex)
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

// TopicExists checks if a topic is registered (regardless of health)
func (a *App) TopicExists(topicName string) bool {
	a.topicHealthMu.RLock()
	defer a.topicHealthMu.RUnlock()
	_, exists := a.topicHealth[topicName]
	return exists
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

// StoreTopicDB stores a topic database connection (used when creating topics)
func (a *App) StoreTopicDB(name string, db *sql.DB) {
	a.topicDBsMu.Lock()
	defer a.topicDBsMu.Unlock()
	a.topicDBs[name] = db
}

// GetTopicDBsForQuery returns database connections for the requested topics
// If topicNames is empty, returns all healthy topics
// Returns error if any requested topic is unhealthy
func (a *App) GetTopicDBsForQuery(topicNames []string) (map[string]*sql.DB, []string, error) {
	// If no topics specified, use all healthy topics
	if len(topicNames) == 0 {
		topicNames = a.ListTopics()
	}

	result := make(map[string]*sql.DB)
	var validNames []string

	for _, name := range topicNames {
		healthy, errMsg := a.IsTopicHealthy(name)
		if !healthy {
			return nil, nil, fmt.Errorf("topic %s is unhealthy: %s", name, errMsg)
		}

		db, err := a.GetTopicDB(name)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get database for topic %s: %w", name, err)
		}

		result[name] = db
		validNames = append(validNames, name)
	}

	return result, validNames, nil
}

// AppState interface implementation
// These methods provide access to App state for the services layer.

// GetOrchestratorDB returns the orchestrator database connection.
func (a *App) GetOrchestratorDB() *sql.DB {
	return a.OrchestratorDB
}

// SetOrchestratorDB sets the orchestrator database connection.
func (a *App) SetOrchestratorDB(db *sql.DB) {
	a.OrchestratorDB = db
}

// GetWorkingDirectory returns the configured working directory path.
func (a *App) GetWorkingDirectory() string {
	return a.Config.WorkingDirectory
}

// GetConfig returns the application configuration.
func (a *App) GetConfig() *config.Config {
	return a.Config
}

// GetLogger returns the application logger.
func (a *App) GetLogger() *logger.Logger {
	return a.Logger
}

// GetQueriesConfig returns the queries configuration.
func (a *App) GetQueriesConfig() *queries.QueriesConfig {
	return a.QueriesConfig
}

// SetQueriesConfig sets the queries configuration.
func (a *App) SetQueriesConfig(qc *queries.QueriesConfig) {
	a.QueriesConfig = qc
}

// GetPromptsManager returns the prompts manager.
func (a *App) GetPromptsManager() *prompts.Manager {
	return a.PromptsManager
}

// SetPromptsManager sets the prompts manager.
func (a *App) SetPromptsManager(pm *prompts.Manager) {
	a.PromptsManager = pm
}

// GetAuditLogger returns the audit logger.
func (a *App) GetAuditLogger() *audit.Logger {
	return a.AuditLogger
}

// GetStartedAt returns the server start time.
func (a *App) GetStartedAt() time.Time {
	return a.StartedAt
}

// GetTopicWriteMu returns the write mutex for a topic, creating it lazily.
// This mutex serializes all write operations (uploads) to a topic to prevent
// byte offset collisions and duplicate detection races.
func (a *App) GetTopicWriteMu(topicName string) *sync.Mutex {
	a.topicWriteMuMu.Lock()
	defer a.topicWriteMuMu.Unlock()

	mu, exists := a.topicWriteMu[topicName]
	if !exists {
		mu = &sync.Mutex{}
		a.topicWriteMu[topicName] = mu
	}
	return mu
}

// GetTopicCreateMu returns the global topic creation mutex.
// Topic creation involves filesystem operations (mkdir, DB init) that must be
// serialized to prevent race conditions when concurrent requests try to create
// the same topic simultaneously.
func (a *App) GetTopicCreateMu() *sync.Mutex {
	return &a.topicCreateMu
}
