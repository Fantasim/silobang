package e2e

import (
	"testing"
	"time"
)

// TestEnhancedPresetsReturnFullFields verifies that enhanced presets return all expected columns
func TestEnhancedPresetsReturnFullFields(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "enhanced-presets")

	// Upload test files
	ts.UploadFileExpectSuccess(t, "enhanced-presets", "test1.png", GenerateTestFile(1024), "")
	ts.UploadFileExpectSuccess(t, "enhanced-presets", "test2.glb", GenerateTestFile(2048), "")

	time.Sleep(100 * time.Millisecond)

	// Expected columns for asset queries
	expectedAssetColumns := []string{"asset_id", "origin_name", "extension", "asset_size", "parent_id", "blob_name", "created_at"}

	// Test recent-imports has all columns
	t.Run("recent-imports columns", func(t *testing.T) {
		resp := ts.ExecuteQuery(t, "recent-imports", []string{"enhanced-presets"}, map[string]interface{}{
			"days":  7,
			"limit": 10,
		})
		verifyColumnsExist(t, resp.Columns, expectedAssetColumns)
	})

	// Test by-hash has all columns
	t.Run("by-hash columns", func(t *testing.T) {
		// First get a hash
		countResp := ts.ExecuteQuery(t, "recent-imports", []string{"enhanced-presets"}, map[string]interface{}{
			"days":  7,
			"limit": 1,
		})
		if len(countResp.Rows) == 0 {
			t.Fatal("No assets found for by-hash test")
		}
		hash := countResp.Rows[0][0].(string)[:3]

		resp := ts.ExecuteQuery(t, "by-hash", []string{"enhanced-presets"}, map[string]interface{}{
			"hash": hash,
		})
		verifyColumnsExist(t, resp.Columns, expectedAssetColumns)
	})

	// Test large-files has all columns
	t.Run("large-files columns", func(t *testing.T) {
		resp := ts.ExecuteQuery(t, "large-files", []string{"enhanced-presets"}, map[string]interface{}{
			"min_size": 1,
			"limit":    10,
		})
		verifyColumnsExist(t, resp.Columns, expectedAssetColumns)
	})
}

// TestNewTopicStats verifies the new topic stats are calculated correctly
func TestNewTopicStats(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "stats-topic")

	// Upload files with different extensions
	rootHash := ts.UploadFileExpectSuccess(t, "stats-topic", "model.glb", GenerateTestFile(1024), "").Hash
	ts.UploadFileExpectSuccess(t, "stats-topic", "texture.png", GenerateTestFile(512), "")
	ts.UploadFileExpectSuccess(t, "stats-topic", "texture2.png", GenerateTestFile(512), "")
	// Create a versioned file (child of root)
	ts.UploadFileExpectSuccess(t, "stats-topic", "model_v2.glb", GenerateTestFile(1536), rootHash)

	// Add metadata to some files
	ts.SetMetadata(t, rootHash, "processor", "test")

	time.Sleep(100 * time.Millisecond)

	// Get topic stats
	topicsResp := ts.GetTopics(t)

	var topic *TopicInfo
	for i := range topicsResp.Topics {
		if topicsResp.Topics[i].Name == "stats-topic" {
			topic = &topicsResp.Topics[i]
			break
		}
	}

	if topic == nil {
		t.Fatal("Topic 'stats-topic' not found")
	}

	t.Logf("Topic stats: %+v", topic.Stats)

	// Test unique_extensions (should be 2: glb, png)
	t.Run("unique_extensions", func(t *testing.T) {
		val, ok := topic.Stats["unique_extensions"]
		if !ok {
			t.Fatal("unique_extensions not found in stats")
		}
		count := int64(val.(float64))
		if count != 2 {
			t.Errorf("Expected 2 unique extensions, got %d", count)
		}
	})

	// Test versioned_count (should be 1: model_v2.glb has parent)
	t.Run("versioned_count", func(t *testing.T) {
		val, ok := topic.Stats["versioned_count"]
		if !ok {
			t.Fatal("versioned_count not found in stats")
		}
		count := int64(val.(float64))
		if count != 1 {
			t.Errorf("Expected 1 versioned asset, got %d", count)
		}
	})

	// Test root_count (should be 3: model.glb, texture.png, texture2.png)
	t.Run("root_count", func(t *testing.T) {
		val, ok := topic.Stats["root_count"]
		if !ok {
			t.Fatal("root_count not found in stats")
		}
		count := int64(val.(float64))
		if count != 3 {
			t.Errorf("Expected 3 root assets, got %d", count)
		}
	})

	// Test metadata_coverage (should be 1: only rootHash has metadata)
	t.Run("metadata_coverage", func(t *testing.T) {
		val, ok := topic.Stats["metadata_coverage"]
		if !ok {
			t.Fatal("metadata_coverage not found in stats")
		}
		count := int64(val.(float64))
		if count != 1 {
			t.Errorf("Expected 1 asset with metadata, got %d", count)
		}
	})

	// Test dat_file_count (should be 1: all assets in one dat file)
	t.Run("dat_file_count", func(t *testing.T) {
		val, ok := topic.Stats["dat_file_count"]
		if !ok {
			t.Fatal("dat_file_count not found in stats")
		}
		count := int64(val.(float64))
		if count != 1 {
			t.Errorf("Expected 1 dat file, got %d", count)
		}
	})

	// Test oldest_asset (should be set)
	t.Run("oldest_asset", func(t *testing.T) {
		val, ok := topic.Stats["oldest_asset"]
		if !ok {
			t.Fatal("oldest_asset not found in stats")
		}
		if val == nil {
			t.Error("oldest_asset should not be nil")
		}
	})
}

