package core

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestAccountLoginBatchServicePasswordLoginSelectsDuePasswordAccounts(t *testing.T) {
	app := newTestApp(t)

	siteID := newID()
	if _, err := app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, supports_checkin, supports_balance, created_at, updated_at)
		VALUES (?, 'Batch Login Site', 'https://batch.example', 'newapi', 'healthy', 1, 1, ?, ?)
	`, siteID, now(), now()); err != nil {
		t.Fatal(err)
	}

	password, err := app.encryptText("secret")
	if err != nil {
		t.Fatal(err)
	}
	dueExpiredID := newID()
	dueCheckinID := newID()
	notDueID := newID()
	noPasswordID := newID()
	if _, err := app.db.Exec(`
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, email, auth_type, password_encrypted, login_status, last_checkin_status, created_at, updated_at)
		VALUES
		  (?, ?, 'Due Expired', 'due-expired@example.com', 'email_password', ?, 'expired', '', ?, ?),
		  (?, ?, 'Due Checkin', 'due-checkin@example.com', 'email_password', ?, 'valid', 'failed', ?, ?),
		  (?, ?, 'Not Due', 'not-due@example.com', 'email_password', ?, 'valid', '', ?, ?),
		  (?, ?, 'No Password', 'no-password@example.com', 'email_password', '', 'expired', '', ?, ?)
	`, dueExpiredID, siteID, password, now(), now(),
		dueCheckinID, siteID, password, now(), now(),
		notDueID, siteID, password, now(), now(),
		noPasswordID, siteID, now(), now()); err != nil {
		t.Fatal(err)
	}

	service := NewAccountLoginBatchService(app)
	seen := map[string]bool{}
	service.passwordLogin = func(ctx context.Context, auth *accountAuthContext) error {
		seen[auth.AccountID] = true
		return nil
	}

	result, err := service.PasswordLogin(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if result.Processed != 2 || result.Success != 2 || result.Failed != 0 {
		t.Fatalf("PasswordLogin counts = processed %d success %d failed %d, want 2/2/0", result.Processed, result.Success, result.Failed)
	}
	if !seen[dueExpiredID] || !seen[dueCheckinID] || seen[notDueID] || seen[noPasswordID] {
		t.Fatalf("selected accounts = %#v, want only due password accounts", seen)
	}
}

func TestAccountLoginBatchServiceRetryPasswordMarksMissingCredentialsManualRequired(t *testing.T) {
	app := newTestApp(t)

	siteID := newID()
	accountID := newID()
	if _, err := app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, created_at, updated_at)
		VALUES (?, 'Manual Login Site', 'https://manual.example', 'newapi', 'healthy', ?, ?)
	`, siteID, now(), now()); err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.Exec(`
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, email, auth_type, login_status, created_at, updated_at)
		VALUES (?, ?, 'Manual Account', 'manual@example.com', 'email_password', 'unknown', ?, ?)
	`, accountID, siteID, now(), now()); err != nil {
		t.Fatal(err)
	}

	result := NewAccountLoginBatchService(app).RetryPasswordLogin(context.Background(), accountID, nil)
	if result.Status != "manual_required" {
		t.Fatalf("RetryPasswordLogin status = %q, want manual_required", result.Status)
	}

	var status string
	if err := app.db.QueryRow(`SELECT login_status FROM channel_accounts WHERE id=?`, accountID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "manual_required" {
		t.Fatalf("persisted login_status = %q, want manual_required", status)
	}
}

func TestAccountLoginBatchServiceRetryPasswordMarksLoginFailureExpired(t *testing.T) {
	app := newTestApp(t)

	service := NewAccountLoginBatchService(app)
	service.passwordLogin = func(ctx context.Context, auth *accountAuthContext) error {
		return errors.New("login failed")
	}
	auth := &accountAuthContext{
		AccountID:    "account-1",
		AccountName:  "Account 1",
		UpstreamSite: "Site 1",
		LoginName:    "user@example.com",
		Password:     "secret",
	}

	result := service.RetryPasswordLogin(context.Background(), auth.AccountID, auth)
	if result.Status != "expired" || result.Message != "login failed" {
		t.Fatalf("RetryPasswordLogin result = %#v, want expired login failed", result)
	}
}

func TestAccountLoginBatchServiceOpenBrowserHonorsExplicitLimit(t *testing.T) {
	app := newTestApp(t)
	service := NewAccountLoginBatchService(app)

	openedIDs := []string{}
	service.openBrowser = func(ctx context.Context, id string, auth *accountAuthContext) browserLoginOpenResult {
		openedIDs = append(openedIDs, id)
		return browserLoginOpenResult{AccountID: id, Status: "opened"}
	}

	result, err := service.OpenBrowser(context.Background(), 2, []string{"a", "b", "c"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Processed != 2 || result.Opened != 2 || result.Failed != 0 {
		t.Fatalf("OpenBrowser counts = processed %d opened %d failed %d, want 2/2/0", result.Processed, result.Opened, result.Failed)
	}
	if len(openedIDs) != 2 || openedIDs[0] != "a" || openedIDs[1] != "b" {
		t.Fatalf("opened IDs = %#v, want [a b]", openedIDs)
	}
}

func TestAccountLoginBatchServiceFinishBrowserDefaultsToActiveSessions(t *testing.T) {
	app := newTestApp(t)
	app.browserSessions.Set("a", BrowserLoginSession{AccountID: "a", Port: 41001, StartedAt: time.Now()})
	app.browserSessions.Set("b", BrowserLoginSession{AccountID: "b", Port: 41002, StartedAt: time.Now()})

	service := NewAccountLoginBatchService(app)
	savedIDs := map[string]bool{}
	service.saveBrowser = func(ctx context.Context, id string, auth *accountAuthContext) browserLoginSaveResult {
		savedIDs[id] = true
		return browserLoginSaveResult{AccountID: id, Status: "saved"}
	}

	result, err := service.FinishBrowser(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Processed != 2 || result.Saved != 2 || result.Failed != 0 {
		t.Fatalf("FinishBrowser counts = processed %d saved %d failed %d, want 2/2/0", result.Processed, result.Saved, result.Failed)
	}
	if !savedIDs["a"] || !savedIDs["b"] {
		t.Fatalf("saved IDs = %#v, want a and b", savedIDs)
	}
}
