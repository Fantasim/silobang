package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"meshbank/internal/constants"
	"meshbank/internal/logger"
)

// contextKey is an unexported type for context keys in this package.
type contextKey int

const (
	identityContextKey contextKey = iota
)

// StoreProvider is a function that returns the current auth store.
// This allows dynamic resolution — the middleware resolves the store on each
// request instead of holding a fixed reference, so it adapts when the auth
// system is initialised after the server starts (e.g. POST /api/config).
type StoreProvider func() *Store

// Middleware provides HTTP middleware for authentication.
type Middleware struct {
	getStore StoreProvider
	logger   *logger.Logger
}

// NewMiddleware creates a new auth middleware with a dynamic store provider.
func NewMiddleware(provider StoreProvider, log *logger.Logger) *Middleware {
	return &Middleware{getStore: provider, logger: log}
}

// Authenticate extracts and validates the identity from the request.
// Sets Identity on context. Handlers that require auth use RequireAuth to check.
// This middleware always calls next — it does not block unauthenticated requests.
// Individual handlers decide whether auth is required for their endpoint.
func (m *Middleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity := m.resolveIdentity(r)
		ctx := context.WithValue(r.Context(), identityContextKey, identity)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// resolveIdentity attempts to extract a valid identity from the request.
// Tries API key first, then session token.
// Returns nil if the store is not yet available (auth not initialised).
func (m *Middleware) resolveIdentity(r *http.Request) *Identity {
	store := m.getStore()
	if store == nil {
		// Auth system not yet initialised (no orchestrator DB)
		return nil
	}

	// Priority 1: X-API-Key header
	if apiKey := r.Header.Get(constants.HeaderXAPIKey); apiKey != "" {
		identity := m.resolveAPIKey(store, apiKey)
		if identity != nil {
			return identity
		}
	}

	// Priority 2: Authorization Bearer header
	if authHeader := r.Header.Get(constants.HeaderAuthorization); authHeader != "" {
		if strings.HasPrefix(authHeader, constants.AuthBearerPrefix) {
			token := strings.TrimPrefix(authHeader, constants.AuthBearerPrefix)
			if IsAPIKey(token) {
				identity := m.resolveAPIKey(store, token)
				if identity != nil {
					return identity
				}
			} else if IsSessionToken(token) {
				identity := m.resolveSession(store, token)
				if identity != nil {
					return identity
				}
			}
		}
	}

	// Priority 3: Query parameter token (for SSE EventSource and downloads
	// which cannot set custom headers)
	if token := r.URL.Query().Get(constants.AuthQueryParamToken); token != "" {
		if IsAPIKey(token) {
			identity := m.resolveAPIKey(store, token)
			if identity != nil {
				return identity
			}
		} else if IsSessionToken(token) {
			identity := m.resolveSession(store, token)
			if identity != nil {
				return identity
			}
		}
	}

	return nil
}

// resolveAPIKey looks up a user by their API key hash.
func (m *Middleware) resolveAPIKey(store *Store, apiKey string) *Identity {
	keyHash := HashToken(apiKey)

	user, err := store.GetUserByAPIKeyHash(keyHash)
	if err != nil {
		m.logger.Debug("Auth: API key lookup failed: %v", err)
		return nil
	}

	if !user.IsActive {
		m.logger.Debug("Auth: API key user %s is inactive", user.Username)
		return nil
	}

	// Check account lockout
	if user.LockedUntil != nil {
		now := time.Now().Unix()
		if now < *user.LockedUntil {
			m.logger.Debug("Auth: API key user %s is locked until %d", user.Username, *user.LockedUntil)
			return nil
		}
	}

	grants, err := store.GetActiveGrantsForUser(user.ID)
	if err != nil {
		m.logger.Error("Auth: failed to load grants for user %s: %v", user.Username, err)
		return nil
	}

	return &Identity{
		User:   &user.User,
		Method: "api_key",
		Grants: grants,
	}
}

// resolveSession looks up a user by their session token hash.
func (m *Middleware) resolveSession(store *Store, token string) *Identity {
	tokenHash := HashToken(token)

	session, user, err := store.GetSessionByTokenHash(tokenHash)
	if err != nil {
		m.logger.Debug("Auth: session lookup failed: %v", err)
		return nil
	}
	if session == nil || user == nil {
		return nil
	}

	// Touch session (update last_active_at)
	if err := store.TouchSession(tokenHash); err != nil {
		m.logger.Warn("Auth: failed to touch session: %v", err)
	}

	grants, err := store.GetActiveGrantsForUser(user.ID)
	if err != nil {
		m.logger.Error("Auth: failed to load grants for user %s: %v", user.Username, err)
		return nil
	}

	return &Identity{
		User:   user,
		Method: "session",
		Grants: grants,
	}
}

// GetIdentity retrieves the authenticated identity from the request context.
// Returns nil if no identity is present (unauthenticated request).
func GetIdentity(r *http.Request) *Identity {
	identity, _ := r.Context().Value(identityContextKey).(*Identity)
	return identity
}

// RequireAuth is a helper that extracts the identity and returns false if not present.
// Handlers use this to enforce authentication:
//
//	identity, ok := auth.RequireAuth(r)
//	if !ok { WriteError(w, 401, ...); return }
func RequireAuth(r *http.Request) (*Identity, bool) {
	identity := GetIdentity(r)
	if identity == nil || identity.User == nil {
		return nil, false
	}
	return identity, true
}