// TestExtensionSummaryPreset tests the extension-summary aggregation query
func TestExtensionSummaryPreset(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "ext-summary")

	// Upload files with varied extensions
	ts.UploadFileExpectSuccess(t, "ext-summary", "file1.png", GenerateTestFile(1000), "")
	ts.UploadFileExpectSuccess(t, "ext-summary", "file2.png", GenerateTestFile(2000), "")
	ts.UploadFileExpectSuccess(t, "ext-summary", "file3.png", GenerateTestFile(3000), "")
	ts.UploadFileExpectSuccess(t, "ext-summary", "model.glb", GenerateTestFile(5000), "")

	time.Sleep(100 * time.Millisecond)

	resp := ts.ExecuteQuery(t, "extension-summary", []string{"ext-summary"}, map[string]interface{}{
		"limit": 10,
	})

	// Should have 2 rows (png, glb)
	if resp.RowCount != 2 {
		t.Errorf("Expected 2 extension groups, got %d", resp.RowCount)
	}

	// Verify expected columns
	expectedColumns := []string{"extension", "count", "total_size", "avg_size", "oldest", "newest"}
	verifyColumnsExist(t, resp.Columns, expectedColumns)

	// Find png row and verify count
	extIdx := findColumnIndex(resp.Columns, "extension")
	countIdx := findColumnIndex(resp.Columns, "count")
	totalSizeIdx := findColumnIndex(resp.Columns, "total_size")

	for _, row := range resp.Rows {
		ext := row[extIdx].(string)
		count := int64(row[countIdx].(float64))
		totalSize := int64(row[totalSizeIdx].(float64))

		if ext == "png" {
			if count != 3 {
				t.Errorf("Expected 3 png files, got %d", count)
			}
			if totalSize != 6000 {
				t.Errorf("Expected total png size 6000, got %d", totalSize)
			}
		} else if ext == "glb" {
			if count != 1 {
				t.Errorf("Expected 1 glb file, got %d", count)
			}
			if totalSize != 5000 {
				t.Errorf("Expected total glb size 5000, got %d", totalSize)
			}
		}
	}
}

