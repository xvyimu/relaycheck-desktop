package core

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAccountTaskServiceStartTestKeysPublishesProgress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("authorization") != "Bearer sk-task-valid" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/v1/models":
			_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"deepseek-chat"},{"id":"gpt-4o-mini"}]}`))
		case "/v1/chat/completions":
			_, _ = w.Write([]byte(`{"id":"chatcmpl-test","choices":[{"message":{"content":"OK"}}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	app := newTestApp(t)
	app.client = server.Client()
	app.allowLocalOutbound = true

	siteID := newID()
	accountID := newID()
	apiKeyEncrypted, err := app.encryptText("sk-task-valid")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, created_at, updated_at)
		VALUES (?, 'Task Key Relay', ?, 'newapi', 'healthy', ?, ?)
	`, siteID, server.URL, now(), now()); err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.Exec(`
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, api_key_encrypted, api_key_fingerprint, login_status, created_at, updated_at)
		VALUES (?, ?, 'Task Key Account', 'api_key', ?, ?, 'unknown', ?, ?)
	`, accountID, siteID, apiKeyEncrypted, secretFingerprint("sk-task-valid"), now(), now()); err != nil {
		t.Fatal(err)
	}

	app.accountTasks.StartTestKeys("task-account-test-keys", map[string]interface{}{"limit": float64(5)})
	progress := waitForTaskDone(t, app, "task-account-test-keys")

	if progress.Type != TaskTestKeys {
		t.Fatalf("task type = %q, want %q", progress.Type, TaskTestKeys)
	}
	if progress.Status != TaskStatusDone {
		t.Fatalf("task status = %q, want done: %#v", progress.Status, progress)
	}
	if progress.Total != 1 || progress.Current != 1 {
		t.Fatalf("progress = %d/%d, want 1/1", progress.Current, progress.Total)
	}
	if len(progress.Results) != 1 {
		t.Fatalf("result count = %d, want 1", len(progress.Results))
	}
	result := progress.Results[0]
	if result.ID != accountID || result.Name != "Task Key Account" || result.Status != "valid" {
		t.Fatalf("unexpected task result: %#v", result)
	}
	if result.Message != "" {
		t.Fatalf("valid API key task result should not include message, got %q", result.Message)
	}

	account, err := app.loadAccountByID(t.Context(), accountID)
	if err != nil {
		t.Fatal(err)
	}
	if account.APIKeyStatus != "valid" {
		t.Fatalf("APIKeyStatus = %q, want valid", account.APIKeyStatus)
	}
	if account.APIKeyModelCount != 2 {
		t.Fatalf("APIKeyModelCount = %d, want 2", account.APIKeyModelCount)
	}
	if !account.APIKeyModelUsable {
		t.Fatal("APIKeyModelUsable = false, want true")
	}
}
