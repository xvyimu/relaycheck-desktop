package core

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
	"strconv"
	"strings"
	"sync"
	"time"
)

// ==================== 配置结构体 ====================

type notificationChannelsConfig struct {
	Enabled       bool           `json:"enabled"`
	DefaultLevels []string       `json:"defaultLevels"`
	Channels      []channelEntry `json:"channels"`
}

type channelEntry struct {
	Type      string           `json:"type"` // "webhook" | "telegram" | "bark" | "serverchan" | "email"
	Name      string           `json:"name"`
	Enabled   bool             `json:"enabled"`
	Config    json.RawMessage  `json:"config"`
	Levels    []string         `json:"levels"`
	Types     []string         `json:"types"`
	RateLimit *rateLimitConfig `json:"rateLimit,omitempty"`
}

// ==================== 各渠道专属配置 ====================

type webhookConfig struct {
	URL               string `json:"url"`
	HMACSecret        string `json:"hmacSecret"`
	Mode              string `json:"mode"`
	TimeoutSeconds    int    `json:"timeoutSeconds"`
	DigestIntervalMin int    `json:"digestIntervalMin"`
	MaxRetries        int    `json:"maxRetries"` // 0=不重试, 默认3
}

type telegramConfig struct {
	BotToken string `json:"botToken"`
	ChatID   string `json:"chatId"`
	Mode     string `json:"mode"`
}

type barkConfig struct {
	URL   string `json:"url"`
	Mode  string `json:"mode"`
	Group string `json:"group"`
}

type serverchanConfig struct {
	SendKey string `json:"sendKey"`
	Mode    string `json:"mode"`
}

type emailConfig struct {
	SMTPHost string `json:"smtpHost"`
	SMTPPort int    `json:"smtpPort"`
	SMTPTLS  bool   `json:"smtpTls"`
	Username string `json:"username"`
	Password string `json:"password"`
	FromAddr string `json:"fromAddr"`
	ToAddr   string `json:"toAddr"`
	Mode     string `json:"mode"`
}

type desktopConfig struct {
	Mode  string `json:"mode"`  // "all" | "failure" | "warning+"
	Sound bool   `json:"sound"` // play sound on notification
}

// desktopChannel is a no-op marker channel. The in-app notification record
// is already inserted by App.notify() in routes.go; desktopChannel.Send()
// previously inserted a duplicate row with related_type='desktop-push', which
// caused duplicate notifications in the frontend (same title/content, different
// id). The frontend does not consume related_type, so the duplicate INSERT was
// pure redundancy. Send() is now a no-op.
type desktopChannel struct {
	httpPort NotificationHTTPPort
	config   desktopConfig
	name     string
	levels   []string
	types    []string
}

func (c *desktopChannel) Name() string     { return c.name }
func (c *desktopChannel) Type() string     { return "desktop" }
func (c *desktopChannel) Levels() []string { return c.levels }
func (c *desktopChannel) Types() []string  { return c.types }
func (c *desktopChannel) Validate() error  { return nil }

func (c *desktopChannel) Send(ctx context.Context, kind, level, title, content string) error {
	if !levelMatchesMode(c.config.Mode, level) {
		return nil
	}
	// No-op: App.notify() already inserted the in-app notification record.
	// Previously this method inserted a duplicate row with
	// related_type='desktop-push', causing duplicate notifications in the
	// frontend. The frontend does not consume related_type, so we skip the
	// redundant INSERT entirely.
	return nil
}

func (c *desktopChannel) EncryptedFields() []string { return nil }

// ==================== 摘要积累 ====================

// ==================== 频率限制 ====================

// rateLimitConfig 定义渠道发送频率限制
type rateLimitConfig struct {
	MaxPerInterval int `json:"maxPerInterval"` // 区间内最大发送次数，0=不限
	IntervalSec    int `json:"intervalSec"`    // 滑动窗口秒数，默认60
}

// channelRateLimiter 运行时频率限制状态
type channelRateLimiter struct {
	mu        sync.Mutex
	sendTimes []time.Time // 最近的发送时间戳
	config    rateLimitConfig
}

