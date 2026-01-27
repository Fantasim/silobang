package auth

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"meshbank/internal/constants"
	"meshbank/internal/database"
)

// setupTestDB creates an in-memory SQLite database with the orchestrator schema.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	schema := database.GetOrchestratorSchema()
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("failed to apply schema: %v", err)
	}
	return db
}

// setupTestStore creates a store backed by an in-memory DB.
func setupTestStore(t *testing.T) *Store {
	t.Helper()
	return NewStore(setupTestDB(t))
}

// ============================================================================
// User CRUD Tests
// ============================================================================

func TestCreateUser(t *testing.T) {
	store := setupTestStore(t)

	user, err := store.CreateUser("testuser", "Test User", "hash123", nil)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	if user.ID == 0 {
		t.Error("expected non-zero user ID")
	}
	if user.Username != "testuser" {
		t.Errorf("expected username 'testuser', got %q", user.Username)
	}
	if user.DisplayName != "Test User" {
		t.Errorf("expected display name 'Test User', got %q", user.DisplayName)
	}
	if !user.IsActive {
		t.Error("expected user to be active")
	}
	if user.IsBootstrap {
		t.Error("expected user not to be bootstrap")
	}
	if user.CreatedAt == 0 {
		t.Error("expected non-zero created_at")
	}
	if user.CreatedBy != nil {
		t.Error("expected nil created_by")
	}
}

func TestCreateUserWithCreatedBy(t *testing.T) {
	store := setupTestStore(t)

	admin, err := store.CreateUser("admin", "Admin", "hash", nil)
	if err != nil {
		t.Fatalf("CreateUser admin failed: %v", err)
	}

	user, err := store.CreateUser("child", "Child User", "hash", &admin.ID)
	if err != nil {
		t.Fatalf("CreateUser child failed: %v", err)
	}

	if user.CreatedBy == nil {
		t.Fatal("expected created_by to be set")
	}
	if *user.CreatedBy != admin.ID {
		t.Errorf("expected created_by=%d, got %d", admin.ID, *user.CreatedBy)
	}
}

func TestCreateUserDuplicate(t *testing.T) {
	store := setupTestStore(t)

	_, err := store.CreateUser("duplicate", "User 1", "hash1", nil)
	if err != nil {
		t.Fatalf("first CreateUser failed: %v", err)
	}

	_, err = store.CreateUser("duplicate", "User 2", "hash2", nil)
	if err == nil {
		t.Fatal("expected error for duplicate username")
	}
}

func TestCreateBootstrapUser(t *testing.T) {
	store := setupTestStore(t)

	user, err := store.CreateBootstrapUser("admin", "Administrator", "pwhash", "keyhash", "mbk_abcd")
	if err != nil {
		t.Fatalf("CreateBootstrapUser failed: %v", err)
	}

	if !user.IsBootstrap {
		t.Error("expected bootstrap user to have is_bootstrap=true")
	}
	if !user.IsActive {
		t.Error("expected bootstrap user to be active")
	}

	// Verify sensitive fields were stored
	sensitive, err := store.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}
	if sensitive.PasswordHash != "pwhash" {
		t.Errorf("expected password hash 'pwhash', got %q", sensitive.PasswordHash)
	}
	if sensitive.APIKeyHash != "keyhash" {
		t.Errorf("expected API key hash 'keyhash', got %q", sensitive.APIKeyHash)
	}
	if sensitive.APIKeyPrefix != "mbk_abcd" {
		t.Errorf("expected API key prefix 'mbk_abcd', got %q", sensitive.APIKeyPrefix)
	}
}

func TestGetUserByID(t *testing.T) {
	store := setupTestStore(t)

	created, _ := store.CreateUser("byid", "By ID", "hash", nil)

	found, err := store.GetUserByID(created.ID)
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}
	if found.Username != "byid" {
		t.Errorf("expected username 'byid', got %q", found.Username)
	}
}

func TestGetUserByIDNotFound(t *testing.T) {
	store := setupTestStore(t)

	_, err := store.GetUserByID(999)
	if err == nil {
		t.Fatal("expected error for non-existent user")
	}
}

