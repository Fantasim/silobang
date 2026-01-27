package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"meshbank/internal/constants"
	"meshbank/internal/logger"
	"meshbank/internal/queries"
)

// TestQueriesDirAutoGeneration verifies that the queries directory and files
// are automatically generated when they don't exist
func TestQueriesDirAutoGeneration(t *testing.T) {
	// Create a temp working directory
	workDir, err := os.MkdirTemp("", "meshbank-queries-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(workDir)

	// Create the .internal directory (simulating an existing project)
	internalDir := filepath.Join(workDir, constants.InternalDir)
	if err := os.MkdirAll(internalDir, 0755); err != nil {
		t.Fatalf("failed to create .internal dir: %v", err)
	}

	log := logger.NewLogger(logger.LevelError)

	// Verify queries directory doesn't exist yet
	queriesDir := queries.GetQueriesDir(workDir)
	if _, err := os.Stat(queriesDir); !os.IsNotExist(err) {
		t.Fatalf("queries dir should not exist yet")
	}

	// Call EnsureQueriesDir - should generate defaults
	if err := queries.EnsureQueriesDir(workDir, log); err != nil {
		t.Fatalf("EnsureQueriesDir failed: %v", err)
	}

	// Verify queries directory was created
	if _, err := os.Stat(queriesDir); os.IsNotExist(err) {
		t.Fatalf("queries dir should exist after EnsureQueriesDir")
	}

	// Verify stats directory was created with files
	statsDir := filepath.Join(queriesDir, constants.QueriesStatsDir)
	statEntries, err := os.ReadDir(statsDir)
	if err != nil {
		t.Fatalf("failed to read stats dir: %v", err)
	}

	expectedStats := len(queries.GetDefaultStats())
	if len(statEntries) != expectedStats {
		t.Errorf("expected %d stat files, got %d", expectedStats, len(statEntries))
	}

	// Verify presets directory was created with files
	presetsDir := filepath.Join(queriesDir, constants.QueriesPresetsDir)
	presetEntries, err := os.ReadDir(presetsDir)
	if err != nil {
		t.Fatalf("failed to read presets dir: %v", err)
	}

	expectedPresets := len(queries.GetDefaultPresets())
	if len(presetEntries) != expectedPresets {
		t.Errorf("expected %d preset files, got %d", expectedPresets, len(presetEntries))
	}
}

// TestQueriesLoadFromSplitFiles verifies that queries can be loaded
// from the split file structure
func TestQueriesLoadFromSplitFiles(t *testing.T) {
	// Create a temp working directory
	workDir, err := os.MkdirTemp("", "meshbank-queries-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(workDir)

	log := logger.NewLogger(logger.LevelError)

	// Generate default queries
	if err := queries.GenerateDefaultQueries(workDir, log); err != nil {
		t.Fatalf("GenerateDefaultQueries failed: %v", err)
	}

	// Load queries from the split files
	config, err := queries.LoadQueriesFromDir(workDir, log)
	if err != nil {
		t.Fatalf("LoadQueriesFromDir failed: %v", err)
	}

	// Verify stats were loaded
	expectedStats := len(queries.GetDefaultStats())
	if len(config.TopicStats) != expectedStats {
		t.Errorf("expected %d topic stats, got %d", expectedStats, len(config.TopicStats))
	}

	// Verify presets were loaded
	expectedPresets := len(queries.GetDefaultPresets())
	if len(config.Presets) != expectedPresets {
		t.Errorf("expected %d presets, got %d", expectedPresets, len(config.Presets))
	}

	// Verify a specific stat
	foundTotalSize := false
	for _, stat := range config.TopicStats {
		if stat.Name == "total_size" {
			foundTotalSize = true
			if stat.Label != "Total Size" {
				t.Errorf("expected label 'Total Size', got '%s'", stat.Label)
			}
			if stat.Format != constants.StatFormatBytes {
				t.Errorf("expected format 'bytes', got '%s'", stat.Format)
			}
			break
		}
	}
	if !foundTotalSize {
		t.Error("total_size stat not found")
	}

	// Verify a specific preset
	preset, exists := config.Presets["recent-imports"]
	if !exists {
		t.Error("recent-imports preset not found")
	} else {
		if preset.Description != "Assets imported in last N days" {
			t.Errorf("unexpected description: %s", preset.Description)
		}
		if len(preset.Params) != 2 {
			t.Errorf("expected 2 params, got %d", len(preset.Params))
		}
	}
}

// TestQueriesImmutability verifies that generated files match the embedded defaults
func TestQueriesImmutability(t *testing.T) {
	// Create a temp working directory
	workDir, err := os.MkdirTemp("", "meshbank-queries-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(workDir)

	log := logger.NewLogger(logger.LevelError)

	// Generate default queries
	if err := queries.GenerateDefaultQueries(workDir, log); err != nil {
		t.Fatalf("GenerateDefaultQueries failed: %v", err)
	}

	// Load from split files
	loadedConfig, err := queries.LoadQueriesFromDir(workDir, log)
	if err != nil {
		t.Fatalf("LoadQueriesFromDir failed: %v", err)
	}

	// Get embedded defaults
	defaultConfig := queries.GetDefaultConfig()

	// Compare stats count
	if len(loadedConfig.TopicStats) != len(defaultConfig.TopicStats) {
		t.Errorf("stats count mismatch: loaded=%d, default=%d",
			len(loadedConfig.TopicStats), len(defaultConfig.TopicStats))
	}

	// Compare presets count
	if len(loadedConfig.Presets) != len(defaultConfig.Presets) {
		t.Errorf("presets count mismatch: loaded=%d, default=%d",
			len(loadedConfig.Presets), len(defaultConfig.Presets))
	}

	// Verify each default preset exists in loaded config
	for name, defaultPreset := range defaultConfig.Presets {
		loadedPreset, exists := loadedConfig.Presets[name]
		if !exists {
			t.Errorf("preset %s missing from loaded config", name)
			continue
		}

		if loadedPreset.Description != defaultPreset.Description {
			t.Errorf("preset %s description mismatch: loaded=%s, default=%s",
				name, loadedPreset.Description, defaultPreset.Description)
		}

		if len(loadedPreset.Params) != len(defaultPreset.Params) {
			t.Errorf("preset %s params count mismatch: loaded=%d, default=%d",
				name, len(loadedPreset.Params), len(defaultPreset.Params))
		}
	}
}

// TestQueriesValidation verifies that invalid files are rejected with proper errors
func TestQueriesValidation(t *testing.T) {
	// Create a temp working directory
	workDir, err := os.MkdirTemp("", "meshbank-queries-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(workDir)

	log := logger.NewLogger(logger.LevelError)

	// Generate default queries first
	if err := queries.GenerateDefaultQueries(workDir, log); err != nil {
		t.Fatalf("GenerateDefaultQueries failed: %v", err)
	}

	queriesDir := queries.GetQueriesDir(workDir)
	presetsDir := filepath.Join(queriesDir, constants.QueriesPresetsDir)

	// Create an invalid preset file (missing SQL)
	invalidPreset := `description: "Invalid preset without SQL"`
	invalidPath := filepath.Join(presetsDir, "invalid-preset.yaml")
	if err := os.WriteFile(invalidPath, []byte(invalidPreset), 0644); err != nil {
		t.Fatalf("failed to write invalid preset: %v", err)
	}

	// Load queries - should skip the invalid file but continue
	config, err := queries.LoadQueriesFromDir(workDir, log)
	if err != nil {
		t.Fatalf("LoadQueriesFromDir should not fail entirely: %v", err)
	}

	// Invalid preset should be skipped
	if _, exists := config.Presets["invalid-preset"]; exists {
		t.Error("invalid preset should not have been loaded")
	}

	// Valid presets should still be loaded
	defaultPresets := queries.GetDefaultPresets()
	if len(config.Presets) != len(defaultPresets) {
		t.Errorf("expected %d valid presets, got %d", len(defaultPresets), len(config.Presets))
	}
}

// TestQueriesCustomQuery verifies that custom user queries work alongside defaults
func TestQueriesCustomQuery(t *testing.T) {
	// Create a temp working directory
	workDir, err := os.MkdirTemp("", "meshbank-queries-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(workDir)

	log := logger.NewLogger(logger.LevelError)

	// Generate default queries
	if err := queries.GenerateDefaultQueries(workDir, log); err != nil {
		t.Fatalf("GenerateDefaultQueries failed: %v", err)
	}

	queriesDir := queries.GetQueriesDir(workDir)
	presetsDir := filepath.Join(queriesDir, constants.QueriesPresetsDir)

	// Create a custom preset file
	customPreset := `description: "Custom user query"
sql: "SELECT * FROM assets WHERE extension = :ext LIMIT 10"
params:
  - name: ext
    required: true
`
	customPath := filepath.Join(presetsDir, "custom-query.yaml")
	if err := os.WriteFile(customPath, []byte(customPreset), 0644); err != nil {
		t.Fatalf("failed to write custom preset: %v", err)
	}

	// Load queries
	config, err := queries.LoadQueriesFromDir(workDir, log)
	if err != nil {
		t.Fatalf("LoadQueriesFromDir failed: %v", err)
	}

	// Verify custom query was loaded
	customQuery, exists := config.Presets["custom-query"]
	if !exists {
		t.Fatal("custom-query preset not found")
	}

	if customQuery.Description != "Custom user query" {
		t.Errorf("unexpected description: %s", customQuery.Description)
	}

	if len(customQuery.Params) != 1 {
		t.Errorf("expected 1 param, got %d", len(customQuery.Params))
	}

	if customQuery.Params[0].Name != "ext" {
		t.Errorf("expected param name 'ext', got '%s'", customQuery.Params[0].Name)
	}

	if !customQuery.Params[0].Required {
		t.Error("expected param to be required")
	}

	// Verify default queries still work
	_, exists = config.Presets["recent-imports"]
	if !exists {
		t.Error("recent-imports preset should still exist")
	}
}

// TestQueriesMovability verifies that queries work when the project folder is moved
func TestQueriesMovability(t *testing.T) {
	// Create first temp working directory
	workDir1, err := os.MkdirTemp("", "meshbank-queries-test-src-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(workDir1)

	log := logger.NewLogger(logger.LevelError)

	// Generate default queries in first location
	if err := queries.GenerateDefaultQueries(workDir1, log); err != nil {
		t.Fatalf("GenerateDefaultQueries failed: %v", err)
	}

	// Load from first location
	config1, err := queries.LoadQueriesFromDir(workDir1, log)
	if err != nil {
		t.Fatalf("LoadQueriesFromDir failed: %v", err)
	}

	// Create second temp working directory (simulating move)
	workDir2, err := os.MkdirTemp("", "meshbank-queries-test-dst-*")
	if err != nil {
		t.Fatalf("failed to create second temp dir: %v", err)
	}
	defer os.RemoveAll(workDir2)

	// Copy the .internal/queries directory to new location
	srcQueriesDir := queries.GetQueriesDir(workDir1)
	dstQueriesDir := queries.GetQueriesDir(workDir2)

	// Create destination .internal directory
	if err := os.MkdirAll(filepath.Dir(dstQueriesDir), 0755); err != nil {
		t.Fatalf("failed to create dest .internal dir: %v", err)
	}

	// Copy recursively
	if err := copyDir(srcQueriesDir, dstQueriesDir); err != nil {
		t.Fatalf("failed to copy queries dir: %v", err)
	}

	// Load from second location
	config2, err := queries.LoadQueriesFromDir(workDir2, log)
	if err != nil {
		t.Fatalf("LoadQueriesFromDir from moved dir failed: %v", err)
	}

	// Verify both configs have the same content
	if len(config1.TopicStats) != len(config2.TopicStats) {
		t.Errorf("stats count differs after move: %d vs %d",
			len(config1.TopicStats), len(config2.TopicStats))
	}

	if len(config1.Presets) != len(config2.Presets) {
		t.Errorf("presets count differs after move: %d vs %d",
			len(config1.Presets), len(config2.Presets))
	}
}

// TestLoadQueriesIntegration tests the full LoadQueries function
func TestLoadQueriesIntegration(t *testing.T) {
	// Create a temp working directory
	workDir, err := os.MkdirTemp("", "meshbank-queries-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(workDir)

	log := logger.NewLogger(logger.LevelError)

	// LoadQueries should auto-generate and then load
	config, err := queries.LoadQueries(workDir, log)
	if err != nil {
		t.Fatalf("LoadQueries failed: %v", err)
	}

	// Verify config is populated
	if len(config.TopicStats) == 0 {
		t.Error("expected topic stats to be loaded")
	}

	if len(config.Presets) == 0 {
		t.Error("expected presets to be loaded")
	}

	// Call LoadQueries again - should work without regenerating
	config2, err := queries.LoadQueries(workDir, log)
	if err != nil {
		t.Fatalf("second LoadQueries failed: %v", err)
	}

	if len(config2.TopicStats) != len(config.TopicStats) {
		t.Error("stats count should be consistent across calls")
	}
}

// TestQueriesAutoGeneratedOnAPIConfig verifies that queries are auto-generated
// when setting the working directory via the API
func TestQueriesAutoGeneratedOnAPIConfig(t *testing.T) {
	ts := StartTestServer(t)

	// Verify queries directory doesn't exist yet
	queriesDir := queries.GetQueriesDir(ts.WorkDir)
	if _, err := os.Stat(queriesDir); !os.IsNotExist(err) {
		t.Fatalf("queries dir should not exist before config")
	}

	// Configure working directory via API
	ts.ConfigureWorkDir(t)

	// Verify queries directory was created
	if _, err := os.Stat(queriesDir); os.IsNotExist(err) {
		t.Fatalf("queries dir should exist after ConfigureWorkDir")
	}

	// Verify stats directory exists and has files
	statsDir := filepath.Join(queriesDir, constants.QueriesStatsDir)
	statEntries, err := os.ReadDir(statsDir)
	if err != nil {
		t.Fatalf("failed to read stats dir: %v", err)
	}

	if len(statEntries) == 0 {
		t.Error("expected stat files to be generated")
	}

	// Verify presets directory exists and has files
	presetsDir := filepath.Join(queriesDir, constants.QueriesPresetsDir)
	presetEntries, err := os.ReadDir(presetsDir)
	if err != nil {
		t.Fatalf("failed to read presets dir: %v", err)
	}

	if len(presetEntries) == 0 {
		t.Error("expected preset files to be generated")
	}

	// Verify we can use presets via API
	ts.CreateTopic(t, "test-topic")
	ts.UploadFileExpectSuccess(t, "test-topic", "test.txt", []byte("hello"), "")

	// Execute a query to verify presets work
	result := ts.ExecuteQuery(t, "count", []string{"test-topic"}, nil)
	if len(result.Columns) == 0 {
		t.Error("query should return columns")
	}
}

// copyDir is defined in portability_test.go
