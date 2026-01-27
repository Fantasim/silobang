package queries

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
	"silobang/internal/constants"
	"silobang/internal/logger"
)

var queryNameRegex = regexp.MustCompile(constants.QueryNameRegex)

// EnsureQueriesDir creates the queries directory structure if it doesn't exist
// and generates default query files
func EnsureQueriesDir(workingDir string, log *logger.Logger) error {
	queriesDir := GetQueriesDir(workingDir)

	// Check if queries directory exists
	info, err := os.Stat(queriesDir)
	if err == nil && info.IsDir() {
		log.Debug("Queries directory already exists: %s", queriesDir)
		return nil
	}

	if !os.IsNotExist(err) && err != nil {
		return fmt.Errorf("failed to check queries directory: %w", err)
	}

	// Directory doesn't exist, generate defaults
	log.Info("Queries directory not found, generating defaults")
	return GenerateDefaultQueries(workingDir, log)
}

// LoadQueriesFromDir loads all query files from the split directory structure
func LoadQueriesFromDir(workingDir string, log *logger.Logger) (*QueriesConfig, error) {
	queriesDir := GetQueriesDir(workingDir)
	statsDir := filepath.Join(queriesDir, constants.QueriesStatsDir)
	presetsDir := filepath.Join(queriesDir, constants.QueriesPresetsDir)

	log.Debug("Loading queries from directory: %s", queriesDir)

	// Check if queries directory exists
	if _, err := os.Stat(queriesDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("queries directory does not exist: %s", queriesDir)
	}

	// Load stats
	stats, err := loadStats(statsDir, log)
	if err != nil {
		return nil, fmt.Errorf("failed to load stats: %w", err)
	}
	log.Info("Loaded %d topic stats from %s", len(stats), statsDir)

	// Load presets
	presets, err := loadPresets(presetsDir, log)
	if err != nil {
		return nil, fmt.Errorf("failed to load presets: %w", err)
	}
	log.Info("Loaded %d query presets from %s", len(presets), presetsDir)

	return &QueriesConfig{
		TopicStats: stats,
		Presets:    presets,
	}, nil
}

// loadStats loads all topic stat files from the stats subdirectory
func loadStats(statsDir string, log *logger.Logger) ([]TopicStat, error) {
	// Check if stats directory exists
	if _, err := os.Stat(statsDir); os.IsNotExist(err) {
		log.Warn("Stats directory does not exist: %s, using empty stats", statsDir)
		return []TopicStat{}, nil
	}

	entries, err := os.ReadDir(statsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read stats directory: %w", err)
	}

	var stats []TopicStat
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		if !strings.HasSuffix(filename, constants.QueryFileExt) {
			log.Debug("Skipping non-YAML file in stats: %s", filename)
			continue
		}

		filePath := filepath.Join(statsDir, filename)
		stat, err := loadSingleStat(filePath)
		if err != nil {
			log.Warn("Skipping invalid stat file %s: %v", filename, err)
			continue
		}

		// Validate stat name matches filename
		expectedName := strings.TrimSuffix(filename, constants.QueryFileExt)
		if stat.Name != expectedName {
			log.Warn("Stat name mismatch in %s: expected %s, got %s", filename, expectedName, stat.Name)
		}

		if err := validateStat(stat, filename); err != nil {
			log.Warn("Skipping invalid stat %s: %v", filename, err)
			continue
		}

		log.Debug("Loaded stat: %s", stat.Name)
		stats = append(stats, *stat)
	}

	return stats, nil
}

