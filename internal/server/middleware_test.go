package server

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"silobang/internal/constants"
)

// =============================================================================
// GzipCompress — Large JSON Response
// =============================================================================

func TestGzipMiddleware_CompressesLargeJSONResponse(t *testing.T) {
	// Handler returns a JSON response > 1KB
	largeJSON := `{"data":"` + strings.Repeat("abcdefghij", 200) + `"}`

	handler := GzipCompress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(largeJSON))
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.Header.Get("Content-Encoding") != "gzip" {
		t.Errorf("Expected Content-Encoding: gzip, got %q", resp.Header.Get("Content-Encoding"))
	}

	if resp.Header.Get("Vary") != "Accept-Encoding" {
		t.Errorf("Expected Vary: Accept-Encoding, got %q", resp.Header.Get("Vary"))
	}

	// Decompress and verify content
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		t.Fatalf("Failed to create gzip reader: %v", err)
	}
	defer gz.Close()

	decompressed, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("Failed to decompress: %v", err)
	}

	if string(decompressed) != largeJSON {
		t.Errorf("Decompressed data mismatch: got %d bytes, want %d bytes", len(decompressed), len(largeJSON))
	}

	// Compressed should be smaller than original
	compressedBody, _ := io.ReadAll(bytes.NewReader(rec.Body.Bytes()))
	if len(compressedBody) >= len(largeJSON) {
		t.Errorf("Expected compressed size (%d) < original size (%d)", len(compressedBody), len(largeJSON))
	}
}

// =============================================================================
// GzipCompress — Large Text Response
// =============================================================================

func TestGzipMiddleware_CompressesLargeTextResponse(t *testing.T) {
	largeText := strings.Repeat("log line content here\n", 100)

	handler := GzipCompress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(largeText))
	}))

	req := httptest.NewRequest("GET", "/api/monitoring/logs/warn/test.log", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.Header.Get("Content-Encoding") != "gzip" {
		t.Errorf("Expected Content-Encoding: gzip for text/plain, got %q", resp.Header.Get("Content-Encoding"))
	}
}

// =============================================================================
// GzipCompress — Small Response Skipped
// =============================================================================

func TestGzipMiddleware_SkipsSmallResponse(t *testing.T) {
	smallJSON := `{"ok":true}`

	handler := GzipCompress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(smallJSON))
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.Header.Get("Content-Encoding") == "gzip" {
		t.Error("Should NOT compress small responses (< CompressionMinSizeBytes)")
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != smallJSON {
		t.Errorf("Expected raw body %q, got %q", smallJSON, string(body))
	}
}

// =============================================================================
// GzipCompress — Exact Threshold Boundary
// =============================================================================

func TestGzipMiddleware_ExactThresholdNotCompressed(t *testing.T) {
	// Generate exactly CompressionMinSizeBytes - 1 bytes of JSON
	padding := strings.Repeat("x", constants.CompressionMinSizeBytes-15)
	belowThreshold := `{"data":"` + padding + `"}`
	if len(belowThreshold) >= constants.CompressionMinSizeBytes {
		// Adjust if we went over
		belowThreshold = belowThreshold[:constants.CompressionMinSizeBytes-1]
	}

	handler := GzipCompress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(belowThreshold))
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") == "gzip" {
		t.Errorf("Should NOT compress responses below %d bytes", constants.CompressionMinSizeBytes)
	}
}

// =============================================================================
// GzipCompress — No Accept-Encoding Header
// =============================================================================

func TestGzipMiddleware_SkipsWithoutAcceptEncoding(t *testing.T) {
	largeJSON := `{"data":"` + strings.Repeat("abcdefghij", 200) + `"}`

	handler := GzipCompress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(largeJSON))
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	// No Accept-Encoding header
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.Header.Get("Content-Encoding") == "gzip" {
		t.Error("Should NOT compress when client doesn't accept gzip")
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != largeJSON {
		t.Errorf("Expected raw body, got %d bytes", len(body))
	}
}

// =============================================================================
// GzipCompress — Non-API Routes Skipped
// =============================================================================

func TestGzipMiddleware_SkipsNonAPIRoutes(t *testing.T) {
	largeHTML := strings.Repeat("<p>Hello World</p>", 200)

	handler := GzipCompress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(largeHTML))
	}))

	req := httptest.NewRequest("GET", "/index.html", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.Header.Get("Content-Encoding") == "gzip" {
		t.Error("Should NOT compress non-API routes")
	}

	// Vary header should NOT be set for non-API routes
	if resp.Header.Get("Vary") == "Accept-Encoding" {
		t.Error("Should NOT set Vary header for non-API routes")
	}
}

// =============================================================================
// GzipCompress — Binary Content Type Skipped
// =============================================================================

