# Phase 9: End-to-End Testing

## Objective
Implement comprehensive E2E tests that verify real workflows against the running server. Focus on integration testing rather than unit tests for trivial utilities.

---

## Prerequisites
- Phases 1-5 completed and functional:
  - Phase 1: Foundation (config, logger, constants, working directory)
  - Phase 2: Database Layer (SQLite, orchestrator, metadata)
  - Phase 3: Blob Format (.dat files with BLAKE3 headers)
  - Phase 4: Core API (HTTP endpoints for topics, assets, metadata)
  - Phase 5: Query System (preset queries with parameter binding)
- Full backend API operational
- All query presets verified in `internal/config/queries_default.go`

---

## Philosophy
- **No unit tests for trivial utils** - waste of time, focus on integration
- **E2E tests verify real workflows** - actual HTTP requests, database operations, file I/O
- **Test edge cases** - thresholds, corruption, multiple uploads, error conditions
- **Isolated test environments** - all tests use temp directories with automatic cleanup
- **Fast and deterministic** - tests should complete in < 5 minutes for CI

---

## Architecture Overview

### Testing Approach
Use `httptest.NewServer` for in-process testing:
- Create `server.App` instance with temp directories
- Pass `nil` for webFS (tests only need API endpoints)
- Avoids subprocess overhead while testing full HTTP stack
- Proper cleanup via `t.Cleanup()` ensures no leaked resources

### Test Infrastructure Components
1. **TestServer** - Wraps HTTP server with temp work/config directories
2. **Fixtures** - Generate test files (random data, valid GLB headers)
3. **Helper Methods** - HTTP wrappers (GET, POST, UploadFile, GetJSON)
4. **Database Helpers** - Direct SQL queries for verification

---

## Task 1: Test Infrastructure (`e2e/setup_test.go`)

### Purpose
Provide reusable helpers for all E2E tests with proper isolation and cleanup.

### Implementation Details

