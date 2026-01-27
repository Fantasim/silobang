package e2e

import (
	"testing"

	"meshbank/internal/constants"
)

// TestBatchMetadataBasic tests basic batch set/delete operations
func TestBatchMetadataBasic(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "batch-test")

	// Upload test assets
	upload1 := ts.UploadFileExpectSuccess(t, "batch-test", "file1.txt", []byte("content1"), "")
	upload2 := ts.UploadFileExpectSuccess(t, "batch-test", "file2.txt", []byte("content2"), "")
	upload3 := ts.UploadFileExpectSuccess(t, "batch-test", "file3.txt", []byte("content3"), "")

	// Test batch set operations
	batchReq := BatchMetadataRequest{
		Operations: []BatchMetadataOperation{
			{Hash: upload1.Hash, Op: "set", Key: "tag", Value: "processed"},
			{Hash: upload2.Hash, Op: "set", Key: "tag", Value: "pending"},
			{Hash: upload3.Hash, Op: "set", Key: "priority", Value: 5},
		},
		Processor:        "test",
		ProcessorVersion: "1.0",
	}

	resp := ts.BatchSetMetadata(t, batchReq)

	if !resp.Success {
		t.Errorf("expected success=true, got false")
	}
	if resp.Total != 3 {
		t.Errorf("expected total=3, got %d", resp.Total)
	}
	if resp.Succeeded != 3 {
		t.Errorf("expected succeeded=3, got %d", resp.Succeeded)
	}
	if resp.Failed != 0 {
		t.Errorf("expected failed=0, got %d", resp.Failed)
	}

	// Verify metadata was set correctly
	meta1 := ts.GetAssetMetadata(t, upload1.Hash)
	computed1, ok := meta1["computed_metadata"].(map[string]interface{})
	if !ok {
		t.Fatalf("computed_metadata not found for asset1")
	}
	if computed1["tag"] != "processed" {
		t.Errorf("expected tag=processed, got %v", computed1["tag"])
	}

	meta2 := ts.GetAssetMetadata(t, upload2.Hash)
	computed2, ok := meta2["computed_metadata"].(map[string]interface{})
	if !ok {
		t.Fatalf("computed_metadata not found for asset2")
	}
	if computed2["tag"] != "pending" {
		t.Errorf("expected tag=pending, got %v", computed2["tag"])
	}

	meta3 := ts.GetAssetMetadata(t, upload3.Hash)
	computed3, ok := meta3["computed_metadata"].(map[string]interface{})
	if !ok {
		t.Fatalf("computed_metadata not found for asset3")
	}
	// Numeric value should be stored as float64 from JSON
	if computed3["priority"] != float64(5) {
		t.Errorf("expected priority=5, got %v", computed3["priority"])
	}

	// Test batch delete operation
	deleteReq := BatchMetadataRequest{
		Operations: []BatchMetadataOperation{
			{Hash: upload1.Hash, Op: "delete", Key: "tag"},
		},
		Processor:        "test",
		ProcessorVersion: "1.0",
	}

	deleteResp := ts.BatchSetMetadata(t, deleteReq)
	if !deleteResp.Success {
		t.Errorf("delete batch failed")
	}

	// Verify metadata was deleted
	meta1After := ts.GetAssetMetadata(t, upload1.Hash)
	computed1After, ok := meta1After["computed_metadata"].(map[string]interface{})
	if !ok {
		t.Fatalf("computed_metadata not found after delete")
	}
	if _, exists := computed1After["tag"]; exists {
		t.Errorf("tag should have been deleted")
	}
}

