package e2e

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"silobang/internal/constants"
	"silobang/internal/storage"
)

// TestServiceInfoBasicStructure verifies the service info response shape and types
func TestServiceInfoBasicStructure(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	topicsResp := ts.GetTopics(t)

	if topicsResp.Service == nil {
		t.Fatal("service field is nil, expected ServiceInfo struct")
	}

	service := topicsResp.Service

	// Verify app version is present (will be "dev" in test builds)
	if service.VersionInfo.AppVersion == "" {
		t.Error("app_version should not be empty")
	}

	// Verify version info matches constants
	if service.VersionInfo.BlobVersion != constants.BlobVersion {
		t.Errorf("blob_version mismatch: got %d, expected %d", service.VersionInfo.BlobVersion, constants.BlobVersion)
	}
	if service.VersionInfo.HeaderSize != constants.HeaderSize {
		t.Errorf("header_size mismatch: got %d, expected %d", service.VersionInfo.HeaderSize, constants.HeaderSize)
	}

	// Verify orchestrator_db_size is positive (empty DB still has schema)
	if service.OrchestratorDBSize <= 0 {
		t.Errorf("orchestrator_db_size should be > 0, got %d", service.OrchestratorDBSize)
	}

	// Empty project: no indexed hashes
	if service.TotalIndexedHashes != 0 {
		t.Errorf("total_indexed_hashes should be 0 for empty project, got %d", service.TotalIndexedHashes)
	}

	// Empty project: no topics
	if service.TopicsSummary.Total != 0 {
		t.Errorf("topics_summary.total should be 0 for empty project, got %d", service.TopicsSummary.Total)
	}

	t.Logf("Service info structure verified: %+v", service)
}

// TestServiceInfoEmptyProject verifies service info with no topics
func TestServiceInfoEmptyProject(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	topicsResp := ts.GetTopics(t)
	service := topicsResp.Service

	if service == nil {
		t.Fatal("service field is nil")
	}

	// All summary counts should be zero
	if service.TopicsSummary.Total != 0 {
		t.Errorf("expected 0 total topics, got %d", service.TopicsSummary.Total)
	}
	if service.TopicsSummary.Healthy != 0 {
		t.Errorf("expected 0 healthy topics, got %d", service.TopicsSummary.Healthy)
	}
	if service.TopicsSummary.Unhealthy != 0 {
		t.Errorf("expected 0 unhealthy topics, got %d", service.TopicsSummary.Unhealthy)
	}

	// Storage summary should be zero
	if service.StorageSummary.TotalDatSize != 0 {
		t.Errorf("expected 0 total_dat_size, got %d", service.StorageSummary.TotalDatSize)
	}
	if service.StorageSummary.TotalDbSize != 0 {
		t.Errorf("expected 0 total_db_size, got %d", service.StorageSummary.TotalDbSize)
	}
	if service.StorageSummary.TotalAssetSize != 0 {
		t.Errorf("expected 0 total_asset_size, got %d", service.StorageSummary.TotalAssetSize)
	}
	if service.StorageSummary.TotalDatFiles != 0 {
		t.Errorf("expected 0 total_dat_files, got %d", service.StorageSummary.TotalDatFiles)
	}
	if service.StorageSummary.AvgAssetSize != 0 {
		t.Errorf("expected 0 avg_asset_size, got %f", service.StorageSummary.AvgAssetSize)
	}

	// But orchestrator DB should exist
	if service.OrchestratorDBSize <= 0 {
		t.Errorf("orchestrator_db_size should be > 0 even for empty project, got %d", service.OrchestratorDBSize)
	}
}

