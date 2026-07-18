package core

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestFreshAppHasSiteAndAccountForeignKeys(t *testing.T) {
	app := newTestApp(t)
	mustHaveFK(t, app, "channel_accounts", "upstream_site_id", "upstream_sites")
	mustHaveFK(t, app, "checkin_logs", "account_id", "channel_accounts")
	mustHaveFK(t, app, "checkin_logs", "upstream_site_id", "upstream_sites")
	mustHaveFK(t, app, "balance_snapshots", "account_id", "channel_accounts")
	mustHaveFK(t, app, "balance_snapshots", "upstream_site_id", "upstream_sites")
	mustHaveFK(t, app, "site_pricing_cache", "site_id", "upstream_sites")
	mustForeignKeyCheckEmpty(t, app)
}

func TestFKMigrateFromLegacyTablesWithoutFKs(t *testing.T) {
	root := t.TempDir()
	createLegacyNoFKDB(t, root)

	app := newTestAppWithDir(t, root)
	mustHaveFK(t, app, "channel_accounts", "upstream_site_id", "upstream_sites")
	mustHaveFK(t, app, "checkin_logs", "account_id", "channel_accounts")
	mustHaveFK(t, app, "site_pricing_cache", "site_id", "upstream_sites")
	mustForeignKeyCheckEmpty(t, app)

	// Orphan log without account must have been scrubbed.
	var n int
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM checkin_logs WHERE account_id='missing-acc'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("expected orphan checkin log scrubbed, still %d", n)
	}
	// Valid rows preserved.
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM channel_accounts`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("accounts=%d want 1", n)
	}
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM checkin_logs`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("checkin_logs=%d want 1 (orphan removed)", n)
	}
}

func TestDeleteAccountCascadesLogsAndBalances(t *testing.T) {
	app := newTestApp(t)
	seedSiteAccountGraph(t, app, "site-a", "acc-a")
	if _, err := app.db.Exec(`
		INSERT INTO checkin_logs (id, account_id, upstream_site_id, status, started_at, finished_at)
		VALUES ('log-a', 'acc-a', 'site-a', 'ok', '2026-01-01T00:00:00Z', '2026-01-01T00:00:01Z');
		INSERT INTO balance_snapshots (id, account_id, upstream_site_id, unit, created_at)
		VALUES ('bal-a', 'acc-a', 'site-a', 'USD', '2026-01-01T00:00:00Z');
	`); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/accounts/acc-a", nil)
	rec := httptest.NewRecorder()
	app.deleteAccount(rec, req, "acc-a")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var n int
	_ = app.db.QueryRow(`SELECT COUNT(*) FROM channel_accounts WHERE id='acc-a'`).Scan(&n)
	if n != 0 {
		t.Fatal("account still present")
	}
	_ = app.db.QueryRow(`SELECT COUNT(*) FROM checkin_logs WHERE account_id='acc-a'`).Scan(&n)
	if n != 0 {
		t.Fatal("orphan checkin logs remain")
	}
	_ = app.db.QueryRow(`SELECT COUNT(*) FROM balance_snapshots WHERE account_id='acc-a'`).Scan(&n)
	if n != 0 {
		t.Fatal("orphan balances remain")
	}
}

func TestSiteDeleteStillWorksWithFK(t *testing.T) {
	app := newTestApp(t)
	seedSiteAccountGraph(t, app, "site-b", "acc-b")
	if _, err := app.db.Exec(`
		INSERT INTO checkin_logs (id, account_id, upstream_site_id, status, started_at, finished_at)
		VALUES ('log-b', 'acc-b', 'site-b', 'ok', '2026-01-01T00:00:00Z', '2026-01-01T00:00:01Z');
		INSERT INTO site_pricing_cache (id, site_id, site_name, base_url, last_synced_at, created_at, updated_at)
		VALUES ('price-b', 'site-b', 'B', 'https://b.example', 't', 't', 't');
	`); err != nil {
		t.Fatal(err)
	}
	result, err := app.sitesService.DeleteUpstreamSite(context.Background(), "site-b")
	if err != nil {
		t.Fatalf("DeleteUpstreamSite: %v", err)
	}
	if !result.Deleted || result.Accounts != 1 || result.CheckinLogs != 1 || result.PricingCache != 1 {
		t.Fatalf("unexpected result %+v", result)
	}
	mustForeignKeyCheckEmpty(t, app)
}

func mustHaveFK(t *testing.T, app *App, table, fromCol, parent string) {
	t.Helper()
	ok, err := app.tableHasFK(context.Background(), table, fromCol, parent)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("expected FK %s.%s -> %s", table, fromCol, parent)
	}
}

