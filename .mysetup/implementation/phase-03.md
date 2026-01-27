# Phase 3: Blob Format (.dat Files)

## Objective
Implement the binary .dat file format for storing assets. This phase creates the low-level blob read/write functions that will be used by the write pipeline (Phase 4).

**Key design decision:** Use `byte_offset` column in SQLite `assets` table for O(1) asset lookup instead of sequential header scanning or in-file index.

---

## Prerequisites
- Phase 1 completed (project structure, constants, config, logger)
- Phase 2 completed (database schema, connections, hash utilities)

---

## Task 1: Constants Updates (`internal/constants/constants.go`)

### 1.1 Update Header Size

The revised header format (removing redundant offset field):

```
Offset  Size    Field
------  ----    -----
0       4       Magic bytes: "MSHB" (0x4D534842)
4       2       Version: uint16 little-endian (current: 1)
6       8       Data length: uint64 little-endian (bytes)
14      64      BLAKE3 hash: 64 ASCII hex characters
78      32      Reserved: 32 bytes (zero-filled)
110     N       Asset data: raw bytes
```

**Total header size: 110 bytes** (reduced from 118 by removing the 8-byte offset field)

Update constants:
```go
const (
    HeaderSize          = 110  // Changed from 118
    MagicBytesOffset    = 0
    MagicBytesSize      = 4
    VersionOffset       = 4
    VersionSize         = 2
    DataLengthOffset    = 6
    DataLengthSize      = 8
    HashOffset          = 14
    HashSize            = 64   // 64 ASCII hex chars
    ReservedOffset      = 78
    ReservedHeaderBytes = 32
    DataOffset          = 110  // Where actual data begins (relative to entry start)
)

var MagicBytes = []byte("MSHB")

const BlobVersion = uint16(1)
```

---

## Task 2: Blob Entry Struct (`internal/storage/blob.go`)

### 2.1 Define Entry Header Struct

```go
package storage

import (
    "bytes"
    "encoding/binary"
    "errors"
    "fmt"
    "io"
    "os"

    "meshbank/internal/constants"
)

// BlobEntry represents a single asset entry in a .dat file
type BlobEntry struct {
    Magic      [4]byte  // "MSHB"
    Version    uint16   // Format version (little-endian)
    DataLength uint64   // Size of asset data (little-endian)
    Hash       string   // 64 hex characters
    Reserved   [32]byte // Future use, zero-filled
    Data       []byte   // Actual asset bytes (not stored in header struct)
}

var (
    ErrInvalidMagic   = errors.New("invalid magic bytes: expected MSHB")
    ErrInvalidVersion = errors.New("unsupported blob version")
    ErrInvalidHash    = errors.New("invalid hash format")
    ErrDataTooLarge   = errors.New("data exceeds maximum allowed size")
    ErrReadTruncated  = errors.New("unexpected end of file while reading entry")
    ErrSeekFailed     = errors.New("failed to seek to offset")
)
```

---

## Task 3: Write Functions (`internal/storage/blob.go`)

### 3.1 SerializeHeader

```go
// SerializeHeader creates the 110-byte header for a blob entry
// Returns the header bytes ready to write to file
func SerializeHeader(hash string, dataLength uint64) ([]byte, error) {
    // Validate hash format (64 hex chars)
    if len(hash) != constants.HashSize {
        return nil, fmt.Errorf("%w: expected %d chars, got %d", ErrInvalidHash, constants.HashSize, len(hash))
    }

    // Validate hash contains only hex characters
    for _, c := range hash {
        if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
            return nil, fmt.Errorf("%w: contains non-hex character", ErrInvalidHash)
        }
    }

    header := make([]byte, constants.HeaderSize)

    // Magic bytes (offset 0, 4 bytes)
    copy(header[constants.MagicBytesOffset:], constants.MagicBytes)

    // Version (offset 4, 2 bytes, little-endian)
    binary.LittleEndian.PutUint16(header[constants.VersionOffset:], constants.BlobVersion)

    // Data length (offset 6, 8 bytes, little-endian)
    binary.LittleEndian.PutUint64(header[constants.DataLengthOffset:], dataLength)

    // Hash (offset 14, 64 bytes ASCII)
    copy(header[constants.HashOffset:], []byte(hash))

    // Reserved bytes (offset 78, 32 bytes) - already zero from make()

    return header, nil
}
```

