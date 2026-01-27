package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"silobang/internal/audit"
	"silobang/internal/auth"
	"silobang/internal/constants"
	"silobang/internal/services"
)

// =============================================================================
// Auth Helpers
// =============================================================================

// requireAuth extracts the authenticated identity from the request.
// Returns nil and writes a 401 response if not authenticated.
func (s *Server) requireAuth(w http.ResponseWriter, r *http.Request) *auth.Identity {
	identity, ok := auth.RequireAuth(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, "Authentication required", constants.ErrCodeAuthRequired)
		return nil
	}
	return identity
}

// authorize evaluates a policy for the given identity and action context.
// Returns true if allowed, writes the appropriate error response and returns false if denied.
func (s *Server) authorize(w http.ResponseWriter, identity *auth.Identity, ctx *auth.ActionContext) bool {
	if s.app.Services.Auth == nil {
		// Auth not initialized (no DB) — deny by default
		WriteError(w, http.StatusServiceUnavailable, "Auth system not available", constants.ErrCodeAuthRequired)
		return false
	}

	result := s.app.Services.Auth.GetEvaluator().Evaluate(identity, ctx)
	if !result.Allowed {
		status := http.StatusForbidden
		if result.DeniedCode == constants.ErrCodeAuthQuotaExceeded {
			status = http.StatusTooManyRequests
		}
		WriteError(w, status, result.Reason, result.DeniedCode)
		return false
	}
	return true
}

// isAuthAvailable returns true if the auth system is initialized.
// When false, auth endpoints should return 503.
func (s *Server) isAuthAvailable() bool {
	return s.app.Services.Auth != nil
}

// =============================================================================
// Public Auth Endpoints
// =============================================================================

// POST /api/auth/login — Authenticate and receive a session token
func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !s.isAuthAvailable() {
		WriteError(w, http.StatusServiceUnavailable, "Auth system not available", constants.ErrCodeNotConfigured)
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid JSON", constants.ErrCodeInvalidRequest)
		return
	}

	if req.Username == "" || req.Password == "" {
		WriteError(w, http.StatusBadRequest, "Username and password are required", constants.ErrCodeInvalidRequest)
		return
	}

	token, user, err := s.app.Services.Auth.Login(
		req.Username, req.Password,
		getClientIP(r), r.UserAgent(),
	)
	if err != nil {
		// Audit failed login attempt
		if s.app.AuditLogger != nil {
			reason := "unknown"
			if code, ok := services.IsServiceError(err); ok {
				reason = code
			}
			s.app.AuditLogger.Log(constants.AuditActionLoginFailed, getClientIP(r), req.Username, audit.LoginFailedDetails{
				AttemptedUsername: req.Username,
				Reason:           reason,
				UserAgent:        r.UserAgent(),
			})
		}
		s.handleServiceError(w, err)
		return
	}

	// Audit successful login
	if s.app.AuditLogger != nil {
		s.app.AuditLogger.Log(constants.AuditActionLoginSuccess, getClientIP(r), req.Username, audit.LoginSuccessDetails{
			UserAgent: r.UserAgent(),
		})
	}

	WriteSuccess(w, map[string]interface{}{
		"token": token,
		"user":  user,
	})
}

// GET /api/auth/status — Check whether the system is bootstrapped
func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !s.isAuthAvailable() {
		WriteSuccess(w, map[string]interface{}{
			"bootstrapped": false,
			"configured":   false,
		})
		return
	}

	bootstrapped, err := s.app.Services.Auth.IsBootstrapped()
	if err != nil {
		s.handleServiceError(w, err)
		return
	}

	WriteSuccess(w, map[string]interface{}{
		"bootstrapped": bootstrapped,
		"configured":   true,
	})
}

// =============================================================================
// Protected Auth Endpoints
// =============================================================================

// POST /api/auth/logout — Invalidate current session
func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	identity := s.requireAuth(w, r)
	if identity == nil {
		return
	}

	// Extract the token from the Authorization header to invalidate it
	authHeader := r.Header.Get(constants.HeaderAuthorization)
	if strings.HasPrefix(authHeader, constants.AuthBearerPrefix) {
		token := strings.TrimPrefix(authHeader, constants.AuthBearerPrefix)
		if auth.IsSessionToken(token) {
			if err := s.app.Services.Auth.Logout(token); err != nil {
				s.logger.Warn("Auth: logout failed for user=%s: %v", identity.User.Username, err)
			}
		}
	}

	s.logger.Info("Auth: user=%s logged out", identity.User.Username)

	// Audit logout
	if s.app.AuditLogger != nil {
		s.app.AuditLogger.Log(constants.AuditActionLogout, getClientIP(r), getAuditUsername(identity), audit.LogoutDetails{})
	}

	WriteSuccess(w, map[string]interface{}{
		"success": true,
	})
}