func TestGetUserByUsername(t *testing.T) {
	store := setupTestStore(t)

	store.CreateUser("byname", "By Name", "hash", nil)

	found, err := store.GetUserByUsername("byname")
	if err != nil {
		t.Fatalf("GetUserByUsername failed: %v", err)
	}
	if found.Username != "byname" {
		t.Errorf("expected username 'byname', got %q", found.Username)
	}
}

func TestGetUserByUsernameNotFound(t *testing.T) {
	store := setupTestStore(t)

	_, err := store.GetUserByUsername("nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent username")
	}
}

func TestGetUserByAPIKeyHash(t *testing.T) {
	store := setupTestStore(t)

	store.CreateBootstrapUser("apiuser", "API User", "pwhash", "apikeyhash123", "mbk_1234")

	found, err := store.GetUserByAPIKeyHash("apikeyhash123")
	if err != nil {
		t.Fatalf("GetUserByAPIKeyHash failed: %v", err)
	}
	if found.Username != "apiuser" {
		t.Errorf("expected username 'apiuser', got %q", found.Username)
	}
}

func TestGetUserByAPIKeyHashNotFound(t *testing.T) {
	store := setupTestStore(t)

	_, err := store.GetUserByAPIKeyHash("nonexistenthash")
	if err == nil {
		t.Fatal("expected error for non-existent API key hash")
	}
}

func TestListUsers(t *testing.T) {
	store := setupTestStore(t)

	store.CreateUser("user1", "User 1", "hash1", nil)
	store.CreateUser("user2", "User 2", "hash2", nil)
	store.CreateUser("user3", "User 3", "hash3", nil)

	users, err := store.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers failed: %v", err)
	}
	if len(users) != 3 {
		t.Fatalf("expected 3 users, got %d", len(users))
	}

	// Verify ordering (by ID ascending)
	if users[0].Username != "user1" || users[1].Username != "user2" || users[2].Username != "user3" {
		t.Error("users not in expected order")
	}
}

func TestListUsersEmpty(t *testing.T) {
	store := setupTestStore(t)

	users, err := store.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers failed: %v", err)
	}
	if len(users) != 0 {
		t.Fatalf("expected 0 users, got %d", len(users))
	}
}

func TestUpdateUser(t *testing.T) {
	store := setupTestStore(t)

	user, _ := store.CreateUser("updateme", "Original", "hash", nil)

	err := store.UpdateUser(user.ID, "Updated Name", false)
	if err != nil {
		t.Fatalf("UpdateUser failed: %v", err)
	}

	updated, _ := store.GetUserByID(user.ID)
	if updated.DisplayName != "Updated Name" {
		t.Errorf("expected display name 'Updated Name', got %q", updated.DisplayName)
	}
	if updated.IsActive {
		t.Error("expected user to be inactive after update")
	}
	if updated.UpdatedAt < user.UpdatedAt {
		t.Error("expected updated_at to be >= original")
	}
}

func TestUpdateUserPassword(t *testing.T) {
	store := setupTestStore(t)

	user, _ := store.CreateUser("pwupdate", "PW User", "oldhash", nil)

	err := store.UpdateUserPassword(user.ID, "newhash")
	if err != nil {
		t.Fatalf("UpdateUserPassword failed: %v", err)
	}

	updated, _ := store.GetUserByID(user.ID)
	if updated.PasswordHash != "newhash" {
		t.Errorf("expected password hash 'newhash', got %q", updated.PasswordHash)
	}
}

func TestUpdateUserAPIKey(t *testing.T) {
	store := setupTestStore(t)

	user, _ := store.CreateUser("keyupdate", "Key User", "hash", nil)

	err := store.UpdateUserAPIKey(user.ID, "newkeyhash", "mbk_new1")
	if err != nil {
		t.Fatalf("UpdateUserAPIKey failed: %v", err)
	}

	updated, _ := store.GetUserByID(user.ID)
	if updated.APIKeyHash != "newkeyhash" {
		t.Errorf("expected API key hash 'newkeyhash', got %q", updated.APIKeyHash)
	}
	if updated.APIKeyPrefix != "mbk_new1" {
		t.Errorf("expected API key prefix 'mbk_new1', got %q", updated.APIKeyPrefix)
	}
}