```go
package e2e

import (
    "bytes"
    "database/sql"
    "encoding/json"
    "fmt"
    "io"
    "mime/multipart"
    "net/http"
    "net/http/httptest"
    "os"
    "path/filepath"
    "testing"

    "meshbank/internal/config"
    "meshbank/internal/constants"
    "meshbank/internal/database"
    "meshbank/internal/logger"
    "meshbank/internal/queries"
    "meshbank/internal/server"
)

// TestServer wraps a running meshbank server for testing
type TestServer struct {
    Server     *httptest.Server
    App        *server.App
    WorkDir    string
    ConfigDir  string
    URL        string
}

// StartTestServer creates a new test server with isolated directories
func StartTestServer(t *testing.T) *TestServer {
    t.Helper()

    // Create temp directories
    workDir, err := os.MkdirTemp("", "meshbank-test-work-*")
    if err != nil {
        t.Fatalf("failed to create work dir: %v", err)
    }

    configDir, err := os.MkdirTemp("", "meshbank-test-config-*")
    if err != nil {
        os.RemoveAll(workDir)
        t.Fatalf("failed to create config dir: %v", err)
    }

    // Create app instance with temp config
    cfg := &config.Config{
        WorkingDirectory: "", // Not configured initially
        Port:             0,  // Let httptest assign port
        MaxDatSize:       constants.DefaultMaxDatSize,
    }

    log := logger.NewLogger(constants.LogLevelError) // Suppress logs in tests
    app := server.NewApp(cfg, log)

    // Load queries config
    queriesConfig, _ := queries.LoadQueriesConfig("")
    if queriesConfig == nil {
        // Use default queries if file doesn't exist
        queriesConfig = &queries.QueriesConfig{} // Minimal config
    }
    app.QueriesConfig = queriesConfig

    // Create HTTP server
    srv := server.NewServer(app, ":0", nil) // nil webFS for API-only testing
    httpServer := httptest.NewServer(srv.Handler())

    ts := &TestServer{
        Server:    httpServer,
        App:       app,
        WorkDir:   workDir,
        ConfigDir: configDir,
        URL:       httpServer.URL,
    }

    // Register cleanup
    t.Cleanup(func() {
        ts.Cleanup()
    })

    return ts
}

// Cleanup removes temp directories and closes connections
func (ts *TestServer) Cleanup() {
    if ts.Server != nil {
        ts.Server.Close()
    }
    if ts.App != nil {
        ts.App.CloseAllTopicDBs()
        if ts.App.OrchestratorDB != nil {
            ts.App.OrchestratorDB.Close()
        }
    }
    os.RemoveAll(ts.WorkDir)
    os.RemoveAll(ts.ConfigDir)
}

// Shutdown gracefully stops the server (for restart tests)
func (ts *TestServer) Shutdown() {
    if ts.Server != nil {
        ts.Server.Close()
        ts.Server = nil
    }
    if ts.App != nil {
        ts.App.CloseAllTopicDBs()
        if ts.App.OrchestratorDB != nil {
            ts.App.OrchestratorDB.Close()
            ts.App.OrchestratorDB = nil
        }
    }
}

// Restart creates a new server with same directories
func (ts *TestServer) Restart(t *testing.T) {
    t.Helper()
    ts.Shutdown()

    // Recreate app with same directories
    cfg := ts.App.Config
    log := logger.NewLogger(constants.LogLevelError)
    app := server.NewApp(cfg, log)
    app.QueriesConfig = ts.App.QueriesConfig

    // Reinitialize if working directory set
    if cfg.WorkingDirectory != "" {
        orchPath := filepath.Join(cfg.WorkingDirectory, constants.InternalDir, constants.OrchestratorDB)
        orchDB, err := database.InitOrchestratorDB(orchPath)
        if err != nil {
            t.Fatalf("failed to reopen orchestrator: %v", err)
        }
        app.OrchestratorDB = orchDB

        // Rediscover topics
        topics, _ := config.DiscoverTopics(cfg.WorkingDirectory)
        for _, topic := range topics {
            app.RegisterTopic(topic.Name, topic.Healthy, topic.Error)
        }
    }

    srv := server.NewServer(app, ":0", nil)
    httpServer := httptest.NewServer(srv.Handler())

    ts.Server = httpServer
    ts.App = app
    ts.URL = httpServer.URL
}

// Helper methods for API calls

func (ts *TestServer) GET(path string) (*http.Response, error) {
    return http.Get(ts.URL + path)
}

func (ts *TestServer) POST(path string, body interface{}) (*http.Response, error) {
    jsonBody, _ := json.Marshal(body)
    return http.Post(ts.URL+path, "application/json", bytes.NewReader(jsonBody))
}

func (ts *TestServer) GetJSON(path string, target interface{}) error {
    resp, err := ts.GET(path)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    return json.NewDecoder(resp.Body).Decode(target)
}

func (ts *TestServer) PostJSON(path string, body, target interface{}) error {
    resp, err := ts.POST(path, body)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    if target != nil {
        return json.NewDecoder(resp.Body).Decode(target)
    }
    return nil
}

func (ts *TestServer) UploadFile(topicName, filename string, content []byte, parentID string) (*http.Response, error) {
    var buf bytes.Buffer
    writer := multipart.NewWriter(&buf)

    part, err := writer.CreateFormFile("file", filename)
    if err != nil {
        return nil, err
    }
    part.Write(content)

    if parentID != "" {
        writer.WriteField("parent_id", parentID)
    }

    writer.Close()

    req, _ := http.NewRequest("POST", ts.URL+"/api/topics/"+topicName+"/assets", &buf)
    req.Header.Set("Content-Type", writer.FormDataContentType())

    return http.DefaultClient.Do(req)
}

// ConfigureWorkDir sets the working directory via API
func (ts *TestServer) ConfigureWorkDir(t *testing.T) {
    t.Helper()
    resp, err := ts.POST("/api/config", map[string]interface{}{
        "working_directory": ts.WorkDir,
    })
    if err != nil {
        t.Fatalf("failed to configure work dir: %v", err)
    }
    defer resp.Body.Close()
    if resp.StatusCode != 200 {
        t.Fatalf("config failed with status %d", resp.StatusCode)
    }
}

// CreateTopic creates a topic via API
func (ts *TestServer) CreateTopic(t *testing.T, name string) {
    t.Helper()
    resp, err := ts.POST("/api/topics", map[string]string{"name": name})
    if err != nil {
        t.Fatalf("failed to create topic: %v", err)
    }
    defer resp.Body.Close()
    if resp.StatusCode != 200 && resp.StatusCode != 201 {
        t.Fatalf("create topic failed with status %d", resp.StatusCode)
    }
}

// GetTopicDB opens a direct connection to topic database for verification
func (ts *TestServer) GetTopicDB(t *testing.T, topicName string) *sql.DB {
    t.Helper()
    dbPath := filepath.Join(ts.WorkDir, topicName, constants.InternalDir, topicName+".db")
    db, err := sql.Open("sqlite3", dbPath)
    if err != nil {
        t.Fatalf("failed to open topic db: %v", err)
    }
    t.Cleanup(func() { db.Close() })
    return db
}

// GetOrchestratorDB opens direct connection to orchestrator database
func (ts *TestServer) GetOrchestratorDB(t *testing.T) *sql.DB {
    t.Helper()
    dbPath := filepath.Join(ts.WorkDir, constants.InternalDir, constants.OrchestratorDB)
    db, err := sql.Open("sqlite3", dbPath)
    if err != nil {
        t.Fatalf("failed to open orchestrator db: %v", err)
    }
    t.Cleanup(func() { db.Close() })
    return db
}
```

