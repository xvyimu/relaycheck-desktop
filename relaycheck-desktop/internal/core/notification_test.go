package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"relaycheck-desktop/internal/notifications"
)

// ==================== 测试辅助 ====================

func enableLocalOutbound(app *App) {
	if app != nil {
		app.mu.Lock()
		app.allowLocalOutbound = true
		app.mu.Unlock()
	}
}

// ==================== DispatchNotification 测试 ====================

func TestDispatchNotification_Disabled(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()
	enableLocalOutbound(app)

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := notifications.ChannelsConfig{
		Enabled: false,
		Channels: []notifications.ChannelEntry{
			{
				Type: "webhook", Name: "test", Enabled: true,
				Config: notifications.MarshalRaw(notifications.WebhookConfig{URL: server.URL, Mode: "all", TimeoutSeconds: 5}),
			},
		},
	}
	app.notificationHub.SetConfig(cfg)

	app.dispatchNotification("test_kind", "warning", "标题", "内容")
	time.Sleep(100 * time.Millisecond)

	if callCount != 0 {
		t.Fatalf("expected 0 calls when disabled, got %d", callCount)
	}
}

func TestDispatchNotification_LevelFilter(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()
	enableLocalOutbound(app)

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := notifications.ChannelsConfig{
		Enabled: true,
		Channels: []notifications.ChannelEntry{
			{
				Type: "webhook", Name: "test", Enabled: true,
				Config: notifications.MarshalRaw(notifications.WebhookConfig{URL: server.URL, Mode: "all", TimeoutSeconds: 5}),
				Levels: []string{"warning"},
			},
		},
	}
	app.notificationHub.SetConfig(cfg)

	app.dispatchNotification("test_kind", "info", "标题", "内容")
	time.Sleep(100 * time.Millisecond)

	if callCount != 0 {
		t.Fatalf("expected 0 calls for info level, got %d", callCount)
	}

	app.dispatchNotification("test_kind", "warning", "标题", "内容")
	time.Sleep(100 * time.Millisecond)

	if callCount != 1 {
		t.Fatalf("expected 1 call for warning level, got %d", callCount)
	}
}

func TestDispatchNotification_ModeFilter(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()
	enableLocalOutbound(app)

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := notifications.ChannelsConfig{
		Enabled: true,
		Channels: []notifications.ChannelEntry{
			{
				Type: "webhook", Name: "test", Enabled: true,
				Config: notifications.MarshalRaw(notifications.WebhookConfig{URL: server.URL, Mode: "all", TimeoutSeconds: 5}),
				Levels: []string{"warning", "error"},
			},
		},
	}
	app.notificationHub.SetConfig(cfg)

	app.dispatchNotification("test_kind", "warning", "标题", "内容")
	time.Sleep(100 * time.Millisecond)

	if callCount == 0 {
		t.Fatal("expected at least one HTTP call for warning level")
	}
}

// ==================== ReloadNotificationConfig 测试 ====================

func TestReloadNotificationConfig_MissingRow(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	_, _ = app.db.Exec(`DELETE FROM system_settings WHERE key = 'notification.channels'`)

	if err := app.reloadNotificationConfig(context.Background()); err != nil {
		t.Fatalf("reload failed: %v", err)
	}

	config := app.currentNotificationChannelsConfig()
	if config.Enabled {
		t.Fatal("expected default config with enabled=false")
	}
}

func TestReloadNotificationConfig_DecryptFailureFallback(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	badEncrypted := `{"enabled":true,"defaultLevels":["warning"],"channels":[{"type":"webhook","name":"bad","enabled":true,"config":{"url":"https://example.com/hook","hmacSecret":"v1.badbadbad","mode":"all","timeoutSeconds":10}}]}`
	_, _ = app.db.Exec(`INSERT OR REPLACE INTO system_settings (id, key, value_json, created_at, updated_at) VALUES (?, 'notification.channels', ?, ?, ?)`,
		newID(), badEncrypted, now(), now())

	if err := app.reloadNotificationConfig(context.Background()); err != nil {
		t.Fatalf("reload should not fail on decrypt error: %v", err)
	}
	config := app.currentNotificationChannelsConfig()
	if !config.Enabled {
		t.Fatal("config should still be enabled")
	}
}

// ==================== 加密解密测试 ====================

