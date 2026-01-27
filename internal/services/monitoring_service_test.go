package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"silobang/internal/config"
	"silobang/internal/constants"
	"silobang/internal/logger"
)

// newMonitoringMock creates a mockAppState pre-configured for monitoring tests.
func newMonitoringMock(workDir string) *mockAppState {
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
	m.startedAt = time.Now().Add(-10 * time.Second)
	return m
}

// =============================================================================
// Helper: create log directory structure with files
// =============================================================================

func setupLogDirs(t *testing.T, workDir string) {
	t.Helper()
	logsBase := filepath.Join(workDir, constants.InternalDir, constants.LogsDir)
	for _, level := range []string{constants.LogsDirDebug, constants.LogsDirInfo, constants.LogsDirWarn, constants.LogsDirError} {
		if err := os.MkdirAll(filepath.Join(logsBase, level), 0755); err != nil {
			t.Fatalf("Failed to create log dir %s: %v", level, err)
		}
	}
}

func writeLogFile(t *testing.T, workDir, level, filename, content string) {
	t.Helper()
	filePath := filepath.Join(workDir, constants.InternalDir, constants.LogsDir, level, filename)
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write log file: %v", err)
	}
}

// =============================================================================
// GetMonitoringInfo Tests
// =============================================================================

func TestGetMonitoringInfo_Success(t *testing.T) {
	tmpDir := t.TempDir()
	setupLogDirs(t, tmpDir)

	// Write some log files
	writeLogFile(t, tmpDir, constants.LogsDirError, "1700000000.log", "[ERROR] test error\n")
	writeLogFile(t, tmpDir, constants.LogsDirWarn, "1700000000.log", "[WARN] test warning\n")

	mock := newMonitoringMock(tmpDir)
	mock.RegisterTopic("topic-a", true, "")
	mock.RegisterTopic("topic-b", false, "unhealthy")

	svc := NewMonitoringService(mock, mock.log)
	info, err := svc.GetMonitoringInfo()
	if err != nil {
		t.Fatalf("GetMonitoringInfo failed: %v", err)
	}

	// System info
	if info.System.RAMUsedBytes == 0 {
		t.Error("Expected RAMUsedBytes > 0")
	}
	if info.System.ProjectDirSizeBytes == 0 {
		t.Error("Expected ProjectDirSizeBytes > 0 (test files exist in tmpDir)")
	}

	// Application info
	if info.Application.UptimeSeconds < 0 {
		t.Error("Expected non-negative UptimeSeconds")
	}
	if info.Application.StartedAt == 0 {
		t.Error("Expected non-zero StartedAt")
	}
	if info.Application.WorkingDirectory != tmpDir {
		t.Errorf("Expected WorkingDirectory=%s, got %s", tmpDir, info.Application.WorkingDirectory)
	}
	if info.Application.Port != constants.DefaultPort {
		t.Errorf("Expected Port=%d, got %d", constants.DefaultPort, info.Application.Port)
	}
	if info.Application.MaxDatSizeBytes != constants.DefaultMaxDatSize {
		t.Errorf("Expected MaxDatSizeBytes=%d, got %d", constants.DefaultMaxDatSize, info.Application.MaxDatSizeBytes)
	}
	if info.Application.MaxMetadataValueBytes != constants.MaxMetadataValueBytes {
		t.Errorf("Expected MaxMetadataValueBytes=%d, got %d", constants.MaxMetadataValueBytes, info.Application.MaxMetadataValueBytes)
	}
	if info.Application.TopicsTotal != 2 {
		t.Errorf("Expected TopicsTotal=2, got %d", info.Application.TopicsTotal)
	}
	if info.Application.TopicsHealthy != 1 {
		t.Errorf("Expected TopicsHealthy=1, got %d", info.Application.TopicsHealthy)
	}
	if info.Application.TopicsUnhealthy != 1 {
		t.Errorf("Expected TopicsUnhealthy=1, got %d", info.Application.TopicsUnhealthy)
	}

	// Logs summary
	if len(info.Logs.Levels) != 4 {
		t.Fatalf("Expected 4 log levels, got %d", len(info.Logs.Levels))
	}

	// Check error level has file details
	var errorLevel *LogLevelSummary
	for i := range info.Logs.Levels {
		if info.Logs.Levels[i].Level == constants.LogsDirError {
			errorLevel = &info.Logs.Levels[i]
			break
		}
	}
	if errorLevel == nil {
		t.Fatal("Expected error level in summary")
	}
	if errorLevel.FileCount != 1 {
		t.Errorf("Expected 1 error file, got %d", errorLevel.FileCount)
	}
	if errorLevel.TotalSize == 0 {
		t.Error("Expected non-zero TotalSize for error level")
	}
	if len(errorLevel.Files) != 1 {
		t.Errorf("Expected 1 file entry for error level, got %d", len(errorLevel.Files))
	}

	// Check warn level has file details
	var warnLevel *LogLevelSummary
	for i := range info.Logs.Levels {
		if info.Logs.Levels[i].Level == constants.LogsDirWarn {
			warnLevel = &info.Logs.Levels[i]
			break
		}
	}
	if warnLevel == nil {
		t.Fatal("Expected warn level in summary")
	}
	if len(warnLevel.Files) != 1 {
		t.Errorf("Expected 1 file entry for warn level, got %d", len(warnLevel.Files))
	}

	// Check debug/info levels do NOT have file details (even if files exist)
	var debugLevel *LogLevelSummary
	for i := range info.Logs.Levels {
		if info.Logs.Levels[i].Level == constants.LogsDirDebug {
			debugLevel = &info.Logs.Levels[i]
			break
		}
	}
	if debugLevel != nil && len(debugLevel.Files) > 0 {
		t.Error("Debug level should not expose file details")
	}
}

