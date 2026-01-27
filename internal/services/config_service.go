package services

import (
	stdsql "database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"meshbank/internal/audit"
	"meshbank/internal/config"
	"meshbank/internal/constants"
	"meshbank/internal/database"
	"meshbank/internal/logger"
	"meshbank/internal/prompts"
	"meshbank/internal/queries"
	"meshbank/internal/storage"
)

var topicNameRegex = regexp.MustCompile(constants.TopicNameRegex)

// ConfigService handles working directory configuration and topic management.
type ConfigService struct {
	app    AppState
	logger *logger.Logger
}

// NewConfigService creates a new config service instance.
func NewConfigService(app AppState, log *logger.Logger) *ConfigService {
	return &ConfigService{
		app:    app,
		logger: log,
	}
}

// ConfigStatus represents the current configuration state.
type ConfigStatus struct {
	Configured       bool   `json:"configured"`
	WorkingDirectory string `json:"working_directory"`
	Port             int    `json:"port"`
	MaxDatSize       int64  `json:"max_dat_size"`
}

// GetStatus returns the current configuration status.
func (s *ConfigService) GetStatus() *ConfigStatus {
	cfg := s.app.GetConfig()
	return &ConfigStatus{
		Configured:       cfg.WorkingDirectory != "",
		WorkingDirectory: cfg.WorkingDirectory,
		Port:             cfg.Port,
		MaxDatSize:       cfg.MaxDatSize,
	}
}

// SetWorkingDirectory changes the working directory and reinitializes all databases.
// This is equivalent to a "project restart" - all existing connections are closed.
func (s *ConfigService) SetWorkingDirectory(workingDir string, serverPort int) error {
	if workingDir == "" {
		return NewServiceError(constants.ErrCodeInvalidRequest, "working_directory is required")
	}

	// Validate directory exists
	if err := config.ValidateWorkingDirectory(workingDir); err != nil {
		return WrapServiceError(constants.ErrCodeInvalidRequest, err.Error(), err)
	}

	// Close existing connections (project restart behavior)
	s.app.CloseAllTopicDBs()
	s.app.ClearTopicRegistry()
	if s.app.GetOrchestratorDB() != nil {
		s.app.GetOrchestratorDB().Close()
		s.app.SetOrchestratorDB(nil)
	}

	// Initialize new working directory
	if err := config.InitializeWorkingDirectory(workingDir); err != nil {
		return WrapInternalError(err)
	}

	// Update and save config
	cfg := s.app.GetConfig()
	cfg.WorkingDirectory = workingDir
	if err := config.SaveConfig(cfg); err != nil {
		return WrapInternalError(fmt.Errorf("failed to save config: %w", err))
	}

	// Open orchestrator DB
	orchPath := filepath.Join(workingDir, constants.InternalDir, constants.OrchestratorDB)
	orchDB, err := database.InitOrchestratorDB(orchPath)
	if err != nil {
		return WrapInternalError(fmt.Errorf("failed to open orchestrator database: %w", err))
	}
	s.app.SetOrchestratorDB(orchDB)

	// Initialize audit logger (need to get the actual logger interface)
	// Note: This requires the App to expose a method to set the audit logger
	// For now, we'll handle this in the handler

	// Discover and register topics
	topics, err := config.DiscoverTopics(workingDir)
	if err != nil {
		s.logger.Warn("Topic discovery error: %v", err)
	}

	for _, topic := range topics {
		s.app.RegisterTopic(topic.Name, topic.Healthy, topic.Error)
		if topic.Healthy {
			// Index to orchestrator
			if err := config.IndexTopicToOrchestrator(topic.Path, topic.Name, s.app.GetOrchestratorDB()); err != nil {
				s.logger.Warn("Failed to index topic %s: %v", topic.Name, err)
			}
		}
	}

	// Load queries from .internal/queries/ directory (auto-generates if missing)
	queriesConfig, err := queries.LoadQueries(workingDir, s.logger)
	if err != nil {
		s.logger.Warn("Failed to load queries: %v, using defaults", err)
		queriesConfig = queries.GetDefaultConfig()
	}
	s.app.SetQueriesConfig(queriesConfig)

	// Initialize prompts manager
	port := serverPort
	if port == 0 {
		port = constants.DefaultPort
	}
	baseURL := fmt.Sprintf("http://localhost:%d", port)
	promptsManager := prompts.NewManager(workingDir, baseURL)
	if err := promptsManager.EnsurePromptsDir(workingDir, s.logger); err != nil {
		s.logger.Warn("Failed to initialize prompts directory: %v", err)
	}
	if err := promptsManager.LoadPrompts(s.logger); err != nil {
		s.logger.Warn("Failed to load prompts: %v", err)
	}
	s.app.SetPromptsManager(promptsManager)

	// Enable file logging for the new working directory
	if err := s.logger.SetWorkDir(workingDir); err != nil {
		s.logger.Warn("Failed to enable file logging: %v", err)
	} else {
		s.logger.Info("File logging enabled in %s", workingDir)
	}

	s.logger.Info("Working directory changed to: %s, discovered %d topics", workingDir, len(topics))

	return nil
}