// allow 检查是否允许发送，并记录本次发送
func (rl *channelRateLimiter) allow() bool {
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

type notificationChannel interface {
	Type() string
	Validate() error
	Send(ctx context.Context, kind, level, title, content string) error
	EncryptedFields() []string
}

type digestChannel interface {
	notificationChannel
	StartDigestLoop(ctx context.Context, entries chan digestEntry)
	FlushDigest(ctx context.Context) error
}

// ==================== 各渠道结构体 ====================

type webhookChannel struct {
	httpPort NotificationHTTPPort
	config   webhookConfig
	name     string
	levels   []string
	types    []string
	// digest 模式
	digestCh chan digestEntry
	entries  []digestEntry
	digestMu sync.Mutex
}

type telegramChannel struct {
	httpPort NotificationHTTPPort
	config   telegramConfig
	name     string
	levels   []string
	types    []string
}

type barkChannel struct {
	httpPort NotificationHTTPPort
	config   barkConfig
	name     string
	levels   []string
	types    []string
}

type serverchanChannel struct {
	httpPort NotificationHTTPPort
	config   serverchanConfig
	name     string
	levels   []string
	types    []string
}

type emailChannel struct {
	httpPort NotificationHTTPPort
	config   emailConfig
	name     string
	levels   []string
	types    []string
}

// ==================== 配置加载函数 ====================

func defaultNotificationChannelsConfig() notificationChannelsConfig {
	return notificationChannelsConfig{
		Enabled:       false,
		DefaultLevels: []string{"warning", "error"},
		Channels:      nil,
	}
}

func marshalRaw(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

// parseNotificationChannelsConfig 解析 JSON 为配置结构体，验证并归一化
func parseNotificationChannelsConfig(valueJSON string) (notificationChannelsConfig, []string) {
	config := defaultNotificationChannelsConfig()
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
	warnings = validateNotificationChannelsConfig(&config)
	return config, warnings
}

// validateNotificationChannelsConfig 验证渠道配置集合
func validateNotificationChannelsConfig(config *notificationChannelsConfig) []string {
	var warnings []string
	for _, ch := range config.Channels {
		if !ch.Enabled {
			continue
		}
		var err error
		switch ch.Type {
		case "webhook":
			var cfg webhookConfig
			if json.Unmarshal(ch.Config, &cfg) == nil {
				err = (&webhookChannel{config: cfg}).Validate()
			}
		case "telegram":
			var cfg telegramConfig
			if json.Unmarshal(ch.Config, &cfg) == nil {
				err = (&telegramChannel{config: cfg}).Validate()
			}
		case "bark":
			var cfg barkConfig
			if json.Unmarshal(ch.Config, &cfg) == nil {
				err = (&barkChannel{config: cfg}).Validate()
			}
		case "serverchan":
			var cfg serverchanConfig
			if json.Unmarshal(ch.Config, &cfg) == nil {
				err = (&serverchanChannel{config: cfg}).Validate()
			}
		case "email":
			var cfg emailConfig
			if json.Unmarshal(ch.Config, &cfg) == nil {
				err = (&emailChannel{config: cfg}).Validate()
			}
		case "desktop":
			var cfg desktopConfig
			if json.Unmarshal(ch.Config, &cfg) == nil {
				err = (&desktopChannel{config: cfg}).Validate()
			}
		}
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("[%s] %s: %v", ch.Type, ch.Name, err))
		}
	}
	return warnings
}

// ==================== App 方法（转发至 NotificationHub）====================

func (a *App) reloadNotificationConfig(ctx context.Context) error {
	return a.notificationHub.Reload(ctx)
}

func (a *App) loadNotificationChannelsConfig(ctx context.Context) (notificationChannelsConfig, error) {
	return a.notificationHub.LoadConfig(ctx)
}

func (a *App) currentNotificationChannelsConfig() notificationChannelsConfig {
	return a.notificationHub.CurrentConfig()
}

// digestEntry 用于 digest 模式的消息条目。
type digestEntry struct {
	Kind    string
	Level   string
	Title   string
	Content string
	Time    time.Time
}

func (a *App) buildChannelFromConfig(entry channelEntry) notificationChannel {
	return a.notificationHub.BuildChannel(entry)
}

func (a *App) encryptChannelEntrySecrets(entry *channelEntry) error {
	return a.notificationHub.EncryptEntrySecrets(entry)
}

func (a *App) decryptChannelEntrySecrets(entry *channelEntry) error {
	return a.notificationHub.DecryptEntrySecrets(entry)
}

func (a *App) dispatchNotification(kind, level, title, content string) {
	a.notificationHub.Dispatch(kind, level, title, content)
}