### 3.2 AppendEntryFromReader (Streaming Version)

**Critical:** This function streams data from an io.Reader to support large files (100GB+) on low memory systems (< 2GB RAM).

```go
// AppendEntryFromReader appends a new asset entry to a .dat file by streaming from a reader
// Returns the byte offset where the entry was written
// Creates the file if it doesn't exist
// This is the primary function for uploads - never loads entire file into memory
func AppendEntryFromReader(datPath string, hash string, dataSize int64, reader io.Reader) (byteOffset int64, err error) {
    // Serialize header
    header, err := SerializeHeader(hash, uint64(dataSize))
    if err != nil {
        return 0, fmt.Errorf("failed to serialize header: %w", err)
    }

    // Open file for appending (create if not exists)
    f, err := os.OpenFile(datPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        return 0, fmt.Errorf("failed to open dat file: %w", err)
    }
    defer f.Close()

    // Get current file size (this will be our offset)
    stat, err := f.Stat()
    if err != nil {
        return 0, fmt.Errorf("failed to stat dat file: %w", err)
    }
    byteOffset = stat.Size()

    // Write header
    if _, err := f.Write(header); err != nil {
        return 0, fmt.Errorf("failed to write header: %w", err)
    }

    // Stream data from reader to file
    written, err := io.Copy(f, reader)
    if err != nil {
        return 0, fmt.Errorf("failed to write data: %w", err)
    }

    if written != dataSize {
        return 0, fmt.Errorf("size mismatch: expected %d bytes, wrote %d", dataSize, written)
    }

    // Sync to ensure durability before DB commit
    if err := f.Sync(); err != nil {
        return 0, fmt.Errorf("failed to sync dat file: %w", err)
    }

    return byteOffset, nil
}

// AppendEntry appends a new asset entry from a byte slice (convenience wrapper)
// Only use for small data or tests - for uploads use AppendEntryFromReader
func AppendEntry(datPath string, hash string, data []byte) (byteOffset int64, err error) {
    return AppendEntryFromReader(datPath, hash, int64(len(data)), bytes.NewReader(data))
}
```

### 3.3 GetDatFileSize

```go
// GetDatFileSize returns the current size of a .dat file
// Returns 0 if file doesn't exist
func GetDatFileSize(datPath string) (int64, error) {
    stat, err := os.Stat(datPath)
    if err != nil {
        if os.IsNotExist(err) {
            return 0, nil
        }
        return 0, fmt.Errorf("failed to stat dat file: %w", err)
    }
    return stat.Size(), nil
}
```

---

## Task 4: Read Functions (`internal/storage/blob.go`)

### 4.1 ReadHeader

```go
// ReadHeader reads and parses the 110-byte header at the given offset
// Does not read the actual data
func ReadHeader(datPath string, offset int64) (*BlobEntry, error) {
    f, err := os.Open(datPath)
    if err != nil {
        return nil, fmt.Errorf("failed to open dat file: %w", err)
    }
    defer f.Close()

    // Seek to offset
    if _, err := f.Seek(offset, io.SeekStart); err != nil {
        return nil, fmt.Errorf("%w: %v", ErrSeekFailed, err)
    }

    // Read header bytes
    header := make([]byte, constants.HeaderSize)
    n, err := io.ReadFull(f, header)
    if err != nil {
        if err == io.EOF || err == io.ErrUnexpectedEOF {
            return nil, fmt.Errorf("%w: read %d of %d bytes", ErrReadTruncated, n, constants.HeaderSize)
        }
        return nil, fmt.Errorf("failed to read header: %w", err)
    }

    return ParseHeader(header)
}
```

