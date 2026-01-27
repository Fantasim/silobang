package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"silobang/internal/constants"
)

// TestTopicStatsCorrectness verifies that topic stats (db_size, dat_size, total_size)
// are correctly calculated in the /api/topics endpoint
func TestTopicStatsCorrectness(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "stats-test")

	// Upload a known-size file
	testData := make([]byte, 1024) // 1KB file
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	uploadResp := ts.UploadFileExpectSuccess(t, "stats-test", "test.bin", testData, "")
	t.Logf("Uploaded file with hash: %s", uploadResp.Hash)

	// Get topics list with stats
	topicsResp := ts.GetTopics(t)

	// Find our topic
	var topic *TopicInfo
	for i := range topicsResp.Topics {
		if topicsResp.Topics[i].Name == "stats-test" {
			topic = &topicsResp.Topics[i]
			break
		}
	}

	if topic == nil {
		t.Fatal("Topic 'stats-test' not found in topics list")
	}

	if !topic.Healthy {
		t.Fatalf("Topic is not healthy: %s", topic.Error)
	}

	// Get actual file sizes from filesystem
	dbPath := filepath.Join(ts.WorkDir, "stats-test", constants.InternalDir, "stats-test.db")
	dbInfo, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("Failed to stat DB file: %v", err)
	}
	actualDBSize := dbInfo.Size()

	datPath := filepath.Join(ts.WorkDir, "stats-test", "000001.dat")
	datInfo, err := os.Stat(datPath)
	if err != nil {
		t.Fatalf("Failed to stat DAT file: %v", err)
	}
	actualDATSize := datInfo.Size()
	expectedDATSize := int64(constants.HeaderSize) + int64(len(testData)) // header + data

	// Verify DAT file size calculation
	if actualDATSize != expectedDATSize {
		t.Errorf("DAT file size unexpected: got %d, expected %d (header=%d + data=%d)",
			actualDATSize, expectedDATSize, constants.HeaderSize, len(testData))
	}

	// Check stats from API
	t.Logf("Topic stats: %+v", topic.Stats)

	// Verify db_size
	if dbSizeVal, ok := topic.Stats["db_size"]; ok {
		dbSizeFromAPI := int64(dbSizeVal.(float64))
		if dbSizeFromAPI != actualDBSize {
			t.Errorf("db_size MISMATCH: API=%d, actual=%d", dbSizeFromAPI, actualDBSize)
		} else {
			t.Logf("✓ db_size correct: %d bytes", dbSizeFromAPI)
		}
	} else {
		t.Error("db_size not found in stats")
	}

	// Verify dat_size
	if datSizeVal, ok := topic.Stats["dat_size"]; ok {
		datSizeFromAPI := int64(datSizeVal.(float64))
		if datSizeFromAPI != actualDATSize {
			t.Errorf("dat_size MISMATCH: API=%d, actual=%d", datSizeFromAPI, actualDATSize)
		} else {
			t.Logf("✓ dat_size correct: %d bytes", datSizeFromAPI)
		}
	} else {
		t.Error("dat_size not found in stats")
	}

	// Verify total_size (sum of asset_size from DB, should equal uploaded data size)
	if totalSizeVal, ok := topic.Stats["total_size"]; ok {
		totalSizeFromAPI := int64(totalSizeVal.(float64))
		expectedTotalSize := int64(len(testData))
		if totalSizeFromAPI != expectedTotalSize {
			t.Errorf("total_size MISMATCH: API=%d, expected=%d (asset data size)", totalSizeFromAPI, expectedTotalSize)
		} else {
			t.Logf("✓ total_size correct: %d bytes", totalSizeFromAPI)
		}
	} else {
		t.Error("total_size not found in stats")
	}

	// Verify file_count
	if fileCountVal, ok := topic.Stats["file_count"]; ok {
		fileCount := int64(fileCountVal.(float64))
		if fileCount != 1 {
			t.Errorf("file_count MISMATCH: API=%d, expected=1", fileCount)
		} else {
			t.Logf("✓ file_count correct: %d", fileCount)
		}
	} else {
		t.Error("file_count not found in stats")
	}
}

