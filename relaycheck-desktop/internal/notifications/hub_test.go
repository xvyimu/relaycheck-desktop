package notifications

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// ==================== DB fixture ====================

// newTestDB returns an in-memory SQLite DB with the system_settings table
// pre-created. MaxOpenConns is forced to 1 so every query hits the same
// in-memory database (modernc/sqlite gives each connection its own :memory:
// DB, which would otherwise lose the seed data).
func newTestDB(t testing.TB) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`CREATE TABLE system_settings (key TEXT PRIMARY KEY, value_json TEXT NOT NULL)`); err != nil {
		db.Close()
		t.Fatalf("create system_settings: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// seedConfig writes the given JSON blob under key 'notification.channels'.
func seedConfig(t testing.TB, db *sql.DB, json string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO system_settings (key, value_json) VALUES ('notification.channels', ?)`, json); err != nil {
		t.Fatalf("seed config: %v", err)
	}
}

// ==================== Construction & nil safety ====================

func TestNewNotificationHub_InitialFields(t *testing.T) {
	hub := NewNotificationHub(nil, &fakeCryptoPort{}, &fakeHTTPPort{})
	if hub.db != nil {
		t.Fatal("db should be nil when constructed with nil")
	}
	if hub.crypto == nil || hub.httpPort == nil {
		t.Fatal("crypto and httpPort should be set")
	}
	if hub.digestChannels == nil {
		t.Fatal("digestChannels map should be initialized")
	}
	if hub.channelRateLimits == nil {
		t.Fatal("channelRateLimits map should be initialized")
	}
}

func TestCurrentConfig_NilHub(t *testing.T) {
	var nilHub *NotificationHub
	cfg := nilHub.CurrentConfig()
	if cfg.Enabled {
		t.Fatal("nil hub should return disabled default config")
	}
	if len(cfg.DefaultLevels) != 2 {
		t.Fatalf("expected 2 default levels, got %d", len(cfg.DefaultLevels))
	}
}

func TestCurrentConfig_FreshHub(t *testing.T) {
	hub := NewNotificationHub(nil, &fakeCryptoPort{}, &fakeHTTPPort{})
	cfg := hub.CurrentConfig()
	if cfg.Enabled {
		t.Fatal("fresh hub should return disabled default config")
	}
}

// ==================== LoadConfig ====================

func TestLoadConfig_NilDB(t *testing.T) {
	hub := NewNotificationHub(nil, &fakeCryptoPort{}, &fakeHTTPPort{})
	cfg, err := hub.LoadConfig(context.Background())
	if err != nil {
		t.Fatalf("nil db should not error: %v", err)
	}
	if cfg.Enabled {
		t.Fatal("nil db should return disabled default config")
	}
}

func TestLoadConfig_NoRow(t *testing.T) {
	db := newTestDB(t)
	hub := NewNotificationHub(db, &fakeCryptoPort{}, &fakeHTTPPort{})
	cfg, err := hub.LoadConfig(context.Background())
	if err != nil {
		t.Fatalf("missing row should not error: %v", err)
	}
	if cfg.Enabled {
		t.Fatal("missing row should return disabled default config")
	}
}

func TestLoadConfig_ValidRow(t *testing.T) {
	db := newTestDB(t)
	seedConfig(t, db, `{"enabled":true,"defaultLevels":["error"],"channels":[{"type":"bark","name":"b1","enabled":true,"config":{"url":"https://api.day.app/x"}}]}`)
	hub := NewNotificationHub(db, &fakeCryptoPort{}, &fakeHTTPPort{})

	cfg, err := hub.LoadConfig(context.Background())
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if !cfg.Enabled {
		t.Fatal("expected enabled=true")
	}
	if len(cfg.Channels) != 1 || cfg.Channels[0].Name != "b1" {
		t.Fatalf("unexpected channels: %+v", cfg.Channels)
	}
}

// ==================== Reload ====================

func TestReload_NilDB(t *testing.T) {
	hub := NewNotificationHub(nil, &fakeCryptoPort{}, &fakeHTTPPort{})
	if err := hub.Reload(context.Background()); err != nil {
		t.Fatalf("nil db Reload should not error: %v", err)
	}
	// No digest goroutine should be running; Close should be a no-op.
	hub.Close()
}

func TestReload_NoRow_LoadsDefault(t *testing.T) {
	db := newTestDB(t)
	hub := NewNotificationHub(db, &fakeCryptoPort{}, &fakeHTTPPort{})
	if err := hub.Reload(context.Background()); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	cfg := hub.CurrentConfig()
	if cfg.Enabled {
		t.Fatal("missing row should leave config disabled")
	}
}

func TestReload_ValidConfigNoDigest(t *testing.T) {
	db := newTestDB(t)
	seedConfig(t, db, `{"enabled":true,"channels":[{"type":"bark","name":"b1","enabled":true,"config":{"url":"https://api.day.app/x"}}]}`)
	hub := NewNotificationHub(db, &fakeCryptoPort{}, &fakeHTTPPort{})

	if err := hub.Reload(context.Background()); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	cfg := hub.CurrentConfig()
	if !cfg.Enabled || len(cfg.Channels) != 1 {
		t.Fatalf("config not loaded: %+v", cfg)
	}
	// No digest channels → Close is a no-op, but must not block.
	hub.Close()
}

func TestReload_StopsPreviousDigestGoroutine(t *testing.T) {
	db := newTestDB(t)
	seedConfig(t, db, `{"enabled":true,"channels":[
		{"type":"webhook","name":"dig","enabled":true,
		 "config":{"url":"https://example.com/hook","mode":"digest","timeoutSeconds":3,"digestIntervalMin":1}}
	]}`)
	hub := NewNotificationHub(db, &fakeCryptoPort{}, &fakeHTTPPort{})

	if err := hub.Reload(context.Background()); err != nil {
		t.Fatalf("first Reload: %v", err)
	}
	// Second Reload should stop the first digest goroutine and start a fresh one.
	if err := hub.Reload(context.Background()); err != nil {
		t.Fatalf("second Reload: %v", err)
	}
	// Close must drain all digest goroutines within a reasonable window.
	done := make(chan struct{})
	go func() {
		hub.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Close timed out; digest goroutine likely leaked")
	}
}

// ==================== SetConfig / CurrentConfig round-trip ====================

func TestSetConfig_CurrentConfig_RoundTrip(t *testing.T) {
	hub := NewNotificationHub(nil, &fakeCryptoPort{}, &fakeHTTPPort{})
	original := ChannelsConfig{
		Enabled:       true,
		DefaultLevels: []string{"error"},
		Channels: []ChannelEntry{
			{Type: "bark", Name: "b1", Enabled: true, Config: MarshalRaw(BarkConfig{URL: "https://x"})},
		},
	}
	hub.SetConfig(original)
	got := hub.CurrentConfig()
	if !got.Enabled || len(got.Channels) != 1 || got.Channels[0].Name != "b1" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

// ==================== BuildChannel edge cases ====================

func TestBuildChannel_NilConfig(t *testing.T) {
	hub := NewNotificationHub(nil, &fakeCryptoPort{}, &fakeHTTPPort{})
	if ch := hub.BuildChannel(ChannelEntry{Type: "webhook", Config: nil}); ch != nil {
		t.Fatal("nil config should return nil channel")
	}
}

func TestBuildChannel_UnknownType(t *testing.T) {
	hub := NewNotificationHub(nil, &fakeCryptoPort{}, &fakeHTTPPort{})
	if ch := hub.BuildChannel(ChannelEntry{Type: "nonexistent", Config: MarshalRaw(BarkConfig{URL: "https://x"})}); ch != nil {
		t.Fatalf("unknown type should return nil, got %T", ch)
	}
}

func TestBuildChannel_MalformedJSON(t *testing.T) {
	hub := NewNotificationHub(nil, &fakeCryptoPort{}, &fakeHTTPPort{})
	if ch := hub.BuildChannel(ChannelEntry{Type: "webhook", Config: json.RawMessage(`{not json`)}); ch != nil {
		t.Fatalf("malformed JSON should return nil, got %T", ch)
	}
}

func TestBuildChannel_AllValidTypes(t *testing.T) {
	hub := NewNotificationHub(nil, &fakeCryptoPort{}, &fakeHTTPPort{})
	cases := []struct {
		typeStr string
		config  interface{}
	}{
		{"webhook", WebhookConfig{URL: "https://example.com/hook"}},
		{"telegram", TelegramConfig{BotToken: "123:ABC", ChatID: "-100"}},
		{"bark", BarkConfig{URL: "https://api.day.app/x"}},
		{"serverchan", ServerChanConfig{SendKey: "SCT123"}},
		{"email", EmailConfig{SMTPHost: "smtp.example.com", FromAddr: "a@b.com", ToAddr: "c@d.com"}},
		{"desktop", DesktopConfig{Mode: "all"}},
	}
	for _, c := range cases {
		t.Run(c.typeStr, func(t *testing.T) {
			ch := hub.BuildChannel(ChannelEntry{Type: c.typeStr, Config: MarshalRaw(c.config)})
			if ch == nil {
				t.Fatalf("%s should build non-nil channel", c.typeStr)
			}
			if ch.Type() != c.typeStr {
				t.Fatalf("type mismatch: got %s, want %s", ch.Type(), c.typeStr)
			}
		})
	}
}

// ==================== EncryptEntrySecrets / DecryptEntrySecrets ====================

func TestEncryptDecryptEntrySecrets_RoundTrip(t *testing.T) {
	hub := NewNotificationHub(nil, &fakeCryptoPort{}, &fakeHTTPPort{})
	cases := []struct {
		typeStr string
		config  interface{}
	}{
		{"webhook", WebhookConfig{URL: "https://x", HMACSecret: "secret"}},
		{"telegram", TelegramConfig{BotToken: "123:ABC", ChatID: "-100"}},
		{"serverchan", ServerChanConfig{SendKey: "SCT123"}},
		{"email", EmailConfig{SMTPHost: "h", FromAddr: "a@b.com", ToAddr: "c@d.com", Password: "pw"}},
	}
	for _, c := range cases {
		t.Run(c.typeStr, func(t *testing.T) {
			entry := &ChannelEntry{Type: c.typeStr, Config: MarshalRaw(c.config)}
			if err := hub.EncryptEntrySecrets(entry); err != nil {
				t.Fatalf("encrypt: %v", err)
			}
			// Verify the sensitive field is prefixed with v1.
			if !hasV1Prefix(c.typeStr, entry.Config) {
				t.Fatalf("sensitive field not encrypted for %s", c.typeStr)
			}
			if err := hub.DecryptEntrySecrets(entry); err != nil {
				t.Fatalf("decrypt: %v", err)
			}
			// Verify the sensitive field is back to original.
			if !matchesOriginalSecret(c.typeStr, entry.Config, c.config) {
				t.Fatalf("round-trip mismatch for %s", c.typeStr)
			}
		})
	}
}

func hasV1Prefix(typeStr string, raw json.RawMessage) bool {
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return false
	}
	field := secretField(typeStr)
	if field == "" {
		return false
	}
	v, ok := m[field].(string)
	return ok && strings.HasPrefix(v, "v1.")
}

func matchesOriginalSecret(typeStr string, raw json.RawMessage, original interface{}) bool {
	field := secretField(typeStr)
	if field == "" {
		return true
	}
	var got map[string]interface{}
	if err := json.Unmarshal(raw, &got); err != nil {
		return false
	}
	// Marshal original to JSON then back to map for uniform comparison.
	origRaw, _ := json.Marshal(original)
	var want map[string]interface{}
	if err := json.Unmarshal(origRaw, &want); err != nil {
		return false
	}
	gotVal, _ := got[field].(string)
	wantVal, _ := want[field].(string)
	return gotVal == wantVal
}

func secretField(typeStr string) string {
	switch typeStr {
	case "webhook":
		return "hmacSecret"
	case "telegram":
		return "botToken"
	case "serverchan":
		return "sendKey"
	case "email":
		return "password"
	default:
		return ""
	}
}

func TestDecryptEntrySecrets_NonV1Prefix_Unchanged(t *testing.T) {
	hub := NewNotificationHub(nil, &fakeCryptoPort{}, &fakeHTTPPort{})
	// Plaintext secret without v1. prefix should be left as-is by Decrypt.
	entry := &ChannelEntry{
		Type:   "webhook",
		Config: MarshalRaw(WebhookConfig{URL: "https://x", HMACSecret: "plaintext"}),
	}
	if err := hub.DecryptEntrySecrets(entry); err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	var cfg WebhookConfig
	json.Unmarshal(entry.Config, &cfg)
	if cfg.HMACSecret != "plaintext" {
		t.Fatalf("non-v1 secret should be untouched, got %q", cfg.HMACSecret)
	}
}

func TestDecryptEntrySecrets_DecryptFailure_ResetsToEmpty(t *testing.T) {
	// fakeCryptoPort.Decrypt fails for non-v1 input, but the prefix check
	// means a "v1." value with a corrupt payload must still attempt decrypt
	// and reset to empty on failure. Use a crypto port that always fails
	// decryption to exercise this path.
	failingCrypto := &failingCryptoPort{}
	hub := NewNotificationHub(nil, failingCrypto, &fakeHTTPPort{})

	entry := &ChannelEntry{
		Type:   "webhook",
		Config: MarshalRaw(WebhookConfig{URL: "https://x", HMACSecret: "v1.corrupt"}),
	}
	if err := hub.DecryptEntrySecrets(entry); err != nil {
		t.Fatalf("decrypt should not surface error: %v", err)
	}
	var cfg WebhookConfig
	json.Unmarshal(entry.Config, &cfg)
	if cfg.HMACSecret != "" {
		t.Fatalf("failed decrypt should reset to empty, got %q", cfg.HMACSecret)
	}
}

// failingCryptoPort is a CryptoPort whose Decrypt always errors. Used to
// exercise the "reset to empty on decrypt failure" branch.
type failingCryptoPort struct{}

func (f *failingCryptoPort) Encrypt(value string) (string, error) { return "v1." + value, nil }
func (f *failingCryptoPort) Decrypt(value string) (string, error) {
	return "", fmt.Errorf("decryption disabled")
}

// ==================== Dispatch ====================

func TestDispatch_DisabledConfig_NoOp(t *testing.T) {
	hub := NewNotificationHub(nil, &fakeCryptoPort{}, &fakeHTTPPort{})
	hub.SetConfig(ChannelsConfig{Enabled: false})
	// Should return without panicking; nothing to assert beyond that.
	hub.Dispatch("any", "error", "title", "content")
}

func TestDispatch_NoMatchingChannels(t *testing.T) {
	hub := NewNotificationHub(nil, &fakeCryptoPort{}, &fakeHTTPPort{})
	hub.SetConfig(ChannelsConfig{
		Enabled: true,
		Channels: []ChannelEntry{
			{Type: "bark", Name: "b1", Enabled: true, Config: MarshalRaw(BarkConfig{URL: "https://x"}), Levels: []string{"error"}},
		},
	})
	// Sending a "warning" level to a channel that only accepts "error" should
	// be filtered out by ShouldSendToChannel; no panic, no network call.
	hub.Dispatch("kind", "warning", "title", "content")
}

func TestDispatch_DisabledChannel_Skipped(t *testing.T) {
	hub := NewNotificationHub(nil, &fakeCryptoPort{}, &fakeHTTPPort{})
	hub.SetConfig(ChannelsConfig{
		Enabled: true,
		Channels: []ChannelEntry{
			{Type: "bark", Name: "disabled-bark", Enabled: false, Config: MarshalRaw(BarkConfig{URL: "https://x"})},
		},
	})
	// Should not attempt any send; the channel is disabled.
	hub.Dispatch("kind", "error", "title", "content")
}

func TestDispatch_RateLimit_BlocksAfterMax(t *testing.T) {
	hub := NewNotificationHub(nil, &fakeCryptoPort{}, &fakeHTTPPort{})

	var sent int64
	server := newCountingServer(t, &sent)

	hub.SetConfig(ChannelsConfig{
		Enabled: true,
		Channels: []ChannelEntry{
			{
				Type:      "bark",
				Name:      "rl-bark",
				Enabled:   true,
				Config:    MarshalRaw(BarkConfig{URL: server.URL, Mode: "all"}),
				RateLimit: &RateLimitConfig{MaxPerInterval: 2, IntervalSec: 60},
			},
		},
	})
	// SetConfig does not initialize channelRateLimits (only Reload does).
	// Populate it directly since we're testing Dispatch's rate-limit path,
	// not Reload's initialization.
	hub.channelRateLimits = map[string]*ChannelRateLimiter{
		"rl-bark": {config: RateLimitConfig{MaxPerInterval: 2, IntervalSec: 60}},
	}

	// First 2 dispatches should send; the 3rd should be rate-limited.
	hub.Dispatch("k", "error", "t1", "c1")
	hub.Dispatch("k", "error", "t2", "c2")
	hub.Dispatch("k", "error", "t3", "c3")

	// Dispatch fires non-digest sends in a goroutine; wait briefly for them.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt64(&sent) >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := atomic.LoadInt64(&sent); got != 2 {
		t.Fatalf("expected exactly 2 sends (3rd rate-limited), got %d", got)
	}
}

// newCountingServer returns an httptest.Server that increments *counter on
// every request. Used to verify rate-limiting behaviour.
func newCountingServer(t testing.TB, counter *int64) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(counter, 1)
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// ==================== Close ====================

func TestClose_NoDigestRunning_NoOp(t *testing.T) {
	hub := NewNotificationHub(nil, &fakeCryptoPort{}, &fakeHTTPPort{})
	// Should not block or panic when no digest goroutine is running.
	done := make(chan struct{})
	go func() {
		hub.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Close blocked with no digest goroutine running")
	}
}

func TestClose_AfterReloadWithDigest_DrainsGoroutine(t *testing.T) {
	db := newTestDB(t)
	seedConfig(t, db, `{"enabled":true,"channels":[
		{"type":"webhook","name":"dig","enabled":true,
		 "config":{"url":"https://example.com/hook","mode":"digest","timeoutSeconds":3,"digestIntervalMin":1}}
	]}`)
	hub := NewNotificationHub(db, &fakeCryptoPort{}, &fakeHTTPPort{})

	if err := hub.Reload(context.Background()); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	done := make(chan struct{})
	go func() {
		hub.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Close blocked; digest goroutine not drained")
	}
}

// ==================== EncryptEntrySecrets on unknown type ====================

func TestEncryptEntrySecrets_UnknownType_NoOp(t *testing.T) {
	hub := NewNotificationHub(nil, &fakeCryptoPort{}, &fakeHTTPPort{})
	entry := &ChannelEntry{Type: "unknown", Config: MarshalRaw(map[string]string{"k": "v"})}
	if err := hub.EncryptEntrySecrets(entry); err != nil {
		t.Fatalf("encrypt unknown type should not error: %v", err)
	}
}

func TestDecryptEntrySecrets_UnknownType_NoOp(t *testing.T) {
	hub := NewNotificationHub(nil, &fakeCryptoPort{}, &fakeHTTPPort{})
	entry := &ChannelEntry{Type: "unknown", Config: MarshalRaw(map[string]string{"k": "v"})}
	if err := hub.DecryptEntrySecrets(entry); err != nil {
		t.Fatalf("decrypt unknown type should not error: %v", err)
	}
}

// ==================== EncryptEntrySecrets nil config ====================

func TestEncryptEntrySecrets_NilConfig(t *testing.T) {
	hub := NewNotificationHub(nil, &fakeCryptoPort{}, &fakeHTTPPort{})
	entry := &ChannelEntry{Type: "webhook", Config: nil}
	if err := hub.EncryptEntrySecrets(entry); err != nil {
		t.Fatalf("nil config should not error: %v", err)
	}
}

func TestDecryptEntrySecrets_NilConfig(t *testing.T) {
	hub := NewNotificationHub(nil, &fakeCryptoPort{}, &fakeHTTPPort{})
	entry := &ChannelEntry{Type: "webhook", Config: nil}
	if err := hub.DecryptEntrySecrets(entry); err != nil {
		t.Fatalf("nil config should not error: %v", err)
	}
}
