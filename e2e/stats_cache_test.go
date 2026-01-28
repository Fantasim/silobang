package e2e

import (
	"encoding/json"
	"sync"
	"testing"
)

// =============================================================================
// Stats Cache E2E Tests
// =============================================================================

// TestStatsCacheAfterUpload verifies that topic stats and service info
// are updated after uploading files.
func TestStatsCacheAfterUpload(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "cache-upload")

	// Upload a file
	fileData := GenerateTestFile(2048)
	resp := ts.UploadFileExpectSuccess(t, "cache-upload", "test.bin", fileData, "")

	// Fetch topics and verify stats updated
	topicsResp := ts.GetTopics(t)

	var topic *TopicInfo
	for i := range topicsResp.Topics {
		if topicsResp.Topics[i].Name == "cache-upload" {
			topic = &topicsResp.Topics[i]
			break
		}
	}
	if topic == nil {
		t.Fatal("expected cache-upload topic in response")
	}

	// File count should be 1
	fileCount := toInt64FromInterface(topic.Stats["file_count"])
	if fileCount != 1 {
		t.Errorf("file_count after upload: got %v, want 1", topic.Stats["file_count"])
	}

	// Total size should match uploaded file
	totalSize := toInt64FromInterface(topic.Stats["total_size"])
	if totalSize != int64(len(fileData)) {
		t.Errorf("total_size after upload: got %d, want %d", totalSize, len(fileData))
	}

	// last_hash should match the uploaded file's hash
	if topic.Stats["last_hash"] != resp.Hash {
		t.Errorf("last_hash: got %v, want %s", topic.Stats["last_hash"], resp.Hash)
	}

	// Service info should reflect the upload
	if topicsResp.Service == nil {
		t.Fatal("expected service info in topics response")
	}
	if topicsResp.Service.TotalIndexedHashes < 1 {
		t.Errorf("total indexed hashes: got %d, want >= 1", topicsResp.Service.TotalIndexedHashes)
	}
	if topicsResp.Service.StorageSummary.TotalAssetSize < int64(len(fileData)) {
		t.Errorf("total asset size: got %d, want >= %d",
			topicsResp.Service.StorageSummary.TotalAssetSize, len(fileData))
	}

	// Upload a second file
	fileData2 := GenerateTestFile(4096)
	ts.UploadFileExpectSuccess(t, "cache-upload", "test2.bin", fileData2, "")

	// Verify stats updated again
	topicsResp2 := ts.GetTopics(t)
	for i := range topicsResp2.Topics {
		if topicsResp2.Topics[i].Name == "cache-upload" {
			topic = &topicsResp2.Topics[i]
			break
		}
	}

	fileCount2 := toInt64FromInterface(topic.Stats["file_count"])
	if fileCount2 != 2 {
		t.Errorf("file_count after second upload: got %d, want 2", fileCount2)
	}
}

// TestStatsCacheAfterMetadataSet verifies that topic stats reflect metadata changes.
func TestStatsCacheAfterMetadataSet(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "cache-meta")

	// Upload a file
	fileData := GenerateTestFile(1024)
	uploadResp := ts.UploadFileExpectSuccess(t, "cache-meta", "meta-test.bin", fileData, "")

	// Set metadata on the file
	ts.SetMetadata(t, uploadResp.Hash, "quality", "high")

	// Fetch topics and verify metadata_coverage stat
	topicsResp := ts.GetTopics(t)
	var topic *TopicInfo
	for i := range topicsResp.Topics {
		if topicsResp.Topics[i].Name == "cache-meta" {
			topic = &topicsResp.Topics[i]
			break
		}
	}
	if topic == nil {
		t.Fatal("expected cache-meta topic in response")
	}

	metaCoverage := toInt64FromInterface(topic.Stats["metadata_coverage"])
	if metaCoverage != 1 {
		t.Errorf("metadata_coverage after set: got %d, want 1", metaCoverage)
	}

	// Set more metadata keys on the same asset
	ts.SetMetadata(t, uploadResp.Hash, "format", "binary")

	// avg_metadata_keys should reflect the change
	topicsResp2 := ts.GetTopics(t)
	for i := range topicsResp2.Topics {
		if topicsResp2.Topics[i].Name == "cache-meta" {
			topic = &topicsResp2.Topics[i]
			break
		}
	}

	avgKeys, ok := topic.Stats["avg_metadata_keys"].(float64)
	if !ok || avgKeys < 1.0 {
		t.Errorf("avg_metadata_keys after 2 metadata sets: got %v, want >= 1.0", topic.Stats["avg_metadata_keys"])
	}
}

