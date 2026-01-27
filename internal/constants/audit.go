package constants

// Audit Log Action Types
const (
	AuditActionConnected             = "connected"
	AuditActionAddingTopic           = "adding_topic"
	AuditActionQuerying              = "querying"
	AuditActionAddingFile            = "adding_file"
	AuditActionVerified              = "verified"
	AuditActionDownloaded            = "downloaded"
	AuditActionDownloadedBulk        = "downloaded_bulk"
	AuditActionReconcileTopicRemoved = "reconcile_topic_removed"
)

// Audit Log Action Types — Authentication
const (
	AuditActionLoginSuccess = "login_success"
	AuditActionLoginFailed  = "login_failed"
	AuditActionLogout       = "logout"
)

// Audit Log Action Types — User Management
const (
	AuditActionUserCreated       = "user_created"
	AuditActionUserUpdated       = "user_updated"
	AuditActionAPIKeyRegenerated = "api_key_regenerated"
)

// Audit Log Action Types — Grant Management
const (
	AuditActionGrantCreated = "grant_created"
	AuditActionGrantUpdated = "grant_updated"
	AuditActionGrantRevoked = "grant_revoked"
)

// Audit Log Action Types — Metadata
const (
	AuditActionMetadataSet   = "metadata_set"
	AuditActionMetadataBatch = "metadata_batch"
	AuditActionMetadataApply = "metadata_apply"
)

// Audit Log Action Types — Configuration
const (
	AuditActionConfigChanged = "config_changed"
)

// Audit Log Action Types — Disk Usage
const (
	AuditActionDiskLimitHit = "disk_limit_hit"
)

// Audit Log Configuration
const (
	AuditLogTableName      = "audit_log"
	AuditDefaultQueryLimit = 100
	AuditMaxQueryLimit     = 1000
	AuditSSEBufferSize     = 100
)

// Audit Log Size Management
const (
	AuditMaxLogSizeBytes     = 10 * 1024 * 1024 * 1024 // 10GB limit
	AuditCleanupIntervalMins = 30                       // Check every 30 mins
	AuditPurgePercentage     = 5                        // Delete 5% oldest when limit hit
	AuditMinPurgeEntries     = 1000                     // Minimum purge batch
)

// Reconciliation
const (
	ReconcileIntervalMins = 5 // Periodic reconciliation check interval
)

// Audit Log Filter Types
const (
	AuditFilterMe     = "me"
	AuditFilterOthers = "others"
	AuditFilterAll    = ""
)
