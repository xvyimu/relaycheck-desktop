package notifications

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// NotificationHTTPPort is the subset of the host application that notification
// channels depend on. Extracting it breaks the reverse reference from channels
// back to the god object. The host (e.g. *core.App) satisfies this interface by
// providing outbound-URL validation (SSRF guard) and an HTTP client with
// timeout.
//
// All methods are exported so that types defined in other packages (the host
// application) can satisfy the interface cross-package.
type NotificationHTTPPort interface {
	// ValidateOutboundURL validates that raw is an http/https URL permitted by
	// the host's outbound policy (e.g. SSRF defences). It returns the parsed
	// URL on success.
	ValidateOutboundURL(ctx context.Context, raw string) (*url.URL, error)
	// DoHTTPWithTimeout executes req with the given timeout.
	DoHTTPWithTimeout(req *http.Request, timeout time.Duration) (*http.Response, error)
}

// CryptoPort is the subset of the host's crypto service that notification
// channels need to encrypt/decrypt sensitive channel secrets. The host's
// CryptoService satisfies this interface.
type CryptoPort interface {
	Encrypt(value string) (string, error)
	Decrypt(value string) (string, error)
}

// NotificationHub owns the notification configuration, digest goroutines,
// and per-channel rate limiters. It is extracted from the host god object so
// that notification channels depend on the narrow NotificationHTTPPort
// interface instead of the full application.
//
// All fields are protected by mu unless noted. The digest goroutines read
// digestChannels / channelRateLimits without holding mu; this mirrors the
// original behaviour and is safe because Reload fully replaces those maps
// atomically under the write lock.
type NotificationHub struct {
	mu                sync.RWMutex
	db                *sql.DB
	crypto            CryptoPort
	httpPort          NotificationHTTPPort
	config            ChannelsConfig
	digestChannels    map[string]*WebhookChannel
	digestCancel      context.CancelFunc
	digestWG          sync.WaitGroup
	channelRateLimits map[string]*ChannelRateLimiter
}

// NewNotificationHub constructs a NotificationHub backed by the given
// database handle, crypto port, and HTTP port (typically the host application
// itself, which satisfies NotificationHTTPPort).
func NewNotificationHub(db *sql.DB, crypto CryptoPort, httpPort NotificationHTTPPort) *NotificationHub {
	return &NotificationHub{
		db:                db,
		crypto:            crypto,
		httpPort:          httpPort,
		digestChannels:    map[string]*WebhookChannel{},
		channelRateLimits: map[string]*ChannelRateLimiter{},
	}
}

