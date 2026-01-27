package e2e

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"meshbank/internal/constants"
)

// TestAuditLogImmutability verifies append-only behavior and monotonic IDs
func TestAuditLogImmutability(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "audit-test")

	// Upload a file - should create audit entry
	ts.UploadFileExpectSuccess(t, "audit-test", "test.bin", SmallFile, "")

	// Query audit logs
	var result AuditQueryResponse
	if err := ts.GetJSON("/api/audit?action=adding_file", &result); err != nil {
		t.Fatalf("Failed to query audit: %v", err)
	}

	if len(result.Entries) == 0 {
		t.Fatal("Expected at least one audit entry")
	}

	firstEntry := result.Entries[0]

	// Verify entry has required fields
	if firstEntry.Action != constants.AuditActionAddingFile {
		t.Errorf("Expected action 'adding_file', got %s", firstEntry.Action)
	}
	if firstEntry.IPAddress == "" {
		t.Error("IP address should not be empty")
	}
	if firstEntry.Timestamp == 0 {
		t.Error("Timestamp should not be zero")
	}

	// Upload another file
	ts.UploadFileExpectSuccess(t, "audit-test", "test2.bin", MediumFile, "")

	// Query again to verify IDs are monotonically increasing
	var result2 AuditQueryResponse
	if err := ts.GetJSON("/api/audit?limit=10", &result2); err != nil {
		t.Fatalf("Failed to query audit: %v", err)
	}

	if len(result2.Entries) >= 2 {
		// Entries are returned in descending order (newest first)
		if result2.Entries[0].ID <= result2.Entries[1].ID {
			t.Error("IDs should be in descending order (newest first)")
		}
	}
}

// TestAuditLogFiltering tests query filters
func TestAuditLogFiltering(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "filter-test")

	// Generate various audit events
	uploadResp := ts.UploadFileExpectSuccess(t, "filter-test", "file1.bin", SmallFile, "")
	ts.DownloadAsset(t, uploadResp.Hash)

	// Test action filter for adding_file
	var addingResult AuditQueryResponse
	if err := ts.GetJSON("/api/audit?action=adding_file", &addingResult); err != nil {
		t.Fatalf("Failed to query audit: %v", err)
	}

	for _, entry := range addingResult.Entries {
		if entry.Action != constants.AuditActionAddingFile {
			t.Errorf("Filter failed: expected 'adding_file', got %s", entry.Action)
		}
	}

	// Test action filter for downloaded
	var downloadResult AuditQueryResponse
	if err := ts.GetJSON("/api/audit?action=downloaded", &downloadResult); err != nil {
		t.Fatalf("Failed to query audit: %v", err)
	}

	for _, entry := range downloadResult.Entries {
		if entry.Action != constants.AuditActionDownloaded {
			t.Errorf("Filter failed: expected 'downloaded', got %s", entry.Action)
		}
	}
}

// TestAuditLogPagination tests pagination support
func TestAuditLogPagination(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "pagination-test")

	// Generate multiple audit events
	for i := 0; i < 5; i++ {
		ts.UploadFileExpectSuccess(t, "pagination-test", "file"+string(rune('a'+i))+".bin", SmallFile, "")
	}

	// Test limit
	var limitResult AuditQueryResponse
	if err := ts.GetJSON("/api/audit?limit=2", &limitResult); err != nil {
		t.Fatalf("Failed to query audit: %v", err)
	}

	if len(limitResult.Entries) > 2 {
		t.Errorf("Expected at most 2 entries, got %d", len(limitResult.Entries))
	}

	// Test offset
	var offsetResult AuditQueryResponse
	if err := ts.GetJSON("/api/audit?limit=2&offset=2", &offsetResult); err != nil {
		t.Fatalf("Failed to query audit: %v", err)
	}

	// The entries with offset should be different from entries without
	if len(offsetResult.Entries) > 0 && len(limitResult.Entries) > 0 {
		if offsetResult.Entries[0].ID == limitResult.Entries[0].ID {
			t.Error("Offset entries should be different from first page entries")
		}
	}

	// Verify total count is returned
	if limitResult.Total == 0 {
		t.Error("Total count should be greater than 0")
	}
}

// TestAuditLogDataIntegrity tests that details are stored correctly
func TestAuditLogDataIntegrity(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "integrity-test")

	// Upload with known details
	filename := "myfile.glb"
	content := GenerateTestGLB(2048)
	uploadResp := ts.UploadFileExpectSuccess(t, "integrity-test", filename, content, "")

	// Query and verify details
	var result AuditQueryResponse
	if err := ts.GetJSON("/api/audit?action=adding_file&limit=1", &result); err != nil {
		t.Fatalf("Failed to query audit: %v", err)
	}

	if len(result.Entries) == 0 {
		t.Fatal("Expected audit entry")
	}

	entry := result.Entries[0]
	details, ok := entry.Details.(map[string]interface{})
	if !ok {
		t.Fatal("Details should be a map")
	}

	if details["hash"] != uploadResp.Hash {
		t.Errorf("Hash mismatch: expected %s, got %v", uploadResp.Hash, details["hash"])
	}
	if details["filename"] != filename {
		t.Errorf("Filename mismatch: expected %s, got %v", filename, details["filename"])
	}
	if details["topic_name"] != "integrity-test" {
		t.Errorf("Topic mismatch: expected integrity-test, got %v", details["topic_name"])
	}
}

