# Phase 5: Query System

## Objective
Implement the query system that allows running preset SQL queries from `queries.yaml` across topics. This includes parsing the queries config, listing available presets, executing queries with parameter binding, and cross-topic aggregation.

---

## Prerequisites
- Phase 1 completed (project structure, constants, config, logger)
- Phase 2 completed (database layer, schema, connections)
- Phase 3 completed (blob format, .dat file operations)
- Phase 4 completed (core API: config, topics, single upload, download, metadata)

---

## Task 1: Queries Config Parser (`internal/queries/parser.go`)

### 1.1 Data Structures

```go
package queries

import (
    "fmt"
    "os"

    "gopkg.in/yaml.v3"
)

// QueriesConfig represents the entire queries.yaml file
type QueriesConfig struct {
    TopicStats []TopicStat        `yaml:"topic_stats"`
    Presets    map[string]Preset  `yaml:"presets"`
}

// TopicStat defines a stat shown on the topic list
type TopicStat struct {
    Name   string `yaml:"name"`
    Label  string `yaml:"label"`
    SQL    string `yaml:"sql,omitempty"`    // SQL query (mutually exclusive with Type)
    Type   string `yaml:"type,omitempty"`   // Special type: "file_size" or "dat_total"
    Format string `yaml:"format,omitempty"` // Display format: bytes|number|date|text
}

// Preset defines a query preset
type Preset struct {
    Description string        `yaml:"description"`
    SQL         string        `yaml:"sql"`
    Params      []PresetParam `yaml:"params,omitempty"`
}

// PresetParam defines a parameter for a preset query
type PresetParam struct {
    Name     string `yaml:"name"`
    Required bool   `yaml:"required,omitempty"`
    Default  string `yaml:"default,omitempty"`
}
```

### 1.2 Functions to Implement

#### `LoadQueriesConfig(path string) (*QueriesConfig, error)`
```go
// LoadQueriesConfig loads and parses the queries.yaml file
func LoadQueriesConfig(path string) (*QueriesConfig, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("failed to read queries config: %w", err)
    }

    var config QueriesConfig
    if err := yaml.Unmarshal(data, &config); err != nil {
        return nil, fmt.Errorf("failed to parse queries config: %w", err)
    }

    return &config, nil
}
```

#### `(c *QueriesConfig) GetPreset(name string) (*Preset, error)`
```go
// GetPreset returns a preset by name
func (c *QueriesConfig) GetPreset(name string) (*Preset, error) {
    preset, exists := c.Presets[name]
    if !exists {
        return nil, fmt.Errorf("preset not found: %s", name)
    }
    return &preset, nil
}
```

#### `(c *QueriesConfig) ListPresets() []PresetInfo`
```go
// PresetInfo contains metadata about a preset for API responses
type PresetInfo struct {
    Name        string             `json:"name"`
    Description string             `json:"description"`
    Params      []PresetParamInfo  `json:"params"`
}

// PresetParamInfo contains parameter info for API responses
type PresetParamInfo struct {
    Name     string `json:"name"`
    Required bool   `json:"required"`
    Default  string `json:"default,omitempty"`
}

// ListPresets returns info about all available presets
func (c *QueriesConfig) ListPresets() []PresetInfo {
    result := make([]PresetInfo, 0, len(c.Presets))

    for name, preset := range c.Presets {
        params := make([]PresetParamInfo, len(preset.Params))
        for i, p := range preset.Params {
            params[i] = PresetParamInfo{
                Name:     p.Name,
                Required: p.Required,
                Default:  p.Default,
            }
        }

        result = append(result, PresetInfo{
            Name:        name,
            Description: preset.Description,
            Params:      params,
        })
    }

    // Sort by name for consistent ordering
    sort.Slice(result, func(i, j int) bool {
        return result[i].Name < result[j].Name
    })

    return result
}
```

**Add import:**
```go
import "sort"
```

---

## Task 2: Query Executor (`internal/queries/executor.go`)

### 2.1 Data Structures

```go
package queries

import (
    "database/sql"
    "fmt"
    "regexp"
    "strings"
)

// QueryResult contains the result of a query execution
type QueryResult struct {
    Preset   string          `json:"preset"`
    RowCount int             `json:"row_count"`
    Columns  []string        `json:"columns"`
    Rows     [][]interface{} `json:"rows"`
}

// QueryRequest contains parameters for executing a query
type QueryRequest struct {
    Params map[string]string `json:"params"`
    Topics []string          `json:"topics"` // If empty, query all healthy topics
}
```