// TestSizeDistributionPreset tests the size-distribution grouping query
func TestSizeDistributionPreset(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "size-dist")

	// Upload files of varied sizes
	ts.UploadFileExpectSuccess(t, "size-dist", "tiny.bin", GenerateTestFile(500), "")           // < 1KB
	ts.UploadFileExpectSuccess(t, "size-dist", "small.bin", GenerateTestFile(50*1024), "")      // 1KB-1MB
	ts.UploadFileExpectSuccess(t, "size-dist", "medium.bin", GenerateTestFile(5*1024*1024), "") // 1MB-10MB

	time.Sleep(100 * time.Millisecond)

	resp := ts.ExecuteQuery(t, "size-distribution", []string{"size-dist"}, nil)

	// Should have 3 rows (tiny, small, medium)
	if resp.RowCount != 3 {
		t.Errorf("Expected 3 size ranges, got %d", resp.RowCount)
	}

	// Verify expected columns
	expectedColumns := []string{"size_range", "count", "total_size", "avg_size"}
	verifyColumnsExist(t, resp.Columns, expectedColumns)

	// Verify each row has count = 1
	countIdx := findColumnIndex(resp.Columns, "count")
	for _, row := range resp.Rows {
		count := int64(row[countIdx].(float64))
		if count != 1 {
			t.Errorf("Expected count 1 per size range, got %d", count)
		}
	}
}

// TestTimeSeriesPreset tests the time-series daily aggregation
func TestTimeSeriesPreset(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "time-series")

	// Upload files
	ts.UploadFileExpectSuccess(t, "time-series", "file1.bin", GenerateTestFile(1024), "")
	ts.UploadFileExpectSuccess(t, "time-series", "file2.bin", GenerateTestFile(2048), "")

	time.Sleep(100 * time.Millisecond)

	resp := ts.ExecuteQuery(t, "time-series", []string{"time-series"}, map[string]interface{}{
		"days": 30,
	})

	// Should have at least 1 row (today)
	if resp.RowCount < 1 {
		t.Errorf("Expected at least 1 day in time series, got %d", resp.RowCount)
	}

	// Verify expected columns
	expectedColumns := []string{"date", "count", "total_size"}
	verifyColumnsExist(t, resp.Columns, expectedColumns)

	// Verify today's row has correct count
	countIdx := findColumnIndex(resp.Columns, "count")
	totalSizeIdx := findColumnIndex(resp.Columns, "total_size")

	// Should have 2 files total
	totalCount := int64(0)
	totalSize := int64(0)
	for _, row := range resp.Rows {
		totalCount += int64(row[countIdx].(float64))
		totalSize += int64(row[totalSizeIdx].(float64))
	}

	if totalCount != 2 {
		t.Errorf("Expected total count 2, got %d", totalCount)
	}
	if totalSize != 3072 {
		t.Errorf("Expected total size 3072, got %d", totalSize)
	}
}

// TestByExtensionPreset tests filtering assets by extension
func TestByExtensionPreset(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "by-ext")

	// Upload mixed file types
	ts.UploadFileExpectSuccess(t, "by-ext", "image1.png", GenerateTestFile(1024), "")
	ts.UploadFileExpectSuccess(t, "by-ext", "image2.png", GenerateTestFile(1024), "")
	ts.UploadFileExpectSuccess(t, "by-ext", "model.glb", GenerateTestFile(2048), "")
	ts.UploadFileExpectSuccess(t, "by-ext", "texture.jpg", GenerateTestFile(512), "")

	time.Sleep(100 * time.Millisecond)

	// Query for png files only
	resp := ts.ExecuteQuery(t, "by-extension", []string{"by-ext"}, map[string]interface{}{
		"ext":   "png",
		"limit": 100,
	})

	if resp.RowCount != 2 {
		t.Errorf("Expected 2 png files, got %d", resp.RowCount)
	}

	// Verify all returned files have png extension
	extIdx := findColumnIndex(resp.Columns, "extension")
	for _, row := range resp.Rows {
		ext := row[extIdx].(string)
		if ext != "png" {
			t.Errorf("Expected extension 'png', got '%s'", ext)
		}
	}
}

