package e2e

import (
	"encoding/json"
	"net/http"
	"testing"

	"silobang/internal/constants"
)

// =============================================================================
// Disk Limit — Upload Rejection
// =============================================================================

// TestDiskLimit_UploadRejectedWhenExceeded sets a 1-byte disk limit and verifies
// that asset upload returns HTTP 507 (Insufficient Storage).
func TestDiskLimit_UploadRejectedWhenExceeded(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "disk-limit-upload")

	// Set an impossibly low disk limit (1 byte — always exceeded)
	ts.App.Config.MaxDiskUsage = 1

	errResp := ts.UploadFileExpectError(t, "disk-limit-upload", "test.bin", []byte("test-content"), "", http.StatusInsufficientStorage)
	if errResp.Code != constants.ErrCodeDiskLimitExceeded {
		t.Errorf("Expected error code %s, got %s", constants.ErrCodeDiskLimitExceeded, errResp.Code)
	}
}

// =============================================================================
// Disk Limit — Topic Creation Rejection
// =============================================================================

// TestDiskLimit_TopicCreationRejectedWhenExceeded sets a 1-byte disk limit and verifies
// that topic creation returns HTTP 507.
func TestDiskLimit_TopicCreationRejectedWhenExceeded(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Set an impossibly low disk limit
	ts.App.Config.MaxDiskUsage = 1

	resp, err := ts.POST("/api/topics", map[string]interface{}{
		"name": "disk-limit-topic",
	})
	if err != nil {
		t.Fatalf("POST /api/topics failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInsufficientStorage {
		t.Errorf("Expected status %d (Insufficient Storage), got %d", http.StatusInsufficientStorage, resp.StatusCode)
	}

	var errResp ErrorResponse
	json.NewDecoder(resp.Body).Decode(&errResp)
	if errResp.Code != constants.ErrCodeDiskLimitExceeded {
		t.Errorf("Expected error code %s, got %s", constants.ErrCodeDiskLimitExceeded, errResp.Code)
	}
}

// =============================================================================
// Disk Limit — Metadata Rejection
// =============================================================================

// TestDiskLimit_MetadataSetRejectedWhenExceeded sets a 1-byte disk limit and verifies
// that metadata set operations return HTTP 507.
func TestDiskLimit_MetadataSetRejectedWhenExceeded(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "disk-limit-meta")

	// Upload a file first (before setting the limit)
	result := ts.UploadFileExpectSuccess(t, "disk-limit-meta", "test.bin", []byte("test-content"), "")

	// Now set the disk limit
	ts.App.Config.MaxDiskUsage = 1

	// Try to set metadata
	errResp := ts.SetMetadataExpectError(t, result.Hash, "test-key", "test-value", http.StatusInsufficientStorage)
	if errResp.Code != constants.ErrCodeDiskLimitExceeded {
		t.Errorf("Expected error code %s, got %s", constants.ErrCodeDiskLimitExceeded, errResp.Code)
	}
}

// TestDiskLimit_MetadataDeleteAllowedWhenExceeded verifies that metadata delete
// operations are still allowed even when disk limit is exceeded (deletes free space).
func TestDiskLimit_MetadataDeleteAllowedWhenExceeded(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "disk-limit-del")

	// Upload and set metadata first (before limit)
	result := ts.UploadFileExpectSuccess(t, "disk-limit-del", "test.bin", []byte("test-content"), "")
	ts.SetMetadata(t, result.Hash, "del-key", "del-value")

	// Now set the disk limit
	ts.App.Config.MaxDiskUsage = 1

	// Delete should still work (it frees space, not consumes it)
	resp, err := ts.POST("/api/assets/"+result.Hash+"/metadata", map[string]interface{}{
		"op":                "delete",
		"key":               "del-key",
		"processor":         "test",
		"processor_version": "1.0",
	})
	if err != nil {
		t.Fatalf("POST metadata delete failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d for delete op (should bypass disk limit), got %d", http.StatusOK, resp.StatusCode)
	}
}

// =============================================================================
// Disk Limit — Zero Means Unlimited
// =============================================================================

// TestDiskLimit_ZeroMeansUnlimited verifies that MaxDiskUsage=0 allows all operations.
func TestDiskLimit_ZeroMeansUnlimited(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "disk-limit-zero")

	// Explicitly ensure limit is 0 (unlimited)
	ts.App.Config.MaxDiskUsage = 0

	// Upload should succeed
	result := ts.UploadFileExpectSuccess(t, "disk-limit-zero", "test.bin", []byte("test-content"), "")
	if result.Hash == "" {
		t.Error("Expected successful upload with zero (unlimited) disk limit")
	}

	// Metadata should succeed
	ts.SetMetadata(t, result.Hash, "test-key", "test-value")

	// Topic creation should succeed
	ts.CreateTopic(t, "disk-limit-zero-2")
}

// =============================================================================
// Disk Limit — Batch Metadata Rejection
// =============================================================================

// TestDiskLimit_BatchMetadataRejectedWhenExceeded verifies batch metadata set
// operations are rejected when disk limit is exceeded.
func TestDiskLimit_BatchMetadataRejectedWhenExceeded(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "disk-limit-batch")

	// Upload a file first
	result := ts.UploadFileExpectSuccess(t, "disk-limit-batch", "test.bin", []byte("test-content"), "")

	// Now set the disk limit
	ts.App.Config.MaxDiskUsage = 1

	// Try batch set
	errResp := ts.BatchSetMetadataExpectError(t, BatchMetadataRequest{
		Operations: []BatchMetadataOperation{
			{
				Hash:  result.Hash,
				Op:    "set",
				Key:   "batch-key",
				Value: "batch-value",
			},
		},
	}, http.StatusInsufficientStorage)

	if errResp.Code != constants.ErrCodeDiskLimitExceeded {
		t.Errorf("Expected error code %s, got %s", constants.ErrCodeDiskLimitExceeded, errResp.Code)
	}
}

// =============================================================================
// Disk Limit — Monitoring Exposes Limit
// =============================================================================

// TestDiskLimit_MonitoringShowsLimit verifies the monitoring endpoint exposes
// the configured disk usage limit.
func TestDiskLimit_MonitoringShowsLimit(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	ts.App.Config.MaxDiskUsage = 5_000_000_000 // 5GB

	mon := ts.GetMonitoring(t)

	if mon.Application.MaxDiskUsageBytes != 5_000_000_000 {
		t.Errorf("Expected MaxDiskUsageBytes=5000000000, got %d", mon.Application.MaxDiskUsageBytes)
	}
}

// TestDiskLimit_MonitoringShowsZeroWhenUnlimited verifies the monitoring endpoint
// exposes 0 when no disk limit is set.
func TestDiskLimit_MonitoringShowsZeroWhenUnlimited(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	ts.App.Config.MaxDiskUsage = 0

	mon := ts.GetMonitoring(t)

	if mon.Application.MaxDiskUsageBytes != 0 {
		t.Errorf("Expected MaxDiskUsageBytes=0, got %d", mon.Application.MaxDiskUsageBytes)
	}
}

