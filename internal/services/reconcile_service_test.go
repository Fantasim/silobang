package services

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"meshbank/internal/audit"
	"meshbank/internal/constants"
	"meshbank/internal/database"
	"meshbank/internal/logger"

	_ "github.com/mattn/go-sqlite3"
)

// setupReconcileTestDB creates an in-memory orchestrator DB with the full schema
// and returns it along with a cleanup function.
func setupReconcileTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	// Apply the orchestrator schema (includes asset_index + audit_log)
	schema := database.GetOrchestratorSchema()
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		t.Fatalf("failed to apply schema: %v", err)
	}

	return db
}

func TestNewReconcileService(t *testing.T) {
	mockApp := newMockAppState()
	log := logger.NewLogger("debug")

	svc := NewReconcileService(mockApp, log)

	if svc == nil {
		t.Fatal("NewReconcileService returned nil")
	}
	if svc.app != mockApp {
		t.Error("app field not set correctly")
	}
	if svc.logger != log {
		t.Error("logger field not set correctly")
	}
}

func TestReconcile_NoOrchestratorDB(t *testing.T) {
	mockApp := newMockAppState()
	mockApp.orchestratorDB = nil
	log := logger.NewLogger("debug")

	svc := NewReconcileService(mockApp, log)
	result, err := svc.Reconcile()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TopicsRemoved != 0 {
		t.Errorf("expected 0 topics removed, got %d", result.TopicsRemoved)
	}
}

func TestReconcile_EmptyIndex(t *testing.T) {
	db := setupReconcileTestDB(t)
	defer db.Close()

	mockApp := newMockAppState()
	mockApp.orchestratorDB = db
	log := logger.NewLogger("debug")

	svc := NewReconcileService(mockApp, log)
	result, err := svc.Reconcile()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TopicsRemoved != 0 {
		t.Errorf("expected 0 topics removed, got %d", result.TopicsRemoved)
	}
	if result.EntriesPurged != 0 {
		t.Errorf("expected 0 entries purged, got %d", result.EntriesPurged)
	}
}

func TestReconcile_AllTopicsRegistered(t *testing.T) {
	db := setupReconcileTestDB(t)
	defer db.Close()

	// Create a temp working directory with both topic folders present
	workDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workDir, "topic-a"), constants.DirPermissions); err != nil {
		t.Fatalf("failed to create topic-a dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(workDir, "topic-b"), constants.DirPermissions); err != nil {
		t.Fatalf("failed to create topic-b dir: %v", err)
	}

	// Insert entries for two topics
	_, err := db.Exec(`INSERT INTO asset_index (hash, topic, dat_file) VALUES
		('hash1', 'topic-a', '001.dat'),
		('hash2', 'topic-a', '001.dat'),
		('hash3', 'topic-b', '001.dat')`)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	mockApp := newMockAppState()
	mockApp.orchestratorDB = db
	mockApp.workingDir = workDir
	mockApp.RegisterTopic("topic-a", true, "")
	mockApp.RegisterTopic("topic-b", true, "")

	log := logger.NewLogger("debug")
	svc := NewReconcileService(mockApp, log)
	result, err := svc.Reconcile()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TopicsRemoved != 0 {
		t.Errorf("expected 0 topics removed, got %d", result.TopicsRemoved)
	}
	if result.EntriesPurged != 0 {
		t.Errorf("expected 0 entries purged, got %d", result.EntriesPurged)
	}
}

func TestReconcile_OrphanedTopicPurged(t *testing.T) {
	db := setupReconcileTestDB(t)
	defer db.Close()

	// Create a temp working directory — topic-a exists, topic-b does NOT
	workDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workDir, "topic-a"), constants.DirPermissions); err != nil {
		t.Fatalf("failed to create topic-a dir: %v", err)
	}
	// topic-b folder does NOT exist on disk

	// Insert entries for both topics in the index
	_, err := db.Exec(`INSERT INTO asset_index (hash, topic, dat_file) VALUES
		('hash1', 'topic-a', '001.dat'),
		('hash2', 'topic-a', '001.dat'),
		('hash3', 'topic-b', '001.dat'),
		('hash4', 'topic-b', '002.dat'),
		('hash5', 'topic-b', '002.dat')`)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	mockApp := newMockAppState()
	mockApp.orchestratorDB = db
	mockApp.workingDir = workDir
	// Only topic-a is registered (topic-b was removed from disk)
	mockApp.RegisterTopic("topic-a", true, "")

	// Set up audit logger so we can verify audit entries
	auditLogger := audit.NewLogger(db)
	defer auditLogger.Stop()
	mockApp.auditLogger = auditLogger

	log := logger.NewLogger("debug")
	svc := NewReconcileService(mockApp, log)
	result, err := svc.Reconcile()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify topic-b was purged
	if result.TopicsRemoved != 1 {
		t.Errorf("expected 1 topic removed, got %d", result.TopicsRemoved)
	}
	if result.EntriesPurged != 3 {
		t.Errorf("expected 3 entries purged, got %d", result.EntriesPurged)
	}
	if len(result.RemovedTopics) != 1 || result.RemovedTopics[0] != "topic-b" {
		t.Errorf("expected removed topics = [topic-b], got %v", result.RemovedTopics)
	}

	// Verify topic-a entries are still in the index
	var countA int
	if err := db.QueryRow("SELECT COUNT(*) FROM asset_index WHERE topic = ?", "topic-a").Scan(&countA); err != nil {
		t.Fatalf("failed to count topic-a entries: %v", err)
	}
	if countA != 2 {
		t.Errorf("expected 2 topic-a entries remaining, got %d", countA)
	}

	// Verify topic-b entries are gone
	var countB int
	if err := db.QueryRow("SELECT COUNT(*) FROM asset_index WHERE topic = ?", "topic-b").Scan(&countB); err != nil {
		t.Fatalf("failed to count topic-b entries: %v", err)
	}
	if countB != 0 {
		t.Errorf("expected 0 topic-b entries, got %d", countB)
	}

	// Verify audit log entry was created
	var auditCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM audit_log WHERE action = ?",
		constants.AuditActionReconcileTopicRemoved).Scan(&auditCount); err != nil {
		t.Fatalf("failed to count audit entries: %v", err)
	}
	if auditCount != 1 {
		t.Errorf("expected 1 audit entry for reconcile_topic_removed, got %d", auditCount)
	}
}

