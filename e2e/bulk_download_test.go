package e2e

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"meshbank/internal/constants"
)

// TestBulkDownload_SingleAsset tests downloading a single asset via IDs mode
func TestBulkDownload_SingleAsset(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "test-topic")

	// Upload a test file
	content := []byte("test file content for bulk download")
	uploadResp := ts.UploadFileExpectSuccess(t, "test-topic", "test.txt", content, "")

	// Download via bulk
	zipBytes := ts.BulkDownloadExpectSuccess(t, BulkDownloadRequest{
		Mode:     "ids",
		AssetIDs: []string{uploadResp.Hash},
	})

	// Verify manifest
	manifest := ExtractZIPManifest(t, zipBytes)
	if manifest.AssetCount != 1 {
		t.Errorf("expected 1 asset, got %d", manifest.AssetCount)
	}
	if manifest.TotalSize != int64(len(content)) {
		t.Errorf("expected total size %d, got %d", len(content), manifest.TotalSize)
	}
	if len(manifest.Assets) != 1 {
		t.Fatalf("expected 1 asset in manifest, got %d", len(manifest.Assets))
	}
	if manifest.Assets[0].Hash != uploadResp.Hash {
		t.Errorf("expected hash %s, got %s", uploadResp.Hash, manifest.Assets[0].Hash)
	}

	// Verify file content - default format is "original"
	expectedFilename := "assets/test.txt"
	downloadedContent := ExtractZIPFile(t, zipBytes, expectedFilename)
	if !bytes.Equal(downloadedContent, content) {
		t.Errorf("downloaded content does not match original")
	}
}

// TestBulkDownload_MultipleAssets tests downloading multiple assets
func TestBulkDownload_MultipleAssets(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "test-topic")

	// Upload multiple test files
	files := map[string][]byte{
		"file1.txt": []byte("content of file 1"),
		"file2.txt": []byte("content of file 2 - longer"),
		"file3.glb": []byte("binary glb content here"),
		"file4.png": []byte("png image data"),
		"file5.obj": []byte("obj model data with more content"),
	}

	var hashes []string
	for filename, content := range files {
		resp := ts.UploadFileExpectSuccess(t, "test-topic", filename, content, "")
		hashes = append(hashes, resp.Hash)
	}

	// Download all via bulk
	zipBytes := ts.BulkDownloadExpectSuccess(t, BulkDownloadRequest{
		Mode:     "ids",
		AssetIDs: hashes,
	})

	// Verify manifest
	manifest := ExtractZIPManifest(t, zipBytes)
	if manifest.AssetCount != 5 {
		t.Errorf("expected 5 assets, got %d", manifest.AssetCount)
	}
	if len(manifest.FailedAssets) != 0 {
		t.Errorf("expected 0 failed assets, got %d", len(manifest.FailedAssets))
	}

	// Verify all files present
	zipFiles := ListZIPFiles(t, zipBytes)
	if len(zipFiles) != 6 { // 5 assets + manifest
		t.Errorf("expected 6 files in zip, got %d: %v", len(zipFiles), zipFiles)
	}
}