### 2.2 Functions to Implement

#### `ValidateParams(preset *Preset, params map[string]string) (map[string]string, error)`
```go
// ValidateParams validates and fills in default values for query parameters
// Returns the final params map with defaults applied
func ValidateParams(preset *Preset, params map[string]string) (map[string]string, error) {
    if params == nil {
        params = make(map[string]string)
    }

    result := make(map[string]string)

    for _, p := range preset.Params {
        value, provided := params[p.Name]

        if !provided || value == "" {
            if p.Required {
                return nil, fmt.Errorf("required parameter missing: %s", p.Name)
            }
            if p.Default != "" {
                result[p.Name] = p.Default
            }
        } else {
            result[p.Name] = value
        }
    }

    return result, nil
}
```

#### `BuildQuery(sqlTemplate string, params map[string]string) (string, []interface{})`
```go
// paramRegex matches :paramName patterns in SQL
var paramRegex = regexp.MustCompile(`:([a-zA-Z_][a-zA-Z0-9_]*)`)

// BuildQuery converts named parameters to positional parameters for SQLite
// Returns the query with ? placeholders and the ordered argument slice
func BuildQuery(sqlTemplate string, params map[string]string) (string, []interface{}) {
    var args []interface{}
    paramIndex := make(map[string]int) // Track which params we've seen
    argCounter := 0

    result := paramRegex.ReplaceAllStringFunc(sqlTemplate, func(match string) string {
        paramName := match[1:] // Remove leading :

        if _, seen := paramIndex[paramName]; !seen {
            paramIndex[paramName] = argCounter
            if value, exists := params[paramName]; exists {
                args = append(args, value)
            } else {
                args = append(args, nil)
            }
            argCounter++
        }

        return "?"
    })

    return result, args
}
```

#### `ExecuteQuery(db *sql.DB, query string, args []interface{}) ([]string, [][]interface{}, error)`
```go
// ExecuteQuery runs a query and returns columns and rows
func ExecuteQuery(db *sql.DB, query string, args []interface{}) ([]string, [][]interface{}, error) {
    rows, err := db.Query(query, args...)
    if err != nil {
        return nil, nil, fmt.Errorf("query execution failed: %w", err)
    }
    defer rows.Close()

    // Get column names
    columns, err := rows.Columns()
    if err != nil {
        return nil, nil, fmt.Errorf("failed to get columns: %w", err)
    }

    // Prepare result slice
    var result [][]interface{}

    // Scan rows
    for rows.Next() {
        // Create a slice of interface{} to hold the values
        values := make([]interface{}, len(columns))
        valuePtrs := make([]interface{}, len(columns))
        for i := range values {
            valuePtrs[i] = &values[i]
        }

        if err := rows.Scan(valuePtrs...); err != nil {
            return nil, nil, fmt.Errorf("failed to scan row: %w", err)
        }

        // Convert []byte to string for JSON serialization
        for i, v := range values {
            if b, ok := v.([]byte); ok {
                values[i] = string(b)
            }
        }

        result = append(result, values)
    }

    if err := rows.Err(); err != nil {
        return nil, nil, fmt.Errorf("row iteration error: %w", err)
    }

    return columns, result, nil
}
```

#### `ExecutePresetQuery(preset *Preset, params map[string]string, db *sql.DB, topicName string) ([]string, [][]interface{}, error)`
```go
// ExecutePresetQuery executes a preset query against a single topic database
// Adds _topic column to results
func ExecutePresetQuery(preset *Preset, params map[string]string, db *sql.DB, topicName string) ([]string, [][]interface{}, error) {
    // Build query with parameters
    query, args := BuildQuery(preset.SQL, params)

    // Execute query
    columns, rows, err := ExecuteQuery(db, query, args)
    if err != nil {
        return nil, nil, err
    }

    // Add _topic column
    columns = append(columns, "_topic")
    for i := range rows {
        rows[i] = append(rows[i], topicName)
    }

    return columns, rows, nil
}
```

