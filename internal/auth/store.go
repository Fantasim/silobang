package auth

import (
	"database/sql"
	"fmt"
	"time"

	"meshbank/internal/constants"
)

// Store provides database operations for the auth system.
// All methods operate on the orchestrator database.
type Store struct {
	db *sql.DB
}

// NewStore creates a new auth store backed by the given database.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// ============================================================================
// User Operations
// ============================================================================

// CreateUser inserts a new user into the database.
// Returns the created user with its assigned ID.
func (s *Store) CreateUser(username, displayName, passwordHash string, createdBy *int64) (*User, error) {
	now := time.Now().Unix()
	result, err := s.db.Exec(`
		INSERT INTO auth_users (username, display_name, password_hash, is_active, is_bootstrap, created_at, updated_at, created_by)
		VALUES (?, ?, ?, 1, 0, ?, ?, ?)
	`, username, displayName, passwordHash, now, now, createdBy)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get user id: %w", err)
	}

	return &User{
		ID:          id,
		Username:    username,
		DisplayName: displayName,
		IsActive:    true,
		IsBootstrap: false,
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedBy:   createdBy,
	}, nil
}

// CreateBootstrapUser inserts the initial admin user with is_bootstrap=1.
func (s *Store) CreateBootstrapUser(username, displayName, passwordHash, apiKeyHash, apiKeyPrefix string) (*User, error) {
	now := time.Now().Unix()
	result, err := s.db.Exec(`
		INSERT INTO auth_users (username, display_name, password_hash, api_key_hash, api_key_prefix,
		                        is_active, is_bootstrap, created_at, updated_at, created_by)
		VALUES (?, ?, ?, ?, ?, 1, 1, ?, ?, NULL)
	`, username, displayName, passwordHash, apiKeyHash, apiKeyPrefix, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to create bootstrap user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get bootstrap user id: %w", err)
	}

	return &User{
		ID:          id,
		Username:    username,
		DisplayName: displayName,
		IsActive:    true,
		IsBootstrap: true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// GetUserByID retrieves a user by ID.
func (s *Store) GetUserByID(id int64) (*UserWithSensitive, error) {
	return s.scanUser(s.db.QueryRow(`
		SELECT id, username, display_name, password_hash, api_key_hash, api_key_prefix,
		       is_active, is_bootstrap, created_at, updated_at, created_by,
		       failed_login_count, locked_until
		FROM auth_users WHERE id = ?
	`, id))
}

// GetUserByUsername retrieves a user by username.
func (s *Store) GetUserByUsername(username string) (*UserWithSensitive, error) {
	return s.scanUser(s.db.QueryRow(`
		SELECT id, username, display_name, password_hash, api_key_hash, api_key_prefix,
		       is_active, is_bootstrap, created_at, updated_at, created_by,
		       failed_login_count, locked_until
		FROM auth_users WHERE username = ?
	`, username))
}

// GetUserByAPIKeyHash retrieves a user by hashed API key.
func (s *Store) GetUserByAPIKeyHash(keyHash string) (*UserWithSensitive, error) {
	return s.scanUser(s.db.QueryRow(`
		SELECT id, username, display_name, password_hash, api_key_hash, api_key_prefix,
		       is_active, is_bootstrap, created_at, updated_at, created_by,
		       failed_login_count, locked_until
		FROM auth_users WHERE api_key_hash = ?
	`, keyHash))
}

// ListUsers returns all users (without sensitive fields).
func (s *Store) ListUsers() ([]User, error) {
	rows, err := s.db.Query(`
		SELECT id, username, display_name, is_active, is_bootstrap, created_at, updated_at, created_by
		FROM auth_users ORDER BY id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.IsActive,
			&u.IsBootstrap, &u.CreatedAt, &u.UpdatedAt, &u.CreatedBy); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// UpdateUser updates a user's display name and active status.
func (s *Store) UpdateUser(id int64, displayName string, isActive bool) error {
	now := time.Now().Unix()
	_, err := s.db.Exec(`
		UPDATE auth_users SET display_name = ?, is_active = ?, updated_at = ?
		WHERE id = ?
	`, displayName, isActive, now, id)
	return err
}

// UpdateUserPassword updates a user's password hash.
func (s *Store) UpdateUserPassword(id int64, passwordHash string) error {
	now := time.Now().Unix()
	_, err := s.db.Exec(`
		UPDATE auth_users SET password_hash = ?, updated_at = ? WHERE id = ?
	`, passwordHash, now, id)
	return err
}

// UpdateUserAPIKey updates a user's API key hash and prefix.
func (s *Store) UpdateUserAPIKey(id int64, apiKeyHash, apiKeyPrefix string) error {
	now := time.Now().Unix()
	_, err := s.db.Exec(`
		UPDATE auth_users SET api_key_hash = ?, api_key_prefix = ?, updated_at = ?
		WHERE id = ?
	`, apiKeyHash, apiKeyPrefix, now, id)
	return err
}

// IncrementFailedLogin increments the failed login counter. Locks the account if threshold reached.
func (s *Store) IncrementFailedLogin(id int64) error {
	now := time.Now().Unix()
	lockUntil := now + int64(constants.AuthLockoutDurationMins*60)

	_, err := s.db.Exec(`
		UPDATE auth_users SET
			failed_login_count = failed_login_count + 1,
			locked_until = CASE
				WHEN failed_login_count + 1 >= ? THEN ?
				ELSE locked_until
			END,
			updated_at = ?
		WHERE id = ?
	`, constants.AuthMaxLoginAttempts, lockUntil, now, id)
	return err
}

// ResetFailedLogin resets the failed login counter and removes lockout.
func (s *Store) ResetFailedLogin(id int64) error {
	now := time.Now().Unix()
	_, err := s.db.Exec(`
		UPDATE auth_users SET failed_login_count = 0, locked_until = NULL, updated_at = ?
		WHERE id = ?
	`, now, id)
	return err
}

// CountUsers returns the total number of users.
func (s *Store) CountUsers() (int64, error) {
	var count int64
	err := s.db.QueryRow("SELECT COUNT(*) FROM auth_users").Scan(&count)
	return count, err
}

// scanUser scans a single user row including sensitive fields.
func (s *Store) scanUser(row *sql.Row) (*UserWithSensitive, error) {
	var u UserWithSensitive
	var apiKeyHash, apiKeyPrefix sql.NullString
	var createdBy sql.NullInt64
	var lockedUntil sql.NullInt64

	err := row.Scan(
		&u.ID, &u.Username, &u.DisplayName, &u.PasswordHash,
		&apiKeyHash, &apiKeyPrefix,
		&u.IsActive, &u.IsBootstrap, &u.CreatedAt, &u.UpdatedAt, &createdBy,
		&u.FailedLoginCount, &lockedUntil,
	)
	if err != nil {
		return nil, err
	}

	if apiKeyHash.Valid {
		u.APIKeyHash = apiKeyHash.String
	}
	if apiKeyPrefix.Valid {
		u.APIKeyPrefix = apiKeyPrefix.String
	}
	if createdBy.Valid {
		u.CreatedBy = &createdBy.Int64
	}
	if lockedUntil.Valid {
		u.LockedUntil = &lockedUntil.Int64
	}

	return &u, nil
}

// ============================================================================
// Grant Operations
// ============================================================================

// CreateGrant inserts a new permission grant for a user.
func (s *Store) CreateGrant(userID int64, action string, constraintsJSON *string, createdBy int64) (*Grant, error) {
	now := time.Now().Unix()
	result, err := s.db.Exec(`
		INSERT INTO auth_grants (user_id, action, constraints_json, is_active, created_at, created_by)
		VALUES (?, ?, ?, 1, ?, ?)
	`, userID, action, constraintsJSON, now, createdBy)
	if err != nil {
		return nil, fmt.Errorf("failed to create grant: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get grant id: %w", err)
	}

	grant := &Grant{
		ID:              id,
		UserID:          userID,
		Action:          action,
		ConstraintsJSON: constraintsJSON,
		IsActive:        true,
		CreatedAt:       now,
		CreatedBy:       createdBy,
	}

	// Log the grant creation
	s.logGrantChange(id, userID, action, constants.AuthGrantChangeCreated, nil, constraintsJSON, createdBy)

	return grant, nil
}

// GetGrantByID retrieves a grant by ID.
func (s *Store) GetGrantByID(id int64) (*Grant, error) {
	var g Grant
	err := s.db.QueryRow(`
		SELECT id, user_id, action, constraints_json, is_active, created_at, created_by
		FROM auth_grants WHERE id = ?
	`, id).Scan(&g.ID, &g.UserID, &g.Action, &g.ConstraintsJSON, &g.IsActive, &g.CreatedAt, &g.CreatedBy)
	if err != nil {
		return nil, err
	}
	return &g, nil
}

// GetActiveGrantsForUser returns all active grants for a user.
func (s *Store) GetActiveGrantsForUser(userID int64) ([]Grant, error) {
	rows, err := s.db.Query(`
		SELECT id, user_id, action, constraints_json, is_active, created_at, created_by
		FROM auth_grants WHERE user_id = ? AND is_active = 1
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to load grants: %w", err)
	}
	defer rows.Close()

	var grants []Grant
	for rows.Next() {
		var g Grant
		if err := rows.Scan(&g.ID, &g.UserID, &g.Action, &g.ConstraintsJSON,
			&g.IsActive, &g.CreatedAt, &g.CreatedBy); err != nil {
			return nil, fmt.Errorf("failed to scan grant: %w", err)
		}
		grants = append(grants, g)
	}
	return grants, rows.Err()
}

// GetAllGrantsForUser returns all grants (including inactive) for a user.
func (s *Store) GetAllGrantsForUser(userID int64) ([]Grant, error) {
	rows, err := s.db.Query(`
		SELECT id, user_id, action, constraints_json, is_active, created_at, created_by
		FROM auth_grants WHERE user_id = ? ORDER BY id ASC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to load grants: %w", err)
	}
	defer rows.Close()

	var grants []Grant
	for rows.Next() {
		var g Grant
		if err := rows.Scan(&g.ID, &g.UserID, &g.Action, &g.ConstraintsJSON,
			&g.IsActive, &g.CreatedAt, &g.CreatedBy); err != nil {
			return nil, fmt.Errorf("failed to scan grant: %w", err)
		}
		grants = append(grants, g)
	}
	return grants, rows.Err()
}

// UpdateGrantConstraints updates the constraints JSON for a grant.
func (s *Store) UpdateGrantConstraints(grantID int64, newConstraintsJSON *string, changedBy int64) error {
	// Read old constraints first for the changelog
	old, err := s.GetGrantByID(grantID)
	if err != nil {
		return fmt.Errorf("grant not found: %w", err)
	}

	_, err = s.db.Exec(`
		UPDATE auth_grants SET constraints_json = ? WHERE id = ? AND is_active = 1
	`, newConstraintsJSON, grantID)
	if err != nil {
		return fmt.Errorf("failed to update grant: %w", err)
	}

	s.logGrantChange(grantID, old.UserID, old.Action, constants.AuthGrantChangeUpdated,
		old.ConstraintsJSON, newConstraintsJSON, changedBy)

	return nil
}

// RevokeGrant soft-deletes a grant by setting is_active=0.
func (s *Store) RevokeGrant(grantID int64, changedBy int64) error {
	old, err := s.GetGrantByID(grantID)
	if err != nil {
		return fmt.Errorf("grant not found: %w", err)
	}

	_, err = s.db.Exec(`UPDATE auth_grants SET is_active = 0 WHERE id = ?`, grantID)
	if err != nil {
		return fmt.Errorf("failed to revoke grant: %w", err)
	}

	s.logGrantChange(grantID, old.UserID, old.Action, constants.AuthGrantChangeRevoked,
		old.ConstraintsJSON, nil, changedBy)

	return nil
}

// logGrantChange inserts an entry into the append-only grant changelog.
func (s *Store) logGrantChange(grantID int64, userID int64, action, changeType string,
	oldConstraints, newConstraints *string, changedBy int64) {

	now := time.Now().Unix()
	s.db.Exec(`
		INSERT INTO auth_grant_log (grant_id, user_id, action, change_type,
		                            old_constraints_json, new_constraints_json, changed_by, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, grantID, userID, action, changeType, oldConstraints, newConstraints, changedBy, now)
}

// GetGrantLog returns the grant changelog for a user.
func (s *Store) GetGrantLog(userID int64, limit int) ([]GrantLogEntry, error) {
	if limit <= 0 {
		limit = constants.DefaultPageSize
	}
	rows, err := s.db.Query(`
		SELECT id, grant_id, user_id, action, change_type,
		       old_constraints_json, new_constraints_json, changed_by, timestamp
		FROM auth_grant_log WHERE user_id = ?
		ORDER BY timestamp DESC, id DESC LIMIT ?
	`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query grant log: %w", err)
	}
	defer rows.Close()

	var entries []GrantLogEntry
	for rows.Next() {
		var e GrantLogEntry
		if err := rows.Scan(&e.ID, &e.GrantID, &e.UserID, &e.Action, &e.ChangeType,
			&e.OldConstraintsJSON, &e.NewConstraintsJSON, &e.ChangedBy, &e.Timestamp); err != nil {
			return nil, fmt.Errorf("failed to scan grant log entry: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// ============================================================================
// Quota Usage Operations
// ============================================================================

// GetTodayUsage returns the quota usage for a user+action for today.
// Returns zero-value usage if no record exists (no usage yet today).
func (s *Store) GetTodayUsage(userID int64, action string) (*QuotaUsage, error) {
	today := time.Now().UTC().Format(constants.QuotaDateFormat)

	var usage QuotaUsage
	err := s.db.QueryRow(`
		SELECT user_id, action, usage_date, request_count, total_bytes, updated_at
		FROM auth_quota_usage
		WHERE user_id = ? AND action = ? AND usage_date = ?
	`, userID, action, today).Scan(
		&usage.UserID, &usage.Action, &usage.UsageDate,
		&usage.RequestCount, &usage.TotalBytes, &usage.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return &QuotaUsage{
			UserID:    userID,
			Action:    action,
			UsageDate: today,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get quota usage: %w", err)
	}
	return &usage, nil
}

// IncrementQuota atomically increments the daily quota counters.
// Uses INSERT OR UPDATE (upsert) for atomic operation.
func (s *Store) IncrementQuota(userID int64, action string, countDelta int64, bytesDelta int64) error {
	today := time.Now().UTC().Format(constants.QuotaDateFormat)
	now := time.Now().Unix()

	_, err := s.db.Exec(`
		INSERT INTO auth_quota_usage (user_id, action, usage_date, request_count, total_bytes, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, action, usage_date) DO UPDATE SET
			request_count = request_count + ?,
			total_bytes = total_bytes + ?,
			updated_at = ?
	`, userID, action, today, countDelta, bytesDelta, now,
		countDelta, bytesDelta, now)
	if err != nil {
		return fmt.Errorf("failed to increment quota: %w", err)
	}
	return nil
}

// GetAllQuotaUsage returns all quota usage records for a user for today.
func (s *Store) GetAllQuotaUsage(userID int64) ([]QuotaUsage, error) {
	today := time.Now().UTC().Format(constants.QuotaDateFormat)

	rows, err := s.db.Query(`
		SELECT user_id, action, usage_date, request_count, total_bytes, updated_at
		FROM auth_quota_usage
		WHERE user_id = ? AND usage_date = ?
		ORDER BY action ASC
	`, userID, today)
	if err != nil {
		return nil, fmt.Errorf("failed to query quota usage: %w", err)
	}
	defer rows.Close()

	var usages []QuotaUsage
	for rows.Next() {
		var u QuotaUsage
		if err := rows.Scan(&u.UserID, &u.Action, &u.UsageDate,
			&u.RequestCount, &u.TotalBytes, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan quota usage: %w", err)
		}
		usages = append(usages, u)
	}
	return usages, rows.Err()
}

// ============================================================================
// Session Operations
// ============================================================================

// CreateSession inserts a new session into the database.
func (s *Store) CreateSession(tokenHash, tokenPrefix string, userID int64, ipAddress, userAgent string) (*Session, error) {
	now := time.Now().Unix()
	expiresAt := now + int64(constants.AuthSessionDuration.Seconds())

	_, err := s.db.Exec(`
		INSERT INTO auth_sessions (token_hash, token_prefix, user_id, ip_address, user_agent,
		                           created_at, expires_at, last_active_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, tokenHash, tokenPrefix, userID, ipAddress, userAgent, now, expiresAt, now)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return &Session{
		TokenHash:    tokenHash,
		TokenPrefix:  tokenPrefix,
		UserID:       userID,
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
		CreatedAt:    now,
		ExpiresAt:    expiresAt,
		LastActiveAt: now,
	}, nil
}

// GetSessionByTokenHash retrieves a session by its hashed token.
// Returns nil if the session doesn't exist, is expired, or the user is inactive.
func (s *Store) GetSessionByTokenHash(tokenHash string) (*Session, *User, error) {
	now := time.Now().Unix()

	var session Session
	var user User
	err := s.db.QueryRow(`
		SELECT s.token_hash, s.token_prefix, s.user_id, s.ip_address, s.user_agent,
		       s.created_at, s.expires_at, s.last_active_at,
		       u.id, u.username, u.display_name, u.is_active, u.is_bootstrap, u.created_at, u.updated_at
		FROM auth_sessions s
		JOIN auth_users u ON s.user_id = u.id
		WHERE s.token_hash = ? AND s.expires_at > ? AND u.is_active = 1
	`, tokenHash, now).Scan(
		&session.TokenHash, &session.TokenPrefix, &session.UserID,
		&session.IPAddress, &session.UserAgent,
		&session.CreatedAt, &session.ExpiresAt, &session.LastActiveAt,
		&user.ID, &user.Username, &user.DisplayName, &user.IsActive, &user.IsBootstrap,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Check inactivity timeout
	inactivityDeadline := session.LastActiveAt + int64(constants.AuthSessionInactivityTimeout.Seconds())
	if now > inactivityDeadline {
		return nil, nil, nil
	}

	return &session, &user, nil
}

// TouchSession updates the last_active_at timestamp for a session.
func (s *Store) TouchSession(tokenHash string) error {
	now := time.Now().Unix()
	_, err := s.db.Exec(`
		UPDATE auth_sessions SET last_active_at = ? WHERE token_hash = ?
	`, now, tokenHash)
	return err
}

// DeleteSession removes a session by its hashed token.
func (s *Store) DeleteSession(tokenHash string) error {
	_, err := s.db.Exec(`DELETE FROM auth_sessions WHERE token_hash = ?`, tokenHash)
	return err
}

// DeleteUserSessions removes all sessions for a user (e.g., on password change or disable).
func (s *Store) DeleteUserSessions(userID int64) error {
	_, err := s.db.Exec(`DELETE FROM auth_sessions WHERE user_id = ?`, userID)
	return err
}

// CleanupExpiredSessions removes all expired sessions from the database.
// Returns the number of sessions removed.
func (s *Store) CleanupExpiredSessions() (int64, error) {
	now := time.Now().Unix()
	result, err := s.db.Exec(`DELETE FROM auth_sessions WHERE expires_at <= ?`, now)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup sessions: %w", err)
	}
	return result.RowsAffected()
}