// loadPresets loads all preset files from the presets subdirectory
func loadPresets(presetsDir string, log *logger.Logger) (map[string]Preset, error) {
	// Check if presets directory exists
	if _, err := os.Stat(presetsDir); os.IsNotExist(err) {
		log.Warn("Presets directory does not exist: %s, using empty presets", presetsDir)
		return map[string]Preset{}, nil
	}

	entries, err := os.ReadDir(presetsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read presets directory: %w", err)
	}

	presets := make(map[string]Preset)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		if !strings.HasSuffix(filename, constants.QueryFileExt) {
			log.Debug("Skipping non-YAML file in presets: %s", filename)
			continue
		}

		filePath := filepath.Join(presetsDir, filename)
		preset, name, err := loadSinglePreset(filePath)
		if err != nil {
			log.Warn("Skipping invalid preset file %s: %v", filename, err)
			continue
		}

		if err := validatePreset(preset, name); err != nil {
			log.Warn("Skipping invalid preset %s: %v", name, err)
			continue
		}

		log.Debug("Loaded preset: %s", name)
		presets[name] = *preset
	}

	return presets, nil
}

// loadSingleStat loads and parses a single stat file
func loadSingleStat(filePath string) (*TopicStat, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var stat TopicStat
	if err := yaml.Unmarshal(data, &stat); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return &stat, nil
}

// loadSinglePreset loads and parses a single preset file
// Returns the preset and the name (derived from filename)
func loadSinglePreset(filePath string) (*Preset, string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read file: %w", err)
	}

	var preset Preset
	if err := yaml.Unmarshal(data, &preset); err != nil {
		return nil, "", fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Derive name from filename
	filename := filepath.Base(filePath)
	name := strings.TrimSuffix(filename, constants.QueryFileExt)

	return &preset, name, nil
}

// validateStat validates a TopicStat structure
func validateStat(stat *TopicStat, filename string) error {
	if stat.Name == "" {
		return fmt.Errorf("stat name is required")
	}

	if len(stat.Name) < constants.MinQueryNameLen || len(stat.Name) > constants.MaxQueryNameLen {
		return fmt.Errorf("stat name length must be between %d and %d", constants.MinQueryNameLen, constants.MaxQueryNameLen)
	}

	if !queryNameRegex.MatchString(stat.Name) {
		return fmt.Errorf("stat name must match pattern: %s", constants.QueryNameRegex)
	}

	if stat.Label == "" {
		return fmt.Errorf("stat label is required")
	}

	// Must have either SQL or Type
	if stat.SQL == "" && stat.Type == "" {
		return fmt.Errorf("stat must have either sql or type")
	}

	// Validate format if SQL is provided
	if stat.SQL != "" && stat.Format != "" {
		validFormats := map[string]bool{
			constants.StatFormatBytes:  true,
			constants.StatFormatNumber: true,
			constants.StatFormatFloat:  true,
			constants.StatFormatDate:   true,
			constants.StatFormatText:   true,
		}
		if !validFormats[stat.Format] {
			return fmt.Errorf("invalid stat format: %s", stat.Format)
		}
	}

	return nil
}

// validatePreset validates a Preset structure
func validatePreset(preset *Preset, name string) error {
	if len(name) < constants.MinQueryNameLen || len(name) > constants.MaxQueryNameLen {
		return fmt.Errorf("preset name length must be between %d and %d", constants.MinQueryNameLen, constants.MaxQueryNameLen)
	}

	if !queryNameRegex.MatchString(name) {
		return fmt.Errorf("preset name must match pattern: %s", constants.QueryNameRegex)
	}

	if preset.SQL == "" {
		return fmt.Errorf("preset SQL is required")
	}

	// Validate params
	for _, param := range preset.Params {
		if param.Name == "" {
			return fmt.Errorf("param name is required")
		}
	}

	return nil
}

// LoadQueries loads queries, ensuring the directory exists and generating defaults if needed
func LoadQueries(workingDir string, log *logger.Logger) (*QueriesConfig, error) {
	// Ensure queries directory exists (generates defaults if missing)
	if err := EnsureQueriesDir(workingDir, log); err != nil {
		return nil, fmt.Errorf("failed to ensure queries directory: %w", err)
	}

	// Load from split files
	return LoadQueriesFromDir(workingDir, log)
}