func TestCountUsers(t *testing.T) {
	store := setupTestStore(t)

	count, _ := store.CountUsers()
	if count != 0 {
		t.Fatalf("expected 0 users, got %d", count)
	}

	store.CreateUser("a", "A", "h", nil)
	store.CreateUser("b", "B", "h", nil)

	count, _ = store.CountUsers()
	if count != 2 {
		t.Fatalf("expected 2 users, got %d", count)
	}
}

// ============================================================================
// Login Lockout Tests
// ============================================================================

func TestIncrementFailedLogin(t *testing.T) {
	store := setupTestStore(t)

	user, _ := store.CreateUser("lockme", "Lock Me", "hash", nil)

	// Increment a few times (below threshold)
	for i := 0; i < constants.AuthMaxLoginAttempts-1; i++ {
		if err := store.IncrementFailedLogin(user.ID); err != nil {
			t.Fatalf("IncrementFailedLogin failed: %v", err)
		}
	}

	u, _ := store.GetUserByID(user.ID)
	if u.FailedLoginCount != constants.AuthMaxLoginAttempts-1 {
		t.Errorf("expected %d failed logins, got %d", constants.AuthMaxLoginAttempts-1, u.FailedLoginCount)
	}
	if u.LockedUntil != nil {
		t.Error("expected account not to be locked yet")
	}
}

func TestIncrementFailedLoginLocksAccount(t *testing.T) {
	store := setupTestStore(t)

	user, _ := store.CreateUser("lockout", "Lockout", "hash", nil)

	// Hit the threshold
	for i := 0; i < constants.AuthMaxLoginAttempts; i++ {
		store.IncrementFailedLogin(user.ID)
	}

	u, _ := store.GetUserByID(user.ID)
	if u.LockedUntil == nil {
		t.Fatal("expected account to be locked after max attempts")
	}

	// locked_until should be in the future
	now := time.Now().Unix()
	if *u.LockedUntil <= now {
		t.Error("expected locked_until to be in the future")
	}
}

func TestResetFailedLogin(t *testing.T) {
	store := setupTestStore(t)

	user, _ := store.CreateUser("resetme", "Reset Me", "hash", nil)

	// Increment and lock
	for i := 0; i < constants.AuthMaxLoginAttempts; i++ {
		store.IncrementFailedLogin(user.ID)
	}

	// Reset
	if err := store.ResetFailedLogin(user.ID); err != nil {
		t.Fatalf("ResetFailedLogin failed: %v", err)
	}

	u, _ := store.GetUserByID(user.ID)
	if u.FailedLoginCount != 0 {
		t.Errorf("expected 0 failed logins after reset, got %d", u.FailedLoginCount)
	}
	if u.LockedUntil != nil {
		t.Error("expected locked_until to be nil after reset")
	}
}

// ============================================================================
// Grant CRUD Tests
// ============================================================================

func TestCreateGrant(t *testing.T) {
	store := setupTestStore(t)

	user, _ := store.CreateUser("grantee", "Grantee", "hash", nil)

	grant, err := store.CreateGrant(user.ID, constants.AuthActionUpload, nil, user.ID)
	if err != nil {
		t.Fatalf("CreateGrant failed: %v", err)
	}

	if grant.ID == 0 {
		t.Error("expected non-zero grant ID")
	}
	if grant.UserID != user.ID {
		t.Errorf("expected user_id=%d, got %d", user.ID, grant.UserID)
	}
	if grant.Action != constants.AuthActionUpload {
		t.Errorf("expected action %q, got %q", constants.AuthActionUpload, grant.Action)
	}
	if !grant.IsActive {
		t.Error("expected grant to be active")
	}
	if grant.ConstraintsJSON != nil {
		t.Error("expected nil constraints for unrestricted grant")
	}
}

func TestCreateGrantWithConstraints(t *testing.T) {
	store := setupTestStore(t)

	user, _ := store.CreateUser("constrained", "Constrained", "hash", nil)

	constraints := `{"allowed_extensions":["png","jpg"]}`
	grant, err := store.CreateGrant(user.ID, constants.AuthActionUpload, &constraints, user.ID)
	if err != nil {
		t.Fatalf("CreateGrant with constraints failed: %v", err)
	}

	if grant.ConstraintsJSON == nil {
		t.Fatal("expected non-nil constraints")
	}
	if *grant.ConstraintsJSON != constraints {
		t.Errorf("expected constraints %q, got %q", constraints, *grant.ConstraintsJSON)
	}
}