func shouldSendToChannel(entry channelEntry, kind, level string) bool {
	if len(entry.Types) > 0 {
		if !stringInSlice(kind, entry.Types) {
			return false
		}
	}
	levels := entry.Levels
	if len(levels) == 0 {
		return true
	}
	if !stringInSlice(level, levels) {
		return false
	}
	// 检查渠道的 mode 是否匹配当前通知级别
	var mode string
	switch entry.Type {
	case "webhook":
		var cfg webhookConfig
		if err := json.Unmarshal(entry.Config, &cfg); err == nil {
			mode = cfg.Mode
		}
	case "telegram":
		var cfg telegramConfig
		if err := json.Unmarshal(entry.Config, &cfg); err == nil {
			mode = cfg.Mode
		}
	case "bark":
		var cfg barkConfig
		if err := json.Unmarshal(entry.Config, &cfg); err == nil {
			mode = cfg.Mode
		}
	case "serverchan":
		var cfg serverchanConfig
		if err := json.Unmarshal(entry.Config, &cfg); err == nil {
			mode = cfg.Mode
		}
	case "email":
		var cfg emailConfig
		if err := json.Unmarshal(entry.Config, &cfg); err == nil {
			mode = cfg.Mode
		}
	}
	return levelMatchesMode(mode, level)
}

