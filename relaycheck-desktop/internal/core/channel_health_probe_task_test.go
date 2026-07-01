package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestChannelHealthProbeTaskRefreshesSitesAndModels(t *testing.T) {
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
	defer server.Close()

	app := newTestApp(t)
	app.client = server.Client()
	app.allowLocalOutbound = true

	channelKey, err := app.encryptText("sk-channel-health")
	if err != nil {
		t.Fatal(err)
	}
	nowText := now()
	siteID := "site-health-probe"
	channelID := "channel-health-probe"
	if _, err := app.db.Exec(`
		INSERT INTO imported_channels (id, source_channel_id, name, base_url, status, upstream_kind, channel_key_encrypted, raw_json, created_at, updated_at)
		VALUES (?, 'source-health-probe', 'Probe Relay', ?, 'enabled', 'newapi', ?, '{}', ?, ?)
	`, channelID, server.URL, channelKey, nowText, nowText); err != nil {
		t.Fatalf("seed channel: %v", err)
	}
	if _, err := app.db.Exec(`
		INSERT INTO upstream_sites (id, channel_id, name, base_url, kind, health_status, supports_models, created_at, updated_at)
		VALUES (?, ?, 'Probe Relay', ?, 'unknown', 'unknown', 0, ?, ?)
	`, siteID, channelID, server.URL, nowText, nowText); err != nil {
		t.Fatalf("seed site: %v", err)
	}

	taskID := "task-channel-health-probe"
	app.startChannelHealthProbeTask(taskID, map[string]interface{}{"limit": float64(5)})
	progress := waitForTaskDone(t, app, taskID)

	if progress.Type != TaskChannelHealthProbe {
		t.Fatalf("task type = %q, want %q", progress.Type, TaskChannelHealthProbe)
	}
	if progress.Status != TaskStatusDone {
		t.Fatalf("task status = %q, want done: %#v", progress.Status, progress)
	}
	if progress.Total != 1 || progress.Current != 1 {
		t.Fatalf("progress = %d/%d, want 1/1", progress.Current, progress.Total)
	}
	if progress.Results[0].Status != "warning" {
		t.Fatalf("result status = %q, want warning: %#v", progress.Results[0].Status, progress.Results[0])
	}

	site, err := app.loadSiteDetail(context.Background(), siteID)
	if err != nil {
		t.Fatalf("load site detail: %v", err)
	}
	if site.Site.Kind != "newapi" {
		t.Fatalf("site kind = %q, want newapi", site.Site.Kind)
	}
	if site.Site.HealthStatus != "auth_required" {
		t.Fatalf("site health = %q, want auth_required", site.Site.HealthStatus)
	}

	channel, err := app.loadChannelByID(context.Background(), channelID)
	if err != nil {
		t.Fatalf("load channel: %v", err)
	}
	if channel.ModelsStatus != "key_invalid" {
		t.Fatalf("models status = %q, want key_invalid", channel.ModelsStatus)
	}

	overview, err := app.channelHealthOverview(httptest.NewRequest("GET", "/api/channels/health/overview", nil))
	if err != nil {
		t.Fatalf("channelHealthOverview: %v", err)
	}
	if overview.FailedModelChannelCount != 1 || overview.Overall != "warning" {
		t.Fatalf("overview = %#v, want one failed model channel warning", overview)
	}

	center, err := app.buildActionCenter(httptest.NewRequest("GET", "/api/system/action-center", nil))
	if err != nil {
		t.Fatalf("buildActionCenter: %v", err)
	}
	item := findActionItem(t, center.Items, "channel-health-risks")
	if item.Count != 1 {
		t.Fatalf("action item count = %d, want 1", item.Count)
	}
}

func waitForTaskDone(t *testing.T, app *App, taskID string) TaskProgress {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		task := app.taskRunner.get(taskID)
		if task != nil {
			progress := task.snapshot()
			if progress.Status != TaskStatusRunning {
				return progress
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	task := app.taskRunner.get(taskID)
	if task == nil {
		t.Fatalf("task %q was not registered", taskID)
	}
	t.Fatalf("task %q did not finish: %#v", taskID, task.snapshot())
	return TaskProgress{}
}
