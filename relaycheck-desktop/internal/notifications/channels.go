package notifications

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/smtp"
	"net/url"
	"strings"
	"sync"
	"time"
)

// ==================== 配置结构体 ====================

// ChannelsConfig is the top-level notification channels configuration stored
// in system_settings under the key "notification.channels".
type ChannelsConfig struct {
	Enabled       bool           `json:"enabled"`
	DefaultLevels []string       `json:"defaultLevels"`
	Channels      []ChannelEntry `json:"channels"`
}

// ChannelEntry describes a single configured notification channel.
type ChannelEntry struct {
	Type      string           `json:"type"` // "webhook" | "telegram" | "bark" | "serverchan" | "email" | "desktop"
	Name      string           `json:"name"`
	Enabled   bool             `json:"enabled"`
	Config    json.RawMessage  `json:"config"`
	Levels    []string         `json:"levels"`
	Types     []string         `json:"types"`
	RateLimit *RateLimitConfig `json:"rateLimit,omitempty"`
}

// ==================== 各渠道专属配置 ====================

// WebhookConfig is the configuration for a webhook notification channel.
type WebhookConfig struct {
	URL               string `json:"url"`
	HMACSecret        string `json:"hmacSecret"`
	Mode              string `json:"mode"`
	TimeoutSeconds    int    `json:"timeoutSeconds"`
	DigestIntervalMin int    `json:"digestIntervalMin"`
	MaxRetries        int    `json:"maxRetries"` // 0=不重试, 默认3
}

// TelegramConfig is the configuration for a Telegram notification channel.
type TelegramConfig struct {
	BotToken string `json:"botToken"`
	ChatID   string `json:"chatId"`
	Mode     string `json:"mode"`
}

// BarkConfig is the configuration for a Bark notification channel.
type BarkConfig struct {
	URL   string `json:"url"`
	Mode  string `json:"mode"`
	Group string `json:"group"`
}

// ServerChanConfig is the configuration for a ServerChan notification channel.
type ServerChanConfig struct {
	SendKey string `json:"sendKey"`
	Mode    string `json:"mode"`
}

// EmailConfig is the configuration for an SMTP email notification channel.
type EmailConfig struct {
	SMTPHost string `json:"smtpHost"`
	SMTPPort int    `json:"smtpPort"`
	SMTPTLS  bool   `json:"smtpTls"`
	Username string `json:"username"`
	Password string `json:"password"`
	FromAddr string `json:"fromAddr"`
	ToAddr   string `json:"toAddr"`
	Mode     string `json:"mode"`
}

// DesktopConfig is the configuration for the in-app desktop notification channel.
type DesktopConfig struct {
	Mode  string `json:"mode"`  // "all" | "failure" | "warning+"
	Sound bool   `json:"sound"` // play sound on notification
}

// ==================== 频率限制 ====================

// RateLimitConfig defines the per-channel send rate limit.
type RateLimitConfig struct {
	MaxPerInterval int `json:"maxPerInterval"` // 区间内最大发送次数，0=不限
	IntervalSec    int `json:"intervalSec"`    // 滑动窗口秒数，默认60
}

// ChannelRateLimiter holds the runtime rate-limit state for a channel.
type ChannelRateLimiter struct {
	mu        sync.Mutex
	sendTimes []time.Time // 最近的发送时间戳
	config    RateLimitConfig
}

// allow checks whether a send is permitted and records the send.
func (rl *ChannelRateLimiter) allow() bool {
	if rl == nil || rl.config.MaxPerInterval <= 0 {
		return true
	}
	rl.mu.Lock()
	defer rl.mu.Unlock()
	window := time.Duration(rl.config.IntervalSec) * time.Second
	if window <= 0 {
		window = 60 * time.Second
	}
	now := time.Now()
	cutoff := now.Add(-window)
	// 移除窗口外的记录
	j := 0
	for _, t := range rl.sendTimes {
		if t.After(cutoff) {
			rl.sendTimes[j] = t
			j++
		}
	}
	rl.sendTimes = rl.sendTimes[:j]
	if len(rl.sendTimes) >= rl.config.MaxPerInterval {
		return false
	}
	rl.sendTimes = append(rl.sendTimes, now)
	return true
}

// ==================== 接口定义 ====================