// TestServiceInfoWithTopics verifies service info with multiple healthy topics and uploads
func TestServiceInfoWithTopics(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Create 3 topics
	ts.CreateTopic(t, "topic-a")
	ts.CreateTopic(t, "topic-b")
	ts.CreateTopic(t, "topic-c")

	// Upload files to each topic
	file1 := make([]byte, 512)
	file2 := make([]byte, 1024)
	file3 := make([]byte, 2048)
	for i := range file1 {
		file1[i] = byte(i % 256)
	}
	for i := range file2 {
		file2[i] = byte((i + 1) % 256)
	}
	for i := range file3 {
		file3[i] = byte((i + 2) % 256)
	}

	ts.UploadFileExpectSuccess(t, "topic-a", "file1.bin", file1, "")
	ts.UploadFileExpectSuccess(t, "topic-b", "file2.bin", file2, "")
	ts.UploadFileExpectSuccess(t, "topic-c", "file3.bin", file3, "")

	topicsResp := ts.GetTopics(t)
	service := topicsResp.Service

	if service == nil {
		t.Fatal("service field is nil")
	}

	// Verify topic counts
	if service.TopicsSummary.Total != 3 {
		t.Errorf("expected 3 total topics, got %d", service.TopicsSummary.Total)
	}
	if service.TopicsSummary.Healthy != 3 {
		t.Errorf("expected 3 healthy topics, got %d", service.TopicsSummary.Healthy)
	}
	if service.TopicsSummary.Unhealthy != 0 {
		t.Errorf("expected 0 unhealthy topics, got %d", service.TopicsSummary.Unhealthy)
	}

	// Verify total indexed hashes (3 unique files)
	if service.TotalIndexedHashes != 3 {
		t.Errorf("expected 3 total_indexed_hashes, got %d", service.TotalIndexedHashes)
	}

	// Verify storage totals
	expectedAssetSize := int64(len(file1) + len(file2) + len(file3))
	if service.StorageSummary.TotalAssetSize != expectedAssetSize {
		t.Errorf("expected total_asset_size %d, got %d", expectedAssetSize, service.StorageSummary.TotalAssetSize)
	}

	// Each topic has one .dat file
	if service.StorageSummary.TotalDatFiles != 3 {
		t.Errorf("expected 3 total_dat_files, got %d", service.StorageSummary.TotalDatFiles)
	}

	// Verify dat_size = headers + asset sizes
	expectedDatSize := int64(3*constants.HeaderSize) + expectedAssetSize
	if service.StorageSummary.TotalDatSize != expectedDatSize {
		t.Errorf("expected total_dat_size %d, got %d", expectedDatSize, service.StorageSummary.TotalDatSize)
	}

	// DB size should be positive (3 topic DBs)
	if service.StorageSummary.TotalDbSize <= 0 {
		t.Errorf("expected total_db_size > 0, got %d", service.StorageSummary.TotalDbSize)
	}

	t.Logf("Service info with topics: %+v", service)
}

// TestServiceInfoWithUnhealthyTopics verifies unhealthy topic handling
func TestServiceInfoWithUnhealthyTopics(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Create topics
	ts.CreateTopic(t, "healthy-topic")
	ts.CreateTopic(t, "corrupt-topic")

	// Upload a file to the healthy topic
	ts.UploadFileExpectSuccess(t, "healthy-topic", "file.bin", []byte("test data"), "")

	// Corrupt the second topic by deleting its database
	corruptDBPath := filepath.Join(ts.WorkDir, "corrupt-topic", constants.InternalDir, "corrupt-topic.db")
	if err := os.Remove(corruptDBPath); err != nil {
		t.Fatalf("Failed to remove db file: %v", err)
	}

	// Restart to detect corrupted topic
	ts.Restart(t)

	topicsResp := ts.GetTopics(t)
	service := topicsResp.Service

	if service == nil {
		t.Fatal("service field is nil")
	}

	// Verify counts
	if service.TopicsSummary.Total != 2 {
		t.Errorf("expected 2 total topics, got %d", service.TopicsSummary.Total)
	}
	if service.TopicsSummary.Healthy != 1 {
		t.Errorf("expected 1 healthy topic, got %d", service.TopicsSummary.Healthy)
	}
	if service.TopicsSummary.Unhealthy != 1 {
		t.Errorf("expected 1 unhealthy topic, got %d", service.TopicsSummary.Unhealthy)
	}

	// Storage should only include healthy topic stats
	// The unhealthy topic's stats should not be included in aggregates
	if service.TotalIndexedHashes != 1 {
		t.Errorf("expected 1 total_indexed_hashes (only healthy topic), got %d", service.TotalIndexedHashes)
	}
}

