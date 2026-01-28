package services

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"silobang/internal/constants"
	"silobang/internal/logger"
	"silobang/internal/storage"
)

// TopicStatsSnapshot holds cached stats for one topic.
type TopicStatsSnapshot struct {
	Stats      map[string]interface{}
	ComputedAt time.Time
}

// ServiceInfoSnapshot holds pre-aggregated service-level metrics.
// Mirrors the fields of the server.ServiceInfo struct, which is in package server.
// We define our own here to avoid circular imports.
type ServiceInfoSnapshot struct {
	OrchestratorDBSize int64                  `json:"orchestrator_db_size"`
	TotalIndexedHashes int64                  `json:"total_indexed_hashes"`
	TopicsSummary      TopicsSummarySnapshot  `json:"topics_summary"`
	StorageSummary     StorageSummarySnapshot `json:"storage_summary"`
	MaxDiskUsageBytes  int64                  `json:"max_disk_usage_bytes"`
	ComputedAt         time.Time              `json:"-"`
}

// TopicsSummarySnapshot provides counts of topics by health status.
type TopicsSummarySnapshot struct {
	Total     int `json:"total"`
	Healthy   int `json:"healthy"`
	Unhealthy int `json:"unhealthy"`
}

// StorageSummarySnapshot provides aggregated storage metrics across all topics.
type StorageSummarySnapshot struct {
	TotalDatSize   int64   `json:"total_dat_size"`
	TotalDbSize    int64   `json:"total_db_size"`
	TotalAssetSize int64   `json:"total_asset_size"`
	TotalDatFiles  int     `json:"total_dat_files"`
	AvgAssetSize   float64 `json:"avg_asset_size"`
}

// StatsCache provides thread-safe cached access to topic stats and service info.
type StatsCache struct {
	app         AppState
	logger      *logger.Logger
	configSvc   *ConfigService
	mu          sync.RWMutex
	topicStats  map[string]*TopicStatsSnapshot
	serviceInfo *ServiceInfoSnapshot
	initialized bool
}

// NewStatsCache creates a new stats cache instance.
func NewStatsCache(app AppState, log *logger.Logger, configSvc *ConfigService) *StatsCache {
	return &StatsCache{
		app:        app,
		logger:     log,
		configSvc:  configSvc,
		topicStats: make(map[string]*TopicStatsSnapshot),
	}
}

// BuildAll rebuilds the entire cache by iterating all healthy topics
// and computing aggregated service-level metrics.
func (s *StatsCache) BuildAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	topicNames := s.app.ListTopics()
	s.topicStats = make(map[string]*TopicStatsSnapshot, len(topicNames))

	for _, name := range topicNames {
		healthy, _ := s.app.IsTopicHealthy(name)
		if !healthy {
			s.logger.Debug("[stats-cache] skipping unhealthy topic %s during full build", name)
			continue
		}

		stats, err := s.configSvc.GetTopicStats(name)
		if err != nil {
			s.logger.Warn("[stats-cache] failed to get stats for topic %s: %v", name, err)
			continue
		}

		s.topicStats[name] = &TopicStatsSnapshot{
			Stats:      stats,
			ComputedAt: time.Now(),
		}
	}

	s.serviceInfo = s.buildServiceInfo()
	s.initialized = true

	s.logger.Info("[stats-cache] cache built: %d topics cached", len(s.topicStats))
}

// InvalidateTopic refreshes the cached stats for a single topic.
// If the topic is healthy, its stats are recomputed; if unhealthy, it is removed from the cache.
// Service info is recomputed after the update.
func (s *StatsCache) InvalidateTopic(topicName string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	healthy, _ := s.app.IsTopicHealthy(topicName)
	if healthy {
		stats, err := s.configSvc.GetTopicStats(topicName)
		if err != nil {
			s.logger.Warn("[stats-cache] failed to refresh stats for topic %s: %v", topicName, err)
		} else {
			s.topicStats[topicName] = &TopicStatsSnapshot{
				Stats:      stats,
				ComputedAt: time.Now(),
			}
		}
	} else {
		delete(s.topicStats, topicName)
	}

	s.serviceInfo = s.buildServiceInfo()

	s.logger.Info("[stats-cache] topic %s invalidated", topicName)
}

// InvalidateTopics refreshes the cached stats for multiple topics in a single lock acquisition.
// Service info is recomputed once after all topics are updated.
func (s *StatsCache) InvalidateTopics(topicNames []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, topicName := range topicNames {
		healthy, _ := s.app.IsTopicHealthy(topicName)
		if healthy {
			stats, err := s.configSvc.GetTopicStats(topicName)
			if err != nil {
				s.logger.Warn("[stats-cache] failed to refresh stats for topic %s: %v", topicName, err)
				continue
			}
			s.topicStats[topicName] = &TopicStatsSnapshot{
				Stats:      stats,
				ComputedAt: time.Now(),
			}
		} else {
			delete(s.topicStats, topicName)
		}
	}

	s.serviceInfo = s.buildServiceInfo()

	s.logger.Info("[stats-cache] %d topics invalidated", len(topicNames))
}

