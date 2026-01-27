package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"silobang/internal/constants"
)

// ====================
// Path Traversal Tests
// ====================

func TestPathTraversal_InFilename(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "security-test")

	// Test various path traversal attempts in filename
	pathTraversalPayloads := []string{
		"../../../etc/passwd",
		"..\\..\\..\\windows\\system32\\config",
		"....//....//....//etc/passwd",
		"..%2F..%2F..%2Fetc/passwd",
		"..%252F..%252F..%252Fetc/passwd",
		"..%c0%af..%c0%afetc/passwd",
		"%2e%2e%2f%2e%2e%2f%2e%2e%2fetc/passwd",
		"..././..././..././etc/passwd",
		"..;/..;/..;/etc/passwd",
	}

	for _, payload := range pathTraversalPayloads {
		t.Run("filename_"+sanitizeTestName(payload), func(t *testing.T) {
			content := []byte("test content for path traversal")

			resp, err := ts.UploadFile("security-test", payload, content, "")
			if err != nil {
				t.Fatalf("upload request failed: %v", err)
			}
			defer resp.Body.Close()

			// Should either succeed (with sanitized filename) or fail with error
			// Must NOT create files outside the topic directory
			if resp.StatusCode == 200 || resp.StatusCode == 201 {
				// If successful, verify the file went to the correct location
				var uploadResp UploadResponse
				json.NewDecoder(resp.Body).Decode(&uploadResp)

				// Download and verify Content-Disposition header is sanitized
				downloadResp, err := ts.GET("/api/assets/" + uploadResp.Hash + "/download")
				if err != nil {
					t.Fatalf("download request failed: %v", err)
				}
				defer downloadResp.Body.Close()

				cd := downloadResp.Header.Get(constants.HeaderContentDisposition)
				assertSafeContentDisposition(t, cd)

				// Verify content matches
				body, _ := io.ReadAll(downloadResp.Body)
				if !bytes.Equal(body, content) {
					t.Error("downloaded content doesn't match uploaded content")
				}
			}
			// If it failed, that's acceptable behavior for security
		})
	}
}

func TestPathTraversal_InTopicName(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Test path traversal in topic name during creation
	maliciousTopicNames := []string{
		"../other-topic",
		"..\\other-topic",
		"topic/../../../etc",
		"..",
		".",
		"topic/../../",
		"valid-topic/../../../etc/passwd",
	}

	for _, name := range maliciousTopicNames {
		t.Run("topic_"+sanitizeTestName(name), func(t *testing.T) {
			resp, err := ts.POST("/api/topics", map[string]string{"name": name})
			if err != nil {
				t.Fatalf("create topic request failed: %v", err)
			}
			defer resp.Body.Close()

			// Should fail with 400 Bad Request due to invalid topic name
			if resp.StatusCode == 200 || resp.StatusCode == 201 {
				t.Errorf("expected topic creation to fail for malicious name %q, but got status %d", name, resp.StatusCode)
			}
		})
	}
}

func TestPathTraversal_InUploadEndpoint(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "security-test")

	// Try accessing upload endpoint with path traversal in URL
	maliciousEndpoints := []string{
		"/api/topics/../../../etc/assets",
		"/api/topics/security-test/../../other/assets",
		"/api/topics/security-test/../../../etc/passwd/assets",
	}

	for _, endpoint := range maliciousEndpoints {
		t.Run("endpoint_"+sanitizeTestName(endpoint), func(t *testing.T) {
			var buf bytes.Buffer
			writer := multipart.NewWriter(&buf)
			part, _ := writer.CreateFormFile("file", "test.txt")
			part.Write([]byte("test content"))
			writer.Close()

			req, _ := http.NewRequest("POST", ts.URL+endpoint, &buf)
			req.Header.Set("Content-Type", writer.FormDataContentType())

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			// Should return 404 or 400, not 200
			if resp.StatusCode == 200 || resp.StatusCode == 201 {
				t.Errorf("expected request to fail for malicious endpoint %q, but got status %d", endpoint, resp.StatusCode)
			}
		})
	}
}

// ====================
// Null Byte Injection Tests
// ====================

