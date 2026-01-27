# Implementation Order

## Phase 1: Foundation
1. `go mod init meshbank`
2. Project structure:
   ```
   cmd/meshbank/main.go
   internal/
       constants/constants.go
       config/config.go
       storage/
           blob.go      # .dat read/write
           hash.go      # BLAKE3 helpers
           dat.go       # .dat file management
       database/
           schema.go    # table creation
           topic.go     # topic DB operations
           orchestrator.go
           metadata.go
       server/
           server.go
           routes.go
           handlers/
       queries/
           parser.go    # queries.yaml parsing
           executor.go  # run queries across topics
   web/
       static/          # embedded assets
       templates/
   e2e/
   ```

3. Constants file
4. Config loading (yaml, home dir resolution)

## Phase 2: Database Layer
1. SQLite connection + pragmas
2. Schema creation (assets, metadata_log, metadata_computed, dat_hashes)
3. Orchestrator DB schema
4. Topic discovery on startup
5. Asset insert/lookup
6. Metadata log insert + computed update
7. Value type detection

## Phase 3: Storage Layer (Blob Format)
1. BLAKE3 hashing utilities
2. Blob header struct + read/write
3. .dat file management (create, append, read entry)
4. DAT file listing and management utilities

## Phase 4: Core API
1. HTTP server setup
2. GET/POST /config
3. GET/POST /topics
4. POST /topics/:name/assets (single upload)
5. GET /assets/:hash/download
6. POST /assets/:hash/metadata

## Phase 5: Query System
1. queries.yaml parser
2. Preset listing endpoint
3. Query executor (single topic)
4. Cross-topic aggregation
5. Parameter binding

## Phase 6: Frontend
1. Tailwind bundling
2. embed.FS setup
3. Setup page (config form)
4. Dashboard (topic list + stats)
5. Topic detail page
6. File upload with progress (frontend loops over files, calls single-file API)
7. Query runner page
8. Logs page

## Phase 7: Polish
1. Corruption detection on startup
2. System log collection
3. Error handling cleanup
4. Cross-platform path handling
5. Final E2E tests

## Dependencies
```
github.com/zeebo/blake3
github.com/mattn/go-sqlite3
gopkg.in/yaml.v3
```

## Build
```bash
go build -o meshbank ./cmd/meshbank
```

Embeds web/ folder. Single binary output.