// TestByOriginNamePreset tests partial filename search
func TestByOriginNamePreset(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "by-name")

	// Upload files with varied names
	ts.UploadFileExpectSuccess(t, "by-name", "character_hero.glb", GenerateTestFile(1024), "")
	ts.UploadFileExpectSuccess(t, "by-name", "character_villain.glb", GenerateTestFile(1024), "")
	ts.UploadFileExpectSuccess(t, "by-name", "environment.glb", GenerateTestFile(1024), "")
	ts.UploadFileExpectSuccess(t, "by-name", "hero_texture.png", GenerateTestFile(512), "")

	time.Sleep(100 * time.Millisecond)

	// Search for "character"
	resp := ts.ExecuteQuery(t, "by-origin-name", []string{"by-name"}, map[string]interface{}{
		"name":  "character",
		"limit": 100,
	})

	if resp.RowCount != 2 {
		t.Errorf("Expected 2 files with 'character' in name, got %d", resp.RowCount)
	}

	// Search for "hero"
	resp = ts.ExecuteQuery(t, "by-origin-name", []string{"by-name"}, map[string]interface{}{
		"name":  "hero",
		"limit": 100,
	})

	if resp.RowCount != 2 {
		t.Errorf("Expected 2 files with 'hero' in name, got %d", resp.RowCount)
	}
}

// TestOrphansPreset tests finding root assets without children
func TestOrphansPreset(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "orphans-test")

	// Create root with child
	rootHash := ts.UploadFileExpectSuccess(t, "orphans-test", "parent.glb", GenerateTestFile(1024), "").Hash
	ts.UploadFileExpectSuccess(t, "orphans-test", "child.glb", GenerateTestFile(1024), rootHash)

	// Create orphan (root without children)
	ts.UploadFileExpectSuccess(t, "orphans-test", "orphan.glb", GenerateTestFile(1024), "")

	time.Sleep(100 * time.Millisecond)

	resp := ts.ExecuteQuery(t, "orphans", []string{"orphans-test"}, map[string]interface{}{
		"limit": 100,
	})

	// Should return only 1 orphan (root without children)
	if resp.RowCount != 1 {
		t.Errorf("Expected 1 orphan, got %d", resp.RowCount)
	}

	// Verify the orphan's origin_name
	originIdx := findColumnIndex(resp.Columns, "origin_name")
	originName := resp.Rows[0][originIdx].(string)
	if originName != "orphan" {
		t.Errorf("Expected orphan 'orphan', got '%s'", originName)
	}
}

// TestRootsWithChildrenPreset tests finding root assets with derived versions
func TestRootsWithChildrenPreset(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "roots-children")

	// Create root with children
	rootHash := ts.UploadFileExpectSuccess(t, "roots-children", "parent.glb", GenerateTestFile(1024), "").Hash
	ts.UploadFileExpectSuccess(t, "roots-children", "child1.glb", GenerateTestFile(1024), rootHash)
	ts.UploadFileExpectSuccess(t, "roots-children", "child2.glb", GenerateTestFile(1024), rootHash)

	// Create orphan
	ts.UploadFileExpectSuccess(t, "roots-children", "orphan.glb", GenerateTestFile(1024), "")

	time.Sleep(100 * time.Millisecond)

	resp := ts.ExecuteQuery(t, "roots-with-children", []string{"roots-children"}, map[string]interface{}{
		"limit": 100,
	})

	// Should return only 1 root with children
	if resp.RowCount != 1 {
		t.Errorf("Expected 1 root with children, got %d", resp.RowCount)
	}

	// Verify direct_children count
	childrenIdx := findColumnIndex(resp.Columns, "direct_children")
	directChildren := int64(resp.Rows[0][childrenIdx].(float64))
	if directChildren != 2 {
		t.Errorf("Expected 2 direct children, got %d", directChildren)
	}
}

