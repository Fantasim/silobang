package e2e

import (
	"encoding/json"
	"testing"
)

// TestAPISchemaEndpoint tests GET /api/schema returns valid schema
func TestAPISchemaEndpoint(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	resp, err := ts.GET("/api/schema")
	if err != nil {
		t.Fatalf("schema request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var schema map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&schema); err != nil {
		t.Fatalf("failed to parse schema: %v", err)
	}

	// Verify required fields
	if schema["version"] == nil {
		t.Error("schema missing version field")
	}
	if schema["base_url"] == nil {
		t.Error("schema missing base_url field")
	}
	if schema["endpoints"] == nil {
		t.Error("schema missing endpoints field")
	}

	// Verify endpoints is an array
	endpoints, ok := schema["endpoints"].([]interface{})
	if !ok {
		t.Fatalf("endpoints is not an array")
	}

	// Should have multiple endpoints
	if len(endpoints) < 5 {
		t.Errorf("expected at least 5 endpoints, got %d", len(endpoints))
	}

	// Verify each endpoint has required fields
	for i, ep := range endpoints {
		endpoint, ok := ep.(map[string]interface{})
		if !ok {
			t.Errorf("endpoint %d is not an object", i)
			continue
		}

		if endpoint["method"] == nil {
			t.Errorf("endpoint %d missing method", i)
		}
		if endpoint["path"] == nil {
			t.Errorf("endpoint %d missing path", i)
		}
		if endpoint["description"] == nil {
			t.Errorf("endpoint %d missing description", i)
		}
		if endpoint["category"] == nil {
			t.Errorf("endpoint %d missing category", i)
		}
	}
}

// TestPromptsListEndpoint tests GET /api/prompts returns list of templates
func TestPromptsListEndpoint(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	resp, err := ts.GET("/api/prompts")
	if err != nil {
		t.Fatalf("prompts request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to parse prompts response: %v", err)
	}

	prompts, ok := result["prompts"].([]interface{})
	if !ok {
		t.Fatalf("prompts is not an array")
	}

	// Should have at least one prompt
	if len(prompts) < 1 {
		t.Errorf("expected at least 1 prompt, got %d", len(prompts))
	}

	// Verify each prompt has required fields
	for i, p := range prompts {
		prompt, ok := p.(map[string]interface{})
		if !ok {
			t.Errorf("prompt %d is not an object", i)
			continue
		}

		if prompt["name"] == nil {
			t.Errorf("prompt %d missing name", i)
		}
		if prompt["description"] == nil {
			t.Errorf("prompt %d missing description", i)
		}
		if prompt["category"] == nil {
			t.Errorf("prompt %d missing category", i)
		}
	}
}

// TestPromptTemplateEndpoint tests GET /api/prompts/:name returns specific template
func TestPromptTemplateEndpoint(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Get batch-metadata template
	resp, err := ts.GET("/api/prompts/batch-metadata")
	if err != nil {
		t.Fatalf("prompt template request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var template map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&template); err != nil {
		t.Fatalf("failed to parse template: %v", err)
	}

	// Verify required fields
	if template["name"] != "batch-metadata" {
		t.Errorf("expected name='batch-metadata', got %v", template["name"])
	}
	if template["description"] == nil {
		t.Error("template missing description")
	}
	if template["category"] == nil {
		t.Error("template missing category")
	}
	if template["template"] == nil {
		t.Error("template missing template content")
	}

	// Template content should be rendered
	content, ok := template["template"].(string)
	if !ok {
		t.Fatal("template content is not a string")
	}

	if len(content) < 100 {
		t.Error("template content seems too short")
	}

	// Template variables should be substituted - {{base_url}} should be replaced with actual URL
	if containsAny(content, "{{base_url}}") {
		t.Error("template should have {{base_url}} replaced with actual URL")
	}

	// Should contain the rendered base URL (http://localhost:PORT)
	if !containsAny(content, "http://localhost:") {
		t.Error("template should contain rendered base URL")
	}

	// Should contain instruction to discover queries via API
	if !containsAny(content, "/api/queries") {
		t.Error("template should reference /api/queries for query discovery")
	}
}

// TestPromptTemplateNotFound tests GET /api/prompts/:name with invalid name
func TestPromptTemplateNotFound(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	resp, err := ts.GET("/api/prompts/nonexistent-template")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 404 {
		t.Errorf("expected status 404, got %d", resp.StatusCode)
	}
}

// TestPromptsDefaultsExist verifies all 5 default prompts are created
func TestPromptsDefaultsExist(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	expectedPrompts := []string{
		"batch-metadata",
		"query-and-apply",
		"upload-files",
		"lineage-relationships",
		"conditional-metadata",
	}

	resp, err := ts.GET("/api/prompts")
	if err != nil {
		t.Fatalf("prompts request failed: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	prompts, ok := result["prompts"].([]interface{})
	if !ok {
		t.Fatalf("prompts is not an array")
	}

	// Build map of prompt names
	promptNames := make(map[string]bool)
	for _, p := range prompts {
		prompt, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		if name, ok := prompt["name"].(string); ok {
			promptNames[name] = true
		}
	}

	// Verify all expected prompts exist
	for _, expected := range expectedPrompts {
		if !promptNames[expected] {
			t.Errorf("expected prompt %q not found", expected)
		}
	}

	// Should have exactly 5 default prompts
	if len(prompts) != 5 {
		t.Errorf("expected 5 prompts, got %d", len(prompts))
	}
}

// TestPromptsBaseURLRendered verifies {{base_url}} is replaced in all prompts
func TestPromptsBaseURLRendered(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	resp, err := ts.GET("/api/prompts")
	if err != nil {
		t.Fatalf("prompts request failed: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	prompts, ok := result["prompts"].([]interface{})
	if !ok {
		t.Fatalf("prompts is not an array")
	}

	for _, p := range prompts {
		prompt, ok := p.(map[string]interface{})
		if !ok {
			continue
		}

		name := prompt["name"].(string)
		template, ok := prompt["template"].(string)
		if !ok {
			t.Errorf("prompt %s has no template", name)
			continue
		}

		// Should not contain raw {{base_url}} placeholder
		if containsAny(template, "{{base_url}}") {
			t.Errorf("prompt %s still contains {{base_url}} placeholder", name)
		}

		// Should contain rendered URL
		if !containsAny(template, "http://localhost:") {
			t.Errorf("prompt %s missing rendered base URL", name)
		}
	}
}

// containsAny checks if s contains any of the substrings
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(sub) > 0 && len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
