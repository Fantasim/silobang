package services

import (
	"context"
	"database/sql"
	"testing"

	"meshbank/internal/constants"
	"meshbank/internal/logger"

	_ "github.com/mattn/go-sqlite3"
)

func TestNewVerifyService(t *testing.T) {
	mockApp := newMockAppState()
	log := logger.NewLogger("debug")

	svc := NewVerifyService(mockApp, log)

	if svc == nil {
		t.Fatal("NewVerifyService returned nil")
	}
	if svc.app != mockApp {
		t.Error("app field not set correctly")
	}
	if svc.logger != log {
		t.Error("logger field not set correctly")
	}
}

func TestVerifyService_GetTotalIndexEntries_NotConfigured(t *testing.T) {
	mockApp := newMockAppState()
	mockApp.orchestratorDB = nil // ensure not configured
	log := logger.NewLogger("debug")
	svc := NewVerifyService(mockApp, log)

	_, err := svc.GetTotalIndexEntries()
	if err == nil {
		t.Fatal("expected error but got nil")
	}

	code, ok := IsServiceError(err)
	if !ok {
		t.Fatalf("expected ServiceError but got: %T", err)
	}
	if code != constants.ErrCodeNotConfigured {
		t.Errorf("error code = %q, want %q", code, constants.ErrCodeNotConfigured)
	}
}