// TestMetadataHistoryPreset tests viewing metadata change history
func TestMetadataHistoryPreset(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "meta-history")

	// Upload file
	hash := ts.UploadFileExpectSuccess(t, "meta-history", "test.glb", GenerateTestFile(1024), "").Hash

	// Set multiple metadata entries
	ts.SetMetadata(t, hash, "key1", "value1")
	ts.SetMetadata(t, hash, "key2", "value2")
	ts.SetMetadata(t, hash, "key1", "updated_value1") // Update key1

	time.Sleep(100 * time.Millisecond)

	resp := ts.ExecuteQuery(t, "metadata-history", []string{"meta-history"}, map[string]interface{}{
		"hash":  hash,
		"limit": 100,
	})

	// Should have 3 log entries
	if resp.RowCount != 3 {
		t.Errorf("Expected 3 metadata log entries, got %d", resp.RowCount)
	}

	// Verify expected columns
	expectedColumns := []string{"id", "op", "key", "value_text", "processor", "processor_version", "timestamp"}
	verifyColumnsExist(t, resp.Columns, expectedColumns)
}

// TestWithoutMetadataPreset tests finding assets missing metadata
func TestWithoutMetadataPreset(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "no-meta")

	// Upload files
	hash1 := ts.UploadFileExpectSuccess(t, "no-meta", "with_meta.glb", GenerateTestFile(1024), "").Hash
	ts.UploadFileExpectSuccess(t, "no-meta", "without_meta1.glb", GenerateTestFile(1024), "")
	ts.UploadFileExpectSuccess(t, "no-meta", "without_meta2.glb", GenerateTestFile(1024), "")

	// Add metadata to only one file
	ts.SetMetadata(t, hash1, "test_key", "test_value")

	time.Sleep(100 * time.Millisecond)

	resp := ts.ExecuteQuery(t, "without-metadata", []string{"no-meta"}, map[string]interface{}{
		"limit": 100,
	})

	// Should return 2 files without metadata
	if resp.RowCount != 2 {
		t.Errorf("Expected 2 files without metadata, got %d", resp.RowCount)
	}
}

// TestDatFileStatsPreset tests DAT file statistics query
func TestDatFileStatsPreset(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "dat-stats")

	// Upload files
	ts.UploadFileExpectSuccess(t, "dat-stats", "file1.bin", GenerateTestFile(1024), "")
	ts.UploadFileExpectSuccess(t, "dat-stats", "file2.bin", GenerateTestFile(2048), "")
	ts.UploadFileExpectSuccess(t, "dat-stats", "file3.bin", GenerateTestFile(512), "")

	time.Sleep(100 * time.Millisecond)

	resp := ts.ExecuteQuery(t, "dat-file-stats", []string{"dat-stats"}, nil)

	// Should have 1 dat file
	if resp.RowCount != 1 {
		t.Errorf("Expected 1 dat file, got %d", resp.RowCount)
	}

	// Verify expected columns
	expectedColumns := []string{"dat_file", "entry_count", "running_hash", "updated_at", "total_data_size", "asset_count"}
	verifyColumnsExist(t, resp.Columns, expectedColumns)

	// Verify entry_count and asset_count match
	entryCountIdx := findColumnIndex(resp.Columns, "entry_count")
	assetCountIdx := findColumnIndex(resp.Columns, "asset_count")
	totalSizeIdx := findColumnIndex(resp.Columns, "total_data_size")

	entryCount := int64(resp.Rows[0][entryCountIdx].(float64))
	assetCount := int64(resp.Rows[0][assetCountIdx].(float64))
	totalSize := int64(resp.Rows[0][totalSizeIdx].(float64))

	if entryCount != 3 {
		t.Errorf("Expected entry_count 3, got %d", entryCount)
	}
	if assetCount != 3 {
		t.Errorf("Expected asset_count 3, got %d", assetCount)
	}
	if totalSize != 3584 { // 1024 + 2048 + 512
		t.Errorf("Expected total_data_size 3584, got %d", totalSize)
	}
}