func TestCreateGrantLogsToChangelog(t *testing.T) {
	store := setupTestStore(t)

	user, _ := store.CreateUser("logged", "Logged", "hash", nil)
	store.CreateGrant(user.ID, constants.AuthActionUpload, nil, user.ID)

	log, err := store.GetGrantLog(user.ID, 10)
	if err != nil {
		t.Fatalf("GetGrantLog failed: %v", err)
	}
	if len(log) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(log))
	}
	if log[0].ChangeType != constants.AuthGrantChangeCreated {
		t.Errorf("expected change_type %q, got %q", constants.AuthGrantChangeCreated, log[0].ChangeType)
	}
	if log[0].Action != constants.AuthActionUpload {
		t.Errorf("expected action %q, got %q", constants.AuthActionUpload, log[0].Action)
	}
}

func TestGetGrantByID(t *testing.T) {
	store := setupTestStore(t)

	user, _ := store.CreateUser("grantid", "Grant ID", "hash", nil)
	created, _ := store.CreateGrant(user.ID, constants.AuthActionDownload, nil, user.ID)

	found, err := store.GetGrantByID(created.ID)
	if err != nil {
		t.Fatalf("GetGrantByID failed: %v", err)
	}
	if found.Action != constants.AuthActionDownload {
		t.Errorf("expected action %q, got %q", constants.AuthActionDownload, found.Action)
	}
}

func TestGetGrantByIDNotFound(t *testing.T) {
	store := setupTestStore(t)

	_, err := store.GetGrantByID(999)
	if err == nil {
		t.Fatal("expected error for non-existent grant")
	}
}

func TestGetActiveGrantsForUser(t *testing.T) {
	store := setupTestStore(t)

	user, _ := store.CreateUser("active-grants", "Active Grants", "hash", nil)

	store.CreateGrant(user.ID, constants.AuthActionUpload, nil, user.ID)
	store.CreateGrant(user.ID, constants.AuthActionDownload, nil, user.ID)
	grant3, _ := store.CreateGrant(user.ID, constants.AuthActionQuery, nil, user.ID)

	// Revoke one grant
	store.RevokeGrant(grant3.ID, user.ID)

	grants, err := store.GetActiveGrantsForUser(user.ID)
	if err != nil {
		t.Fatalf("GetActiveGrantsForUser failed: %v", err)
	}
	if len(grants) != 2 {
		t.Fatalf("expected 2 active grants, got %d", len(grants))
	}
}

func TestGetAllGrantsForUser(t *testing.T) {
	store := setupTestStore(t)

	user, _ := store.CreateUser("all-grants", "All Grants", "hash", nil)

	store.CreateGrant(user.ID, constants.AuthActionUpload, nil, user.ID)
	store.CreateGrant(user.ID, constants.AuthActionDownload, nil, user.ID)
	grant3, _ := store.CreateGrant(user.ID, constants.AuthActionQuery, nil, user.ID)

	// Revoke one
	store.RevokeGrant(grant3.ID, user.ID)

	grants, err := store.GetAllGrantsForUser(user.ID)
	if err != nil {
		t.Fatalf("GetAllGrantsForUser failed: %v", err)
	}
	if len(grants) != 3 {
		t.Fatalf("expected 3 total grants (including revoked), got %d", len(grants))
	}
}

func TestUpdateGrantConstraints(t *testing.T) {
	store := setupTestStore(t)

	user, _ := store.CreateUser("update-grant", "Update Grant", "hash", nil)
	grant, _ := store.CreateGrant(user.ID, constants.AuthActionUpload, nil, user.ID)

	newConstraints := `{"max_file_size_bytes":1048576}`
	err := store.UpdateGrantConstraints(grant.ID, &newConstraints, user.ID)
	if err != nil {
		t.Fatalf("UpdateGrantConstraints failed: %v", err)
	}

	updated, _ := store.GetGrantByID(grant.ID)
	if updated.ConstraintsJSON == nil || *updated.ConstraintsJSON != newConstraints {
		t.Errorf("expected constraints %q, got %v", newConstraints, updated.ConstraintsJSON)
	}
}

