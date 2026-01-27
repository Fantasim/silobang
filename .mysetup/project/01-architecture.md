# Architecture

## Folder Structure

```
~/.config/meshbank/
    config.yaml          # global config (working_dir, port, max_dat_size)
    queries.yaml         # custom queries + dashboard display config

<working_directory>/
    .internal/
        orchestrator.db  # global hash index: hash -> topic -> dat_file

    <topic_name>/        # lowercase, numbers, "-", "_" only
        .internal/
            mapping.json # hash of each .dat for corruption detection
            <topic>.db   # SQLite: assets, metadata_log, metadata_computed
        001.dat
        002.dat
        ...
```

## Topic Discovery
On startup:
1. Check ~/.config/meshbank/config.yaml exists
2. If not: wait for user to set working_directory via dashboard
3. If exists: scan working_directory for valid topic folders
4. Valid topic = has `.internal/` with db + at least one .dat file matching `NNN.dat` pattern
5. For each topic: query db for stats (total_size, file_count, dat_count, avg_size, last_added)

## Orchestrator.db Purpose
- Global hash lookup: hash_id -> topic_name -> dat_file
- Prevents duplicates across entire bank
- If hash exists anywhere -> skip on upload

## Topic Isolation
- Each topic fully self-contained
- Can zip topic folder, move to another machine
- New orchestrator will discover and index on startup

## Corruption Handling
- mapping.json stores hash of each .dat file content
- On read: verify hash matches
- If mismatch: disable topic entirely
- Log error to backend log list (shown on dashboard)
- User must fix manually + restart server

## Concurrency Model
- Single writer process
- Multiple readers allowed
- SQLite WAL mode for read concurrency
