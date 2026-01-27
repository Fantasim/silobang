package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"
)

func TestConcurrentUploads(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "concurrent-topic")

	const numUploads = 10
	var wg sync.WaitGroup

	type uploadResult struct {
		index   int
		content []byte
		resp    UploadResponse
	}

	results := make(chan uploadResult, numUploads)
	errors := make(chan error, numUploads)

	// Upload files in parallel, each goroutine generates its own content
	for i := 0; i < numUploads; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			// Generate unique content inside goroutine
			content := []byte(fmt.Sprintf("concurrent file content index=%d unique=%d", index, index*12345))
			filename := fmt.Sprintf("concurrent_%d.bin", index)

			resp, err := ts.UploadFile("concurrent-topic", filename, content, "")
			if err != nil {
				errors <- fmt.Errorf("upload %d failed: %v", index, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != 200 && resp.StatusCode != 201 {
				errors <- fmt.Errorf("upload %d got status %d", index, resp.StatusCode)
				return
			}

			var uploadResp UploadResponse
			if err := decodeJSON(resp.Body, &uploadResp); err != nil {
				errors <- fmt.Errorf("upload %d decode failed: %v", index, err)
				return
			}
			results <- uploadResult{index: index, content: content, resp: uploadResp}
		}(i)
	}

	wg.Wait()
	close(results)
	close(errors)

	// Check for errors
	for err := range errors {
		t.Error(err)
	}

	// Collect results with their content
	uploadedFiles := make([]uploadResult, 0, numUploads)
	for res := range results {
		if res.resp.Hash == "" {
			t.Error("Got empty hash in response")
			continue
		}
		uploadedFiles = append(uploadedFiles, res)
	}

	if len(uploadedFiles) != numUploads {
		t.Errorf("Expected %d successful uploads, got %d", numUploads, len(uploadedFiles))
	}

	// Verify all assets appear in queries
	result := ts.ExecuteQuery(t, "recent-imports", []string{"concurrent-topic"}, map[string]interface{}{
		"days": 1,
	})

	if result.RowCount != numUploads {
		t.Errorf("Expected %d rows in query, got %d", numUploads, result.RowCount)
	}

	// Verify downloads return correct content byte-for-byte
	for _, uploaded := range uploadedFiles {
		downloaded := ts.DownloadAsset(t, uploaded.resp.Hash)
		if !bytes.Equal(downloaded, uploaded.content) {
			t.Errorf("Content mismatch for upload %d (hash %s): uploaded %d bytes, downloaded %d bytes",
				uploaded.index, uploaded.resp.Hash[:16], len(uploaded.content), len(downloaded))
		}
	}
}

func TestConcurrentDuplicateUploads(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "dedup-topic")

	const numUploads = 10
	// All goroutines upload identical content
	sharedContent := []byte("identical content for dedup test")

	var wg sync.WaitGroup
	results := make(chan UploadResponse, numUploads)
	errors := make(chan error, numUploads)

	for i := 0; i < numUploads; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			resp, err := ts.UploadFile("dedup-topic", "same_file.bin", sharedContent, "")
			if err != nil {
				errors <- fmt.Errorf("upload %d failed: %v", index, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != 200 && resp.StatusCode != 201 {
				body, _ := io.ReadAll(resp.Body)
				errors <- fmt.Errorf("upload %d got status %d: %s", index, resp.StatusCode, string(body))
				return
			}

			var uploadResp UploadResponse
			if err := decodeJSON(resp.Body, &uploadResp); err != nil {
				errors <- fmt.Errorf("upload %d decode failed: %v", index, err)
				return
			}
			results <- uploadResp
		}(i)
	}

	wg.Wait()
	close(results)
	close(errors)

	for err := range errors {
		t.Error(err)
	}

	// Count how many were actually written vs skipped
	var written, skipped int
	var hash string
	for res := range results {
		if res.Skipped {
			skipped++
		} else {
			written++
			hash = res.Hash
		}
		// All should report the same hash
		if hash == "" {
			hash = res.Hash
		}
		if res.Hash != hash {
			t.Errorf("Got different hashes for same content: %s vs %s", res.Hash[:16], hash[:16])
		}
	}

	if written != 1 {
		t.Errorf("Expected exactly 1 non-skipped upload, got %d", written)
	}
	if skipped != numUploads-1 {
		t.Errorf("Expected %d skipped uploads, got %d", numUploads-1, skipped)
	}

	// Verify content integrity of the single written asset
	if hash != "" {
		downloaded := ts.DownloadAsset(t, hash)
		if !bytes.Equal(downloaded, sharedContent) {
			t.Errorf("Downloaded content doesn't match uploaded: got %d bytes, want %d bytes",
				len(downloaded), len(sharedContent))
		}
	}
}