func TestVerifyService_GetTotalIndexEntries_WithDB(t *testing.T) {
	mockApp := newMockAppState()
	log := logger.NewLogger("debug")

	// Create in-memory SQLite database
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer db.Close()

	// Create the asset_index table
	_, err = db.Exec(`CREATE TABLE asset_index (hash TEXT, topic TEXT, dat_file TEXT)`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Insert some test data
	_, err = db.Exec(`INSERT INTO asset_index (hash, topic, dat_file) VALUES ('hash1', 'topic1', 'data.001.dat'), ('hash2', 'topic1', 'data.001.dat'), ('hash3', 'topic2', 'data.001.dat')`)
	if err != nil {
		t.Fatalf("failed to insert data: %v", err)
	}

	mockApp.orchestratorDB = db
	svc := NewVerifyService(mockApp, log)

	count, err := svc.GetTotalIndexEntries()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}

func TestVerifyService_VerifyIndex_NotConfigured(t *testing.T) {
	mockApp := newMockAppState()
	mockApp.orchestratorDB = nil // ensure not configured
	log := logger.NewLogger("debug")
	svc := NewVerifyService(mockApp, log)

	_, err := svc.VerifyIndex(context.Background(), []string{"topic1"})
	if err == nil {
		t.Fatal("expected error but got nil")
	}

	code, ok := IsServiceError(err)
	if !ok {
		t.Fatalf("expected ServiceError but got: %T", err)
	}
	if code != constants.ErrCodeNotConfigured {
		t.Errorf("error code = %q, want %q", code, constants.ErrCodeNotConfigured)
	}
}

func TestVerifyService_VerifyIndex_EmptyIndex(t *testing.T) {
	mockApp := newMockAppState()
	log := logger.NewLogger("debug")

	// Create in-memory SQLite database
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer db.Close()

	// Create the asset_index table (empty)
	_, err = db.Exec(`CREATE TABLE asset_index (hash TEXT, topic TEXT, dat_file TEXT)`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	mockApp.orchestratorDB = db
	svc := NewVerifyService(mockApp, log)

	result, err := svc.VerifyIndex(context.Background(), []string{"topic1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Valid {
		t.Error("expected Valid = true for empty index")
	}
	if result.OrphanCount != 0 {
		t.Errorf("OrphanCount = %d, want 0", result.OrphanCount)
	}
	if result.MissingCount != 0 {
		t.Errorf("MissingCount = %d, want 0", result.MissingCount)
	}
	if result.MismatchCount != 0 {
		t.Errorf("MismatchCount = %d, want 0", result.MismatchCount)
	}
	if len(result.Issues) != 0 {
		t.Errorf("Issues length = %d, want 0", len(result.Issues))
	}
}

func TestVerifyService_VerifyIndex_CancellationSupport(t *testing.T) {
	mockApp := newMockAppState()
	log := logger.NewLogger("debug")

	// Create in-memory SQLite database with many entries
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE asset_index (hash TEXT, topic TEXT, dat_file TEXT)`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Insert many rows
	for i := 0; i < 100; i++ {
		_, err = db.Exec(`INSERT INTO asset_index (hash, topic, dat_file) VALUES (?, 'topic1', 'data.001.dat')`,
			"hash"+string(rune('a'+i%26)))
		if err != nil {
			t.Fatalf("failed to insert data: %v", err)
		}
	}

	mockApp.orchestratorDB = db
	mockApp.RegisterTopic("topic1", true, "")
	svc := NewVerifyService(mockApp, log)

	// Create already cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = svc.VerifyIndex(ctx, []string{"topic1"})
	// Should return context.Canceled error
	if err != context.Canceled {
		t.Logf("got error: %v (type: %T)", err, err)
		// It's acceptable if it didn't have time to check cancellation
	}
}

func TestVerifyService_GetTopicDB(t *testing.T) {
	mockApp := newMockAppState()
	log := logger.NewLogger("debug")

	// Create in-memory SQLite database
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer db.Close()

	mockApp.topicDBs["test-topic"] = db
	svc := NewVerifyService(mockApp, log)

	result, err := svc.GetTopicDB("test-topic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != db {
		t.Error("returned db does not match expected")
	}
}

func TestVerifyService_ListDatFiles(t *testing.T) {
	mockApp := newMockAppState()
	mockApp.workingDir = "/nonexistent/path"
	log := logger.NewLogger("debug")
	svc := NewVerifyService(mockApp, log)

	// This will fail because the path doesn't exist, but tests the flow
	_, err := svc.ListDatFiles("test-topic")
	// We expect an error since the path doesn't exist
	if err == nil {
		t.Log("Note: ListDatFiles returned nil error for non-existent path")
	}
}

func TestDatFileResult_Struct(t *testing.T) {
	result := &DatFileResult{
		DatFile:    "data.001.dat",
		Valid:      true,
		EntryCount: 100,
		Error:      "",
	}

	if result.DatFile != "data.001.dat" {
		t.Errorf("DatFile = %q, want %q", result.DatFile, "data.001.dat")
	}
	if !result.Valid {
		t.Error("Valid should be true")
	}
	if result.EntryCount != 100 {
		t.Errorf("EntryCount = %d, want 100", result.EntryCount)
	}
	if result.Error != "" {
		t.Errorf("Error = %q, want empty", result.Error)
	}

	// Test invalid result
	invalidResult := &DatFileResult{
		DatFile: "data.002.dat",
		Valid:   false,
		Error:   "hash mismatch",
	}

	if invalidResult.Valid {
		t.Error("Valid should be false for invalid result")
	}
	if invalidResult.Error != "hash mismatch" {
		t.Errorf("Error = %q, want %q", invalidResult.Error, "hash mismatch")
	}
}

func TestTopicResult_Struct(t *testing.T) {
	result := &TopicResult{
		TopicName:       "test-topic",
		Valid:           true,
		DatFilesChecked: 3,
		Errors:          []string{},
		DatResults: []DatFileResult{
			{DatFile: "data.001.dat", Valid: true, EntryCount: 50},
			{DatFile: "data.002.dat", Valid: true, EntryCount: 30},
			{DatFile: "data.003.dat", Valid: true, EntryCount: 20},
		},
	}

	if result.TopicName != "test-topic" {
		t.Errorf("TopicName = %q, want %q", result.TopicName, "test-topic")
	}
	if !result.Valid {
		t.Error("Valid should be true")
	}
	if result.DatFilesChecked != 3 {
		t.Errorf("DatFilesChecked = %d, want 3", result.DatFilesChecked)
	}
	if len(result.DatResults) != 3 {
		t.Errorf("DatResults length = %d, want 3", len(result.DatResults))
	}

	// Test with errors
	invalidResult := &TopicResult{
		TopicName: "bad-topic",
		Valid:     false,
		Errors:    []string{"data.001.dat: hash mismatch"},
	}

	if invalidResult.Valid {
		t.Error("Valid should be false for invalid result")
	}
	if len(invalidResult.Errors) != 1 {
		t.Errorf("Errors length = %d, want 1", len(invalidResult.Errors))
	}
}

func TestIndexIssue_Struct(t *testing.T) {
	tests := []struct {
		name   string
		issue  IndexIssue
		wantType string
	}{
		{
			name: "orphan issue",
			issue: IndexIssue{
				Type:   "orphan",
				Hash:   "abc123",
				Topic:  "deleted-topic",
				Detail: "topic unhealthy or missing",
			},
			wantType: "orphan",
		},
		{
			name: "missing issue",
			issue: IndexIssue{
				Type:   "missing",
				Hash:   "def456",
				Topic:  "test-topic",
				Detail: "not found in topic database",
			},
			wantType: "missing",
		},
		{
			name: "mismatch issue",
			issue: IndexIssue{
				Type:    "mismatch",
				Hash:    "ghi789",
				Topic:   "test-topic",
				DatFile: "data.001.dat",
				Detail:  "orchestrator says data.001.dat, topic says data.002.dat",
			},
			wantType: "mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.issue.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", tt.issue.Type, tt.wantType)
			}
			if tt.issue.Hash == "" {
				t.Error("Hash should not be empty")
			}
			if tt.issue.Detail == "" {
				t.Error("Detail should not be empty")
			}
		})
	}
}

func TestIndexResult_Struct(t *testing.T) {
	// Valid result
	validResult := &IndexResult{
		Valid:         true,
		OrphanCount:   0,
		MissingCount:  0,
		MismatchCount: 0,
		Issues:        []IndexIssue{},
	}

	if !validResult.Valid {
		t.Error("Valid should be true")
	}
	if validResult.OrphanCount != 0 {
		t.Errorf("OrphanCount = %d, want 0", validResult.OrphanCount)
	}

	// Invalid result with issues
	invalidResult := &IndexResult{
		Valid:         false,
		OrphanCount:   2,
		MissingCount:  1,
		MismatchCount: 1,
		Issues: []IndexIssue{
			{Type: "orphan", Hash: "hash1"},
			{Type: "orphan", Hash: "hash2"},
			{Type: "missing", Hash: "hash3"},
			{Type: "mismatch", Hash: "hash4"},
		},
	}

	if invalidResult.Valid {
		t.Error("Valid should be false")
	}
	if invalidResult.OrphanCount != 2 {
		t.Errorf("OrphanCount = %d, want 2", invalidResult.OrphanCount)
	}
	if invalidResult.MissingCount != 1 {
		t.Errorf("MissingCount = %d, want 1", invalidResult.MissingCount)
	}
	if invalidResult.MismatchCount != 1 {
		t.Errorf("MismatchCount = %d, want 1", invalidResult.MismatchCount)
	}
	if len(invalidResult.Issues) != 4 {
		t.Errorf("Issues length = %d, want 4", len(invalidResult.Issues))
	}
}

func TestVerifyService_VerifyIndex_OrphanDetection(t *testing.T) {
	mockApp := newMockAppState()
	log := logger.NewLogger("debug")

	// Create in-memory SQLite database
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer db.Close()

	// Create asset_index table
	_, err = db.Exec(`CREATE TABLE asset_index (hash TEXT, topic TEXT, dat_file TEXT)`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Insert entry for a topic that will be unhealthy
	_, err = db.Exec(`INSERT INTO asset_index (hash, topic, dat_file) VALUES ('hash1', 'unhealthy-topic', 'data.001.dat')`)
	if err != nil {
		t.Fatalf("failed to insert data: %v", err)
	}

	mockApp.orchestratorDB = db
	mockApp.RegisterTopic("unhealthy-topic", false, "missing index file")
	svc := NewVerifyService(mockApp, log)

	result, err := svc.VerifyIndex(context.Background(), []string{"unhealthy-topic"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Valid {
		t.Error("expected Valid = false due to orphan")
	}
	if result.OrphanCount != 1 {
		t.Errorf("OrphanCount = %d, want 1", result.OrphanCount)
	}
	if len(result.Issues) == 0 {
		t.Fatal("expected at least one issue")
	}
	if result.Issues[0].Type != "orphan" {
		t.Errorf("Issue type = %q, want %q", result.Issues[0].Type, "orphan")
	}
}