func TestGzipMiddleware_SkipsBinaryContentType(t *testing.T) {
	binaryData := bytes.Repeat([]byte{0x00, 0xFF, 0xAB}, 1000)

	handler := GzipCompress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(binaryData)
	}))

	req := httptest.NewRequest("GET", "/api/assets/abc123/download", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") == "gzip" {
		t.Error("Should NOT compress binary content types (application/octet-stream)")
	}
}

func TestGzipMiddleware_SkipsZipContentType(t *testing.T) {
	zipData := bytes.Repeat([]byte{0x50, 0x4B}, 1000)

	handler := GzipCompress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		w.Write(zipData)
	}))

	req := httptest.NewRequest("GET", "/api/download/bulk/test-id", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") == "gzip" {
		t.Error("Should NOT compress ZIP content types")
	}
}

// =============================================================================
// GzipCompress — SSE Stream Bypass via Flush
// =============================================================================

func TestGzipMiddleware_StreamingBypassesCompression(t *testing.T) {
	handler := GzipCompress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)

		// Simulate SSE: write + flush
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("ResponseWriter should implement Flusher")
		}

		w.Write([]byte("data: {\"type\":\"start\"}\n\n"))
		flusher.Flush()
		w.Write([]byte("data: {\"type\":\"end\"}\n\n"))
		flusher.Flush()
	}))

	req := httptest.NewRequest("GET", "/api/audit/stream", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	// SSE should NOT be compressed
	if resp.Header.Get("Content-Encoding") == "gzip" {
		t.Error("Should NOT compress streaming/SSE responses")
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "data: {\"type\":\"start\"}") {
		t.Error("Expected raw SSE data in response")
	}
}

// =============================================================================
// GzipCompress — Status Code Preserved
// =============================================================================

func TestGzipMiddleware_PreservesStatusCode(t *testing.T) {
	largeJSON := `{"error":true,"code":"NOT_FOUND","message":"` + strings.Repeat("x", 2000) + `"}`

	handler := GzipCompress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(largeJSON))
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", rec.Code)
	}

	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Error("Expected gzip compression for large error response")
	}
}

// =============================================================================
// GzipCompress — Empty Body (e.g., 204 No Content)
// =============================================================================

func TestGzipMiddleware_EmptyBody(t *testing.T) {
	handler := GzipCompress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest("DELETE", "/api/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", rec.Code)
	}

	if rec.Header().Get("Content-Encoding") == "gzip" {
		t.Error("Should NOT set Content-Encoding for empty body")
	}

	if rec.Body.Len() != 0 {
		t.Errorf("Expected empty body, got %d bytes", rec.Body.Len())
	}
}

// =============================================================================
// GzipCompress — JSON with charset
// =============================================================================

func TestGzipMiddleware_CompressesJSONWithCharset(t *testing.T) {
	largeJSON := `{"data":"` + strings.Repeat("abcdefghij", 200) + `"}`

	handler := GzipCompress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write([]byte(largeJSON))
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Error("Expected gzip compression for application/json; charset=utf-8")
	}
}

// =============================================================================
// GzipCompress — Pre-Compressed (Content-Encoding already set)
// =============================================================================

func TestGzipMiddleware_PassthroughPreCompressed(t *testing.T) {
	// Simulate a handler that serves pre-compressed data (e.g., cached response)
	preCompressed := []byte{0x1f, 0x8b, 0x08, 0x00} // gzip magic bytes (partial)

	handler := GzipCompress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Encoding", "gzip")
		w.Write(preCompressed)
	}))

	req := httptest.NewRequest("GET", "/api/schema", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	// Content-Encoding should remain "gzip" (passthrough, not double-compressed)
	if resp.Header.Get("Content-Encoding") != "gzip" {
		t.Errorf("Expected Content-Encoding: gzip passthrough, got %q", resp.Header.Get("Content-Encoding"))
	}

	// Body should be exactly the pre-compressed bytes (not re-compressed)
	body, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(body, preCompressed) {
		t.Error("Pre-compressed data should pass through unchanged")
	}
}

// =============================================================================
// isCompressibleContentType
// =============================================================================

func TestIsCompressibleContentType(t *testing.T) {
	tests := []struct {
		ct   string
		want bool
	}{
		{"application/json", true},
		{"application/json; charset=utf-8", true},
		{"text/plain", true},
		{"text/html", true},
		{"text/event-stream", false},
		{"application/octet-stream", false},
		{"application/zip", false},
		{"image/png", false},
		{"model/gltf-binary", false},
		{"", false},
	}

	for _, tt := range tests {
		got := isCompressibleContentType(tt.ct)
		if got != tt.want {
			t.Errorf("isCompressibleContentType(%q) = %v, want %v", tt.ct, got, tt.want)
		}
	}
}