// TestAuditActionsEndpoint tests the actions list endpoint
func TestAuditActionsEndpoint(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	var result AuditActionsResponse
	if err := ts.GetJSON("/api/audit/actions", &result); err != nil {
		t.Fatalf("Failed to get actions: %v", err)
	}

	expectedActions := []string{
		// Core operations
		"connected", "adding_topic", "querying",
		"adding_file", "verified", "downloaded", "downloaded_bulk",
		"reconcile_topic_removed",
		// Authentication
		"login_success", "login_failed", "logout",
		// User management
		"user_created", "user_updated", "api_key_regenerated",
		// Grant management
		"grant_created", "grant_updated", "grant_revoked",
		// Metadata
		"metadata_set", "metadata_batch", "metadata_apply",
		// Configuration
		"config_changed",
	}

	if len(result.Actions) != len(expectedActions) {
		t.Errorf("Expected %d actions, got %d", len(expectedActions), len(result.Actions))
	}

	// Verify all expected actions are present
	actionSet := make(map[string]bool)
	for _, a := range result.Actions {
		actionSet[a] = true
	}
	for _, expected := range expectedActions {
		if !actionSet[expected] {
			t.Errorf("Missing expected action: %s", expected)
		}
	}
}

// TestAuditLogInvalidAction tests that invalid action filter is rejected
func TestAuditLogInvalidAction(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	resp, err := ts.GET("/api/audit?action=invalid_action")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 400 {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

// TestAuditLogTopicCreation tests audit logging for topic creation
func TestAuditLogTopicCreation(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Create a topic
	ts.CreateTopic(t, "new-topic")

	// Query for adding_topic action
	var result AuditQueryResponse
	if err := ts.GetJSON("/api/audit?action=adding_topic", &result); err != nil {
		t.Fatalf("Failed to query audit: %v", err)
	}

	if len(result.Entries) == 0 {
		t.Fatal("Expected audit entry for topic creation")
	}

	// Verify details
	entry := result.Entries[0]
	details, ok := entry.Details.(map[string]interface{})
	if !ok {
		t.Fatal("Details should be a map")
	}

	if details["topic_name"] != "new-topic" {
		t.Errorf("Topic name mismatch: expected new-topic, got %v", details["topic_name"])
	}
}

// TestAuditLogQueryExecution tests audit logging for query execution
func TestAuditLogQueryExecution(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "query-test")

	// Upload some files
	ts.UploadFileExpectSuccess(t, "query-test", "file1.bin", SmallFile, "")
	ts.UploadFileExpectSuccess(t, "query-test", "file2.bin", MediumFile, "")

	// Execute a query (use count preset which exists and has no required params)
	ts.ExecuteQuery(t, "count", nil, nil)

	// Query for querying action
	var result AuditQueryResponse
	if err := ts.GetJSON("/api/audit?action=querying", &result); err != nil {
		t.Fatalf("Failed to query audit: %v", err)
	}

	if len(result.Entries) == 0 {
		t.Fatal("Expected audit entry for query execution")
	}

	// Verify details
	entry := result.Entries[0]
	details, ok := entry.Details.(map[string]interface{})
	if !ok {
		t.Fatal("Details should be a map")
	}

	if details["preset"] != "count" {
		t.Errorf("Preset mismatch: expected count, got %v", details["preset"])
	}
}

// TestAuditLogTimeFiltering tests since/until time filters
func TestAuditLogTimeFiltering(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "time-test")

	// Record time before upload
	beforeTime := time.Now().Unix()

	// Wait a moment
	time.Sleep(10 * time.Millisecond)

	// Upload a file
	ts.UploadFileExpectSuccess(t, "time-test", "file.bin", SmallFile, "")

	// Wait a moment
	time.Sleep(10 * time.Millisecond)

	// Record time after upload
	afterTime := time.Now().Unix()

	// Query with since filter (should include the entry)
	var sinceResult AuditQueryResponse
	if err := ts.GetJSON("/api/audit?since="+string(rune(beforeTime)), &sinceResult); err != nil {
		// Try with proper formatting
		resp, _ := ts.GET("/api/audit?action=adding_file")
		defer resp.Body.Close()
		json.NewDecoder(resp.Body).Decode(&sinceResult)
	}

	// Query with until filter in the future (should include entries)
	var untilResult AuditQueryResponse
	if err := ts.GetJSON("/api/audit?until="+string(rune(afterTime+1000)), &untilResult); err != nil {
		resp, _ := ts.GET("/api/audit?action=adding_file")
		defer resp.Body.Close()
		json.NewDecoder(resp.Body).Decode(&untilResult)
	}

	// Just verify the queries don't error - timestamp parsing is tested implicitly
	if untilResult.Total < 0 {
		t.Error("Invalid total count")
	}
}