func TestUpdateGrantConstraintsLogsChange(t *testing.T) {
	store := setupTestStore(t)

	user, _ := store.CreateUser("update-log", "Update Log", "hash", nil)
	grant, _ := store.CreateGrant(user.ID, constants.AuthActionUpload, nil, user.ID)

	newConstraints := `{"max_file_size_bytes":1048576}`
	store.UpdateGrantConstraints(grant.ID, &newConstraints, user.ID)

	log, _ := store.GetGrantLog(user.ID, 10)
	if len(log) != 2 { // created + updated
		t.Fatalf("expected 2 log entries, got %d", len(log))
	}

	// Most recent first (DESC order)
	if log[0].ChangeType != constants.AuthGrantChangeUpdated {
		t.Errorf("expected latest change to be %q, got %q", constants.AuthGrantChangeUpdated, log[0].ChangeType)
	}
}

func TestRevokeGrant(t *testing.T) {
	store := setupTestStore(t)

	user, _ := store.CreateUser("revoke", "Revoke", "hash", nil)
	grant, _ := store.CreateGrant(user.ID, constants.AuthActionUpload, nil, user.ID)

	err := store.RevokeGrant(grant.ID, user.ID)
	if err != nil {
		t.Fatalf("RevokeGrant failed: %v", err)
	}

	revoked, _ := store.GetGrantByID(grant.ID)
	if revoked.IsActive {
		t.Error("expected grant to be inactive after revocation")
	}
}

func TestRevokeGrantLogsChange(t *testing.T) {
	store := setupTestStore(t)

	user, _ := store.CreateUser("revoke-log", "Revoke Log", "hash", nil)
	grant, _ := store.CreateGrant(user.ID, constants.AuthActionUpload, nil, user.ID)

	store.RevokeGrant(grant.ID, user.ID)

	log, _ := store.GetGrantLog(user.ID, 10)
	if len(log) != 2 { // created + revoked
		t.Fatalf("expected 2 log entries, got %d", len(log))
	}
	if log[0].ChangeType != constants.AuthGrantChangeRevoked {
		t.Errorf("expected latest change to be %q, got %q", constants.AuthGrantChangeRevoked, log[0].ChangeType)
	}
}

func TestRevokeGrantNotFound(t *testing.T) {
	store := setupTestStore(t)

	err := store.RevokeGrant(999, 1)
	if err == nil {
		t.Fatal("expected error for revoking non-existent grant")
	}
}

// ============================================================================
// Quota Usage Tests
// ============================================================================

func TestGetTodayUsageEmpty(t *testing.T) {
	store := setupTestStore(t)

	user, _ := store.CreateUser("quota-empty", "Quota Empty", "hash", nil)

	usage, err := store.GetTodayUsage(user.ID, constants.AuthActionUpload)
	if err != nil {
		t.Fatalf("GetTodayUsage failed: %v", err)
	}

	if usage.RequestCount != 0 {
		t.Errorf("expected 0 request count, got %d", usage.RequestCount)
	}
	if usage.TotalBytes != 0 {
		t.Errorf("expected 0 total bytes, got %d", usage.TotalBytes)
	}
	if usage.UserID != user.ID {
		t.Errorf("expected user_id=%d, got %d", user.ID, usage.UserID)
	}
}

func TestIncrementQuota(t *testing.T) {
	store := setupTestStore(t)

	user, _ := store.CreateUser("quota-inc", "Quota Inc", "hash", nil)

	// First increment (creates row)
	err := store.IncrementQuota(user.ID, constants.AuthActionUpload, 1, 1024)
	if err != nil {
		t.Fatalf("IncrementQuota failed: %v", err)
	}

	usage, _ := store.GetTodayUsage(user.ID, constants.AuthActionUpload)
	if usage.RequestCount != 1 {
		t.Errorf("expected 1 request, got %d", usage.RequestCount)
	}
	if usage.TotalBytes != 1024 {
		t.Errorf("expected 1024 bytes, got %d", usage.TotalBytes)
	}

	// Second increment (updates row via upsert)
	store.IncrementQuota(user.ID, constants.AuthActionUpload, 1, 2048)

	usage, _ = store.GetTodayUsage(user.ID, constants.AuthActionUpload)
	if usage.RequestCount != 2 {
		t.Errorf("expected 2 requests, got %d", usage.RequestCount)
	}
	if usage.TotalBytes != 3072 {
		t.Errorf("expected 3072 bytes, got %d", usage.TotalBytes)
	}
}