// RemoveTopic deletes a topic from the cache and recomputes service info.
func (s *StatsCache) RemoveTopic(topicName string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.topicStats, topicName)
	s.serviceInfo = s.buildServiceInfo()

	s.logger.Info("[stats-cache] topic %s removed from cache", topicName)
}

// GetTopicStats returns the cached stats for a single topic.
// Returns the stats map and true if found, or nil and false if not cached.
func (s *StatsCache) GetTopicStats(topicName string) (map[string]interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot, ok := s.topicStats[topicName]
	if !ok {
		return nil, false
	}
	return snapshot.Stats, true
}

// GetAllTopicStats returns a copy of all cached topic stats.
func (s *StatsCache) GetAllTopicStats() map[string]map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]map[string]interface{}, len(s.topicStats))
	for name, snapshot := range s.topicStats {
		result[name] = snapshot.Stats
	}
	return result
}

// GetServiceInfo returns the cached service-level metrics snapshot.
// Returns nil if the cache has not been initialized yet.
func (s *StatsCache) GetServiceInfo() *ServiceInfoSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.serviceInfo
}

// IsInitialized reports whether the cache has completed its initial build.
func (s *StatsCache) IsInitialized() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.initialized
}

// buildServiceInfo aggregates metrics from cached topic stats into a ServiceInfoSnapshot.
// MUST be called with the write lock held; this method does not acquire any locks.
func (s *StatsCache) buildServiceInfo() *ServiceInfoSnapshot {
	info := &ServiceInfoSnapshot{
		ComputedAt: time.Now(),
	}

	// --- Orchestrator DB size ---
	orchPath := filepath.Join(s.app.GetWorkingDirectory(), constants.InternalDir, constants.OrchestratorDB)
	if fi, err := os.Stat(orchPath); err == nil {
		info.OrchestratorDBSize = fi.Size()
	} else {
		s.logger.Debug("[stats-cache] unable to stat orchestrator DB at %s: %v", orchPath, err)
	}

	// --- Total indexed hashes from orchestrator ---
	orchDB := s.app.GetOrchestratorDB()
	if orchDB != nil {
		var count int64
		if err := orchDB.QueryRow(constants.OrchestratorCountHashesQuery).Scan(&count); err != nil {
			s.logger.Warn("[stats-cache] failed to count indexed hashes: %v", err)
		} else {
			info.TotalIndexedHashes = count
		}
	}

	// --- Topic health counts ---
	topicNames := s.app.ListTopics()
	info.TopicsSummary.Total = len(topicNames)
	for _, name := range topicNames {
		healthy, _ := s.app.IsTopicHealthy(name)
		if healthy {
			info.TopicsSummary.Healthy++
		} else {
			info.TopicsSummary.Unhealthy++
		}
	}

	// --- Aggregate storage metrics from cached topic stats ---
	var storageSummary StorageSummarySnapshot
	for topicName, snapshot := range s.topicStats {
		storageSummary.TotalDatSize += toInt64(snapshot.Stats["dat_size"])
		storageSummary.TotalDbSize += toInt64(snapshot.Stats["db_size"])
		storageSummary.TotalAssetSize += toInt64(snapshot.Stats["total_size"])

		topicPath := s.app.GetTopicPath(topicName)
		datCount, err := storage.CountDatFiles(topicPath)
		if err != nil {
			s.logger.Warn("[stats-cache] failed to count dat files for topic %s: %v", topicName, err)
		} else {
			storageSummary.TotalDatFiles += datCount
		}
	}

	// Compute average asset size
	if info.TotalIndexedHashes > 0 {
		storageSummary.AvgAssetSize = float64(storageSummary.TotalAssetSize) / float64(info.TotalIndexedHashes)
	}
	info.StorageSummary = storageSummary

	// --- Max disk usage from config ---
	info.MaxDiskUsageBytes = s.app.GetConfig().MaxDiskUsage

	s.logger.Debug("[stats-cache] service info built: topics=%d, indexed_hashes=%d, total_dat_size=%d",
		info.TopicsSummary.Total, info.TotalIndexedHashes, info.StorageSummary.TotalDatSize)

	return info
}

// toInt64 safely extracts an int64 from an interface{} value.
// Handles int64, float64, and int types; returns 0 for anything else.
func toInt64(v interface{}) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case float64:
		return int64(n)
	case int:
		return int64(n)
	default:
		return 0
	}
}