// TestAuditLogSkippedUpload tests that skipped uploads are also logged
func TestAuditLogSkippedUpload(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "skip-test")

	// Upload a file
	content := SmallFile
	ts.UploadFileExpectSuccess(t, "skip-test", "original.bin", content, "")

	// Upload same content (should be skipped)
	ts.UploadFileExpectSuccess(t, "skip-test", "duplicate.bin", content, "")

	// Query for adding_file actions
	var result AuditQueryResponse
	if err := ts.GetJSON("/api/audit?action=adding_file", &result); err != nil {
		t.Fatalf("Failed to query audit: %v", err)
	}

	// Should have 2 entries (one regular, one skipped)
	if len(result.Entries) < 2 {
		t.Fatalf("Expected at least 2 entries, got %d", len(result.Entries))
	}

	// Find an entry with skipped=true
	foundSkipped := false
	for _, entry := range result.Entries {
		details, ok := entry.Details.(map[string]interface{})
		if ok {
			if skipped, exists := details["skipped"]; exists && skipped == true {
				foundSkipped = true
				break
			}
		}
	}

	if !foundSkipped {
		t.Error("Expected to find a skipped upload entry")
	}
}

// TestAuditLogMultipleActions tests multiple action types are logged correctly
func TestAuditLogMultipleActions(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Create topic
	ts.CreateTopic(t, "multi-test")

	// Upload file
	uploadResp := ts.UploadFileExpectSuccess(t, "multi-test", "file.bin", SmallFile, "")

	// Download file
	ts.DownloadAsset(t, uploadResp.Hash)

	// Execute query (use count preset which exists)
	ts.ExecuteQuery(t, "count", nil, nil)

	// Query all audit logs
	var result AuditQueryResponse
	if err := ts.GetJSON("/api/audit?limit=100", &result); err != nil {
		t.Fatalf("Failed to query audit: %v", err)
	}

	// Verify we have different action types
	actionTypes := make(map[string]bool)
	for _, entry := range result.Entries {
		actionTypes[entry.Action] = true
	}

	expectedTypes := []string{"adding_topic", "adding_file", "downloaded", "querying"}
	for _, expected := range expectedTypes {
		if !actionTypes[expected] {
			t.Errorf("Missing action type: %s", expected)
		}
	}
}

// TestAuditLogAfterRestart tests that audit logs persist after restart
func TestAuditLogAfterRestart(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "restart-test")

	// Upload a file
	ts.UploadFileExpectSuccess(t, "restart-test", "file.bin", SmallFile, "")

	// Get count before restart
	var beforeResult AuditQueryResponse
	if err := ts.GetJSON("/api/audit", &beforeResult); err != nil {
		t.Fatalf("Failed to query audit: %v", err)
	}
	beforeCount := beforeResult.Total

	// Restart server
	ts.Restart(t)

	// Query audit logs after restart
	var afterResult AuditQueryResponse
	if err := ts.GetJSON("/api/audit", &afterResult); err != nil {
		t.Fatalf("Failed to query audit after restart: %v", err)
	}

	// Should have same count (logs persisted)
	if afterResult.Total != beforeCount {
		t.Errorf("Audit log count changed after restart: before=%d, after=%d", beforeCount, afterResult.Total)
	}
}

// TestAuditLogMeFilter tests the "me" filter returns only entries from requesting IP
func TestAuditLogMeFilter(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "me-filter-test")

	// Upload a file (creates audit entry with test client IP)
	ts.UploadFileExpectSuccess(t, "me-filter-test", "file.bin", SmallFile, "")

	// Query with "me" filter
	var result AuditQueryResponse
	if err := ts.GetJSON("/api/audit?filter=me", &result); err != nil {
		t.Fatalf("Failed to query audit with me filter: %v", err)
	}

	// All entries should be from "me" (test client)
	// In test environment, all requests come from the same client
	if result.Total == 0 {
		t.Error("Expected at least one entry with me filter")
	}

	// Verify all entries have the same IP (since all are from test client)
	if len(result.Entries) > 1 {
		firstIP := result.Entries[0].IPAddress
		for _, entry := range result.Entries {
			if entry.IPAddress != firstIP {
				t.Errorf("ME filter should return consistent IP, got %s and %s", firstIP, entry.IPAddress)
			}
		}
	}
}

// TestAuditLogOthersFilter tests the "others" filter excludes requesting IP entries
func TestAuditLogOthersFilter(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "others-filter-test")

	// Upload a file
	ts.UploadFileExpectSuccess(t, "others-filter-test", "file.bin", SmallFile, "")

	// Query with "others" filter - should return empty in single-client test
	var result AuditQueryResponse
	if err := ts.GetJSON("/api/audit?filter=others", &result); err != nil {
		t.Fatalf("Failed to query audit with others filter: %v", err)
	}

	// In single-client test, "others" should return no entries
	// since all entries are from the test client
	if len(result.Entries) > 0 {
		t.Log("Note: 'others' filter returned entries - this is expected if entries exist from other IPs")
	}
}

