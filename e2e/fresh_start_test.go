package e2e

import (
	"testing"
)

func TestFreshStart(t *testing.T) {
	ts := StartTestServer(t)

	// 1. GET /api/config - verify configured: false
	var configResp map[string]interface{}
	err := ts.GetJSON("/api/config", &configResp)
	if err != nil {
		t.Fatalf("GET /api/config failed: %v", err)
	}

	if configured, ok := configResp["configured"].(bool); !ok || configured {
		t.Errorf("Expected configured: false, got: %v", configResp["configured"])
	}

	// 2. POST /api/config with working_directory
	ts.ConfigureWorkDir(t)

	// 3. Verify config persisted
	err = ts.GetJSON("/api/config", &configResp)
	if err != nil {
		t.Fatalf("GET /api/config after config failed: %v", err)
	}

	if configured, ok := configResp["configured"].(bool); !ok || !configured {
		t.Errorf("Expected configured: true after POST, got: %v", configResp["configured"])
	}

	if workDir, ok := configResp["working_directory"].(string); !ok || workDir != ts.WorkDir {
		t.Errorf("Expected working_directory: %s, got: %v", ts.WorkDir, configResp["working_directory"])
	}

	// 4. GET /api/topics - should return empty list
	var topicsResp map[string]interface{}
	err = ts.GetJSON("/api/topics", &topicsResp)
	if err != nil {
		t.Fatalf("GET /api/topics failed: %v", err)
	}

	topics, ok := topicsResp["topics"].([]interface{})
	if !ok {
		t.Fatalf("Expected topics array, got: %T", topicsResp["topics"])
	}

	if len(topics) != 0 {
		t.Errorf("Expected empty topics list, got %d topics", len(topics))
	}
}
