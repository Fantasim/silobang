package e2e

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"silobang/internal/constants"
)

// TestBulkDownloadSSE_BasicFlow tests the complete SSE flow with query mode
func TestBulkDownloadSSE_BasicFlow(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "test-topic")

	// Upload test assets
	content1 := []byte("Hello World 1")
	content2 := []byte("Hello World 2")
	upload1 := ts.UploadFileExpectSuccess(t, "test-topic", "file1.txt", content1, "")
	upload2 := ts.UploadFileExpectSuccess(t, "test-topic", "file2.txt", content2, "")

	// Start SSE bulk download
	resp, err := ts.BulkDownloadSSE(t, "ids", "", nil, nil, []string{upload1.Hash, upload2.Hash}, false, "original")
	if err != nil {
		t.Fatalf("SSE request failed: %v", err)
	}
	defer resp.Body.Close()

	// Parse events
	events := ParseBulkDownloadSSEEvents(t, resp)

	// Verify event sequence
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events (download_start, complete), got %d", len(events))
	}

	// Check download_start event
	startEvent := FindBulkDownloadSSEEvent(events, "download_start")
	if startEvent == nil {
		t.Fatal("missing download_start event")
	}
	totalAssets, _ := startEvent.Data["total_assets"].(float64)
	if int(totalAssets) != 2 {
		t.Errorf("expected total_assets=2, got %v", totalAssets)
	}

	// Check complete event
	completeEvent := FindBulkDownloadSSEEvent(events, "complete")
	if completeEvent == nil {
		t.Fatal("missing complete event")
	}

	downloadID := GetDownloadIDFromEvents(t, events)
	if downloadID == "" {
		t.Fatal("download_id is empty")
	}

	// Fetch the ZIP
	zipBytes := ts.FetchBulkDownloadZIP(t, downloadID)
	if len(zipBytes) == 0 {
		t.Fatal("ZIP file is empty")
	}

	// Verify ZIP contents
	manifest := ExtractZIPManifest(t, zipBytes)
	if manifest.AssetCount != 2 {
		t.Errorf("expected 2 assets in manifest, got %d", manifest.AssetCount)
	}

	// Verify file content
	file1Bytes := ExtractZIPFile(t, zipBytes, "assets/file1.txt")
	if !bytes.Equal(file1Bytes, content1) {
		t.Errorf("file1 content mismatch")
	}

	file2Bytes := ExtractZIPFile(t, zipBytes, "assets/file2.txt")
	if !bytes.Equal(file2Bytes, content2) {
		t.Errorf("file2 content mismatch")
	}
}

// TestBulkDownloadSSE_QueryMode tests SSE with query mode
func TestBulkDownloadSSE_QueryMode(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "query-topic")

	// Upload test assets
	content1 := []byte("Content A")
	content2 := []byte("Content B")
	ts.UploadFileExpectSuccess(t, "query-topic", "a.txt", content1, "")
	ts.UploadFileExpectSuccess(t, "query-topic", "b.txt", content2, "")

	// Start SSE bulk download with query mode (using "recent-imports" preset)
	resp, err := ts.BulkDownloadSSE(t, "query", "recent-imports", map[string]interface{}{"days": "30", "limit": "100"}, []string{"query-topic"}, nil, false, "original")
	if err != nil {
		t.Fatalf("SSE request failed: %v", err)
	}
	defer resp.Body.Close()

	events := ParseBulkDownloadSSEEvents(t, resp)

	// Verify complete event
	completeEvent := FindBulkDownloadSSEEvent(events, "complete")
	if completeEvent == nil {
		t.Fatal("missing complete event")
	}

	// Verify mode in download_start
	startEvent := FindBulkDownloadSSEEvent(events, "download_start")
	if startEvent == nil {
		t.Fatal("missing download_start event")
	}
	mode, _ := startEvent.Data["mode"].(string)
	if mode != "query" {
		t.Errorf("expected mode=query, got %s", mode)
	}

	// Fetch and verify ZIP
	downloadID := GetDownloadIDFromEvents(t, events)
	zipBytes := ts.FetchBulkDownloadZIP(t, downloadID)
	manifest := ExtractZIPManifest(t, zipBytes)

	if manifest.AssetCount != 2 {
		t.Errorf("expected 2 assets, got %d", manifest.AssetCount)
	}
}

