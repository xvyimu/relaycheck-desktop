package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCredentialsAreEncryptedAtRestAndExportsAreFingerprinted(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	siteID := newID()
	accountID := newID()
	password := "plain-password-for-test"
	cookie := "session=plain-cookie-for-test"
	accessToken := "plain-access-token-for-test"
	refreshToken := "plain-refresh-token-for-test"
	apiKey := "sk-plain-api-key-for-test"
	encryptedPassword, err := app.encryptText(password)
	if err != nil {
		t.Fatal(err)
	}
	encryptedCookie, err := app.encryptText(cookie)
	if err != nil {
		t.Fatal(err)
	}
	encryptedAccessToken, err := app.encryptText(accessToken)
	if err != nil {
		t.Fatal(err)
	}
	encryptedRefreshToken, err := app.encryptText(refreshToken)
	if err != nil {
		t.Fatal(err)
	}
	encryptedAPIKey, err := app.encryptText(apiKey)
	if err != nil {
		t.Fatal(err)
	}

	_, err = app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, supports_checkin, created_at, updated_at)
		VALUES (?, 'Secure Relay', 'https://secure.example', 'newapi', 'healthy', 1, ?, ?)
	`, siteID, now(), now())
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.db.Exec(`
		INSERT INTO channel_accounts (
			id, upstream_site_id, display_name, auth_type, password_encrypted, cookie_encrypted,
			access_token_encrypted, refresh_token_encrypted, api_key_encrypted, api_key_fingerprint,
			api_key_status, created_at, updated_at
		)
		VALUES (?, ?, 'Secure Account', 'mixed', ?, ?, ?, ?, ?, ?, 'valid', ?, ?)
	`, accountID, siteID, encryptedPassword, encryptedCookie, encryptedAccessToken, encryptedRefreshToken, encryptedAPIKey, secretFingerprint(apiKey), now(), now())
	if err != nil {
		t.Fatal(err)
	}

	var storedPassword, storedCookie, storedAccessToken, storedRefreshToken, storedAPIKey string
	err = app.db.QueryRowContext(context.Background(), `
		SELECT password_encrypted, cookie_encrypted, access_token_encrypted, refresh_token_encrypted, api_key_encrypted
		FROM channel_accounts
		WHERE id=?
	`, accountID).Scan(&storedPassword, &storedCookie, &storedAccessToken, &storedRefreshToken, &storedAPIKey)
	if err != nil {
		t.Fatal(err)
	}
	storedValues := []string{storedPassword, storedCookie, storedAccessToken, storedRefreshToken, storedAPIKey}
	plainValues := []string{password, cookie, accessToken, refreshToken, apiKey}
	for index, stored := range storedValues {
		if !strings.HasPrefix(stored, "v1.") {
			t.Fatalf("expected encrypted v1 envelope, got %q", stored)
		}
		if strings.Contains(stored, plainValues[index]) {
			t.Fatalf("stored secret leaked plaintext %q in %q", plainValues[index], stored)
		}
		decrypted, err := app.decryptText(stored)
		if err != nil {
			t.Fatal(err)
		}
		if decrypted != plainValues[index] {
			t.Fatalf("expected decrypt roundtrip %q, got %q", plainValues[index], decrypted)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/keys/export-preview", nil)
	rec := httptest.NewRecorder()
	app.handleKeyExportPreview(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected export preview 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, secret := range plainValues {
		if strings.Contains(body, secret) {
			t.Fatalf("export preview leaked plaintext secret %q in %s", secret, body)
		}
	}
	var payload apiResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.OK {
		t.Fatalf("expected ok export preview, got %+v", payload)
	}
	if !strings.Contains(body, secretFingerprint(apiKey)) {
		t.Fatalf("expected export preview to include fingerprint, got %s", body)
	}
}
