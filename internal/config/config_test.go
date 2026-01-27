package config

import (
	"os"
	"strings"
	"testing"
	"time"

	"silobang/internal/constants"
)

// setTestHome overrides HOME so GetConfigDir/GetConfigPath use a temp directory.
// Returns the temp home directory path.
func setTestHome(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	t.Cleanup(func() { os.Setenv("HOME", originalHome) })
	return tmpDir
}

// =============================================================================
// ApplyDefaults Tests
// =============================================================================

func TestApplyDefaults_AllFieldsPopulated(t *testing.T) {
	cfg := &Config{}
	cfg.ApplyDefaults()

	// Top-level
	if cfg.Port != constants.DefaultPort {
		t.Errorf("Port: got %d, want %d", cfg.Port, constants.DefaultPort)
	}
	if cfg.MaxDatSize != constants.DefaultMaxDatSize {
		t.Errorf("MaxDatSize: got %d, want %d", cfg.MaxDatSize, constants.DefaultMaxDatSize)
	}

	// Auth
	if cfg.Auth.MaxLoginAttempts != constants.AuthMaxLoginAttempts {
		t.Errorf("Auth.MaxLoginAttempts: got %d, want %d", cfg.Auth.MaxLoginAttempts, constants.AuthMaxLoginAttempts)
	}
	if cfg.Auth.LockoutDurationMins != constants.AuthLockoutDurationMins {
		t.Errorf("Auth.LockoutDurationMins: got %d, want %d", cfg.Auth.LockoutDurationMins, constants.AuthLockoutDurationMins)
	}
	if cfg.Auth.SessionDurationHours != int(constants.AuthSessionDuration.Hours()) {
		t.Errorf("Auth.SessionDurationHours: got %d, want %d", cfg.Auth.SessionDurationHours, int(constants.AuthSessionDuration.Hours()))
	}
	if cfg.Auth.SessionMaxDurationHours != int(constants.AuthSessionMaxDuration.Hours()) {
		t.Errorf("Auth.SessionMaxDurationHours: got %d, want %d", cfg.Auth.SessionMaxDurationHours, int(constants.AuthSessionMaxDuration.Hours()))
	}

	// Bulk download
	if cfg.BulkDownload.SessionTTLMins != constants.BulkDownloadSessionTTLMins {
		t.Errorf("BulkDownload.SessionTTLMins: got %d, want %d", cfg.BulkDownload.SessionTTLMins, constants.BulkDownloadSessionTTLMins)
	}
	if cfg.BulkDownload.MaxAssets != constants.BulkDownloadMaxAssets {
		t.Errorf("BulkDownload.MaxAssets: got %d, want %d", cfg.BulkDownload.MaxAssets, constants.BulkDownloadMaxAssets)
	}

	// Audit
	if cfg.Audit.MaxLogSizeBytes != constants.AuditMaxLogSizeBytes {
		t.Errorf("Audit.MaxLogSizeBytes: got %d, want %d", cfg.Audit.MaxLogSizeBytes, constants.AuditMaxLogSizeBytes)
	}
	if cfg.Audit.PurgePercentage != constants.AuditPurgePercentage {
		t.Errorf("Audit.PurgePercentage: got %d, want %d", cfg.Audit.PurgePercentage, constants.AuditPurgePercentage)
	}

	// Metadata
	if cfg.Metadata.MaxValueBytes != constants.MaxMetadataValueBytes {
		t.Errorf("Metadata.MaxValueBytes: got %d, want %d", cfg.Metadata.MaxValueBytes, constants.MaxMetadataValueBytes)
	}

	// Batch
	if cfg.Batch.MaxOperations != constants.BatchMetadataMaxOperations {
		t.Errorf("Batch.MaxOperations: got %d, want %d", cfg.Batch.MaxOperations, constants.BatchMetadataMaxOperations)
	}

	// Monitoring
	if cfg.Monitoring.LogFileMaxReadBytes != constants.MonitoringLogFileMaxReadBytes {
		t.Errorf("Monitoring.LogFileMaxReadBytes: got %d, want %d", cfg.Monitoring.LogFileMaxReadBytes, constants.MonitoringLogFileMaxReadBytes)
	}
}

