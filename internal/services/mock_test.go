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

// =============================================================================
// Shared mock AppState for all service tests
// =============================================================================

// mockAppState implements AppState for testing across all service test files.
type mockAppState struct {
	orchestratorDB *sql.DB
	topicDBs       map[string]*sql.DB
	topics         map[string]struct{ healthy bool; errMsg string }
	workingDir     string
	queriesConfig  *queries.QueriesConfig
	promptsManager *prompts.Manager
	cfg            *config.Config
	log            *logger.Logger
	auditLogger    *audit.Logger
	startedAt      time.Time

	// Concurrency control
	topicWriteMu   map[string]*sync.Mutex
	topicWriteMuMu sync.Mutex
	topicCreateMu  sync.Mutex
}

func newMockAppState() *mockAppState {
	// Create a config with all defaults applied
	cfg := &config.Config{}
	cfg.ApplyDefaults()

	return &mockAppState{
		topicDBs:     make(map[string]*sql.DB),
		topics:       make(map[string]struct{ healthy bool; errMsg string }),
		topicWriteMu: make(map[string]*sync.Mutex),
		startedAt:    time.Now(),
		cfg:          cfg,
	}
}

func (m *mockAppState) GetOrchestratorDB() *sql.DB                   { return m.orchestratorDB }
func (m *mockAppState) GetTopicDB(topicName string) (*sql.DB, error) { return m.topicDBs[topicName], nil }
func (m *mockAppState) GetTopicDBsForQuery(topicNames []string) (map[string]*sql.DB, []string, error) {
	if len(topicNames) == 0 {
		return m.topicDBs, m.ListTopics(), nil
	}
	result := make(map[string]*sql.DB)
	var names []string
	for _, name := range topicNames {
		if db, ok := m.topicDBs[name]; ok {
			result[name] = db
			names = append(names, name)
		}
	}
	return result, names, nil
}
func (m *mockAppState) StoreTopicDB(name string, db *sql.DB) { m.topicDBs[name] = db }
func (m *mockAppState) RegisterTopic(name string, healthy bool, errMsg string) {
	m.topics[name] = struct{ healthy bool; errMsg string }{healthy, errMsg}
}
func (m *mockAppState) UnregisterTopic(name string)      { delete(m.topics, name) }
func (m *mockAppState) TopicExists(topicName string) bool { _, ok := m.topics[topicName]; return ok }
func (m *mockAppState) IsTopicHealthy(topicName string) (bool, string) {
	if info, ok := m.topics[topicName]; ok {
		return info.healthy, info.errMsg
	}
	return false, "topic not found"
}
func (m *mockAppState) ListTopics() []string {
	var names []string
	for name := range m.topics {
		names = append(names, name)
	}
	return names
}
func (m *mockAppState) ClearTopicRegistry() {
	m.topics = make(map[string]struct{ healthy bool; errMsg string })
}
func (m *mockAppState) CloseAllTopicDBs()                            {}
func (m *mockAppState) GetTopicPath(topicName string) string         { return m.workingDir + "/" + topicName }
func (m *mockAppState) GetWorkingDirectory() string                  { return m.workingDir }
func (m *mockAppState) GetConfig() *config.Config                    { return m.cfg }
func (m *mockAppState) GetLogger() *logger.Logger                    { return m.log }
func (m *mockAppState) GetQueriesConfig() *queries.QueriesConfig     { return m.queriesConfig }
func (m *mockAppState) SetQueriesConfig(qc *queries.QueriesConfig)   { m.queriesConfig = qc }
func (m *mockAppState) GetPromptsManager() *prompts.Manager          { return m.promptsManager }
func (m *mockAppState) SetPromptsManager(pm *prompts.Manager)        { m.promptsManager = pm }
func (m *mockAppState) GetAuditLogger() *audit.Logger                { return m.auditLogger }
func (m *mockAppState) SetOrchestratorDB(db *sql.DB) { m.orchestratorDB = db }
func (m *mockAppState) GetStartedAt() time.Time     { return m.startedAt }
func (m *mockAppState) GetTopicWriteMu(topicName string) *sync.Mutex {
	m.topicWriteMuMu.Lock()
	defer m.topicWriteMuMu.Unlock()
	mu, exists := m.topicWriteMu[topicName]
	if !exists {
		mu = &sync.Mutex{}
		m.topicWriteMu[topicName] = mu
	}
	return mu
}
func (m *mockAppState) GetTopicCreateMu() *sync.Mutex { return &m.topicCreateMu }