// GET /api/auth/me — Current user info + grants
func (s *Server) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	identity := s.requireAuth(w, r)
	if identity == nil {
		return
	}

	WriteSuccess(w, map[string]interface{}{
		"user":   identity.User,
		"method": identity.Method,
		"grants": identity.Grants,
	})
}

// GET /api/auth/me/quota — Current user's quota usage
func (s *Server) handleAuthMeQuota(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	identity := s.requireAuth(w, r)
	if identity == nil {
		return
	}

	quotas, err := s.app.Services.Auth.GetUserQuota(identity.User.ID)
	if err != nil {
		s.handleServiceError(w, err)
		return
	}

	WriteSuccess(w, map[string]interface{}{
		"user_id": identity.User.ID,
		"quotas":  quotas,
	})
}

// =============================================================================
// User Management Endpoints (requires manage_users grant)
// =============================================================================

// /api/auth/users — GET (list) or POST (create)
func (s *Server) handleAuthUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listUsers(w, r)
	case http.MethodPost:
		s.createUser(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	identity := s.requireAuth(w, r)
	if identity == nil {
		return
	}

	if !s.authorize(w, identity, &auth.ActionContext{Action: constants.AuthActionManageUsers}) {
		return
	}

	users, err := s.app.Services.Auth.ListUsers()
	if err != nil {
		s.handleServiceError(w, err)
		return
	}

	WriteSuccess(w, map[string]interface{}{
		"users": users,
	})
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	identity := s.requireAuth(w, r)
	if identity == nil {
		return
	}

	if !s.authorize(w, identity, &auth.ActionContext{
		Action:    constants.AuthActionManageUsers,
		SubAction: "create",
	}) {
		return
	}

	var req services.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid JSON", constants.ErrCodeInvalidRequest)
		return
	}

	resp, err := s.app.Services.Auth.CreateUser(identity, req)
	if err != nil {
		s.handleServiceError(w, err)
		return
	}

	// Audit user creation
	if s.app.AuditLogger != nil {
		s.app.AuditLogger.Log(constants.AuditActionUserCreated, getClientIP(r), getAuditUsername(identity), audit.UserCreatedDetails{
			CreatedUserID:   resp.User.ID,
			CreatedUsername: resp.User.Username,
		})
	}

	WriteJSON(w, http.StatusCreated, resp)
}

// /api/auth/users/{id} — GET or PATCH
func (s *Server) handleAuthUserByID(w http.ResponseWriter, r *http.Request, userID int64) {
	switch r.Method {
	case http.MethodGet:
		s.getUserByID(w, r, userID)
	case http.MethodPatch:
		s.updateUser(w, r, userID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) getUserByID(w http.ResponseWriter, r *http.Request, userID int64) {
	identity := s.requireAuth(w, r)
	if identity == nil {
		return
	}

	if !s.authorize(w, identity, &auth.ActionContext{Action: constants.AuthActionManageUsers}) {
		return
	}

	user, err := s.app.Services.Auth.GetUser(userID)
	if err != nil {
		s.handleServiceError(w, err)
		return
	}

	// Also fetch grants for this user
	grants, err := s.app.Services.Auth.GetUserGrants(userID)
	if err != nil {
		s.handleServiceError(w, err)
		return
	}

	WriteSuccess(w, map[string]interface{}{
		"user":   user,
		"grants": grants,
	})
}

func (s *Server) updateUser(w http.ResponseWriter, r *http.Request, userID int64) {
	identity := s.requireAuth(w, r)
	if identity == nil {
		return
	}

	if !s.authorize(w, identity, &auth.ActionContext{
		Action:    constants.AuthActionManageUsers,
		SubAction: "edit",
	}) {
		return
	}

	var req services.UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid JSON", constants.ErrCodeInvalidRequest)
		return
	}

	if err := s.app.Services.Auth.UpdateUser(identity, userID, req); err != nil {
		s.handleServiceError(w, err)
		return
	}

	// Audit user update
	if s.app.AuditLogger != nil {
		var fieldsChanged []string
		if req.DisplayName != nil {
			fieldsChanged = append(fieldsChanged, "display_name")
		}
		if req.IsActive != nil {
			fieldsChanged = append(fieldsChanged, "is_active")
		}
		if req.NewPassword != nil {
			fieldsChanged = append(fieldsChanged, "password")
		}
		targetUsername := ""
		if targetUser, err := s.app.Services.Auth.GetUser(userID); err == nil {
			targetUsername = targetUser.Username
		}
		s.app.AuditLogger.Log(constants.AuditActionUserUpdated, getClientIP(r), getAuditUsername(identity), audit.UserUpdatedDetails{
			TargetUserID:   userID,
			TargetUsername: targetUsername,
			FieldsChanged:  fieldsChanged,
		})
	}

	WriteSuccess(w, map[string]interface{}{
		"success": true,
	})
}

// POST /api/auth/users/{id}/api-key — Regenerate API key
func (s *Server) handleRegenerateAPIKey(w http.ResponseWriter, r *http.Request, userID int64) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	identity := s.requireAuth(w, r)
	if identity == nil {
		return
	}

	if !s.authorize(w, identity, &auth.ActionContext{
		Action:    constants.AuthActionManageUsers,
		SubAction: "edit",
	}) {
		return
	}

	apiKey, err := s.app.Services.Auth.RegenerateAPIKey(identity, userID)
	if err != nil {
		s.handleServiceError(w, err)
		return
	}

	// Audit API key regeneration
	if s.app.AuditLogger != nil {
		targetUsername := ""
		if targetUser, err := s.app.Services.Auth.GetUser(userID); err == nil {
			targetUsername = targetUser.Username
		}
		s.app.AuditLogger.Log(constants.AuditActionAPIKeyRegenerated, getClientIP(r), getAuditUsername(identity), audit.APIKeyRegeneratedDetails{
			TargetUserID:   userID,
			TargetUsername: targetUsername,
		})
	}

	WriteSuccess(w, map[string]interface{}{
		"api_key": apiKey,
	})
}