// TestBulkDownload_WithMetadata tests including metadata files
func TestBulkDownload_WithMetadata(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "test-topic")

	// Upload a file and set metadata
	content := []byte("test content with metadata")
	uploadResp := ts.UploadFileExpectSuccess(t, "test-topic", "model.glb", content, "")

	// Set some metadata
	ts.SetMetadata(t, uploadResp.Hash, "polygon_count", "50000")
	ts.SetMetadata(t, uploadResp.Hash, "author", "test_artist")
	ts.SetMetadata(t, uploadResp.Hash, "verified", "true")

	// Download with metadata
	zipBytes := ts.BulkDownloadExpectSuccess(t, BulkDownloadRequest{
		Mode:            "ids",
		AssetIDs:        []string{uploadResp.Hash},
		IncludeMetadata: true,
	})

	// Verify manifest indicates metadata
	manifest := ExtractZIPManifest(t, zipBytes)
	if !manifest.IncludeMetadata {
		t.Error("manifest should indicate include_metadata=true")
	}

	// Verify metadata file exists and has correct content
	// Default format is "original", so metadata filename mirrors origin name
	metadata := ExtractAssetMetadata(t, zipBytes, "model")

	if metadata.Asset.Hash != uploadResp.Hash {
		t.Errorf("metadata hash mismatch: got %s, want %s", metadata.Asset.Hash, uploadResp.Hash)
	}
	if metadata.Asset.Extension != "glb" {
		t.Errorf("metadata extension mismatch: got %s, want glb", metadata.Asset.Extension)
	}
	if metadata.Asset.OriginName != "model" {
		t.Errorf("metadata origin_name mismatch: got %s, want model", metadata.Asset.OriginName)
	}

	// Check computed_metadata
	if metadata.ComputedMetadata == nil {
		t.Fatal("computed_metadata should not be nil")
	}
	if v, ok := metadata.ComputedMetadata["polygon_count"].(float64); !ok || v != 50000 {
		t.Errorf("polygon_count mismatch: got %v", metadata.ComputedMetadata["polygon_count"])
	}
	if v, ok := metadata.ComputedMetadata["author"].(string); !ok || v != "test_artist" {
		t.Errorf("author mismatch: got %v", metadata.ComputedMetadata["author"])
	}

	// Verify ZIP structure has both assets/ and metadata/ dirs
	zipFiles := ListZIPFiles(t, zipBytes)
	hasAssetDir := false
	hasMetadataDir := false
	for _, f := range zipFiles {
		if strings.HasPrefix(f, "assets/") {
			hasAssetDir = true
		}
		if strings.HasPrefix(f, "metadata/") {
			hasMetadataDir = true
		}
	}
	if !hasAssetDir {
		t.Error("ZIP should contain assets/ directory")
	}
	if !hasMetadataDir {
		t.Error("ZIP should contain metadata/ directory")
	}
}

// TestBulkDownload_WithoutMetadata tests that metadata is not included when not requested
func TestBulkDownload_WithoutMetadata(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "test-topic")

	// Upload a file and set metadata
	content := []byte("test content")
	uploadResp := ts.UploadFileExpectSuccess(t, "test-topic", "file.txt", content, "")
	ts.SetMetadata(t, uploadResp.Hash, "key", "value")

	// Download without metadata (default)
	zipBytes := ts.BulkDownloadExpectSuccess(t, BulkDownloadRequest{
		Mode:            "ids",
		AssetIDs:        []string{uploadResp.Hash},
		IncludeMetadata: false,
	})

	// Verify no metadata directory
	zipFiles := ListZIPFiles(t, zipBytes)
	for _, f := range zipFiles {
		if strings.HasPrefix(f, "metadata/") {
			t.Errorf("ZIP should not contain metadata files, found: %s", f)
		}
	}

	// Manifest should indicate no metadata
	manifest := ExtractZIPManifest(t, zipBytes)
	if manifest.IncludeMetadata {
		t.Error("manifest should indicate include_metadata=false")
	}
}

// TestBulkDownload_QueryMode tests downloading via query preset
func TestBulkDownload_QueryMode(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "test-topic")

	// Upload files of different sizes
	smallContent := []byte("small")
	largeContent := make([]byte, 10000)
	for i := range largeContent {
		largeContent[i] = byte(i % 256)
	}

	ts.UploadFileExpectSuccess(t, "test-topic", "small.txt", smallContent, "")
	largeResp := ts.UploadFileExpectSuccess(t, "test-topic", "large.bin", largeContent, "")

	// Download via query - large files only (>100 bytes)
	zipBytes := ts.BulkDownloadExpectSuccess(t, BulkDownloadRequest{
		Mode:   "query",
		Preset: "large-files",
		Params: map[string]interface{}{
			"min_size": 100,
		},
	})

	// Verify only the large file is included
	manifest := ExtractZIPManifest(t, zipBytes)
	if manifest.AssetCount != 1 {
		t.Errorf("expected 1 asset from query, got %d", manifest.AssetCount)
	}
	if len(manifest.Assets) > 0 && manifest.Assets[0].Hash != largeResp.Hash {
		t.Errorf("expected large file hash %s, got %s", largeResp.Hash, manifest.Assets[0].Hash)
	}
}

