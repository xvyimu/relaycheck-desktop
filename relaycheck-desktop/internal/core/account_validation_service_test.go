package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAccountValidationServiceDoesNotReturnDatabaseErrors(t *testing.T) {
	app := newTestApp(t)
	if err := app.db.Close(); err != nil {
		t.Fatal(err)
	}

	result := app.accountValidation.TestAPIKey(t.Context(), "account-1", nil)
	if result.Message != "加载账号授权失败。" {
		t.Fatalf("unexpected API key load public message: %q", result.Message)
	}
	if strings.Contains(strings.ToLower(result.Message), "closed") || strings.Contains(result.Message, "C:\\") {
		t.Fatalf("API key result leaked an internal database error: %q", result.Message)
	}
}

func TestAccountValidationServiceDoesNotReturnUpstreamBodies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"token=TOP_SECRET"}`))
	}))
	defer server.Close()

	app := newTestApp(t)
	defer app.Close()
	app.allowLocalOutbound = true

	result := app.accountValidation.TestAPIKey(t.Context(), "account-1", &accountAuthContext{
		AccountID: "account-1",
		BaseURL:   server.URL,
		APIKey:    "sk-validation",
	})
	if strings.Contains(result.Message, "TOP_SECRET") || strings.Contains(result.ModelTestMessage, "TOP_SECRET") {
		t.Fatalf("API key result leaked an upstream body: message=%q modelMessage=%q", result.Message, result.ModelTestMessage)
	}
}

func TestAccountValidationServiceTestLoginUsesAccountAPIHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/user/self" {
			t.Fatalf("path = %q, want /api/user/self", r.URL.Path)
		}
		if got := r.Header.Get("user-agent"); got != "RelayCheck-Test/1.0" {
			t.Fatalf("user-agent = %q, want RelayCheck-Test/1.0", got)
		}
		if got := r.Header.Get("cookie"); got != "session=abc" {
			t.Fatalf("cookie = %q, want session=abc", got)
		}
		if got := r.Header.Get("New-Api-User"); got != "42" {
			t.Fatalf("New-Api-User = %q, want 42", got)
		}
		if got := r.Header.Get("authorization"); got != "Bearer access-token" {
			t.Fatalf("authorization = %q, want bearer access token", got)
		}
		_, _ = w.Write([]byte(`{"id":42}`))
	}))
	defer server.Close()

	app := newTestApp(t)
	defer app.Close()
	app.client = server.Client()
	app.allowLocalOutbound = true

	siteID := newID()
	accountID := newID()
	cookieEncrypted, err := app.encryptText("session=abc")
	if err != nil {
		t.Fatal(err)
	}
	accessEncrypted, err := app.encryptText("access-token")
	if err != nil {
		t.Fatal(err)
	}
	apiKeyEncrypted, err := app.encryptText("sk-fallback")
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, created_at, updated_at)
		VALUES (?, 'Login Test Relay', ?, 'newapi', 'healthy', ?, ?)
	`, siteID, server.URL, now(), now())
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.db.Exec(`
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, cookie_encrypted, access_token_encrypted, api_key_encrypted, auth_user_id, user_agent, login_status, created_at, updated_at)
		VALUES (?, ?, 'Login Test Account', 'browser_profile', ?, ?, ?, '42', 'RelayCheck-Test/1.0', 'unknown', ?, ?)
	`, accountID, siteID, cookieEncrypted, accessEncrypted, apiKeyEncrypted, now(), now())
	if err != nil {
		t.Fatal(err)
	}

	result, err := app.accountValidation.TestLogin(context.Background(), accountID)
	if err != nil {
		t.Fatalf("TestLogin returned error: %v", err)
	}
	if result.Status != "valid" {
		t.Fatalf("status = %q, want valid", result.Status)
	}
	if result.HTTPStatus != http.StatusOK {
		t.Fatalf("httpStatus = %d, want 200", result.HTTPStatus)
	}

	account, err := app.loadAccountByID(context.Background(), accountID)
	if err != nil {
		t.Fatal(err)
	}
	if account.LoginStatus != "valid" {
		t.Fatalf("persisted login status = %q, want valid", account.LoginStatus)
	}
}

func TestAccountValidationServiceTestAPIKeyPersistsResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("authorization") != "Bearer sk-validation" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/v1/models":
			_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4o-mini"}]}`))
		case "/v1/chat/completions":
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	app := newTestApp(t)
	defer app.Close()
	app.client = server.Client()
	app.allowLocalOutbound = true

	siteID := newID()
	accountID := newID()
	apiKeyEncrypted, err := app.encryptText("sk-validation")
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, created_at, updated_at)
		VALUES (?, 'Validation Relay', ?, 'newapi', 'healthy', ?, ?)
	`, siteID, server.URL, now(), now())
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.db.Exec(`
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, api_key_encrypted, created_at, updated_at)
		VALUES (?, ?, 'Validation Account', 'api_key', ?, ?, ?)
	`, accountID, siteID, apiKeyEncrypted, now(), now())
	if err != nil {
		t.Fatal(err)
	}

	result := app.accountValidation.TestAPIKey(context.Background(), accountID, nil)
	if result.Status != "valid" {
		t.Fatalf("status = %q, want valid: %+v", result.Status, result)
	}
	if !result.ModelUsable {
		t.Fatalf("expected model usable: %+v", result)
	}

	account, err := app.loadAccountByID(context.Background(), accountID)
	if err != nil {
		t.Fatal(err)
	}
	if account.APIKeyStatus != "valid" {
		t.Fatalf("persisted API key status = %q, want valid", account.APIKeyStatus)
	}
	if !account.APIKeyModelUsable {
		t.Fatalf("expected persisted model usable flag")
	}
}