func TestReconcile_FolderExistsButNotRegistered_NoDelete(t *testing.T) {
	db := setupReconcileTestDB(t)
	defer db.Close()

	// Create a temp working directory — topic-c folder EXISTS on disk
	workDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workDir, "topic-c"), constants.DirPermissions); err != nil {
		t.Fatalf("failed to create topic-c dir: %v", err)
	}

	// topic-c is in the index but NOT registered (e.g., unhealthy)
	_, err := db.Exec(`INSERT INTO asset_index (hash, topic, dat_file) VALUES
		('hash1', 'topic-c', '001.dat')`)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	mockApp := newMockAppState()
	mockApp.orchestratorDB = db
	mockApp.workingDir = workDir
	// topic-c is NOT registered but its folder exists on disk

	log := logger.NewLogger("debug")
	svc := NewReconcileService(mockApp, log)
	result, err := svc.Reconcile()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should NOT purge because the folder still exists
	if result.TopicsRemoved != 0 {
		t.Errorf("expected 0 topics removed (folder exists on disk), got %d", result.TopicsRemoved)
	}

	// Verify entry is still in the index
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM asset_index WHERE topic = ?", "topic-c").Scan(&count); err != nil {
		t.Fatalf("failed to count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 entry remaining, got %d", count)
	}
}

func TestReconcile_MultipleOrphanedTopics(t *testing.T) {
	db := setupReconcileTestDB(t)
	defer db.Close()

	workDir := t.TempDir()
	// No topic folders created — all are orphaned

	_, err := db.Exec(`INSERT INTO asset_index (hash, topic, dat_file) VALUES
		('hash1', 'gone-topic-1', '001.dat'),
		('hash2', 'gone-topic-1', '002.dat'),
		('hash3', 'gone-topic-2', '001.dat'),
		('hash4', 'gone-topic-3', '001.dat'),
		('hash5', 'gone-topic-3', '001.dat'),
		('hash6', 'gone-topic-3', '002.dat')`)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	mockApp := newMockAppState()
	mockApp.orchestratorDB = db
	mockApp.workingDir = workDir

	auditLogger := audit.NewLogger(db)
	defer auditLogger.Stop()
	mockApp.auditLogger = auditLogger

	log := logger.NewLogger("debug")
	svc := NewReconcileService(mockApp, log)
	result, err := svc.Reconcile()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TopicsRemoved != 3 {
		t.Errorf("expected 3 topics removed, got %d", result.TopicsRemoved)
	}
	if result.EntriesPurged != 6 {
		t.Errorf("expected 6 entries purged, got %d", result.EntriesPurged)
	}

	// Verify asset_index is empty
	var total int
	if err := db.QueryRow("SELECT COUNT(*) FROM asset_index").Scan(&total); err != nil {
		t.Fatalf("failed to count: %v", err)
	}
	if total != 0 {
		t.Errorf("expected 0 entries in asset_index, got %d", total)
	}

	// Verify 3 audit entries
	var auditCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM audit_log WHERE action = ?",
		constants.AuditActionReconcileTopicRemoved).Scan(&auditCount); err != nil {
		t.Fatalf("failed to count audit entries: %v", err)
	}
	if auditCount != 3 {
		t.Errorf("expected 3 audit entries, got %d", auditCount)
	}
}

func TestReconcile_UnregistersClearedTopic(t *testing.T) {
	db := setupReconcileTestDB(t)
	defer db.Close()

	workDir := t.TempDir()
	// No topic-x folder on disk

	_, err := db.Exec(`INSERT INTO asset_index (hash, topic, dat_file) VALUES
		('hash1', 'topic-x', '001.dat')`)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	mockApp := newMockAppState()
	mockApp.orchestratorDB = db
	mockApp.workingDir = workDir
	// Simulate a topic that was registered but its folder was removed
	mockApp.RegisterTopic("topic-x", false, "folder missing")

	log := logger.NewLogger("debug")
	svc := NewReconcileService(mockApp, log)
	_, err = svc.Reconcile()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify topic-x is unregistered from in-memory state
	if mockApp.TopicExists("topic-x") {
		t.Error("expected topic-x to be unregistered after reconciliation")
	}
}

func TestReconcile_StartStop(t *testing.T) {
	mockApp := newMockAppState()
	log := logger.NewLogger("debug")

	svc := NewReconcileService(mockApp, log)

	// Start should not panic
	svc.Start(1 * 60 * 1e9) // 1 minute as time.Duration

	// Double start should be a no-op
	svc.Start(1 * 60 * 1e9)

	// Stop should not panic
	svc.Stop()

	// Double stop should not panic
	svc.Stop()
}