func TestApplyDefaults_PreservesCustomValues(t *testing.T) {
	cfg := &Config{
		Port:       9999,
		MaxDatSize: 512,
		Auth: AuthConfig{
			MaxLoginAttempts: 10,
		},
		BulkDownload: BulkDownloadConfig{
			MaxAssets: 42,
		},
		Audit: AuditConfig{
			PurgePercentage: 50,
		},
	}
	cfg.ApplyDefaults()

	// Custom values must be preserved
	if cfg.Port != 9999 {
		t.Errorf("Port should be preserved: got %d, want 9999", cfg.Port)
	}
	if cfg.MaxDatSize != 512 {
		t.Errorf("MaxDatSize should be preserved: got %d, want 512", cfg.MaxDatSize)
	}
	if cfg.Auth.MaxLoginAttempts != 10 {
		t.Errorf("Auth.MaxLoginAttempts should be preserved: got %d, want 10", cfg.Auth.MaxLoginAttempts)
	}
	if cfg.BulkDownload.MaxAssets != 42 {
		t.Errorf("BulkDownload.MaxAssets should be preserved: got %d, want 42", cfg.BulkDownload.MaxAssets)
	}
	if cfg.Audit.PurgePercentage != 50 {
		t.Errorf("Audit.PurgePercentage should be preserved: got %d, want 50", cfg.Audit.PurgePercentage)
	}

	// Non-set fields should get defaults
	if cfg.Auth.LockoutDurationMins != constants.AuthLockoutDurationMins {
		t.Errorf("Auth.LockoutDurationMins should get default: got %d, want %d", cfg.Auth.LockoutDurationMins, constants.AuthLockoutDurationMins)
	}
	if cfg.BulkDownload.SessionTTLMins != constants.BulkDownloadSessionTTLMins {
		t.Errorf("BulkDownload.SessionTTLMins should get default: got %d, want %d", cfg.BulkDownload.SessionTTLMins, constants.BulkDownloadSessionTTLMins)
	}
	if cfg.Audit.MaxLogSizeBytes != constants.AuditMaxLogSizeBytes {
		t.Errorf("Audit.MaxLogSizeBytes should get default: got %d, want %d", cfg.Audit.MaxLogSizeBytes, constants.AuditMaxLogSizeBytes)
	}
}

// =============================================================================
// Validate Tests
// =============================================================================

func TestValidate_DefaultConfig(t *testing.T) {
	cfg := &Config{}
	cfg.ApplyDefaults()

	if err := cfg.validate(); err != nil {
		t.Errorf("default config should be valid, got: %v", err)
	}
}

func TestValidate_InvalidAuth(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*Config)
		want  string
	}{
		{
			"MaxLoginAttempts_zero",
			func(c *Config) { c.Auth.MaxLoginAttempts = 0 },
			"max_login_attempts must be >= 1",
		},
		{
			"LockoutDurationMins_zero",
			func(c *Config) { c.Auth.LockoutDurationMins = 0 },
			"lockout_duration_mins must be >= 1",
		},
		{
			"SessionDurationHours_zero",
			func(c *Config) { c.Auth.SessionDurationHours = 0 },
			"session_duration_hours must be >= 1",
		},
		{
			"SessionMaxDuration_less_than_SessionDuration",
			func(c *Config) {
				c.Auth.SessionDurationHours = 24
				c.Auth.SessionMaxDurationHours = 12
			},
			"session_max_duration_hours must be >= auth.session_duration_hours",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{}
			cfg.ApplyDefaults()
			tt.setup(cfg)

			err := cfg.validate()
			if err == nil {
				t.Fatal("expected validation error, got nil")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error should contain %q, got: %v", tt.want, err)
			}
		})
	}
}

