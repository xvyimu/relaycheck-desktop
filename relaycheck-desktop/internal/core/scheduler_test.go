package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBuildSchedulerStatusIncludesKnownJobs(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	status := app.buildSchedulerStatus(context.Background())
	if len(status.Jobs) != 3 {
		t.Fatalf("expected three scheduler jobs, got %d", len(status.Jobs))
	}
	if status.Jobs[0].Key != schedulerJobCheckin || status.Jobs[1].Key != schedulerJobSync || status.Jobs[2].Key != schedulerJobChannelHealth {
		t.Fatalf("unexpected scheduler jobs: %#v", status.Jobs)
	}
}

func TestChannelHealthSchedulerWaitsForDefaultIntervalOnStartup(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	startedAt := time.Date(2026, 6, 19, 8, 0, 0, 0, time.Local)
	app.schedulerStartedAt = startedAt
	app.tickChannelHealthScheduler(context.Background(), startedAt.Add(30*time.Minute))

	record, err := app.loadSchedulerRun(context.Background(), schedulerJobChannelHealth)
	if err != nil {
		t.Fatal(err)
	}
	if record.Status != "scheduled" {
		t.Fatalf("expected scheduled status, got %s", record.Status)
	}
	if record.LastFinishedAt != "" {
		t.Fatalf("expected no immediate startup channel health probe, got finished at %s", record.LastFinishedAt)
	}
	nextRunAt, err := time.Parse(time.RFC3339Nano, record.NextRunAt)
	if err != nil {
		t.Fatal(err)
	}
	if nextRunAt.Sub(startedAt) != 60*time.Minute {
		t.Fatalf("expected next run 60 minutes after startup, got %s", nextRunAt.Sub(startedAt))
	}
}

func TestChannelHealthSchedulerRunsWhenDue(t *testing.T) {
	app, siteID, channelID := newChannelHealthSchedulerFixture(t)

	currentTime := time.Date(2026, 6, 19, 9, 0, 0, 0, time.Local)
	app.tickChannelHealthScheduler(context.Background(), currentTime)

	record, err := app.loadSchedulerRun(context.Background(), schedulerJobChannelHealth)
	if err != nil {
		t.Fatal(err)
	}
	if record.Status != "warning" {
		t.Fatalf("expected warning status for auth/model risk, got %#v", record)
	}
	if record.LastFinishedAt == "" || record.LastSuccessAt == "" {
		t.Fatalf("expected finished/success timestamps, got %#v", record)
	}
	if !strings.Contains(record.Summary, "processed 1/1") {
		t.Fatalf("expected processed summary, got %q", record.Summary)
	}

	site, err := app.loadSiteDetail(context.Background(), siteID)
	if err != nil {
		t.Fatalf("load site detail: %v", err)
	}
	if site.Site.Kind != "newapi" || site.Site.HealthStatus != "auth_required" {
		t.Fatalf("site after scheduler = %#v, want newapi/auth_required", site.Site)
	}
	channel, err := app.loadChannelByID(context.Background(), channelID)
	if err != nil {
		t.Fatalf("load channel: %v", err)
	}
	if channel.ModelsStatus != "key_invalid" {
		t.Fatalf("models status = %q, want key_invalid", channel.ModelsStatus)
	}
}

func TestChannelHealthSchedulerWarningNotificationHasSamplesAndDedupe(t *testing.T) {
	app, _, _ := newChannelHealthSchedulerFixture(t)

	app.tickChannelHealthScheduler(context.Background(), time.Now())

	var count int
	var content string
	err := app.db.QueryRow(`
		SELECT COUNT(*), COALESCE(MAX(content),'')
		FROM app_notifications
		WHERE type='scheduled_channel_health_probe_warning'
		  AND related_type='scheduler'
		  AND related_id=?
	`, schedulerJobChannelHealth).Scan(&count, &content)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected one health warning notification, got %d", count)
	}
	if !strings.Contains(content, "Scheduled Relay") || !strings.Contains(content, "auth_required") {
		t.Fatalf("expected notification content to include site sample and status, got %q", content)
	}

	_, err = app.db.Exec(`
		UPDATE scheduler_runs
		SET last_finished_at=?, last_success_at=?
		WHERE job_key=?
	`, time.Now().Add(-31*time.Minute).UTC().Format(time.RFC3339Nano), time.Now().Add(-31*time.Minute).UTC().Format(time.RFC3339Nano), schedulerJobChannelHealth)
	if err != nil {
		t.Fatal(err)
	}
	app.tickChannelHealthScheduler(context.Background(), time.Now())

	err = app.db.QueryRow(`
		SELECT COUNT(*)
		FROM app_notifications
		WHERE type='scheduled_channel_health_probe_warning'
		  AND related_type='scheduler'
		  AND related_id=?
	`, schedulerJobChannelHealth).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected duplicate health warning notification to be suppressed, got %d", count)
	}
}

