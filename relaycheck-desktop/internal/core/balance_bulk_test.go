package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoadBalanceRefreshAccountIDsSelectsMissingSupportedAccounts(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()
	var err error

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
	app := newTestApp(t)
	defer app.Close()

	var err error

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

func TestLoginWithPasswordSavesCookieFromRedirectResponse(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/user/login":
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "from-redirect", Path: "/api"})
			http.Redirect(w, r, "/dashboard", http.StatusFound)
		case "/dashboard":
			_, _ = w.Write([]byte(`<html>dashboard</html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	app.allowLocalOutbound = true
	accountID := newID()
	_, err := app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, created_at, updated_at)
		VALUES (?, 'Redirect Login Site', ?, 'newapi', 'healthy', ?, ?)
	`, "redirect-login-site", server.URL, now(), now())
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.db.Exec(`
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, login_status, created_at, updated_at)
		VALUES (?, 'redirect-login-site', 'Redirect Login Account', 'email_password', 'expired', ?, ?)
	`, accountID, now(), now())
	if err != nil {
		t.Fatal(err)
	}

	err = app.loginWithPassword(context.Background(), &accountAuthContext{
		AccountID: accountID,
		BaseURL:   server.URL,
		LoginName: "user@example.com",
		Password:  "password",
		LoginPath: "/api/user/login",
		UserAgent: "RelayCheck-Test/1.0",
	})
	if err != nil {
		t.Fatalf("loginWithPassword returned error: %v", err)
	}

	var encryptedCookie, loginStatus string
	if err := app.db.QueryRow(`SELECT COALESCE(cookie_encrypted,''), login_status FROM channel_accounts WHERE id=?`, accountID).Scan(&encryptedCookie, &loginStatus); err != nil {
		t.Fatal(err)
	}
	cookie, _ := app.decryptText(encryptedCookie)
	if cookie != "session=from-redirect" {
		t.Fatalf("expected redirect cookie to be saved, got %q", cookie)
	}
	if loginStatus != "valid" {
		t.Fatalf("expected login_status valid, got %q", loginStatus)
	}
}

func TestResolveLoginTargetURLHandlesRelativeLoginURL(t *testing.T) {
	got := resolveLoginTargetURL("https://relay.example/base", "/console/login?next=%2Fdashboard")
	want := "https://relay.example/console/login?next=%2Fdashboard"
	if got != want {
		t.Fatalf("resolveLoginTargetURL() = %q, want %q", got, want)
	}
}

func TestResolveLoginTargetURLRejectsUnsafeResolvedURL(t *testing.T) {
	for _, loginURL := range []string{"//evil.example/login", "javascript:alert(1)"} {
		got := resolveLoginTargetURL("https://relay.example/base", loginURL)
		want := "https://relay.example/login"
		if got != want {
			t.Fatalf("resolveLoginTargetURL(%q) = %q, want %q", loginURL, got, want)
		}
	}
}
