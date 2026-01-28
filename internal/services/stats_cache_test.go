package services

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"silobang/internal/config"
	"silobang/internal/constants"
	"silobang/internal/database"
	"silobang/internal/logger"
)

// =============================================================================
// Test Helpers
// =============================================================================

// newStatsCacheMock creates a mockAppState configured for stats cache tests.
// The workDir parameter sets the base working directory; topics are placed as
// subdirectories of workDir (mock GetTopicPath returns workDir + "/" + topicName).
func newStatsCacheMock(workDir string) *mockAppState {
	m := newMockAppState()
	m.workingDir = workDir
	cfg := &config.Config{
		WorkingDirectory: workDir,
		Port:             constants.DefaultPort,
		MaxDatSize:       constants.DefaultMaxDatSize,
	}
	cfg.ApplyDefaults()
	m.cfg = cfg
	m.log = logger.NewLogger(logger.LevelError)
	return m
}

// setupTopicDir creates the topic directory structure with .internal/ and
// a topic database containing the provided assets. Returns the topic DB.
func setupTopicDir(t *testing.T, workDir, topicName string, assets []testAsset) *sql.DB {
	t.Helper()

	topicPath := filepath.Join(workDir, topicName)
	internalPath := filepath.Join(topicPath, constants.InternalDir)
	if err := os.MkdirAll(internalPath, 0755); err != nil {
		t.Fatalf("failed to create internal dir for topic %s: %v", topicName, err)
	}

	dbPath := filepath.Join(internalPath, topicName+".db")
	db, err := database.InitTopicDB(dbPath)
	if err != nil {
		t.Fatalf("failed to init topic db for %s: %v", topicName, err)
	}
	t.Cleanup(func() { db.Close() })

	for _, a := range assets {
		_, err := db.Exec(
			`INSERT INTO assets (asset_id, asset_size, extension, blob_name, byte_offset, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
			a.id, a.size, a.ext, a.blobName, a.offset, a.createdAt,
		)
		if err != nil {
			t.Fatalf("failed to insert asset %s: %v", a.id, err)
		}
	}

	return db
}

// createDatFile creates a .dat file in the topic directory with the given size.
func createDatFile(t *testing.T, workDir, topicName, datName string, sizeBytes int) {
	t.Helper()
	topicPath := filepath.Join(workDir, topicName)
	filePath := filepath.Join(topicPath, datName)
	if err := os.WriteFile(filePath, make([]byte, sizeBytes), 0644); err != nil {
		t.Fatalf("failed to create dat file %s: %v", datName, err)
	}
}

// setupOrchestratorDB creates an orchestrator database with the asset_index table
// and the provided entries. Returns the DB handle.
func setupOrchestratorDB(t *testing.T, workDir string, entries []orchestratorEntry) *sql.DB {
	t.Helper()

	internalPath := filepath.Join(workDir, constants.InternalDir)
	if err := os.MkdirAll(internalPath, 0755); err != nil {
		t.Fatalf("failed to create working dir internal: %v", err)
	}

	orchPath := filepath.Join(internalPath, constants.OrchestratorDB)
	db, err := database.InitOrchestratorDB(orchPath)
	if err != nil {
		t.Fatalf("failed to init orchestrator db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	for _, e := range entries {
		_, err := db.Exec(
			`INSERT INTO asset_index (hash, topic, dat_file) VALUES (?, ?, ?)`,
			e.hash, e.topic, e.datFile,
		)
		if err != nil {
			t.Fatalf("failed to insert orchestrator entry: %v", err)
		}
	}

	return db
}

// newTestStatsCache creates a StatsCache with its dependencies wired up for testing.
func newTestStatsCache(mock *mockAppState) *StatsCache {
	configSvc := NewConfigService(mock, mock.log)
	return NewStatsCache(mock, mock.log, configSvc)
}

// testAsset holds data for a single asset row inserted into a topic database.
type testAsset struct {
	id        string
	size      int64
	ext       string
	blobName  string
	offset    int64
	createdAt int64
}

// orchestratorEntry holds data for a single row in the orchestrator asset_index table.
type orchestratorEntry struct {
	hash    string
	topic   string
	datFile string
}

// =============================================================================
// Constructor Tests
// =============================================================================

func TestNewStatsCache(t *testing.T) {
	mock := newStatsCacheMock(t.TempDir())
	cache := newTestStatsCache(mock)

	if cache == nil {
		t.Fatal("NewStatsCache returned nil")
	}
	if cache.IsInitialized() {
		t.Error("cache should not be initialized before BuildAll")
	}
	if info := cache.GetServiceInfo(); info != nil {
		t.Error("service info should be nil before BuildAll")
	}
}

// =============================================================================
// BuildAll Tests
// =============================================================================

func TestStatsCacheBuildAll(t *testing.T) {
	workDir := t.TempDir()
	mock := newStatsCacheMock(workDir)

	// Set up two healthy topics with assets and DAT files
	db1 := setupTopicDir(t, workDir, "topic-a", []testAsset{
		{id: "aaa111", size: 1000, ext: "png", blobName: "000001.dat", offset: 0, createdAt: 1700000000},
		{id: "aaa222", size: 2000, ext: "jpg", blobName: "000001.dat", offset: 1000, createdAt: 1700000001},
	})
	createDatFile(t, workDir, "topic-a", "000001.dat", 3000)

	db2 := setupTopicDir(t, workDir, "topic-b", []testAsset{
		{id: "bbb111", size: 5000, ext: "fbx", blobName: "000001.dat", offset: 0, createdAt: 1700000002},
	})
	createDatFile(t, workDir, "topic-b", "000001.dat", 5000)

	mock.StoreTopicDB("topic-a", db1)
	mock.StoreTopicDB("topic-b", db2)
	mock.RegisterTopic("topic-a", true, "")
	mock.RegisterTopic("topic-b", true, "")

	// Set up orchestrator DB with indexed hashes
	orchDB := setupOrchestratorDB(t, workDir, []orchestratorEntry{
		{hash: "aaa111", topic: "topic-a", datFile: "000001.dat"},
		{hash: "aaa222", topic: "topic-a", datFile: "000001.dat"},
		{hash: "bbb111", topic: "topic-b", datFile: "000001.dat"},
	})
	mock.SetOrchestratorDB(orchDB)

	cache := newTestStatsCache(mock)
	cache.BuildAll()

	// Verify initialization
	if !cache.IsInitialized() {
		t.Fatal("cache should be initialized after BuildAll")
	}

	// Verify topic-a stats
	statsA, ok := cache.GetTopicStats("topic-a")
	if !ok {
		t.Fatal("expected topic-a to be cached")
	}
	if statsA["file_count"] != int64(2) {
		t.Errorf("topic-a file_count: got %v, want 2", statsA["file_count"])
	}
	if statsA["total_size"] != int64(3000) {
		t.Errorf("topic-a total_size: got %v, want 3000", statsA["total_size"])
	}

	// Verify topic-b stats
	statsB, ok := cache.GetTopicStats("topic-b")
	if !ok {
		t.Fatal("expected topic-b to be cached")
	}
	if statsB["file_count"] != int64(1) {
		t.Errorf("topic-b file_count: got %v, want 1", statsB["file_count"])
	}

	// Verify service info
	info := cache.GetServiceInfo()
	if info == nil {
		t.Fatal("expected service info to be non-nil after BuildAll")
	}
	if info.TotalIndexedHashes != 3 {
		t.Errorf("total indexed hashes: got %d, want 3", info.TotalIndexedHashes)
	}
	if info.TopicsSummary.Total != 2 {
		t.Errorf("topics total: got %d, want 2", info.TopicsSummary.Total)
	}
	if info.TopicsSummary.Healthy != 2 {
		t.Errorf("topics healthy: got %d, want 2", info.TopicsSummary.Healthy)
	}
}

func TestStatsCacheBuildAll_SkipsUnhealthy(t *testing.T) {
	workDir := t.TempDir()
	mock := newStatsCacheMock(workDir)

	// One healthy topic, one unhealthy
	db1 := setupTopicDir(t, workDir, "healthy-topic", []testAsset{
		{id: "aaa111", size: 1000, ext: "png", blobName: "000001.dat", offset: 0, createdAt: 1700000000},
	})
	createDatFile(t, workDir, "healthy-topic", "000001.dat", 1000)
	mock.StoreTopicDB("healthy-topic", db1)
	mock.RegisterTopic("healthy-topic", true, "")
	mock.RegisterTopic("broken-topic", false, "corrupted database")

	cache := newTestStatsCache(mock)
	cache.BuildAll()

	// Healthy topic should be cached
	_, ok := cache.GetTopicStats("healthy-topic")
	if !ok {
		t.Error("expected healthy-topic to be cached")
	}

	// Unhealthy topic should NOT be cached
	_, ok = cache.GetTopicStats("broken-topic")
	if ok {
		t.Error("expected broken-topic NOT to be cached")
	}

	// Service info should reflect correct health counts
	info := cache.GetServiceInfo()
	if info.TopicsSummary.Total != 2 {
		t.Errorf("topics total: got %d, want 2", info.TopicsSummary.Total)
	}
	if info.TopicsSummary.Healthy != 1 {
		t.Errorf("topics healthy: got %d, want 1", info.TopicsSummary.Healthy)
	}
	if info.TopicsSummary.Unhealthy != 1 {
		t.Errorf("topics unhealthy: got %d, want 1", info.TopicsSummary.Unhealthy)
	}
}

func TestStatsCacheBuildAll_EmptyState(t *testing.T) {
	workDir := t.TempDir()
	mock := newStatsCacheMock(workDir)

	cache := newTestStatsCache(mock)
	cache.BuildAll()

	if !cache.IsInitialized() {
		t.Error("cache should be initialized even with zero topics")
	}

	allStats := cache.GetAllTopicStats()
	if len(allStats) != 0 {
		t.Errorf("expected 0 cached topics, got %d", len(allStats))
	}

	info := cache.GetServiceInfo()
	if info == nil {
		t.Fatal("service info should be non-nil even with zero topics")
	}
	if info.TopicsSummary.Total != 0 {
		t.Errorf("topics total: got %d, want 0", info.TopicsSummary.Total)
	}
	if info.TotalIndexedHashes != 0 {
		t.Errorf("total indexed hashes: got %d, want 0", info.TotalIndexedHashes)
	}
}

func TestStatsCacheBuildAll_RebuildsCleanly(t *testing.T) {
	workDir := t.TempDir()
	mock := newStatsCacheMock(workDir)

	db1 := setupTopicDir(t, workDir, "topic-x", []testAsset{
		{id: "xxx111", size: 500, ext: "bin", blobName: "000001.dat", offset: 0, createdAt: 1700000000},
	})
	createDatFile(t, workDir, "topic-x", "000001.dat", 500)
	mock.StoreTopicDB("topic-x", db1)
	mock.RegisterTopic("topic-x", true, "")

	cache := newTestStatsCache(mock)
	cache.BuildAll()

	statsFirst, _ := cache.GetTopicStats("topic-x")
	if statsFirst["file_count"] != int64(1) {
		t.Fatalf("first build: expected 1 file, got %v", statsFirst["file_count"])
	}

	// Insert more data and rebuild
	_, err := db1.Exec(
		`INSERT INTO assets (asset_id, asset_size, extension, blob_name, byte_offset, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"xxx222", 700, "bin", "000001.dat", 500, 1700000001,
	)
	if err != nil {
		t.Fatalf("failed to insert second asset: %v", err)
	}

	cache.BuildAll()

	statsSecond, _ := cache.GetTopicStats("topic-x")
	if statsSecond["file_count"] != int64(2) {
		t.Errorf("second build: expected 2 files, got %v", statsSecond["file_count"])
	}
}

// =============================================================================
// InvalidateTopic Tests
// =============================================================================

func TestStatsCacheInvalidateTopic(t *testing.T) {
	workDir := t.TempDir()
	mock := newStatsCacheMock(workDir)

	db1 := setupTopicDir(t, workDir, "topic-a", []testAsset{
		{id: "aaa111", size: 1000, ext: "png", blobName: "000001.dat", offset: 0, createdAt: 1700000000},
	})
	createDatFile(t, workDir, "topic-a", "000001.dat", 1000)
	mock.StoreTopicDB("topic-a", db1)
	mock.RegisterTopic("topic-a", true, "")

	cache := newTestStatsCache(mock)
	cache.BuildAll()

	// Verify initial state
	stats, _ := cache.GetTopicStats("topic-a")
	if stats["file_count"] != int64(1) {
		t.Fatalf("initial file_count: got %v, want 1", stats["file_count"])
	}

	// Insert more data
	_, err := db1.Exec(
		`INSERT INTO assets (asset_id, asset_size, extension, blob_name, byte_offset, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"aaa222", 2000, "jpg", "000001.dat", 1000, 1700000001,
	)
	if err != nil {
		t.Fatalf("failed to insert second asset: %v", err)
	}

	// Invalidate and verify refresh
	cache.InvalidateTopic("topic-a")

	stats, _ = cache.GetTopicStats("topic-a")
	if stats["file_count"] != int64(2) {
		t.Errorf("after invalidation file_count: got %v, want 2", stats["file_count"])
	}
	if stats["total_size"] != int64(3000) {
		t.Errorf("after invalidation total_size: got %v, want 3000", stats["total_size"])
	}
}

func TestStatsCacheInvalidateTopic_UnhealthyRemoved(t *testing.T) {
	workDir := t.TempDir()
	mock := newStatsCacheMock(workDir)

	db1 := setupTopicDir(t, workDir, "topic-a", []testAsset{
		{id: "aaa111", size: 1000, ext: "png", blobName: "000001.dat", offset: 0, createdAt: 1700000000},
	})
	createDatFile(t, workDir, "topic-a", "000001.dat", 1000)
	mock.StoreTopicDB("topic-a", db1)
	mock.RegisterTopic("topic-a", true, "")

	cache := newTestStatsCache(mock)
	cache.BuildAll()

	// Verify it's cached
	_, ok := cache.GetTopicStats("topic-a")
	if !ok {
		t.Fatal("topic-a should be cached initially")
	}

	// Mark topic as unhealthy and invalidate
	mock.RegisterTopic("topic-a", false, "went bad")
	cache.InvalidateTopic("topic-a")

	// Should be removed from cache
	_, ok = cache.GetTopicStats("topic-a")
	if ok {
		t.Error("topic-a should be removed from cache after being marked unhealthy")
	}
}

func TestStatsCacheInvalidateTopic_NewTopic(t *testing.T) {
	workDir := t.TempDir()
	mock := newStatsCacheMock(workDir)

	cache := newTestStatsCache(mock)
	cache.BuildAll()

	// Add a new topic after initial build
	db1 := setupTopicDir(t, workDir, "new-topic", []testAsset{
		{id: "nnn111", size: 4000, ext: "fbx", blobName: "000001.dat", offset: 0, createdAt: 1700000000},
	})
	createDatFile(t, workDir, "new-topic", "000001.dat", 4000)
	mock.StoreTopicDB("new-topic", db1)
	mock.RegisterTopic("new-topic", true, "")

	// Invalidate the new topic
	cache.InvalidateTopic("new-topic")

	stats, ok := cache.GetTopicStats("new-topic")
	if !ok {
		t.Fatal("new-topic should be cached after invalidation")
	}
	if stats["file_count"] != int64(1) {
		t.Errorf("new-topic file_count: got %v, want 1", stats["file_count"])
	}
}

// =============================================================================
// InvalidateTopics (batch) Tests
// =============================================================================

func TestStatsCacheInvalidateTopics(t *testing.T) {
	workDir := t.TempDir()
	mock := newStatsCacheMock(workDir)

	db1 := setupTopicDir(t, workDir, "topic-a", []testAsset{
		{id: "aaa111", size: 1000, ext: "png", blobName: "000001.dat", offset: 0, createdAt: 1700000000},
	})
	createDatFile(t, workDir, "topic-a", "000001.dat", 1000)

	db2 := setupTopicDir(t, workDir, "topic-b", []testAsset{
		{id: "bbb111", size: 2000, ext: "jpg", blobName: "000001.dat", offset: 0, createdAt: 1700000000},
	})
	createDatFile(t, workDir, "topic-b", "000001.dat", 2000)

	mock.StoreTopicDB("topic-a", db1)
	mock.StoreTopicDB("topic-b", db2)
	mock.RegisterTopic("topic-a", true, "")
	mock.RegisterTopic("topic-b", true, "")

	cache := newTestStatsCache(mock)
	cache.BuildAll()

	// Insert into both topics
	_, _ = db1.Exec(
		`INSERT INTO assets (asset_id, asset_size, extension, blob_name, byte_offset, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"aaa222", 500, "png", "000001.dat", 1000, 1700000001,
	)
	_, _ = db2.Exec(
		`INSERT INTO assets (asset_id, asset_size, extension, blob_name, byte_offset, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"bbb222", 800, "jpg", "000001.dat", 2000, 1700000001,
	)

	// Batch invalidate
	cache.InvalidateTopics([]string{"topic-a", "topic-b"})

	statsA, _ := cache.GetTopicStats("topic-a")
	if statsA["file_count"] != int64(2) {
		t.Errorf("topic-a file_count after batch invalidation: got %v, want 2", statsA["file_count"])
	}

	statsB, _ := cache.GetTopicStats("topic-b")
	if statsB["file_count"] != int64(2) {
		t.Errorf("topic-b file_count after batch invalidation: got %v, want 2", statsB["file_count"])
	}
}

// =============================================================================
// RemoveTopic Tests
// =============================================================================

func TestStatsCacheRemoveTopic(t *testing.T) {
	workDir := t.TempDir()
	mock := newStatsCacheMock(workDir)

	db1 := setupTopicDir(t, workDir, "topic-a", []testAsset{
		{id: "aaa111", size: 1000, ext: "png", blobName: "000001.dat", offset: 0, createdAt: 1700000000},
	})
	createDatFile(t, workDir, "topic-a", "000001.dat", 1000)

	db2 := setupTopicDir(t, workDir, "topic-b", []testAsset{
		{id: "bbb111", size: 2000, ext: "jpg", blobName: "000001.dat", offset: 0, createdAt: 1700000000},
	})
	createDatFile(t, workDir, "topic-b", "000001.dat", 2000)

	mock.StoreTopicDB("topic-a", db1)
	mock.StoreTopicDB("topic-b", db2)
	mock.RegisterTopic("topic-a", true, "")
	mock.RegisterTopic("topic-b", true, "")

	cache := newTestStatsCache(mock)
	cache.BuildAll()

	// Remove topic-a
	cache.RemoveTopic("topic-a")

	// topic-a should be gone
	_, ok := cache.GetTopicStats("topic-a")
	if ok {
		t.Error("topic-a should not be in cache after removal")
	}

	// topic-b should still be there
	_, ok = cache.GetTopicStats("topic-b")
	if !ok {
		t.Error("topic-b should still be in cache")
	}

	// GetAllTopicStats should only have topic-b
	all := cache.GetAllTopicStats()
	if len(all) != 1 {
		t.Errorf("expected 1 cached topic after removal, got %d", len(all))
	}
	if _, exists := all["topic-b"]; !exists {
		t.Error("topic-b should be in GetAllTopicStats result")
	}
}

func TestStatsCacheRemoveTopic_ServiceInfoRecomputed(t *testing.T) {
	workDir := t.TempDir()
	mock := newStatsCacheMock(workDir)

	db1 := setupTopicDir(t, workDir, "topic-a", []testAsset{
		{id: "aaa111", size: 1000, ext: "png", blobName: "000001.dat", offset: 0, createdAt: 1700000000},
	})
	createDatFile(t, workDir, "topic-a", "000001.dat", 1000)

	db2 := setupTopicDir(t, workDir, "topic-b", []testAsset{
		{id: "bbb111", size: 3000, ext: "jpg", blobName: "000001.dat", offset: 0, createdAt: 1700000000},
	})
	createDatFile(t, workDir, "topic-b", "000001.dat", 3000)

	mock.StoreTopicDB("topic-a", db1)
	mock.StoreTopicDB("topic-b", db2)
	mock.RegisterTopic("topic-a", true, "")
	mock.RegisterTopic("topic-b", true, "")

	orchDB := setupOrchestratorDB(t, workDir, []orchestratorEntry{
		{hash: "aaa111", topic: "topic-a", datFile: "000001.dat"},
		{hash: "bbb111", topic: "topic-b", datFile: "000001.dat"},
	})
	mock.SetOrchestratorDB(orchDB)

	cache := newTestStatsCache(mock)
	cache.BuildAll()

	infoBefore := cache.GetServiceInfo()
	datSizeBefore := infoBefore.StorageSummary.TotalDatSize

	// Remove topic-a (had 1000 bytes DAT)
	cache.RemoveTopic("topic-a")

	infoAfter := cache.GetServiceInfo()
	datSizeAfter := infoAfter.StorageSummary.TotalDatSize

	// DAT size should decrease after removing a topic
	if datSizeAfter >= datSizeBefore {
		t.Errorf("expected total dat size to decrease after removal: before=%d, after=%d",
			datSizeBefore, datSizeAfter)
	}
}

func TestStatsCacheRemoveTopic_NonExistent(t *testing.T) {
	workDir := t.TempDir()
	mock := newStatsCacheMock(workDir)

	cache := newTestStatsCache(mock)
	cache.BuildAll()

	// Should not panic on removing a non-existent topic
	cache.RemoveTopic("does-not-exist")

	if !cache.IsInitialized() {
		t.Error("cache should remain initialized after removing non-existent topic")
	}
}

// =============================================================================
// GetTopicStats Tests
// =============================================================================

func TestStatsCacheGetTopicStats_Miss(t *testing.T) {
	workDir := t.TempDir()
	mock := newStatsCacheMock(workDir)

	cache := newTestStatsCache(mock)
	cache.BuildAll()

	stats, ok := cache.GetTopicStats("nonexistent")
	if ok {
		t.Error("expected cache miss for nonexistent topic")
	}
	if stats != nil {
		t.Error("expected nil stats for cache miss")
	}
}

func TestStatsCacheGetTopicStats_BeforeBuild(t *testing.T) {
	workDir := t.TempDir()
	mock := newStatsCacheMock(workDir)

	cache := newTestStatsCache(mock)

	// Not built yet
	stats, ok := cache.GetTopicStats("anything")
	if ok {
		t.Error("expected cache miss before BuildAll")
	}
	if stats != nil {
		t.Error("expected nil stats before BuildAll")
	}
}

// =============================================================================
// GetAllTopicStats Tests
// =============================================================================

func TestStatsCacheGetAllTopicStats(t *testing.T) {
	workDir := t.TempDir()
	mock := newStatsCacheMock(workDir)

	db1 := setupTopicDir(t, workDir, "alpha", []testAsset{
		{id: "aaa111", size: 100, ext: "bin", blobName: "000001.dat", offset: 0, createdAt: 1700000000},
	})
	createDatFile(t, workDir, "alpha", "000001.dat", 100)

	db2 := setupTopicDir(t, workDir, "beta", []testAsset{
		{id: "bbb111", size: 200, ext: "bin", blobName: "000001.dat", offset: 0, createdAt: 1700000000},
	})
	createDatFile(t, workDir, "beta", "000001.dat", 200)

	mock.StoreTopicDB("alpha", db1)
	mock.StoreTopicDB("beta", db2)
	mock.RegisterTopic("alpha", true, "")
	mock.RegisterTopic("beta", true, "")

	cache := newTestStatsCache(mock)
	cache.BuildAll()

	all := cache.GetAllTopicStats()
	if len(all) != 2 {
		t.Fatalf("expected 2 topics in GetAllTopicStats, got %d", len(all))
	}

	if _, ok := all["alpha"]; !ok {
		t.Error("expected alpha in results")
	}
	if _, ok := all["beta"]; !ok {
		t.Error("expected beta in results")
	}
}

// =============================================================================
// GetServiceInfo Tests
// =============================================================================

func TestStatsCacheGetServiceInfo_BeforeBuild(t *testing.T) {
	workDir := t.TempDir()
	mock := newStatsCacheMock(workDir)

	cache := newTestStatsCache(mock)
	info := cache.GetServiceInfo()

	if info != nil {
		t.Error("service info should be nil before BuildAll")
	}
}

func TestStatsCacheServiceInfoAggregation(t *testing.T) {
	workDir := t.TempDir()
	mock := newStatsCacheMock(workDir)

	db1 := setupTopicDir(t, workDir, "topic-a", []testAsset{
		{id: "aaa111", size: 1000, ext: "png", blobName: "000001.dat", offset: 0, createdAt: 1700000000},
		{id: "aaa222", size: 2000, ext: "jpg", blobName: "000001.dat", offset: 1000, createdAt: 1700000001},
	})
	createDatFile(t, workDir, "topic-a", "000001.dat", 3000)

	db2 := setupTopicDir(t, workDir, "topic-b", []testAsset{
		{id: "bbb111", size: 5000, ext: "fbx", blobName: "000001.dat", offset: 0, createdAt: 1700000002},
		{id: "bbb222", size: 3000, ext: "fbx", blobName: "000002.dat", offset: 0, createdAt: 1700000003},
	})
	createDatFile(t, workDir, "topic-b", "000001.dat", 5000)
	createDatFile(t, workDir, "topic-b", "000002.dat", 3000)

	mock.StoreTopicDB("topic-a", db1)
	mock.StoreTopicDB("topic-b", db2)
	mock.RegisterTopic("topic-a", true, "")
	mock.RegisterTopic("topic-b", true, "")

	orchDB := setupOrchestratorDB(t, workDir, []orchestratorEntry{
		{hash: "aaa111", topic: "topic-a", datFile: "000001.dat"},
		{hash: "aaa222", topic: "topic-a", datFile: "000001.dat"},
		{hash: "bbb111", topic: "topic-b", datFile: "000001.dat"},
		{hash: "bbb222", topic: "topic-b", datFile: "000002.dat"},
	})
	mock.SetOrchestratorDB(orchDB)

	cache := newTestStatsCache(mock)
	cache.BuildAll()

	info := cache.GetServiceInfo()
	if info == nil {
		t.Fatal("expected non-nil service info")
	}

	// Total indexed hashes from orchestrator
	if info.TotalIndexedHashes != 4 {
		t.Errorf("total indexed hashes: got %d, want 4", info.TotalIndexedHashes)
	}

	// Storage aggregation: total DAT size should be 3000 + 5000 + 3000 = 11000
	if info.StorageSummary.TotalDatSize != 11000 {
		t.Errorf("total dat size: got %d, want 11000", info.StorageSummary.TotalDatSize)
	}

	// Total asset size should be 1000+2000+5000+3000 = 11000
	if info.StorageSummary.TotalAssetSize != 11000 {
		t.Errorf("total asset size: got %d, want 11000", info.StorageSummary.TotalAssetSize)
	}

	// DAT file count: topic-a has 1, topic-b has 2
	if info.StorageSummary.TotalDatFiles != 3 {
		t.Errorf("total dat files: got %d, want 3", info.StorageSummary.TotalDatFiles)
	}

	// Average asset size: 11000 / 4 = 2750
	expectedAvg := float64(11000) / float64(4)
	if info.StorageSummary.AvgAssetSize != expectedAvg {
		t.Errorf("avg asset size: got %f, want %f", info.StorageSummary.AvgAssetSize, expectedAvg)
	}

	// Topic health counts
	if info.TopicsSummary.Total != 2 {
		t.Errorf("topics total: got %d, want 2", info.TopicsSummary.Total)
	}
	if info.TopicsSummary.Healthy != 2 {
		t.Errorf("topics healthy: got %d, want 2", info.TopicsSummary.Healthy)
	}
	if info.TopicsSummary.Unhealthy != 0 {
		t.Errorf("topics unhealthy: got %d, want 0", info.TopicsSummary.Unhealthy)
	}
}

func TestStatsCacheServiceInfoMaxDiskUsage(t *testing.T) {
	workDir := t.TempDir()
	mock := newStatsCacheMock(workDir)
	mock.cfg.MaxDiskUsage = 10_000_000_000 // 10 GB

	cache := newTestStatsCache(mock)
	cache.BuildAll()

	info := cache.GetServiceInfo()
	if info.MaxDiskUsageBytes != 10_000_000_000 {
		t.Errorf("max disk usage: got %d, want 10000000000", info.MaxDiskUsageBytes)
	}
}

func TestStatsCacheServiceInfoOrchestratorDBSize(t *testing.T) {
	workDir := t.TempDir()
	mock := newStatsCacheMock(workDir)

	// Create orchestrator DB on disk so file size is > 0
	orchDB := setupOrchestratorDB(t, workDir, nil)
	mock.SetOrchestratorDB(orchDB)

	cache := newTestStatsCache(mock)
	cache.BuildAll()

	info := cache.GetServiceInfo()
	if info.OrchestratorDBSize <= 0 {
		t.Errorf("expected orchestrator db size > 0, got %d", info.OrchestratorDBSize)
	}
}

// =============================================================================
// IsInitialized Tests
// =============================================================================

func TestStatsCacheIsInitialized(t *testing.T) {
	workDir := t.TempDir()
	mock := newStatsCacheMock(workDir)

	cache := newTestStatsCache(mock)

	if cache.IsInitialized() {
		t.Error("should not be initialized before BuildAll")
	}

	cache.BuildAll()

	if !cache.IsInitialized() {
		t.Error("should be initialized after BuildAll")
	}
}

// =============================================================================
// Concurrent Access Tests
// =============================================================================

func TestStatsCacheConcurrentAccess(t *testing.T) {
	workDir := t.TempDir()
	mock := newStatsCacheMock(workDir)

	db1 := setupTopicDir(t, workDir, "concurrent-topic", []testAsset{
		{id: "ccc111", size: 1000, ext: "bin", blobName: "000001.dat", offset: 0, createdAt: 1700000000},
	})
	createDatFile(t, workDir, "concurrent-topic", "000001.dat", 1000)
	mock.StoreTopicDB("concurrent-topic", db1)
	mock.RegisterTopic("concurrent-topic", true, "")

	cache := newTestStatsCache(mock)
	cache.BuildAll()

	// Run concurrent reads and writes
	var wg sync.WaitGroup
	const numReaders = 10
	const numIterations = 50

	// Readers
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				cache.GetTopicStats("concurrent-topic")
				cache.GetAllTopicStats()
				cache.GetServiceInfo()
				cache.IsInitialized()
			}
		}()
	}

	// Writer (invalidations)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < numIterations; j++ {
			cache.InvalidateTopic("concurrent-topic")
		}
	}()

	// Writer (build all)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < numIterations/10; j++ {
			cache.BuildAll()
		}
	}()

	wg.Wait()

	// If we get here without a race condition panic, the test passes.
	// The -race flag will detect data races.
	if !cache.IsInitialized() {
		t.Error("cache should still be initialized after concurrent access")
	}
}

func TestStatsCacheConcurrentReadWrite(t *testing.T) {
	workDir := t.TempDir()
	mock := newStatsCacheMock(workDir)

	// Set up multiple topics
	for i := 0; i < 5; i++ {
		name := topicNameForIndex(i)
		db := setupTopicDir(t, workDir, name, []testAsset{
			{id: hashForIndex(i, 0), size: int64((i + 1) * 1000), ext: "bin", blobName: "000001.dat", offset: 0, createdAt: 1700000000},
		})
		createDatFile(t, workDir, name, "000001.dat", (i+1)*1000)
		mock.StoreTopicDB(name, db)
		mock.RegisterTopic(name, true, "")
	}

	cache := newTestStatsCache(mock)
	cache.BuildAll()

	var wg sync.WaitGroup

	// Concurrent readers for different topics
	for i := 0; i < 5; i++ {
		name := topicNameForIndex(i)
		wg.Add(1)
		go func(topicName string) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				cache.GetTopicStats(topicName)
			}
		}(name)
	}

	// Concurrent writer invalidating random topics
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 100; j++ {
			name := topicNameForIndex(j % 5)
			cache.InvalidateTopic(name)
		}
	}()

	// Concurrent removal + re-addition
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 20; j++ {
			cache.RemoveTopic(topicNameForIndex(0))
			cache.InvalidateTopic(topicNameForIndex(0))
		}
	}()

	wg.Wait()
}

// =============================================================================
// toInt64 Tests
// =============================================================================

func TestToInt64(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected int64
	}{
		{"int64 value", int64(42), 42},
		{"float64 value", float64(3.14), 3},
		{"int value", int(99), 99},
		{"string value", "hello", 0},
		{"nil value", nil, 0},
		{"bool value", true, 0},
		{"negative int64", int64(-5), -5},
		{"zero int64", int64(0), 0},
		{"large int64", int64(9999999999), 9999999999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toInt64(tt.input)
			if got != tt.expected {
				t.Errorf("toInt64(%v) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestStatsCacheBuildAll_TopicWithNoAssets(t *testing.T) {
	workDir := t.TempDir()
	mock := newStatsCacheMock(workDir)

	// Topic with empty database (no assets)
	db1 := setupTopicDir(t, workDir, "empty-topic", nil)
	mock.StoreTopicDB("empty-topic", db1)
	mock.RegisterTopic("empty-topic", true, "")

	cache := newTestStatsCache(mock)
	cache.BuildAll()

	stats, ok := cache.GetTopicStats("empty-topic")
	if !ok {
		t.Fatal("empty-topic should be cached")
	}
	if stats["file_count"] != int64(0) {
		t.Errorf("file_count for empty topic: got %v, want 0", stats["file_count"])
	}
	if stats["total_size"] != int64(0) {
		t.Errorf("total_size for empty topic: got %v, want 0", stats["total_size"])
	}
}

func TestStatsCacheInvalidateTopics_MixedHealth(t *testing.T) {
	workDir := t.TempDir()
	mock := newStatsCacheMock(workDir)

	db1 := setupTopicDir(t, workDir, "topic-a", []testAsset{
		{id: "aaa111", size: 1000, ext: "png", blobName: "000001.dat", offset: 0, createdAt: 1700000000},
	})
	createDatFile(t, workDir, "topic-a", "000001.dat", 1000)

	db2 := setupTopicDir(t, workDir, "topic-b", []testAsset{
		{id: "bbb111", size: 2000, ext: "jpg", blobName: "000001.dat", offset: 0, createdAt: 1700000000},
	})
	createDatFile(t, workDir, "topic-b", "000001.dat", 2000)

	mock.StoreTopicDB("topic-a", db1)
	mock.StoreTopicDB("topic-b", db2)
	mock.RegisterTopic("topic-a", true, "")
	mock.RegisterTopic("topic-b", true, "")

	cache := newTestStatsCache(mock)
	cache.BuildAll()

	// Mark topic-b as unhealthy, then batch invalidate both
	mock.RegisterTopic("topic-b", false, "corrupted")
	cache.InvalidateTopics([]string{"topic-a", "topic-b"})

	// topic-a should still be cached (healthy)
	_, ok := cache.GetTopicStats("topic-a")
	if !ok {
		t.Error("topic-a should remain cached (still healthy)")
	}

	// topic-b should be removed (now unhealthy)
	_, ok = cache.GetTopicStats("topic-b")
	if ok {
		t.Error("topic-b should be removed from cache (now unhealthy)")
	}
}

func TestStatsCacheBuildAll_ClearsOldData(t *testing.T) {
	workDir := t.TempDir()
	mock := newStatsCacheMock(workDir)

	db1 := setupTopicDir(t, workDir, "temp-topic", []testAsset{
		{id: "ttt111", size: 500, ext: "bin", blobName: "000001.dat", offset: 0, createdAt: 1700000000},
	})
	createDatFile(t, workDir, "temp-topic", "000001.dat", 500)
	mock.StoreTopicDB("temp-topic", db1)
	mock.RegisterTopic("temp-topic", true, "")

	cache := newTestStatsCache(mock)
	cache.BuildAll()

	_, ok := cache.GetTopicStats("temp-topic")
	if !ok {
		t.Fatal("temp-topic should be cached")
	}

	// Remove the topic from registry and rebuild
	mock.UnregisterTopic("temp-topic")
	cache.BuildAll()

	_, ok = cache.GetTopicStats("temp-topic")
	if ok {
		t.Error("temp-topic should not be cached after rebuild without it in registry")
	}
}

func TestStatsCacheServiceInfoDbSize(t *testing.T) {
	workDir := t.TempDir()
	mock := newStatsCacheMock(workDir)

	// Create a topic with a real database file so db_size stat is populated
	db1 := setupTopicDir(t, workDir, "db-size-topic", []testAsset{
		{id: "ddd111", size: 1000, ext: "bin", blobName: "000001.dat", offset: 0, createdAt: 1700000000},
	})
	createDatFile(t, workDir, "db-size-topic", "000001.dat", 1000)
	mock.StoreTopicDB("db-size-topic", db1)
	mock.RegisterTopic("db-size-topic", true, "")

	cache := newTestStatsCache(mock)
	cache.BuildAll()

	info := cache.GetServiceInfo()

	// The DB file should have a non-zero size
	if info.StorageSummary.TotalDbSize <= 0 {
		t.Errorf("expected total db size > 0, got %d", info.StorageSummary.TotalDbSize)
	}
}

// =============================================================================
// Helpers for concurrent tests
// =============================================================================

func topicNameForIndex(i int) string {
	names := []string{"alpha", "beta", "gamma", "delta", "epsilon"}
	return names[i%len(names)]
}

func hashForIndex(topicIdx, assetIdx int) string {
	// Generate unique hex-like hashes for test data
	return fmt.Sprintf("%06x%06x", topicIdx, assetIdx)
}

