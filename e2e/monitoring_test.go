package e2e

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"silobang/internal/constants"
)

// =============================================================================
// GET /api/monitoring — Basic Endpoint Tests
// =============================================================================

// TestMonitoringEndpoint_NotConfigured verifies monitoring returns 400 before workdir config
func TestMonitoringEndpoint_NotConfigured(t *testing.T) {
	ts := StartTestServer(t)
	// Without configuration, auth store doesn't exist so request returns 401

	resp, err := ts.UnauthenticatedGET("/api/monitoring")
	if err != nil {
		t.Fatalf("monitoring request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected status %d (auth required before config), got %d", http.StatusUnauthorized, resp.StatusCode)
	}
}

// TestMonitoringEndpoint_MethodNotAllowed verifies POST is rejected
func TestMonitoringEndpoint_MethodNotAllowed(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	resp, err := ts.POST("/api/monitoring", nil)
	if err != nil {
		t.Fatalf("monitoring POST request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, resp.StatusCode)
	}
}

// TestMonitoringEndpoint_BasicInfo verifies basic monitoring data is correct
func TestMonitoringEndpoint_BasicInfo(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	mon := ts.GetMonitoring(t)

	// System info
	if mon.System.RAMUsedBytes == 0 {
		t.Error("Expected RAMUsedBytes > 0")
	}

	// Application info
	if mon.Application.UptimeSeconds < 0 {
		t.Error("Expected non-negative UptimeSeconds")
	}
	if mon.Application.StartedAt == 0 {
		t.Error("Expected non-zero StartedAt")
	}
	if mon.Application.WorkingDirectory != ts.WorkDir {
		t.Errorf("Expected WorkingDirectory=%s, got %s", ts.WorkDir, mon.Application.WorkingDirectory)
	}
	if mon.Application.MaxDatSizeBytes != constants.DefaultMaxDatSize {
		t.Errorf("Expected MaxDatSizeBytes=%d, got %d", constants.DefaultMaxDatSize, mon.Application.MaxDatSizeBytes)
	}
	if mon.Application.MaxMetadataValueBytes != ts.App.Config.Metadata.MaxValueBytes {
		t.Errorf("Expected MaxMetadataValueBytes=%d, got %d", ts.App.Config.Metadata.MaxValueBytes, mon.Application.MaxMetadataValueBytes)
	}

	// Logs summary should have 4 levels
	if len(mon.Logs.Levels) != 4 {
		t.Errorf("Expected 4 log levels, got %d", len(mon.Logs.Levels))
	}
}

// =============================================================================
// GET /api/monitoring — Topic Counts
// =============================================================================

// TestMonitoringEndpoint_WithTopics verifies topic counts are accurate
func TestMonitoringEndpoint_WithTopics(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Create topics
	ts.CreateTopic(t, "mon-topic-a")
	ts.CreateTopic(t, "mon-topic-b")
	ts.CreateTopic(t, "mon-topic-c")

	mon := ts.GetMonitoring(t)

	if mon.Application.TopicsTotal != 3 {
		t.Errorf("Expected TopicsTotal=3, got %d", mon.Application.TopicsTotal)
	}
	if mon.Application.TopicsHealthy != 3 {
		t.Errorf("Expected TopicsHealthy=3, got %d", mon.Application.TopicsHealthy)
	}
	if mon.Application.TopicsUnhealthy != 0 {
		t.Errorf("Expected TopicsUnhealthy=0, got %d", mon.Application.TopicsUnhealthy)
	}
}

// TestMonitoringEndpoint_IndexedHashes verifies hash count after uploads
func TestMonitoringEndpoint_IndexedHashes(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "mon-hashes")

	// Upload 3 distinct files
	ts.UploadFileExpectSuccess(t, "mon-hashes", "file1.bin", []byte("content-one"), "")
	ts.UploadFileExpectSuccess(t, "mon-hashes", "file2.bin", []byte("content-two"), "")
	ts.UploadFileExpectSuccess(t, "mon-hashes", "file3.bin", []byte("content-three"), "")

	mon := ts.GetMonitoring(t)

	if mon.Application.TotalIndexedHashes < 3 {
		t.Errorf("Expected TotalIndexedHashes >= 3, got %d", mon.Application.TotalIndexedHashes)
	}
}