// TestBulkDownloadSSE_ProgressEvents tests that progress events are sent
func TestBulkDownloadSSE_ProgressEvents(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "progress-topic")

	// Upload more assets than progress interval to trigger progress events
	var hashes []string
	for i := 0; i < 15; i++ {
		content := []byte("Content for file " + string(rune('A'+i)))
		upload := ts.UploadFileExpectSuccess(t, "progress-topic", "file"+string(rune('A'+i))+".txt", content, "")
		hashes = append(hashes, upload.Hash)
	}

	resp, err := ts.BulkDownloadSSE(t, "ids", "", nil, nil, hashes, false, "hash")
	if err != nil {
		t.Fatalf("SSE request failed: %v", err)
	}
	defer resp.Body.Close()

	events := ParseBulkDownloadSSEEvents(t, resp)

	// Should have progress events
	progressEvents := FindAllBulkDownloadSSEEvents(events, "asset_progress")
	if len(progressEvents) == 0 {
		t.Error("expected asset_progress events but got none")
	}

	// Verify progress event structure
	if len(progressEvents) > 0 {
		firstProgress := progressEvents[0]
		if _, ok := firstProgress.Data["asset_index"]; !ok {
			t.Error("asset_progress missing asset_index")
		}
		if _, ok := firstProgress.Data["total_assets"]; !ok {
			t.Error("asset_progress missing total_assets")
		}
		if _, ok := firstProgress.Data["hash"]; !ok {
			t.Error("asset_progress missing hash")
		}
	}

	// Verify complete
	completeEvent := FindBulkDownloadSSEEvent(events, "complete")
	if completeEvent == nil {
		t.Fatal("missing complete event")
	}
}

// TestBulkDownloadSSE_SessionNotFound tests 404 for invalid download ID
func TestBulkDownloadSSE_SessionNotFound(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	errResp := ts.FetchBulkDownloadZIPExpectError(t, "nonexistent-id-12345", 404)
	if errResp.Code != constants.ErrCodeDownloadSessionNotFound {
		t.Errorf("expected error code %s, got %s", constants.ErrCodeDownloadSessionNotFound, errResp.Code)
	}
}

// TestBulkDownloadSSE_NotConfigured tests error before work dir is set
func TestBulkDownloadSSE_NotConfigured(t *testing.T) {
	ts := StartTestServer(t)
	// Without configuration, auth store doesn't exist so request returns 401

	resp, err := ts.UnauthenticatedGET("/api/download/bulk/start?mode=ids&asset_ids=somehash")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status %d (auth required before config), got %d", http.StatusUnauthorized, resp.StatusCode)
	}
}

// TestBulkDownloadSSE_InvalidMode tests error for invalid mode
func TestBulkDownloadSSE_InvalidMode(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	resp, err := ts.BulkDownloadSSE(t, "invalid", "", nil, nil, []string{"somehash"}, false, "")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// With SSE, errors come as events with HTTP 200
	if resp.StatusCode != 200 {
		t.Errorf("expected status 200 for SSE, got %d", resp.StatusCode)
	}

	events := ParseBulkDownloadSSEEvents(t, resp)
	if len(events) == 0 {
		t.Fatal("expected at least one SSE event")
	}

	// Should have an error event
	lastEvent := events[len(events)-1]
	if lastEvent.Type != "error" {
		t.Errorf("expected error event, got %s", lastEvent.Type)
	}
	code, _ := lastEvent.Data["code"].(string)
	if code != constants.ErrCodeInvalidDownloadMode {
		t.Errorf("expected error code %s, got %s", constants.ErrCodeInvalidDownloadMode, code)
	}
}

