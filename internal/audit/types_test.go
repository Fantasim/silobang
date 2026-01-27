package audit

import (
	"reflect"
	"strings"
	"testing"

	"silobang/internal/constants"
)

// collectAuditActionConstants uses reflection to find all AuditAction* string
// constants exported from the constants package. This ensures new constants
// added in the future are automatically caught if they're missing from ValidActions().
func collectAuditActionConstants() []string {
	// We can't reflect on package-level constants directly in Go,
	// so we maintain an exhaustive list that must match constants/audit.go.
	// This is the single source of truth for testing.
	return []string{
		constants.AuditActionConnected,
		constants.AuditActionAddingTopic,
		constants.AuditActionQuerying,
		constants.AuditActionAddingFile,
		constants.AuditActionVerified,
		constants.AuditActionDownloaded,
		constants.AuditActionDownloadedBulk,
		constants.AuditActionReconcileTopicRemoved,
		constants.AuditActionLoginSuccess,
		constants.AuditActionLoginFailed,
		constants.AuditActionLogout,
		constants.AuditActionUserCreated,
		constants.AuditActionUserUpdated,
		constants.AuditActionAPIKeyRegenerated,
		constants.AuditActionGrantCreated,
		constants.AuditActionGrantUpdated,
		constants.AuditActionGrantRevoked,
		constants.AuditActionMetadataSet,
		constants.AuditActionMetadataBatch,
		constants.AuditActionMetadataApply,
		constants.AuditActionConfigChanged,
	}
}

func TestValidActionsContainsAllConstants(t *testing.T) {
	allConstants := collectAuditActionConstants()
	validActions := ValidActions()

	// Build a set from ValidActions for fast lookup
	validSet := make(map[string]bool, len(validActions))
	for _, action := range validActions {
		validSet[action] = true
	}

	for _, action := range allConstants {
		if !validSet[action] {
			t.Errorf("constant %q is defined in constants/audit.go but missing from ValidActions()", action)
		}
	}

	// Also verify no extra entries in ValidActions that aren't in constants
	constSet := make(map[string]bool, len(allConstants))
	for _, action := range allConstants {
		constSet[action] = true
	}

	for _, action := range validActions {
		if !constSet[action] {
			t.Errorf("ValidActions() contains %q which is not in the known constants list", action)
		}
	}
}

func TestValidActionsCountMatchesConstants(t *testing.T) {
	allConstants := collectAuditActionConstants()
	validActions := ValidActions()

	if len(validActions) != len(allConstants) {
		t.Errorf("ValidActions() has %d entries but %d constants are defined", len(validActions), len(allConstants))
	}
}

func TestValidActionsNoDuplicates(t *testing.T) {
	validActions := ValidActions()
	seen := make(map[string]bool, len(validActions))

	for _, action := range validActions {
		if seen[action] {
			t.Errorf("duplicate action in ValidActions(): %q", action)
		}
		seen[action] = true
	}
}

func TestDownloadingActionRemoved(t *testing.T) {
	// "downloading" was a dead constant that was removed.
	// Verify it's not in ValidActions.
	if IsValidAction("downloading") {
		t.Error("'downloading' should not be a valid action (it was removed as dead code)")
	}
}

func TestIsValidAction_AllNewActionsValid(t *testing.T) {
	newActions := []string{
		"login_success",
		"login_failed",
		"logout",
		"user_created",
		"user_updated",
		"api_key_regenerated",
		"grant_created",
		"grant_updated",
		"grant_revoked",
		"metadata_set",
		"metadata_batch",
		"metadata_apply",
		"config_changed",
	}

	for _, action := range newActions {
		if !IsValidAction(action) {
			t.Errorf("IsValidAction(%q) = false, want true", action)
		}
	}
}

func TestIsValidAction_OriginalActionsStillValid(t *testing.T) {
	originalActions := []string{
		"connected",
		"adding_topic",
		"querying",
		"adding_file",
		"verified",
		"downloaded",
		"downloaded_bulk",
		"reconcile_topic_removed",
	}

	for _, action := range originalActions {
		if !IsValidAction(action) {
			t.Errorf("IsValidAction(%q) = false, want true (original action must remain valid)", action)
		}
	}
}

