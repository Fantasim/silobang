package storage

import (
	"os"
	"path/filepath"
	"testing"

	"meshbank/internal/constants"
)

func TestHeaderSerializationRoundtrip(t *testing.T) {
	hash := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	dataLength := uint64(1024)

	// Serialize header
	header, err := SerializeHeader(hash, dataLength)
	if err != nil {
		t.Fatalf("SerializeHeader failed: %v", err)
	}

	// Verify header size
	if len(header) != constants.HeaderSize {
		t.Errorf("Expected header size %d, got %d", constants.HeaderSize, len(header))
	}

	// Parse header back
	entry, err := ParseHeader(header)
	if err != nil {
		t.Fatalf("ParseHeader failed: %v", err)
	}

	// Verify fields
	if string(entry.Magic[:]) != "MSHB" {
		t.Errorf("Expected magic 'MSHB', got %q", string(entry.Magic[:]))
	}
	if entry.Version != constants.BlobVersion {
		t.Errorf("Expected version %d, got %d", constants.BlobVersion, entry.Version)
	}
	if entry.DataLength != dataLength {
		t.Errorf("Expected data length %d, got %d", dataLength, entry.DataLength)
	}
	if entry.Hash != hash {
		t.Errorf("Expected hash %s, got %s", hash, entry.Hash)
	}
}

func TestAppendAndReadEntry(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "meshbank-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	datPath := filepath.Join(tmpDir, FormatDatFilename(1))
	testData := []byte("Hello, MeshBank!")
	hash := ComputeBlake3Hex(testData)

	// Append entry
	offset, err := AppendEntry(datPath, hash, testData)
	if err != nil {
		t.Fatalf("AppendEntry failed: %v", err)
	}

	// Verify offset is 0 (first entry)
	if offset != 0 {
		t.Errorf("Expected offset 0, got %d", offset)
	}

	// Read entry back
	entry, err := ReadEntry(datPath, offset)
	if err != nil {
		t.Fatalf("ReadEntry failed: %v", err)
	}

	// Verify data matches
	if string(entry.Data) != string(testData) {
		t.Errorf("Expected data %q, got %q", string(testData), string(entry.Data))
	}

	// Verify hash matches
	if entry.Hash != hash {
		t.Errorf("Expected hash %s, got %s", hash, entry.Hash)
	}
}

func TestMultipleEntries(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "meshbank-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	datPath := filepath.Join(tmpDir, FormatDatFilename(1))

	// Append three entries
	data1 := []byte("First entry")
	data2 := []byte("Second entry with more data")
	data3 := []byte("Third")

	hash1 := ComputeBlake3Hex(data1)
	hash2 := ComputeBlake3Hex(data2)
	hash3 := ComputeBlake3Hex(data3)

	offset1, _ := AppendEntry(datPath, hash1, data1)
	offset2, _ := AppendEntry(datPath, hash2, data2)
	offset3, _ := AppendEntry(datPath, hash3, data3)

	// Verify offsets
	expectedOffset2 := int64(constants.HeaderSize) + int64(len(data1))
	expectedOffset3 := expectedOffset2 + int64(constants.HeaderSize) + int64(len(data2))

	if offset1 != 0 {
		t.Errorf("Expected offset1 = 0, got %d", offset1)
	}
	if offset2 != expectedOffset2 {
		t.Errorf("Expected offset2 = %d, got %d", expectedOffset2, offset2)
	}
	if offset3 != expectedOffset3 {
		t.Errorf("Expected offset3 = %d, got %d", expectedOffset3, offset3)
	}

	// Read all entries back
	entry1, _ := ReadEntry(datPath, offset1)
	entry2, _ := ReadEntry(datPath, offset2)
	entry3, _ := ReadEntry(datPath, offset3)

	if string(entry1.Data) != string(data1) {
		t.Errorf("Entry 1 data mismatch")
	}
	if string(entry2.Data) != string(data2) {
		t.Errorf("Entry 2 data mismatch")
	}
	if string(entry3.Data) != string(data3) {
		t.Errorf("Entry 3 data mismatch")
	}
}

func TestScanEntries(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "meshbank-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	datPath := filepath.Join(tmpDir, FormatDatFilename(1))

	// Append three entries
	data1 := []byte("Entry 1")
	data2 := []byte("Entry 2")
	data3 := []byte("Entry 3")

	AppendEntry(datPath, ComputeBlake3Hex(data1), data1)
	AppendEntry(datPath, ComputeBlake3Hex(data2), data2)
	AppendEntry(datPath, ComputeBlake3Hex(data3), data3)

	// Scan and collect all hashes
	var hashes []string
	err = ScanEntries(datPath, func(offset int64, entry *BlobEntry) error {
		hashes = append(hashes, entry.Hash)
		return nil
	})

	if err != nil {
		t.Fatalf("ScanEntries failed: %v", err)
	}

	// Verify we found all 3 entries
	if len(hashes) != 3 {
		t.Errorf("Expected 3 entries, found %d", len(hashes))
	}
}

func TestInvalidHash(t *testing.T) {
	// Test hash too short
	_, err := SerializeHeader("tooshort", 100)
	if err == nil {
		t.Error("Expected error for short hash")
	}

	// Test hash with invalid characters
	invalidHash := "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"
	_, err = SerializeHeader(invalidHash, 100)
	if err == nil {
		t.Error("Expected error for invalid hash characters")
	}
}

func TestValidateEntry(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "meshbank-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	datPath := filepath.Join(tmpDir, FormatDatFilename(1))
	testData := []byte("Validation test data")
	hash := ComputeBlake3Hex(testData)

	// Append valid entry
	offset, _ := AppendEntry(datPath, hash, testData)

	// Validate should pass
	err = ValidateEntry(datPath, offset)
	if err != nil {
		t.Errorf("ValidateEntry failed for valid entry: %v", err)
	}

	// Append entry with wrong hash
	wrongHash := "0000000000000000000000000000000000000000000000000000000000000000"
	offset2, _ := AppendEntry(datPath, wrongHash, testData)

	// Validate should fail
	err = ValidateEntry(datPath, offset2)
	if err == nil {
		t.Error("Expected ValidateEntry to fail for mismatched hash")
	}
}