#### `ExecuteCrossTopicQuery(preset *Preset, params map[string]string, topicDBs map[string]*sql.DB, topicNames []string) (*QueryResult, error)`
```go
// ExecuteCrossTopicQuery executes a preset query across multiple topics
// Results are interleaved (not grouped by topic)
func ExecuteCrossTopicQuery(preset *Preset, params map[string]string, topicDBs map[string]*sql.DB, topicNames []string) (*QueryResult, error) {
    var allColumns []string
    var allRows [][]interface{}

    for _, topicName := range topicNames {
        db, exists := topicDBs[topicName]
        if !exists {
            continue
        }

        columns, rows, err := ExecutePresetQuery(preset, params, db, topicName)
        if err != nil {
            // Log error but continue with other topics
            continue
        }

        // Set columns from first successful query
        if allColumns == nil {
            allColumns = columns
        }

        // Append rows (interleaved)
        allRows = append(allRows, rows...)
    }

    if allColumns == nil {
        allColumns = []string{}
    }
    if allRows == nil {
        allRows = [][]interface{}{}
    }

    return &QueryResult{
        RowCount: len(allRows),
        Columns:  allColumns,
        Rows:     allRows,
    }, nil
}
```

---

## Task 3: Update App State (`internal/server/app.go`)

### 3.1 Add Queries Config to App

Update the `App` struct to hold the queries configuration:

```go
type App struct {
    Config         *config.Config
    Logger         *logger.Logger
    OrchestratorDB *sql.DB
    QueriesConfig  *queries.QueriesConfig  // ADD THIS

    // ... existing fields ...
}
```

### 3.2 Add Helper Method

```go
// GetTopicDBsForQuery returns database connections for the requested topics
// If topicNames is empty, returns all healthy topics
// Returns error if any requested topic is unhealthy
func (a *App) GetTopicDBsForQuery(topicNames []string) (map[string]*sql.DB, []string, error) {
    // If no topics specified, use all healthy topics
    if len(topicNames) == 0 {
        topicNames = a.ListTopics()
    }

    result := make(map[string]*sql.DB)
    var validNames []string

    for _, name := range topicNames {
        healthy, errMsg := a.IsTopicHealthy(name)
        if !healthy {
            return nil, nil, fmt.Errorf("topic %s is unhealthy: %s", name, errMsg)
        }

        db, err := a.GetTopicDB(name)
        if err != nil {
            return nil, nil, fmt.Errorf("failed to get database for topic %s: %w", name, err)
        }

        result[name] = db
        validNames = append(validNames, name)
    }

    return result, validNames, nil
}
```

---

## Task 4: Query API Handlers (`internal/server/handlers.go`)

### 4.1 Add Error Code

Add to `internal/constants/errors.go`:

```go
const (
    // ... existing codes ...
    ErrCodePresetNotFound     = "PRESET_NOT_FOUND"
    ErrCodeQueryError         = "QUERY_ERROR"
    ErrCodeMissingParam       = "MISSING_PARAM"
)
```

### 4.2 Register Routes

Update `registerRoutes` in `internal/server/server.go`:

```go
func (s *Server) registerRoutes(mux *http.ServeMux) {
    // ... existing routes ...
    mux.HandleFunc("/api/queries", s.handleQueries)
    mux.HandleFunc("/api/query/", s.handleQueryExecution)
}
```

### 4.3 List Queries Handler

```go
// GET /api/queries - List available query presets
func (s *Server) handleQueries(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    if s.app.QueriesConfig == nil {
        WriteError(w, http.StatusInternalServerError, "Queries config not loaded", constants.ErrCodeInternalError)
        return
    }

    presets := s.app.QueriesConfig.ListPresets()

    WriteSuccess(w, map[string]interface{}{
        "presets": presets,
    })
}
```

### 4.4 Execute Query Handler