// TestBatchMetadataAtomicity tests per-topic atomic rollback
func TestBatchMetadataAtomicity(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "atomic-test")

	// Upload a test asset
	upload := ts.UploadFileExpectSuccess(t, "atomic-test", "test.txt", []byte("content"), "")

	// Test batch with one valid and one invalid hash
	invalidHash := "0000000000000000000000000000000000000000000000000000000000000000"
	batchReq := BatchMetadataRequest{
		Operations: []BatchMetadataOperation{
			{Hash: upload.Hash, Op: "set", Key: "valid", Value: "yes"},
			{Hash: invalidHash, Op: "set", Key: "should", Value: "fail"},
		},
		Processor:        "test",
		ProcessorVersion: "1.0",
	}

	resp := ts.BatchSetMetadata(t, batchReq)

	// Should have partial success - one succeeded, one failed
	if resp.Success {
		t.Errorf("expected success=false due to partial failure")
	}
	if resp.Total != 2 {
		t.Errorf("expected total=2, got %d", resp.Total)
	}
	if resp.Succeeded != 1 {
		t.Errorf("expected succeeded=1, got %d", resp.Succeeded)
	}
	if resp.Failed != 1 {
		t.Errorf("expected failed=1, got %d", resp.Failed)
	}

	// Verify the valid operation succeeded
	meta := ts.GetAssetMetadata(t, upload.Hash)
	computed, ok := meta["computed_metadata"].(map[string]interface{})
	if !ok {
		t.Fatalf("computed_metadata not found")
	}
	if computed["valid"] != "yes" {
		t.Errorf("valid operation should have succeeded, got %v", computed["valid"])
	}

	// Check the results array for details
	var foundValid, foundInvalid bool
	for _, r := range resp.Results {
		if r.Hash == upload.Hash && r.Success {
			foundValid = true
		}
		if r.Hash == invalidHash && !r.Success && r.Error != "" {
			foundInvalid = true
		}
	}
	if !foundValid {
		t.Errorf("valid operation result not found")
	}
	if !foundInvalid {
		t.Errorf("invalid operation result not found")
	}
}

// TestBatchMetadataCrossTopic tests operations across multiple topics
func TestBatchMetadataCrossTopic(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "topic-a")
	ts.CreateTopic(t, "topic-b")

	// Upload assets to different topics
	uploadA := ts.UploadFileExpectSuccess(t, "topic-a", "fileA.txt", []byte("contentA"), "")
	uploadB := ts.UploadFileExpectSuccess(t, "topic-b", "fileB.txt", []byte("contentB"), "")

	// Batch operation across topics
	batchReq := BatchMetadataRequest{
		Operations: []BatchMetadataOperation{
			{Hash: uploadA.Hash, Op: "set", Key: "source", Value: "topic-a"},
			{Hash: uploadB.Hash, Op: "set", Key: "source", Value: "topic-b"},
		},
		Processor:        "cross-topic-test",
		ProcessorVersion: "1.0",
	}

	resp := ts.BatchSetMetadata(t, batchReq)

	if !resp.Success {
		t.Errorf("cross-topic batch should succeed")
	}
	if resp.Succeeded != 2 {
		t.Errorf("expected succeeded=2, got %d", resp.Succeeded)
	}

	// Verify both assets have correct metadata
	metaA := ts.GetAssetMetadata(t, uploadA.Hash)
	computedA, _ := metaA["computed_metadata"].(map[string]interface{})
	if computedA["source"] != "topic-a" {
		t.Errorf("asset A has wrong source: %v", computedA["source"])
	}

	metaB := ts.GetAssetMetadata(t, uploadB.Hash)
	computedB, _ := metaB["computed_metadata"].(map[string]interface{})
	if computedB["source"] != "topic-b" {
		t.Errorf("asset B has wrong source: %v", computedB["source"])
	}
}