func mustForeignKeyCheckEmpty(t *testing.T, app *App) {
	t.Helper()
	if err := app.assertForeignKeyCheck(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func seedSiteAccountGraph(t *testing.T, app *App, siteID, accountID string) {
	t.Helper()
	ts := now()
	if _, err := app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, created_at, updated_at)
		VALUES (?, 'Site', 'https://example.test', 'newapi', 'healthy', ?, ?)
	`, siteID, ts, ts); err != nil {
		t.Fatalf("insert site: %v", err)
	}
	if _, err := app.db.Exec(`
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, login_status, created_at, updated_at)
		VALUES (?, ?, 'Acc', 'password', 'unknown', ?, ?)
	`, accountID, siteID, ts, ts); err != nil {
		t.Fatalf("insert account: %v", err)
	}
}

// createLegacyNoFKDB builds a pre-FK schema with one valid graph row and one orphan log.
func createLegacyNoFKDB(t *testing.T, root string) {
	t.Helper()
	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(dataDir, "relaycheck.db")
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(dbPath)+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`
CREATE TABLE system_settings (
	id TEXT PRIMARY KEY, key TEXT NOT NULL UNIQUE, value_json TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE TABLE upstream_sites (
	id TEXT PRIMARY KEY, channel_id TEXT, name TEXT NOT NULL, homepage_url TEXT, base_url TEXT NOT NULL,
	login_url TEXT, kind TEXT NOT NULL DEFAULT 'unknown', detection_confidence REAL NOT NULL DEFAULT 0,
	health_status TEXT NOT NULL DEFAULT 'unknown', supports_checkin INTEGER NOT NULL DEFAULT 0,
	supports_balance INTEGER NOT NULL DEFAULT 0, supports_models INTEGER NOT NULL DEFAULT 0,
	supports_pricing INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE TABLE channel_accounts (
	id TEXT PRIMARY KEY, upstream_site_id TEXT NOT NULL, display_name TEXT NOT NULL, email TEXT, username TEXT,
	auth_type TEXT NOT NULL, login_status TEXT NOT NULL DEFAULT 'unknown', created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE TABLE checkin_logs (
	id TEXT PRIMARY KEY, account_id TEXT NOT NULL, upstream_site_id TEXT NOT NULL, channel_id TEXT,
	status TEXT NOT NULL, started_at TEXT NOT NULL, finished_at TEXT NOT NULL
);
CREATE TABLE balance_snapshots (
	id TEXT PRIMARY KEY, account_id TEXT NOT NULL, upstream_site_id TEXT NOT NULL, unit TEXT NOT NULL DEFAULT 'unknown', created_at TEXT NOT NULL
);
CREATE TABLE site_pricing_cache (
	id TEXT PRIMARY KEY, site_id TEXT NOT NULL, site_name TEXT NOT NULL, base_url TEXT NOT NULL,
	kind TEXT NOT NULL DEFAULT 'unknown', status TEXT NOT NULL DEFAULT 'unknown',
	http_status INTEGER NOT NULL DEFAULT 0, latency_ms INTEGER NOT NULL DEFAULT 0,
	source_path TEXT NOT NULL DEFAULT '/api/pricing', model_count INTEGER NOT NULL DEFAULT 0,
	source_count INTEGER NOT NULL DEFAULT 0, last_synced_at TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
	UNIQUE(site_id, source_path)
);
CREATE TABLE channel_schedules (
	id TEXT PRIMARY KEY, upstream_site_id TEXT, enabled INTEGER NOT NULL DEFAULT 1,
	checkin_time TEXT NOT NULL DEFAULT '08:00', cron_expr TEXT NOT NULL DEFAULT '',
	skip_dates_json TEXT NOT NULL DEFAULT '[]', random_delay_min INTEGER NOT NULL DEFAULT 0,
	random_delay_max INTEGER NOT NULL DEFAULT 30, last_run_at TEXT, next_run_at TEXT,
	created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
INSERT INTO upstream_sites (id, name, base_url, created_at, updated_at) VALUES ('site-1', 'S', 'https://s.example', 't', 't');
INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, created_at, updated_at)
VALUES ('acc-1', 'site-1', 'A', 'password', 't', 't');
INSERT INTO checkin_logs (id, account_id, upstream_site_id, status, started_at, finished_at)
VALUES ('log-ok', 'acc-1', 'site-1', 'ok', 't', 't');
INSERT INTO checkin_logs (id, account_id, upstream_site_id, status, started_at, finished_at)
VALUES ('log-orphan', 'missing-acc', 'site-1', 'ok', 't', 't');
INSERT INTO site_pricing_cache (id, site_id, site_name, base_url, last_synced_at, created_at, updated_at)
VALUES ('p1', 'site-1', 'S', 'https://s.example', 't', 't', 't');
`)
	if err != nil {
		t.Fatal(err)
	}
}
