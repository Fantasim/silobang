// Package services provides the business logic layer for silobang.
// Services orchestrate operations across database, storage, and config packages.
// HTTP handlers should delegate to services for all business logic.
package services

import (
	"database/sql"
	"sync"
	"time"

	"silobang/internal/audit"
	"silobang/internal/config"
	"silobang/internal/logger"
	"silobang/internal/prompts"
	"silobang/internal/queries"
)

// AppState provides access to shared application state.
// This interface decouples services from the concrete App type.
type AppState interface {
	// Database access
	GetOrchestratorDB() *sql.DB
	GetTopicDB(topicName string) (*sql.DB, error)
	GetTopicDBsForQuery(topicNames []string) (map[string]*sql.DB, []string, error)
	StoreTopicDB(name string, db *sql.DB)

	// Topic registry
	RegisterTopic(name string, healthy bool, errMsg string)
	UnregisterTopic(name string)
	TopicExists(topicName string) bool
	IsTopicHealthy(topicName string) (bool, string)
	ListTopics() []string
	ClearTopicRegistry()
	CloseAllTopicDBs()

	// Paths
	GetTopicPath(topicName string) string
	GetWorkingDirectory() string

	// Config and dependencies
	GetConfig() *config.Config
	GetLogger() *logger.Logger
	GetQueriesConfig() *queries.QueriesConfig
	SetQueriesConfig(qc *queries.QueriesConfig)
	GetPromptsManager() *prompts.Manager
	SetPromptsManager(pm *prompts.Manager)
	GetAuditLogger() *audit.Logger
	SetOrchestratorDB(db *sql.DB)
	GetStartedAt() time.Time

	// Concurrency control
	GetTopicWriteMu(topicName string) *sync.Mutex
	GetTopicCreateMu() *sync.Mutex
}

// Services holds all service instances for the application.
// It acts as a service container that is initialized once at startup.
type Services struct {
	app    AppState
	logger *logger.Logger

	// Service instances
	Asset      *AssetService
	Auth       *AuthService
	Config     *ConfigService
	Metadata   *MetadataService
	Query      *QueryService
	Bulk       *BulkService
	Verify     *VerifyService
	Schema     *SchemaService
	Monitoring *MonitoringService
	Reconcile  *ReconcileService
}

// NewServices creates a new service container with all services initialized.
func NewServices(app AppState, log *logger.Logger) *Services {
	s := &Services{
		app:    app,
		logger: log,
	}

	// Initialize services
	s.Asset = NewAssetService(app, log)
	s.Auth = NewAuthService(app, log)
	s.Config = NewConfigService(app, log)
	s.Metadata = NewMetadataService(app, log)
	s.Query = NewQueryService(app, log)
	s.Bulk = NewBulkService(app, log)
	s.Verify = NewVerifyService(app, log)
	s.Schema = NewSchemaService(app, log)
	s.Monitoring = NewMonitoringService(app, log)
	s.Reconcile = NewReconcileService(app, log)

	return s
}

// App returns the underlying app state for services that need direct access.
// This should be used sparingly - prefer using service methods.
func (s *Services) App() AppState {
	return s.app
}

// Logger returns the application logger.
func (s *Services) Logger() *logger.Logger {
	return s.logger
}
