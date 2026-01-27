package e2e

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestMetadataOperations(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "test-topic")

	// Upload asset
	resp, err := ts.UploadFile("test-topic", "test.bin", SmallFile, "")
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}
	defer resp.Body.Close()

	var uploadResp map[string]interface{}
	bodyBytes, _ := io.ReadAll(resp.Body)
	json.Unmarshal(bodyBytes, &uploadResp)

	hash := uploadResp["hash"].(string)

	// Set metadata: polycount
	resp, err = ts.POST("/api/assets/"+hash+"/metadata", map[string]interface{}{
		"op":        "set",
		"key":       "polycount",
		"value":     "1000",
		"processor":         "test",
		"processor_version": "1.0",
	})
	if err != nil {
		t.Fatalf("Set metadata failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("Set metadata failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Query metadata_log
	db := ts.GetTopicDB(t, "test-topic")
	var valueText string
	var valueNum *float64
	err = db.QueryRow(`
		SELECT value_text, value_num
		FROM metadata_log
		WHERE asset_id = ? AND key = 'polycount'
		ORDER BY id DESC LIMIT 1
	`, hash).Scan(&valueText, &valueNum)
	if err != nil {
		t.Fatalf("Failed to query metadata_log: %v", err)
	}

	if valueText != "1000" {
		t.Errorf("Expected value_text: 1000, got: %s", valueText)
	}
	if valueNum == nil || *valueNum != 1000 {
		t.Errorf("Expected value_num: 1000, got: %v", valueNum)
	}

	// Query metadata_computed
	var computedJSON string
	err = db.QueryRow(`
		SELECT metadata_json
		FROM metadata_computed
		WHERE asset_id = ?
	`, hash).Scan(&computedJSON)
	if err != nil {
		t.Fatalf("Failed to query metadata_computed: %v", err)
	}

	var computed map[string]interface{}
	json.Unmarshal([]byte(computedJSON), &computed)

	polycount, ok := computed["polycount"].(float64)
	if !ok {
		t.Fatalf("Expected polycount as number in metadata_computed, got: %v (type: %T)", computed["polycount"], computed["polycount"])
	}

	if polycount != 1000 {
		t.Errorf("Expected polycount: 1000, got: %v", polycount)
	}

	// Set another metadata key
	resp, err = ts.POST("/api/assets/"+hash+"/metadata", map[string]interface{}{
		"op":        "set",
		"key":       "has_skeleton",
		"value":     "true",
		"processor":         "test",
		"processor_version": "1.0",
	})
	if err != nil {
		t.Fatalf("Set has_skeleton failed: %v", err)
	}
	defer resp.Body.Close()

	// Query computed state again
	err = db.QueryRow(`
		SELECT metadata_json
		FROM metadata_computed
		WHERE asset_id = ?
	`, hash).Scan(&computedJSON)
	if err != nil {
		t.Fatalf("Failed to query metadata_computed after second set: %v", err)
	}

	json.Unmarshal([]byte(computedJSON), &computed)

	if _, ok := computed["polycount"]; !ok {
		t.Errorf("Expected polycount to still exist in computed state")
	}
	if _, ok := computed["has_skeleton"]; !ok {
		t.Errorf("Expected has_skeleton in computed state")
	}

	// Check computed state before delete
	err = db.QueryRow("SELECT metadata_json FROM metadata_computed WHERE asset_id = ?", hash).Scan(&computedJSON)
	if err == nil {
		json.Unmarshal([]byte(computedJSON), &computed)
		t.Logf("Before delete, computed state: %v", computed)
	}

	// Delete polycount
	resp, err = ts.POST("/api/assets/"+hash+"/metadata", map[string]interface{}{
		"op":        "delete",
		"key":       "polycount",
		"processor":         "test",
		"processor_version": "1.0",
	})
	if err != nil {
		t.Fatalf("Delete metadata failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ = io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Fatalf("Delete metadata failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var deleteResp map[string]interface{}
	json.Unmarshal(bodyBytes, &deleteResp)

	// Verify the API response contains the updated computed metadata
	computedFromAPI, ok := deleteResp["computed_metadata"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected computed_metadata in delete response, got: %v", deleteResp)
	}

	if _, ok := computedFromAPI["polycount"]; ok {
		t.Errorf("Expected polycount to be deleted from computed metadata, got: %v", computedFromAPI)
	}
	if _, ok := computedFromAPI["has_skeleton"]; !ok {
		t.Errorf("Expected has_skeleton to remain in computed metadata, got: %v", computedFromAPI)
	}
}

func TestGetMetadata(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "test-topic")

	// Upload asset with a specific filename
	resp, err := ts.UploadFile("test-topic", "mymodel.glb", SmallFile, "")
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}
	defer resp.Body.Close()

	var uploadResp map[string]interface{}
	bodyBytes, _ := io.ReadAll(resp.Body)
	json.Unmarshal(bodyBytes, &uploadResp)

	hash := uploadResp["hash"].(string)

	// Test GET metadata before any metadata is set
	resp, err = ts.GET("/api/assets/" + hash + "/metadata")
	if err != nil {
		t.Fatalf("GET metadata failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET metadata failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var metaResp map[string]interface{}
	bodyBytes, _ = io.ReadAll(resp.Body)
	json.Unmarshal(bodyBytes, &metaResp)

	// Verify asset info
	asset, ok := metaResp["asset"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected asset object in response, got: %v", metaResp)
	}

	if asset["origin_name"] != "mymodel" {
		t.Errorf("Expected origin_name: mymodel, got: %v", asset["origin_name"])
	}
	if asset["extension"] != "glb" {
		t.Errorf("Expected extension: glb, got: %v", asset["extension"])
	}
	if asset["created_at"] == nil {
		t.Errorf("Expected created_at to be set")
	}
	if asset["parent_id"] != nil {
		t.Errorf("Expected parent_id to be nil for root asset, got: %v", asset["parent_id"])
	}

	// Verify computed_metadata is empty initially
	computed := metaResp["computed_metadata"]
	if computed != nil {
		// Check if it's an empty map
		if compMap, ok := computed.(map[string]interface{}); ok && len(compMap) > 0 {
			t.Errorf("Expected empty computed_metadata initially, got: %v", computed)
		}
	}

	// Set some metadata
	resp, err = ts.POST("/api/assets/"+hash+"/metadata", map[string]interface{}{
		"op":                "set",
		"key":               "polycount",
		"value":             1500,
		"processor":         "test-processor",
		"processor_version": "2.0",
	})
	if err != nil {
		t.Fatalf("Set metadata failed: %v", err)
	}
	resp.Body.Close()

	resp, err = ts.POST("/api/assets/"+hash+"/metadata", map[string]interface{}{
		"op":                "set",
		"key":               "has_textures",
		"value":             true,
		"processor":         "test-processor",
		"processor_version": "2.0",
	})
	if err != nil {
		t.Fatalf("Set metadata failed: %v", err)
	}
	resp.Body.Close()

	// GET metadata again and verify computed_metadata contains the values
	resp, err = ts.GET("/api/assets/" + hash + "/metadata")
	if err != nil {
		t.Fatalf("GET metadata failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET metadata failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	bodyBytes, _ = io.ReadAll(resp.Body)
	json.Unmarshal(bodyBytes, &metaResp)

	computed = metaResp["computed_metadata"]
	compMap, ok := computed.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected computed_metadata as map, got: %T", computed)
	}

	polycount, ok := compMap["polycount"].(float64)
	if !ok || polycount != 1500 {
		t.Errorf("Expected polycount: 1500, got: %v", compMap["polycount"])
	}

	// Booleans are stored as strings in metadata
	hasTextures, ok := compMap["has_textures"].(string)
	if !ok || hasTextures != "true" {
		t.Errorf("Expected has_textures: 'true', got: %v (type: %T)", compMap["has_textures"], compMap["has_textures"])
	}
}

func TestGetMetadataNotFound(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "test-topic")

	// Try to GET metadata for a non-existent hash
	fakeHash := "0000000000000000000000000000000000000000000000000000000000000000"
	resp, err := ts.GET("/api/assets/" + fakeHash + "/metadata")
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404 for non-existent hash, got: %d", resp.StatusCode)
	}
}

func TestGetMetadataWithParent(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "test-topic")

	// Upload parent asset
	resp, err := ts.UploadFile("test-topic", "parent.bin", SmallFile, "")
	if err != nil {
		t.Fatalf("Upload parent failed: %v", err)
	}
	defer resp.Body.Close()

	var parentResp map[string]interface{}
	bodyBytes, _ := io.ReadAll(resp.Body)
	json.Unmarshal(bodyBytes, &parentResp)

	parentHash := parentResp["hash"].(string)

	// Upload child asset with parent_id
	resp, err = ts.UploadFile("test-topic", "child.bin", LargeFile, parentHash)
	if err != nil {
		t.Fatalf("Upload child failed: %v", err)
	}
	defer resp.Body.Close()

	var childResp map[string]interface{}
	bodyBytes, _ = io.ReadAll(resp.Body)
	json.Unmarshal(bodyBytes, &childResp)

	childHash := childResp["hash"].(string)

	// GET metadata for child and verify parent_id
	resp, err = ts.GET("/api/assets/" + childHash + "/metadata")
	if err != nil {
		t.Fatalf("GET metadata failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET metadata failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var metaResp map[string]interface{}
	bodyBytes, _ = io.ReadAll(resp.Body)
	json.Unmarshal(bodyBytes, &metaResp)

	asset, ok := metaResp["asset"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected asset object in response, got: %v", metaResp)
	}

	if asset["parent_id"] != parentHash {
		t.Errorf("Expected parent_id: %s, got: %v", parentHash, asset["parent_id"])
	}
}
