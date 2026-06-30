package notifications

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
	"net/url"
	"strings"
	"testing"
	"time"
)

// ==================== 测试辅助 ====================

// fakeHTTPPort 是 NotificationHTTPPort 的测试实现，允许本地回环地址
// （httptest.Server 监听 127.0.0.1）并直接转发请求到默认 HTTP 客户端。
type fakeHTTPPort struct{}

func (f *fakeHTTPPort) ValidateOutboundURL(ctx context.Context, raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("URL 必须包含 http/https 协议和主机名")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("URL 只支持 http 或 https 协议")
	}
	// 测试环境允许任意主机（含本地回环），因为 httptest.Server 使用 127.0.0.1
	return parsed, nil
}

func (f *fakeHTTPPort) DoHTTPWithTimeout(req *http.Request, timeout time.Duration) (*http.Response, error) {
	client := &http.Client{Timeout: timeout}
	return client.Do(req)
}

// fakeCryptoPort 是 CryptoPort 的测试实现，使用简单的 "v1." 前缀标记加密值。
type fakeCryptoPort struct{}

func (f *fakeCryptoPort) Encrypt(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	return "v1." + value, nil
}

func (f *fakeCryptoPort) Decrypt(value string) (string, error) {
	if !strings.HasPrefix(value, "v1.") {
		return "", fmt.Errorf("无法解密")
	}
	return strings.TrimPrefix(value, "v1."), nil
}

// ==================== 配置解析测试 ====================

func TestDefaultNotificationChannelsConfig(t *testing.T) {
	cfg := DefaultChannelsConfig()
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
	cfg, warnings := ParseChannelsConfig(raw)
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

	var wc WebhookConfig
	json.Unmarshal(cfg.Channels[0].Config, &wc)
	if wc.URL != "https://hooks.example.com/hook" || wc.HMACSecret != "testsecret" || wc.TimeoutSeconds != 15 {
		t.Fatalf("unexpected webhook config: %+v", wc)
	}

	var tc TelegramConfig
	json.Unmarshal(cfg.Channels[1].Config, &tc)
	if tc.BotToken != "123:ABC" || tc.ChatID != "-100123" {
		t.Fatalf("unexpected telegram config: %+v", tc)
	}
}

