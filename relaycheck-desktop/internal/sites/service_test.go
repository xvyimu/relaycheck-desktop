package sites

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "modernc.org/sqlite"
)

func TestCreateUpstreamSitePersistsLoginDiscovery(t *testing.T) {
	svc, db := newPersistenceTestService(t, nil)
	discovery := &LoginDiscovery{
		URL:        "https://relay.example/console/login",
		Source:     "html_link",
		Confidence: 0.85,
		Candidates: []string{"https://relay.example/console/login"},
	}

	siteID, err := svc.CreateUpstreamSite(context.Background(), CreateSiteInput{
		Name:    "Relay",
		BaseURL: "https://relay.example",
	}, Detection{
		BaseURL:             "https://relay.example",
		HomepageURL:         "https://relay.example",
		LoginURL:            discovery.URL,
		LoginDiscovery:      discovery,
		Kind:                "newapi",
		HealthStatus:        "healthy",
		DetectionConfidence: 0.9,
	})
	if err != nil {
		t.Fatalf("CreateUpstreamSite() error = %v", err)
	}

	var loginURL, source, discoveryJSON, detectionJSON string
	var confidence float64
	err = db.QueryRowContext(context.Background(), `
		SELECT login_url, login_url_source, login_url_confidence, login_discovery_json, detection_json
		FROM upstream_sites WHERE id=?
	`, siteID).Scan(&loginURL, &source, &confidence, &discoveryJSON, &detectionJSON)
	if err != nil {
		t.Fatalf("query upstream site: %v", err)
	}
	if loginURL != discovery.URL {
		t.Fatalf("login_url = %q, want %q", loginURL, discovery.URL)
	}
	if source != "html_link" {
		t.Fatalf("login_url_source = %q, want html_link", source)
	}
	if confidence != 0.85 {
		t.Fatalf("login_url_confidence = %.2f, want 0.85", confidence)
	}
	assertStoredLoginDiscovery(t, discoveryJSON, discovery.URL, "html_link", 0.85)
	assertStoredDetectionLoginDiscovery(t, detectionJSON, discovery.URL, "html_link")
}

func TestCreateUpstreamSiteMarksManualLoginOverride(t *testing.T) {
	svc, db := newPersistenceTestService(t, nil)
	auto := &LoginDiscovery{
		URL:        "https://relay.example/console/login",
		Source:     "html_link",
		Confidence: 0.85,
		Candidates: []string{"https://relay.example/console/login"},
	}

	siteID, err := svc.CreateUpstreamSite(context.Background(), CreateSiteInput{
		Name:     "Relay",
		BaseURL:  "https://relay.example",
		LoginURL: "https://relay.example/custom-login",
	}, Detection{
		BaseURL:             "https://relay.example",
		HomepageURL:         "https://relay.example",
		LoginURL:            auto.URL,
		LoginDiscovery:      auto,
		Kind:                "newapi",
		HealthStatus:        "healthy",
		DetectionConfidence: 0.9,
	})
	if err != nil {
		t.Fatalf("CreateUpstreamSite() error = %v", err)
	}

	var loginURL, source, discoveryJSON, detectionJSON string
	var confidence float64
	err = db.QueryRowContext(context.Background(), `
		SELECT login_url, login_url_source, login_url_confidence, login_discovery_json, detection_json
		FROM upstream_sites WHERE id=?
	`, siteID).Scan(&loginURL, &source, &confidence, &discoveryJSON, &detectionJSON)
	if err != nil {
		t.Fatalf("query upstream site: %v", err)
	}
	if loginURL != "https://relay.example/custom-login" {
		t.Fatalf("login_url = %q, want manual override", loginURL)
	}
	if source != "manual" {
		t.Fatalf("login_url_source = %q, want manual", source)
	}
	if confidence != 1 {
		t.Fatalf("login_url_confidence = %.2f, want 1", confidence)
	}
	assertStoredLoginDiscovery(t, discoveryJSON, "https://relay.example/custom-login", "manual", 1)
	assertStoredDetectionLoginDiscovery(t, detectionJSON, auto.URL, "html_link")
}