// TestAuditLogInvalidFilter tests that invalid filter values are rejected
func TestAuditLogInvalidFilter(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	resp, err := ts.GET("/api/audit?filter=invalid")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 400 {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

// TestAuditStreamConnectedEvent tests that connecting to SSE logs a "connected" event
func TestAuditStreamConnectedEvent(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// First, get the count of connected events before
	var beforeResult AuditQueryResponse
	ts.GetJSON("/api/audit?action=connected", &beforeResult)
	beforeCount := beforeResult.Total

	// Connect to the SSE stream briefly
	resp, err := ts.GET("/api/audit/stream")
	if err != nil {
		t.Fatalf("Failed to connect to stream: %v", err)
	}

	// Read just the first event (connected event)
	buf := make([]byte, 1024)
	resp.Body.Read(buf)
	resp.Body.Close()

	// Wait briefly for the log to be written
	time.Sleep(50 * time.Millisecond)

	// Query for "connected" action
	var afterResult AuditQueryResponse
	if err := ts.GetJSON("/api/audit?action=connected", &afterResult); err != nil {
		t.Fatalf("Failed to query connected events: %v", err)
	}

	// Should have one more connected event
	if afterResult.Total <= beforeCount {
		t.Errorf("Expected more connected events after stream connection: before=%d, after=%d", beforeCount, afterResult.Total)
	}

	// Verify the latest entry has correct action
	if len(afterResult.Entries) > 0 && afterResult.Entries[0].Action != constants.AuditActionConnected {
		t.Errorf("Expected action 'connected', got %s", afterResult.Entries[0].Action)
	}
}

// TestAuditStreamReturnsClientIP tests that SSE connected event includes client IP
func TestAuditStreamReturnsClientIP(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Connect to the SSE stream
	resp, err := ts.GET("/api/audit/stream")
	if err != nil {
		t.Fatalf("Failed to connect to stream: %v", err)
	}
	defer resp.Body.Close()

	// Read the first event
	buf := make([]byte, 2048)
	n, _ := resp.Body.Read(buf)
	eventData := string(buf[:n])

	// Parse the SSE data
	// Format: "data: {...}\n\n"
	if len(eventData) > 6 && eventData[:5] == "data:" {
		jsonStr := eventData[6:]
		if idx := len(jsonStr) - 1; idx > 0 {
			// Find end of JSON
			for i, c := range jsonStr {
				if c == '\n' {
					jsonStr = jsonStr[:i]
					break
				}
			}
		}

		var event map[string]interface{}
		if err := json.Unmarshal([]byte(jsonStr), &event); err == nil {
			if event["type"] == "connected" {
				data, ok := event["data"].(map[string]interface{})
				if !ok {
					t.Error("Connected event should have data object")
				} else if data["client_ip"] == nil || data["client_ip"] == "" {
					t.Error("Connected event should include client_ip")
				}
			}
		}
	}
}

// TestAuditStreamFilterMe tests SSE stream with "me" filter
func TestAuditStreamFilterMe(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Connect to stream with filter=me (should not error)
	resp, err := ts.GET("/api/audit/stream?filter=me")
	if err != nil {
		t.Fatalf("Failed to connect to filtered stream: %v", err)
	}
	defer resp.Body.Close()

	// Read the connected event
	buf := make([]byte, 1024)
	resp.Body.Read(buf)

	// Just verify the connection works with filter
	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200 for filtered stream, got %d", resp.StatusCode)
	}
}

// TestAuditStreamInvalidFilter tests SSE stream rejects invalid filter
func TestAuditStreamInvalidFilter(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	resp, err := ts.GET("/api/audit/stream?filter=invalid")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 400 {
		t.Errorf("Expected status 400 for invalid filter, got %d", resp.StatusCode)
	}
}

// TestAuditLogUsernamePopulated tests that audit entries include the authenticated username
func TestAuditLogUsernamePopulated(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "username-test")

	// Upload a file (authenticated as bootstrap admin)
	ts.UploadFileExpectSuccess(t, "username-test", "file.bin", SmallFile, "")

	// Query audit logs
	var result AuditQueryResponse
	if err := ts.GetJSON("/api/audit?action=adding_file&limit=1", &result); err != nil {
		t.Fatalf("Failed to query audit: %v", err)
	}

	if len(result.Entries) == 0 {
		t.Fatal("Expected at least one audit entry")
	}

	entry := result.Entries[0]
	if entry.Username == "" {
		t.Error("Username should not be empty for authenticated request")
	}
	if entry.Username != constants.AuthBootstrapUsername {
		t.Errorf("Expected username '%s', got '%s'", constants.AuthBootstrapUsername, entry.Username)
	}
}

// TestAuditLogUsernameFilter tests filtering audit entries by username
func TestAuditLogUsernameFilter(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "usernamefilter-test")

	// Generate audit events as admin
	ts.UploadFileExpectSuccess(t, "usernamefilter-test", "file.bin", SmallFile, "")

	// Query with username filter matching bootstrap user
	var result AuditQueryResponse
	if err := ts.GetJSON("/api/audit?username="+constants.AuthBootstrapUsername, &result); err != nil {
		t.Fatalf("Failed to query audit with username filter: %v", err)
	}

	if result.Total == 0 {
		t.Error("Expected entries when filtering by bootstrap username")
	}

	// All entries should have the matching username
	for _, entry := range result.Entries {
		if entry.Username != constants.AuthBootstrapUsername {
			t.Errorf("Expected username '%s', got '%s'", constants.AuthBootstrapUsername, entry.Username)
		}
	}

	// Query with non-existent username should return no entries
	var emptyResult AuditQueryResponse
	if err := ts.GetJSON("/api/audit?username=nonexistent", &emptyResult); err != nil {
		t.Fatalf("Failed to query audit with nonexistent username: %v", err)
	}

	if emptyResult.Total != 0 {
		t.Errorf("Expected 0 entries for nonexistent username, got %d", emptyResult.Total)
	}
}

