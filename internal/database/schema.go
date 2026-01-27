package database

import (
	"database/sql"
	"silobang/internal/constants"
)

// GetTopicSchema returns the full SQL schema for topic databases
func GetTopicSchema() string {
	return `
-- assets table
CREATE TABLE IF NOT EXISTS assets (
    asset_id TEXT PRIMARY KEY,     -- BLAKE3 hash (64 hex chars)
    asset_size INTEGER NOT NULL,   -- bytes
    origin_name TEXT,              -- original filename without extension and dot
    parent_id TEXT,                -- lineage (optional)
    extension TEXT NOT NULL,       -- file extension without dot
    blob_name TEXT NOT NULL,       -- which .dat file (e.g., "003.dat")
    byte_offset INTEGER NOT NULL,  -- offset in .dat file for O(1) lookup
    created_at INTEGER NOT NULL    -- unix timestamp
);

CREATE INDEX IF NOT EXISTS idx_assets_parent ON assets(parent_id);
CREATE INDEX IF NOT EXISTS idx_assets_created ON assets(created_at);
CREATE INDEX IF NOT EXISTS idx_assets_extension ON assets(extension);
CREATE INDEX IF NOT EXISTS idx_assets_origin_name ON assets(origin_name);

-- metadata_log table (append-only)
CREATE TABLE IF NOT EXISTS metadata_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    asset_id TEXT NOT NULL,
    op TEXT NOT NULL,                    -- 'set' | 'delete'
    key TEXT NOT NULL,
    value_text TEXT,                     -- string value
    value_num REAL,                      -- numeric value (if applicable)
    processor TEXT NOT NULL,
    processor_version TEXT NOT NULL,
    timestamp INTEGER NOT NULL,
    FOREIGN KEY (asset_id) REFERENCES assets(asset_id)
);

CREATE INDEX IF NOT EXISTS idx_metadata_asset ON metadata_log(asset_id);
CREATE INDEX IF NOT EXISTS idx_metadata_key ON metadata_log(key);
CREATE INDEX IF NOT EXISTS idx_metadata_processor ON metadata_log(processor);

-- metadata_computed table (materialized view)
CREATE TABLE IF NOT EXISTS metadata_computed (
    asset_id TEXT PRIMARY KEY,
    metadata_json TEXT NOT NULL,   -- JSON object of current key:value pairs
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (asset_id) REFERENCES assets(asset_id)
);

-- dat_hashes table (replaces mapping.json)
-- Uses running hash chain for O(1) append updates
CREATE TABLE IF NOT EXISTS dat_hashes (
    dat_file TEXT PRIMARY KEY,     -- e.g., "001.dat"
    running_hash TEXT NOT NULL,    -- chain hash: BLAKE3(prev || entry_hash || offset || size)
    entry_count INTEGER NOT NULL DEFAULT 0,  -- number of entries in the .dat file
    updated_at INTEGER NOT NULL    -- unix timestamp
);
`
}

// GetOrchestratorSchema returns the full SQL schema for orchestrator.db
func GetOrchestratorSchema() string {
	return `
CREATE TABLE IF NOT EXISTS asset_index (
    hash TEXT PRIMARY KEY,
    topic TEXT NOT NULL,
    dat_file TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_asset_topic ON asset_index(topic);

-- Audit log table (append-only for immutability)
CREATE TABLE IF NOT EXISTS audit_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp INTEGER NOT NULL,
    action TEXT NOT NULL,
    ip_address TEXT NOT NULL,
    username TEXT NOT NULL DEFAULT '',
    details_json TEXT,
    created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now'))
);

CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_log(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_log(action);
CREATE INDEX IF NOT EXISTS idx_audit_ip ON audit_log(ip_address);
CREATE INDEX IF NOT EXISTS idx_audit_ip_timestamp ON audit_log(ip_address, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_audit_username ON audit_log(username);
CREATE INDEX IF NOT EXISTS idx_audit_username_timestamp ON audit_log(username, timestamp DESC);

-- ============================================================================
-- AUTH TABLES
-- ============================================================================

-- Users table (disabled, never hard-deleted for audit trail integrity)
CREATE TABLE IF NOT EXISTS auth_users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL DEFAULT '',
    password_hash TEXT NOT NULL,
    api_key_hash TEXT,
    api_key_prefix TEXT,
    is_active INTEGER NOT NULL DEFAULT 1,
    is_bootstrap INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    created_by INTEGER,
    failed_login_count INTEGER NOT NULL DEFAULT 0,
    locked_until INTEGER,
    FOREIGN KEY (created_by) REFERENCES auth_users(id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_auth_users_username ON auth_users(username);
CREATE INDEX IF NOT EXISTS idx_auth_users_api_key_hash ON auth_users(api_key_hash);
CREATE INDEX IF NOT EXISTS idx_auth_users_active ON auth_users(is_active);

-- Grants table: per-user, per-action permission with JSON constraints
CREATE TABLE IF NOT EXISTS auth_grants (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    action TEXT NOT NULL,
    constraints_json TEXT,
    is_active INTEGER NOT NULL DEFAULT 1,
    created_at INTEGER NOT NULL,
    created_by INTEGER NOT NULL,
    FOREIGN KEY (user_id) REFERENCES auth_users(id),
    FOREIGN KEY (created_by) REFERENCES auth_users(id)
);

CREATE INDEX IF NOT EXISTS idx_auth_grants_user ON auth_grants(user_id);
CREATE INDEX IF NOT EXISTS idx_auth_grants_user_action ON auth_grants(user_id, action);

-- Grant changelog (append-only, immutable audit of all permission changes)
CREATE TABLE IF NOT EXISTS auth_grant_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    grant_id INTEGER,
    user_id INTEGER NOT NULL,
    action TEXT NOT NULL,
    change_type TEXT NOT NULL,
    old_constraints_json TEXT,
    new_constraints_json TEXT,
    changed_by INTEGER NOT NULL,
    timestamp INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_auth_grant_log_user ON auth_grant_log(user_id);
CREATE INDEX IF NOT EXISTS idx_auth_grant_log_timestamp ON auth_grant_log(timestamp DESC);

-- Quota usage tracking (daily counters)
CREATE TABLE IF NOT EXISTS auth_quota_usage (
    user_id INTEGER NOT NULL,
    action TEXT NOT NULL,
    usage_date TEXT NOT NULL,
    request_count INTEGER NOT NULL DEFAULT 0,
    total_bytes INTEGER NOT NULL DEFAULT 0,
    updated_at INTEGER NOT NULL,
    UNIQUE(user_id, action, usage_date)
);

CREATE INDEX IF NOT EXISTS idx_auth_quota_user_action_date ON auth_quota_usage(user_id, action, usage_date);

-- Sessions table (opaque tokens, hashed)
CREATE TABLE IF NOT EXISTS auth_sessions (
    token_hash TEXT PRIMARY KEY,
    token_prefix TEXT NOT NULL,
    user_id INTEGER NOT NULL,
    ip_address TEXT NOT NULL,
    user_agent TEXT,
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL,
    last_active_at INTEGER NOT NULL,
    FOREIGN KEY (user_id) REFERENCES auth_users(id)
);

CREATE INDEX IF NOT EXISTS idx_auth_sessions_user ON auth_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_auth_sessions_expires ON auth_sessions(expires_at);
`
}

// ApplyPragmas applies all SQLite pragmas from constants.SQLitePragmas
// Must be called immediately after opening any database connection
func ApplyPragmas(db *sql.DB) error {
	for _, pragma := range constants.SQLitePragmas {
		if _, err := db.Exec(pragma); err != nil {
			return err
		}
	}
	return nil
}
