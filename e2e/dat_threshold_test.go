package e2e

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"silobang/internal/storage"
)

func TestDATThreshold(t *testing.T) {
	ts := StartTestServer(t)

	// Set max_dat_size directly (1 MB)
	ts.App.Config.MaxDatSize = 1048576

	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "test-topic")

	// Upload 500 KB file
	file1 := GenerateTestFile(500 * 1024)
	resp, err := ts.UploadFile("test-topic", "file1.bin", file1, "")
	if err != nil {
		t.Fatalf("Upload 1 failed: %v", err)
	}
	defer resp.Body.Close()

	var upload1 map[string]interface{}
	bodyBytes, _ := io.ReadAll(resp.Body)
	json.Unmarshal(bodyBytes, &upload1)

	blob1, _ := upload1["blob"].(string)
	expectedFirst := storage.FormatDatFilename(1)
	if blob1 != expectedFirst {
		t.Errorf("Expected blob: %s for first file, got: %s", expectedFirst, blob1)
	}

	// Upload another 500 KB file (total 1 MB, within threshold)
	file2 := GenerateTestFile(500 * 1024)
	resp, err = ts.UploadFile("test-topic", "file2.bin", file2, "")
	if err != nil {
		t.Fatalf("Upload 2 failed: %v", err)
	}
	defer resp.Body.Close()

	var upload2 map[string]interface{}
	bodyBytes, _ = io.ReadAll(resp.Body)
	json.Unmarshal(bodyBytes, &upload2)

	blob2, _ := upload2["blob"].(string)
	if blob2 != expectedFirst {
		t.Errorf("Expected blob: %s for second file, got: %s", expectedFirst, blob2)
	}

	// Upload 500 KB file (should trigger rollover to 002.dat)
	file3 := GenerateTestFile(500 * 1024)
	resp, err = ts.UploadFile("test-topic", "file3.bin", file3, "")
	if err != nil {
		t.Fatalf("Upload 3 failed: %v", err)
	}
	defer resp.Body.Close()

	var upload3 map[string]interface{}
	bodyBytes, _ = io.ReadAll(resp.Body)
	json.Unmarshal(bodyBytes, &upload3)

	blob3, _ := upload3["blob"].(string)
	expectedSecond := storage.FormatDatFilename(2)
	if blob3 != expectedSecond {
		t.Errorf("Expected blob: %s after rollover, got: %s", expectedSecond, blob3)
	}

	// Verify both .dat files exist
	dat1Path := filepath.Join(ts.WorkDir, "test-topic", expectedFirst)
	dat2Path := filepath.Join(ts.WorkDir, "test-topic", expectedSecond)

	if _, err := os.Stat(dat1Path); os.IsNotExist(err) {
		t.Errorf("Expected %s to exist", expectedFirst)
	}
	if _, err := os.Stat(dat2Path); os.IsNotExist(err) {
		t.Errorf("Expected %s to exist", expectedSecond)
	}

	// Verify dat_hashes table has entries for both files
	db := ts.GetTopicDB(t, "test-topic")
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM dat_hashes").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query dat_hashes: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 dat_hashes entries, got %d", count)
	}
}
