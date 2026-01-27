package e2e

import (
	"encoding/json"
	"io"
	"testing"
)

func TestCrossTopicQueries(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Create two topics
	ts.CreateTopic(t, "topic-1")
	ts.CreateTopic(t, "topic-2")

	// Upload 10 files to topic-1
	topic1Hashes := make([]string, 10)
	for i := 0; i < 10; i++ {
		content := GenerateTestFile(1024 * (i + 1))
		resp, err := ts.UploadFile("topic-1", "file1.bin", content, "")
		if err != nil {
			t.Fatalf("Upload to topic-1 failed: %v", err)
		}
		defer resp.Body.Close()

		var uploadResp map[string]interface{}
		bodyBytes, _ := io.ReadAll(resp.Body)
		json.Unmarshal(bodyBytes, &uploadResp)
		topic1Hashes[i] = uploadResp["hash"].(string)
	}

	// Upload 10 different files to topic-2
	topic2Hashes := make([]string, 10)
	for i := 0; i < 10; i++ {
		content := GenerateTestFile(2048 * (i + 1))
		resp, err := ts.UploadFile("topic-2", "file2.bin", content, "")
		if err != nil {
			t.Fatalf("Upload to topic-2 failed: %v", err)
		}
		defer resp.Body.Close()

		var uploadResp map[string]interface{}
		bodyBytes, _ := io.ReadAll(resp.Body)
		json.Unmarshal(bodyBytes, &uploadResp)
		topic2Hashes[i] = uploadResp["hash"].(string)
	}

	// Query all topics (empty topics array)
	resp, err := ts.POST("/api/query/recent-imports", map[string]interface{}{
		"topics": []string{},
		"params": map[string]interface{}{
			"days":  7,
			"limit": 100,
		},
	})
	if err != nil {
		t.Fatalf("Cross-topic query failed: %v", err)
	}
	defer resp.Body.Close()

	var allResp map[string]interface{}
	bodyBytes, _ := io.ReadAll(resp.Body)
	json.Unmarshal(bodyBytes, &allResp)

	rows, ok := allResp["rows"].([]interface{})
	if !ok {
		t.Fatalf("Expected rows array, got: %v", allResp)
	}

	// Should return 20 results (10 from each topic)
	if len(rows) != 20 {
		t.Errorf("Expected 20 results from cross-topic query, got %d", len(rows))
	}

	// Verify each result has _topic column
	columns := allResp["columns"].([]interface{})
	topicIdx := -1
	for i, col := range columns {
		if col.(string) == "_topic" {
			topicIdx = i
			break
		}
	}
	if topicIdx == -1 {
		t.Fatal("_topic column not found")
	}

	topicCounts := make(map[string]int)
	for _, row := range rows {
		rowArray := row.([]interface{})
		topic := rowArray[topicIdx].(string)
		topicCounts[topic]++
	}

	if topicCounts["topic-1"] != 10 {
		t.Errorf("Expected 10 results from topic-1, got %d", topicCounts["topic-1"])
	}
	if topicCounts["topic-2"] != 10 {
		t.Errorf("Expected 10 results from topic-2, got %d", topicCounts["topic-2"])
	}

	// Query specific topic only
	resp, err = ts.POST("/api/query/recent-imports", map[string]interface{}{
		"topics": []string{"topic-1"},
		"params": map[string]interface{}{
			"days":  7,
			"limit": 100,
		},
	})
	if err != nil {
		t.Fatalf("Single topic query failed: %v", err)
	}
	defer resp.Body.Close()

	var singleResp map[string]interface{}
	bodyBytes, _ = io.ReadAll(resp.Body)
	json.Unmarshal(bodyBytes, &singleResp)

	rows, ok = singleResp["rows"].([]interface{})
	if !ok {
		t.Fatalf("Expected rows array, got: %v", singleResp)
	}

	// Should return only 10 results from topic-1
	if len(rows) != 10 {
		t.Errorf("Expected 10 results from topic-1, got %d", len(rows))
	}

	// Verify all have _topic: "topic-1"
	columns = singleResp["columns"].([]interface{})
	topicIdx = -1
	for i, col := range columns {
		if col.(string) == "_topic" {
			topicIdx = i
			break
		}
	}
	if topicIdx == -1 {
		t.Fatal("_topic column not found in single topic query")
	}

	for _, row := range rows {
		rowArray := row.([]interface{})
		topic := rowArray[topicIdx].(string)
		if topic != "topic-1" {
			t.Errorf("Expected _topic: topic-1, got: %s", topic)
		}
	}
}
