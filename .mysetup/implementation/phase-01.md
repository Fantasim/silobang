# Phase 1: Foundation

## Objective
Set up the complete project structure, configuration system, and core initialization logic. After this phase, the server should start, load/create config files, and be ready for storage/database layers.

---

## Task 1: Project Initialization

### 1.1 Initialize Go Module
```bash
cd /home/louis/Desktop/Flyff/meshbank
go mod init meshbank
```

### 1.2 Create Directory Structure
```
meshbank/
├── cmd/
│   └── meshbank/
│       └── main.go
├── internal/
│   ├── constants/
│   │   └── constants.go
│   ├── config/
│   │   └── config.go
│   ├── storage/
│   │   ├── blob.go
│   │   └── hash.go
│   ├── database/
│   │   ├── schema.go
│   │   ├── topic.go
│   │   ├── orchestrator.go
│   │   └── metadata.go
│   ├── server/
│   │   ├── server.go
│   │   ├── app.go
│   │   ├── response.go
│   │   └── handlers.go
│   ├── queries/
│   │   ├── parser.go
│   │   └── executor.go
│   └── logger/
│       └── logger.go
├── web/
│   ├── static/
│   └── templates/
├── e2e/
├── go.mod
└── go.sum
```

Create all directories and placeholder `.go` files with package declarations.

---

## Task 2: Constants (`internal/constants/constants.go`)

Define all constants from `project/07-constants.md`:

```go
package constants

// Storage
const (
    DefaultMaxDatSize   = 1073741824 // 1GB in bytes
    DatFilePattern      = "%03d.dat" // 001.dat, 002.dat, etc.
    MinDatDigits        = 3
    HeaderSize          = 110 // bytes per entry header (see phase-03 for format)
    BlobVersion         = uint16(1)
    ReservedHeaderBytes = 32
)

var MagicBytes = []byte("MSHB")

// Header field offsets (all multi-byte integers are little-endian)
const (
    MagicBytesOffset  = 0   // 4 bytes: "MSHB"
    VersionOffset     = 4   // 2 bytes: uint16
    DataLengthOffset  = 6   // 8 bytes: uint64
    HashOffset        = 14  // 64 bytes: ASCII hex
    ReservedOffset    = 78  // 32 bytes: zero-filled
    DataStartOffset   = 110 // where asset data begins
)

// Paths
const (
    ConfigDir      = ".config/meshbank"
    ConfigFile     = "config.yaml"
    QueriesFile    = "queries.yaml"
    InternalDir    = ".internal"
    OrchestratorDB = "orchestrator.db"
)

// API
const (
    DefaultPort       = 2369
    MaxUploadSize     = 0 // no limit, handled by max_dat_size
    DefaultQueryLimit = 1000
    MaxQueryLimit     = 10000
)

// Validation
const (
    TopicNameRegex  = `^[a-z0-9_-]+$`
    MinTopicNameLen = 1
    MaxTopicNameLen = 64
    HashLength      = 64 // BLAKE3 hex string length (32 bytes = 64 hex chars)
)

// Database pragmas (optimized for low memory: < 2GB RAM)
var SQLitePragmas = []string{
    "PRAGMA journal_mode=WAL",
    "PRAGMA busy_timeout=5000",
    "PRAGMA synchronous=NORMAL",
    "PRAGMA cache_size=-8000", // 8MB per connection (reduced for low memory)
    "PRAGMA foreign_keys=ON",
}

// Logging
const (
    DefaultLogLevel = "debug"
)

// Pagination
const (
    DefaultPageSize = 100
    MaxPageSize     = 1000
)

// MIME types
var ExtensionMimeTypes = map[string]string{
    "glb":  "model/gltf-binary",
    "gltf": "model/gltf+json",
    "obj":  "text/plain",
    "fbx":  "application/octet-stream",
    "png":  "image/png",
    "jpg":  "image/jpeg",
    "jpeg": "image/jpeg",
}

const DefaultMimeType = "application/octet-stream"
```

---

## Task 3: Configuration System (`internal/config/config.go`)

### 3.1 Config Struct
```go
type Config struct {
    WorkingDirectory string `yaml:"working_directory"`
    Port             int    `yaml:"port,omitempty"`
    MaxDatSize       int64  `yaml:"max_dat_size,omitempty"`
}
```

### 3.2 Functions to Implement

#### `GetConfigDir() string`
- Returns `~/.config/meshbank/` (expand `~` to user home directory)
- Use `os.UserHomeDir()` for home directory resolution

#### `GetConfigPath() string`
- Returns full path to `config.yaml`

#### `GetQueriesPath() string`
- Returns full path to `queries.yaml`

#### `EnsureConfigDir() error`
- Create `~/.config/meshbank/` if it doesn't exist
- Use `os.MkdirAll` with `0755` permissions