// TestTopicStatsMultipleFiles verifies stats are correct with multiple uploads
func TestTopicStatsMultipleFiles(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "multi-stats")

	// Upload multiple files of different sizes
	file1 := make([]byte, 512)
	file2 := make([]byte, 2048)
	file3 := make([]byte, 1024)

	for i := range file1 {
		file1[i] = byte(i % 256)
	}
	for i := range file2 {
		file2[i] = byte((i + 1) % 256)
	}
	for i := range file3 {
		file3[i] = byte((i + 2) % 256)
	}

	ts.UploadFileExpectSuccess(t, "multi-stats", "file1.bin", file1, "")
	ts.UploadFileExpectSuccess(t, "multi-stats", "file2.bin", file2, "")
	ts.UploadFileExpectSuccess(t, "multi-stats", "file3.bin", file3, "")

	// Get topics
	topicsResp := ts.GetTopics(t)

	var topic *TopicInfo
	for i := range topicsResp.Topics {
		if topicsResp.Topics[i].Name == "multi-stats" {
			topic = &topicsResp.Topics[i]
			break
		}
	}

	if topic == nil {
		t.Fatal("Topic 'multi-stats' not found")
	}

	// Verify total_size = sum of all file sizes
	expectedTotalSize := int64(len(file1) + len(file2) + len(file3))
	if totalSizeVal, ok := topic.Stats["total_size"]; ok {
		totalSize := int64(totalSizeVal.(float64))
		if totalSize != expectedTotalSize {
			t.Errorf("total_size MISMATCH: API=%d, expected=%d", totalSize, expectedTotalSize)
		} else {
			t.Logf("✓ total_size correct: %d bytes", totalSize)
		}
	}

	// Verify dat_size = sum of all (header + data)
	expectedDATSize := int64(3*constants.HeaderSize) + expectedTotalSize
	datPath := filepath.Join(ts.WorkDir, "multi-stats", "000001.dat")
	datInfo, err := os.Stat(datPath)
	if err != nil {
		t.Fatalf("Failed to stat DAT file: %v", err)
	}

	if datInfo.Size() != expectedDATSize {
		t.Errorf("Actual DAT file size unexpected: got %d, expected %d", datInfo.Size(), expectedDATSize)
	}

	if datSizeVal, ok := topic.Stats["dat_size"]; ok {
		datSize := int64(datSizeVal.(float64))
		if datSize != datInfo.Size() {
			t.Errorf("dat_size MISMATCH: API=%d, actual=%d", datSize, datInfo.Size())
		} else {
			t.Logf("✓ dat_size correct: %d bytes", datSize)
		}
	}

	// Verify file_count = 3
	if fileCountVal, ok := topic.Stats["file_count"]; ok {
		fileCount := int64(fileCountVal.(float64))
		if fileCount != 3 {
			t.Errorf("file_count MISMATCH: API=%d, expected=3", fileCount)
		} else {
			t.Logf("✓ file_count correct: %d", fileCount)
		}
	}
}

// TestTopicStatsEmptyTopic verifies stats for a topic with no files
func TestTopicStatsEmptyTopic(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "empty-topic")

	topicsResp := ts.GetTopics(t)

	var topic *TopicInfo
	for i := range topicsResp.Topics {
		if topicsResp.Topics[i].Name == "empty-topic" {
			topic = &topicsResp.Topics[i]
			break
		}
	}

	if topic == nil {
		t.Fatal("Topic 'empty-topic' not found")
	}

	// Verify total_size = 0 (or nil/null)
	if totalSizeVal, ok := topic.Stats["total_size"]; ok {
		if totalSizeVal != nil {
			totalSize := int64(totalSizeVal.(float64))
			if totalSize != 0 {
				t.Errorf("total_size for empty topic should be 0, got %d", totalSize)
			}
		}
	}

	// Verify file_count = 0
	if fileCountVal, ok := topic.Stats["file_count"]; ok {
		fileCount := int64(fileCountVal.(float64))
		if fileCount != 0 {
			t.Errorf("file_count for empty topic should be 0, got %d", fileCount)
		}
	}

	// Verify dat_size = 0 (no .dat files)
	if datSizeVal, ok := topic.Stats["dat_size"]; ok {
		if datSizeVal != nil {
			datSize := int64(datSizeVal.(float64))
			if datSize != 0 {
				t.Errorf("dat_size for empty topic should be 0, got %d", datSize)
			}
		}
	}

	// db_size should be > 0 (SQLite file exists with schema)
	if dbSizeVal, ok := topic.Stats["db_size"]; ok {
		if dbSizeVal == nil {
			t.Error("db_size should not be nil for existing topic")
		} else {
			dbSize := int64(dbSizeVal.(float64))
			if dbSize <= 0 {
				t.Errorf("db_size should be > 0 for existing topic, got %d", dbSize)
			} else {
				t.Logf("✓ db_size for empty topic: %d bytes", dbSize)
			}
		}
	}
}