### 4.2 ParseHeader

```go
// ParseHeader parses a 110-byte header buffer into a BlobEntry
func ParseHeader(header []byte) (*BlobEntry, error) {
    if len(header) < constants.HeaderSize {
        return nil, fmt.Errorf("header too short: %d bytes", len(header))
    }

    entry := &BlobEntry{}

    // Validate magic bytes
    copy(entry.Magic[:], header[constants.MagicBytesOffset:constants.MagicBytesOffset+constants.MagicBytesSize])
    if string(entry.Magic[:]) != string(constants.MagicBytes) {
        return nil, fmt.Errorf("%w: got %q", ErrInvalidMagic, string(entry.Magic[:]))
    }

    // Parse version
    entry.Version = binary.LittleEndian.Uint16(header[constants.VersionOffset:])
    if entry.Version != constants.BlobVersion {
        return nil, fmt.Errorf("%w: got %d, expected %d", ErrInvalidVersion, entry.Version, constants.BlobVersion)
    }

    // Parse data length
    entry.DataLength = binary.LittleEndian.Uint64(header[constants.DataLengthOffset:])

    // Parse hash (64 ASCII hex chars)
    entry.Hash = string(header[constants.HashOffset : constants.HashOffset+constants.HashSize])

    // Copy reserved bytes
    copy(entry.Reserved[:], header[constants.ReservedOffset:constants.ReservedOffset+constants.ReservedHeaderBytes])

    return entry, nil
}
```

### 4.3 ReadEntry

```go
// ReadEntry reads a complete blob entry (header + data) at the given offset
func ReadEntry(datPath string, offset int64) (*BlobEntry, error) {
    f, err := os.Open(datPath)
    if err != nil {
        return nil, fmt.Errorf("failed to open dat file: %w", err)
    }
    defer f.Close()

    // Seek to offset
    if _, err := f.Seek(offset, io.SeekStart); err != nil {
        return nil, fmt.Errorf("%w: %v", ErrSeekFailed, err)
    }

    // Read header
    headerBuf := make([]byte, constants.HeaderSize)
    if _, err := io.ReadFull(f, headerBuf); err != nil {
        if err == io.EOF || err == io.ErrUnexpectedEOF {
            return nil, ErrReadTruncated
        }
        return nil, fmt.Errorf("failed to read header: %w", err)
    }

    entry, err := ParseHeader(headerBuf)
    if err != nil {
        return nil, err
    }

    // Read data
    entry.Data = make([]byte, entry.DataLength)
    if _, err := io.ReadFull(f, entry.Data); err != nil {
        if err == io.EOF || err == io.ErrUnexpectedEOF {
            return nil, ErrReadTruncated
        }
        return nil, fmt.Errorf("failed to read data: %w", err)
    }

    return entry, nil
}
```

### 4.4 ReadData

```go
// ReadData reads only the data portion of an entry (skips header)
// More efficient when you already know the data length from the database
func ReadData(datPath string, offset int64, dataLength uint64) ([]byte, error) {
    f, err := os.Open(datPath)
    if err != nil {
        return nil, fmt.Errorf("failed to open dat file: %w", err)
    }
    defer f.Close()

    // Seek past header to data
    dataStart := offset + int64(constants.HeaderSize)
    if _, err := f.Seek(dataStart, io.SeekStart); err != nil {
        return nil, fmt.Errorf("%w: %v", ErrSeekFailed, err)
    }

    // Read data
    data := make([]byte, dataLength)
    if _, err := io.ReadFull(f, data); err != nil {
        if err == io.EOF || err == io.ErrUnexpectedEOF {
            return nil, ErrReadTruncated
        }
        return nil, fmt.Errorf("failed to read data: %w", err)
    }

    return data, nil
}
```

---

## Task 5: Validation Functions (`internal/storage/blob.go`)

### 5.1 ValidateEntry

