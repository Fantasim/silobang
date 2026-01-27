package e2e

import (
	"encoding/json"
	"io"
	"testing"
)

func TestDuplicateDetection(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Create two topics
	ts.CreateTopic(t, "topic-1")
	ts.CreateTopic(t, "topic-2")

	// Upload fileA to topic-1
	fileA := GenerateTestFile(5000)
	resp, err := ts.UploadFile("topic-1", "fileA.bin", fileA, "")
	if err != nil {
		t.Fatalf("First upload failed: %v", err)
	}
	defer resp.Body.Close()

	var upload1 map[string]interface{}
	bodyBytes, _ := io.ReadAll(resp.Body)
	json.Unmarshal(bodyBytes, &upload1)

	hashA, ok := upload1["hash"].(string)
	if !ok || hashA == "" {
		t.Fatalf("Expected hash from first upload, got: %v", upload1)
	}

	// Upload same fileA to topic-1 again
	resp, err = ts.UploadFile("topic-1", "fileA.bin", fileA, "")
	if err != nil {
		t.Fatalf("Second upload failed: %v", err)
	}
	defer resp.Body.Close()

	var upload2 map[string]interface{}
	bodyBytes, _ = io.ReadAll(resp.Body)
	json.Unmarshal(bodyBytes, &upload2)

	// Verify skipped: true
	skipped, ok := upload2["skipped"].(bool)
	if !ok || !skipped {
		t.Errorf("Expected skipped: true for duplicate in same topic, got: %v", upload2)
	}

	// Verify same hash
	hash2, _ := upload2["hash"].(string)
	if hash2 != hashA {
		t.Errorf("Expected same hash, got: %s vs %s", hashA, hash2)
	}

	// Upload same fileA to topic-2 (cross-topic dedup)
	resp, err = ts.UploadFile("topic-2", "fileA.bin", fileA, "")
	if err != nil {
		t.Fatalf("Third upload failed: %v", err)
	}
	defer resp.Body.Close()

	var upload3 map[string]interface{}
	bodyBytes, _ = io.ReadAll(resp.Body)
	json.Unmarshal(bodyBytes, &upload3)

	// Verify global dedup works
	skipped3, ok := upload3["skipped"].(bool)
	if !ok || !skipped3 {
		t.Errorf("Expected skipped: true for cross-topic duplicate, got: %v", upload3)
	}

	hash3, _ := upload3["hash"].(string)
	if hash3 != hashA {
		t.Errorf("Expected same hash for cross-topic duplicate, got: %s vs %s", hashA, hash3)
	}

	// Verify orchestrator has single entry
	orchDB := ts.GetOrchestratorDB(t)
	var count int
	err = orchDB.QueryRow("SELECT COUNT(*) FROM asset_index WHERE hash = ?", hashA).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query orchestrator: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 orchestrator entry for deduplicated file, got %d", count)
	}

	// Verify topic DBs don't have duplicate entries
	db1 := ts.GetTopicDB(t, "topic-1")
	err = db1.QueryRow("SELECT COUNT(*) FROM assets WHERE asset_id = ?", hashA).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query topic-1 db: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 asset entry in topic-1, got %d", count)
	}

	// Verify topic-2 has NO asset (deduplication skipped storage entirely)
	db2 := ts.GetTopicDB(t, "topic-2")
	err = db2.QueryRow("SELECT COUNT(*) FROM assets WHERE asset_id = ?", hashA).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query topic-2 db: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 asset entries in topic-2 (deduplicated), got %d", count)
	}
}
