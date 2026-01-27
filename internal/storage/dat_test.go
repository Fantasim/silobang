package storage

import (
	"silobang/internal/constants"
	"os"
	"path/filepath"
	"testing"
)

func TestListDatFiles(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "silobang-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create some .dat files
	os.WriteFile(filepath.Join(tmpDir, "001.dat"), []byte{}, 0644)
	os.WriteFile(filepath.Join(tmpDir, "002.dat"), []byte{}, 0644)
	os.WriteFile(filepath.Join(tmpDir, "010.dat"), []byte{}, 0644)
	os.WriteFile(filepath.Join(tmpDir, "invalid.txt"), []byte{}, 0644) // Should be ignored

	files, err := ListDatFiles(tmpDir)
	if err != nil {
		t.Fatalf("ListDatFiles failed: %v", err)
	}

	// Verify count
	if len(files) != 3 {
		t.Errorf("Expected 3 .dat files, got %d", len(files))
	}

	// Verify ordering (should be 001.dat, 002.dat, 010.dat)
	expectedOrder := []string{"001.dat", "002.dat", "010.dat"}
	for i, expected := range expectedOrder {
		if files[i] != expected {
			t.Errorf("Expected files[%d] = %s, got %s", i, expected, files[i])
		}
	}
}

func TestGetNextDatFilename(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "silobang-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test empty directory
	next, err := GetNextDatFilename(tmpDir)
	if err != nil {
		t.Fatalf("GetNextDatFilename failed: %v", err)
	}
	if next != constants.FirstDatFilename {
		t.Errorf("Expected '%s' for empty dir, got %s", constants.FirstDatFilename, next)
	}

	// Create some files
	os.WriteFile(filepath.Join(tmpDir, "001.dat"), []byte{}, 0644)
	os.WriteFile(filepath.Join(tmpDir, "002.dat"), []byte{}, 0644)

	next, err = GetNextDatFilename(tmpDir)
	if err != nil {
		t.Fatalf("GetNextDatFilename failed: %v", err)
	}
	if next != FormatDatFilename(3) {
		t.Errorf("Expected '%s', got %s", FormatDatFilename(3), next)
	}
}

func TestGetCurrentDatFile(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "silobang-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test empty directory
	filename, size, err := GetCurrentDatFile(tmpDir)
	if err != nil {
		t.Fatalf("GetCurrentDatFile failed: %v", err)
	}
	if filename != "" || size != 0 {
		t.Errorf("Expected empty filename and 0 size for empty dir, got %s, %d", filename, size)
	}

	// Create files
	os.WriteFile(filepath.Join(tmpDir, "001.dat"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "002.dat"), []byte("testdata"), 0644)

	filename, size, err = GetCurrentDatFile(tmpDir)
	if err != nil {
		t.Fatalf("GetCurrentDatFile failed: %v", err)
	}
	if filename != "002.dat" {
		t.Errorf("Expected current file '002.dat', got %s", filename)
	}
	if size != 8 {
		t.Errorf("Expected size 8, got %d", size)
	}
}

func TestDetermineTargetDatFile(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "silobang-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	maxSize := int64(1000)

	// Test empty directory - should create first dat file
	filename, isNew, err := DetermineTargetDatFile(tmpDir, 100, maxSize)
	if err != nil {
		t.Fatalf("DetermineTargetDatFile failed: %v", err)
	}
	if filename != constants.FirstDatFilename || !isNew {
		t.Errorf("Expected '%s' (new), got %s (new=%v)", constants.FirstDatFilename, filename, isNew)
	}

	// Create a file with 500 bytes
	os.WriteFile(filepath.Join(tmpDir, "001.dat"), make([]byte, 500), 0644)

	// Request 400 bytes - should fit (500 + 400 = 900 <= 1000)
	filename, isNew, err = DetermineTargetDatFile(tmpDir, 400, maxSize)
	if err != nil {
		t.Fatalf("DetermineTargetDatFile failed: %v", err)
	}
	if filename != "001.dat" || isNew {
		t.Errorf("Expected '001.dat' (existing), got %s (new=%v)", filename, isNew)
	}

	// Request 600 bytes - should not fit (500 + 600 = 1100 > 1000)
	filename, isNew, err = DetermineTargetDatFile(tmpDir, 600, maxSize)
	if err != nil {
		t.Fatalf("DetermineTargetDatFile failed: %v", err)
	}
	if filename != FormatDatFilename(2) || !isNew {
		t.Errorf("Expected '%s' (new), got %s (new=%v)", FormatDatFilename(2), filename, isNew)
	}
}

