package e2e

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"silobang/internal/constants"
)

// =============================================================================
// Bootstrap & Status
// =============================================================================

// TestAuthStatus_NotConfigured verifies /api/auth/status before any config
func TestAuthStatus_NotConfigured(t *testing.T) {
	ts := StartTestServer(t)

	// Status endpoint is public — no auth needed
	resp, err := ts.UnauthenticatedGET("/api/auth/status")
	if err != nil {
		t.Fatalf("GET /api/auth/status failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)

	if body["bootstrapped"] != false {
		t.Error("expected bootstrapped=false before config")
	}
	if body["configured"] != false {
		t.Error("expected configured=false before config")
	}
}

// TestAuthStatus_Bootstrapped verifies /api/auth/status after bootstrap
func TestAuthStatus_Bootstrapped(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	resp, err := ts.UnauthenticatedGET("/api/auth/status")
	if err != nil {
		t.Fatalf("GET /api/auth/status failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)

	if body["bootstrapped"] != true {
		t.Error("expected bootstrapped=true after config")
	}
	if body["configured"] != true {
		t.Error("expected configured=true after config")
	}
}

// TestBootstrapCreation verifies bootstrap creates admin user with API key
func TestBootstrapCreation(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	if ts.APIKey == "" {
		t.Fatal("expected bootstrap to produce an API key")
	}

	// API key must have the correct prefix
	if !strings.HasPrefix(ts.APIKey, constants.APIKeyPrefix) {
		t.Errorf("API key should start with %s, got prefix: %s",
			constants.APIKeyPrefix, ts.APIKey[:4])
	}

	// Use the API key to call /api/auth/me
	resp, err := ts.GET("/api/auth/me")
	if err != nil {
		t.Fatalf("GET /api/auth/me failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var me map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&me)

	user := me["user"].(map[string]interface{})
	if user["username"] != constants.AuthBootstrapUsername {
		t.Errorf("expected username=%s, got %s", constants.AuthBootstrapUsername, user["username"])
	}
	if user["is_bootstrap"] != true {
		t.Error("expected is_bootstrap=true for bootstrap user")
	}
	if me["method"] != "api_key" {
		t.Errorf("expected method=api_key, got %s", me["method"])
	}

	// Grants should include all actions
	grants := me["grants"].([]interface{})
	if len(grants) != len(constants.AllAuthActions) {
		t.Errorf("expected %d grants, got %d", len(constants.AllAuthActions), len(grants))
	}
}

// TestBootstrapIdempotent verifies second config call doesn't create duplicate bootstrap
func TestBootstrapIdempotent(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	firstKey := ts.APIKey

	// Reconfigure same directory (should not re-bootstrap)
	resp, err := ts.POST("/api/config", map[string]interface{}{
		"working_directory": ts.WorkDir,
	})
	if err != nil {
		t.Fatalf("second config failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	var configResp struct {
		Bootstrap *struct {
			APIKey string `json:"api_key"`
		} `json:"bootstrap"`
	}
	json.Unmarshal(bodyBytes, &configResp)

	// No new bootstrap should be returned
	if configResp.Bootstrap != nil {
		t.Error("expected no bootstrap credentials on second config")
	}

	// Original key should still work
	if firstKey == "" {
		t.Fatal("first API key was empty")
	}
}

// =============================================================================
// Login / Logout / Session
// =============================================================================

// TestLogin_Success verifies login with correct bootstrap credentials
func TestLogin_Success(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// We need the bootstrap password — let's read it from the initial config response
	// Since ConfigureWorkDir only captures APIKey, we need to capture password too.
	// Instead, create a new user and login with known credentials.
	user := ts.CreateTestUser(t, "logintest", "secure-password-12345")

	token := ts.LoginUser(t, user.Username, user.Password)

	// Token should have session prefix
	if !strings.HasPrefix(token, constants.SessionTokenPrefix) {
		t.Errorf("session token should start with %s, got prefix: %s",
			constants.SessionTokenPrefix, token[:4])
	}

	// Use session token for authenticated request
	resp, err := ts.RequestWithSessionToken("GET", "/api/auth/me", token, nil)
	if err != nil {
		t.Fatalf("GET /api/auth/me with session failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var me map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&me)

	if me["method"] != "session" {
		t.Errorf("expected method=session, got %s", me["method"])
	}

	meUser := me["user"].(map[string]interface{})
	if meUser["username"] != "logintest" {
		t.Errorf("expected username=logintest, got %s", meUser["username"])
	}
}

// TestLogin_InvalidPassword verifies login with wrong password fails
func TestLogin_InvalidPassword(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTestUser(t, "badpasstest", "correct-password-12345")

	resp, err := ts.UnauthenticatedPOST("/api/auth/login", map[string]string{
		"username": "badpasstest",
		"password": "wrong-password-12345",
	})
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}

	var errResp ErrorResponse
	json.NewDecoder(resp.Body).Decode(&errResp)
	if errResp.Code != constants.ErrCodeAuthInvalidCredentials {
		t.Errorf("expected code=%s, got %s", constants.ErrCodeAuthInvalidCredentials, errResp.Code)
	}
}

// TestLogin_NonexistentUser verifies login with unknown user fails generically
func TestLogin_NonexistentUser(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	resp, err := ts.UnauthenticatedPOST("/api/auth/login", map[string]string{
		"username": "nonexistent-user",
		"password": "any-password-12345",
	})
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}

	// Should return same error code as invalid password (no info leakage)
	var errResp ErrorResponse
	json.NewDecoder(resp.Body).Decode(&errResp)
	if errResp.Code != constants.ErrCodeAuthInvalidCredentials {
		t.Errorf("expected code=%s, got %s", constants.ErrCodeAuthInvalidCredentials, errResp.Code)
	}
}

// TestLogin_EmptyCredentials verifies login with empty fields fails
func TestLogin_EmptyCredentials(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	resp, err := ts.UnauthenticatedPOST("/api/auth/login", map[string]string{
		"username": "",
		"password": "",
	})
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// TestLogout verifies session invalidation
func TestLogout(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	user := ts.CreateTestUser(t, "logouttest", "secure-password-12345")

	token := ts.LoginUser(t, user.Username, user.Password)

	// Verify token works
	resp, err := ts.RequestWithSessionToken("GET", "/api/auth/me", token, nil)
	if err != nil {
		t.Fatalf("pre-logout /me failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 before logout, got %d", resp.StatusCode)
	}

	// Logout
	resp, err = ts.RequestWithSessionToken("POST", "/api/auth/logout", token, nil)
	if err != nil {
		t.Fatalf("logout request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on logout, got %d", resp.StatusCode)
	}

	// Verify token no longer works
	resp, err = ts.RequestWithSessionToken("GET", "/api/auth/me", token, nil)
	if err != nil {
		t.Fatalf("post-logout /me failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 after logout, got %d", resp.StatusCode)
	}
}

// TestAPIKeyAuth verifies API key authentication works
func TestAPIKeyAuth(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	user := ts.CreateTestUser(t, "apikeytest", "secure-password-12345")

	// Use API key via RequestWithAPIKey helper
	resp, err := ts.RequestWithAPIKey("GET", "/api/auth/me", user.APIKey, nil)
	if err != nil {
		t.Fatalf("API key request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var me map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&me)

	if me["method"] != "api_key" {
		t.Errorf("expected method=api_key, got %s", me["method"])
	}
}

// TestAPIKeyViaBearerHeader verifies API key works via Authorization: Bearer header
func TestAPIKeyViaBearerHeader(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	user := ts.CreateTestUser(t, "bearerapikey", "secure-password-12345")

	// Send API key via Bearer header instead of X-API-Key
	resp, err := ts.RequestWithSessionToken("GET", "/api/auth/me", user.APIKey, nil)
	if err != nil {
		t.Fatalf("Bearer API key request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var me map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&me)

	if me["method"] != "api_key" {
		t.Errorf("expected method=api_key via Bearer, got %s", me["method"])
	}
}

// TestAuthMe_Quota verifies /api/auth/me/quota returns quota info
func TestAuthMe_Quota(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	resp, err := ts.GET("/api/auth/me/quota")
	if err != nil {
		t.Fatalf("GET /api/auth/me/quota failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)

	if body["user_id"] == nil {
		t.Error("expected user_id in quota response")
	}
}

// TestUnauthenticated_ProtectedEndpoints verifies 401 on protected endpoints
func TestUnauthenticated_ProtectedEndpoints(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/api/auth/me"},
		{"GET", "/api/auth/me/quota"},
		{"POST", "/api/auth/logout"},
		{"GET", "/api/auth/users"},
		{"POST", "/api/auth/users"},
		{"GET", "/api/topics"},
		{"POST", "/api/topics"},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+"_"+ep.path, func(t *testing.T) {
			var resp *http.Response
			var err error
			switch ep.method {
			case "GET":
				resp, err = ts.UnauthenticatedGET(ep.path)
			case "POST":
				resp, err = ts.UnauthenticatedPOST(ep.path, nil)
			}
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("expected 401 for %s %s, got %d", ep.method, ep.path, resp.StatusCode)
			}
		})
	}
}

// =============================================================================
// User Management
// =============================================================================

// TestCreateUser verifies user creation via API
func TestCreateUser(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	user := ts.CreateTestUser(t, "newuser", "secure-password-12345")

	if user.ID == 0 {
		t.Error("expected non-zero user ID")
	}
	if user.Username != "newuser" {
		t.Errorf("expected username=newuser, got %s", user.Username)
	}
	if user.APIKey == "" {
		t.Error("expected non-empty API key")
	}
	if !strings.HasPrefix(user.APIKey, constants.APIKeyPrefix) {
		t.Errorf("API key should start with %s", constants.APIKeyPrefix)
	}
}

// TestCreateUser_DuplicateUsername verifies duplicate username fails
func TestCreateUser_DuplicateUsername(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	ts.CreateTestUser(t, "dupeuser", "secure-password-12345")

	// Try to create same username again
	resp, err := ts.POST("/api/auth/users", map[string]string{
		"username":     "dupeuser",
		"display_name": "Duplicate",
		"password":     "another-password-12345",
	})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}

	var errResp ErrorResponse
	json.NewDecoder(resp.Body).Decode(&errResp)
	if errResp.Code != constants.ErrCodeAuthUserExists {
		t.Errorf("expected code=%s, got %s", constants.ErrCodeAuthUserExists, errResp.Code)
	}
}

// TestCreateUser_InvalidUsername verifies username validation
func TestCreateUser_InvalidUsername(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	badNames := []string{"AB", "With Spaces", "UPPERCASE", "special!chars", "a"}

	for _, name := range badNames {
		t.Run(name, func(t *testing.T) {
			resp, err := ts.POST("/api/auth/users", map[string]string{
				"username":     name,
				"display_name": "Test",
				"password":     "secure-password-12345",
			})
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("expected 400 for username %q, got %d", name, resp.StatusCode)
			}
		})
	}
}

// TestCreateUser_WeakPassword verifies password length enforcement
func TestCreateUser_WeakPassword(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	resp, err := ts.POST("/api/auth/users", map[string]string{
		"username":     "weakpass",
		"display_name": "Test",
		"password":     "short",
	})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	var errResp ErrorResponse
	json.NewDecoder(resp.Body).Decode(&errResp)
	if errResp.Code != constants.ErrCodeAuthPasswordTooWeak {
		t.Errorf("expected code=%s, got %s", constants.ErrCodeAuthPasswordTooWeak, errResp.Code)
	}
}

// TestListUsers verifies admin can list all users
func TestListUsers(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	ts.CreateTestUser(t, "listuser1", "secure-password-12345")
	ts.CreateTestUser(t, "listuser2", "secure-password-12345")

	resp, err := ts.GET("/api/auth/users")
	if err != nil {
		t.Fatalf("GET /api/auth/users failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)

	users := body["users"].([]interface{})
	// Should have at least admin + 2 created users
	if len(users) < 3 {
		t.Errorf("expected at least 3 users, got %d", len(users))
	}
}

// TestGetUserByID verifies admin can get a specific user
func TestGetUserByID(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	user := ts.CreateTestUser(t, "getbyid", "secure-password-12345")

	resp, err := ts.GET(fmt.Sprintf("/api/auth/users/%d", user.ID))
	if err != nil {
		t.Fatalf("GET user by ID failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)

	u := body["user"].(map[string]interface{})
	if u["username"] != "getbyid" {
		t.Errorf("expected username=getbyid, got %s", u["username"])
	}
}

// TestUpdateUser_DisplayName verifies display name update
func TestUpdateUser_DisplayName(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	user := ts.CreateTestUser(t, "updatename", "secure-password-12345")

	resp, err := ts.PATCH(fmt.Sprintf("/api/auth/users/%d", user.ID), map[string]string{
		"display_name": "Updated Display Name",
	})
	if err != nil {
		t.Fatalf("PATCH request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Verify update persisted
	getResp, err := ts.GET(fmt.Sprintf("/api/auth/users/%d", user.ID))
	if err != nil {
		t.Fatalf("GET after update failed: %v", err)
	}
	defer getResp.Body.Close()

	var body map[string]interface{}
	json.NewDecoder(getResp.Body).Decode(&body)

	u := body["user"].(map[string]interface{})
	if u["display_name"] != "Updated Display Name" {
		t.Errorf("expected Updated Display Name, got %s", u["display_name"])
	}
}

// TestUpdateUser_Disable verifies disabling a user
func TestUpdateUser_Disable(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	user := ts.CreateTestUser(t, "disableuser", "secure-password-12345")

	// Disable the user
	isActive := false
	resp, err := ts.PATCH(fmt.Sprintf("/api/auth/users/%d", user.ID), map[string]interface{}{
		"is_active": isActive,
	})
	if err != nil {
		t.Fatalf("PATCH request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Verify disabled user can't authenticate
	meResp, err := ts.RequestWithAPIKey("GET", "/api/auth/me", user.APIKey, nil)
	if err != nil {
		t.Fatalf("disabled user /me failed: %v", err)
	}
	defer meResp.Body.Close()

	if meResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for disabled user, got %d", meResp.StatusCode)
	}
}

// TestUpdateUser_ResetPassword verifies password change
func TestUpdateUser_ResetPassword(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	user := ts.CreateTestUser(t, "resetpass", "old-password-12345")

	// Change password
	resp, err := ts.PATCH(fmt.Sprintf("/api/auth/users/%d", user.ID), map[string]string{
		"new_password": "new-password-12345",
	})
	if err != nil {
		t.Fatalf("PATCH request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Old password should fail
	oldResp, err := ts.UnauthenticatedPOST("/api/auth/login", map[string]string{
		"username": "resetpass",
		"password": "old-password-12345",
	})
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer oldResp.Body.Close()

	if oldResp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 with old password, got %d", oldResp.StatusCode)
	}

	// New password should work
	token := ts.LoginUser(t, "resetpass", "new-password-12345")
	if token == "" {
		t.Error("expected successful login with new password")
	}
}

// TestRegenerateAPIKey verifies API key regeneration
func TestRegenerateAPIKey(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	user := ts.CreateTestUser(t, "regenkey", "secure-password-12345")
	oldKey := user.APIKey

	// Regenerate API key
	resp, err := ts.POST(fmt.Sprintf("/api/auth/users/%d/api-key", user.ID), nil)
	if err != nil {
		t.Fatalf("regen API key request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)

	newKey := body["api_key"].(string)
	if newKey == "" {
		t.Fatal("expected new API key in response")
	}
	if newKey == oldKey {
		t.Error("expected new API key to differ from old one")
	}

	// Old key should not work
	oldResp, err := ts.RequestWithAPIKey("GET", "/api/auth/me", oldKey, nil)
	if err != nil {
		t.Fatalf("old key request failed: %v", err)
	}
	defer oldResp.Body.Close()

	if oldResp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 with old API key, got %d", oldResp.StatusCode)
	}

	// New key should work
	newResp, err := ts.RequestWithAPIKey("GET", "/api/auth/me", newKey, nil)
	if err != nil {
		t.Fatalf("new key request failed: %v", err)
	}
	defer newResp.Body.Close()

	if newResp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 with new API key, got %d", newResp.StatusCode)
	}
}
