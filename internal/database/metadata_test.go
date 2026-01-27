package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// createTestTopicDB creates a temp-file SQLite database with the topic schema for testing.
// Uses a real file instead of :memory: so that concurrent goroutines share the same database.
func createTestTopicDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test-topic.db")
	db, err := InitTopicDB(dbPath)
	if err != nil {
		t.Fatalf("failed to init test topic db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// insertTestAsset inserts a minimal asset record for metadata tests.
func insertTestAsset(t *testing.T, db *sql.DB, assetID string) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO assets (asset_id, asset_size, extension, blob_name, byte_offset, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		assetID, 1024, "bin", "001.dat", 0, 1700000000,
	)
	if err != nil {
		t.Fatalf("failed to insert test asset: %v", err)
	}
}

func TestInsertMetadataLog_Transactional(t *testing.T) {
	db := createTestTopicDB(t)

	assetID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	insertTestAsset(t, db, assetID)

	entry := MetadataLogEntry{
		AssetID:          assetID,
		Op:               "set",
		Key:              "width",
		Value:            "1920",
		Processor:        "test",
		ProcessorVersion: "1.0",
	}

	logID, err := InsertMetadataLog(db, entry)
	if err != nil {
		t.Fatalf("InsertMetadataLog failed: %v", err)
	}
	if logID <= 0 {
		t.Errorf("expected positive log ID, got %d", logID)
	}

	// Verify metadata_log has the entry
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM metadata_log WHERE asset_id = ?", assetID).Scan(&count)
	if err != nil {
		t.Fatalf("failed to count metadata_log: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 metadata_log entry, got %d", count)
	}

	// Verify metadata_computed was populated atomically
	computed, err := GetMetadataComputed(db, assetID)
	if err != nil {
		t.Fatalf("GetMetadataComputed failed: %v", err)
	}
	if computed == nil {
		t.Fatal("expected computed metadata, got nil")
	}
	if computed["width"] != float64(1920) {
		t.Errorf("computed[width] = %v, want 1920", computed["width"])
	}
}

func TestInsertMetadataLog_SetThenDelete(t *testing.T) {
	db := createTestTopicDB(t)

	assetID := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	insertTestAsset(t, db, assetID)

	// Set a key
	_, err := InsertMetadataLog(db, MetadataLogEntry{
		AssetID:          assetID,
		Op:               "set",
		Key:              "color",
		Value:            "red",
		Processor:        "test",
		ProcessorVersion: "1.0",
	})
	if err != nil {
		t.Fatalf("InsertMetadataLog (set) failed: %v", err)
	}

	// Verify it exists in computed
	computed, err := GetMetadataComputed(db, assetID)
	if err != nil {
		t.Fatalf("GetMetadataComputed after set failed: %v", err)
	}
	if computed["color"] != "red" {
		t.Errorf("after set: computed[color] = %v, want 'red'", computed["color"])
	}

	// Delete the key
	_, err = InsertMetadataLog(db, MetadataLogEntry{
		AssetID:          assetID,
		Op:               "delete",
		Key:              "color",
		Processor:        "test",
		ProcessorVersion: "1.0",
	})
	if err != nil {
		t.Fatalf("InsertMetadataLog (delete) failed: %v", err)
	}

	// Verify it's gone from computed
	computed, err = GetMetadataComputed(db, assetID)
	if err != nil {
		t.Fatalf("GetMetadataComputed after delete failed: %v", err)
	}
	if _, exists := computed["color"]; exists {
		t.Errorf("after delete: computed[color] should not exist, got %v", computed["color"])
	}

	// Verify metadata_log has both entries (append-only)
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM metadata_log WHERE asset_id = ?", assetID).Scan(&count)
	if err != nil {
		t.Fatalf("failed to count metadata_log: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 metadata_log entries (set + delete), got %d", count)
	}
}

func TestInsertMetadataLog_ConcurrentSetSameAsset(t *testing.T) {
	db := createTestTopicDB(t)

	assetID := "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	insertTestAsset(t, db, assetID)

	const numOps = 10
	var wg sync.WaitGroup
	errors := make(chan error, numOps)

	// Set different keys concurrently
	for i := 0; i < numOps; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			entry := MetadataLogEntry{
				AssetID:          assetID,
				Op:               "set",
				Key:              fmt.Sprintf("key_%d", index),
				Value:            fmt.Sprintf("value_%d", index),
				Processor:        "test",
				ProcessorVersion: "1.0",
			}
			_, err := InsertMetadataLog(db, entry)
			if err != nil {
				errors <- fmt.Errorf("goroutine %d: %w", index, err)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}

	// Verify all keys are present in computed metadata
	computed, err := GetMetadataComputed(db, assetID)
	if err != nil {
		t.Fatalf("GetMetadataComputed failed: %v", err)
	}

	for i := 0; i < numOps; i++ {
		key := fmt.Sprintf("key_%d", i)
		expectedValue := fmt.Sprintf("value_%d", i)
		if computed[key] != expectedValue {
			t.Errorf("computed[%s] = %v, want %q", key, computed[key], expectedValue)
		}
	}

	// Verify metadata_log has all entries
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM metadata_log WHERE asset_id = ?", assetID).Scan(&count)
	if err != nil {
		t.Fatalf("failed to count metadata_log: %v", err)
	}
	if count != numOps {
		t.Errorf("expected %d metadata_log entries, got %d", numOps, count)
	}
}

func TestInsertMetadataLog_MultipleValues(t *testing.T) {
	db := createTestTopicDB(t)

	assetID := "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	insertTestAsset(t, db, assetID)

	// Set multiple keys sequentially
	keys := map[string]string{
		"format":     "png",
		"width":      "1920",
		"height":     "1080",
		"color_mode": "rgb",
	}

	for key, value := range keys {
		_, err := InsertMetadataLog(db, MetadataLogEntry{
			AssetID:          assetID,
			Op:               "set",
			Key:              key,
			Value:            value,
			Processor:        "test",
			ProcessorVersion: "1.0",
		})
		if err != nil {
			t.Fatalf("InsertMetadataLog for %s failed: %v", key, err)
		}
	}

	// Verify computed has all keys
	computed, err := GetMetadataComputed(db, assetID)
	if err != nil {
		t.Fatalf("GetMetadataComputed failed: %v", err)
	}

	// Verify JSON structure
	jsonBytes, err := json.Marshal(computed)
	if err != nil {
		t.Fatalf("failed to marshal computed: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		t.Fatalf("failed to parse computed JSON: %v", err)
	}

	if len(parsed) != len(keys) {
		t.Errorf("expected %d keys in computed, got %d", len(keys), len(parsed))
	}
}
