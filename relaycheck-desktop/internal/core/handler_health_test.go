package core

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleHealthReturnsOK(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	app.handleHealth(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, `"status"`) {
		t.Fatalf("expected health status to contain \"status\" field, got: %s", body)
	}
	if !strings.Contains(body, `"checks"`) {
		t.Fatalf("expected health status to contain \"checks\" field, got: %s", body)
	}
}

func TestHandleHealthWrongMethod(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/health", nil)
	app.handleHealth(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for POST, got %d", rec.Code)
	}
}

func TestHandleChannelsEmpty(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/channels", nil)
	app.handleChannels(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, `"ok"`) {
		t.Fatalf("expected response with \"ok\" field, got: %s", body)
	}
}

func TestHandleAuditLogEmpty(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/system/audit-log", nil)
	app.handleAuditLog(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), `[`) {
		t.Fatalf("expected empty JSON array, got: %s", rec.Body.String())
	}
}

func TestHandleSystemDiagnostics(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/system/diagnostics", nil)
	app.handleSystemDiagnostics(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, `"items"`) {
		t.Fatalf("expected diagnostics to contain \"items\" field, got: %s", body)
	}
	if !strings.Contains(body, `"generatedAt"`) {
		t.Fatalf("expected diagnostics to contain \"generatedAt\" field, got: %s", body)
	}
}

func TestHandleSystemDiagnosticsWrongMethod(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/system/diagnostics", nil)
	app.handleSystemDiagnostics(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for POST, got %d", rec.Code)
	}
}

func TestHandleNotificationsEmpty(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/notifications", nil)
	app.handleNotifications(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"data"`) {
		t.Fatalf("expected response with \"data\" field, got: %s", rec.Body.String())
	}
}

func TestHandleModelPricingWrongMethod(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/models/pricing", nil)
	app.handleModelPricing(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for POST, got %d", rec.Code)
	}
}

func TestHandleModelPricingSyncWrongMethod(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/models/pricing/sync", nil)
	app.handleModelPricingSync(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for GET, got %d", rec.Code)
	}
}

func TestHandleUsageOverviewEmpty(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/usage/overview", nil)
	app.handleUsageOverview(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"ok"`) {
		t.Fatalf("expected response with \"ok\" field, got: %s", rec.Body.String())
	}
}

func TestHandleSystemVersionCheckWrongMethod(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/system/version-check", nil)
	app.handleVersionCheck(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for POST, got %d", rec.Code)
	}
}

func TestHandleSystemAutoStartWrongMethod(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/system/autostart", nil)
	app.handleSystemAutoStart(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for POST, got %d", rec.Code)
	}
}

func TestHandleSystemAutostartStatus(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/system/autostart", nil)
	app.handleSystemAutoStart(rec, req)
	if rec.Code != http.StatusOK && rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 200 or 500 for GET, got %d", rec.Code)
	}
	if rec.Code == http.StatusOK {
		if !strings.Contains(rec.Body.String(), `"enabled"`) {
			t.Fatalf("expected autostart status to contain \"enabled\" field, got: %s", rec.Body.String())
		}
	}
}

func TestHandleBalanceSnapshotsEmpty(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/balances/snapshots", nil)
	app.handleBalanceSnapshots(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `[`) {
		t.Fatalf("expected empty JSON array, got: %s", rec.Body.String())
	}
}

func TestHandleActionCenterWrongMethod(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/system/action-center", nil)
	app.handleActionCenter(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for POST, got %d", rec.Code)
	}
}

func TestHandleSystemSettingsUpdateInvalidJSON(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/system/settings",
		strings.NewReader(`not json`))
	req.Header.Set("content-type", "application/json")
	app.handleSystemSettings(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid JSON, got %d", rec.Code)
	}
}