// TestServiceInfoOrchestratorDBSizeAccuracy verifies orchestrator DB size matches file system
func TestServiceInfoOrchestratorDBSizeAccuracy(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "test-topic")
	ts.UploadFileExpectSuccess(t, "test-topic", "file.bin", []byte("test content"), "")

	topicsResp := ts.GetTopics(t)
	service := topicsResp.Service

	if service == nil {
		t.Fatal("service field is nil")
	}

	// Get actual file size
	orchPath := filepath.Join(ts.WorkDir, constants.InternalDir, constants.OrchestratorDB)
	info, err := os.Stat(orchPath)
	if err != nil {
		t.Fatalf("Failed to stat orchestrator.db: %v", err)
	}

	if service.OrchestratorDBSize != info.Size() {
		t.Errorf("orchestrator_db_size mismatch: API=%d, actual=%d", service.OrchestratorDBSize, info.Size())
	} else {
		t.Logf("orchestrator_db_size correct: %d bytes", service.OrchestratorDBSize)
	}
}

// TestServiceInfoIndexedHashesAccuracy verifies hash count matches uploads
func TestServiceInfoIndexedHashesAccuracy(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "hash-count-test")

	// Upload 5 unique files
	for i := 0; i < 5; i++ {
		data := make([]byte, 100+i)
		for j := range data {
			data[j] = byte((i + j) % 256)
		}
		ts.UploadFileExpectSuccess(t, "hash-count-test", "file"+string(rune('a'+i))+".bin", data, "")
	}

	topicsResp := ts.GetTopics(t)
	if topicsResp.Service.TotalIndexedHashes != 5 {
		t.Errorf("expected 5 indexed hashes, got %d", topicsResp.Service.TotalIndexedHashes)
	}

	// Upload a duplicate (should not increment)
	firstData := make([]byte, 100)
	for j := range firstData {
		firstData[j] = byte(j % 256)
	}
	ts.UploadFileExpectSuccess(t, "hash-count-test", "duplicate.bin", firstData, "")

	topicsResp = ts.GetTopics(t)
	if topicsResp.Service.TotalIndexedHashes != 5 {
		t.Errorf("after duplicate upload, expected 5 indexed hashes (unchanged), got %d", topicsResp.Service.TotalIndexedHashes)
	}

	// Verify against direct DB query
	orchDB := ts.GetOrchestratorDB(t)
	var actualCount int64
	err := orchDB.QueryRow(constants.OrchestratorCountHashesQuery).Scan(&actualCount)
	if err != nil {
		t.Fatalf("Failed to query orchestrator: %v", err)
	}
	if actualCount != 5 {
		t.Errorf("direct DB count mismatch: expected 5, got %d", actualCount)
	}
}

// TestServiceInfoStorageAggregation verifies storage totals match individual topic sums
func TestServiceInfoStorageAggregation(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Create topics with different file counts
	ts.CreateTopic(t, "topic-1")
	ts.CreateTopic(t, "topic-2")

	// Topic 1: 2 files
	ts.UploadFileExpectSuccess(t, "topic-1", "a.bin", make([]byte, 100), "")
	ts.UploadFileExpectSuccess(t, "topic-1", "b.bin", make([]byte, 200), "")

	// Topic 2: 1 file
	data := make([]byte, 300)
	for i := range data {
		data[i] = byte(i % 256)
	}
	ts.UploadFileExpectSuccess(t, "topic-2", "c.bin", data, "")

	topicsResp := ts.GetTopics(t)

	// Sum individual topic stats
	var sumDatSize, sumDbSize, sumAssetSize int64
	for _, topic := range topicsResp.Topics {
		if topic.Stats != nil {
			if v, ok := topic.Stats["dat_size"].(float64); ok {
				sumDatSize += int64(v)
			}
			if v, ok := topic.Stats["db_size"].(float64); ok {
				sumDbSize += int64(v)
			}
			if v, ok := topic.Stats["total_size"].(float64); ok {
				sumAssetSize += int64(v)
			}
		}
	}

	service := topicsResp.Service

	if service.StorageSummary.TotalDatSize != sumDatSize {
		t.Errorf("total_dat_size mismatch: aggregate=%d, sum=%d", service.StorageSummary.TotalDatSize, sumDatSize)
	}
	if service.StorageSummary.TotalDbSize != sumDbSize {
		t.Errorf("total_db_size mismatch: aggregate=%d, sum=%d", service.StorageSummary.TotalDbSize, sumDbSize)
	}
	if service.StorageSummary.TotalAssetSize != sumAssetSize {
		t.Errorf("total_asset_size mismatch: aggregate=%d, sum=%d", service.StorageSummary.TotalAssetSize, sumAssetSize)
	}

	t.Logf("Storage aggregation verified: dat=%d, db=%d, asset=%d", sumDatSize, sumDbSize, sumAssetSize)
}

