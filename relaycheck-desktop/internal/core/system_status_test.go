package core

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSystemStatusUsesDesktopProductIdentity(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	status, err := app.systemStatus(httptest.NewRequest("GET", "/api/system/status", nil))
	if err != nil {
		t.Fatal(err)
	}

	if status.ProductName != "RelayCheck Desktop" {
		t.Fatalf("expected desktop product name, got %q", status.ProductName)
	}
	if status.ProductVersion == "" || status.BuildTime == "" {
		t.Fatalf("expected version and build time, got version=%q build=%q", status.ProductVersion, status.BuildTime)
	}
	if status.LastDiagnostics.Overall == "" || status.LastDiagnostics.ItemCount == 0 {
		t.Fatalf("expected diagnostics summary, got %#v", status.LastDiagnostics)
	}
}

func TestUpdateSystemSettingsNormalizesChannelHealthSchedule(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	body, _ := json.Marshal(map[string]interface{}{
		"settings": []SystemSetting{{
			Key:       "channel.health.schedule",
			ValueJSON: `{"enabled":true,"intervalMinutes":1,"runOnStartup":true,"limit":999,"onlyRisky":true}`,
		}},
	})
	req := httptest.NewRequest(http.MethodPut, "/api/system/settings", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	app.handleSystemSettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected OK, got %d: %s", rec.Code, rec.Body.String())
	}

	config := app.loadChannelHealthScheduleConfig(req.Context())
	if config.IntervalMinutes != 30 {
		t.Fatalf("intervalMinutes = %d, want 30", config.IntervalMinutes)
	}
	if config.Limit != 50 {
		t.Fatalf("limit = %d, want 50", config.Limit)
	}
	if !config.RunOnStartup || !config.OnlyRisky {
		t.Fatalf("expected boolean options to persist, got %#v", config)
	}
}
