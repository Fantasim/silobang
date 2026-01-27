package auth

import (
	"encoding/json"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"meshbank/internal/constants"
	"meshbank/internal/logger"
)

// setupEvaluator creates a PolicyEvaluator backed by an in-memory DB.
func setupEvaluator(t *testing.T) (*PolicyEvaluator, *Store) {
	t.Helper()
	store := setupTestStore(t)
	log := logger.NewLogger(logger.LevelError)
	eval := NewPolicyEvaluator(store, log)
	return eval, store
}

// makeIdentity creates an Identity with the given user and grants.
func makeIdentity(user *User, grants []Grant) *Identity {
	return &Identity{User: user, Grants: grants, Method: "api_key"}
}

// marshalConstraints converts a constraint struct to a *string for grant creation.
func marshalConstraints(t *testing.T, v interface{}) *string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("failed to marshal constraints: %v", err)
	}
	s := string(data)
	return &s
}

// ============================================================================
// Fundamental Authorization Tests
// ============================================================================

func TestEvaluate_NilIdentity(t *testing.T) {
	eval, _ := setupEvaluator(t)

	result := eval.Evaluate(nil, &ActionContext{Action: constants.AuthActionUpload})
	if result.Allowed {
		t.Fatal("expected denial for nil identity")
	}
	if result.DeniedCode != constants.ErrCodeAuthRequired {
		t.Errorf("expected code %q, got %q", constants.ErrCodeAuthRequired, result.DeniedCode)
	}
}

func TestEvaluate_DisabledUser(t *testing.T) {
	eval, _ := setupEvaluator(t)

	user := &User{ID: 1, Username: "disabled", IsActive: false}
	identity := makeIdentity(user, nil)

	result := eval.Evaluate(identity, &ActionContext{Action: constants.AuthActionUpload})
	if result.Allowed {
		t.Fatal("expected denial for disabled user")
	}
	if result.DeniedCode != constants.ErrCodeAuthUserDisabled {
		t.Errorf("expected code %q, got %q", constants.ErrCodeAuthUserDisabled, result.DeniedCode)
	}
}

func TestEvaluate_NoGrants(t *testing.T) {
	eval, _ := setupEvaluator(t)

	user := &User{ID: 1, Username: "noperms", IsActive: true}
	identity := makeIdentity(user, []Grant{})

	result := eval.Evaluate(identity, &ActionContext{Action: constants.AuthActionUpload})
	if result.Allowed {
		t.Fatal("expected denial for user with no grants")
	}
	if result.DeniedCode != constants.ErrCodeAuthForbidden {
		t.Errorf("expected code %q, got %q", constants.ErrCodeAuthForbidden, result.DeniedCode)
	}
}

func TestEvaluate_WrongAction(t *testing.T) {
	eval, _ := setupEvaluator(t)

	user := &User{ID: 1, Username: "wrongaction", IsActive: true}
	grants := []Grant{{ID: 1, UserID: 1, Action: constants.AuthActionDownload, IsActive: true}}
	identity := makeIdentity(user, grants)

	result := eval.Evaluate(identity, &ActionContext{Action: constants.AuthActionUpload})
	if result.Allowed {
		t.Fatal("expected denial when user has download grant but requests upload")
	}
}

func TestEvaluate_UnrestrictedGrant(t *testing.T) {
	eval, _ := setupEvaluator(t)

	user := &User{ID: 1, Username: "unrestricted", IsActive: true}
	grants := []Grant{{ID: 1, UserID: 1, Action: constants.AuthActionUpload, IsActive: true}}
	identity := makeIdentity(user, grants)

	result := eval.Evaluate(identity, &ActionContext{Action: constants.AuthActionUpload})
	if !result.Allowed {
		t.Fatalf("expected allow for unrestricted grant, got: %s", result.Reason)
	}
	if result.MatchedGrant == nil {
		t.Fatal("expected matched grant")
	}
}

