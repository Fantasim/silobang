# Phase 8: Notification/Logging System

## Objective
Implement an in-memory notification system that captures errors, warnings, and important events for display on the dashboard. This replaces the placeholder `/api/logs` endpoint from Phase 4.

---

## Prerequisites
- Phase 4 completed (HTTP server, `/api/logs` placeholder)
- Phase 6 completed (frontend logs page to display entries)

---

## Task 1: Log Entry Structure (`internal/logger/notifications.go`)

```go
package logger

import (
    "sync"
    "time"
)

// LogLevel represents the severity of a log entry
type LogLevel string

const (
    LogLevelDebug   LogLevel = "debug"
    LogLevelInfo    LogLevel = "info"
    LogLevelWarning LogLevel = "warning"
    LogLevelError   LogLevel = "error"
)

// LogEntry represents a single notification/log entry
type LogEntry struct {
    ID        int64     `json:"id"`
    Timestamp time.Time `json:"timestamp"`
    Level     LogLevel  `json:"level"`
    Topic     string    `json:"topic,omitempty"` // Empty for system-level logs
    Message   string    `json:"message"`
    Details   string    `json:"details,omitempty"` // Extended info (stack trace, etc.)
}
```

---

## Task 2: Notification Store (`internal/logger/notifications.go`)

```go
// NotificationStore holds in-memory log entries
type NotificationStore struct {
    entries    []LogEntry
    maxEntries int
    nextID     int64
    mu         sync.RWMutex
}

// NewNotificationStore creates a new notification store
func NewNotificationStore(maxEntries int) *NotificationStore {
    return &NotificationStore{
        entries:    make([]LogEntry, 0),
        maxEntries: maxEntries,
        nextID:     1,
    }
}

// Add adds a new log entry
func (ns *NotificationStore) Add(level LogLevel, topic, message, details string) {
    ns.mu.Lock()
    defer ns.mu.Unlock()

    entry := LogEntry{
        ID:        ns.nextID,
        Timestamp: time.Now(),
        Level:     level,
        Topic:     topic,
        Message:   message,
        Details:   details,
    }
    ns.nextID++

    ns.entries = append(ns.entries, entry)

    // Trim if exceeds max
    if len(ns.entries) > ns.maxEntries {
        ns.entries = ns.entries[len(ns.entries)-ns.maxEntries:]
    }
}

// GetAll returns all entries (newest first)
func (ns *NotificationStore) GetAll() []LogEntry {
    ns.mu.RLock()
    defer ns.mu.RUnlock()

    // Return copy in reverse order (newest first)
    result := make([]LogEntry, len(ns.entries))
    for i, entry := range ns.entries {
        result[len(ns.entries)-1-i] = entry
    }
    return result
}

// GetSince returns entries after the given ID
func (ns *NotificationStore) GetSince(afterID int64) []LogEntry {
    ns.mu.RLock()
    defer ns.mu.RUnlock()

    var result []LogEntry
    for _, entry := range ns.entries {
        if entry.ID > afterID {
            result = append(result, entry)
        }
    }

    // Reverse for newest first
    for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
        result[i], result[j] = result[j], result[i]
    }
    return result
}

// Clear removes all entries
func (ns *NotificationStore) Clear() {
    ns.mu.Lock()
    defer ns.mu.Unlock()
    ns.entries = make([]LogEntry, 0)
}
```

---

## Task 3: Convenience Methods

```go
// Helper methods for common log levels
func (ns *NotificationStore) Info(topic, message string) {
    ns.Add(LogLevelInfo, topic, message, "")
}

func (ns *NotificationStore) Warning(topic, message string) {
    ns.Add(LogLevelWarning, topic, message, "")
}

func (ns *NotificationStore) Error(topic, message, details string) {
    ns.Add(LogLevelError, topic, message, details)
}

func (ns *NotificationStore) SystemError(message, details string) {
    ns.Add(LogLevelError, "", message, details)
}

func (ns *NotificationStore) TopicError(topic, message, details string) {
    ns.Add(LogLevelError, topic, message, details)
}
```

---

## Task 4: Integration with App (`internal/server/app.go`)

### 4.1 Add to App struct

```go
type App struct {
    Config         *config.Config
    Logger         *logger.Logger
    Notifications  *logger.NotificationStore  // ADD THIS
    OrchestratorDB *sql.DB
    QueriesConfig  *queries.QueriesConfig
    // ... existing fields ...
}
```

### 4.2 Initialize in NewApp

