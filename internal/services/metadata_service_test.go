package services

import (
	"database/sql"
	"strings"
	"testing"

	"meshbank/internal/constants"
	"meshbank/internal/logger"

	_ "github.com/mattn/go-sqlite3"
)

func TestNewMetadataService(t *testing.T) {
	mockApp := newMockAppState()
	log := logger.NewLogger("debug")

	svc := NewMetadataService(mockApp, log)

	if svc == nil {
		t.Fatal("NewMetadataService returned nil")
	}
	if svc.app != mockApp {
		t.Error("app field not set correctly")
	}
	if svc.logger != log {
		t.Error("logger field not set correctly")
	}
}

func TestMetadataService_Get_InvalidHash(t *testing.T) {
	mockApp := newMockAppState()
	log := logger.NewLogger("debug")
	svc := NewMetadataService(mockApp, log)

	tests := []struct {
		name string
		hash string
	}{
		{"empty hash", ""},
		{"too short", "abc123"},
		{"too long", strings.Repeat("a", 65)},
		{"wrong length", strings.Repeat("a", 32)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Get(tt.hash)
			if err == nil {
				t.Fatal("expected error but got nil")
			}

			code, ok := IsServiceError(err)
			if !ok {
				t.Fatalf("expected ServiceError but got: %T", err)
			}
			if code != constants.ErrCodeInvalidHash {
				t.Errorf("error code = %q, want %q", code, constants.ErrCodeInvalidHash)
			}
		})
	}
}

func TestMetadataService_Get_AssetNotFound(t *testing.T) {
	mockApp := newMockAppState()
	log := logger.NewLogger("debug")

	// Create in-memory orchestrator database
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer db.Close()

	// Create asset_index table (empty)
	_, err = db.Exec(`CREATE TABLE asset_index (hash TEXT PRIMARY KEY, topic TEXT, dat_file TEXT)`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	mockApp.orchestratorDB = db
	svc := NewMetadataService(mockApp, log)

	validHash := strings.Repeat("a", constants.HashLength)
	_, err = svc.Get(validHash)
	if err == nil {
		t.Fatal("expected error but got nil")
	}

	code, ok := IsServiceError(err)
	if !ok {
		t.Fatalf("expected ServiceError but got: %T", err)
	}
	if code != constants.ErrCodeAssetNotFound {
		t.Errorf("error code = %q, want %q", code, constants.ErrCodeAssetNotFound)
	}
}

func TestMetadataService_Get_TopicUnhealthy(t *testing.T) {
	mockApp := newMockAppState()
	log := logger.NewLogger("debug")

	// Create in-memory orchestrator database
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer db.Close()

	// Create asset_index table with an entry
	_, err = db.Exec(`CREATE TABLE asset_index (hash TEXT PRIMARY KEY, topic TEXT, dat_file TEXT)`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	validHash := strings.Repeat("a", constants.HashLength)
	_, err = db.Exec(`INSERT INTO asset_index (hash, topic, dat_file) VALUES (?, 'unhealthy-topic', 'data.001.dat')`, validHash)
	if err != nil {
		t.Fatalf("failed to insert data: %v", err)
	}

	mockApp.orchestratorDB = db
	mockApp.RegisterTopic("unhealthy-topic", false, "missing index file")
	svc := NewMetadataService(mockApp, log)

	_, err = svc.Get(validHash)
	if err == nil {
		t.Fatal("expected error but got nil")
	}

	code, ok := IsServiceError(err)
	if !ok {
		t.Fatalf("expected ServiceError but got: %T", err)
	}
	if code != constants.ErrCodeTopicUnhealthy {
		t.Errorf("error code = %q, want %q", code, constants.ErrCodeTopicUnhealthy)
	}
}

func TestMetadataService_Set_InvalidHash(t *testing.T) {
	mockApp := newMockAppState()
	log := logger.NewLogger("debug")
	svc := NewMetadataService(mockApp, log)

	tests := []struct {
		name string
		hash string
	}{
		{"empty hash", ""},
		{"too short", "abc123"},
		{"too long", strings.Repeat("a", 65)},
		{"wrong length", strings.Repeat("a", 32)},
	}

	req := &MetadataSetRequest{
		Op:               constants.BatchMetadataOpSet,
		Key:              "test-key",
		Value:            "test-value",
		Processor:        "test",
		ProcessorVersion: "1.0",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Set(tt.hash, req)
			if err == nil {
				t.Fatal("expected error but got nil")
			}

			code, ok := IsServiceError(err)
			if !ok {
				t.Fatalf("expected ServiceError but got: %T", err)
			}
			if code != constants.ErrCodeInvalidHash {
				t.Errorf("error code = %q, want %q", code, constants.ErrCodeInvalidHash)
			}
		})
	}
}

