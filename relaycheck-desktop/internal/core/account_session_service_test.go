package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAccountSessionServiceLoginWithPasswordSavesTokenAndUserID(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()
	app.allowLocalOutbound = true

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/user/login" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"access_token":"token-from-login","id":42}}`))
	}))
	defer server.Close()

	accountID := newID()
	if _, err := app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, created_at, updated_at)
		VALUES ('token-login-site', 'Token Login Site', ?, 'newapi', 'healthy', ?, ?)
	`, server.URL, now(), now()); err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.Exec(`
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, login_status, created_at, updated_at)
		VALUES (?, 'token-login-site', 'Token Login Account', 'email_password', 'expired', ?, ?)
	`, accountID, now(), now()); err != nil {
		t.Fatal(err)
	}

	auth := &accountAuthContext{
		AccountID: accountID,
		BaseURL:   server.URL,
		LoginName: "user@example.com",
		Password:  "password",
		LoginPath: "/api/user/login",
	}
	service := NewAccountSessionService(app)
	if err := service.LoginWithPassword(context.Background(), auth); err != nil {
		t.Fatalf("LoginWithPassword returned error: %v", err)
	}

	if auth.AccessToken != "token-from-login" {
		t.Fatalf("AccessToken = %q, want token-from-login", auth.AccessToken)
	}
	if auth.AuthUserID != "42" {
		t.Fatalf("AuthUserID = %q, want 42", auth.AuthUserID)
	}

	var encryptedToken, authUserID, loginStatus string
	if err := app.db.QueryRow(`
		SELECT COALESCE(access_token_encrypted,''), COALESCE(auth_user_id,''), login_status
		FROM channel_accounts
		WHERE id=?
	`, accountID).Scan(&encryptedToken, &authUserID, &loginStatus); err != nil {
		t.Fatal(err)
	}
	token, _ := app.decryptText(encryptedToken)
	if token != "token-from-login" {
		t.Fatalf("saved token = %q, want token-from-login", token)
	}
	if authUserID != "42" {
		t.Fatalf("saved auth_user_id = %q, want 42", authUserID)
	}
	if loginStatus != "valid" {
		t.Fatalf("login_status = %q, want valid", loginStatus)
	}
}
