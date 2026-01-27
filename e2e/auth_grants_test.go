package e2e

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"meshbank/internal/constants"
)

// =============================================================================
// Grant CRUD
// =============================================================================

// TestCreateGrant verifies admin can grant permissions to a user
func TestCreateGrant(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	user := ts.CreateTestUser(t, "grantuser", "secure-password-12345")

	// Grant upload permission
	resp, err := ts.POST(fmt.Sprintf("/api/auth/users/%d/grants", user.ID), map[string]interface{}{
		"action": constants.AuthActionUpload,
	})
	if err != nil {
		t.Fatalf("create grant request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var grant map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&grant)

	if grant["action"] != constants.AuthActionUpload {
		t.Errorf("expected action=upload, got %s", grant["action"])
	}
	if grant["is_active"] != true {
		t.Error("expected is_active=true")
	}
}

// TestListUserGrants verifies listing grants for a user
func TestListUserGrants(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	user := ts.CreateTestUserWithGrants(t, "listgrants", "secure-password-12345", []map[string]interface{}{
		{"action": constants.AuthActionUpload},
		{"action": constants.AuthActionDownload},
	})

	resp, err := ts.GET(fmt.Sprintf("/api/auth/users/%d/grants", user.ID))
	if err != nil {
		t.Fatalf("GET grants failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)

	grants := body["grants"].([]interface{})
	if len(grants) < 2 {
		t.Errorf("expected at least 2 grants, got %d", len(grants))
	}
}

// TestRevokeGrant verifies revoking a grant
func TestRevokeGrant(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	user := ts.CreateTestUserWithGrants(t, "revokegrant", "secure-password-12345", []map[string]interface{}{
		{"action": constants.AuthActionUpload},
		{"action": constants.AuthActionDownload},
	})

	// List grants to get grant ID
	resp, err := ts.GET(fmt.Sprintf("/api/auth/users/%d/grants", user.ID))
	if err != nil {
		t.Fatalf("GET grants failed: %v", err)
	}

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)
	resp.Body.Close()

	grants := body["grants"].([]interface{})
	firstGrant := grants[0].(map[string]interface{})
	grantID := int64(firstGrant["id"].(float64))

	// Revoke the grant
	delResp, err := ts.DELETE(fmt.Sprintf("/api/auth/grants/%d", grantID))
	if err != nil {
		t.Fatalf("DELETE grant failed: %v", err)
	}
	defer delResp.Body.Close()

	if delResp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(delResp.Body)
		t.Fatalf("expected 200 on revoke, got %d: %s", delResp.StatusCode, string(bodyBytes))
	}
}

// TestUpdateGrantConstraints verifies updating constraints on a grant
func TestUpdateGrantConstraints(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	user := ts.CreateTestUserWithGrants(t, "updategrant", "secure-password-12345", []map[string]interface{}{
		{"action": constants.AuthActionUpload},
	})

	// Get grant ID
	resp, err := ts.GET(fmt.Sprintf("/api/auth/users/%d/grants", user.ID))
	if err != nil {
		t.Fatalf("GET grants failed: %v", err)
	}

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)
	resp.Body.Close()

	grants := body["grants"].([]interface{})
	firstGrant := grants[0].(map[string]interface{})
	grantID := int64(firstGrant["id"].(float64))

	// Update constraints
	constraints := `{"allowed_extensions":["png","jpg"],"max_file_size_bytes":1048576}`
	patchResp, err := ts.PATCH(fmt.Sprintf("/api/auth/grants/%d", grantID), map[string]interface{}{
		"constraints_json": constraints,
	})
	if err != nil {
		t.Fatalf("PATCH grant failed: %v", err)
	}
	defer patchResp.Body.Close()

	if patchResp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(patchResp.Body)
		t.Fatalf("expected 200, got %d: %s", patchResp.StatusCode, string(bodyBytes))
	}
}

// =============================================================================
// Constraint Enforcement
// =============================================================================

