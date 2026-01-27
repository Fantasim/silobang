package audit

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"meshbank/internal/constants"
)

// subscription wraps a channel with safe closure tracking to prevent
// "send on closed channel" panics during concurrent unsubscribe/notify
type subscription struct {
	ch       chan Entry
	closedMu sync.Mutex
	closed   bool
}

// trySend safely sends an entry, returning false if channel is closed or full
func (s *subscription) trySend(entry Entry) bool {
	s.closedMu.Lock()
	defer s.closedMu.Unlock()
	if s.closed {
		return false
	}
	select {
	case s.ch <- entry:
		return true
	default:
		return false // Channel full, skip
	}
}

// close safely closes the channel once
func (s *subscription) close() {
	s.closedMu.Lock()
	defer s.closedMu.Unlock()
	if !s.closed {
		s.closed = true
		close(s.ch)
	}
}

// Logger provides thread-safe audit logging with pub/sub for SSE streaming
type Logger struct {
	db          *sql.DB
	mu          sync.Mutex
	subscribers map[chan Entry]*subscription
	subMu       sync.RWMutex
	stopClean   chan struct{} // For cleanup goroutine shutdown
}

// NewLogger creates a new audit logger and starts the cleanup goroutine
func NewLogger(db *sql.DB) *Logger {
	l := &Logger{
		db:          db,
		subscribers: make(map[chan Entry]*subscription),
		stopClean:   make(chan struct{}),
	}

	// Start cleanup goroutine for log size management
	go l.cleanupLoop()

	return l
}

// Stop stops the cleanup goroutine (call during graceful shutdown)
func (l *Logger) Stop() {
	close(l.stopClean)
}

// Log records an audit entry (thread-safe, append-only)
func (l *Logger) Log(action string, ipAddress string, username string, details interface{}) error {
	if !IsValidAction(action) {
		return fmt.Errorf("invalid action type: %s", action)
	}

	var detailsJSON sql.NullString
	if details != nil {
		jsonBytes, err := json.Marshal(details)
		if err != nil {
			return fmt.Errorf("failed to marshal details: %w", err)
		}
		detailsJSON = sql.NullString{String: string(jsonBytes), Valid: true}
	}

	timestamp := time.Now().Unix()

	l.mu.Lock()
	defer l.mu.Unlock()

	result, err := l.db.Exec(`
		INSERT INTO audit_log (timestamp, action, ip_address, username, details_json)
		VALUES (?, ?, ?, ?, ?)
	`, timestamp, action, ipAddress, username, detailsJSON)
	if err != nil {
		return fmt.Errorf("failed to insert audit log: %w", err)
	}

	id, _ := result.LastInsertId()

	// Notify subscribers (non-blocking)
	entry := Entry{
		ID:        id,
		Timestamp: timestamp,
		Action:    action,
		IPAddress: ipAddress,
		Username:  username,
		Details:   details,
	}
	l.notifySubscribers(entry)

	return nil
}

// Subscribe returns a channel that receives new audit entries
func (l *Logger) Subscribe() chan Entry {
	ch := make(chan Entry, constants.AuditSSEBufferSize)
	sub := &subscription{ch: ch}
	l.subMu.Lock()
	l.subscribers[ch] = sub
	l.subMu.Unlock()
	return ch
}

// Unsubscribe removes a subscriber channel
func (l *Logger) Unsubscribe(ch chan Entry) {
	l.subMu.Lock()
	if sub, exists := l.subscribers[ch]; exists {
		delete(l.subscribers, ch)
		sub.close() // Safe close via wrapper
	}
	l.subMu.Unlock()
}

// notifySubscribers sends entry to all subscribers (non-blocking)
func (l *Logger) notifySubscribers(entry Entry) {
	l.subMu.RLock()
	defer l.subMu.RUnlock()

	for _, sub := range l.subscribers {
		sub.trySend(entry) // Safe send via wrapper
	}
}

// cleanupLoop periodically checks and enforces the log size limit
func (l *Logger) cleanupLoop() {
	ticker := time.NewTicker(time.Duration(constants.AuditCleanupIntervalMins) * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-l.stopClean:
			return
		case <-ticker.C:
			l.enforceLogSizeLimit()
		}
	}
}

// enforceLogSizeLimit checks the audit log size and purges oldest entries if needed
func (l *Logger) enforceLogSizeLimit() {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Get current database size using SQLite pragmas
	var pageCount, pageSize int64
	if err := l.db.QueryRow("SELECT page_count FROM pragma_page_count()").Scan(&pageCount); err != nil {
		return
	}
	if err := l.db.QueryRow("SELECT page_size FROM pragma_page_size()").Scan(&pageSize); err != nil {
		return
	}

	totalSize := pageCount * pageSize
	if totalSize < constants.AuditMaxLogSizeBytes {
		return // Under limit, nothing to do
	}

	// Calculate how many entries to purge
	var totalEntries int64
	if err := l.db.QueryRow("SELECT COUNT(*) FROM audit_log").Scan(&totalEntries); err != nil {
		return
	}

	// Purge percentage of entries (or minimum threshold)
	purgeCount := totalEntries * int64(constants.AuditPurgePercentage) / 100
	if purgeCount < int64(constants.AuditMinPurgeEntries) {
		purgeCount = int64(constants.AuditMinPurgeEntries)
	}

	// Don't purge more than we have
	if purgeCount > totalEntries {
		purgeCount = totalEntries / 2 // Keep at least half
	}

	if purgeCount <= 0 {
		return
	}

	// Transactional delete of oldest entries (lowest IDs)
	tx, err := l.db.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		DELETE FROM audit_log
		WHERE id IN (
			SELECT id FROM audit_log
			ORDER BY id ASC
			LIMIT ?
		)
	`, purgeCount)
	if err != nil {
		return
	}

	tx.Commit()
}