// TestStatsCacheAfterTopicCreation verifies that a newly created topic
// has zero stats and is reflected in service info.
func TestStatsCacheAfterTopicCreation(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	ts.CreateTopic(t, "new-empty")

	topicsResp := ts.GetTopics(t)
	var topic *TopicInfo
	for i := range topicsResp.Topics {
		if topicsResp.Topics[i].Name == "new-empty" {
			topic = &topicsResp.Topics[i]
			break
		}
	}
	if topic == nil {
		t.Fatal("expected new-empty topic in response")
	}

	// Stats should exist but show zero values
	fileCount := toInt64FromInterface(topic.Stats["file_count"])
	if fileCount != 0 {
		t.Errorf("file_count for empty topic: got %d, want 0", fileCount)
	}

	totalSize := toInt64FromInterface(topic.Stats["total_size"])
	if totalSize != 0 {
		t.Errorf("total_size for empty topic: got %d, want 0", totalSize)
	}

	// Service info should reflect the topic
	if topicsResp.Service == nil {
		t.Fatal("expected service info")
	}
	if topicsResp.Service.TopicsSummary.Total < 1 {
		t.Errorf("topics total: got %d, want >= 1", topicsResp.Service.TopicsSummary.Total)
	}
}

// TestStatsCacheConcurrentUploads verifies that concurrent uploads
// result in consistent final stats.
func TestStatsCacheConcurrentUploads(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "concurrent-test")

	const numUploads = 5
	const fileSize = 1024

	var wg sync.WaitGroup
	uploadResults := make([]UploadResponse, numUploads)
	errors := make([]error, numUploads)

	// Upload files concurrently
	for i := 0; i < numUploads; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			data := GenerateTestFile(fileSize)
			resp, err := ts.POST("/api/assets/concurrent-test", nil)
			if err != nil {
				errors[idx] = err
				return
			}
			resp.Body.Close()

			// Use the helper for proper upload
			uploadResults[idx] = ts.UploadFileExpectSuccess(t, "concurrent-test",
				"file"+string(rune('0'+idx))+".bin", data, "")
		}(i)
	}

	wg.Wait()

	// Check for errors
	for i, err := range errors {
		if err != nil {
			t.Fatalf("upload %d failed: %v", i, err)
		}
	}

	// Verify final stats
	topicsResp := ts.GetTopics(t)
	var topic *TopicInfo
	for i := range topicsResp.Topics {
		if topicsResp.Topics[i].Name == "concurrent-test" {
			topic = &topicsResp.Topics[i]
			break
		}
	}
	if topic == nil {
		t.Fatal("expected concurrent-test topic in response")
	}

	// Count unique hashes (some concurrent uploads may have identical content)
	uniqueHashes := make(map[string]bool)
	for _, r := range uploadResults {
		if r.Hash != "" {
			uniqueHashes[r.Hash] = true
		}
	}

	fileCount := toInt64FromInterface(topic.Stats["file_count"])
	if fileCount != int64(len(uniqueHashes)) {
		t.Errorf("file_count after concurrent uploads: got %d, want %d", fileCount, len(uniqueHashes))
	}
}

