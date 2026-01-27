# Phase 2: Database Layer

## Objective
Implement all SQLite database operations: schema creation, topic databases, orchestrator database, and the atomic write pipeline for assets.

**Important change from original spec:** `mapping.json` is replaced by a `dat_hashes` table in each topic database. This allows atomic transactions when writing assets.

---

## Task 1: Dependencies

### 1.1 Add SQLite and BLAKE3
```bash
go get github.com/mattn/go-sqlite3
go get github.com/zeebo/blake3
```

Note: `go-sqlite3` requires CGO. Ensure `CGO_ENABLED=1` is set.

---

## Task 2: Database Schema (`internal/database/schema.go`)

### 2.1 Topic Database Schema

```sql
-- assets table
CREATE TABLE IF NOT EXISTS assets (
    asset_id TEXT PRIMARY KEY,     -- BLAKE3 hash (64 hex chars)
    asset_size INTEGER NOT NULL,   -- bytes
    origin_name TEXT,              -- original filename without extension and dot
    parent_id TEXT,                -- lineage (optional)
    extension TEXT NOT NULL,       -- file extension without dot
    blob_name TEXT NOT NULL,       -- which .dat file (e.g., "003.dat")
    byte_offset INTEGER NOT NULL,  -- offset in .dat file for O(1) lookup
    created_at INTEGER NOT NULL    -- unix timestamp
);

CREATE INDEX IF NOT EXISTS idx_assets_parent ON assets(parent_id);
CREATE INDEX IF NOT EXISTS idx_assets_created ON assets(created_at);
CREATE INDEX IF NOT EXISTS idx_assets_extension ON assets(extension);
CREATE INDEX IF NOT EXISTS idx_assets_origin_name ON assets(origin_name);

-- metadata_log table (append-only)
CREATE TABLE IF NOT EXISTS metadata_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    asset_id TEXT NOT NULL,
    op TEXT NOT NULL,                    -- 'set' | 'delete'
    key TEXT NOT NULL,
    value_text TEXT,                     -- string value
    value_num REAL,                      -- numeric value (if applicable)
    processor TEXT NOT NULL,
    processor_version TEXT NOT NULL,
    timestamp INTEGER NOT NULL,
    FOREIGN KEY (asset_id) REFERENCES assets(asset_id)
);

CREATE INDEX IF NOT EXISTS idx_metadata_asset ON metadata_log(asset_id);
CREATE INDEX IF NOT EXISTS idx_metadata_key ON metadata_log(key);
CREATE INDEX IF NOT EXISTS idx_metadata_processor ON metadata_log(processor);

-- metadata_computed table (materialized view)
CREATE TABLE IF NOT EXISTS metadata_computed (
    asset_id TEXT PRIMARY KEY,
    metadata_json TEXT NOT NULL,   -- JSON object of current key:value pairs
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (asset_id) REFERENCES assets(asset_id)
);

-- dat_hashes table (replaces mapping.json)
CREATE TABLE IF NOT EXISTS dat_hashes (
    dat_file TEXT PRIMARY KEY,     -- e.g., "001.dat"
    blake3_hash TEXT NOT NULL,     -- hash of entire .dat file
    updated_at INTEGER NOT NULL    -- unix timestamp
);
```

### 2.2 Orchestrator Database Schema

```sql
CREATE TABLE IF NOT EXISTS asset_index (
    hash TEXT PRIMARY KEY,
    topic TEXT NOT NULL,
    dat_file TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_asset_topic ON asset_index(topic);
```

### 2.3 Functions to Implement

#### `GetTopicSchema() string`
- Returns the full SQL schema for topic databases

#### `GetOrchestratorSchema() string`
- Returns the full SQL schema for orchestrator.db

#### `ApplyPragmas(db *sql.DB) error`
- Apply all SQLite pragmas from `constants.SQLitePragmas`
- Must be called immediately after opening any database connection

---

## Task 3: Database Connections (`internal/database/connection.go`)

### 3.1 Functions to Implement

#### `OpenDatabase(path string) (*sql.DB, error)`
- Open SQLite database at given path
- Apply pragmas via `ApplyPragmas()`
- Return configured connection

#### `InitTopicDB(path string) (*sql.DB, error)`
- Call `OpenDatabase()`
- Execute `GetTopicSchema()` to create tables if not exist
- Return connection

#### `InitOrchestratorDB(path string) (*sql.DB, error)`
- Call `OpenDatabase()`
- Execute `GetOrchestratorSchema()` to create tables if not exist
- Return connection

---

## Task 4: Orchestrator Operations (`internal/database/orchestrator.go`)

### 4.1 Functions to Implement

#### `CheckHashExists(db *sql.DB, hash string) (exists bool, topic string, datFile string, err error)`
- Query `asset_index` for given hash
- Return whether it exists and where (topic + dat_file)

#### `InsertAssetIndex(tx *sql.Tx, hash, topic, datFile string) error`
- Insert into `asset_index` table
- Use provided transaction (for atomic writes)

