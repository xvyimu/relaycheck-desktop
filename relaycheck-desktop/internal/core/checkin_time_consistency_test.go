package core

import (
	"context"
	"testing"
	"time"
)

func TestCSTDayBoundsUseUTCWindowForCSTCalendarDay(t *testing.T) {
	currentTime := time.Date(2026, time.July, 18, 0, 1, 0, 0, cstZone())
	start, end := cstDayBounds(currentTime)
	if start != "2026-07-17T16:00:00Z" || end != "2026-07-18T16:00:00Z" {
		t.Fatalf("cstDayBounds() = (%q, %q), want UTC bounds for the CST day", start, end)
	}
}

func TestLoadDueAccountsUsesCSTDayBounds(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	currentTime := time.Date(2026, time.July, 18, 0, 1, 0, 0, cstZone())
	siteID := newID()
	if _, err := app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, supports_checkin, created_at, updated_at)
		VALUES (?, 'CST Boundary', 'https://boundary.example', 'newapi', 'healthy', 1, ?, ?)
	`, siteID, currentTime.UTC().Format(time.RFC3339), currentTime.UTC().Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}

	fixtures := []struct {
		id     string
		status string
		lastAt string
	}{
		{id: "checked-in-current-cst-day", status: "success", lastAt: "2026-07-17T16:00:00Z"},
		{id: "checked-in-previous-cst-day", status: "success", lastAt: "2026-07-17T15:59:59Z"},
		{id: "failed-current-cst-day", status: "failed", lastAt: "2026-07-17T16:00:00Z"},
	}
	for _, fixture := range fixtures {
		if _, err := app.db.Exec(`
			INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, login_status, last_checkin_status, last_checkin_at, created_at, updated_at)
			VALUES (?, ?, ?, 'cookie', 'valid', ?, ?, ?, ?)
		`, fixture.id, siteID, fixture.id, fixture.status, fixture.lastAt, fixture.lastAt, fixture.lastAt); err != nil {
			t.Fatal(err)
		}
	}

	due, err := app.checkinBatch.loadDueAccountsAt(context.Background(), "", 0, currentTime)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, account := range due {
		got[account.ID] = true
	}
	if got["checked-in-current-cst-day"] {
		t.Fatalf("account checked in at the CST day start must not be due: %#v", due)
	}
	if !got["checked-in-previous-cst-day"] || !got["failed-current-cst-day"] || len(got) != 2 {
		t.Fatalf("due accounts = %#v, want previous-day success and current-day failure", due)
	}
}

func TestCheckinTodaySummaryUsesCSTDayBounds(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	currentTime := time.Date(2026, time.July, 18, 0, 1, 0, 0, cstZone())
	siteID := newID()
	accountID := newID()
	if _, err := app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, created_at, updated_at)
		VALUES (?, 'CST Summary', 'https://summary.example', 'newapi', 'healthy', ?, ?)
	`, siteID, currentTime.UTC().Format(time.RFC3339), currentTime.UTC().Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.Exec(`
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, login_status, created_at, updated_at)
		VALUES (?, ?, 'Summary Account', 'cookie', 'valid', ?, ?)
	`, accountID, siteID, currentTime.UTC().Format(time.RFC3339), currentTime.UTC().Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []struct {
		id        string
		status    string
		startedAt string
	}{
		{id: "current-cst-log", status: "failed", startedAt: "2026-07-17T16:00:00Z"},
		{id: "previous-cst-log", status: "success", startedAt: "2026-07-17T15:59:59Z"},
	} {
		if _, err := app.db.Exec(`
			INSERT INTO checkin_logs (id, account_id, upstream_site_id, status, started_at, finished_at)
			VALUES (?, ?, ?, ?, ?, ?)
		`, fixture.id, accountID, siteID, fixture.status, fixture.startedAt, fixture.startedAt); err != nil {
			t.Fatal(err)
		}
	}

	summary, err := app.checkinTodaySummaryAt(context.Background(), currentTime)
	if err != nil {
		t.Fatal(err)
	}
	if summary.TotalLogs != 1 || summary.FailedCount != 1 || summary.SuccessCount != 0 {
		t.Fatalf("today summary = %#v, want only current CST-day log", summary)
	}
}

func TestSaveResultRollsBackLogWhenAccountDoesNotExist(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	err := app.checkinExecutor.SaveResult(context.Background(), accountAuthContext{
		AccountID:      "missing-account",
		UpstreamSiteID: "missing-site",
		AccountName:    "Missing Account",
	}, checkinResult{Status: "success", Message: "should not persist"}, "2026-07-17T16:00:00Z", "2026-07-17T16:00:01Z")
	if err == nil {
		t.Fatal("SaveResult must fail when the account update affects no rows")
	}
	var logs int
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM checkin_logs WHERE account_id='missing-account'`).Scan(&logs); err != nil {
		t.Fatal(err)
	}
	if logs != 0 {
		t.Fatalf("failed account update must leave no orphaned log, logs=%d", logs)
	}
}
