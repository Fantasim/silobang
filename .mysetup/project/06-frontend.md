# Frontend Dashboard

## Tech Stack
- VanillaJS (no framework)
- Tailwind CSS (bundled, no CDN)
- Embedded in Go binary via embed.FS
- Must work fully offline

## Routes/Pages

### 1. Setup Page (/)
Shown when config.yaml missing or no working_directory.

Elements:
- Folder path input
- "Set Working Directory" button
- Validation feedback

### 2. Dashboard Home (/)
Main view after config set.

Elements:
- Topic list with stats table
- Columns from queries.yaml `topic_stats`
- Health indicator per topic (green/red)
- "Create Topic" button
- Link to query runner
- Link to system logs

### 3. Topic Detail (/topic/:name)
View single topic.

Elements:
- Stats display (same as list but larger)
- Drag-drop file upload zone (multiple files supported)
- Upload progress (frontend handles sequential uploads):
  - Current file #/total
  - Added/skipped/error counts
  - Error list with details
- Optional parent_id field for lineage tracking

### 4. Query Runner (/query)
Execute preset queries.

Elements:
- Dropdown: select preset
- Dynamic param inputs based on selected preset
- Optional topic filter (multi-select)
- "Run Query" button
- Results table with columns from response
- "Export JSON" button
- Pagination controls

### 5. System Logs (/logs)
View error log list.

Elements:
- Table: timestamp, level, topic, message
- Expandable rows for details
- Auto-refresh toggle

## UI Components

### Topic Stats Table
| Name | Total Size | DB | DAT | Files | Avg | Last Added | Status |
|------|------------|-----|-----|-------|-----|------------|--------|
| game-1 | 5.2 GB | 1 MB | 5.1 GB | 12,500 | 420 KB | 2024-01-01 | ✓ |
| broken | - | - | - | - | - | - | ✗ |

### File Upload Zone
```
┌─────────────────────────────────────┐
│                                     │
│   Drag & drop files here            │
│   or click to browse                │
│                                     │
└─────────────────────────────────────┘

Uploading: 45/1000 (sequential via single-file API)
Added: 40 | Skipped: 3 | Errors: 2

Errors:
- file123.glb: exceeds max size
- file456.glb: read error
```

Note: Multiple file uploads are handled by the frontend, which calls the single-file upload API sequentially for each file.

### Query Results Table
- Sortable columns (client-side)
- Column types formatted per queries.yaml format spec
- Max rows from pagination constant

## Styling
- Light mode only (no dark mode)
- Clean, minimal
- Tailwind utility classes
- Responsive (works on smaller screens)

## API Integration
All data via fetch() to /api/* endpoints.
Handle loading states, errors gracefully.
