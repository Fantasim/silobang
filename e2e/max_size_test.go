package e2e

import (
	"io"
	"testing"
)

func TestMaxFileSize(t *testing.T) {
	ts := StartTestServer(t)

	// Set max_dat_size directly (1 MB)
	ts.App.Config.MaxDatSize = 1048576

	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "test-topic")

	// Generate 2 MB file (exceeds max_dat_size)
	largeFile := GenerateTestFile(2 * 1024 * 1024)

	// Try to upload - should fail with 413
	resp, err := ts.UploadFile("test-topic", "large.bin", largeFile, "")
	if err != nil {
		t.Fatalf("Upload request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 413 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected 413 Payload Too Large, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Verify no asset entries created
	db := ts.GetTopicDB(t, "test-topic")
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM assets").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query assets: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 asset entries after rejected upload, got %d", count)
	}

	// Verify no orchestrator entries
	orchDB := ts.GetOrchestratorDB(t)
	err = orchDB.QueryRow("SELECT COUNT(*) FROM asset_index").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query orchestrator: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 orchestrator entries after rejected upload, got %d", count)
	}
}
