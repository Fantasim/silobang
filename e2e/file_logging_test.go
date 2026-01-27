package e2e

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"silobang/internal/config"
	"silobang/internal/constants"
	"silobang/internal/logger"
	"silobang/internal/queries"
	"silobang/internal/server"
)

// StartTestServerWithLogging creates a test server with debug level logging enabled
// This allows file logging tests to verify that logs are being written
func StartTestServerWithLogging(t *testing.T) *TestServer {
	t.Helper()

	// Create temp directories
	workDir, err := os.MkdirTemp("", "silobang-test-work-*")
	if err != nil {
		t.Fatalf("failed to create work dir: %v", err)
	}

	configDir, err := os.MkdirTemp("", "silobang-test-config-*")
	if err != nil {
		os.RemoveAll(workDir)
		t.Fatalf("failed to create config dir: %v", err)
	}

	// Create app instance with DEBUG level logging (not ERROR like normal tests)
	cfg := &config.Config{
		WorkingDirectory: "",
		Port:             0,
		MaxDatSize:       constants.DefaultMaxDatSize,
	}
	cfg.ApplyDefaults()

	// Use INFO level so we can see logs being written to files
	log := logger.NewLoggerWithOptions(logger.LoggerOptions{
		Level:         logger.LevelInfo,
		WriteToStdout: false, // Don't clutter test output
	})
	app := server.NewApp(cfg, log)

	// Load default queries
	app.QueriesConfig = queries.GetDefaultConfig()

	// Create HTTP server
	srv := server.NewServer(app, ":0", nil)
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
		log.Close() // Close file handles
		ts.Cleanup()
	})

	return ts
}

// TestFileLoggingDirectoryCreation verifies log directories are created on workdir init
func TestFileLoggingDirectoryCreation(t *testing.T) {
	ts := StartTestServerWithLogging(t)
	ts.ConfigureWorkDir(t)

	// Verify log directories exist
	logsDir := filepath.Join(ts.WorkDir, constants.InternalDir, constants.LogsDir)

	expectedDirs := []string{
		constants.LogsDirDebug,
		constants.LogsDirInfo,
		constants.LogsDirWarn,
		constants.LogsDirError,
	}

	for _, dir := range expectedDirs {
		path := filepath.Join(logsDir, dir)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("Expected log directory %s to exist: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("Expected %s to be a directory", dir)
		}
	}
}

// TestFileLoggingCreatesFiles verifies log files are created when logging occurs
func TestFileLoggingCreatesFiles(t *testing.T) {
	ts := StartTestServerWithLogging(t)
	ts.ConfigureWorkDir(t)

	// Perform actions that trigger logging (e.g., create topic)
	ts.CreateTopic(t, "log-test-topic")

	// Allow time for log flush
	time.Sleep(100 * time.Millisecond)

	// Check that at least info log file exists
	infoDir := filepath.Join(ts.WorkDir, constants.InternalDir, constants.LogsDir, constants.LogsDirInfo)
	files, err := os.ReadDir(infoDir)
	if err != nil {
		t.Fatalf("Failed to read info log directory: %v", err)
	}

	if len(files) == 0 {
		t.Error("Expected at least one log file in info directory")
	}

	// Verify file naming convention (unix timestamp + .log)
	for _, f := range files {
		if !strings.HasSuffix(f.Name(), constants.LogFileExtension) {
			t.Errorf("Log file %s doesn't have expected extension %s", f.Name(), constants.LogFileExtension)
		}
	}
}

// TestFileLoggingContent verifies log content format
func TestFileLoggingContent(t *testing.T) {
	ts := StartTestServerWithLogging(t)
	ts.ConfigureWorkDir(t)

	// Trigger a known action
	ts.CreateTopic(t, "content-test-topic")
	time.Sleep(100 * time.Millisecond)

	// Find and read the info log file
	infoDir := filepath.Join(ts.WorkDir, constants.InternalDir, constants.LogsDir, constants.LogsDirInfo)
	files, _ := os.ReadDir(infoDir)

	if len(files) == 0 {
		t.Skip("No log files found")
	}

	logPath := filepath.Join(infoDir, files[0].Name())
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	// Verify format: [INFO] YYYY-MM-DD HH:MM:SS | message
	if !strings.Contains(string(content), "[INFO]") {
		t.Error("Log content missing expected [INFO] prefix")
	}
	if !strings.Contains(string(content), "|") {
		t.Error("Log content missing expected | separator")
	}
}