// TestBulkDownload_CrossTopic tests downloading assets from multiple topics
func TestBulkDownload_CrossTopic(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "topic-a")
	ts.CreateTopic(t, "topic-b")

	// Upload to different topics
	contentA := []byte("content from topic A")
	contentB := []byte("content from topic B")

	respA := ts.UploadFileExpectSuccess(t, "topic-a", "file-a.txt", contentA, "")
	respB := ts.UploadFileExpectSuccess(t, "topic-b", "file-b.txt", contentB, "")

	// Download both via IDs
	zipBytes := ts.BulkDownloadExpectSuccess(t, BulkDownloadRequest{
		Mode:     "ids",
		AssetIDs: []string{respA.Hash, respB.Hash},
	})

	// Verify both are included
	manifest := ExtractZIPManifest(t, zipBytes)
	if manifest.AssetCount != 2 {
		t.Errorf("expected 2 assets, got %d", manifest.AssetCount)
	}

	// Verify topics are correctly recorded
	topics := make(map[string]bool)
	for _, asset := range manifest.Assets {
		topics[asset.Topic] = true
	}
	if !topics["topic-a"] || !topics["topic-b"] {
		t.Errorf("expected assets from both topics, got topics: %v", topics)
	}
}

// TestBulkDownload_FilenameFormatHash tests hash filename format
func TestBulkDownload_FilenameFormatHash(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "test-topic")

	content := []byte("test content")
	uploadResp := ts.UploadFileExpectSuccess(t, "test-topic", "original_name.txt", content, "")

	// Download with hash format
	zipBytes := ts.BulkDownloadExpectSuccess(t, BulkDownloadRequest{
		Mode:           "ids",
		AssetIDs:       []string{uploadResp.Hash},
		FilenameFormat: "hash",
	})

	// Verify filename is hash.ext
	expectedFilename := "assets/" + uploadResp.Hash + ".txt"
	_ = ExtractZIPFile(t, zipBytes, expectedFilename) // will fail if not found

	// Verify manifest has correct filename
	manifest := ExtractZIPManifest(t, zipBytes)
	if manifest.Assets[0].Filename != expectedFilename {
		t.Errorf("expected filename %s, got %s", expectedFilename, manifest.Assets[0].Filename)
	}
}

// TestBulkDownload_FilenameFormatOriginal tests original filename format
func TestBulkDownload_FilenameFormatOriginal(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "test-topic")

	content := []byte("test content for original format")
	uploadResp := ts.UploadFileExpectSuccess(t, "test-topic", "my_model.glb", content, "")

	// Download with original format
	zipBytes := ts.BulkDownloadExpectSuccess(t, BulkDownloadRequest{
		Mode:           "ids",
		AssetIDs:       []string{uploadResp.Hash},
		FilenameFormat: "original",
	})

	// Verify filename is origin_name.ext
	expectedFilename := "assets/my_model.glb"
	_ = ExtractZIPFile(t, zipBytes, expectedFilename)
}

// TestBulkDownload_FilenameFormatHashOriginal tests hash_original filename format
func TestBulkDownload_FilenameFormatHashOriginal(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "test-topic")

	content := []byte("test content")
	uploadResp := ts.UploadFileExpectSuccess(t, "test-topic", "model.glb", content, "")

	// Download with hash_original format
	zipBytes := ts.BulkDownloadExpectSuccess(t, BulkDownloadRequest{
		Mode:           "ids",
		AssetIDs:       []string{uploadResp.Hash},
		FilenameFormat: "hash_original",
	})

	// Verify filename is hash_origin_name.ext
	expectedFilename := "assets/" + uploadResp.Hash + "_model.glb"
	_ = ExtractZIPFile(t, zipBytes, expectedFilename)
}

// TestBulkDownload_DuplicateNames tests collision handling for original format
func TestBulkDownload_DuplicateNames(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "test-topic")

	// Upload multiple files with same original name (different content = different hash)
	content1 := []byte("content version 1")
	content2 := []byte("content version 2")
	content3 := []byte("content version 3")

	resp1 := ts.UploadFileExpectSuccess(t, "test-topic", "model.glb", content1, "")
	resp2 := ts.UploadFileExpectSuccess(t, "test-topic", "model.glb", content2, "")
	resp3 := ts.UploadFileExpectSuccess(t, "test-topic", "model.glb", content3, "")

	// Download all with original format
	zipBytes := ts.BulkDownloadExpectSuccess(t, BulkDownloadRequest{
		Mode:           "ids",
		AssetIDs:       []string{resp1.Hash, resp2.Hash, resp3.Hash},
		FilenameFormat: "original",
	})

	// Verify collision handling (model.glb, model_2.glb, model_3.glb)
	zipFiles := ListZIPFiles(t, zipBytes)

	hasModel := false
	hasModel2 := false
	hasModel3 := false
	for _, f := range zipFiles {
		switch f {
		case "assets/model.glb":
			hasModel = true
		case "assets/model_2.glb":
			hasModel2 = true
		case "assets/model_3.glb":
			hasModel3 = true
		}
	}

	if !hasModel || !hasModel2 || !hasModel3 {
		t.Errorf("expected collision handling: model.glb, model_2.glb, model_3.glb; got files: %v", zipFiles)
	}
}

