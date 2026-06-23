package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoadBalanceRefreshAccountIDsSelectsMissingSupportedAccounts(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	supportedSiteID := newID()
	unsupportedSiteID := newID()
	missingAccountID := newID()
	existingBalanceID := newID()
	unsupportedAccountID := newID()
	_, err = app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, supports_balance, created_at, updated_at)
		VALUES
		  (?, 'Supported', 'https://supported.example', 'newapi', 'healthy', 1, ?, ?),
		  (?, 'Unsupported', 'https://unsupported.example', 'newapi', 'healthy', 0, ?, ?)
	`, supportedSiteID, now(), now(), unsupportedSiteID, now(), now())
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.db.Exec(`
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, login_status, balance, created_at, updated_at)
		VALUES
		  (?, ?, 'Missing Balance', 'cookie', 'valid', NULL, ?, ?),
		  (?, ?, 'Existing Balance', 'cookie', 'valid', 12.5, ?, ?),
		  (?, ?, 'Unsupported Balance', 'cookie', 'valid', NULL, ?, ?)
	`, missingAccountID, supportedSiteID, now(), now(), existingBalanceID, supportedSiteID, now(), now(), unsupportedAccountID, unsupportedSiteID, now(), now())
	if err != nil {
		t.Fatal(err)
	}

	ids, err := app.loadBalanceRefreshAccountIDs(context.Background(), 10, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != missingAccountID {
		t.Fatalf("expected only missing supported account, got %v", ids)
	}

	allIDs, err := app.loadBalanceRefreshAccountIDs(context.Background(), 10, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(allIDs) != 2 {
		t.Fatalf("expected two supported accounts when missingOnly=false, got %v", allIDs)
	}
}

func TestLoginWithPasswordReportsEveryCandidatePath(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
	}))
	defer server.Close()

	err = app.loginWithPassword(context.Background(), &accountAuthContext{
		BaseURL:   server.URL,
		LoginName: "user@example.com",
		Password:  "password",
	})
	if err == nil {
		t.Fatal("expected login error")
	}
	message := err.Error()
	for _, expected := range []string{"/api/user/login", "/api/login", "/api/auth/login", "网页登录授权保存会话"} {
		if !strings.Contains(message, expected) {
			t.Fatalf("expected error to contain %q, got %q", expected, message)
		}
	}
}
