package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"relaycheck-desktop/internal/notifications"
)

func TestGetSystemSettingsMasksNotificationSecrets(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	// Seed encrypted-looking notification config directly into DB.
	raw := notifications.ChannelsConfig{
		Enabled:       true,
		DefaultLevels: []string{"warning", "error"},
		Channels: []notifications.ChannelEntry{
			{
				Type:    "telegram",
				Name:    "ops",
				Enabled: true,
				Config:  notifications.MarshalRaw(notifications.TelegramConfig{BotToken: "v1.super-secret-token", ChatID: "1"}),
			},
		},
	}
	encoded, _ := json.Marshal(raw)
	if _, err := app.db.Exec(`
		INSERT INTO system_settings (id, key, value_json, created_at, updated_at)
		VALUES (?, 'notification.channels', ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value_json=excluded.value_json, updated_at=excluded.updated_at
	`, newID(), string(encoded), now(), now()); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/system/settings", nil)
	rec := httptest.NewRecorder()
	app.handleSystemSettings(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "v1.super-secret-token") {
		t.Fatalf("GET settings leaked ciphertext: %s", body)
	}
	if strings.Contains(body, "super-secret-token") {
		t.Fatalf("GET settings leaked secret material: %s", body)
	}
	if !strings.Contains(body, "botTokenConfigured") && !strings.Contains(body, `"botToken":""`) {
		t.Fatalf("expected masked notification config markers, body=%s", body)
	}
}

func TestUpdateSystemSettingsPreservesBlankNotificationSecrets(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	// First save a real secret.
	first := map[string]interface{}{
		"settings": []SystemSetting{{
			Key: "notification.channels",
			ValueJSON: mustJSON(notifications.ChannelsConfig{
				Enabled:       true,
				DefaultLevels: []string{"warning", "error"},
				Channels: []notifications.ChannelEntry{{
					Type:    "telegram",
					Name:    "ops",
					Enabled: true,
					Config:  notifications.MarshalRaw(notifications.TelegramConfig{BotToken: "real-token-value", ChatID: "9"}),
				}},
			}),
		}},
	}
	body1, _ := json.Marshal(first)
	req1 := httptest.NewRequest(http.MethodPut, "/api/system/settings", strings.NewReader(string(body1)))
	rec1 := httptest.NewRecorder()
	app.handleSystemSettings(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first put status %d: %s", rec1.Code, rec1.Body.String())
	}

	var stored string
	if err := app.db.QueryRow(`SELECT value_json FROM system_settings WHERE key='notification.channels'`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stored, "v1.") {
		t.Fatalf("expected encrypted token stored, got %s", stored)
	}

	// Second save with empty botToken should preserve previous ciphertext.
	second := map[string]interface{}{
		"settings": []SystemSetting{{
			Key: "notification.channels",
			ValueJSON: mustJSON(notifications.ChannelsConfig{
				Enabled:       true,
				DefaultLevels: []string{"warning", "error"},
				Channels: []notifications.ChannelEntry{{
					Type:    "telegram",
					Name:    "ops",
					Enabled: true,
					Config:  notifications.MarshalRaw(map[string]interface{}{"botToken": "", "chatId": "9", "botTokenConfigured": true}),
				}},
			}),
		}},
	}
	body2, _ := json.Marshal(second)
	req2 := httptest.NewRequest(http.MethodPut, "/api/system/settings", strings.NewReader(string(body2)))
	rec2 := httptest.NewRecorder()
	app.handleSystemSettings(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("second put status %d: %s", rec2.Code, rec2.Body.String())
	}

	var stored2 string
	if err := app.db.QueryRow(`SELECT value_json FROM system_settings WHERE key='notification.channels'`).Scan(&stored2); err != nil {
		t.Fatal(err)
	}
	if stored2 != stored {
		// Chat id same; secret must still be present as v1.
		if !strings.Contains(stored2, "v1.") {
			t.Fatalf("blank secret overwrite wiped encryption: %s", stored2)
		}
	}
}

func mustJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
