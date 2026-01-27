package audit

import (
	"meshbank/internal/constants"
)

// Entry represents a single audit log entry
type Entry struct {
	ID        int64       `json:"id"`
	Timestamp int64       `json:"timestamp"`
	Action    string      `json:"action"`
	IPAddress string      `json:"ip_address"`
	Username  string      `json:"username"`
	Details   interface{} `json:"details,omitempty"`
}

// Event follows the existing SSE event pattern for real-time streaming
type Event struct {
	Type      string      `json:"type"`
	Timestamp int64       `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// =============================================================================
// Detail Structs — Core Operations
// =============================================================================

// ConnectedDetails holds details for connected action
type ConnectedDetails struct {
	UserAgent string `json:"user_agent,omitempty"`
}

// AddingTopicDetails holds details for adding_topic action
type AddingTopicDetails struct {
	TopicName string `json:"topic_name"`
}

// QueryingDetails holds details for querying action
type QueryingDetails struct {
	Preset   string   `json:"preset"`
	Topics   []string `json:"topics,omitempty"`
	RowCount int      `json:"row_count"`
}

// AddingFileDetails holds details for adding_file action
type AddingFileDetails struct {
	Hash      string `json:"hash"`
	TopicName string `json:"topic_name"`
	Filename  string `json:"filename"`
	Size      int64  `json:"size"`
	Skipped   bool   `json:"skipped"`
}

// VerifiedDetails holds details for verified action
type VerifiedDetails struct {
	TopicsChecked int  `json:"topics_checked"`
	TopicsValid   int  `json:"topics_valid"`
	IndexValid    bool `json:"index_valid"`
	DurationMs    int  `json:"duration_ms"`
}

// DownloadedDetails holds details for downloaded action
type DownloadedDetails struct {
	Hash     string `json:"hash"`
	Topic    string `json:"topic"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
}

// DownloadedBulkDetails holds details for downloaded_bulk action
type DownloadedBulkDetails struct {
	Mode       string   `json:"mode"`
	AssetCount int      `json:"asset_count"`
	TotalSize  int64    `json:"total_size"`
	Topics     []string `json:"topics,omitempty"`
	Preset     string   `json:"preset,omitempty"`
}

// ReconcileTopicRemovedDetails holds details for reconcile_topic_removed action
type ReconcileTopicRemovedDetails struct {
	TopicName     string `json:"topic_name"`
	EntriesPurged int64  `json:"entries_purged"`
}

// =============================================================================
// Detail Structs — Authentication
// =============================================================================

// LoginSuccessDetails holds details for login_success action
type LoginSuccessDetails struct {
	UserAgent string `json:"user_agent"`
}

// LoginFailedDetails holds details for login_failed action
type LoginFailedDetails struct {
	AttemptedUsername string `json:"attempted_username"`
	Reason           string `json:"reason"`
	UserAgent        string `json:"user_agent"`
}

// LogoutDetails holds details for logout action
type LogoutDetails struct{}

// =============================================================================
// Detail Structs — User Management
// =============================================================================

// UserCreatedDetails holds details for user_created action
type UserCreatedDetails struct {
	CreatedUserID   int64  `json:"created_user_id"`
	CreatedUsername string `json:"created_username"`
}

// UserUpdatedDetails holds details for user_updated action
type UserUpdatedDetails struct {
	TargetUserID   int64    `json:"target_user_id"`
	TargetUsername string   `json:"target_username"`
	FieldsChanged  []string `json:"fields_changed"`
}

// APIKeyRegeneratedDetails holds details for api_key_regenerated action
type APIKeyRegeneratedDetails struct {
	TargetUserID   int64  `json:"target_user_id"`
	TargetUsername string `json:"target_username"`
}

// =============================================================================
// Detail Structs — Grant Management
// =============================================================================

// GrantCreatedDetails holds details for grant_created action
type GrantCreatedDetails struct {
	GrantID        int64  `json:"grant_id"`
	TargetUserID   int64  `json:"target_user_id"`
	Action         string `json:"action"`
	HasConstraints bool   `json:"has_constraints"`
}

// GrantUpdatedDetails holds details for grant_updated action
type GrantUpdatedDetails struct {
	GrantID        int64  `json:"grant_id"`
	TargetUserID   int64  `json:"target_user_id"`
	Action         string `json:"action"`
	HasConstraints bool   `json:"has_constraints"`
}

// GrantRevokedDetails holds details for grant_revoked action
type GrantRevokedDetails struct {
	GrantID      int64  `json:"grant_id"`
	TargetUserID int64  `json:"target_user_id"`
	Action       string `json:"action"`
}

// =============================================================================
// Detail Structs — Metadata Operations
// =============================================================================

// MetadataSetDetails holds details for metadata_set action
type MetadataSetDetails struct {
	Hash string `json:"hash"`
	Op   string `json:"op"`
	Key  string `json:"key"`
}

// MetadataBatchDetails holds details for metadata_batch action
type MetadataBatchDetails struct {
	OperationCount int    `json:"operation_count"`
	Succeeded      int    `json:"succeeded"`
	Failed         int    `json:"failed"`
	Processor      string `json:"processor"`
}

// MetadataApplyDetails holds details for metadata_apply action
type MetadataApplyDetails struct {
	QueryPreset    string `json:"query_preset"`
	Op             string `json:"op"`
	Key            string `json:"key"`
	OperationCount int    `json:"operation_count"`
	Succeeded      int    `json:"succeeded"`
	Failed         int    `json:"failed"`
	Processor      string `json:"processor"`
}

// =============================================================================
// Detail Structs — Configuration
// =============================================================================

// ConfigChangedDetails holds details for config_changed action
type ConfigChangedDetails struct {
	WorkingDirectory string `json:"working_directory"`
	IsBootstrap      bool   `json:"is_bootstrap"`
}

// =============================================================================
// Validation
// =============================================================================

// ValidActions returns all valid audit action types
func ValidActions() []string {
	return []string{
		// Core operations
		constants.AuditActionConnected,
		constants.AuditActionAddingTopic,
		constants.AuditActionQuerying,
		constants.AuditActionAddingFile,
		constants.AuditActionVerified,
		constants.AuditActionDownloaded,
		constants.AuditActionDownloadedBulk,
		constants.AuditActionReconcileTopicRemoved,
		// Authentication
		constants.AuditActionLoginSuccess,
		constants.AuditActionLoginFailed,
		constants.AuditActionLogout,
		// User management
		constants.AuditActionUserCreated,
		constants.AuditActionUserUpdated,
		constants.AuditActionAPIKeyRegenerated,
		// Grant management
		constants.AuditActionGrantCreated,
		constants.AuditActionGrantUpdated,
		constants.AuditActionGrantRevoked,
		// Metadata
		constants.AuditActionMetadataSet,
		constants.AuditActionMetadataBatch,
		constants.AuditActionMetadataApply,
		// Configuration
		constants.AuditActionConfigChanged,
	}
}

// IsValidAction checks if an action type is valid
func IsValidAction(action string) bool {
	for _, valid := range ValidActions() {
		if action == valid {
			return true
		}
	}
	return false
}
