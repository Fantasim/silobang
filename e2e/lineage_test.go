package e2e

import (
	"encoding/json"
	"io"
	"testing"
)

func TestLineageTracking(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)
	ts.CreateTopic(t, "test-topic")

	// Upload fileA (no parent)
	fileA := GenerateTestFile(1024)
	resp, err := ts.UploadFile("test-topic", "fileA.bin", fileA, "")
	if err != nil {
		t.Fatalf("Upload A failed: %v", err)
	}
	defer resp.Body.Close()

	var uploadA map[string]interface{}
	bodyBytes, _ := io.ReadAll(resp.Body)
	json.Unmarshal(bodyBytes, &uploadA)
	hashA := uploadA["hash"].(string)

	// Upload fileB with parent_id=hashA
	fileB := GenerateTestFile(2048)
	resp, err = ts.UploadFile("test-topic", "fileB.bin", fileB, hashA)
	if err != nil {
		t.Fatalf("Upload B failed: %v", err)
	}
	defer resp.Body.Close()

	var uploadB map[string]interface{}
	bodyBytes, _ = io.ReadAll(resp.Body)
	json.Unmarshal(bodyBytes, &uploadB)
	hashB := uploadB["hash"].(string)

	// Upload fileC with parent_id=hashB
	fileC := GenerateTestFile(3072)
	resp, err = ts.UploadFile("test-topic", "fileC.bin", fileC, hashB)
	if err != nil {
		t.Fatalf("Upload C failed: %v", err)
	}
	defer resp.Body.Close()

	var uploadC map[string]interface{}
	bodyBytes, _ = io.ReadAll(resp.Body)
	json.Unmarshal(bodyBytes, &uploadC)
	hashC := uploadC["hash"].(string)

	// Query assets table to verify parent_id columns
	db := ts.GetTopicDB(t, "test-topic")

	var parentA *string
	err = db.QueryRow("SELECT parent_id FROM assets WHERE asset_id = ?", hashA).Scan(&parentA)
	if err != nil {
		t.Fatalf("Failed to query parent_id for A: %v", err)
	}
	if parentA != nil {
		t.Errorf("Expected parent_id NULL for A, got: %v", *parentA)
	}

	var parentB string
	err = db.QueryRow("SELECT parent_id FROM assets WHERE asset_id = ?", hashB).Scan(&parentB)
	if err != nil {
		t.Fatalf("Failed to query parent_id for B: %v", err)
	}
	if parentB != hashA {
		t.Errorf("Expected parent_id %s for B, got: %s", hashA, parentB)
	}

	var parentC string
	err = db.QueryRow("SELECT parent_id FROM assets WHERE asset_id = ?", hashC).Scan(&parentC)
	if err != nil {
		t.Fatalf("Failed to query parent_id for C: %v", err)
	}
	if parentC != hashB {
		t.Errorf("Expected parent_id %s for C, got: %s", hashB, parentC)
	}

	// Query lineage for hashC (should return A, B, C chain)
	resp, err = ts.POST("/api/query/lineage", map[string]interface{}{
		"params": map[string]interface{}{
			"hash": hashC,
		},
	})
	if err != nil {
		t.Fatalf("Lineage query failed: %v", err)
	}
	defer resp.Body.Close()

	var lineageResp map[string]interface{}
	bodyBytes, _ = io.ReadAll(resp.Body)
	json.Unmarshal(bodyBytes, &lineageResp)

	rows, ok := lineageResp["rows"].([]interface{})
	if !ok {
		t.Fatalf("Expected rows array, got: %v", lineageResp)
	}

	// Should return full ancestry chain (C, B, A in reverse order or A, B, C)
	if len(rows) != 3 {
		t.Errorf("Expected 3 results in lineage, got %d", len(rows))
	}

	// Verify all three hashes present
	columns := lineageResp["columns"].([]interface{})
	hashIdx := -1
	for i, col := range columns {
		if col.(string) == "asset_id" {
			hashIdx = i
			break
		}
	}
	if hashIdx == -1 {
		t.Fatal("asset_id column not found")
	}

	foundHashes := make(map[string]bool)
	for _, row := range rows {
		rowArray := row.([]interface{})
		foundHashes[rowArray[hashIdx].(string)] = true
	}

	if !foundHashes[hashA] || !foundHashes[hashB] || !foundHashes[hashC] {
		t.Errorf("Lineage query missing hashes. Found: %v", foundHashes)
	}

	// Query derived assets for hashA (should return B and C)
	resp, err = ts.POST("/api/query/derived", map[string]interface{}{
		"params": map[string]interface{}{
			"hash": hashA,
		},
	})
	if err != nil {
		t.Fatalf("Derived query failed: %v", err)
	}
	defer resp.Body.Close()

	var derivedResp map[string]interface{}
	bodyBytes, _ = io.ReadAll(resp.Body)
	json.Unmarshal(bodyBytes, &derivedResp)

	rows, ok = derivedResp["rows"].([]interface{})
	if !ok {
		t.Fatalf("Expected rows array, got: %v", derivedResp)
	}

	// Should return descendants (B and C)
	if len(rows) != 2 {
		t.Errorf("Expected 2 results in derived, got %d", len(rows))
	}

	columns = derivedResp["columns"].([]interface{})
	hashIdx = -1
	for i, col := range columns {
		if col.(string) == "asset_id" {
			hashIdx = i
			break
		}
	}
	if hashIdx == -1 {
		t.Fatal("asset_id column not found in derived query")
	}

	foundDerived := make(map[string]bool)
	for _, row := range rows {
		rowArray := row.([]interface{})
		foundDerived[rowArray[hashIdx].(string)] = true
	}

	if !foundDerived[hashB] || !foundDerived[hashC] {
		t.Errorf("Derived query missing hashes. Found: %v", foundDerived)
	}
}