// TestUploadConstraint_AllowedExtensions verifies extension restrictions
func TestUploadConstraint_AllowedExtensions(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	constraints := `{"allowed_extensions":["png","jpg"]}`
	user := ts.CreateTestUserWithGrants(t, "extuser", "secure-password-12345", []map[string]interface{}{
		{"action": constants.AuthActionUpload, "constraints_json": constraints},
		{"action": constants.AuthActionManageTopics},
	})

	// Create topic as admin, then try uploads as restricted user
	ts.CreateTopic(t, "ext-test-topic")

	// Upload .png — should succeed (need to use restricted user's API key)
	oldKey := ts.APIKey
	ts.APIKey = user.APIKey
	defer func() { ts.APIKey = oldKey }()

	resp, err := ts.UploadFile("ext-test-topic", "valid.png", []byte("png-content"), "")
	if err != nil {
		t.Fatalf("upload .png failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Errorf("expected success for .png upload, got %d", resp.StatusCode)
	}

	// Upload .gif — should be forbidden
	resp, err = ts.UploadFile("ext-test-topic", "invalid.gif", []byte("gif-content"), "")
	if err != nil {
		t.Fatalf("upload .gif failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 403 for .gif upload, got %d: %s", resp.StatusCode, string(bodyBytes))
	}
}

// TestUploadConstraint_MaxFileSize verifies file size limit
func TestUploadConstraint_MaxFileSize(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	constraints := `{"max_file_size_bytes":100}`
	user := ts.CreateTestUserWithGrants(t, "sizeuser", "secure-password-12345", []map[string]interface{}{
		{"action": constants.AuthActionUpload, "constraints_json": constraints},
		{"action": constants.AuthActionManageTopics},
	})

	ts.CreateTopic(t, "size-test-topic")

	oldKey := ts.APIKey
	ts.APIKey = user.APIKey
	defer func() { ts.APIKey = oldKey }()

	// Small file — should succeed
	resp, err := ts.UploadFile("size-test-topic", "small.bin", make([]byte, 50), "")
	if err != nil {
		t.Fatalf("small upload failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Errorf("expected success for small file, got %d", resp.StatusCode)
	}

	// Large file — should be forbidden
	resp, err = ts.UploadFile("size-test-topic", "large.bin", make([]byte, 200), "")
	if err != nil {
		t.Fatalf("large upload failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 for oversized file, got %d", resp.StatusCode)
	}
}

// TestUploadConstraint_AllowedTopics verifies topic restrictions
func TestUploadConstraint_AllowedTopics(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	constraints := `{"allowed_topics":["allowed-topic"]}`
	user := ts.CreateTestUserWithGrants(t, "topicuser", "secure-password-12345", []map[string]interface{}{
		{"action": constants.AuthActionUpload, "constraints_json": constraints},
	})

	ts.CreateTopic(t, "allowed-topic")
	ts.CreateTopic(t, "forbidden-topic")

	oldKey := ts.APIKey
	ts.APIKey = user.APIKey
	defer func() { ts.APIKey = oldKey }()

	// Upload to allowed topic — should succeed
	resp, err := ts.UploadFile("allowed-topic", "file.bin", []byte("content"), "")
	if err != nil {
		t.Fatalf("upload to allowed topic failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Errorf("expected success for allowed topic, got %d", resp.StatusCode)
	}

	// Upload to forbidden topic — should fail
	resp, err = ts.UploadFile("forbidden-topic", "file.bin", []byte("content"), "")
	if err != nil {
		t.Fatalf("upload to forbidden topic failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 for forbidden topic, got %d", resp.StatusCode)
	}
}

// TestNoGrantUser_Forbidden verifies user with no grants is denied on protected actions
func TestNoGrantUser_Forbidden(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Create user with no grants
	user := ts.CreateTestUser(t, "nogrants", "secure-password-12345")
	ts.CreateTopic(t, "nogrant-topic")

	// Try to upload — should be forbidden (no upload grant)
	oldKey := ts.APIKey
	ts.APIKey = user.APIKey
	defer func() { ts.APIKey = oldKey }()

	resp, err := ts.UploadFile("nogrant-topic", "file.bin", []byte("content"), "")
	if err != nil {
		t.Fatalf("upload request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 for user with no grants, got %d", resp.StatusCode)
	}
}

// TestQueryConstraint_AllowedPresets verifies query preset restrictions
func TestQueryConstraint_AllowedPresets(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	constraints := `{"allowed_presets":["count"]}`
	user := ts.CreateTestUserWithGrants(t, "presetuser", "secure-password-12345", []map[string]interface{}{
		{"action": constants.AuthActionQuery, "constraints_json": constraints},
		{"action": constants.AuthActionUpload},
		{"action": constants.AuthActionManageTopics},
	})

	ts.CreateTopic(t, "preset-topic")

	// Upload a file as admin first
	ts.UploadFileExpectSuccess(t, "preset-topic", "test.bin", []byte("test-content"), "")

	oldKey := ts.APIKey
	ts.APIKey = user.APIKey
	defer func() { ts.APIKey = oldKey }()

	// Query with allowed preset ("count") — should succeed
	resp, err := ts.POST("/api/query/count", map[string]interface{}{
		"topics": []string{"preset-topic"},
	})
	if err != nil {
		t.Fatalf("allowed query failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for allowed preset, got %d", resp.StatusCode)
	}

	// Query with non-allowed preset ("large-files") — should fail
	resp, err = ts.POST("/api/query/large-files", map[string]interface{}{
		"topics": []string{"preset-topic"},
	})
	if err != nil {
		t.Fatalf("forbidden query failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 for non-allowed preset, got %d", resp.StatusCode)
	}
}
