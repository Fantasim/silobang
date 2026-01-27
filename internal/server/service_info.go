package server

import (
	"os"
	"path/filepath"

	"silobang/internal/constants"
	"silobang/internal/storage"
	"silobang/internal/version"
)

// ServiceInfo holds service-level metrics for the GET /api/topics response
type ServiceInfo struct {
	OrchestratorDBSize int64          `json:"orchestrator_db_size"`
	TotalIndexedHashes int64          `json:"total_indexed_hashes"`
	TopicsSummary      TopicsSummary  `json:"topics_summary"`
	StorageSummary     StorageSummary `json:"storage_summary"`
	VersionInfo        VersionInfo    `json:"version_info"`
}

// TopicsSummary provides counts of topics by health status
type TopicsSummary struct {
	Total     int `json:"total"`
	Healthy   int `json:"healthy"`
	Unhealthy int `json:"unhealthy"`
}

// StorageSummary provides aggregated storage metrics across all topics
type StorageSummary struct {
	TotalDatSize   int64   `json:"total_dat_size"`
	TotalDbSize    int64   `json:"total_db_size"`
	TotalAssetSize int64   `json:"total_asset_size"`
	TotalDatFiles  int     `json:"total_dat_files"`
	AvgAssetSize   float64 `json:"avg_asset_size"`
}

// VersionInfo provides version and format information
type VersionInfo struct {
	AppVersion  string `json:"app_version"`
	BlobVersion uint16 `json:"blob_version"`
	HeaderSize  int    `json:"header_size"`
}

// getServiceInfo gathers all service-level metrics
// It takes pre-computed topic stats to avoid redundant queries
func (s *Server) getServiceInfo(topicStats map[string]map[string]interface{}) (*ServiceInfo, error) {
	info := &ServiceInfo{
		VersionInfo: VersionInfo{
			AppVersion:  version.Version,
			BlobVersion: constants.BlobVersion,
			HeaderSize:  constants.HeaderSize,
		},
	}

	// Orchestrator DB size
	orchDBSize, err := s.getOrchestratorDBSize()
	if err != nil {
		s.logger.Warn("Failed to get orchestrator DB size: %v", err)
	}
	info.OrchestratorDBSize = orchDBSize

	// Total indexed hashes
	totalHashes, err := s.getTotalIndexedHashes()
	if err != nil {
		s.logger.Warn("Failed to get total indexed hashes: %v", err)
	}
	info.TotalIndexedHashes = totalHashes

	// Aggregate from app state and topic stats
	info.TopicsSummary, info.StorageSummary = s.aggregateTopicsStats(topicStats, totalHashes)

	return info, nil
}

// getOrchestratorDBSize returns the file size of the orchestrator database
func (s *Server) getOrchestratorDBSize() (int64, error) {
	if s.app.OrchestratorDB == nil {
		return 0, nil
	}

	orchPath := filepath.Join(
		s.app.Config.WorkingDirectory,
		constants.InternalDir,
		constants.OrchestratorDB,
	)

	info, err := os.Stat(orchPath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// getTotalIndexedHashes returns the count of all hashes in the orchestrator index
func (s *Server) getTotalIndexedHashes() (int64, error) {
	if s.app.OrchestratorDB == nil {
		return 0, nil
	}

	var count int64
	err := s.app.OrchestratorDB.QueryRow(constants.OrchestratorCountHashesQuery).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// aggregateTopicsStats computes summary statistics from individual topic stats
func (s *Server) aggregateTopicsStats(topicStats map[string]map[string]interface{}, totalHashes int64) (TopicsSummary, StorageSummary) {
	summary := TopicsSummary{}
	storageSummary := StorageSummary{}

	// Get counts from app state (thread-safe)
	topicNames := s.app.ListTopics()
	summary.Total = len(topicNames)

	for _, name := range topicNames {
		healthy, _ := s.app.IsTopicHealthy(name)
		if healthy {
			summary.Healthy++
		} else {
			summary.Unhealthy++
		}
	}

	// Aggregate storage from pre-computed topic stats
	for topicName, stats := range topicStats {
		// dat_size
		if datSize, ok := stats["dat_size"]; ok {
			if v, ok := datSize.(int64); ok {
				storageSummary.TotalDatSize += v
			}
		}

		// db_size
		if dbSize, ok := stats["db_size"]; ok {
			if v, ok := dbSize.(int64); ok {
				storageSummary.TotalDbSize += v
			}
		}

		// total_size (asset size without headers)
		if totalSize, ok := stats["total_size"]; ok {
			if v, ok := totalSize.(int64); ok {
				storageSummary.TotalAssetSize += v
			}
		}

		// Count dat files for this topic
		topicPath := s.app.GetTopicPath(topicName)
		datCount, err := storage.CountDatFiles(topicPath)
		if err != nil {
			s.logger.Warn("Failed to count dat files for topic %s: %v", topicName, err)
		} else {
			storageSummary.TotalDatFiles += datCount
		}
	}

	// Compute average asset size
	if totalHashes > 0 {
		storageSummary.AvgAssetSize = float64(storageSummary.TotalAssetSize) / float64(totalHashes)
	}

	return summary, storageSummary
}