func TestValidate_InvalidBulkDownload(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*Config)
		want  string
	}{
		{
			"SessionTTLMins_zero",
			func(c *Config) { c.BulkDownload.SessionTTLMins = 0 },
			"session_ttl_mins must be >= 1",
		},
		{
			"MaxAssets_zero",
			func(c *Config) { c.BulkDownload.MaxAssets = 0 },
			"max_assets must be >= 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{}
			cfg.ApplyDefaults()
			tt.setup(cfg)

			err := cfg.validate()
			if err == nil {
				t.Fatal("expected validation error, got nil")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error should contain %q, got: %v", tt.want, err)
			}
		})
	}
}

func TestValidate_InvalidAudit(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*Config)
		want  string
	}{
		{
			"MaxLogSizeBytes_too_small",
			func(c *Config) { c.Audit.MaxLogSizeBytes = 1000 },
			"max_log_size_bytes must be >= 1048576",
		},
		{
			"PurgePercentage_zero",
			func(c *Config) { c.Audit.PurgePercentage = 0 },
			"purge_percentage must be between 1 and 100",
		},
		{
			"PurgePercentage_over_100",
			func(c *Config) { c.Audit.PurgePercentage = 101 },
			"purge_percentage must be between 1 and 100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{}
			cfg.ApplyDefaults()
			tt.setup(cfg)

			err := cfg.validate()
			if err == nil {
				t.Fatal("expected validation error, got nil")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error should contain %q, got: %v", tt.want, err)
			}
		})
	}
}

func TestValidate_InvalidMetadata(t *testing.T) {
	cfg := &Config{}
	cfg.ApplyDefaults()
	cfg.Metadata.MaxValueBytes = 0

	err := cfg.validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "max_value_bytes must be >= 1") {
		t.Errorf("expected max_value_bytes error, got: %v", err)
	}
}

func TestValidate_InvalidBatch(t *testing.T) {
	cfg := &Config{}
	cfg.ApplyDefaults()
	cfg.Batch.MaxOperations = 0

	err := cfg.validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "max_operations must be >= 1") {
		t.Errorf("expected max_operations error, got: %v", err)
	}
}

func TestValidate_InvalidMonitoring(t *testing.T) {
	cfg := &Config{}
	cfg.ApplyDefaults()
	cfg.Monitoring.LogFileMaxReadBytes = 512

	err := cfg.validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "log_file_max_read_bytes must be >= 1024") {
		t.Errorf("expected log_file_max_read_bytes error, got: %v", err)
	}
}

func TestValidate_InvalidDiskUsage(t *testing.T) {
	tests := []struct {
		name  string
		value int64
		valid bool
	}{
		{"zero_unlimited", 0, true},
		{"valid_1gb", constants.MinMaxDiskUsageBytes, true},
		{"valid_10gb", 10 * constants.MinMaxDiskUsageBytes, true},
		{"too_small_1byte", 1, false},
		{"too_small_500mb", 500 * 1024 * 1024, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{}
			cfg.ApplyDefaults()
			cfg.MaxDiskUsage = tt.value

			err := cfg.validate()
			if tt.valid && err != nil {
				t.Errorf("expected valid config, got error: %v", err)
			}
			if !tt.valid && err == nil {
				t.Error("expected validation error, got nil")
			}
			if !tt.valid && err != nil && !strings.Contains(err.Error(), "max_disk_usage") {
				t.Errorf("expected max_disk_usage error, got: %v", err)
			}
		})
	}
}

func TestApplyDefaults_MaxDiskUsage_ZeroByDefault(t *testing.T) {
	cfg := &Config{}
	cfg.ApplyDefaults()

	if cfg.MaxDiskUsage != constants.DefaultMaxDiskUsageBytes {
		t.Errorf("MaxDiskUsage: got %d, want %d", cfg.MaxDiskUsage, constants.DefaultMaxDiskUsageBytes)
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	cfg := &Config{}
	cfg.ApplyDefaults()
	cfg.Auth.MaxLoginAttempts = 0
	cfg.Audit.PurgePercentage = 200
	cfg.Batch.MaxOperations = 0

	err := cfg.validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "max_login_attempts") {
		t.Error("missing max_login_attempts error")
	}
	if !strings.Contains(errStr, "purge_percentage") {
		t.Error("missing purge_percentage error")
	}
	if !strings.Contains(errStr, "max_operations") {
		t.Error("missing max_operations error")
	}
}