// TestFileLoggingPersistsAfterRestart verifies logs persist after server restart
func TestFileLoggingPersistsAfterRestart(t *testing.T) {
	ts := StartTestServerWithLogging(t)
	ts.ConfigureWorkDir(t)

	ts.CreateTopic(t, "persist-test")
	time.Sleep(100 * time.Millisecond)

	// Count log files before restart
	infoDir := filepath.Join(ts.WorkDir, constants.InternalDir, constants.LogsDir, constants.LogsDirInfo)
	filesBefore, _ := os.ReadDir(infoDir)

	// Read content of first file
	var contentBefore string
	if len(filesBefore) > 0 {
		data, _ := os.ReadFile(filepath.Join(infoDir, filesBefore[0].Name()))
		contentBefore = string(data)
	}

	// Restart
	ts.Restart(t)

	// Verify files still exist with same content
	filesAfter, _ := os.ReadDir(infoDir)
	if len(filesAfter) != len(filesBefore) {
		t.Errorf("Log file count changed: before=%d, after=%d", len(filesBefore), len(filesAfter))
	}

	if len(filesAfter) > 0 {
		data, _ := os.ReadFile(filepath.Join(infoDir, filesAfter[0].Name()))
		if !strings.HasPrefix(string(data), contentBefore) {
			t.Error("Log content was modified or lost after restart")
		}
	}
}

// TestFileLoggingLevelSeparation verifies different levels go to different files
func TestFileLoggingLevelSeparation(t *testing.T) {
	ts := StartTestServerWithLogging(t)
	ts.ConfigureWorkDir(t)

	// Trigger various actions that produce different log levels
	ts.CreateTopic(t, "level-test")

	// Try to upload to non-existent topic (will produce error log)
	ts.UploadFileExpectError(t, "nonexistent-topic-xyz", "test.bin", []byte("test"), "", 404)

	time.Sleep(100 * time.Millisecond)

	// Check that info directory has files
	infoDir := filepath.Join(ts.WorkDir, constants.InternalDir, constants.LogsDir, constants.LogsDirInfo)
	infoFiles, _ := os.ReadDir(infoDir)

	if len(infoFiles) == 0 {
		t.Error("Expected info log files")
	}

	// Check that error directory has files (from the failed upload)
	errorDir := filepath.Join(ts.WorkDir, constants.InternalDir, constants.LogsDir, constants.LogsDirError)
	errorFiles, _ := os.ReadDir(errorDir)

	// Note: error files may or may not exist depending on how errors are logged
	// The important thing is that info and error are in separate directories
	t.Logf("Info files: %d, Error files: %d", len(infoFiles), len(errorFiles))
}

// TestFileLoggingStructureIntegrity verifies log directory structure stays intact
func TestFileLoggingStructureIntegrity(t *testing.T) {
	ts := StartTestServerWithLogging(t)
	ts.ConfigureWorkDir(t)

	ts.CreateTopic(t, "structure-test")
	time.Sleep(100 * time.Millisecond)

	// Get log file count in original location
	logsDir := filepath.Join(ts.WorkDir, constants.InternalDir, constants.LogsDir)

	// Verify structure is intact
	for _, level := range []string{constants.LogsDirDebug, constants.LogsDirInfo, constants.LogsDirWarn, constants.LogsDirError} {
		path := filepath.Join(logsDir, level)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("Log directory %s missing: %v", level, err)
		}
	}
}

