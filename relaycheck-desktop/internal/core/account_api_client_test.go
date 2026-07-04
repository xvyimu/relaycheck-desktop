package core

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAccountAPIClientDoAddsAccountAuthHeaders(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("path = %q, want /v1/models", r.URL.Path)
		}
		if got := r.Header.Get("user-agent"); got != "RelayCheck-Test/1.0" {
			t.Errorf("user-agent = %q, want RelayCheck-Test/1.0", got)
		}
		if got := r.Header.Get("accept"); got != "application/json, text/plain, */*" {
			t.Errorf("accept = %q, want JSON/text accept", got)
		}
		if got := r.Header.Get("cookie"); got != "session=abc" {
			t.Errorf("cookie = %q, want session=abc", got)
		}
		if got := r.Header.Get("New-Api-User"); got != "42" {
			t.Errorf("New-Api-User = %q, want 42", got)
		}
		if got := r.Header.Get("authorization"); got != "Bearer access-token" {
			t.Errorf("authorization = %q, want bearer access token", got)
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := NewAccountAPIClient(app)
	status, body, err := client.Do(context.Background(), accountAuthContext{
		BaseURL:     server.URL,
		SiteKind:    "newapi",
		UserAgent:   "RelayCheck-Test/1.0",
		Cookie:      "session=abc",
		AuthUserID:  "42",
		AccessToken: "access-token",
		APIKey:      "api-key",
	}, http.MethodGet, "/v1/models", nil)
	if err != nil {
		t.Fatalf("Do returned error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if body != `{"ok":true}` {
		t.Fatalf("body = %q, want ok JSON", body)
	}
}

func TestAccountAPIClientDoWithTimeoutUsesAPIKeyAndBody(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()
	app.allowLocalOutbound = true

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("content-type"); got != "application/json" {
			t.Errorf("content-type = %q, want application/json", got)
		}
		if got := r.Header.Get("authorization"); got != "Bearer api-key" {
			t.Errorf("authorization = %q, want bearer API key", got)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"model":"test"}` {
			t.Errorf("body = %q, want model payload", string(body))
		}
		_, _ = w.Write([]byte(`{"choices":[{}]}`))
	}))
	defer server.Close()

	client := NewAccountAPIClient(app)
	status, body, err := client.DoWithTimeout(context.Background(), accountAuthContext{
		BaseURL: server.URL,
		APIKey:  "api-key",
	}, http.MethodPost, "/v1/chat/completions", []byte(`{"model":"test"}`), time.Second)
	if err != nil {
		t.Fatalf("DoWithTimeout returned error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if body != `{"choices":[{}]}` {
		t.Fatalf("body = %q, want choices JSON", body)
	}
}

func TestAccountAPIClientDoLimitsResponseBody(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("a", 300*1024)))
	}))
	defer server.Close()

	client := NewAccountAPIClient(app)
	_, body, err := client.Do(context.Background(), accountAuthContext{
		BaseURL: server.URL,
	}, http.MethodGet, "/large", nil)
	if err != nil {
		t.Fatalf("Do returned error: %v", err)
	}
	if len(body) != 256*1024 {
		t.Fatalf("body length = %d, want 256 KiB limit", len(body))
	}
}