func TestGetMonitoringInfo_NotConfigured(t *testing.T) {
	mock := newMonitoringMock("")
	mock.cfg.WorkingDirectory = ""
	svc := NewMonitoringService(mock, mock.log)

	_, err := svc.GetMonitoringInfo()
	if err == nil {
		t.Fatal("Expected error when not configured")
	}

	code, ok := IsServiceError(err)
	if !ok {
		t.Fatal("Expected ServiceError")
	}
	if code != constants.ErrCodeNotConfigured {
		t.Errorf("Expected error code %s, got %s", constants.ErrCodeNotConfigured, code)
	}
}

// =============================================================================
// GetLogFileContent Tests
// =============================================================================

func TestGetLogFileContent_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	setupLogDirs(t, tmpDir)

	expectedContent := "[ERROR] 2024-01-15 10:30:00 | Something went wrong\n"
	writeLogFile(t, tmpDir, constants.LogsDirError, "1700000000.log", expectedContent)

	mock := newMonitoringMock(tmpDir)
	svc := NewMonitoringService(mock, mock.log)

	content, err := svc.GetLogFileContent(constants.LogsDirError, "1700000000.log")
	if err != nil {
		t.Fatalf("GetLogFileContent failed: %v", err)
	}

	if string(content) != expectedContent {
		t.Errorf("Expected content %q, got %q", expectedContent, string(content))
	}
}

func TestGetLogFileContent_DisallowedLevel_Debug(t *testing.T) {
	tmpDir := t.TempDir()
	setupLogDirs(t, tmpDir)
	writeLogFile(t, tmpDir, constants.LogsDirDebug, "1700000000.log", "debug content")

	mock := newMonitoringMock(tmpDir)
	svc := NewMonitoringService(mock, mock.log)

	_, err := svc.GetLogFileContent(constants.LogsDirDebug, "1700000000.log")
	if err == nil {
		t.Fatal("Expected error for disallowed debug level")
	}

	code, ok := IsServiceError(err)
	if !ok {
		t.Fatal("Expected ServiceError")
	}
	if code != constants.ErrCodeLogLevelNotAllowed {
		t.Errorf("Expected error code %s, got %s", constants.ErrCodeLogLevelNotAllowed, code)
	}
}

func TestGetLogFileContent_DisallowedLevel_Info(t *testing.T) {
	tmpDir := t.TempDir()
	setupLogDirs(t, tmpDir)
	writeLogFile(t, tmpDir, constants.LogsDirInfo, "1700000000.log", "info content")

	mock := newMonitoringMock(tmpDir)
	svc := NewMonitoringService(mock, mock.log)

	_, err := svc.GetLogFileContent(constants.LogsDirInfo, "1700000000.log")
	if err == nil {
		t.Fatal("Expected error for disallowed info level")
	}

	code, _ := IsServiceError(err)
	if code != constants.ErrCodeLogLevelNotAllowed {
		t.Errorf("Expected error code %s, got %s", constants.ErrCodeLogLevelNotAllowed, code)
	}
}