func TestGetTotalDatSize(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "silobang-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create files with known sizes
	os.WriteFile(filepath.Join(tmpDir, "001.dat"), make([]byte, 100), 0644)
	os.WriteFile(filepath.Join(tmpDir, "002.dat"), make([]byte, 200), 0644)
	os.WriteFile(filepath.Join(tmpDir, "003.dat"), make([]byte, 300), 0644)

	total, err := GetTotalDatSize(tmpDir)
	if err != nil {
		t.Fatalf("GetTotalDatSize failed: %v", err)
	}

	expected := int64(600)
	if total != expected {
		t.Errorf("Expected total size %d, got %d", expected, total)
	}
}

func TestCountDatFiles(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "silobang-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create files
	os.WriteFile(filepath.Join(tmpDir, "001.dat"), []byte{}, 0644)
	os.WriteFile(filepath.Join(tmpDir, "002.dat"), []byte{}, 0644)
	os.WriteFile(filepath.Join(tmpDir, "003.dat"), []byte{}, 0644)
	os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte{}, 0644) // Should not be counted

	count, err := CountDatFiles(tmpDir)
	if err != nil {
		t.Fatalf("CountDatFiles failed: %v", err)
	}

	if count != 3 {
		t.Errorf("Expected count 3, got %d", count)
	}
}

func TestFormatDatFilename(t *testing.T) {
	tests := []struct {
		num      int
		expected string
	}{
		{1, "000001.dat"},
		{10, "000010.dat"},
		{100, "000100.dat"},
		{999, "000999.dat"},
		{1000, "001000.dat"},
		{10000, "010000.dat"},
		{999999, "999999.dat"},
	}

	for _, test := range tests {
		result := FormatDatFilename(test.num)
		if result != test.expected {
			t.Errorf("FormatDatFilename(%d) = %s, expected %s", test.num, result, test.expected)
		}
	}
}

func TestMixedDatFileFormats(t *testing.T) {
	// Test backward compatibility: system should handle mixed 3-digit and 6-digit files
	tmpDir, err := os.MkdirTemp("", "silobang-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create legacy 3-digit files (backward compatibility)
	os.WriteFile(filepath.Join(tmpDir, "001.dat"), []byte{}, 0644)
	os.WriteFile(filepath.Join(tmpDir, "002.dat"), []byte{}, 0644)

	// List should work with 3-digit files
	files, err := ListDatFiles(tmpDir)
	if err != nil {
		t.Fatalf("ListDatFiles failed with 3-digit files: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("Expected 2 files, got %d", len(files))
	}

	// Next filename should be in 6-digit format
	next, err := GetNextDatFilename(tmpDir)
	if err != nil {
		t.Fatalf("GetNextDatFilename failed: %v", err)
	}
	if next != FormatDatFilename(3) {
		t.Errorf("Expected '%s', got %s", FormatDatFilename(3), next)
	}

	// Create the new 6-digit file
	os.WriteFile(filepath.Join(tmpDir, next), []byte{}, 0644)

	// List should handle mixed formats and sort numerically
	files, err = ListDatFiles(tmpDir)
	if err != nil {
		t.Fatalf("ListDatFiles failed with mixed formats: %v", err)
	}
	if len(files) != 3 {
		t.Errorf("Expected 3 files, got %d", len(files))
	}

	// Verify numeric ordering (001.dat, 002.dat, 000003.dat)
	expectedNums := []int{1, 2, 3}
	for i, f := range files {
		num := extractDatNumber(f)
		if num != expectedNums[i] {
			t.Errorf("Expected file %d to have number %d, got %d (filename: %s)", i, expectedNums[i], num, f)
		}
	}
}