// Reload re-reads the notification config from the database, stops any
// running digest goroutine, and starts a new one for digest-mode webhooks.
func (h *NotificationHub) Reload(ctx context.Context) error {
	if h.db == nil {
		return nil
	}
	// 停止旧的 digest 循环（如有）
	if cancel := func() context.CancelFunc {
		h.mu.Lock()
		defer h.mu.Unlock()
		c := h.digestCancel
		h.digestCancel = nil
		// Note: do NOT reset digestWG here. The old digest goroutine still
		// holds a reference to it and will call Done() on exit; replacing the
		// WG would cause "negative WaitGroup counter" panics. After
		// cancel()+Wait() below, the counter returns to 0 and the same WG
		// can be reused for the next generation of digest goroutines.
		h.digestChannels = map[string]*WebhookChannel{}
		h.channelRateLimits = map[string]*ChannelRateLimiter{}
		return c
	}(); cancel != nil {
		cancel()
		h.digestWG.Wait()
	}
	config, err := h.LoadConfig(ctx)
	if err != nil {
		config = DefaultChannelsConfig()
	} else {
		for i := range config.Channels {
			if decErr := h.DecryptEntrySecrets(&config.Channels[i]); decErr != nil {
				log.Printf("[notification] 解密渠道 %s 密钥失败: %v", config.Channels[i].Name, decErr)
			}
		}
	}
	h.mu.Lock()
	h.config = config
	// 为 digest 模式的 webhook 启动专属 goroutine
	for _, entry := range config.Channels {
		if !entry.Enabled || entry.Type != "webhook" {
			continue
		}
		var cfg WebhookConfig
		if err := json.Unmarshal(entry.Config, &cfg); err != nil {
			continue
		}
		if cfg.Mode != "digest" {
			continue
		}
		ch := &WebhookChannel{
			httpPort: h.httpPort,
			config:   cfg,
			name:     entry.Name,
			levels:   entry.Levels,
			types:    entry.Types,
			entries:  []DigestEntry{},
			digestCh: make(chan DigestEntry, 100),
		}
		if err := ch.Validate(); err != nil {
			log.Printf("[notification] digest webhook %q 验证失败: %v", entry.Name, err)
			continue
		}
		h.digestChannels[entry.Name] = ch
		digestCtx, digestCancel := context.WithCancel(context.Background())
		h.digestCancel = digestCancel
		h.digestWG.Add(1)
		go func(c *WebhookChannel) {
			defer h.digestWG.Done()
			c.StartDigestLoop(digestCtx, c.digestCh)
		}(ch)
	}
	// 初始化频率限制器
	for _, entry := range config.Channels {
		if !entry.Enabled || entry.RateLimit == nil || entry.RateLimit.MaxPerInterval <= 0 {
			continue
		}
		h.channelRateLimits[entry.Name] = &ChannelRateLimiter{
			config: *entry.RateLimit,
		}
	}
	h.mu.Unlock()
	return nil
}

// LoadConfig reads the notification channels config from the database.
func (h *NotificationHub) LoadConfig(ctx context.Context) (ChannelsConfig, error) {
	if h.db == nil {
		return DefaultChannelsConfig(), nil
	}
	var valueJSON string
	err := h.db.QueryRowContext(ctx, `SELECT value_json FROM system_settings WHERE key='notification.channels'`).Scan(&valueJSON)
	if err == sql.ErrNoRows {
		return DefaultChannelsConfig(), nil
	}
	if err != nil {
		return DefaultChannelsConfig(), err
	}
	config, _ := ParseChannelsConfig(valueJSON)
	return config, nil
}