### Key Features
- **Isolated environments**: Each test gets fresh temp directories
- **Automatic cleanup**: Uses `t.Cleanup()` for guaranteed cleanup
- **Restart support**: `Shutdown()` and `Restart()` for discovery tests
- **Direct DB access**: Helper methods for verification queries
- **JSON helpers**: `GetJSON()`, `PostJSON()` for easier assertions
- **Convenience methods**: `ConfigureWorkDir()`, `CreateTopic()`

---

## Task 2: Test Fixtures (`e2e/fixtures_test.go`)

### Purpose
Generate consistent, reusable test data for uploads and validation.

### Implementation

```go
package e2e

import (
    "crypto/rand"
    "encoding/binary"
)

// GenerateTestFile creates a random file of given size
func GenerateTestFile(size int) []byte {
    data := make([]byte, size)
    rand.Read(data)
    return data
}

// GenerateTestGLB creates a minimal valid GLB file
// GLB format: https://registry.khronos.org/glTF/specs/2.0/glTF-2.0.html#glb-file-format-specification
func GenerateTestGLB(size int) []byte {
    if size < 12 {
        size = 12 // Minimum GLB header size
    }

    data := make([]byte, size)

    // Magic: "glTF" (0x46546C67 in little-endian)
    data[0] = 0x67
    data[1] = 0x6C
    data[2] = 0x54
    data[3] = 0x46

    // Version: 2 (uint32 little-endian)
    binary.LittleEndian.PutUint32(data[4:8], 2)

    // Length: total file size (uint32 little-endian)
    binary.LittleEndian.PutUint32(data[8:12], uint32(size))

    // Fill rest with random data
    if size > 12 {
        rand.Read(data[12:])
    }

    return data
}

// Pre-generated test files for common scenarios
var (
    SmallFile  = GenerateTestFile(1024)        // 1 KB
    MediumFile = GenerateTestFile(100 * 1024)  // 100 KB
    LargeFile  = GenerateTestFile(1024 * 1024) // 1 MB
)
```

### Test Files
- **SmallFile**: 1 KB - for quick tests
- **MediumFile**: 100 KB - typical asset size
- **LargeFile**: 1 MB - near threshold tests
- **GenerateTestGLB**: Creates valid GLB files with proper headers

---

