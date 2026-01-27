# Changelog

All notable changes to SiloBang will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0] - 2026-01-28

### Added
- Working directory storage bar on monitoring page — shows project size / configured limit with color-coded progress
- Configurable disk usage limit (`max_disk_usage` in config) — rejects uploads, metadata writes, and topic creation when exceeded (HTTP 507)
- Gzip compression middleware for large JSON API responses (>1KB, `/api/` routes)
- Pre-computed cache with ETag/304 support for `/api/schema` and `/api/prompts` endpoints
- Build-time brotli and gzip compression for static CSS/JS assets served from the embedded filesystem
- Custom static file handler with brotli-preferred, gzip-fallback, immutable cache headers, and ETag/304 conditional request support
- "Select All / Deselect All" toggle in the audit action filter dropdown
- "Recent Files" button on topic page — navigates to query page with `recent-imports` preset and topic pre-selected, auto-runs
- URL parameter support for the query page (`/query?preset=X&topics=Y&param=val`)

### Fixed
- Audit `CanViewAll` constraint now enforced: non-admin users with `can_view_all: false` are restricted to their own audit entries regardless of the requested filter
- Audit stream endpoint also enforces `CanViewAll` constraint for SSE connections
- Client-side router now correctly preserves URL query parameters during navigation (fixes "Recent Files" button)

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
