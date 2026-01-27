package queries

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
	"meshbank/internal/constants"
	"meshbank/internal/logger"
)

// GenerateDefaultQueries creates all default query YAML files in the queries directory
func GenerateDefaultQueries(workingDir string, log *logger.Logger) error {
	queriesDir := GetQueriesDir(workingDir)
	statsDir := filepath.Join(queriesDir, constants.QueriesStatsDir)
	presetsDir := filepath.Join(queriesDir, constants.QueriesPresetsDir)

	log.Info("Generating default query files in: %s", queriesDir)

	// Create stats directory
	if err := os.MkdirAll(statsDir, constants.DirPermissions); err != nil {
		return fmt.Errorf("failed to create stats directory: %w", err)
	}
	log.Debug("Created stats directory: %s", statsDir)

	// Create presets directory
	if err := os.MkdirAll(presetsDir, constants.DirPermissions); err != nil {
		return fmt.Errorf("failed to create presets directory: %w", err)
	}
	log.Debug("Created presets directory: %s", presetsDir)

	// Generate stat files
	stats := GetDefaultStats()
	for _, stat := range stats {
		if err := generateStatFile(stat, statsDir); err != nil {
			return fmt.Errorf("failed to generate stat file %s: %w", stat.Name, err)
		}
		log.Debug("Generated stat file: %s%s", stat.Name, constants.QueryFileExt)
	}
	log.Info("Generated %d topic stat files", len(stats))

	// Generate preset files
	presets := GetDefaultPresets()
	for name, preset := range presets {
		if err := generatePresetFile(name, preset, presetsDir); err != nil {
			return fmt.Errorf("failed to generate preset file %s: %w", name, err)
		}
		log.Debug("Generated preset file: %s%s", name, constants.QueryFileExt)
	}
	log.Info("Generated %d query preset files", len(presets))

	return nil
}

// generateStatFile writes a single stat to its YAML file
func generateStatFile(stat TopicStat, dirPath string) error {
	filename := stat.Name + constants.QueryFileExt
	filePath := filepath.Join(dirPath, filename)

	data, err := yaml.Marshal(stat)
	if err != nil {
		return fmt.Errorf("failed to marshal stat: %w", err)
	}

	return os.WriteFile(filePath, data, constants.FilePermissions)
}

// generatePresetFile writes a single preset to its YAML file
func generatePresetFile(name string, preset Preset, dirPath string) error {
	filename := name + constants.QueryFileExt
	filePath := filepath.Join(dirPath, filename)

	data, err := yaml.Marshal(preset)
	if err != nil {
		return fmt.Errorf("failed to marshal preset: %w", err)
	}

	return os.WriteFile(filePath, data, constants.FilePermissions)
}

// GetQueriesDir returns the full path to the queries directory for a working directory
func GetQueriesDir(workingDir string) string {
	return filepath.Join(workingDir, constants.InternalDir, constants.QueriesDir)
}