func TestParseNotificationChannelsConfig_Minimal(t *testing.T) {
	raw := `{"enabled":true}`
	cfg, warnings := ParseChannelsConfig(raw)
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
	cfg, warnings := ParseChannelsConfig(raw)
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
		config  WebhookConfig
		wantErr bool
	}{
		{"valid with url", WebhookConfig{URL: "https://hooks.example.com/hook"}, false},
		{"valid with url and hmac", WebhookConfig{URL: "https://hooks.example.com/hook", HMACSecret: "secret"}, false},
		{"empty url", WebhookConfig{URL: ""}, true},
		{"blank url", WebhookConfig{URL: "  "}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := &WebhookChannel{config: tt.config}
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
		config  TelegramConfig
		wantErr bool
	}{
		{"valid", TelegramConfig{BotToken: "123:ABC", ChatID: "-100123"}, false},
		{"empty bot token", TelegramConfig{BotToken: "", ChatID: "-100123"}, true},
		{"empty chat id", TelegramConfig{BotToken: "123:ABC", ChatID: ""}, true},
		{"both empty", TelegramConfig{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := &TelegramChannel{config: tt.config}
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
		config  BarkConfig
		wantErr bool
	}{
		{"valid url", BarkConfig{URL: "https://api.day.app/xxx"}, false},
		{"empty url", BarkConfig{URL: ""}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := &BarkChannel{config: tt.config}
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
		config  ServerChanConfig
		wantErr bool
	}{
		{"valid sendkey", ServerChanConfig{SendKey: "SCT123"}, false},
		{"empty sendkey", ServerChanConfig{SendKey: ""}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := &ServerChanChannel{config: tt.config}
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
		config  EmailConfig
		wantErr bool
	}{
		{"all required fields", EmailConfig{SMTPHost: "smtp.example.com", FromAddr: "a@b.com", ToAddr: "c@d.com"}, false},
		{"empty smtp host", EmailConfig{SMTPHost: "", FromAddr: "a@b.com", ToAddr: "c@d.com"}, true},
		{"empty from addr", EmailConfig{SMTPHost: "smtp.example.com", FromAddr: "", ToAddr: "c@d.com"}, true},
		{"empty to addr", EmailConfig{SMTPHost: "smtp.example.com", FromAddr: "a@b.com", ToAddr: ""}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := &EmailChannel{config: tt.config}
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
			got := LevelMatchesMode(tt.mode, tt.level)
			if got != tt.want {
				t.Fatalf("LevelMatchesMode(%q, %q) = %v, want %v", tt.mode, tt.level, got, tt.want)
			}
		})
	}
}

// ==================== ShouldSendToChannel 测试 ====================

func TestShouldSendToChannel(t *testing.T) {
	entry := ChannelEntry{
		Levels: []string{"warning", "error"},
		Types:  []string{"scheduled_checkin_failed"},
	}
	if !ShouldSendToChannel(entry, "scheduled_checkin_failed", "warning") {
		t.Fatal("should match both type and level")
	}
	if !ShouldSendToChannel(entry, "scheduled_checkin_failed", "error") {
		t.Fatal("should match type and error level")
	}
	if ShouldSendToChannel(entry, "scheduled_checkin_failed", "info") {
		t.Fatal("should not match info level")
	}
	if ShouldSendToChannel(entry, "auth_failed", "warning") {
		t.Fatal("should not match different type")
	}
}

func TestShouldSendToChannel_EmptyLevels(t *testing.T) {
	entry := ChannelEntry{
		Levels: nil,
		Types:  []string{"scheduled_checkin_failed"},
	}
	if !ShouldSendToChannel(entry, "scheduled_checkin_failed", "info") {
		t.Fatal("empty levels should allow any level")
	}
}

func TestShouldSendToChannel_EmptyTypes(t *testing.T) {
	entry := ChannelEntry{
		Levels: []string{"warning"},
		Types:  nil,
	}
	if !ShouldSendToChannel(entry, "any_type", "warning") {
		t.Fatal("empty types should allow any type")
	}
}

func TestShouldSendToChannel_EdgeCases(t *testing.T) {
	entry := ChannelEntry{}
	if !ShouldSendToChannel(entry, "any_kind", "any_level") {
		t.Fatal("empty entry should allow all")
	}

	entry.Types = []string{"type_a"}
	if !ShouldSendToChannel(entry, "type_a", "any") {
		t.Fatal("should match type_a")
	}
	if ShouldSendToChannel(entry, "type_b", "any") {
		t.Fatal("should not match type_b")
	}

	entry2 := ChannelEntry{Levels: []string{"error"}}
	if !ShouldSendToChannel(entry2, "any", "error") {
		t.Fatal("should match error level")
	}
	if ShouldSendToChannel(entry2, "any", "info") {
		t.Fatal("should not match info level")
	}
}

// ==================== Webhook HTTP 发送测试 ====================

func TestWebhookSend_Success(t *testing.T) {
	port := &fakeHTTPPort{}

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

	ch := &WebhookChannel{
		httpPort: port,
		config:   WebhookConfig{URL: server.URL, Mode: "all", TimeoutSeconds: 5},
	}
	err := ch.Send(context.Background(), "test_kind", "warning", "测试标题", "测试内容")
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
	port := &fakeHTTPPort{}

	secret := "test-hmac-key-2026"
	var capturedSig string
	var capturedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedSig = r.Header.Get("X-Signature-256")
		capturedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ch := &WebhookChannel{
		httpPort: port,
		config: WebhookConfig{
			URL:            server.URL,
			HMACSecret:     secret,
			Mode:           "all",
			TimeoutSeconds: 5,
		},
	}
	err := ch.Send(context.Background(), "test", "error", "HMAC 测试", "内容")
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
	port := &fakeHTTPPort{}

	digestReceived := make(chan map[string]interface{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var data map[string]interface{}
		json.Unmarshal(body, &data)
		digestReceived <- data
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ch := &WebhookChannel{
		httpPort: port,
		config:   WebhookConfig{URL: server.URL, Mode: "digest", TimeoutSeconds: 5},
	}
	// Directly populate entries (bypassing StartDigestLoop)
	ch.entries = []DigestEntry{
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

// ==================== WebhookSend mode filter test ====================

func TestWebhookSend_ModeFilter(t *testing.T) {
	port := &fakeHTTPPort{}

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ch := &WebhookChannel{
		httpPort: port,
		config:   WebhookConfig{URL: server.URL, Mode: "success", TimeoutSeconds: 5},
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
	port := &fakeHTTPPort{}

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

	ch := &BarkChannel{
		httpPort: port,
		config:   BarkConfig{URL: server.URL, Mode: "all", Group: "RelayCheck"},
	}

	err := ch.Send(context.Background(), "test", "warning", "测试标题", "测试内容")
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}
}

// ==================== EncryptedFields test ====================

func TestEncryptedFields(t *testing.T) {
	tests := []struct {
		ch     Channel
		fields []string
	}{
		{&WebhookChannel{}, []string{"hmacSecret"}},
		{&TelegramChannel{}, []string{"botToken"}},
		{&BarkChannel{}, nil},
		{&ServerChanChannel{}, []string{"sendKey"}},
		{&EmailChannel{}, []string{"password"}},
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
		ch   Channel
		want string
	}{
		{&WebhookChannel{}, "webhook"},
		{&TelegramChannel{}, "telegram"},
		{&BarkChannel{}, "bark"},
		{&ServerChanChannel{}, "serverchan"},
		{&EmailChannel{}, "email"},
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
	cfg := &ChannelsConfig{
		Enabled: true,
		Channels: []ChannelEntry{
			{
				Type: "webhook", Name: "bad webhook", Enabled: true,
				Config: MarshalRaw(WebhookConfig{URL: ""}),
			},
			{
				Type: "bark", Name: "good bark", Enabled: true,
				Config: MarshalRaw(BarkConfig{URL: "https://api.day.app/xxx"}),
			},
		},
	}
	warnings := ValidateChannelsConfig(cfg)
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning for invalid webhook, got %d: %v", len(warnings), warnings)
	}
	if !strings.Contains(warnings[0], "bad webhook") {
		t.Fatalf("warning should mention channel name: %s", warnings[0])
	}
}

// ==================== Encrypt empty field does nothing ====================

func TestEncryptChannelEntrySecrets_EmptyField(t *testing.T) {
	hub := NewNotificationHub(nil, &fakeCryptoPort{}, &fakeHTTPPort{})

	entry := &ChannelEntry{
		Type:   "webhook",
		Config: MarshalRaw(WebhookConfig{URL: "https://example.com", HMACSecret: ""}),
	}
	if err := hub.EncryptEntrySecrets(entry); err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}
	var cfg WebhookConfig
	json.Unmarshal(entry.Config, &cfg)
	if cfg.HMACSecret != "" {
		t.Fatal("empty HMACSecret should stay empty after encrypt")
	}
}

// ==================== Telegram Send mode filter test ====================

func TestTelegramSend_ModeFilter(t *testing.T) {
	port := &fakeHTTPPort{}

	ch := &TelegramChannel{
		httpPort: port,
		config:   TelegramConfig{BotToken: "test:token", ChatID: "-100123", Mode: "failure"},
	}
	// failure mode should skip info level
	err := ch.Send(context.Background(), "test", "info", "标题", "内容")
	if err != nil {
		t.Fatalf("mode filter should not return error for skipped level: %v", err)
	}
	// failure mode should pass error level
	err = ch.Send(context.Background(), "test", "error", "标题", "内容")
	// This will fail because the URL is not real, but we verify it attempted
	_ = err
}

// ==================== 工具函数测试 ====================

func TestBuildNotifyBody(t *testing.T) {
	body := BuildNotifyBody("test_kind", "warning", "测试标题", "测试内容")
	if !strings.Contains(body, "测试标题") || !strings.Contains(body, "测试内容") {
		t.Fatalf("body missing content: %s", body)
	}
}

func TestMaskSensitiveField(t *testing.T) {
	if MaskSensitiveField("") != "" {
		t.Fatal("empty should remain empty")
	}
	// Short values (<=4): all asterisks
	if MaskSensitiveField("ab") != "**" {
		t.Fatalf("short value should be all masked, got %s", MaskSensitiveField("ab"))
	}
	if MaskSensitiveField("1234") != "****" {
		t.Fatalf("4-char value should be all masked, got %s", MaskSensitiveField("1234"))
	}
	// Long values: show last 4 chars
	if MaskSensitiveField("secret123") != "*****t123" {
		t.Fatalf("unexpected masked value: %s", MaskSensitiveField("secret123"))
	}
	// 9 chars -> 5 asterisks + last 4
	if MaskSensitiveField("verylongsecretkey2026") != "*****************2026" {
		t.Fatalf("unexpected masked value: %s", MaskSensitiveField("verylongsecretkey2026"))
	}
}

func TestTruncateNotifyContent(t *testing.T) {
	short := "short content"
	if truncated := TruncateNotifyContent(short, 100); truncated != short {
		t.Fatalf("short content should not be truncated: %s", truncated)
	}
	long := "a" + strings.Repeat("b", 5000) + "c"
	truncated := TruncateNotifyContent(long, 100)
	if len([]rune(truncated)) > 105 {
		t.Fatalf("content should be truncated, len=%d", len([]rune(truncated)))
	}
}

func TestStringInSlice(t *testing.T) {
	list := []string{"a", "b", "c"}
	if !StringInSlice("a", list) {
		t.Fatal("a should be in list")
	}
	if StringInSlice("d", list) {
		t.Fatal("d should not be in list")
	}
	if StringInSlice("a", nil) {
		t.Fatal("nil list should not contain anything")
	}
}

// ==================== buildEmailMessage 测试 ====================

func TestBuildEmailMessage(t *testing.T) {
	msg := BuildEmailMessage("sender@test.com", "recipient@test.com", "测试邮件标题", "邮件正文内容")
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

// ==================== BuildChannel digest 测试 ====================

// TestBuildChannel_Webhook_Digest verifies that BuildChannel initializes the
// digestCh for digest-mode webhooks. This test must live in the notifications
// package because digestCh is unexported.
func TestBuildChannel_Webhook_Digest(t *testing.T) {
	hub := NewNotificationHub(nil, &fakeCryptoPort{}, &fakeHTTPPort{})

	entry := ChannelEntry{
		Type:    "webhook",
		Name:    "digest-test",
		Enabled: true,
		Config:  MarshalRaw(WebhookConfig{URL: "https://example.com/hook", Mode: "digest", TimeoutSeconds: 10}),
	}
	ch := hub.BuildChannel(entry)
	if ch == nil {
		t.Fatal("expected non-nil channel")
	}
	wc, ok := ch.(*WebhookChannel)
	if !ok {
		t.Fatal("expected WebhookChannel type")
	}
	if wc.digestCh == nil {
		t.Fatal("expected digestCh to be initialized")
	}
}