func TestNullByteInjection_InFilename(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "security-test")

	// Test null byte injection in filename
	nullBytePayloads := []string{
		"file\x00.txt",
		"file.txt\x00.exe",
		"../\x00../etc/passwd",
		"test\x00/../../../etc/passwd",
	}

	for i, payload := range nullBytePayloads {
		t.Run("nullbyte_"+string(rune('a'+i)), func(t *testing.T) {
			content := []byte("test content")

			resp, err := ts.UploadFile("security-test", payload, content, "")
			if err != nil {
				t.Fatalf("upload request failed: %v", err)
			}
			defer resp.Body.Close()

			// Should either handle gracefully or reject
			// Must NOT allow path traversal via null byte
			if resp.StatusCode == 200 || resp.StatusCode == 201 {
				var uploadResp UploadResponse
				json.NewDecoder(resp.Body).Decode(&uploadResp)

				// Verify Content-Disposition header has no null bytes or traversal
				downloadResp, err := ts.GET("/api/assets/" + uploadResp.Hash + "/download")
				if err != nil {
					t.Fatalf("download request failed: %v", err)
				}
				defer downloadResp.Body.Close()

				cd := downloadResp.Header.Get(constants.HeaderContentDisposition)
				assertSafeContentDisposition(t, cd)

				if strings.Contains(cd, "\x00") {
					t.Errorf("Content-Disposition contains null byte: %q", cd)
				}
			}
		})
	}
}

// ====================
// SQL Injection Tests (via Query Params)
// ====================

func TestSQLInjection_InQueryParams(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "sql-test")

	// Upload a test file
	ts.UploadFileExpectSuccess(t, "sql-test", "test.txt", []byte("test content"), "")

	// SQL injection attempts in query parameters
	sqlInjectionPayloads := []string{
		"'; DROP TABLE assets; --",
		"1 OR 1=1",
		"1'; DELETE FROM assets WHERE 1=1; --",
		"1 UNION SELECT * FROM assets",
		"1; UPDATE assets SET hash='hacked'",
		"1 AND (SELECT COUNT(*) FROM assets) > 0",
		"' OR ''='",
	}

	for _, payload := range sqlInjectionPayloads {
		t.Run("sql_"+sanitizeTestName(payload), func(t *testing.T) {
			// Try SQL injection in query preset params
			resp, err := ts.POST("/api/query/recent-imports", map[string]interface{}{
				"params": map[string]interface{}{
					"limit": payload,
				},
			})
			if err != nil {
				t.Fatalf("query request failed: %v", err)
			}
			defer resp.Body.Close()

			// Should either return error or safe result (no crash, no data leak)
			// If it returns 200, verify it didn't affect data
			if resp.StatusCode == 200 {
				// Query should work but not return injected results
				topics := ts.GetTopics(t)
				for _, topic := range topics.Topics {
					if topic.Name == "sql-test" && !topic.Healthy {
						t.Error("SQL injection may have corrupted topic")
					}
				}
			}
		})
	}
}

func TestSQLInjection_InMetadataKey(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "sql-meta-test")

	// Upload a test file
	uploadResp := ts.UploadFileExpectSuccess(t, "sql-meta-test", "test.txt", []byte("test"), "")

	// SQL injection attempts in metadata key
	sqlInjectionKeys := []string{
		"key'; DROP TABLE metadata; --",
		"key' OR '1'='1",
		"key'; DELETE FROM metadata_log;--",
		"key' UNION SELECT * FROM metadata_log WHERE '1'='1",
	}

	for _, key := range sqlInjectionKeys {
		t.Run("meta_key_"+sanitizeTestName(key), func(t *testing.T) {
			resp, err := ts.POST("/api/assets/"+uploadResp.Hash+"/metadata", map[string]interface{}{
				"op":                "set",
				"key":               key,
				"value":             "test-value",
				"processor":         "test",
				"processor_version": "1.0",
			})
			if err != nil {
				t.Fatalf("metadata request failed: %v", err)
			}
			defer resp.Body.Close()

			// Should handle gracefully (either succeed with escaped key or fail validation)
		})
	}
}

// ====================
// XSS Tests (Metadata Values)
// ====================

func TestXSS_InMetadataValue(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "xss-test")

	// Upload a test file
	uploadResp := ts.UploadFileExpectSuccess(t, "xss-test", "test.txt", []byte("test"), "")

	// XSS payloads in metadata value
	xssPayloads := []string{
		"<script>alert('xss')</script>",
		"<img src=x onerror=alert('xss')>",
		"<svg onload=alert('xss')>",
		"javascript:alert('xss')",
		"<a href='javascript:alert(1)'>click</a>",
		"<body onload=alert('xss')>",
		"<iframe src='javascript:alert(1)'>",
		"\"><script>alert('xss')</script>",
	}

	for _, payload := range xssPayloads {
		t.Run("xss_"+sanitizeTestName(payload), func(t *testing.T) {
			// Store the XSS payload
			metaResp := ts.SetMetadata(t, uploadResp.Hash, "xss-test-key", payload)

			// Retrieve it and verify it's stored as-is (no execution context)
			retrievedMeta := ts.GetAssetMetadata(t, uploadResp.Hash)
			computed, ok := retrievedMeta["computed_metadata"].(map[string]interface{})
			if !ok {
				t.Fatal("computed_metadata not found")
			}

			storedValue, ok := computed["xss-test-key"]
			if !ok {
				t.Fatal("xss-test-key not found in metadata")
			}

			// The value should be stored verbatim (API returns JSON, no HTML context)
			if storedValue != payload {
				t.Errorf("XSS payload was modified: got %q, want %q", storedValue, payload)
			}

			// Verify log_id was returned (operation succeeded)
			if metaResp.LogID == 0 {
				t.Error("expected non-zero log_id")
			}
		})
	}
}

