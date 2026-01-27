package e2e

import (
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func TestTopicNameValidation(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	testCases := []struct {
		name        string
		valid       bool
		errCode     string
		description string
	}{
		{"valid-name", true, "", "lowercase with hyphens"},
		{"valid_name", true, "", "lowercase with underscores"},
		{"valid123", true, "", "lowercase with numbers"},
		{"a", true, "", "single character (min length is 1)"},
		{"ab", true, "", "two characters"},
		{"123topic", true, "", "starts with number"},
		{"UPPERCASE", false, "INVALID_TOPIC_NAME", "uppercase letters"},
		{"Mixed", false, "INVALID_TOPIC_NAME", "mixed case"},
		{"with spaces", false, "INVALID_TOPIC_NAME", "contains spaces"},
		{"special!chars", false, "INVALID_TOPIC_NAME", "special characters"},
		{"has@symbol", false, "INVALID_TOPIC_NAME", "at symbol"},
		{"has.dot", false, "INVALID_TOPIC_NAME", "dot character"},
		{"", false, "INVALID_REQUEST", "empty string"},
		{strings.Repeat("a", 64), true, "", "exactly 64 characters (max length)"},
		{strings.Repeat("a", 65), false, "INVALID_TOPIC_NAME", "65 characters (exceeds max)"},
	}

	createdTopics := make(map[string]bool)

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			resp, err := ts.POST("/api/topics", map[string]string{"name": tc.name})
			if err != nil {
				t.Fatalf("POST topics request failed: %v", err)
			}
			defer resp.Body.Close()

			bodyBytes, _ := io.ReadAll(resp.Body)

			if tc.valid {
				if resp.StatusCode != 200 && resp.StatusCode != 201 {
					t.Errorf("Expected success for valid topic name %q, got status %d: %s",
						tc.name, resp.StatusCode, string(bodyBytes))
				}
				createdTopics[tc.name] = true
			} else {
				if resp.StatusCode != 400 {
					t.Errorf("Expected 400 for invalid topic name %q, got status %d: %s",
						tc.name, resp.StatusCode, string(bodyBytes))
					return
				}

				var errResp ErrorResponse
				if err := json.Unmarshal(bodyBytes, &errResp); err != nil {
					t.Errorf("Failed to decode error response for %q: %v", tc.name, err)
					return
				}

				if errResp.Code != tc.errCode {
					t.Errorf("Expected error code %s for %q, got: %s",
						tc.errCode, tc.name, errResp.Code)
				}
			}
		})
	}
}

func TestQueryParameterValidation(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Create a topic so queries have something to work with
	ts.CreateTopic(t, "param-test")

	// Test 1: Missing required parameter for by-hash (hash is required, no default)
	errResp := ts.ExecuteQueryExpectError(t, "by-hash", []string{"param-test"}, nil, 400)
	if errResp.Code != "MISSING_PARAM" {
		t.Errorf("Expected error code MISSING_PARAM, got: %s", errResp.Code)
	}

	// Test 2: Empty string for required parameter
	errResp = ts.ExecuteQueryExpectError(t, "by-hash", []string{"param-test"}, map[string]interface{}{
		"hash": "",
	}, 400)
	if errResp.Code != "MISSING_PARAM" {
		t.Errorf("Expected error code MISSING_PARAM for empty param, got: %s", errResp.Code)
	}

	// Test 3: Valid parameter should work (recent-imports has defaults so no params needed)
	result := ts.ExecuteQuery(t, "recent-imports", []string{"param-test"}, nil)
	if result.Preset != "recent-imports" {
		t.Errorf("Expected preset name 'recent-imports', got: %s", result.Preset)
	}

	// Test 4: by-hash with valid parameter should work
	result = ts.ExecuteQuery(t, "by-hash", []string{"param-test"}, map[string]interface{}{
		"hash": "abc",
	})
	if result.Preset != "by-hash" {
		t.Errorf("Expected preset name 'by-hash', got: %s", result.Preset)
	}
}

func TestNonExistentQueryPreset(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	ts.CreateTopic(t, "preset-test")

	errResp := ts.ExecuteQueryExpectError(t, "nonexistent-preset", []string{"preset-test"}, nil, 404)
	if errResp.Code != "PRESET_NOT_FOUND" {
		t.Errorf("Expected error code PRESET_NOT_FOUND, got: %s", errResp.Code)
	}
}

func TestQueryOnUnhealthyTopic(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Create a healthy topic for baseline
	ts.CreateTopic(t, "healthy-topic")

	// Try to query a topic that doesn't exist (which is effectively unhealthy)
	errResp := ts.ExecuteQueryExpectError(t, "recent-imports", []string{"nonexistent-topic"}, map[string]interface{}{
		"days": 7,
	}, 400)

	// The error could be TOPIC_UNHEALTHY or another related error
	if errResp.Code != "TOPIC_UNHEALTHY" && errResp.Code != "TOPIC_NOT_FOUND" {
		t.Errorf("Expected error code TOPIC_UNHEALTHY or TOPIC_NOT_FOUND, got: %s", errResp.Code)
	}
}