#### `InsertAssetIndexIgnore(db *sql.DB, hash, topic, datFile string) error`
- `INSERT OR IGNORE` - for re-indexing discovered topics
- Uses its own transaction (not part of write pipeline)

#### `DeleteAssetIndex(tx *sql.Tx, hash string) error`
- Delete from `asset_index` (for future use when deletion is supported)

---

## Task 5: Topic Database Operations (`internal/database/topic.go`)

### 5.1 Functions to Implement

#### `InsertAsset(tx *sql.Tx, asset Asset) error`
- Insert into `assets` table
- Use provided transaction

#### `GetAsset(db *sql.DB, assetID string) (*Asset, error)`
- Query single asset by hash

#### `GetAssetsByParent(db *sql.DB, parentID string) ([]Asset, error)`
- Query all assets with given parent_id

#### `ValidateParentExists(orchestratorDB *sql.DB, parentID string) error`
- Check if parent_id exists in ANY topic via orchestrator.db
- Return error if not found

**Note:** `GetCurrentDatFile` and `GetNextDatFile` are implemented in `internal/storage/dat.go` (Phase 3) using filesystem operations, not database queries. This is more reliable since it checks actual .dat files on disk.

### 5.2 Asset Struct
```go
type Asset struct {
    AssetID    string  // BLAKE3 hash (64 hex chars)
    AssetSize  int64   // bytes
    OriginName string  // original filename without extension
    ParentID   *string // nullable, for lineage
    Extension  string  // file extension without dot
    BlobName   string  // which .dat file (e.g., "003.dat")
    ByteOffset int64   // offset in .dat file for O(1) lookup
    CreatedAt  int64   // unix timestamp
}
```

---

## Task 6: DAT Hash Operations (`internal/database/dathash.go`)

### 6.1 Functions to Implement

#### `GetDatHash(db *sql.DB, datFile string) (string, error)`
- Query `dat_hashes` for given dat file
- Return stored hash or error if not found

#### `UpdateDatHash(tx *sql.Tx, datFile, hash string) error`
- `INSERT OR REPLACE` into `dat_hashes`
- Use provided transaction (for atomic writes)

#### `VerifyDatHash(db *sql.DB, datFile, topicPath string) (bool, error)`
- Get stored hash from DB
- Compute actual hash of .dat file
- Return true if match, false if mismatch

#### `VerifyAllDatHashes(db *sql.DB, topicPath string) ([]string, error)`
- Verify all .dat files in topic
- Return list of mismatched files (empty = all good)

---

## Task 7: Metadata Operations (`internal/database/metadata.go`)

### 7.1 Value Type Detection

```go
func DetectValueType(value string) (text string, num *float64, err error) {
    // Rules:
    // 1. Empty string -> error (reject)
    // 2. Try parse as float64
    // 3. If parse succeeds:
    //    - Check if string representation matches original
    //    - "1.0" -> text only (trailing zero)
    //    - "01234" -> text only (leading zero)
    //    - "1e10" -> text only (scientific notation)
    //    - "123" -> both text="123" and num=123.0
    //    - "123.45" -> both text="123.45" and num=123.45
    // 4. If parse fails -> text only
    // 5. "true"/"false" -> text only
}
```

**Detailed rules:**
- Empty string `""` → reject with error
- Leading zeros (`"0123"`) → text only
- Trailing zeros after decimal (`"1.0"`, `"1.00"`) → text only
- Scientific notation (`"1e10"`, `"1E5"`) → text only
- Boolean strings (`"true"`, `"false"`) → text only
- Valid integers (`"123"`, `"-45"`, `"0"`) → both text and num
- Valid decimals (`"123.45"`, `-0.5"`) → both text and num
- Exceeds float64 precision → text only
- Everything else → text only

### 7.2 Functions to Implement

#### `InsertMetadataLog(db *sql.DB, entry MetadataLogEntry) (int64, error)`
- Insert into `metadata_log`
- Call `DetectValueType()` to populate value_text and value_num
- Call `UpdateMetadataComputed()` after insert
- Return log entry ID

#### `UpdateMetadataComputed(db *sql.DB, assetID string) error`
- Rebuild `metadata_computed` for given asset
- Query all `metadata_log` entries for asset, ordered by id
- Apply ops in order: set adds/updates key, delete removes key
- Store result as JSON in `metadata_computed`
- Values in JSON should be typed (number vs string based on detection)

#### `GetMetadataComputed(db *sql.DB, assetID string) (map[string]interface{}, error)`
- Query `metadata_computed` and parse JSON
- Return nil if no metadata exists (no row)

#### `GetMetadataLog(db *sql.DB, assetID string) ([]MetadataLogEntry, error)`
- Query all log entries for asset

### 7.3 Structs
```go
type MetadataLogEntry struct {
    ID               int64
    AssetID          string
    Op               string // "set" | "delete"
    Key              string
    Value            string // original value (for set)
    Processor        string
    ProcessorVersion string
    Timestamp        int64
}
```