func levelMatchesMode(mode, level string) bool {
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

// ==================== webhookChannel 方法 ====================

func (c *webhookChannel) Type() string {
	return "webhook"
}

func (c *webhookChannel) Validate() error {
	if strings.TrimSpace(c.config.URL) == "" {
		return fmt.Errorf("Webhook URL 不能为空")
	}
	return nil
}

func (c *webhookChannel) Send(ctx context.Context, kind, level, title, content string) error {
	if !levelMatchesMode(c.config.Mode, level) {
		return nil
	}
	policy := c.httpPort.externalURLPolicy()
	if _, err := validateOutboundHTTPURL(ctx, c.config.URL, policy); err != nil {
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

		resp, err := c.httpPort.doHTTPWithTimeout(req, time.Duration(timeout)*time.Second)
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

func (c *webhookChannel) EncryptedFields() []string {
	return []string{"hmacSecret"}
}

// ===== digest 扩展 =====

func (c *webhookChannel) StartDigestLoop(ctx context.Context, entries chan digestEntry) {
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
			snapshot := make([]digestEntry, len(c.entries))
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

func (c *webhookChannel) FlushDigest(ctx context.Context) error {
	c.digestMu.Lock()
	if len(c.entries) == 0 {
		c.digestMu.Unlock()
		return nil
	}
	snapshot := make([]digestEntry, len(c.entries))
	copy(snapshot, c.entries)
	c.entries = nil
	c.digestMu.Unlock()
	return c.sendDigest(ctx, snapshot)
}

func (c *webhookChannel) sendDigest(ctx context.Context, entries []digestEntry) error {
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
			Content:   truncateNotifyContent(e.Content, 2000),
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

	resp, err := c.httpPort.doHTTPWithTimeout(req, time.Duration(timeout)*time.Second)
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

// ==================== telegramChannel 方法 ====================

func (c *telegramChannel) Type() string {
	return "telegram"
}

func (c *telegramChannel) Validate() error {
	if strings.TrimSpace(c.config.BotToken) == "" {
		return fmt.Errorf("Bot Token 不能为空")
	}
	if strings.TrimSpace(c.config.ChatID) == "" {
		return fmt.Errorf("Chat ID 不能为空")
	}
	return nil
}

func (c *telegramChannel) Send(ctx context.Context, kind, level, title, content string) error {
	if !levelMatchesMode(c.config.Mode, level) {
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

	resp, err := c.httpPort.doHTTPWithTimeout(req, 10*time.Second)
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

func (c *telegramChannel) EncryptedFields() []string {
	return []string{"botToken"}
}

// ==================== barkChannel 方法 ====================

func (c *barkChannel) Type() string {
	return "bark"
}

func (c *barkChannel) Validate() error {
	if strings.TrimSpace(c.config.URL) == "" {
		return fmt.Errorf("Bark URL 不能为空")
	}
	return nil
}

func (c *barkChannel) Send(ctx context.Context, kind, level, title, content string) error {
	if !levelMatchesMode(c.config.Mode, level) {
		return nil
	}
	policy := c.httpPort.externalURLPolicy()
	if _, err := validateOutboundHTTPURL(ctx, c.config.URL, policy); err != nil {
		return fmt.Errorf("SSRF 验证失败: %w", err)
	}

	group := c.config.Group
	if group == "" {
		group = "RelayCheck"
	}
	safeTitle := url.PathEscape(title)
	safeContent := url.PathEscape(truncateNotifyContent(content, 2000))
	fullURL := fmt.Sprintf("%s/%s/%s?group=%s&autoCopy=1",
		strings.TrimRight(c.config.URL, "/"),
		safeTitle, safeContent, url.QueryEscape(group))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpPort.doHTTPWithTimeout(req, 10*time.Second)
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

func (c *barkChannel) EncryptedFields() []string {
	return nil
}

// ==================== serverchanChannel 方法 ====================

func (c *serverchanChannel) Type() string {
	return "serverchan"
}

func (c *serverchanChannel) Validate() error {
	if strings.TrimSpace(c.config.SendKey) == "" {
		return fmt.Errorf("SendKey 不能为空")
	}
	return nil
}

func (c *serverchanChannel) Send(ctx context.Context, kind, level, title, content string) error {
	if !levelMatchesMode(c.config.Mode, level) {
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

	resp, err := c.httpPort.doHTTPWithTimeout(req, 10*time.Second)
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

func (c *serverchanChannel) EncryptedFields() []string {
	return []string{"sendKey"}
}

// ==================== emailChannel 方法 ====================

func (c *emailChannel) Type() string {
	return "email"
}

func (c *emailChannel) Validate() error {
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

func (c *emailChannel) Send(ctx context.Context, kind, level, title, content string) error {
	if !levelMatchesMode(c.config.Mode, level) {
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

	msg := buildEmailMessage(fromAddr, toAddr, title, content)
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

func (c *emailChannel) EncryptedFields() []string {
	return []string{"password"}
}

// ==================== 工具函数 ====================

func buildNotifyBody(kind, level, title, content string) string {
	return fmt.Sprintf("类型: %s\n等级: %s\n标题: %s\n内容: %s", kind, level, title, content)
}

func maskSensitiveField(value string) string {
	if value == "" {
		return ""
	}
	if len(value) <= 4 {
		return strings.Repeat("*", len(value))
	}
	return strings.Repeat("*", len(value)-4) + value[len(value)-4:]
}

func truncateNotifyContent(content string, maxLen int) string {
	runes := []rune(content)
	if len(runes) <= maxLen {
		return content
	}
	return string(runes[:maxLen]) + "..."
}

func stringInSlice(s string, list []string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

func buildEmailMessage(fromAddr, toAddr, title, content string) string {
	subject := mime.BEncoding.Encode("utf-8", title)
	body := truncateNotifyContent(content, 20000)

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

// ==================== 通知入库与去重（从 routes.go 归拢）====================

func (a *App) notify(kind, level, title, content, relatedType, relatedID string) {
	// Deduplicate: skip if an identical notification (same kind+relatedType+
	// relatedID+content) was inserted within the dedup window. This prevents
	// recurring events (e.g. "checkin_unsupported" for sites without a checkin
	// endpoint) from flooding the notification table on every scheduler tick.
	dedupWindow := 30 * time.Minute
	if kind == "scheduled_channel_health_probe_warning" {
		dedupWindow = 30 * time.Minute
	}
	if a.recentNotificationExists(context.Background(), kind, relatedType, relatedID, content, dedupWindow) {
		return
	}
	if _, execErr := a.db.Exec(`
		INSERT INTO app_notifications (id, type, level, title, content, read, related_type, related_id, created_at)
		VALUES (?, ?, ?, ?, ?, 0, ?, ?, ?)
	`, newID(), kind, level, title, content, relatedType, relatedID, now()); execErr != nil {
		log.Printf("[notify] notification insert failed: %v", execErr)
	}
	a.invalidateReadCache()

	// 异步分发到外部通知渠道
	go a.dispatchNotification(kind, level, title, content)
}

func (a *App) recentNotificationExists(ctx context.Context, kind string, relatedType string, relatedID string, content string, window time.Duration) bool {
	if window <= 0 {
		return false
	}
	cutoff := time.Now().Add(-window).UTC().Format(time.RFC3339Nano)
	var count int
	err := a.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM app_notifications
		WHERE type=?
		  AND related_type=?
		  AND related_id=?
		  AND content=?
		  AND created_at >= ?
	`, kind, relatedType, relatedID, content, cutoff).Scan(&count)
	return err == nil && count > 0
}

// ==================== 通知 HTTP handler（从 routes.go 归拢）====================

func (a *App) handleNotifications(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	levelFilter := r.URL.Query().Get("level")
	typeFilter := r.URL.Query().Get("type")
	unreadOnly := r.URL.Query().Get("unread") == "1"
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			limit = v
		}
	}
	limit = clampBatchLimit(limit, 100)
	offset := 0
	if raw := r.URL.Query().Get("offset"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 0 {
			offset = v
		}
	}

	query := `SELECT id, type, level, title, content, read, created_at FROM app_notifications WHERE 1=1`
	var args []interface{}
	if levelFilter != "" {
		query += ` AND level = ?`
		args = append(args, levelFilter)
	}
	if typeFilter != "" {
		query += ` AND type = ?`
		args = append(args, typeFilter)
	}
	if unreadOnly {
		query += ` AND read = 0`
	}
	query += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := a.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	items := []Notification{}
	for rows.Next() {
		var item Notification
		var read int
		if err := rows.Scan(&item.ID, &item.Type, &item.Level, &item.Title, &item.Content, &read, &item.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		item.Read = read == 1
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (a *App) handleMarkAllNotificationsRead(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	_, err := a.db.ExecContext(r.Context(), `UPDATE app_notifications SET read = 1 WHERE read = 0`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.invalidateReadCache()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) handleClearReadNotifications(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	_, err := a.db.ExecContext(r.Context(), `DELETE FROM app_notifications WHERE read = 1`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.invalidateReadCache()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) handleTrimNotifications(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	keep := 10
	if k := r.URL.Query().Get("keep"); k != "" {
		if n, err := strconv.Atoi(k); err == nil && n > 0 {
			keep = n
		}
	}
	_, err := a.db.ExecContext(r.Context(),
		`DELETE FROM app_notifications WHERE id NOT IN (SELECT id FROM app_notifications ORDER BY created_at DESC LIMIT ?)`, keep)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.invalidateReadCache()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) handleMarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var body struct {
		ID        string `json:"id"`
		AllOfType string `json:"allOfType"` // mark all of a type as read
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败")
		return
	}
	if body.AllOfType != "" {
		_, err := a.db.ExecContext(r.Context(), `UPDATE app_notifications SET read = 1 WHERE type = ? AND read = 0`, body.AllOfType)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	} else if body.ID != "" {
		_, err := a.db.ExecContext(r.Context(), `UPDATE app_notifications SET read = 1 WHERE id = ?`, body.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	a.invalidateReadCache()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ==================== 通知健康检查（从 health.go 归拢）====================

func (a *App) healthCheckNotificationChannels() HealthCheck {
	config := a.currentNotificationChannelsConfig()
	if !config.Enabled {
		return HealthCheck{ID: "notification", Label: "通知渠道", Status: "ok", Message: "外部通知未启用。"}
	}
	enabledCount := 0
	totalCount := len(config.Channels)
	for _, ch := range config.Channels {
		if ch.Enabled {
			enabledCount++
		}
	}
	if enabledCount == 0 {
		return HealthCheck{ID: "notification", Label: "通知渠道", Status: "warning", Message: "外部通知已启用，但未启用任何渠道。"}
	}
	return HealthCheck{ID: "notification", Label: "通知渠道", Status: "ok", Message: fmt.Sprintf("已启用 %d/%d 个外部通知渠道。", enabledCount, totalCount)}
}

// ==================== 默认配置注入（从 app.go 归拢）====================

func withDefaultHealthNotificationTypes(valueJSON string) string {
	config, _ := parseNotificationChannelsConfig(valueJSON)
	for index := range config.Channels {
		switch config.Channels[index].Type {
		case "webhook", "telegram":
			appendUniqueString(&config.Channels[index].Types, "scheduled_channel_health_probe_failed", 20)
			appendUniqueString(&config.Channels[index].Types, "scheduled_channel_health_probe_warning", 20)
		}
	}
	body, err := json.Marshal(config)
	if err != nil {
		return valueJSON
	}
	return string(body)
}
