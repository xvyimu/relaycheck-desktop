package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBuildUsageOverviewDetectsDeclineAndLowBalance(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()
	var err error

	siteID := newID()
	accountID := newID()
	channelID := newID()
	previousAt := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339Nano)
	latestAt := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, created_at, updated_at)
		VALUES (?, 'Usage Relay', 'https://usage.example', 'newapi', 'healthy', ?, ?)
	`, siteID, now(), now())
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.db.Exec(`
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, login_status, created_at, updated_at)
		VALUES (?, ?, 'Usage Account', 'api_key', 'valid', ?, ?)
	`, accountID, siteID, now(), now())
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.db.Exec(`
		INSERT INTO balance_snapshots (id, account_id, upstream_site_id, channel_id, balance, used_quota, total_quota, unit, created_at)
		VALUES
		  (?, ?, ?, ?, 10, NULL, NULL, 'usd', ?),
		  (?, ?, ?, ?, 4, NULL, NULL, 'usd', ?)
	`, newID(), accountID, siteID, channelID, previousAt, newID(), accountID, siteID, channelID, latestAt)
	if err != nil {
		t.Fatal(err)
	}

	overview, err := app.buildUsageOverview(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if overview.AccountCount != 1 || overview.SiteCount != 1 {
		t.Fatalf("expected one account and site, got %+v", overview)
	}
	if overview.LowBalanceCount != 1 || overview.DecliningCount != 1 {
		t.Fatalf("expected low balance and declining account, got %+v", overview)
	}
	if len(overview.Accounts) != 1 || overview.Accounts[0].EstimatedDailyUse == nil {
		t.Fatalf("expected account daily use estimate, got %+v", overview.Accounts)
	}
	if *overview.Accounts[0].EstimatedDailyUse < 5.9 || *overview.Accounts[0].EstimatedDailyUse > 6.1 {
		t.Fatalf("expected about 6 usd/day, got %+v", *overview.Accounts[0].EstimatedDailyUse)
	}
}

func TestUsageOverviewSupportsSiteFilterLimitAndTruncation(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	nowText := now()
	for _, site := range []struct{ id, name string }{{"site-a", "Site A"}, {"site-b", "Site B"}} {
		if _, err := app.db.Exec(`INSERT INTO upstream_sites (id, name, base_url, kind, health_status, created_at, updated_at) VALUES (?, ?, ?, 'newapi', 'healthy', ?, ?)`, site.id, site.name, "https://"+site.id+".example", nowText, nowText); err != nil {
			t.Fatal(err)
		}
	}
	for _, account := range []struct{ id, siteID string }{{"account-a1", "site-a"}, {"account-a2", "site-a"}, {"account-b1", "site-b"}} {
		if _, err := app.db.Exec(`INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, login_status, created_at, updated_at) VALUES (?, ?, ?, 'api_key', 'valid', ?, ?)`, account.id, account.siteID, account.id, nowText, nowText); err != nil {
			t.Fatal(err)
		}
		if _, err := app.db.Exec(`INSERT INTO balance_snapshots (id, account_id, upstream_site_id, balance, unit, created_at) VALUES (?, ?, ?, 10, 'usd', ?)`, newID(), account.id, account.siteID, nowText); err != nil {
			t.Fatal(err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/usage/overview?siteId=site-a&limit=1", nil)
	rec := httptest.NewRecorder()
	app.handleUsageOverview(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var response struct {
		Data struct {
			AccountCount int                `json:"accountCount"`
			SiteCount    int                `json:"siteCount"`
			Truncated    bool               `json:"truncated"`
			Accounts     []usageAccountItem `json:"accounts"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Data.AccountCount != 1 || response.Data.SiteCount != 1 || len(response.Data.Accounts) != 1 {
		t.Fatalf("unexpected filtered overview: %#v", response.Data)
	}
	if response.Data.Accounts[0].SiteID != "site-a" {
		t.Fatalf("site filter was ignored: %#v", response.Data.Accounts)
	}
	if !response.Data.Truncated {
		t.Fatal("expected truncated=true when matching accounts exceed limit")
	}
}
