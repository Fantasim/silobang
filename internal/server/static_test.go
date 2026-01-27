package server

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"silobang/internal/constants"
	"silobang/internal/logger"
)

// newTestServerWithFS creates a minimal Server with a mock embedded filesystem.
func newTestServerWithFS(webFS fs.FS) *Server {
	return &Server{
		webFS:  webFS,
		logger: logger.NewLogger(logger.LevelError),
	}
}

// =============================================================================
// Brotli Preferred Over Gzip
// =============================================================================

func TestStaticCompression_BrotliPreferredOverGzip(t *testing.T) {
	mockFS := fstest.MapFS{
		"assets/index-abc123.js":    {Data: []byte("/* original JS content */")},
		"assets/index-abc123.js.br": {Data: []byte("br-compressed-data")},
		"assets/index-abc123.js.gz": {Data: []byte("gz-compressed-data")},
	}

	s := newTestServerWithFS(mockFS)
	req := httptest.NewRequest("GET", "/assets/index-abc123.js", nil)
	req.Header.Set("Accept-Encoding", "br, gzip, deflate")
	rec := httptest.NewRecorder()

	s.serveStaticWithCompression(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Content-Encoding") != "br" {
		t.Errorf("Expected Content-Encoding: br, got %q", resp.Header.Get("Content-Encoding"))
	}
	if resp.Header.Get("Vary") != "Accept-Encoding" {
		t.Errorf("Expected Vary: Accept-Encoding, got %q", resp.Header.Get("Vary"))
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "javascript") {
		t.Errorf("Expected JavaScript content type, got %q", resp.Header.Get("Content-Type"))
	}
}

// =============================================================================
// Gzip Fallback When Brotli Not Accepted
// =============================================================================

