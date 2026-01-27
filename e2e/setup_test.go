package e2e

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"silobang/internal/audit"
	"silobang/internal/config"
	"silobang/internal/constants"
	"silobang/internal/database"
	"silobang/internal/logger"
	"silobang/internal/queries"
	"silobang/internal/server"
)

// TestServer wraps a running silobang server for testing
type TestServer struct {
	Server    *httptest.Server
	App       *server.App
	WorkDir   string
	ConfigDir string
	URL       string
	APIKey    string // Bootstrap admin API key for authenticated requests
}

// StartTestServer creates a new test server with isolated directories
func StartTestServer(t *testing.T) *TestServer {
	t.Helper()

	// Create temp directories
	workDir, err := os.MkdirTemp("", "silobang-test-work-*")
	if err != nil {
		t.Fatalf("failed to create work dir: %v", err)
	}

	configDir, err := os.MkdirTemp("", "silobang-test-config-*")
	if err != nil {
		os.RemoveAll(workDir)
		t.Fatalf("failed to create config dir: %v", err)
	}

	// Create app instance with temp config
	cfg := &config.Config{
		WorkingDirectory: "", // Not configured initially
		Port:             0,  // Let httptest assign port
		MaxDatSize:       constants.DefaultMaxDatSize,
	}
	cfg.ApplyDefaults()

	log := logger.NewLogger(logger.LevelError) // Suppress logs in tests
	app := server.NewApp(cfg, log)

	// Load default queries
	app.QueriesConfig = queries.GetDefaultConfig()

	// Create HTTP server
	srv := server.NewServer(app, ":0", nil) // nil webFS for API-only testing
	httpServer := httptest.NewServer(srv.Handler())

	ts := &TestServer{
		Server:    httpServer,
		App:       app,
		WorkDir:   workDir,
		ConfigDir: configDir,
		URL:       httpServer.URL,
	}

	// Register cleanup
	t.Cleanup(func() {
		ts.Cleanup()
	})

	return ts
}

// Cleanup removes temp directories and closes connections
func (ts *TestServer) Cleanup() {
	if ts.Server != nil {
		ts.Server.Close()
	}
	if ts.App != nil {
		ts.App.CloseAllTopicDBs()
		if ts.App.OrchestratorDB != nil {
			ts.App.OrchestratorDB.Close()
		}
	}
	os.RemoveAll(ts.WorkDir)
	os.RemoveAll(ts.ConfigDir)
}

// Shutdown gracefully stops the server (for restart tests)
func (ts *TestServer) Shutdown() {
	if ts.Server != nil {
		ts.Server.Close()
		ts.Server = nil
	}
	if ts.App != nil {
		ts.App.CloseAllTopicDBs()
		if ts.App.OrchestratorDB != nil {
			ts.App.OrchestratorDB.Close()
			ts.App.OrchestratorDB = nil
		}
	}
}

// Restart creates a new server with same directories
func (ts *TestServer) Restart(t *testing.T) {
	t.Helper()
	ts.Shutdown()

	// Recreate app with same directories
	cfg := ts.App.Config
	log := logger.NewLogger(logger.LevelError)
	app := server.NewApp(cfg, log)
	app.QueriesConfig = ts.App.QueriesConfig

	// Reinitialize if working directory set
	if cfg.WorkingDirectory != "" {
		orchPath := filepath.Join(cfg.WorkingDirectory, constants.InternalDir, constants.OrchestratorDB)
		orchDB, err := database.InitOrchestratorDB(orchPath)
		if err != nil {
			t.Fatalf("failed to reopen orchestrator: %v", err)
		}
		app.OrchestratorDB = orchDB

		// Initialize audit logger
		app.AuditLogger = audit.NewLogger(orchDB, cfg.Audit.MaxLogSizeBytes, cfg.Audit.PurgePercentage)

		// Re-initialize services (including auth)
		app.ReinitServices()

		// Rediscover topics
		topics, _ := config.DiscoverTopics(cfg.WorkingDirectory)
		for _, topic := range topics {
			app.RegisterTopic(topic.Name, topic.Healthy, topic.Error)
		}
	}

	srv := server.NewServer(app, ":0", nil)
	httpServer := httptest.NewServer(srv.Handler())

	ts.Server = httpServer
	ts.App = app
	ts.URL = httpServer.URL
}