---

## Task 8: Error Types (`internal/database/errors.go`)

**Note:** The atomic write pipeline is NOT implemented here. Phase 4 implements a streaming version in the HTTP handlers that never loads entire files into memory. This phase only provides the building blocks (InsertAsset, InsertAssetIndex, UpdateDatHash, etc.) that Phase 4 combines with streaming I/O.

### 8.1 Error Types
```go
type DuplicateAssetError struct {
    Hash          string
    ExistingTopic string
}

func (e *DuplicateAssetError) Error() string {
    return fmt.Sprintf("asset %s already exists in topic %s", e.Hash, e.ExistingTopic)
}
```

---

## Task 9: Topic Discovery Integration

Update the discovery logic from Phase 1 to use real database operations.

### 9.1 Update `DiscoverTopics`
- After finding valid topics, verify dat hashes using `VerifyAllDatHashes()`
- If any mismatch: mark topic as unhealthy with specific error

### 9.2 Update `IndexTopicToOrchestrator`
```go
func IndexTopicToOrchestrator(topicPath string, topicName string, orchestratorDB *sql.DB) error {
    // Open topic database
    topicDBPath := filepath.Join(topicPath, constants.InternalDir, topicName+".db")
    topicDB, err := OpenDatabase(topicDBPath)
    if err != nil {
        return err
    }
    defer topicDB.Close()

    // Query all assets
    rows, err := topicDB.Query("SELECT asset_id, blob_name FROM assets")
    if err != nil {
        return err
    }
    defer rows.Close()

    // Index each asset (INSERT OR IGNORE for duplicates)
    for rows.Next() {
        var hash, datFile string
        if err := rows.Scan(&hash, &datFile); err != nil {
            return err
        }

        // INSERT OR IGNORE - first topic wins
        if err := InsertAssetIndexIgnore(orchestratorDB, hash, topicName, datFile); err != nil {
            return err
        }
    }

    return rows.Err()
}
```

---

## Task 10: Hash Utilities (`internal/storage/hash.go`)

### 10.1 Functions to Implement

#### `ComputeBlake3(data []byte) []byte`
- Compute BLAKE3 hash of byte slice
- Return raw 32 bytes

#### `ComputeBlake3Hex(data []byte) string`
- Compute BLAKE3 hash and return as 64-char hex string

#### `ComputeFileBlake3(path string) ([]byte, error)`
- Compute BLAKE3 hash of file contents
- Stream file to avoid loading entirely in memory

#### `ComputeFileBlake3Hex(path string) (string, error)`
- Same as above but returns hex string

---

## Verification Checklist

After completing Phase 2, verify:

1. **Schema creation:**
   - Create a test topic database, verify all tables exist
   - Create orchestrator.db, verify `asset_index` table exists

2. **Value type detection:**
   - `"123"` → text="123", num=123.0
   - `"1.0"` → text="1.0", num=nil
   - `"0123"` → text="0123", num=nil
   - `""` → error
   - `"hello"` → text="hello", num=nil

3. **Database operations (building blocks for Phase 4 pipeline):**
   - Insert an asset record into topic.db
   - Insert into orchestrator.db asset_index
   - Update dat_hashes table
   - Test DuplicateAssetError detection via CheckHashExists

4. **Topic discovery:**
   - Create a topic manually, restart, verify it's indexed

5. **Corruption detection:**
   - Manually corrupt a .dat file, verify `VerifyDatHash` returns false

---

## Files to Create/Update

| File | Status |
|------|--------|
| `internal/database/schema.go` | Implement |
| `internal/database/connection.go` | Implement |
| `internal/database/orchestrator.go` | Implement |
| `internal/database/topic.go` | Implement |
| `internal/database/dathash.go` | Implement |
| `internal/database/metadata.go` | Implement |
| `internal/database/errors.go` | Implement |
| `internal/storage/hash.go` | Implement |
| `internal/config/discovery.go` | Update (use real DB operations) |

---

## Notes for Agent

- All database operations must use `sql.Tx` when part of the write pipeline
- Always use `defer tx.Rollback()` - it's a no-op after commit
- The pipeline order: file I/O first (to get byte_offset), then DB inserts, then commit
- `INSERT OR IGNORE` is only for re-indexing, never for normal writes
- Parent validation checks orchestrator.db (cross-topic)
- Value type detection is strict: when in doubt, store as text only
- The .dat file may have orphan bytes if commit fails - this is acceptable
- File hashing should stream for large files (don't load entire .dat in memory)
- **Streaming:** The write pipeline is implemented in Phase 4's HTTP handlers to support streaming (100GB+ files on 2GB RAM)
- **Header size:** Uses `constants.HeaderSize` (110 bytes) - ensure Phase 1 constants are correct
- **Low memory:** SQLite cache_size reduced to 8MB per connection for low memory environments