func TestEvaluate_InactiveGrantIgnored(t *testing.T) {
	eval, _ := setupEvaluator(t)

	user := &User{ID: 1, Username: "inactivegrant", IsActive: true}
	grants := []Grant{{ID: 1, UserID: 1, Action: constants.AuthActionUpload, IsActive: false}}
	identity := makeIdentity(user, grants)

	result := eval.Evaluate(identity, &ActionContext{Action: constants.AuthActionUpload})
	if result.Allowed {
		t.Fatal("expected denial when only grant is inactive")
	}
}

func TestEvaluate_EmptyConstraintsAllowed(t *testing.T) {
	eval, _ := setupEvaluator(t)

	user := &User{ID: 1, Username: "empty-constraints", IsActive: true}

	// Test various "empty" constraint values
	emptyCases := []*string{nil, strPtr(""), strPtr("{}"), strPtr("null")}
	for _, c := range emptyCases {
		grants := []Grant{{ID: 1, UserID: 1, Action: constants.AuthActionUpload, IsActive: true, ConstraintsJSON: c}}
		identity := makeIdentity(user, grants)
		result := eval.Evaluate(identity, &ActionContext{Action: constants.AuthActionUpload})
		if !result.Allowed {
			t.Errorf("expected allow for empty constraints %v, got: %s", c, result.Reason)
		}
	}
}

// ============================================================================
// Upload Constraint Tests
// ============================================================================

func TestEvaluateUpload_AllowedExtensions(t *testing.T) {
	eval, _ := setupEvaluator(t)

	user := &User{ID: 1, Username: "ext-check", IsActive: true}
	constraints := UploadConstraints{AllowedExtensions: []string{"png", "jpg"}}

	tests := []struct {
		name      string
		extension string
		allowed   bool
	}{
		{"allowed extension png", "png", true},
		{"allowed extension jpg", "jpg", true},
		{"disallowed extension gif", "gif", false},
		{"disallowed extension exe", "exe", false},
		{"empty extension (no check)", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grants := []Grant{{ID: 1, UserID: 1, Action: constants.AuthActionUpload, IsActive: true,
				ConstraintsJSON: marshalConstraints(t, constraints)}}
			identity := makeIdentity(user, grants)

			result := eval.Evaluate(identity, &ActionContext{
				Action:    constants.AuthActionUpload,
				Extension: tt.extension,
			})

			if result.Allowed != tt.allowed {
				t.Errorf("expected allowed=%v, got %v (reason: %s)", tt.allowed, result.Allowed, result.Reason)
			}
			if !tt.allowed && result.DeniedCode != constants.ErrCodeAuthConstraintViolation {
				t.Errorf("expected code %q, got %q", constants.ErrCodeAuthConstraintViolation, result.DeniedCode)
			}
		})
	}
}

func TestEvaluateUpload_MaxFileSize(t *testing.T) {
	eval, _ := setupEvaluator(t)

	user := &User{ID: 1, Username: "size-check", IsActive: true}
	constraints := UploadConstraints{MaxFileSizeBytes: 1024}

	tests := []struct {
		name    string
		size    int64
		allowed bool
	}{
		{"within limit", 512, true},
		{"exact limit", 1024, true},
		{"over limit", 1025, false},
		{"zero size (no check)", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grants := []Grant{{ID: 1, UserID: 1, Action: constants.AuthActionUpload, IsActive: true,
				ConstraintsJSON: marshalConstraints(t, constraints)}}
			identity := makeIdentity(user, grants)

			result := eval.Evaluate(identity, &ActionContext{
				Action:   constants.AuthActionUpload,
				FileSize: tt.size,
			})

			if result.Allowed != tt.allowed {
				t.Errorf("expected allowed=%v, got %v (reason: %s)", tt.allowed, result.Allowed, result.Reason)
			}
		})
	}
}

func TestEvaluateUpload_AllowedTopics(t *testing.T) {
	eval, _ := setupEvaluator(t)

	user := &User{ID: 1, Username: "topic-check", IsActive: true}
	constraints := UploadConstraints{AllowedTopics: []string{"topic-a", "topic-b"}}

	tests := []struct {
		name    string
		topic   string
		allowed bool
	}{
		{"allowed topic-a", "topic-a", true},
		{"allowed topic-b", "topic-b", true},
		{"disallowed topic-c", "topic-c", false},
		{"empty topic (no check)", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grants := []Grant{{ID: 1, UserID: 1, Action: constants.AuthActionUpload, IsActive: true,
				ConstraintsJSON: marshalConstraints(t, constraints)}}
			identity := makeIdentity(user, grants)

			result := eval.Evaluate(identity, &ActionContext{
				Action:    constants.AuthActionUpload,
				TopicName: tt.topic,
			})

			if result.Allowed != tt.allowed {
				t.Errorf("expected allowed=%v, got %v (reason: %s)", tt.allowed, result.Allowed, result.Reason)
			}
		})
	}
}