// =============================================================================
// Duration Helper Tests
// =============================================================================

func TestAuthConfig_SessionDuration(t *testing.T) {
	ac := AuthConfig{SessionDurationHours: 12}
	got := ac.SessionDuration()
	want := 12 * time.Hour
	if got != want {
		t.Errorf("SessionDuration: got %v, want %v", got, want)
	}
}

func TestAuthConfig_SessionMaxDuration(t *testing.T) {
	ac := AuthConfig{SessionMaxDurationHours: 72}
	got := ac.SessionMaxDuration()
	want := 72 * time.Hour
	if got != want {
		t.Errorf("SessionMaxDuration: got %v, want %v", got, want)
	}
}

func TestAuthConfig_DurationHelpers_WithDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.ApplyDefaults()

	if cfg.Auth.SessionDuration() != constants.AuthSessionDuration {
		t.Errorf("SessionDuration: got %v, want %v", cfg.Auth.SessionDuration(), constants.AuthSessionDuration)
	}
	if cfg.Auth.SessionMaxDuration() != constants.AuthSessionMaxDuration {
		t.Errorf("SessionMaxDuration: got %v, want %v", cfg.Auth.SessionMaxDuration(), constants.AuthSessionMaxDuration)
	}
}

// =============================================================================
// LoadConfig Tests
// =============================================================================

func TestLoadConfig_CreatesDefaultIfMissing(t *testing.T) {
	setTestHome(t)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Port != constants.DefaultPort {
		t.Errorf("Port: got %d, want %d", cfg.Port, constants.DefaultPort)
	}
	if cfg.Auth.MaxLoginAttempts != constants.AuthMaxLoginAttempts {
		t.Errorf("Auth.MaxLoginAttempts: got %d, want %d", cfg.Auth.MaxLoginAttempts, constants.AuthMaxLoginAttempts)
	}

	// Config file should have been created
	if _, err := os.Stat(GetConfigPath()); os.IsNotExist(err) {
		t.Error("config file was not created")
	}
}

func TestLoadConfig_BackwardCompatibility(t *testing.T) {
	setTestHome(t)
	EnsureConfigDir()

	// Write an old-format config with only working_directory
	oldYAML := "working_directory: /tmp/test\n"
	os.WriteFile(GetConfigPath(), []byte(oldYAML), 0644)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.WorkingDirectory != "/tmp/test" {
		t.Errorf("WorkingDirectory: got %q, want /tmp/test", cfg.WorkingDirectory)
	}
	// All other fields should be defaults
	if cfg.Port != constants.DefaultPort {
		t.Errorf("Port should default: got %d, want %d", cfg.Port, constants.DefaultPort)
	}
	if cfg.Auth.MaxLoginAttempts != constants.AuthMaxLoginAttempts {
		t.Errorf("Auth.MaxLoginAttempts should default: got %d, want %d",
			cfg.Auth.MaxLoginAttempts, constants.AuthMaxLoginAttempts)
	}
	if cfg.Batch.MaxOperations != constants.BatchMetadataMaxOperations {
		t.Errorf("Batch.MaxOperations should default: got %d, want %d",
			cfg.Batch.MaxOperations, constants.BatchMetadataMaxOperations)
	}
}