func TestMetadataService_validateMetadataRequest(t *testing.T) {
	mockApp := newMockAppState()
	log := logger.NewLogger("debug")
	svc := NewMetadataService(mockApp, log)

	tests := []struct {
		name     string
		req      *MetadataSetRequest
		wantErr  bool
		wantCode string
	}{
		{
			name: "valid set request",
			req: &MetadataSetRequest{
				Op:               constants.BatchMetadataOpSet,
				Key:              "test-key",
				Value:            "test-value",
				Processor:        "api",
				ProcessorVersion: "1.0",
			},
			wantErr: false,
		},
		{
			name: "valid delete request",
			req: &MetadataSetRequest{
				Op:               constants.BatchMetadataOpDelete,
				Key:              "test-key",
				Processor:        "api",
				ProcessorVersion: "1.0",
			},
			wantErr: false,
		},
		{
			name: "invalid op",
			req: &MetadataSetRequest{
				Op:               "update",
				Key:              "test-key",
				Processor:        "api",
				ProcessorVersion: "1.0",
			},
			wantErr:  true,
			wantCode: constants.ErrCodeInvalidRequest,
		},
		{
			name: "empty op",
			req: &MetadataSetRequest{
				Key:              "test-key",
				Processor:        "api",
				ProcessorVersion: "1.0",
			},
			wantErr:  true,
			wantCode: constants.ErrCodeInvalidRequest,
		},
		{
			name: "empty key",
			req: &MetadataSetRequest{
				Op:               constants.BatchMetadataOpSet,
				Processor:        "api",
				ProcessorVersion: "1.0",
			},
			wantErr:  true,
			wantCode: constants.ErrCodeInvalidRequest,
		},
		{
			name: "key too long",
			req: &MetadataSetRequest{
				Op:               constants.BatchMetadataOpSet,
				Key:              strings.Repeat("a", constants.MaxMetadataKeyLength+1),
				Processor:        "api",
				ProcessorVersion: "1.0",
			},
			wantErr:  true,
			wantCode: constants.ErrCodeMetadataKeyTooLong,
		},
		{
			name: "missing processor",
			req: &MetadataSetRequest{
				Op:               constants.BatchMetadataOpSet,
				Key:              "test-key",
				ProcessorVersion: "1.0",
			},
			wantErr:  true,
			wantCode: constants.ErrCodeInvalidRequest,
		},
		{
			name: "missing processor_version",
			req: &MetadataSetRequest{
				Op:        constants.BatchMetadataOpSet,
				Key:       "test-key",
				Processor: "api",
			},
			wantErr:  true,
			wantCode: constants.ErrCodeInvalidRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.validateMetadataRequest(tt.req)

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

func TestMetadataService_convertValueToString(t *testing.T) {
	mockApp := newMockAppState()
	log := logger.NewLogger("debug")
	svc := NewMetadataService(mockApp, log)

	tests := []struct {
		name      string
		op        string
		value     interface{}
		wantStr   string
		wantErr   bool
		wantCode  string
	}{
		{
			name:    "string value",
			op:      constants.BatchMetadataOpSet,
			value:   "hello world",
			wantStr: "hello world",
			wantErr: false,
		},
		{
			name:    "integer as float64 (JSON unmarshal)",
			op:      constants.BatchMetadataOpSet,
			value:   float64(42),
			wantStr: "42",
			wantErr: false,
		},
		{
			name:    "float value",
			op:      constants.BatchMetadataOpSet,
			value:   float64(3.14159),
			wantStr: "3.14159",
			wantErr: false,
		},
		{
			name:    "boolean true",
			op:      constants.BatchMetadataOpSet,
			value:   true,
			wantStr: "true",
			wantErr: false,
		},
		{
			name:    "boolean false",
			op:      constants.BatchMetadataOpSet,
			value:   false,
			wantStr: "false",
			wantErr: false,
		},
		{
			name:    "delete op ignores value",
			op:      constants.BatchMetadataOpDelete,
			value:   nil,
			wantStr: "",
			wantErr: false,
		},
		{
			name:     "set op with nil value",
			op:       constants.BatchMetadataOpSet,
			value:    nil,
			wantErr:  true,
			wantCode: constants.ErrCodeInvalidRequest,
		},
		{
			name:     "unsupported type (slice)",
			op:       constants.BatchMetadataOpSet,
			value:    []string{"a", "b"},
			wantErr:  true,
			wantCode: constants.ErrCodeInvalidRequest,
		},
		{
			name:     "unsupported type (map)",
			op:       constants.BatchMetadataOpSet,
			value:    map[string]string{"key": "value"},
			wantErr:  true,
			wantCode: constants.ErrCodeInvalidRequest,
		},
		{
			name:     "value too long",
			op:       constants.BatchMetadataOpSet,
			value:    strings.Repeat("a", constants.MaxMetadataValueBytes+1),
			wantErr:  true,
			wantCode: constants.ErrCodeMetadataValueTooLong,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := svc.convertValueToString(tt.op, tt.value)

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
				if result != tt.wantStr {
					t.Errorf("result = %q, want %q", result, tt.wantStr)
				}
			}
		})
	}
}

func TestMetadataService_ValidateKeyLength(t *testing.T) {
	mockApp := newMockAppState()
	log := logger.NewLogger("debug")
	svc := NewMetadataService(mockApp, log)

	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{"valid short key", "key", false},
		{"valid max length key", strings.Repeat("a", constants.MaxMetadataKeyLength), false},
		{"key too long", strings.Repeat("a", constants.MaxMetadataKeyLength+1), true},
		{"empty key", "", false}, // Empty is valid length-wise, validation happens elsewhere
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.ValidateKeyLength(tt.key)
			if tt.wantErr && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestMetadataService_ValidateValueForBatch(t *testing.T) {
	mockApp := newMockAppState()
	log := logger.NewLogger("debug")
	svc := NewMetadataService(mockApp, log)

	// This is essentially a wrapper for convertValueToString, so basic test
	result, err := svc.ValidateValueForBatch(constants.BatchMetadataOpSet, "test-value")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "test-value" {
		t.Errorf("result = %q, want %q", result, "test-value")
	}

	// Test delete op
	result, err = svc.ValidateValueForBatch(constants.BatchMetadataOpDelete, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty string", result)
	}
}

func TestMetadataService_GetTopicForHash_NotFound(t *testing.T) {
	mockApp := newMockAppState()
	log := logger.NewLogger("debug")

	// Create in-memory orchestrator database
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer db.Close()

	// Create asset_index table (empty)
	_, err = db.Exec(`CREATE TABLE asset_index (hash TEXT PRIMARY KEY, topic TEXT, dat_file TEXT)`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	mockApp.orchestratorDB = db
	svc := NewMetadataService(mockApp, log)

	validHash := strings.Repeat("a", constants.HashLength)
	_, err = svc.GetTopicForHash(validHash)
	if err == nil {
		t.Fatal("expected error but got nil")
	}

	code, ok := IsServiceError(err)
	if !ok {
		t.Fatalf("expected ServiceError but got: %T", err)
	}
	if code != constants.ErrCodeAssetNotFound {
		t.Errorf("error code = %q, want %q", code, constants.ErrCodeAssetNotFound)
	}
}

func TestMetadataService_GetTopicForHash_Found(t *testing.T) {
	mockApp := newMockAppState()
	log := logger.NewLogger("debug")

	// Create in-memory orchestrator database
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer db.Close()

	// Create asset_index table with an entry
	_, err = db.Exec(`CREATE TABLE asset_index (hash TEXT PRIMARY KEY, topic TEXT, dat_file TEXT)`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	validHash := strings.Repeat("a", constants.HashLength)
	_, err = db.Exec(`INSERT INTO asset_index (hash, topic, dat_file) VALUES (?, 'test-topic', 'data.001.dat')`, validHash)
	if err != nil {
		t.Fatalf("failed to insert data: %v", err)
	}

	mockApp.orchestratorDB = db
	svc := NewMetadataService(mockApp, log)

	topicName, err := svc.GetTopicForHash(validHash)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if topicName != "test-topic" {
		t.Errorf("topicName = %q, want %q", topicName, "test-topic")
	}
}

func TestAssetMetadata_Struct(t *testing.T) {
	metadata := &AssetMetadata{
		ComputedMetadata: map[string]interface{}{
			"width":  float64(1920),
			"height": float64(1080),
		},
		MetadataWithProcessor: nil,
	}
	metadata.Asset.OriginName = "test.png"
	metadata.Asset.Extension = ".png"
	metadata.Asset.Size = 12345
	metadata.Asset.CreatedAt = 1234567890

	if metadata.Asset.OriginName != "test.png" {
		t.Errorf("OriginName = %q, want %q", metadata.Asset.OriginName, "test.png")
	}
	if metadata.Asset.Extension != ".png" {
		t.Errorf("Extension = %q, want %q", metadata.Asset.Extension, ".png")
	}
	if metadata.Asset.Size != 12345 {
		t.Errorf("Size = %d, want 12345", metadata.Asset.Size)
	}
	if metadata.ComputedMetadata["width"] != float64(1920) {
		t.Errorf("ComputedMetadata[width] = %v, want 1920", metadata.ComputedMetadata["width"])
	}
}

func TestMetadataSetRequest_Struct(t *testing.T) {
	req := &MetadataSetRequest{
		Op:               constants.BatchMetadataOpSet,
		Key:              "test-key",
		Value:            "test-value",
		Processor:        "api",
		ProcessorVersion: "1.0",
	}

	if req.Op != constants.BatchMetadataOpSet {
		t.Errorf("Op = %q, want %q", req.Op, constants.BatchMetadataOpSet)
	}
	if req.Key != "test-key" {
		t.Errorf("Key = %q, want %q", req.Key, "test-key")
	}
	if req.Value != "test-value" {
		t.Errorf("Value = %v, want %q", req.Value, "test-value")
	}
	if req.Processor != "api" {
		t.Errorf("Processor = %q, want %q", req.Processor, "api")
	}
	if req.ProcessorVersion != "1.0" {
		t.Errorf("ProcessorVersion = %q, want %q", req.ProcessorVersion, "1.0")
	}
}

func TestMetadataSetResult_Struct(t *testing.T) {
	result := &MetadataSetResult{
		LogID: 42,
		ComputedMetadata: map[string]interface{}{
			"key1": "value1",
			"key2": float64(100),
		},
	}

	if result.LogID != 42 {
		t.Errorf("LogID = %d, want 42", result.LogID)
	}
	if result.ComputedMetadata["key1"] != "value1" {
		t.Errorf("ComputedMetadata[key1] = %v, want %q", result.ComputedMetadata["key1"], "value1")
	}
}