func TestEncryptDecryptChannelEntrySecrets(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	entries := []struct {
		name         string
		channelType  string
		plainField   string
		buildConfig  func(string) json.RawMessage
		checkEncrypt func(*testing.T, *notifications.ChannelEntry)
		checkDecrypt func(*testing.T, *notifications.ChannelEntry)
	}{
		{
			name:        "webhook hmacSecret",
			channelType: "webhook",
			plainField:  "my-hmac-secret",
			buildConfig: func(secret string) json.RawMessage {
				return notifications.MarshalRaw(notifications.WebhookConfig{HMACSecret: secret})
			},
			checkEncrypt: func(t *testing.T, e *notifications.ChannelEntry) {
				var cfg notifications.WebhookConfig
				json.Unmarshal(e.Config, &cfg)
				if cfg.HMACSecret == "" || !strings.HasPrefix(cfg.HMACSecret, "v1.") {
					t.Fatalf("HMACSecret should be encrypted, got: %s", cfg.HMACSecret)
				}
			},
			checkDecrypt: func(t *testing.T, e *notifications.ChannelEntry) {
				var cfg notifications.WebhookConfig
				json.Unmarshal(e.Config, &cfg)
				if cfg.HMACSecret != "my-hmac-secret" {
					t.Fatalf("HMACSecret should be decrypted, got: %s", cfg.HMACSecret)
				}
			},
		},
		{
			name:        "telegram botToken",
			channelType: "telegram",
			plainField:  "123456:ABC-DEF",
			buildConfig: func(token string) json.RawMessage {
				return notifications.MarshalRaw(notifications.TelegramConfig{BotToken: token})
			},
			checkEncrypt: func(t *testing.T, e *notifications.ChannelEntry) {
				var cfg notifications.TelegramConfig
				json.Unmarshal(e.Config, &cfg)
				if cfg.BotToken == "" || !strings.HasPrefix(cfg.BotToken, "v1.") {
					t.Fatalf("BotToken should be encrypted, got: %s", cfg.BotToken)
				}
			},
			checkDecrypt: func(t *testing.T, e *notifications.ChannelEntry) {
				var cfg notifications.TelegramConfig
				json.Unmarshal(e.Config, &cfg)
				if cfg.BotToken != "123456:ABC-DEF" {
					t.Fatalf("BotToken should be decrypted, got: %s", cfg.BotToken)
				}
			},
		},
		{
			name:        "serverchan sendKey",
			channelType: "serverchan",
			plainField:  "SCT123456Key",
			buildConfig: func(key string) json.RawMessage {
				return notifications.MarshalRaw(notifications.ServerChanConfig{SendKey: key})
			},
			checkEncrypt: func(t *testing.T, e *notifications.ChannelEntry) {
				var cfg notifications.ServerChanConfig
				json.Unmarshal(e.Config, &cfg)
				if cfg.SendKey == "" || !strings.HasPrefix(cfg.SendKey, "v1.") {
					t.Fatalf("SendKey should be encrypted, got: %s", cfg.SendKey)
				}
			},
			checkDecrypt: func(t *testing.T, e *notifications.ChannelEntry) {
				var cfg notifications.ServerChanConfig
				json.Unmarshal(e.Config, &cfg)
				if cfg.SendKey != "SCT123456Key" {
					t.Fatalf("SendKey should be decrypted, got: %s", cfg.SendKey)
				}
			},
		},
		{
			name:        "email password",
			channelType: "email",
			plainField:  "smtp-pass-2026",
			buildConfig: func(pwd string) json.RawMessage {
				return notifications.MarshalRaw(notifications.EmailConfig{Password: pwd})
			},
			checkEncrypt: func(t *testing.T, e *notifications.ChannelEntry) {
				var cfg notifications.EmailConfig
				json.Unmarshal(e.Config, &cfg)
				if cfg.Password == "" || !strings.HasPrefix(cfg.Password, "v1.") {
					t.Fatalf("Password should be encrypted, got: %s", cfg.Password)
				}
			},
			checkDecrypt: func(t *testing.T, e *notifications.ChannelEntry) {
				var cfg notifications.EmailConfig
				json.Unmarshal(e.Config, &cfg)
				if cfg.Password != "smtp-pass-2026" {
					t.Fatalf("Password should be decrypted, got: %s", cfg.Password)
				}
			},
		},
	}

	for _, tc := range entries {
		t.Run(tc.name, func(t *testing.T) {
			entry := &notifications.ChannelEntry{
				Type:   tc.channelType,
				Config: tc.buildConfig(tc.plainField),
			}
			if err := app.encryptChannelEntrySecrets(entry); err != nil {
				t.Fatalf("encrypt failed: %v", err)
			}
			tc.checkEncrypt(t, entry)

			if err := app.decryptChannelEntrySecrets(entry); err != nil {
				t.Fatalf("decrypt failed: %v", err)
			}
			tc.checkDecrypt(t, entry)
		})
	}
}

// ==================== BuildChannelFromConfig 测试 ====================

func TestBuildChannelFromConfig_Webhook(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	entry := notifications.ChannelEntry{
		Type:    "webhook",
		Name:    "test",
		Enabled: true,
		Config:  notifications.MarshalRaw(notifications.WebhookConfig{URL: "https://example.com/hook", Mode: "all", TimeoutSeconds: 10}),
	}
	ch := app.buildChannelFromConfig(entry)
	if ch == nil {
		t.Fatal("expected non-nil channel")
	}
	if ch.Type() != "webhook" {
		t.Fatalf("expected type webhook, got %s", ch.Type())
	}
}

