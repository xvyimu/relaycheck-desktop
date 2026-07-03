package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListUpstreamSitesHidesGlobalScheduleRecord(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/upstream-sites", nil)
	w := httptest.NewRecorder()
	app.listUpstreamSites(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var items []UpstreamSite
	parseAPIResponse(t, w.Body.String(), &items)
	for _, item := range items {
		if item.ID == globalScheduleSiteID {
			t.Fatalf("global schedule record leaked into upstream sites: %#v", item)
		}
	}
}

func TestLoadSiteDetailHydratesLoginDiscoveryJSON(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	loginDiscoveryJSON := `{"url":"https://relay.example/console/login","source":"html_link","confidence":0.85,"candidates":["https://relay.example/console/login"]}`
	detectionJSON := `{"baseUrl":"https://relay.example","homepageUrl":"https://relay.example","loginUrl":"https://relay.example/console/login","loginDiscovery":{"url":"https://relay.example/console/login","source":"html_link","confidence":0.85},"kind":"newapi","healthStatus":"healthy","detectionConfidence":0.9}`
	if _, err := app.db.ExecContext(context.Background(), `
		INSERT INTO upstream_sites (
			id, name, homepage_url, base_url, login_url, login_url_source, login_url_confidence,
			login_discovery_json, kind, detection_confidence, health_status, detection_json, created_at, updated_at
		) VALUES ('site-detail-login', 'Relay', 'https://relay.example', 'https://relay.example',
			'https://relay.example/console/login', 'html_link', 0.85, ?, 'newapi', 0.9, 'healthy', ?, 'now', 'now')
	`, loginDiscoveryJSON, detectionJSON); err != nil {
		t.Fatalf("seed site: %v", err)
	}

	detail, err := app.loadSiteDetail(context.Background(), "site-detail-login")
	if err != nil {
		t.Fatalf("loadSiteDetail() error = %v", err)
	}
	if detail.Site.LoginDiscovery == nil {
		t.Fatal("expected site login discovery")
	}
	if detail.Site.LoginDiscovery.Source != "html_link" {
		t.Fatalf("site login discovery source = %q, want html_link", detail.Site.LoginDiscovery.Source)
	}
	if detail.Site.LoginURLSource != "html_link" || detail.Site.LoginURLConfidence != 0.85 {
		t.Fatalf("site login metadata = %q %.2f, want html_link 0.85", detail.Site.LoginURLSource, detail.Site.LoginURLConfidence)
	}
}

func TestLoadSiteDetailCacheMissPreservesManualLoginURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(`<html><body><a href="/console/login">Login</a></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	app := newTestApp(t)
	defer app.Close()
	app.allowLocalOutbound = true

	manualURL := server.URL + "/manual-login"
	if _, err := app.db.ExecContext(context.Background(), `
		INSERT INTO upstream_sites (
			id, name, homepage_url, base_url, login_url, login_url_source, login_url_confidence,
			kind, detection_confidence, health_status, created_at, updated_at
		) VALUES ('site-manual-login', 'Relay', ?, ?, ?, 'manual', 1, 'unknown', 0, 'unknown', 'now', 'now')
	`, server.URL, server.URL, manualURL); err != nil {
		t.Fatalf("seed site: %v", err)
	}

	detail, err := app.loadSiteDetail(context.Background(), "site-manual-login")
	if err != nil {
		t.Fatalf("loadSiteDetail() error = %v", err)
	}
	if detail.Site.LoginURL != manualURL {
		t.Fatalf("detail login URL = %q, want manual %q", detail.Site.LoginURL, manualURL)
	}
	if detail.Site.LoginDiscovery == nil || detail.Site.LoginDiscovery.Source != "manual" {
		t.Fatalf("detail login discovery = %#v, want manual", detail.Site.LoginDiscovery)
	}

	var loginURL, source, detectionJSON string
	if err := app.db.QueryRowContext(context.Background(), `
		SELECT login_url, login_url_source, COALESCE(detection_json,'')
		FROM upstream_sites WHERE id='site-manual-login'
	`).Scan(&loginURL, &source, &detectionJSON); err != nil {
		t.Fatalf("query site: %v", err)
	}
	if loginURL != manualURL || source != "manual" {
		t.Fatalf("stored login = %q %q, want manual URL/source", loginURL, source)
	}
	detection, ok := parseDetectionJSON(detectionJSON)
	if !ok {
		t.Fatalf("expected detection_json to be populated, got %q", detectionJSON)
	}
	if detection.LoginDiscovery == nil || detection.LoginDiscovery.URL != server.URL+"/console/login" {
		t.Fatalf("detection snapshot login discovery = %#v, want auto console login", detection.LoginDiscovery)
	}
}

func TestEnsureManualAccountSiteStoresManualLoginMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(`<html><body><a href="/console/login">Login</a></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	app := newTestApp(t)
	defer app.Close()
	app.allowLocalOutbound = true

	manualURL := server.URL + "/custom-login"
	siteID, err := app.ensureManualAccountSite(context.Background(), "Relay", server.URL, manualURL, "newapi")
	if err != nil {
		t.Fatalf("ensureManualAccountSite() error = %v", err)
	}

	var loginURL, source, discoveryJSON, detectionJSON string
	var confidence float64
	if err := app.db.QueryRowContext(context.Background(), `
		SELECT login_url, login_url_source, login_url_confidence, COALESCE(login_discovery_json,''), COALESCE(detection_json,'')
		FROM upstream_sites WHERE id=?
	`, siteID).Scan(&loginURL, &source, &confidence, &discoveryJSON, &detectionJSON); err != nil {
		t.Fatalf("query site: %v", err)
	}
	if loginURL != manualURL || source != "manual" || confidence != 1 {
		t.Fatalf("stored login = %q %q %.2f, want manual", loginURL, source, confidence)
	}
	manualDiscovery := parseLoginDiscoveryJSON(discoveryJSON)
	if manualDiscovery == nil || manualDiscovery.URL != manualURL || manualDiscovery.Source != "manual" {
		t.Fatalf("manual discovery = %#v, want manual URL", manualDiscovery)
	}
	detection, ok := parseDetectionJSON(detectionJSON)
	if !ok {
		t.Fatalf("expected detection_json, got %q", detectionJSON)
	}
	if detection.LoginDiscovery == nil || detection.LoginDiscovery.URL != server.URL+"/console/login" {
		t.Fatalf("detection login discovery = %#v, want auto console login", detection.LoginDiscovery)
	}
}

func TestUpdateAccountSiteMetadataMarksLoginURLManual(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	if _, err := app.db.ExecContext(context.Background(), `
		INSERT INTO upstream_sites (id, name, homepage_url, base_url, login_url, kind, health_status, created_at, updated_at)
		VALUES ('site-edit-login', 'Relay', 'https://relay.example', 'https://relay.example', 'https://relay.example/login', 'newapi', 'healthy', 'now', 'now')
	`); err != nil {
		t.Fatalf("seed site: %v", err)
	}

	manualURL := "https://relay.example/manual-login"
	if err := app.updateAccountSiteMetadata(context.Background(), "site-edit-login", "", manualURL, ""); err != nil {
		t.Fatalf("updateAccountSiteMetadata() error = %v", err)
	}

	var loginURL, source, discoveryJSON string
	var confidence float64
	if err := app.db.QueryRowContext(context.Background(), `
		SELECT login_url, login_url_source, login_url_confidence, COALESCE(login_discovery_json,'')
		FROM upstream_sites WHERE id='site-edit-login'
	`).Scan(&loginURL, &source, &confidence, &discoveryJSON); err != nil {
		t.Fatalf("query site: %v", err)
	}
	if loginURL != manualURL || source != "manual" || confidence != 1 {
		t.Fatalf("stored login = %q %q %.2f, want manual", loginURL, source, confidence)
	}
	if discovery := parseLoginDiscoveryJSON(discoveryJSON); discovery == nil || discovery.Source != "manual" || discovery.URL != manualURL {
		t.Fatalf("manual discovery = %#v, want manual URL", discovery)
	}
}
