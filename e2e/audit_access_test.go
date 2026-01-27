package e2e

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"silobang/internal/constants"
)

// =============================================================================
// Admin Can Use All Audit Filters
// =============================================================================

// TestAuditAccess_AdminCanUseAllFilters verifies that the bootstrap admin
// can query audit logs with all filter values (me, others, empty).
func TestAuditAccess_AdminCanUseAllFilters(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Generate some audit data
	ts.CreateTopic(t, "admin-audit-test")

	for _, filter := range []string{"", "me", "others"} {
		path := "/api/audit?limit=5"
		if filter != "" {
			path += "&filter=" + filter
		}

		resp, err := ts.GET(path)
		if err != nil {
			t.Fatalf("Audit query with filter=%q failed: %v", filter, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("Admin audit query with filter=%q returned %d: %s", filter, resp.StatusCode, body)
		}
	}
}

// =============================================================================
// Non-Admin With CanViewAll=true Can See Others' Actions
// =============================================================================

// TestAuditAccess_NonAdminWithCanViewAll_SeesOthers verifies that a non-admin
// user with a view_audit grant where can_view_all=true can use filter=others
// and see the admin's audit entries.
func TestAuditAccess_NonAdminWithCanViewAll_SeesOthers(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Admin creates a topic (generates audit entries under admin username)
	ts.CreateTopic(t, "canviewall-test")

	// Create a non-admin user with view_audit grant (can_view_all: true)
	viewer := ts.CreateTestUserWithGrants(t, "viewer-full", "ViewerPass123!", []map[string]interface{}{
		{
			"action":           constants.AuthActionViewAudit,
			"constraints_json": `{"can_view_all": true, "can_stream": true}`,
		},
	})

	// Query with filter=others using the non-admin's API key
	resp, err := ts.RequestWithAPIKey("GET", "/api/audit?filter=others&limit=50", viewer.APIKey, nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, body)
	}

	var result AuditQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Should see admin's entries (topic creation, etc.)
	if len(result.Entries) == 0 {
		t.Error("Expected to see admin's audit entries with can_view_all=true, got none")
	}

	// Verify none of the entries belong to the viewer
	for _, entry := range result.Entries {
		if entry.Username == viewer.Username {
			t.Errorf("filter=others should not include own entries, found entry from %s", entry.Username)
		}
	}
}

// =============================================================================
// Non-Admin Without CanViewAll Is Forced To "me" Filter
// =============================================================================

// TestAuditAccess_NonAdminWithoutCanViewAll_ForcedToMe verifies that a non-admin
// user with can_view_all=false is restricted to their own entries only,
// even if they request filter=others or filter="" (all).
func TestAuditAccess_NonAdminWithoutCanViewAll_ForcedToMe(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Admin creates a topic (generates audit entries)
	ts.CreateTopic(t, "restricted-audit-test")

	// Create non-admin with view_audit grant (can_view_all: false, can_stream: true)
	restricted := ts.CreateTestUserWithGrants(t, "viewer-restricted", "RestrictedPass123!", []map[string]interface{}{
		{
			"action":           constants.AuthActionViewAudit,
			"constraints_json": `{"can_view_all": false, "can_stream": true}`,
		},
	})

	// Test 1: filter=others should be overridden to "me"
	resp, err := ts.RequestWithAPIKey("GET", "/api/audit?filter=others&limit=50", restricted.APIKey, nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, body)
	}

	var othersResult AuditQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&othersResult); err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}

	// With can_view_all=false and filter=others overridden to "me",
	// should only see own entries (or none if user hasn't done anything audit-worthy)
	for _, entry := range othersResult.Entries {
		if entry.Username != restricted.Username {
			t.Errorf("With can_view_all=false, filter=others should be forced to me, but saw entry from %s", entry.Username)
		}
	}

	// Test 2: filter="" (all) should also be overridden to "me"
	resp2, err := ts.RequestWithAPIKey("GET", "/api/audit?limit=50", restricted.APIKey, nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp2.Body)
		t.Fatalf("Expected 200, got %d: %s", resp2.StatusCode, body)
	}

	var allResult AuditQueryResponse
	if err := json.NewDecoder(resp2.Body).Decode(&allResult); err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}

	// Should only see own entries — admin's entries must not appear
	for _, entry := range allResult.Entries {
		if entry.Username != restricted.Username {
			t.Errorf("With can_view_all=false, empty filter should be forced to me, but saw entry from %s", entry.Username)
		}
	}
}

