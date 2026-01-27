package storage

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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

// SerializeHeader creates the 110-byte header for a blob entry
// Returns the header bytes ready to write to file
func SerializeHeader(hash string, dataLength uint64) ([]byte, error) {
	// Validate hash format (64 hex chars)
	if len(hash) != constants.HashLength {
		return nil, fmt.Errorf("%w: expected %d chars, got %d", ErrInvalidHash, constants.HashLength, len(hash))
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

// AppendEntryFromReader appends a new asset entry to a .dat file by streaming from a reader.
// Returns the byte offset where the entry was written.
// Creates the file if it doesn't exist.
// This is the primary function for uploads - never loads entire file into memory.
//
// IMPORTANT: This function is NOT concurrency-safe on its own. The caller must hold
// a per-topic write lock to prevent byte offset races between concurrent writers
// to the same .dat file. The offset is determined via Stat() before writing, and
// concurrent calls could read the same offset before either write completes.
// See AssetService.Upload() for the locking strategy.
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

// ParseHeader parses a 110-byte header buffer into a BlobEntry
func ParseHeader(header []byte) (*BlobEntry, error) {
	if len(header) < constants.HeaderSize {
		return nil, fmt.Errorf("header too short: %d bytes", len(header))
	}

	entry := &BlobEntry{}

	// Validate magic bytes
	copy(entry.Magic[:], header[constants.MagicBytesOffset:constants.MagicBytesOffset+4])
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
	entry.Hash = string(header[constants.HashOffset : constants.HashOffset+constants.HashLength])

	// Copy reserved bytes
	copy(entry.Reserved[:], header[constants.ReservedOffset:constants.ReservedOffset+constants.ReservedHeaderBytes])

	return entry, nil
}

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

// ReadAssetData reads an asset's data given its metadata from the database
// This is the primary function for downloading assets
func ReadAssetData(topicPath string, blobName string, byteOffset int64, assetSize int64) ([]byte, error) {
	datPath := filepath.Join(topicPath, blobName)
	return ReadData(datPath, byteOffset, uint64(assetSize))
}