## Task 3: Test Scenarios (15 Tests)

### 3.1 Fresh Start Flow (`e2e/fresh_start_test.go`)

**Purpose**: Verify server starts correctly without prior configuration.

**Test Steps**:
1. Start server (no config exists)
2. GET `/api/config` → verify `configured: false`
3. POST `/api/config` with `working_directory`
4. Verify config persisted in app state
5. GET `/api/topics` → empty list `[]`

**Success Criteria**:
- Server starts without errors
- Unconfigured state detected correctly
- Configuration persists after POST
- Working directory initialized with `.internal/orchestrator.db`

---

### 3.2 Topic Management (`e2e/topics_test.go`)

**Purpose**: Test topic creation, validation, and conflict handling.

**Test Steps**:
1. POST `/api/topics` `{"name": "test-topic"}` → 200/201
2. POST same topic → 409 Conflict
3. POST `{"name": "INVALID"}` → 400 (uppercase)
4. POST `{"name": "special@chars"}` → 400 (invalid chars)
5. POST `{"name": "a"}` → 200 (valid single char)
6. GET `/api/topics` → verify test-topic exists with zero stats

**Validation Rules**:
- Name must match regex: `^[a-z0-9_-]+$`
- Length: 1-64 characters
- No uppercase, no special characters except `_` and `-`

**Success Criteria**:
- Valid topics created successfully
- Duplicate detection works
- Validation errors return 400
- Topic folder structure created: `<topic>/.internal/<topic>.db`

---

### 3.3 Single Asset Upload (`e2e/upload_test.go`)

**Purpose**: Test basic file upload workflow end-to-end.

**Test Steps**:
1. Create topic `test-topic`
2. Upload `SmallFile` with filename `test.bin`
3. Verify response contains correct BLAKE3 hash
4. Verify response contains `blob_name: "001.dat"`, `byte_offset: 0`
5. Verify `001.dat` exists in topic folder
6. Query `assets` table → verify entry with correct hash, size, extension
7. Query `orchestrator.db` → verify asset index exists
8. GET `/api/assets/:hash/download` → verify content matches original

**Success Criteria**:
- Hash computation correct (BLAKE3 hex)
- File stored in 001.dat with proper header
- Database entries created atomically
- Download returns original bytes
- MIME type header correct (application/octet-stream for .bin)

---

### 3.4 Duplicate Detection (`e2e/dedup_test.go`)

**Purpose**: Test global deduplication across topics.

**Test Steps**:
1. Upload `fileA` to `topic-1` → get `hashA`
2. Upload same `fileA` to `topic-1` → verify `skipped: true`, same `hashA`
3. Create `topic-2`
4. Upload same `fileA` to `topic-2` → verify `skipped: true` (global dedup)
5. Query `orchestrator.db` → verify single entry for `hashA`
6. Query both topic DBs → verify no duplicate asset entries

**Success Criteria**:
- Duplicate detection works within same topic
- Global deduplication works across topics
- No redundant storage of identical files
- Orchestrator tracks single entry per hash

---

### 3.5 Multiple Sequential Uploads (`e2e/multi_upload_test.go`)

**Purpose**: Stress test with multiple files, verify all succeed.

**Test Steps**:
1. Generate 10 files of varied sizes (1KB, 5KB, 10KB, 20KB, 50KB, 100KB, 200KB, 500KB, 1MB, 1MB)
2. Upload each file sequentially
3. Store returned hashes
4. Download each file by hash → verify content matches original
5. Re-upload all 10 files → verify all return `skipped: true`

**Success Criteria**:
- All files upload successfully
- Hashes computed correctly
- All downloads return correct bytes
- Deduplication works for all re-uploads
- No orphan .dat files created

---

### 3.6 DAT Threshold (`e2e/dat_threshold_test.go`)

**Purpose**: Test .dat file rollover when exceeding max_dat_size.

