package services

import (
	"testing"

	"meshbank/internal/constants"
	"meshbank/internal/logger"
)

func TestNewBulkService(t *testing.T) {
	mockApp := newMockAppState()
	log := logger.NewLogger("debug")

	svc := NewBulkService(mockApp, log)

	if svc == nil {
		t.Fatal("NewBulkService returned nil")
	}
	if svc.app != mockApp {
		t.Error("app field not set correctly")
	}
	if svc.logger != log {
		t.Error("logger field not set correctly")
	}
}

func TestBulkService_ValidateRequest(t *testing.T) {
	mockApp := newMockAppState()
	log := logger.NewLogger("debug")
	svc := NewBulkService(mockApp, log)

	tests := []struct {
		name     string
		req      *BulkResolveRequest
		wantErr  bool
		wantCode string
	}{
		{
			name: "valid query mode request",
			req: &BulkResolveRequest{
				Mode:           "query",
				Preset:         "recent-imports",
				FilenameFormat: constants.FilenameFormatHash,
			},
			wantErr: false,
		},
		{
			name: "valid ids mode request",
			req: &BulkResolveRequest{
				Mode:           "ids",
				AssetIDs:       []string{"abc123", "def456"},
				FilenameFormat: constants.FilenameFormatOriginal,
			},
			wantErr: false,
		},
		{
			name: "empty filename format gets default",
			req: &BulkResolveRequest{
				Mode:     "ids",
				AssetIDs: []string{"abc123"},
			},
			wantErr: false,
		},
		{
			name: "invalid filename format",
			req: &BulkResolveRequest{
				Mode:           "ids",
				AssetIDs:       []string{"abc123"},
				FilenameFormat: "invalid",
			},
			wantErr:  true,
			wantCode: constants.ErrCodeInvalidFilenameFormat,
		},
		{
			name: "query mode without preset",
			req: &BulkResolveRequest{
				Mode:           "query",
				FilenameFormat: constants.FilenameFormatHash,
			},
			wantErr:  true,
			wantCode: constants.ErrCodeInvalidRequest,
		},
		{
			name: "ids mode without asset_ids",
			req: &BulkResolveRequest{
				Mode:           "ids",
				FilenameFormat: constants.FilenameFormatHash,
			},
			wantErr:  true,
			wantCode: constants.ErrCodeInvalidRequest,
		},
		{
			name: "ids mode with empty asset_ids",
			req: &BulkResolveRequest{
				Mode:           "ids",
				AssetIDs:       []string{},
				FilenameFormat: constants.FilenameFormatHash,
			},
			wantErr:  true,
			wantCode: constants.ErrCodeInvalidRequest,
		},
		{
			name: "invalid mode",
			req: &BulkResolveRequest{
				Mode:           "invalid",
				FilenameFormat: constants.FilenameFormatHash,
			},
			wantErr:  true,
			wantCode: constants.ErrCodeInvalidDownloadMode,
		},
		{
			name: "empty mode",
			req: &BulkResolveRequest{
				FilenameFormat: constants.FilenameFormatHash,
			},
			wantErr:  true,
			wantCode: constants.ErrCodeInvalidDownloadMode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.ValidateRequest(tt.req)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error but got nil")
				}
				code, ok := IsServiceError(err)
				if !ok {
					t.Fatalf("expected ServiceError but got: %T", err)
				}
				if code != tt.wantCode {
					t.Errorf("error code = %q, want %q", code, tt.wantCode)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestBulkService_ValidateRequest_SetsDefaultFilenameFormat(t *testing.T) {
	mockApp := newMockAppState()
	log := logger.NewLogger("debug")
	svc := NewBulkService(mockApp, log)

	req := &BulkResolveRequest{
		Mode:     "ids",
		AssetIDs: []string{"abc123"},
	}

	err := svc.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if req.FilenameFormat != constants.DefaultFilenameFormat {
		t.Errorf("FilenameFormat = %q, want %q", req.FilenameFormat, constants.DefaultFilenameFormat)
	}
}

func TestBulkService_ValidateAssetCount(t *testing.T) {
	mockApp := newMockAppState()
	log := logger.NewLogger("debug")
	svc := NewBulkService(mockApp, log)

	tests := []struct {
		name     string
		count    int
		wantErr  bool
		wantCode string
	}{
		{
			name:    "valid count",
			count:   100,
			wantErr: false,
		},
		{
			name:    "single asset",
			count:   1,
			wantErr: false,
		},
		{
			name:    "maximum allowed",
			count:   constants.BulkDownloadMaxAssets,
			wantErr: false,
		},
		{
			name:     "zero assets",
			count:    0,
			wantErr:  true,
			wantCode: constants.ErrCodeBulkDownloadEmpty,
		},
		{
			name:     "exceeds maximum",
			count:    constants.BulkDownloadMaxAssets + 1,
			wantErr:  true,
			wantCode: constants.ErrCodeBulkDownloadTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.ValidateAssetCount(tt.count)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error but got nil")
				}
				code, ok := IsServiceError(err)
				if !ok {
					t.Fatalf("expected ServiceError but got: %T", err)
				}
				if code != tt.wantCode {
					t.Errorf("error code = %q, want %q", code, tt.wantCode)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestBulkService_FilenameFormatValidation(t *testing.T) {
	mockApp := newMockAppState()
	log := logger.NewLogger("debug")
	svc := NewBulkService(mockApp, log)

	validFormats := []string{
		constants.FilenameFormatHash,
		constants.FilenameFormatOriginal,
		constants.FilenameFormatHashOriginal,
	}

	invalidFormats := []string{
		"",
		"invalid",
		"HASH",
		"Original",
		"hash-original",
		"hash_Original",
	}

	for _, format := range validFormats {
		t.Run("valid_"+format, func(t *testing.T) {
			req := &BulkResolveRequest{
				Mode:           "ids",
				AssetIDs:       []string{"abc123"},
				FilenameFormat: format,
			}
			err := svc.ValidateRequest(req)
			if err != nil {
				t.Errorf("expected valid format %q but got error: %v", format, err)
			}
		})
	}

	for _, format := range invalidFormats {
		if format == "" {
			continue // empty gets default, tested elsewhere
		}
		t.Run("invalid_"+format, func(t *testing.T) {
			req := &BulkResolveRequest{
				Mode:           "ids",
				AssetIDs:       []string{"abc123"},
				FilenameFormat: format,
			}
			err := svc.ValidateRequest(req)
			if err == nil {
				t.Errorf("expected error for invalid format %q but got nil", format)
			}
			code, _ := IsServiceError(err)
			if code != constants.ErrCodeInvalidFilenameFormat {
				t.Errorf("expected error code %q but got %q", constants.ErrCodeInvalidFilenameFormat, code)
			}
		})
	}
}

func TestBulkService_ResolveAssets_InvalidMode(t *testing.T) {
	mockApp := newMockAppState()
	log := logger.NewLogger("debug")
	svc := NewBulkService(mockApp, log)

	req := &BulkResolveRequest{
		Mode: "invalid",
	}

	_, err := svc.ResolveAssets(req)
	if err == nil {
		t.Fatal("expected error but got nil")
	}

	code, ok := IsServiceError(err)
	if !ok {
		t.Fatalf("expected ServiceError but got: %T", err)
	}
	if code != constants.ErrCodeInvalidDownloadMode {
		t.Errorf("error code = %q, want %q", code, constants.ErrCodeInvalidDownloadMode)
	}
}

func TestBulkService_ResolveAssets_QueryModeNoConfig(t *testing.T) {
	mockApp := newMockAppState()
	mockApp.queriesConfig = nil // ensure not configured
	log := logger.NewLogger("debug")
	svc := NewBulkService(mockApp, log)

	req := &BulkResolveRequest{
		Mode:   "query",
		Preset: "recent-imports",
	}

	_, err := svc.ResolveAssets(req)
	if err == nil {
		t.Fatal("expected error but got nil")
	}

	code, ok := IsServiceError(err)
	if !ok {
		t.Fatalf("expected ServiceError but got: %T", err)
	}
	if code != constants.ErrCodeNotConfigured {
		t.Errorf("error code = %q, want %q", code, constants.ErrCodeNotConfigured)
	}
}

func TestBulkService_ResolveAssets_IDsModeEmptyIDs(t *testing.T) {
	mockApp := newMockAppState()
	log := logger.NewLogger("debug")
	svc := NewBulkService(mockApp, log)

	req := &BulkResolveRequest{
		Mode:     "ids",
		AssetIDs: []string{},
	}

	_, err := svc.ResolveAssets(req)
	if err == nil {
		t.Fatal("expected error but got nil")
	}

	code, ok := IsServiceError(err)
	if !ok {
		t.Fatalf("expected ServiceError but got: %T", err)
	}
	if code != constants.ErrCodeInvalidRequest {
		t.Errorf("error code = %q, want %q", code, constants.ErrCodeInvalidRequest)
	}
}

func TestResolvedAsset_Struct(t *testing.T) {
	// Test that ResolvedAsset struct is correctly defined
	asset := &ResolvedAsset{
		Hash:      "abc123",
		Topic:     "test-topic",
		TopicPath: "/path/to/topic",
	}

	if asset.Hash != "abc123" {
		t.Errorf("Hash = %q, want %q", asset.Hash, "abc123")
	}
	if asset.Topic != "test-topic" {
		t.Errorf("Topic = %q, want %q", asset.Topic, "test-topic")
	}
	if asset.TopicPath != "/path/to/topic" {
		t.Errorf("TopicPath = %q, want %q", asset.TopicPath, "/path/to/topic")
	}
}

func TestBulkResolveRequest_Struct(t *testing.T) {
	// Test that BulkResolveRequest struct is correctly defined
	req := &BulkResolveRequest{
		Mode:           "query",
		Preset:         "recent-imports",
		Params:         map[string]interface{}{"limit": 10},
		Topics:         []string{"topic1", "topic2"},
		AssetIDs:       []string{"hash1", "hash2"},
		FilenameFormat: constants.FilenameFormatHash,
	}

	if req.Mode != "query" {
		t.Errorf("Mode = %q, want %q", req.Mode, "query")
	}
	if req.Preset != "recent-imports" {
		t.Errorf("Preset = %q, want %q", req.Preset, "recent-imports")
	}
	if req.FilenameFormat != constants.FilenameFormatHash {
		t.Errorf("FilenameFormat = %q, want %q", req.FilenameFormat, constants.FilenameFormatHash)
	}
	if len(req.Topics) != 2 {
		t.Errorf("Topics length = %d, want %d", len(req.Topics), 2)
	}
	if len(req.AssetIDs) != 2 {
		t.Errorf("AssetIDs length = %d, want %d", len(req.AssetIDs), 2)
	}
}
