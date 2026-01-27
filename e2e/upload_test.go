package e2e

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"silobang/internal/storage"
)

func TestSingleAssetUpload(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "test-topic")

	// Upload SmallFile
	resp, err := ts.UploadFile("test-topic", "test.bin", SmallFile, "")
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("Upload failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var uploadResp map[string]interface{}
	bodyBytes, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(bodyBytes, &uploadResp); err != nil {
		t.Fatalf("Failed to parse upload response: %v", err)
	}

	// Verify response contains hash
	hash, ok := uploadResp["hash"].(string)
	if !ok || hash == "" {
		t.Fatalf("Expected hash in response, got: %v", uploadResp)
	}

	// Verify blob is first dat file
	expectedDat := storage.FormatDatFilename(1)
	blob, ok := uploadResp["blob"].(string)
	if !ok || blob != expectedDat {
		t.Errorf("Expected blob: %s, got: %v", expectedDat, uploadResp["blob"])
	}

	// Verify dat file exists
	datPath := filepath.Join(ts.WorkDir, "test-topic", expectedDat)
	if _, err := os.Stat(datPath); os.IsNotExist(err) {
		t.Errorf("Expected %s to exist at %s", expectedDat, datPath)
	}

	// Query assets table
	db := ts.GetTopicDB(t, "test-topic")
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM assets WHERE asset_id = ?", hash).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query assets table: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 asset entry, got %d", count)
	}

	// Verify asset details
	var assetSize int64
	var extension string
	err = db.QueryRow("SELECT asset_size, extension FROM assets WHERE asset_id = ?", hash).Scan(&assetSize, &extension)
	if err != nil {
		t.Fatalf("Failed to query asset details: %v", err)
	}
	if assetSize != int64(len(SmallFile)) {
		t.Errorf("Expected asset_size: %d, got: %d", len(SmallFile), assetSize)
	}
	if extension != "bin" {
		t.Errorf("Expected extension: bin, got: %s", extension)
	}

	// Query orchestrator DB
	orchDB := ts.GetOrchestratorDB(t)
	err = orchDB.QueryRow("SELECT COUNT(*) FROM asset_index WHERE hash = ?", hash).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query orchestrator: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 orchestrator entry, got %d", count)
	}

	// Download and verify content
	downloadResp, err := ts.GET("/api/assets/" + hash + "/download")
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}
	defer downloadResp.Body.Close()

	if downloadResp.StatusCode != 200 {
		t.Fatalf("Download failed with status %d", downloadResp.StatusCode)
	}

	downloadedBytes, err := io.ReadAll(downloadResp.Body)
	if err != nil {
		t.Fatalf("Failed to read download response: %v", err)
	}

	if len(downloadedBytes) != len(SmallFile) {
		t.Errorf("Downloaded size mismatch: expected %d, got %d", len(SmallFile), len(downloadedBytes))
	}

	// Verify bytes match
	for i := range SmallFile {
		if downloadedBytes[i] != SmallFile[i] {
			t.Errorf("Downloaded content mismatch at byte %d", i)
			break
		}
	}
}