```go
// POST /api/query/:preset - Run a preset query
func (s *Server) handleQueryExecution(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // Check if configured
    if s.app.Config.WorkingDirectory == "" {
        WriteError(w, http.StatusBadRequest, "Working directory not configured", constants.ErrCodeNotConfigured)
        return
    }

    if s.app.QueriesConfig == nil {
        WriteError(w, http.StatusInternalServerError, "Queries config not loaded", constants.ErrCodeInternalError)
        return
    }

    // Parse preset name from path: /api/query/:preset
    path := r.URL.Path
    prefix := "/api/query/"

    if !strings.HasPrefix(path, prefix) {
        http.NotFound(w, r)
        return
    }

    presetName := path[len(prefix):]
    if presetName == "" {
        WriteError(w, http.StatusBadRequest, "Preset name is required", constants.ErrCodeInvalidRequest)
        return
    }

    // Get preset
    preset, err := s.app.QueriesConfig.GetPreset(presetName)
    if err != nil {
        WriteError(w, http.StatusNotFound, "Preset not found: "+presetName, constants.ErrCodePresetNotFound)
        return
    }

    // Parse request body
    var req queries.QueryRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        // Empty body is OK - use defaults
        req = queries.QueryRequest{}
    }

    // Validate and apply default params
    params, err := queries.ValidateParams(preset, req.Params)
    if err != nil {
        WriteError(w, http.StatusBadRequest, err.Error(), constants.ErrCodeMissingParam)
        return
    }

    // Get topic databases
    topicDBs, topicNames, err := s.app.GetTopicDBsForQuery(req.Topics)
    if err != nil {
        WriteError(w, http.StatusBadRequest, err.Error(), constants.ErrCodeTopicUnhealthy)
        return
    }

    if len(topicNames) == 0 {
        // No topics available - return empty result
        WriteSuccess(w, &queries.QueryResult{
            Preset:   presetName,
            RowCount: 0,
            Columns:  []string{},
            Rows:     [][]interface{}{},
        })
        return
    }

    // Execute query across topics
    result, err := queries.ExecuteCrossTopicQuery(preset, params, topicDBs, topicNames)
    if err != nil {
        WriteError(w, http.StatusInternalServerError, "Query execution failed: "+err.Error(), constants.ErrCodeQueryError)
        return
    }

    result.Preset = presetName

    s.logger.Debug("Executed query %s across %d topics, returned %d rows", presetName, len(topicNames), result.RowCount)

    WriteSuccess(w, result)
}
```

**Add import to handlers.go:**
```go
import (
    "meshbank/internal/queries"
)
```

---

## Task 5: Update Main Entry Point (`cmd/meshbank/main.go`)

### 5.1 Load Queries Config on Startup

Add after loading config:

```go
// Load queries config
queriesPath := config.GetQueriesPath()
queriesConfig, err := queries.LoadQueriesConfig(queriesPath)
if err != nil {
    log.Warn("Failed to load queries config: %v", err)
    // Continue without queries - API will return error
} else {
    log.Debug("Loaded %d query presets", len(queriesConfig.Presets))
}

// ... later when creating app ...
app := server.NewApp(cfg, log)
app.QueriesConfig = queriesConfig  // ADD THIS
```

**Add import:**
```go
import (
    "meshbank/internal/queries"
)
```

---

## Task 6: Update Topic Stats (`internal/server/handlers.go`)

Update `getTopicStats` to use `QueriesConfig.TopicStats` for dynamic stat computation:

```go
func (s *Server) getTopicStats(topicName string) (map[string]interface{}, error) {
    db, err := s.app.GetTopicDB(topicName)
    if err != nil {
        return nil, err
    }

    topicPath := s.app.GetTopicPath(topicName)
    stats := make(map[string]interface{})

    // If no queries config, use hardcoded defaults
    if s.app.QueriesConfig == nil || len(s.app.QueriesConfig.TopicStats) == 0 {
        return s.getDefaultTopicStats(db, topicName, topicPath)
    }

    // Execute each stat query
    for _, stat := range s.app.QueriesConfig.TopicStats {
        var value interface{}
        var err error

        switch stat.Type {
        case "file_size":
            // Special: read database file size
            dbPath := filepath.Join(topicPath, constants.InternalDir, topicName+".db")
            if info, statErr := os.Stat(dbPath); statErr == nil {
                value = info.Size()
            } else {
                value = int64(0)
            }
        case "dat_total":
            // Special: sum of all .dat file sizes
            value, err = storage.GetTotalDatSize(topicPath)
            if err != nil {
                value = int64(0)
            }
        default:
            // SQL query
            if stat.SQL != "" {
                var result sql.NullString
                err = db.QueryRow(stat.SQL).Scan(&result)
                if err == nil && result.Valid {
                    value = result.String
                } else {
                    value = nil
                }
            }
        }

        stats[stat.Name] = value
    }

    return stats, nil
}

// getDefaultTopicStats returns hardcoded stats when no config is available
func (s *Server) getDefaultTopicStats(db *sql.DB, topicName string, topicPath string) (map[string]interface{}, error) {
    stats := make(map[string]interface{})

    // Total size from assets
    var totalSize sql.NullInt64
    db.QueryRow("SELECT SUM(asset_size) FROM assets").Scan(&totalSize)
    stats["total_size"] = totalSize.Int64

    // File count
    var fileCount int64
    db.QueryRow("SELECT COUNT(*) FROM assets").Scan(&fileCount)
    stats["file_count"] = fileCount

    // Average size
    var avgSize sql.NullFloat64
    db.QueryRow("SELECT AVG(asset_size) FROM assets").Scan(&avgSize)
    stats["avg_size"] = avgSize.Float64

    // Last added
    var lastAdded sql.NullInt64
    db.QueryRow("SELECT MAX(created_at) FROM assets").Scan(&lastAdded)
    stats["last_added"] = lastAdded.Int64

    // Last hash
    var lastHash sql.NullString
    db.QueryRow("SELECT asset_id FROM assets ORDER BY created_at DESC LIMIT 1").Scan(&lastHash)
    stats["last_hash"] = lastHash.String

    // DB size (file size)
    dbPath := filepath.Join(topicPath, constants.InternalDir, topicName+".db")
    if info, err := os.Stat(dbPath); err == nil {
        stats["db_size"] = info.Size()
    }

    // DAT total size
    datSize, err := storage.GetTotalDatSize(topicPath)
    if err == nil {
        stats["dat_size"] = datSize
    }

    return stats, nil
}
```

---

## Verification Checklist

After completing Phase 5, verify:

1. **Queries config loading:**
   - Server starts and loads queries.yaml
   - Log shows number of presets loaded
   - Invalid YAML → warning, server continues

2. **GET /api/queries:**
   - Returns list of all presets
   - Each preset has name, description, params
   - Params show required flag and defaults

3. **POST /api/query/:preset:**
   - Run `recent-imports` with default params → returns results
   - Run `by-hash` without required `hash` param → 400 error
   - Run with `topics: ["test-topic"]` → only queries that topic
   - Run with `topics: ["unhealthy-topic"]` → 400 error
   - Unknown preset name → 404 error

4. **Cross-topic queries:**
   - Create 2 topics with files
   - Run query without `topics` param → results from both
   - Results include `_topic` column
   - Results are interleaved (not grouped)

5. **Parameter binding:**
   - Run `large-files` with `min_size: 1000` → filters correctly
   - Run `lineage` with `hash: <valid_hash>` → returns lineage chain
   - Default values applied when params omitted

6. **Topic stats:**
   - GET /api/topics returns stats computed from queries.yaml
   - `file_size` type returns DB file size
   - `dat_total` type returns sum of .dat sizes

---

## Files to Create/Update

| File | Action | Description |
|------|--------|-------------|
| `internal/queries/parser.go` | Create | Queries config parsing and structures |
| `internal/queries/executor.go` | Create | Query execution and cross-topic aggregation |
| `internal/constants/errors.go` | Update | Add query-related error codes |
| `internal/server/app.go` | Update | Add QueriesConfig field and helper methods |
| `internal/server/server.go` | Update | Register query routes |
| `internal/server/handlers.go` | Update | Add query handlers, update getTopicStats |
| `cmd/meshbank/main.go` | Update | Load queries config on startup |

---

## Notes for Agent

- **No arbitrary SQL:** Only presets from queries.yaml can be executed. Never expose raw SQL execution.
- **Parameter safety:** Use `?` placeholders via `BuildQuery()` - SQLite handles escaping.
- **Unhealthy topics:** Return error immediately if any requested topic is unhealthy. Do not silently skip.
- **Empty results:** Return empty arrays, not null, when no results found.
- **_topic column:** Always append `_topic` column to results for cross-topic identification.
- **Interleaved results:** Do not group by topic. Results are returned in the order topics are processed.
- **Config reload:** Currently queries.yaml is loaded once at startup. Future enhancement could add hot-reload.
- **Import queries package:** Add import in handlers.go and main.go.
- **Null handling:** Convert SQL NULL values appropriately for JSON (nil → null).
- **[]byte to string:** SQLite may return []byte for TEXT columns - convert to string for JSON.