// Channel is the uniform interface implemented by every notification channel.
type Channel interface {
	Type() string
	Validate() error
	Send(ctx context.Context, kind, level, title, content string) error
	EncryptedFields() []string
}

// DigestChannel extends Channel with digest-mode loop and flush behaviour.
type DigestChannel interface {
	Channel
	StartDigestLoop(ctx context.Context, entries chan DigestEntry)
	FlushDigest(ctx context.Context) error
}

// ==================== 摘要条目 ====================

// DigestEntry is a single notification accumulated for digest-mode webhooks.
type DigestEntry struct {
	Kind    string
	Level   string
	Title   string
	Content string
	Time    time.Time
}

// ==================== 各渠道结构体 ====================

// WebhookChannel sends notifications to an HTTP webhook endpoint.
type WebhookChannel struct {
	httpPort NotificationHTTPPort
	config   WebhookConfig
	name     string
	levels   []string
	types    []string
	// digest 模式
	digestCh chan DigestEntry
	entries  []DigestEntry
	digestMu sync.Mutex
}

// TelegramChannel sends notifications via the Telegram Bot API.
type TelegramChannel struct {
	httpPort NotificationHTTPPort
	config   TelegramConfig
	name     string
	levels   []string
	types    []string
}

// BarkChannel sends notifications via a Bark server.
type BarkChannel struct {
	httpPort NotificationHTTPPort
	config   BarkConfig
	name     string
	levels   []string
	types    []string
}

// ServerChanChannel sends notifications via ServerChan (Server酱).
type ServerChanChannel struct {
	httpPort NotificationHTTPPort
	config   ServerChanConfig
	name     string
	levels   []string
	types    []string
}

// EmailChannel sends notifications via SMTP.
type EmailChannel struct {
	httpPort NotificationHTTPPort
	config   EmailConfig
	name     string
	levels   []string
	types    []string
}

// DesktopChannel is a no-op marker channel. The in-app notification record
// is already inserted by the host application; DesktopChannel.Send()
// previously inserted a duplicate row with related_type='desktop-push', which
// caused duplicate notifications in the frontend (same title/content, different
// id). The frontend does not consume related_type, so the duplicate INSERT was
// pure redundancy. Send() is now a no-op.
type DesktopChannel struct {
	httpPort NotificationHTTPPort
	config   DesktopConfig
	name     string
	levels   []string
	types    []string
}

func (c *DesktopChannel) Name() string     { return c.name }
func (c *DesktopChannel) Type() string     { return "desktop" }
func (c *DesktopChannel) Levels() []string { return c.levels }
func (c *DesktopChannel) Types() []string  { return c.types }
func (c *DesktopChannel) Validate() error  { return nil }

func (c *DesktopChannel) Send(ctx context.Context, kind, level, title, content string) error {
	if !LevelMatchesMode(c.config.Mode, level) {
		return nil
	}
	// No-op: the host application already inserted the in-app notification
	// record. Previously this method inserted a duplicate row with
	// related_type='desktop-push', causing duplicate notifications in the
	// frontend. The frontend does not consume related_type, so we skip the
	// redundant INSERT entirely.
	return nil
}

func (c *DesktopChannel) EncryptedFields() []string { return nil }

// ==================== 配置加载函数 ====================

// DefaultChannelsConfig returns the default (disabled) channels configuration.
func DefaultChannelsConfig() ChannelsConfig {
	return ChannelsConfig{
		Enabled:       false,
		DefaultLevels: []string{"warning", "error"},
		Channels:      nil,
	}
}

