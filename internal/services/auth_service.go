package services

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"meshbank/internal/auth"
	"meshbank/internal/constants"
	"meshbank/internal/logger"
)

var usernameRegex = regexp.MustCompile(constants.AuthUsernameRegex)

// AuthService manages user CRUD, grants, sessions, and authentication.
type AuthService struct {
	app       AppState
	logger    *logger.Logger
	store     *auth.Store
	evaluator *auth.PolicyEvaluator
	stopClean chan struct{} // For session cleanup goroutine shutdown
}

// NewAuthService creates a new auth service.
// Returns nil if the orchestrator DB is not available.
func NewAuthService(app AppState, log *logger.Logger) *AuthService {
	db := app.GetOrchestratorDB()
	if db == nil {
		return nil
	}

	store := auth.NewStore(db)
	evaluator := auth.NewPolicyEvaluator(store, log)

	svc := &AuthService{
		app:       app,
		logger:    log,
		store:     store,
		evaluator: evaluator,
		stopClean: make(chan struct{}),
	}

	// Start session cleanup goroutine
	go svc.sessionCleanupLoop()

	return svc
}

// GetStore returns the underlying auth store (for middleware initialization).
func (s *AuthService) GetStore() *auth.Store {
	return s.store
}

// GetEvaluator returns the policy evaluator (for handler authorization checks).
func (s *AuthService) GetEvaluator() *auth.PolicyEvaluator {
	return s.evaluator
}

// ============================================================================
// Authentication
// ============================================================================

// Login validates credentials and creates a session.
// Returns the plaintext session token and user info.
func (s *AuthService) Login(username, password, ipAddress, userAgent string) (string, *auth.User, error) {
	s.logger.Info("Auth: login attempt for user=%s from ip=%s", username, ipAddress)

	user, err := s.store.GetUserByUsername(username)
	if err != nil {
		// Generic error to prevent user enumeration
		s.logger.Debug("Auth: user not found: %s", username)
		return "", nil, NewServiceError(constants.ErrCodeAuthInvalidCredentials, "invalid credentials")
	}

	if !user.IsActive {
		s.logger.Info("Auth: login denied for disabled user=%s", username)
		return "", nil, NewServiceError(constants.ErrCodeAuthUserDisabled, "account is disabled")
	}

	// Check lockout
	if user.LockedUntil != nil {
		now := time.Now().Unix()
		if now < *user.LockedUntil {
			s.logger.Info("Auth: login denied for locked user=%s (locked until %d)", username, *user.LockedUntil)
			return "", nil, NewServiceError(constants.ErrCodeAuthAccountLocked, "account is temporarily locked")
		}
		// Lockout expired, reset counter
		s.store.ResetFailedLogin(user.ID)
	}

	// Verify password
	if err := auth.VerifyPassword(password, user.PasswordHash); err != nil {
		s.store.IncrementFailedLogin(user.ID)
		s.logger.Info("Auth: invalid password for user=%s (attempt %d)", username, user.FailedLoginCount+1)
		return "", nil, NewServiceError(constants.ErrCodeAuthInvalidCredentials, "invalid credentials")
	}

	// Reset failed login count on success
	if user.FailedLoginCount > 0 {
		s.store.ResetFailedLogin(user.ID)
	}

	// Create session
	token, err := auth.GenerateSessionToken()
	if err != nil {
		return "", nil, WrapInternalError(err)
	}

	tokenHash := auth.HashToken(token)
	tokenPrefix := auth.ExtractTokenPrefix(token)

	_, err = s.store.CreateSession(tokenHash, tokenPrefix, user.ID, ipAddress, userAgent)
	if err != nil {
		return "", nil, WrapInternalError(err)
	}

	s.logger.Info("Auth: user=%s logged in from ip=%s", username, ipAddress)

	return token, &user.User, nil
}

// Logout invalidates a session by its token.
func (s *AuthService) Logout(token string) error {
	tokenHash := auth.HashToken(token)
	return s.store.DeleteSession(tokenHash)
}

// IsBootstrapped returns true if at least one user exists.
func (s *AuthService) IsBootstrapped() (bool, error) {
	count, err := s.store.CountUsers()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// ============================================================================
// User Management
// ============================================================================

// CreateUserRequest contains the fields for creating a new user.
type CreateUserRequest struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
}

// CreateUserResponse contains the result of creating a user.
type CreateUserResponse struct {
	User   *auth.User `json:"user"`
	APIKey string     `json:"api_key"` // plaintext, shown once
}