```go
func NewApp(cfg *config.Config, log *logger.Logger) *App {
    return &App{
        Config:        cfg,
        Logger:        log,
        Notifications: logger.NewNotificationStore(1000), // Keep last 1000 entries
        topicDBs:      make(map[string]*sql.DB),
        topicHealth:   make(map[string]*TopicHealth),
    }
}
```

---

## Task 5: Update Handlers to Log Events

### 5.1 Topic Discovery Errors

In `cmd/meshbank/main.go` or discovery code:

```go
for _, t := range topics {
    if !t.Healthy {
        app.Notifications.TopicError(t.Name, "Topic unhealthy on startup", t.Error)
    }
}
```

### 5.2 Corruption Detection

In discovery or verification code:

```go
if !hashValid {
    app.Notifications.TopicError(topicName,
        "DAT file corruption detected",
        fmt.Sprintf("File %s hash mismatch", datFile))
}
```

### 5.3 Upload Errors

In upload handler:

```go
if err != nil {
    s.app.Notifications.TopicError(topicName,
        "Upload failed",
        fmt.Sprintf("File: %s, Error: %v", filename, err))
}
```

### 5.4 Query Errors

In query handler:

```go
if err != nil {
    s.app.Notifications.Warning(topicName,
        fmt.Sprintf("Query execution warning: %v", err))
}
```

---

## Task 6: Update Logs API Handler (`internal/server/handlers.go`)

Replace the placeholder implementation:

```go
// GET /api/logs - Get notification log entries
// Query params:
//   - after: only return entries with ID > this value (for polling)
//   - level: filter by level (debug|info|warning|error)
func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // Parse query params
    afterIDStr := r.URL.Query().Get("after")
    levelFilter := r.URL.Query().Get("level")

    var entries []logger.LogEntry

    if afterIDStr != "" {
        afterID, err := strconv.ParseInt(afterIDStr, 10, 64)
        if err != nil {
            WriteError(w, http.StatusBadRequest, "Invalid 'after' parameter", constants.ErrCodeInvalidRequest)
            return
        }
        entries = s.app.Notifications.GetSince(afterID)
    } else {
        entries = s.app.Notifications.GetAll()
    }

    // Filter by level if specified
    if levelFilter != "" {
        filtered := make([]logger.LogEntry, 0)
        for _, e := range entries {
            if string(e.Level) == levelFilter {
                filtered = append(filtered, e)
            }
        }
        entries = filtered
    }

    WriteSuccess(w, map[string]interface{}{
        "logs":  entries,
        "count": len(entries),
    })
}
```

---

## Task 7: Constants

Add to `internal/constants/constants.go`:

```go
const (
    MaxNotificationEntries = 1000  // Maximum log entries to keep in memory
)
```

---

## Events to Capture

| Event | Level | Topic | When |
|-------|-------|-------|------|
| Server started | info | - | Startup |
| Working directory set | info | - | Config change |
| Topic created | info | topic-name | Topic creation |
| Topic discovered | info | topic-name | Startup discovery |
| Topic unhealthy | error | topic-name | Discovery/verification |
| DAT corruption | error | topic-name | Hash verification |
| Upload failed | error | topic-name | Upload handler |
| Query failed | warning | topic-name | Query execution |
| Config error | error | - | Config loading |
| Database error | error | topic-name | DB operations |

---

## Verification Checklist

After completing Phase 8, verify:

1. **Notification store:**
   - Entries are added correctly
   - Max entries limit enforced (oldest removed)
   - Thread-safe (concurrent access works)

2. **API endpoint:**
   - `GET /api/logs` returns all entries
   - `GET /api/logs?after=5` returns entries after ID 5
   - `GET /api/logs?level=error` filters by level
   - Entries sorted newest-first

3. **Event logging:**
   - Server startup logs "Server started"
   - Topic creation logs info
   - Corruption detection logs error with details
   - Unhealthy topics have log entries

4. **Frontend integration:**
   - Logs page displays entries
   - Auto-refresh polls with `after` param
   - Expandable details work

---

## Files to Create/Update

| File | Action | Description |
|------|--------|-------------|
| `internal/logger/notifications.go` | Create | NotificationStore implementation |
| `internal/server/app.go` | Update | Add Notifications field |
| `internal/server/handlers.go` | Update | Implement handleLogs fully |
| `cmd/meshbank/main.go` | Update | Add startup notifications |
| `internal/constants/constants.go` | Update | Add MaxNotificationEntries |

---

## Notes

- In-memory only - logs are lost on restart (acceptable for v1)
- Future enhancement: persist to SQLite for history
- Keep memory usage bounded with maxEntries limit
- Use RWMutex for thread-safe concurrent access
- Debug-level logs not shown by default in UI (filter in frontend)
- Consider adding log rotation based on age (not just count) in future