// =============================================================================
// GET /api/monitoring — Log Files Summary
// =============================================================================

// TestMonitoringEndpoint_LogFiles verifies log file summary with file logging enabled
func TestMonitoringEndpoint_LogFiles(t *testing.T) {
	ts := StartTestServerWithLogging(t)
	ts.ConfigureWorkDir(t)

	// Trigger some actions to generate log entries
	ts.CreateTopic(t, "log-mon-test")
	ts.UploadFileExpectSuccess(t, "log-mon-test", "test.bin", []byte("test-content"), "")

	// Allow time for log flush
	time.Sleep(200 * time.Millisecond)

	mon := ts.GetMonitoring(t)

	// Verify info level has files (topic creation generates info logs)
	var infoLevel *MonitoringLogLevel
	for i := range mon.Logs.Levels {
		if mon.Logs.Levels[i].Level == constants.LogsDirInfo {
			infoLevel = &mon.Logs.Levels[i]
			break
		}
	}
	if infoLevel == nil {
		t.Fatal("Expected info level in log summary")
	}
	if infoLevel.FileCount == 0 {
		t.Error("Expected at least one info log file")
	}

	// Info/debug levels should NOT expose individual file details (security)
	if len(infoLevel.Files) > 0 {
		t.Error("Info level should not expose individual file details via API")
	}
}

// =============================================================================
// GET /api/monitoring/logs/:level/:filename — Log File Content
// =============================================================================

// TestMonitoringLogFile_ReadContent verifies reading a valid warn/error log file
func TestMonitoringLogFile_ReadContent(t *testing.T) {
	ts := StartTestServerWithLogging(t)
	ts.ConfigureWorkDir(t)

	// Write a known error log file directly
	errorDir := filepath.Join(ts.WorkDir, constants.InternalDir, constants.LogsDir, constants.LogsDirError)
	os.MkdirAll(errorDir, 0755)
	testContent := "[ERROR] 2024-01-15 10:30:00 | Test error message\n"
	testFilename := "1700000000.log"
	os.WriteFile(filepath.Join(errorDir, testFilename), []byte(testContent), 0644)

	content, status := ts.GetMonitoringLogFile(t, constants.LogsDirError, testFilename)

	if status != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", status, content)
	}
	if content != testContent {
		t.Errorf("Expected content %q, got %q", testContent, content)
	}
}

// TestMonitoringLogFile_WarnLevel verifies warn level files are accessible
func TestMonitoringLogFile_WarnLevel(t *testing.T) {
	ts := StartTestServerWithLogging(t)
	ts.ConfigureWorkDir(t)

	// Write a known warn log file
	warnDir := filepath.Join(ts.WorkDir, constants.InternalDir, constants.LogsDir, constants.LogsDirWarn)
	os.MkdirAll(warnDir, 0755)
	testContent := "[WARN] 2024-01-15 10:30:00 | Test warning\n"
	testFilename := "1700000000.log"
	os.WriteFile(filepath.Join(warnDir, testFilename), []byte(testContent), 0644)

	content, status := ts.GetMonitoringLogFile(t, constants.LogsDirWarn, testFilename)

	if status != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", status, content)
	}
	if content != testContent {
		t.Errorf("Expected content %q, got %q", testContent, content)
	}
}

// =============================================================================
// Security Tests — Path Traversal & Access Control
// =============================================================================