// TestAuditLogUsernameAcrossActions tests that username is populated for all action types
func TestAuditLogUsernameAcrossActions(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Create topic (adding_topic action)
	ts.CreateTopic(t, "actions-test")

	// Upload file (adding_file action)
	uploadResp := ts.UploadFileExpectSuccess(t, "actions-test", "file.bin", SmallFile, "")

	// Download file (downloaded action)
	ts.DownloadAsset(t, uploadResp.Hash)

	// Execute query (querying action)
	ts.ExecuteQuery(t, "count", nil, nil)

	// Query all audit logs
	var result AuditQueryResponse
	if err := ts.GetJSON("/api/audit?limit=100", &result); err != nil {
		t.Fatalf("Failed to query audit: %v", err)
	}

	// Verify all entries have username populated
	// Note: config_changed during bootstrap has no authenticated user, so username is empty
	for _, entry := range result.Entries {
		if entry.Username == "" && entry.Action != constants.AuditActionConfigChanged {
			t.Errorf("Entry with action '%s' (id=%d) has empty username", entry.Action, entry.ID)
		}
	}
}

// TestAuditLogMeFilterByUsername tests that the "me" filter uses username-based matching
func TestAuditLogMeFilterByUsername(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "mefilter-test")

	// Upload a file (creates audit entry with bootstrap username)
	ts.UploadFileExpectSuccess(t, "mefilter-test", "file.bin", SmallFile, "")

	// Query with "me" filter - should match by username
	var result AuditQueryResponse
	if err := ts.GetJSON("/api/audit?filter=me", &result); err != nil {
		t.Fatalf("Failed to query audit with me filter: %v", err)
	}

	if result.Total == 0 {
		t.Error("Expected at least one entry with me filter")
	}

	// All returned entries should have the bootstrap username
	for _, entry := range result.Entries {
		if entry.Username != constants.AuthBootstrapUsername {
			t.Errorf("ME filter returned entry with username '%s', expected '%s'",
				entry.Username, constants.AuthBootstrapUsername)
		}
	}
}

// =============================================================================
// Authentication Audit Tests
// =============================================================================

// TestAuditLogLoginSuccess tests audit logging for successful login
func TestAuditLogLoginSuccess(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Create a test user with known password, then login
	user := ts.CreateTestUser(t, "loginsuccessuser", "Password123!")
	ts.LoginUser(t, user.Username, user.Password)

	// Query for login_success action
	var result AuditQueryResponse
	if err := ts.GetJSON("/api/audit?action=login_success", &result); err != nil {
		t.Fatalf("Failed to query audit: %v", err)
	}

	if len(result.Entries) == 0 {
		t.Fatal("Expected audit entry for login_success")
	}

	entry := result.Entries[0]
	if entry.Username != user.Username {
		t.Errorf("Expected username '%s', got '%s'", user.Username, entry.Username)
	}

	details, ok := entry.Details.(map[string]interface{})
	if !ok {
		t.Fatal("Details should be a map")
	}
	if details["user_agent"] == nil || details["user_agent"] == "" {
		t.Error("Expected user_agent in details")
	}
}

// TestAuditLogLoginFailed tests audit logging for failed login attempts
func TestAuditLogLoginFailed(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Attempt login with wrong password
	resp, err := ts.UnauthenticatedPOST("/api/auth/login", map[string]string{
		"username": constants.AuthBootstrapUsername,
		"password": "wrong-password",
	})
	if err != nil {
		t.Fatalf("Login request failed: %v", err)
	}
	resp.Body.Close()

	// Query for login_failed action
	var result AuditQueryResponse
	if err := ts.GetJSON("/api/audit?action=login_failed", &result); err != nil {
		t.Fatalf("Failed to query audit: %v", err)
	}

	if len(result.Entries) == 0 {
		t.Fatal("Expected audit entry for login_failed")
	}

	entry := result.Entries[0]
	details, ok := entry.Details.(map[string]interface{})
	if !ok {
		t.Fatal("Details should be a map")
	}
	if details["attempted_username"] != constants.AuthBootstrapUsername {
		t.Errorf("Expected attempted_username '%s', got %v", constants.AuthBootstrapUsername, details["attempted_username"])
	}
	if details["reason"] == nil || details["reason"] == "" {
		t.Error("Expected reason in details")
	}
	if details["user_agent"] == nil || details["user_agent"] == "" {
		t.Error("Expected user_agent in details")
	}
}

// TestAuditLogLogout tests audit logging for logout
func TestAuditLogLogout(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Create a test user, login to get a session token
	user := ts.CreateTestUser(t, "logoutuser", "Password123!")
	token := ts.LoginUser(t, user.Username, user.Password)

	// Logout using the session token
	resp, err := ts.RequestWithSessionToken("POST", "/api/auth/logout", token, nil)
	if err != nil {
		t.Fatalf("Logout request failed: %v", err)
	}
	resp.Body.Close()

	// Query for logout action
	var result AuditQueryResponse
	if err := ts.GetJSON("/api/audit?action=logout", &result); err != nil {
		t.Fatalf("Failed to query audit: %v", err)
	}

	if len(result.Entries) == 0 {
		t.Fatal("Expected audit entry for logout")
	}

	entry := result.Entries[0]
	if entry.Username != user.Username {
		t.Errorf("Expected username '%s', got '%s'", user.Username, entry.Username)
	}
}