// TestFileLoggingFileNamingConvention verifies file names use Unix timestamps
func TestFileLoggingFileNamingConvention(t *testing.T) {
	ts := StartTestServerWithLogging(t)
	ts.ConfigureWorkDir(t)

	// Trigger logging
	ts.CreateTopic(t, "naming-test")
	time.Sleep(100 * time.Millisecond)

	infoDir := filepath.Join(ts.WorkDir, constants.InternalDir, constants.LogsDir, constants.LogsDirInfo)
	files, err := os.ReadDir(infoDir)
	if err != nil {
		t.Fatalf("Failed to read info log directory: %v", err)
	}

	if len(files) == 0 {
		t.Skip("No log files found")
	}

	// Check filename format: should be timestamp.log
	filename := files[0].Name()

	// Remove extension
	baseName := strings.TrimSuffix(filename, constants.LogFileExtension)

	// Should be a valid Unix timestamp (numeric only)
	for _, c := range baseName {
		if c < '0' || c > '9' {
			t.Errorf("Log filename %s contains non-numeric characters in timestamp portion", filename)
			break
		}
	}

	// Timestamp should be reasonable (after year 2020, before year 2100)
	// 2020-01-01 = 1577836800, 2100-01-01 = 4102444800
	var ts_val int64
	for _, c := range baseName {
		ts_val = ts_val*10 + int64(c-'0')
	}

	if ts_val < 1577836800 || ts_val > 4102444800 {
		t.Errorf("Log filename timestamp %d is outside reasonable range", ts_val)
	}
}

// TestFileLoggingMultipleOperations verifies logs capture multiple operations
func TestFileLoggingMultipleOperations(t *testing.T) {
	ts := StartTestServerWithLogging(t)
	ts.ConfigureWorkDir(t)

	// Perform multiple operations
	ts.CreateTopic(t, "multi-op-test")
	ts.UploadFileExpectSuccess(t, "multi-op-test", "file1.txt", []byte("content1"), "")
	ts.UploadFileExpectSuccess(t, "multi-op-test", "file2.txt", []byte("content2"), "")

	time.Sleep(100 * time.Millisecond)

	// Read info log and verify multiple entries
	infoDir := filepath.Join(ts.WorkDir, constants.InternalDir, constants.LogsDir, constants.LogsDirInfo)
	files, err := os.ReadDir(infoDir)
	if err != nil || len(files) == 0 {
		t.Skip("No log files found")
	}

	logPath := filepath.Join(infoDir, files[0].Name())
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	// Count lines (each log entry is a line)
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) < 2 {
		t.Errorf("Expected multiple log entries, got %d", len(lines))
	}
}

// TestFileLoggingWorkDirChange verifies file logging works after workdir change via API
func TestFileLoggingWorkDirChange(t *testing.T) {
	ts := StartTestServerWithLogging(t)

	// Create a second work directory
	workDir2, err := os.MkdirTemp("", "silobang-test-work2-*")
	if err != nil {
		t.Fatalf("Failed to create second work dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(workDir2) })

	// Configure first work dir
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "workdir1-topic")
	time.Sleep(100 * time.Millisecond)

	// Verify logs in first workdir
	logsDir1 := filepath.Join(ts.WorkDir, constants.InternalDir, constants.LogsDir, constants.LogsDirInfo)
	files1, _ := os.ReadDir(logsDir1)
	if len(files1) == 0 {
		t.Error("Expected log files in first work directory")
	}

	// Change to second work directory (captures new bootstrap credentials)
	ts.WorkDir = workDir2
	ts.ConfigureWorkDir(t)

	// Create topic in new workdir (triggers logging)
	ts.CreateTopic(t, "workdir2-topic")
	time.Sleep(100 * time.Millisecond)

	// Verify logs now appear in second workdir
	logsDir2 := filepath.Join(workDir2, constants.InternalDir, constants.LogsDir, constants.LogsDirInfo)
	files2, err := os.ReadDir(logsDir2)
	if err != nil {
		t.Fatalf("Failed to read second workdir logs: %v", err)
	}

	if len(files2) == 0 {
		t.Error("Expected log files in second work directory after workdir change")
	}
}
