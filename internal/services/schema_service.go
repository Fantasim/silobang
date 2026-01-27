package services

import (
	"fmt"

	"meshbank/internal/constants"
	"meshbank/internal/logger"
	"meshbank/internal/prompts"
)

// SchemaService handles API schema and prompts operations.
type SchemaService struct {
	app    AppState
	logger *logger.Logger
}

// NewSchemaService creates a new schema service instance.
func NewSchemaService(app AppState, log *logger.Logger) *SchemaService {
	return &SchemaService{
		app:    app,
		logger: log,
	}
}

// APISchema represents the complete API schema for machine consumption.
type APISchema struct {
	Version   string         `json:"version"`
	BaseURL   string         `json:"base_url"`
	Endpoints []EndpointSpec `json:"endpoints"`
}

// EndpointSpec describes a single API endpoint.
type EndpointSpec struct {
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	Description string            `json:"description"`
	Category    string            `json:"category"`
	Request     *RequestSpec      `json:"request,omitempty"`
	Response    *ResponseSpec     `json:"response,omitempty"`
}

// RequestSpec describes the expected request format.
type RequestSpec struct {
	ContentType string                 `json:"content_type,omitempty"`
	Body        map[string]interface{} `json:"body,omitempty"`
	Params      []ParamSpec            `json:"params,omitempty"`
}

// ResponseSpec describes the response format.
type ResponseSpec struct {
	ContentType string                 `json:"content_type"`
	Body        map[string]interface{} `json:"body,omitempty"`
}

// ParamSpec describes a request parameter.
type ParamSpec struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
	Default     string `json:"default,omitempty"`
}

// GetAPISchema returns the complete API schema.
func (s *SchemaService) GetAPISchema() *APISchema {
	return buildAPISchema()
}

// ListPrompts returns all prompts with full templates.
func (s *SchemaService) ListPrompts() ([]prompts.RenderedPrompt, error) {
	pm := s.app.GetPromptsManager()
	if pm == nil {
		return nil, NewServiceError(constants.ErrCodeNotConfigured, "prompts not available - working directory not configured")
	}
	return pm.ListPromptsFull(), nil
}

// GetPrompt returns a specific prompt by name.
func (s *SchemaService) GetPrompt(name string) (*prompts.RenderedPrompt, error) {
	pm := s.app.GetPromptsManager()
	if pm == nil {
		return nil, NewServiceError(constants.ErrCodeNotConfigured, "prompts not available - working directory not configured")
	}

	prompt, err := pm.GetPrompt(name)
	if err != nil {
		if _, ok := err.(*prompts.PromptNotFoundError); ok {
			return nil, NewServiceError(constants.ErrCodePromptNotFound, "prompt not found: "+name)
		}
		return nil, WrapInternalError(err)
	}

	return prompt, nil
}