// CreateUser creates a new user. The actor must have manage_users permission with can_create.
func (s *AuthService) CreateUser(actor *auth.Identity, req CreateUserRequest) (*CreateUserResponse, error) {
	s.logger.Info("Auth: user=%s creating new user=%s", actor.User.Username, req.Username)

	// Validate username
	if !usernameRegex.MatchString(req.Username) {
		return nil, NewServiceError(constants.ErrCodeAuthUsernameInvalid,
			fmt.Sprintf("username must match pattern: %s", constants.AuthUsernameRegex))
	}

	// Validate password
	if len(req.Password) < constants.AuthMinPasswordLength {
		return nil, NewServiceError(constants.ErrCodeAuthPasswordTooWeak,
			fmt.Sprintf("password must be at least %d characters", constants.AuthMinPasswordLength))
	}
	if len(req.Password) > constants.AuthMaxPasswordLength {
		return nil, NewServiceError(constants.ErrCodeAuthPasswordTooWeak,
			fmt.Sprintf("password must be at most %d characters", constants.AuthMaxPasswordLength))
	}

	// Check for duplicate username
	existing, err := s.store.GetUserByUsername(req.Username)
	if err == nil && existing != nil {
		return nil, NewServiceError(constants.ErrCodeAuthUserExists, "username already taken")
	}

	// Hash password
	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		return nil, WrapInternalError(err)
	}

	// Generate API key
	apiKey, err := auth.GenerateAPIKey()
	if err != nil {
		return nil, WrapInternalError(err)
	}
	apiKeyHash := auth.HashToken(apiKey)
	apiKeyPrefix := auth.ExtractTokenPrefix(apiKey)

	// Create user
	user, err := s.store.CreateUser(req.Username, req.DisplayName, passwordHash, &actor.User.ID)
	if err != nil {
		return nil, WrapInternalError(err)
	}

	// Set API key
	if err := s.store.UpdateUserAPIKey(user.ID, apiKeyHash, apiKeyPrefix); err != nil {
		return nil, WrapInternalError(err)
	}

	s.logger.Info("Auth: user=%s created by=%s (id=%d)", req.Username, actor.User.Username, user.ID)

	return &CreateUserResponse{
		User:   user,
		APIKey: apiKey,
	}, nil
}

// GetUser returns a user by ID.
func (s *AuthService) GetUser(id int64) (*auth.User, error) {
	user, err := s.store.GetUserByID(id)
	if err != nil {
		return nil, NewServiceError(constants.ErrCodeAuthUserNotFound, "user not found")
	}
	return &user.User, nil
}

// ListUsers returns all users.
func (s *AuthService) ListUsers() ([]auth.User, error) {
	return s.store.ListUsers()
}

// UpdateUserRequest contains fields for updating a user.
type UpdateUserRequest struct {
	DisplayName *string `json:"display_name,omitempty"`
	IsActive    *bool   `json:"is_active,omitempty"`
	NewPassword *string `json:"new_password,omitempty"`
}

// UpdateUser updates a user's profile.
func (s *AuthService) UpdateUser(actor *auth.Identity, userID int64, req UpdateUserRequest) error {
	user, err := s.store.GetUserByID(userID)
	if err != nil {
		return NewServiceError(constants.ErrCodeAuthUserNotFound, "user not found")
	}

	// Protect bootstrap user from being disabled
	if user.IsBootstrap && req.IsActive != nil && !*req.IsActive {
		return NewServiceError(constants.ErrCodeAuthBootstrapProtected, "cannot disable bootstrap user")
	}

	if req.DisplayName != nil || req.IsActive != nil {
		displayName := user.DisplayName
		isActive := user.IsActive
		if req.DisplayName != nil {
			displayName = *req.DisplayName
		}
		if req.IsActive != nil {
			isActive = *req.IsActive
		}
		if err := s.store.UpdateUser(userID, displayName, isActive); err != nil {
			return WrapInternalError(err)
		}

		// If disabling, invalidate all sessions
		if req.IsActive != nil && !*req.IsActive {
			s.store.DeleteUserSessions(userID)
			s.logger.Info("Auth: user id=%d disabled by=%s, sessions invalidated",
				userID, actor.User.Username)
		}
	}

	if req.NewPassword != nil {
		if len(*req.NewPassword) < constants.AuthMinPasswordLength {
			return NewServiceError(constants.ErrCodeAuthPasswordTooWeak,
				fmt.Sprintf("password must be at least %d characters", constants.AuthMinPasswordLength))
		}
		hash, err := auth.HashPassword(*req.NewPassword)
		if err != nil {
			return WrapInternalError(err)
		}
		if err := s.store.UpdateUserPassword(userID, hash); err != nil {
			return WrapInternalError(err)
		}
		// Invalidate sessions on password change
		s.store.DeleteUserSessions(userID)
		s.logger.Info("Auth: password changed for user id=%d by=%s",
			userID, actor.User.Username)
	}

	return nil
}

