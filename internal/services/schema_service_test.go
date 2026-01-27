package services

import (
	"testing"

	"meshbank/internal/constants"
	"meshbank/internal/logger"
	"meshbank/internal/prompts"
)

func TestNewSchemaService(t *testing.T) {
	mockApp := newMockAppState()
	log := logger.NewLogger("debug")

	svc := NewSchemaService(mockApp, log)

	if svc == nil {
		t.Fatal("NewSchemaService returned nil")
	}
	if svc.app != mockApp {
		t.Error("app field not set correctly")
	}
	if svc.logger != log {
		t.Error("logger field not set correctly")
	}
}

func TestSchemaService_GetAPISchema(t *testing.T) {
	mockApp := newMockAppState()
	log := logger.NewLogger("debug")
	svc := NewSchemaService(mockApp, log)

	schema := svc.GetAPISchema()

	if schema == nil {
		t.Fatal("GetAPISchema returned nil")
	}
	if schema.Version == "" {
		t.Error("Version should not be empty")
	}
	if schema.BaseURL == "" {
		t.Error("BaseURL should not be empty")
	}
	if len(schema.Endpoints) == 0 {
		t.Error("Endpoints should not be empty")
	}
}

func TestSchemaService_GetAPISchema_HasRequiredEndpoints(t *testing.T) {
	mockApp := newMockAppState()
	log := logger.NewLogger("debug")
	svc := NewSchemaService(mockApp, log)

	schema := svc.GetAPISchema()

	// Map endpoints by path for easier testing
	endpointsByPath := make(map[string]EndpointSpec)
	for _, ep := range schema.Endpoints {
		endpointsByPath[ep.Path] = ep
	}

	requiredEndpoints := []struct {
		path     string
		method   string
		category string
	}{
		{"/api/config", "GET", "config"},
		{"/api/config", "POST", "config"},
		{"/api/topics", "GET", "topics"},
		{"/api/topics", "POST", "topics"},
		{"/api/assets/:hash/download", "GET", "assets"},
		{"/api/assets/:hash/metadata", "GET", "metadata"},
		{"/api/assets/:hash/metadata", "POST", "metadata"},
		{"/api/queries", "GET", "queries"},
		{"/api/download/bulk", "POST", "download"},
		{"/api/verify", "GET", "system"},
	}

	for _, req := range requiredEndpoints {
		found := false
		for _, ep := range schema.Endpoints {
			if ep.Path == req.path && ep.Method == req.method {
				found = true
				if ep.Category != req.category {
					t.Errorf("Endpoint %s %s category = %q, want %q", req.method, req.path, ep.Category, req.category)
				}
				break
			}
		}
		if !found {
			t.Errorf("Required endpoint %s %s not found", req.method, req.path)
		}
	}
}

func TestSchemaService_GetAPISchema_EndpointSpec(t *testing.T) {
	mockApp := newMockAppState()
	log := logger.NewLogger("debug")
	svc := NewSchemaService(mockApp, log)

	schema := svc.GetAPISchema()

	// Find the POST /api/config endpoint and check its structure
	var configEndpoint *EndpointSpec
	for i := range schema.Endpoints {
		if schema.Endpoints[i].Path == "/api/config" && schema.Endpoints[i].Method == "POST" {
			configEndpoint = &schema.Endpoints[i]
			break
		}
	}

	if configEndpoint == nil {
		t.Fatal("POST /api/config endpoint not found")
	}

	if configEndpoint.Description == "" {
		t.Error("Description should not be empty")
	}
	if configEndpoint.Request == nil {
		t.Error("Request spec should not be nil for POST endpoint")
	}
	if configEndpoint.Request != nil && configEndpoint.Request.ContentType != "application/json" {
		t.Errorf("ContentType = %q, want %q", configEndpoint.Request.ContentType, "application/json")
	}
}