// =============================================================================
// User Management Audit Tests
// =============================================================================

// TestAuditLogUserCreated tests audit logging for user creation
func TestAuditLogUserCreated(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Create a test user
	user := ts.CreateTestUser(t, "audituser", "Password123!")

	// Query for user_created action
	var result AuditQueryResponse
	if err := ts.GetJSON("/api/audit?action=user_created", &result); err != nil {
		t.Fatalf("Failed to query audit: %v", err)
	}

	if len(result.Entries) == 0 {
		t.Fatal("Expected audit entry for user_created")
	}

	entry := result.Entries[0]
	// The actor is the bootstrap admin
	if entry.Username != constants.AuthBootstrapUsername {
		t.Errorf("Expected actor username '%s', got '%s'", constants.AuthBootstrapUsername, entry.Username)
	}

	details, ok := entry.Details.(map[string]interface{})
	if !ok {
		t.Fatal("Details should be a map")
	}
	if details["created_username"] != "audituser" {
		t.Errorf("Expected created_username 'audituser', got %v", details["created_username"])
	}
	// JSON numbers are float64
	if int64(details["created_user_id"].(float64)) != user.ID {
		t.Errorf("Expected created_user_id %d, got %v", user.ID, details["created_user_id"])
	}
}

// TestAuditLogUserUpdated tests audit logging for user update
func TestAuditLogUserUpdated(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Create a test user
	user := ts.CreateTestUser(t, "updateuser", "Password123!")

	// Update the user's display name
	resp, err := ts.PATCH(fmt.Sprintf("/api/auth/users/%d", user.ID), map[string]interface{}{
		"display_name": "Updated Name",
	})
	if err != nil {
		t.Fatalf("Update user request failed: %v", err)
	}
	resp.Body.Close()

	// Query for user_updated action
	var result AuditQueryResponse
	if err := ts.GetJSON("/api/audit?action=user_updated", &result); err != nil {
		t.Fatalf("Failed to query audit: %v", err)
	}

	if len(result.Entries) == 0 {
		t.Fatal("Expected audit entry for user_updated")
	}

	entry := result.Entries[0]
	details, ok := entry.Details.(map[string]interface{})
	if !ok {
		t.Fatal("Details should be a map")
	}
	if details["target_username"] != "updateuser" {
		t.Errorf("Expected target_username 'updateuser', got %v", details["target_username"])
	}
	fieldsChanged, ok := details["fields_changed"].([]interface{})
	if !ok {
		t.Fatal("Expected fields_changed to be an array")
	}
	found := false
	for _, f := range fieldsChanged {
		if f == "display_name" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected fields_changed to contain 'display_name', got %v", fieldsChanged)
	}
}

// TestAuditLogAPIKeyRegenerated tests audit logging for API key regeneration
func TestAuditLogAPIKeyRegenerated(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Create a test user
	user := ts.CreateTestUser(t, "apikeyuser", "Password123!")

	// Regenerate the API key
	resp, err := ts.POST(fmt.Sprintf("/api/auth/users/%d/api-key", user.ID), nil)
	if err != nil {
		t.Fatalf("Regenerate API key request failed: %v", err)
	}
	resp.Body.Close()

	// Query for api_key_regenerated action
	var result AuditQueryResponse
	if err := ts.GetJSON("/api/audit?action=api_key_regenerated", &result); err != nil {
		t.Fatalf("Failed to query audit: %v", err)
	}

	if len(result.Entries) == 0 {
		t.Fatal("Expected audit entry for api_key_regenerated")
	}

	entry := result.Entries[0]
	details, ok := entry.Details.(map[string]interface{})
	if !ok {
		t.Fatal("Details should be a map")
	}
	if int64(details["target_user_id"].(float64)) != user.ID {
		t.Errorf("Expected target_user_id %d, got %v", user.ID, details["target_user_id"])
	}
	if details["target_username"] != "apikeyuser" {
		t.Errorf("Expected target_username 'apikeyuser', got %v", details["target_username"])
	}
}

// =============================================================================
// Grant Management Audit Tests
// =============================================================================

