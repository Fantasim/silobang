package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"
)

func TestTopicDiscovery(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "test-topic")

	// Upload 3 files
	hashes := make([]string, 3)
	contents := make([][]byte, 3)

	for i := 0; i < 3; i++ {
		contents[i] = GenerateTestFile(1024 * (i + 1))
		resp, err := ts.UploadFile("test-topic", "file.bin", contents[i], "")
		if err != nil {
			t.Fatalf("Upload %d failed: %v", i, err)
		}
		defer resp.Body.Close()

		var uploadResp map[string]interface{}
		bodyBytes, _ := io.ReadAll(resp.Body)
		json.Unmarshal(bodyBytes, &uploadResp)
		hashes[i] = uploadResp["hash"].(string)
	}

	// Get topic stats before restart
	var topicsBefore map[string]interface{}
	err := ts.GetJSON("/api/topics", &topicsBefore)
	if err != nil {
		t.Fatalf("GET /api/topics failed: %v", err)
	}

	topicsList := topicsBefore["topics"].([]interface{})
	if len(topicsList) != 1 {
		t.Fatalf("Expected 1 topic, got %d", len(topicsList))
	}

	topicInfo := topicsList[0].(map[string]interface{})
	stats := topicInfo["stats"].(map[string]interface{})
	fileCountBefore := stats["file_count"].(float64)
	totalSizeBefore := stats["total_size"].(float64)

	// Restart server
	ts.Restart(t)

	// Verify topic rediscovered
	var topicsAfter map[string]interface{}
	err = ts.GetJSON("/api/topics", &topicsAfter)
	if err != nil {
		t.Fatalf("GET /api/topics after restart failed: %v", err)
	}

	topicsListAfter := topicsAfter["topics"].([]interface{})
	if len(topicsListAfter) != 1 {
		t.Fatalf("Expected 1 topic after restart, got %d", len(topicsListAfter))
	}

	topicInfoAfter := topicsListAfter[0].(map[string]interface{})

	if topicInfoAfter["name"].(string) != "test-topic" {
		t.Errorf("Expected topic name: test-topic, got: %v", topicInfoAfter["name"])
	}

	// Verify stats match
	statsAfter := topicInfoAfter["stats"].(map[string]interface{})
	fileCountAfter := statsAfter["file_count"].(float64)
	totalSizeAfter := statsAfter["total_size"].(float64)

	if fileCountAfter != fileCountBefore {
		t.Errorf("File count mismatch after restart: before=%f, after=%f", fileCountBefore, fileCountAfter)
	}

	if totalSizeAfter != totalSizeBefore {
		t.Errorf("Total size mismatch after restart: before=%f, after=%f", totalSizeBefore, totalSizeAfter)
	}

	// Verify files still downloadable
	for i, hash := range hashes {
		downloadResp, err := ts.GET("/api/assets/" + hash + "/download")
		if err != nil {
			t.Fatalf("Download %d after restart failed: %v", i, err)
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
			t.Errorf("Download %d content mismatch after restart", i)
		}
	}

	// Verify healthy status
	healthy, ok := topicInfoAfter["healthy"].(bool)
	if !ok || !healthy {
		t.Errorf("Expected topic to be healthy after restart, got: %v", topicInfoAfter["healthy"])
	}
}