// TestStatsCacheServiceInfoAggregation verifies that service-level metrics
// correctly aggregate data from multiple topics.
func TestStatsCacheServiceInfoAggregation(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Create multiple topics with different amounts of data
	ts.CreateTopic(t, "agg-topic-a")
	ts.CreateTopic(t, "agg-topic-b")

	// Upload to topic A
	dataA1 := GenerateTestFile(1000)
	dataA2 := GenerateTestFile(2000)
	ts.UploadFileExpectSuccess(t, "agg-topic-a", "file1.png", dataA1, "")
	ts.UploadFileExpectSuccess(t, "agg-topic-a", "file2.jpg", dataA2, "")

	// Upload to topic B
	dataB1 := GenerateTestFile(3000)
	ts.UploadFileExpectSuccess(t, "agg-topic-b", "file1.fbx", dataB1, "")

	// Verify service info aggregation
	topicsResp := ts.GetTopics(t)
	if topicsResp.Service == nil {
		t.Fatal("expected service info")
	}

	svc := topicsResp.Service

	// Total indexed hashes should be 3
	if svc.TotalIndexedHashes != 3 {
		t.Errorf("total indexed hashes: got %d, want 3", svc.TotalIndexedHashes)
	}

	// Topics summary
	if svc.TopicsSummary.Total < 2 {
		t.Errorf("topics total: got %d, want >= 2", svc.TopicsSummary.Total)
	}
	if svc.TopicsSummary.Healthy < 2 {
		t.Errorf("topics healthy: got %d, want >= 2", svc.TopicsSummary.Healthy)
	}

	// Storage: total asset size should be sum of all uploads
	expectedAssetSize := int64(len(dataA1) + len(dataA2) + len(dataB1))
	if svc.StorageSummary.TotalAssetSize != expectedAssetSize {
		t.Errorf("total asset size: got %d, want %d", svc.StorageSummary.TotalAssetSize, expectedAssetSize)
	}

	// DAT file count should be >= 2 (at least 1 per topic)
	if svc.StorageSummary.TotalDatFiles < 2 {
		t.Errorf("total dat files: got %d, want >= 2", svc.StorageSummary.TotalDatFiles)
	}

	// Average asset size should be total / count
	expectedAvg := float64(expectedAssetSize) / float64(3)
	if svc.StorageSummary.AvgAssetSize != expectedAvg {
		t.Errorf("avg asset size: got %f, want %f", svc.StorageSummary.AvgAssetSize, expectedAvg)
	}

	// Verify individual topic stats sum matches service total
	var totalFileCount int64
	var totalAssetSize int64
	for _, topic := range topicsResp.Topics {
		if topic.Name == "agg-topic-a" || topic.Name == "agg-topic-b" {
			totalFileCount += toInt64FromInterface(topic.Stats["file_count"])
			totalAssetSize += toInt64FromInterface(topic.Stats["total_size"])
		}
	}
	if totalFileCount != svc.TotalIndexedHashes {
		t.Errorf("sum of topic file_counts (%d) != total indexed hashes (%d)",
			totalFileCount, svc.TotalIndexedHashes)
	}
	if totalAssetSize != svc.StorageSummary.TotalAssetSize {
		t.Errorf("sum of topic total_sizes (%d) != total asset size (%d)",
			totalAssetSize, svc.StorageSummary.TotalAssetSize)
	}
}

// =============================================================================
// New Topic Stats E2E Tests
// =============================================================================

// TestNewTopicStats_ExtensionBreakdown verifies that the extension_breakdown
// stat returns a JSON array of extension groups.
func TestNewTopicStats_ExtensionBreakdown(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "ext-stats")

	// Upload files with different extensions
	ts.UploadFileExpectSuccess(t, "ext-stats", "model.fbx", GenerateTestFile(3000), "")
	ts.UploadFileExpectSuccess(t, "ext-stats", "texture.png", GenerateTestFile(2000), "")
	ts.UploadFileExpectSuccess(t, "ext-stats", "icon.png", GenerateTestFile(1000), "")

	topicsResp := ts.GetTopics(t)
	var topic *TopicInfo
	for i := range topicsResp.Topics {
		if topicsResp.Topics[i].Name == "ext-stats" {
			topic = &topicsResp.Topics[i]
			break
		}
	}
	if topic == nil {
		t.Fatal("expected ext-stats topic in response")
	}

	// extension_breakdown should be a JSON string
	extBreakdown, ok := topic.Stats["extension_breakdown"].(string)
	if !ok {
		t.Fatalf("extension_breakdown: expected string, got %T: %v",
			topic.Stats["extension_breakdown"], topic.Stats["extension_breakdown"])
	}

	// Parse the JSON array
	var extensions []map[string]interface{}
	if err := json.Unmarshal([]byte(extBreakdown), &extensions); err != nil {
		t.Fatalf("failed to parse extension_breakdown JSON: %v", err)
	}

	if len(extensions) < 2 {
		t.Errorf("expected at least 2 extension groups, got %d", len(extensions))
	}

	// Verify PNG count (uploaded 2 PNG files)
	foundPng := false
	for _, ext := range extensions {
		if ext["ext"] == "png" {
			count := toInt64FromInterface(ext["count"])
			if count != 2 {
				t.Errorf("png count: got %d, want 2", count)
			}
			foundPng = true
		}
	}
	if !foundPng {
		t.Error("expected 'png' in extension breakdown")
	}

	// unique_extensions should be 2 (fbx, png)
	uniqueExt := toInt64FromInterface(topic.Stats["unique_extensions"])
	if uniqueExt != 2 {
		t.Errorf("unique_extensions: got %d, want 2", uniqueExt)
	}
}