// MarshalRaw marshals v to json.RawMessage, ignoring errors (used for tests
// and config building where the value is known to be marshalable).
func MarshalRaw(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

// ParseChannelsConfig parses JSON into a ChannelsConfig, validating and
// normalizing the result. Returns the config and any validation warnings.
func ParseChannelsConfig(valueJSON string) (ChannelsConfig, []string) {
	config := DefaultChannelsConfig()
	var warnings []string
	if strings.TrimSpace(valueJSON) == "" {
		return config, append(warnings, "通知渠道配置为空，使用默认配置。")
	}
	if err := json.Unmarshal([]byte(valueJSON), &config); err != nil {
		return config, []string{"解析通知渠道配置失败: " + err.Error()}
	}
	for i := range config.Channels {
		config.Channels[i].Name = strings.TrimSpace(config.Channels[i].Name)
		if config.Channels[i].Name == "" {
			config.Channels[i].Name = config.Channels[i].Type
		}
	}
	warnings = ValidateChannelsConfig(&config)
	return config, warnings
}

// ValidateChannelsConfig validates the channel configuration set and returns
// any warnings.
func ValidateChannelsConfig(config *ChannelsConfig) []string {
	var warnings []string
	for _, ch := range config.Channels {
		if !ch.Enabled {
			continue
		}
		var err error
		switch ch.Type {
		case "webhook":
			var cfg WebhookConfig
			if json.Unmarshal(ch.Config, &cfg) == nil {
				err = (&WebhookChannel{config: cfg}).Validate()
			}
		case "telegram":
			var cfg TelegramConfig
			if json.Unmarshal(ch.Config, &cfg) == nil {
				err = (&TelegramChannel{config: cfg}).Validate()
			}
		case "bark":
			var cfg BarkConfig
			if json.Unmarshal(ch.Config, &cfg) == nil {
				err = (&BarkChannel{config: cfg}).Validate()
			}
		case "serverchan":
			var cfg ServerChanConfig
			if json.Unmarshal(ch.Config, &cfg) == nil {
				err = (&ServerChanChannel{config: cfg}).Validate()
			}
		case "email":
			var cfg EmailConfig
			if json.Unmarshal(ch.Config, &cfg) == nil {
				err = (&EmailChannel{config: cfg}).Validate()
			}
		case "desktop":
			var cfg DesktopConfig
			if json.Unmarshal(ch.Config, &cfg) == nil {
				err = (&DesktopChannel{config: cfg}).Validate()
			}
		}
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("[%s] %s: %v", ch.Type, ch.Name, err))
		}
	}
	return warnings
}

// ShouldSendToChannel reports whether a notification of the given kind/level
// should be sent to the channel described by entry, taking into account the
// channel's type/level filters and mode.
func ShouldSendToChannel(entry ChannelEntry, kind, level string) bool {
	if len(entry.Types) > 0 {
		if !StringInSlice(kind, entry.Types) {
			return false
		}
	}
	levels := entry.Levels
	if len(levels) == 0 {
		return true
	}
	if !StringInSlice(level, levels) {
		return false
	}
	// 检查渠道的 mode 是否匹配当前通知级别
	var mode string
	switch entry.Type {
	case "webhook":
		var cfg WebhookConfig
		if err := json.Unmarshal(entry.Config, &cfg); err == nil {
			mode = cfg.Mode
		}
	case "telegram":
		var cfg TelegramConfig
		if err := json.Unmarshal(entry.Config, &cfg); err == nil {
			mode = cfg.Mode
		}
	case "bark":
		var cfg BarkConfig
		if err := json.Unmarshal(entry.Config, &cfg); err == nil {
			mode = cfg.Mode
		}
	case "serverchan":
		var cfg ServerChanConfig
		if err := json.Unmarshal(entry.Config, &cfg); err == nil {
			mode = cfg.Mode
		}
	case "email":
		var cfg EmailConfig
		if err := json.Unmarshal(entry.Config, &cfg); err == nil {
			mode = cfg.Mode
		}
	}
	return LevelMatchesMode(mode, level)
}

// LevelMatchesMode reports whether the given level passes the channel mode
// filter.
func LevelMatchesMode(mode, level string) bool {
	switch mode {
	case "all":
		return true
	case "success":
		return level == "success" || level == "info"
	case "failure":
		return level == "warning" || level == "error"
	case "warning+":
		return level == "warning" || level == "error"
	default:
		return true
	}
}

// ==================== WebhookChannel 方法 ====================

func (c *WebhookChannel) Type() string {
	return "webhook"
}

func (c *WebhookChannel) Validate() error {
	if strings.TrimSpace(c.config.URL) == "" {
		return fmt.Errorf("Webhook URL 不能为空")
	}
	return nil
}