func TestGetLogFileContent_PathTraversal_DotDot(t *testing.T) {
	tmpDir := t.TempDir()
	setupLogDirs(t, tmpDir)

	mock := newMonitoringMock(tmpDir)
	svc := NewMonitoringService(mock, mock.log)

	_, err := svc.GetLogFileContent(constants.LogsDirError, "../../etc/passwd")
	if err == nil {
		t.Fatal("Expected error for path traversal with ..")
	}

	code, ok := IsServiceError(err)
	if !ok {
		t.Fatal("Expected ServiceError")
	}
	if code != constants.ErrCodeInvalidRequest {
		t.Errorf("Expected error code %s, got %s", constants.ErrCodeInvalidRequest, code)
	}
}

func TestGetLogFileContent_PathTraversal_Slash(t *testing.T) {
	tmpDir := t.TempDir()
	setupLogDirs(t, tmpDir)

	mock := newMonitoringMock(tmpDir)
	svc := NewMonitoringService(mock, mock.log)

	_, err := svc.GetLogFileContent(constants.LogsDirError, "/etc/passwd")
	if err == nil {
		t.Fatal("Expected error for path traversal with /")
	}

	code, _ := IsServiceError(err)
	if code != constants.ErrCodeInvalidRequest {
		t.Errorf("Expected error code %s, got %s", constants.ErrCodeInvalidRequest, code)
	}
}

func TestGetLogFileContent_PathTraversal_Backslash(t *testing.T) {
	tmpDir := t.TempDir()
	setupLogDirs(t, tmpDir)

	mock := newMonitoringMock(tmpDir)
	svc := NewMonitoringService(mock, mock.log)

	_, err := svc.GetLogFileContent(constants.LogsDirError, "..\\etc\\passwd")
	if err == nil {
		t.Fatal("Expected error for path traversal with backslash")
	}

	code, _ := IsServiceError(err)
	if code != constants.ErrCodeInvalidRequest {
		t.Errorf("Expected error code %s, got %s", constants.ErrCodeInvalidRequest, code)
	}
}

func TestGetLogFileContent_NonLogExtension(t *testing.T) {
	tmpDir := t.TempDir()
	setupLogDirs(t, tmpDir)

	mock := newMonitoringMock(tmpDir)
	svc := NewMonitoringService(mock, mock.log)

	testCases := []string{"test.txt", "test.exe", "test", "test.log.bak"}
	for _, filename := range testCases {
		t.Run(filename, func(t *testing.T) {
			_, err := svc.GetLogFileContent(constants.LogsDirError, filename)
			if err == nil {
				t.Fatalf("Expected error for filename %q", filename)
			}
			code, _ := IsServiceError(err)
			if code != constants.ErrCodeInvalidRequest {
				t.Errorf("Expected error code %s, got %s", constants.ErrCodeInvalidRequest, code)
			}
		})
	}
}

func TestGetLogFileContent_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	setupLogDirs(t, tmpDir)

	mock := newMonitoringMock(tmpDir)
	svc := NewMonitoringService(mock, mock.log)

	_, err := svc.GetLogFileContent(constants.LogsDirError, "9999999999.log")
	if err == nil {
		t.Fatal("Expected error for non-existent file")
	}

	code, ok := IsServiceError(err)
	if !ok {
		t.Fatal("Expected ServiceError")
	}
	if code != constants.ErrCodeLogFileNotFound {
		t.Errorf("Expected error code %s, got %s", constants.ErrCodeLogFileNotFound, code)
	}
}

func TestGetLogFileContent_SizeCap(t *testing.T) {
	tmpDir := t.TempDir()
	setupLogDirs(t, tmpDir)

	// Create a file larger than MonitoringLogFileMaxReadBytes
	largeContent := strings.Repeat("A", constants.MonitoringLogFileMaxReadBytes+1000)
	writeLogFile(t, tmpDir, constants.LogsDirError, "large.log", largeContent)

	mock := newMonitoringMock(tmpDir)
	svc := NewMonitoringService(mock, mock.log)

	content, err := svc.GetLogFileContent(constants.LogsDirError, "large.log")
	if err != nil {
		t.Fatalf("GetLogFileContent failed: %v", err)
	}

	if len(content) != constants.MonitoringLogFileMaxReadBytes {
		t.Errorf("Expected content capped at %d bytes, got %d",
			constants.MonitoringLogFileMaxReadBytes, len(content))
	}
}