// TestBulkDownloadSSE_EmptyResult tests error when no assets found
func TestBulkDownloadSSE_EmptyResult(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "empty-topic")

	// Try to download with non-existent hash
	resp, err := ts.BulkDownloadSSE(t, "ids", "", nil, nil, []string{"0000000000000000000000000000000000000000000000000000000000000000"}, false, "")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// With SSE, errors come as events with HTTP 200
	if resp.StatusCode != 200 {
		t.Errorf("expected status 200 for SSE, got %d", resp.StatusCode)
	}

	events := ParseBulkDownloadSSEEvents(t, resp)
	if len(events) == 0 {
		t.Fatal("expected at least one SSE event")
	}

	// Should have an error event
	lastEvent := events[len(events)-1]
	if lastEvent.Type != "error" {
		t.Errorf("expected error event, got %s", lastEvent.Type)
	}
	code, _ := lastEvent.Data["code"].(string)
	if code != constants.ErrCodeBulkDownloadEmpty {
		t.Errorf("expected error code %s, got %s", constants.ErrCodeBulkDownloadEmpty, code)
	}
}

// TestBulkDownloadSSE_Immutability verifies .dat files are unchanged after download
func TestBulkDownloadSSE_Immutability(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "immutable-topic")

	content := []byte("Immutability test content that should not change")
	upload := ts.UploadFileExpectSuccess(t, "immutable-topic", "test.bin", content, "")

	// Get hash of .dat file before download
	datPath := filepath.Join(ts.WorkDir, "immutable-topic", "000001.dat")
	beforeHash := hashFile(t, datPath)

	// Perform SSE download
	resp, err := ts.BulkDownloadSSE(t, "ids", "", nil, nil, []string{upload.Hash}, false, "hash")
	if err != nil {
		t.Fatalf("SSE request failed: %v", err)
	}
	defer resp.Body.Close()

	events := ParseBulkDownloadSSEEvents(t, resp)
	downloadID := GetDownloadIDFromEvents(t, events)

	// Fetch the ZIP to complete the download
	zipBytes := ts.FetchBulkDownloadZIP(t, downloadID)
	if len(zipBytes) == 0 {
		t.Fatal("ZIP is empty")
	}

	// Verify .dat file hash is unchanged
	afterHash := hashFile(t, datPath)
	if beforeHash != afterHash {
		t.Errorf("dat file was modified during download: before=%s, after=%s", beforeHash, afterHash)
	}
}

// TestBulkDownloadSSE_ContentMatch verifies downloaded content matches original
func TestBulkDownloadSSE_ContentMatch(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "content-topic")

	// Upload file with known content
	content := []byte("This is the exact content that should be in the ZIP")
	upload := ts.UploadFileExpectSuccess(t, "content-topic", "exact.txt", content, "")

	// SSE download
	resp, err := ts.BulkDownloadSSE(t, "ids", "", nil, nil, []string{upload.Hash}, false, "original")
	if err != nil {
		t.Fatalf("SSE request failed: %v", err)
	}
	defer resp.Body.Close()

	events := ParseBulkDownloadSSEEvents(t, resp)
	downloadID := GetDownloadIDFromEvents(t, events)

	zipBytes := ts.FetchBulkDownloadZIP(t, downloadID)

	// Extract and compare
	extracted := ExtractZIPFile(t, zipBytes, "assets/exact.txt")
	if !bytes.Equal(extracted, content) {
		t.Errorf("content mismatch: expected %q, got %q", string(content), string(extracted))
	}
}

