package e2e

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"meshbank/internal/constants"

	_ "github.com/mattn/go-sqlite3"
)

// TestReconcile_PurgesOrphanedTopicAfterFolderRemoved verifies the full
// reconciliation lifecycle:
//  1. Create topic and upload assets
//  2. Verify assets are in the orchestrator index
//  3. Manually remove the topic folder from disk
//  4. Run reconciliation
//  5. Verify orphaned index entries are purged
//  6. Verify audit log records the reconciliation
//  7. Verify the topic is no longer listed
//  8. Verify downloads for those hashes return errors
func TestReconcile_PurgesOrphanedTopicAfterFolderRemoved(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// --- Step 1: Create topic and upload assets ---
	ts.CreateTopic(t, "doomed-topic")
	hash1 := ts.UploadFileExpectSuccess(t, "doomed-topic", "file1.txt", []byte("content-1"), "")
	hash2 := ts.UploadFileExpectSuccess(t, "doomed-topic", "file2.txt", []byte("content-2"), "")

	// Also create a surviving topic with its own assets
	ts.CreateTopic(t, "surviving-topic")
	survivorHash := ts.UploadFileExpectSuccess(t, "surviving-topic", "survivor.txt", []byte("survivor-content"), "")

	// --- Step 2: Verify assets exist in orchestrator index ---
	orchDB := ts.GetOrchestratorDB(t)

	var count int
	err := orchDB.QueryRow("SELECT COUNT(*) FROM asset_index WHERE topic = ?", "doomed-topic").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count doomed-topic entries: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 entries for doomed-topic, got %d", count)
	}

	// Verify downloads work before removal
	ts.DownloadAsset(t, hash1.Hash)
	ts.DownloadAsset(t, hash2.Hash)

	// Close the direct orchestrator DB connection before manipulating files
	orchDB.Close()

	// --- Step 3: Manually remove the topic folder ---
	doomedPath := filepath.Join(ts.WorkDir, "doomed-topic")
	if err := os.RemoveAll(doomedPath); err != nil {
		t.Fatalf("failed to remove topic folder: %v", err)
	}

	// Verify folder is gone
	if _, err := os.Stat(doomedPath); !os.IsNotExist(err) {
		t.Fatalf("topic folder should not exist after removal")
	}

	// --- Step 4: Run reconciliation ---
	result, err := ts.App.Services.Reconcile.Reconcile()
	if err != nil {
		t.Fatalf("reconciliation failed: %v", err)
	}

	// --- Step 5: Verify reconciliation result ---
	if result.TopicsRemoved != 1 {
		t.Errorf("expected 1 topic removed, got %d", result.TopicsRemoved)
	}
	if result.EntriesPurged != 2 {
		t.Errorf("expected 2 entries purged, got %d", result.EntriesPurged)
	}
	if len(result.RemovedTopics) != 1 || result.RemovedTopics[0] != "doomed-topic" {
		t.Errorf("expected removed topics = [doomed-topic], got %v", result.RemovedTopics)
	}

	// --- Step 6: Verify index is clean ---
	orchDB2 := ts.GetOrchestratorDB(t)

	var doomedCount int
	err = orchDB2.QueryRow("SELECT COUNT(*) FROM asset_index WHERE topic = ?", "doomed-topic").Scan(&doomedCount)
	if err != nil {
		t.Fatalf("failed to count doomed-topic entries after reconcile: %v", err)
	}
	if doomedCount != 0 {
		t.Errorf("expected 0 entries for doomed-topic after reconcile, got %d", doomedCount)
	}

	// Verify surviving topic entries are untouched
	var survivorCount int
	err = orchDB2.QueryRow("SELECT COUNT(*) FROM asset_index WHERE topic = ?", "surviving-topic").Scan(&survivorCount)
	if err != nil {
		t.Fatalf("failed to count surviving-topic entries: %v", err)
	}
	if survivorCount != 1 {
		t.Errorf("expected 1 entry for surviving-topic, got %d", survivorCount)
	}

	// --- Step 7: Verify audit log ---
	var auditCount int
	err = orchDB2.QueryRow("SELECT COUNT(*) FROM audit_log WHERE action = ?",
		constants.AuditActionReconcileTopicRemoved).Scan(&auditCount)
	if err != nil {
		t.Fatalf("failed to count audit entries: %v", err)
	}
	if auditCount != 1 {
		t.Errorf("expected 1 reconcile_topic_removed audit entry, got %d", auditCount)
	}

	// Verify audit entry details
	var detailsJSON sql.NullString
	err = orchDB2.QueryRow(
		"SELECT details_json FROM audit_log WHERE action = ?",
		constants.AuditActionReconcileTopicRemoved,
	).Scan(&detailsJSON)
	if err != nil {
		t.Fatalf("failed to read audit details: %v", err)
	}
	if detailsJSON.Valid {
		var details map[string]interface{}
		if err := json.Unmarshal([]byte(detailsJSON.String), &details); err != nil {
			t.Fatalf("failed to parse audit details JSON: %v", err)
		}
		if details["topic_name"] != "doomed-topic" {
			t.Errorf("audit details topic_name = %v, want doomed-topic", details["topic_name"])
		}
		if purged, ok := details["entries_purged"].(float64); !ok || int(purged) != 2 {
			t.Errorf("audit details entries_purged = %v, want 2", details["entries_purged"])
		}
	} else {
		t.Error("expected audit entry to have details_json")
	}

	orchDB2.Close()

	// --- Step 8: Verify topic is no longer listed ---
	topicsResp := ts.GetTopics(t)
	for _, topic := range topicsResp.Topics {
		if topic.Name == "doomed-topic" {
			t.Errorf("doomed-topic should not appear in topics list after reconciliation")
		}
	}

	// Verify surviving topic is still accessible
	ts.DownloadAsset(t, survivorHash.Hash)
}