func TestDetectAndSaveSitePreservesManualLoginURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(`<html><body><a href="/console/login">登录</a></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	svc, db := newPersistenceTestService(t, server.Client())
	seedUpstreamSite(t, db, "site-1", "Relay", server.URL, server.URL+"/manual-login", "manual", 1)

	result := svc.DetectAndSaveSite(context.Background(), "site-1", "Relay", server.URL)
	if result.Error != "" {
		t.Fatalf("DetectAndSaveSite() error = %s", result.Error)
	}

	var loginURL, source, discoveryJSON string
	var confidence float64
	err := db.QueryRowContext(context.Background(), `
		SELECT login_url, login_url_source, login_url_confidence, login_discovery_json
		FROM upstream_sites WHERE id='site-1'
	`).Scan(&loginURL, &source, &confidence, &discoveryJSON)
	if err != nil {
		t.Fatalf("query upstream site: %v", err)
	}
	if loginURL != server.URL+"/manual-login" {
		t.Fatalf("manual login_url was overwritten: %q", loginURL)
	}
	if source != "manual" || confidence != 1 {
		t.Fatalf("manual login metadata = %q %.2f, want manual 1", source, confidence)
	}
	assertStoredLoginDiscovery(t, discoveryJSON, server.URL+"/manual-login", "manual", 1)
}

func TestDetectAndSaveSiteRefreshesAutoLoginURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(`<html><body><a href="/console/login">登录</a></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	svc, db := newPersistenceTestService(t, server.Client())
	seedUpstreamSite(t, db, "site-1", "Relay", server.URL, server.URL+"/login", "fallback", 0.4)

	result := svc.DetectAndSaveSite(context.Background(), "site-1", "Relay", server.URL)
	if result.Error != "" {
		t.Fatalf("DetectAndSaveSite() error = %s", result.Error)
	}

	var loginURL, source, discoveryJSON string
	var confidence float64
	err := db.QueryRowContext(context.Background(), `
		SELECT login_url, login_url_source, login_url_confidence, login_discovery_json
		FROM upstream_sites WHERE id='site-1'
	`).Scan(&loginURL, &source, &confidence, &discoveryJSON)
	if err != nil {
		t.Fatalf("query upstream site: %v", err)
	}
	want := server.URL + "/console/login"
	if loginURL != want {
		t.Fatalf("login_url = %q, want %q", loginURL, want)
	}
	if source != "html_link" || confidence != 0.85 {
		t.Fatalf("login metadata = %q %.2f, want html_link 0.85", source, confidence)
	}
	assertStoredLoginDiscovery(t, discoveryJSON, want, "html_link", 0.85)
}

func TestEnsureUpstreamSiteForChannelWithoutDetectionPreservesExistingLoginURL(t *testing.T) {
	svc, db := newPersistenceTestService(t, nil)
	seedUpstreamSite(t, db, "site-1", "Relay", "https://relay.example", "https://relay.example/console/login", "html_link", 0.85)

	siteID, created, err := svc.EnsureUpstreamSiteForChannel(context.Background(), EnsureSiteInput{
		ChannelID:  "channel-1",
		Name:       "Relay",
		RawBaseURL: "https://relay.example",
		Kind:       "newapi",
		Detection:  nil,
	})
	if err != nil {
		t.Fatalf("EnsureUpstreamSiteForChannel() error = %v", err)
	}
	if created {
		t.Fatal("expected existing site to be updated")
	}
	if siteID != "site-1" {
		t.Fatalf("siteID = %q, want site-1", siteID)
	}

	var loginURL, source string
	var confidence float64
	err = db.QueryRowContext(context.Background(), `
		SELECT login_url, login_url_source, login_url_confidence
		FROM upstream_sites WHERE id='site-1'
	`).Scan(&loginURL, &source, &confidence)
	if err != nil {
		t.Fatalf("query upstream site: %v", err)
	}
	if loginURL != "https://relay.example/console/login" || source != "html_link" || confidence != 0.85 {
		t.Fatalf("login metadata = %q %q %.2f, want existing html_link", loginURL, source, confidence)
	}
}