// TestTopicStatsAvgSizeFloat verifies avg_size returns correct floating-point value
func TestTopicStatsAvgSizeFloat(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "avg-test")

	// Upload files of known sizes to create predictable average
	// 100 + 200 + 300 = 600, avg = 200.0 exactly
	file1 := make([]byte, 100)
	file2 := make([]byte, 200)
	file3 := make([]byte, 300)

	// Ensure unique content for each file
	for i := range file1 {
		file1[i] = byte(i % 256)
	}
	for i := range file2 {
		file2[i] = byte((i + 1) % 256)
	}
	for i := range file3 {
		file3[i] = byte((i + 2) % 256)
	}

	ts.UploadFileExpectSuccess(t, "avg-test", "file1.bin", file1, "")
	ts.UploadFileExpectSuccess(t, "avg-test", "file2.bin", file2, "")
	ts.UploadFileExpectSuccess(t, "avg-test", "file3.bin", file3, "")

	topicsResp := ts.GetTopics(t)

	var topic *TopicInfo
	for i := range topicsResp.Topics {
		if topicsResp.Topics[i].Name == "avg-test" {
			topic = &topicsResp.Topics[i]
			break
		}
	}

	if topic == nil {
		t.Fatal("Topic 'avg-test' not found")
	}

	// Verify avg_size is present and correct (should be 200.0)
	avgSizeVal, ok := topic.Stats["avg_size"]
	if !ok {
		t.Fatal("avg_size not found in stats")
	}

	if avgSizeVal == nil {
		t.Fatal("avg_size is nil - this is the bug we're fixing!")
	}

	avgSize, ok := avgSizeVal.(float64)
	if !ok {
		t.Fatalf("avg_size is not a float64, got type %T", avgSizeVal)
	}

	expectedAvg := float64(100+200+300) / 3.0
	tolerance := 0.01
	if avgSize < expectedAvg-tolerance || avgSize > expectedAvg+tolerance {
		t.Errorf("avg_size mismatch: got %f, expected %f", avgSize, expectedAvg)
	} else {
		t.Logf("✓ avg_size correct: %f", avgSize)
	}
}

// TestTopicStatsAvgSizeEmptyTopic verifies avg_size is null for empty topics
func TestTopicStatsAvgSizeEmptyTopic(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "empty-avg")

	topicsResp := ts.GetTopics(t)

	var topic *TopicInfo
	for i := range topicsResp.Topics {
		if topicsResp.Topics[i].Name == "empty-avg" {
			topic = &topicsResp.Topics[i]
			break
		}
	}

	if topic == nil {
		t.Fatal("Topic 'empty-avg' not found")
	}

	// For empty topics, AVG returns NULL - should be nil in Go
	avgSizeVal, ok := topic.Stats["avg_size"]
	if !ok {
		t.Fatal("avg_size key not found in stats")
	}

	if avgSizeVal != nil {
		t.Errorf("avg_size for empty topic should be nil, got %v", avgSizeVal)
	} else {
		t.Log("✓ avg_size is nil for empty topic (correct)")
	}
}

// TestTopicStatsAvgSizeImmutability verifies avg_size value persists across server restart
func TestTopicStatsAvgSizeImmutability(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "avg-immutable")

	// Upload a file
	data := make([]byte, 1000)
	for i := range data {
		data[i] = byte(i % 256)
	}
	ts.UploadFileExpectSuccess(t, "avg-immutable", "file.bin", data, "")

	// Get initial avg_size
	resp1 := ts.GetTopics(t)
	var initialAvg float64
	for _, topic := range resp1.Topics {
		if topic.Name == "avg-immutable" {
			avgVal := topic.Stats["avg_size"]
			if avgVal == nil {
				t.Fatal("avg_size is nil before restart")
			}
			initialAvg = avgVal.(float64)
			break
		}
	}

	// Restart server (movability test)
	ts.Restart(t)

	// Verify avg_size preserved
	resp2 := ts.GetTopics(t)
	for _, topic := range resp2.Topics {
		if topic.Name == "avg-immutable" {
			avgVal := topic.Stats["avg_size"]
			if avgVal == nil {
				t.Fatal("avg_size is nil after restart")
			}
			afterAvg := avgVal.(float64)
			if afterAvg != initialAvg {
				t.Errorf("avg_size changed after restart: before=%f, after=%f", initialAvg, afterAvg)
			} else {
				t.Logf("✓ avg_size preserved after restart: %f", afterAvg)
			}
			break
		}
	}
}
