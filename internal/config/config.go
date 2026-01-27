package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"silobang/internal/constants"
	"silobang/internal/logger"
)

// AuthConfig holds user-configurable authentication settings.
type AuthConfig struct {
	MaxLoginAttempts        int `yaml:"max_login_attempts"`
	LockoutDurationMins     int `yaml:"lockout_duration_mins"`
	SessionDurationHours    int `yaml:"session_duration_hours"`
	SessionMaxDurationHours int `yaml:"session_max_duration_hours"`
}

// SessionDuration returns the session duration as time.Duration.
func (c *AuthConfig) SessionDuration() time.Duration {
	return time.Duration(c.SessionDurationHours) * time.Hour
}

// SessionMaxDuration returns the maximum session duration as time.Duration.
func (c *AuthConfig) SessionMaxDuration() time.Duration {
	return time.Duration(c.SessionMaxDurationHours) * time.Hour
}

// BulkDownloadConfig holds user-configurable bulk download settings.
type BulkDownloadConfig struct {
	SessionTTLMins int `yaml:"session_ttl_mins"`
	MaxAssets      int `yaml:"max_assets"`
}

// AuditConfig holds user-configurable audit log settings.
type AuditConfig struct {
	MaxLogSizeBytes int64 `yaml:"max_log_size_bytes"`
	PurgePercentage int   `yaml:"purge_percentage"`
}

// MetadataConfig holds user-configurable metadata settings.
type MetadataConfig struct {
	MaxValueBytes int `yaml:"max_value_bytes"`
}

// BatchConfig holds user-configurable batch operation settings.
type BatchConfig struct {
	MaxOperations int `yaml:"max_operations"`
}

// MonitoringConfig holds user-configurable monitoring settings.
type MonitoringConfig struct {
	LogFileMaxReadBytes int64 `yaml:"log_file_max_read_bytes"`
}

// Config holds all application configuration.
type Config struct {
	WorkingDirectory string             `yaml:"working_directory"`
	Port             int                `yaml:"port"`
	MaxDatSize       int64              `yaml:"max_dat_size"`
	Auth             AuthConfig         `yaml:"auth"`
	BulkDownload     BulkDownloadConfig `yaml:"bulk_download"`
	Audit            AuditConfig        `yaml:"audit"`
	Metadata         MetadataConfig     `yaml:"metadata"`
	Batch            BatchConfig        `yaml:"batch"`
	Monitoring       MonitoringConfig   `yaml:"monitoring"`
}

// ApplyDefaults fills zero-valued fields with constant defaults.
func (cfg *Config) ApplyDefaults() {
	if cfg.Port == 0 {
		cfg.Port = constants.DefaultPort
	}
	if cfg.MaxDatSize == 0 {
		cfg.MaxDatSize = constants.DefaultMaxDatSize
	}

	// Auth defaults
	if cfg.Auth.MaxLoginAttempts == 0 {
		cfg.Auth.MaxLoginAttempts = constants.AuthMaxLoginAttempts
	}
	if cfg.Auth.LockoutDurationMins == 0 {
		cfg.Auth.LockoutDurationMins = constants.AuthLockoutDurationMins
	}
	if cfg.Auth.SessionDurationHours == 0 {
		cfg.Auth.SessionDurationHours = int(constants.AuthSessionDuration.Hours())
	}
	if cfg.Auth.SessionMaxDurationHours == 0 {
		cfg.Auth.SessionMaxDurationHours = int(constants.AuthSessionMaxDuration.Hours())
	}

	// Bulk download defaults
	if cfg.BulkDownload.SessionTTLMins == 0 {
		cfg.BulkDownload.SessionTTLMins = constants.BulkDownloadSessionTTLMins
	}
	if cfg.BulkDownload.MaxAssets == 0 {
		cfg.BulkDownload.MaxAssets = constants.BulkDownloadMaxAssets
	}

	// Audit defaults
	if cfg.Audit.MaxLogSizeBytes == 0 {
		cfg.Audit.MaxLogSizeBytes = constants.AuditMaxLogSizeBytes
	}
	if cfg.Audit.PurgePercentage == 0 {
		cfg.Audit.PurgePercentage = constants.AuditPurgePercentage
	}

	// Metadata defaults
	if cfg.Metadata.MaxValueBytes == 0 {
		cfg.Metadata.MaxValueBytes = constants.MaxMetadataValueBytes
	}

	// Batch defaults
	if cfg.Batch.MaxOperations == 0 {
		cfg.Batch.MaxOperations = constants.BatchMetadataMaxOperations
	}

	// Monitoring defaults
	if cfg.Monitoring.LogFileMaxReadBytes == 0 {
		cfg.Monitoring.LogFileMaxReadBytes = constants.MonitoringLogFileMaxReadBytes
	}
}

