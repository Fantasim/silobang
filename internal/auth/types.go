// Package auth provides authentication, authorization, and quota enforcement
// for the meshbank system. It implements a grant-based permission model where
// each user has per-action grants with optional JSON constraints and daily quotas.
package auth

// User represents an authenticated user in the system.
// Sensitive fields (password hash, API key hash) are excluded from JSON serialization.
type User struct {
	ID               int64  `json:"id"`
	Username         string `json:"username"`
	DisplayName      string `json:"display_name"`
	IsActive         bool   `json:"is_active"`
	IsBootstrap      bool   `json:"is_bootstrap"`
	CreatedAt        int64  `json:"created_at"`
	UpdatedAt        int64  `json:"updated_at"`
	CreatedBy        *int64 `json:"created_by,omitempty"`
	FailedLoginCount int    `json:"-"`
	LockedUntil      *int64 `json:"-"`
}

// UserWithSensitive includes password hash and API key fields for internal use.
// These fields must never be serialized to JSON or returned in API responses.
type UserWithSensitive struct {
	User
	PasswordHash string `json:"-"`
	APIKeyHash   string `json:"-"`
	APIKeyPrefix string `json:"api_key_prefix,omitempty"`
}

// Grant represents a single permission grant for a user.
// Each grant authorizes one action with optional JSON constraints.
type Grant struct {
	ID              int64   `json:"id"`
	UserID          int64   `json:"user_id"`
	Action          string  `json:"action"`
	ConstraintsJSON *string `json:"constraints_json,omitempty"`
	IsActive        bool    `json:"is_active"`
	CreatedAt       int64   `json:"created_at"`
	CreatedBy       int64   `json:"created_by"`
}

// GrantLogEntry represents an immutable record of a permission change.
// These entries form an append-only audit trail of all grant modifications.
type GrantLogEntry struct {
	ID                 int64   `json:"id"`
	GrantID            *int64  `json:"grant_id,omitempty"`
	UserID             int64   `json:"user_id"`
	Action             string  `json:"action"`
	ChangeType         string  `json:"change_type"` // "created", "revoked", "updated"
	OldConstraintsJSON *string `json:"old_constraints_json,omitempty"`
	NewConstraintsJSON *string `json:"new_constraints_json,omitempty"`
	ChangedBy          int64   `json:"changed_by"`
	Timestamp          int64   `json:"timestamp"`
}

// QuotaUsage tracks daily usage for a specific user+action combination.
type QuotaUsage struct {
	UserID       int64  `json:"user_id"`
	Action       string `json:"action"`
	UsageDate    string `json:"usage_date"` // "2026-01-26"
	RequestCount int64  `json:"request_count"`
	TotalBytes   int64  `json:"total_bytes"`
	UpdatedAt    int64  `json:"updated_at"`
}

// Session represents an active login session (opaque token stored hashed).
type Session struct {
	TokenHash    string `json:"-"`
	TokenPrefix  string `json:"token_prefix"`
	UserID       int64  `json:"user_id"`
	IPAddress    string `json:"ip_address"`
	UserAgent    string `json:"user_agent,omitempty"`
	CreatedAt    int64  `json:"created_at"`
	ExpiresAt    int64  `json:"expires_at"`
	LastActiveAt int64  `json:"last_active_at"`
}

// Identity represents the resolved identity of an authenticated request.
// It is attached to the request context by the auth middleware.
type Identity struct {
	User   *User   `json:"user"`
	Method string  `json:"method"` // "session", "api_key"
	Grants []Grant `json:"grants"`
}

// ActionContext carries the context for a policy evaluation.
// Fields are populated based on the specific action being evaluated.
type ActionContext struct {
	Action     string // required: which auth action
	FileSize   int64  // for upload: file size in bytes
	Extension  string // for upload: file extension without dot
	TopicName  string // for upload/download/topic actions
	PresetName string // for query: preset name
	AssetCount int    // for bulk_download: number of assets
	VolumeBytes int64 // for download: estimated volume
	SubAction  string // for manage_users: "create", "edit", "disable"
}

// PolicyResult represents the outcome of a policy evaluation.
type PolicyResult struct {
	Allowed      bool   `json:"allowed"`
	Reason       string `json:"reason,omitempty"`
	DeniedCode   string `json:"denied_code,omitempty"`
	MatchedGrant *Grant `json:"matched_grant,omitempty"`
}