func (c *WebhookChannel) Send(ctx context.Context, kind, level, title, content string) error {
	if !LevelMatchesMode(c.config.Mode, level) {
		return nil
	}
	if _, err := c.httpPort.ValidateOutboundURL(ctx, c.config.URL); err != nil {
		return fmt.Errorf("SSRF 验证失败: %w", err)
	}

	timeout := c.config.TimeoutSeconds
	if timeout <= 0 {
		timeout = 10
	}
	if timeout < 3 {
		timeout = 3
	}
	if timeout > 60 {
		timeout = 60
	}

	maxRetries := c.config.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}
	if maxRetries > 5 {
		maxRetries = 5
	}

	bodyMap := map[string]interface{}{
		"type":      kind,
		"level":     level,
		"title":     title,
		"content":   content,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	bodyBytes, err := json.Marshal(bodyMap)
	if err != nil {
		return err
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second // 1s, 2s, 4s, 8s, 16s
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.URL, bytes.NewReader(bodyBytes))
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		if c.config.HMACSecret != "" {
			mac := hmac.New(sha256.New, []byte(c.config.HMACSecret))
			mac.Write(bodyBytes)
			sig := hex.EncodeToString(mac.Sum(nil))
			req.Header.Set("X-Signature-256", sig)
		}

		resp, err := c.httpPort.DoHTTPWithTimeout(req, time.Duration(timeout)*time.Second)
		if err != nil {
			lastErr = err
			log.Printf("[notification] webhook 发送失败 (attempt %d/%d): %v", attempt+1, maxRetries+1, err)
			continue
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		// 4xx 客户端错误不重试（除了 429）
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != 429 {
			return fmt.Errorf("HTTP 状态码 %d（不重试）", resp.StatusCode)
		}
		lastErr = fmt.Errorf("HTTP 状态码 %d", resp.StatusCode)
		log.Printf("[notification] webhook 返回非 2xx (attempt %d/%d): %d", attempt+1, maxRetries+1, resp.StatusCode)
	}
	return lastErr
}

func (c *WebhookChannel) EncryptedFields() []string {
	return []string{"hmacSecret"}
}

// ===== digest 扩展 =====

func (c *WebhookChannel) StartDigestLoop(ctx context.Context, entries chan DigestEntry) {
	interval := time.Duration(c.config.DigestIntervalMin) * time.Minute
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	if interval < time.Minute {
		interval = time.Minute
	}
	if interval > time.Hour {
		interval = time.Hour
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case entry, ok := <-entries:
			if !ok {
				return
			}
			c.digestMu.Lock()
			c.entries = append(c.entries, entry)
			c.digestMu.Unlock()
		case <-ticker.C:
			c.digestMu.Lock()
			if len(c.entries) == 0 {
				c.digestMu.Unlock()
				continue
			}
			snapshot := make([]DigestEntry, len(c.entries))
			copy(snapshot, c.entries)
			c.entries = nil
			c.digestMu.Unlock()

			go func() {
				if err := c.sendDigest(ctx, snapshot); err != nil {
					log.Printf("[notification] webhook digest 发送失败: %v", err)
				}
			}()
		}
	}
}

func (c *WebhookChannel) FlushDigest(ctx context.Context) error {
	c.digestMu.Lock()
	if len(c.entries) == 0 {
		c.digestMu.Unlock()
		return nil
	}
	snapshot := make([]DigestEntry, len(c.entries))
	copy(snapshot, c.entries)
	c.entries = nil
	c.digestMu.Unlock()
	return c.sendDigest(ctx, snapshot)
}

