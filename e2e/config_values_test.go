package e2e

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"silobang/internal/constants"
)

// =============================================================================
// Helper: Start test server with custom config overrides
// =============================================================================

// startTestServerCustomConfig creates a test server then applies config customizations.
// The customize function runs BEFORE ConfigureWorkDir, so services are
// initialized with the custom values.
func startTestServerCustomConfig(t *testing.T, customize func(ts *TestServer)) *TestServer {
	t.Helper()
	ts := StartTestServer(t)
	customize(ts)
	ts.ConfigureWorkDir(t)
	return ts
}

// =============================================================================
// Auth Config: Custom max_login_attempts
// =============================================================================

// TestConfigCustomAuth_LoginLockout verifies that reducing max_login_attempts
// causes earlier lockout than the default.
func TestConfigCustomAuth_LoginLockout(t *testing.T) {
	ts := startTestServerCustomConfig(t, func(ts *TestServer) {
		ts.App.Config.Auth.MaxLoginAttempts = 2
	})

	user := ts.CreateTestUser(t, "lockcfg", "correct-password-12345")

	// Send exactly 2 wrong passwords (the custom limit)
	for i := 0; i < 2; i++ {
		resp, err := ts.UnauthenticatedPOST("/api/auth/login", map[string]string{
			"username": user.Username,
			"password": fmt.Sprintf("wrong-password-%d", i),
		})
		if err != nil {
			t.Fatalf("login attempt %d failed: %v", i, err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected 401, got %d", i, resp.StatusCode)
		}
	}

	// Now try with correct password — should be locked (429)
	resp, err := ts.UnauthenticatedPOST("/api/auth/login", map[string]string{
		"username": user.Username,
		"password": user.Password,
	})
	if err != nil {
		t.Fatalf("locked login request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected 429 for locked account after 2 attempts, got %d", resp.StatusCode)
	}
}

// =============================================================================
// Batch Config: Custom max_operations
// =============================================================================

// TestConfigCustomBatch_MaxOperations verifies that a custom batch.max_operations
// limit is enforced. Sets limit to 3, then sends 4 operations expecting rejection.
func TestConfigCustomBatch_MaxOperations(t *testing.T) {
	ts := startTestServerCustomConfig(t, func(ts *TestServer) {
		ts.App.Config.Batch.MaxOperations = 3
	})

	ts.CreateTopic(t, "batch-cfg-test")
	upload := ts.UploadFileExpectSuccess(t, "batch-cfg-test", "test.bin", []byte("content"), "")

	// Build 4 operations (one more than the limit)
	operations := make([]BatchMetadataOperation, 4)
	for i := 0; i < 4; i++ {
		operations[i] = BatchMetadataOperation{
			Hash:  upload.Hash,
			Op:    "set",
			Key:   fmt.Sprintf("key%d", i),
			Value: "value",
		}
	}

	batchReq := BatchMetadataRequest{
		Operations:       operations,
		Processor:        "test",
		ProcessorVersion: "1.0",
	}

	errResp := ts.BatchSetMetadataExpectError(t, batchReq, http.StatusBadRequest)
	if errResp.Code != constants.ErrCodeBatchTooManyOperations {
		t.Errorf("expected error code %s, got %s", constants.ErrCodeBatchTooManyOperations, errResp.Code)
	}

	// Verify 3 operations (exactly at limit) succeed
	batchReq.Operations = operations[:3]
	resp := ts.BatchSetMetadata(t, batchReq)
	if !resp.Success {
		t.Errorf("batch with %d operations (at limit) should succeed", 3)
	}
	if resp.Succeeded != 3 {
		t.Errorf("expected succeeded=3, got %d", resp.Succeeded)
	}
}

// =============================================================================
// Metadata Config: Custom max_value_bytes
// =============================================================================

// TestConfigCustomMetadata_MaxValueBytes verifies that a custom
// metadata.max_value_bytes limit is enforced. Sets a small limit (100 bytes),
// then verifies oversized values are rejected and values at the limit succeed.
func TestConfigCustomMetadata_MaxValueBytes(t *testing.T) {
	ts := startTestServerCustomConfig(t, func(ts *TestServer) {
		ts.App.Config.Metadata.MaxValueBytes = 100
	})

	ts.CreateTopic(t, "meta-cfg-test")
	upload := ts.UploadFileExpectSuccess(t, "meta-cfg-test", "test.bin", []byte("content"), "")

	// Value at exactly the limit (100 bytes) should succeed
	exactValue := strings.Repeat("x", 100)
	ts.SetMetadata(t, upload.Hash, "exact-key", exactValue)

	// Verify it was stored
	meta := ts.GetAssetMetadata(t, upload.Hash)
	computed, ok := meta["computed_metadata"].(map[string]interface{})
	if !ok {
		t.Fatal("computed_metadata not found")
	}
	if computed["exact-key"] != exactValue {
		t.Error("value at limit should be stored correctly")
	}

	// Value exceeding limit (101 bytes) should fail
	oversizedValue := strings.Repeat("x", 101)
	errResp := ts.SetMetadataExpectError(t, upload.Hash, "over-key", oversizedValue, http.StatusBadRequest)
	if errResp.Code != constants.ErrCodeMetadataValueTooLong {
		t.Errorf("expected error code %s, got %s", constants.ErrCodeMetadataValueTooLong, errResp.Code)
	}
}

// =============================================================================
// Monitoring Config: Custom log_file_max_read_bytes
// =============================================================================

// TestConfigCustomMonitoring_LogMaxRead verifies that a custom
// monitoring.log_file_max_read_bytes limit truncates large log files.
func TestConfigCustomMonitoring_LogMaxRead(t *testing.T) {
	ts := startTestServerCustomConfig(t, func(ts *TestServer) {
		ts.App.Config.Monitoring.LogFileMaxReadBytes = 2048 // 2KB
	})

	// Write a large error log file (4KB — double the limit)
	errorDir := filepath.Join(ts.WorkDir, constants.InternalDir, constants.LogsDir, constants.LogsDirError)
	os.MkdirAll(errorDir, 0755)

	largeContent := strings.Repeat("[ERROR] line\n", 400) // ~5.2KB
	testFilename := "1700000000.log"
	os.WriteFile(filepath.Join(errorDir, testFilename), []byte(largeContent), 0644)

	content, status := ts.GetMonitoringLogFile(t, constants.LogsDirError, testFilename)

	if status != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", status, content)
	}

	// The response should be truncated to the configured limit
	if len(content) > 2048 {
		t.Errorf("response should be truncated to 2048 bytes, got %d bytes", len(content))
	}

	// It should contain some content (not empty)
	if len(content) == 0 {
		t.Error("truncated response should not be empty")
	}
}

// =============================================================================
// Monitoring Config: Small log file not truncated
// =============================================================================

// TestConfigCustomMonitoring_SmallFileNotTruncated verifies that a file within
// the read limit is returned fully.
func TestConfigCustomMonitoring_SmallFileNotTruncated(t *testing.T) {
	ts := startTestServerCustomConfig(t, func(ts *TestServer) {
		ts.App.Config.Monitoring.LogFileMaxReadBytes = 4096 // 4KB
	})

	// Write a small error log file (under limit)
	errorDir := filepath.Join(ts.WorkDir, constants.InternalDir, constants.LogsDir, constants.LogsDirError)
	os.MkdirAll(errorDir, 0755)

	smallContent := "[ERROR] 2024-01-15 10:30:00 | Test error message\n"
	testFilename := "1700000001.log"
	os.WriteFile(filepath.Join(errorDir, testFilename), []byte(smallContent), 0644)

	content, status := ts.GetMonitoringLogFile(t, constants.LogsDirError, testFilename)

	if status != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", status, content)
	}

	// The response should match the full content
	if content != smallContent {
		t.Errorf("small file should be returned fully, got %d bytes instead of %d", len(content), len(smallContent))
	}
}