// =============================================================================
// Grant Management Endpoints (requires manage_users grant)
// =============================================================================

// /api/auth/users/{id}/grants — GET (list) or POST (create)
func (s *Server) handleUserGrants(w http.ResponseWriter, r *http.Request, userID int64) {
	switch r.Method {
	case http.MethodGet:
		s.listUserGrants(w, r, userID)
	case http.MethodPost:
		s.createUserGrant(w, r, userID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) listUserGrants(w http.ResponseWriter, r *http.Request, userID int64) {
	identity := s.requireAuth(w, r)
	if identity == nil {
		return
	}

	if !s.authorize(w, identity, &auth.ActionContext{Action: constants.AuthActionManageUsers}) {
		return
	}

	grants, err := s.app.Services.Auth.GetUserGrants(userID)
	if err != nil {
		s.handleServiceError(w, err)
		return
	}

	WriteSuccess(w, map[string]interface{}{
		"grants": grants,
	})
}

func (s *Server) createUserGrant(w http.ResponseWriter, r *http.Request, userID int64) {
	identity := s.requireAuth(w, r)
	if identity == nil {
		return
	}

	if !s.authorize(w, identity, &auth.ActionContext{
		Action:    constants.AuthActionManageUsers,
		SubAction: "create",
	}) {
		return
	}

	var req services.CreateGrantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid JSON", constants.ErrCodeInvalidRequest)
		return
	}

	// Override user_id from URL path (prevent parameter injection)
	req.UserID = userID

	grant, err := s.app.Services.Auth.CreateGrant(identity, req)
	if err != nil {
		s.handleServiceError(w, err)
		return
	}

	// Audit grant creation
	if s.app.AuditLogger != nil {
		s.app.AuditLogger.Log(constants.AuditActionGrantCreated, getClientIP(r), getAuditUsername(identity), audit.GrantCreatedDetails{
			GrantID:        grant.ID,
			TargetUserID:   userID,
			Action:         req.Action,
			HasConstraints: req.ConstraintsJSON != nil,
		})
	}

	WriteJSON(w, http.StatusCreated, grant)
}

// /api/auth/grants/{id} — PATCH (update) or DELETE (revoke)
func (s *Server) handleGrantByID(w http.ResponseWriter, r *http.Request, grantID int64) {
	switch r.Method {
	case http.MethodPatch:
		s.updateGrant(w, r, grantID)
	case http.MethodDelete:
		s.revokeGrant(w, r, grantID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) updateGrant(w http.ResponseWriter, r *http.Request, grantID int64) {
	identity := s.requireAuth(w, r)
	if identity == nil {
		return
	}

	if !s.authorize(w, identity, &auth.ActionContext{
		Action:    constants.AuthActionManageUsers,
		SubAction: "edit",
	}) {
		return
	}

	var req struct {
		ConstraintsJSON *string `json:"constraints_json"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid JSON", constants.ErrCodeInvalidRequest)
		return
	}

	grant, err := s.app.Services.Auth.UpdateGrant(identity, grantID, req.ConstraintsJSON)
	if err != nil {
		s.handleServiceError(w, err)
		return
	}

	// Audit grant update
	if s.app.AuditLogger != nil {
		s.app.AuditLogger.Log(constants.AuditActionGrantUpdated, getClientIP(r), getAuditUsername(identity), audit.GrantUpdatedDetails{
			GrantID:        grantID,
			TargetUserID:   grant.UserID,
			Action:         grant.Action,
			HasConstraints: req.ConstraintsJSON != nil,
		})
	}

	WriteSuccess(w, map[string]interface{}{
		"success": true,
	})
}

func (s *Server) revokeGrant(w http.ResponseWriter, r *http.Request, grantID int64) {
	identity := s.requireAuth(w, r)
	if identity == nil {
		return
	}

	if !s.authorize(w, identity, &auth.ActionContext{
		Action:    constants.AuthActionManageUsers,
		SubAction: "edit",
	}) {
		return
	}

	grant, err := s.app.Services.Auth.RevokeGrant(identity, grantID)
	if err != nil {
		s.handleServiceError(w, err)
		return
	}

	// Audit grant revocation
	if s.app.AuditLogger != nil {
		s.app.AuditLogger.Log(constants.AuditActionGrantRevoked, getClientIP(r), getAuditUsername(identity), audit.GrantRevokedDetails{
			GrantID:      grantID,
			TargetUserID: grant.UserID,
			Action:       grant.Action,
		})
	}

	WriteSuccess(w, map[string]interface{}{
		"success": true,
	})
}

// =============================================================================
// Quota Endpoints
// =============================================================================

// GET /api/auth/users/{id}/quota — Admin: view user's quota
func (s *Server) handleUserQuota(w http.ResponseWriter, r *http.Request, userID int64) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	identity := s.requireAuth(w, r)
	if identity == nil {
		return
	}

	if !s.authorize(w, identity, &auth.ActionContext{Action: constants.AuthActionManageUsers}) {
		return
	}

	quotas, err := s.app.Services.Auth.GetUserQuota(userID)
	if err != nil {
		s.handleServiceError(w, err)
		return
	}

	WriteSuccess(w, map[string]interface{}{
		"user_id": userID,
		"quotas":  quotas,
	})
}

// =============================================================================
// Auth Route Dispatcher
// =============================================================================

// handleAuthRoutes dispatches /api/auth/... routes.
func (s *Server) handleAuthRoutes(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	prefix := "/api/auth/"

	if !strings.HasPrefix(path, prefix) {
		http.NotFound(w, r)
		return
	}

	remaining := strings.TrimPrefix(path, prefix)

	switch {
	// /api/auth/login
	case remaining == "login":
		s.handleAuthLogin(w, r)

	// /api/auth/status
	case remaining == "status":
		s.handleAuthStatus(w, r)

	// /api/auth/logout
	case remaining == "logout":
		s.handleAuthLogout(w, r)

	// /api/auth/me
	case remaining == "me":
		s.handleAuthMe(w, r)

	// /api/auth/me/quota
	case remaining == "me/quota":
		s.handleAuthMeQuota(w, r)

	// /api/auth/users
	case remaining == "users":
		s.handleAuthUsers(w, r)

	// /api/auth/users/{id}
	// /api/auth/users/{id}/api-key
	// /api/auth/users/{id}/grants
	// /api/auth/users/{id}/quota
	case strings.HasPrefix(remaining, "users/"):
		s.routeAuthUserSub(w, r, strings.TrimPrefix(remaining, "users/"))

	// /api/auth/grants/{id}
	case strings.HasPrefix(remaining, "grants/"):
		s.routeAuthGrantSub(w, r, strings.TrimPrefix(remaining, "grants/"))

	default:
		http.NotFound(w, r)
	}
}

// routeAuthUserSub handles /api/auth/users/{id}[/sub-resource]
func (s *Server) routeAuthUserSub(w http.ResponseWriter, r *http.Request, remaining string) {
	parts := strings.SplitN(remaining, "/", 2)
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}

	userID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid user ID", constants.ErrCodeInvalidRequest)
		return
	}

	if len(parts) == 1 {
		// /api/auth/users/{id}
		s.handleAuthUserByID(w, r, userID)
		return
	}

	subResource := parts[1]
	switch subResource {
	case "api-key":
		s.handleRegenerateAPIKey(w, r, userID)
	case "grants":
		s.handleUserGrants(w, r, userID)
	case "quota":
		s.handleUserQuota(w, r, userID)
	default:
		http.NotFound(w, r)
	}
}

// routeAuthGrantSub handles /api/auth/grants/{id}
func (s *Server) routeAuthGrantSub(w http.ResponseWriter, r *http.Request, remaining string) {
	grantID, err := strconv.ParseInt(remaining, 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid grant ID", constants.ErrCodeInvalidRequest)
		return
	}

	s.handleGrantByID(w, r, grantID)
}
