package server

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"

	"silobang/internal/constants"
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

// =============================================================================
// Gzip Compression Middleware
// =============================================================================

// GzipCompress conditionally gzip-compresses API responses.
// Compression is applied only when all conditions are met:
//   - Request path starts with /api/
//   - Client sends Accept-Encoding: gzip
//   - Response body exceeds the minimum size threshold
//   - Response content type is compressible (JSON, text — not binary)
//
// Streaming responses (SSE) bypass compression via Flush() detection.
func GzipCompress(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only compress API routes
		if !strings.HasPrefix(r.URL.Path, constants.CompressionAPIPathPrefix) {
			next.ServeHTTP(w, r)
			return
		}

		// Only if client accepts gzip
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		// Always set Vary header so caches key on Accept-Encoding
		w.Header().Set("Vary", "Accept-Encoding")

		grw := newGzipResponseWriter(w)
		defer grw.finish()

		next.ServeHTTP(grw, r)
	})
}

// gzipResponseWriter buffers response data and conditionally compresses it.
// If the response exceeds the size threshold and has a compressible content type,
// the data is gzip-compressed before being sent to the client.
// If Flush() is called (e.g., for SSE streams), compression is disabled and
// buffered data is flushed raw immediately.
type gzipResponseWriter struct {
	http.ResponseWriter
	buf           bytes.Buffer
	statusCode    int
	headerWritten bool // true if handler called WriteHeader explicitly
	flushed       bool // true = streaming mode, compression disabled
}

func newGzipResponseWriter(w http.ResponseWriter) *gzipResponseWriter {
	return &gzipResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

// WriteHeader captures the status code without writing to the underlying writer.
// The actual header write is deferred to finish() so we can modify headers
// (e.g., add Content-Encoding) before they are sent.
func (g *gzipResponseWriter) WriteHeader(code int) {
	if g.flushed {
		g.ResponseWriter.WriteHeader(code)
		return
	}
	g.statusCode = code
	g.headerWritten = true
}

// Write buffers data. If already flushed (streaming mode), writes directly.
func (g *gzipResponseWriter) Write(b []byte) (int, error) {
	if g.flushed {
		return g.ResponseWriter.Write(b)
	}
	return g.buf.Write(b)
}

// Flush switches to streaming mode, bypassing compression.
// This supports SSE (text/event-stream) and other streaming responses
// that need incremental delivery.
func (g *gzipResponseWriter) Flush() {
	if !g.flushed {
		g.flushed = true
		// Drain buffered data raw
		if g.headerWritten {
			g.ResponseWriter.WriteHeader(g.statusCode)
		}
		if g.buf.Len() > 0 {
			g.ResponseWriter.Write(g.buf.Bytes()) //nolint:errcheck
			g.buf.Reset()
		}
	}
	if f, ok := g.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap returns the underlying ResponseWriter for http.ResponseController.
func (g *gzipResponseWriter) Unwrap() http.ResponseWriter {
	return g.ResponseWriter
}

// finish is called via defer after the handler completes.
// It decides whether to compress the buffered data or write it raw.
func (g *gzipResponseWriter) finish() {
	if g.flushed {
		return // Already flushed raw (streaming response)
	}

	data := g.buf.Bytes()

	// Skip compression if Content-Encoding is already set
	// (e.g., pre-compressed cached responses from schema/prompts handlers)
	if g.Header().Get("Content-Encoding") != "" {
		if g.headerWritten {
			g.ResponseWriter.WriteHeader(g.statusCode)
		}
		if len(data) > 0 {
			g.ResponseWriter.Write(data) //nolint:errcheck
		}
		return
	}

	// Compress if response exceeds threshold and content type is compressible
	if len(data) >= constants.CompressionMinSizeBytes && isCompressibleContentType(g.Header().Get("Content-Type")) {
		var compressed bytes.Buffer
		gz, err := gzip.NewWriterLevel(&compressed, constants.CompressionLevel)
		if err == nil {
			if _, werr := gz.Write(data); werr == nil {
				gz.Close()
				// Only use compressed version if it actually saves bytes
				if compressed.Len() < len(data) {
					g.ResponseWriter.Header().Set("Content-Encoding", "gzip")
					g.ResponseWriter.Header().Del("Content-Length")
					if g.headerWritten {
						g.ResponseWriter.WriteHeader(g.statusCode)
					}
					g.ResponseWriter.Write(compressed.Bytes()) //nolint:errcheck
					return
				}
			} else {
				gz.Close()
			}
		}
	}

	// Write raw (no compression)
	if g.headerWritten {
		g.ResponseWriter.WriteHeader(g.statusCode)
	}
	if len(data) > 0 {
		g.ResponseWriter.Write(data) //nolint:errcheck
	}
}

// isCompressibleContentType returns true for text-based and JSON content types.
// Binary formats (octet-stream, ZIP, images, models) are already compact or
// pre-compressed, so compressing them wastes CPU without meaningful size savings.
func isCompressibleContentType(ct string) bool {
	if ct == "" {
		return false
	}
	// text/* except event-stream (SSE is handled by Flush, but belt-and-suspenders)
	if strings.HasPrefix(ct, "text/") {
		return !strings.HasPrefix(ct, "text/event-stream")
	}
	// JSON (application/json, application/json; charset=utf-8, etc.)
	if strings.Contains(ct, "application/json") {
		return true
	}
	return false
}
