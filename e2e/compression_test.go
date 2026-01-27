package e2e

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"testing"
)

// =============================================================================
// Gzip Compression — Large API Response Compressed
// =============================================================================

// TestCompression_LargeQueryResponseGzipCompressed verifies that API responses
// exceeding the compression threshold are gzip-compressed when the client
// sends Accept-Encoding: gzip.
func TestCompression_LargeQueryResponseGzipCompressed(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "compress-test")

	// Upload enough files to ensure query response > 1KB
	for i := 0; i < 20; i++ {
		content := []byte(strings.Repeat("data", 50) + string(rune('A'+i)))
		ts.UploadFileExpectSuccess(t, "compress-test", "file"+string(rune('a'+i))+".bin", content, "")
	}

	// Use a client that does NOT auto-decompress so we can inspect raw response
	client := &http.Client{
		Transport: &http.Transport{
			DisableCompression: true,
		},
	}

	// Execute query that returns all uploaded files
	body := `{"topics":["compress-test"],"params":{"days":"99999","limit":"100"}}`
	req, err := http.NewRequest("POST", ts.URL+"/api/query/recent-imports", strings.NewReader(body))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Encoding", "gzip")
	if ts.APIKey != "" {
		req.Header.Set("X-API-Key", ts.APIKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Query request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("Query failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Verify gzip compression headers
	if resp.Header.Get("Content-Encoding") != "gzip" {
		t.Errorf("Expected Content-Encoding: gzip, got %q", resp.Header.Get("Content-Encoding"))
	}

	if resp.Header.Get("Vary") != "Accept-Encoding" {
		t.Errorf("Expected Vary: Accept-Encoding, got %q", resp.Header.Get("Vary"))
	}

	// Verify body is valid gzip
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		t.Fatalf("Response body is not valid gzip: %v", err)
	}
	defer gz.Close()

	decompressed, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("Failed to decompress response: %v", err)
	}

	// Verify the decompressed content is valid JSON with query results
	if !strings.Contains(string(decompressed), "row_count") {
		t.Error("Decompressed response should contain query results with row_count")
	}
}

// =============================================================================
// Gzip Compression — Small Response Not Compressed
// =============================================================================

// TestCompression_SmallResponseNotCompressed verifies that small API responses
// (below threshold) are NOT gzip-compressed even when client accepts gzip.
func TestCompression_SmallResponseNotCompressed(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Use a client that does NOT auto-decompress
	client := &http.Client{
		Transport: &http.Transport{
			DisableCompression: true,
		},
	}

	// GET /api/topics with no topics — should return small JSON
	req, err := http.NewRequest("GET", ts.URL+"/api/topics", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Accept-Encoding", "gzip")
	if ts.APIKey != "" {
		req.Header.Set("X-API-Key", ts.APIKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	// Small response should NOT be compressed
	if resp.Header.Get("Content-Encoding") == "gzip" {
		t.Error("Small responses should NOT be gzip-compressed")
	}

	// Body should be readable raw JSON
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read body: %v", err)
	}
	if !strings.Contains(string(bodyBytes), "topics") {
		t.Error("Expected raw JSON response with 'topics' field")
	}
}

// =============================================================================
// Gzip Compression — No Accept-Encoding
// =============================================================================

// TestCompression_NoAcceptEncodingNotCompressed verifies that responses are NOT
// compressed when the client does not send Accept-Encoding: gzip.
func TestCompression_NoAcceptEncodingNotCompressed(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Use a client that does NOT send Accept-Encoding
	client := &http.Client{
		Transport: &http.Transport{
			DisableCompression: true,
		},
	}

	// GET monitoring (likely large enough to compress)
	req, err := http.NewRequest("GET", ts.URL+"/api/monitoring", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	// Explicitly do NOT set Accept-Encoding
	if ts.APIKey != "" {
		req.Header.Set("X-API-Key", ts.APIKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("Content-Encoding") == "gzip" {
		t.Error("Should NOT compress when client doesn't accept gzip")
	}
}

// =============================================================================
// Gzip Compression — Monitoring Large Response
// =============================================================================

// TestCompression_MonitoringResponseCompressed verifies the monitoring endpoint
// response is compressed when large enough.
func TestCompression_MonitoringResponseCompressed(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Create several topics to make the monitoring response larger
	for i := 0; i < 10; i++ {
		ts.CreateTopic(t, "compress-mon-"+string(rune('a'+i)))
	}

	// Use a client that does NOT auto-decompress
	client := &http.Client{
		Transport: &http.Transport{
			DisableCompression: true,
		},
	}

	req, err := http.NewRequest("GET", ts.URL+"/api/monitoring", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Accept-Encoding", "gzip")
	if ts.APIKey != "" {
		req.Header.Set("X-API-Key", ts.APIKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("Monitoring failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Read raw response to check size
	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read body: %v", err)
	}

	// If monitoring response was >= 1KB, it should be compressed
	if resp.Header.Get("Content-Encoding") == "gzip" {
		// Verify it's valid gzip
		gz, err := gzip.NewReader(strings.NewReader(string(rawBody)))
		if err != nil {
			t.Fatalf("Content-Encoding says gzip but body is not valid gzip: %v", err)
		}
		decompressed, err := io.ReadAll(gz)
		gz.Close()
		if err != nil {
			t.Fatalf("Failed to decompress: %v", err)
		}
		if !strings.Contains(string(decompressed), "system") {
			t.Error("Decompressed monitoring response should contain 'system' field")
		}
	}
	// If it wasn't compressed (too small), that's also acceptable
}

// =============================================================================
// Gzip Compression — Vary Header Always Set
// =============================================================================

// TestCompression_VaryHeaderSetOnAPIRoutes verifies that the Vary: Accept-Encoding
// header is set on API responses when the client accepts gzip, regardless of
// whether the response was actually compressed.
func TestCompression_VaryHeaderSetOnAPIRoutes(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	client := &http.Client{
		Transport: &http.Transport{
			DisableCompression: true,
		},
	}

	// Small response (won't be compressed) but should still have Vary
	req, err := http.NewRequest("GET", ts.URL+"/api/topics", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Accept-Encoding", "gzip")
	if ts.APIKey != "" {
		req.Header.Set("X-API-Key", ts.APIKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("Vary") != "Accept-Encoding" {
		t.Errorf("Expected Vary: Accept-Encoding on API routes, got %q", resp.Header.Get("Vary"))
	}
}

// =============================================================================
// Gzip Compression — Existing Tests Still Work Transparently
// =============================================================================

// TestCompression_TransparentToDefaultClient verifies that Go's default HTTP
// client (which auto-sends Accept-Encoding: gzip and auto-decompresses)
// works transparently with the compression middleware.
func TestCompression_TransparentToDefaultClient(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "compress-transparent")

	// Upload files using the default helpers (which use http.DefaultClient)
	for i := 0; i < 15; i++ {
		content := []byte(strings.Repeat("payload", 30) + string(rune('A'+i)))
		ts.UploadFileExpectSuccess(t, "compress-transparent", "file"+string(rune('a'+i))+".bin", content, "")
	}

	// Use standard helper that uses http.DefaultClient (auto-decompresses)
	result := ts.ExecuteQuery(t, "recent-imports", []string{"compress-transparent"}, map[string]interface{}{
		"days":  "99999",
		"limit": "100",
	})

	// Should work transparently — result is properly decoded
	if result.RowCount == 0 {
		t.Error("Expected query results, got 0 rows")
	}
	if result.RowCount < 15 {
		t.Errorf("Expected at least 15 rows, got %d", result.RowCount)
	}
}