// TestBulkDownload_EmptyResult tests error when query returns no assets
func TestBulkDownload_EmptyResult(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "test-topic")

	// Try to download non-existent assets
	errResp := ts.BulkDownloadExpectError(t, BulkDownloadRequest{
		Mode:     "ids",
		AssetIDs: []string{"0000000000000000000000000000000000000000000000000000000000000000"},
	}, 400)

	if errResp.Code != constants.ErrCodeBulkDownloadEmpty {
		t.Errorf("expected error code %s, got %s", constants.ErrCodeBulkDownloadEmpty, errResp.Code)
	}
}

// TestBulkDownload_MissingAsset tests partial download when some assets don't exist
func TestBulkDownload_MissingAsset(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "test-topic")

	// Upload one real file
	content := []byte("real content")
	uploadResp := ts.UploadFileExpectSuccess(t, "test-topic", "real.txt", content, "")

	// Try to download with one real and one fake hash
	fakeHash := "0000000000000000000000000000000000000000000000000000000000000000"
	zipBytes := ts.BulkDownloadExpectSuccess(t, BulkDownloadRequest{
		Mode:     "ids",
		AssetIDs: []string{uploadResp.Hash, fakeHash},
	})

	// Verify partial download - real file included, fake in failed
	manifest := ExtractZIPManifest(t, zipBytes)
	if manifest.AssetCount != 1 {
		t.Errorf("expected 1 successful asset, got %d", manifest.AssetCount)
	}
	// Note: The fake hash won't appear in failed_assets because it's filtered out during resolution
	// before streaming starts. This is by design - we skip non-existent assets.
}

// TestBulkDownload_NotConfigured tests error when working directory not set
func TestBulkDownload_NotConfigured(t *testing.T) {
	ts := StartTestServer(t)
	// Without configuration, auth store doesn't exist so request returns 401

	resp, err := ts.UnauthenticatedPOST("/api/download/bulk", BulkDownloadRequest{
		Mode:     "ids",
		AssetIDs: []string{"somehash"},
	})
	if err != nil {
		t.Fatalf("bulk download request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status %d (auth required before config), got %d", http.StatusUnauthorized, resp.StatusCode)
	}
}

// TestBulkDownload_InvalidMode tests error for invalid mode
func TestBulkDownload_InvalidMode(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	errResp := ts.BulkDownloadExpectError(t, BulkDownloadRequest{
		Mode: "invalid",
	}, 400)

	if errResp.Code != constants.ErrCodeInvalidDownloadMode {
		t.Errorf("expected error code %s, got %s", constants.ErrCodeInvalidDownloadMode, errResp.Code)
	}
}

// TestBulkDownload_InvalidFilenameFormat tests error for invalid filename format
func TestBulkDownload_InvalidFilenameFormat(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	errResp := ts.BulkDownloadExpectError(t, BulkDownloadRequest{
		Mode:           "ids",
		AssetIDs:       []string{"somehash"},
		FilenameFormat: "invalid_format",
	}, 400)

	if errResp.Code != constants.ErrCodeInvalidFilenameFormat {
		t.Errorf("expected error code %s, got %s", constants.ErrCodeInvalidFilenameFormat, errResp.Code)
	}
}

