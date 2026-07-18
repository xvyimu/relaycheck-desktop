package core

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)


func seedAccountSiteForLogs(t *testing.T, app *App, siteID, accountID string) {
	t.Helper()
	ts := now()
	if _, err := app.db.Exec(`
		INSERT OR IGNORE INTO upstream_sites (id, name, base_url, kind, health_status, created_at, updated_at)
		VALUES (?, 'Site', 'https://example.test', 'newapi', 'healthy', ?, ?)
	`, siteID, ts, ts); err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.Exec(`
		INSERT OR IGNORE INTO channel_accounts (id, upstream_site_id, display_name, auth_type, login_status, created_at, updated_at)
		VALUES (?, ?, 'Acc', 'password', 'unknown', ?, ?)
	`, accountID, siteID, ts, ts); err != nil {
		t.Fatal(err)
	}
}

func TestActionCenterCheckinProblemCountUsesCSTBounds(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()
	seedAccountSiteForLogs(t, app, "site", "account")

	currentTime := time.Date(2026, time.July, 18, 0, 1, 0, 0, cstZone())
	for _, fixture := range []struct {
		id        string
		status    string
		startedAt string
	}{
		{id: "current-cst-failure", status: "failed", startedAt: "2026-07-17T16:00:00Z"},
		{id: "previous-cst-failure", status: "failed", startedAt: "2026-07-17T15:59:59Z"},
	} {
		if _, err := app.db.Exec(`
			INSERT INTO checkin_logs (id, account_id, upstream_site_id, status, started_at, finished_at)
			VALUES (?, 'account', 'site', ?, ?, ?)
		`, fixture.id, fixture.status, fixture.startedAt, fixture.startedAt); err != nil {
			t.Fatal(err)
		}
	}

	var query *actionQuery
	queries := actionCenterQueriesAt(currentTime)
	for index := range queries {
		candidate := queries[index]
		if candidate.id == "today-checkin-problems" {
			query = &candidate
			break
		}
	}
	if query == nil {
		t.Fatal("today check-in action query not found")
	}
	count, err := app.queryActionCount(httptest.NewRequest(http.MethodGet, "/api/system/action-center", nil), query.countSQL, query.args)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("today check-in problem count = %d, want 1 current CST-day failure", count)
	}
}

func TestDiagnosticsFailedCheckinsCountUsesCSTBounds(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()
	seedAccountSiteForLogs(t, app, "site", "account")

	currentTime := time.Date(2026, time.July, 18, 0, 1, 0, 0, cstZone())
	for _, fixture := range []struct {
		id        string
		status    string
		startedAt string
	}{
		{id: "current-cst-failure", status: "failed", startedAt: "2026-07-17T16:00:00Z"},
		{id: "previous-cst-failure", status: "failed", startedAt: "2026-07-17T15:59:59Z"},
	} {
		if _, err := app.db.Exec(`
			INSERT INTO checkin_logs (id, account_id, upstream_site_id, status, started_at, finished_at)
			VALUES (?, 'account', 'site', ?, ?, ?)
		`, fixture.id, fixture.status, fixture.startedAt, fixture.startedAt); err != nil {
			t.Fatal(err)
		}
	}

	diagnostics, err := app.systemDiagnosticsAt(httptest.NewRequest(http.MethodGet, "/api/system/diagnostics", nil), currentTime)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range diagnostics.Items {
		if item.ID != "failed-checkins-today" {
			continue
		}
		if item.Count != 1 {
			t.Fatalf("failed-checkins-today count = %d, want 1 current CST-day failure", item.Count)
		}
		return
	}
	t.Fatal("failed-checkins-today diagnostic not found")
}