func newChannelHealthSchedulerFixture(t *testing.T) (*App, string, string) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/about":
			_, _ = w.Write([]byte(`{"success":true,"data":{"system_name":"New API","version":"1.0.0"}}`))
		case "/api/channel/":
			http.Error(w, `{"message":"unauthorized"}`, http.StatusUnauthorized)
		case "/api/user/self":
			http.Error(w, `{"message":"unauthorized"}`, http.StatusUnauthorized)
		case "/v1/models":
			http.Error(w, `{"error":{"message":"invalid channel key"}}`, http.StatusUnauthorized)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	app := newTestApp(t)
	app.client = server.Client()
	app.allowLocalOutbound = true

	channelKey, err := app.encryptText("sk-channel-health-scheduler")
	if err != nil {
		t.Fatal(err)
	}
	nowText := now()
	siteID := "site-health-scheduler"
	channelID := "channel-health-scheduler"
	if _, err := app.db.Exec(`
		INSERT INTO imported_channels (id, source_channel_id, name, base_url, status, upstream_kind, channel_key_encrypted, raw_json, created_at, updated_at)
		VALUES (?, 'source-health-scheduler', 'Scheduled Relay', ?, 'enabled', 'newapi', ?, '{}', ?, ?)
	`, channelID, server.URL, channelKey, nowText, nowText); err != nil {
		t.Fatalf("seed channel: %v", err)
	}
	if _, err := app.db.Exec(`
		INSERT INTO upstream_sites (id, channel_id, name, base_url, kind, health_status, supports_models, created_at, updated_at)
		VALUES (?, ?, 'Scheduled Relay', ?, 'unknown', 'unknown', 0, ?, ?)
	`, siteID, channelID, server.URL, nowText, nowText); err != nil {
		t.Fatalf("seed site: %v", err)
	}
	if _, err := app.db.Exec(`INSERT OR REPLACE INTO system_settings (id, key, value_json, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		"setting-channel-health-schedule", "channel.health.schedule", `{"enabled":true,"intervalMinutes":5,"runOnStartup":true,"limit":5,"onlyRisky":false}`, nowText, nowText); err != nil {
		t.Fatalf("seed schedule setting: %v", err)
	}
	return app, siteID, channelID
}

func TestSyncSchedulerWaitsForDefaultIntervalOnStartup(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	startedAt := time.Date(2026, 6, 19, 8, 0, 0, 0, time.Local)
	app.schedulerStartedAt = startedAt
	app.tickSyncScheduler(context.Background(), startedAt.Add(10*time.Minute))

	record, err := app.loadSchedulerRun(context.Background(), schedulerJobSync)
	if err != nil {
		t.Fatal(err)
	}
	if record.Status != "scheduled" {
		t.Fatalf("expected scheduled status, got %s", record.Status)
	}
	if record.LastFinishedAt != "" {
		t.Fatalf("expected no immediate startup sync, got finished at %s", record.LastFinishedAt)
	}
	nextRunAt, err := time.Parse(time.RFC3339Nano, record.NextRunAt)
	if err != nil {
		t.Fatal(err)
	}
	if nextRunAt.Sub(startedAt) != 30*time.Minute {
		t.Fatalf("expected next run 30 minutes after startup, got %s", nextRunAt.Sub(startedAt))
	}
}

func TestTickChannelScheduler_TriggersDueSite(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	ctx := context.Background()

	// Create an upstream site
	_, err := app.db.ExecContext(ctx,
		`INSERT INTO upstream_sites (id, name, base_url, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		"site-tick-1", "定时站点", "https://example.com", now(), now())
	if err != nil {
		t.Fatalf("create upstream site: %v", err)
	}

	// Insert a channel schedule with a past next_run_at (due immediately)
	pastTime := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	_, err = app.db.ExecContext(ctx, `
		INSERT INTO channel_schedules (id, upstream_site_id, enabled, checkin_time, random_delay_min, random_delay_max, next_run_at, created_at, updated_at)
		VALUES (?, ?, 1, '08:00', 0, 30, ?, ?, ?)
	`, "site-tick-1", "site-tick-1", pastTime, now(), now())
	if err != nil {
		t.Fatalf("create channel schedule: %v", err)
	}

	// Tick the channel scheduler
	app.tickChannelScheduler(ctx, time.Now())

	// Verify next_run_at was updated to a future time
	var newNextRunAt string
	err = app.db.QueryRowContext(ctx,
		`SELECT next_run_at FROM channel_schedules WHERE id=?`, "site-tick-1").Scan(&newNextRunAt)
	if err != nil {
		t.Fatalf("query next_run_at: %v", err)
	}
	if newNextRunAt == "" || newNextRunAt == pastTime {
		t.Fatalf("expected next_run_at to be updated, got %s (was %s)", newNextRunAt, pastTime)
	}
	parsed, err := time.Parse(time.RFC3339, newNextRunAt)
	if err != nil {
		t.Fatalf("parse next_run_at: %v", err)
	}
	if !parsed.After(time.Now().Add(-24 * time.Hour)) {
		t.Fatalf("next_run_at %s should be a recent future time", newNextRunAt)
	}

	// Verify last_run_at was set
	var lastRunAt string
	err = app.db.QueryRowContext(ctx,
		`SELECT COALESCE(last_run_at,'') FROM channel_schedules WHERE id=?`, "site-tick-1").Scan(&lastRunAt)
	if err != nil {
		t.Fatalf("query last_run_at: %v", err)
	}
	if lastRunAt == "" {
		t.Fatal("expected last_run_at to be set after tick")
	}
}