// TestMonitoringLogFile_PathTraversal_DotDot verifies .. in filename is rejected.
// Note: Go's HTTP mux may clean path traversal before it reaches the handler,
// resulting in 404 (path not matched) instead of 400 (handler validation).
// Both are acceptable — the traversal attempt is rejected either way.
func TestMonitoringLogFile_PathTraversal_DotDot(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	_, status := ts.GetMonitoringLogFile(t, constants.LogsDirError, "../../etc/passwd")

	if status != http.StatusBadRequest && status != http.StatusNotFound {
		t.Errorf("Expected status 400 or 404 for path traversal with .., got %d", status)
	}
}

// TestMonitoringLogFile_PathTraversal_Slash verifies / in filename is rejected
func TestMonitoringLogFile_PathTraversal_Slash(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	resp, err := ts.GET("/api/monitoring/logs/" + constants.LogsDirError + "//etc/passwd")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Either 400 or 404 is acceptable for path traversal attempts
	if resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 400 or 404 for path traversal with /, got %d", resp.StatusCode)
	}
}

// TestMonitoringLogFile_PathTraversal_Backslash verifies backslash in filename is rejected
func TestMonitoringLogFile_PathTraversal_Backslash(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	_, status := ts.GetMonitoringLogFile(t, constants.LogsDirError, "..\\etc\\passwd")

	if status != http.StatusBadRequest {
		t.Errorf("Expected status %d for path traversal with backslash, got %d", http.StatusBadRequest, status)
	}
}

// TestMonitoringLogFile_DisallowedLevel_Debug verifies debug log access is forbidden
func TestMonitoringLogFile_DisallowedLevel_Debug(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	_, status := ts.GetMonitoringLogFile(t, constants.LogsDirDebug, "1700000000.log")

	if status != http.StatusForbidden {
		t.Errorf("Expected status %d for disallowed debug level, got %d", http.StatusForbidden, status)
	}
}

// TestMonitoringLogFile_DisallowedLevel_Info verifies info log access is forbidden
func TestMonitoringLogFile_DisallowedLevel_Info(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	_, status := ts.GetMonitoringLogFile(t, constants.LogsDirInfo, "1700000000.log")

	if status != http.StatusForbidden {
		t.Errorf("Expected status %d for disallowed info level, got %d", http.StatusForbidden, status)
	}
}

// TestMonitoringLogFile_NonLogExtension verifies non-.log files are rejected
func TestMonitoringLogFile_NonLogExtension(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	badFilenames := []string{"test.txt", "test.exe", "test", "test.log.bak"}
	for _, filename := range badFilenames {
		t.Run(filename, func(t *testing.T) {
			_, status := ts.GetMonitoringLogFile(t, constants.LogsDirError, filename)
			if status != http.StatusBadRequest {
				t.Errorf("Expected status %d for filename %q, got %d", http.StatusBadRequest, filename, status)
			}
		})
	}
}

// TestMonitoringLogFile_FileNotFound verifies 404 for non-existent log files
func TestMonitoringLogFile_FileNotFound(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	content, status := ts.GetMonitoringLogFile(t, constants.LogsDirError, "9999999999.log")

	if status != http.StatusNotFound {
		t.Errorf("Expected status %d for non-existent file, got %d: %s", http.StatusNotFound, status, content)
	}
}

// TestMonitoringLogFile_NotConfigured verifies log file access before workdir config
// With auth enabled, unconfigured system returns 401 (no auth store available)
func TestMonitoringLogFile_NotConfigured(t *testing.T) {
	ts := StartTestServer(t)

	resp, err := ts.GET("/api/monitoring/logs/" + constants.LogsDirError + "/test.log")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected status %d before config, got %d", http.StatusUnauthorized, resp.StatusCode)
	}
}

// TestMonitoringLogFile_MethodNotAllowed verifies POST to log file is rejected
func TestMonitoringLogFile_MethodNotAllowed(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	resp, err := ts.POST("/api/monitoring/logs/"+constants.LogsDirError+"/test.log", nil)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, resp.StatusCode)
	}
}