func TestEvaluateUpload_DailyCountLimit(t *testing.T) {
	eval, store := setupEvaluator(t)

	user, _ := store.CreateUser("quota-count", "Quota Count", "hash", nil)
	constraints := UploadConstraints{DailyCountLimit: 3}

	grants := []Grant{{ID: 1, UserID: user.ID, Action: constants.AuthActionUpload, IsActive: true,
		ConstraintsJSON: marshalConstraints(t, constraints)}}
	identity := makeIdentity(user, grants)

	// First 3 should be allowed
	for i := 0; i < 3; i++ {
		result := eval.Evaluate(identity, &ActionContext{Action: constants.AuthActionUpload})
		if !result.Allowed {
			t.Fatalf("upload %d should be allowed", i+1)
		}
		store.IncrementQuota(user.ID, constants.AuthActionUpload, 1, 0)
	}

	// 4th should be denied
	result := eval.Evaluate(identity, &ActionContext{Action: constants.AuthActionUpload})
	if result.Allowed {
		t.Fatal("expected denial when daily count limit exceeded")
	}
	if result.DeniedCode != constants.ErrCodeAuthQuotaExceeded {
		t.Errorf("expected code %q, got %q", constants.ErrCodeAuthQuotaExceeded, result.DeniedCode)
	}
}

func TestEvaluateUpload_DailyVolumeLimit(t *testing.T) {
	eval, store := setupEvaluator(t)

	user, _ := store.CreateUser("quota-vol", "Quota Vol", "hash", nil)
	constraints := UploadConstraints{DailyVolumeBytes: 1000}

	grants := []Grant{{ID: 1, UserID: user.ID, Action: constants.AuthActionUpload, IsActive: true,
		ConstraintsJSON: marshalConstraints(t, constraints)}}
	identity := makeIdentity(user, grants)

	// Upload 800 bytes — should be allowed
	result := eval.Evaluate(identity, &ActionContext{Action: constants.AuthActionUpload, FileSize: 800})
	if !result.Allowed {
		t.Fatalf("first upload should be allowed: %s", result.Reason)
	}
	store.IncrementQuota(user.ID, constants.AuthActionUpload, 1, 800)

	// Upload 201 bytes — would exceed 1000, should be denied
	result = eval.Evaluate(identity, &ActionContext{Action: constants.AuthActionUpload, FileSize: 201})
	if result.Allowed {
		t.Fatal("expected denial when volume limit would be exceeded")
	}
	if result.DeniedCode != constants.ErrCodeAuthQuotaExceeded {
		t.Errorf("expected code %q, got %q", constants.ErrCodeAuthQuotaExceeded, result.DeniedCode)
	}

	// Upload 200 bytes — exactly at limit, should be allowed
	result = eval.Evaluate(identity, &ActionContext{Action: constants.AuthActionUpload, FileSize: 200})
	if !result.Allowed {
		t.Fatalf("upload at exact remaining capacity should be allowed: %s", result.Reason)
	}
}

// ============================================================================
// Download Constraint Tests
// ============================================================================

func TestEvaluateDownload_AllowedTopics(t *testing.T) {
	eval, _ := setupEvaluator(t)

	user := &User{ID: 1, Username: "dl-topics", IsActive: true}
	constraints := DownloadConstraints{AllowedTopics: []string{"public"}}

	grants := []Grant{{ID: 1, UserID: 1, Action: constants.AuthActionDownload, IsActive: true,
		ConstraintsJSON: marshalConstraints(t, constraints)}}
	identity := makeIdentity(user, grants)

	result := eval.Evaluate(identity, &ActionContext{Action: constants.AuthActionDownload, TopicName: "public"})
	if !result.Allowed {
		t.Fatalf("download from allowed topic should succeed: %s", result.Reason)
	}

	result = eval.Evaluate(identity, &ActionContext{Action: constants.AuthActionDownload, TopicName: "private"})
	if result.Allowed {
		t.Fatal("download from disallowed topic should be denied")
	}
}

