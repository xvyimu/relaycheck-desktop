package core

import (
	"net/http/httptest"
	"testing"
)

func TestChannelHealthOverviewSummarizesSiteKeyAndModelRisk(t *testing.T) {
	app := newTestApp(t)
	nowText := now()

	if _, err := app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, supports_models, created_at, updated_at)
		VALUES
			('site-healthy', 'Healthy Site', 'https://healthy.example', 'newapi', 'healthy', 1, ?, ?),
			('site-down', 'Down Site', 'https://down.example', 'oneapi', 'unreachable', 1, ?, ?)
	`, nowText, nowText, nowText, nowText); err != nil {
		t.Fatalf("seed sites: %v", err)
	}
	if _, err := app.db.Exec(`
		INSERT INTO imported_channels (id, source_channel_id, name, base_url, upstream_kind, raw_json, models_status, model_count, created_at, updated_at)
		VALUES
			('channel-ok', 'source-ok', 'Healthy Channel', 'https://healthy.example', 'newapi', '{}', 'live_key', 2, ?, ?),
			('channel-bad', 'source-bad', 'Down Channel', 'https://down.example', 'oneapi', '{}', 'failed', 0, ?, ?)
	`, nowText, nowText, nowText, nowText); err != nil {
		t.Fatalf("seed channels: %v", err)
	}
	if _, err := app.db.Exec(`
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, login_status, api_key_fingerprint, api_key_status, api_key_model_count, api_key_model_usable, created_at, updated_at)
		VALUES
			('account-ok', 'site-healthy', 'Good Key', 'api_key', 'logged_in', 'fp-ok', 'valid', 2, 1, ?, ?),
			('account-bad', 'site-down', 'Bad Key', 'api_key', 'logged_in', 'fp-bad', 'invalid', 0, 0, ?, ?)
	`, nowText, nowText, nowText, nowText); err != nil {
		t.Fatalf("seed accounts: %v", err)
	}

	overview, err := app.channelHealthOverview(httptest.NewRequest("GET", "/api/channels/health/overview", nil))
	if err != nil {
		t.Fatalf("channelHealthOverview: %v", err)
	}

	if overview.Overall != "danger" {
		t.Fatalf("overall = %q, want danger", overview.Overall)
	}
	if overview.SiteCount != 2 || overview.UnreachableSiteCount != 1 {
		t.Fatalf("site counts = total %d unreachable %d, want 2/1", overview.SiteCount, overview.UnreachableSiteCount)
	}
	if overview.InvalidKeyCount != 1 || overview.ValidKeyCount != 1 {
		t.Fatalf("key counts = valid %d invalid %d, want 1/1", overview.ValidKeyCount, overview.InvalidKeyCount)
	}
	if overview.FailedModelChannelCount != 1 || overview.LiveModelChannelCount != 1 {
		t.Fatalf("model counts = live %d failed %d, want 1/1", overview.LiveModelChannelCount, overview.FailedModelChannelCount)
	}

	down := findChannelHealthSite(t, overview.Sites, "site-down")
	if down.Level != "danger" {
		t.Fatalf("down site level = %q, want danger", down.Level)
	}
	if down.RecommendedAction == "" {
		t.Fatal("down site should include recommended action")
	}
	if down.InvalidKeyCount != 1 || down.FailedModelChannelCount != 1 {
		t.Fatalf("down site key/model counts = invalid %d failed %d, want 1/1", down.InvalidKeyCount, down.FailedModelChannelCount)
	}
}

func findChannelHealthSite(t *testing.T, sites []ChannelHealthSite, id string) ChannelHealthSite {
	t.Helper()
	for _, site := range sites {
		if site.SiteID == id {
			return site
		}
	}
	t.Fatalf("missing health site %q in %#v", id, sites)
	return ChannelHealthSite{}
}