// validate checks that all configurable values are within acceptable ranges.
func (cfg *Config) validate() error {
	var errs []string

	// Auth validation
	if cfg.Auth.MaxLoginAttempts < 1 {
		errs = append(errs, "auth.max_login_attempts must be >= 1")
	}
	if cfg.Auth.LockoutDurationMins < 1 {
		errs = append(errs, "auth.lockout_duration_mins must be >= 1")
	}
	if cfg.Auth.SessionDurationHours < 1 {
		errs = append(errs, "auth.session_duration_hours must be >= 1")
	}
	if cfg.Auth.SessionMaxDurationHours < cfg.Auth.SessionDurationHours {
		errs = append(errs, "auth.session_max_duration_hours must be >= auth.session_duration_hours")
	}

	// Bulk download validation
	if cfg.BulkDownload.SessionTTLMins < 1 {
		errs = append(errs, "bulk_download.session_ttl_mins must be >= 1")
	}
	if cfg.BulkDownload.MaxAssets < 1 {
		errs = append(errs, "bulk_download.max_assets must be >= 1")
	}

	// Audit validation
	if cfg.Audit.MaxLogSizeBytes < 1048576 {
		errs = append(errs, "audit.max_log_size_bytes must be >= 1048576 (1MB)")
	}
	if cfg.Audit.PurgePercentage < 1 || cfg.Audit.PurgePercentage > 100 {
		errs = append(errs, "audit.purge_percentage must be between 1 and 100")
	}

	// Metadata validation
	if cfg.Metadata.MaxValueBytes < 1 {
		errs = append(errs, "metadata.max_value_bytes must be >= 1")
	}

	// Batch validation
	if cfg.Batch.MaxOperations < 1 {
		errs = append(errs, "batch.max_operations must be >= 1")
	}

	// Monitoring validation
	if cfg.Monitoring.LogFileMaxReadBytes < 1024 {
		errs = append(errs, "monitoring.log_file_max_read_bytes must be >= 1024 (1KB)")
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

// LogEffectiveValues logs all effective configuration values at startup.
func (cfg *Config) LogEffectiveValues(log *logger.Logger) {
	log.Info("config: port=%d", cfg.Port)
	log.Info("config: max_dat_size=%d", cfg.MaxDatSize)
	log.Info("config: auth.max_login_attempts=%d", cfg.Auth.MaxLoginAttempts)
	log.Info("config: auth.lockout_duration_mins=%d", cfg.Auth.LockoutDurationMins)
	log.Info("config: auth.session_duration_hours=%d", cfg.Auth.SessionDurationHours)
	log.Info("config: auth.session_max_duration_hours=%d", cfg.Auth.SessionMaxDurationHours)
	log.Info("config: bulk_download.session_ttl_mins=%d", cfg.BulkDownload.SessionTTLMins)
	log.Info("config: bulk_download.max_assets=%d", cfg.BulkDownload.MaxAssets)
	log.Info("config: audit.max_log_size_bytes=%d", cfg.Audit.MaxLogSizeBytes)
	log.Info("config: audit.purge_percentage=%d", cfg.Audit.PurgePercentage)
	log.Info("config: metadata.max_value_bytes=%d", cfg.Metadata.MaxValueBytes)
	log.Info("config: batch.max_operations=%d", cfg.Batch.MaxOperations)
	log.Info("config: monitoring.log_file_max_read_bytes=%d", cfg.Monitoring.LogFileMaxReadBytes)
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
		cfg := &Config{}
		cfg.ApplyDefaults()

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
	cfg.ApplyDefaults()

	// Validate all values
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func SaveConfig(cfg *Config) error {
	if err := EnsureConfigDir(); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	configPath := GetConfigPath()
	return os.WriteFile(configPath, data, constants.FilePermissions)
}
