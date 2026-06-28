package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestKeyExportPreviewResponseBodyExcludesPlaintextSecret 验证：Key 安全导出预览的
// HTTP 响应体本身不包含明文 API Key，只包含指纹。这是对 audit_test.go 中审计日志
// 断言的补充——审计干净不等于响应体干净，两者必须分别锁定。
func TestKeyExportPreviewResponseBodyExcludesPlaintextSecret(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	siteID := newID()
	accountID := newID()
	apiKey := "sk-export-body-secret-12345"
	encryptedAPIKey, err := app.encryptText(apiKey)
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, created_at, updated_at)
		VALUES (?, 'Export Body Relay', 'https://export-body.example', 'newapi', 'healthy', ?, ?)
	`, siteID, now(), now())
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.db.Exec(`
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, api_key_encrypted, api_key_fingerprint, api_key_status, api_key_model_count, api_key_model_usable, api_key_latency_ms, created_at, updated_at)
		VALUES (?, ?, 'Export Body Account', 'api_key', ?, ?, 'valid', 2, 1, 120, ?, ?)
	`, accountID, siteID, encryptedAPIKey, secretFingerprint(apiKey), now(), now())
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/keys/export-preview", nil)
	rec := httptest.NewRecorder()
	app.handleKeyExportPreview(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if strings.Contains(body, apiKey) {
		t.Fatalf("response body leaked plaintext API key %q:\n%s", apiKey, body)
	}

	var response struct {
		OK   bool             `json:"ok"`
		Data keyExportPreview `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	preview := response.Data
	if preview.Total != 1 {
		t.Fatalf("expected total=1, got %d", preview.Total)
	}
	if preview.Valid != 1 {
		t.Fatalf("expected valid=1, got %d", preview.Valid)
	}
	if preview.Usable != 1 {
		t.Fatalf("expected usable=1, got %d", preview.Usable)
	}
	if len(preview.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(preview.Items))
	}
	item := preview.Items[0]
	if item.Fingerprint == "" {
		t.Fatal("expected non-empty fingerprint")
	}
	if item.Fingerprint == apiKey {
		t.Fatalf("fingerprint must not equal plaintext key")
	}
	if strings.Contains(item.MaskedExportRef, apiKey) {
		t.Fatalf("maskedExportRef leaked key: %s", item.MaskedExportRef)
	}
	if !strings.Contains(item.MaskedExportRef, item.Fingerprint) {
		t.Fatalf("expected maskedExportRef to contain fingerprint, got %q", item.MaskedExportRef)
	}
	if !strings.Contains(preview.Notice, "不导出真实 API Key") {
		t.Fatalf("expected notice to warn about no plaintext export, got %q", preview.Notice)
	}
}

// TestKeyExportPreviewOrdersValidFirst 验证：导出预览将 valid 状态的账号排在前面，
// 且无效状态排在后面，方便用户优先查看可用 Key。
func TestKeyExportPreviewOrdersValidFirst(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()
	var err error

	siteID := newID()
	_, err = app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, created_at, updated_at)
		VALUES (?, 'Order Relay', 'https://order.example', 'newapi', 'healthy', ?, ?)
	`, siteID, now(), now())
	if err != nil {
		t.Fatal(err)
	}

	validKey, _ := app.encryptText("sk-valid-order")
	expiredKey, _ := app.encryptText("sk-expired-order")
	_, err = app.db.Exec(`
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, api_key_encrypted, api_key_fingerprint, api_key_status, created_at, updated_at)
		VALUES (?, ?, 'Expired Account', 'api_key', ?, ?, 'expired', ?, ?),
		       (?, ?, 'Valid Account', 'api_key', ?, ?, 'valid', ?, ?)
	`, newID(), siteID, expiredKey, secretFingerprint("sk-expired-order"), now(), now(),
		newID(), siteID, validKey, secretFingerprint("sk-valid-order"), now(), now())
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/keys/export-preview", nil)
	rec := httptest.NewRecorder()
	app.handleKeyExportPreview(rec, req)
	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var response struct {
		OK   bool             `json:"ok"`
		Data keyExportPreview `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	preview := response.Data
	if len(preview.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(preview.Items))
	}
	if preview.Items[0].Status != "valid" {
		t.Fatalf("expected valid account first, got status=%q name=%q", preview.Items[0].Status, preview.Items[0].AccountName)
	}
	if preview.Items[1].Status != "expired" {
		t.Fatalf("expected expired account second, got status=%q name=%q", preview.Items[1].Status, preview.Items[1].AccountName)
	}
}

// TestKeyExportPreviewSkipsAccountsWithoutFingerprint 验证：没有 fingerprint 的账号
// 不会被纳入导出预览，避免泄露未检测的账号信息。
func TestKeyExportPreviewSkipsAccountsWithoutFingerprint(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()
	var err error

	siteID := newID()
	_, err = app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, created_at, updated_at)
		VALUES (?, 'Skip Relay', 'https://skip.example', 'newapi', 'healthy', ?, ?)
	`, siteID, now(), now())
	if err != nil {
		t.Fatal(err)
	}

	withFingerprint, _ := app.encryptText("sk-with-fp")
	_, err = app.db.Exec(`
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, api_key_encrypted, api_key_fingerprint, created_at, updated_at)
		VALUES (?, ?, 'With Fingerprint', 'api_key', ?, 'fp-1234', ?, ?),
		       (?, ?, 'No Fingerprint', 'api_key', ?, '', ?, ?)
	`, newID(), siteID, withFingerprint, now(), now(),
		newID(), siteID, withFingerprint, now(), now())
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/keys/export-preview", nil)
	rec := httptest.NewRecorder()
	app.handleKeyExportPreview(rec, req)
	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var response struct {
		OK   bool             `json:"ok"`
		Data keyExportPreview `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	preview := response.Data
	if preview.Total != 1 {
		t.Fatalf("expected only 1 account with fingerprint, got total=%d", preview.Total)
	}
	if len(preview.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(preview.Items))
	}
	if preview.Items[0].AccountName != "With Fingerprint" {
		t.Fatalf("expected 'With Fingerprint', got %q", preview.Items[0].AccountName)
	}
}