func TestEvaluateDownload_DailyCountLimit(t *testing.T) {
	eval, store := setupEvaluator(t)

	user, _ := store.CreateUser("dl-count", "DL Count", "hash", nil)
	constraints := DownloadConstraints{DailyCountLimit: 2}

	grants := []Grant{{ID: 1, UserID: user.ID, Action: constants.AuthActionDownload, IsActive: true,
		ConstraintsJSON: marshalConstraints(t, constraints)}}
	identity := makeIdentity(user, grants)

	// Use quota up
	store.IncrementQuota(user.ID, constants.AuthActionDownload, 2, 0)

	result := eval.Evaluate(identity, &ActionContext{Action: constants.AuthActionDownload})
	if result.Allowed {
		t.Fatal("expected denial when download count limit exceeded")
	}
}

// ============================================================================
// Query Constraint Tests
// ============================================================================

func TestEvaluateQuery_AllowedPresets(t *testing.T) {
	eval, _ := setupEvaluator(t)

	user := &User{ID: 1, Username: "query-presets", IsActive: true}
	constraints := QueryConstraints{AllowedPresets: []string{"recent-imports", "by-hash"}}

	grants := []Grant{{ID: 1, UserID: 1, Action: constants.AuthActionQuery, IsActive: true,
		ConstraintsJSON: marshalConstraints(t, constraints)}}
	identity := makeIdentity(user, grants)

	result := eval.Evaluate(identity, &ActionContext{
		Action: constants.AuthActionQuery, PresetName: "recent-imports",
	})
	if !result.Allowed {
		t.Fatalf("allowed preset should succeed: %s", result.Reason)
	}

	result = eval.Evaluate(identity, &ActionContext{
		Action: constants.AuthActionQuery, PresetName: "dangerous-query",
	})
	if result.Allowed {
		t.Fatal("disallowed preset should be denied")
	}
}

// ============================================================================
// ManageUsers Constraint Tests
// ============================================================================

func TestEvaluateManageUsers_SubActions(t *testing.T) {
	eval, _ := setupEvaluator(t)

	user := &User{ID: 1, Username: "manager", IsActive: true}
	constraints := ManageUsersConstraints{CanCreate: true, CanEdit: false, CanDisable: false}

	grants := []Grant{{ID: 1, UserID: 1, Action: constants.AuthActionManageUsers, IsActive: true,
		ConstraintsJSON: marshalConstraints(t, constraints)}}
	identity := makeIdentity(user, grants)

	tests := []struct {
		subAction string
		allowed   bool
	}{
		{"create", true},
		{"edit", false},
		{"disable", false},
	}

	for _, tt := range tests {
		t.Run(tt.subAction, func(t *testing.T) {
			result := eval.Evaluate(identity, &ActionContext{
				Action: constants.AuthActionManageUsers, SubAction: tt.subAction,
			})
			if result.Allowed != tt.allowed {
				t.Errorf("expected allowed=%v for %s, got %v (reason: %s)",
					tt.allowed, tt.subAction, result.Allowed, result.Reason)
			}
		})
	}
}

// ============================================================================
// ManageTopics Constraint Tests
// ============================================================================

func TestEvaluateManageTopics_SubActions(t *testing.T) {
	eval, _ := setupEvaluator(t)

	user := &User{ID: 1, Username: "topic-mgr", IsActive: true}
	constraints := ManageTopicsConstraints{CanCreate: true, CanDelete: false}

	grants := []Grant{{ID: 1, UserID: 1, Action: constants.AuthActionManageTopics, IsActive: true,
		ConstraintsJSON: marshalConstraints(t, constraints)}}
	identity := makeIdentity(user, grants)

	result := eval.Evaluate(identity, &ActionContext{
		Action: constants.AuthActionManageTopics, SubAction: "create",
	})
	if !result.Allowed {
		t.Fatalf("topic create should be allowed: %s", result.Reason)
	}

	result = eval.Evaluate(identity, &ActionContext{
		Action: constants.AuthActionManageTopics, SubAction: "delete",
	})
	if result.Allowed {
		t.Fatal("topic delete should be denied")
	}
}