// TestServiceInfoVersionConsistency verifies version info matches constants
func TestServiceInfoVersionConsistency(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	topicsResp := ts.GetTopics(t)
	service := topicsResp.Service

	if service == nil {
		t.Fatal("service field is nil")
	}

	if service.VersionInfo.BlobVersion != constants.BlobVersion {
		t.Errorf("blob_version mismatch: got %d, expected %d", service.VersionInfo.BlobVersion, constants.BlobVersion)
	}

	if service.VersionInfo.HeaderSize != constants.HeaderSize {
		t.Errorf("header_size mismatch: got %d, expected %d", service.VersionInfo.HeaderSize, constants.HeaderSize)
	}
}

// TestServiceInfoAfterRestart verifies service info is restored after restart (movability)
func TestServiceInfoAfterRestart(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	ts.CreateTopic(t, "persistent-topic")
	ts.UploadFileExpectSuccess(t, "persistent-topic", "file.bin", []byte("persistent data"), "")

	// Get service info before restart
	beforeResp := ts.GetTopics(t)
	beforeService := beforeResp.Service

	if beforeService == nil {
		t.Fatal("service field is nil before restart")
	}

	// Restart the server
	ts.Restart(t)

	// Get service info after restart
	afterResp := ts.GetTopics(t)
	afterService := afterResp.Service

	if afterService == nil {
		t.Fatal("service field is nil after restart")
	}

	// Core metrics should be preserved
	if afterService.TotalIndexedHashes != beforeService.TotalIndexedHashes {
		t.Errorf("total_indexed_hashes changed after restart: before=%d, after=%d",
			beforeService.TotalIndexedHashes, afterService.TotalIndexedHashes)
	}

	if afterService.TopicsSummary.Total != beforeService.TopicsSummary.Total {
		t.Errorf("topics_summary.total changed after restart: before=%d, after=%d",
			beforeService.TopicsSummary.Total, afterService.TopicsSummary.Total)
	}

	if afterService.StorageSummary.TotalAssetSize != beforeService.StorageSummary.TotalAssetSize {
		t.Errorf("total_asset_size changed after restart: before=%d, after=%d",
			beforeService.StorageSummary.TotalAssetSize, afterService.StorageSummary.TotalAssetSize)
	}

	t.Logf("Service info correctly restored after restart")
}

// TestServiceInfoConcurrentAccess verifies no race conditions during concurrent access
func TestServiceInfoConcurrentAccess(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "concurrent-topic")

	var wg sync.WaitGroup
	const numUploaders = 5
	const numReaders = 10
	const filesPerUploader = 3

	// Start uploaders
	for i := 0; i < numUploaders; i++ {
		wg.Add(1)
		go func(uploaderID int) {
			defer wg.Done()
			for j := 0; j < filesPerUploader; j++ {
				data := make([]byte, 50+uploaderID*10+j)
				for k := range data {
					data[k] = byte((uploaderID + j + k) % 256)
				}
				filename := "file_" + string(rune('a'+uploaderID)) + string(rune('0'+j)) + ".bin"
				ts.UploadFileExpectSuccess(t, "concurrent-topic", filename, data, "")
			}
		}(i)
	}

	// Start readers (concurrently reading /api/topics)
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				resp := ts.GetTopics(t)
				if resp.Service == nil {
					t.Error("service field is nil during concurrent access")
					return
				}
				// Basic sanity checks
				if resp.Service.TotalIndexedHashes < 0 {
					t.Errorf("negative total_indexed_hashes: %d", resp.Service.TotalIndexedHashes)
				}
			}
		}()
	}

	wg.Wait()

	// Final verification
	finalResp := ts.GetTopics(t)
	if finalResp.Service == nil {
		t.Fatal("service field is nil after concurrent operations")
	}

	// Should have some indexed hashes (exact count depends on deduplication)
	if finalResp.Service.TotalIndexedHashes == 0 {
		t.Error("expected some indexed hashes after concurrent uploads")
	}

	t.Logf("Concurrent access test passed: %d final indexed hashes", finalResp.Service.TotalIndexedHashes)
}