// =============================================================================
// Advanced Security Tests
// =============================================================================

// TestMonitoringLogFile_PathTraversal_EncodedDotDot tests URL-encoded traversal
func TestMonitoringLogFile_PathTraversal_EncodedDotDot(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Try URL-encoded ../ — Go's HTTP mux should normalize this
	resp, err := http.Get(ts.URL + "/api/monitoring/logs/" + constants.LogsDirError + "/%2e%2e%2fetc%2fpasswd")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Any non-200 status is acceptable for traversal attempts
	if resp.StatusCode == http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("Path traversal should not succeed, got 200 with body: %s", string(body))
	}
}

// TestMonitoringLogFile_EmptyLevelOrFilename verifies missing params are rejected
func TestMonitoringLogFile_EmptyLevelOrFilename(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Missing filename
	resp, err := ts.GET("/api/monitoring/logs/" + constants.LogsDirError + "/")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		t.Error("Expected error for empty filename, got 200")
	}

	// Missing level and filename
	resp, err = ts.GET("/api/monitoring/logs/")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		t.Error("Expected error for empty level/filename, got 200")
	}
}

// =============================================================================
// Content Validation
// =============================================================================

// TestMonitoringLogFile_ContentType verifies the response content type is text/plain
func TestMonitoringLogFile_ContentType(t *testing.T) {
	ts := StartTestServerWithLogging(t)
	ts.ConfigureWorkDir(t)

	// Write a test log file
	errorDir := filepath.Join(ts.WorkDir, constants.InternalDir, constants.LogsDir, constants.LogsDirError)
	os.MkdirAll(errorDir, 0755)
	os.WriteFile(filepath.Join(errorDir, "1700000000.log"), []byte("[ERROR] test\n"), 0644)

	resp, err := ts.GET("/api/monitoring/logs/" + constants.LogsDirError + "/1700000000.log")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Expected Content-Type text/plain, got %s", ct)
	}
}

// TestMonitoringEndpoint_ContentType verifies monitoring response is JSON
func TestMonitoringEndpoint_ContentType(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	resp, err := ts.GET("/api/monitoring")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Expected Content-Type application/json, got %s", ct)
	}
}

// TestMonitoringEndpoint_ProjectDirSize verifies project directory size is populated
func TestMonitoringEndpoint_ProjectDirSize(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Create a topic and upload a file to ensure the working directory has content
	ts.CreateTopic(t, "dirsize-test")
	ts.UploadFileExpectSuccess(t, "dirsize-test", "test.bin", []byte("test-content-for-size"), "")

	mon := ts.GetMonitoring(t)

	if mon.System.ProjectDirSizeBytes == 0 {
		t.Error("Expected ProjectDirSizeBytes > 0 after creating topic and uploading file")
	}
}

// TestMonitoringEndpoint_NoRuntimeField verifies the runtime section is not in the response
func TestMonitoringEndpoint_NoRuntimeField(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	resp, err := ts.GET("/api/monitoring")
	if err != nil {
		t.Fatalf("monitoring request failed: %v", err)
	}
	defer resp.Body.Close()

	var raw map[string]json.RawMessage
	json.NewDecoder(resp.Body).Decode(&raw)

	if _, exists := raw["runtime"]; exists {
		t.Error("API response should not contain 'runtime' field")
	}
}

// TestMonitoringEndpoint_UptimeProgression verifies uptime increases over time
func TestMonitoringEndpoint_UptimeProgression(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	mon1 := ts.GetMonitoring(t)
	time.Sleep(1100 * time.Millisecond)
	mon2 := ts.GetMonitoring(t)

	if mon2.Application.UptimeSeconds <= mon1.Application.UptimeSeconds {
		t.Errorf("Expected uptime to increase: first=%d, second=%d",
			mon1.Application.UptimeSeconds, mon2.Application.UptimeSeconds)
	}
}
