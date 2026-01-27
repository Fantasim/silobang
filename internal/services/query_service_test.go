package services

import (
	"testing"

	"silobang/internal/constants"
	"silobang/internal/logger"
	"silobang/internal/queries"
)

func TestNewQueryService(t *testing.T) {
	mockApp := newMockAppState()
	log := logger.NewLogger("debug")

	svc := NewQueryService(mockApp, log)

	if svc == nil {
		t.Fatal("NewQueryService returned nil")
	}
	if svc.app != mockApp {
		t.Error("app field not set correctly")
	}
	if svc.logger != log {
		t.Error("logger field not set correctly")
	}
}

func TestQueryService_ListPresets_NotConfigured(t *testing.T) {
	mockApp := newMockAppState()
	mockApp.queriesConfig = nil // ensure not configured
	log := logger.NewLogger("debug")
	svc := NewQueryService(mockApp, log)

	_, err := svc.ListPresets()
	if err == nil {
		t.Fatal("expected error but got nil")
	}

	code, ok := IsServiceError(err)
	if !ok {
		t.Fatalf("expected ServiceError but got: %T", err)
	}
	if code != constants.ErrCodeInternalError {
		t.Errorf("error code = %q, want %q", code, constants.ErrCodeInternalError)
	}
}

func TestQueryService_Execute_NotConfigured(t *testing.T) {
	mockApp := newMockAppState()
	mockApp.workingDir = "" // not configured
	log := logger.NewLogger("debug")
	svc := NewQueryService(mockApp, log)

	_, _, err := svc.Execute("recent-imports", &QueryRequest{})
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

func TestQueryService_Execute_NoQueriesConfig(t *testing.T) {
	mockApp := newMockAppState()
	mockApp.workingDir = "/some/path"
	mockApp.queriesConfig = nil
	log := logger.NewLogger("debug")
	svc := NewQueryService(mockApp, log)

	_, _, err := svc.Execute("recent-imports", &QueryRequest{})
	if err == nil {
		t.Fatal("expected error but got nil")
	}

	code, ok := IsServiceError(err)
	if !ok {
		t.Fatalf("expected ServiceError but got: %T", err)
	}
	if code != constants.ErrCodeInternalError {
		t.Errorf("error code = %q, want %q", code, constants.ErrCodeInternalError)
	}
}

func TestQueryService_Execute_PresetNotFound(t *testing.T) {
	mockApp := newMockAppState()
	mockApp.workingDir = "/some/path"
	// Create a queries config with no presets
	mockApp.queriesConfig = &queries.QueriesConfig{
		Presets: map[string]queries.Preset{},
	}
	log := logger.NewLogger("debug")
	svc := NewQueryService(mockApp, log)

	_, _, err := svc.Execute("nonexistent-preset", &QueryRequest{})
	if err == nil {
		t.Fatal("expected error but got nil")
	}

	code, ok := IsServiceError(err)
	if !ok {
		t.Fatalf("expected ServiceError but got: %T", err)
	}
	if code != constants.ErrCodePresetNotFound {
		t.Errorf("error code = %q, want %q", code, constants.ErrCodePresetNotFound)
	}
}

func TestQueryService_Execute_NoTopics(t *testing.T) {
	mockApp := newMockAppState()
	mockApp.workingDir = "/some/path"
	// Create queries config with a preset
	mockApp.queriesConfig = &queries.QueriesConfig{
		Presets: map[string]queries.Preset{
			"test-query": {
				Description: "Test query",
				SQL:         "SELECT * FROM assets",
			},
		},
	}
	log := logger.NewLogger("debug")
	svc := NewQueryService(mockApp, log)

	// Execute with no topics registered
	result, topics, err := svc.Execute("test-query", &QueryRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected result but got nil")
	}
	if result.RowCount != 0 {
		t.Errorf("RowCount = %d, want 0", result.RowCount)
	}
	if len(topics) != 0 {
		t.Errorf("topics length = %d, want 0", len(topics))
	}
}

func TestQueryService_Execute_NilRequest(t *testing.T) {
	mockApp := newMockAppState()
	mockApp.workingDir = "/some/path"
	// Create queries config with a preset
	mockApp.queriesConfig = &queries.QueriesConfig{
		Presets: map[string]queries.Preset{
			"test-query": {
				Description: "Test query",
				SQL:         "SELECT * FROM assets",
			},
		},
	}
	log := logger.NewLogger("debug")
	svc := NewQueryService(mockApp, log)

	// Execute with nil request
	result, _, err := svc.Execute("test-query", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected result but got nil")
	}
}

func TestQueryRequest_Struct(t *testing.T) {
	req := &QueryRequest{
		Params: map[string]interface{}{
			"limit": 10,
			"key":   "value",
		},
		Topics: []string{"topic1", "topic2"},
	}

	if req.Params == nil {
		t.Error("Params should not be nil")
	}
	if req.Params["limit"] != 10 {
		t.Errorf("Params[limit] = %v, want 10", req.Params["limit"])
	}
	if len(req.Topics) != 2 {
		t.Errorf("Topics length = %d, want 2", len(req.Topics))
	}
	if req.Topics[0] != "topic1" {
		t.Errorf("Topics[0] = %q, want %q", req.Topics[0], "topic1")
	}
}

func TestQueryService_Execute_WithMissingRequiredParams(t *testing.T) {
	mockApp := newMockAppState()
	mockApp.workingDir = "/some/path"
	// Create queries config with a preset that has required params
	mockApp.queriesConfig = &queries.QueriesConfig{
		Presets: map[string]queries.Preset{
			"test-query": {
				Description: "Test query with required param",
				SQL:         "SELECT * FROM assets WHERE key = :required_param",
				Params: []queries.PresetParam{
					{Name: "required_param", Required: true},
				},
			},
		},
	}
	log := logger.NewLogger("debug")
	svc := NewQueryService(mockApp, log)

	// Execute without the required param
	_, _, err := svc.Execute("test-query", &QueryRequest{})
	if err == nil {
		t.Fatal("expected error but got nil")
	}

	code, ok := IsServiceError(err)
	if !ok {
		t.Fatalf("expected ServiceError but got: %T", err)
	}
	if code != constants.ErrCodeMissingParam {
		t.Errorf("error code = %q, want %q", code, constants.ErrCodeMissingParam)
	}
}

func TestQueryService_ListPresets_Empty(t *testing.T) {
	mockApp := newMockAppState()
	// Create queries config with no presets
	mockApp.queriesConfig = &queries.QueriesConfig{
		Presets: map[string]queries.Preset{},
	}
	log := logger.NewLogger("debug")
	svc := NewQueryService(mockApp, log)

	presets, err := svc.ListPresets()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(presets) != 0 {
		t.Errorf("presets length = %d, want 0", len(presets))
	}
}

func TestQueryService_ListPresets_WithPresets(t *testing.T) {
	mockApp := newMockAppState()
	// Create queries config with presets
	mockApp.queriesConfig = &queries.QueriesConfig{
		Presets: map[string]queries.Preset{
			"query-one": {
				Description: "First query",
				SQL:         "SELECT * FROM assets",
			},
			"query-two": {
				Description: "Second query",
				SQL:         "SELECT * FROM assets WHERE x = :param",
				Params: []queries.PresetParam{
					{Name: "param", Required: true},
				},
			},
		},
	}
	log := logger.NewLogger("debug")
	svc := NewQueryService(mockApp, log)

	presets, err := svc.ListPresets()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(presets) != 2 {
		t.Errorf("presets length = %d, want 2", len(presets))
	}

	// Verify preset info is correct
	found := make(map[string]bool)
	for _, p := range presets {
		found[p.Name] = true
	}
	if !found["query-one"] {
		t.Error("expected to find query-one")
	}
	if !found["query-two"] {
		t.Error("expected to find query-two")
	}
}
