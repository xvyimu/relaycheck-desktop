package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAccountSessionCleanupServiceClearsSessionFieldsAndProfile(t *testing.T) {
	app := newTestApp(t)

	siteID := newID()
	accountID := newID()
	profilePath := filepath.Join(app.dataDir, "browser-profiles", accountID)
	if err := os.MkdirAll(profilePath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profilePath, "profile.txt"), []byte("profile"), 0o600); err != nil {
		t.Fatal(err)
	}
	cookieEncrypted, err := app.encryptText("session=abc")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.ExecContext(context.Background(), `
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, created_at, updated_at)
		VALUES (?, 'Cleanup Relay', 'https://cleanup.example', 'newapi', 'healthy', ?, ?)
	`, siteID, now(), now()); err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.ExecContext(context.Background(), `
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, cookie_encrypted, browser_profile_path, user_agent, login_status, created_at, updated_at)
		VALUES (?, ?, 'Cleanup Account', 'browser_profile', ?, ?, 'UA', 'valid', ?, ?)
	`, accountID, siteID, cookieEncrypted, profilePath, now(), now()); err != nil {
		t.Fatal(err)
	}
	app.browserSessions.Set(accountID, BrowserLoginSession{AccountID: accountID, Port: 9222, StartedAt: time.Now(), PID: 123})

	if err := NewAccountSessionCleanupService(app).Clear(context.Background(), accountID); err != nil {
		t.Fatal(err)
	}
	if _, ok := app.browserSessions.Get(accountID); ok {
		t.Fatal("browser session still exists after Clear")
	}
	if _, err := os.Stat(profilePath); !os.IsNotExist(err) {
		t.Fatalf("profile path stat err = %v, want not exist", err)
	}

	var cookie, storedProfilePath, userAgent, loginStatus string
	if err := app.db.QueryRow(`
		SELECT cookie_encrypted, browser_profile_path, user_agent, login_status
		FROM channel_accounts WHERE id=?
	`, accountID).Scan(&cookie, &storedProfilePath, &userAgent, &loginStatus); err != nil {
		t.Fatal(err)
	}
	if cookie != "" || storedProfilePath != "" || userAgent != "" || loginStatus != "manual_required" {
		t.Fatalf("session fields = %q/%q/%q/%q, want cleared/manual_required", cookie, storedProfilePath, userAgent, loginStatus)
	}

	var action string
	if err := app.db.QueryRow(`SELECT action FROM audit_log WHERE resource_id=?`, accountID).Scan(&action); err != nil {
		t.Fatal(err)
	}
	if action != "browser_auth.disconnected" {
		t.Fatalf("audit action = %q, want browser_auth.disconnected", action)
	}
}

func TestAccountSessionCleanupServiceDoesNotRemoveProfileOutsideDataDir(t *testing.T) {
	app := newTestApp(t)

	siteID := newID()
	accountID := newID()
	outsideDir := t.TempDir()
	outsideProfile := filepath.Join(outsideDir, "external-profile")
	if err := os.MkdirAll(outsideProfile, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.ExecContext(context.Background(), `
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, created_at, updated_at)
		VALUES (?, 'Outside Relay', 'https://outside.example', 'newapi', 'healthy', ?, ?)
	`, siteID, now(), now()); err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.ExecContext(context.Background(), `
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, browser_profile_path, login_status, created_at, updated_at)
		VALUES (?, ?, 'Outside Account', 'browser_profile', ?, 'valid', ?, ?)
	`, accountID, siteID, outsideProfile, now(), now()); err != nil {
		t.Fatal(err)
	}

	if err := NewAccountSessionCleanupService(app).Clear(context.Background(), accountID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(outsideProfile); err != nil {
		t.Fatalf("outside profile stat err = %v, want still exists", err)
	}
}

func TestAccountSessionCleanupServiceDoesNotRemoveSiblingDataDirPrefix(t *testing.T) {
	app := newTestApp(t)

	siteID := newID()
	accountID := newID()
	siblingProfile := filepath.Clean(app.dataDir) + "-sibling"
	if err := os.MkdirAll(siblingProfile, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.ExecContext(context.Background(), `
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, created_at, updated_at)
		VALUES (?, 'Sibling Relay', 'https://sibling.example', 'newapi', 'healthy', ?, ?)
	`, siteID, now(), now()); err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.ExecContext(context.Background(), `
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, browser_profile_path, login_status, created_at, updated_at)
		VALUES (?, ?, 'Sibling Account', 'browser_profile', ?, 'valid', ?, ?)
	`, accountID, siteID, siblingProfile, now(), now()); err != nil {
		t.Fatal(err)
	}

	if err := NewAccountSessionCleanupService(app).Clear(context.Background(), accountID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(siblingProfile); err != nil {
		t.Fatalf("sibling profile stat err = %v, want still exists", err)
	}
}