// TestBulkDownloadSSE_ZIPIntegrity verifies ZIP structure is valid
func TestBulkDownloadSSE_ZIPIntegrity(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "zip-topic")

	content := []byte("ZIP integrity test")
	upload := ts.UploadFileExpectSuccess(t, "zip-topic", "integrity.txt", content, "")

	resp, err := ts.BulkDownloadSSE(t, "ids", "", nil, nil, []string{upload.Hash}, false, "original")
	if err != nil {
		t.Fatalf("SSE request failed: %v", err)
	}
	defer resp.Body.Close()

	events := ParseBulkDownloadSSEEvents(t, resp)
	downloadID := GetDownloadIDFromEvents(t, events)

	zipBytes := ts.FetchBulkDownloadZIP(t, downloadID)

	// Verify ZIP can be opened and read
	reader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("failed to open ZIP: %v", err)
	}

	// Verify expected files exist
	files := ListZIPFiles(t, zipBytes)
	hasManifest := false
	hasAsset := false
	for _, f := range files {
		if f == "manifest.json" {
			hasManifest = true
		}
		if f == "assets/integrity.txt" {
			hasAsset = true
		}
	}

	if !hasManifest {
		t.Error("ZIP missing manifest.json")
	}
	if !hasAsset {
		t.Error("ZIP missing asset file")
	}

	// Verify all files can be read
	for _, file := range reader.File {
		rc, err := file.Open()
		if err != nil {
			t.Errorf("failed to open %s: %v", file.Name, err)
			continue
		}
		_, err = io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Errorf("failed to read %s: %v", file.Name, err)
		}
	}
}

// TestBulkDownloadSSE_WithMetadata tests metadata inclusion
func TestBulkDownloadSSE_WithMetadata(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "meta-topic")

	content := []byte("Metadata test content")
	upload := ts.UploadFileExpectSuccess(t, "meta-topic", "meta.txt", content, "")

	// Set some metadata
	ts.SetMetadata(t, upload.Hash, "custom_field", "custom_value")

	// SSE download with metadata
	resp, err := ts.BulkDownloadSSE(t, "ids", "", nil, nil, []string{upload.Hash}, true, "original")
	if err != nil {
		t.Fatalf("SSE request failed: %v", err)
	}
	defer resp.Body.Close()

	events := ParseBulkDownloadSSEEvents(t, resp)
	downloadID := GetDownloadIDFromEvents(t, events)

	zipBytes := ts.FetchBulkDownloadZIP(t, downloadID)

	// Verify manifest shows include_metadata=true
	manifest := ExtractZIPManifest(t, zipBytes)
	if !manifest.IncludeMetadata {
		t.Error("manifest should have include_metadata=true")
	}

	// Verify metadata file exists (format is "original", uploaded as "meta.txt")
	metaFile := ExtractAssetMetadata(t, zipBytes, "meta")
	if metaFile.Asset.Hash != upload.Hash {
		t.Errorf("metadata hash mismatch: expected %s, got %s", upload.Hash, metaFile.Asset.Hash)
	}

	// Check custom metadata is included
	if val, ok := metaFile.ComputedMetadata["custom_field"]; !ok || val != "custom_value" {
		t.Errorf("custom metadata not found or incorrect: %v", metaFile.ComputedMetadata)
	}
}

// TestBulkDownloadSSE_FilenameFormats tests different filename formats
func TestBulkDownloadSSE_FilenameFormats(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "format-topic")

	content := []byte("Format test content")
	upload := ts.UploadFileExpectSuccess(t, "format-topic", "myfile.txt", content, "")

	tests := []struct {
		format   string
		contains string // What the filename should contain
	}{
		{"hash", upload.Hash},
		{"original", "myfile"},
		{"hash_original", upload.Hash},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			resp, err := ts.BulkDownloadSSE(t, "ids", "", nil, nil, []string{upload.Hash}, false, tt.format)
			if err != nil {
				t.Fatalf("SSE request failed: %v", err)
			}
			defer resp.Body.Close()

			events := ParseBulkDownloadSSEEvents(t, resp)
			downloadID := GetDownloadIDFromEvents(t, events)

			zipBytes := ts.FetchBulkDownloadZIP(t, downloadID)
			manifest := ExtractZIPManifest(t, zipBytes)

			if len(manifest.Assets) == 0 {
				t.Fatal("no assets in manifest")
			}

			filename := manifest.Assets[0].Filename
			if !containsString(filename, tt.contains) {
				t.Errorf("filename %q should contain %q for format %s", filename, tt.contains, tt.format)
			}
		})
	}
}