#### `LoadConfig() (*Config, error)`
- Call `EnsureConfigDir()` first
- If `config.yaml` doesn't exist: create it with empty `working_directory`, return config with defaults applied
- If exists: parse YAML, apply defaults for missing fields (`Port` = 2369, `MaxDatSize` = 1GB)
- Return loaded config

#### `SaveConfig(cfg *Config) error`
- Write only `working_directory` to YAML (port and max_dat_size only written if user explicitly set them)
- Actually, write all non-default fields. If `Port == DefaultPort`, omit it. If `MaxDatSize == DefaultMaxDatSize`, omit it.

#### `EnsureQueriesFile() error`
- If `queries.yaml` doesn't exist: create it with default content (see below)
- If exists: do nothing

#### `GetDefaultQueriesYAML() string`
- Return the full default queries.yaml content from `project/04-config.md`

### 3.3 Default queries.yaml Content
Embed the full content from `project/04-config.md` as a constant string. This includes:
- `topic_stats` section with: total_size, db_size, dat_size, file_count, avg_size, last_added, last_hash
- `presets` section with: recent-imports, by-hash, large-files, count, with-metadata, by-processor, lineage, derived

---

## Task 4: Debug Logger (`internal/logger/logger.go`)

Standard logging system for debugging and development. Outputs to stdout.

### 4.1 Log Levels
```go
const (
    LevelDebug = "DEBUG"
    LevelInfo  = "INFO"
    LevelWarn  = "WARN"
    LevelError = "ERROR"
)
```

### 4.2 Logger Struct
```go
type Logger struct {
    level  string // minimum level to output
    prefix string // optional prefix for all messages
}
```

### 4.3 Functions to Implement

#### `NewLogger(level string) *Logger`
- Initialize with given minimum log level
- Default to "DEBUG" if invalid level provided

#### `(l *Logger) Debug(format string, args ...interface{})`
- Log at DEBUG level if level threshold allows
- Format: `[DEBUG] 2024-01-15 10:30:45 | message`

#### `(l *Logger) Info(format string, args ...interface{})`
- Log at INFO level if level threshold allows
- Format: `[INFO]  2024-01-15 10:30:45 | message`

#### `(l *Logger) Warn(format string, args ...interface{})`
- Log at WARN level if level threshold allows
- Format: `[WARN]  2024-01-15 10:30:45 | message`

#### `(l *Logger) Error(format string, args ...interface{})`
- Log at ERROR level (always outputs)
- Format: `[ERROR] 2024-01-15 10:30:45 | message`

#### `(l *Logger) SetLevel(level string)`
- Change minimum log level at runtime

### 4.4 Usage
```go
log := logger.NewLogger(constants.DefaultLogLevel)
log.Debug("Loading config from %s", path)
log.Info("Server starting on port %d", port)
log.Warn("Working directory not set")
log.Error("Failed to open database: %v", err)
```

**Note:** User-facing notifications (corruption, missing files) will be implemented in Phase 8 as a separate in-memory notification system shown on the dashboard.

---

## Task 5: Working Directory Initialization

### 5.1 In `internal/config/config.go` or new file `internal/config/workdir.go`

#### `ValidateWorkingDirectory(path string) error`
- Check if directory exists using `os.Stat`
- If not exists: return error "directory does not exist"
- If not a directory: return error "path is not a directory"
- Return nil if valid

#### `InitializeWorkingDirectory(path string) error`
- Call `ValidateWorkingDirectory` first
- Create `.internal/` subdirectory if doesn't exist
- Create `orchestrator.db` if doesn't exist (just create empty file for now, schema in Phase 2)
- Return nil on success

#### `SetWorkingDirectory(cfg *Config, path string) error`
- Validate path
- Initialize working directory
- Update cfg.WorkingDirectory
- Save config
- Trigger topic discovery (call `DiscoverTopics`)

---

## Task 6: Topic Discovery (`internal/config/discovery.go` or `internal/database/discovery.go`)

### 6.1 TopicInfo Struct
```go
type TopicInfo struct {
    Name    string
    Path    string // full path to topic folder
    Healthy bool
    Error   string // if not healthy
}
```

### 6.2 Functions to Implement

#### `DiscoverTopics(workingDir string) ([]TopicInfo, error)`
- List all subdirectories in workingDir (excluding `.internal`)
- For each subdirectory:
  - Check if name matches `TopicNameRegex`
  - Check if `.internal/` subfolder exists
  - Check if `.internal/<name>.db` exists
  - If .internal/ and db exist: add as healthy topic (no .dat files is OK - means empty topic)
  - If partial (has .internal but missing db): mark unhealthy with error
  - If no .internal at all: skip (not a topic folder)
- Return list of discovered topics

