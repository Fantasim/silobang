# Database Schemas

## Topic Database (<topic>.db)

### assets table
```sql
CREATE TABLE assets (
    asset_id TEXT PRIMARY KEY,     -- BLAKE3 hash (full)
    asset_size INTEGER NOT NULL,   -- bytes
    parent_id TEXT,                -- lineage (optional, explicit on single upload)
    extension TEXT NOT NULL,       -- file extension without dot (glb, png, obj)
    blob_name TEXT NOT NULL,       -- which .dat file (e.g., "003.dat")
    created_at INTEGER NOT NULL    -- unix timestamp
);

CREATE INDEX idx_assets_parent ON assets(parent_id);
CREATE INDEX idx_assets_created ON assets(created_at);
CREATE INDEX idx_assets_extension ON assets(extension);
```

### metadata_log table
```sql
CREATE TABLE metadata_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    asset_id TEXT NOT NULL,
    op TEXT NOT NULL,                    -- 'set' | 'delete'
    key TEXT NOT NULL,
    value_text TEXT,                     -- string value (auto-detect)
    value_num REAL,                      -- numeric value (auto-detect)
    processor TEXT NOT NULL,
    processor_version TEXT NOT NULL,
    timestamp INTEGER NOT NULL,          -- unix timestamp
    FOREIGN KEY (asset_id) REFERENCES assets(asset_id)
);

CREATE INDEX idx_metadata_asset ON metadata_log(asset_id);
CREATE INDEX idx_metadata_key ON metadata_log(key);
CREATE INDEX idx_metadata_processor ON metadata_log(processor);
```

### metadata_computed table
```sql
CREATE TABLE metadata_computed (
    asset_id TEXT PRIMARY KEY,
    metadata_json TEXT NOT NULL,   -- JSON object of all current key:value pairs
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (asset_id) REFERENCES assets(asset_id)
);
```

## Value Type Detection
- Try parse as number
- If conversion loses precision (trailing zeros, chars) -> store as text
- Otherwise store as num
- Both columns can be set (text as canonical, num for queries)

## Materialized View Updates
On each metadata_log insert:
1. Get current metadata_computed.metadata_json for asset
2. If op=set: merge key:value into object
3. If op=delete: remove key from object
4. Update metadata_computed row

## Orchestrator Database (orchestrator.db)

```sql
CREATE TABLE asset_index (
    hash TEXT PRIMARY KEY,
    topic TEXT NOT NULL,
    dat_file TEXT NOT NULL        -- e.g., "003.dat"
);

CREATE INDEX idx_asset_topic ON asset_index(topic);
```

Purpose: fast global lookup, deduplication check
