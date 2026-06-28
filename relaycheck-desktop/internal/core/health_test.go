package core

import (
	"net/http"
	"net/http/httptest"
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
