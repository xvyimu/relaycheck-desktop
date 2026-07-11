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

	center, err := app.buildActionCenter(httptest.NewRequest("GET", "/api/system/action-center", nil))
	if err != nil {
		t.Fatalf("buildActionCenter: %v", err)
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
	if len(keyIssue.Samples) != 1 || keyIssue.Samples[0].Label == "" {
		t.Fatalf("api-key-problems samples = %#v, want one non-empty sample", keyIssue.Samples)
	}

	siteIssue := findActionItem(t, center.Items, "unreachable-sites")
	if siteIssue.Category != "site" {
		t.Fatalf("unreachable-sites category = %q, want site", siteIssue.Category)
	}
	if siteIssue.Impact == "" || siteIssue.RecommendedAction == "" {
		t.Fatalf("unreachable-sites metadata missing impact/action: %#v", siteIssue)
	}
	if len(siteIssue.Samples) != 1 {
		t.Fatalf("unreachable-sites samples = %#v, want one sample with site entity", siteIssue.Samples)
	}
	if siteIssue.Samples[0].EntityType != "site" || siteIssue.Samples[0].EntityID != "site-down" {
		t.Fatalf("unreachable-sites sample entity = %#v, want site/site-down", siteIssue.Samples[0])
	}
	if siteIssue.Samples[0].Label == "" {
		t.Fatal("unreachable-sites sample label should be non-empty")
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

	center, err := app.buildActionCenter(httptest.NewRequest("GET", "/api/system/action-center", nil))
	if err != nil {
		t.Fatalf("buildActionCenter: %v", err)
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
	if len(item.Samples) != 1 {
		t.Fatalf("channel-health-risks samples = %#v, want one site sample", item.Samples)
	}
	if item.Samples[0].EntityType != "site" || item.Samples[0].EntityID != "site-health-risk" {
		t.Fatalf("channel-health-risks sample entity = %#v, want site/site-health-risk", item.Samples[0])
	}
	if item.Samples[0].Label == "" {
		t.Fatal("channel-health-risks sample label should be non-empty")
	}
}

func TestActionCenterShowsSetupNextStepForEmptyWorkspace(t *testing.T) {
	app := newTestApp(t)

	center, err := app.buildActionCenter(httptest.NewRequest("GET", "/api/system/action-center", nil))
	if err != nil {
		t.Fatalf("buildActionCenter: %v", err)
	}

	item := findActionItem(t, center.Items, "setup-connect-newapi")
	if item.Level != "info" {
		t.Fatalf("level = %q, want info", item.Level)
	}
	if item.Category != "setup" {
		t.Fatalf("category = %q, want setup", item.Category)
	}
	if item.Target != "scan" {
		t.Fatalf("target = %q, want scan", item.Target)
	}
	if item.Count != 1 {
		t.Fatalf("count = %d, want 1", item.Count)
	}
	if item.RecommendedAction == "" || item.Impact == "" {
		t.Fatalf("missing setup guidance metadata: %#v", item)
	}
}

func TestActionCenterSetupNextStepProgressesWithWorkspaceState(t *testing.T) {
	app := newTestApp(t)
	nowText := now()

	if _, err := app.db.Exec(`
		INSERT INTO local_newapi_instances (id, name, base_url, status, created_at, updated_at)
		VALUES ('inst-1', 'Local NewAPI', 'http://127.0.0.1:3000', 'reachable', ?, ?)
	`, nowText, nowText); err != nil {
		t.Fatalf("seed instance: %v", err)
	}

	center, err := app.buildActionCenter(httptest.NewRequest("GET", "/api/system/action-center", nil))
	if err != nil {
		t.Fatalf("buildActionCenter after instance: %v", err)
	}
	importItem := findActionItem(t, center.Items, "setup-import-channels")
	if importItem.Target != "scan" || importItem.Filter != "import" {
		t.Fatalf("import target/filter = %q/%q, want scan/import", importItem.Target, importItem.Filter)
	}
	assertMissingActionItem(t, center.Items, "setup-connect-newapi")

	if _, err := app.db.Exec(`
		INSERT INTO imported_channels (id, local_instance_id, source_channel_id, name, base_url, status, upstream_kind, raw_json, created_at, updated_at)
		VALUES ('channel-1', 'inst-1', 'source-1', 'Imported Channel', 'https://relay.example', 'active', 'newapi', '{}', ?, ?)
	`, nowText, nowText); err != nil {
		t.Fatalf("seed channel: %v", err)
	}

	center, err = app.buildActionCenter(httptest.NewRequest("GET", "/api/system/action-center", nil))
	if err != nil {
		t.Fatalf("buildActionCenter after channel: %v", err)
	}
	accountItem := findActionItem(t, center.Items, "setup-add-account")
	if accountItem.Target != "accounts" {
		t.Fatalf("account target = %q, want accounts", accountItem.Target)
	}
	assertMissingActionItem(t, center.Items, "setup-import-channels")

	if _, err := app.db.Exec(`
		INSERT INTO upstream_sites (id, channel_id, name, base_url, kind, health_status, supports_checkin, created_at, updated_at)
		VALUES ('site-1', 'channel-1', 'Relay Site', 'https://relay.example', 'newapi', 'healthy', 1, ?, ?)
	`, nowText, nowText); err != nil {
		t.Fatalf("seed site: %v", err)
	}
	if _, err := app.db.Exec(`
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, login_status, created_at, updated_at)
		VALUES ('account-1', 'site-1', 'Relay Account', 'api_key', 'unknown', ?, ?)
	`, nowText, nowText); err != nil {
		t.Fatalf("seed account: %v", err)
	}

	center, err = app.buildActionCenter(httptest.NewRequest("GET", "/api/system/action-center", nil))
	if err != nil {
		t.Fatalf("buildActionCenter after account: %v", err)
	}
	verifyItem := findActionItem(t, center.Items, "setup-verify-dry-run")
	if verifyItem.Target != "checkins" || verifyItem.Filter != "all" {
		t.Fatalf("verify target/filter = %q/%q, want checkins/all", verifyItem.Target, verifyItem.Filter)
	}
	assertMissingActionItem(t, center.Items, "setup-add-account")

	if _, err := app.db.Exec(`
		INSERT INTO checkin_logs (id, account_id, upstream_site_id, status, started_at, finished_at)
		VALUES ('log-1', 'account-1', 'site-1', 'success', ?, ?)
	`, nowText, nowText); err != nil {
		t.Fatalf("seed checkin log: %v", err)
	}

	center, err = app.buildActionCenter(httptest.NewRequest("GET", "/api/system/action-center", nil))
	if err != nil {
		t.Fatalf("buildActionCenter after checkin log: %v", err)
	}
	assertMissingActionItem(t, center.Items, "setup-verify-dry-run")
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

func assertMissingActionItem(t *testing.T, items []ActionItem, id string) {
	t.Helper()
	for _, item := range items {
		if item.ID == id {
			t.Fatalf("unexpected action item %q in %#v", id, items)
		}
	}
}
