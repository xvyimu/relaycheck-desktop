package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIKeyCheckFetchesModelsAndSpeedTestsModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("authorization") != "Bearer sk-valid" {
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

	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.client = server.Client()
	app.allowLocalOutbound = true

	siteID := newID()
	accountID := newID()
	apiKeyEncrypted, err := app.encryptText("sk-valid")
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, created_at, updated_at)
		VALUES (?, 'Test Relay', ?, 'newapi', 'healthy', ?, ?)
	`, siteID, server.URL, now(), now())
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.db.Exec(`
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, api_key_encrypted, api_key_fingerprint, login_status, created_at, updated_at)
		VALUES (?, ?, 'Key Account', 'api_key', ?, ?, 'unknown', ?, ?)
	`, accountID, siteID, apiKeyEncrypted, secretFingerprint("sk-valid"), now(), now())
	if err != nil {
		t.Fatal(err)
	}

	result := app.testAPIKeyForAccount(context.Background(), accountID)
	if result.Status != "valid" {
		t.Fatalf("expected valid, got %+v", result)
	}
	if result.ModelCount != 2 {
		t.Fatalf("expected 2 models, got %+v", result)
	}
	if result.TestedModel != "gpt-4o-mini" {
		t.Fatalf("expected preferred test model, got %s", result.TestedModel)
	}
	if !result.ModelUsable {
		t.Fatalf("expected model usable, got %+v", result)
	}
	if result.ModelTestLatencyMs < 0 {
		t.Fatalf("expected latency, got %+v", result)
	}

	account, err := app.loadAccountByID(context.Background(), accountID)
	if err != nil {
		t.Fatal(err)
	}
	if account.APIKeyModelCount != 2 {
		t.Fatalf("expected persisted model count, got %+v", account)
	}
	if len(account.APIKeySampleModels) != 2 || account.APIKeySampleModels[1] != "gpt-4o-mini" {
		t.Fatalf("expected persisted sample models, got %+v", account.APIKeySampleModels)
	}
	if account.APIKeyTestModel != "gpt-4o-mini" {
		t.Fatalf("expected persisted test model, got %+v", account.APIKeyTestModel)
	}
	if !account.APIKeyModelUsable {
		t.Fatalf("expected persisted usable model flag, got %+v", account)
	}
	if account.APIKeyLatencyMs < 0 {
		t.Fatalf("expected persisted latency, got %+v", account.APIKeyLatencyMs)
	}
}