// TestReconcile_NoOrphans_NoChanges verifies reconciliation is a no-op
// when all indexed topics still exist on disk.
func TestReconcile_NoOrphans_NoChanges(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	ts.CreateTopic(t, "healthy-topic")
	ts.UploadFileExpectSuccess(t, "healthy-topic", "test.txt", []byte("data"), "")

	// Verify index entry exists
	orchDB := ts.GetOrchestratorDB(t)
	var count int
	err := orchDB.QueryRow("SELECT COUNT(*) FROM asset_index WHERE topic = ?", "healthy-topic").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 entry, got %d", count)
	}
	orchDB.Close()

	// Run reconciliation — should be a no-op
	result, err := ts.App.Services.Reconcile.Reconcile()
	if err != nil {
		t.Fatalf("reconciliation failed: %v", err)
	}
	if result.TopicsRemoved != 0 {
		t.Errorf("expected 0 topics removed, got %d", result.TopicsRemoved)
	}
	if result.EntriesPurged != 0 {
		t.Errorf("expected 0 entries purged, got %d", result.EntriesPurged)
	}

	// Verify entry is still there
	orchDB2 := ts.GetOrchestratorDB(t)
	err = orchDB2.QueryRow("SELECT COUNT(*) FROM asset_index WHERE topic = ?", "healthy-topic").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count after reconcile: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 entry to remain, got %d", count)
	}
	orchDB2.Close()
}

// TestReconcile_MultipleOrphanedTopics verifies reconciliation handles
// multiple orphaned topics in a single pass.
func TestReconcile_MultipleOrphanedTopics(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Create several topics with uploads
	topicNames := []string{"topic-alpha", "topic-beta", "topic-gamma"}
	hashes := make(map[string][]string)

	for _, name := range topicNames {
		ts.CreateTopic(t, name)
		for i := 0; i < 3; i++ {
			content := []byte(fmt.Sprintf("content-%s-%d", name, i))
			resp := ts.UploadFileExpectSuccess(t, name, fmt.Sprintf("file-%d.txt", i), content, "")
			hashes[name] = append(hashes[name], resp.Hash)
		}
	}

	// Remove topic-alpha and topic-gamma, keep topic-beta
	for _, name := range []string{"topic-alpha", "topic-gamma"} {
		if err := os.RemoveAll(filepath.Join(ts.WorkDir, name)); err != nil {
			t.Fatalf("failed to remove %s: %v", name, err)
		}
	}

	result, err := ts.App.Services.Reconcile.Reconcile()
	if err != nil {
		t.Fatalf("reconciliation failed: %v", err)
	}

	if result.TopicsRemoved != 2 {
		t.Errorf("expected 2 topics removed, got %d", result.TopicsRemoved)
	}
	if result.EntriesPurged != 6 {
		t.Errorf("expected 6 entries purged (3 per topic × 2), got %d", result.EntriesPurged)
	}

	// Verify topic-beta is fully intact
	orchDB := ts.GetOrchestratorDB(t)
	var betaCount int
	err = orchDB.QueryRow("SELECT COUNT(*) FROM asset_index WHERE topic = ?", "topic-beta").Scan(&betaCount)
	if err != nil {
		t.Fatalf("failed to count topic-beta: %v", err)
	}
	if betaCount != 3 {
		t.Errorf("expected 3 entries for topic-beta, got %d", betaCount)
	}
	orchDB.Close()

	// Downloads for topic-beta should still work
	for _, h := range hashes["topic-beta"] {
		ts.DownloadAsset(t, h)
	}
}