// ====================
// Large Payload Tests
// ====================

func TestOversizedJSONPayload(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Create a very large JSON payload (10MB+)
	largeValue := strings.Repeat("x", 10*1024*1024) // 10MB string
	payload := map[string]interface{}{
		"working_directory": largeValue,
	}

	jsonBody, _ := json.Marshal(payload)

	resp, err := ts.POSTRaw("/api/config", "application/json", jsonBody)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Should reject with appropriate error (413 or 400)
	if resp.StatusCode == 200 {
		t.Error("server accepted oversized payload")
	}
}

func TestDeeplyNestedJSON(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "nested-test")

	// Create deeply nested JSON structure
	depth := 100
	nested := make(map[string]interface{})
	current := nested
	for i := 0; i < depth; i++ {
		next := make(map[string]interface{})
		current["nested"] = next
		current = next
	}
	current["value"] = "deep"

	uploadResp := ts.UploadFileExpectSuccess(t, "nested-test", "test.txt", []byte("test"), "")

	// Try to set deeply nested value as metadata
	resp, err := ts.POST("/api/assets/"+uploadResp.Hash+"/metadata", map[string]interface{}{
		"op":                "set",
		"key":               "nested-key",
		"value":             nested, // This should fail as it's not a simple type
		"processor":         "test",
		"processor_version": "1.0",
	})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Should fail because nested objects are not allowed as metadata values
	if resp.StatusCode == 200 {
		t.Error("server accepted deeply nested JSON as metadata value")
	}
}

// ====================
// Hash Validation Tests
// ====================

func TestInvalidHashFormat(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	invalidHashes := []string{
		"",                               // empty
		"abc",                            // too short
		strings.Repeat("x", 63),          // one char too short
		strings.Repeat("x", 65),          // one char too long
		strings.Repeat("g", 64),          // invalid hex chars
		strings.Repeat("X", 64),          // uppercase (if not normalized)
		"<script>alert(1)</script>",      // XSS attempt
		"'; DROP TABLE assets; --",       // SQL injection
		strings.Repeat("a", 64) + "\x00", // null byte
	}

	for _, hash := range invalidHashes {
		t.Run("hash_"+sanitizeTestName(hash), func(t *testing.T) {
			// Try to download with invalid hash
			resp, err := ts.GET("/api/assets/" + hash + "/download")
			if err != nil {
				return // Network error is acceptable
			}
			defer resp.Body.Close()

			// Should return 400 or 404, not 500
			if resp.StatusCode == 500 {
				t.Errorf("server returned 500 for invalid hash %q", hash)
			}
		})
	}
}

// ====================
// Topic Name Validation Tests
// ====================

func TestInvalidTopicNames(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	invalidNames := []string{
		"",                          // empty
		"Topic",                     // uppercase
		"TOPIC",                     // all uppercase
		"topic with spaces",         // spaces
		"topic\ttab",                // tab character
		"topic\nnewline",            // newline
		"topic<script>",             // XSS attempt
		"topic'; DROP TABLE --",     // SQL injection
		strings.Repeat("a", 256),    // too long
		".hidden",                   // starts with dot (not allowed by regex)
		"topic/slash",               // contains slash
		"topic\\backslash",          // contains backslash
		"topic:colon",               // contains colon
		"topic*asterisk",            // contains asterisk
		"topic?question",            // contains question mark
		"topic\"quote",              // contains quote
		"topic<angle",               // contains angle bracket
		"topic>angle",               // contains angle bracket
		"topic|pipe",                // contains pipe
		string([]byte{0x00}),        // null byte
		string([]byte{0x7f}),        // DEL character
	}
	// Note: Topic names starting with - or _ are valid per regex ^[a-z0-9_-]+$

	for _, name := range invalidNames {
		t.Run("name_"+sanitizeTestName(name), func(t *testing.T) {
			resp, err := ts.POST("/api/topics", map[string]string{"name": name})
			if err != nil {
				return // Network error is acceptable
			}
			defer resp.Body.Close()

			// Should reject invalid names
			if resp.StatusCode == 200 || resp.StatusCode == 201 {
				t.Errorf("server accepted invalid topic name %q", name)
			}
		})
	}
}