```go
// ValidateEntry reads an entry and verifies the hash matches the stored hash
// Returns nil if valid, error describing mismatch if invalid
func ValidateEntry(datPath string, offset int64) error {
    entry, err := ReadEntry(datPath, offset)
    if err != nil {
        return fmt.Errorf("failed to read entry: %w", err)
    }

    // Compute hash of data
    computedHash := ComputeBlake3Hex(entry.Data)

    // Compare with stored hash (case-insensitive)
    if !strings.EqualFold(computedHash, entry.Hash) {
        return fmt.Errorf("hash mismatch: stored=%s computed=%s", entry.Hash, computedHash)
    }

    return nil
}
```

### 5.2 ScanEntries

```go
// ScanEntries iterates through all entries in a .dat file
// Calls the callback function for each valid entry found
// Useful for rebuilding indexes or verification
// Stops on first error (invalid entry found)
func ScanEntries(datPath string, callback func(offset int64, entry *BlobEntry) error) error {
    f, err := os.Open(datPath)
    if err != nil {
        return fmt.Errorf("failed to open dat file: %w", err)
    }
    defer f.Close()

    stat, err := f.Stat()
    if err != nil {
        return fmt.Errorf("failed to stat dat file: %w", err)
    }
    fileSize := stat.Size()

    var offset int64 = 0
    headerBuf := make([]byte, constants.HeaderSize)

    for offset < fileSize {
        // Check if enough bytes remaining for header
        if offset+int64(constants.HeaderSize) > fileSize {
            // Orphan bytes at end - stop scanning (not an error)
            break
        }

        // Read header
        if _, err := f.Seek(offset, io.SeekStart); err != nil {
            return fmt.Errorf("seek failed at offset %d: %w", offset, err)
        }

        n, err := io.ReadFull(f, headerBuf)
        if err != nil {
            if err == io.EOF || err == io.ErrUnexpectedEOF {
                // Orphan bytes - stop scanning
                break
            }
            return fmt.Errorf("read failed at offset %d: %w", offset, err)
        }
        if n < constants.HeaderSize {
            break
        }

        // Parse header
        entry, err := ParseHeader(headerBuf)
        if err != nil {
            // Invalid entry - could be orphan data, stop scanning
            break
        }

        // Call callback
        if err := callback(offset, entry); err != nil {
            return err
        }

        // Move to next entry
        offset += int64(constants.HeaderSize) + int64(entry.DataLength)
    }

    return nil
}
```

---

## Task 6: DAT File Management (`internal/storage/dat.go`)

### 6.1 Create new file for DAT-level operations

