package e2e

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"meshbank/internal/storage"
)

func TestCorruptionDetection(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "test-topic")

	// Upload 3 files
	for i := 0; i < 3; i++ {
		content := GenerateTestFile(1024 * (i + 1))
		resp, err := ts.UploadFile("test-topic", "file.bin", content, "")
		if err != nil {
			t.Fatalf("Upload %d failed: %v", i, err)
		}
		resp.Body.Close()
	}

	// Shutdown server
	ts.Shutdown()

	// Corrupt first dat file by modifying the hash in the first entry header
	// Hash is at offset 14 (constants.HashOffset), 64 bytes
	// This will cause the running hash chain verification to fail
	datPath := filepath.Join(ts.WorkDir, "test-topic", storage.FormatDatFilename(1))
	file, err := os.OpenFile(datPath, os.O_RDWR, 0644)
	if err != nil {
		t.Fatalf("Failed to open dat file: %v", err)
	}

	// Overwrite the hash field (offset 14, 64 bytes) with zeros
	// This corrupts the entry header which the chain verification will detect
	zeros := make([]byte, 64)
	_, err = file.WriteAt(zeros, 14)
	if err != nil {
		file.Close()
		t.Fatalf("Failed to corrupt dat file: %v", err)
	}
	file.Close()

	// Restart server
	ts.Restart(t)

	// Verify topic marked unhealthy
	var topicsResp map[string]interface{}
	err = ts.GetJSON("/api/topics", &topicsResp)
	if err != nil {
		t.Fatalf("GET /api/topics failed: %v", err)
	}

	topics := topicsResp["topics"].([]interface{})
	if len(topics) != 1 {
		t.Fatalf("Expected 1 topic, got %d", len(topics))
	}

	topicInfo := topics[0].(map[string]interface{})
	healthy, ok := topicInfo["healthy"].(bool)
	if !ok {
		t.Fatalf("Expected healthy field, got: %v", topicInfo)
	}

	if healthy {
		t.Errorf("Expected topic to be unhealthy after corruption, got healthy: true")
	}

	// Verify error message present
	errMsg, ok := topicInfo["error"].(string)
	if !ok || errMsg == "" {
		t.Errorf("Expected error message for corrupted topic, got: %v", topicInfo["error"])
	}

	// Try to upload to corrupted topic - should fail
	resp, err := ts.UploadFile("test-topic", "new.bin", SmallFile, "")
	if err != nil {
		t.Fatalf("Upload request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 503 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected 503 Service Unavailable for unhealthy topic, got %d: %s",
			resp.StatusCode, string(bodyBytes))
	}

	// Try to query corrupted topic - should fail
	resp, err = ts.POST("/api/query/recent-imports", map[string]interface{}{
		"topics": []string{"test-topic"},
		"params": map[string]interface{}{
			"days":  7,
			"limit": 10,
		},
	})
	if err != nil {
		t.Fatalf("Query request failed: %v", err)
	}
	defer resp.Body.Close()

	// Should return error (400 or 503)
	if resp.StatusCode != 400 && resp.StatusCode != 503 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected error status for query on unhealthy topic, got %d: %s",
			resp.StatusCode, string(bodyBytes))
	}
}