// TestBulkDownload_ManifestCorrectness tests manifest accuracy
func TestBulkDownload_ManifestCorrectness(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "test-topic")

	// Upload files with known sizes
	content1 := []byte("content one")
	content2 := []byte("content two longer")

	resp1 := ts.UploadFileExpectSuccess(t, "test-topic", "file1.txt", content1, "")
	resp2 := ts.UploadFileExpectSuccess(t, "test-topic", "file2.txt", content2, "")

	zipBytes := ts.BulkDownloadExpectSuccess(t, BulkDownloadRequest{
		Mode:     "ids",
		AssetIDs: []string{resp1.Hash, resp2.Hash},
	})

	manifest := ExtractZIPManifest(t, zipBytes)

	// Verify counts
	if manifest.AssetCount != 2 {
		t.Errorf("expected asset_count 2, got %d", manifest.AssetCount)
	}

	// Verify total size matches sum of individual sizes
	expectedTotalSize := int64(len(content1) + len(content2))
	if manifest.TotalSize != expectedTotalSize {
		t.Errorf("expected total_size %d, got %d", expectedTotalSize, manifest.TotalSize)
	}

	// Verify each asset has correct size
	sizeMap := make(map[string]int64)
	for _, asset := range manifest.Assets {
		sizeMap[asset.Hash] = asset.Size
	}

	if sizeMap[resp1.Hash] != int64(len(content1)) {
		t.Errorf("file1 size mismatch: expected %d, got %d", len(content1), sizeMap[resp1.Hash])
	}
	if sizeMap[resp2.Hash] != int64(len(content2)) {
		t.Errorf("file2 size mismatch: expected %d, got %d", len(content2), sizeMap[resp2.Hash])
	}

	// Verify created_at is recent
	if manifest.CreatedAt == 0 {
		t.Error("created_at should not be zero")
	}
}

// TestBulkDownload_Immutability tests that download doesn't modify source files
func TestBulkDownload_Immutability(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "test-topic")

	// Upload a file
	content := []byte("immutability test content")
	uploadResp := ts.UploadFileExpectSuccess(t, "test-topic", "immutable.txt", content, "")

	// Get checksum of .dat file before download
	datPath := filepath.Join(ts.WorkDir, "test-topic", "000001.dat")
	datBefore, err := os.ReadFile(datPath)
	if err != nil {
		t.Fatalf("failed to read dat file: %v", err)
	}
	checksumBefore := sha256.Sum256(datBefore)

	// Perform bulk download
	ts.BulkDownloadExpectSuccess(t, BulkDownloadRequest{
		Mode:            "ids",
		AssetIDs:        []string{uploadResp.Hash},
		IncludeMetadata: true, // Also include metadata
	})

	// Get checksum of .dat file after download
	datAfter, err := os.ReadFile(datPath)
	if err != nil {
		t.Fatalf("failed to read dat file after download: %v", err)
	}
	checksumAfter := sha256.Sum256(datAfter)

	// Verify no modification
	if checksumBefore != checksumAfter {
		t.Error("dat file was modified during bulk download - immutability violated")
	}
}

// TestBulkDownload_ContentIntegrity tests byte-for-byte content match
func TestBulkDownload_ContentIntegrity(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "test-topic")

	// Upload file with known content
	originalContent := make([]byte, 5000)
	for i := range originalContent {
		originalContent[i] = byte(i % 256)
	}
	uploadResp := ts.UploadFileExpectSuccess(t, "test-topic", "binary.bin", originalContent, "")

	// Download via bulk
	zipBytes := ts.BulkDownloadExpectSuccess(t, BulkDownloadRequest{
		Mode:           "ids",
		AssetIDs:       []string{uploadResp.Hash},
		FilenameFormat: "hash",
	})

	// Extract and verify content
	filename := "assets/" + uploadResp.Hash + ".bin"
	downloadedContent := ExtractZIPFile(t, zipBytes, filename)

	if len(downloadedContent) != len(originalContent) {
		t.Fatalf("content length mismatch: expected %d, got %d", len(originalContent), len(downloadedContent))
	}

	if !bytes.Equal(downloadedContent, originalContent) {
		// Find first difference
		for i := range originalContent {
			if i >= len(downloadedContent) || originalContent[i] != downloadedContent[i] {
				t.Fatalf("content mismatch at byte %d: expected %d, got %d", i, originalContent[i], downloadedContent[i])
			}
		}
	}
}