func TestLoadConfig_CustomValues(t *testing.T) {
	setTestHome(t)
	EnsureConfigDir()

	customYAML := `working_directory: /data
port: 8080
max_dat_size: 2147483648
auth:
  max_login_attempts: 10
  lockout_duration_mins: 60
  session_duration_hours: 48
  session_max_duration_hours: 720
bulk_download:
  session_ttl_mins: 240
  max_assets: 100000
audit:
  max_log_size_bytes: 21474836480
  purge_percentage: 20
metadata:
  max_value_bytes: 20971520
batch:
  max_operations: 200000
monitoring:
  log_file_max_read_bytes: 10485760
`
	os.WriteFile(GetConfigPath(), []byte(customYAML), 0644)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Port != 8080 {
		t.Errorf("Port: got %d, want 8080", cfg.Port)
	}
	if cfg.MaxDatSize != 2147483648 {
		t.Errorf("MaxDatSize: got %d, want 2147483648", cfg.MaxDatSize)
	}
	if cfg.Auth.MaxLoginAttempts != 10 {
		t.Errorf("Auth.MaxLoginAttempts: got %d, want 10", cfg.Auth.MaxLoginAttempts)
	}
	if cfg.Auth.LockoutDurationMins != 60 {
		t.Errorf("Auth.LockoutDurationMins: got %d, want 60", cfg.Auth.LockoutDurationMins)
	}
	if cfg.Auth.SessionDurationHours != 48 {
		t.Errorf("Auth.SessionDurationHours: got %d, want 48", cfg.Auth.SessionDurationHours)
	}
	if cfg.Auth.SessionMaxDurationHours != 720 {
		t.Errorf("Auth.SessionMaxDurationHours: got %d, want 720", cfg.Auth.SessionMaxDurationHours)
	}
	if cfg.BulkDownload.SessionTTLMins != 240 {
		t.Errorf("BulkDownload.SessionTTLMins: got %d, want 240", cfg.BulkDownload.SessionTTLMins)
	}
	if cfg.BulkDownload.MaxAssets != 100000 {
		t.Errorf("BulkDownload.MaxAssets: got %d, want 100000", cfg.BulkDownload.MaxAssets)
	}
	if cfg.Audit.MaxLogSizeBytes != 21474836480 {
		t.Errorf("Audit.MaxLogSizeBytes: got %d, want 21474836480", cfg.Audit.MaxLogSizeBytes)
	}
	if cfg.Audit.PurgePercentage != 20 {
		t.Errorf("Audit.PurgePercentage: got %d, want 20", cfg.Audit.PurgePercentage)
	}
	if cfg.Metadata.MaxValueBytes != 20971520 {
		t.Errorf("Metadata.MaxValueBytes: got %d, want 20971520", cfg.Metadata.MaxValueBytes)
	}
	if cfg.Batch.MaxOperations != 200000 {
		t.Errorf("Batch.MaxOperations: got %d, want 200000", cfg.Batch.MaxOperations)
	}
	if cfg.Monitoring.LogFileMaxReadBytes != 10485760 {
		t.Errorf("Monitoring.LogFileMaxReadBytes: got %d, want 10485760", cfg.Monitoring.LogFileMaxReadBytes)
	}
}

func TestLoadConfig_PartialOverride(t *testing.T) {
	setTestHome(t)
	EnsureConfigDir()

	// Only override auth and audit — everything else should get defaults
	partialYAML := `working_directory: /data
auth:
  max_login_attempts: 7
audit:
  purge_percentage: 15
`
	os.WriteFile(GetConfigPath(), []byte(partialYAML), 0644)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Custom values
	if cfg.Auth.MaxLoginAttempts != 7 {
		t.Errorf("Auth.MaxLoginAttempts: got %d, want 7", cfg.Auth.MaxLoginAttempts)
	}
	if cfg.Audit.PurgePercentage != 15 {
		t.Errorf("Audit.PurgePercentage: got %d, want 15", cfg.Audit.PurgePercentage)
	}

	// Default values
	if cfg.Port != constants.DefaultPort {
		t.Errorf("Port should default: got %d, want %d", cfg.Port, constants.DefaultPort)
	}
	if cfg.Auth.LockoutDurationMins != constants.AuthLockoutDurationMins {
		t.Errorf("Auth.LockoutDurationMins should default: got %d, want %d",
			cfg.Auth.LockoutDurationMins, constants.AuthLockoutDurationMins)
	}
	if cfg.BulkDownload.MaxAssets != constants.BulkDownloadMaxAssets {
		t.Errorf("BulkDownload.MaxAssets should default: got %d, want %d",
			cfg.BulkDownload.MaxAssets, constants.BulkDownloadMaxAssets)
	}
	if cfg.Batch.MaxOperations != constants.BatchMetadataMaxOperations {
		t.Errorf("Batch.MaxOperations should default: got %d, want %d",
			cfg.Batch.MaxOperations, constants.BatchMetadataMaxOperations)
	}
	if cfg.Monitoring.LogFileMaxReadBytes != constants.MonitoringLogFileMaxReadBytes {
		t.Errorf("Monitoring.LogFileMaxReadBytes should default: got %d, want %d",
			cfg.Monitoring.LogFileMaxReadBytes, constants.MonitoringLogFileMaxReadBytes)
	}
}

