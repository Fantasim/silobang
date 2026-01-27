package e2e

import (
	"encoding/json"
	"io"
	"strings"
	"testing"

	"meshbank/internal/constants"
)

// TestMetadataKeyLengthValidation tests key length limits
func TestMetadataKeyLengthValidation(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "key-length-test")

	// Upload test asset
	upload := ts.UploadFileExpectSuccess(t, "key-length-test", "test.bin", SmallFile, "")

	// Test 1: Key at exactly max length (256 chars) - should succeed
	maxKey := strings.Repeat("a", constants.MaxMetadataKeyLength)
	resp, err := ts.POST("/api/assets/"+upload.Hash+"/metadata", map[string]interface{}{
		"op":                "set",
		"key":               maxKey,
		"value":             "test",
		"processor":         "test",
		"processor_version": "1.0",
	})
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected 200 for key at max length (%d chars), got %d: %s",
			constants.MaxMetadataKeyLength, resp.StatusCode, string(bodyBytes))
	}

	// Test 2: Key exceeding max length (257 chars) - should fail
	tooLongKey := strings.Repeat("a", constants.MaxMetadataKeyLength+1)
	resp, err = ts.POST("/api/assets/"+upload.Hash+"/metadata", map[string]interface{}{
		"op":                "set",
		"key":               tooLongKey,
		"value":             "test",
		"processor":         "test",
		"processor_version": "1.0",
	})
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 400 {
		t.Errorf("Expected 400 for key exceeding max length, got %d", resp.StatusCode)
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	var errResp ErrorResponse
	if err := json.Unmarshal(bodyBytes, &errResp); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	if errResp.Code != "METADATA_KEY_TOO_LONG" {
		t.Errorf("Expected error code METADATA_KEY_TOO_LONG, got: %s", errResp.Code)
	}
}

// TestMetadataValueLengthValidation tests value length limits
func TestMetadataValueLengthValidation(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "value-length-test")

	// Upload test asset
	upload := ts.UploadFileExpectSuccess(t, "value-length-test", "test.bin", SmallFile, "")

	// Test 1: Value at exactly max size (10MB) - should succeed
	maxValue := strings.Repeat("x", constants.MaxMetadataValueBytes)
	resp, err := ts.POST("/api/assets/"+upload.Hash+"/metadata", map[string]interface{}{
		"op":                "set",
		"key":               "large_value",
		"value":             maxValue,
		"processor":         "test",
		"processor_version": "1.0",
	})
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected 200 for value at max size (%d bytes), got %d: %s",
			constants.MaxMetadataValueBytes, resp.StatusCode, string(bodyBytes))
	}

	// Test 2: Value exceeding max size - should fail
	tooLargeValue := strings.Repeat("x", constants.MaxMetadataValueBytes+1)
	resp, err = ts.POST("/api/assets/"+upload.Hash+"/metadata", map[string]interface{}{
		"op":                "set",
		"key":               "too_large",
		"value":             tooLargeValue,
		"processor":         "test",
		"processor_version": "1.0",
	})
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 400 {
		t.Errorf("Expected 400 for value exceeding max size, got %d", resp.StatusCode)
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	var errResp ErrorResponse
	if err := json.Unmarshal(bodyBytes, &errResp); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	if errResp.Code != "METADATA_VALUE_TOO_LONG" {
		t.Errorf("Expected error code METADATA_VALUE_TOO_LONG, got: %s", errResp.Code)
	}
}

// TestBatchMetadataKeyLengthValidation tests key length in batch operations
func TestBatchMetadataKeyLengthValidation(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "batch-key-test")

	// Upload test assets
	upload1 := ts.UploadFileExpectSuccess(t, "batch-key-test", "file1.bin", []byte("content1"), "")
	upload2 := ts.UploadFileExpectSuccess(t, "batch-key-test", "file2.bin", []byte("content2"), "")

	// Test batch with one valid key and one too long key
	tooLongKey := strings.Repeat("k", constants.MaxMetadataKeyLength+1)
	batchReq := BatchMetadataRequest{
		Operations: []BatchMetadataOperation{
			{Hash: upload1.Hash, Op: "set", Key: "valid_key", Value: "value1"},
			{Hash: upload2.Hash, Op: "set", Key: tooLongKey, Value: "value2"},
		},
		Processor:        "test",
		ProcessorVersion: "1.0",
	}

	resp := ts.BatchSetMetadata(t, batchReq)

	// Should have partial success - one succeeded, one failed
	if resp.Success {
		t.Errorf("Expected success=false due to partial failure")
	}
	if resp.Succeeded != 1 {
		t.Errorf("Expected succeeded=1, got %d", resp.Succeeded)
	}
	if resp.Failed != 1 {
		t.Errorf("Expected failed=1, got %d", resp.Failed)
	}

	// Verify the valid operation succeeded
	meta := ts.GetAssetMetadata(t, upload1.Hash)
	computed, ok := meta["computed_metadata"].(map[string]interface{})
	if !ok {
		t.Fatalf("computed_metadata not found")
	}
	if computed["valid_key"] != "value1" {
		t.Errorf("Valid operation should have succeeded, got %v", computed["valid_key"])
	}

	// Check error message for failed operation
	var foundKeyError bool
	for _, r := range resp.Results {
		if r.Hash == upload2.Hash && !r.Success {
			if strings.Contains(r.Error, "key exceeds maximum length") {
				foundKeyError = true
			}
		}
	}
	if !foundKeyError {
		t.Errorf("Expected key length error in results for upload2")
	}
}