// TopicInfo represents information about a topic.
type TopicInfo struct {
	Name    string                 `json:"name"`
	Stats   map[string]interface{} `json:"stats,omitempty"`
	Healthy bool                   `json:"healthy"`
	Error   string                 `json:"error,omitempty"`
}

// TopicsListResult contains the list of topics and their stats for aggregation.
type TopicsListResult struct {
	Topics   []TopicInfo
	AllStats map[string]map[string]interface{}
}

// ListTopics returns all registered topics with their stats.
// The caller is responsible for computing service-level info using AllStats.
func (s *ConfigService) ListTopics() (*TopicsListResult, error) {
	if s.app.GetWorkingDirectory() == "" {
		return nil, ErrNotConfigured
	}

	topicNames := s.app.ListTopics()
	topics := make([]TopicInfo, 0, len(topicNames))

	// Collect stats for aggregation
	allStats := make(map[string]map[string]interface{})

	for _, name := range topicNames {
		healthy, errMsg := s.app.IsTopicHealthy(name)

		ti := TopicInfo{
			Name:    name,
			Healthy: healthy,
		}

		if !healthy {
			ti.Error = errMsg
		} else {
			// Get stats for healthy topics
			stats, err := s.GetTopicStats(name)
			if err != nil {
				s.logger.Warn("Failed to get stats for topic %s: %v", name, err)
			} else {
				ti.Stats = stats
				allStats[name] = stats
			}
		}

		topics = append(topics, ti)
	}

	return &TopicsListResult{
		Topics:   topics,
		AllStats: allStats,
	}, nil
}

// GetTopicStats returns statistics for a single topic.
func (s *ConfigService) GetTopicStats(topicName string) (map[string]interface{}, error) {
	db, err := s.app.GetTopicDB(topicName)
	if err != nil {
		return nil, s.wrapTopicError(topicName, err)
	}

	topicPath := s.app.GetTopicPath(topicName)
	stats := make(map[string]interface{})

	// If no queries config, use hardcoded defaults
	qc := s.app.GetQueriesConfig()
	if qc == nil || len(qc.TopicStats) == 0 {
		return s.getDefaultTopicStats(db, topicName, topicPath)
	}

	// Execute each stat query
	for _, stat := range qc.TopicStats {
		var value interface{}

		switch stat.Type {
		case "file_size":
			// Special: read database file size
			dbPath := filepath.Join(topicPath, constants.InternalDir, topicName+".db")
			if info, statErr := os.Stat(dbPath); statErr == nil {
				value = info.Size()
			} else {
				value = int64(0)
			}
		case "dat_total":
			// Special: sum of all .dat file sizes
			datSize, datErr := storage.GetTotalDatSize(topicPath)
			if datErr != nil {
				value = int64(0)
			} else {
				value = datSize
			}
		default:
			// SQL query
			if stat.SQL != "" {
				value = s.executeStat(db, stat.SQL, stat.Format)
			}
		}

		stats[stat.Name] = value
	}

	return stats, nil
}

// executeStat executes a stat query and returns the appropriate type.
func (s *ConfigService) executeStat(db *stdsql.DB, sql string, format string) interface{} {
	switch format {
	case constants.StatFormatFloat:
		var result stdsql.NullFloat64
		if err := db.QueryRow(sql).Scan(&result); err == nil && result.Valid {
			return result.Float64
		}
	case constants.StatFormatNumber, constants.StatFormatBytes:
		var result stdsql.NullInt64
		if err := db.QueryRow(sql).Scan(&result); err == nil && result.Valid {
			return result.Int64
		}
	case constants.StatFormatDate:
		var result stdsql.NullInt64
		if err := db.QueryRow(sql).Scan(&result); err == nil && result.Valid {
			return result.Int64
		}
	default:
		var result stdsql.NullString
		if err := db.QueryRow(sql).Scan(&result); err == nil && result.Valid {
			return result.String
		}
	}
	return nil
}

