package server

import (
	"silobang/internal/auth"
	"net/http"
	"strings"
)

// getClientIP extracts the client IP address from the request
// It checks proxy headers first, then falls back to RemoteAddr
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (reverse proxy)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the chain (original client)
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	// Remove port if present
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

// getAuditUsername extracts the username from an authenticated identity for audit logging.
// Returns empty string if identity is nil (e.g. unauthenticated or system actions).
func getAuditUsername(identity *auth.Identity) string {
	if identity != nil && identity.User != nil {
		return identity.User.Username
	}
	return ""
}
