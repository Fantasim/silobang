package services

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"silobang/internal/constants"
	"silobang/internal/logger"
	"silobang/internal/sanitize"
)

// MonitoringService provides system health metrics and log file access.
type MonitoringService struct {
	app    AppState
	logger *logger.Logger
	statsCache *StatsCache
}

// NewMonitoringService creates a new monitoring service instance.
func NewMonitoringService(app AppState, log *logger.Logger) *MonitoringService {
	return &MonitoringService{
		app:    app,
		logger: log,
	}
}

// SetStatsCache sets the stats cache reference for monitoring.
// Called after StatsCache is initialized in the services container.
func (s *MonitoringService) SetStatsCache(cache *StatsCache) {
	s.statsCache = cache
}

// =============================================================================
// Response Types
// =============================================================================

// MonitoringInfo is the full response for GET /api/monitoring.
type MonitoringInfo struct {
	System      SystemInfo      `json:"system"`
	Application ApplicationInfo `json:"application"`
	Logs        LogsSummary     `json:"logs"`
	Service     *ServiceInfoSnapshot `json:"service,omitempty"`
}

// SystemInfo holds OS-level resource metrics.
type SystemInfo struct {
	RAMUsedBytes        uint64 `json:"ram_used_bytes"`
	RAMTotalBytes       uint64 `json:"ram_total_bytes"`
	ProjectDirSizeBytes uint64 `json:"project_dir_size_bytes"`
}

// ApplicationInfo holds application-level configuration and state metrics.
type ApplicationInfo struct {
	UptimeSeconds         int64  `json:"uptime_seconds"`
	StartedAt             int64  `json:"started_at"`
	WorkingDirectory      string `json:"working_directory"`
	Port                  int    `json:"port"`
	MaxDatSizeBytes       int64  `json:"max_dat_size_bytes"`
	MaxMetadataValueBytes int    `json:"max_metadata_value_bytes"`
	MaxDiskUsageBytes     int64  `json:"max_disk_usage_bytes"`
	TopicsTotal           int    `json:"topics_total"`
	TopicsHealthy         int    `json:"topics_healthy"`
	TopicsUnhealthy       int    `json:"topics_unhealthy"`
	TotalIndexedHashes    int64  `json:"total_indexed_hashes"`
}

// LogsSummary holds log file summaries per level.
type LogsSummary struct {
	Levels []LogLevelSummary `json:"levels"`
}

// LogLevelSummary holds the summary for a single log level directory.
type LogLevelSummary struct {
	Level     string        `json:"level"`
	FileCount int           `json:"file_count"`
	TotalSize int64         `json:"total_size"`
	Files     []LogFileInfo `json:"files,omitempty"`
}

