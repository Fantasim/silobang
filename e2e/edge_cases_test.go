package e2e

import (
	"bytes"
	"strings"
	"testing"
)

func TestFileSizeBoundaries(t *testing.T) {
	// Use a small max_dat_size for fast tests (10KB)
	maxSize := int64(10 * 1024) // 10KB
	ts := StartTestServerWithMaxSize(t, maxSize)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "size-test")

	// The actual limit accounts for header overhead (110 bytes per entry)
	// So max uploadable file is maxSize - 110
	headerSize := int64(110)
	maxFileSize := maxSize - headerSize

	// Test 1: Upload 0 byte file - should succeed
	t.Run("zero byte file", func(t *testing.T) {
		resp := ts.UploadFileExpectSuccess(t, "size-test", "empty.bin", []byte{}, "")
		if resp.Hash == "" {
			t.Error("Expected hash for empty file")
		}
	})

	// Test 2: Upload 1 byte file - should succeed
	t.Run("one byte file", func(t *testing.T) {
		resp := ts.UploadFileExpectSuccess(t, "size-test", "one.bin", []byte{0x42}, "")
		if resp.Hash == "" {
			t.Error("Expected hash for 1-byte file")
		}
	})

	// Test 3: Upload exactly max size - should succeed
	t.Run("exact max size", func(t *testing.T) {
		content := bytes.Repeat([]byte{0xAB}, int(maxFileSize))
		resp := ts.UploadFileExpectSuccess(t, "size-test", "maxsize.bin", content, "")
		if resp.Hash == "" {
			t.Error("Expected hash for max-size file")
		}
	})

	// Test 4: Upload max size + 1 byte - should fail with 413
	t.Run("over max size", func(t *testing.T) {
		content := bytes.Repeat([]byte{0xCD}, int(maxFileSize)+1)
		errResp := ts.UploadFileExpectError(t, "size-test", "tooarge.bin", content, "", 413)
		if errResp.Code != "ASSET_TOO_LARGE" {
			t.Errorf("Expected error code ASSET_TOO_LARGE, got: %s", errResp.Code)
		}
	})
}

func TestSpecialFilenames(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "filename-test")

	testCases := []struct {
		filename    string
		description string
	}{
		{"my file.bin", "filename with spaces"},
		{"文件.bin", "filename with unicode"},
		{"noext", "filename with no extension"},
		{"file.test.bin", "filename with multiple dots"},
		{"file-with-dashes.txt", "filename with dashes"},
		{"file_with_underscores.dat", "filename with underscores"},
		{".hidden", "hidden file (starts with dot)"},
		{"UPPERCASE.BIN", "uppercase filename"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			// Generate unique content for each file to avoid dedup
			content := []byte("content for: " + tc.filename)
			resp := ts.UploadFileExpectSuccess(t, "filename-test", tc.filename, content, "")

			if resp.Hash == "" {
				t.Error("Expected hash in response")
			}

			// Verify we can download it back
			downloaded := ts.DownloadAsset(t, resp.Hash)
			if !bytes.Equal(downloaded, content) {
				t.Error("Downloaded content doesn't match uploaded content")
			}
		})
	}

	// Test very long filename (255 chars is common filesystem limit)
	t.Run("very long filename", func(t *testing.T) {
		longName := strings.Repeat("a", 250) + ".bin"
		content := []byte("long filename content")
		resp := ts.UploadFileExpectSuccess(t, "filename-test", longName, content, "")
		if resp.Hash == "" {
			t.Error("Expected hash for long filename")
		}
	})
}

func TestMetadataEdgeCases(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "metadata-edge")

	// Upload a file to test metadata on
	uploadResp := ts.UploadFileExpectSuccess(t, "metadata-edge", "test.bin", []byte("test content"), "")
	hash := uploadResp.Hash

	testCases := []struct {
		key         string
		value       interface{}
		description string
	}{
		{"normal_key", "normal value", "normal string value"},
		{"numeric_zero", 0, "numeric value 0"},
		{"negative_num", -123, "negative number"},
		{"float_value", 3.14159265358979, "float with many decimals"},
		{"bool_true", true, "boolean true"},
		{"bool_false", false, "boolean false"},
		{"large_int", 9999999999, "large integer"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			metaResp := ts.SetMetadata(t, hash, tc.key, tc.value)
			if !metaResp.Success {
				t.Errorf("Expected success for %s", tc.description)
			}
		})
	}

	// Test empty string value - should fail (API rejects empty strings)
	t.Run("empty string value rejected", func(t *testing.T) {
		errResp := ts.SetMetadataExpectError(t, hash, "empty_key", "", 500)
		if errResp.Code != "METADATA_ERROR" {
			t.Errorf("Expected error code METADATA_ERROR for empty value, got: %s", errResp.Code)
		}
	})

	// Test very long key name
	t.Run("very long key name", func(t *testing.T) {
		longKey := strings.Repeat("k", 256)
		metaResp := ts.SetMetadata(t, hash, longKey, "value")
		if !metaResp.Success {
			t.Error("Expected success for long key name")
		}
	})

	// Test very long value
	t.Run("very long value", func(t *testing.T) {
		longValue := strings.Repeat("v", 10000)
		metaResp := ts.SetMetadata(t, hash, "long_value_key", longValue)
		if !metaResp.Success {
			t.Error("Expected success for long value")
		}
	})
}

func TestQueryEmptyTopic(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Create a topic but don't upload anything
	ts.CreateTopic(t, "empty-topic")

	// Run recent-imports query on empty topic
	result := ts.ExecuteQuery(t, "recent-imports", []string{"empty-topic"}, map[string]interface{}{
		"days": 7,
	})

	// Should return empty rows, not an error
	if result.RowCount != 0 {
		t.Errorf("Expected 0 rows for empty topic, got: %d", result.RowCount)
	}

	if result.Rows == nil {
		t.Error("Expected empty rows array, got nil")
	}
}

func TestQueryAllTopicsEmpty(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Create multiple empty topics
	ts.CreateTopic(t, "empty1")
	ts.CreateTopic(t, "empty2")

	// Query all topics (empty topics array means all)
	result := ts.ExecuteQuery(t, "recent-imports", []string{}, map[string]interface{}{
		"days": 7,
	})

	// Should return empty rows, not an error
	if result.RowCount != 0 {
		t.Errorf("Expected 0 rows when all topics are empty, got: %d", result.RowCount)
	}
}