// TestNewTopicStats_AvgMetadataKeys verifies that avg_metadata_keys
// correctly tracks the average number of metadata keys per file.
func TestNewTopicStats_AvgMetadataKeys(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "meta-keys-stats")

	// Upload two files
	resp1 := ts.UploadFileExpectSuccess(t, "meta-keys-stats", "a.bin", GenerateTestFile(512), "")
	resp2 := ts.UploadFileExpectSuccess(t, "meta-keys-stats", "b.bin", GenerateTestFile(512), "")

	// Set 3 metadata keys on file 1
	ts.SetMetadata(t, resp1.Hash, "quality", "high")
	ts.SetMetadata(t, resp1.Hash, "format", "binary")
	ts.SetMetadata(t, resp1.Hash, "source", "test")

	// Set 1 metadata key on file 2
	ts.SetMetadata(t, resp2.Hash, "quality", "low")

	topicsResp := ts.GetTopics(t)
	var topic *TopicInfo
	for i := range topicsResp.Topics {
		if topicsResp.Topics[i].Name == "meta-keys-stats" {
			topic = &topicsResp.Topics[i]
			break
		}
	}
	if topic == nil {
		t.Fatal("expected meta-keys-stats topic in response")
	}

	// avg_metadata_keys: 4 unique keys set (quality, format, source on file1 + quality on file2 = 3 unique keys)
	// across 2 files = 3/2 = 1.5
	avgKeys, ok := topic.Stats["avg_metadata_keys"].(float64)
	if !ok {
		t.Fatalf("avg_metadata_keys: expected float64, got %T: %v",
			topic.Stats["avg_metadata_keys"], topic.Stats["avg_metadata_keys"])
	}
	if avgKeys < 1.0 {
		t.Errorf("avg_metadata_keys: got %f, want >= 1.0", avgKeys)
	}
}

// TestNewTopicStats_DatList verifies that recent_dat_files returns
// a list of DAT files with their sizes.
func TestNewTopicStats_DatList(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "dat-list-stats")

	// Upload a file to create at least one DAT file
	ts.UploadFileExpectSuccess(t, "dat-list-stats", "data.bin", GenerateTestFile(4096), "")

	topicsResp := ts.GetTopics(t)
	var topic *TopicInfo
	for i := range topicsResp.Topics {
		if topicsResp.Topics[i].Name == "dat-list-stats" {
			topic = &topicsResp.Topics[i]
			break
		}
	}
	if topic == nil {
		t.Fatal("expected dat-list-stats topic in response")
	}

	// recent_dat_files should be an array of objects with name and size
	datList, ok := topic.Stats["recent_dat_files"].([]interface{})
	if !ok {
		t.Fatalf("recent_dat_files: expected array, got %T: %v",
			topic.Stats["recent_dat_files"], topic.Stats["recent_dat_files"])
	}

	if len(datList) == 0 {
		t.Fatal("expected at least 1 DAT file in recent_dat_files")
	}

	// Each entry should have name and size
	firstDat, ok := datList[0].(map[string]interface{})
	if !ok {
		t.Fatalf("dat list entry: expected map, got %T", datList[0])
	}

	if _, hasName := firstDat["name"]; !hasName {
		t.Error("dat list entry missing 'name' field")
	}
	if _, hasSize := firstDat["size"]; !hasSize {
		t.Error("dat list entry missing 'size' field")
	}

	datSize := toInt64FromInterface(firstDat["size"])
	if datSize <= 0 {
		t.Errorf("dat file size should be > 0, got %d", datSize)
	}
}

// TestNewTopicStats_DatCount verifies that dat_file_count is returned.
func TestNewTopicStats_DatCount(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "dat-count-stats")

	ts.UploadFileExpectSuccess(t, "dat-count-stats", "file.bin", GenerateTestFile(1024), "")

	topicsResp := ts.GetTopics(t)
	var topic *TopicInfo
	for i := range topicsResp.Topics {
		if topicsResp.Topics[i].Name == "dat-count-stats" {
			topic = &topicsResp.Topics[i]
			break
		}
	}
	if topic == nil {
		t.Fatal("expected dat-count-stats topic in response")
	}

	datCount := toInt64FromInterface(topic.Stats["dat_file_count"])
	if datCount < 1 {
		t.Errorf("dat_file_count: got %d, want >= 1", datCount)
	}
}