func TestBuildChannelFromConfig_InvalidConfig(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	entry := notifications.ChannelEntry{
		Type:    "webhook",
		Name:    "invalid",
		Enabled: true,
		Config:  notifications.MarshalRaw(notifications.WebhookConfig{URL: "", Mode: "all"}),
	}
	ch := app.buildChannelFromConfig(entry)
	if ch != nil {
		t.Fatal("expected nil for invalid config")
	}
}

func TestBuildChannelFromConfig_EmptyConfig(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	entry := notifications.ChannelEntry{
		Type:    "webhook",
		Name:    "empty",
		Enabled: true,
		Config:  nil,
	}
	ch := app.buildChannelFromConfig(entry)
	if ch != nil {
		t.Fatal("expected nil for nil Config")
	}
}

func TestBuildChannelFromConfig_UnknownType(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	entry := notifications.ChannelEntry{
		Type:    "unknown_type",
		Name:    "test",
		Enabled: true,
		Config:  notifications.MarshalRaw(map[string]string{"key": "val"}),
	}
	ch := app.buildChannelFromConfig(entry)
	if ch != nil {
		t.Fatal("expected nil for unknown type")
	}
}

// ==================== HealthCheckNotificationChannels test ====================

func TestHealthCheckNotificationChannels(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	// Disabled config
	app.notificationHub.SetConfig(notifications.ChannelsConfig{Enabled: false})

	check := app.healthCheckNotificationChannels()
	if check.Status != "ok" {
		t.Fatalf("expected ok for disabled config, got %s: %s", check.Status, check.Message)
	}

	// Enabled with no channels
	app.notificationHub.SetConfig(notifications.ChannelsConfig{Enabled: true})
	check = app.healthCheckNotificationChannels()
	if check.Status != "warning" {
		t.Fatalf("expected warning for enabled but no channels, got %s", check.Status)
	}

	// Enabled with channel but all disabled
	app.notificationHub.SetConfig(notifications.ChannelsConfig{
		Enabled: true,
		Channels: []notifications.ChannelEntry{
			{Type: "webhook", Name: "w1", Enabled: false},
		},
	})
	check = app.healthCheckNotificationChannels()
	if check.Status != "warning" {
		t.Fatalf("expected warning for all channels disabled, got %s", check.Status)
	}

	// Enabled with some enabled channels
	app.notificationHub.SetConfig(notifications.ChannelsConfig{
		Enabled: true,
		Channels: []notifications.ChannelEntry{
			{Type: "webhook", Name: "w1", Enabled: true},
			{Type: "bark", Name: "b1", Enabled: false},
		},
	})
	check = app.healthCheckNotificationChannels()
	if check.Status != "ok" {
		t.Fatalf("expected ok for enabled channels, got %s", check.Status)
	}
}

// ==================== Integration: notify -> dispatchNotification ====================

func TestNotifyTriggersDispatch(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()
	enableLocalOutbound(app)

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := notifications.ChannelsConfig{
		Enabled: true,
		Channels: []notifications.ChannelEntry{
			{
				Type: "webhook", Name: "test", Enabled: true,
				Config: notifications.MarshalRaw(notifications.WebhookConfig{URL: server.URL, Mode: "all", TimeoutSeconds: 5}),
				Levels: []string{"warning"},
			},
		},
	}
	app.notificationHub.SetConfig(cfg)

	// notify uses go routine for dispatch, so wait a bit
	app.notify("checkin_failed", "warning", "签到失败", "详情内容", "account", "id123")
	time.Sleep(200 * time.Millisecond)

	if callCount == 0 {
		t.Fatal("expected dispatch to trigger at least one HTTP call")
	}
}

// ==================== ensureDefaultSettings includes notification.channels ====================

func TestEnsureDefaultSettings_IncludesNotificationChannels(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	var count int
	_ = app.db.QueryRow(`SELECT COUNT(*) FROM system_settings WHERE key = 'notification.channels'`).Scan(&count)
	if count == 0 {
		t.Fatal("expected notification.channels to be inserted by ensureDefaultSettings")
	}
}

func TestEnsureDefaultSettings_NotificationChannelNamesAreUTF8(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	cfg, err := app.loadNotificationChannelsConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(cfg.Channels))
	for _, channel := range cfg.Channels {
		names = append(names, channel.Name)
	}

	expected := []string{"默认 Webhook", "SMTP 邮件", "桌面通知"}
	for _, name := range expected {
		found := false
		for _, actual := range names {
			if actual == name {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected default notification channel %q in %v", name, names)
		}
	}
}

func TestEnsureDefaultSettingsIncludesChannelHealthNotificationTypes(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	cfg, err := app.loadNotificationChannelsConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, channel := range cfg.Channels {
		if channel.Type != "webhook" {
			continue
		}
		if notifications.StringInSlice("scheduled_channel_health_probe_failed", channel.Types) && notifications.StringInSlice("scheduled_channel_health_probe_warning", channel.Types) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected default webhook notification types to include scheduled channel health probe alerts: %#v", cfg.Channels)
	}
}