func TestStaticCompression_GzipFallbackWhenBrotliNotAccepted(t *testing.T) {
	mockFS := fstest.MapFS{
		"assets/index-abc123.js":    {Data: []byte("/* original JS */")},
		"assets/index-abc123.js.br": {Data: []byte("br-compressed-data")},
		"assets/index-abc123.js.gz": {Data: []byte("gz-compressed-data")},
	}

	s := newTestServerWithFS(mockFS)
	req := httptest.NewRequest("GET", "/assets/index-abc123.js", nil)
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	rec := httptest.NewRecorder()

	s.serveStaticWithCompression(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.Header.Get("Content-Encoding") != "gzip" {
		t.Errorf("Expected Content-Encoding: gzip, got %q", resp.Header.Get("Content-Encoding"))
	}
	if rec.Body.String() != "gz-compressed-data" {
		t.Errorf("Expected gz-compressed-data, got %q", rec.Body.String())
	}
}

// =============================================================================
// Uncompressed Fallback When No Accept-Encoding
// =============================================================================

func TestStaticCompression_UncompressedWhenNoAcceptEncoding(t *testing.T) {
	originalContent := "/* original uncompressed JS */"
	mockFS := fstest.MapFS{
		"assets/index-abc123.js":    {Data: []byte(originalContent)},
		"assets/index-abc123.js.br": {Data: []byte("br-data")},
		"assets/index-abc123.js.gz": {Data: []byte("gz-data")},
	}

	s := newTestServerWithFS(mockFS)
	req := httptest.NewRequest("GET", "/assets/index-abc123.js", nil)
	// No Accept-Encoding header
	rec := httptest.NewRecorder()

	s.serveStaticWithCompression(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.Header.Get("Content-Encoding") != "" {
		t.Errorf("Expected no Content-Encoding, got %q", resp.Header.Get("Content-Encoding"))
	}
	if rec.Body.String() != originalContent {
		t.Errorf("Expected original content, got %q", rec.Body.String())
	}
}

// =============================================================================
// Uncompressed Fallback When No Compressed Variants Exist
// =============================================================================

func TestStaticCompression_UncompressedWhenNoVariantsExist(t *testing.T) {
	originalContent := "/* only original file, no .br/.gz */"
	mockFS := fstest.MapFS{
		"assets/index-abc123.js": {Data: []byte(originalContent)},
	}

	s := newTestServerWithFS(mockFS)
	req := httptest.NewRequest("GET", "/assets/index-abc123.js", nil)
	req.Header.Set("Accept-Encoding", "br, gzip")
	rec := httptest.NewRecorder()

	s.serveStaticWithCompression(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.Header.Get("Content-Encoding") != "" {
		t.Errorf("Expected no Content-Encoding, got %q", resp.Header.Get("Content-Encoding"))
	}
	if rec.Body.String() != originalContent {
		t.Errorf("Expected original content, got %q", rec.Body.String())
	}
}

// =============================================================================
// Hashed Assets Get Immutable Cache-Control
// =============================================================================

func TestStaticCompression_HashedAssetsGetImmutableCacheControl(t *testing.T) {
	mockFS := fstest.MapFS{
		"assets/index-abc123.js": {Data: []byte("js")},
	}

	s := newTestServerWithFS(mockFS)
	req := httptest.NewRequest("GET", "/assets/index-abc123.js", nil)
	rec := httptest.NewRecorder()

	s.serveStaticWithCompression(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.Header.Get("Cache-Control") != constants.CacheControlStaticHash {
		t.Errorf("Expected Cache-Control %q for hashed asset, got %q",
			constants.CacheControlStaticHash, resp.Header.Get("Cache-Control"))
	}
}

// =============================================================================
// index.html Gets no-cache Cache-Control
// =============================================================================

func TestStaticCompression_IndexHTMLGetsNoCacheCacheControl(t *testing.T) {
	mockFS := fstest.MapFS{
		"index.html": {Data: []byte("<html></html>")},
	}

	s := newTestServerWithFS(mockFS)
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	s.serveStaticWithCompression(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.Header.Get("Cache-Control") != constants.CacheControlNoCache {
		t.Errorf("Expected Cache-Control %q for index.html, got %q",
			constants.CacheControlNoCache, resp.Header.Get("Cache-Control"))
	}
}

// =============================================================================
// SPA Fallback — Unknown Path Serves index.html
// =============================================================================

func TestStaticCompression_SPAFallbackServesIndexHTML(t *testing.T) {
	indexContent := "<html><body>SPA</body></html>"
	mockFS := fstest.MapFS{
		"index.html": {Data: []byte(indexContent)},
	}

	s := newTestServerWithFS(mockFS)
	req := httptest.NewRequest("GET", "/topics/my-topic", nil)
	rec := httptest.NewRecorder()

	s.serveStaticWithCompression(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 for SPA route, got %d", resp.StatusCode)
	}
	if rec.Body.String() != indexContent {
		t.Errorf("Expected index.html content for SPA route, got %q", rec.Body.String())
	}
	if resp.Header.Get("Cache-Control") != constants.CacheControlNoCache {
		t.Errorf("Expected no-cache for SPA fallback, got %q", resp.Header.Get("Cache-Control"))
	}
}

// =============================================================================
// SPA Fallback With Compression
// =============================================================================

func TestStaticCompression_SPAFallbackWithBrotli(t *testing.T) {
	mockFS := fstest.MapFS{
		"index.html":    {Data: []byte("<html>raw</html>")},
		"index.html.br": {Data: []byte("br-index")},
	}

	s := newTestServerWithFS(mockFS)
	req := httptest.NewRequest("GET", "/monitoring", nil)
	req.Header.Set("Accept-Encoding", "br, gzip")
	rec := httptest.NewRecorder()

	s.serveStaticWithCompression(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.Header.Get("Content-Encoding") != "br" {
		t.Errorf("Expected Content-Encoding: br for SPA fallback, got %q", resp.Header.Get("Content-Encoding"))
	}
	if rec.Body.String() != "br-index" {
		t.Errorf("Expected br-compressed index.html, got %q", rec.Body.String())
	}
}

// =============================================================================
// CSS Content-Type Detected Correctly
// =============================================================================

func TestStaticCompression_CSSContentType(t *testing.T) {
	mockFS := fstest.MapFS{
		"assets/style-def456.css":    {Data: []byte("body{}")},
		"assets/style-def456.css.br": {Data: []byte("br-css")},
	}

	s := newTestServerWithFS(mockFS)
	req := httptest.NewRequest("GET", "/assets/style-def456.css", nil)
	req.Header.Set("Accept-Encoding", "br")
	rec := httptest.NewRecorder()

	s.serveStaticWithCompression(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if !strings.Contains(resp.Header.Get("Content-Type"), "css") {
		t.Errorf("Expected CSS content type, got %q", resp.Header.Get("Content-Type"))
	}
	if resp.Header.Get("Content-Encoding") != "br" {
		t.Errorf("Expected Content-Encoding: br, got %q", resp.Header.Get("Content-Encoding"))
	}
}

// =============================================================================
// Non-Asset Root File Gets no-cache
// =============================================================================

func TestStaticCompression_FaviconGetsNoCache(t *testing.T) {
	mockFS := fstest.MapFS{
		"favicon.ico": {Data: []byte("icon-data")},
	}

	s := newTestServerWithFS(mockFS)
	req := httptest.NewRequest("GET", "/favicon.ico", nil)
	rec := httptest.NewRecorder()

	s.serveStaticWithCompression(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.Header.Get("Cache-Control") != constants.CacheControlNoCache {
		t.Errorf("Expected no-cache for non-hashed file, got %q", resp.Header.Get("Cache-Control"))
	}
}

// =============================================================================
// 404 When File Not Found And No index.html Fallback
// =============================================================================

func TestStaticCompression_404WhenNoIndexHTML(t *testing.T) {
	mockFS := fstest.MapFS{
		// Empty FS — no index.html
	}

	s := newTestServerWithFS(mockFS)
	req := httptest.NewRequest("GET", "/anything", nil)
	rec := httptest.NewRecorder()

	s.serveStaticWithCompression(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404 when no files exist, got %d", resp.StatusCode)
	}
}

// =============================================================================
// Only Brotli Variant Exists (No Gzip)
// =============================================================================

func TestStaticCompression_OnlyBrotliVariantExists(t *testing.T) {
	mockFS := fstest.MapFS{
		"assets/app-xyz.js":    {Data: []byte("original")},
		"assets/app-xyz.js.br": {Data: []byte("br-only")},
	}

	s := newTestServerWithFS(mockFS)
	req := httptest.NewRequest("GET", "/assets/app-xyz.js", nil)
	req.Header.Set("Accept-Encoding", "gzip") // Only accepts gzip
	rec := httptest.NewRecorder()

	s.serveStaticWithCompression(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	// Should fall back to uncompressed since no .gz exists
	if resp.Header.Get("Content-Encoding") != "" {
		t.Errorf("Expected no Content-Encoding (no .gz variant), got %q", resp.Header.Get("Content-Encoding"))
	}
	if rec.Body.String() != "original" {
		t.Errorf("Expected original content, got %q", rec.Body.String())
	}
}

// =============================================================================
// Vary Header Set On Compressed Responses
// =============================================================================

func TestStaticCompression_VaryHeaderSetOnCompressed(t *testing.T) {
	mockFS := fstest.MapFS{
		"assets/app.js":    {Data: []byte("js")},
		"assets/app.js.gz": {Data: []byte("gz-js")},
	}

	s := newTestServerWithFS(mockFS)
	req := httptest.NewRequest("GET", "/assets/app.js", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	s.serveStaticWithCompression(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.Header.Get("Vary") != "Accept-Encoding" {
		t.Errorf("Expected Vary: Accept-Encoding on compressed response, got %q", resp.Header.Get("Vary"))
	}
}

// =============================================================================
// Compressed Variant Cache-Control Matches Hashed Asset
// =============================================================================

func TestStaticCompression_CompressedVariantCacheControl(t *testing.T) {
	mockFS := fstest.MapFS{
		"assets/index-hash.js":    {Data: []byte("js")},
		"assets/index-hash.js.br": {Data: []byte("br")},
	}

	s := newTestServerWithFS(mockFS)
	req := httptest.NewRequest("GET", "/assets/index-hash.js", nil)
	req.Header.Set("Accept-Encoding", "br")
	rec := httptest.NewRecorder()

	s.serveStaticWithCompression(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.Header.Get("Cache-Control") != constants.CacheControlStaticHash {
		t.Errorf("Expected immutable cache for compressed hashed asset, got %q", resp.Header.Get("Cache-Control"))
	}
}

// =============================================================================
// ETag Header Set On All Responses
// =============================================================================

func TestStaticCompression_ETagSetOnUncompressedResponse(t *testing.T) {
	mockFS := fstest.MapFS{
		"index.html": {Data: []byte("<html>test</html>")},
	}

	s := newTestServerWithFS(mockFS)
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	s.serveStaticWithCompression(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	etag := resp.Header.Get("ETag")
	if etag == "" {
		t.Error("Expected ETag header on uncompressed response")
	}
	if !strings.HasPrefix(etag, `"`) || !strings.HasSuffix(etag, `"`) {
		t.Errorf("Expected quoted ETag, got %q", etag)
	}
}

func TestStaticCompression_ETagSetOnCompressedResponse(t *testing.T) {
	mockFS := fstest.MapFS{
		"assets/app.js":    {Data: []byte("js-content")},
		"assets/app.js.br": {Data: []byte("br-content")},
	}

	s := newTestServerWithFS(mockFS)
	req := httptest.NewRequest("GET", "/assets/app.js", nil)
	req.Header.Set("Accept-Encoding", "br")
	rec := httptest.NewRecorder()

	s.serveStaticWithCompression(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	etag := resp.Header.Get("ETag")
	if etag == "" {
		t.Error("Expected ETag header on compressed response")
	}
}

// =============================================================================
// Conditional Request Returns 304 Not Modified
// =============================================================================

func TestStaticCompression_ConditionalRequest_304(t *testing.T) {
	mockFS := fstest.MapFS{
		"index.html": {Data: []byte("<html>conditional</html>")},
	}

	s := newTestServerWithFS(mockFS)

	// First request: get the ETag
	req1 := httptest.NewRequest("GET", "/", nil)
	rec1 := httptest.NewRecorder()
	s.serveStaticWithCompression(rec1, req1)
	etag := rec1.Result().Header.Get("ETag")
	if etag == "" {
		t.Fatal("First request did not return ETag")
	}

	// Second request: conditional with If-None-Match
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()
	s.serveStaticWithCompression(rec2, req2)

	resp2 := rec2.Result()
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusNotModified {
		t.Errorf("Expected 304 Not Modified, got %d", resp2.StatusCode)
	}
	if rec2.Body.Len() != 0 {
		t.Errorf("Expected empty body for 304, got %d bytes", rec2.Body.Len())
	}
}

func TestStaticCompression_ConditionalRequest_CompressedVariant_304(t *testing.T) {
	mockFS := fstest.MapFS{
		"assets/app.js":    {Data: []byte("js")},
		"assets/app.js.br": {Data: []byte("br-js")},
	}

	s := newTestServerWithFS(mockFS)

	// First request: get the ETag
	req1 := httptest.NewRequest("GET", "/assets/app.js", nil)
	req1.Header.Set("Accept-Encoding", "br")
	rec1 := httptest.NewRecorder()
	s.serveStaticWithCompression(rec1, req1)
	etag := rec1.Result().Header.Get("ETag")
	if etag == "" {
		t.Fatal("First request did not return ETag")
	}

	// Second request: conditional
	req2 := httptest.NewRequest("GET", "/assets/app.js", nil)
	req2.Header.Set("Accept-Encoding", "br")
	req2.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()
	s.serveStaticWithCompression(rec2, req2)

	resp2 := rec2.Result()
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusNotModified {
		t.Errorf("Expected 304 for compressed variant, got %d", resp2.StatusCode)
	}
}

func TestStaticCompression_ConditionalRequest_DifferentETag_200(t *testing.T) {
	mockFS := fstest.MapFS{
		"index.html": {Data: []byte("<html>content</html>")},
	}

	s := newTestServerWithFS(mockFS)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("If-None-Match", `"stale-etag-value"`)
	rec := httptest.NewRecorder()
	s.serveStaticWithCompression(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 for non-matching ETag, got %d", resp.StatusCode)
	}
	if rec.Body.String() != "<html>content</html>" {
		t.Errorf("Expected full body for non-matching ETag")
	}
}