func TestLoadConfig_InvalidValues_Rejected(t *testing.T) {
	setTestHome(t)
	EnsureConfigDir()

	invalidYAML := `working_directory: /data
auth:
  max_login_attempts: -1
`
	os.WriteFile(GetConfigPath(), []byte(invalidYAML), 0644)

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected validation error for invalid auth config")
	}
	if !strings.Contains(err.Error(), "max_login_attempts") {
		t.Errorf("expected max_login_attempts error, got: %v", err)
	}
}

// =============================================================================
// SaveConfig Tests
// =============================================================================

func TestSaveConfig_WritesAllValues(t *testing.T) {
	setTestHome(t)

	cfg := &Config{
		WorkingDirectory: "/data/test",
	}
	cfg.ApplyDefaults()

	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	data, err := os.ReadFile(GetConfigPath())
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	content := string(data)

	// All config sections and fields should be present (self-documenting YAML)
	requiredFields := []string{
		"working_directory",
		"port:",
		"max_dat_size:",
		"max_login_attempts:",
		"lockout_duration_mins:",
		"session_duration_hours:",
		"session_max_duration_hours:",
		"session_ttl_mins:",
		"max_assets:",
		"max_log_size_bytes:",
		"purge_percentage:",
		"max_value_bytes:",
		"max_operations:",
		"log_file_max_read_bytes:",
	}
	for _, field := range requiredFields {
		if !strings.Contains(content, field) {
			t.Errorf("saved config should contain %q", field)
		}
	}

	// All section headers should be present
	requiredSections := []string{
		"auth:",
		"bulk_download:",
		"audit:",
		"metadata:",
		"batch:",
		"monitoring:",
	}
	for _, section := range requiredSections {
		if !strings.Contains(content, section) {
			t.Errorf("saved config should contain section %q", section)
		}
	}
}

func TestSaveConfig_PersistsNonDefaults(t *testing.T) {
	setTestHome(t)

	cfg := &Config{
		WorkingDirectory: "/data/test",
		Port:             8080,
	}
	cfg.ApplyDefaults()
	cfg.Auth.MaxLoginAttempts = 10
	cfg.Audit.PurgePercentage = 25
	cfg.Batch.MaxOperations = 5000

	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	data, err := os.ReadFile(GetConfigPath())
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	content := string(data)

	if !strings.Contains(content, "port:") {
		t.Error("non-default port should be persisted")
	}
	if !strings.Contains(content, "max_login_attempts") {
		t.Error("non-default max_login_attempts should be persisted")
	}
	if !strings.Contains(content, "purge_percentage") {
		t.Error("non-default purge_percentage should be persisted")
	}
	if !strings.Contains(content, "max_operations") {
		t.Error("non-default max_operations should be persisted")
	}
}