func TestConcurrentTopicCreation(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	const numAttempts = 10
	var wg sync.WaitGroup
	successes := make(chan int, numAttempts)
	failures := make(chan int, numAttempts)
	errors := make(chan error, numAttempts)

	for i := 0; i < numAttempts; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			resp, err := ts.POST("/api/topics", map[string]interface{}{
				"name": "race-topic",
			})
			if err != nil {
				errors <- fmt.Errorf("create topic %d request failed: %v", index, err)
				return
			}
			resp.Body.Close()

			if resp.StatusCode == 200 {
				successes <- index
			} else if resp.StatusCode == 409 {
				failures <- index
			} else {
				errors <- fmt.Errorf("create topic %d got unexpected status %d", index, resp.StatusCode)
			}
		}(i)
	}

	wg.Wait()
	close(successes)
	close(failures)
	close(errors)

	for err := range errors {
		t.Error(err)
	}

	successCount := 0
	for range successes {
		successCount++
	}

	failureCount := 0
	for range failures {
		failureCount++
	}

	if successCount != 1 {
		t.Errorf("Expected exactly 1 successful topic creation, got %d", successCount)
	}
	if failureCount != numAttempts-1 {
		t.Errorf("Expected %d conflict responses, got %d", numAttempts-1, failureCount)
	}

	// Verify the topic is functional
	content := []byte("test content after concurrent creation")
	uploadResp := ts.UploadFileExpectSuccess(t, "race-topic", "test.bin", content, "")
	downloaded := ts.DownloadAsset(t, uploadResp.Hash)
	if !bytes.Equal(downloaded, content) {
		t.Error("Topic created via concurrent race is not functional: content mismatch")
	}
}

func TestUploadWhileQuerying(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "query-upload-topic")

	// Pre-populate with some files to make query take longer
	for i := 0; i < 20; i++ {
		content := []byte(fmt.Sprintf("prepopulate content %d", i))
		ts.UploadFileExpectSuccess(t, "query-upload-topic", fmt.Sprintf("pre_%d.bin", i), content, "")
	}

	var wg sync.WaitGroup
	queryDone := make(chan bool, 1)
	uploadDone := make(chan bool, 1)
	queryErr := make(chan error, 1)
	uploadErr := make(chan error, 1)

	// Start query in goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Run multiple queries to increase overlap chance
		for i := 0; i < 5; i++ {
			resp, err := ts.POST("/api/query/recent-imports", map[string]interface{}{
				"topics": []string{"query-upload-topic"},
				"params": map[string]interface{}{"days": 7},
			})
			if err != nil {
				queryErr <- fmt.Errorf("query request failed: %v", err)
				return
			}
			resp.Body.Close()
			if resp.StatusCode != 200 {
				queryErr <- fmt.Errorf("query got status %d", resp.StatusCode)
				return
			}
		}
		queryDone <- true
	}()

	// Start uploads in another goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			content := []byte(fmt.Sprintf("concurrent upload during query %d", i))
			resp, err := ts.UploadFile("query-upload-topic", fmt.Sprintf("during_query_%d.bin", i), content, "")
			if err != nil {
				uploadErr <- fmt.Errorf("upload request failed: %v", err)
				return
			}
			resp.Body.Close()
			if resp.StatusCode != 200 && resp.StatusCode != 201 {
				uploadErr <- fmt.Errorf("upload got status %d", resp.StatusCode)
				return
			}
		}
		uploadDone <- true
	}()

	// Wait with timeout to detect deadlocks
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success - both completed
	case <-time.After(30 * time.Second):
		t.Fatal("Timeout - possible deadlock detected during concurrent query and upload")
	}

	// Check for errors
	select {
	case err := <-queryErr:
		t.Errorf("Query error: %v", err)
	default:
	}

	select {
	case err := <-uploadErr:
		t.Errorf("Upload error: %v", err)
	default:
	}

	// Verify both operations completed
	select {
	case <-queryDone:
		// Query completed successfully
	default:
		t.Error("Query did not complete")
	}

	select {
	case <-uploadDone:
		// Upload completed successfully
	default:
		t.Error("Upload did not complete")
	}
}

