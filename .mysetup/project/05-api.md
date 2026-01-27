# REST API

Base URL: `http://localhost:2369/api`

## Configuration

### GET /config
Get current config status.

Response:
```json
{
  "configured": true,
  "working_directory": "/path/to/data",
  "port": 2369,
  "max_dat_size": 1073741824
}
```

### POST /config
Set/update configuration.

Request:
```json
{
  "working_directory": "/path/to/data"
}
```

Response:
```json
{
  "success": true
}
```

---

## Topics

### GET /topics
List all topics with stats (from queries.yaml topic_stats).

Response:
```json
{
  "topics": [
    {
      "name": "game-assets",
      "stats": {
        "total_size": 5368709120,
        "db_size": 1048576,
        "dat_size": 5367660544,
        "file_count": 12500,
        "avg_size": 429496,
        "last_added": 1704067200,
        "last_hash": "a3f2..."
      },
      "healthy": true
    },
    {
      "name": "corrupted-topic",
      "healthy": false,
      "error": "mapping.json hash mismatch for 002.dat"
    }
  ]
}
```

### POST /topics
Create new topic.

Request:
```json
{
  "name": "new-topic"
}
```

Validation: lowercase, numbers, "-", "_" only.

Response:
```json
{
  "success": true,
  "name": "new-topic"
}
```

---

## Assets

### POST /topics/:name/assets
Upload single asset (multipart form).

Form fields:
- `file`: binary file data (required)
- `parent_id`: BLAKE3 hash string (optional, for lineage)

Response:
```json
{
  "success": true,
  "hash": "a3f2c8...",
  "size": 1048576,
  "blob": "003.dat",
  "skipped": false
}
```

If duplicate:
```json
{
  "success": true,
  "hash": "a3f2c8...",
  "skipped": true,
  "existing_topic": "other-topic"
}
```

### GET /assets/:hash/download
Download asset by hash.

Response: raw binary file with appropriate Content-Type header based on extension.

---

## Metadata

### POST /assets/:hash/metadata
Add/delete metadata log entry.

Request:
```json
{
  "op": "set",
  "key": "polycount",
  "value": 12500,
  "processor": "mesh_analyzer",
  "processor_version": "1.0"
}
```

For delete:
```json
{
  "op": "delete",
  "key": "polycount",
  "processor": "mesh_analyzer",
  "processor_version": "1.0"
}
```

Response:
```json
{
  "success": true,
  "log_id": 12345,
  "computed_metadata": {
    "polycount": 12500,
    "has_skeleton": true
  }
}
```

---

## Queries

### GET /queries
List available query presets.

Response:
```json
{
  "presets": [
    {
      "name": "recent-imports",
      "description": "Models imported in last N days",
      "params": [
        {"name": "days", "required": false, "default": "7"},
        {"name": "limit", "required": false, "default": "1000"}
      ]
    },
    {
      "name": "by-hash",
      "description": "Find by hash prefix",
      "params": [
        {"name": "hash", "required": true}
      ]
    }
  ]
}
```

### POST /query/:preset
Run preset query.

Request:
```json
{
  "params": {
    "days": "30",
    "limit": "500"
  },
  "topics": ["game-assets"]  // optional, defaults to all
}
```

Response:
```json
{
  "preset": "recent-imports",
  "row_count": 245,
  "columns": ["asset_id", "asset_size", "created_at", "_topic"],
  "rows": [
    ["a3f2...", 1048576, 1704067200, "game-assets"],
    ["b4c3...", 2097152, 1704067100, "game-assets"]
  ]
}
```

---

## System

### GET /logs
Get system error logs (corruption, missing files, etc).

Response:
```json
{
  "logs": [
    {
      "timestamp": 1704067200,
      "level": "error",
      "message": "mapping.json hash mismatch",
      "topic": "broken-topic",
      "details": {"file": "002.dat", "expected": "abc...", "got": "def..."}
    }
  ]
}
```

---

## Error Responses
All errors return:
```json
{
  "error": true,
  "message": "Human readable error",
  "code": "ERROR_CODE"
}
```

HTTP status codes:
- 400: bad request (validation)
- 404: not found
- 409: conflict (topic exists)
- 500: internal error