// TestBulkDownloadSSE_ConcurrentDownloads tests multiple simultaneous downloads
func TestBulkDownloadSSE_ConcurrentDownloads(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "concurrent-topic")

	content1 := []byte("Content for download 1")
	content2 := []byte("Content for download 2")
	upload1 := ts.UploadFileExpectSuccess(t, "concurrent-topic", "file1.txt", content1, "")
	upload2 := ts.UploadFileExpectSuccess(t, "concurrent-topic", "file2.txt", content2, "")

	// Start two SSE downloads
	resp1, err := ts.BulkDownloadSSE(t, "ids", "", nil, nil, []string{upload1.Hash}, false, "original")
	if err != nil {
		t.Fatalf("SSE request 1 failed: %v", err)
	}
	defer resp1.Body.Close()

	resp2, err := ts.BulkDownloadSSE(t, "ids", "", nil, nil, []string{upload2.Hash}, false, "original")
	if err != nil {
		t.Fatalf("SSE request 2 failed: %v", err)
	}
	defer resp2.Body.Close()

	// Parse both
	events1 := ParseBulkDownloadSSEEvents(t, resp1)
	events2 := ParseBulkDownloadSSEEvents(t, resp2)

	downloadID1 := GetDownloadIDFromEvents(t, events1)
	downloadID2 := GetDownloadIDFromEvents(t, events2)

	// Download IDs should be different
	if downloadID1 == downloadID2 {
		t.Error("concurrent downloads have same ID")
	}

	// Both should be fetchable
	zip1 := ts.FetchBulkDownloadZIP(t, downloadID1)
	zip2 := ts.FetchBulkDownloadZIP(t, downloadID2)

	// Verify different content
	file1 := ExtractZIPFile(t, zip1, "assets/file1.txt")
	file2 := ExtractZIPFile(t, zip2, "assets/file2.txt")

	if !bytes.Equal(file1, content1) {
		t.Error("download 1 content mismatch")
	}
	if !bytes.Equal(file2, content2) {
		t.Error("download 2 content mismatch")
	}
}

// TestBulkDownloadSSE_TempFileCreated verifies ZIP is created in temp directory
func TestBulkDownloadSSE_TempFileCreated(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "temp-topic")

	content := []byte("Temp file test")
	upload := ts.UploadFileExpectSuccess(t, "temp-topic", "temp.txt", content, "")

	resp, err := ts.BulkDownloadSSE(t, "ids", "", nil, nil, []string{upload.Hash}, false, "original")
	if err != nil {
		t.Fatalf("SSE request failed: %v", err)
	}
	defer resp.Body.Close()

	events := ParseBulkDownloadSSEEvents(t, resp)
	downloadID := GetDownloadIDFromEvents(t, events)

	// Check that temp file exists
	tempDir := filepath.Join(ts.WorkDir, constants.InternalDir, constants.BulkDownloadTempDir)
	zipPath := filepath.Join(tempDir, downloadID+".zip")

	if _, err := os.Stat(zipPath); os.IsNotExist(err) {
		t.Errorf("expected temp ZIP file at %s but it doesn't exist", zipPath)
	}
}