// =============================================================================
// Non-Admin Without Grant Gets 403
// =============================================================================

// TestAuditAccess_NonAdminWithoutGrant_Gets403 verifies that a user
// without any view_audit grant receives 403 Forbidden.
func TestAuditAccess_NonAdminWithoutGrant_Gets403(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Create user with no audit grant (only upload grant)
	noAudit := ts.CreateTestUserWithGrants(t, "no-audit-user", "NoAuditPass123!", []map[string]interface{}{
		{
			"action":           constants.AuthActionUpload,
			"constraints_json": `{}`,
		},
	})

	resp, err := ts.RequestWithAPIKey("GET", "/api/audit?limit=5", noAudit.APIKey, nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected 403, got %d: %s", resp.StatusCode, body)
	}
}

// =============================================================================
// No Constraints = Full Access (Unrestricted Grant)
// =============================================================================

// TestAuditAccess_NoConstraints_FullAccess verifies that a view_audit grant
// with no constraints JSON (null/empty) allows unrestricted access to all filters.
func TestAuditAccess_NoConstraints_FullAccess(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Admin creates audit data
	ts.CreateTopic(t, "no-constraints-test")

	// Create user with view_audit grant but NO constraints
	// (the evaluator short-circuits and returns allowed for null constraints)
	unconstrained := ts.CreateTestUserWithGrants(t, "viewer-unconstrained", "UnconstrainedPass123!", []map[string]interface{}{
		{
			"action": constants.AuthActionViewAudit,
			// No constraints_json — grant has no restrictions
		},
	})

	// Should be able to use filter=others and see admin's entries
	resp, err := ts.RequestWithAPIKey("GET", "/api/audit?filter=others&limit=50", unconstrained.APIKey, nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, body)
	}

	var result AuditQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}

	// Should see admin's entries since no constraints means full access
	if len(result.Entries) == 0 {
		t.Error("Expected to see admin's entries with unconstrained grant, got none")
	}
}

// =============================================================================
// Non-Admin filter=me Works For Both can_view_all=true and false
// =============================================================================

// TestAuditAccess_FilterMeAlwaysAllowed verifies that filter=me is always
// allowed regardless of the can_view_all constraint value.
func TestAuditAccess_FilterMeAlwaysAllowed(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Create restricted user
	restricted := ts.CreateTestUserWithGrants(t, "viewer-me-only", "MeOnlyPass123!", []map[string]interface{}{
		{
			"action":           constants.AuthActionViewAudit,
			"constraints_json": `{"can_view_all": false, "can_stream": true}`,
		},
	})

	// filter=me should work normally
	resp, err := ts.RequestWithAPIKey("GET", "/api/audit?filter=me&limit=50", restricted.APIKey, nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected 200 for filter=me, got %d: %s", resp.StatusCode, body)
	}

	var result AuditQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}

	// All returned entries should belong to the restricted user
	for _, entry := range result.Entries {
		if entry.Username != restricted.Username {
			t.Errorf("filter=me returned entry from %s, expected only %s", entry.Username, restricted.Username)
		}
	}
}

// =============================================================================
// Default can_view_all=false When Constraints Only Have can_stream
// =============================================================================

// TestAuditAccess_DefaultCanViewAllFalse verifies that when constraints
// only specify can_stream (omitting can_view_all), the Go zero-value (false)
// is enforced — the user is restricted to their own entries.
func TestAuditAccess_DefaultCanViewAllFalse(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Admin creates data
	ts.CreateTopic(t, "default-canviewall-test")

	// Create user with grant that only sets can_stream (can_view_all defaults to false)
	streamOnly := ts.CreateTestUserWithGrants(t, "viewer-stream-only", "StreamOnlyPass123!", []map[string]interface{}{
		{
			"action":           constants.AuthActionViewAudit,
			"constraints_json": `{"can_stream": true}`,
		},
	})

	// Empty filter should be overridden to "me" since can_view_all defaults false
	resp, err := ts.RequestWithAPIKey("GET", "/api/audit?limit=50", streamOnly.APIKey, nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, body)
	}

	var result AuditQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}

	// Should only see own entries — admin's entries must not leak
	for _, entry := range result.Entries {
		if entry.Username != streamOnly.Username {
			t.Errorf("With default can_view_all=false, saw entry from %s (expected only %s)",
				entry.Username, streamOnly.Username)
		}
	}
}