func (c *WebhookChannel) sendDigest(ctx context.Context, entries []DigestEntry) error {
	type digestItem struct {
		Kind      string `json:"kind"`
		Level     string `json:"level"`
		Title     string `json:"title"`
		Content   string `json:"content"`
		Timestamp string `json:"timestamp"`
	}
	items := make([]digestItem, 0, len(entries))
	for _, e := range entries {
		items = append(items, digestItem{
			Kind:      e.Kind,
			Level:     e.Level,
			Title:     e.Title,
			Content:   TruncateNotifyContent(e.Content, 2000),
			Timestamp: e.Time.UTC().Format(time.RFC3339),
		})
	}
	digestMap := map[string]interface{}{
		"type":      "digest",
		"count":     len(items),
		"entries":   items,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	bodyBytes, _ := json.Marshal(digestMap)

	timeout := c.config.TimeoutSeconds
	if timeout <= 0 {
		timeout = 10
	}
	if timeout < 3 {
		timeout = 3
	}
	if timeout > 60 {
		timeout = 60
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.URL, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	if c.config.HMACSecret != "" {
		mac := hmac.New(sha256.New, []byte(c.config.HMACSecret))
		mac.Write(bodyBytes)
		sig := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-Signature-256", sig)
	}

	resp, err := c.httpPort.DoHTTPWithTimeout(req, time.Duration(timeout)*time.Second)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP 状态码 %d", resp.StatusCode)
	}
	return nil
}

// ==================== TelegramChannel 方法 ====================

func (c *TelegramChannel) Type() string {
	return "telegram"
}

func (c *TelegramChannel) Validate() error {
	if strings.TrimSpace(c.config.BotToken) == "" {
		return fmt.Errorf("Bot Token 不能为空")
	}
	if strings.TrimSpace(c.config.ChatID) == "" {
		return fmt.Errorf("Chat ID 不能为空")
	}
	return nil
}

func (c *TelegramChannel) Send(ctx context.Context, kind, level, title, content string) error {
	if !LevelMatchesMode(c.config.Mode, level) {
		return nil
	}
	text := fmt.Sprintf(
		"<b>RelayCheck 通知</b>\n类型: %s\n等级: %s\n标题: %s\n内容: %s",
		kind, level, title, content,
	)
	bodyMap := map[string]interface{}{
		"chat_id":    c.config.ChatID,
		"text":       text,
		"parse_mode": "HTML",
	}
	bodyBytes, _ := json.Marshal(bodyMap)

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", c.config.BotToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpPort.DoHTTPWithTimeout(req, 10*time.Second)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Telegram API 返回 HTTP 状态码 %d", resp.StatusCode)
	}
	return nil
}

func (c *TelegramChannel) EncryptedFields() []string {
	return []string{"botToken"}
}

// ==================== BarkChannel 方法 ====================

func (c *BarkChannel) Type() string {
	return "bark"
}

func (c *BarkChannel) Validate() error {
	if strings.TrimSpace(c.config.URL) == "" {
		return fmt.Errorf("Bark URL 不能为空")
	}
	return nil
}

func (c *BarkChannel) Send(ctx context.Context, kind, level, title, content string) error {
	if !LevelMatchesMode(c.config.Mode, level) {
		return nil
	}
	if _, err := c.httpPort.ValidateOutboundURL(ctx, c.config.URL); err != nil {
		return fmt.Errorf("SSRF 验证失败: %w", err)
	}

	group := c.config.Group
	if group == "" {
		group = "RelayCheck"
	}
	safeTitle := url.PathEscape(title)
	safeContent := url.PathEscape(TruncateNotifyContent(content, 2000))
	fullURL := fmt.Sprintf("%s/%s/%s?group=%s&autoCopy=1",
		strings.TrimRight(c.config.URL, "/"),
		safeTitle, safeContent, url.QueryEscape(group))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpPort.DoHTTPWithTimeout(req, 10*time.Second)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Bark 返回 HTTP 状态码 %d", resp.StatusCode)
	}
	return nil
}

func (c *BarkChannel) EncryptedFields() []string {
	return nil
}

// ==================== ServerChanChannel 方法 ====================

func (c *ServerChanChannel) Type() string {
	return "serverchan"
}

func (c *ServerChanChannel) Validate() error {
	if strings.TrimSpace(c.config.SendKey) == "" {
		return fmt.Errorf("SendKey 不能为空")
	}
	return nil
}

func (c *ServerChanChannel) Send(ctx context.Context, kind, level, title, content string) error {
	if !LevelMatchesMode(c.config.Mode, level) {
		return nil
	}
	bodyMap := map[string]interface{}{
		"title":   title,
		"content": content,
		"channel": 9,
	}
	bodyBytes, _ := json.Marshal(bodyMap)

	apiURL := fmt.Sprintf("https://sctapi.ftqq.com/%s.send", c.config.SendKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpPort.DoHTTPWithTimeout(req, 10*time.Second)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ServerChan API 返回 HTTP 状态码 %d", resp.StatusCode)
	}
	return nil
}

func (c *ServerChanChannel) EncryptedFields() []string {
	return []string{"sendKey"}
}

// ==================== EmailChannel 方法 ====================

func (c *EmailChannel) Type() string {
	return "email"
}

