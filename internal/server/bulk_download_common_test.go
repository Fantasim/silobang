package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"meshbank/internal/constants"
	"meshbank/internal/database"
	"meshbank/internal/sanitize"
)

func TestBuildFilename(t *testing.T) {
	tests := []struct {
		name      string
		asset     *database.Asset
		format    string
		usedNames map[string]int
		expected  string
	}{
		{
			name: "hash format with extension",
			asset: &database.Asset{
				AssetID:    "abc123",
				Extension:  "glb",
				OriginName: "model",
			},
			format:    constants.FilenameFormatHash,
			usedNames: make(map[string]int),
			expected:  "abc123.glb",
		},
		{
			name: "hash format without extension",
			asset: &database.Asset{
				AssetID:    "abc123",
				OriginName: "LICENSE",
			},
			format:    constants.FilenameFormatHash,
			usedNames: make(map[string]int),
			expected:  "abc123",
		},
		{
			name: "original format with origin name",
			asset: &database.Asset{
				AssetID:    "abc123",
				Extension:  "glb",
				OriginName: "model",
			},
			format:    constants.FilenameFormatOriginal,
			usedNames: make(map[string]int),
			expected:  "model.glb",
		},
		{
			name: "original format without origin name falls back to hash",
			asset: &database.Asset{
				AssetID:   "abc123",
				Extension: "bin",
			},
			format:    constants.FilenameFormatOriginal,
			usedNames: make(map[string]int),
			expected:  "abc123.bin",
		},
		{
			name: "hash_original format with origin name",
			asset: &database.Asset{
				AssetID:    "abc123",
				Extension:  "glb",
				OriginName: "model",
			},
			format:    constants.FilenameFormatHashOriginal,
			usedNames: make(map[string]int),
			expected:  "abc123_model.glb",
		},
		{
			name: "hash_original format without origin name falls back to hash",
			asset: &database.Asset{
				AssetID:   "abc123",
				Extension: "glb",
			},
			format:    constants.FilenameFormatHashOriginal,
			usedNames: make(map[string]int),
			expected:  "abc123.glb",
		},
		{
			name: "unknown format defaults to hash",
			asset: &database.Asset{
				AssetID:    "abc123",
				Extension:  "glb",
				OriginName: "model",
			},
			format:    "nonexistent_format",
			usedNames: make(map[string]int),
			expected:  "abc123.glb",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := buildFilename(tc.asset, tc.format, tc.usedNames)
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestBuildFilename_CollisionHandling(t *testing.T) {
	usedNames := make(map[string]int)

	// First file: model.glb
	asset1 := &database.Asset{AssetID: "hash1", Extension: "glb", OriginName: "model"}
	result1 := buildFilename(asset1, constants.FilenameFormatOriginal, usedNames)
	if result1 != "model.glb" {
		t.Errorf("First file: expected %q, got %q", "model.glb", result1)
	}

	// Second file with same name: model_2.glb
	asset2 := &database.Asset{AssetID: "hash2", Extension: "glb", OriginName: "model"}
	result2 := buildFilename(asset2, constants.FilenameFormatOriginal, usedNames)
	if result2 != "model_2.glb" {
		t.Errorf("Second file: expected %q, got %q", "model_2.glb", result2)
	}

	// Third file with same name: model_3.glb
	asset3 := &database.Asset{AssetID: "hash3", Extension: "glb", OriginName: "model"}
	result3 := buildFilename(asset3, constants.FilenameFormatOriginal, usedNames)
	if result3 != "model_3.glb" {
		t.Errorf("Third file: expected %q, got %q", "model_3.glb", result3)
	}
}

func TestBuildFilename_CollisionWithoutExtension(t *testing.T) {
	usedNames := make(map[string]int)

	// First file: LICENSE (no extension)
	asset1 := &database.Asset{AssetID: "hash1", OriginName: "LICENSE"}
	result1 := buildFilename(asset1, constants.FilenameFormatOriginal, usedNames)
	if result1 != "LICENSE" {
		t.Errorf("First file: expected %q, got %q", "LICENSE", result1)
	}

	// Second file with same name: LICENSE_2
	asset2 := &database.Asset{AssetID: "hash2", OriginName: "LICENSE"}
	result2 := buildFilename(asset2, constants.FilenameFormatOriginal, usedNames)
	if result2 != "LICENSE_2" {
		t.Errorf("Second file: expected %q, got %q", "LICENSE_2", result2)
	}
}

func TestBuildFilename_NoCollisionForHashFormat(t *testing.T) {
	usedNames := make(map[string]int)

	// Hash format should not track collisions (hashes are unique)
	asset1 := &database.Asset{AssetID: "hash1", Extension: "glb", OriginName: "model"}
	asset2 := &database.Asset{AssetID: "hash2", Extension: "glb", OriginName: "model"}

	result1 := buildFilename(asset1, constants.FilenameFormatHash, usedNames)
	result2 := buildFilename(asset2, constants.FilenameFormatHash, usedNames)

	if result1 != "hash1.glb" {
		t.Errorf("Expected %q, got %q", "hash1.glb", result1)
	}
	if result2 != "hash2.glb" {
		t.Errorf("Expected %q, got %q", "hash2.glb", result2)
	}
}

func TestBuildFilename_SanitizesMaliciousOriginName(t *testing.T) {
	tests := []struct {
		name       string
		asset      *database.Asset
		format     string
		notContain []string // result must not contain these substrings
	}{
		{
			name: "original format with path traversal",
			asset: &database.Asset{
				AssetID:    "abc123",
				Extension:  "glb",
				OriginName: "../../../etc/passwd",
			},
			format:     constants.FilenameFormatOriginal,
			notContain: []string{"..", "/", "\\"},
		},
		{
			name: "hash_original format with path traversal",
			asset: &database.Asset{
				AssetID:    "abc123",
				Extension:  "glb",
				OriginName: "../../../etc/passwd",
			},
			format:     constants.FilenameFormatHashOriginal,
			notContain: []string{"..", "/", "\\"},
		},
		{
			name: "original format with null byte",
			asset: &database.Asset{
				AssetID:    "abc123",
				Extension:  "glb",
				OriginName: "model\x00evil",
			},
			format:     constants.FilenameFormatOriginal,
			notContain: []string{"\x00"},
		},
		{
			name: "original format with control chars",
			asset: &database.Asset{
				AssetID:    "abc123",
				Extension:  "txt",
				OriginName: "file\r\nX-Evil: yes",
			},
			format:     constants.FilenameFormatOriginal,
			notContain: []string{"\r", "\n"},
		},
		{
			name: "windows path traversal",
			asset: &database.Asset{
				AssetID:    "abc123",
				Extension:  "dll",
				OriginName: "..\\..\\windows\\system32",
			},
			format:     constants.FilenameFormatOriginal,
			notContain: []string{"..", "\\", "/"},
		},
		{
			name: "original format with empty origin after sanitization falls back to hash",
			asset: &database.Asset{
				AssetID:    "abc123",
				Extension:  "txt",
				OriginName: "...",
			},
			format:     constants.FilenameFormatOriginal,
			notContain: []string{"..."},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			usedNames := make(map[string]int)
			result := buildFilename(tc.asset, tc.format, usedNames)

			for _, forbidden := range tc.notContain {
				if strings.Contains(result, forbidden) {
					t.Errorf("buildFilename result %q contains forbidden substring %q", result, forbidden)
				}
			}

			// Result must not be empty
			if result == "" {
				t.Error("buildFilename returned empty string")
			}
		})
	}
}

func TestMetadataFilename_MatchesAssetFilename(t *testing.T) {
	tests := []struct {
		name             string
		asset            *database.Asset
		format           string
		expectedAsset    string // expected asset filename
		expectedMetadata string // expected metadata filename (without metadata/ prefix)
	}{
		{
			name: "original format",
			asset: &database.Asset{
				AssetID:    "abc123",
				Extension:  "glb",
				OriginName: "model",
			},
			format:           constants.FilenameFormatOriginal,
			expectedAsset:    "model.glb",
			expectedMetadata: "model.json",
		},
		{
			name: "hash format",
			asset: &database.Asset{
				AssetID:    "abc123",
				Extension:  "glb",
				OriginName: "model",
			},
			format:           constants.FilenameFormatHash,
			expectedAsset:    "abc123.glb",
			expectedMetadata: "abc123.json",
		},
		{
			name: "hash_original format",
			asset: &database.Asset{
				AssetID:    "abc123",
				Extension:  "glb",
				OriginName: "model",
			},
			format:           constants.FilenameFormatHashOriginal,
			expectedAsset:    "abc123_model.glb",
			expectedMetadata: "abc123_model.json",
		},
		{
			name: "original format without extension",
			asset: &database.Asset{
				AssetID:    "abc123",
				OriginName: "LICENSE",
			},
			format:           constants.FilenameFormatOriginal,
			expectedAsset:    "LICENSE",
			expectedMetadata: "LICENSE.json",
		},
		{
			name: "original format fallback to hash when origin empty",
			asset: &database.Asset{
				AssetID:   "abc123",
				Extension: "bin",
			},
			format:           constants.FilenameFormatOriginal,
			expectedAsset:    "abc123.bin",
			expectedMetadata: "abc123.json",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			usedNames := make(map[string]int)
			filename := buildFilename(tc.asset, tc.format, usedNames)

			if filename != tc.expectedAsset {
				t.Errorf("asset filename: got %q, want %q", filename, tc.expectedAsset)
			}

			// Derive metadata filename the same way buildZIPArchive does
			metadataBaseName := filename
			cleanExt := sanitize.Extension(tc.asset.Extension)
			if cleanExt != "" {
				metadataBaseName = strings.TrimSuffix(filename, "."+cleanExt)
			}
			metadataFilename := metadataBaseName + ".json"

			if metadataFilename != tc.expectedMetadata {
				t.Errorf("metadata filename: got %q, want %q", metadataFilename, tc.expectedMetadata)
			}
		})
	}
}

func TestWriteManifestToZip(t *testing.T) {
	t.Run("writes correct JSON content", func(t *testing.T) {
		var buf bytes.Buffer
		zipWriter := zip.NewWriter(&buf)

		manifest := BulkDownloadManifest{
			CreatedAt:       1700000000,
			AssetCount:      2,
			TotalSize:       3072,
			IncludeMetadata: true,
			Assets: []ManifestAsset{
				{Hash: "hash1", Filename: "assets/file1.glb", Size: 1024, Extension: "glb", OriginName: "file1", Topic: "test"},
				{Hash: "hash2", Filename: "assets/file2.glb", Size: 2048, Extension: "glb", OriginName: "file2", Topic: "test"},
			},
			FailedAssets: []FailedAsset{},
		}

		err := writeManifestToZip(zipWriter, manifest)
		if err != nil {
			t.Fatalf("writeManifestToZip failed: %v", err)
		}

		if err := zipWriter.Close(); err != nil {
			t.Fatalf("Failed to close zip writer: %v", err)
		}

		// Read back the ZIP
		reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		if err != nil {
			t.Fatalf("Failed to open ZIP reader: %v", err)
		}

		if len(reader.File) != 1 {
			t.Fatalf("Expected 1 file in ZIP, got %d", len(reader.File))
		}

		// Verify filename
		if reader.File[0].Name != constants.ManifestFilename {
			t.Errorf("Expected filename %q, got %q", constants.ManifestFilename, reader.File[0].Name)
		}

		// Verify Store method (no compression)
		if reader.File[0].Method != zip.Store {
			t.Errorf("Expected Store method, got %d", reader.File[0].Method)
		}

		// Verify JSON content
		rc, err := reader.File[0].Open()
		if err != nil {
			t.Fatalf("Failed to open manifest entry: %v", err)
		}
		defer rc.Close()

		var decoded BulkDownloadManifest
		if err := json.NewDecoder(rc).Decode(&decoded); err != nil {
			t.Fatalf("Failed to decode manifest JSON: %v", err)
		}

		if decoded.AssetCount != 2 {
			t.Errorf("Expected AssetCount 2, got %d", decoded.AssetCount)
		}
		if decoded.TotalSize != 3072 {
			t.Errorf("Expected TotalSize 3072, got %d", decoded.TotalSize)
		}
		if len(decoded.Assets) != 2 {
			t.Errorf("Expected 2 assets, got %d", len(decoded.Assets))
		}
		if !decoded.IncludeMetadata {
			t.Error("Expected IncludeMetadata to be true")
		}
	})

	t.Run("empty assets list", func(t *testing.T) {
		var buf bytes.Buffer
		zipWriter := zip.NewWriter(&buf)

		manifest := BulkDownloadManifest{
			CreatedAt:       1700000000,
			AssetCount:      0,
			TotalSize:       0,
			IncludeMetadata: false,
			Assets:          []ManifestAsset{},
			FailedAssets:    []FailedAsset{},
		}

		err := writeManifestToZip(zipWriter, manifest)
		if err != nil {
			t.Fatalf("writeManifestToZip failed: %v", err)
		}

		if err := zipWriter.Close(); err != nil {
			t.Fatalf("Failed to close zip writer: %v", err)
		}

		reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		if err != nil {
			t.Fatalf("Failed to open ZIP reader: %v", err)
		}

		rc, err := reader.File[0].Open()
		if err != nil {
			t.Fatalf("Failed to open manifest entry: %v", err)
		}
		defer rc.Close()

		var decoded BulkDownloadManifest
		if err := json.NewDecoder(rc).Decode(&decoded); err != nil {
			t.Fatalf("Failed to decode manifest JSON: %v", err)
		}

		if decoded.AssetCount != 0 {
			t.Errorf("Expected AssetCount 0, got %d", decoded.AssetCount)
		}
		if len(decoded.Assets) != 0 {
			t.Errorf("Expected 0 assets, got %d", len(decoded.Assets))
		}
	})

	t.Run("manifest with failed assets", func(t *testing.T) {
		var buf bytes.Buffer
		zipWriter := zip.NewWriter(&buf)

		manifest := BulkDownloadManifest{
			CreatedAt:       1700000000,
			AssetCount:      1,
			TotalSize:       1024,
			IncludeMetadata: false,
			Assets: []ManifestAsset{
				{Hash: "hash1", Filename: "assets/file1.glb", Size: 1024, Extension: "glb", Topic: "test"},
			},
			FailedAssets: []FailedAsset{
				{Hash: "hash2", Error: "file not found", Topic: "test"},
				{Hash: "hash3", Error: "corrupted data"},
			},
		}

		err := writeManifestToZip(zipWriter, manifest)
		if err != nil {
			t.Fatalf("writeManifestToZip failed: %v", err)
		}

		if err := zipWriter.Close(); err != nil {
			t.Fatalf("Failed to close zip writer: %v", err)
		}

		reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		if err != nil {
			t.Fatalf("Failed to open ZIP reader: %v", err)
		}

		rc, err := reader.File[0].Open()
		if err != nil {
			t.Fatalf("Failed to open manifest entry: %v", err)
		}
		defer rc.Close()

		var decoded BulkDownloadManifest
		if err := json.NewDecoder(rc).Decode(&decoded); err != nil {
			t.Fatalf("Failed to decode manifest JSON: %v", err)
		}

		if len(decoded.FailedAssets) != 2 {
			t.Fatalf("Expected 2 failed assets, got %d", len(decoded.FailedAssets))
		}
		if decoded.FailedAssets[0].Hash != "hash2" {
			t.Errorf("Expected first failed hash %q, got %q", "hash2", decoded.FailedAssets[0].Hash)
		}
		if decoded.FailedAssets[0].Error != "file not found" {
			t.Errorf("Expected first failed error %q, got %q", "file not found", decoded.FailedAssets[0].Error)
		}
		if decoded.FailedAssets[1].Topic != "" {
			t.Errorf("Expected second failed topic to be empty (omitempty), got %q", decoded.FailedAssets[1].Topic)
		}
	})
}
