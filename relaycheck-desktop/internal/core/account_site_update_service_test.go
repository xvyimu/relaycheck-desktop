package core

import (
	"context"
	"testing"
)

func TestAccountSiteUpdateServiceSharedScopeMovesAccountsToExistingSite(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	oldSiteID := newID()
	existingSiteID := newID()
	firstAccountID := newID()
	secondAccountID := newID()
	targetBaseURL := "https://target.example"
	manualLoginURL := targetBaseURL + "/login"

	if _, err := app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, created_at, updated_at)
		VALUES
		  (?, 'Old Relay', 'https://old.example', 'oneapi', 'healthy', ?, ?),
		  (?, 'Existing Relay', ?, 'oneapi', 'healthy', ?, ?)
	`, oldSiteID, now(), now(), existingSiteID, targetBaseURL, now(), now()); err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.Exec(`
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, login_status, created_at, updated_at)
		VALUES
		  (?, ?, 'First Account', 'cookie', 'valid', ?, ?),
		  (?, ?, 'Second Account', 'cookie', 'valid', ?, ?)
	`, firstAccountID, oldSiteID, now(), now(), secondAccountID, oldSiteID, now(), now()); err != nil {
		t.Fatal(err)
	}

	current := ChannelAccount{
		ID:                  firstAccountID,
		UpstreamSiteID:      oldSiteID,
		UpstreamSiteName:    "Old Relay",
		UpstreamSiteBaseURL: "https://old.example",
	}
	siteID, changed, err := app.accountSiteUpdates.Resolve(context.Background(), current, "Merged Relay", targetBaseURL, manualLoginURL, "newapi", "shared")
	if err != nil {
		t.Fatal(err)
	}
	if siteID != existingSiteID || !changed {
		t.Fatalf("Resolve() = (%q, %v), want (%q, true)", siteID, changed, existingSiteID)
	}

	var oldCount, existingCount int
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM channel_accounts WHERE upstream_site_id=?`, oldSiteID).Scan(&oldCount); err != nil {
		t.Fatal(err)
	}
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM channel_accounts WHERE upstream_site_id=?`, existingSiteID).Scan(&existingCount); err != nil {
		t.Fatal(err)
	}
	if oldCount != 0 || existingCount != 2 {
		t.Fatalf("account reassignment old=%d existing=%d, want old=0 existing=2", oldCount, existingCount)
	}

	var name, loginURL, source, kind, discoveryJSON string
	var confidence float64
	if err := app.db.QueryRow(`
		SELECT name, login_url, login_url_source, login_url_confidence, kind, COALESCE(login_discovery_json,'')
		FROM upstream_sites WHERE id=?
	`, existingSiteID).Scan(&name, &loginURL, &source, &confidence, &kind, &discoveryJSON); err != nil {
		t.Fatal(err)
	}
	if name != "Merged Relay" || loginURL != manualLoginURL || source != "manual" || confidence != 1 || kind != "newapi" {
		t.Fatalf("site metadata = (%q, %q, %q, %.1f, %q), want manual newapi metadata", name, loginURL, source, confidence, kind)
	}
	if discovery := parseLoginDiscoveryJSON(discoveryJSON); discovery == nil || discovery.URL != manualLoginURL || discovery.Source != "manual" {
		t.Fatalf("manual discovery = %#v, want %q manual", discovery, manualLoginURL)
	}
}
