package e2e

import (
	"encoding/json"
	"io"
	"testing"
)

func TestValueTypeDetection(t *testing.T) {
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

	db := ts.GetTopicDB(t, "test-topic")

	testCases := []struct {
		name        string
		value       string
		expectText  string
		expectNum   *float64
		description string
	}{
		{
			name:        "pure integer",
			value:       "123",
			expectText:  "",
			expectNum:   float64Ptr(123),
			description: "Pure integer should be stored as number",
		},
		{
			name:        "pure float",
			value:       "123.45",
			expectText:  "",
			expectNum:   float64Ptr(123.45),
			description: "Pure float should be stored as number",
		},
		{
			name:        "leading zeros",
			value:       "00123",
			expectText:  "00123",
			expectNum:   nil,
			description: "Leading zeros should be text only",
		},
		{
			name:        "trailing decimal zero",
			value:       "1.0",
			expectText:  "1.0",
			expectNum:   nil,
			description: "Trailing decimal zero should be text only",
		},
		{
			name:        "non-numeric",
			value:       "hello",
			expectText:  "hello",
			expectNum:   nil,
			description: "Non-numeric should be text only",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set metadata
			resp, err := ts.POST("/api/assets/"+hash+"/metadata", map[string]interface{}{
				"op":        "set",
				"key":       tc.name,
				"value":     tc.value,
				"processor":         "test",
				"processor_version": "1.0",
			})
			if err != nil {
				t.Fatalf("Set metadata failed: %v", err)
			}
			resp.Body.Close()

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

			// Verify key exists
			value, ok := computed[tc.name]
			if !ok {
				t.Fatalf("Expected %s in metadata_computed, got: %v", tc.name, computed)
			}

			// Check if value is numeric or text
			if tc.expectNum != nil {
				// Should be stored as number
				num, ok := value.(float64)
				if !ok {
					t.Errorf("%s: Expected number, got: %v (type: %T)", tc.description, value, value)
				} else if num != *tc.expectNum {
					t.Errorf("%s: Expected %f, got %f", tc.description, *tc.expectNum, num)
				}
			} else {
				// Should be stored as text
				text, ok := value.(string)
				if !ok {
					t.Errorf("%s: Expected string, got: %v (type: %T)", tc.description, value, value)
				} else if text != tc.expectText {
					t.Errorf("%s: Expected %s, got %s", tc.description, tc.expectText, text)
				}
			}

			// Clean up for next test case
			resp, err = ts.POST("/api/assets/"+hash+"/metadata", map[string]interface{}{
				"op":        "delete",
				"key":       tc.name,
				"processor":         "test",
				"processor_version": "1.0",
			})
			if err != nil {
				t.Fatalf("Delete metadata failed: %v", err)
			}
			resp.Body.Close()
		})
	}
}

func float64Ptr(f float64) *float64 {
	return &f
}