// TestBulkDownloadSSE_ManifestCorrectness verifies manifest metadata is accurate
func TestBulkDownloadSSE_ManifestCorrectness(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "manifest-topic")

	content1 := []byte("File one content")
	content2 := []byte("File two content with more bytes")
	upload1 := ts.UploadFileExpectSuccess(t, "manifest-topic", "one.txt", content1, "")
	upload2 := ts.UploadFileExpectSuccess(t, "manifest-topic", "two.txt", content2, "")

	resp, err := ts.BulkDownloadSSE(t, "ids", "", nil, nil, []string{upload1.Hash, upload2.Hash}, false, "original")
	if err != nil {
		t.Fatalf("SSE request failed: %v", err)
	}
	defer resp.Body.Close()

	events := ParseBulkDownloadSSEEvents(t, resp)
	downloadID := GetDownloadIDFromEvents(t, events)

	zipBytes := ts.FetchBulkDownloadZIP(t, downloadID)
	manifest := ExtractZIPManifest(t, zipBytes)

	// Verify counts
	if manifest.AssetCount != 2 {
		t.Errorf("expected asset_count=2, got %d", manifest.AssetCount)
	}

	// Verify total size
	expectedSize := int64(len(content1) + len(content2))
	if manifest.TotalSize != expectedSize {
		t.Errorf("expected total_size=%d, got %d", expectedSize, manifest.TotalSize)
	}

	// Verify each asset in manifest
	hashToContent := map[string][]byte{
		upload1.Hash: content1,
		upload2.Hash: content2,
	}

	for _, asset := range manifest.Assets {
		content, ok := hashToContent[asset.Hash]
		if !ok {
			t.Errorf("unexpected hash in manifest: %s", asset.Hash)
			continue
		}
		if asset.Size != int64(len(content)) {
			t.Errorf("asset %s: expected size=%d, got %d", asset.Hash, len(content), asset.Size)
		}
		if asset.Topic != "manifest-topic" {
			t.Errorf("asset %s: expected topic=manifest-topic, got %s", asset.Hash, asset.Topic)
		}
	}

	// Verify no failed assets
	if len(manifest.FailedAssets) != 0 {
		t.Errorf("expected no failed assets, got %d", len(manifest.FailedAssets))
	}
}

// TestBulkDownloadSSE_EventOrder verifies events arrive in correct order
func TestBulkDownloadSSE_EventOrder(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "order-topic")

	content := []byte("Event order test")
	upload := ts.UploadFileExpectSuccess(t, "order-topic", "order.txt", content, "")

	resp, err := ts.BulkDownloadSSE(t, "ids", "", nil, nil, []string{upload.Hash}, false, "original")
	if err != nil {
		t.Fatalf("SSE request failed: %v", err)
	}
	defer resp.Body.Close()

	events := ParseBulkDownloadSSEEvents(t, resp)

	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}

	// First event should be download_start
	if events[0].Type != "download_start" {
		t.Errorf("first event should be download_start, got %s", events[0].Type)
	}

	// Last event should be complete
	if events[len(events)-1].Type != "complete" {
		t.Errorf("last event should be complete, got %s", events[len(events)-1].Type)
	}

	// Timestamps should be monotonically increasing
	for i := 1; i < len(events); i++ {
		if events[i].Timestamp < events[i-1].Timestamp {
			t.Errorf("timestamps not monotonically increasing: event %d (%d) < event %d (%d)",
				i, events[i].Timestamp, i-1, events[i-1].Timestamp)
		}
	}
}

// TestBulkDownloadSSE_CrossTopic tests downloading from multiple topics
func TestBulkDownloadSSE_CrossTopic(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "topic-a")
	ts.CreateTopic(t, "topic-b")

	contentA := []byte("Content from topic A")
	contentB := []byte("Content from topic B")
	uploadA := ts.UploadFileExpectSuccess(t, "topic-a", "a.txt", contentA, "")
	uploadB := ts.UploadFileExpectSuccess(t, "topic-b", "b.txt", contentB, "")

	resp, err := ts.BulkDownloadSSE(t, "ids", "", nil, nil, []string{uploadA.Hash, uploadB.Hash}, false, "original")
	if err != nil {
		t.Fatalf("SSE request failed: %v", err)
	}
	defer resp.Body.Close()

	events := ParseBulkDownloadSSEEvents(t, resp)
	downloadID := GetDownloadIDFromEvents(t, events)

	zipBytes := ts.FetchBulkDownloadZIP(t, downloadID)
	manifest := ExtractZIPManifest(t, zipBytes)

	// Should have assets from both topics
	topics := make(map[string]bool)
	for _, asset := range manifest.Assets {
		topics[asset.Topic] = true
	}

	if !topics["topic-a"] {
		t.Error("missing asset from topic-a")
	}
	if !topics["topic-b"] {
		t.Error("missing asset from topic-b")
	}
}

