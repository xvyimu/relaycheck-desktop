package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestAccountsPageUsesStableCursorAndServerTotals(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	siteID := newID()
	if _, err := app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, created_at, updated_at)
		VALUES (?, 'Paged Site', 'https://paged.example', 'newapi', 'healthy', ?, ?)
	`, siteID, now(), now()); err != nil {
		t.Fatal(err)
	}

	accountIDs := []string{newID(), newID(), newID(), newID(), newID()}
	updated := []string{
		"2026-07-13T05:00:00Z",
		"2026-07-13T04:00:00Z",
		"2026-07-13T03:00:00Z",
		"2026-07-13T02:00:00Z",
		"2026-07-13T01:00:00Z",
	}
	for index, id := range accountIDs {
		loginStatus := "valid"
		lastCheckinStatus := "success"
		if index == 1 {
			loginStatus = "expired"
		}
		if index == 3 {
			lastCheckinStatus = "failed"
		}
		if _, err := app.db.Exec(`
			INSERT INTO channel_accounts (
				id, upstream_site_id, display_name, email, auth_type, login_status,
				last_checkin_status, created_at, updated_at
			) VALUES (?, ?, ?, ?, 'cookie', ?, ?, ?, ?)
		`, id, siteID, "Account "+string(rune('A'+index)), "user@example.com", loginStatus, lastCheckinStatus, updated[index], updated[index]); err != nil {
			t.Fatalf("insert account %d: %v", index, err)
		}
	}

	first := requestAccountsPage(t, app, "/api/accounts/page?limit=2")
	if len(first.Items) != 2 {
		t.Fatalf("expected first page size 2, got %d", len(first.Items))
	}
	if first.Total != 5 || first.AccountTotal != 5 || first.ProblemTotal != 2 {
		t.Fatalf("unexpected totals: %#v", first)
	}
	if first.Items[0].ID != accountIDs[0] || first.Items[1].ID != accountIDs[1] {
		t.Fatalf("unexpected first page order: %#v", first.Items)
	}
	if first.NextCursor == "" {
		t.Fatal("expected next cursor")
	}

	second := requestAccountsPage(t, app, "/api/accounts/page?limit=2&cursor="+url.QueryEscape(first.NextCursor))
	if len(second.Items) != 2 {
		t.Fatalf("expected second page size 2, got %d", len(second.Items))
	}
	if second.Items[0].ID != accountIDs[2] || second.Items[1].ID != accountIDs[3] {
		t.Fatalf("unexpected second page order: %#v", second.Items)
	}
	if second.Items[0].ID == first.Items[0].ID || second.Items[0].ID == first.Items[1].ID {
		t.Fatalf("cursor page repeated an account: %#v", second.Items)
	}
}

func TestAccountsPageFiltersAndRejectsInvalidCursor(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	siteID := newID()
	if _, err := app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, created_at, updated_at)
		VALUES (?, 'Filter Site', 'https://filter.example', 'newapi', 'healthy', ?, ?)
	`, siteID, now(), now()); err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.Exec(`
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, email, auth_type, login_status, last_checkin_status, created_at, updated_at)
		VALUES
		  (?, ?, 'Alpha Account', 'alpha@example.com', 'cookie', 'expired', 'success', '2026-07-13T02:00:00Z', '2026-07-13T02:00:00Z'),
		  (?, ?, 'Beta Account', 'beta@example.com', 'cookie', 'valid', 'success', '2026-07-13T01:00:00Z', '2026-07-13T01:00:00Z')
	`, newID(), siteID, newID(), siteID); err != nil {
		t.Fatal(err)
	}

	filtered := requestAccountsPage(t, app, "/api/accounts/page?limit=20&status=problem&query=alpha&upstreamSiteId="+siteID)
	if len(filtered.Items) != 1 || filtered.Items[0].DisplayName != "Alpha Account" {
		t.Fatalf("expected only Alpha problem account, got %#v", filtered.Items)
	}
	if filtered.Total != 1 || filtered.AccountTotal != 2 || filtered.ProblemTotal != 1 {
		t.Fatalf("unexpected filtered totals: %#v", filtered)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/accounts/page?cursor=not-a-cursor", nil)
	rec := httptest.NewRecorder()
	app.handleAccountsPage(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid cursor 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAccountSummaryAndSearchIndexUseCompactServerData(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	siteID := newID()
	if _, err := app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, created_at, updated_at)
		VALUES (?, 'Search Site', 'https://search.example', 'newapi', 'healthy', ?, ?)
	`, siteID, now(), now()); err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.Exec(`
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, email, username, auth_type, login_status, created_at, updated_at)
		VALUES
		  (?, ?, 'Alpha Account', 'alpha@example.com', 'alpha-user', 'cookie', 'expired', ?, ?),
		  (?, ?, 'Beta Account', 'beta@example.com', 'beta-user', 'cookie', 'valid', ?, ?)
	`, newID(), siteID, now(), now(), newID(), siteID, now(), now()); err != nil {
		t.Fatal(err)
	}

	summary, err := app.loadAccountSummary(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if summary.AccountTotal != 2 || summary.ProblemTotal != 1 {
		t.Fatalf("unexpected account summary: %#v", summary)
	}

	index, err := app.loadAccountSearchIndex(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(index) != 1 || index[0].UpstreamSiteID != siteID {
		t.Fatalf("unexpected search index: %#v", index)
	}
	for _, expected := range []string{"Alpha Account", "alpha@example.com", "beta-user"} {
		if !strings.Contains(index[0].SearchText, expected) {
			t.Fatalf("search index missing %q: %#v", expected, index[0])
		}
	}
}

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

func requestAccountsPage(t *testing.T, app *App, target string) AccountPage {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	w := httptest.NewRecorder()
	app.handleAccountsPage(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", w.Code, w.Body.String())
	}
	var response struct {
		OK   bool        `json:"ok"`
		Data AccountPage `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode accounts page response: %v", err)
	}
	if !response.OK {
		t.Fatalf("expected ok response, got body=%s", w.Body.String())
	}
	return response.Data
}