func TestConcurrentMetadataOperations(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "meta-concurrent")

	// Upload a file to set metadata on
	uploadResp := ts.UploadFileExpectSuccess(t, "meta-concurrent", "test.bin", []byte("metadata test content"), "")
	hash := uploadResp.Hash

	const numOps = 10
	var wg sync.WaitGroup
	errors := make(chan error, numOps)

	// Set different metadata keys concurrently
	for i := 0; i < numOps; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			key := fmt.Sprintf("concurrent_key_%d", index)
			value := fmt.Sprintf("value_%d", index)

			resp, err := ts.POST("/api/assets/"+hash+"/metadata", map[string]interface{}{
				"op":                "set",
				"key":               key,
				"value":             value,
				"processor":         "test",
				"processor_version": "1.0",
			})
			if err != nil {
				errors <- fmt.Errorf("metadata %d request failed: %v", index, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != 200 {
				errors <- fmt.Errorf("metadata %d got status %d", index, resp.StatusCode)
				return
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Error(err)
	}

	// Verify all assets appear in query
	result := ts.ExecuteQuery(t, "recent-imports", []string{"meta-concurrent"}, map[string]interface{}{
		"days": 1,
	})

	if result.RowCount != 1 {
		t.Errorf("Expected 1 asset, got %d", result.RowCount)
	}

	// Verify all 10 keys are present in computed metadata with correct values
	fullResp := ts.GetAssetMetadata(t, hash)
	computed, ok := fullResp["computed_metadata"].(map[string]interface{})
	if !ok {
		t.Fatalf("computed_metadata not found or wrong type in response: %v", fullResp)
	}
	for i := 0; i < numOps; i++ {
		key := fmt.Sprintf("concurrent_key_%d", i)
		expectedValue := fmt.Sprintf("value_%d", i)
		if computed[key] != expectedValue {
			t.Errorf("computed_metadata[%s] = %v, want %q", key, computed[key], expectedValue)
		}
	}
}

func TestConcurrentMetadataSameKey(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "meta-same-key")

	uploadResp := ts.UploadFileExpectSuccess(t, "meta-same-key", "test.bin", []byte("same key test"), "")
	hash := uploadResp.Hash

	const numOps = 10
	var wg sync.WaitGroup
	errors := make(chan error, numOps)

	// All goroutines set the same key to different values
	for i := 0; i < numOps; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			resp, err := ts.POST("/api/assets/"+hash+"/metadata", map[string]interface{}{
				"op":                "set",
				"key":               "shared_key",
				"value":             fmt.Sprintf("value_%d", index),
				"processor":         "test",
				"processor_version": "1.0",
			})
			if err != nil {
				errors <- fmt.Errorf("metadata %d request failed: %v", index, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != 200 {
				errors <- fmt.Errorf("metadata %d got status %d", index, resp.StatusCode)
				return
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}

	// Verify computed metadata has exactly 1 entry for shared_key (last-write-wins)
	fullResp := ts.GetAssetMetadata(t, hash)
	computed, ok := fullResp["computed_metadata"].(map[string]interface{})
	if !ok {
		t.Fatalf("computed_metadata not found or wrong type in response: %v", fullResp)
	}
	val, exists := computed["shared_key"]
	if !exists {
		t.Fatal("shared_key not found in computed metadata")
	}
	// The value should be one of "value_0" through "value_9" (whichever was last)
	valStr, ok := val.(string)
	if !ok {
		t.Fatalf("shared_key value is not a string: %T", val)
	}
	found := false
	for i := 0; i < numOps; i++ {
		if valStr == fmt.Sprintf("value_%d", i) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("shared_key has unexpected value: %q", valStr)
	}

	// Verify metadata_log has all 10 entries (append-only log preserves all)
	topicDB := ts.GetTopicDB(t, "meta-same-key")
	var logCount int
	err := topicDB.QueryRow("SELECT COUNT(*) FROM metadata_log WHERE asset_id = ? AND key = ?", hash, "shared_key").Scan(&logCount)
	if err != nil {
		t.Fatalf("Failed to count metadata_log entries: %v", err)
	}
	if logCount != numOps {
		t.Errorf("Expected %d metadata_log entries for shared_key, got %d", numOps, logCount)
	}
}

// Helper function for JSON decoding in goroutines
func decodeJSON(r io.Reader, v interface{}) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
