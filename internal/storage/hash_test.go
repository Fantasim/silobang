package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenesisHash(t *testing.T) {
	firstDat := FormatDatFilename(1)
	secondDat := FormatDatFilename(2)

	// Genesis hash should be deterministic
	hash1 := GenesisHash(firstDat)
	hash2 := GenesisHash(firstDat)

	if hash1 != hash2 {
		t.Errorf("Genesis hash is not deterministic: %s != %s", hash1, hash2)
	}

	// Different filenames should produce different hashes
	hash3 := GenesisHash(secondDat)
	if hash1 == hash3 {
		t.Error("Different filenames should produce different genesis hashes")
	}

	// Hash should be 64 hex characters
	if len(hash1) != 64 {
		t.Errorf("Genesis hash should be 64 chars, got %d", len(hash1))
	}
}

func TestComputeRunningHash(t *testing.T) {
	prevHash := GenesisHash(FormatDatFilename(1))
	entryHash := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	// Running hash should be deterministic
	hash1, err := ComputeRunningHash(prevHash, entryHash, 0, 1024)
	if err != nil {
		t.Fatalf("ComputeRunningHash failed: %v", err)
	}

	hash2, err := ComputeRunningHash(prevHash, entryHash, 0, 1024)
	if err != nil {
		t.Fatalf("ComputeRunningHash failed: %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("Running hash is not deterministic: %s != %s", hash1, hash2)
	}

	// Different offset should produce different hash
	hash3, err := ComputeRunningHash(prevHash, entryHash, 100, 1024)
	if err != nil {
		t.Fatalf("ComputeRunningHash failed: %v", err)
	}

	if hash1 == hash3 {
		t.Error("Different offset should produce different running hash")
	}

	// Different size should produce different hash
	hash4, err := ComputeRunningHash(prevHash, entryHash, 0, 2048)
	if err != nil {
		t.Fatalf("ComputeRunningHash failed: %v", err)
	}

	if hash1 == hash4 {
		t.Error("Different size should produce different running hash")
	}

	// Hash should be 64 hex characters
	if len(hash1) != 64 {
		t.Errorf("Running hash should be 64 chars, got %d", len(hash1))
	}
}

func TestComputeRunningHashInvalidInput(t *testing.T) {
	validHash := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	// Invalid prev hash (too short)
	_, err := ComputeRunningHash("tooshort", validHash, 0, 1024)
	if err == nil {
		t.Error("Expected error for short prev hash")
	}

	// Invalid entry hash (not hex)
	_, err = ComputeRunningHash(validHash, "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz", 0, 1024)
	if err == nil {
		t.Error("Expected error for invalid entry hash")
	}
}

func TestReplayRunningHash(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "silobang-hash-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	datFilename := FormatDatFilename(1)
	datPath := filepath.Join(tmpDir, datFilename)

	// Append three entries and compute running hash manually
	data1 := []byte("First entry")
	data2 := []byte("Second entry with more data")
	data3 := []byte("Third")

	hash1 := ComputeBlake3Hex(data1)
	hash2 := ComputeBlake3Hex(data2)
	hash3 := ComputeBlake3Hex(data3)

	offset1, _ := AppendEntry(datPath, hash1, data1)
	offset2, _ := AppendEntry(datPath, hash2, data2)
	offset3, _ := AppendEntry(datPath, hash3, data3)

	// Compute expected running hash manually
	genesis := GenesisHash(datFilename)
	runningHash, _ := ComputeRunningHash(genesis, hash1, offset1, int64(len(data1)))
	runningHash, _ = ComputeRunningHash(runningHash, hash2, offset2, int64(len(data2)))
	expectedHash, _ := ComputeRunningHash(runningHash, hash3, offset3, int64(len(data3)))

	// Replay should match
	replayedHash, entryCount, err := ReplayRunningHash(datPath)
	if err != nil {
		t.Fatalf("ReplayRunningHash failed: %v", err)
	}

	if replayedHash != expectedHash {
		t.Errorf("Replayed hash mismatch: expected %s, got %s", expectedHash, replayedHash)
	}

	if entryCount != 3 {
		t.Errorf("Expected 3 entries, got %d", entryCount)
	}
}

func TestReplayRunningHashEmptyFile(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "silobang-hash-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	datFilename := FormatDatFilename(1)
	datPath := filepath.Join(tmpDir, datFilename)

	// Create empty file
	f, err := os.Create(datPath)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	f.Close()

	// Replay on empty file should return genesis hash
	hash, count, err := ReplayRunningHash(datPath)
	if err != nil {
		t.Fatalf("ReplayRunningHash failed: %v", err)
	}

	expectedGenesis := GenesisHash(datFilename)
	if hash != expectedGenesis {
		t.Errorf("Empty file hash should be genesis: expected %s, got %s", expectedGenesis, hash)
	}

	if count != 0 {
		t.Errorf("Empty file should have 0 entries, got %d", count)
	}
}

func TestVerifyRunningHash(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "silobang-hash-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	datPath := filepath.Join(tmpDir, FormatDatFilename(1))

	// Append some entries
	data1 := []byte("Test data 1")
	data2 := []byte("Test data 2")

	hash1 := ComputeBlake3Hex(data1)
	hash2 := ComputeBlake3Hex(data2)

	AppendEntry(datPath, hash1, data1)
	AppendEntry(datPath, hash2, data2)

	// Get the correct hash
	correctHash, correctCount, _ := ReplayRunningHash(datPath)

	// Verification should pass with correct values
	match, err := VerifyRunningHash(datPath, correctHash, correctCount)
	if err != nil {
		t.Fatalf("VerifyRunningHash failed: %v", err)
	}
	if !match {
		t.Error("Verification should pass with correct hash and count")
	}

	// Verification should fail with wrong hash
	match, err = VerifyRunningHash(datPath, "0000000000000000000000000000000000000000000000000000000000000000", correctCount)
	if err != nil {
		t.Fatalf("VerifyRunningHash failed: %v", err)
	}
	if match {
		t.Error("Verification should fail with wrong hash")
	}

	// Verification should fail with wrong count
	match, err = VerifyRunningHash(datPath, correctHash, correctCount+1)
	if err != nil {
		t.Fatalf("VerifyRunningHash failed: %v", err)
	}
	if match {
		t.Error("Verification should fail with wrong count")
	}
}

func TestRunningHashChainIntegrity(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "silobang-hash-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	datFilename := FormatDatFilename(1)
	datPath := filepath.Join(tmpDir, datFilename)

	// Append entries one by one and track running hash
	testData := [][]byte{
		[]byte("Entry 1"),
		[]byte("Entry 2"),
		[]byte("Entry 3"),
	}

	runningHash := GenesisHash(datFilename)
	var entryCount int

	for _, data := range testData {
		hash := ComputeBlake3Hex(data)
		offset, err := AppendEntry(datPath, hash, data)
		if err != nil {
			t.Fatalf("AppendEntry failed: %v", err)
		}

		// Update running hash
		runningHash, err = ComputeRunningHash(runningHash, hash, offset, int64(len(data)))
		if err != nil {
			t.Fatalf("ComputeRunningHash failed: %v", err)
		}
		entryCount++

		// Verify at each step
		match, err := VerifyRunningHash(datPath, runningHash, entryCount)
		if err != nil {
			t.Fatalf("VerifyRunningHash failed: %v", err)
		}
		if !match {
			t.Errorf("Hash chain broken at entry %d", entryCount)
		}
	}
}
