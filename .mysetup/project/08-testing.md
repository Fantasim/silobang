# Testing Strategy

## Philosophy
- No unit tests for trivial utils (waste of time)
- Focus on E2E tests that verify real workflows
- Test edge cases: thresholds, corruption, multiple uploads

## E2E Test Scenarios

### 1. Fresh Start Flow
```
1. No config.yaml exists
2. Start server
3. API returns configured=false
4. POST /config with working_directory
5. Config file created
6. Topics list empty
```

### 2. Topic Management
```
1. POST /topics "test-topic" -> success
2. POST /topics "test-topic" -> 409 conflict
3. POST /topics "INVALID NAME" -> 400 validation
4. GET /topics -> contains test-topic with zero stats
```

### 3. Single Asset Upload
```
1. Create topic
2. POST file to /topics/:name/assets
3. Verify hash returned
4. Verify 001.dat created
5. Verify assets table entry
6. Verify orchestrator.db entry
7. GET /assets/:hash/download -> matches original
```

### 4. Duplicate Detection
```
1. Upload file A to topic-1
2. Upload same file A to topic-1 -> skipped
3. Upload same file A to topic-2 -> skipped (global dedup)
4. Verify only one entry in orchestrator.db
```

### 5. Multiple Sequential Uploads
```
1. Create 10 test files (varied sizes)
2. Upload each file sequentially via single-file API
3. Verify all hashes returned correctly
4. Verify all files accessible via download
5. Verify correct deduplication (upload same file twice -> skipped)
```

### 6. DAT Threshold
```
1. Set max_dat_size to 1MB for test
2. Upload 500KB file -> goes to 001.dat
3. Upload another 500KB file -> goes to 001.dat
4. Upload another 500KB file -> creates 002.dat
5. Verify dat_hashes table updated
```

### 7. Max File Size
```
1. Set max_dat_size to 1MB
2. Try upload 2MB file -> error (exceeds limit)
3. Verify no partial writes
```

### 8. Metadata Operations
```
1. Upload asset
2. POST metadata set key="polycount" value=1000
3. Verify metadata_log entry
4. Verify metadata_computed updated
5. POST metadata set key="has_skeleton" value=true
6. Verify metadata_computed has both keys
7. POST metadata delete key="polycount"
8. Verify metadata_computed only has has_skeleton
```

### 9. Lineage Tracking
```
1. Upload file A (no parent)
2. Upload file B with parent_id=A.hash
3. Run lineage query for B -> returns A,B chain
4. Run derived query for A -> returns B
```

### 10. Query Execution
```
1. Upload 50 files
2. Add metadata to some
3. Run various presets (recent-imports, large-files, with-metadata)
4. Verify correct filtering and results
```

### 11. Cross-Topic Queries
```
1. Create topic-1 with 10 files
2. Create topic-2 with 10 files
3. Run query without topic filter -> 20 results
4. Run query with topics=["topic-1"] -> 10 results
5. Verify _topic column in results
```

### 12. Topic Discovery
```
1. Create topic via API
2. Restart server
3. Topic auto-discovered with correct stats
```

### 13. Portable Topic
```
1. Create topic, add files
2. Copy topic folder to new location
3. Point new server at that location
4. Topic discovered, files accessible
```

### 14. Corruption Detection
```
1. Create topic, add files
2. Manually corrupt a .dat file
3. Restart server
4. Topic marked unhealthy
5. Error in /logs
6. Queries to that topic fail
```

### 15. Value Type Detection
```
1. POST metadata value="123" -> stored as num
2. POST metadata value="123.45" -> stored as num
3. POST metadata value="00123" -> stored as text (leading zero)
4. POST metadata value="hello" -> stored as text
5. Verify queries work on both types
```

## Test Fixtures
- Generate test GLB files (small, valid headers)
- Various sizes: 1KB, 100KB, 1MB, 10MB
- Corrupted files for error testing

## CI Integration
```bash
go test ./e2e/... -v
```

## Temp Directory
All E2E tests use temp directories. Cleanup after each test.