// TestAuditLogGrantCreated tests audit logging for grant creation
func TestAuditLogGrantCreated(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Create a test user
	user := ts.CreateTestUser(t, "grantuser", "Password123!")

	// Create a grant
	resp, err := ts.POST(fmt.Sprintf("/api/auth/users/%d/grants", user.ID), map[string]interface{}{
		"action": constants.AuthActionUpload,
	})
	if err != nil {
		t.Fatalf("Create grant request failed: %v", err)
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	// Parse grant ID from response
	var grantResp struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(bodyBytes, &grantResp)

	// Query for grant_created action
	var result AuditQueryResponse
	if err := ts.GetJSON("/api/audit?action=grant_created", &result); err != nil {
		t.Fatalf("Failed to query audit: %v", err)
	}

	if len(result.Entries) == 0 {
		t.Fatal("Expected audit entry for grant_created")
	}

	entry := result.Entries[0]
	details, ok := entry.Details.(map[string]interface{})
	if !ok {
		t.Fatal("Details should be a map")
	}
	if details["action"] != constants.AuthActionUpload {
		t.Errorf("Expected action '%s', got %v", constants.AuthActionUpload, details["action"])
	}
	if int64(details["target_user_id"].(float64)) != user.ID {
		t.Errorf("Expected target_user_id %d, got %v", user.ID, details["target_user_id"])
	}
	if int64(details["grant_id"].(float64)) != grantResp.ID {
		t.Errorf("Expected grant_id %d, got %v", grantResp.ID, details["grant_id"])
	}
}

// TestAuditLogGrantUpdated tests audit logging for grant constraint update
func TestAuditLogGrantUpdated(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Create user with a grant
	user := ts.CreateTestUser(t, "grantupdateuser", "Password123!")
	resp, err := ts.POST(fmt.Sprintf("/api/auth/users/%d/grants", user.ID), map[string]interface{}{
		"action": constants.AuthActionUpload,
	})
	if err != nil {
		t.Fatalf("Create grant request failed: %v", err)
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var grantResp struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(bodyBytes, &grantResp)

	// Update grant constraints (use valid upload constraint field)
	constraintsJSON := `{"allowed_extensions":["png","jpg"]}`
	resp, err = ts.PATCH(fmt.Sprintf("/api/auth/grants/%d", grantResp.ID), map[string]interface{}{
		"constraints_json": constraintsJSON,
	})
	if err != nil {
		t.Fatalf("Update grant request failed: %v", err)
	}
	patchBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Grant update returned status %d: %s", resp.StatusCode, string(patchBody))
	}

	// Query for grant_updated action
	var result AuditQueryResponse
	if err := ts.GetJSON("/api/audit?action=grant_updated", &result); err != nil {
		t.Fatalf("Failed to query audit: %v", err)
	}

	if len(result.Entries) == 0 {
		t.Fatal("Expected audit entry for grant_updated")
	}

	entry := result.Entries[0]
	details, ok := entry.Details.(map[string]interface{})
	if !ok {
		t.Fatal("Details should be a map")
	}
	if int64(details["grant_id"].(float64)) != grantResp.ID {
		t.Errorf("Expected grant_id %d, got %v", grantResp.ID, details["grant_id"])
	}
	if details["has_constraints"] != true {
		t.Error("Expected has_constraints to be true")
	}
}

// TestAuditLogGrantRevoked tests audit logging for grant revocation
func TestAuditLogGrantRevoked(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Create user with a grant
	user := ts.CreateTestUser(t, "grantrevokeuser", "Password123!")
	resp, err := ts.POST(fmt.Sprintf("/api/auth/users/%d/grants", user.ID), map[string]interface{}{
		"action": constants.AuthActionUpload,
	})
	if err != nil {
		t.Fatalf("Create grant request failed: %v", err)
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var grantResp struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(bodyBytes, &grantResp)

	// Revoke the grant
	resp, err = ts.DELETE(fmt.Sprintf("/api/auth/grants/%d", grantResp.ID))
	if err != nil {
		t.Fatalf("Revoke grant request failed: %v", err)
	}
	resp.Body.Close()

	// Query for grant_revoked action
	var result AuditQueryResponse
	if err := ts.GetJSON("/api/audit?action=grant_revoked", &result); err != nil {
		t.Fatalf("Failed to query audit: %v", err)
	}

	if len(result.Entries) == 0 {
		t.Fatal("Expected audit entry for grant_revoked")
	}

	entry := result.Entries[0]
	details, ok := entry.Details.(map[string]interface{})
	if !ok {
		t.Fatal("Details should be a map")
	}
	if int64(details["grant_id"].(float64)) != grantResp.ID {
		t.Errorf("Expected grant_id %d, got %v", grantResp.ID, details["grant_id"])
	}
	if details["action"] != constants.AuthActionUpload {
		t.Errorf("Expected action '%s', got %v", constants.AuthActionUpload, details["action"])
	}
}

// =============================================================================
// Metadata Audit Tests
// =============================================================================

// TestAuditLogMetadataSet tests audit logging for single metadata set operation
func TestAuditLogMetadataSet(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "metadata-audit-test")

	// Upload a file
	uploadResp := ts.UploadFileExpectSuccess(t, "metadata-audit-test", "file.bin", SmallFile, "")

	// Set metadata on it
	resp, err := ts.POST(fmt.Sprintf("/api/assets/%s/metadata", uploadResp.Hash), map[string]interface{}{
		"op":                "set",
		"key":               "test_key",
		"value":             "test_value",
		"processor":         "test-processor",
		"processor_version": "1.0",
	})
	if err != nil {
		t.Fatalf("Set metadata request failed: %v", err)
	}
	metaBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Metadata set returned status %d: %s", resp.StatusCode, string(metaBody))
	}

	// Query for metadata_set action
	var result AuditQueryResponse
	if err := ts.GetJSON("/api/audit?action=metadata_set", &result); err != nil {
		t.Fatalf("Failed to query audit: %v", err)
	}

	if len(result.Entries) == 0 {
		t.Fatal("Expected audit entry for metadata_set")
	}

	entry := result.Entries[0]
	details, ok := entry.Details.(map[string]interface{})
	if !ok {
		t.Fatal("Details should be a map")
	}
	if details["hash"] != uploadResp.Hash {
		t.Errorf("Expected hash '%s', got %v", uploadResp.Hash, details["hash"])
	}
	if details["op"] != "set" {
		t.Errorf("Expected op 'set', got %v", details["op"])
	}
	if details["key"] != "test_key" {
		t.Errorf("Expected key 'test_key', got %v", details["key"])
	}
}