// Helper methods for API calls

func (ts *TestServer) GET(path string) (*http.Response, error) {
	req, err := http.NewRequest("GET", ts.URL+path, nil)
	if err != nil {
		return nil, err
	}
	if ts.APIKey != "" {
		req.Header.Set(constants.HeaderXAPIKey, ts.APIKey)
	}
	return http.DefaultClient.Do(req)
}

func (ts *TestServer) POST(path string, body interface{}) (*http.Response, error) {
	jsonBody, _ := json.Marshal(body)
	req, err := http.NewRequest("POST", ts.URL+path, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if ts.APIKey != "" {
		req.Header.Set(constants.HeaderXAPIKey, ts.APIKey)
	}
	return http.DefaultClient.Do(req)
}

func (ts *TestServer) PATCH(path string, body interface{}) (*http.Response, error) {
	jsonBody, _ := json.Marshal(body)
	req, err := http.NewRequest("PATCH", ts.URL+path, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if ts.APIKey != "" {
		req.Header.Set(constants.HeaderXAPIKey, ts.APIKey)
	}
	return http.DefaultClient.Do(req)
}

func (ts *TestServer) DELETE(path string) (*http.Response, error) {
	req, err := http.NewRequest("DELETE", ts.URL+path, nil)
	if err != nil {
		return nil, err
	}
	if ts.APIKey != "" {
		req.Header.Set(constants.HeaderXAPIKey, ts.APIKey)
	}
	return http.DefaultClient.Do(req)
}

// UnauthenticatedGET sends a GET request without any auth headers
func (ts *TestServer) UnauthenticatedGET(path string) (*http.Response, error) {
	return http.Get(ts.URL + path)
}

// UnauthenticatedPOST sends a POST request without any auth headers
func (ts *TestServer) UnauthenticatedPOST(path string, body interface{}) (*http.Response, error) {
	jsonBody, _ := json.Marshal(body)
	return http.Post(ts.URL+path, "application/json", bytes.NewReader(jsonBody))
}

// RequestWithAPIKey sends a request using a specific API key (for testing restricted users)
func (ts *TestServer) RequestWithAPIKey(method, path, apiKey string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, _ := json.Marshal(body)
		reqBody = bytes.NewReader(jsonBody)
	}
	req, err := http.NewRequest(method, ts.URL+path, reqBody)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set(constants.HeaderXAPIKey, apiKey)
	return http.DefaultClient.Do(req)
}

func (ts *TestServer) GetJSON(path string, target interface{}) error {
	resp, err := ts.GET(path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(target)
}

func (ts *TestServer) PostJSON(path string, body, target interface{}) error {
	resp, err := ts.POST(path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if target != nil {
		return json.NewDecoder(resp.Body).Decode(target)
	}
	return nil
}

func (ts *TestServer) UploadFile(topicName, filename string, content []byte, parentID string) (*http.Response, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, err
	}
	part.Write(content)

	if parentID != "" {
		writer.WriteField("parent_id", parentID)
	}

	writer.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/api/topics/"+topicName+"/assets", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if ts.APIKey != "" {
		req.Header.Set(constants.HeaderXAPIKey, ts.APIKey)
	}

	return http.DefaultClient.Do(req)
}

// ConfigureWorkDir sets the working directory via API and captures bootstrap credentials
func (ts *TestServer) ConfigureWorkDir(t *testing.T) {
	t.Helper()
	resp, err := ts.POST("/api/config", map[string]interface{}{
		"working_directory": ts.WorkDir,
	})
	if err != nil {
		t.Fatalf("failed to configure work dir: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read config response: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("config failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response to capture bootstrap credentials
	var configResp struct {
		Bootstrap *struct {
			Username string `json:"username"`
			Password string `json:"password"`
			APIKey   string `json:"api_key"`
		} `json:"bootstrap"`
	}
	if err := json.Unmarshal(bodyBytes, &configResp); err == nil && configResp.Bootstrap != nil {
		ts.APIKey = configResp.Bootstrap.APIKey
	}
}

// CreateTopic creates a topic via API
func (ts *TestServer) CreateTopic(t *testing.T, name string) {
	t.Helper()
	resp, err := ts.POST("/api/topics", map[string]string{"name": name})
	if err != nil {
		t.Fatalf("failed to create topic: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("create topic failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}
}

// GetTopicDB opens a direct connection to topic database for verification
func (ts *TestServer) GetTopicDB(t *testing.T, topicName string) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(ts.WorkDir, topicName, constants.InternalDir, topicName+".db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open topic db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// GetOrchestratorDB opens direct connection to orchestrator database
func (ts *TestServer) GetOrchestratorDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(ts.WorkDir, constants.InternalDir, constants.OrchestratorDB)
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open orchestrator db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// UploadFileExpectSuccess uploads and returns parsed response, fails test on error
func (ts *TestServer) UploadFileExpectSuccess(t *testing.T, topic, filename string, content []byte, parentID string) UploadResponse {
	t.Helper()
	resp, err := ts.UploadFile(topic, filename, content, parentID)
	if err != nil {
		t.Fatalf("upload request failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read upload response: %v", err)
	}

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		t.Fatalf("upload failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var uploadResp UploadResponse
	if err := json.Unmarshal(bodyBytes, &uploadResp); err != nil {
		t.Fatalf("failed to parse upload response: %v", err)
	}
	return uploadResp
}

// UploadFileExpectError uploads and expects specific status code
func (ts *TestServer) UploadFileExpectError(t *testing.T, topic, filename string, content []byte, parentID string, expectedStatus int) ErrorResponse {
	t.Helper()
	resp, err := ts.UploadFile(topic, filename, content, parentID)
	if err != nil {
		t.Fatalf("upload request failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read upload response: %v", err)
	}

	if resp.StatusCode != expectedStatus {
		t.Fatalf("expected status %d, got %d: %s", expectedStatus, resp.StatusCode, string(bodyBytes))
	}

	var errResp ErrorResponse
	if err := json.Unmarshal(bodyBytes, &errResp); err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}
	return errResp
}

// ExecuteQuery runs query and returns parsed response
func (ts *TestServer) ExecuteQuery(t *testing.T, preset string, topics []string, params map[string]interface{}) QueryResponse {
	t.Helper()
	resp, err := ts.POST("/api/query/"+preset, map[string]interface{}{
		"topics": topics,
		"params": params,
	})
	if err != nil {
		t.Fatalf("query request failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read query response: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("query failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var queryResp QueryResponse
	if err := json.Unmarshal(bodyBytes, &queryResp); err != nil {
		t.Fatalf("failed to parse query response: %v", err)
	}
	return queryResp
}

// ExecuteQueryExpectError runs query expecting failure
func (ts *TestServer) ExecuteQueryExpectError(t *testing.T, preset string, topics []string, params map[string]interface{}, expectedStatus int) ErrorResponse {
	t.Helper()
	resp, err := ts.POST("/api/query/"+preset, map[string]interface{}{
		"topics": topics,
		"params": params,
	})
	if err != nil {
		t.Fatalf("query request failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read query response: %v", err)
	}

	if resp.StatusCode != expectedStatus {
		t.Fatalf("expected status %d, got %d: %s", expectedStatus, resp.StatusCode, string(bodyBytes))
	}

	var errResp ErrorResponse
	if err := json.Unmarshal(bodyBytes, &errResp); err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}
	return errResp
}

// GetTopics returns topic list
func (ts *TestServer) GetTopics(t *testing.T) TopicsResponse {
	t.Helper()
	resp, err := ts.GET("/api/topics")
	if err != nil {
		t.Fatalf("get topics request failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read topics response: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("get topics failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var topicsResp TopicsResponse
	if err := json.Unmarshal(bodyBytes, &topicsResp); err != nil {
		t.Fatalf("failed to parse topics response: %v", err)
	}
	return topicsResp
}

// SetMetadata sets metadata and returns response
func (ts *TestServer) SetMetadata(t *testing.T, hash, key string, value interface{}) MetadataResponse {
	t.Helper()
	resp, err := ts.POST("/api/assets/"+hash+"/metadata", map[string]interface{}{
		"op":                "set",
		"key":               key,
		"value":             value,
		"processor":         "test",
		"processor_version": "1.0",
	})
	if err != nil {
		t.Fatalf("set metadata request failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read metadata response: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("set metadata failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var metaResp MetadataResponse
	if err := json.Unmarshal(bodyBytes, &metaResp); err != nil {
		t.Fatalf("failed to parse metadata response: %v", err)
	}
	return metaResp
}

// DeleteMetadata deletes metadata key
func (ts *TestServer) DeleteMetadata(t *testing.T, hash, key string) MetadataResponse {
	t.Helper()
	resp, err := ts.POST("/api/assets/"+hash+"/metadata", map[string]interface{}{
		"op":                "delete",
		"key":               key,
		"processor":         "test",
		"processor_version": "1.0",
	})
	if err != nil {
		t.Fatalf("delete metadata request failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read metadata response: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("delete metadata failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var metaResp MetadataResponse
	if err := json.Unmarshal(bodyBytes, &metaResp); err != nil {
		t.Fatalf("failed to parse metadata response: %v", err)
	}
	return metaResp
}

// DownloadAsset downloads and returns bytes, fails on error
func (ts *TestServer) DownloadAsset(t *testing.T, hash string) []byte {
	t.Helper()
	resp, err := ts.GET("/api/assets/" + hash + "/download")
	if err != nil {
		t.Fatalf("download request failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read download response: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("download failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return bodyBytes
}

// DownloadAssetExpectError expects download to fail
func (ts *TestServer) DownloadAssetExpectError(t *testing.T, hash string, expectedStatus int) ErrorResponse {
	t.Helper()
	resp, err := ts.GET("/api/assets/" + hash + "/download")
	if err != nil {
		t.Fatalf("download request failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read download response: %v", err)
	}

	if resp.StatusCode != expectedStatus {
		t.Fatalf("expected status %d, got %d: %s", expectedStatus, resp.StatusCode, string(bodyBytes))
	}

	var errResp ErrorResponse
	if err := json.Unmarshal(bodyBytes, &errResp); err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}
	return errResp
}

// WaitFor polls a condition until it returns true or timeout is reached
func WaitFor(t *testing.T, timeout time.Duration, condition func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for: %s", msg)
}

// POSTRaw sends raw bytes as body (useful for invalid JSON tests)
func (ts *TestServer) POSTRaw(path string, contentType string, body []byte) (*http.Response, error) {
	req, err := http.NewRequest("POST", ts.URL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	if ts.APIKey != "" {
		req.Header.Set(constants.HeaderXAPIKey, ts.APIKey)
	}
	return http.DefaultClient.Do(req)
}

// SetMetadataExpectError sets metadata and expects specific error status
func (ts *TestServer) SetMetadataExpectError(t *testing.T, hash, key string, value interface{}, expectedStatus int) ErrorResponse {
	t.Helper()
	resp, err := ts.POST("/api/assets/"+hash+"/metadata", map[string]interface{}{
		"op":                "set",
		"key":               key,
		"value":             value,
		"processor":         "test",
		"processor_version": "1.0",
	})
	if err != nil {
		t.Fatalf("set metadata request failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read metadata response: %v", err)
	}

	if resp.StatusCode != expectedStatus {
		t.Fatalf("expected status %d, got %d: %s", expectedStatus, resp.StatusCode, string(bodyBytes))
	}

	var errResp ErrorResponse
	if err := json.Unmarshal(bodyBytes, &errResp); err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}
	return errResp
}

// BulkDownload performs a bulk download request and returns raw response
func (ts *TestServer) BulkDownload(t *testing.T, req BulkDownloadRequest) (*http.Response, error) {
	t.Helper()
	return ts.POST("/api/download/bulk", req)
}

// BulkDownloadExpectSuccess downloads and returns the ZIP bytes, fails test on error
func (ts *TestServer) BulkDownloadExpectSuccess(t *testing.T, req BulkDownloadRequest) []byte {
	t.Helper()
	resp, err := ts.BulkDownload(t, req)
	if err != nil {
		t.Fatalf("bulk download request failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read bulk download response: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("bulk download failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return bodyBytes
}

// BulkDownloadExpectError expects bulk download to fail with specific status
func (ts *TestServer) BulkDownloadExpectError(t *testing.T, req BulkDownloadRequest, expectedStatus int) ErrorResponse {
	t.Helper()
	resp, err := ts.BulkDownload(t, req)
	if err != nil {
		t.Fatalf("bulk download request failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read bulk download response: %v", err)
	}

	if resp.StatusCode != expectedStatus {
		t.Fatalf("expected status %d, got %d: %s", expectedStatus, resp.StatusCode, string(bodyBytes))
	}

	var errResp ErrorResponse
	if err := json.Unmarshal(bodyBytes, &errResp); err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}
	return errResp
}

// ExtractZIPManifest extracts and parses manifest.json from ZIP bytes
func ExtractZIPManifest(t *testing.T, zipBytes []byte) BulkDownloadManifest {
	t.Helper()
	manifestBytes := ExtractZIPFile(t, zipBytes, "manifest.json")

	var manifest BulkDownloadManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatalf("failed to parse manifest.json: %v", err)
	}
	return manifest
}

// ExtractZIPFile extracts a specific file from ZIP bytes
func ExtractZIPFile(t *testing.T, zipBytes []byte, filename string) []byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("failed to open zip: %v", err)
	}

	for _, file := range reader.File {
		if file.Name == filename {
			rc, err := file.Open()
			if err != nil {
				t.Fatalf("failed to open file %s in zip: %v", filename, err)
			}
			defer rc.Close()

			content, err := io.ReadAll(rc)
			if err != nil {
				t.Fatalf("failed to read file %s from zip: %v", filename, err)
			}
			return content
		}
	}

	t.Fatalf("file %s not found in zip", filename)
	return nil
}

// ListZIPFiles returns all filenames in ZIP
func ListZIPFiles(t *testing.T, zipBytes []byte) []string {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("failed to open zip: %v", err)
	}

	var files []string
	for _, file := range reader.File {
		files = append(files, file.Name)
	}
	return files
}

// ExtractAssetMetadata extracts and parses a metadata JSON file from ZIP
func ExtractAssetMetadata(t *testing.T, zipBytes []byte, metadataFilename string) AssetMetadataFile {
	t.Helper()
	path := "metadata/" + metadataFilename + ".json"
	content := ExtractZIPFile(t, zipBytes, path)

	var metadata AssetMetadataFile
	if err := json.Unmarshal(content, &metadata); err != nil {
		t.Fatalf("failed to parse metadata file %s: %v", path, err)
	}
	return metadata
}

// BulkDownloadSSE initiates SSE bulk download and returns response for reading events
func (ts *TestServer) BulkDownloadSSE(t *testing.T, mode, preset string, params map[string]interface{}, topics, assetIDs []string, includeMetadata bool, filenameFormat string) (*http.Response, error) {
	t.Helper()

	// Build query string
	query := "?mode=" + mode
	if preset != "" {
		query += "&preset=" + preset
	}
	if params != nil {
		paramsJSON, _ := json.Marshal(params)
		query += "&params=" + url.QueryEscape(string(paramsJSON))
	}
	if len(topics) > 0 {
		query += "&topics=" + joinStrings(topics, ",")
	}
	if len(assetIDs) > 0 {
		query += "&asset_ids=" + joinStrings(assetIDs, ",")
	}
	if includeMetadata {
		query += "&include_metadata=true"
	}
	if filenameFormat != "" {
		query += "&filename_format=" + filenameFormat
	}

	req, err := http.NewRequest("GET", ts.URL+"/api/download/bulk/start"+query, nil)
	if err != nil {
		return nil, err
	}
	if ts.APIKey != "" {
		req.Header.Set(constants.HeaderXAPIKey, ts.APIKey)
	}
	return http.DefaultClient.Do(req)
}

// ParseBulkDownloadSSEEvents parses SSE events from response body
func ParseBulkDownloadSSEEvents(t *testing.T, resp *http.Response) []BulkDownloadSSEEvent {
	t.Helper()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read SSE response: %v", err)
	}

	var events []BulkDownloadSSEEvent
	lines := bytes.Split(bodyBytes, []byte("\n"))

	for _, line := range lines {
		if bytes.HasPrefix(line, []byte("data: ")) {
			jsonData := bytes.TrimPrefix(line, []byte("data: "))
			var event BulkDownloadSSEEvent
			if err := json.Unmarshal(jsonData, &event); err != nil {
				t.Logf("failed to parse SSE event: %v (data: %s)", err, string(jsonData))
				continue
			}
			events = append(events, event)
		}
	}

	return events
}

// FindBulkDownloadSSEEvent finds first event of given type
func FindBulkDownloadSSEEvent(events []BulkDownloadSSEEvent, eventType string) *BulkDownloadSSEEvent {
	for i, event := range events {
		if event.Type == eventType {
			return &events[i]
		}
	}
	return nil
}

// FindAllBulkDownloadSSEEvents finds all events of given type
func FindAllBulkDownloadSSEEvents(events []BulkDownloadSSEEvent, eventType string) []BulkDownloadSSEEvent {
	var result []BulkDownloadSSEEvent
	for _, event := range events {
		if event.Type == eventType {
			result = append(result, event)
		}
	}
	return result
}

// FetchBulkDownloadZIP downloads the completed ZIP file by ID
func (ts *TestServer) FetchBulkDownloadZIP(t *testing.T, downloadID string) []byte {
	t.Helper()

	resp, err := ts.GET("/api/download/bulk/" + downloadID)
	if err != nil {
		t.Fatalf("failed to fetch bulk download: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read bulk download response: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("fetch bulk download failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return bodyBytes
}

// FetchBulkDownloadZIPExpectError expects fetch to fail with specific status
func (ts *TestServer) FetchBulkDownloadZIPExpectError(t *testing.T, downloadID string, expectedStatus int) ErrorResponse {
	t.Helper()

	resp, err := ts.GET("/api/download/bulk/" + downloadID)
	if err != nil {
		t.Fatalf("failed to fetch bulk download: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read bulk download response: %v", err)
	}

	if resp.StatusCode != expectedStatus {
		t.Fatalf("expected status %d, got %d: %s", expectedStatus, resp.StatusCode, string(bodyBytes))
	}

	var errResp ErrorResponse
	if err := json.Unmarshal(bodyBytes, &errResp); err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}
	return errResp
}

// GetDownloadIDFromEvents extracts download_id from complete event
func GetDownloadIDFromEvents(t *testing.T, events []BulkDownloadSSEEvent) string {
	t.Helper()

	completeEvent := FindBulkDownloadSSEEvent(events, "complete")
	if completeEvent == nil {
		t.Fatalf("no complete event found in SSE events")
	}

	downloadID, ok := completeEvent.Data["download_id"].(string)
	if !ok {
		t.Fatalf("download_id not found in complete event")
	}
	return downloadID
}

// joinStrings joins strings with separator
func joinStrings(strs []string, sep string) string {
	result := ""
	for i, s := range strs {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}

// BatchSetMetadata sends batch metadata request and returns response
func (ts *TestServer) BatchSetMetadata(t *testing.T, req BatchMetadataRequest) BatchMetadataResponse {
	t.Helper()
	resp, err := ts.POST("/api/metadata/batch", req)
	if err != nil {
		t.Fatalf("batch metadata request failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read batch metadata response: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("batch metadata failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var batchResp BatchMetadataResponse
	if err := json.Unmarshal(bodyBytes, &batchResp); err != nil {
		t.Fatalf("failed to parse batch metadata response: %v", err)
	}
	return batchResp
}

// BatchSetMetadataExpectError sends batch metadata request and expects error
func (ts *TestServer) BatchSetMetadataExpectError(t *testing.T, req BatchMetadataRequest, expectedStatus int) ErrorResponse {
	t.Helper()
	resp, err := ts.POST("/api/metadata/batch", req)
	if err != nil {
		t.Fatalf("batch metadata request failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read batch metadata response: %v", err)
	}

	if resp.StatusCode != expectedStatus {
		t.Fatalf("expected status %d, got %d: %s", expectedStatus, resp.StatusCode, string(bodyBytes))
	}

	var errResp ErrorResponse
	if err := json.Unmarshal(bodyBytes, &errResp); err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}
	return errResp
}

// ApplyMetadata sends apply metadata request and returns response
func (ts *TestServer) ApplyMetadata(t *testing.T, req ApplyMetadataRequest) BatchMetadataResponse {
	t.Helper()
	resp, err := ts.POST("/api/metadata/apply", req)
	if err != nil {
		t.Fatalf("apply metadata request failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read apply metadata response: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("apply metadata failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var applyResp BatchMetadataResponse
	if err := json.Unmarshal(bodyBytes, &applyResp); err != nil {
		t.Fatalf("failed to parse apply metadata response: %v", err)
	}
	return applyResp
}

// ApplyMetadataExpectError sends apply metadata request and expects error
func (ts *TestServer) ApplyMetadataExpectError(t *testing.T, req ApplyMetadataRequest, expectedStatus int) ErrorResponse {
	t.Helper()
	resp, err := ts.POST("/api/metadata/apply", req)
	if err != nil {
		t.Fatalf("apply metadata request failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read apply metadata response: %v", err)
	}

	if resp.StatusCode != expectedStatus {
		t.Fatalf("expected status %d, got %d: %s", expectedStatus, resp.StatusCode, string(bodyBytes))
	}

	var errResp ErrorResponse
	if err := json.Unmarshal(bodyBytes, &errResp); err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}
	return errResp
}

// GetAssetMetadata retrieves asset metadata via API
func (ts *TestServer) GetAssetMetadata(t *testing.T, hash string) map[string]interface{} {
	t.Helper()
	resp, err := ts.GET("/api/assets/" + hash + "/metadata")
	if err != nil {
		t.Fatalf("get metadata request failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read metadata response: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("get metadata failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		t.Fatalf("failed to parse metadata response: %v", err)
	}
	return result
}

// GetMonitoring retrieves monitoring data from the API
func (ts *TestServer) GetMonitoring(t *testing.T) MonitoringResponse {
	t.Helper()
	resp, err := ts.GET("/api/monitoring")
	if err != nil {
		t.Fatalf("monitoring request failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read monitoring response: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("monitoring failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var monResp MonitoringResponse
	if err := json.Unmarshal(bodyBytes, &monResp); err != nil {
		t.Fatalf("failed to parse monitoring response: %v", err)
	}
	return monResp
}

// GetMonitoringLogFile retrieves log file content from the monitoring API.
// Returns the response body as string and the HTTP status code.
func (ts *TestServer) GetMonitoringLogFile(t *testing.T, level, filename string) (string, int) {
	t.Helper()
	resp, err := ts.GET("/api/monitoring/logs/" + level + "/" + filename)
	if err != nil {
		t.Fatalf("log file request failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read log file response: %v", err)
	}

	return string(bodyBytes), resp.StatusCode
}

// RequestWithSessionToken sends a request using a session token (Bearer auth)
func (ts *TestServer) RequestWithSessionToken(method, path, token string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, _ := json.Marshal(body)
		reqBody = bytes.NewReader(jsonBody)
	}
	req, err := http.NewRequest(method, ts.URL+path, reqBody)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set(constants.HeaderAuthorization, constants.AuthBearerPrefix+token)
	return http.DefaultClient.Do(req)
}

// LoginUser logs in with username/password and returns the session token
func (ts *TestServer) LoginUser(t *testing.T, username, password string) string {
	t.Helper()
	resp, err := ts.UnauthenticatedPOST("/api/auth/login", map[string]string{
		"username": username,
		"password": password,
	})
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read login response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var loginResp struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(bodyBytes, &loginResp); err != nil {
		t.Fatalf("failed to parse login response: %v", err)
	}

	if loginResp.Token == "" {
		t.Fatal("login response contained empty token")
	}

	return loginResp.Token
}

// TestUserInfo holds credentials for a created test user
type TestUserInfo struct {
	ID       int64
	Username string
	APIKey   string
	Password string
}

// CreateTestUser creates a new user with the given username and password, returns their info
func (ts *TestServer) CreateTestUser(t *testing.T, username, password string) TestUserInfo {
	t.Helper()
	resp, err := ts.POST("/api/auth/users", map[string]string{
		"username":     username,
		"display_name": username,
		"password":     password,
	})
	if err != nil {
		t.Fatalf("create user request failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read create user response: %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create user failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var createResp struct {
		User struct {
			ID       int64  `json:"id"`
			Username string `json:"username"`
		} `json:"user"`
		APIKey string `json:"api_key"`
	}
	if err := json.Unmarshal(bodyBytes, &createResp); err != nil {
		t.Fatalf("failed to parse create user response: %v", err)
	}

	return TestUserInfo{
		ID:       createResp.User.ID,
		Username: createResp.User.Username,
		APIKey:   createResp.APIKey,
		Password: password,
	}
}

// CreateTestUserWithGrants creates a user and adds specific grants, returns their info
func (ts *TestServer) CreateTestUserWithGrants(t *testing.T, username, password string, grants []map[string]interface{}) TestUserInfo {
	t.Helper()
	user := ts.CreateTestUser(t, username, password)

	for _, grant := range grants {
		grant["user_id"] = user.ID
		resp, err := ts.POST(fmt.Sprintf("/api/auth/users/%d/grants", user.ID), grant)
		if err != nil {
			t.Fatalf("create grant request failed: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Fatalf("create grant failed with status %d: %s", resp.StatusCode, string(bodyBytes))
		}
	}

	return user
}

// StartTestServerWithMaxSize creates a test server with custom max_dat_size
func StartTestServerWithMaxSize(t *testing.T, maxSize int64) *TestServer {
	t.Helper()

	// Create temp directories
	workDir, err := os.MkdirTemp("", "silobang-test-work-*")
	if err != nil {
		t.Fatalf("failed to create work dir: %v", err)
	}

	configDir, err := os.MkdirTemp("", "silobang-test-config-*")
	if err != nil {
		os.RemoveAll(workDir)
		t.Fatalf("failed to create config dir: %v", err)
	}

	// Create app instance with custom max size
	cfg := &config.Config{
		WorkingDirectory: "",
		Port:             0,
		MaxDatSize:       maxSize,
	}
	cfg.ApplyDefaults()

	log := logger.NewLogger(logger.LevelError)
	app := server.NewApp(cfg, log)

	// Load default queries
	app.QueriesConfig = queries.GetDefaultConfig()

	// Create HTTP server
	srv := server.NewServer(app, ":0", nil)
	httpServer := httptest.NewServer(srv.Handler())

	ts := &TestServer{
		Server:    httpServer,
		App:       app,
		WorkDir:   workDir,
		ConfigDir: configDir,
		URL:       httpServer.URL,
	}

	t.Cleanup(func() {
		ts.Cleanup()
	})

	return ts
}
