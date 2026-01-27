# Blob Format (.dat files)

## File Structure
Each .dat contains multiple assets concatenated. Max size configurable (default 1GB).

## Entry Header Format (per asset)
```
Offset  Size    Field
------  ----    -----
0       4       Magic bytes: "MSHB" (0x4D534842)
4       2       Version: uint16 (current: 1)
6       8       Offset to data start: uint64 (from file start)
14      8       Data length: uint64 (bytes)
22      64      BLAKE3 hash: 64 bytes (full hash)
86      32      Reserved: 32 bytes (future use, zero-filled)
118     N       Asset data: raw bytes (no compression)
```

Total header size: 118 bytes per entry

## Reading Single Asset
1. Look up hash in orchestrator.db -> get topic + dat_file
2. Look up hash in topic.db assets table -> get blob_name (confirms dat_file)
3. Open dat_file
4. Scan headers sequentially until hash matches
5. Seek to offset, read data_length bytes

## Writing New Asset
1. Compute BLAKE3 hash
2. Check orchestrator.db for duplicate -> skip if exists
3. Find current .dat file for topic
4. If current .dat + new asset > max_dat_size -> create next .dat (NNN+1)
5. Append header + data to .dat
6. Update mapping.json with new .dat hash
7. Insert into assets table
8. Insert into orchestrator.db

## .dat Naming
- Sequential: 001.dat, 002.dat, ..., 999.dat, 1000.dat
- Zero-padded to 3 digits minimum

## mapping.json Format
```json
{
  "001.dat": "<blake3_hash_of_entire_001.dat>",
  "002.dat": "<blake3_hash_of_entire_002.dat>"
}
```

Recomputed after each append to any .dat.

## Max Asset Size
Single asset cannot exceed: max_dat_size - header_size (118 bytes)
If asset larger -> reject with error

## No Compression
Assets stored raw. User can pre-compress if needed.
