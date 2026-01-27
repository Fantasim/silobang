# Configuration Files

## Location
`~/.config/meshbank/`

## config.yaml
```yaml
working_directory: /path/to/meshbank-data
port: 2369
max_dat_size: 1073741824  # 1GB in bytes
```

All fields optional on first run. Dashboard prompts for working_directory if missing.

## queries.yaml
Controls: preset SQL queries + dashboard display customization

```yaml
# Topic stats shown on topic list page
topic_stats:
  - name: total_size
    label: "Total Size"
    sql: "SELECT SUM(asset_size) FROM assets"
    format: bytes  # bytes|number|date|text
  - name: db_size
    label: "DB Size"
    type: file_size  # special: reads .internal/<topic>.db file size
  - name: dat_size
    label: "DAT Size"
    type: dat_total  # special: sum of all .dat file sizes
  - name: file_count
    label: "Files"
    sql: "SELECT COUNT(*) FROM assets"
    format: number
  - name: avg_size
    label: "Avg Size"
    sql: "SELECT AVG(asset_size) FROM assets"
    format: bytes
  - name: last_added
    label: "Last Added"
    sql: "SELECT MAX(created_at) FROM assets"
    format: date
  - name: last_hash
    label: "Last Hash"
    sql: "SELECT asset_id FROM assets ORDER BY created_at DESC LIMIT 1"
    format: text

# Custom query presets for query runner
presets:
  recent-imports:
    description: "Models imported in last N days"
    sql: |
      SELECT asset_id, asset_size, created_at
      FROM assets
      WHERE created_at >= strftime('%s', 'now') - (:days * 86400)
      ORDER BY created_at DESC
      LIMIT :limit
    params:
      - name: days
        default: "7"
      - name: limit
        default: "1000"

  by-hash:
    description: "Find by hash prefix"
    sql: |
      SELECT asset_id, extension, asset_size, blob_name, created_at
      FROM assets
      WHERE asset_id LIKE :hash || '%'
      LIMIT 10
    params:
      - name: hash
        required: true

  large-files:
    description: "Files larger than N bytes"
    sql: |
      SELECT asset_id, extension, asset_size, created_at
      FROM assets
      WHERE asset_size > :min_size
      ORDER BY asset_size DESC
      LIMIT :limit
    params:
      - name: min_size
        default: "10000000"
      - name: limit
        default: "20"

  count:
    description: "Total file count"
    sql: "SELECT COUNT(*) as count FROM assets"

  with-metadata:
    description: "Assets with specific metadata key"
    sql: |
      SELECT a.asset_id, a.extension, mc.metadata_json
      FROM assets a
      JOIN metadata_computed mc ON a.asset_id = mc.asset_id
      WHERE mc.metadata_json LIKE '%' || :key || '%'
      LIMIT :limit
    params:
      - name: key
        required: true
      - name: limit
        default: "100"

  by-processor:
    description: "Metadata from specific processor"
    sql: |
      SELECT asset_id, key, value_text, value_num, processor_version, timestamp
      FROM metadata_log
      WHERE processor = :processor
      ORDER BY timestamp DESC
      LIMIT :limit
    params:
      - name: processor
        required: true
      - name: limit
        default: "100"

  lineage:
    description: "Get asset lineage chain"
    sql: |
      WITH RECURSIVE chain AS (
        SELECT asset_id, parent_id, extension, created_at, 0 as depth
        FROM assets WHERE asset_id = :hash
        UNION ALL
        SELECT a.asset_id, a.parent_id, a.extension, a.created_at, c.depth + 1
        FROM assets a JOIN chain c ON a.asset_id = c.parent_id
      )
      SELECT * FROM chain ORDER BY depth
    params:
      - name: hash
        required: true

  derived:
    description: "All versions derived from asset"
    sql: |
      WITH RECURSIVE descendants AS (
        SELECT asset_id, parent_id, extension, created_at
        FROM assets WHERE parent_id = :hash
        UNION ALL
        SELECT a.asset_id, a.parent_id, a.extension, a.created_at
        FROM assets a JOIN descendants d ON a.parent_id = d.asset_id
      )
      SELECT * FROM descendants
    params:
      - name: hash
        required: true
```

## Cross-Topic Queries
- Presets run against all topics by default
- Results aggregated in topic creation order
- Each result row includes implicit `_topic` field

## Format Types
- `bytes`: human readable (KB, MB, GB)
- `number`: plain number with comma separators
- `date`: unix timestamp -> readable date
- `text`: raw string