func TestTickChannelScheduler_SkipsFutureSite(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	ctx := context.Background()

	// Create an upstream site
	_, err := app.db.ExecContext(ctx,
		`INSERT INTO upstream_sites (id, name, base_url, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		"site-tick-2", "未来站点", "https://example.com", now(), now())
	if err != nil {
		t.Fatalf("create upstream site: %v", err)
	}

	// Insert a channel schedule with a future next_run_at (should NOT trigger)
	futureTime := time.Now().Add(2 * time.Hour).Format(time.RFC3339)
	_, err = app.db.ExecContext(ctx, `
		INSERT INTO channel_schedules (id, upstream_site_id, enabled, checkin_time, random_delay_min, random_delay_max, next_run_at, created_at, updated_at)
		VALUES (?, ?, 1, '08:00', 0, 30, ?, ?, ?)
	`, "site-tick-2", "site-tick-2", futureTime, now(), now())
	if err != nil {
		t.Fatalf("create channel schedule: %v", err)
	}

	// Tick the channel scheduler
	app.tickChannelScheduler(ctx, time.Now())

	// Verify next_run_at was NOT changed
	var newNextRunAt string
	err = app.db.QueryRowContext(ctx,
		`SELECT next_run_at FROM channel_schedules WHERE id=?`, "site-tick-2").Scan(&newNextRunAt)
	if err != nil {
		t.Fatalf("query next_run_at: %v", err)
	}
	if newNextRunAt != futureTime {
		t.Fatalf("expected next_run_at to remain %s, got %s", futureTime, newNextRunAt)
	}
}

func TestTickChannelScheduler_SkipsDisabledSite(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	ctx := context.Background()

	// Create an upstream site
	_, err := app.db.ExecContext(ctx,
		`INSERT INTO upstream_sites (id, name, base_url, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		"site-tick-3", "禁用站点", "https://example.com", now(), now())
	if err != nil {
		t.Fatalf("create upstream site: %v", err)
	}

	// Insert a disabled channel schedule with past next_run_at (should NOT trigger)
	pastTime := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	_, err = app.db.ExecContext(ctx, `
		INSERT INTO channel_schedules (id, upstream_site_id, enabled, checkin_time, random_delay_min, random_delay_max, next_run_at, created_at, updated_at)
		VALUES (?, ?, 0, '08:00', 0, 30, ?, ?, ?)
	`, "site-tick-3", "site-tick-3", pastTime, now(), now())
	if err != nil {
		t.Fatalf("create channel schedule: %v", err)
	}

	// Tick the channel scheduler
	app.tickChannelScheduler(ctx, time.Now())

	// Verify next_run_at was NOT changed (disabled schedule)
	var newNextRunAt string
	err = app.db.QueryRowContext(ctx,
		`SELECT next_run_at FROM channel_schedules WHERE id=?`, "site-tick-3").Scan(&newNextRunAt)
	if err != nil {
		t.Fatalf("query next_run_at: %v", err)
	}
	if newNextRunAt != pastTime {
		t.Fatalf("expected next_run_at to remain %s (disabled), got %s", pastTime, newNextRunAt)
	}
}

func TestTickChannelScheduler_DoesNotAdvanceWhenCheckinRunBusy(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	ctx := context.Background()
	_, err := app.db.ExecContext(ctx,
		`INSERT INTO upstream_sites (id, name, base_url, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		"site-tick-busy", "Busy Site", "https://busy.example", now(), now())
	if err != nil {
		t.Fatalf("create upstream site: %v", err)
	}
	_, err = app.db.ExecContext(ctx, `
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, login_status, created_at, updated_at)
		VALUES (?, ?, ?, 'cookie', 'valid', ?, ?)
	`, "account-tick-busy", "site-tick-busy", "Busy Account", now(), now())
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	pastTime := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	_, err = app.db.ExecContext(ctx, `
		INSERT INTO channel_schedules (id, upstream_site_id, enabled, checkin_time, random_delay_min, random_delay_max, next_run_at, created_at, updated_at)
		VALUES (?, ?, 1, '08:00', 0, 30, ?, ?, ?)
	`, "site-tick-busy", "site-tick-busy", pastTime, now(), now())
	if err != nil {
		t.Fatalf("create channel schedule: %v", err)
	}

	if !app.beginCheckinRun("manual", 1) {
		t.Fatal("expected to start blocking checkin run")
	}
	defer app.finishCheckinRun()

	app.tickChannelScheduler(ctx, time.Now())

	var nextRunAt, lastRunAt string
	err = app.db.QueryRowContext(ctx,
		`SELECT next_run_at, COALESCE(last_run_at,'') FROM channel_schedules WHERE id=?`,
		"site-tick-busy").Scan(&nextRunAt, &lastRunAt)
	if err != nil {
		t.Fatalf("query schedule: %v", err)
	}
	if nextRunAt != pastTime {
		t.Fatalf("expected busy schedule next_run_at to remain %s, got %s", pastTime, nextRunAt)
	}
	if lastRunAt != "" {
		t.Fatalf("expected busy schedule not to set last_run_at, got %s", lastRunAt)
	}
}

func TestTickChannelScheduler_SkipsGlobalScheduleRecord(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	ctx := context.Background()
	pastTime := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	_, err := app.db.ExecContext(ctx, `
		UPDATE channel_schedules
		SET enabled=1, next_run_at=?, last_run_at=NULL
		WHERE id=?
	`, pastTime, globalScheduleSiteID)
	if err != nil {
		t.Fatalf("update global schedule: %v", err)
	}

	app.tickChannelScheduler(ctx, time.Now())

	var lastRunAt string
	err = app.db.QueryRowContext(ctx,
		`SELECT COALESCE(last_run_at,'') FROM channel_schedules WHERE id=?`,
		globalScheduleSiteID,
	).Scan(&lastRunAt)
	if err != nil {
		t.Fatalf("query global last_run_at: %v", err)
	}
	if lastRunAt != "" {
		t.Fatalf("expected global schedule to be skipped, got last_run_at=%s", lastRunAt)
	}
}

func TestTickChannelScheduler_EmptySchedules_NoOp(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	// This should not panic or error when no schedules exist
	app.tickChannelScheduler(context.Background(), time.Now())
}