// RegenerateAPIKey generates a new API key for a user.
// Returns the plaintext key (shown once).
func (s *AuthService) RegenerateAPIKey(actor *auth.Identity, userID int64) (string, error) {
	user, err := s.store.GetUserByID(userID)
	if err != nil {
		return "", NewServiceError(constants.ErrCodeAuthUserNotFound, "user not found")
	}

	apiKey, err := auth.GenerateAPIKey()
	if err != nil {
		return "", WrapInternalError(err)
	}
	apiKeyHash := auth.HashToken(apiKey)
	apiKeyPrefix := auth.ExtractTokenPrefix(apiKey)

	if err := s.store.UpdateUserAPIKey(user.ID, apiKeyHash, apiKeyPrefix); err != nil {
		return "", WrapInternalError(err)
	}

	s.logger.Info("Auth: API key regenerated for user=%s by=%s", user.Username, actor.User.Username)

	return apiKey, nil
}

// ============================================================================
// Grant Management
// ============================================================================

// CreateGrantRequest contains fields for creating a permission grant.
type CreateGrantRequest struct {
	UserID          int64   `json:"user_id"`
	Action          string  `json:"action"`
	ConstraintsJSON *string `json:"constraints_json,omitempty"`
}

// CreateGrant adds a permission grant to a user.
func (s *AuthService) CreateGrant(actor *auth.Identity, req CreateGrantRequest) (*auth.Grant, error) {
	// Validate action
	if !isValidAction(req.Action) {
		return nil, NewServiceError(constants.ErrCodeAuthInvalidGrant,
			fmt.Sprintf("invalid action: %s", req.Action))
	}

	// Validate constraints JSON schema (reject unknown fields / typos)
	if err := auth.ValidateConstraintsJSON(req.Action, req.ConstraintsJSON); err != nil {
		s.logger.Warn("Auth: invalid constraints for grant action=%s by user=%s: %v",
			req.Action, actor.User.Username, err)
		return nil, NewServiceError(constants.ErrCodeAuthInvalidConstraints, err.Error())
	}

	// Check can_grant_actions restriction on the actor's manage_users grant
	if !s.actorCanGrantAction(actor, req.Action) {
		s.logger.Warn("Auth: grant action denied - user=%s tried to grant action=%s outside can_grant_actions",
			actor.User.Username, req.Action)
		return nil, NewServiceError(constants.ErrCodeAuthGrantActionDenied,
			fmt.Sprintf("not permitted to grant action: %s", req.Action))
	}

	// Check escalation: does the actor have this action themselves?
	if !s.actorHasAction(actor, req.Action) {
		// Check if actor has escalation_allowed
		if !s.actorHasEscalation(actor) {
			s.logger.Warn("Auth: escalation denied - user=%s tried to grant action=%s they don't have",
				actor.User.Username, req.Action)
			return nil, NewServiceError(constants.ErrCodeAuthEscalationDenied,
				"cannot grant permissions you don't have")
		}
	}

	grant, err := s.store.CreateGrant(req.UserID, req.Action, req.ConstraintsJSON, actor.User.ID)
	if err != nil {
		return nil, WrapInternalError(err)
	}

	s.logger.Info("Auth: grant created id=%d action=%s for user_id=%d by=%s",
		grant.ID, req.Action, req.UserID, actor.User.Username)

	return grant, nil
}

// GetUserGrants returns all grants for a user.
func (s *AuthService) GetUserGrants(userID int64) ([]auth.Grant, error) {
	return s.store.GetAllGrantsForUser(userID)
}

// UpdateGrant updates the constraints on an existing grant.
// Returns the grant that was updated so callers can use it for audit logging.
func (s *AuthService) UpdateGrant(actor *auth.Identity, grantID int64, newConstraintsJSON *string) (*auth.Grant, error) {
	grant, err := s.store.GetGrantByID(grantID)
	if err != nil {
		return nil, NewServiceError(constants.ErrCodeAuthInvalidGrant, "grant not found")
	}

	// Validate constraints JSON schema (reject unknown fields / typos)
	if err := auth.ValidateConstraintsJSON(grant.Action, newConstraintsJSON); err != nil {
		s.logger.Warn("Auth: invalid constraints for grant update id=%d action=%s by user=%s: %v",
			grantID, grant.Action, actor.User.Username, err)
		return nil, NewServiceError(constants.ErrCodeAuthInvalidConstraints, err.Error())
	}

	// Check escalation for the action being modified
	if !s.actorHasAction(actor, grant.Action) && !s.actorHasEscalation(actor) {
		return nil, NewServiceError(constants.ErrCodeAuthEscalationDenied,
			"cannot modify grants for actions you don't have")
	}

	if err := s.store.UpdateGrantConstraints(grantID, newConstraintsJSON, actor.User.ID); err != nil {
		return nil, WrapInternalError(err)
	}

	s.logger.Info("Auth: grant id=%d updated by=%s", grantID, actor.User.Username)
	return grant, nil
}