func newPersistenceTestService(t *testing.T, client *http.Client) (*Service, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
		CREATE TABLE imported_channels (
			id TEXT PRIMARY KEY,
			source_channel_id TEXT NOT NULL,
			name TEXT NOT NULL,
			base_url TEXT,
			status TEXT,
			upstream_kind TEXT NOT NULL DEFAULT 'unknown',
			supports_checkin INTEGER NOT NULL DEFAULT 0,
			supports_balance INTEGER NOT NULL DEFAULT 0,
			supports_models INTEGER NOT NULL DEFAULT 0,
			supports_pricing INTEGER NOT NULL DEFAULT 0,
			raw_json TEXT NOT NULL,
			detection_json TEXT,
			last_detected_at TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
		CREATE TABLE upstream_sites (
			id TEXT PRIMARY KEY,
			channel_id TEXT,
			name TEXT NOT NULL,
			homepage_url TEXT,
			base_url TEXT NOT NULL,
			login_url TEXT,
			login_url_source TEXT,
			login_url_confidence REAL NOT NULL DEFAULT 0,
			login_discovery_json TEXT,
			kind TEXT NOT NULL DEFAULT 'unknown',
			detection_confidence REAL NOT NULL DEFAULT 0,
			health_status TEXT NOT NULL DEFAULT 'unknown',
			supports_checkin INTEGER NOT NULL DEFAULT 0,
			supports_balance INTEGER NOT NULL DEFAULT 0,
			supports_models INTEGER NOT NULL DEFAULT 0,
			supports_pricing INTEGER NOT NULL DEFAULT 0,
			detection_json TEXT,
			last_health_check_at TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
		CREATE TABLE channel_accounts (
			id TEXT PRIMARY KEY,
			upstream_site_id TEXT NOT NULL,
			display_name TEXT NOT NULL DEFAULT '',
			auth_type TEXT NOT NULL DEFAULT 'cookie',
			login_status TEXT NOT NULL DEFAULT 'unknown',
			created_at TEXT NOT NULL DEFAULT 'now',
			updated_at TEXT NOT NULL DEFAULT 'now'
		);
		CREATE TABLE checkin_logs (
			id TEXT PRIMARY KEY,
			account_id TEXT NOT NULL,
			upstream_site_id TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'success',
			started_at TEXT NOT NULL DEFAULT 'now',
			finished_at TEXT NOT NULL DEFAULT 'now'
		);
		CREATE TABLE balance_snapshots (
			id TEXT PRIMARY KEY,
			account_id TEXT NOT NULL,
			upstream_site_id TEXT NOT NULL,
			unit TEXT NOT NULL DEFAULT 'unknown',
			created_at TEXT NOT NULL DEFAULT 'now'
		);
		CREATE TABLE channel_schedules (
			id TEXT PRIMARY KEY,
			upstream_site_id TEXT,
			enabled INTEGER NOT NULL DEFAULT 1,
			checkin_time TEXT NOT NULL DEFAULT '08:00',
			cron_expr TEXT NOT NULL DEFAULT '',
			skip_dates_json TEXT NOT NULL DEFAULT '[]',
			random_delay_min INTEGER NOT NULL DEFAULT 0,
			random_delay_max INTEGER NOT NULL DEFAULT 30,
			created_at TEXT NOT NULL DEFAULT 'now',
			updated_at TEXT NOT NULL DEFAULT 'now'
		);
		CREATE TABLE site_pricing_cache (
			id TEXT PRIMARY KEY,
			site_id TEXT NOT NULL,
			site_name TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT 'now'
		);
	`)
	if err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return NewService(&testInfra{client: client, db: db}), db
}

func seedUpstreamSite(t *testing.T, db *sql.DB, id string, name string, baseURL string, loginURL string, source string, confidence float64) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), `
		INSERT INTO upstream_sites (
			id, name, homepage_url, base_url, login_url, login_url_source, login_url_confidence,
			kind, health_status, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, 'unknown', 'unknown', 'now', 'now')
	`, id, name, baseURL, baseURL, loginURL, source, confidence)
	if err != nil {
		t.Fatalf("seed upstream site: %v", err)
	}
}

func countTable(t *testing.T, db *sql.DB, query string, args ...interface{}) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("count query %q: %v", query, err)
	}
	return n
}

func TestDeleteUpstreamSiteCascadesRelatedRows(t *testing.T) {
	svc, db := newPersistenceTestService(t, nil)
	seedUpstreamSite(t, db, "site-1", "Relay", "https://relay.example", "", "", 0)
	seedUpstreamSite(t, db, "site-2", "Other", "https://other.example", "", "", 0)

	_, err := db.Exec(`
		INSERT INTO channel_accounts (id, upstream_site_id, display_name) VALUES
			('acc-1', 'site-1', 'A'),
			('acc-2', 'site-1', 'B'),
			('acc-keep', 'site-2', 'Keep');
		INSERT INTO checkin_logs (id, account_id, upstream_site_id) VALUES
			('log-1', 'acc-1', 'site-1'),
			('log-keep', 'acc-keep', 'site-2');
		INSERT INTO balance_snapshots (id, account_id, upstream_site_id) VALUES
			('bal-1', 'acc-1', 'site-1'),
			('bal-keep', 'acc-keep', 'site-2');
		INSERT INTO channel_schedules (id, upstream_site_id) VALUES
			('sch-1', 'site-1'),
			('sch-keep', 'site-2');
		INSERT INTO site_pricing_cache (id, site_id, site_name) VALUES
			('price-1', 'site-1', 'Relay'),
			('price-keep', 'site-2', 'Other');
	`)
	if err != nil {
		t.Fatalf("seed related rows: %v", err)
	}

	result, err := svc.DeleteUpstreamSite(context.Background(), "site-1")
	if err != nil {
		t.Fatalf("DeleteUpstreamSite() error = %v", err)
	}
	if !result.Deleted || result.Accounts != 2 || result.CheckinLogs != 1 || result.BalanceSnapshots != 1 || result.Schedules != 1 || result.PricingCache != 1 {
		t.Fatalf("unexpected cascade counts: %+v", result)
	}

	if countTable(t, db, `SELECT COUNT(1) FROM upstream_sites WHERE id='site-1'`) != 0 {
		t.Fatal("site-1 still present")
	}
	if countTable(t, db, `SELECT COUNT(1) FROM channel_accounts WHERE upstream_site_id='site-1'`) != 0 {
		t.Fatal("orphan accounts remain")
	}
	if countTable(t, db, `SELECT COUNT(1) FROM checkin_logs WHERE upstream_site_id='site-1'`) != 0 {
		t.Fatal("orphan checkin logs remain")
	}
	if countTable(t, db, `SELECT COUNT(1) FROM balance_snapshots WHERE upstream_site_id='site-1'`) != 0 {
		t.Fatal("orphan balance snapshots remain")
	}
	if countTable(t, db, `SELECT COUNT(1) FROM channel_schedules WHERE upstream_site_id='site-1'`) != 0 {
		t.Fatal("orphan schedules remain")
	}
	if countTable(t, db, `SELECT COUNT(1) FROM site_pricing_cache WHERE site_id='site-1'`) != 0 {
		t.Fatal("orphan pricing cache remain")
	}

	// 其他站点数据必须完整保留。
	if countTable(t, db, `SELECT COUNT(1) FROM upstream_sites WHERE id='site-2'`) != 1 {
		t.Fatal("site-2 was deleted")
	}
	if countTable(t, db, `SELECT COUNT(1) FROM channel_accounts WHERE id='acc-keep'`) != 1 {
		t.Fatal("kept account missing")
	}
	if countTable(t, db, `SELECT COUNT(1) FROM checkin_logs WHERE id='log-keep'`) != 1 {
		t.Fatal("kept log missing")
	}
}

func TestDeleteUpstreamSiteRejectsMissingAndGlobal(t *testing.T) {
	svc, _ := newPersistenceTestService(t, nil)
	if _, err := svc.DeleteUpstreamSite(context.Background(), "missing"); err != sql.ErrNoRows {
		t.Fatalf("missing site error = %v, want sql.ErrNoRows", err)
	}
	if _, err := svc.DeleteUpstreamSite(context.Background(), globalScheduleSiteID); err == nil {
		t.Fatal("expected error deleting global schedule site")
	}
	if _, err := svc.DeleteUpstreamSite(context.Background(), "  "); err == nil {
		t.Fatal("expected error for empty id")
	}
}

func assertStoredLoginDiscovery(t *testing.T, raw string, url string, source string, confidence float64) {
	t.Helper()
	var discovery LoginDiscovery
	if err := json.Unmarshal([]byte(raw), &discovery); err != nil {
		t.Fatalf("unmarshal login discovery %q: %v", raw, err)
	}
	if discovery.URL != url {
		t.Fatalf("stored discovery URL = %q, want %q", discovery.URL, url)
	}
	if discovery.Source != source {
		t.Fatalf("stored discovery source = %q, want %q", discovery.Source, source)
	}
	if discovery.Confidence != confidence {
		t.Fatalf("stored discovery confidence = %.2f, want %.2f", discovery.Confidence, confidence)
	}
}

func assertStoredDetectionLoginDiscovery(t *testing.T, raw string, url string, source string) {
	t.Helper()
	var detection Detection
	if err := json.Unmarshal([]byte(raw), &detection); err != nil {
		t.Fatalf("unmarshal detection %q: %v", raw, err)
	}
	if detection.LoginDiscovery == nil {
		t.Fatal("expected detection login discovery")
	}
	if detection.LoginDiscovery.URL != url || detection.LoginDiscovery.Source != source {
		t.Fatalf("detection login discovery = %#v, want %s %s", detection.LoginDiscovery, url, source)
	}
}