func TestSchemaService_ListPrompts_NotConfigured(t *testing.T) {
	mockApp := newMockAppState()
	mockApp.promptsManager = nil // ensure not configured
	log := logger.NewLogger("debug")
	svc := NewSchemaService(mockApp, log)

	_, err := svc.ListPrompts()
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

func TestSchemaService_GetPrompt_NotConfigured(t *testing.T) {
	mockApp := newMockAppState()
	mockApp.promptsManager = nil // ensure not configured
	log := logger.NewLogger("debug")
	svc := NewSchemaService(mockApp, log)

	_, err := svc.GetPrompt("test-prompt")
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

func TestAPISchema_Struct(t *testing.T) {
	schema := &APISchema{
		Version: "1.0",
		BaseURL: "http://localhost:8080",
		Endpoints: []EndpointSpec{
			{
				Method:      "GET",
				Path:        "/api/test",
				Description: "Test endpoint",
				Category:    "test",
			},
		},
	}

	if schema.Version != "1.0" {
		t.Errorf("Version = %q, want %q", schema.Version, "1.0")
	}
	if schema.BaseURL != "http://localhost:8080" {
		t.Errorf("BaseURL = %q, want %q", schema.BaseURL, "http://localhost:8080")
	}
	if len(schema.Endpoints) != 1 {
		t.Errorf("Endpoints length = %d, want 1", len(schema.Endpoints))
	}
}

func TestEndpointSpec_Struct(t *testing.T) {
	endpoint := EndpointSpec{
		Method:      "POST",
		Path:        "/api/upload",
		Description: "Upload a file",
		Category:    "files",
		Request: &RequestSpec{
			ContentType: "multipart/form-data",
			Body: map[string]interface{}{
				"file": "file (required)",
			},
			Params: []ParamSpec{
				{Name: "topic", Type: "string", Required: true, Description: "Target topic"},
			},
		},
		Response: &ResponseSpec{
			ContentType: "application/json",
			Body: map[string]interface{}{
				"success": "boolean",
				"hash":    "string",
			},
		},
	}

	if endpoint.Method != "POST" {
		t.Errorf("Method = %q, want %q", endpoint.Method, "POST")
	}
	if endpoint.Path != "/api/upload" {
		t.Errorf("Path = %q, want %q", endpoint.Path, "/api/upload")
	}
	if endpoint.Request == nil {
		t.Fatal("Request should not be nil")
	}
	if endpoint.Request.ContentType != "multipart/form-data" {
		t.Errorf("Request.ContentType = %q, want %q", endpoint.Request.ContentType, "multipart/form-data")
	}
	if len(endpoint.Request.Params) != 1 {
		t.Errorf("Request.Params length = %d, want 1", len(endpoint.Request.Params))
	}
	if endpoint.Response == nil {
		t.Fatal("Response should not be nil")
	}
	if endpoint.Response.ContentType != "application/json" {
		t.Errorf("Response.ContentType = %q, want %q", endpoint.Response.ContentType, "application/json")
	}
}

func TestParamSpec_Struct(t *testing.T) {
	param := ParamSpec{
		Name:        "limit",
		Type:        "integer",
		Required:    false,
		Description: "Maximum number of results",
		Default:     "100",
	}

	if param.Name != "limit" {
		t.Errorf("Name = %q, want %q", param.Name, "limit")
	}
	if param.Type != "integer" {
		t.Errorf("Type = %q, want %q", param.Type, "integer")
	}
	if param.Required {
		t.Error("Required should be false")
	}
	if param.Default != "100" {
		t.Errorf("Default = %q, want %q", param.Default, "100")
	}
}

func TestRequestSpec_Struct(t *testing.T) {
	req := &RequestSpec{
		ContentType: "application/json",
		Body: map[string]interface{}{
			"name": "string (required)",
		},
		Params: []ParamSpec{
			{Name: "dry_run", Type: "boolean", Required: false},
		},
	}

	if req.ContentType != "application/json" {
		t.Errorf("ContentType = %q, want %q", req.ContentType, "application/json")
	}
	if req.Body["name"] != "string (required)" {
		t.Errorf("Body[name] = %v, want %q", req.Body["name"], "string (required)")
	}
	if len(req.Params) != 1 {
		t.Errorf("Params length = %d, want 1", len(req.Params))
	}
}

func TestResponseSpec_Struct(t *testing.T) {
	resp := &ResponseSpec{
		ContentType: "application/json",
		Body: map[string]interface{}{
			"success": "boolean",
			"data":    "object",
		},
	}

	if resp.ContentType != "application/json" {
		t.Errorf("ContentType = %q, want %q", resp.ContentType, "application/json")
	}
	if resp.Body["success"] != "boolean" {
		t.Errorf("Body[success] = %v, want %q", resp.Body["success"], "boolean")
	}
}

func TestBuildAPISchema(t *testing.T) {
	schema := buildAPISchema()

	if schema == nil {
		t.Fatal("buildAPISchema returned nil")
	}
	if schema.Version != "1.0" {
		t.Errorf("Version = %q, want %q", schema.Version, "1.0")
	}
	if len(schema.Endpoints) < 10 {
		t.Errorf("Expected at least 10 endpoints, got %d", len(schema.Endpoints))
	}

	// Verify each endpoint has required fields
	for _, ep := range schema.Endpoints {
		if ep.Method == "" {
			t.Errorf("Endpoint %q has empty Method", ep.Path)
		}
		if ep.Path == "" {
			t.Error("Found endpoint with empty Path")
		}
		if ep.Description == "" {
			t.Errorf("Endpoint %s %s has empty Description", ep.Method, ep.Path)
		}
		if ep.Category == "" {
			t.Errorf("Endpoint %s %s has empty Category", ep.Method, ep.Path)
		}
	}
}

// mockPromptsManager for testing
type mockPromptsManager struct {
	prompts map[string]*prompts.RenderedPrompt
}

func newMockPromptsManager() *mockPromptsManager {
	return &mockPromptsManager{
		prompts: make(map[string]*prompts.RenderedPrompt),
	}
}

func (m *mockPromptsManager) ListPromptsFull() []prompts.RenderedPrompt {
	result := make([]prompts.RenderedPrompt, 0, len(m.prompts))
	for _, p := range m.prompts {
		result = append(result, *p)
	}
	return result
}

func (m *mockPromptsManager) GetPrompt(name string) (*prompts.RenderedPrompt, error) {
	if p, ok := m.prompts[name]; ok {
		return p, nil
	}
	return nil, &prompts.PromptNotFoundError{Name: name}
}