#### `IndexTopicToOrchestrator(topicInfo TopicInfo, orchestratorDB string) error`
- Open topic's database
- Query all assets: `SELECT asset_id, blob_name FROM assets`
- For each asset: insert into orchestrator.db `asset_index` table
- Handle duplicates gracefully (INSERT OR IGNORE)
- This will be fully implemented in Phase 3, but define the function signature now

---

## Task 7: Main Entry Point (`cmd/meshbank/main.go`)

### 7.1 Startup Flow
```go
func main() {
    // 1. Initialize debug logger
    log := logger.NewLogger(constants.DefaultLogLevel)

    // 2. Load or create config
    log.Info("Loading configuration...")
    cfg, err := config.LoadConfig()
    if err != nil {
        log.Error("Failed to load config: %v", err)
        os.Exit(1)
    }
    log.Debug("Config directory: %s", config.GetConfigDir())

    // 3. Ensure queries.yaml exists with defaults
    if err := config.EnsureQueriesFile(); err != nil {
        log.Error("Failed to create queries.yaml: %v", err)
        os.Exit(1)
    }
    log.Debug("Queries file ready: %s", config.GetQueriesPath())

    // 4. If working_directory is set and valid, initialize it
    if cfg.WorkingDirectory != "" {
        log.Info("Initializing working directory: %s", cfg.WorkingDirectory)
        if err := config.InitializeWorkingDirectory(cfg.WorkingDirectory); err != nil {
            log.Error("Failed to initialize working directory: %v", err)
            // Don't exit - allow server to start, user can fix via dashboard
            cfg.WorkingDirectory = "" // Clear invalid path
        } else {
            // Discover existing topics
            topics, err := config.DiscoverTopics(cfg.WorkingDirectory)
            if err != nil {
                log.Warn("Topic discovery failed: %v", err)
            } else {
                log.Info("Discovered %d topic(s)", len(topics))
                for _, t := range topics {
                    if t.Healthy {
                        log.Debug("  - %s (healthy)", t.Name)
                    } else {
                        log.Warn("  - %s (unhealthy: %s)", t.Name, t.Error)
                    }
                }
                // Index topics to orchestrator (Phase 3 will implement fully)
            }
        }
    } else {
        log.Warn("Working directory not set - configure via dashboard")
    }

    // 5. Start HTTP server (Phase 4 will implement fully)
    port := cfg.Port
    if port == 0 {
        port = constants.DefaultPort
    }

    log.Info("Starting MeshBank server on port %d", port)

    // Placeholder: just keep running
    select {}
}
```

---

## Task 8: Dependencies

### 8.1 Add Required Dependencies
```bash
go get gopkg.in/yaml.v3
```

Note: `github.com/zeebo/blake3` and `github.com/mattn/go-sqlite3` will be added in Phase 2.

---

## Verification Checklist

After completing Phase 1, verify:

1. **`go build ./cmd/meshbank`** compiles without errors
2. **Running the binary**:
   - Creates `~/.config/meshbank/` directory
   - Creates `~/.config/meshbank/config.yaml` with empty working_directory
   - Creates `~/.config/meshbank/queries.yaml` with default presets
   - Prints startup messages indicating config directory and "working directory not set"
3. **If you manually set `working_directory` in config.yaml** to an existing folder:
   - Binary creates `.internal/` and `orchestrator.db` in that folder
   - Topic discovery runs (finds nothing in empty folder)
4. **Logger** correctly stores and retrieves log entries (write a simple test)

---

## Files to Create

| File | Status |
|------|--------|
| `cmd/meshbank/main.go` | Implement |
| `internal/constants/constants.go` | Implement |
| `internal/config/config.go` | Implement |
| `internal/config/workdir.go` | Implement |
| `internal/config/discovery.go` | Implement |
| `internal/config/queries_default.go` | Implement (embedded default YAML) |
| `internal/logger/logger.go` | Implement |
| `internal/storage/blob.go` | Placeholder (package declaration only) |
| `internal/storage/hash.go` | Placeholder |
| `internal/database/schema.go` | Placeholder |
| `internal/database/topic.go` | Placeholder |
| `internal/database/orchestrator.go` | Placeholder |
| `internal/database/metadata.go` | Placeholder |
| `internal/server/server.go` | Placeholder |
| `internal/server/app.go` | Placeholder |
| `internal/server/response.go` | Placeholder |
| `internal/server/handlers.go` | Placeholder |
| `internal/queries/parser.go` | Placeholder |
| `internal/queries/executor.go` | Placeholder |

---

## Notes for Agent

- Use `os.UserHomeDir()` for `~` expansion, not environment variable parsing
- All file operations should use `0755` for directories and `0644` for files
- Use `filepath.Join()` for all path construction (cross-platform)
- Topic discovery does NOT require .dat files - a newly created topic with only .internal/ and .db is valid
- Config YAML should use `omitempty` tags appropriately
- Logger must be thread-safe (multiple goroutines will log)
- Don't implement HTTP server yet - just the startup logic and `select{}` to keep running