// getDefaultTopicStats returns hardcoded stats when no config is available.
func (s *ConfigService) getDefaultTopicStats(db *stdsql.DB, topicName string, topicPath string) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total size from assets
	var totalSize stdsql.NullInt64
	db.QueryRow("SELECT SUM(asset_size) FROM assets").Scan(&totalSize)
	stats["total_size"] = totalSize.Int64

	// File count
	var fileCount int64
	db.QueryRow("SELECT COUNT(*) FROM assets").Scan(&fileCount)
	stats["file_count"] = fileCount

	// Average size
	var avgSize stdsql.NullFloat64
	db.QueryRow("SELECT AVG(asset_size) FROM assets").Scan(&avgSize)
	stats["avg_size"] = avgSize.Float64

	// Last added
	var lastAdded stdsql.NullInt64
	db.QueryRow("SELECT MAX(created_at) FROM assets").Scan(&lastAdded)
	stats["last_added"] = lastAdded.Int64

	// Last hash
	var lastHash stdsql.NullString
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

// CreateTopic creates a new topic with the given name.
func (s *ConfigService) CreateTopic(name string) error {
	if s.app.GetWorkingDirectory() == "" {
		return ErrNotConfigured
	}

	// Validate topic name
	if name == "" {
		return NewServiceError(constants.ErrCodeInvalidRequest, "topic name is required")
	}

	if len(name) < constants.MinTopicNameLen || len(name) > constants.MaxTopicNameLen {
		return ErrInvalidTopicName
	}

	if !topicNameRegex.MatchString(name) {
		return NewServiceError(constants.ErrCodeInvalidTopicName, "topic name must contain only lowercase letters, numbers, hyphens, and underscores")
	}

	// Acquire global topic creation lock to prevent filesystem races
	// when concurrent requests try to create the same topic simultaneously
	mu := s.app.GetTopicCreateMu()
	mu.Lock()
	defer mu.Unlock()

	s.logger.Debug("Acquired topic creation lock for topic %s", name)

	// Check if already exists in registry (inside lock to prevent race)
	if s.app.TopicExists(name) {
		return ErrTopicAlreadyExists
	}

	topicPath := s.app.GetTopicPath(name)

	// Check if folder already exists on disk
	if _, err := os.Stat(topicPath); err == nil {
		return NewServiceError(constants.ErrCodeTopicAlreadyExists, "topic folder already exists")
	}

	// Create topic folder structure
	if err := os.MkdirAll(topicPath, constants.DirPermissions); err != nil {
		return WrapInternalError(fmt.Errorf("failed to create topic folder: %w", err))
	}

	// Create .internal folder
	internalPath := filepath.Join(topicPath, constants.InternalDir)
	if err := os.MkdirAll(internalPath, constants.DirPermissions); err != nil {
		os.RemoveAll(topicPath) // Cleanup on failure
		return WrapInternalError(fmt.Errorf("failed to create internal folder: %w", err))
	}

	// Create topic database with schema
	dbPath := filepath.Join(internalPath, name+".db")
	topicDB, err := database.InitTopicDB(dbPath)
	if err != nil {
		os.RemoveAll(topicPath) // Cleanup on failure
		return WrapInternalError(fmt.Errorf("failed to create topic database: %w", err))
	}

	// Store the DB connection and register topic
	s.app.StoreTopicDB(name, topicDB)
	s.app.RegisterTopic(name, true, "")

	s.logger.Info("Created new topic: %s", name)

	return nil
}

// SetAuditLogger initializes the audit logger after working directory is set.
// This should be called from the handler after SetWorkingDirectory.
func (s *ConfigService) SetAuditLogger() *audit.Logger {
	orchDB := s.app.GetOrchestratorDB()
	if orchDB == nil {
		return nil
	}
	return audit.NewLogger(orchDB)
}

// wrapTopicError wraps topic-related errors with appropriate service errors.
func (s *ConfigService) wrapTopicError(topicName string, err error) *ServiceError {
	errStr := err.Error()
	if contains(errStr, "topic not found") {
		return ErrTopicNotFoundWithName(topicName)
	}
	if contains(errStr, "topic unhealthy") {
		return ErrTopicUnhealthyWithReason(topicName, errStr)
	}
	return WrapInternalError(err)
}

// contains checks if substr is in s (case-insensitive).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