// TestBulkDownloadSSE_AutoDeleteAfterFetch verifies ZIP is removed after successful download
func TestBulkDownloadSSE_AutoDeleteAfterFetch(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "auto-delete-topic")

	content := []byte("Auto-delete test content")
	upload := ts.UploadFileExpectSuccess(t, "auto-delete-topic", "test.txt", content, "")

	// Start SSE download
	resp, err := ts.BulkDownloadSSE(t, "ids", "", nil, nil, []string{upload.Hash}, false, "original")
	if err != nil {
		t.Fatalf("SSE request failed: %v", err)
	}
	defer resp.Body.Close()

	events := ParseBulkDownloadSSEEvents(t, resp)
	downloadID := GetDownloadIDFromEvents(t, events)

	// Verify ZIP file exists before fetch
	tempDir := filepath.Join(ts.WorkDir, constants.InternalDir, constants.BulkDownloadTempDir)
	zipPath := filepath.Join(tempDir, downloadID+".zip")

	if _, err := os.Stat(zipPath); os.IsNotExist(err) {
		t.Fatalf("ZIP file should exist before fetch: %s", zipPath)
	}

	// Fetch the ZIP (this should trigger auto-delete)
	zipBytes := ts.FetchBulkDownloadZIP(t, downloadID)
	if len(zipBytes) == 0 {
		t.Fatal("ZIP should not be empty")
	}

	// Verify content is correct
	extracted := ExtractZIPFile(t, zipBytes, "assets/test.txt")
	if !bytes.Equal(extracted, content) {
		t.Error("content mismatch")
	}

	// Verify ZIP file is deleted after fetch
	if _, err := os.Stat(zipPath); !os.IsNotExist(err) {
		t.Errorf("ZIP file should be deleted after successful fetch: %s", zipPath)
	}

	// Verify second fetch returns 404
	errResp := ts.FetchBulkDownloadZIPExpectError(t, downloadID, 404)
	if errResp.Code != constants.ErrCodeDownloadSessionNotFound {
		t.Errorf("expected %s, got %s", constants.ErrCodeDownloadSessionNotFound, errResp.Code)
	}
}

// TestBulkDownloadSSE_StartupCleanup verifies leftover ZIPs are cleaned on new session manager init
func TestBulkDownloadSSE_StartupCleanup(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "cleanup-topic")

	// Create a download
	content := []byte("Cleanup test content")
	upload := ts.UploadFileExpectSuccess(t, "cleanup-topic", "test.txt", content, "")

	resp, err := ts.BulkDownloadSSE(t, "ids", "", nil, nil, []string{upload.Hash}, false, "original")
	if err != nil {
		t.Fatalf("SSE request failed: %v", err)
	}
	defer resp.Body.Close()

	events := ParseBulkDownloadSSEEvents(t, resp)
	downloadID := GetDownloadIDFromEvents(t, events)

	// Verify ZIP exists
	tempDir := filepath.Join(ts.WorkDir, constants.InternalDir, constants.BulkDownloadTempDir)
	zipPath := filepath.Join(tempDir, downloadID+".zip")

	if _, err := os.Stat(zipPath); os.IsNotExist(err) {
		t.Fatalf("ZIP file should exist: %s", zipPath)
	}

	// Restart the server (simulates server restart)
	ts.Restart(t)
	ts.ConfigureWorkDir(t) // Re-configure to trigger download manager init

	// Create another download to trigger lazy init of download manager
	ts.CreateTopic(t, "cleanup-topic-2")
	content2 := []byte("New content")
	upload2 := ts.UploadFileExpectSuccess(t, "cleanup-topic-2", "test2.txt", content2, "")

	resp2, err := ts.BulkDownloadSSE(t, "ids", "", nil, nil, []string{upload2.Hash}, false, "original")
	if err != nil {
		t.Fatalf("SSE request failed: %v", err)
	}
	defer resp2.Body.Close()

	// Old ZIP should be cleaned up
	if _, err := os.Stat(zipPath); !os.IsNotExist(err) {
		t.Errorf("Old ZIP file should be deleted on restart: %s", zipPath)
	}
}