// TestServiceInfoImmutability verifies hash count only increases (append-only)
func TestServiceInfoImmutability(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "immutable-topic")

	// Record initial state
	initialResp := ts.GetTopics(t)
	initialHashes := initialResp.Service.TotalIndexedHashes

	// Upload files and verify count only increases
	var prevCount int64 = initialHashes
	for i := 0; i < 5; i++ {
		data := make([]byte, 100+i*50)
		for j := range data {
			data[j] = byte((i + j) % 256)
		}
		filename := "immutable_" + string(rune('a'+i)) + ".bin"
		ts.UploadFileExpectSuccess(t, "immutable-topic", filename, data, "")

		resp := ts.GetTopics(t)
		currentCount := resp.Service.TotalIndexedHashes

		if currentCount < prevCount {
			t.Errorf("hash count decreased: was %d, now %d (iteration %d)", prevCount, currentCount, i)
		}
		prevCount = currentCount
	}

	t.Logf("Immutability verified: hash count went from %d to %d", initialHashes, prevCount)
}

// TestServiceInfoDatFileCount verifies total_dat_files matches actual count
func TestServiceInfoDatFileCount(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Create 2 topics
	ts.CreateTopic(t, "dat-count-1")
	ts.CreateTopic(t, "dat-count-2")

	// Upload to create .dat files
	ts.UploadFileExpectSuccess(t, "dat-count-1", "file.bin", []byte("data1"), "")
	ts.UploadFileExpectSuccess(t, "dat-count-2", "file.bin", []byte("data2"), "")

	topicsResp := ts.GetTopics(t)
	service := topicsResp.Service

	// Count actual .dat files
	var actualDatFiles int
	for _, topicName := range []string{"dat-count-1", "dat-count-2"} {
		topicPath := filepath.Join(ts.WorkDir, topicName)
		count, err := storage.CountDatFiles(topicPath)
		if err != nil {
			t.Fatalf("Failed to count dat files for %s: %v", topicName, err)
		}
		actualDatFiles += count
	}

	if service.StorageSummary.TotalDatFiles != actualDatFiles {
		t.Errorf("total_dat_files mismatch: API=%d, actual=%d", service.StorageSummary.TotalDatFiles, actualDatFiles)
	} else {
		t.Logf("total_dat_files correct: %d", service.StorageSummary.TotalDatFiles)
	}
}

// TestServiceInfoAvgAssetSize verifies avg_asset_size calculation
func TestServiceInfoAvgAssetSize(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "avg-size-test")

	// Upload files of known sizes
	sizes := []int{100, 200, 300, 400}
	for i, size := range sizes {
		data := make([]byte, size)
		for j := range data {
			data[j] = byte((i + j) % 256)
		}
		filename := "avg_" + string(rune('a'+i)) + ".bin"
		ts.UploadFileExpectSuccess(t, "avg-size-test", filename, data, "")
	}

	topicsResp := ts.GetTopics(t)
	service := topicsResp.Service

	// Calculate expected average
	totalSize := 0
	for _, s := range sizes {
		totalSize += s
	}
	expectedAvg := float64(totalSize) / float64(len(sizes))

	// Allow small floating point tolerance
	tolerance := 0.01
	diff := service.StorageSummary.AvgAssetSize - expectedAvg
	if diff < -tolerance || diff > tolerance {
		t.Errorf("avg_asset_size mismatch: API=%f, expected=%f", service.StorageSummary.AvgAssetSize, expectedAvg)
	} else {
		t.Logf("avg_asset_size correct: %f (expected %f)", service.StorageSummary.AvgAssetSize, expectedAvg)
	}
}