func (c *EmailChannel) Validate() error {
	if strings.TrimSpace(c.config.SMTPHost) == "" {
		return fmt.Errorf("SMTP 服务器地址不能为空")
	}
	if strings.TrimSpace(c.config.FromAddr) == "" {
		return fmt.Errorf("发件人地址不能为空")
	}
	if strings.TrimSpace(c.config.ToAddr) == "" {
		return fmt.Errorf("收件人地址不能为空")
	}
	return nil
}

func (c *EmailChannel) Send(ctx context.Context, kind, level, title, content string) error {
	if !LevelMatchesMode(c.config.Mode, level) {
		return nil
	}

	host := strings.TrimSpace(c.config.SMTPHost)
	port := c.config.SMTPPort
	if port <= 0 {
		if c.config.SMTPTLS {
			port = 465
		} else {
			port = 587
		}
	}
	fromAddr := strings.TrimSpace(c.config.FromAddr)
	toAddr := strings.TrimSpace(c.config.ToAddr)
	username := strings.TrimSpace(c.config.Username)
	password := c.config.Password

	msg := BuildEmailMessage(fromAddr, toAddr, title, content)
	addr := fmt.Sprintf("%s:%d", host, port)

	if c.config.SMTPTLS && port == 465 {
		// 直接 TLS 连接 (SMTPS port 465)
		tlsConfig := &tls.Config{ServerName: host}
		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return fmt.Errorf("TLS 连接失败: %w", err)
		}
		client, err := smtp.NewClient(conn, host)
		if err != nil {
			conn.Close()
			return fmt.Errorf("SMTP 客户端创建失败: %w", err)
		}
		defer client.Close()

		if username != "" || password != "" {
			auth := smtp.PlainAuth("", username, password, host)
			if err := client.Auth(auth); err != nil {
				return fmt.Errorf("SMTP 认证失败: %w", err)
			}
		}
		if err := client.Mail(fromAddr); err != nil {
			return fmt.Errorf("发件人地址错误: %w", err)
		}
		if err := client.Rcpt(toAddr); err != nil {
			return fmt.Errorf("收件人地址错误: %w", err)
		}
		w, err := client.Data()
		if err != nil {
			return fmt.Errorf("SMTP 数据通道失败: %w", err)
		}
		if _, err := w.Write([]byte(msg)); err != nil {
			w.Close()
			return fmt.Errorf("邮件内容写入失败: %w", err)
		}
		w.Close()
		client.Quit()
		return nil
	}

	// STARTTLS (port 587) 或明文 25
	auth := smtp.PlainAuth("", username, password, host)
	if err := smtp.SendMail(addr, auth, fromAddr, []string{toAddr}, []byte(msg)); err != nil {
		return fmt.Errorf("邮件发送失败: %w", err)
	}
	return nil
}

func (c *EmailChannel) EncryptedFields() []string {
	return []string{"password"}
}

// ==================== 工具函数 ====================

// BuildNotifyBody formats a plain-text notification body.
func BuildNotifyBody(kind, level, title, content string) string {
	return fmt.Sprintf("类型: %s\n等级: %s\n标题: %s\n内容: %s", kind, level, title, content)
}

// MaskSensitiveField masks all but the last 4 characters of value. Short
// values (<=4 chars) are fully masked.
func MaskSensitiveField(value string) string {
	if value == "" {
		return ""
	}
	if len(value) <= 4 {
		return strings.Repeat("*", len(value))
	}
	return strings.Repeat("*", len(value)-4) + value[len(value)-4:]
}

// TruncateNotifyContent truncates content to maxLen runes, appending "..." if
// truncation occurred.
func TruncateNotifyContent(content string, maxLen int) string {
	runes := []rune(content)
	if len(runes) <= maxLen {
		return content
	}
	return string(runes[:maxLen]) + "..."
}

// StringInSlice reports whether s is present in list.
func StringInSlice(s string, list []string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

// BuildEmailMessage builds an RFC 822 email message string.
func BuildEmailMessage(fromAddr, toAddr, title, content string) string {
	subject := mime.BEncoding.Encode("utf-8", title)
	body := TruncateNotifyContent(content, 20000)

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("From: %s\r\n", fromAddr))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", toAddr))
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
	buf.WriteString("\r\n")
	buf.WriteString(body)
	return buf.String()
}
