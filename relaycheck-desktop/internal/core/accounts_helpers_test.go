package core

import (
	"strings"
	"testing"
	"time"
)

func TestInferAccountAuthType(t *testing.T) {
	cases := []struct {
		name         string
		password     string
		cookie       string
		accessToken  string
		refreshToken string
		apiKey       string
		want         string
	}{
		{"api-key-wins", "password", "cookie", "access", "refresh", "sk-test", "api_key"},
		{"cookie", "password", " session=abc ", "access", "refresh", "", "cookie"},
		{"access-token", "password", "", " access ", "refresh", "", "access_token"},
		{"refresh-token", "password", "", "", " refresh ", "", "refresh_token"},
		{"password", " password ", "", "", "", "", "email_password"},
		{"browser-profile", "", "", "", "", "", "browser_profile"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := inferAccountAuthType(tc.password, tc.cookie, tc.accessToken, tc.refreshToken, tc.apiKey)
			if got != tc.want {
				t.Fatalf("inferAccountAuthType() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDefaultAccountDisplayName(t *testing.T) {
	if got := defaultAccountDisplayName(" user@example.com ", "username", "sk-test"); got != "user@example.com" {
		t.Fatalf("email display name = %q", got)
	}
	if got := defaultAccountDisplayName("", " username ", "sk-test"); got != "username" {
		t.Fatalf("username display name = %q", got)
	}

	got := defaultAccountDisplayName("", "", " sk-test ")
	want := "API Key " + secretFingerprint("sk-test")
	if got != want {
		t.Fatalf("api key display name = %q, want %q", got, want)
	}

	if got := defaultAccountDisplayName("", "", ""); strings.TrimSpace(got) == "" {
		t.Fatal("fallback display name should not be empty")
	}
}

func TestEstimateCookieExpiry(t *testing.T) {
	before := time.Now().UTC().Add(30 * 24 * time.Hour).Truncate(time.Second).Add(-time.Second)
	got := estimateCookieExpiry()
	after := time.Now().UTC().Add(30 * 24 * time.Hour).Truncate(time.Second).Add(time.Second)

	expiry, err := time.Parse(time.RFC3339, got)
	if err != nil {
		t.Fatalf("estimateCookieExpiry() produced invalid RFC3339 %q: %v", got, err)
	}
	if expiry.Before(before) || expiry.After(after) {
		t.Fatalf("estimateCookieExpiry() = %s, want between %s and %s", expiry, before, after)
	}
}

func TestBuildCookieHeader(t *testing.T) {
	cookies := []cdpCookie{
		{Name: "session", Value: "abc"},
		{Name: "", Value: "skip-me"},
		{Name: "theme", Value: "dark"},
	}
	if got := buildCookieHeader(cookies); got != "session=abc; theme=dark" {
		t.Fatalf("buildCookieHeader() = %q", got)
	}
	if got := buildCookieHeader(nil); got != "" {
		t.Fatalf("buildCookieHeader(nil) = %q, want empty", got)
	}
}

func TestFreeDebugPortSkipsUsedPorts(t *testing.T) {
	port, err := freeDebugPort(map[int]bool{9222: true})
	if err != nil {
		t.Fatalf("freeDebugPort() error = %v", err)
	}
	if port == 0 || port == 9222 {
		t.Fatalf("freeDebugPort() = %d, want available port other than 9222", port)
	}
}

func TestStatusFromKey(t *testing.T) {
	if got := statusFromKey(""); got != "" {
		t.Fatalf("statusFromKey(empty) = %q", got)
	}
	if got := statusFromKey("key_abc"); got != "unchecked" {
		t.Fatalf("statusFromKey(non-empty) = %q", got)
	}
}