// RevokeGrant revokes a grant (soft delete).
// Returns the grant that was revoked so callers can use it for audit logging.
func (s *AuthService) RevokeGrant(actor *auth.Identity, grantID int64) (*auth.Grant, error) {
	grant, err := s.store.GetGrantByID(grantID)
	if err != nil {
		return nil, NewServiceError(constants.ErrCodeAuthInvalidGrant, "grant not found")
	}

	// Check if target user is bootstrap - prevent revoking all grants
	user, err := s.store.GetUserByID(grant.UserID)
	if err == nil && user.IsBootstrap {
		// Count remaining active grants for this user
		grants, _ := s.store.GetActiveGrantsForUser(grant.UserID)
		if len(grants) <= 1 {
			return nil, NewServiceError(constants.ErrCodeAuthBootstrapProtected,
				"cannot revoke the last grant from the bootstrap user")
		}
	}

	if err := s.store.RevokeGrant(grantID, actor.User.ID); err != nil {
		return nil, WrapInternalError(err)
	}

	s.logger.Info("Auth: grant id=%d revoked by=%s", grantID, actor.User.Username)
	return grant, nil
}

// ============================================================================
// Quota
// ============================================================================

// GetUserQuota returns the current quota usage for a user.
func (s *AuthService) GetUserQuota(userID int64) ([]auth.QuotaUsage, error) {
	return s.store.GetAllQuotaUsage(userID)
}

// ============================================================================
// Helpers
// ============================================================================

func isValidAction(action string) bool {
	for _, a := range constants.AllAuthActions {
		if a == action {
			return true
		}
	}
	return false
}

func (s *AuthService) actorHasAction(actor *auth.Identity, action string) bool {
	for _, g := range actor.Grants {
		if g.Action == action && g.IsActive {
			return true
		}
	}
	return false
}

// actorCanGrantAction checks whether the actor's manage_users grant allows
// granting the specified action. If the actor's manage_users constraint defines
// a can_grant_actions whitelist, only actions in that list are permitted.
// An unconstrained manage_users grant or an empty can_grant_actions list allows all.
func (s *AuthService) actorCanGrantAction(actor *auth.Identity, action string) bool {
	for _, g := range actor.Grants {
		if g.Action != constants.AuthActionManageUsers || !g.IsActive {
			continue
		}

		// No constraints = unrestricted = can grant anything
		if g.ConstraintsJSON == nil {
			return true
		}

		var c auth.ManageUsersConstraints
		if err := parseJSON(*g.ConstraintsJSON, &c); err != nil {
			continue
		}

		// Empty can_grant_actions = can grant any action they have
		if len(c.CanGrantActions) == 0 {
			return true
		}

		// Check if the requested action is in the allowed list
		for _, a := range c.CanGrantActions {
			if a == action {
				return true
			}
		}
	}
	return false
}

func (s *AuthService) actorHasEscalation(actor *auth.Identity) bool {
	for _, g := range actor.Grants {
		if g.Action == constants.AuthActionManageUsers && g.IsActive && g.ConstraintsJSON != nil {
			// Parse to check escalation_allowed
			var c auth.ManageUsersConstraints
			if err := parseJSON(*g.ConstraintsJSON, &c); err == nil {
				if c.EscalationAllowed {
					return true
				}
			}
		}
		// Grant with no constraints = unrestricted = escalation allowed
		if g.Action == constants.AuthActionManageUsers && g.IsActive && g.ConstraintsJSON == nil {
			return true
		}
	}
	return false
}

func parseJSON(data string, v interface{}) error {
	return json.Unmarshal([]byte(data), v)
}

// ============================================================================
// Session Cleanup
// ============================================================================

// Stop stops the session cleanup goroutine (call during graceful shutdown).
func (s *AuthService) Stop() {
	close(s.stopClean)
}

// sessionCleanupLoop periodically purges expired sessions from the database.
func (s *AuthService) sessionCleanupLoop() {
	ticker := time.NewTicker(constants.AuthSessionCleanupInterval)
	defer ticker.Stop()

	s.logger.Info("Auth: session cleanup goroutine started (interval=%s)", constants.AuthSessionCleanupInterval)

	for {
		select {
		case <-s.stopClean:
			s.logger.Info("Auth: session cleanup goroutine stopped")
			return
		case <-ticker.C:
			removed, err := s.store.CleanupExpiredSessions()
			if err != nil {
				s.logger.Error("Auth: session cleanup failed: %v", err)
				continue
			}
			if removed > 0 {
				s.logger.Info("Auth: session cleanup removed %d expired sessions", removed)
			} else {
				s.logger.Debug("Auth: session cleanup found no expired sessions")
			}
		}
	}
}
