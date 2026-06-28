package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestBulkTestAPIKeysReturnsSummaryAndExcludesPlaintextSecret 验证：批量重测 API Key
// 接口返回正确的 processed/valid/usable 计数，且响应体不包含明文 API Key。
func TestBulkTestAPIKeysReturnsSummaryAndExcludesPlaintextSecret(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			if r.Header.Get("authorization") != "Bearer sk-bulk-test-secret" {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4o-mini"},{"id":"claude-3-5-haiku"}]}`))
			return
		}
		if r.URL.Path == "/v1/chat/completions" {
			_, _ = w.Write([]byte(`{"id":"chatcmpl-test","choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	app := newTestApp(t)
	defer app.Close()
	app.allowLocalOutbound = true

	siteID := newID()
	apiKey := "sk-bulk-test-secret"
	encryptedAPIKey, err := app.encryptText(apiKey)
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, created_at, updated_at)
		VALUES (?, 'Bulk Test Relay', ?, 'newapi', 'healthy', ?, ?)
	`, siteID, server.URL, now(), now())
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.db.Exec(`
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, api_key_encrypted, api_key_fingerprint, created_at, updated_at)
		VALUES (?, ?, 'Bulk Test Account', 'api_key', ?, ?, ?, ?)
	`, newID(), siteID, encryptedAPIKey, secretFingerprint(apiKey), now(), now())
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/accounts/bulk-test-api-keys", strings.NewReader(`{"limit":10}`))
	req.Header.Set("content-type", "application/json")
	rec := httptest.NewRecorder()
	app.handleBulkTestAPIKeys(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if strings.Contains(body, apiKey) {
		t.Fatalf("response body leaked plaintext API key %q:\n%s", apiKey, body)
	}

	var response struct {
		OK   bool `json:"ok"`
		Data struct {
			Processed int                `json:"processed"`
			Valid      int                `json:"valid"`
			Usable     int                `json:"usable"`
			Invalid    int                `json:"invalid"`
			Results    []apiKeyTestResult `json:"results"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Processed != 1 {
		t.Fatalf("expected processed=1, got %d", response.Data.Processed)
	}
	if response.Data.Valid != 1 {
		t.Fatalf("expected valid=1, got %d", response.Data.Valid)
	}
	if response.Data.Usable != 1 {
		t.Fatalf("expected usable=1, got %d", response.Data.Usable)
	}
	if response.Data.Invalid != 0 {
		t.Fatalf("expected invalid=0, got %d", response.Data.Invalid)
	}
	if len(response.Data.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(response.Data.Results))
	}
	result := response.Data.Results[0]
	if result.Status != "valid" {
		t.Fatalf("expected result status=valid, got %q", result.Status)
	}
	if result.ModelCount != 2 {
		t.Fatalf("expected model count=2, got %d", result.ModelCount)
	}
	if !result.ModelUsable {
		t.Fatal("expected model usable=true")
	}
	if result.Fingerprint == "" {
		t.Fatal("expected non-empty fingerprint")
	}
	if result.Fingerprint == apiKey {
		t.Fatalf("fingerprint must not equal plaintext key")
	}
}

// TestBulkTestAPIKeysSkipsAccountsWithoutKey 验证：没有 API Key 的账号不会被纳入
// 批量重测，避免对无 Key 账号发起无意义的请求。
func TestBulkTestAPIKeysSkipsAccountsWithoutKey(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()
	var err error

	siteID := newID()
	_, err = app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, created_at, updated_at)
		VALUES (?, 'No Key Relay', 'https://no-key.example', 'newapi', 'healthy', ?, ?)
	`, siteID, now(), now())
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.db.Exec(`
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, created_at, updated_at)
		VALUES (?, ?, 'No Key Account', 'api_key', ?, ?)
	`, newID(), siteID, now(), now())
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/accounts/bulk-test-api-keys", strings.NewReader(`{"limit":10}`))
	req.Header.Set("content-type", "application/json")
	rec := httptest.NewRecorder()
	app.handleBulkTestAPIKeys(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var response struct {
		OK   bool `json:"ok"`
		Data struct {
			Processed int `json:"processed"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Processed != 0 {
		t.Fatalf("expected 0 processed (no accounts with key), got %d", response.Data.Processed)
	}
}

// TestBulkTestAPIKeysClampsLimit 验证：批量重测的 limit 参数会被 clamp 到合法范围，
// 防止过大 limit 对上游站点造成瞬时压力（符合 project_memory 中批量动作 1..10 的约定）。
func TestBulkTestAPIKeysClampsLimit(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	// limit=0 应被 clamp 到默认值（不会返回 error）
	req := httptest.NewRequest(http.MethodPost, "/api/accounts/bulk-test-api-keys", strings.NewReader(`{"limit":0}`))
	req.Header.Set("content-type", "application/json")
	rec := httptest.NewRecorder()
	app.handleBulkTestAPIKeys(rec, req)
	if rec.Code != 200 {
		t.Fatalf("expected 200 for limit=0, got %d: %s", rec.Code, rec.Body.String())
	}

	// limit=999 应被 clamp 到 10，不会 panic 或报错
	req = httptest.NewRequest(http.MethodPost, "/api/accounts/bulk-test-api-keys", strings.NewReader(`{"limit":999}`))
	req.Header.Set("content-type", "application/json")
	rec = httptest.NewRecorder()
	app.handleBulkTestAPIKeys(rec, req)
	if rec.Code != 200 {
		t.Fatalf("expected 200 for limit=999, got %d: %s", rec.Code, rec.Body.String())
	}
}
