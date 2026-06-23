package core

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"database/sql"
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

type notificationChannelsConfig struct {
	Enabled       bool           `json:"enabled"`
	DefaultLevels []string       `json:"defaultLevels"`
	Channels      []channelEntry `json:"channels"`
}

type channelEntry struct {
	Type    string          `json:"type"`    // "webhook" | "telegram" | "bark" | "serverchan" | "email"
	Name    string          `json:"name"`
	Enabled bool            `json:"enabled"`
	Config  json.RawMessage `json:"config"`
	Levels  []string        `json:"levels"`
	Types   []string        `json:"types"`
	RateLimit *rateLimitConfig `json:"rateLimit,omitempty"`
}

// ==================== 各渠道专属配置 ====================

type webhookConfig struct {
	URL                string `json:"url"`
	HMACSecret         string `json:"hmacSecret"`
	Mode               string `json:"mode"`
	TimeoutSeconds     int    `json:"timeoutSeconds"`
	DigestIntervalMin  int    `json:"digestIntervalMin"`
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

// ==================== 摘要积累 ====================

// ==================== 频率限制 ====================

// rateLimitConfig 定义渠道发送频率限制
type rateLimitConfig struct {
	MaxPerInterval int `json:"maxPerInterval"` // 区间内最大发送次数，0=不限
	IntervalSec    int `json:"intervalSec"`    // 滑动窗口秒数，默认60
}

// channelRateLimiter 运行时频率限制状态
type channelRateLimiter struct {
	mu         sync.Mutex
	sendTimes  []time.Time // 最近的发送时间戳
	config     rateLimitConfig
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
	app    *App
	config webhookConfig
	name   string
	levels []string
	types  []string
	// digest 模式
	digestCh chan digestEntry
	entries  []digestEntry
	digestMu sync.Mutex
}

type telegramChannel struct {
	app    *App
	config telegramConfig
	name   string
	levels []string
	types  []string
}

type barkChannel struct {
	app    *App
	config barkConfig
	name   string
	levels []string
	types  []string
}

type serverchanChannel struct {
	app    *App
	config serverchanConfig
	name   string
	levels []string
	types  []string
}

type emailChannel struct {
	app    *App
	config emailConfig
	name   string
	levels []string
	types  []string
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
		}
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("[%s] %s: %v", ch.Type, ch.Name, err))
		}
	}
	return warnings
}

// ==================== App 方法 ====================

func (a *App) reloadNotificationConfig(ctx context.Context) error {
	if a.db == nil {
		return nil
	}
	// 停止旧的 digest 循环（如有）
	if cancel := func() context.CancelFunc {
		a.mu.Lock()
		defer a.mu.Unlock()
		c := a.digestCancel
		a.digestCancel = nil
		a.digestWG = sync.WaitGroup{}
		a.digestChannels = map[string]*webhookChannel{}
		a.channelRateLimits = map[string]*channelRateLimiter{}
		return c
	}(); cancel != nil {
		cancel()
		a.digestWG.Wait()
	}
	config, err := a.loadNotificationChannelsConfig(ctx)
	if err != nil {
		config = defaultNotificationChannelsConfig()
	} else {
		for i := range config.Channels {
			if decErr := a.decryptChannelEntrySecrets(&config.Channels[i]); decErr != nil {
				log.Printf("[notification] 解密渠道 %s 密钥失败: %v", config.Channels[i].Name, decErr)
			}
		}
	}
	a.mu.Lock()
	a.notificationConfig = config
	// 为 digest 模式的 webhook 启动专属 goroutine
	for _, entry := range config.Channels {
		if !entry.Enabled || entry.Type != "webhook" {
			continue
		}
		var cfg webhookConfig
		if err := json.Unmarshal(entry.Config, &cfg); err != nil {
			continue
		}
		if cfg.Mode != "digest" {
			continue
		}
		ch := &webhookChannel{
			app:    a,
			config: cfg,
			name:   entry.Name,
			levels: entry.Levels,
			types:  entry.Types,
			entries: []digestEntry{},
			digestCh: make(chan digestEntry, 100),
		}
		if err := ch.Validate(); err != nil {
			log.Printf("[notification] digest webhook %q 验证失败: %v", entry.Name, err)
			continue
		}
		a.digestChannels[entry.Name] = ch
		digestCtx, digestCancel := context.WithCancel(context.Background())
		a.mu.Lock()
		a.digestCancel = digestCancel
		a.mu.Unlock()
		a.digestWG.Add(1)
		go func(c *webhookChannel) {
			defer a.digestWG.Done()
			c.StartDigestLoop(digestCtx, c.digestCh)
		}(ch)
	}
	// 初始化频率限制器
	for _, entry := range config.Channels {
		if !entry.Enabled || entry.RateLimit == nil || entry.RateLimit.MaxPerInterval <= 0 {
			continue
		}
		a.channelRateLimits[entry.Name] = &channelRateLimiter{
			config: *entry.RateLimit,
		}
	}
	a.mu.Unlock()
	return nil
}