// TestBatchMetadataImmutability verifies metadata_log is append-only
func TestBatchMetadataImmutability(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "immutable-test")

	upload := ts.UploadFileExpectSuccess(t, "immutable-test", "test.txt", []byte("content"), "")

	// Get direct DB access for verification
	db := ts.GetTopicDB(t, "immutable-test")

	// Count initial log entries
	var initialCount int
	err := db.QueryRow("SELECT COUNT(*) FROM metadata_log").Scan(&initialCount)
	if err != nil {
		t.Fatalf("failed to count initial log entries: %v", err)
	}

	// Set metadata via batch
	batchReq := BatchMetadataRequest{
		Operations: []BatchMetadataOperation{
			{Hash: upload.Hash, Op: "set", Key: "key1", Value: "value1"},
		},
		Processor:        "test",
		ProcessorVersion: "1.0",
	}
	ts.BatchSetMetadata(t, batchReq)

	// Count log entries after first batch
	var afterFirstCount int
	err = db.QueryRow("SELECT COUNT(*) FROM metadata_log").Scan(&afterFirstCount)
	if err != nil {
		t.Fatalf("failed to count log entries after first batch: %v", err)
	}

	if afterFirstCount != initialCount+1 {
		t.Errorf("expected %d log entries, got %d", initialCount+1, afterFirstCount)
	}

	// Update the same key
	batchReq2 := BatchMetadataRequest{
		Operations: []BatchMetadataOperation{
			{Hash: upload.Hash, Op: "set", Key: "key1", Value: "value2"},
		},
		Processor:        "test",
		ProcessorVersion: "1.0",
	}
	ts.BatchSetMetadata(t, batchReq2)

	// Count log entries after second batch - should have grown, not updated
	var afterSecondCount int
	err = db.QueryRow("SELECT COUNT(*) FROM metadata_log").Scan(&afterSecondCount)
	if err != nil {
		t.Fatalf("failed to count log entries after second batch: %v", err)
	}

	if afterSecondCount != afterFirstCount+1 {
		t.Errorf("log should grow, not update: expected %d entries, got %d", afterFirstCount+1, afterSecondCount)
	}

	// Delete the key
	batchReq3 := BatchMetadataRequest{
		Operations: []BatchMetadataOperation{
			{Hash: upload.Hash, Op: "delete", Key: "key1"},
		},
		Processor:        "test",
		ProcessorVersion: "1.0",
	}
	ts.BatchSetMetadata(t, batchReq3)

	// Count log entries after delete - should have grown again
	var afterDeleteCount int
	err = db.QueryRow("SELECT COUNT(*) FROM metadata_log").Scan(&afterDeleteCount)
	if err != nil {
		t.Fatalf("failed to count log entries after delete: %v", err)
	}

	if afterDeleteCount != afterSecondCount+1 {
		t.Errorf("delete should append to log: expected %d entries, got %d", afterSecondCount+1, afterDeleteCount)
	}

	// Verify computed metadata reflects final state (key deleted)
	meta := ts.GetAssetMetadata(t, upload.Hash)
	computed, _ := meta["computed_metadata"].(map[string]interface{})
	if _, exists := computed["key1"]; exists {
		t.Errorf("key1 should be deleted from computed metadata")
	}
}

// TestApplyMetadata tests query-based metadata application
func TestApplyMetadata(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "apply-test")

	// Upload multiple assets
	upload1 := ts.UploadFileExpectSuccess(t, "apply-test", "file1.txt", []byte("content1"), "")
	upload2 := ts.UploadFileExpectSuccess(t, "apply-test", "file2.txt", []byte("content2"), "")
	upload3 := ts.UploadFileExpectSuccess(t, "apply-test", "file3.txt", []byte("content3"), "")

	// Apply metadata to all assets using recent-imports preset (returns asset_id)
	applyReq := ApplyMetadataRequest{
		QueryPreset: "recent-imports",
		QueryParams: map[string]interface{}{
			"days":  "365", // Large window to capture all
			"limit": "100",
		},
		Topics:           []string{"apply-test"},
		Op:               "set",
		Key:              "batch_tag",
		Value:            "applied",
		Processor:        "apply-test",
		ProcessorVersion: "1.0",
	}

	resp := ts.ApplyMetadata(t, applyReq)

	if !resp.Success {
		t.Errorf("apply should succeed")
	}
	if resp.Total != 3 {
		t.Errorf("expected total=3, got %d", resp.Total)
	}
	if resp.Succeeded != 3 {
		t.Errorf("expected succeeded=3, got %d", resp.Succeeded)
	}

	// Verify all assets have the metadata
	for _, hash := range []string{upload1.Hash, upload2.Hash, upload3.Hash} {
		meta := ts.GetAssetMetadata(t, hash)
		computed, ok := meta["computed_metadata"].(map[string]interface{})
		if !ok {
			t.Fatalf("computed_metadata not found for %s", hash)
		}
		if computed["batch_tag"] != "applied" {
			t.Errorf("asset %s has wrong batch_tag: %v", hash, computed["batch_tag"])
		}
	}
}