// TestEnhancedLineagePreset tests enhanced lineage with full fields
func TestEnhancedLineagePreset(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "lineage-enhanced")

	// Create lineage chain: grandparent -> parent -> child
	grandparentHash := ts.UploadFileExpectSuccess(t, "lineage-enhanced", "grandparent.glb", GenerateTestFile(1024), "").Hash
	parentHash := ts.UploadFileExpectSuccess(t, "lineage-enhanced", "parent.glb", GenerateTestFile(2048), grandparentHash).Hash
	childHash := ts.UploadFileExpectSuccess(t, "lineage-enhanced", "child.glb", GenerateTestFile(3072), parentHash).Hash

	time.Sleep(100 * time.Millisecond)

	// Query lineage from child
	resp := ts.ExecuteQuery(t, "lineage", []string{"lineage-enhanced"}, map[string]interface{}{
		"hash": childHash,
	})

	// Should return 3 entries (child, parent, grandparent)
	if resp.RowCount != 3 {
		t.Errorf("Expected 3 lineage entries, got %d", resp.RowCount)
	}

	// Verify enhanced columns exist
	expectedColumns := []string{"asset_id", "parent_id", "origin_name", "extension", "asset_size", "blob_name", "created_at", "depth"}
	verifyColumnsExist(t, resp.Columns, expectedColumns)

	// Verify depth ordering
	depthIdx := findColumnIndex(resp.Columns, "depth")
	for i, row := range resp.Rows {
		depth := int64(row[depthIdx].(float64))
		if depth != int64(i) {
			t.Errorf("Expected depth %d, got %d", i, depth)
		}
	}
}

// TestEnhancedDerivedPreset tests enhanced derived query with depth
func TestEnhancedDerivedPreset(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "derived-enhanced")

	// Create tree: root -> (child1, child2) where child1 -> grandchild
	rootHash := ts.UploadFileExpectSuccess(t, "derived-enhanced", "root.glb", GenerateTestFile(1024), "").Hash
	child1Hash := ts.UploadFileExpectSuccess(t, "derived-enhanced", "child1.glb", GenerateTestFile(1024), rootHash).Hash
	ts.UploadFileExpectSuccess(t, "derived-enhanced", "child2.glb", GenerateTestFile(1024), rootHash)
	ts.UploadFileExpectSuccess(t, "derived-enhanced", "grandchild.glb", GenerateTestFile(1024), child1Hash)

	time.Sleep(100 * time.Millisecond)

	// Query derived from root
	resp := ts.ExecuteQuery(t, "derived", []string{"derived-enhanced"}, map[string]interface{}{
		"hash": rootHash,
	})

	// Should return 3 descendants (child1, child2, grandchild)
	if resp.RowCount != 3 {
		t.Errorf("Expected 3 derived entries, got %d", resp.RowCount)
	}

	// Verify depth column exists and has correct values
	depthIdx := findColumnIndex(resp.Columns, "depth")
	depths := make(map[int64]int)
	for _, row := range resp.Rows {
		depth := int64(row[depthIdx].(float64))
		depths[depth]++
	}

	// Should have 2 at depth 1, 1 at depth 2
	if depths[1] != 2 {
		t.Errorf("Expected 2 at depth 1, got %d", depths[1])
	}
	if depths[2] != 1 {
		t.Errorf("Expected 1 at depth 2, got %d", depths[2])
	}
}

// Helper functions

func verifyColumnsExist(t *testing.T, actual []string, expected []string) {
	t.Helper()
	actualSet := make(map[string]bool)
	for _, col := range actual {
		actualSet[col] = true
	}

	for _, exp := range expected {
		if !actualSet[exp] {
			t.Errorf("Expected column '%s' not found in response. Actual columns: %v", exp, actual)
		}
	}
}

func findColumnIndex(columns []string, name string) int {
	for i, col := range columns {
		if col == name {
			return i
		}
	}
	return -1
}
