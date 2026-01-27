package storage

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/zeebo/blake3"
)

// ComputeBlake3 computes the BLAKE3 hash of a byte slice
// Returns raw 32 bytes
func ComputeBlake3(data []byte) []byte {
	hash := blake3.Sum256(data)
	return hash[:]
}

// ComputeBlake3Hex computes the BLAKE3 hash and returns as 64-char hex string
func ComputeBlake3Hex(data []byte) string {
	hash := ComputeBlake3(data)
	return hex.EncodeToString(hash)
}

// ComputeFileBlake3 computes the BLAKE3 hash of file contents
// Streams file to avoid loading entirely in memory
func ComputeFileBlake3(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	hasher := blake3.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return nil, err
	}

	hash := hasher.Sum(nil)
	return hash, nil
}

// ComputeFileBlake3Hex computes the BLAKE3 hash of file contents and returns hex string
func ComputeFileBlake3Hex(path string) (string, error) {
	hash, err := ComputeFileBlake3(path)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hash), nil
}

// GenesisHash computes the initial hash for an empty .dat file
// Format: BLAKE3("MSHB_INIT" || dat_filename)
func GenesisHash(datFilename string) string {
	input := append([]byte("MSHB_INIT"), []byte(datFilename)...)
	hash := blake3.Sum256(input)
	return hex.EncodeToString(hash[:])
}

// ComputeRunningHash computes the next hash in the chain after appending an entry
// Format: BLAKE3(prev_hash_bytes || entry_hash_bytes || offset_le || size_le)
// Input is 80 bytes total: 32 (prev) + 32 (entry) + 8 (offset) + 8 (size)
func ComputeRunningHash(prevHashHex, entryHashHex string, entryOffset, entrySize int64) (string, error) {
	// Decode previous hash (32 bytes)
	prevHash, err := hex.DecodeString(prevHashHex)
	if err != nil {
		return "", fmt.Errorf("invalid prev hash: %w", err)
	}
	if len(prevHash) != 32 {
		return "", fmt.Errorf("prev hash must be 32 bytes, got %d", len(prevHash))
	}

	// Decode entry hash (32 bytes)
	entryHash, err := hex.DecodeString(entryHashHex)
	if err != nil {
		return "", fmt.Errorf("invalid entry hash: %w", err)
	}
	if len(entryHash) != 32 {
		return "", fmt.Errorf("entry hash must be 32 bytes, got %d", len(entryHash))
	}

	// Build input: prev_hash (32) || entry_hash (32) || offset (8) || size (8) = 80 bytes
	input := make([]byte, 80)
	copy(input[0:32], prevHash)
	copy(input[32:64], entryHash)
	binary.LittleEndian.PutUint64(input[64:72], uint64(entryOffset))
	binary.LittleEndian.PutUint64(input[72:80], uint64(entrySize))

	// Compute new hash
	hash := blake3.Sum256(input)
	return hex.EncodeToString(hash[:]), nil
}

// ReplayRunningHash rebuilds the running hash by scanning all entries in a .dat file
// Used for verification and initial computation
func ReplayRunningHash(datPath string) (runningHash string, entryCount int, err error) {
	datFilename := filepath.Base(datPath)
	runningHash = GenesisHash(datFilename)
	entryCount = 0

	err = ScanEntries(datPath, func(offset int64, entry *BlobEntry) error {
		runningHash, err = ComputeRunningHash(runningHash, entry.Hash, offset, int64(entry.DataLength))
		if err != nil {
			return err
		}
		entryCount++
		return nil
	})

	if err != nil {
		return "", 0, err
	}

	return runningHash, entryCount, nil
}

// VerifyRunningHash checks if a stored running hash matches the computed one
func VerifyRunningHash(datPath string, storedHash string, storedCount int) (bool, error) {
	computedHash, computedCount, err := ReplayRunningHash(datPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	return storedHash == computedHash && storedCount == computedCount, nil
}

// HashProgressCallback is called during hash verification with progress updates
// Return an error to cancel the operation (e.g., context.Canceled)
type HashProgressCallback func(entriesProcessed int) error

// ReplayRunningHashWithProgress rebuilds the running hash with progress reporting
// progressInterval controls how often the callback is invoked (every N entries)
// Set progressInterval to 0 to disable progress callbacks
func ReplayRunningHashWithProgress(datPath string, progressInterval int, callback HashProgressCallback) (runningHash string, entryCount int, err error) {
	datFilename := filepath.Base(datPath)
	runningHash = GenesisHash(datFilename)
	entryCount = 0

	err = ScanEntries(datPath, func(offset int64, entry *BlobEntry) error {
		runningHash, err = ComputeRunningHash(runningHash, entry.Hash, offset, int64(entry.DataLength))
		if err != nil {
			return err
		}
		entryCount++

		// Report progress at intervals
		if callback != nil && progressInterval > 0 && entryCount%progressInterval == 0 {
			if err := callback(entryCount); err != nil {
				return err // Allows cancellation
			}
		}
		return nil
	})

	if err != nil {
		return "", 0, err
	}

	return runningHash, entryCount, nil
}
