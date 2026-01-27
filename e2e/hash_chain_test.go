package e2e

import (
	"sync"
	"testing"
)

// TestHashChainIntegritySequential verifies hash chain works correctly for sequential uploads
// This is the basic test that ensures the fix for the race condition works
func TestHashChainIntegritySequential(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "hash-test")

	// Upload multiple files sequentially
	for i := 0; i < 5; i++ {
		content := GenerateTestFile(1024 * (i + 1))
		ts.UploadFileExpectSuccess(t, "hash-test", "file.bin", content, "")
	}

	// Restart server - this triggers VerifyAllDatHashes during topic discovery
	ts.Restart(t)

	// Verify topic is healthy after restart
	topics := ts.GetTopics(t)
	found := false
	for _, topic := range topics.Topics {
		if topic.Name == "hash-test" {
			found = true
			if !topic.Healthy {
				t.Errorf("Topic should be healthy after restart, error: %s", topic.Error)
			}
		}
	}
	if !found {
		t.Error("Topic not found after restart")
	}
}

// TestHashChainIntegrityConcurrent verifies hash chain works correctly for concurrent uploads
// This is the critical test that catches the race condition bug
func TestHashChainIntegrityConcurrent(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "concurrent-hash-test")

	const numUploads = 10
	var wg sync.WaitGroup
	errChan := make(chan error, numUploads)

	// Upload files concurrently - this is where the race condition would occur
	for i := 0; i < numUploads; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			content := GenerateTestFile(512 * (idx + 1))
			resp, err := ts.UploadFile("concurrent-hash-test", "file.bin", content, "")
			if err != nil {
				errChan <- err
				return
			}
			resp.Body.Close()
			if resp.StatusCode != 200 && resp.StatusCode != 201 {
				t.Logf("Upload %d returned status %d", idx, resp.StatusCode)
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	// Check for upload errors
	for err := range errChan {
		t.Errorf("Upload error: %v", err)
	}

	// Restart server - this is when the hash mismatch would be detected
	ts.Restart(t)

	// Verify topic is healthy after restart
	topics := ts.GetTopics(t)
	for _, topic := range topics.Topics {
		if topic.Name == "concurrent-hash-test" {
			if !topic.Healthy {
				t.Errorf("Topic should be healthy after concurrent uploads and restart, error: %s", topic.Error)
			}
			return
		}
	}
	t.Error("Topic not found after restart")
}

// TestHashChainMultipleRestarts verifies hash chain survives multiple restart cycles
func TestHashChainMultipleRestarts(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "multi-restart")

	// Upload, restart, upload more, restart again - repeat
	for round := 0; round < 3; round++ {
		// Upload some files in each round
		for i := 0; i < 3; i++ {
			content := GenerateTestFile(1024)
			ts.UploadFileExpectSuccess(t, "multi-restart", "file.bin", content, "")
		}

		// Restart and verify health
		ts.Restart(t)

		topics := ts.GetTopics(t)
		for _, topic := range topics.Topics {
			if topic.Name == "multi-restart" {
				if !topic.Healthy {
					t.Fatalf("Topic unhealthy after restart round %d: %s", round+1, topic.Error)
				}
				break
			}
		}
	}
}

// TestHashChainMultipleTopicsConcurrent tests concurrent uploads across multiple topics
func TestHashChainMultipleTopicsConcurrent(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	topicNames := []string{"topic-a", "topic-b", "topic-c"}
	for _, name := range topicNames {
		ts.CreateTopic(t, name)
	}

	const uploadsPerTopic = 5
	var wg sync.WaitGroup

	// Upload concurrently to all topics
	for _, topicName := range topicNames {
		for i := 0; i < uploadsPerTopic; i++ {
			wg.Add(1)
			go func(topic string, idx int) {
				defer wg.Done()
				content := GenerateTestFile(512 * (idx + 1))
				resp, err := ts.UploadFile(topic, "file.bin", content, "")
				if err != nil {
					t.Errorf("Upload to %s failed: %v", topic, err)
					return
				}
				resp.Body.Close()
			}(topicName, i)
		}
	}

	wg.Wait()

	// Restart and verify all topics are healthy
	ts.Restart(t)

	topics := ts.GetTopics(t)
	for _, expectedName := range topicNames {
		found := false
		for _, topic := range topics.Topics {
			if topic.Name == expectedName {
				found = true
				if !topic.Healthy {
					t.Errorf("Topic %s should be healthy after restart, error: %s", expectedName, topic.Error)
				}
				break
			}
		}
		if !found {
			t.Errorf("Topic %s not found after restart", expectedName)
		}
	}
}

// TestHashChainEmptyTopicRestart ensures empty topics don't cause hash issues
func TestHashChainEmptyTopicRestart(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "empty-topic")

	// Restart without uploading anything
	ts.Restart(t)

	topics := ts.GetTopics(t)
	for _, topic := range topics.Topics {
		if topic.Name == "empty-topic" {
			if !topic.Healthy {
				t.Errorf("Empty topic should be healthy after restart, error: %s", topic.Error)
			}
			return
		}
	}
	t.Error("Empty topic not found after restart")
}

// TestHashChainSingleFileRestart tests the simplest case - one file, one restart
func TestHashChainSingleFileRestart(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "single-file")

	content := GenerateTestFile(1024)
	ts.UploadFileExpectSuccess(t, "single-file", "test.bin", content, "")

	ts.Restart(t)

	topics := ts.GetTopics(t)
	for _, topic := range topics.Topics {
		if topic.Name == "single-file" {
			if !topic.Healthy {
				t.Errorf("Topic should be healthy after single file upload and restart, error: %s", topic.Error)
			}
			return
		}
	}
	t.Error("Topic not found after restart")
}