**Test Steps**:
1. POST `/api/config` with `max_dat_size: 1048576` (1 MB)
2. Upload 500 KB file → verify `blob_name: "001.dat"`
3. Upload 500 KB file → verify `blob_name: "001.dat"` (total 1 MB)
4. Upload 500 KB file → verify `blob_name: "002.dat"` (rollover)
5. Verify both `001.dat` and `002.dat` exist
6. Query `dat_hashes` table → verify both files have hash entries

**Success Criteria**:
- Files fill 001.dat up to threshold
- Rollover creates 002.dat correctly
- DAT hash tracking updated for both files
- File numbering sequential (001, 002, 003, ...)

---

### 3.7 Max File Size (`e2e/max_size_test.go`)

**Purpose**: Test rejection of files exceeding max_dat_size.

**Test Steps**:
1. POST `/api/config` with `max_dat_size: 1048576` (1 MB)
2. Generate 2 MB file
3. Try upload → verify 413 Payload Too Large
4. Verify no `.dat` file created, or if created, has size 0
5. Query `assets` table → verify no entry
6. Query `orchestrator.db` → verify no entry

**Success Criteria**:
- Large file rejected before processing
- No partial writes to .dat file
- No database corruption
- Clear error message in response

---

### 3.8 Metadata Operations (`e2e/metadata_test.go`)

**Purpose**: Test metadata set/delete operations and computed state.

**Test Steps**:
1. Upload asset → get `hash`
2. POST `/api/assets/:hash/metadata` `{"op": "set", "key": "polycount", "value": "1000"}`
3. Query `metadata_log` → verify entry with `value_text: "1000"`, `value_num: 1000`
4. Query `metadata_computed` → verify `{"polycount": {"text": "1000", "num": 1000}}`
5. POST set `has_skeleton=true`
6. Query `metadata_computed` → verify both keys present
7. POST `{"op": "delete", "key": "polycount"}`
8. Query `metadata_computed` → verify only `has_skeleton` remains

**Success Criteria**:
- Metadata log append-only (no updates)
- Computed state rebuilt correctly after each operation
- Value type detection works (text + num for numeric values)
- Delete removes key from computed state

---

### 3.9 Lineage Tracking (`e2e/lineage_test.go`)

**Purpose**: Test parent/child asset relationships and queries.

**Test Steps**:
1. Upload `fileA` (no parent) → `hashA`
2. Upload `fileB` with `parent_id=hashA` → `hashB`
3. Upload `fileC` with `parent_id=hashB` → `hashC`
4. Query `assets` table → verify `parent_id` columns: `A→null, B→hashA, C→hashB`
5. POST `/api/query/lineage` `{"params": {"hash": hashC}}` → verify returns A, B, C chain
6. POST `/api/query/derived` `{"params": {"hash": hashA}}` → verify returns B, C

**Query Presets Required**:
- `lineage`: Recursive CTE to walk up parent chain
- `derived`: Recursive CTE to walk down children

**Success Criteria**:
- Parent IDs stored correctly
- Lineage query returns full ancestry chain
- Derived query returns all descendants
- Cross-topic lineage works (if parent in different topic)

---

### 3.10 Query Execution (`e2e/queries_test.go`)

**Purpose**: Test query preset execution with parameter binding.

**Test Steps**:
1. Upload 50 files with varied sizes and timestamps
2. Add metadata to 10 files (e.g., `{"processor": "test"}`)
3. POST `/api/query/recent-imports` `{"params": {"days": 7, "limit": 10}}`
   - Verify returns ≤ 10 results
   - Verify all have `created_at` within last 7 days
4. POST `/api/query/large-files` `{"params": {"min_size": 50000}}`
   - Verify all results have `asset_size >= 50000`
5. POST `/api/query/with-metadata` `{"params": {"key": "processor"}}`
   - Verify only files with metadata returned
6. POST `/api/query/by-hash` `{"params": {"hash": "abc"}}`
   - Verify results have hash starting with "abc"

**Query Presets Tested**:
- `recent-imports`: Time-based filtering
- `large-files`: Size filtering
- `with-metadata`: Metadata presence check
- `by-hash`: Hash prefix matching