// TestBulkDownload_LargeAsset tests downloading a large file
func TestBulkDownload_LargeAsset(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "test-topic")

	// Create a 1MB file
	largeContent := make([]byte, 1024*1024)
	for i := range largeContent {
		largeContent[i] = byte(i % 256)
	}

	uploadResp := ts.UploadFileExpectSuccess(t, "test-topic", "large.bin", largeContent, "")

	// Download via bulk
	zipBytes := ts.BulkDownloadExpectSuccess(t, BulkDownloadRequest{
		Mode:           "ids",
		AssetIDs:       []string{uploadResp.Hash},
		FilenameFormat: "hash",
	})

	// Verify content
	filename := "assets/" + uploadResp.Hash + ".bin"
	downloadedContent := ExtractZIPFile(t, zipBytes, filename)

	if len(downloadedContent) != len(largeContent) {
		t.Errorf("large file size mismatch: expected %d, got %d", len(largeContent), len(downloadedContent))
	}

	// Verify first and last bytes
	if downloadedContent[0] != largeContent[0] {
		t.Error("first byte mismatch")
	}
	if downloadedContent[len(downloadedContent)-1] != largeContent[len(largeContent)-1] {
		t.Error("last byte mismatch")
	}
}

// TestBulkDownload_QueryModeEmptyAssetIDs tests that mode=ids requires asset_ids
func TestBulkDownload_QueryModeEmptyAssetIDs(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "test-topic")

	errResp := ts.BulkDownloadExpectError(t, BulkDownloadRequest{
		Mode:     "ids",
		AssetIDs: []string{}, // Empty
	}, 400)

	if errResp.Code != constants.ErrCodeInvalidRequest {
		t.Errorf("expected error code %s, got %s", constants.ErrCodeInvalidRequest, errResp.Code)
	}
}

// TestBulkDownload_PresetNotFound tests error when preset doesn't exist
func TestBulkDownload_PresetNotFound(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "test-topic")

	errResp := ts.BulkDownloadExpectError(t, BulkDownloadRequest{
		Mode:   "query",
		Preset: "nonexistent-preset",
	}, 400)

	// Should return invalid request since preset is not found
	if errResp.Code != constants.ErrCodeInvalidRequest {
		t.Errorf("expected error code %s, got %s", constants.ErrCodeInvalidRequest, errResp.Code)
	}
}

// TestBulkDownload_DefaultFilenameFormat tests that default is "original"
func TestBulkDownload_DefaultFilenameFormat(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "test-topic")

	content := []byte("test content")
	uploadResp := ts.UploadFileExpectSuccess(t, "test-topic", "my_file.txt", content, "")

	// Download without specifying filename_format
	zipBytes := ts.BulkDownloadExpectSuccess(t, BulkDownloadRequest{
		Mode:     "ids",
		AssetIDs: []string{uploadResp.Hash},
		// FilenameFormat not specified - should default to "original"
	})

	// Verify filename uses original name
	expectedFilename := "assets/my_file.txt"
	_ = ExtractZIPFile(t, zipBytes, expectedFilename) // Will fail if not found
}

// TestBulkDownload_ZIPContentType tests correct Content-Type header
func TestBulkDownload_ZIPContentType(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "test-topic")

	content := []byte("test")
	uploadResp := ts.UploadFileExpectSuccess(t, "test-topic", "file.txt", content, "")

	resp, err := ts.BulkDownload(t, BulkDownloadRequest{
		Mode:     "ids",
		AssetIDs: []string{uploadResp.Hash},
	})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	if contentType != constants.MimeTypeZIP {
		t.Errorf("expected Content-Type %s, got %s", constants.MimeTypeZIP, contentType)
	}

	contentDisposition := resp.Header.Get("Content-Disposition")
	if !strings.Contains(contentDisposition, "attachment") {
		t.Errorf("expected attachment disposition, got %s", contentDisposition)
	}
}

// TestBulkDownload_NoExtension tests files without extension
func TestBulkDownload_NoExtension(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "test-topic")

	content := []byte("file without extension")
	uploadResp := ts.UploadFileExpectSuccess(t, "test-topic", "LICENSE", content, "")

	zipBytes := ts.BulkDownloadExpectSuccess(t, BulkDownloadRequest{
		Mode:           "ids",
		AssetIDs:       []string{uploadResp.Hash},
		FilenameFormat: "original",
	})

	// Should work with no extension
	_ = ExtractZIPFile(t, zipBytes, "assets/LICENSE")
}

// Helper to compute hash for comparison
func computeHash(content []byte) string {
	h := sha256.Sum256(content)
	return hex.EncodeToString(h[:])
}