func TestGetLogFileContent_NotConfigured(t *testing.T) {
	mock := newMonitoringMock("")
	mock.cfg.WorkingDirectory = ""
	svc := NewMonitoringService(mock, mock.log)

	_, err := svc.GetLogFileContent(constants.LogsDirError, "test.log")
	if err == nil {
		t.Fatal("Expected error when not configured")
	}

	code, _ := IsServiceError(err)
	if code != constants.ErrCodeNotConfigured {
		t.Errorf("Expected error code %s, got %s", constants.ErrCodeNotConfigured, code)
	}
}

func TestGetLogFileContent_WarnLevel(t *testing.T) {
	tmpDir := t.TempDir()
	setupLogDirs(t, tmpDir)

	expectedContent := "[WARN] warning message\n"
	writeLogFile(t, tmpDir, constants.LogsDirWarn, "1700000000.log", expectedContent)

	mock := newMonitoringMock(tmpDir)
	svc := NewMonitoringService(mock, mock.log)

	content, err := svc.GetLogFileContent(constants.LogsDirWarn, "1700000000.log")
	if err != nil {
		t.Fatalf("GetLogFileContent failed for warn level: %v", err)
	}

	if string(content) != expectedContent {
		t.Errorf("Expected content %q, got %q", expectedContent, string(content))
	}
}

// =============================================================================
// isAllowedLogLevel Tests
// =============================================================================

func TestIsAllowedLogLevel(t *testing.T) {
	tests := []struct {
		level   string
		allowed bool
	}{
		{constants.LogsDirWarn, true},
		{constants.LogsDirError, true},
		{constants.LogsDirDebug, false},
		{constants.LogsDirInfo, false},
		{"", false},
		{"critical", false},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			got := isAllowedLogLevel(tt.level)
			if got != tt.allowed {
				t.Errorf("isAllowedLogLevel(%q) = %v, want %v", tt.level, got, tt.allowed)
			}
		})
	}
}

// =============================================================================
// calculateDirSize Tests
// =============================================================================

func TestCalculateDirSize(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files with known sizes
	if err := os.WriteFile(filepath.Join(tmpDir, "file1.bin"), make([]byte, 1000), 0644); err != nil {
		t.Fatalf("Failed to write file1: %v", err)
	}
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "file2.bin"), make([]byte, 2000), 0644); err != nil {
		t.Fatalf("Failed to write file2: %v", err)
	}

	mock := newMonitoringMock(tmpDir)
	svc := NewMonitoringService(mock, mock.log)

	size := svc.calculateDirSize(tmpDir)

	if size != 3000 {
		t.Errorf("Expected directory size 3000, got %d", size)
	}
}

func TestCalculateDirSize_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	mock := newMonitoringMock(tmpDir)
	svc := NewMonitoringService(mock, mock.log)

	size := svc.calculateDirSize(tmpDir)

	if size != 0 {
		t.Errorf("Expected directory size 0 for empty dir, got %d", size)
	}
}

func TestCalculateDirSize_NonExistentDir(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistent := filepath.Join(tmpDir, "does-not-exist")

	mock := newMonitoringMock(tmpDir)
	svc := NewMonitoringService(mock, mock.log)

	size := svc.calculateDirSize(nonExistent)

	if size != 0 {
		t.Errorf("Expected directory size 0 for non-existent dir, got %d", size)
	}
}

// =============================================================================
// Regression: No runtime field in JSON
// =============================================================================

func TestGetMonitoringInfo_NoRuntimeField(t *testing.T) {
	tmpDir := t.TempDir()
	setupLogDirs(t, tmpDir)

	mock := newMonitoringMock(tmpDir)
	svc := NewMonitoringService(mock, mock.log)

	info, err := svc.GetMonitoringInfo()
	if err != nil {
		t.Fatalf("GetMonitoringInfo failed: %v", err)
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Failed to unmarshal to map: %v", err)
	}

	if _, exists := raw["runtime"]; exists {
		t.Error("MonitoringInfo should not contain 'runtime' field")
	}
}