func TestEvaluateManageTopics_AllowedTopics(t *testing.T) {
	eval, _ := setupEvaluator(t)

	user := &User{ID: 1, Username: "topic-limited", IsActive: true}
	constraints := ManageTopicsConstraints{CanCreate: true, CanDelete: true, AllowedTopics: []string{"my-topic"}}

	grants := []Grant{{ID: 1, UserID: 1, Action: constants.AuthActionManageTopics, IsActive: true,
		ConstraintsJSON: marshalConstraints(t, constraints)}}
	identity := makeIdentity(user, grants)

	result := eval.Evaluate(identity, &ActionContext{
		Action: constants.AuthActionManageTopics, SubAction: "create", TopicName: "my-topic",
	})
	if !result.Allowed {
		t.Fatalf("allowed topic should succeed: %s", result.Reason)
	}

	result = eval.Evaluate(identity, &ActionContext{
		Action: constants.AuthActionManageTopics, SubAction: "create", TopicName: "other-topic",
	})
	if result.Allowed {
		t.Fatal("disallowed topic should be denied")
	}
}

// ============================================================================
// BulkDownload Constraint Tests
// ============================================================================

func TestEvaluateBulkDownload_MaxAssetsPerRequest(t *testing.T) {
	eval, _ := setupEvaluator(t)

	user := &User{ID: 1, Username: "bulk-limit", IsActive: true}
	constraints := BulkDownloadConstraints{MaxAssetsPerRequest: 100}

	grants := []Grant{{ID: 1, UserID: 1, Action: constants.AuthActionBulkDownload, IsActive: true,
		ConstraintsJSON: marshalConstraints(t, constraints)}}
	identity := makeIdentity(user, grants)

	result := eval.Evaluate(identity, &ActionContext{
		Action: constants.AuthActionBulkDownload, AssetCount: 50,
	})
	if !result.Allowed {
		t.Fatalf("50 assets should be allowed: %s", result.Reason)
	}

	result = eval.Evaluate(identity, &ActionContext{
		Action: constants.AuthActionBulkDownload, AssetCount: 101,
	})
	if result.Allowed {
		t.Fatal("101 assets should exceed max per request")
	}
}

// ============================================================================
// ViewAudit Constraint Tests
// ============================================================================

func TestEvaluateViewAudit_StreamRestriction(t *testing.T) {
	eval, _ := setupEvaluator(t)

	user := &User{ID: 1, Username: "audit-viewer", IsActive: true}
	constraints := ViewAuditConstraints{CanViewAll: true, CanStream: false}

	grants := []Grant{{ID: 1, UserID: 1, Action: constants.AuthActionViewAudit, IsActive: true,
		ConstraintsJSON: marshalConstraints(t, constraints)}}
	identity := makeIdentity(user, grants)

	// Regular audit view should be allowed
	result := eval.Evaluate(identity, &ActionContext{Action: constants.AuthActionViewAudit})
	if !result.Allowed {
		t.Fatalf("audit view should be allowed: %s", result.Reason)
	}

	// Streaming should be denied
	result = eval.Evaluate(identity, &ActionContext{
		Action: constants.AuthActionViewAudit, SubAction: "stream",
	})
	if result.Allowed {
		t.Fatal("audit streaming should be denied when can_stream=false")
	}
}

// ============================================================================
// Verify Constraint Tests
// ============================================================================

func TestEvaluateVerify_DailyCountLimit(t *testing.T) {
	eval, store := setupEvaluator(t)

	user, _ := store.CreateUser("verify-limit", "Verify Limit", "hash", nil)
	constraints := VerifyConstraints{DailyCountLimit: 1}

	grants := []Grant{{ID: 1, UserID: user.ID, Action: constants.AuthActionVerify, IsActive: true,
		ConstraintsJSON: marshalConstraints(t, constraints)}}
	identity := makeIdentity(user, grants)

	result := eval.Evaluate(identity, &ActionContext{Action: constants.AuthActionVerify})
	if !result.Allowed {
		t.Fatalf("first verify should be allowed: %s", result.Reason)
	}
	store.IncrementQuota(user.ID, constants.AuthActionVerify, 1, 0)

	result = eval.Evaluate(identity, &ActionContext{Action: constants.AuthActionVerify})
	if result.Allowed {
		t.Fatal("second verify should be denied when daily limit is 1")
	}
}

