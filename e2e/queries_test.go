package e2e

import (
	"encoding/json"
	"io"
	"testing"
	"time"
)

func TestQueryExecution(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "test-topic")

	// Upload varied files
	testFiles := []struct {
		size     int
		filename string
	}{
		{1024, "small1.bin"},
		{5000, "small2.bin"},
		{10000, "medium1.bin"},
		{50000, "medium2.bin"},
		{100000, "large1.bin"},
		{200000, "large2.bin"},
	}

	hashes := make([]string, len(testFiles))

	for i, tf := range testFiles {
		content := GenerateTestFile(tf.size)
		resp, err := ts.UploadFile("test-topic", tf.filename, content, "")
		if err != nil {
			t.Fatalf("Upload %d failed: %v", i, err)
		}
		defer resp.Body.Close()

		var uploadResp map[string]interface{}
		bodyBytes, _ := io.ReadAll(resp.Body)
		json.Unmarshal(bodyBytes, &uploadResp)
		hashes[i] = uploadResp["hash"].(string)
	}
	t.Logf("Uploaded %d files with hashes: %v", len(hashes), hashes)

	// Wait a moment for topic metadata to be updated
	time.Sleep(100 * time.Millisecond)

	// Add metadata to some files
	for i := 0; i < 3; i++ {
		resp, err := ts.POST("/api/assets/"+hashes[i]+"/metadata", map[string]interface{}{
			"op":                "set",
			"key":               "processor",
			"value":             "test",
			"processor":         "test",
			"processor_version": "1.0",
		})
		if err != nil {
			t.Fatalf("Set metadata failed: %v", err)
		}
		resp.Body.Close()
	}

	// Wait a moment to ensure created_at timestamps
	time.Sleep(100 * time.Millisecond)

	// Test 1: recent-imports query
	resp, err := ts.POST("/api/query/recent-imports", map[string]interface{}{
		"topics": []string{"test-topic"},
		"params": map[string]interface{}{
			"days":  7,
			"limit": 10,
		},
	})
	if err != nil {
		t.Fatalf("recent-imports query failed: %v", err)
	}
	defer resp.Body.Close()

	var recentResp map[string]interface{}
	bodyBytes, _ := io.ReadAll(resp.Body)
	json.Unmarshal(bodyBytes, &recentResp)

	rows, ok := recentResp["rows"].([]interface{})
	if !ok {
		t.Fatalf("Expected rows array, got: %v", recentResp)
	}

	if len(rows) > 10 {
		t.Errorf("Expected <= 10 results, got %d", len(rows))
	}

	// Test 2: large-files query (min_size: 49999 to match files > 49999)
	resp, err = ts.POST("/api/query/large-files", map[string]interface{}{
		"topics": []string{"test-topic"},
		"params": map[string]interface{}{
			"min_size": 49999,
		},
	})
	if err != nil {
		t.Fatalf("large-files query failed: %v", err)
	}
	defer resp.Body.Close()

	var largeResp map[string]interface{}
	bodyBytes, _ = io.ReadAll(resp.Body)
	json.Unmarshal(bodyBytes, &largeResp)

	rows, ok = largeResp["rows"].([]interface{})
	if !ok {
		t.Fatalf("Expected rows array, got: %v", largeResp)
	}

	// Should return files > 49999 bytes (large1.bin=50000, large2.bin=100000, medium2.bin=200000)
	if len(rows) != 3 {
		t.Logf("Query response: %v", largeResp)
		t.Errorf("Expected 3 large files, got %d", len(rows))
	}

	// Verify all have asset_size > 49999
	columns := largeResp["columns"].([]interface{})
	sizeIdx := -1
	for i, col := range columns {
		if col.(string) == "asset_size" {
			sizeIdx = i
			break
		}
	}
	if sizeIdx == -1 {
		t.Fatal("asset_size column not found")
	}

	for _, row := range rows {
		rowArray := row.([]interface{})
		size := rowArray[sizeIdx].(float64)
		if size <= 49999 {
			t.Errorf("Expected asset_size > 49999, got %f", size)
		}
	}

	// Test 3: with-metadata query
	resp, err = ts.POST("/api/query/with-metadata", map[string]interface{}{
		"topics": []string{"test-topic"},
		"params": map[string]interface{}{
			"key": "processor",
		},
	})
	if err != nil {
		t.Fatalf("with-metadata query failed: %v", err)
	}
	defer resp.Body.Close()

	var metaResp map[string]interface{}
	bodyBytes, _ = io.ReadAll(resp.Body)
	json.Unmarshal(bodyBytes, &metaResp)

	rows, ok = metaResp["rows"].([]interface{})
	if !ok {
		t.Fatalf("Expected rows array, got: %v", metaResp)
	}

	// Should return 3 files with metadata
	if len(rows) != 3 {
		t.Errorf("Expected 3 files with metadata, got %d", len(rows))
	}

	// Test 4: by-hash query (hash prefix)
	if len(hashes) > 0 {
		hashPrefix := hashes[0][:3] // Use first 3 chars

		resp, err = ts.POST("/api/query/by-hash", map[string]interface{}{
			"topics": []string{"test-topic"},
			"params": map[string]interface{}{
				"hash": hashPrefix,
			},
		})
		if err != nil {
			t.Fatalf("by-hash query failed: %v", err)
		}
		defer resp.Body.Close()

		var hashResp map[string]interface{}
		bodyBytes, _ = io.ReadAll(resp.Body)
		json.Unmarshal(bodyBytes, &hashResp)

		rows, ok = hashResp["rows"].([]interface{})
		if !ok {
			t.Fatalf("Expected rows array, got: %v", hashResp)
		}

		// Should return at least one result
		if len(rows) == 0 {
			t.Errorf("Expected at least 1 result for hash prefix %s", hashPrefix)
		}

		// Verify results have hash starting with prefix
		columns = hashResp["columns"].([]interface{})
		hashIdx := -1
		for i, col := range columns {
			if col.(string) == "asset_id" {
				hashIdx = i
				break
			}
		}
		if hashIdx == -1 {
			t.Fatal("asset_id column not found")
		}

		for _, row := range rows {
			rowArray := row.([]interface{})
			hash := rowArray[hashIdx].(string)
			if len(hash) < len(hashPrefix) || hash[:len(hashPrefix)] != hashPrefix {
				t.Errorf("Expected hash to start with %s, got %s", hashPrefix, hash)
			}
		}
	}
}