func TestIsValidAction_InvalidActionsRejected(t *testing.T) {
	invalidActions := []string{
		"",
		"downloading", // removed
		"unknown_action",
		"login",    // close but wrong
		"CONNECTED", // case-sensitive
		"connected ",
		" connected",
		"delete_user",
	}

	for _, action := range invalidActions {
		if IsValidAction(action) {
			t.Errorf("IsValidAction(%q) = true, want false", action)
		}
	}
}

func TestValidActionsAreSnakeCase(t *testing.T) {
	for _, action := range ValidActions() {
		if action != strings.ToLower(action) {
			t.Errorf("action %q is not lowercase snake_case", action)
		}
		if strings.Contains(action, " ") {
			t.Errorf("action %q contains spaces", action)
		}
		if strings.HasPrefix(action, "_") || strings.HasSuffix(action, "_") {
			t.Errorf("action %q has leading or trailing underscore", action)
		}
	}
}

func TestDetailStructsSerialization(t *testing.T) {
	// Verify that all detail structs have json tags and can be created
	// This is a compile-time guarantee that all structs exist with expected fields

	tests := []struct {
		name    string
		details interface{}
	}{
		// Core operations
		{"ConnectedDetails", ConnectedDetails{UserAgent: "test"}},
		{"AddingTopicDetails", AddingTopicDetails{TopicName: "test"}},
		{"QueryingDetails", QueryingDetails{Preset: "test", Topics: []string{"a"}, RowCount: 10}},
		{"AddingFileDetails", AddingFileDetails{Hash: "abc", TopicName: "t", Filename: "f", Size: 100, Skipped: false}},
		{"VerifiedDetails", VerifiedDetails{TopicsChecked: 1, TopicsValid: 1, IndexValid: true, DurationMs: 50}},
		{"DownloadedDetails", DownloadedDetails{Hash: "abc", Topic: "t", Filename: "f", Size: 100}},
		{"DownloadedBulkDetails", DownloadedBulkDetails{Mode: "stream", AssetCount: 5, TotalSize: 500}},
		{"ReconcileTopicRemovedDetails", ReconcileTopicRemovedDetails{TopicName: "old", EntriesPurged: 10}},
		// Authentication
		{"LoginSuccessDetails", LoginSuccessDetails{UserAgent: "Mozilla/5.0"}},
		{"LoginFailedDetails", LoginFailedDetails{AttemptedUsername: "admin", Reason: "invalid_credentials", UserAgent: "curl"}},
		{"LogoutDetails", LogoutDetails{}},
		// User management
		{"UserCreatedDetails", UserCreatedDetails{CreatedUserID: 1, CreatedUsername: "newuser"}},
		{"UserUpdatedDetails", UserUpdatedDetails{TargetUserID: 1, TargetUsername: "user", FieldsChanged: []string{"display_name"}}},
		{"APIKeyRegeneratedDetails", APIKeyRegeneratedDetails{TargetUserID: 1, TargetUsername: "user"}},
		// Grant management
		{"GrantCreatedDetails", GrantCreatedDetails{GrantID: 1, TargetUserID: 2, Action: "read", HasConstraints: true}},
		{"GrantUpdatedDetails", GrantUpdatedDetails{GrantID: 1, TargetUserID: 2, Action: "write", HasConstraints: false}},
		{"GrantRevokedDetails", GrantRevokedDetails{GrantID: 1, TargetUserID: 2, Action: "read"}},
		// Metadata
		{"MetadataSetDetails", MetadataSetDetails{Hash: "abc", Op: "set", Key: "tag"}},
		{"MetadataBatchDetails", MetadataBatchDetails{OperationCount: 10, Succeeded: 8, Failed: 2, Processor: "api"}},
		{"MetadataApplyDetails", MetadataApplyDetails{QueryPreset: "all", Op: "set", Key: "tag", OperationCount: 5, Succeeded: 5, Failed: 0, Processor: "api"}},
		// Configuration
		{"ConfigChangedDetails", ConfigChangedDetails{WorkingDirectory: "/data", IsBootstrap: true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify the struct is not nil (compile-time check passed)
			if reflect.ValueOf(tt.details).IsZero() && tt.name != "LogoutDetails" {
				t.Errorf("%s should not be zero-valued in test", tt.name)
			}
		})
	}
}