**Success Criteria**:
- Parameter binding works correctly
- SQL injection prevented (parameterized queries)
- Results match filter criteria
- Empty results return `[]` not `null`

---

### 3.11 Cross-Topic Queries (`e2e/cross_topic_test.go`)

**Purpose**: Test query execution across multiple topics.

**Test Steps**:
1. Create `topic-1`, upload 10 files
2. Create `topic-2`, upload 10 files (different files)
3. POST `/api/query/recent-imports` `{"topics": []}` → verify 20 results
4. Verify each result has `_topic` column with correct topic name
5. POST `/api/query/recent-imports` `{"topics": ["topic-1"]}` → verify 10 results
6. Verify all have `_topic: "topic-1"`

**Success Criteria**:
- Empty topics array queries all healthy topics
- Explicit topic filter works correctly
- `_topic` column appended to all results
- Results interleaved from all topics
- Unhealthy topics skipped automatically

---

### 3.12 Topic Discovery (`e2e/discovery_test.go`)

**Purpose**: Test automatic topic discovery on server restart.

**Test Steps**:
1. Start server, configure work dir
2. Create `test-topic`, upload 3 files
3. Get topic stats → store for comparison
4. Call `ts.Shutdown()` to stop server
5. Call `ts.Restart(t)` to start new server with same work dir
6. GET `/api/topics` → verify `test-topic` exists
7. Verify stats match (file count, total size)
8. Download files by hash → verify still accessible

**Success Criteria**:
- Topics auto-discovered on startup
- Stats computed correctly from database
- Files remain downloadable after restart
- DAT hashes verified during discovery
- Healthy status maintained

---

### 3.13 Portable Topic (`e2e/portability_test.go`)

**Purpose**: Test topic portability to new location.

**Test Steps**:
1. Create topic in `workDir1`, upload files
2. Copy entire topic folder to `workDir2` (use `cp -r` or Go file copy)
3. Start new server pointing to `workDir2`
4. Verify topic discovered
5. Download files → verify accessible
6. Upload new file → verify writes work

**Success Criteria**:
- Topics self-contained (no absolute paths)
- Discovery works in new location
- All files accessible
- New uploads work correctly
- Orchestrator DB rebuilt correctly

---

### 3.14 Corruption Detection (`e2e/corruption_test.go`)

**Purpose**: Test error handling when .dat files are corrupted.

**Test Steps**:
1. Create topic, upload 3 files
2. Call `ts.Shutdown()`
3. Manually corrupt `001.dat` (overwrite bytes 500-600 with zeros)
4. Call `ts.Restart(t)`
5. GET `/api/topics` → verify topic marked `healthy: false`
6. Verify error message mentions corruption/hash mismatch
7. GET `/api/logs` → verify error logged
8. POST upload to corrupted topic → verify 503 Service Unavailable
9. POST query to corrupted topic → verify error

**Corruption Detection**:
- During discovery, `VerifyAllDatHashes()` runs
- BLAKE3 hash mismatch marks topic unhealthy
- Error stored in topic health registry

**Success Criteria**:
- Corruption detected on discovery
- Topic marked unhealthy with clear error
- Uploads/queries to unhealthy topic fail gracefully
- Other topics remain functional

---

### 3.15 Value Type Detection (`e2e/value_type_test.go`)

**Purpose**: Test metadata value type inference (text vs numeric).

**Test Steps**:
1. Upload asset → `hash`
2. Test case: `value="123"`
   - POST set metadata
   - Query `metadata_computed` → verify `{"text": "123", "num": 123}`
3. Test case: `value="123.45"`
   - Verify `{"text": "123.45", "num": 123.45}`
4. Test case: `value="00123"` (leading zeros)
   - Verify `{"text": "00123"}` only (no `num` field)
5. Test case: `value="1.0"` (trailing zero)
   - Verify `{"text": "1.0"}` only