```go
package storage

import (
    "fmt"
    "os"
    "path/filepath"
    "regexp"
    "sort"
    "strconv"

    "meshbank/internal/constants"
)

var datFileRegex = regexp.MustCompile(`^(\d{3,})\.dat$`)

// ListDatFiles returns all .dat files in a topic directory, sorted numerically
func ListDatFiles(topicPath string) ([]string, error) {
    entries, err := os.ReadDir(topicPath)
    if err != nil {
        return nil, fmt.Errorf("failed to read topic directory: %w", err)
    }

    var datFiles []string
    for _, entry := range entries {
        if entry.IsDir() {
            continue
        }
        if datFileRegex.MatchString(entry.Name()) {
            datFiles = append(datFiles, entry.Name())
        }
    }

    // Sort numerically (001.dat, 002.dat, ..., 010.dat, ...)
    sort.Slice(datFiles, func(i, j int) bool {
        numI := extractDatNumber(datFiles[i])
        numJ := extractDatNumber(datFiles[j])
        return numI < numJ
    })

    return datFiles, nil
}

// extractDatNumber extracts the numeric part from a .dat filename
func extractDatNumber(filename string) int {
    matches := datFileRegex.FindStringSubmatch(filename)
    if len(matches) < 2 {
        return 0
    }
    num, _ := strconv.Atoi(matches[1])
    return num
}

// FormatDatFilename formats a number into a .dat filename (e.g., 3 -> "003.dat")
func FormatDatFilename(num int) string {
    return fmt.Sprintf(constants.DatFilePattern, num)
}

// GetNextDatFilename determines the next .dat filename for a topic
// If no .dat files exist, returns "001.dat"
func GetNextDatFilename(topicPath string) (string, error) {
    datFiles, err := ListDatFiles(topicPath)
    if err != nil {
        return "", err
    }

    if len(datFiles) == 0 {
        return "001.dat", nil
    }

    // Get highest number and increment
    lastFile := datFiles[len(datFiles)-1]
    lastNum := extractDatNumber(lastFile)
    return FormatDatFilename(lastNum + 1), nil
}

// GetCurrentDatFile returns the current (latest) .dat file and its size
// If no .dat files exist, returns empty string and 0 size
func GetCurrentDatFile(topicPath string) (filename string, size int64, err error) {
    datFiles, err := ListDatFiles(topicPath)
    if err != nil {
        return "", 0, err
    }

    if len(datFiles) == 0 {
        return "", 0, nil
    }

    currentFile := datFiles[len(datFiles)-1]
    currentPath := filepath.Join(topicPath, currentFile)

    size, err = GetDatFileSize(currentPath)
    if err != nil {
        return "", 0, err
    }

    return currentFile, size, nil
}

// DetermineTargetDatFile decides which .dat file to write to
// Creates a new .dat if current would exceed maxSize after adding entrySize
// Returns the filename (not full path) and whether it's a new file
func DetermineTargetDatFile(topicPath string, entrySize int64, maxDatSize int64) (filename string, isNew bool, err error) {
    currentFile, currentSize, err := GetCurrentDatFile(topicPath)
    if err != nil {
        return "", false, err
    }

    // No existing .dat file
    if currentFile == "" {
        return "001.dat", true, nil
    }

    // Check if current file can accommodate the entry
    if currentSize+entrySize <= maxDatSize {
        return currentFile, false, nil
    }

    // Need a new file
    nextFile, err := GetNextDatFilename(topicPath)
    if err != nil {
        return "", false, err
    }

    return nextFile, true, nil
}

// GetTotalDatSize calculates the total size of all .dat files in a topic
func GetTotalDatSize(topicPath string) (int64, error) {
    datFiles, err := ListDatFiles(topicPath)
    if err != nil {
        return 0, err
    }

    var total int64
    for _, filename := range datFiles {
        path := filepath.Join(topicPath, filename)
        size, err := GetDatFileSize(path)
        if err != nil {
            return 0, err
        }
        total += size
    }

    return total, nil
}

// CountDatFiles returns the number of .dat files in a topic
func CountDatFiles(topicPath string) (int, error) {
    datFiles, err := ListDatFiles(topicPath)
    if err != nil {
        return 0, err
    }
    return len(datFiles), nil
}
```

---

## Task 7: Integration with Database (`internal/database/topic.go`)

### 7.1 Update InsertAsset to include ByteOffset

Update the `InsertAsset` function to handle the new `byte_offset` column:

```go
func InsertAsset(tx *sql.Tx, asset Asset) error {
    _, err := tx.Exec(`
        INSERT INTO assets (asset_id, asset_size, origin_name, parent_id, extension, blob_name, byte_offset, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)
    `, asset.AssetID, asset.AssetSize, asset.OriginName, asset.ParentID, asset.Extension, asset.BlobName, asset.ByteOffset, asset.CreatedAt)

    if err != nil {
        return fmt.Errorf("failed to insert asset: %w", err)
    }
    return nil
}
```

### 7.2 Update GetAsset to retrieve ByteOffset

