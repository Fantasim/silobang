package config

import (
	"fmt"
	"os"
	"path/filepath"

	"silobang/internal/constants"
)

func ValidateWorkingDirectory(path string) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return fmt.Errorf("directory does not exist")
	}
	if err != nil {
		return err
	}

	if !info.IsDir() {
		return fmt.Errorf("path is not a directory")
	}

	return nil
}

func InitializeWorkingDirectory(path string) error {
	if err := ValidateWorkingDirectory(path); err != nil {
		return err
	}

	// Create .internal/ subdirectory
	internalDir := filepath.Join(path, constants.InternalDir)
	if err := os.MkdirAll(internalDir, constants.DirPermissions); err != nil {
		return err
	}

	// Create logs directories
	logsBaseDir := filepath.Join(internalDir, constants.LogsDir)
	logSubDirs := []string{
		constants.LogsDirDebug,
		constants.LogsDirInfo,
		constants.LogsDirWarn,
		constants.LogsDirError,
	}
	for _, subDir := range logSubDirs {
		logDir := filepath.Join(logsBaseDir, subDir)
		if err := os.MkdirAll(logDir, constants.DirPermissions); err != nil {
			return fmt.Errorf("failed to create log directory %s: %w", logDir, err)
		}
	}

	// Create orchestrator.db if it doesn't exist
	orchestratorPath := filepath.Join(internalDir, constants.OrchestratorDB)
	if _, err := os.Stat(orchestratorPath); os.IsNotExist(err) {
		// Create empty file
		file, err := os.Create(orchestratorPath)
		if err != nil {
			return err
		}
		file.Close()
	}

	return nil
}

func SetWorkingDirectory(cfg *Config, path string) error {
	if err := ValidateWorkingDirectory(path); err != nil {
		return err
	}

	if err := InitializeWorkingDirectory(path); err != nil {
		return err
	}

	cfg.WorkingDirectory = path

	if err := SaveConfig(cfg); err != nil {
		return err
	}

	// Trigger topic discovery
	_, err := DiscoverTopics(path)
	return err
}