func TestGetAllQuotaUsage(t *testing.T) {
	store := setupTestStore(t)

	user, _ := store.CreateUser("quota-all", "Quota All", "hash", nil)

	store.IncrementQuota(user.ID, constants.AuthActionUpload, 1, 1024)
	store.IncrementQuota(user.ID, constants.AuthActionDownload, 3, 4096)

	usages, err := store.GetAllQuotaUsage(user.ID)
	if err != nil {
		t.Fatalf("GetAllQuotaUsage failed: %v", err)
	}
	if len(usages) != 2 {
		t.Fatalf("expected 2 quota records, got %d", len(usages))
	}
}

func TestQuotaIsolation(t *testing.T) {
	store := setupTestStore(t)

	user1, _ := store.CreateUser("quota-iso1", "Iso 1", "hash", nil)
	user2, _ := store.CreateUser("quota-iso2", "Iso 2", "hash", nil)

	store.IncrementQuota(user1.ID, constants.AuthActionUpload, 5, 5000)
	store.IncrementQuota(user2.ID, constants.AuthActionUpload, 3, 3000)

	usage1, _ := store.GetTodayUsage(user1.ID, constants.AuthActionUpload)
	usage2, _ := store.GetTodayUsage(user2.ID, constants.AuthActionUpload)

	if usage1.RequestCount != 5 {
		t.Errorf("user1 expected 5 requests, got %d", usage1.RequestCount)
	}
	if usage2.RequestCount != 3 {
		t.Errorf("user2 expected 3 requests, got %d", usage2.RequestCount)
	}
}

// ============================================================================
// Session Tests
// ============================================================================

func TestCreateSession(t *testing.T) {
	store := setupTestStore(t)

	user, _ := store.CreateUser("session-user", "Session User", "hash", nil)

	session, err := store.CreateSession("tokenhash", "mbs_abc", user.ID, "127.0.0.1", "TestAgent/1.0")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if session.TokenHash != "tokenhash" {
		t.Errorf("expected token hash 'tokenhash', got %q", session.TokenHash)
	}
	if session.UserID != user.ID {
		t.Errorf("expected user_id=%d, got %d", user.ID, session.UserID)
	}
	if session.IPAddress != "127.0.0.1" {
		t.Errorf("expected IP '127.0.0.1', got %q", session.IPAddress)
	}
	if session.ExpiresAt <= session.CreatedAt {
		t.Error("expected expires_at > created_at")
	}
}

func TestGetSessionByTokenHash(t *testing.T) {
	store := setupTestStore(t)

	user, _ := store.CreateUser("getsession", "Get Session", "hash", nil)
	store.CreateSession("lookuptoken", "mbs_look", user.ID, "127.0.0.1", "Test")

	session, foundUser, err := store.GetSessionByTokenHash("lookuptoken")
	if err != nil {
		t.Fatalf("GetSessionByTokenHash failed: %v", err)
	}
	if session == nil {
		t.Fatal("expected session to be found")
	}
	if foundUser == nil {
		t.Fatal("expected user to be found")
	}
	if foundUser.Username != "getsession" {
		t.Errorf("expected username 'getsession', got %q", foundUser.Username)
	}
}

func TestGetSessionByTokenHashNotFound(t *testing.T) {
	store := setupTestStore(t)

	session, user, err := store.GetSessionByTokenHash("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session != nil || user != nil {
		t.Fatal("expected nil session and user for non-existent token")
	}
}

