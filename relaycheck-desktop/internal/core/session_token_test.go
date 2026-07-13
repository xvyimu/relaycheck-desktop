package core

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewSessionTokenIsRandom64Hex(t *testing.T) {
	a := NewSessionToken()
	b := NewSessionToken()
	if len(a) != 64 || len(b) != 64 {
		t.Fatalf("expected 64-char hex tokens, got %d and %d", len(a), len(b))
	}
	if a == b {
		t.Fatalf("expected distinct tokens, got %q twice", a)
	}
}

func TestValidateSessionTokenDisabledByDefault(t *testing.T) {
	app := newTestApp(t)
	if app.tokenEnforced() {
		t.Fatal("token enforcement should be off by default")
	}
	// With no token configured, any request validates (fail-open only when
	// enforcement is explicitly disabled).
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:3001/api/example", nil)
	if !app.validateSessionToken(req) {
		t.Fatal("expected validation to pass when enforcement is disabled")
	}
}

func TestValidateSessionTokenEnforced(t *testing.T) {
	app := newTestApp(t)
	token := NewSessionToken()
	app.SetLocalToken(token)
	if !app.tokenEnforced() {
		t.Fatal("token enforcement should be on after SetLocalToken")
	}

	cases := []struct {
		name   string
		cookie string
		set    bool
		want   bool
	}{
		{name: "no cookie", set: false, want: false},
		{name: "empty cookie", cookie: "", set: true, want: false},
		{name: "wrong cookie", cookie: "deadbeef", set: true, want: false},
		{name: "correct cookie", cookie: token, set: true, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:3001/api/example", nil)
			if tc.set {
				req.AddCookie(&http.Cookie{Name: sessionTokenCookie, Value: tc.cookie})
			}
			if got := app.validateSessionToken(req); got != tc.want {
				t.Fatalf("validateSessionToken=%v, want %v", got, tc.want)
			}
		})
	}
}

func TestSetSessionCookieIfEnabled(t *testing.T) {
	app := newTestApp(t)

	// Disabled: no cookie written.
	rec := httptest.NewRecorder()
	app.SetSessionCookieIfEnabled(rec)
	if len(rec.Result().Cookies()) != 0 {
		t.Fatal("expected no cookie when enforcement is disabled")
	}

	// Enabled: HttpOnly, Strict cookie carrying the token.
	token := NewSessionToken()
	app.SetLocalToken(token)
	rec = httptest.NewRecorder()
	app.SetSessionCookieIfEnabled(rec)
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	c := cookies[0]
	if c.Name != sessionTokenCookie || c.Value != token {
		t.Fatalf("unexpected cookie %q=%q", c.Name, c.Value)
	}
	if !c.HttpOnly {
		t.Fatal("session cookie must be HttpOnly")
	}
	if c.SameSite != http.SameSiteStrictMode {
		t.Fatal("session cookie must be SameSite=Strict")
	}
}
