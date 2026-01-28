package services

import (
	"os"
	"sync"
	"time"

	"silobang/internal/audit"
	"silobang/internal/constants"
	"silobang/internal/database"
	"silobang/internal/logger"
)

// ReconcileResult holds the outcome of a reconciliation run.
type ReconcileResult struct {
	TopicsRemoved int      // Number of orphaned topics cleaned up
	EntriesPurged int64    // Total asset_index entries deleted
	RemovedTopics []string // Names of removed topics
}

// ReconcileService detects topic folders that have been manually removed
// from disk and purges their orphaned entries from the orchestrator database.
// It runs once at startup and periodically in the background.
type ReconcileService struct {
	app        AppState
	logger     *logger.Logger
	statsCache *StatsCache

	stopCh  chan struct{}
	running bool
	mu      sync.Mutex // serializes concurrent Reconcile calls
}

// NewReconcileService creates a new reconciliation service.
func NewReconcileService(app AppState, log *logger.Logger) *ReconcileService {
	return &ReconcileService{
		app:    app,
		logger: log,
		stopCh: make(chan struct{}),
	}
}

// SetStatsCache sets the stats cache reference for reconciliation.
// Called after StatsCache is initialized in the services container.
func (s *ReconcileService) SetStatsCache(cache *StatsCache) {
	s.statsCache = cache
}

// Reconcile performs a single reconciliation pass.
// It compares topics referenced in asset_index against topics actually
// present on disk and purges entries for any that no longer exist.
func (s *ReconcileService) Reconcile() (*ReconcileResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	orchDB := s.app.GetOrchestratorDB()
	if orchDB == nil {
		s.logger.Debug("[reconcile] skipping — orchestrator DB not available")
		return &ReconcileResult{}, nil
	}

	s.logger.Debug("[reconcile] starting reconciliation pass")

	// 1. Get all distinct topics referenced in asset_index
	indexedTopics, err := database.ListIndexedTopics(orchDB)
	if err != nil {
		s.logger.Error("[reconcile] failed to list indexed topics: %v", err)
		return nil, err
	}

	if len(indexedTopics) == 0 {
		s.logger.Debug("[reconcile] no topics in asset_index, nothing to reconcile")
		return &ReconcileResult{}, nil
	}

	// 2. Check each indexed topic against the filesystem.
	// The filesystem is the source of truth — if the folder is gone,
	// the topic's index entries are orphaned regardless of registration status.
	result := &ReconcileResult{}

	for _, topic := range indexedTopics {
		topicPath := s.app.GetTopicPath(topic)
		if _, statErr := os.Stat(topicPath); statErr == nil {
			continue // Folder exists on disk, topic is fine
		}

		// Folder is gone — purge the orphaned index entries
		purged, err := database.DeleteAssetIndexByTopic(orchDB, topic)
		if err != nil {
			s.logger.Error("[reconcile] failed to purge asset_index entries for topic %q: %v", topic, err)
			continue // best-effort: continue with other topics
		}

		// Unregister from in-memory state (no-op if already absent)
		s.app.UnregisterTopic(topic)

		// Evict from stats cache so stale metrics are not served
		if s.statsCache != nil {
			s.statsCache.RemoveTopic(topic)
			s.logger.Debug("[reconcile] evicted topic %q from stats cache", topic)
		}

		s.logger.Info("[reconcile] purged %d orphaned asset_index entries for removed topic %q", purged, topic)

		// Audit-log the removal (preserving forensic trail)
		auditLogger := s.app.GetAuditLogger()
		if auditLogger != nil {
			if auditErr := auditLogger.Log(
				constants.AuditActionReconcileTopicRemoved,
				"system",
				"system",
				audit.ReconcileTopicRemovedDetails{
					TopicName:     topic,
					EntriesPurged: purged,
				},
			); auditErr != nil {
				s.logger.Error("[reconcile] failed to write audit entry for topic %q removal: %v", topic, auditErr)
			}
		}

		result.TopicsRemoved++
		result.EntriesPurged += purged
		result.RemovedTopics = append(result.RemovedTopics, topic)
	}

	if result.TopicsRemoved > 0 {
		s.logger.Info("[reconcile] completed: removed %d topic(s), purged %d index entries", result.TopicsRemoved, result.EntriesPurged)
	} else {
		s.logger.Debug("[reconcile] completed: no orphaned topics found")
	}

	return result, nil
}

// Start launches the periodic reconciliation goroutine.
// Safe to call multiple times — subsequent calls are no-ops.
func (s *ReconcileService) Start(interval time.Duration) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	s.logger.Info("[reconcile] periodic reconciliation started (interval: %v)", interval)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-s.stopCh:
				s.logger.Info("[reconcile] periodic reconciliation stopped")
				return
			case <-ticker.C:
				if _, err := s.Reconcile(); err != nil {
					s.logger.Error("[reconcile] periodic reconciliation failed: %v", err)
				}
			}
		}
	}()
}

// Stop signals the periodic reconciliation goroutine to exit.
func (s *ReconcileService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		close(s.stopCh)
		s.running = false
	}
}