func TestSaveConfig_RoundTrip(t *testing.T) {
	setTestHome(t)

	original := &Config{
		WorkingDirectory: "/data/roundtrip",
		Port:             4000,
		MaxDatSize:       500000000,
		Auth: AuthConfig{
			MaxLoginAttempts:        8,
			LockoutDurationMins:     45,
			SessionDurationHours:    36,
			SessionMaxDurationHours: 360,
		},
		BulkDownload: BulkDownloadConfig{
			SessionTTLMins: 180,
			MaxAssets:      50000,
		},
		Audit: AuditConfig{
			MaxLogSizeBytes: 5368709120,
			PurgePercentage: 15,
		},
		Metadata: MetadataConfig{
			MaxValueBytes: 20971520,
		},
		Batch: BatchConfig{
			MaxOperations: 75000,
		},
		Monitoring: MonitoringConfig{
			LogFileMaxReadBytes: 8388608,
		},
	}

	if err := SaveConfig(original); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	loaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify all custom values survived the round-trip
	if loaded.WorkingDirectory != original.WorkingDirectory {
		t.Errorf("WorkingDirectory: got %q, want %q", loaded.WorkingDirectory, original.WorkingDirectory)
	}
	if loaded.Port != original.Port {
		t.Errorf("Port: got %d, want %d", loaded.Port, original.Port)
	}
	if loaded.MaxDatSize != original.MaxDatSize {
		t.Errorf("MaxDatSize: got %d, want %d", loaded.MaxDatSize, original.MaxDatSize)
	}
	if loaded.Auth.MaxLoginAttempts != original.Auth.MaxLoginAttempts {
		t.Errorf("Auth.MaxLoginAttempts: got %d, want %d", loaded.Auth.MaxLoginAttempts, original.Auth.MaxLoginAttempts)
	}
	if loaded.Auth.LockoutDurationMins != original.Auth.LockoutDurationMins {
		t.Errorf("Auth.LockoutDurationMins: got %d, want %d", loaded.Auth.LockoutDurationMins, original.Auth.LockoutDurationMins)
	}
	if loaded.Auth.SessionDurationHours != original.Auth.SessionDurationHours {
		t.Errorf("Auth.SessionDurationHours: got %d, want %d", loaded.Auth.SessionDurationHours, original.Auth.SessionDurationHours)
	}
	if loaded.Auth.SessionMaxDurationHours != original.Auth.SessionMaxDurationHours {
		t.Errorf("Auth.SessionMaxDurationHours: got %d, want %d", loaded.Auth.SessionMaxDurationHours, original.Auth.SessionMaxDurationHours)
	}
	if loaded.BulkDownload.SessionTTLMins != original.BulkDownload.SessionTTLMins {
		t.Errorf("BulkDownload.SessionTTLMins: got %d, want %d", loaded.BulkDownload.SessionTTLMins, original.BulkDownload.SessionTTLMins)
	}
	if loaded.BulkDownload.MaxAssets != original.BulkDownload.MaxAssets {
		t.Errorf("BulkDownload.MaxAssets: got %d, want %d", loaded.BulkDownload.MaxAssets, original.BulkDownload.MaxAssets)
	}
	if loaded.Audit.MaxLogSizeBytes != original.Audit.MaxLogSizeBytes {
		t.Errorf("Audit.MaxLogSizeBytes: got %d, want %d", loaded.Audit.MaxLogSizeBytes, original.Audit.MaxLogSizeBytes)
	}
	if loaded.Audit.PurgePercentage != original.Audit.PurgePercentage {
		t.Errorf("Audit.PurgePercentage: got %d, want %d", loaded.Audit.PurgePercentage, original.Audit.PurgePercentage)
	}
	if loaded.Metadata.MaxValueBytes != original.Metadata.MaxValueBytes {
		t.Errorf("Metadata.MaxValueBytes: got %d, want %d", loaded.Metadata.MaxValueBytes, original.Metadata.MaxValueBytes)
	}
	if loaded.Batch.MaxOperations != original.Batch.MaxOperations {
		t.Errorf("Batch.MaxOperations: got %d, want %d", loaded.Batch.MaxOperations, original.Batch.MaxOperations)
	}
	if loaded.Monitoring.LogFileMaxReadBytes != original.Monitoring.LogFileMaxReadBytes {
		t.Errorf("Monitoring.LogFileMaxReadBytes: got %d, want %d", loaded.Monitoring.LogFileMaxReadBytes, original.Monitoring.LogFileMaxReadBytes)
	}
}