// CurrentConfig returns a snapshot of the currently loaded notification
// config.
func (h *NotificationHub) CurrentConfig() ChannelsConfig {
	if h == nil {
		return DefaultChannelsConfig()
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.config
}

// BuildChannel constructs a Channel from a ChannelEntry, injecting the hub's
// httpPort instead of a back-reference to the host application.
func (h *NotificationHub) BuildChannel(entry ChannelEntry) Channel {
	if entry.Config == nil {
		return nil
	}
	switch entry.Type {
	case "webhook":
		var cfg WebhookConfig
		if err := json.Unmarshal(entry.Config, &cfg); err != nil {
			return nil
		}
		ch := &WebhookChannel{
			httpPort: h.httpPort,
			config:   cfg,
			name:     entry.Name,
			levels:   entry.Levels,
			types:    entry.Types,
		}
		if err := ch.Validate(); err != nil {
			return nil
		}
		if cfg.Mode == "digest" {
			ch.digestCh = make(chan DigestEntry, 100)
		}
		return ch
	case "telegram":
		var cfg TelegramConfig
		if err := json.Unmarshal(entry.Config, &cfg); err != nil {
			return nil
		}
		ch := &TelegramChannel{
			httpPort: h.httpPort,
			config:   cfg,
			name:     entry.Name,
			levels:   entry.Levels,
			types:    entry.Types,
		}
		if err := ch.Validate(); err != nil {
			return nil
		}
		return ch
	case "bark":
		var cfg BarkConfig
		if err := json.Unmarshal(entry.Config, &cfg); err != nil {
			return nil
		}
		ch := &BarkChannel{
			httpPort: h.httpPort,
			config:   cfg,
			name:     entry.Name,
			levels:   entry.Levels,
			types:    entry.Types,
		}
		if err := ch.Validate(); err != nil {
			return nil
		}
		return ch
	case "serverchan":
		var cfg ServerChanConfig
		if err := json.Unmarshal(entry.Config, &cfg); err != nil {
			return nil
		}
		ch := &ServerChanChannel{
			httpPort: h.httpPort,
			config:   cfg,
			name:     entry.Name,
			levels:   entry.Levels,
			types:    entry.Types,
		}
		if err := ch.Validate(); err != nil {
			return nil
		}
		return ch
	case "email":
		var cfg EmailConfig
		if err := json.Unmarshal(entry.Config, &cfg); err != nil {
			return nil
		}
		ch := &EmailChannel{
			httpPort: h.httpPort,
			config:   cfg,
			name:     entry.Name,
			levels:   entry.Levels,
			types:    entry.Types,
		}
		if err := ch.Validate(); err != nil {
			return nil
		}
		return ch
	case "desktop":
		var cfg DesktopConfig
		if err := json.Unmarshal(entry.Config, &cfg); err != nil {
			return nil
		}
		ch := &DesktopChannel{
			httpPort: h.httpPort,
			config:   cfg,
			name:     entry.Name,
			levels:   entry.Levels,
			types:    entry.Types,
		}
		return ch
	default:
		return nil
	}
}

// EncryptEntrySecrets encrypts sensitive fields on the ChannelEntry in place
// using the hub's crypto port.
func (h *NotificationHub) EncryptEntrySecrets(entry *ChannelEntry) error {
	if entry.Config == nil {
		return nil
	}
	switch entry.Type {
	case "webhook":
		var cfg WebhookConfig
		if err := json.Unmarshal(entry.Config, &cfg); err != nil {
			return nil
		}
		if cfg.HMACSecret != "" {
			enc, err := h.crypto.Encrypt(cfg.HMACSecret)
			if err != nil {
				return err
			}
			cfg.HMACSecret = enc
		}
		entry.Config, _ = json.Marshal(cfg)
	case "telegram":
		var cfg TelegramConfig
		if err := json.Unmarshal(entry.Config, &cfg); err != nil {
			return nil
		}
		if cfg.BotToken != "" {
			enc, err := h.crypto.Encrypt(cfg.BotToken)
			if err != nil {
				return err
			}
			cfg.BotToken = enc
		}
		entry.Config, _ = json.Marshal(cfg)
	case "serverchan":
		var cfg ServerChanConfig
		if err := json.Unmarshal(entry.Config, &cfg); err != nil {
			return nil
		}
		if cfg.SendKey != "" {
			enc, err := h.crypto.Encrypt(cfg.SendKey)
			if err != nil {
				return err
			}
			cfg.SendKey = enc
		}
		entry.Config, _ = json.Marshal(cfg)
	case "email":
		var cfg EmailConfig
		if err := json.Unmarshal(entry.Config, &cfg); err != nil {
			return nil
		}
		if cfg.Password != "" {
			enc, err := h.crypto.Encrypt(cfg.Password)
			if err != nil {
				return err
			}
			cfg.Password = enc
		}
		entry.Config, _ = json.Marshal(cfg)
	}
	return nil
}

// DecryptEntrySecrets decrypts sensitive fields on the ChannelEntry in place.
// Fields that fail to decrypt are reset to empty string (matching the original
// fallback behaviour).
func (h *NotificationHub) DecryptEntrySecrets(entry *ChannelEntry) error {
	if entry.Config == nil {
		return nil
	}
	switch entry.Type {
	case "webhook":
		var cfg WebhookConfig
		if err := json.Unmarshal(entry.Config, &cfg); err != nil {
			return nil
		}
		if cfg.HMACSecret != "" && strings.HasPrefix(cfg.HMACSecret, "v1.") {
			dec, err := h.crypto.Decrypt(cfg.HMACSecret)
			if err == nil {
				cfg.HMACSecret = dec
			} else {
				// B3: 解密失败回退为空字符串
				cfg.HMACSecret = ""
			}
		}
		entry.Config, _ = json.Marshal(cfg)
	case "telegram":
		var cfg TelegramConfig
		if err := json.Unmarshal(entry.Config, &cfg); err != nil {
			return nil
		}
		if cfg.BotToken != "" && strings.HasPrefix(cfg.BotToken, "v1.") {
			dec, err := h.crypto.Decrypt(cfg.BotToken)
			if err == nil {
				cfg.BotToken = dec
			} else {
				cfg.BotToken = ""
			}
		}
		entry.Config, _ = json.Marshal(cfg)
	case "serverchan":
		var cfg ServerChanConfig
		if err := json.Unmarshal(entry.Config, &cfg); err != nil {
			return nil
		}
		if cfg.SendKey != "" && strings.HasPrefix(cfg.SendKey, "v1.") {
			dec, err := h.crypto.Decrypt(cfg.SendKey)
			if err == nil {
				cfg.SendKey = dec
			} else {
				cfg.SendKey = ""
			}
		}
		entry.Config, _ = json.Marshal(cfg)
	case "email":
		var cfg EmailConfig
		if err := json.Unmarshal(entry.Config, &cfg); err != nil {
			return nil
		}
		if cfg.Password != "" && strings.HasPrefix(cfg.Password, "v1.") {
			dec, err := h.crypto.Decrypt(cfg.Password)
			if err == nil {
				cfg.Password = dec
			} else {
				cfg.Password = ""
			}
		}
		entry.Config, _ = json.Marshal(cfg)
	}
	return nil
}

// Dispatch fans a notification out to all enabled, matching channels.
// Digest-mode webhooks are routed to the long-lived digest goroutine; all
// other channels are sent in a background goroutine.
func (h *NotificationHub) Dispatch(kind, level, title, content string) {
	config := h.CurrentConfig()
	if !config.Enabled {
		return
	}
	for _, entry := range config.Channels {
		if !entry.Enabled {
			continue
		}
		if !ShouldSendToChannel(entry, kind, level) {
			continue
		}
		// 频率限制检查
		if entry.RateLimit != nil && entry.RateLimit.MaxPerInterval > 0 {
			if rl, ok := h.channelRateLimits[entry.Name]; !ok || !rl.allow() {
				if ok {
					log.Printf("[notification] 渠道 %q 触发频率限制，跳过: %s/%s", entry.Name, kind, level)
				}
				continue
			}
		}
		// digest 模式的 webhook 走 hub 级别管理的 channel
		if entry.Type == "webhook" {
			var cfg WebhookConfig
			if err := json.Unmarshal(entry.Config, &cfg); err == nil && cfg.Mode == "digest" {
				if dc, ok := h.digestChannels[entry.Name]; ok && dc.digestCh != nil {
					select {
					case dc.digestCh <- DigestEntry{Kind: kind, Level: level, Title: title, Content: content, Time: time.Now()}:
					default:
						log.Printf("[notification] webhook digest 通道已满，丢弃通知: %s/%s", kind, level)
					}
					continue
				}
			}
		}
		// 普通渠道（非阻塞 goroutine）
		channel := h.BuildChannel(entry)
		if channel == nil {
			continue
		}
		go func(ch Channel) {
			if err := ch.Send(context.Background(), kind, level, title, content); err != nil {
				log.Printf("[notification] %s 发送失败: %v", ch.Type(), err)
			}
		}(channel)
	}
}

// Close stops the digest goroutine (if running) and waits for it to drain.
func (h *NotificationHub) Close() {
	h.mu.Lock()
	cancel := h.digestCancel
	h.digestCancel = nil
	h.mu.Unlock()
	if cancel != nil {
		cancel()
		h.digestWG.Wait()
	}
}

// SetConfig replaces the currently loaded notification config. It is
// intended for tests that need to inject a config without going through
// the DB-backed Reload path.
func (h *NotificationHub) SetConfig(cfg ChannelsConfig) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.config = cfg
}