6. Test case: `value="hello"`
   - Verify `{"text": "hello"}` only

**Type Detection Rules** (from Phase 2):
- Pure integer/float without leading zeros → stored as both text and num
- Leading zeros (00123) → text only
- Trailing decimal zeros (1.0) → text only
- Non-numeric → text only

**Success Criteria**:
- Type detection matches specification
- Edge cases handled correctly
- Metadata queries can filter by numeric values

---

## Task 4: Running Tests

### 4.1 Test Commands

```bash
# Run all E2E tests
go test ./e2e/... -v

# Run specific test
go test ./e2e/... -v -run TestSingleAssetUpload

# Run with race detector (check concurrency issues)
go test ./e2e/... -v -race

# Run with coverage report
go test ./e2e/... -v -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html

# Run multiple times to catch flaky tests
for i in {1..5}; do go test ./e2e/... -v || break; done
```

### 4.2 Makefile Integration

The existing Makefile already supports:
```bash
make test            # Run all tests (including e2e)
make test-verbose    # Verbose output
make test-coverage   # Generate coverage report
```

### 4.3 CI Integration (Optional)

```yaml
# .github/workflows/test.yml
name: Tests
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      - name: Run unit tests
        run: go test ./internal/... -v
      - name: Run E2E tests
        run: go test ./e2e/... -v
      - name: Run with race detector
        run: go test ./e2e/... -race
```

---

## Verification Checklist

After completing Phase 9, verify:

### ✅ All Tests Pass
- [ ] `go test ./e2e/... -v` completes successfully
- [ ] No flaky tests (run 3-5 times, all pass)
- [ ] All 15 test scenarios covered

### ✅ Edge Cases Covered
- [ ] Empty topic (no files)
- [ ] Large files near threshold (999 KB, 1000 KB, 1001 KB)
- [ ] Unicode filenames (if supported)
- [ ] Concurrent access with `-race` detector

### ✅ Cleanup Works
- [ ] No temp directories left: `ls /tmp | grep meshbank-test` → empty
- [ ] No orphan processes: `ps aux | grep meshbank` → only current
- [ ] Database connections closed properly

### ✅ CI Ready
- [ ] Tests run in isolation (no shared state)
- [ ] No external dependencies (no Docker, no real files)
- [ ] Execution time < 5 minutes
- [ ] Tests use `t.Parallel()` where safe

### ✅ Code Quality
- [ ] All helper functions use `t.Helper()`
- [ ] All tests use `t.Cleanup()` for cleanup
- [ ] Tests verify response bodies, not just status codes
- [ ] Error messages are descriptive

---

## Files to Create

| File | Lines | Description |
|------|-------|-------------|
| `e2e/setup_test.go` | ~300 | Test infrastructure, TestServer, helpers |
| `e2e/fixtures_test.go` | ~50 | Test file generators (random, GLB) |
| `e2e/fresh_start_test.go` | ~40 | Fresh installation tests |
| `e2e/topics_test.go` | ~80 | Topic CRUD and validation tests |
| `e2e/upload_test.go` | ~70 | Single upload workflow tests |
| `e2e/dedup_test.go` | ~60 | Duplicate detection tests |
| `e2e/multi_upload_test.go` | ~80 | Multiple file upload tests |
| `e2e/dat_threshold_test.go` | ~70 | DAT rollover tests |
| `e2e/max_size_test.go` | ~50 | File size limit tests |
| `e2e/metadata_test.go` | ~90 | Metadata CRUD tests |
| `e2e/lineage_test.go` | ~80 | Parent/child tracking tests |
| `e2e/queries_test.go` | ~120 | Query preset execution tests |
| `e2e/cross_topic_test.go` | ~70 | Cross-topic query tests |
| `e2e/discovery_test.go` | ~80 | Topic discovery tests |
| `e2e/portability_test.go` | ~70 | Portable topic tests |
| `e2e/corruption_test.go` | ~90 | Corruption detection tests |
| `e2e/value_type_test.go` | ~100 | Metadata type detection tests |

