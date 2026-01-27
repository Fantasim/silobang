package audit

import (
	"database/sql"
	"encoding/json"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"silobang/internal/constants"
)

// createTestDB creates an in-memory SQLite database with the audit_log schema
func createTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory DB: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS audit_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp INTEGER NOT NULL,
			action TEXT NOT NULL,
			ip_address TEXT NOT NULL,
			username TEXT NOT NULL DEFAULT '',
			details_json TEXT,
			created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now'))
		);
		CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_log(action);
	`)
	if err != nil {
		t.Fatalf("failed to create audit_log table: %v", err)
	}

	return db
}

// newTestLogger creates a Logger for testing without the cleanup goroutine
func newTestLogger(t *testing.T) (*Logger, *sql.DB) {
	t.Helper()
	db := createTestDB(t)
	logger := NewLogger(db, constants.AuditMaxLogSizeBytes, constants.AuditPurgePercentage)
	t.Cleanup(func() {
		logger.Stop()
		db.Close()
	})
	return logger, db
}

func TestLogAndQueryNewActions(t *testing.T) {
	logger, db := newTestLogger(t)

	// Define all new actions with their detail structs
	testCases := []struct {
		action  string
		details interface{}
	}{
		{constants.AuditActionLoginSuccess, LoginSuccessDetails{UserAgent: "Mozilla/5.0"}},
		{constants.AuditActionLoginFailed, LoginFailedDetails{AttemptedUsername: "baduser", Reason: "AUTH_INVALID_CREDENTIALS", UserAgent: "curl/7.88"}},
		{constants.AuditActionLogout, LogoutDetails{}},
		{constants.AuditActionUserCreated, UserCreatedDetails{CreatedUserID: 42, CreatedUsername: "newuser"}},
		{constants.AuditActionUserUpdated, UserUpdatedDetails{TargetUserID: 42, TargetUsername: "newuser", FieldsChanged: []string{"display_name", "password"}}},
		{constants.AuditActionAPIKeyRegenerated, APIKeyRegeneratedDetails{TargetUserID: 42, TargetUsername: "newuser"}},
		{constants.AuditActionGrantCreated, GrantCreatedDetails{GrantID: 1, TargetUserID: 42, Action: "read", HasConstraints: true}},
		{constants.AuditActionGrantUpdated, GrantUpdatedDetails{GrantID: 1, TargetUserID: 42, Action: "read", HasConstraints: false}},
		{constants.AuditActionGrantRevoked, GrantRevokedDetails{GrantID: 1, TargetUserID: 42, Action: "read"}},
		{constants.AuditActionMetadataSet, MetadataSetDetails{Hash: "abc123def456", Op: "set", Key: "tag"}},
		{constants.AuditActionMetadataBatch, MetadataBatchDetails{OperationCount: 50, Succeeded: 48, Failed: 2, Processor: "api"}},
		{constants.AuditActionMetadataApply, MetadataApplyDetails{QueryPreset: "all_assets", Op: "set", Key: "status", OperationCount: 100, Succeeded: 95, Failed: 5, Processor: "pipeline"}},
		{constants.AuditActionConfigChanged, ConfigChangedDetails{WorkingDirectory: "/data/silobang", IsBootstrap: true}},
	}

	for _, tc := range testCases {
		err := logger.Log(tc.action, "127.0.0.1", "admin", tc.details)
		if err != nil {
			t.Fatalf("Log(%q) failed: %v", tc.action, err)
		}
	}

	// Verify each action was stored and is queryable
	for _, tc := range testCases {
		t.Run(tc.action, func(t *testing.T) {
			entries, err := Query(db, QueryOptions{Action: tc.action})
			if err != nil {
				t.Fatalf("Query(action=%q) failed: %v", tc.action, err)
			}
			if len(entries) != 1 {
				t.Fatalf("expected 1 entry for action %q, got %d", tc.action, len(entries))
			}

			entry := entries[0]
			if entry.Action != tc.action {
				t.Errorf("entry.Action = %q, want %q", entry.Action, tc.action)
			}
			if entry.IPAddress != "127.0.0.1" {
				t.Errorf("entry.IPAddress = %q, want %q", entry.IPAddress, "127.0.0.1")
			}
			if entry.Username != "admin" {
				t.Errorf("entry.Username = %q, want %q", entry.Username, "admin")
			}
			if entry.Details == nil && tc.action != constants.AuditActionLogout {
				t.Errorf("entry.Details is nil for %q", tc.action)
			}
		})
	}
}

func TestLogInvalidActionRejected(t *testing.T) {
	logger, _ := newTestLogger(t)

	err := logger.Log("invalid_action", "127.0.0.1", "admin", nil)
	if err == nil {
		t.Error("expected error for invalid action, got nil")
	}
}

func TestLogDetailsSerialization(t *testing.T) {
	logger, db := newTestLogger(t)

	// Log a complex detail struct
	details := MetadataApplyDetails{
		QueryPreset:    "my_preset",
		Op:             "set",
		Key:            "category",
		OperationCount: 250,
		Succeeded:      245,
		Failed:         5,
		Processor:      "batch_pipeline",
	}

	err := logger.Log(constants.AuditActionMetadataApply, "10.0.0.1", "pipeline_user", details)
	if err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	entries, err := Query(db, QueryOptions{Action: constants.AuditActionMetadataApply})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	// Details come back as map[string]interface{} from JSON unmarshaling
	detailsMap, ok := entries[0].Details.(map[string]interface{})
	if !ok {
		t.Fatalf("details is not map[string]interface{}, got %T", entries[0].Details)
	}

	if detailsMap["query_preset"] != "my_preset" {
		t.Errorf("query_preset = %v, want %q", detailsMap["query_preset"], "my_preset")
	}
	if detailsMap["op"] != "set" {
		t.Errorf("op = %v, want %q", detailsMap["op"], "set")
	}
	if detailsMap["key"] != "category" {
		t.Errorf("key = %v, want %q", detailsMap["key"], "category")
	}
	// JSON numbers come back as float64
	if detailsMap["operation_count"] != float64(250) {
		t.Errorf("operation_count = %v, want 250", detailsMap["operation_count"])
	}
	if detailsMap["succeeded"] != float64(245) {
		t.Errorf("succeeded = %v, want 245", detailsMap["succeeded"])
	}
	if detailsMap["failed"] != float64(5) {
		t.Errorf("failed = %v, want 5", detailsMap["failed"])
	}
	if detailsMap["processor"] != "batch_pipeline" {
		t.Errorf("processor = %v, want %q", detailsMap["processor"], "batch_pipeline")
	}
}

func TestLogLoginFailedDetailsSerialization(t *testing.T) {
	logger, db := newTestLogger(t)

	details := LoginFailedDetails{
		AttemptedUsername: "hacker",
		Reason:           "AUTH_INVALID_CREDENTIALS",
		UserAgent:        "curl/7.88.1",
	}

	err := logger.Log(constants.AuditActionLoginFailed, "192.168.1.100", "", details)
	if err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	entries, err := Query(db, QueryOptions{Action: constants.AuditActionLoginFailed})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	// Verify username is empty (failed login has no authenticated user)
	if entries[0].Username != "" {
		t.Errorf("username should be empty for failed login, got %q", entries[0].Username)
	}

	detailsMap, ok := entries[0].Details.(map[string]interface{})
	if !ok {
		t.Fatalf("details is not map[string]interface{}, got %T", entries[0].Details)
	}

	if detailsMap["attempted_username"] != "hacker" {
		t.Errorf("attempted_username = %v, want %q", detailsMap["attempted_username"], "hacker")
	}
	if detailsMap["reason"] != "AUTH_INVALID_CREDENTIALS" {
		t.Errorf("reason = %v, want %q", detailsMap["reason"], "AUTH_INVALID_CREDENTIALS")
	}
	if detailsMap["user_agent"] != "curl/7.88.1" {
		t.Errorf("user_agent = %v, want %q", detailsMap["user_agent"], "curl/7.88.1")
	}
}

func TestLogUserUpdatedFieldsChanged(t *testing.T) {
	logger, db := newTestLogger(t)

	details := UserUpdatedDetails{
		TargetUserID:   5,
		TargetUsername: "testuser",
		FieldsChanged:  []string{"display_name", "password"},
	}

	err := logger.Log(constants.AuditActionUserUpdated, "10.0.0.1", "admin", details)
	if err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	entries, err := Query(db, QueryOptions{Action: constants.AuditActionUserUpdated})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	detailsMap := entries[0].Details.(map[string]interface{})

	// fields_changed should be a JSON array
	fieldsRaw, ok := detailsMap["fields_changed"]
	if !ok {
		t.Fatal("fields_changed not found in details")
	}

	fields, ok := fieldsRaw.([]interface{})
	if !ok {
		t.Fatalf("fields_changed is not []interface{}, got %T", fieldsRaw)
	}

	if len(fields) != 2 {
		t.Fatalf("expected 2 fields_changed, got %d", len(fields))
	}
	if fields[0] != "display_name" {
		t.Errorf("fields_changed[0] = %v, want %q", fields[0], "display_name")
	}
	if fields[1] != "password" {
		t.Errorf("fields_changed[1] = %v, want %q", fields[1], "password")
	}
}

func TestLogGrantCreatedConstraintsFlag(t *testing.T) {
	logger, db := newTestLogger(t)

	// Test with constraints
	err := logger.Log(constants.AuditActionGrantCreated, "10.0.0.1", "admin",
		GrantCreatedDetails{GrantID: 1, TargetUserID: 5, Action: "upload", HasConstraints: true})
	if err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	// Test without constraints
	err = logger.Log(constants.AuditActionGrantCreated, "10.0.0.1", "admin",
		GrantCreatedDetails{GrantID: 2, TargetUserID: 5, Action: "read", HasConstraints: false})
	if err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	entries, err := Query(db, QueryOptions{Action: constants.AuditActionGrantCreated})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// Entries are ordered DESC by id, so newest first
	noConstraints := entries[0].Details.(map[string]interface{})
	withConstraints := entries[1].Details.(map[string]interface{})

	if withConstraints["has_constraints"] != true {
		t.Errorf("expected has_constraints=true, got %v", withConstraints["has_constraints"])
	}
	if noConstraints["has_constraints"] != false {
		t.Errorf("expected has_constraints=false, got %v", noConstraints["has_constraints"])
	}
}

func TestLogConfigChangedBootstrapFlag(t *testing.T) {
	logger, db := newTestLogger(t)

	// Bootstrap config
	err := logger.Log(constants.AuditActionConfigChanged, "127.0.0.1", "", ConfigChangedDetails{
		WorkingDirectory: "/data/project",
		IsBootstrap:      true,
	})
	if err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	entries, err := Query(db, QueryOptions{Action: constants.AuditActionConfigChanged})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	detailsMap := entries[0].Details.(map[string]interface{})
	if detailsMap["working_directory"] != "/data/project" {
		t.Errorf("working_directory = %v, want %q", detailsMap["working_directory"], "/data/project")
	}
	if detailsMap["is_bootstrap"] != true {
		t.Errorf("is_bootstrap = %v, want true", detailsMap["is_bootstrap"])
	}
}

func TestLogNilDetails(t *testing.T) {
	logger, db := newTestLogger(t)

	err := logger.Log(constants.AuditActionLogout, "127.0.0.1", "admin", nil)
	if err != nil {
		t.Fatalf("Log with nil details failed: %v", err)
	}

	entries, err := Query(db, QueryOptions{Action: constants.AuditActionLogout})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	if entries[0].Details != nil {
		t.Errorf("expected nil details, got %v", entries[0].Details)
	}
}

func TestLogEmptyStructDetails(t *testing.T) {
	logger, db := newTestLogger(t)

	// LogoutDetails is an empty struct — it should still serialize as {}
	err := logger.Log(constants.AuditActionLogout, "127.0.0.1", "admin", LogoutDetails{})
	if err != nil {
		t.Fatalf("Log with empty struct failed: %v", err)
	}

	entries, err := Query(db, QueryOptions{Action: constants.AuditActionLogout})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	// Empty struct serializes to {} which unmarshals as map[string]interface{}
	if entries[0].Details == nil {
		t.Error("expected non-nil details for empty struct (should be {})")
	}
}

func TestLogSubscribersReceiveNewActions(t *testing.T) {
	logger, _ := newTestLogger(t)

	ch := logger.Subscribe()
	defer logger.Unsubscribe(ch)

	// Log a new action
	err := logger.Log(constants.AuditActionLoginSuccess, "10.0.0.1", "admin",
		LoginSuccessDetails{UserAgent: "test-agent"})
	if err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	// Subscriber should receive the entry
	entry := <-ch
	if entry.Action != constants.AuditActionLoginSuccess {
		t.Errorf("subscriber received action %q, want %q", entry.Action, constants.AuditActionLoginSuccess)
	}
	if entry.Username != "admin" {
		t.Errorf("subscriber received username %q, want %q", entry.Username, "admin")
	}

	// Details in subscriber events are the original struct, not JSON-unmarshaled map
	details, ok := entry.Details.(LoginSuccessDetails)
	if !ok {
		t.Fatalf("subscriber details is not LoginSuccessDetails, got %T", entry.Details)
	}
	if details.UserAgent != "test-agent" {
		t.Errorf("subscriber details.UserAgent = %q, want %q", details.UserAgent, "test-agent")
	}
}

func TestLogAllOriginalActionsStillWork(t *testing.T) {
	logger, db := newTestLogger(t)

	// Verify original actions still log correctly alongside new ones
	originalCases := []struct {
		action  string
		details interface{}
	}{
		{constants.AuditActionConnected, ConnectedDetails{UserAgent: "browser"}},
		{constants.AuditActionAddingTopic, AddingTopicDetails{TopicName: "models"}},
		{constants.AuditActionQuerying, QueryingDetails{Preset: "all", RowCount: 100}},
		{constants.AuditActionAddingFile, AddingFileDetails{Hash: "abc", TopicName: "t", Filename: "f.obj", Size: 1024}},
		{constants.AuditActionVerified, VerifiedDetails{TopicsChecked: 2, TopicsValid: 2, IndexValid: true, DurationMs: 150}},
		{constants.AuditActionDownloaded, DownloadedDetails{Hash: "abc", Topic: "t", Filename: "f.obj", Size: 1024}},
		{constants.AuditActionDownloadedBulk, DownloadedBulkDetails{Mode: "stream", AssetCount: 10, TotalSize: 10240}},
		{constants.AuditActionReconcileTopicRemoved, ReconcileTopicRemovedDetails{TopicName: "old", EntriesPurged: 5}},
	}

	for _, tc := range originalCases {
		err := logger.Log(tc.action, "127.0.0.1", "admin", tc.details)
		if err != nil {
			t.Errorf("Log(%q) failed: %v", tc.action, err)
		}
	}

	// Verify all were stored
	for _, tc := range originalCases {
		entries, err := Query(db, QueryOptions{Action: tc.action})
		if err != nil {
			t.Errorf("Query(%q) failed: %v", tc.action, err)
			continue
		}
		if len(entries) != 1 {
			t.Errorf("expected 1 entry for %q, got %d", tc.action, len(entries))
		}
	}
}

func TestDetailStructsJSONTags(t *testing.T) {
	// Verify that detail structs serialize with expected JSON keys
	tests := []struct {
		name       string
		details    interface{}
		expectKeys []string
	}{
		{
			"LoginSuccessDetails",
			LoginSuccessDetails{UserAgent: "test"},
			[]string{"user_agent"},
		},
		{
			"LoginFailedDetails",
			LoginFailedDetails{AttemptedUsername: "u", Reason: "r", UserAgent: "ua"},
			[]string{"attempted_username", "reason", "user_agent"},
		},
		{
			"UserCreatedDetails",
			UserCreatedDetails{CreatedUserID: 1, CreatedUsername: "u"},
			[]string{"created_user_id", "created_username"},
		},
		{
			"UserUpdatedDetails",
			UserUpdatedDetails{TargetUserID: 1, TargetUsername: "u", FieldsChanged: []string{"a"}},
			[]string{"target_user_id", "target_username", "fields_changed"},
		},
		{
			"APIKeyRegeneratedDetails",
			APIKeyRegeneratedDetails{TargetUserID: 1, TargetUsername: "u"},
			[]string{"target_user_id", "target_username"},
		},
		{
			"GrantCreatedDetails",
			GrantCreatedDetails{GrantID: 1, TargetUserID: 2, Action: "a", HasConstraints: true},
			[]string{"grant_id", "target_user_id", "action", "has_constraints"},
		},
		{
			"GrantUpdatedDetails",
			GrantUpdatedDetails{GrantID: 1, TargetUserID: 2, Action: "a", HasConstraints: false},
			[]string{"grant_id", "target_user_id", "action", "has_constraints"},
		},
		{
			"GrantRevokedDetails",
			GrantRevokedDetails{GrantID: 1, TargetUserID: 2, Action: "a"},
			[]string{"grant_id", "target_user_id", "action"},
		},
		{
			"MetadataSetDetails",
			MetadataSetDetails{Hash: "h", Op: "set", Key: "k"},
			[]string{"hash", "op", "key"},
		},
		{
			"MetadataBatchDetails",
			MetadataBatchDetails{OperationCount: 1, Succeeded: 1, Failed: 0, Processor: "p"},
			[]string{"operation_count", "succeeded", "failed", "processor"},
		},
		{
			"MetadataApplyDetails",
			MetadataApplyDetails{QueryPreset: "q", Op: "set", Key: "k", OperationCount: 1, Succeeded: 1, Failed: 0, Processor: "p"},
			[]string{"query_preset", "op", "key", "operation_count", "succeeded", "failed", "processor"},
		},
		{
			"ConfigChangedDetails",
			ConfigChangedDetails{WorkingDirectory: "/d", IsBootstrap: true},
			[]string{"working_directory", "is_bootstrap"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.details)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var m map[string]interface{}
			if err := json.Unmarshal(data, &m); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			for _, key := range tt.expectKeys {
				if _, ok := m[key]; !ok {
					t.Errorf("expected JSON key %q not found in %s", key, tt.name)
				}
			}
		})
	}
}