// ============================================================================
// ManageConfig Tests
// ============================================================================

func TestEvaluateManageConfig_GrantSuffices(t *testing.T) {
	eval, _ := setupEvaluator(t)

	user := &User{ID: 1, Username: "config-mgr", IsActive: true}
	grants := []Grant{{ID: 1, UserID: 1, Action: constants.AuthActionManageConfig, IsActive: true}}
	identity := makeIdentity(user, grants)

	result := eval.Evaluate(identity, &ActionContext{Action: constants.AuthActionManageConfig})
	if !result.Allowed {
		t.Fatalf("manage_config grant should suffice: %s", result.Reason)
	}
}

// ============================================================================
// Multiple Grants (First Passing Grant Wins)
// ============================================================================

func TestEvaluate_FirstPassingGrantWins(t *testing.T) {
	eval, _ := setupEvaluator(t)

	user := &User{ID: 1, Username: "multi-grant", IsActive: true}

	// Grant 1: restricted to topic-a only
	restrictedConstraints := UploadConstraints{AllowedTopics: []string{"topic-a"}}
	// Grant 2: unrestricted
	grants := []Grant{
		{ID: 1, UserID: 1, Action: constants.AuthActionUpload, IsActive: true,
			ConstraintsJSON: marshalConstraints(t, restrictedConstraints)},
		{ID: 2, UserID: 1, Action: constants.AuthActionUpload, IsActive: true},
	}
	identity := makeIdentity(user, grants)

	// Upload to topic-b: first grant fails (topic-b not in list), second grant succeeds (unrestricted)
	result := eval.Evaluate(identity, &ActionContext{
		Action: constants.AuthActionUpload, TopicName: "topic-b",
	})
	if !result.Allowed {
		t.Fatalf("should be allowed via second (unrestricted) grant: %s", result.Reason)
	}
	if result.MatchedGrant.ID != 2 {
		t.Errorf("expected matched grant ID=2, got %d", result.MatchedGrant.ID)
	}
}

// ============================================================================
// Malformed Constraints Tests
// ============================================================================

func TestEvaluateUpload_MalformedConstraints(t *testing.T) {
	eval, _ := setupEvaluator(t)

	user := &User{ID: 1, Username: "malformed", IsActive: true}
	badJSON := "not valid json"
	grants := []Grant{{ID: 1, UserID: 1, Action: constants.AuthActionUpload, IsActive: true,
		ConstraintsJSON: &badJSON}}
	identity := makeIdentity(user, grants)

	result := eval.Evaluate(identity, &ActionContext{Action: constants.AuthActionUpload})
	if result.Allowed {
		t.Fatal("malformed constraints should result in denial")
	}
	if result.DeniedCode != constants.ErrCodeAuthConstraintViolation {
		t.Errorf("expected code %q, got %q", constants.ErrCodeAuthConstraintViolation, result.DeniedCode)
	}
}

// ============================================================================
// IncrementQuota Tests
// ============================================================================

func TestIncrementQuota_ViaEvaluator(t *testing.T) {
	eval, store := setupEvaluator(t)

	user, _ := store.CreateUser("inc-quota", "Inc Quota", "hash", nil)

	eval.IncrementQuota(user.ID, constants.AuthActionUpload, 4096)

	usage, _ := store.GetTodayUsage(user.ID, constants.AuthActionUpload)
	if usage.RequestCount != 1 {
		t.Errorf("expected 1 request, got %d", usage.RequestCount)
	}
	if usage.TotalBytes != 4096 {
		t.Errorf("expected 4096 bytes, got %d", usage.TotalBytes)
	}
}

// ============================================================================
// Helper
// ============================================================================

func strPtr(s string) *string {
	return &s
}
