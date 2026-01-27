package e2e

import (
	"fmt"
	"net/http"
	"testing"

	"silobang/internal/constants"
)

// =============================================================================
// Quota Enforcement
// =============================================================================

// TestUploadQuota_DailyCount verifies daily upload count limit enforcement
func TestUploadQuota_DailyCount(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	constraints := `{"daily_count_limit":3}`
	user := ts.CreateTestUserWithGrants(t, "quotacount", "secure-password-12345", []map[string]interface{}{
		{"action": constants.AuthActionUpload, "constraints_json": constraints},
		{"action": constants.AuthActionManageTopics},
	})

	ts.CreateTopic(t, "quota-count-topic")

	oldKey := ts.APIKey
	ts.APIKey = user.APIKey
	defer func() { ts.APIKey = oldKey }()

	// First 3 uploads should succeed
	for i := 0; i < 3; i++ {
		content := GenerateTestFile(64)
		resp, err := ts.UploadFile("quota-count-topic", fmt.Sprintf("file%d.bin", i), content, "")
		if err != nil {
			t.Fatalf("upload %d request failed: %v", i, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			t.Fatalf("upload %d expected success, got %d", i, resp.StatusCode)
		}
	}

	// 4th upload should be quota exceeded (429)
	resp, err := ts.UploadFile("quota-count-topic", "file3.bin", GenerateTestFile(64), "")
	if err != nil {
		t.Fatalf("quota-exceeded upload request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected 429 after quota exceeded, got %d", resp.StatusCode)
	}
}

// TestUploadQuota_DailyVolume verifies daily upload volume limit enforcement
func TestUploadQuota_DailyVolume(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	constraints := `{"daily_volume_bytes":500}`
	user := ts.CreateTestUserWithGrants(t, "quotavol", "secure-password-12345", []map[string]interface{}{
		{"action": constants.AuthActionUpload, "constraints_json": constraints},
		{"action": constants.AuthActionManageTopics},
	})

	ts.CreateTopic(t, "quota-vol-topic")

	oldKey := ts.APIKey
	ts.APIKey = user.APIKey
	defer func() { ts.APIKey = oldKey }()

	// Upload 300 bytes — should succeed
	resp, err := ts.UploadFile("quota-vol-topic", "file1.bin", make([]byte, 300), "")
	if err != nil {
		t.Fatalf("first upload request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Fatalf("first upload expected success, got %d", resp.StatusCode)
	}

	// Upload another 300 bytes — should exceed 500-byte daily volume limit
	resp, err = ts.UploadFile("quota-vol-topic", "file2.bin", make([]byte, 300), "")
	if err != nil {
		t.Fatalf("second upload request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected 429 after volume quota exceeded, got %d", resp.StatusCode)
	}
}

// TestDownloadQuota_DailyCount verifies daily download count limit
func TestDownloadQuota_DailyCount(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Create topic and upload a file as admin
	ts.CreateTopic(t, "dl-quota-topic")
	uploadResp := ts.UploadFileExpectSuccess(t, "dl-quota-topic", "test.bin", GenerateTestFile(128), "")
	hash := uploadResp.Hash

	constraints := `{"daily_count_limit":2}`
	user := ts.CreateTestUserWithGrants(t, "dlquota", "secure-password-12345", []map[string]interface{}{
		{"action": constants.AuthActionDownload, "constraints_json": constraints},
	})

	oldKey := ts.APIKey
	ts.APIKey = user.APIKey
	defer func() { ts.APIKey = oldKey }()

	// First 2 downloads should succeed
	for i := 0; i < 2; i++ {
		resp, err := ts.GET("/api/assets/" + hash + "/download")
		if err != nil {
			t.Fatalf("download %d failed: %v", i, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("download %d expected 200, got %d", i, resp.StatusCode)
		}
	}

	// 3rd download should be quota exceeded
	resp, err := ts.GET("/api/assets/" + hash + "/download")
	if err != nil {
		t.Fatalf("quota download request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected 429 after download quota exceeded, got %d", resp.StatusCode)
	}
}

// TestQuotaVisibility verifies user can see their own quota
func TestQuotaVisibility(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	constraints := `{"daily_count_limit":10}`
	user := ts.CreateTestUserWithGrants(t, "quotavis", "secure-password-12345", []map[string]interface{}{
		{"action": constants.AuthActionUpload, "constraints_json": constraints},
		{"action": constants.AuthActionManageTopics},
	})

	ts.CreateTopic(t, "quota-vis-topic")

	oldKey := ts.APIKey
	ts.APIKey = user.APIKey
	defer func() { ts.APIKey = oldKey }()

	// Upload a file to generate quota usage
	resp, err := ts.UploadFile("quota-vis-topic", "file.bin", GenerateTestFile(64), "")
	if err != nil {
		t.Fatalf("upload failed: %v", err)
	}
	resp.Body.Close()

	// Check own quota
	qResp, err := ts.GET("/api/auth/me/quota")
	if err != nil {
		t.Fatalf("GET /api/auth/me/quota failed: %v", err)
	}
	defer qResp.Body.Close()

	if qResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", qResp.StatusCode)
	}
}

// TestAdminQuotaView verifies admin can view another user's quota
func TestAdminQuotaView(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	user := ts.CreateTestUserWithGrants(t, "quotaadmin", "secure-password-12345", []map[string]interface{}{
		{"action": constants.AuthActionUpload},
		{"action": constants.AuthActionManageTopics},
	})

	// Admin views the user's quota
	resp, err := ts.GET(fmt.Sprintf("/api/auth/users/%d/quota", user.ID))
	if err != nil {
		t.Fatalf("GET user quota failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}
