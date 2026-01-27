package e2e

import (
	"testing"
)

func TestTopicManagement(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Test 1: Create valid topic
	ts.CreateTopic(t, "test-topic")

	// Test 2: Try creating same topic again - should conflict
	resp, err := ts.POST("/api/topics", map[string]string{"name": "test-topic"})
	if err != nil {
		t.Fatalf("POST duplicate topic request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 409 {
		t.Errorf("Expected 409 Conflict for duplicate topic, got: %d", resp.StatusCode)
	}

	// Test 3: Try uppercase - should fail validation
	resp, err = ts.POST("/api/topics", map[string]string{"name": "INVALID"})
	if err != nil {
		t.Fatalf("POST uppercase topic request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Errorf("Expected 400 Bad Request for uppercase, got: %d", resp.StatusCode)
	}

	// Test 4: Try special characters - should fail validation
	resp, err = ts.POST("/api/topics", map[string]string{"name": "special@chars"})
	if err != nil {
		t.Fatalf("POST special chars topic request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Errorf("Expected 400 Bad Request for special chars, got: %d", resp.StatusCode)
	}

	// Test 5: Single character is valid
	ts.CreateTopic(t, "a")

	// Test 6: Verify topics list
	var topicsResp map[string]interface{}
	err = ts.GetJSON("/api/topics", &topicsResp)
	if err != nil {
		t.Fatalf("GET /api/topics failed: %v", err)
	}

	topics, ok := topicsResp["topics"].([]interface{})
	if !ok {
		t.Fatalf("Expected topics array, got: %T", topicsResp["topics"])
	}

	if len(topics) != 2 {
		t.Errorf("Expected 2 topics, got %d", len(topics))
	}

	// Verify topic names exist
	topicNames := make(map[string]bool)
	for _, topic := range topics {
		topicMap := topic.(map[string]interface{})
		name := topicMap["name"].(string)
		topicNames[name] = true
	}

	if !topicNames["test-topic"] {
		t.Errorf("Topic 'test-topic' not found in list")
	}
	if !topicNames["a"] {
		t.Errorf("Topic 'a' not found in list")
	}
}
