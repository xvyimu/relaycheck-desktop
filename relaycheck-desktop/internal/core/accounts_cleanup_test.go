package core

import (
	"context"
	"fmt"
	"testing"
)

func TestDeleteUnsupportedCheckinAccountsDeletesOnlyUnsupportedTargets(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	supportedSiteID := newID()
	unsupportedSiteID := newID()
	lastUnsupportedSiteID := newID()
	keepAccountID := newID()
	unsupportedSiteAccountID := newID()
	lastUnsupportedAccountID := newID()
	_, err = app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, supports_checkin, created_at, updated_at)
		VALUES
		  (?, 'Supported', 'https://supported.example', 'newapi', 'healthy', 1, ?, ?),
		  (?, 'No Checkin', 'https://nocheckin.example', 'oneapi', 'healthy', 0, ?, ?),
		  (?, 'Last Unsupported', 'https://lastunsupported.example', 'newapi', 'healthy', 1, ?, ?)
	`, supportedSiteID, now(), now(), unsupportedSiteID, now(), now(), lastUnsupportedSiteID, now(), now())
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.db.Exec(`
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, login_status, last_checkin_status, created_at, updated_at)
		VALUES
		  (?, ?, 'Keep Me', 'cookie', 'valid', 'success', ?, ?),
		  (?, ?, 'Delete Site Unsupported', 'cookie', 'valid', '', ?, ?),
		  (?, ?, 'Delete Last Unsupported', 'cookie', 'valid', 'unsupported', ?, ?)
	`, keepAccountID, supportedSiteID, now(), now(), unsupportedSiteAccountID, unsupportedSiteID, now(), now(), lastUnsupportedAccountID, lastUnsupportedSiteID, now(), now())
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.db.Exec(`
		INSERT INTO checkin_logs (id, account_id, upstream_site_id, status, started_at, finished_at)
		VALUES (?, ?, ?, 'unsupported', ?, ?)
	`, newID(), unsupportedSiteAccountID, unsupportedSiteID, now(), now())
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.db.Exec(`
		INSERT INTO balance_snapshots (id, account_id, upstream_site_id, unit, created_at)
		VALUES (?, ?, ?, 'quota', ?)
	`, newID(), unsupportedSiteAccountID, unsupportedSiteID, now())
	if err != nil {
		t.Fatal(err)
	}

	preview, err := app.deleteUnsupportedCheckinAccounts(context.Background(), 10, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Matched != 2 || preview.Deleted != 0 || !preview.DryRun {
		t.Fatalf("unexpected preview result: %#v", preview)
	}

	result, err := app.deleteUnsupportedCheckinAccounts(context.Background(), 10, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Matched != 2 || result.Deleted != 2 {
		t.Fatalf("expected two deleted accounts, got %#v", result)
	}

	var remaining int
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM channel_accounts WHERE id IN (?, ?)`, unsupportedSiteAccountID, lastUnsupportedAccountID).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Fatalf("expected unsupported accounts to be deleted, remaining=%d", remaining)
	}
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM channel_accounts WHERE id = ?`, keepAccountID).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 1 {
		t.Fatalf("expected supported account to remain, remaining=%d", remaining)
	}
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM checkin_logs WHERE account_id = ?`, unsupportedSiteAccountID).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Fatalf("expected check-in logs for deleted account to be removed, remaining=%d", remaining)
	}
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM balance_snapshots WHERE account_id = ?`, unsupportedSiteAccountID).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Fatalf("expected balance snapshots for deleted account to be removed, remaining=%d", remaining)
	}
}

func TestDeleteUnsupportedCheckinAccountsReportsBatchProgress(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	supportedSiteID := newID()
	unsupportedSiteID := newID()
	if _, err := app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, supports_checkin, created_at, updated_at)
		VALUES
		  (?, 'Supported', 'https://supported-batch.example', 'newapi', 'healthy', 1, ?, ?),
		  (?, 'No Checkin Batch', 'https://nocheckin-batch.example', 'oneapi', 'healthy', 0, ?, ?)
	`, supportedSiteID, now(), now(), unsupportedSiteID, now(), now()); err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.Exec(`
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, login_status, last_checkin_status, created_at, updated_at)
		VALUES (?, ?, 'Keep Batch', 'cookie', 'valid', 'success', ?, ?)
	`, newID(), supportedSiteID, now(), now()); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 12; i++ {
		if _, err := app.db.Exec(`
			INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, login_status, last_checkin_status, created_at, updated_at)
			VALUES (?, ?, ?, 'cookie', 'valid', '', ?, ?)
		`, newID(), unsupportedSiteID, fmt.Sprintf("Delete Batch %02d", i+1), now(), now()); err != nil {
			t.Fatal(err)
		}
	}

	preview, err := app.deleteUnsupportedCheckinAccounts(context.Background(), 10, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Matched != 10 || preview.Deleted != 0 || preview.Limit != 10 || !preview.HasMore || !preview.DryRun || !preview.IncludeLastUnsupported || len(preview.Items) != 10 {
		t.Fatalf("unexpected first preview result: %#v", preview)
	}
	var remaining int
	if err := app.db.QueryRow(`
		SELECT COUNT(*)
		FROM channel_accounts a
		JOIN upstream_sites s ON s.id = a.upstream_site_id
		WHERE s.supports_checkin = 0
	`).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 12 {
		t.Fatalf("dry-run should not delete accounts, remaining=%d", remaining)
	}

	result, err := app.deleteUnsupportedCheckinAccounts(context.Background(), 10, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Matched != 10 || result.Deleted != 10 || result.Limit != 10 || !result.HasMore || result.DryRun {
		t.Fatalf("unexpected first delete result: %#v", result)
	}

	nextPreview, err := app.deleteUnsupportedCheckinAccounts(context.Background(), 10, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if nextPreview.Matched != 2 || nextPreview.Deleted != 0 || nextPreview.Limit != 10 || nextPreview.HasMore || len(nextPreview.Items) != 2 {
		t.Fatalf("unexpected next preview result: %#v", nextPreview)
	}

	nextDelete, err := app.deleteUnsupportedCheckinAccounts(context.Background(), 10, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if nextDelete.Matched != 2 || nextDelete.Deleted != 2 || nextDelete.HasMore {
		t.Fatalf("unexpected next delete result: %#v", nextDelete)
	}
	if err := app.db.QueryRow(`
		SELECT COUNT(*)
		FROM channel_accounts a
		JOIN upstream_sites s ON s.id = a.upstream_site_id
		WHERE s.supports_checkin = 0
	`).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Fatalf("expected all unsupported accounts to be deleted, remaining=%d", remaining)
	}
}