// TestBatchMetadataValueLengthValidation tests value length in batch operations
func TestBatchMetadataValueLengthValidation(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "batch-value-test")

	// Upload test assets
	upload1 := ts.UploadFileExpectSuccess(t, "batch-value-test", "file1.bin", []byte("content1"), "")
	upload2 := ts.UploadFileExpectSuccess(t, "batch-value-test", "file2.bin", []byte("content2"), "")

	// Test batch with one valid value and one too large value
	tooLargeValue := strings.Repeat("v", constants.MaxMetadataValueBytes+1)
	batchReq := BatchMetadataRequest{
		Operations: []BatchMetadataOperation{
			{Hash: upload1.Hash, Op: "set", Key: "key1", Value: "valid_value"},
			{Hash: upload2.Hash, Op: "set", Key: "key2", Value: tooLargeValue},
		},
		Processor:        "test",
		ProcessorVersion: "1.0",
	}

	resp := ts.BatchSetMetadata(t, batchReq)

	// Should have partial success - one succeeded, one failed
	if resp.Success {
		t.Errorf("Expected success=false due to partial failure")
	}
	if resp.Succeeded != 1 {
		t.Errorf("Expected succeeded=1, got %d", resp.Succeeded)
	}
	if resp.Failed != 1 {
		t.Errorf("Expected failed=1, got %d", resp.Failed)
	}

	// Verify the valid operation succeeded
	meta := ts.GetAssetMetadata(t, upload1.Hash)
	computed, ok := meta["computed_metadata"].(map[string]interface{})
	if !ok {
		t.Fatalf("computed_metadata not found")
	}
	if computed["key1"] != "valid_value" {
		t.Errorf("Valid operation should have succeeded, got %v", computed["key1"])
	}

	// Check error message for failed operation
	var foundValueError bool
	for _, r := range resp.Results {
		if r.Hash == upload2.Hash && !r.Success {
			if strings.Contains(r.Error, "value exceeds maximum size") {
				foundValueError = true
			}
		}
	}
	if !foundValueError {
		t.Errorf("Expected value size error in results for upload2")
	}
}

// TestApplyMetadataKeyLengthValidation tests key length in apply metadata
func TestApplyMetadataKeyLengthValidation(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "apply-key-test")

	// Upload test asset
	ts.UploadFileExpectSuccess(t, "apply-key-test", "test.bin", SmallFile, "")

	// Test apply with key too long - should fail before execution
	tooLongKey := strings.Repeat("k", constants.MaxMetadataKeyLength+1)
	resp, err := ts.POST("/api/metadata/apply", map[string]interface{}{
		"query_preset":      "recent-imports",
		"topics":            []string{"apply-key-test"},
		"op":                "set",
		"key":               tooLongKey,
		"value":             "test",
		"processor":         "test",
		"processor_version": "1.0",
	})
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 400 {
		t.Errorf("Expected 400 for key exceeding max length, got %d", resp.StatusCode)
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	var errResp ErrorResponse
	if err := json.Unmarshal(bodyBytes, &errResp); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	if errResp.Code != "METADATA_KEY_TOO_LONG" {
		t.Errorf("Expected error code METADATA_KEY_TOO_LONG, got: %s", errResp.Code)
	}
}

// TestApplyMetadataValueLengthValidation tests value length in apply metadata
func TestApplyMetadataValueLengthValidation(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "apply-value-test")

	// Upload test asset
	ts.UploadFileExpectSuccess(t, "apply-value-test", "test.bin", SmallFile, "")

	// Test apply with value too large - should fail before execution
	tooLargeValue := strings.Repeat("v", constants.MaxMetadataValueBytes+1)
	resp, err := ts.POST("/api/metadata/apply", map[string]interface{}{
		"query_preset":      "recent-imports",
		"topics":            []string{"apply-value-test"},
		"op":                "set",
		"key":               "test_key",
		"value":             tooLargeValue,
		"processor":         "test",
		"processor_version": "1.0",
	})
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 400 {
		t.Errorf("Expected 400 for value exceeding max size, got %d", resp.StatusCode)
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	var errResp ErrorResponse
	if err := json.Unmarshal(bodyBytes, &errResp); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	if errResp.Code != "METADATA_VALUE_TOO_LONG" {
		t.Errorf("Expected error code METADATA_VALUE_TOO_LONG, got: %s", errResp.Code)
	}
}

// TestDeleteOperationKeyLengthValidation ensures delete operations also validate key length
func TestDeleteOperationKeyLengthValidation(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "delete-key-test")

	// Upload test asset
	upload := ts.UploadFileExpectSuccess(t, "delete-key-test", "test.bin", SmallFile, "")

	// Test delete with key too long - should fail
	tooLongKey := strings.Repeat("d", constants.MaxMetadataKeyLength+1)
	resp, err := ts.POST("/api/assets/"+upload.Hash+"/metadata", map[string]interface{}{
		"op":                "delete",
		"key":               tooLongKey,
		"processor":         "test",
		"processor_version": "1.0",
	})
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 400 {
		t.Errorf("Expected 400 for delete with key exceeding max length, got %d", resp.StatusCode)
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	var errResp ErrorResponse
	if err := json.Unmarshal(bodyBytes, &errResp); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	if errResp.Code != "METADATA_KEY_TOO_LONG" {
		t.Errorf("Expected error code METADATA_KEY_TOO_LONG, got: %s", errResp.Code)
	}
}
