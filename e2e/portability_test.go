package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestPortableTopic(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "portable-topic")

	// Upload files
	hashes := make([]string, 3)
	contents := make([][]byte, 3)

	for i := 0; i < 3; i++ {
		contents[i] = GenerateTestFile(1024 * (i + 1))
		resp, err := ts.UploadFile("portable-topic", "file.bin", contents[i], "")
		if err != nil {
			t.Fatalf("Upload %d failed: %v", i, err)
		}
		defer resp.Body.Close()

		var uploadResp map[string]interface{}
		bodyBytes, _ := io.ReadAll(resp.Body)
		json.Unmarshal(bodyBytes, &uploadResp)
		hashes[i] = uploadResp["hash"].(string)
	}

	// Create second working directory
	workDir2, err := os.MkdirTemp("", "silobang-test-work2-*")
	if err != nil {
		t.Fatalf("Failed to create second work dir: %v", err)
	}
	defer os.RemoveAll(workDir2)

	// Copy topic folder to new location
	srcTopicPath := filepath.Join(ts.WorkDir, "portable-topic")
	dstTopicPath := filepath.Join(workDir2, "portable-topic")

	err = copyDir(srcTopicPath, dstTopicPath)
	if err != nil {
		t.Fatalf("Failed to copy topic folder: %v", err)
	}

	// Shutdown first server
	ts.Shutdown()

	// Start new server with second working directory
	ts2 := StartTestServer(t)
	ts2.WorkDir = workDir2
	ts2.ConfigureWorkDir(t)

	// Verify topic discovered
	var topicsResp map[string]interface{}
	err = ts2.GetJSON("/api/topics", &topicsResp)
	if err != nil {
		t.Fatalf("GET /api/topics on second server failed: %v", err)
	}

	topics := topicsResp["topics"].([]interface{})
	if len(topics) != 1 {
		t.Fatalf("Expected 1 topic in second server, got %d", len(topics))
	}

	topicInfo := topics[0].(map[string]interface{})
	if topicInfo["name"].(string) != "portable-topic" {
		t.Errorf("Expected topic name: portable-topic, got: %v", topicInfo["name"])
	}

	// Verify files downloadable
	for i, hash := range hashes {
		downloadResp, err := ts2.GET("/api/assets/" + hash + "/download")
		if err != nil {
			t.Fatalf("Download %d failed: %v", i, err)
		}
		defer downloadResp.Body.Close()

		if downloadResp.StatusCode != 200 {
			t.Fatalf("Download %d failed with status %d", i, downloadResp.StatusCode)
		}

		downloadedBytes, err := io.ReadAll(downloadResp.Body)
		if err != nil {
			t.Fatalf("Failed to read download %d: %v", i, err)
		}

		if !bytes.Equal(downloadedBytes, contents[i]) {
			t.Errorf("Download %d content mismatch in new location", i)
		}
	}

	// Upload new file to verify writes work
	newFile := GenerateTestFile(4096)
	uploadResp, err := ts2.UploadFile("portable-topic", "new.bin", newFile, "")
	if err != nil {
		t.Fatalf("Upload to portable topic failed: %v", err)
	}
	defer uploadResp.Body.Close()

	if uploadResp.StatusCode != 200 && uploadResp.StatusCode != 201 {
		bodyBytes, _ := io.ReadAll(uploadResp.Body)
		t.Fatalf("Upload failed with status %d: %s", uploadResp.StatusCode, string(bodyBytes))
	}
}

// copyDir recursively copies a directory
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dstPath, data, info.Mode())
	})
}
