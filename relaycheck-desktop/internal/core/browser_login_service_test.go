package core

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestBrowserLoginServiceOpenAlreadyOpenIncludesResolvedEntryMetadata(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	accountID := "service-browser-login-account"
	app.browserSessions.Set(accountID, BrowserLoginSession{
		AccountID: accountID,
		Port:      9333,
		StartedAt: time.Now(),
		PID:       1,
	})

	service := NewBrowserLoginService(app)
	result := service.Open(context.Background(), accountID, &accountAuthContext{
		AccountID:              accountID,
		AccountName:            "Browser Account",
		UpstreamSite:           "Relay",
		BaseURL:                "https://relay.example/base",
		BrowserLoginURL:        "/panel/login",
		BrowserLoginSource:     "path_probe",
		BrowserLoginConfidence: 0.45,
		BrowserLoginReason:     "Low confidence login candidate; verify manually",
	})

	if result.Status != "already_open" {
		t.Fatalf("Status = %q, want already_open", result.Status)
	}
	if result.URL != "https://relay.example/panel/login" {
		t.Fatalf("URL = %q, want resolved login URL", result.URL)
	}
	if result.LoginURLSource != "path_probe" {
		t.Fatalf("LoginURLSource = %q, want path_probe", result.LoginURLSource)
	}
	if result.LoginURLConfidence != 0.45 {
		t.Fatalf("LoginURLConfidence = %.2f, want 0.45", result.LoginURLConfidence)
	}
	if !strings.Contains(result.LoginURLReason, "Low confidence") {
		t.Fatalf("LoginURLReason = %q, want low-confidence reason", result.LoginURLReason)
	}
}