// ====================
// Metadata Key/Value Length Tests
// ====================

func TestMetadataKeyTooLong(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "meta-test")

	uploadResp := ts.UploadFileExpectSuccess(t, "meta-test", "test.txt", []byte("test"), "")

	// Key exceeding max length
	longKey := strings.Repeat("k", constants.MaxMetadataKeyLength+1)

	errResp := ts.SetMetadataExpectError(t, uploadResp.Hash, longKey, "value", 400)

	if errResp.Code != constants.ErrCodeMetadataKeyTooLong {
		t.Errorf("expected error code %q, got %q", constants.ErrCodeMetadataKeyTooLong, errResp.Code)
	}
}

func TestMetadataValueTooLong(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "meta-test")

	uploadResp := ts.UploadFileExpectSuccess(t, "meta-test", "test.txt", []byte("test"), "")

	// Value exceeding max length (10MB + 1)
	longValue := strings.Repeat("v", ts.App.Config.Metadata.MaxValueBytes+1)

	errResp := ts.SetMetadataExpectError(t, uploadResp.Hash, "key", longValue, 400)

	if errResp.Code != constants.ErrCodeMetadataValueTooLong {
		t.Errorf("expected error code %q, got %q", constants.ErrCodeMetadataValueTooLong, errResp.Code)
	}
}

// ====================
// Request Smuggling Tests
// ====================

func TestHTTPRequestSmuggling(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Attempt Content-Length / Transfer-Encoding smuggling
	smugglingPayloads := []struct {
		name        string
		contentType string
		body        string
	}{
		{
			"cl_te_smuggle",
			"application/json",
			`{"working_directory": "/tmp"}` + "\r\n0\r\n\r\nGET /secret HTTP/1.1\r\n",
		},
	}

	for _, payload := range smugglingPayloads {
		t.Run(payload.name, func(t *testing.T) {
			req, err := http.NewRequest("POST", ts.URL+"/api/config", strings.NewReader(payload.body))
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}
			req.Header.Set("Content-Type", payload.contentType)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				// Connection error is acceptable
				return
			}
			defer resp.Body.Close()

			// Server should handle gracefully
			if resp.StatusCode == 500 {
				t.Error("server crashed on smuggling attempt")
			}
		})
	}
}

// ====================
// Content-Type Validation Tests
// ====================

func TestInvalidContentType(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	invalidContentTypes := []string{
		"",
		"text/plain",
		"application/xml",
		"text/html",
		"application/x-www-form-urlencoded",
		"image/png",
		"application/json; charset=utf-8; boundary=something",
	}

	for _, ct := range invalidContentTypes {
		t.Run("ct_"+sanitizeTestName(ct), func(t *testing.T) {
			req, _ := http.NewRequest("POST", ts.URL+"/api/config", strings.NewReader(`{"working_directory": "/tmp"}`))
			if ct != "" {
				req.Header.Set("Content-Type", ct)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return
			}
			defer resp.Body.Close()

			// Some content types should be rejected, others may be lenient
			// Main thing is it shouldn't crash
		})
	}
}

// ====================
// Helper Functions
// ====================

// assertSafeContentDisposition verifies that a Content-Disposition header value
// does not contain path traversal sequences, directory separators, or control characters.
func assertSafeContentDisposition(t *testing.T, cd string) {
	t.Helper()
	if strings.Contains(cd, "..") {
		t.Errorf("Content-Disposition contains path traversal: %s", cd)
	}
	if strings.Contains(cd, "/") || strings.Contains(cd, "\\") {
		t.Errorf("Content-Disposition contains directory separator: %s", cd)
	}
	for _, c := range cd {
		if c < 0x20 && c != ' ' {
			t.Errorf("Content-Disposition contains control character 0x%02x: %q", c, cd)
			break
		}
	}
}

// sanitizeTestName creates a safe test name from potentially dangerous input
func sanitizeTestName(s string) string {
	if len(s) > 30 {
		s = s[:30]
	}
	result := make([]byte, 0, len(s))
	for _, c := range []byte(s) {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			result = append(result, c)
		} else {
			result = append(result, '_')
		}
	}
	if len(result) == 0 {
		return "empty"
	}
	return string(result)
}