**Total**: ~1,500 lines of test code across 17 files

---

## Implementation Order

Recommended order to maximize early feedback:

1. **Infrastructure First** (30 min)
   - `setup_test.go` + `fixtures_test.go`
   - Run `go test ./e2e/... -v` to verify compilation

2. **Basic Flows** (30 min)
   - `fresh_start_test.go` + `topics_test.go`
   - Verify server starts, config works, topics created

3. **Core Upload Logic** (45 min)
   - `upload_test.go` + `dedup_test.go`
   - Most critical functionality: file storage and deduplication

4. **File Handling** (1 hr)
   - `multi_upload_test.go` + `dat_threshold_test.go` + `max_size_test.go`
   - Stress test file operations, thresholds

5. **Metadata System** (45 min)
   - `metadata_test.go` + `value_type_test.go`
   - Test metadata CRUD and type detection

6. **Lineage** (30 min)
   - `lineage_test.go`
   - Parent/child relationships and recursive queries

7. **Query System** (1 hr)
   - `queries_test.go` + `cross_topic_test.go`
   - Preset queries, parameter binding, cross-topic

8. **Persistence** (45 min)
   - `discovery_test.go` + `portability_test.go`
   - Server restart, topic portability

9. **Error Handling** (30 min)
   - `corruption_test.go`
   - Corruption detection, unhealthy topics

**Total estimated time**: ~6 hours

---

## Additional Tests (Optional Future Work)

These tests are NOT in the phase-09 specification but could be valuable:

1. **Unicode Filename Test** - Upload/download files with unicode characters
2. **Concurrent Upload Test** - Multiple files uploaded simultaneously (use `-race`)
3. **Large File Streaming Test** - Upload 900MB file (may be slow for CI)
4. **Extension Detection Test** - Verify .glb, .png, .jpg extensions and MIME types
5. **Empty File Test** - Upload 0-byte file, verify handling
6. **Topic Stats Accuracy Test** - Verify all stats match exact values
7. **Parent Validation Test** - Upload with non-existent `parent_id`, verify error
8. **Error Recovery Test** - Disk full simulation (difficult to test reliably)
9. **Query Parameter Validation Test** - Missing/invalid params, verify errors
10. **Config Validation Test** - Invalid working directory, negative `max_dat_size`

**Recommendation**: Implement the 15 core tests first. Add these later if comprehensive edge-case coverage is desired.

---

## Success Criteria

- ✅ All 15 E2E test files created and passing
- ✅ Test infrastructure reusable and well-documented
- ✅ No flaky tests (consistent results across runs)
- ✅ Coverage of all major API endpoints
- ✅ Edge cases tested (thresholds, errors, corruption)
- ✅ Tests run in isolation with automatic cleanup
- ✅ CI-ready (fast, no external dependencies)
- ✅ Following Go testing best practices (`t.Helper`, `t.Cleanup`, `t.Parallel`)

---

## Notes

- **Use `t.Helper()`** in helper functions for better error source locations
- **Use `t.Parallel()`** where tests are independent (avoid for restart tests)
- **Use `t.Cleanup()`** for automatic cleanup on test end
- **Keep tests deterministic** - no random test order dependencies
- **Test both success and error paths** - 200, 400, 409, 413, 503
- **Verify response bodies** - don't just check status codes
- **Use direct DB queries** for verification (not just API responses)
- **httptest.Server** provides in-process testing without subprocess overhead

---

## Query Presets Verification

All required query presets exist in `internal/config/queries_default.go`:

- ✅ `recent-imports` (params: days, limit)
- ✅ `by-hash` (params: hash)
- ✅ `large-files` (params: min_size, limit)
- ✅ `with-metadata` (params: key, limit)
- ✅ `lineage` (params: hash) - recursive CTE
- ✅ `derived` (params: hash) - recursive CTE
- ✅ `count` (no params)
- ✅ `by-processor` (params: processor, limit)

**No code changes needed** - all query tests can proceed as planned.
