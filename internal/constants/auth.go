package constants

import "time"

// Auth Actions — the granular permission actions
const (
	AuthActionUpload       = "upload"
	AuthActionDownload     = "download"
	AuthActionQuery        = "query"
	AuthActionManageUsers  = "manage_users"
	AuthActionManageTopics = "manage_topics"
	AuthActionMetadata     = "metadata"
	AuthActionBulkDownload = "bulk_download"
	AuthActionViewAudit    = "view_audit"
	AuthActionVerify       = "verify"
	AuthActionManageConfig = "manage_config"
)

// AllAuthActions returns all defined auth actions.
var AllAuthActions = []string{
	AuthActionUpload,
	AuthActionDownload,
	AuthActionQuery,
	AuthActionManageUsers,
	AuthActionManageTopics,
	AuthActionMetadata,
	AuthActionBulkDownload,
	AuthActionViewAudit,
	AuthActionVerify,
	AuthActionManageConfig,
}

// Auth Grant Change Types
const (
	AuthGrantChangeCreated = "created"
	AuthGrantChangeRevoked = "revoked"
	AuthGrantChangeUpdated = "updated"
)

// Auth Error Codes
const (
	ErrCodeAuthRequired          = "AUTH_REQUIRED"
	ErrCodeAuthInvalidCredentials = "AUTH_INVALID_CREDENTIALS"
	ErrCodeAuthForbidden         = "AUTH_FORBIDDEN"
	ErrCodeAuthQuotaExceeded     = "AUTH_QUOTA_EXCEEDED"
	ErrCodeAuthConstraintViolation = "AUTH_CONSTRAINT_VIOLATION"
	ErrCodeAuthUserNotFound      = "AUTH_USER_NOT_FOUND"
	ErrCodeAuthUserExists        = "AUTH_USER_ALREADY_EXISTS"
	ErrCodeAuthUserDisabled      = "AUTH_USER_DISABLED"
	ErrCodeAuthSessionExpired    = "AUTH_SESSION_EXPIRED"
	ErrCodeAuthEscalationDenied  = "AUTH_ESCALATION_DENIED"
	ErrCodeAuthBootstrapProtected = "AUTH_BOOTSTRAP_PROTECTED"
	ErrCodeAuthAccountLocked     = "AUTH_ACCOUNT_LOCKED"
	ErrCodeAuthInvalidGrant      = "AUTH_INVALID_GRANT"
	ErrCodeAuthInvalidAPIKey     = "AUTH_INVALID_API_KEY"
	ErrCodeAuthPasswordTooWeak    = "AUTH_PASSWORD_TOO_WEAK"
	ErrCodeAuthUsernameInvalid    = "AUTH_USERNAME_INVALID"
	ErrCodeAuthInvalidConstraints = "AUTH_INVALID_CONSTRAINTS"
	ErrCodeAuthGrantActionDenied  = "AUTH_GRANT_ACTION_DENIED"
)

// Auth HTTP Headers
const (
	HeaderAuthorization = "Authorization"
	HeaderXAPIKey       = "X-API-Key"
	AuthBearerPrefix    = "Bearer "
)

// Auth Query Parameter (fallback for SSE EventSource and downloads which cannot set headers)
const (
	AuthQueryParamToken = "token"
)

// Auth Token Prefixes (for disambiguation without DB lookup)
const (
	APIKeyPrefix      = "mbk_"
	SessionTokenPrefix = "mbs_"
)

// Auth Configuration
const (
	AuthBcryptCost          = 12
	AuthAPIKeyRandomBytes   = 48  // 384 bits of entropy
	AuthSessionTokenBytes   = 32  // 256 bits of entropy
	AuthAPIKeyPrefixLength  = 8   // visible prefix for identification in logs/UI
	AuthMinPasswordLength   = 12
	AuthMaxPasswordLength   = 128
	AuthMaxLoginAttempts    = 5
	AuthLockoutDurationMins = 15
	AuthBootstrapUsername   = "admin"
	AuthUsernameRegex       = `^[a-z0-9_-]{3,64}$`
	AuthPasswordGenLength   = 24 // chars for auto-generated passwords
)

// Auth Session Configuration
const (
	AuthSessionDuration        = 24 * time.Hour
	AuthSessionMaxDuration     = 7 * 24 * time.Hour
	AuthSessionInactivityTimeout = 24 * time.Hour
	AuthSessionCleanupInterval = 30 * time.Minute
)

// Auth Audit Actions
const (
	AuditActionAuthLogin        = "auth_login"
	AuditActionAuthLoginFailed  = "auth_login_failed"
	AuditActionAuthLogout       = "auth_logout"
	AuditActionAuthUserCreated  = "auth_user_created"
	AuditActionAuthUserUpdated  = "auth_user_updated"
	AuditActionAuthUserDisabled = "auth_user_disabled"
	AuditActionAuthGrantCreated = "auth_grant_created"
	AuditActionAuthGrantRevoked = "auth_grant_revoked"
	AuditActionAuthGrantUpdated = "auth_grant_updated"
	AuditActionAuthDenied       = "auth_denied"
	AuditActionAuthQuotaHit     = "auth_quota_exceeded"
	AuditActionAuthAPIKeyGen    = "auth_apikey_generated"
	AuditActionAuthBootstrap    = "auth_bootstrap"
)

// Quota Date Format (for daily bucketing)
const (
	QuotaDateFormat = "2006-01-02"
)