// LogFileInfo holds metadata about a single log file.
type LogFileInfo struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"mod_time"`
}

// =============================================================================
// Allowed log levels for API access
// =============================================================================

// monitoringAllowedLogLevels defines which log level directories are accessible
// via the monitoring API. Debug and info logs are excluded to limit information
// exposure — only warn and error are operationally relevant for monitoring.
var monitoringAllowedLogLevels = []string{
	constants.LogsDirWarn,
	constants.LogsDirError,
}

// allLogLevels is the full set of log level directories for summary stats.
var allLogLevels = []string{
	constants.LogsDirDebug,
	constants.LogsDirInfo,
	constants.LogsDirWarn,
	constants.LogsDirError,
}

// =============================================================================
// Service Methods
// =============================================================================

// GetMonitoringInfo collects and returns all system monitoring metrics.
func (s *MonitoringService) GetMonitoringInfo() (*MonitoringInfo, error) {
	workDir := s.app.GetWorkingDirectory()
	if workDir == "" {
		return nil, ErrNotConfigured
	}

	s.logger.Debug("Monitoring: collecting system metrics")

	info := &MonitoringInfo{}

	// System info (RAM + Project directory size)
	info.System = s.getSystemInfo(workDir)

	// Application info
	info.Application = s.getApplicationInfo()

	// Logs summary
	info.Logs = s.getLogsSummary(workDir)

	// Include cached service info if available
	// This provides the monitoring page with topic/storage stats
	// without requiring a separate API call
	if s.statsCache != nil && s.statsCache.IsInitialized() {
		info.Service = s.statsCache.GetServiceInfo()
	}

	s.logger.Debug("Monitoring: metrics collected successfully")
	return info, nil
}

// getSystemInfo collects OS-level resource metrics.
func (s *MonitoringService) getSystemInfo(workDir string) SystemInfo {
	si := SystemInfo{}

	// RAM: Go process memory via runtime.MemStats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	si.RAMUsedBytes = memStats.Sys

	// RAM total: read from /proc/meminfo (Linux)
	si.RAMTotalBytes = readTotalRAM()

	// Project directory size: walk the working directory tree
	si.ProjectDirSizeBytes = s.calculateDirSize(workDir)

	return si
}

// CheckDiskLimit verifies that disk usage is below the configured limit.
// Returns nil if no limit is set (maxDiskUsage == 0) or if usage is within bounds.
// Returns ErrDiskLimitExceeded if usage exceeds the limit.
// Fails closed: returns an error if disk stats cannot be read.
func CheckDiskLimit(path string, maxDiskUsage int64) error {
	if maxDiskUsage <= 0 {
		return nil // No limit configured
	}

	usedBytes, err := GetDiskUsageBytes(path)
	if err != nil {
		// Fail closed: if we can't read disk stats, reject the operation
		return NewServiceError(constants.ErrCodeDiskLimitExceeded,
			"disk usage limit check failed: unable to read disk stats")
	}

	if usedBytes >= uint64(maxDiskUsage) {
		return NewServiceError(constants.ErrCodeDiskLimitExceeded,
			fmt.Sprintf("disk usage limit exceeded: used %d bytes, limit %d bytes", usedBytes, maxDiskUsage))
	}

	return nil
}

// calculateDirSize walks the directory tree and sums all regular file sizes.
// Returns 0 if the directory cannot be walked.
func (s *MonitoringService) calculateDirSize(dirPath string) uint64 {
	s.logger.Debug("Monitoring: calculating directory size for %s", dirPath)

	var totalSize uint64
	err := filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			s.logger.Warn("Monitoring: error walking path %s: %v", path, err)
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			s.logger.Warn("Monitoring: failed to stat file %s: %v", path, err)
			return nil
		}
		totalSize += uint64(info.Size())
		return nil
	})
	if err != nil {
		s.logger.Warn("Monitoring: failed to walk directory %s: %v", dirPath, err)
		return 0
	}

	s.logger.Debug("Monitoring: directory size calculated: %d bytes", totalSize)
	return totalSize
}

// readTotalRAM reads the total system RAM from /proc/meminfo.
// Returns 0 if the file cannot be read (non-Linux systems).
func readTotalRAM() uint64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}

	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				var kb uint64
				for _, c := range fields[1] {
					if c >= '0' && c <= '9' {
						kb = kb*10 + uint64(c-'0')
					}
				}
				return kb * 1024 // Convert KB to bytes
			}
		}
	}
	return 0
}

// getApplicationInfo collects application-level configuration and state metrics.
func (s *MonitoringService) getApplicationInfo() ApplicationInfo {
	cfg := s.app.GetConfig()
	startedAt := s.app.GetStartedAt()

	ai := ApplicationInfo{
		UptimeSeconds:         int64(time.Since(startedAt).Seconds()),
		StartedAt:             startedAt.Unix(),
		WorkingDirectory:      cfg.WorkingDirectory,
		Port:                  cfg.Port,
		MaxDatSizeBytes:       cfg.MaxDatSize,
		MaxMetadataValueBytes: cfg.Metadata.MaxValueBytes,
		MaxDiskUsageBytes:     cfg.MaxDiskUsage,
	}

	// Topic counts
	topicNames := s.app.ListTopics()
	ai.TopicsTotal = len(topicNames)
	for _, name := range topicNames {
		healthy, _ := s.app.IsTopicHealthy(name)
		if healthy {
			ai.TopicsHealthy++
		} else {
			ai.TopicsUnhealthy++
		}
	}

	// Total indexed hashes from orchestrator
	orchDB := s.app.GetOrchestratorDB()
	if orchDB != nil {
		var count int64
		if err := orchDB.QueryRow(constants.OrchestratorCountHashesQuery).Scan(&count); err != nil {
			s.logger.Warn("Monitoring: failed to count indexed hashes: %v", err)
		} else {
			ai.TotalIndexedHashes = count
		}
	}

	return ai
}

// getLogsSummary scans log directories and returns file summaries.
func (s *MonitoringService) getLogsSummary(workDir string) LogsSummary {
	logsBase := filepath.Join(workDir, constants.InternalDir, constants.LogsDir)
	summary := LogsSummary{
		Levels: make([]LogLevelSummary, 0, len(allLogLevels)),
	}

	for _, level := range allLogLevels {
		levelDir := filepath.Join(logsBase, level)
		ls := LogLevelSummary{Level: level}

		entries, err := os.ReadDir(levelDir)
		if err != nil {
			// Directory may not exist yet — that's fine
			summary.Levels = append(summary.Levels, ls)
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if !strings.HasSuffix(entry.Name(), constants.LogFileExtension) {
				continue
			}

			info, err := entry.Info()
			if err != nil {
				continue
			}

			ls.FileCount++
			ls.TotalSize += info.Size()

			// Only include individual file info for warn/error levels
			if isAllowedLogLevel(level) {
				ls.Files = append(ls.Files, LogFileInfo{
					Name:    entry.Name(),
					Size:    info.Size(),
					ModTime: info.ModTime().Unix(),
				})
			}
		}

		summary.Levels = append(summary.Levels, ls)
	}

	return summary
}

// =============================================================================
// Log File Content Access
// =============================================================================

// GetLogFileContent reads and returns the content of a specific log file.
// This method implements triple-layered path traversal defense:
// 1. Level must be in the allowed set (warn/error only)
// 2. Filename must not contain path separators or ".."
// 3. Resolved absolute path must be within the expected log directory
func (s *MonitoringService) GetLogFileContent(level, filename string) ([]byte, error) {
	s.logger.Info("Monitoring: reading log file level=%s filename=%s", level, filename)

	workDir := s.app.GetWorkingDirectory()
	if workDir == "" {
		return nil, ErrNotConfigured
	}

	// Layer 1: Validate level is in the allowed set
	if !isAllowedLogLevel(level) {
		s.logger.Warn("Monitoring: rejected log level access: %s", level)
		return nil, NewServiceError(constants.ErrCodeLogLevelNotAllowed,
			"log level not accessible: "+level)
	}

	// Layer 2: Validate filename contains no path traversal characters
	if sanitize.IsPathTraversal(filename) {
		s.logger.Warn("Monitoring: rejected suspicious filename: %s", filename)
		return nil, NewServiceError(constants.ErrCodeInvalidRequest,
			"invalid log filename")
	}
	if !strings.HasSuffix(filename, constants.LogFileExtension) {
		s.logger.Warn("Monitoring: rejected non-log file extension: %s", filename)
		return nil, NewServiceError(constants.ErrCodeInvalidRequest,
			"invalid log file extension")
	}

	// Layer 3: Construct path and verify it stays within the expected directory
	logDir := filepath.Join(workDir, constants.InternalDir, constants.LogsDir, level)
	fullPath := filepath.Join(logDir, filename)

	absLogDir, err := filepath.Abs(logDir)
	if err != nil {
		return nil, WrapInternalError(err)
	}
	absFullPath, err := filepath.Abs(fullPath)
	if err != nil {
		return nil, WrapInternalError(err)
	}
	if !strings.HasPrefix(absFullPath, absLogDir+string(filepath.Separator)) {
		s.logger.Warn("Monitoring: path traversal detected: resolved=%s expected_prefix=%s", absFullPath, absLogDir)
		return nil, NewServiceError(constants.ErrCodeInvalidRequest,
			"path traversal detected")
	}

	// Check file exists
	info, err := os.Stat(fullPath)
	if os.IsNotExist(err) {
		return nil, NewServiceError(constants.ErrCodeLogFileNotFound,
			"log file not found: "+filename)
	}
	if err != nil {
		return nil, WrapInternalError(err)
	}

	// Read with size cap
	maxReadBytes := s.app.GetConfig().Monitoring.LogFileMaxReadBytes
	readSize := info.Size()
	if readSize > maxReadBytes {
		readSize = maxReadBytes
		s.logger.Info("Monitoring: truncating log file %s/%s to %d bytes (file size: %d)",
			level, filename, readSize, info.Size())
	}

	file, err := os.Open(fullPath)
	if err != nil {
		return nil, WrapInternalError(err)
	}
	defer file.Close()

	buf := make([]byte, readSize)
	n, err := io.ReadFull(file, buf)
	if err != nil && err != io.ErrUnexpectedEOF {
		return nil, WrapInternalError(err)
	}

	s.logger.Debug("Monitoring: read %d bytes from log file %s/%s", n, level, filename)
	return buf[:n], nil
}

// =============================================================================
// Helpers
// =============================================================================

// isAllowedLogLevel checks if a log level is in the allowed set for API access.
func isAllowedLogLevel(level string) bool {
	for _, allowed := range monitoringAllowedLogLevels {
		if allowed == level {
			return true
		}
	}
	return false
}
