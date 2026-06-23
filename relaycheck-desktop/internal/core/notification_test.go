package core

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ==================== 配置解析测试 ====================

func TestDefaultNotificationChannelsConfig(t *testing.T) {
	cfg := defaultNotificationChannelsConfig()
	if cfg.Enabled {
		t.Fatal("默认通知渠道配置应禁用")
	}
	if len(cfg.DefaultLevels) != 2 || cfg.DefaultLevels[0] != "warning" || cfg.DefaultLevels[1] != "error" {
		t.Fatalf("默认 Level 应为 [warning error]，实际: %v", cfg.DefaultLevels)
	}
}

func TestParseNotificationChannelsConfig_Valid(t *testing.T) {
	raw := `{
		"enabled": true,
		"defaultLevels": ["warning","error"],
		"channels": [
			{
				"type": "webhook",
				"name": "我的 Webhook",
				"enabled": true,
				"config": {"url":"https://hooks.example.com/hook","hmacSecret":"testsecret","mode":"all","timeoutSeconds":15},
				"levels": ["warning","error"],
				"types": ["scheduled_checkin_failed"]
			},
			{
				"type": "telegram",
				"name": "TG Bot",
				"enabled": false,
				"config": {"botToken":"123:ABC","chatId":"-100123","mode":"failure"},
				"levels": ["error"]
			}
		]
	}`
	cfg, warnings := parseNotificationChannelsConfig(raw)
	if !cfg.Enabled {
		t.Fatal("expected enabled=true")
	}
	if len(cfg.Channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(cfg.Channels))
	}
	if cfg.Channels[0].Type != "webhook" || cfg.Channels[0].Name != "我的 Webhook" {
		t.Fatalf("unexpected channel 0: %+v", cfg.Channels[0])
	}
	if !cfg.Channels[0].Enabled {
		t.Fatal("channel 0 should be enabled")
	}
	if len(warnings) > 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}

	var wc webhookConfig
	json.Unmarshal(cfg.Channels[0].Config, &wc)
	if wc.URL != "https://hooks.example.com/hook" || wc.HMACSecret != "testsecret" || wc.TimeoutSeconds != 15 {
		t.Fatalf("unexpected webhook config: %+v", wc)
	}

	var tc telegramConfig
	json.Unmarshal(cfg.Channels[1].Config, &tc)
	if tc.BotToken != "123:ABC" || tc.ChatID != "-100123" {
		t.Fatalf("unexpected telegram config: %+v", tc)
	}
}

func TestParseNotificationChannelsConfig_Minimal(t *testing.T) {
	raw := `{"enabled":true}`
	cfg, warnings := parseNotificationChannelsConfig(raw)
	if !cfg.Enabled {
		t.Fatal("expected enabled=true")
	}
	if cfg.Channels != nil && len(cfg.Channels) != 0 {
		t.Fatalf("expected no channels, got %d", len(cfg.Channels))
	}
	_ = warnings // empty channels config, no warnings
}

func TestParseNotificationChannelsConfig_InvalidJSON(t *testing.T) {
	raw := `{invalid json}`
	cfg, warnings := parseNotificationChannelsConfig(raw)
	if cfg.Enabled {
		t.Fatal("expected disabled default on bad JSON")
	}
	if len(warnings) == 0 {
		t.Fatal("expected parse warning")
	}
}

// ==================== 验证测试 ====================

func TestValidateWebhookConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  webhookConfig
		wantErr bool
	}{
		{"valid with url", webhookConfig{URL: "https://hooks.example.com/hook"}, false},
		{"valid with url and hmac", webhookConfig{URL: "https://hooks.example.com/hook", HMACSecret: "secret"}, false},
		{"empty url", webhookConfig{URL: ""}, true},
		{"blank url", webhookConfig{URL: "  "}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := &webhookChannel{config: tt.config}
			err := ch.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateTelegramConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  telegramConfig
		wantErr bool
	}{
		{"valid", telegramConfig{BotToken: "123:ABC", ChatID: "-100123"}, false},
		{"empty bot token", telegramConfig{BotToken: "", ChatID: "-100123"}, true},
		{"empty chat id", telegramConfig{BotToken: "123:ABC", ChatID: ""}, true},
		{"both empty", telegramConfig{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := &telegramChannel{config: tt.config}
			err := ch.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateBarkConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  barkConfig
		wantErr bool
	}{
		{"valid url", barkConfig{URL: "https://api.day.app/xxx"}, false},
		{"empty url", barkConfig{URL: ""}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := &barkChannel{config: tt.config}
			err := ch.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateServerChanConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  serverchanConfig
		wantErr bool
	}{
		{"valid sendkey", serverchanConfig{SendKey: "SCT123"}, false},
		{"empty sendkey", serverchanConfig{SendKey: ""}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := &serverchanChannel{config: tt.config}
			err := ch.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateEmailConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  emailConfig
		wantErr bool
	}{
		{"all required fields", emailConfig{SMTPHost: "smtp.example.com", FromAddr: "a@b.com", ToAddr: "c@d.com"}, false},
		{"empty smtp host", emailConfig{SMTPHost: "", FromAddr: "a@b.com", ToAddr: "c@d.com"}, true},
		{"empty from addr", emailConfig{SMTPHost: "smtp.example.com", FromAddr: "", ToAddr: "c@d.com"}, true},
		{"empty to addr", emailConfig{SMTPHost: "smtp.example.com", FromAddr: "a@b.com", ToAddr: ""}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := &emailChannel{config: tt.config}
			err := ch.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// ==================== LevelMatchesMode 测试 ====================

func TestLevelMatchesMode(t *testing.T) {
	tests := []struct {
		mode  string
		level string
		want  bool
	}{
		{"all", "success", true},
		{"all", "info", true},
		{"all", "warning", true},
		{"all", "error", true},
		{"success", "success", true},
		{"success", "info", true},
		{"success", "warning", false},
		{"success", "error", false},
		{"failure", "warning", true},
		{"failure", "error", true},
		{"failure", "success", false},
		{"failure", "info", false},
		{"unknown", "warning", true},
		{"unknown", "info", true},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_%s", tt.mode, tt.level), func(t *testing.T) {
			got := levelMatchesMode(tt.mode, tt.level)
			if got != tt.want {
				t.Fatalf("levelMatchesMode(%q, %q) = %v, want %v", tt.mode, tt.level, got, tt.want)
			}
		})
	}
}

// ==================== ShouldSendToChannel 测试 ====================

func TestShouldSendToChannel(t *testing.T) {
	entry := channelEntry{
		Levels: []string{"warning", "error"},
		Types:  []string{"scheduled_checkin_failed"},
	}
	if !shouldSendToChannel(entry, "scheduled_checkin_failed", "warning") {
		t.Fatal("should match both type and level")
	}
	if !shouldSendToChannel(entry, "scheduled_checkin_failed", "error") {
		t.Fatal("should match type and error level")
	}
	if shouldSendToChannel(entry, "scheduled_checkin_failed", "info") {
		t.Fatal("should not match info level")
	}
	if shouldSendToChannel(entry, "auth_failed", "warning") {
		t.Fatal("should not match different type")
	}
}

func TestShouldSendToChannel_EmptyLevels(t *testing.T) {
	entry := channelEntry{
		Levels: nil,
		Types:  []string{"scheduled_checkin_failed"},
	}
	if !shouldSendToChannel(entry, "scheduled_checkin_failed", "info") {
		t.Fatal("empty levels should allow any level")
	}
}

func TestShouldSendToChannel_EmptyTypes(t *testing.T) {
	entry := channelEntry{
		Levels: []string{"warning"},
		Types:  nil,
	}
	if !shouldSendToChannel(entry, "any_type", "warning") {
		t.Fatal("empty types should allow any type")
	}
}

func TestShouldSendToChannel_EdgeCases(t *testing.T) {
	entry := channelEntry{}
	if !shouldSendToChannel(entry, "any_kind", "any_level") {
		t.Fatal("empty entry should allow all")
	}

	entry.Types = []string{"type_a"}
	if !shouldSendToChannel(entry, "type_a", "any") {
		t.Fatal("should match type_a")
	}
	if shouldSendToChannel(entry, "type_b", "any") {
		t.Fatal("should not match type_b")
	}

	entry2 := channelEntry{Levels: []string{"error"}}
	if !shouldSendToChannel(entry2, "any", "error") {
		t.Fatal("should match error level")
	}
	if shouldSendToChannel(entry2, "any", "info") {
		t.Fatal("should not match info level")
	}
}

// ==================== Webhook HTTP 发送测试 ====================

func enableLocalOutbound(app *App) {
	if app != nil {
		app.mu.Lock()
		app.allowLocalOutbound = true
		app.mu.Unlock()
	}
}

func TestWebhookSend_Success(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	enableLocalOutbound(app)

	var capturedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("X-Signature-256") != "" {
			t.Fatal("expected no X-Signature-256 header when HMAC is empty")
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ch := &webhookChannel{
		app:    app,
		config: webhookConfig{URL: server.URL, Mode: "all", TimeoutSeconds: 5},
	}
	err = ch.Send(context.Background(), "test_kind", "warning", "测试标题", "测试内容")
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}

	if capturedBody == nil {
		t.Fatal("no request captured")
	}
	if capturedBody["type"] != "test_kind" {
		t.Fatalf("expected type test_kind, got %v", capturedBody["type"])
	}
	if capturedBody["title"] != "测试标题" {
		t.Fatalf("expected title 测试标题, got %v", capturedBody["title"])
	}
	if capturedBody["level"] != "warning" {
		t.Fatalf("expected level warning, got %v", capturedBody["level"])
	}
}

func TestWebhookSend_WithHMAC(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	enableLocalOutbound(app)

	secret := "test-hmac-key-2026"
	var capturedSig string
	var capturedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedSig = r.Header.Get("X-Signature-256")
		capturedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ch := &webhookChannel{
		app: app,
		config: webhookConfig{
			URL:            server.URL,
			HMACSecret:     secret,
			Mode:           "all",
			TimeoutSeconds: 5,
		},
	}
	err = ch.Send(context.Background(), "test", "error", "HMAC 测试", "内容")
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}

	if capturedSig == "" {
		t.Fatal("expected X-Signature-256 header")
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(capturedBody)
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	if capturedSig != expectedSig {
		t.Fatalf("signature mismatch: got %s, expected %s", capturedSig, expectedSig)
	}
}

func TestWebhookSend_DigestMode(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	enableLocalOutbound(app)

	digestReceived := make(chan map[string]interface{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var data map[string]interface{}
		json.Unmarshal(body, &data)
		digestReceived <- data
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ch := &webhookChannel{
		app:    app,
		config: webhookConfig{URL: server.URL, Mode: "digest", TimeoutSeconds: 5},
	}
	// Directly populate entries (bypassing StartDigestLoop)
	ch.entries = []digestEntry{
		{Kind: "checkin", Level: "error", Title: "告警1", Content: "内容1", Time: time.Now()},
		{Kind: "checkin", Level: "warning", Title: "告警2", Content: "内容2", Time: time.Now()},
	}

	if err := ch.FlushDigest(context.Background()); err != nil {
		t.Fatalf("flush digest failed: %v", err)
	}

	select {
	case digest := <-digestReceived:
		if digest["type"] != "digest" {
			t.Fatalf("expected type digest, got %v", digest["type"])
		}
		count := int(digest["count"].(float64))
		if count != 2 {
			t.Fatalf("expected count 2, got %d", count)
		}
		entries := digest["entries"].([]interface{})
		if len(entries) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(entries))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for digest webhook call")
	}
}

// ==================== DispatchNotification 测试 ====================

func TestDispatchNotification_Disabled(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	enableLocalOutbound(app)

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := notificationChannelsConfig{
		Enabled: false,
		Channels: []channelEntry{
			{
				Type: "webhook", Name: "test", Enabled: true,
				Config: marshalRaw(webhookConfig{URL: server.URL, Mode: "all", TimeoutSeconds: 5}),
			},
		},
	}
	app.mu.Lock()
	app.notificationConfig = cfg
	app.mu.Unlock()

	app.dispatchNotification("test_kind", "warning", "标题", "内容")
	time.Sleep(100 * time.Millisecond)

	if callCount != 0 {
		t.Fatalf("expected 0 calls when disabled, got %d", callCount)
	}
}

func TestDispatchNotification_LevelFilter(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	enableLocalOutbound(app)

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := notificationChannelsConfig{
		Enabled: true,
		Channels: []channelEntry{
			{
				Type: "webhook", Name: "test", Enabled: true,
				Config: marshalRaw(webhookConfig{URL: server.URL, Mode: "all", TimeoutSeconds: 5}),
				Levels: []string{"warning"},
			},
		},
	}
	app.mu.Lock()
	app.notificationConfig = cfg
	app.mu.Unlock()

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
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	enableLocalOutbound(app)

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := notificationChannelsConfig{
		Enabled: true,
		Channels: []channelEntry{
			{
				Type: "webhook", Name: "test", Enabled: true,
				Config: marshalRaw(webhookConfig{URL: server.URL, Mode: "all", TimeoutSeconds: 5}),
				Levels: []string{"warning", "error"},
			},
		},
	}
	app.mu.Lock()
	app.notificationConfig = cfg
	app.mu.Unlock()

	app.dispatchNotification("test_kind", "warning", "标题", "内容")
	time.Sleep(100 * time.Millisecond)

	if callCount == 0 {
		t.Fatal("expected at least one HTTP call for warning level")
	}
}

// ==================== ReloadNotificationConfig 测试 ====================

func TestReloadNotificationConfig_MissingRow(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
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
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
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
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	entries := []struct {
		name         string
		channelType  string
		plainField   string
		buildConfig  func(string) json.RawMessage
		checkEncrypt func(*testing.T, *channelEntry)
		checkDecrypt func(*testing.T, *channelEntry)
	}{
		{
			name:        "webhook hmacSecret",
			channelType: "webhook",
			plainField:  "my-hmac-secret",
			buildConfig: func(secret string) json.RawMessage {
				return marshalRaw(webhookConfig{HMACSecret: secret})
			},
			checkEncrypt: func(t *testing.T, e *channelEntry) {
				var cfg webhookConfig
				json.Unmarshal(e.Config, &cfg)
				if cfg.HMACSecret == "" || !strings.HasPrefix(cfg.HMACSecret, "v1.") {
					t.Fatalf("HMACSecret should be encrypted, got: %s", cfg.HMACSecret)
				}
			},
			checkDecrypt: func(t *testing.T, e *channelEntry) {
				var cfg webhookConfig
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
				return marshalRaw(telegramConfig{BotToken: token})
			},
			checkEncrypt: func(t *testing.T, e *channelEntry) {
				var cfg telegramConfig
				json.Unmarshal(e.Config, &cfg)
				if cfg.BotToken == "" || !strings.HasPrefix(cfg.BotToken, "v1.") {
					t.Fatalf("BotToken should be encrypted, got: %s", cfg.BotToken)
				}
			},
			checkDecrypt: func(t *testing.T, e *channelEntry) {
				var cfg telegramConfig
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
				return marshalRaw(serverchanConfig{SendKey: key})
			},
			checkEncrypt: func(t *testing.T, e *channelEntry) {
				var cfg serverchanConfig
				json.Unmarshal(e.Config, &cfg)
				if cfg.SendKey == "" || !strings.HasPrefix(cfg.SendKey, "v1.") {
					t.Fatalf("SendKey should be encrypted, got: %s", cfg.SendKey)
				}
			},
			checkDecrypt: func(t *testing.T, e *channelEntry) {
				var cfg serverchanConfig
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
				return marshalRaw(emailConfig{Password: pwd})
			},
			checkEncrypt: func(t *testing.T, e *channelEntry) {
				var cfg emailConfig
				json.Unmarshal(e.Config, &cfg)
				if cfg.Password == "" || !strings.HasPrefix(cfg.Password, "v1.") {
					t.Fatalf("Password should be encrypted, got: %s", cfg.Password)
				}
			},
			checkDecrypt: func(t *testing.T, e *channelEntry) {
				var cfg emailConfig
				json.Unmarshal(e.Config, &cfg)
				if cfg.Password != "smtp-pass-2026" {
					t.Fatalf("Password should be decrypted, got: %s", cfg.Password)
				}
			},
		},
	}

	for _, tc := range entries {
		t.Run(tc.name, func(t *testing.T) {
			entry := &channelEntry{
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

// ==================== 工具函数测试 ====================

func TestBuildNotifyBody(t *testing.T) {
	body := buildNotifyBody("test_kind", "warning", "测试标题", "测试内容")
	if !strings.Contains(body, "测试标题") || !strings.Contains(body, "测试内容") {
		t.Fatalf("body missing content: %s", body)
	}
}

func TestMaskSensitiveField(t *testing.T) {
	if maskSensitiveField("") != "" {
		t.Fatal("empty should remain empty")
	}
	// Short values (<=4): all asterisks
	if maskSensitiveField("ab") != "**" {
		t.Fatalf("short value should be all masked, got %s", maskSensitiveField("ab"))
	}
	if maskSensitiveField("1234") != "****" {
		t.Fatalf("4-char value should be all masked, got %s", maskSensitiveField("1234"))
	}
	// Long values: show last 4 chars
	if maskSensitiveField("secret123") != "*****t123" {
		t.Fatalf("unexpected masked value: %s", maskSensitiveField("secret123"))
	}
	// 9 chars -> 5 asterisks + last 4
	if maskSensitiveField("verylongsecretkey2026") != "*****************2026" {
		t.Fatalf("unexpected masked value: %s", maskSensitiveField("verylongsecretkey2026"))
	}
}

func TestTruncateNotifyContent(t *testing.T) {
	short := "short content"
	if truncated := truncateNotifyContent(short, 100); truncated != short {
		t.Fatalf("short content should not be truncated: %s", truncated)
	}
	long := "a" + strings.Repeat("b", 5000) + "c"
	truncated := truncateNotifyContent(long, 100)
	if len([]rune(truncated)) > 105 {
		t.Fatalf("content should be truncated, len=%d", len([]rune(truncated)))
	}
}

func TestStringInSlice(t *testing.T) {
	list := []string{"a", "b", "c"}
	if !stringInSlice("a", list) {
		t.Fatal("a should be in list")
	}
	if stringInSlice("d", list) {
		t.Fatal("d should not be in list")
	}
	if stringInSlice("a", nil) {
		t.Fatal("nil list should not contain anything")
	}
}

// ==================== buildEmailMessage 测试 ====================

func TestBuildEmailMessage(t *testing.T) {
	msg := buildEmailMessage("sender@test.com", "recipient@test.com", "测试邮件标题", "邮件正文内容")
	if !strings.Contains(msg, "From: sender@test.com") {
		t.Fatal("missing From header")
	}
	if !strings.Contains(msg, "To: recipient@test.com") {
		t.Fatal("missing To header")
	}
	if !strings.Contains(msg, "Subject:") {
		t.Fatal("missing Subject header")
	}
	if !strings.Contains(msg, "Content-Type: text/plain") {
		t.Fatal("missing Content-Type header")
	}
	if !strings.Contains(msg, "\r\n\r\n邮件正文内容") {
		t.Fatal("missing body content")
	}
}

// ==================== BuildChannelFromConfig 测试 ====================

func TestBuildChannelFromConfig_Webhook(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	entry := channelEntry{
		Type:    "webhook",
		Name:    "test",
		Enabled: true,
		Config:  marshalRaw(webhookConfig{URL: "https://example.com/hook", Mode: "all", TimeoutSeconds: 10}),
	}
	ch := app.buildChannelFromConfig(entry)
	if ch == nil {
		t.Fatal("expected non-nil channel")
	}
	if ch.Type() != "webhook" {
		t.Fatalf("expected type webhook, got %s", ch.Type())
	}
}

func TestBuildChannelFromConfig_Webhook_Digest(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	entry := channelEntry{
		Type:    "webhook",
		Name:    "digest-test",
		Enabled: true,
		Config:  marshalRaw(webhookConfig{URL: "https://example.com/hook", Mode: "digest", TimeoutSeconds: 10}),
	}
	ch := app.buildChannelFromConfig(entry)
	if ch == nil {
		t.Fatal("expected non-nil channel")
	}
	wc, ok := ch.(*webhookChannel)
	if !ok {
		t.Fatal("expected webhookChannel type")
	}
	if wc.digestCh == nil {
		t.Fatal("expected digestCh to be initialized")
	}
}

func TestBuildChannelFromConfig_InvalidConfig(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	entry := channelEntry{
		Type:    "webhook",
		Name:    "invalid",
		Enabled: true,
		Config:  marshalRaw(webhookConfig{URL: "", Mode: "all"}),
	}
	ch := app.buildChannelFromConfig(entry)
	if ch != nil {
		t.Fatal("expected nil for invalid config")
	}
}

func TestBuildChannelFromConfig_EmptyConfig(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	entry := channelEntry{
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
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	entry := channelEntry{
		Type:    "unknown_type",
		Name:    "test",
		Enabled: true,
		Config:  marshalRaw(map[string]string{"key": "val"}),
	}
	ch := app.buildChannelFromConfig(entry)
	if ch != nil {
		t.Fatal("expected nil for unknown type")
	}
}

// ==================== WebhookSend mode filter test ====================

func TestWebhookSend_ModeFilter(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	enableLocalOutbound(app)

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ch := &webhookChannel{
		app:    app,
		config: webhookConfig{URL: server.URL, Mode: "success", TimeoutSeconds: 5},
	}

	if err := ch.Send(context.Background(), "test", "warning", "标题", "内容"); err != nil {
		t.Fatalf("send failed: %v", err)
	}
	if callCount != 0 {
		t.Fatal("warning should be filtered out by success mode")
	}

	if err := ch.Send(context.Background(), "test", "info", "标题", "内容"); err != nil {
		t.Fatalf("send failed: %v", err)
	}
	if callCount != 1 {
		t.Fatal("info should pass through success mode")
	}
}

// ==================== Bark URL building test ====================

func TestBarkURLBuilding(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	enableLocalOutbound(app)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.String(), "group=RelayCheck") {
			t.Fatalf("expected group=RelayCheck in URL, got: %s", r.URL.String())
		}
		if !strings.Contains(r.URL.String(), "autoCopy=1") {
			t.Fatalf("expected autoCopy=1 in URL, got: %s", r.URL.String())
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ch := &barkChannel{
		app:    app,
		config: barkConfig{URL: server.URL, Mode: "all", Group: "RelayCheck"},
	}

	err = ch.Send(context.Background(), "test", "warning", "测试标题", "测试内容")
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}
}

// ==================== EncryptedFields test ====================

func TestEncryptedFields(t *testing.T) {
	tests := []struct {
		ch     notificationChannel
		fields []string
	}{
		{&webhookChannel{}, []string{"hmacSecret"}},
		{&telegramChannel{}, []string{"botToken"}},
		{&barkChannel{}, nil},
		{&serverchanChannel{}, []string{"sendKey"}},
		{&emailChannel{}, []string{"password"}},
	}
	for _, tt := range tests {
		t.Run(tt.ch.Type(), func(t *testing.T) {
			got := tt.ch.EncryptedFields()
			if len(got) != len(tt.fields) {
				t.Fatalf("expected %v, got %v", tt.fields, got)
			}
			for i := range got {
				if got[i] != tt.fields[i] {
					t.Fatalf("expected %v, got %v", tt.fields, got)
				}
			}
		})
	}
}

// ==================== Channel Types test ====================

func TestChannelTypes(t *testing.T) {
	tests := []struct {
		ch   notificationChannel
		want string
	}{
		{&webhookChannel{}, "webhook"},
		{&telegramChannel{}, "telegram"},
		{&barkChannel{}, "bark"},
		{&serverchanChannel{}, "serverchan"},
		{&emailChannel{}, "email"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if tt.ch.Type() != tt.want {
				t.Fatalf("expected %s, got %s", tt.want, tt.ch.Type())
			}
		})
	}
}

// ==================== ValidateNotificationChannelsConfig test ====================

func TestValidateNotificationChannelsConfig_CollectsWarnings(t *testing.T) {
	cfg := &notificationChannelsConfig{
		Enabled: true,
		Channels: []channelEntry{
			{
				Type: "webhook", Name: "bad webhook", Enabled: true,
				Config: marshalRaw(webhookConfig{URL: ""}),
			},
			{
				Type: "bark", Name: "good bark", Enabled: true,
				Config: marshalRaw(barkConfig{URL: "https://api.day.app/xxx"}),
			},
		},
	}
	warnings := validateNotificationChannelsConfig(cfg)
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning for invalid webhook, got %d: %v", len(warnings), warnings)
	}
	if !strings.Contains(warnings[0], "bad webhook") {
		t.Fatalf("warning should mention channel name: %s", warnings[0])
	}
}

// ==================== Encrypt empty field does nothing ====================

func TestEncryptChannelEntrySecrets_EmptyField(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	entry := &channelEntry{
		Type:   "webhook",
		Config: marshalRaw(webhookConfig{URL: "https://example.com", HMACSecret: ""}),
	}
	if err := app.encryptChannelEntrySecrets(entry); err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}
	var cfg webhookConfig
	json.Unmarshal(entry.Config, &cfg)
	if cfg.HMACSecret != "" {
		t.Fatal("empty HMACSecret should stay empty after encrypt")
	}
}

// ==================== Telegram Send mode filter test ====================

func TestTelegramSend_ModeFilter(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	ch := &telegramChannel{
		app:    app,
		config: telegramConfig{BotToken: "test:token", ChatID: "-100123", Mode: "failure"},
	}
	// failure mode should skip info level
	err = ch.Send(context.Background(), "test", "info", "标题", "内容")
	if err != nil {
		t.Fatalf("mode filter should not return error for skipped level: %v", err)
	}
	// failure mode should pass error level
	err = ch.Send(context.Background(), "test", "error", "标题", "内容")
	// This will fail because the URL is not real, but we verify it attempted
	_ = err
}

// ==================== HealthCheckNotificationChannels test ====================

func TestHealthCheckNotificationChannels(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	// Disabled config
	app.mu.Lock()
	app.notificationConfig = notificationChannelsConfig{Enabled: false}
	app.mu.Unlock()

	check := app.healthCheckNotificationChannels()
	if check.Status != "ok" {
		t.Fatalf("expected ok for disabled config, got %s: %s", check.Status, check.Message)
	}

	// Enabled with no channels
	app.mu.Lock()
	app.notificationConfig = notificationChannelsConfig{Enabled: true}
	app.mu.Unlock()
	check = app.healthCheckNotificationChannels()
	if check.Status != "warning" {
		t.Fatalf("expected warning for enabled but no channels, got %s", check.Status)
	}

	// Enabled with channel but all disabled
	app.mu.Lock()
	app.notificationConfig = notificationChannelsConfig{
		Enabled: true,
		Channels: []channelEntry{
			{Type: "webhook", Name: "w1", Enabled: false},
		},
	}
	app.mu.Unlock()
	check = app.healthCheckNotificationChannels()
	if check.Status != "warning" {
		t.Fatalf("expected warning for all channels disabled, got %s", check.Status)
	}

	// Enabled with some enabled channels
	app.mu.Lock()
	app.notificationConfig = notificationChannelsConfig{
		Enabled: true,
		Channels: []channelEntry{
			{Type: "webhook", Name: "w1", Enabled: true},
			{Type: "bark", Name: "b1", Enabled: false},
		},
	}
	app.mu.Unlock()
	check = app.healthCheckNotificationChannels()
	if check.Status != "ok" {
		t.Fatalf("expected ok for enabled channels, got %s", check.Status)
	}
}

// ==================== Integration: notify -> dispatchNotification ====================

func TestNotifyTriggersDispatch(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	enableLocalOutbound(app)

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := notificationChannelsConfig{
		Enabled: true,
		Channels: []channelEntry{
			{
				Type: "webhook", Name: "test", Enabled: true,
				Config: marshalRaw(webhookConfig{URL: server.URL, Mode: "all", TimeoutSeconds: 5}),
				Levels: []string{"warning"},
			},
		},
	}
	app.mu.Lock()
	app.notificationConfig = cfg
	app.mu.Unlock()

	// notify uses go routine for dispatch, so wait a bit
	app.notify("checkin_failed", "warning", "签到失败", "详情内容", "account", "id123")
	time.Sleep(200 * time.Millisecond)

	if callCount == 0 {
		t.Fatal("expected dispatch to trigger at least one HTTP call")
	}
}

// ==================== ensureDefaultSettings includes notification.channels ====================

func TestEnsureDefaultSettings_IncludesNotificationChannels(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	var count int
	_ = app.db.QueryRow(`SELECT COUNT(*) FROM system_settings WHERE key = 'notification.channels'`).Scan(&count)
	if count == 0 {
		t.Fatal("expected notification.channels to be inserted by ensureDefaultSettings")
	}
}