package core

import (
	"context"
	"testing"
	"time"
)

func TestBuildUsageOverviewDetectsDeclineAndLowBalance(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

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
