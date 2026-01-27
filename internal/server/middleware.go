package server

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

// Chain applies middlewares in order. The first middleware is the outermost (runs first).
// Usage: Chain(handler, requestID, securityHeaders, authenticate)
// Request flow: requestID → securityHeaders → authenticate → handler
func Chain(handler http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	// Apply in reverse so the first middleware in the list is outermost
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}

// SecurityHeaders adds standard security headers to every response.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// requestIDHeaderKey is the HTTP header for request tracing.
const requestIDHeaderKey = "X-Request-ID"

// RequestID generates a unique request ID and sets it on the response header.
// If the incoming request already has an X-Request-ID header, it is preserved.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(requestIDHeaderKey)
		if id == "" {
			id = generateRequestID()
		}
		w.Header().Set(requestIDHeaderKey, id)
		next.ServeHTTP(w, r)
	})
}

// generateRequestID creates a random 16-byte hex string.
func generateRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b)
}