func TestGetSessionByTokenHashExpired(t *testing.T) {
	store := setupTestStore(t)

	user, _ := store.CreateUser("expired-session", "Expired", "hash", nil)

	// Insert an already-expired session directly
	now := time.Now().Unix()
	store.db.Exec(`
		INSERT INTO auth_sessions (token_hash, token_prefix, user_id, ip_address, user_agent,
		                           created_at, expires_at, last_active_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "expiredtoken", "mbs_exp", user.ID, "127.0.0.1", "Test",
		now-7200, now-3600, now-3600) // created 2h ago, expired 1h ago

	session, foundUser, err := store.GetSessionByTokenHash("expiredtoken")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session != nil || foundUser != nil {
		t.Fatal("expected nil for expired session")
	}
}

func TestGetSessionByTokenHashInactiveUser(t *testing.T) {
	store := setupTestStore(t)

	user, _ := store.CreateUser("inactive-session", "Inactive", "hash", nil)
	store.CreateSession("inactivetoken", "mbs_ina", user.ID, "127.0.0.1", "Test")

	// Disable the user
	store.UpdateUser(user.ID, "Inactive", false)

	session, foundUser, err := store.GetSessionByTokenHash("inactivetoken")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session != nil || foundUser != nil {
		t.Fatal("expected nil for session of inactive user")
	}
}

func TestTouchSession(t *testing.T) {
	store := setupTestStore(t)

	user, _ := store.CreateUser("touch-session", "Touch Session", "hash", nil)
	store.CreateSession("touchtoken", "mbs_tou", user.ID, "127.0.0.1", "Test")

	// Small delay to ensure timestamp changes
	time.Sleep(10 * time.Millisecond)

	err := store.TouchSession("touchtoken")
	if err != nil {
		t.Fatalf("TouchSession failed: %v", err)
	}

	session, _, _ := store.GetSessionByTokenHash("touchtoken")
	if session == nil {
		t.Fatal("session should still be valid")
	}
	// last_active_at should be updated (can't easily verify the exact value in a race-free way,
	// but the session should still be valid)
}

func TestDeleteSession(t *testing.T) {
	store := setupTestStore(t)

	user, _ := store.CreateUser("delete-session", "Delete Session", "hash", nil)
	store.CreateSession("deletetoken", "mbs_del", user.ID, "127.0.0.1", "Test")

	err := store.DeleteSession("deletetoken")
	if err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}

	session, _, _ := store.GetSessionByTokenHash("deletetoken")
	if session != nil {
		t.Fatal("expected session to be deleted")
	}
}

func TestDeleteUserSessions(t *testing.T) {
	store := setupTestStore(t)

	user, _ := store.CreateUser("delete-all", "Delete All", "hash", nil)
	store.CreateSession("tok1", "mbs_t1", user.ID, "127.0.0.1", "Test")
	store.CreateSession("tok2", "mbs_t2", user.ID, "127.0.0.1", "Test")
	store.CreateSession("tok3", "mbs_t3", user.ID, "127.0.0.1", "Test")

	err := store.DeleteUserSessions(user.ID)
	if err != nil {
		t.Fatalf("DeleteUserSessions failed: %v", err)
	}

	// All sessions should be gone
	for _, hash := range []string{"tok1", "tok2", "tok3"} {
		session, _, _ := store.GetSessionByTokenHash(hash)
		if session != nil {
			t.Errorf("expected session %q to be deleted", hash)
		}
	}
}

func TestCleanupExpiredSessions(t *testing.T) {
	store := setupTestStore(t)

	user, _ := store.CreateUser("cleanup-user", "Cleanup User", "hash", nil)

	now := time.Now().Unix()

	// Insert expired session directly
	store.db.Exec(`
		INSERT INTO auth_sessions (token_hash, token_prefix, user_id, ip_address, user_agent,
		                           created_at, expires_at, last_active_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "expired1", "mbs_ex1", user.ID, "127.0.0.1", "Test",
		now-7200, now-3600, now-3600)

	// Insert valid session
	store.CreateSession("valid1", "mbs_val", user.ID, "127.0.0.1", "Test")

	removed, err := store.CleanupExpiredSessions()
	if err != nil {
		t.Fatalf("CleanupExpiredSessions failed: %v", err)
	}
	if removed != 1 {
		t.Errorf("expected 1 session removed, got %d", removed)
	}

	// Valid session should still exist
	session, _, _ := store.GetSessionByTokenHash("valid1")
	if session == nil {
		t.Fatal("valid session should not be removed by cleanup")
	}
}

// ============================================================================
// Grant Log Immutability Tests
// ============================================================================

func TestGrantLogImmutability(t *testing.T) {
	store := setupTestStore(t)

	user, _ := store.CreateUser("immutable-log", "Immutable", "hash", nil)

	// Create, update, revoke a grant — each should produce a log entry
	constraints := `{"max_file_size_bytes":1000}`
	grant, _ := store.CreateGrant(user.ID, constants.AuthActionUpload, nil, user.ID)
	store.UpdateGrantConstraints(grant.ID, &constraints, user.ID)
	store.RevokeGrant(grant.ID, user.ID)

	log, err := store.GetGrantLog(user.ID, 10)
	if err != nil {
		t.Fatalf("GetGrantLog failed: %v", err)
	}
	if len(log) != 3 {
		t.Fatalf("expected 3 log entries (created + updated + revoked), got %d", len(log))
	}

	// Entries should be in reverse chronological order
	if log[0].ChangeType != constants.AuthGrantChangeRevoked {
		t.Errorf("expected most recent entry to be 'revoked', got %q", log[0].ChangeType)
	}
	if log[1].ChangeType != constants.AuthGrantChangeUpdated {
		t.Errorf("expected middle entry to be 'updated', got %q", log[1].ChangeType)
	}
	if log[2].ChangeType != constants.AuthGrantChangeCreated {
		t.Errorf("expected oldest entry to be 'created', got %q", log[2].ChangeType)
	}

	// Verify updated entry has old and new constraints
	if log[1].OldConstraintsJSON != nil {
		t.Errorf("expected old constraints to be nil (was unrestricted), got %v", log[1].OldConstraintsJSON)
	}
	if log[1].NewConstraintsJSON == nil || *log[1].NewConstraintsJSON != constraints {
		t.Errorf("expected new constraints %q, got %v", constraints, log[1].NewConstraintsJSON)
	}
}

func TestGrantLogLimitParam(t *testing.T) {
	store := setupTestStore(t)

	user, _ := store.CreateUser("log-limit", "Log Limit", "hash", nil)

	// Create 5 grants
	for i := 0; i < 5; i++ {
		store.CreateGrant(user.ID, constants.AuthActionUpload, nil, user.ID)
	}

	log, _ := store.GetGrantLog(user.ID, 3)
	if len(log) != 3 {
		t.Fatalf("expected 3 log entries with limit=3, got %d", len(log))
	}
}

// ============================================================================
// Multi-User Isolation Tests
// ============================================================================

func TestGrantIsolationBetweenUsers(t *testing.T) {
	store := setupTestStore(t)

	user1, _ := store.CreateUser("iso1", "Iso 1", "hash", nil)
	user2, _ := store.CreateUser("iso2", "Iso 2", "hash", nil)

	store.CreateGrant(user1.ID, constants.AuthActionUpload, nil, user1.ID)
	store.CreateGrant(user1.ID, constants.AuthActionDownload, nil, user1.ID)
	store.CreateGrant(user2.ID, constants.AuthActionQuery, nil, user2.ID)

	grants1, _ := store.GetActiveGrantsForUser(user1.ID)
	grants2, _ := store.GetActiveGrantsForUser(user2.ID)

	if len(grants1) != 2 {
		t.Errorf("user1 expected 2 grants, got %d", len(grants1))
	}
	if len(grants2) != 1 {
		t.Errorf("user2 expected 1 grant, got %d", len(grants2))
	}
}

func TestSessionIsolationBetweenUsers(t *testing.T) {
	store := setupTestStore(t)

	user1, _ := store.CreateUser("sess-iso1", "Sess Iso 1", "hash", nil)
	user2, _ := store.CreateUser("sess-iso2", "Sess Iso 2", "hash", nil)

	store.CreateSession("u1tok1", "mbs_u1t1", user1.ID, "127.0.0.1", "Test")
	store.CreateSession("u1tok2", "mbs_u1t2", user1.ID, "127.0.0.1", "Test")
	store.CreateSession("u2tok1", "mbs_u2t1", user2.ID, "127.0.0.1", "Test")

	// Delete user1's sessions
	store.DeleteUserSessions(user1.ID)

	// User2's session should still exist
	session, _, _ := store.GetSessionByTokenHash("u2tok1")
	if session == nil {
		t.Fatal("user2's session should not be affected by user1's session deletion")
	}
}