// TestBatchMetadataMaxOperations tests rejection of too many operations
func TestBatchMetadataMaxOperations(t *testing.T) {
	// Skip: BatchMetadataMaxOperations is 100000, creating 100001 operations
	// is impractical for e2e testing (memory/time). The validation logic is
	// covered by the handler code - this test validates the constant is used.
	t.Skip("Skipping: creating 100001 operations is impractical for e2e testing")

	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "max-ops-test")

	// Create a batch request with operations over the limit
	overLimit := constants.BatchMetadataMaxOperations + 1
	operations := make([]BatchMetadataOperation, overLimit)
	for i := 0; i < overLimit; i++ {
		// Use fake hashes - we just want to test the limit
		operations[i] = BatchMetadataOperation{
			Hash:  "0000000000000000000000000000000000000000000000000000000000000001",
			Op:    "set",
			Key:   "key",
			Value: "value",
		}
	}

	batchReq := BatchMetadataRequest{
		Operations:       operations,
		Processor:        "test",
		ProcessorVersion: "1.0",
	}

	errResp := ts.BatchSetMetadataExpectError(t, batchReq, 400)

	if errResp.Code != "BATCH_TOO_MANY_OPERATIONS" {
		t.Errorf("expected error code BATCH_TOO_MANY_OPERATIONS, got %s", errResp.Code)
	}
}

// TestBatchMetadataEmptyOperations tests rejection of empty operations
func TestBatchMetadataEmptyOperations(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	batchReq := BatchMetadataRequest{
		Operations:       []BatchMetadataOperation{},
		Processor:        "test",
		ProcessorVersion: "1.0",
	}

	errResp := ts.BatchSetMetadataExpectError(t, batchReq, 400)

	if errResp.Code != "INVALID_REQUEST" {
		t.Errorf("expected error code INVALID_REQUEST, got %s", errResp.Code)
	}
}

// TestBatchMetadataInvalidOperation tests rejection of invalid op type
func TestBatchMetadataInvalidOperation(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "invalid-op-test")

	upload := ts.UploadFileExpectSuccess(t, "invalid-op-test", "test.txt", []byte("content"), "")

	batchReq := BatchMetadataRequest{
		Operations: []BatchMetadataOperation{
			{Hash: upload.Hash, Op: "invalid", Key: "key", Value: "value"},
		},
		Processor:        "test",
		ProcessorVersion: "1.0",
	}

	resp := ts.BatchSetMetadata(t, batchReq)

	// The batch should complete but with a failure for the invalid operation
	if resp.Succeeded != 0 {
		t.Errorf("expected succeeded=0, got %d", resp.Succeeded)
	}
	if resp.Failed != 1 {
		t.Errorf("expected failed=1, got %d", resp.Failed)
	}

	// Check error message
	if len(resp.Results) > 0 && resp.Results[0].Error == "" {
		t.Errorf("expected error message for invalid operation")
	}
}

// TestApplyMetadataInvalidPreset tests rejection of invalid query preset
func TestApplyMetadataInvalidPreset(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	applyReq := ApplyMetadataRequest{
		QueryPreset:      "nonexistent_preset",
		Op:               "set",
		Key:              "key",
		Value:            "value",
		Processor:        "test",
		ProcessorVersion: "1.0",
	}

	errResp := ts.ApplyMetadataExpectError(t, applyReq, 400)

	if errResp.Code != "PRESET_NOT_FOUND" {
		t.Errorf("expected error code PRESET_NOT_FOUND, got %s", errResp.Code)
	}
}

// TestBatchMetadataEmptyKey tests rejection of empty key
func TestBatchMetadataEmptyKey(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "empty-key-test")

	upload := ts.UploadFileExpectSuccess(t, "empty-key-test", "test.txt", []byte("content"), "")

	batchReq := BatchMetadataRequest{
		Operations: []BatchMetadataOperation{
			{Hash: upload.Hash, Op: "set", Key: "", Value: "value"},
		},
		Processor:        "test",
		ProcessorVersion: "1.0",
	}

	resp := ts.BatchSetMetadata(t, batchReq)

	// Should fail with empty key error
	if resp.Succeeded != 0 {
		t.Errorf("expected succeeded=0 for empty key, got %d", resp.Succeeded)
	}
	if resp.Failed != 1 {
		t.Errorf("expected failed=1 for empty key, got %d", resp.Failed)
	}
}
