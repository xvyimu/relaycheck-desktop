package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSiteTaskServiceStartDetectSitesPublishesProgress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/about":
			_, _ = w.Write([]byte(`{"success":true,"data":{"system_name":"New API","version":"1.0.0"}}`))
		case "/":
			_, _ = w.Write([]byte(`<html><body><a href="/login">login</a></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	app := newTestApp(t)
	app.client = server.Client()
	app.allowLocalOutbound = true

	siteID := "site-task-detect"
	if _, err := app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, created_at, updated_at)
		VALUES (?, 'Task Detect Relay', ?, 'unknown', 'unknown', ?, ?)
	`, siteID, server.URL, now(), now()); err != nil {
		t.Fatal(err)
	}

	app.siteTasks.StartDetectSites("task-detect-sites-service", map[string]interface{}{
		"limit":               float64(5),
		"onlyUnknownOrOpenAI": true,
	})
	progress := waitForTaskDone(t, app, "task-detect-sites-service")

	if progress.Type != TaskDetectSites {
		t.Fatalf("task type = %q, want %q", progress.Type, TaskDetectSites)
	}
	if progress.Status != TaskStatusDone {
		t.Fatalf("task status = %q, want done: %#v", progress.Status, progress)
	}
	if progress.Total != 1 || progress.Current != 1 {
		t.Fatalf("progress = %d/%d, want 1/1", progress.Current, progress.Total)
	}
	if len(progress.Results) != 1 {
		t.Fatalf("result count = %d, want 1", len(progress.Results))
	}
	result := progress.Results[0]
	if result.ID != siteID || result.Name != "Task Detect Relay" || result.Status != "success" {
		t.Fatalf("unexpected task result: %#v", result)
	}
	if result.Message != "newapi" {
		t.Fatalf("task message = %q, want detected kind newapi", result.Message)
	}

	site, err := app.loadSiteDetail(context.Background(), siteID)
	if err != nil {
		t.Fatalf("load site detail: %v", err)
	}
	if site.Site.Kind != "newapi" {
		t.Fatalf("site kind = %q, want newapi", site.Site.Kind)
	}
}
