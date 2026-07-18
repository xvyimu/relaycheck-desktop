package core

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthStatusChecksDatabaseAndPaths(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	status := app.healthStatus(httptest.NewRequest(http.MethodGet, "/api/health", nil).Context())
	if status.Status != "degraded" {
		t.Fatalf("expected degraded without scheduler, got %q", status.Status)
	}
	if len(status.Checks) < 4 {
		t.Fatalf("expected health checks, got %#v", status.Checks)
	}
	assertHealthCheck(t, status.Checks, "db", "ok")
	assertHealthCheck(t, status.Checks, "database", "ok")
	assertHealthCheck(t, status.Checks, "data_dir", "ok")
	assertHealthCheck(t, status.Checks, "scheduler", "warning")
}

func TestHealthStatusDoesNotExposeDatabaseDriverErrors(t *testing.T) {
	app := newTestApp(t)
	if err := app.db.Close(); err != nil {
		t.Fatal(err)
	}

	status := app.healthStatus(t.Context())
	for _, check := range status.Checks {
		if check.ID != "db" {
			continue
		}
		if check.Message != "数据库连接失败。" {
			t.Fatalf("unexpected public database health message: %q", check.Message)
		}
		if strings.Contains(strings.ToLower(check.Message), "closed") || strings.Contains(check.Message, "C:\\") {
			t.Fatalf("database health leaked an internal error: %q", check.Message)
		}
		return
	}
	t.Fatal("missing database health check")
}

func assertHealthCheck(t *testing.T, checks []HealthCheck, id string, want string) {
	t.Helper()
	for _, check := range checks {
		if check.ID == id {
			if check.Status != want {
				t.Fatalf("expected %s status %q, got %q", id, want, check.Status)
			}
			return
		}
	}
	t.Fatalf("missing health check %q in %#v", id, checks)
}
