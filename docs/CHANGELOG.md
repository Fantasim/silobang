# Changelog

All notable changes to SiloBang will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0] - 2026-01-28

### Added
- In-memory stats cache with event-driven invalidation for topic and service-level metrics — eliminates redundant filesystem walks on polling requests
- New topic stats: extension breakdown, average metadata keys per file, recent DAT files list, DAT file count
- Storage usage percentage with color-coded thresholds (cyan/amber/red) on monitoring page
- Topic cards show file count and total size with share percentages and DetailTooltip breakdowns on hover
- DetailTooltip component for rich hover-triggered data panels with smart positioning
- Service overview section on monitoring page with Topics, Total DAT, Indexed Assets, and Total Storage cards
- Footer with clickable version linking to GitHub repository
- Relative time formatting for "last added" timestamps on topic cards
- Working directory storage bar on monitoring page — shows project size / configured limit with color-coded progress
- Configurable disk usage limit (`max_disk_usage` in config) — rejects uploads, metadata writes, and topic creation when exceeded (HTTP 507)
- Gzip compression middleware for large JSON API responses (>1KB, `/api/` routes)
- Pre-computed cache with ETag/304 support for `/api/schema` and `/api/prompts` endpoints
- Build-time brotli and gzip compression for static CSS/JS assets served from the embedded filesystem
- Custom static file handler with brotli-preferred, gzip-fallback, immutable cache headers, and ETag/304 conditional request support
- "Select All / Deselect All" toggle in the audit action filter dropdown
- "Recent Files" button on topic page — navigates to query page with `recent-imports` preset and topic pre-selected, auto-runs
- URL parameter support for the query page (`/query?preset=X&topics=Y&param=val`)
- Cached service info (topic/storage stats) included in `/api/monitoring` response — enables stats bar rendering without a separate API call
- Configuration section on monitoring page showing working directory, port, limits, and version info

### Fixed
- Query results DataTable: `size_range` column showing "NaN undefined" due to `formatBytes()` applied to text values
- `size-distribution` preset results now render correctly
- Cached service info now includes version info in `/api/topics` response (was missing when served from cache)
- Audit `CanViewAll` constraint now enforced: non-admin users with `can_view_all: false` are restricted to their own audit entries regardless of the requested filter
- Audit stream endpoint also enforces `CanViewAll` constraint for SSE connections
- Client-side router now correctly preserves URL query parameters during navigation (fixes "Recent Files" button)
- Reconcile service now evicts removed topics from the stats cache — prevents stale metrics from being served after orphaned topic cleanup
- Monitoring storage bar no longer shows "0 B" when `max_disk_usage` is not configured — displays used size only with "No limit set" subtitle
- Footer no longer shows "vdev" for dev builds — version label is hidden when not a release build
- Footer sticks to viewport bottom when page content is shorter than the screen

### Changed
- Dashboard page focused solely on topic management (service stats bar moved to monitoring)
- Topic cards redesigned: show name, file count with share %, total size with share %, relative "last added" timestamp
- Database Size and Orchestrator DB Size consolidated into Total Storage tooltip on monitoring page
- Topic card name uses larger, bolder typography for better visibility
- Topics search/filter toolbar hidden when fewer than 15 topics (configurable via `TOPICS_TOOLBAR_MIN_COUNT`)

### Removed
- "Searches all available topics" subtitle from the query page preset selector (redundant with topic chips)

## [0.1.0] - 2025-01-27

### Added
- Immutable asset storage with BLAKE3 content-addressable hashing
- SQLite-backed metadata and orchestration database
- Embedded Preact web dashboard
- Topic-based asset organization with per-topic databases
- Authentication system with API keys and session tokens
- Role-based access control with granular grants
- Bulk download with SSE progress streaming
- Batch metadata operations
- Custom query system with presets
- Audit logging with configurable retention
- Data verification and integrity checking
- Automatic reconciliation of orphaned entries
- Cross-platform release builds (Linux, macOS, Windows)