```go
func GetAsset(db *sql.DB, assetID string) (*Asset, error) {
    row := db.QueryRow(`
        SELECT asset_id, asset_size, origin_name, parent_id, extension, blob_name, byte_offset, created_at
        FROM assets WHERE asset_id = ?
    `, assetID)

    var asset Asset
    var originName, parentID sql.NullString

    err := row.Scan(
        &asset.AssetID,
        &asset.AssetSize,
        &originName,
        &parentID,
        &asset.Extension,
        &asset.BlobName,
        &asset.ByteOffset,
        &asset.CreatedAt,
    )

    if err == sql.ErrNoRows {
        return nil, nil
    }
    if err != nil {
        return nil, fmt.Errorf("failed to get asset: %w", err)
    }

    if originName.Valid {
        asset.OriginName = originName.String
    }
    if parentID.Valid {
        asset.ParentID = &parentID.String
    }

    return &asset, nil
}
```

---

## Task 8: High-Level Read Function (`internal/storage/blob.go`)

### 8.1 ReadAssetData

Convenience function that combines DB lookup with blob read:

```go
// ReadAssetData reads an asset's data given its metadata from the database
// This is the primary function for downloading assets
func ReadAssetData(topicPath string, blobName string, byteOffset int64, assetSize int64) ([]byte, error) {
    datPath := filepath.Join(topicPath, blobName)
    return ReadData(datPath, byteOffset, uint64(assetSize))
}
```

---

## Verification Checklist

After completing Phase 3, verify:

1. **Header serialization:**
   - Create a header with a known hash and length
   - Verify total size is 110 bytes
   - Verify magic bytes at offset 0-3 are "MSHB"
   - Verify version at offset 4-5 is 1 (little-endian)
   - Verify data length at offset 6-13 is correct (little-endian)
   - Verify hash at offset 14-77 is correct ASCII

2. **Write and read roundtrip:**
   - Create a temp .dat file
   - Append an entry with known data
   - Read back the entry
   - Verify data matches exactly
   - Verify returned byte_offset is 0 (first entry)

3. **Multiple entries:**
   - Append 3 entries of different sizes
   - Verify each byte_offset is correct (0, 110+len1, 110+len1+110+len2)
   - Read each entry back by offset
   - Verify all data matches

4. **DAT file management:**
   - Test ListDatFiles with mixed valid/invalid filenames
   - Test GetNextDatFilename with empty dir -> "001.dat"
   - Test GetNextDatFilename with existing 001.dat, 002.dat -> "003.dat"
   - Test DetermineTargetDatFile threshold logic

5. **Scan function:**
   - Create .dat with multiple entries
   - Scan and collect all hashes
   - Verify all entries found

6. **Error handling:**
   - ReadEntry on empty file -> appropriate error
   - ReadEntry at invalid offset -> appropriate error
   - SerializeHeader with invalid hash -> error

---

## Files to Create/Update

| File | Action | Description |
|------|--------|-------------|
| `internal/constants/constants.go` | Update | Add header offset constants, update HeaderSize to 110 |
| `internal/storage/blob.go` | Create | Entry struct, serialize, read, write functions |
| `internal/storage/dat.go` | Create | DAT file listing and management utilities |
| `internal/database/schema.go` | Update | Add `byte_offset` column to assets table |
| `internal/database/topic.go` | Update | Update Asset struct and queries for byte_offset |

---

## Notes for Agent

- **Endianness:** All multi-byte integers use **little-endian** encoding via `binary.LittleEndian`
- **Hash format:** 64 ASCII hex characters stored directly (not raw bytes) for human readability
- **File permissions:** Use `0644` for .dat files (readable by all, writable by owner)
- **Sync on write:** Always `f.Sync()` after writing to ensure durability before DB commit
- **Orphan handling:** ScanEntries stops gracefully at orphan/corrupted bytes (not an error)
- **No compression:** Data stored raw as per original spec
- **Thread safety:** File operations are not thread-safe; higher-level code must handle locking
- **Import note:** `ReadAssetData` needs `filepath` import
- **Import note:** `AppendEntry` needs `bytes` import for `bytes.NewReader`
- **Validation function needs:** Import `strings` package for `strings.EqualFold`
- **Streaming:** Always use `AppendEntryFromReader` for uploads to support 100GB+ files on 2GB RAM