func (a *App) loadNotificationChannelsConfig(ctx context.Context) (notificationChannelsConfig, error) {
	if a.db == nil {
		return defaultNotificationChannelsConfig(), nil
	}
	var valueJSON string
	err := a.db.QueryRowContext(ctx, `SELECT value_json FROM system_settings WHERE key='notification.channels'`).Scan(&valueJSON)
	if err == sql.ErrNoRows {
		return defaultNotificationChannelsConfig(), nil
	}
	if err != nil {
		return defaultNotificationChannelsConfig(), err
	}
	config, _ := parseNotificationChannelsConfig(valueJSON)
	return config, nil
}

func (a *App) currentNotificationChannelsConfig() notificationChannelsConfig {
	if a == nil {
		return defaultNotificationChannelsConfig()
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.notificationConfig
}

// ==================== 渠道工厂 ====================

// digestEntry 用于 digest 模式的消息条目。
type digestEntry struct {
	Kind      string
	Level     string
	Title     string
	Content   string
	Time      time.Time
}

func (a *App) buildChannelFromConfig(entry channelEntry) notificationChannel {
	if entry.Config == nil {
		return nil
	}
	switch entry.Type {
	case "webhook":
		var cfg webhookConfig
		if err := json.Unmarshal(entry.Config, &cfg); err != nil {
			return nil
		}
		ch := &webhookChannel{
			app:    a,
			config: cfg,
			name:   entry.Name,
			levels: entry.Levels,
			types:  entry.Types,
		}
		if err := ch.Validate(); err != nil {
			return nil
		}
		if cfg.Mode == "digest" {
			ch.digestCh = make(chan digestEntry, 100)
		}
		return ch
	case "telegram":
		var cfg telegramConfig
		if err := json.Unmarshal(entry.Config, &cfg); err != nil {
			return nil
		}
		ch := &telegramChannel{
			app:    a,
			config: cfg,
			name:   entry.Name,
			levels: entry.Levels,
			types:  entry.Types,
		}
		if err := ch.Validate(); err != nil {
			return nil
		}
		return ch
	case "bark":
		var cfg barkConfig
		if err := json.Unmarshal(entry.Config, &cfg); err != nil {
			return nil
		}
		ch := &barkChannel{
			app:    a,
			config: cfg,
			name:   entry.Name,
			levels: entry.Levels,
			types:  entry.Types,
		}
		if err := ch.Validate(); err != nil {
			return nil
		}
		return ch
	case "serverchan":
		var cfg serverchanConfig
		if err := json.Unmarshal(entry.Config, &cfg); err != nil {
			return nil
		}
		ch := &serverchanChannel{
			app:    a,
			config: cfg,
			name:   entry.Name,
			levels: entry.Levels,
			types:  entry.Types,
		}
		if err := ch.Validate(); err != nil {
			return nil
		}
		return ch
	case "email":
		var cfg emailConfig
		if err := json.Unmarshal(entry.Config, &cfg); err != nil {
			return nil
		}
		ch := &emailChannel{
			app:    a,
			config: cfg,
			name:   entry.Name,
			levels: entry.Levels,
			types:  entry.Types,
		}
		if err := ch.Validate(); err != nil {
			return nil
		}
		return ch
	default:
		return nil
	}
}

// ==================== 加密处理 ====================

func (a *App) encryptChannelEntrySecrets(entry *channelEntry) error {
	if entry.Config == nil {
		return nil
	}
	switch entry.Type {
	case "webhook":
		var cfg webhookConfig
		if err := json.Unmarshal(entry.Config, &cfg); err != nil {
			return nil
		}
		if cfg.HMACSecret != "" {
			enc, err := a.encryptText(cfg.HMACSecret)
			if err != nil {
				return err
			}
			cfg.HMACSecret = enc
		}
		entry.Config, _ = json.Marshal(cfg)
	case "telegram":
		var cfg telegramConfig
		if err := json.Unmarshal(entry.Config, &cfg); err != nil {
			return nil
		}
		if cfg.BotToken != "" {
			enc, err := a.encryptText(cfg.BotToken)
			if err != nil {
				return err
			}
			cfg.BotToken = enc
		}
		entry.Config, _ = json.Marshal(cfg)
	case "serverchan":
		var cfg serverchanConfig
		if err := json.Unmarshal(entry.Config, &cfg); err != nil {
			return nil
		}
		if cfg.SendKey != "" {
			enc, err := a.encryptText(cfg.SendKey)
			if err != nil {
				return err
			}
			cfg.SendKey = enc
		}
		entry.Config, _ = json.Marshal(cfg)
	case "email":
		var cfg emailConfig
		if err := json.Unmarshal(entry.Config, &cfg); err != nil {
			return nil
		}
		if cfg.Password != "" {
			enc, err := a.encryptText(cfg.Password)
			if err != nil {
				return err
			}
			cfg.Password = enc
		}
		entry.Config, _ = json.Marshal(cfg)
	}
	return nil
}

func (a *App) decryptChannelEntrySecrets(entry *channelEntry) error {
	if entry.Config == nil {
		return nil
	}
	switch entry.Type {
	case "webhook":
		var cfg webhookConfig
		if err := json.Unmarshal(entry.Config, &cfg); err != nil {
			return nil
		}
		if cfg.HMACSecret != "" && strings.HasPrefix(cfg.HMACSecret, "v1.") {
			dec, err := a.decryptText(cfg.HMACSecret)
			if err == nil {
				cfg.HMACSecret = dec
			} else {
				// B3: 解密失败回退为空字符串
				cfg.HMACSecret = ""
			}
		}
		entry.Config, _ = json.Marshal(cfg)
	case "telegram":
		var cfg telegramConfig
		if err := json.Unmarshal(entry.Config, &cfg); err != nil {
			return nil
		}
		if cfg.BotToken != "" && strings.HasPrefix(cfg.BotToken, "v1.") {
			dec, err := a.decryptText(cfg.BotToken)
			if err == nil {
				cfg.BotToken = dec
			} else {
				cfg.BotToken = ""
			}
		}
		entry.Config, _ = json.Marshal(cfg)
	case "serverchan":
		var cfg serverchanConfig
		if err := json.Unmarshal(entry.Config, &cfg); err != nil {
			return nil
		}
		if cfg.SendKey != "" && strings.HasPrefix(cfg.SendKey, "v1.") {
			dec, err := a.decryptText(cfg.SendKey)
			if err == nil {
				cfg.SendKey = dec
			} else {
				cfg.SendKey = ""
			}
		}
		entry.Config, _ = json.Marshal(cfg)
	case "email":
		var cfg emailConfig
		if err := json.Unmarshal(entry.Config, &cfg); err != nil {
			return nil
		}
		if cfg.Password != "" && strings.HasPrefix(cfg.Password, "v1.") {
			dec, err := a.decryptText(cfg.Password)
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

// ==================== 分发引擎 ====================

func (a *App) dispatchNotification(kind, level, title, content string) {
	config := a.currentNotificationChannelsConfig()
	if !config.Enabled {
		return
	}
	for _, entry := range config.Channels {
		if !entry.Enabled {
			continue
		}
		if !shouldSendToChannel(entry, kind, level) {
			continue
		}
		// 频率限制检查
		if entry.RateLimit != nil && entry.RateLimit.MaxPerInterval > 0 {
			if rl, ok := a.channelRateLimits[entry.Name]; !ok || !rl.allow() {
				if ok {
					log.Printf("[notification] 渠道 %q 触发频率限制，跳过: %s/%s", entry.Name, kind, level)
				}
				continue
			}
		}
		// digest 模式的 webhook 走 App 级别管理的 channel
		if entry.Type == "webhook" {
			var cfg webhookConfig
			if err := json.Unmarshal(entry.Config, &cfg); err == nil && cfg.Mode == "digest" {
				if dc, ok := a.digestChannels[entry.Name]; ok && dc.digestCh != nil {
					select {
					case dc.digestCh <- digestEntry{Kind: kind, Level: level, Title: title, Content: content, Time: time.Now()}:
					default:
						log.Printf("[notification] webhook digest 通道已满，丢弃通知: %s/%s", kind, level)
					}
					continue
				}
			}
		}
		// 普通渠道（非阻塞 goroutine）
		channel := a.buildChannelFromConfig(entry)
		if channel == nil {
			continue
		}
		go func(ch notificationChannel) {
			if err := ch.Send(context.Background(), kind, level, title, content); err != nil {
				log.Printf("[notification] %s 发送失败: %v", ch.Type(), err)
			}
		}(channel)
	}
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
	policy := c.app.externalURLPolicy()
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

	resp, err := c.app.doHTTPWithTimeout(req, time.Duration(timeout)*time.Second)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
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

	resp, err := c.app.doHTTPWithTimeout(req, time.Duration(timeout)*time.Second)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
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

	resp, err := c.app.doHTTPWithTimeout(req, 10*time.Second)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Telegram API HTTP %d", resp.StatusCode)
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
	policy := c.app.externalURLPolicy()
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

	resp, err := c.app.doHTTPWithTimeout(req, 10*time.Second)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Bark HTTP %d", resp.StatusCode)
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

	resp, err := c.app.doHTTPWithTimeout(req, 10*time.Second)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ServerChan API HTTP %d", resp.StatusCode)
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
