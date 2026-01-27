package e2e

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"meshbank/internal/constants"
	"meshbank/internal/storage"
)

// VerifyEvent represents an SSE event from the verify endpoint
type VerifyEvent struct {
	Type      string                 `json:"type"`
	Timestamp int64                  `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// parseSSEEvents reads SSE events from response body
func parseSSEEvents(t *testing.T, resp *http.Response) []VerifyEvent {
	t.Helper()
	var events []VerifyEvent

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			var event VerifyEvent
			if err := json.Unmarshal([]byte(line[6:]), &event); err != nil {
				t.Errorf("Failed to parse SSE event: %v, line: %s", err, line)
				continue
			}
			events = append(events, event)
		}
	}

	if err := scanner.Err(); err != nil {
		t.Errorf("Error reading SSE stream: %v", err)
	}

	return events
}

// getEventTypes returns a map of event type -> count
func getEventTypes(events []VerifyEvent) map[string]int {
	types := make(map[string]int)
	for _, e := range events {
		types[e.Type]++
	}
	return types
}

// findEvent finds the first event of given type
func findEvent(events []VerifyEvent, eventType string) *VerifyEvent {
	for _, e := range events {
		if e.Type == eventType {
			return &e
		}
	}
	return nil
}

// TestVerifyEndpoint tests basic verification flow with healthy topics
func TestVerifyEndpoint(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "test-topic")

	// Upload some files
	for i := 0; i < 3; i++ {
		content := make([]byte, 1024*(i+1))
		for j := range content {
			content[j] = byte(i + j)
		}
		ts.UploadFileExpectSuccess(t, "test-topic", "file.bin", content, "")
	}

	// Call verify endpoint
	resp, err := ts.GET("/api/verify")
	if err != nil {
		t.Fatalf("Verify request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	// Check content type
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Errorf("Expected text/event-stream, got %s", ct)
	}

	// Parse SSE events
	events := parseSSEEvents(t, resp)
	eventTypes := getEventTypes(events)

	// Verify required events are present
	requiredEvents := []string{"scan_start", "topic_start", "dat_complete", "topic_complete", "index_start", "index_complete", "complete"}
	for _, req := range requiredEvents {
		if eventTypes[req] == 0 {
			t.Errorf("Missing required event type: %s", req)
		}
	}

	// Verify scan_start has correct topics
	scanStart := findEvent(events, "scan_start")
	if scanStart == nil {
		t.Fatal("No scan_start event")
	}
	topics, ok := scanStart.Data["topics"].([]interface{})
	if !ok || len(topics) != 1 {
		t.Errorf("Expected 1 topic, got %v", scanStart.Data["topics"])
	}

	// Verify complete event shows success
	complete := findEvent(events, "complete")
	if complete == nil {
		t.Fatal("No complete event")
	}
	if complete.Data["topics_checked"].(float64) != 1 {
		t.Errorf("Expected 1 topic checked, got %v", complete.Data["topics_checked"])
	}
	if complete.Data["topics_valid"].(float64) != 1 {
		t.Errorf("Expected 1 topic valid, got %v", complete.Data["topics_valid"])
	}
	if complete.Data["index_valid"] != true {
		t.Errorf("Expected index_valid=true, got %v", complete.Data["index_valid"])
	}
}

// TestVerifyWithCorruption tests that corrupted .dat files are detected
func TestVerifyWithCorruption(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "corrupt-topic")

	// Upload a file
	content := make([]byte, 1024)
	for i := range content {
		content[i] = byte(i)
	}
	ts.UploadFileExpectSuccess(t, "corrupt-topic", "file.bin", content, "")

	// Corrupt the .dat file by modifying the hash field in the header
	// Header layout: Magic(4) + Version(2) + DataLength(8) + Hash(64) + Reserved(32) = 110 bytes
	// Hash is at offset 14 (4+2+8=14)
	// This corrupts the running hash chain because the hash field is used in chain computation
	expectedDat := storage.FormatDatFilename(1)
	datPath := filepath.Join(ts.WorkDir, "corrupt-topic", expectedDat)
	datFile, err := os.OpenFile(datPath, os.O_RDWR, 0644)
	if err != nil {
		t.Fatalf("Failed to open dat file: %v", err)
	}

	// Write garbage over part of the hash field - this corrupts the running hash chain
	datFile.WriteAt([]byte("0000000"), 14)
	datFile.Close()

	// Call verify endpoint
	resp, err := ts.GET("/api/verify")
	if err != nil {
		t.Fatalf("Verify request failed: %v", err)
	}
	defer resp.Body.Close()

	events := parseSSEEvents(t, resp)

	// Find dat_complete for first dat file
	var datComplete *VerifyEvent
	for _, e := range events {
		if e.Type == "dat_complete" && e.Data["dat_file"] == expectedDat {
			datComplete = &e
			break
		}
	}

	if datComplete == nil {
		t.Fatalf("No dat_complete event for %s", expectedDat)
	}

	// Should show as invalid with hash mismatch
	if datComplete.Data["valid"] != false {
		t.Errorf("Expected valid=false for corrupted file, got %v", datComplete.Data["valid"])
	}
	errStr, ok := datComplete.Data["error"].(string)
	if !ok || !strings.Contains(errStr, "hash mismatch") {
		t.Errorf("Expected 'hash mismatch' error, got %v", datComplete.Data["error"])
	}

	// topic_complete should show errors
	topicComplete := findEvent(events, "topic_complete")
	if topicComplete == nil {
		t.Fatal("No topic_complete event")
	}
	if topicComplete.Data["valid"] != false {
		t.Errorf("Expected topic valid=false, got %v", topicComplete.Data["valid"])
	}

	// complete should show topics_valid=0
	complete := findEvent(events, "complete")
	if complete == nil {
		t.Fatal("No complete event")
	}
	if complete.Data["topics_valid"].(float64) != 0 {
		t.Errorf("Expected 0 topics valid, got %v", complete.Data["topics_valid"])
	}
}

// TestVerifyTopicsFilter tests the ?topics= query parameter
func TestVerifyTopicsFilter(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Create multiple topics
	ts.CreateTopic(t, "topic-a")
	ts.CreateTopic(t, "topic-b")
	ts.CreateTopic(t, "topic-c")

	// Upload file to each
	content := []byte("test content")
	ts.UploadFileExpectSuccess(t, "topic-a", "file.txt", content, "")
	ts.UploadFileExpectSuccess(t, "topic-b", "file.txt", content, "")
	ts.UploadFileExpectSuccess(t, "topic-c", "file.txt", content, "")

	// Verify only topic-a and topic-c
	resp, err := ts.GET("/api/verify?topics=topic-a,topic-c")
	if err != nil {
		t.Fatalf("Verify request failed: %v", err)
	}
	defer resp.Body.Close()

	events := parseSSEEvents(t, resp)

	// Check scan_start has only 2 topics
	scanStart := findEvent(events, "scan_start")
	topics, _ := scanStart.Data["topics"].([]interface{})
	if len(topics) != 2 {
		t.Errorf("Expected 2 topics, got %d", len(topics))
	}

	// Count topic_start events - should be 2
	eventTypes := getEventTypes(events)
	if eventTypes["topic_start"] != 2 {
		t.Errorf("Expected 2 topic_start events, got %d", eventTypes["topic_start"])
	}

	// Verify topic-b is not in any topic_start
	for _, e := range events {
		if e.Type == "topic_start" && e.Data["topic"] == "topic-b" {
			t.Error("topic-b should not be verified")
		}
	}
}

// TestVerifyCheckIndexFalse tests the ?check_index=false parameter
func TestVerifyCheckIndexFalse(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "test-topic")

	content := []byte("test content")
	ts.UploadFileExpectSuccess(t, "test-topic", "file.txt", content, "")

	// Verify with check_index=false
	resp, err := ts.GET("/api/verify?check_index=false")
	if err != nil {
		t.Fatalf("Verify request failed: %v", err)
	}
	defer resp.Body.Close()

	events := parseSSEEvents(t, resp)
	eventTypes := getEventTypes(events)

	// Should NOT have index_start or index_complete events
	if eventTypes["index_start"] > 0 {
		t.Error("Should not have index_start when check_index=false")
	}
	if eventTypes["index_complete"] > 0 {
		t.Error("Should not have index_complete when check_index=false")
	}

	// scan_start should show check_index=false
	scanStart := findEvent(events, "scan_start")
	if scanStart.Data["check_index"] != false {
		t.Errorf("Expected check_index=false, got %v", scanStart.Data["check_index"])
	}
}

// TestVerifyNotConfigured tests verify before working directory is set
func TestVerifyNotConfigured(t *testing.T) {
	ts := StartTestServer(t)
	// Don't configure working directory

	// Without configuration, auth store doesn't exist so request returns 401
	resp, err := ts.UnauthenticatedGET("/api/verify")
	if err != nil {
		t.Fatalf("Verify request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 (auth required before config), got %d", resp.StatusCode)
	}
}

// TestVerifyTopicNotFound tests verify with non-existent topic filter
func TestVerifyTopicNotFound(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	resp, err := ts.GET("/api/verify?topics=nonexistent")
	if err != nil {
		t.Fatalf("Verify request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 404 {
		t.Errorf("Expected 404, got %d", resp.StatusCode)
	}

	var errResp ErrorResponse
	json.NewDecoder(resp.Body).Decode(&errResp)
	if errResp.Code != constants.ErrCodeTopicNotFound {
		t.Errorf("Expected TOPIC_NOT_FOUND error code, got %s", errResp.Code)
	}
}

// TestVerifyEmptyTopic tests verify on topic with no files
func TestVerifyEmptyTopic(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "empty-topic")

	resp, err := ts.GET("/api/verify")
	if err != nil {
		t.Fatalf("Verify request failed: %v", err)
	}
	defer resp.Body.Close()

	events := parseSSEEvents(t, resp)

	// Should have topic_start with 0 dat files
	topicStart := findEvent(events, "topic_start")
	if topicStart == nil {
		t.Fatal("No topic_start event")
	}
	if topicStart.Data["dat_files"].(float64) != 0 {
		t.Errorf("Expected 0 dat_files, got %v", topicStart.Data["dat_files"])
	}

	// topic_complete should be valid (no files to verify = success)
	topicComplete := findEvent(events, "topic_complete")
	if topicComplete == nil {
		t.Fatal("No topic_complete event")
	}
	if topicComplete.Data["valid"] != true {
		t.Errorf("Expected valid=true for empty topic, got %v", topicComplete.Data["valid"])
	}
}

// TestVerifyMultipleTopics tests verify with multiple topics
func TestVerifyMultipleTopics(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Create 3 topics with files
	for _, name := range []string{"alpha", "beta", "gamma"} {
		ts.CreateTopic(t, name)
		content := []byte("content for " + name)
		ts.UploadFileExpectSuccess(t, name, "file.txt", content, "")
	}

	resp, err := ts.GET("/api/verify")
	if err != nil {
		t.Fatalf("Verify request failed: %v", err)
	}
	defer resp.Body.Close()

	events := parseSSEEvents(t, resp)
	eventTypes := getEventTypes(events)

	// Should have 3 topic_start and 3 topic_complete events
	if eventTypes["topic_start"] != 3 {
		t.Errorf("Expected 3 topic_start events, got %d", eventTypes["topic_start"])
	}
	if eventTypes["topic_complete"] != 3 {
		t.Errorf("Expected 3 topic_complete events, got %d", eventTypes["topic_complete"])
	}

	// complete should show all 3 valid
	complete := findEvent(events, "complete")
	if complete.Data["topics_checked"].(float64) != 3 {
		t.Errorf("Expected 3 topics checked, got %v", complete.Data["topics_checked"])
	}
	if complete.Data["topics_valid"].(float64) != 3 {
		t.Errorf("Expected 3 topics valid, got %v", complete.Data["topics_valid"])
	}
}

// TestVerifyMethodNotAllowed tests that only GET is allowed
func TestVerifyMethodNotAllowed(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	resp, err := ts.POST("/api/verify", nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 405 {
		t.Errorf("Expected 405 Method Not Allowed, got %d", resp.StatusCode)
	}
}
