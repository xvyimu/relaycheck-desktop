package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListAccountsHonorsUpstreamSiteIDFilter(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	siteA := newID()
	siteB := newID()
	accountA := newID()
	accountB := newID()
	if _, err := app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, created_at, updated_at)
		VALUES
		  (?, 'Site A', 'https://site-a.example', 'newapi', 'healthy', ?, ?),
		  (?, 'Site B', 'https://site-b.example', 'oneapi', 'healthy', ?, ?)
	`, siteA, now(), now(), siteB, now(), now()); err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.Exec(`
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, login_status, created_at, updated_at)
		VALUES
		  (?, ?, 'Account A', 'cookie', 'valid', ?, ?),
		  (?, ?, 'Account B', 'cookie', 'valid', ?, ?)
	`, accountA, siteA, now(), now(), accountB, siteB, now(), now()); err != nil {
		t.Fatal(err)
	}

	// Populate the unfiltered cache first. A filtered request must not reuse this
	// full-table cache entry.
	unfiltered := requestAccounts(t, app, "/api/accounts")
	if len(unfiltered) != 2 {
		t.Fatalf("expected unfiltered request to return both accounts, got %d", len(unfiltered))
	}

	filteredA := requestAccounts(t, app, "/api/accounts?upstreamSiteId="+siteA)
	if len(filteredA) != 1 || filteredA[0].ID != accountA || filteredA[0].UpstreamSiteID != siteA {
		t.Fatalf("expected only site A account, got %#v", filteredA)
	}

	filteredB := requestAccounts(t, app, "/api/accounts?upstreamSiteId="+siteB)
	if len(filteredB) != 1 || filteredB[0].ID != accountB || filteredB[0].UpstreamSiteID != siteB {
		t.Fatalf("expected only site B account, got %#v", filteredB)
	}
}

func requestAccounts(t *testing.T, app *App, target string) []ChannelAccount {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	w := httptest.NewRecorder()
	app.listAccounts(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", w.Code, w.Body.String())
	}
	var response struct {
		OK   bool             `json:"ok"`
		Data []ChannelAccount `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode accounts response: %v", err)
	}
	if !response.OK {
		t.Fatalf("expected ok response, got body=%s", w.Body.String())
	}
	return response.Data
}