// TestBulkDownloadSSE_CleanupDoesNotAffectSourceData verifies .dat files unchanged during cleanup
func TestBulkDownloadSSE_CleanupDoesNotAffectSourceData(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "immutable-cleanup")

	content := []byte("Source data that must remain unchanged")
	upload := ts.UploadFileExpectSuccess(t, "immutable-cleanup", "source.bin", content, "")

	// Hash the .dat file before any downloads
	datPath := filepath.Join(ts.WorkDir, "immutable-cleanup", "000001.dat")
	beforeHash := hashFile(t, datPath)

	// Create and fetch multiple downloads (triggering cleanup)
	for i := 0; i < 3; i++ {
		resp, _ := ts.BulkDownloadSSE(t, "ids", "", nil, nil, []string{upload.Hash}, false, "original")
		events := ParseBulkDownloadSSEEvents(t, resp)
		downloadID := GetDownloadIDFromEvents(t, events)
		ts.FetchBulkDownloadZIP(t, downloadID) // Triggers auto-delete
		resp.Body.Close()
	}

	// Verify .dat file is unchanged
	afterHash := hashFile(t, datPath)
	if beforeHash != afterHash {
		t.Errorf("Source .dat file was modified during cleanup operations")
	}

	// Verify data is still downloadable via single asset download
	downloaded := ts.DownloadAsset(t, upload.Hash)
	if !bytes.Equal(downloaded, content) {
		t.Error("Source data corrupted")
	}
}

// TestBulkDownloadSSE_CleanupOnlyTargetsDownloadsDir verifies cleanup is scoped to downloads dir
func TestBulkDownloadSSE_CleanupOnlyTargetsDownloadsDir(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Create a .zip file OUTSIDE the downloads directory (simulate potential attack vector)
	maliciousZipPath := filepath.Join(ts.WorkDir, "malicious.zip")
	os.WriteFile(maliciousZipPath, []byte("not a real zip"), 0644)

	// Also create one in .internal but not in downloads
	internalDir := filepath.Join(ts.WorkDir, constants.InternalDir)
	os.MkdirAll(internalDir, 0755)
	internalZipPath := filepath.Join(internalDir, "important.zip")
	os.WriteFile(internalZipPath, []byte("important data"), 0644)

	// Trigger cleanup by creating a download
	ts.CreateTopic(t, "security-topic")
	content := []byte("test")
	upload := ts.UploadFileExpectSuccess(t, "security-topic", "test.txt", content, "")

	resp, _ := ts.BulkDownloadSSE(t, "ids", "", nil, nil, []string{upload.Hash}, false, "original")
	resp.Body.Close()

	// Verify files outside downloads directory are NOT deleted
	if _, err := os.Stat(maliciousZipPath); os.IsNotExist(err) {
		t.Error("File outside downloads directory was incorrectly deleted")
	}
	if _, err := os.Stat(internalZipPath); os.IsNotExist(err) {
		t.Error("File in .internal but not in downloads was incorrectly deleted")
	}
}

// Helper functions

func hashFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file %s: %v", path, err)
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func containsString(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}