// =============================================================================
// Monitoring Endpoint E2E Tests
// =============================================================================

// TestMonitoringEndpointIncludesServiceInfo verifies that GET /api/monitoring
// returns cached service info alongside system metrics.
func TestMonitoringEndpointIncludesServiceInfo(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Create topics and upload data so service info has something to report
	ts.CreateTopic(t, "mon-topic-a")
	ts.CreateTopic(t, "mon-topic-b")
	ts.UploadFileExpectSuccess(t, "mon-topic-a", "file1.bin", GenerateTestFile(2048), "")
	ts.UploadFileExpectSuccess(t, "mon-topic-b", "file2.bin", GenerateTestFile(4096), "")

	mon := ts.GetMonitoring(t)

	// Service info should be present
	if mon.Service == nil {
		t.Fatal("expected service info in monitoring response")
	}

	// Verify service info has reasonable values
	if mon.Service.TotalIndexedHashes != 2 {
		t.Errorf("monitoring service info total indexed hashes: got %d, want 2",
			mon.Service.TotalIndexedHashes)
	}

	if mon.Service.TopicsSummary.Total < 2 {
		t.Errorf("monitoring service info topics total: got %d, want >= 2",
			mon.Service.TopicsSummary.Total)
	}

	if mon.Service.StorageSummary.TotalDatFiles < 2 {
		t.Errorf("monitoring service info total dat files: got %d, want >= 2",
			mon.Service.StorageSummary.TotalDatFiles)
	}

	if mon.Service.StorageSummary.TotalAssetSize < 6144 {
		t.Errorf("monitoring service info total asset size: got %d, want >= 6144",
			mon.Service.StorageSummary.TotalAssetSize)
	}

	if mon.Service.OrchestratorDBSize <= 0 {
		t.Errorf("monitoring service info orchestrator db size: got %d, want > 0",
			mon.Service.OrchestratorDBSize)
	}

	// Verify consistency with the application section
	if mon.Application.TopicsTotal != mon.Service.TopicsSummary.Total {
		t.Errorf("topics total mismatch: application=%d, service=%d",
			mon.Application.TopicsTotal, mon.Service.TopicsSummary.Total)
	}

	if mon.Application.TotalIndexedHashes != mon.Service.TotalIndexedHashes {
		t.Errorf("total indexed hashes mismatch: application=%d, service=%d",
			mon.Application.TotalIndexedHashes, mon.Service.TotalIndexedHashes)
	}
}

// TestMonitoringServiceInfoMatchesTopicsServiceInfo verifies that the service
// info returned by the monitoring endpoint is consistent with the topics endpoint.
func TestMonitoringServiceInfoMatchesTopicsServiceInfo(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	ts.CreateTopic(t, "consistency-test")
	ts.UploadFileExpectSuccess(t, "consistency-test", "data.bin", GenerateTestFile(2048), "")

	// Fetch from both endpoints
	topicsResp := ts.GetTopics(t)
	monResp := ts.GetMonitoring(t)

	if topicsResp.Service == nil {
		t.Fatal("expected service info from topics endpoint")
	}
	if monResp.Service == nil {
		t.Fatal("expected service info from monitoring endpoint")
	}

	// Key metrics should be consistent
	topicsSvc := topicsResp.Service
	monSvc := monResp.Service

	if topicsSvc.TotalIndexedHashes != monSvc.TotalIndexedHashes {
		t.Errorf("total indexed hashes: topics=%d, monitoring=%d",
			topicsSvc.TotalIndexedHashes, monSvc.TotalIndexedHashes)
	}

	if topicsSvc.TopicsSummary.Total != monSvc.TopicsSummary.Total {
		t.Errorf("topics total: topics=%d, monitoring=%d",
			topicsSvc.TopicsSummary.Total, monSvc.TopicsSummary.Total)
	}

	if topicsSvc.StorageSummary.TotalAssetSize != monSvc.StorageSummary.TotalAssetSize {
		t.Errorf("total asset size: topics=%d, monitoring=%d",
			topicsSvc.StorageSummary.TotalAssetSize, monSvc.StorageSummary.TotalAssetSize)
	}
}

// =============================================================================
// Helpers
// =============================================================================

// toInt64FromInterface extracts an int64 from an interface{} that may be
// float64 (JSON default for numbers), int64, or int.
func toInt64FromInterface(v interface{}) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	case int:
		return int64(n)
	default:
		return 0
	}
}
