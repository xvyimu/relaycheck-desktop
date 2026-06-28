package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuditStoresMetadataWithoutSecrets(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	app.audit("account.updated", "info", "tester", "account", "acct_1", "账号已更新", map[string]interface{}{
		"updatedFields": []string{"password", "apiKey"},
	})

	var summary, metadata string
	if err := app.db.QueryRow(`SELECT summary, metadata_json FROM audit_log WHERE action='account.updated'`).Scan(&summary, &metadata); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(summary, "账号已更新") {
		t.Fatalf("unexpected summary: %s", summary)
	}
	if strings.Contains(strings.ToLower(metadata), "sk-") || strings.Contains(strings.ToLower(metadata), "password123") {
		t.Fatalf("metadata should not contain secret values: %s", metadata)
	}
	if !strings.Contains(metadata, "updatedFields") {
		t.Fatalf("expected structured metadata, got %s", metadata)
	}
}

func TestKeyExportPreviewWritesAuditWithoutPlaintextSecret(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	siteID := newID()
	accountID := newID()
	apiKey := "sk-audit-export-secret"
	encryptedAPIKey, err := app.encryptText(apiKey)
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, created_at, updated_at)
		VALUES (?, 'Audit Relay', 'https://audit.example', 'newapi', 'healthy', ?, ?)
	`, siteID, now(), now())
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.db.Exec(`
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, api_key_encrypted, api_key_fingerprint, api_key_status, created_at, updated_at)
		VALUES (?, ?, 'Audit Account', 'api_key', ?, ?, 'valid', ?, ?)
	`, accountID, siteID, encryptedAPIKey, secretFingerprint(apiKey), now(), now())
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/keys/export-preview", nil)
	rec := httptest.NewRecorder()
	app.handleKeyExportPreview(rec, req)
	if rec.Code != 200 {
		t.Fatalf("expected export preview 200, got %d", rec.Code)
	}

	var summary, metadata string
	if err := app.db.QueryRow(`SELECT summary, metadata_json FROM audit_log WHERE action='keys.export_preview'`).Scan(&summary, &metadata); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(summary, "Key 脱敏导出预览") {
		t.Fatalf("unexpected summary: %s", summary)
	}
	if strings.Contains(summary+metadata, apiKey) {
		t.Fatalf("audit log leaked plaintext key: %s %s", summary, metadata)
	}
	if !strings.Contains(metadata, `"total":1`) {
		t.Fatalf("expected export counts in metadata, got %s", metadata)
	}
}

func TestClearAccountSessionWritesBrowserDisconnectAudit(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	siteID := newID()
	accountID := newID()
	cookieEncrypted, err := app.encryptText("session=audit-cookie")
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.db.ExecContext(context.Background(), `
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, created_at, updated_at)
		VALUES (?, 'Browser Audit Relay', 'https://browser-audit.example', 'newapi', 'healthy', ?, ?)
	`, siteID, now(), now())
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.db.ExecContext(context.Background(), `
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, cookie_encrypted, browser_profile_path, login_status, created_at, updated_at)
		VALUES (?, ?, 'Browser Audit Account', 'browser_profile', ?, '', 'valid', ?, ?)
	`, accountID, siteID, cookieEncrypted, now(), now())
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/accounts/"+accountID+"/clear-session", nil)
	rec := httptest.NewRecorder()
	app.clearAccountSession(rec, req, accountID)
	if rec.Code != 200 {
		t.Fatalf("expected clear session 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var action, summary string
	if err := app.db.QueryRow(`SELECT action, summary FROM audit_log WHERE resource_id=?`, accountID).Scan(&action, &summary); err != nil {
		t.Fatal(err)
	}
	if action != "browser_auth.disconnected" {
		t.Fatalf("expected browser disconnect audit, got %s", action)
	}
	if !strings.Contains(summary, "网页登录授权已断开") {
		t.Fatalf("unexpected summary: %s", summary)
	}
}