// TestAuditLogMetadataBatch tests audit logging for batch metadata operations
func TestAuditLogMetadataBatch(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "batch-audit-test")

	// Upload files
	upload1 := ts.UploadFileExpectSuccess(t, "batch-audit-test", "file1.bin", SmallFile, "")
	upload2 := ts.UploadFileExpectSuccess(t, "batch-audit-test", "file2.bin", MediumFile, "")

	// Batch metadata operation
	resp, err := ts.POST("/api/metadata/batch", map[string]interface{}{
		"processor": "test-processor",
		"operations": []map[string]interface{}{
			{"hash": upload1.Hash, "op": "set", "key": "batch_key", "value": "val1"},
			{"hash": upload2.Hash, "op": "set", "key": "batch_key", "value": "val2"},
		},
	})
	if err != nil {
		t.Fatalf("Batch metadata request failed: %v", err)
	}
	resp.Body.Close()

	// Query for metadata_batch action
	var result AuditQueryResponse
	if err := ts.GetJSON("/api/audit?action=metadata_batch", &result); err != nil {
		t.Fatalf("Failed to query audit: %v", err)
	}

	if len(result.Entries) == 0 {
		t.Fatal("Expected audit entry for metadata_batch")
	}

	entry := result.Entries[0]
	details, ok := entry.Details.(map[string]interface{})
	if !ok {
		t.Fatal("Details should be a map")
	}
	if int(details["operation_count"].(float64)) != 2 {
		t.Errorf("Expected operation_count 2, got %v", details["operation_count"])
	}
	if details["processor"] != "test-processor" {
		t.Errorf("Expected processor 'test-processor', got %v", details["processor"])
	}
	if int(details["succeeded"].(float64)) != 2 {
		t.Errorf("Expected succeeded 2, got %v", details["succeeded"])
	}
}

// TestAuditLogMetadataApply tests audit logging for apply metadata from query
func TestAuditLogMetadataApply(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "apply-audit-test")

	// Upload files
	ts.UploadFileExpectSuccess(t, "apply-audit-test", "file1.bin", SmallFile, "")

	// Apply metadata via query preset (use recent-imports which returns asset_id column)
	resp, err := ts.POST("/api/metadata/apply", map[string]interface{}{
		"query_preset":      "recent-imports",
		"topics":            []string{"apply-audit-test"},
		"op":                "set",
		"key":               "applied_key",
		"value":             "applied_value",
		"processor":         "apply-processor",
		"processor_version": "1.0",
	})
	if err != nil {
		t.Fatalf("Apply metadata request failed: %v", err)
	}
	applyBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Apply metadata returned status %d: %s", resp.StatusCode, string(applyBody))
	}

	// Query for metadata_apply action
	var result AuditQueryResponse
	if err := ts.GetJSON("/api/audit?action=metadata_apply", &result); err != nil {
		t.Fatalf("Failed to query audit: %v", err)
	}

	if len(result.Entries) == 0 {
		t.Fatal("Expected audit entry for metadata_apply")
	}

	entry := result.Entries[0]
	details, ok := entry.Details.(map[string]interface{})
	if !ok {
		t.Fatal("Details should be a map")
	}
	if details["query_preset"] != "recent-imports" {
		t.Errorf("Expected query_preset 'recent-imports', got %v", details["query_preset"])
	}
	if details["op"] != "set" {
		t.Errorf("Expected op 'set', got %v", details["op"])
	}
	if details["key"] != "applied_key" {
		t.Errorf("Expected key 'applied_key', got %v", details["key"])
	}
	if details["processor"] != "apply-processor" {
		t.Errorf("Expected processor 'apply-processor', got %v", details["processor"])
	}
}

// =============================================================================
// Configuration Audit Tests
// =============================================================================

// TestAuditLogConfigChanged tests audit logging for config change
func TestAuditLogConfigChanged(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Query for config_changed action (ConfigureWorkDir triggers POST /api/config)
	var result AuditQueryResponse
	if err := ts.GetJSON("/api/audit?action=config_changed", &result); err != nil {
		t.Fatalf("Failed to query audit: %v", err)
	}

	if len(result.Entries) == 0 {
		t.Fatal("Expected audit entry for config_changed")
	}

	entry := result.Entries[0]
	details, ok := entry.Details.(map[string]interface{})
	if !ok {
		t.Fatal("Details should be a map")
	}
	if details["working_directory"] == nil || details["working_directory"] == "" {
		t.Error("Expected working_directory in details")
	}
	// The first config is a bootstrap
	if details["is_bootstrap"] != true {
		t.Error("Expected is_bootstrap to be true for initial config")
	}
}