// TestReconcile_DownloadFailsAfterReconciliation verifies that downloading
// an asset from a reconciled (removed) topic returns an error.
func TestReconcile_DownloadFailsAfterReconciliation(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	ts.CreateTopic(t, "removed-topic")
	uploaded := ts.UploadFileExpectSuccess(t, "removed-topic", "test.bin", []byte("binary-data"), "")

	// Verify download works
	ts.DownloadAsset(t, uploaded.Hash)

	// Remove folder and reconcile
	os.RemoveAll(filepath.Join(ts.WorkDir, "removed-topic"))

	_, err := ts.App.Services.Reconcile.Reconcile()
	if err != nil {
		t.Fatalf("reconciliation failed: %v", err)
	}

	// Download should fail — hash no longer in index
	resp, err := ts.GET("/api/assets/" + uploaded.Hash + "/download")
	if err != nil {
		t.Fatalf("download request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		t.Error("expected download to fail after reconciliation, but got 200 OK")
	}
}

// TestReconcile_Idempotent verifies that running reconciliation multiple
// times produces the same result (second run is a no-op).
func TestReconcile_Idempotent(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	ts.CreateTopic(t, "idempotent-topic")
	ts.UploadFileExpectSuccess(t, "idempotent-topic", "file.txt", []byte("test"), "")

	os.RemoveAll(filepath.Join(ts.WorkDir, "idempotent-topic"))

	// First reconciliation — should clean up
	result1, err := ts.App.Services.Reconcile.Reconcile()
	if err != nil {
		t.Fatalf("first reconciliation failed: %v", err)
	}
	if result1.TopicsRemoved != 1 {
		t.Errorf("first pass: expected 1 topic removed, got %d", result1.TopicsRemoved)
	}

	// Second reconciliation — should be a no-op
	result2, err := ts.App.Services.Reconcile.Reconcile()
	if err != nil {
		t.Fatalf("second reconciliation failed: %v", err)
	}
	if result2.TopicsRemoved != 0 {
		t.Errorf("second pass: expected 0 topics removed, got %d", result2.TopicsRemoved)
	}
	if result2.EntriesPurged != 0 {
		t.Errorf("second pass: expected 0 entries purged, got %d", result2.EntriesPurged)
	}
}

// TestReconcile_AuditLogPreserved verifies that audit log entries from
// normal operations are preserved after reconciliation (only asset_index
// entries are purged, not audit trail).
func TestReconcile_AuditLogPreserved(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	ts.CreateTopic(t, "audit-test")
	ts.UploadFileExpectSuccess(t, "audit-test", "file.txt", []byte("data"), "")

	// Check audit entries before reconciliation (adding_topic + adding_file)
	orchDB := ts.GetOrchestratorDB(t)
	var auditBefore int
	err := orchDB.QueryRow("SELECT COUNT(*) FROM audit_log").Scan(&auditBefore)
	if err != nil {
		t.Fatalf("failed to count audit entries: %v", err)
	}
	orchDB.Close()

	// Remove and reconcile
	os.RemoveAll(filepath.Join(ts.WorkDir, "audit-test"))
	_, err = ts.App.Services.Reconcile.Reconcile()
	if err != nil {
		t.Fatalf("reconciliation failed: %v", err)
	}

	// Audit entries should have INCREASED (original entries preserved + reconcile entry added)
	orchDB2 := ts.GetOrchestratorDB(t)
	var auditAfter int
	err = orchDB2.QueryRow("SELECT COUNT(*) FROM audit_log").Scan(&auditAfter)
	if err != nil {
		t.Fatalf("failed to count audit entries after: %v", err)
	}
	orchDB2.Close()

	if auditAfter <= auditBefore {
		t.Errorf("audit log should have grown: before=%d, after=%d", auditBefore, auditAfter)
	}

	// Verify we can still query the audit API
	resp, err := ts.GET("/api/audit?action=" + constants.AuditActionReconcileTopicRemoved)
	if err != nil {
		t.Fatalf("audit query failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("audit query returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var auditResp AuditQueryResponse
	if err := json.Unmarshal(bodyBytes, &auditResp); err != nil {
		t.Fatalf("failed to parse audit response: %v", err)
	}
	if len(auditResp.Entries) == 0 {
		t.Error("expected at least 1 reconcile_topic_removed audit entry via API")
	}
}