// buildAPISchema constructs the complete API schema.
func buildAPISchema() *APISchema {
	return &APISchema{
		Version: "1.0",
		BaseURL: fmt.Sprintf("http://localhost:%d", constants.DefaultPort),
		Endpoints: []EndpointSpec{
			// Config
			{
				Method:      "GET",
				Path:        "/api/config",
				Description: "Get current configuration status",
				Category:    "config",
				Response: &ResponseSpec{
					ContentType: "application/json",
					Body: map[string]interface{}{
						"configured":        "boolean",
						"working_directory": "string",
						"port":              "number",
						"max_dat_size":      "number",
					},
				},
			},
			{
				Method:      "POST",
				Path:        "/api/config",
				Description: "Set working directory",
				Category:    "config",
				Request: &RequestSpec{
					ContentType: "application/json",
					Body: map[string]interface{}{
						"working_directory": "string (required)",
					},
				},
			},

			// Topics
			{
				Method:      "GET",
				Path:        "/api/topics",
				Description: "List all topics with stats and service info",
				Category:    "topics",
			},
			{
				Method:      "POST",
				Path:        "/api/topics",
				Description: "Create a new topic",
				Category:    "topics",
				Request: &RequestSpec{
					ContentType: "application/json",
					Body: map[string]interface{}{
						"name": "string (required, lowercase alphanumeric with - and _)",
					},
				},
			},
			{
				Method:      "POST",
				Path:        "/api/topics/:name/assets",
				Description: "Upload an asset to a topic",
				Category:    "topics",
				Request: &RequestSpec{
					ContentType: "multipart/form-data",
					Body: map[string]interface{}{
						"file":      "file (required)",
						"parent_id": "string (optional, 64-char hash)",
					},
				},
				Response: &ResponseSpec{
					ContentType: "application/json",
					Body: map[string]interface{}{
						"success":        "boolean",
						"hash":           "string (64-char BLAKE3 hash)",
						"skipped":        "boolean (true if duplicate)",
						"existing_topic": "string (if skipped)",
					},
				},
			},

			// Assets
			{
				Method:      "GET",
				Path:        "/api/assets/:hash/download",
				Description: "Download an asset by hash",
				Category:    "assets",
			},
			{
				Method:      "GET",
				Path:        "/api/assets/:hash/metadata",
				Description: "Get asset info and computed metadata",
				Category:    "metadata",
				Response: &ResponseSpec{
					ContentType: "application/json",
					Body: map[string]interface{}{
						"asset": map[string]interface{}{
							"origin_name": "string",
							"extension":   "string",
							"created_at":  "number (unix timestamp)",
							"parent_id":   "string|null",
						},
						"computed_metadata": "object (key-value pairs)",
					},
				},
			},
			{
				Method:      "POST",
				Path:        "/api/assets/:hash/metadata",
				Description: "Set or delete metadata for an asset",
				Category:    "metadata",
				Request: &RequestSpec{
					ContentType: "application/json",
					Body: map[string]interface{}{
						"op":                "string (required: 'set' or 'delete')",
						"key":               "string (required)",
						"value":             "any (required for 'set')",
						"processor":         "string (optional, default: 'api')",
						"processor_version": "string (optional, default: '1.0')",
					},
				},
			},

			// Batch Metadata
			{
				Method:      "POST",
				Path:        "/api/metadata/batch",
				Description: "Set or delete metadata on multiple assets atomically",
				Category:    "metadata",
				Request: &RequestSpec{
					ContentType: "application/json",
					Body: map[string]interface{}{
						"operations": []map[string]interface{}{
							{
								"hash":  "string (64-char hash)",
								"op":    "string ('set' or 'delete')",
								"key":   "string",
								"value": "any (for 'set')",
							},
						},
						"processor":         "string (optional)",
						"processor_version": "string (optional)",
					},
				},
				Response: &ResponseSpec{
					ContentType: "application/json",
					Body: map[string]interface{}{
						"success":   "boolean",
						"total":     "number",
						"succeeded": "number",
						"failed":    "number",
						"results": []map[string]interface{}{
							{
								"hash":    "string",
								"success": "boolean",
								"error":   "string (if failed)",
								"log_id":  "number (if succeeded)",
							},
						},
					},
				},
			},
			{
				Method:      "POST",
				Path:        "/api/metadata/apply",
				Description: "Apply metadata to assets matching a query",
				Category:    "metadata",
				Request: &RequestSpec{
					ContentType: "application/json",
					Body: map[string]interface{}{
						"query_preset":      "string (required, e.g. 'recent-imports')",
						"query_params":      "object (optional, preset parameters)",
						"topics":            "array of strings (optional, defaults to all)",
						"op":                "string (required: 'set' or 'delete')",
						"key":               "string (required)",
						"value":             "any (required for 'set')",
						"processor":         "string (optional)",
						"processor_version": "string (optional)",
					},
				},
			},

			// Queries
			{
				Method:      "GET",
				Path:        "/api/queries",
				Description: "List available query presets",
				Category:    "queries",
			},
			{
				Method:      "POST",
				Path:        "/api/query/:preset",
				Description: "Execute a query preset",
				Category:    "queries",
				Request: &RequestSpec{
					ContentType: "application/json",
					Body: map[string]interface{}{
						"topics": "array of strings (optional)",
						"params": "object (preset-specific parameters)",
					},
				},
				Response: &ResponseSpec{
					ContentType: "application/json",
					Body: map[string]interface{}{
						"preset":    "string",
						"row_count": "number",
						"columns":   "array of strings",
						"rows":      "array of arrays",
					},
				},
			},

			// Bulk Download
			{
				Method:      "POST",
				Path:        "/api/download/bulk",
				Description: "Download multiple assets as ZIP",
				Category:    "download",
			},
			{
				Method:      "POST",
				Path:        "/api/download/bulk/start",
				Description: "Start SSE bulk download session",
				Category:    "download",
			},
			{
				Method:      "GET",
				Path:        "/api/download/bulk/:sessionID",
				Description: "Fetch completed bulk download ZIP",
				Category:    "download",
			},

			// Verification
			{
				Method:      "GET",
				Path:        "/api/verify",
				Description: "Verify topic integrity (SSE stream)",
				Category:    "system",
			},
		},
	}
}
