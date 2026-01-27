package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"
)

func TestMultipleSequentialUploads(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "test-topic")

	// Generate varied size files
	testFiles := []struct {
		name string
		size int
	}{
		{"file1.bin", 1024},          // 1 KB
		{"file2.bin", 5 * 1024},      // 5 KB
		{"file3.bin", 10 * 1024},     // 10 KB
		{"file4.bin", 20 * 1024},     // 20 KB
		{"file5.bin", 50 * 1024},     // 50 KB
		{"file6.bin", 100 * 1024},    // 100 KB
		{"file7.bin", 200 * 1024},    // 200 KB
		{"file8.bin", 500 * 1024},    // 500 KB
		{"file9.bin", 1024 * 1024},   // 1 MB
		{"file10.bin", 1024 * 1024},  // 1 MB
	}

	hashes := make([]string, len(testFiles))
	fileContents := make([][]byte, len(testFiles))

	// Upload all files
	for i, tf := range testFiles {
		content := GenerateTestFile(tf.size)
		fileContents[i] = content

		resp, err := ts.UploadFile("test-topic", tf.name, content, "")
		if err != nil {
			t.Fatalf("Upload %d failed: %v", i+1, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 && resp.StatusCode != 201 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Fatalf("Upload %d failed with status %d: %s", i+1, resp.StatusCode, string(bodyBytes))
		}

		var uploadResp map[string]interface{}
		bodyBytes, _ := io.ReadAll(resp.Body)
		json.Unmarshal(bodyBytes, &uploadResp)

		hash, ok := uploadResp["hash"].(string)
		if !ok || hash == "" {
			t.Fatalf("Upload %d: expected hash, got: %v", i+1, uploadResp)
		}
		hashes[i] = hash
	}

	// Download each file and verify content
	for i, hash := range hashes {
		downloadResp, err := ts.GET("/api/assets/" + hash + "/download")
		if err != nil {
			t.Fatalf("Download %d failed: %v", i+1, err)
		}
		defer downloadResp.Body.Close()

		if downloadResp.StatusCode != 200 {
			t.Fatalf("Download %d failed with status %d", i+1, downloadResp.StatusCode)
		}

		downloadedBytes, err := io.ReadAll(downloadResp.Body)
		if err != nil {
			t.Fatalf("Download %d: failed to read response: %v", i+1, err)
		}

		if !bytes.Equal(downloadedBytes, fileContents[i]) {
			t.Errorf("Download %d: content mismatch (expected %d bytes, got %d)",
				i+1, len(fileContents[i]), len(downloadedBytes))
		}
	}

	// Re-upload all files - should be skipped
	for i, tf := range testFiles {
		resp, err := ts.UploadFile("test-topic", tf.name, fileContents[i], "")
		if err != nil {
			t.Fatalf("Re-upload %d failed: %v", i+1, err)
		}
		defer resp.Body.Close()

		var uploadResp map[string]interface{}
		bodyBytes, _ := io.ReadAll(resp.Body)
		json.Unmarshal(bodyBytes, &uploadResp)

		skipped, ok := uploadResp["skipped"].(bool)
		if !ok || !skipped {
			t.Errorf("Re-upload %d: expected skipped: true, got: %v", i+1, uploadResp)
		}

		hash, _ := uploadResp["hash"].(string)
		if hash != hashes[i] {
			t.Errorf("Re-upload %d: hash mismatch (expected %s, got %s)", i+1, hashes[i], hash)
		}
	}
}
