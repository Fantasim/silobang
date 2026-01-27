package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
	"meshbank/internal/constants"
)

type Config struct {
	WorkingDirectory string `yaml:"working_directory"`
	Port             int    `yaml:"port,omitempty"`
	MaxDatSize       int64  `yaml:"max_dat_size,omitempty"`
}

func GetConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, constants.ConfigDir)
}

func GetConfigPath() string {
	return filepath.Join(GetConfigDir(), constants.ConfigFile)
}

func EnsureConfigDir() error {
	configDir := GetConfigDir()
	return os.MkdirAll(configDir, constants.DirPermissions)
}

func LoadConfig() (*Config, error) {
	if err := EnsureConfigDir(); err != nil {
		return nil, err
	}

	configPath := GetConfigPath()

	// Check if config file exists
	_, err := os.Stat(configPath)
	if os.IsNotExist(err) {
		// Create new config with defaults
		cfg := &Config{
			WorkingDirectory: "",
			Port:             constants.DefaultPort,
			MaxDatSize:       constants.DefaultMaxDatSize,
		}

		// Save initial config
		if err := SaveConfig(cfg); err != nil {
			return nil, err
		}

		return cfg, nil
	}

	// Read existing config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Apply defaults for missing fields
	if cfg.Port == 0 {
		cfg.Port = constants.DefaultPort
	}
	if cfg.MaxDatSize == 0 {
		cfg.MaxDatSize = constants.DefaultMaxDatSize
	}

	return cfg, nil
}

func SaveConfig(cfg *Config) error {
	if err := EnsureConfigDir(); err != nil {
		return err
	}

	// Create a copy for saving, omitting default values
	saveCfg := struct {
		WorkingDirectory string `yaml:"working_directory"`
		Port             int    `yaml:"port,omitempty"`
		MaxDatSize       int64  `yaml:"max_dat_size,omitempty"`
	}{
		WorkingDirectory: cfg.WorkingDirectory,
	}

	// Only include non-default values
	if cfg.Port != constants.DefaultPort {
		saveCfg.Port = cfg.Port
	}
	if cfg.MaxDatSize != constants.DefaultMaxDatSize {
		saveCfg.MaxDatSize = cfg.MaxDatSize
	}

	data, err := yaml.Marshal(saveCfg)
	if err != nil {
		return err
	}

	configPath := GetConfigPath()
	return os.WriteFile(configPath, data, constants.FilePermissions)
}
