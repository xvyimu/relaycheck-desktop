package core

import (
	"net/http/httptest"
	"testing"
)

func TestActionCenterAnnotatesOperationalMetadata(t *testing.T) {
	app := newTestApp(t)
	nowText := now()

	if _, err := app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, supports_balance, created_at, updated_at)
		VALUES
			('site-key', 'Key Site', 'https://key.example', 'newapi', 'healthy', 1, ?, ?),
			('site-down', 'Down Site', 'https://down.example', 'oneapi', 'unreachable', 0, ?, ?)
	`, nowText, nowText, nowText, nowText); err != nil {
		t.Fatalf("seed sites: %v", err)
	}
	if _, err := app.db.Exec(`
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, login_status, api_key_fingerprint, api_key_status, created_at, updated_at)
		VALUES ('account-key', 'site-key', 'Broken Key', 'api_key', 'logged_in', 'fp-test', 'invalid', ?, ?)
	`, nowText, nowText); err != nil {
		t.Fatalf("seed accounts: %v", err)
	}

	center, err := app.actionCenter(httptest.NewRequest("GET", "/api/system/action-center", nil))
	if err != nil {
		t.Fatalf("actionCenter: %v", err)
	}

	keyIssue := findActionItem(t, center.Items, "api-key-problems")
	if keyIssue.Level != "danger" {
		t.Fatalf("api-key-problems level = %q, want danger", keyIssue.Level)
	}
	if keyIssue.Category != "key" {
		t.Fatalf("api-key-problems category = %q, want key", keyIssue.Category)
	}
	if keyIssue.Impact == "" {
		t.Fatal("api-key-problems impact should explain the operational risk")
	}
	if keyIssue.RecommendedAction == "" {
		t.Fatal("api-key-problems recommended action should be populated")
	}
	if len(keyIssue.Samples) != 1 || keyIssue.Samples[0] == "" {
		t.Fatalf("api-key-problems samples = %#v, want one non-empty sample", keyIssue.Samples)
	}

	siteIssue := findActionItem(t, center.Items, "unreachable-sites")
	if siteIssue.Category != "site" {
		t.Fatalf("unreachable-sites category = %q, want site", siteIssue.Category)
	}
	if siteIssue.Impact == "" || siteIssue.RecommendedAction == "" {
		t.Fatalf("unreachable-sites metadata missing impact/action: %#v", siteIssue)
	}
}

func TestActionCenterOverallLevel(t *testing.T) {
	if got := actionCenterLevel(nil); got != "success" {
		t.Fatalf("empty overall = %q, want success", got)
	}
	if got := actionCenterLevel([]ActionItem{{Level: "info"}}); got != "info" {
		t.Fatalf("info overall = %q, want info", got)
	}
	if got := actionCenterLevel([]ActionItem{{Level: "info"}, {Level: "warning"}}); got != "warning" {
		t.Fatalf("warning overall = %q, want warning", got)
	}
	if got := actionCenterLevel([]ActionItem{{Level: "warning"}, {Level: "danger"}}); got != "danger" {
		t.Fatalf("danger overall = %q, want danger", got)
	}
}

func TestActionCenterIncludesChannelHealthRisks(t *testing.T) {
	app := newTestApp(t)
	nowText := now()

	if _, err := app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, created_at, updated_at)
		VALUES ('site-health-risk', 'Health Risk Site', 'https://health-risk.example', 'newapi', 'unreachable', ?, ?)
	`, nowText, nowText); err != nil {
		t.Fatalf("seed site: %v", err)
	}
	if _, err := app.db.Exec(`
		INSERT INTO imported_channels (id, source_channel_id, name, base_url, upstream_kind, raw_json, models_status, model_count, created_at, updated_at)
		VALUES ('channel-health-risk', 'source-health-risk', 'Health Risk Channel', 'https://health-risk.example', 'newapi', '{}', 'failed', 0, ?, ?)
	`, nowText, nowText); err != nil {
		t.Fatalf("seed channel: %v", err)
	}

	center, err := app.actionCenter(httptest.NewRequest("GET", "/api/system/action-center", nil))
	if err != nil {
		t.Fatalf("actionCenter: %v", err)
	}

	item := findActionItem(t, center.Items, "channel-health-risks")
	if item.Category != "health" {
		t.Fatalf("category = %q, want health", item.Category)
	}
	if item.Target != "channels" || item.Filter != "health" {
		t.Fatalf("target/filter = %q/%q, want channels/health", item.Target, item.Filter)
	}
	if item.Count != 1 {
		t.Fatalf("count = %d, want 1", item.Count)
	}
	if item.Impact == "" || item.RecommendedAction == "" {
		t.Fatalf("missing health risk metadata: %#v", item)
	}
}

func findActionItem(t *testing.T, items []ActionItem, id string) ActionItem {
	t.Helper()
	for _, item := range items {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("missing action item %q in %#v", id, items)
	return ActionItem{}
}
