package core

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestBrowserLoginServiceDoesNotReturnInternalLoadErrors(t *testing.T) {
	secretError := errors.New(`open C:\Users\secret-user\relaycheck.db: token=TOP_SECRET`)
	service := &BrowserLoginService{
		loadAuth: func(context.Context, string) (accountAuthContext, error) {
			return accountAuthContext{}, secretError
		},
	}

	openResult := service.Open(t.Context(), "account-1", nil)
	if openResult.Message != "加载账号授权失败。" {
		t.Fatalf("unexpected browser open public message: %q", openResult.Message)
	}
	saveResult := service.Save(t.Context(), "account-1", nil)
	if saveResult.Message != "加载账号授权失败。" {
		t.Fatalf("unexpected browser save public message: %q", saveResult.Message)
	}
	for _, message := range []string{openResult.Message, saveResult.Message} {
		if strings.Contains(message, "secret-user") || strings.Contains(message, "TOP_SECRET") {
			t.Fatalf("browser login result leaked an internal error: %q", message)
		}
	}
}

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
