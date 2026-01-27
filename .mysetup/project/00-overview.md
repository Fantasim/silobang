# MeshBank Overview

## Purpose
Content-addressed asset bank for millions of files (GLB first, generic later). Immutable blobs + versioned metadata logs.

## Core Principles
- **Immutable storage**: files never modified/deleted, new hash = new file
- **Append-only metadata**: log-based, recomputable materialized state
- **Multi-topic**: each topic = isolated folder + SQLite DB
- **Cross-platform**: Go backend, VanillaJS frontend, works offline
- **Single writer, multiple readers**

## Tech Stack
- **Backend**: Go only
- **Frontend**: VanillaJS + Tailwind (embedded in binary, no CDN)
- **Database**: SQLite per topic + orchestrator.db
- **Hashing**: BLAKE3 (full length)
- **API**: REST + JSON
- **Auth**: local-only (structure ready for future auth)

## Key Features
1. Store assets in packed .dat files (max 1GB default, configurable)
2. Track metadata via append-only log table
3. Materialized metadata_computed updated on each log entry
4. Lineage tracking via parent_id
5. Processor version tracking for reproducibility
6. Custom queries via queries.yaml
7. Global hash deduplication across all topics
8. Corruption detection via mapping.json hash verification

## Default Port
2369 (configurable in config.yaml)
