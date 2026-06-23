# T3.3 Notification Channel Expansion — 实现规范

## 实现步骤概览

本任务只需修改/创建 Go 后端文件，不需要修改前端。

### 实现次序（按依赖）

1. 数据结构定义 + 配置加载（`notification.go`）
2. 渠道接口定义 + 各渠道实现（`notification.go`）
3. 通知分发引擎（`notification.go`）
4. 在 `notify()` 尾部接入分发引擎（`routes.go`）
5. 默认配置注册 + App struct 字段（`app.go`）
6. 设置保存时的验证和归一化（`system.go`）
7. 健康检查项（`health.go`）
8. 单元测试（`notification_test.go`）

---

## 1. 文件清单

### 创建
| 文件 | 内容 |
|------|------|
| `E:\zidqiandao\relaycheck-desktop\internal\core\notification.go` | 所有通知渠道实现的单一文件 |
| `E:\zidqiandao\relaycheck-desktop\internal\core\notification_test.go` | 通知系统单元测试 |

### 修改
| 文件 | 改动 |
|------|------|
| `E:\zidqiandao\relaycheck-desktop\internal\core\routes.go` | 在 `notify()` 尾部添加 `a.dispatchNotification(...)` 调用（第 296-298 行） |
| `E:\zidqiandao\relaycheck-desktop\internal\core\app.go` | `App` 结构体增加 `notificationConfig` 字段（第 32 行附近）；`ensureDefaultSettings()` 增加默认键（第 178 行附近）；`NewApp()` 调用 `reloadNotificationConfig`（第 121 行附近） |
| `E:\zidqiandao\relaycheck-desktop\internal\core\system.go` | 在 `handleUpdateSystemSettings` 中，`network.proxy` 分支后（第 86 行）添加 `notification.channels` 验证和加密分支 |
| `E:\zidqiandao\relaycheck-desktop\internal\core\health.go` | 在 `healthStatus()` 增加 `a.healthCheckNotificationChannels()` 检查项 |
| `E:\zidqiandao\relaycheck-desktop\internal\core\models.go` | 新增 `NotificationChannelStatus` 结构体（用于系统状态 API 暴露） |

---

## 2. 设计参考

参考现有文件中的成熟模式：

- **配置加载模式**：`E:\zidqiandao\relaycheck-desktop\internal\core\network.go` 的 `NetworkProxyConfig` / `parseNetworkProxyConfig` / `validateNetworkProxyConfig` / `reloadNetworkProxyConfig` 风格。`notification.go` 照此模式实现 `notificationChannelsConfig` / `parseNotificationChannelsConfig` / `reloadNotificationConfig`。

- **配置存储**：`system_settings` 表，key/value_json，同现有模式（`db.go` 第 21-27 行）。

- **默认值注册**：`app.go` 的 `ensureDefaultSettings()` 使用 `INSERT OR IGNORE`（`app.go` 第 173-191 行）。

- **三级配置锁**（与 `network.go` 一致）：
  1. `loadNotificationChannelsConfig(ctx)` — 从数据库读原始 JSON 并解析
  2. `reloadNotificationConfig(ctx)` — 从 DB 加载并更新到 `a.notificationConfig`
  3. `currentNotificationChannelsConfig()` — 线程安全地返回当前内存配置

- **健康检查**：仿照 `health.go` 的 `healthCheckPath` / `healthCheckScheduler` 风格。

- **测试模式**：仿照 `network_test.go` / `health_test.go` / `scheduler_test.go` 风格，使用 `app, err := NewApp(t.TempDir())`。

- **HTTP 客户端**：使用 `a.doHTTPWithTimeout(req, timeout)`，自动继承全局代理配置。参考 `checkin_balance.go` 中 `sendCheckinRequest` 的 `a.client.Do(req)` 用法（但 `doHTTPWithTimeout` 已经包了代理）。

- **加密**：使用 `crypto.go` 的 `encryptText` / `decryptText`（AES-256-GCM）。

- **SSRF 防护**：出站 URL 需要先通过 `validateOutboundHTTPURL(ctx, url, outboundURLPolicy{})` 验证（参考 `network.go` 第 213 行）。

---

## 3. 数据结构定义（notification.go）

### 配置结构体系

```go
// ==================== 顶层配置 ====================

type notificationChannelsConfig struct {
	Enabled       bool           `json:"enabled"`
	DefaultLevels []string       `json:"defaultLevels"`
	Channels      []channelEntry `json:"channels"`
}

type channelEntry struct {
	Type    string          `json:"type"`    // "webhook" | "telegram" | "bark" | "serverchan" | "email"
	Name    string          `json:"name"`
	Enabled bool            `json:"enabled"`
	Config  json.RawMessage `json:"config"`  // 各渠道的专属配置，解析时按 type 分配
	Levels  []string        `json:"levels"`  // 该渠道处理哪些 level，为空则用 DefaultLevels
	Types   []string        `json:"types"`   // 该渠道处理哪些通知 type，为空则全处理
}

// ==================== 各渠道专属配置 ====================

type webhookConfig struct {
	URL            string `json:"url"`
	HMACSecret     string `json:"hmacSecret"`     // 加密存储
	Mode           string `json:"mode"`            // "all" | "success" | "failure" | "digest"
	TimeoutSeconds int    `json:"timeoutSeconds"`
}

type telegramConfig struct {
	BotToken string `json:"botToken"` // 加密存储
	ChatID   string `json:"chatId"`
	Mode     string `json:"mode"`     // "all" | "success" | "failure"
}

type barkConfig struct {
	URL   string `json:"url"`
	Mode  string `json:"mode"`  // "all" | "success" | "failure"
	Group string `json:"group"`
}

type serverchanConfig struct {
	SendKey string `json:"sendKey"` // 加密存储
	Mode    string `json:"mode"`    // "all" | "success" | "failure"
}

type emailConfig struct {
	SMTPHost string `json:"smtpHost"`
	SMTPPort int    `json:"smtpPort"`
	SMTPTLS  bool   `json:"smtpTls"`
	Username string `json:"username"`
	Password string `json:"password"` // 加密存储
	FromAddr string `json:"fromAddr"`
	ToAddr   string `json:"toAddr"`
	Mode     string `json:"mode"`     // "all" | "success" | "failure"
}

// ==================== App 新增字段 ====================
// 在 App struct（app.go ~line 32）中新增：
//   notificationConfig   notificationChannelsConfig

// ==================== 摘要积累 ====================

type digestEntry struct {
	Kind    string
	Level   string
	Title   string
	Content string
	Time    time.Time
}
```

### 接口定义

```go
// notificationChannel 是所有通知渠道必须实现的接口
type notificationChannel interface {
	Type() string                          // "webhook" | "telegram" | "bark" | "serverchan" | "email"
	Validate() error                       // 验证配置完整性
	Send(ctx context.Context, kind, level, title, content string) error
	EncryptedFields() []string             // 需要在存储前加密的字段名列表，用于保存配置时自动加密
}

// digestChannel 是可选的摘要模式扩展接口，仅 webhook 实现
type digestChannel interface {
	notificationChannel
	StartDigestLoop(ctx context.Context, entries chan digestEntry)
	FlushDigest(ctx context.Context) error
}
```

---

## 4. 函数签名与内部实现（全部在 notification.go）

### 4.1 配置加载函数

```go
// defaultNotificationChannelsConfig 返回默认配置
func defaultNotificationChannelsConfig() notificationChannelsConfig

// parseNotificationChannelsConfig 解析 JSON 为配置结构体，验证并归一化
// 返回配置和验证消息列表
func parseNotificationChannelsConfig(valueJSON string) (notificationChannelsConfig, []string)

// validateNotificationChannelsConfig 验证渠道配置集合
// 遍历每个渠道调用其 Validate()，收集所有验证消息
func validateNotificationChannelsConfig(config *notificationChannelsConfig) []string
```

### 4.2 App 方法（同 network.go 模式）

```go
// reloadNotificationConfig 从数据库重新加载并应用通知配置
// 在 NewApp() 初始化时 + handleUpdateSystemSettings 保存后调用
// 加载时对加密字段解密，结果存入 a.notificationConfig
func (a *App) reloadNotificationConfig(ctx context.Context) error

// loadNotificationChannelsConfig 从 system_settings 读取原始 JSON
func (a *App) loadNotificationChannelsConfig(ctx context.Context) (notificationChannelsConfig, error)

// currentNotificationChannelsConfig 线程安全地返回当前内存配置
// 使用 a.mu.RLock
func (a *App) currentNotificationChannelsConfig() notificationChannelsConfig
```

### 4.3 渠道工厂

```go
// buildChannelFromConfig 根据 channelEntry 创建对应的渠道实例
// 返回 notificationChannel。如果配置无效返回 nil（不阻止启动）
func (a *App) buildChannelFromConfig(entry channelEntry) notificationChannel

// encryptChannelEntrySecrets 对 channelEntry 中的敏感字段加密
// 在保存到数据库前调用
func (a *App) encryptChannelEntrySecrets(entry *channelEntry) error

// decryptChannelEntrySecrets 对 channelEntry 中的敏感字段解密
// 在从数据库加载后调用
func (a *App) decryptChannelEntrySecrets(entry *channelEntry) error
```

### 4.4 分发引擎

```go
// dispatchNotification 根据配置将通知分发到匹配的渠道
// 在 notify() 尾部以 goroutine 方式调用
func (a *App) dispatchNotification(kind, level, title, content string)

// shouldSendToChannel 判断通知是否应该发送给指定渠道
// 匹配规则：types 列表包含 kind 或 types 为空；levels 列表包含 level 或用 DefaultLevels
func shouldSendToChannel(entry channelEntry, kind, level string) bool

// levelMatchesMode 根据渠道 mode 判断 level 是否匹配
// "all" -> 全部；"success" -> success/info；"failure" -> warning/error
func levelMatchesMode(mode, level string) bool
```

### 4.5 各渠道结构体定义

```go
type webhookChannel struct {
	app    *App
	config webhookConfig
	name   string
	levels []string
	types  []string
	// digest 模式
	digestEntries []digestEntry
	digestMu      sync.Mutex
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
```

### 4.6 各渠道方法（强制实现）

```go
// ===== webhookChannel =====
func (c *webhookChannel) Type() string              // "webhook"
func (c *webhookChannel) Validate() error
func (c *webhookChannel) Send(ctx context.Context, kind, level, title, content string) error
func (c *webhookChannel) EncryptedFields() []string // ["hmacSecret"]

// ===== telegramChannel =====
func (c *telegramChannel) Type() string              // "telegram"
func (c *telegramChannel) Validate() error
func (c *telegramChannel) Send(ctx context.Context, kind, level, title, content string) error
func (c *telegramChannel) EncryptedFields() []string // ["botToken"]

// ===== barkChannel =====
func (c *barkChannel) Type() string              // "bark"
func (c *barkChannel) Validate() error
func (c *barkChannel) Send(ctx context.Context, kind, level, title, content string) error
func (c *barkChannel) EncryptedFields() []string // [] (无敏感字段)

// ===== serverchanChannel =====
func (c *serverchanChannel) Type() string              // "serverchan"
func (c *serverchanChannel) Validate() error
func (c *serverchanChannel) Send(ctx context.Context, kind, level, title, content string) error
func (c *serverchanChannel) EncryptedFields() []string // ["sendKey"]

// ===== emailChannel =====
func (c *emailChannel) Type() string              // "email"
func (c *emailChannel) Validate() error
func (c *emailChannel) Send(ctx context.Context, kind, level, title, content string) error
func (c *emailChannel) EncryptedFields() []string // ["password"]
```

### 4.7 digestChannel 扩展（仅 webhook）

```go
func (c *webhookChannel) StartDigestLoop(ctx context.Context, entries chan digestEntry)
func (c *webhookChannel) FlushDigest(ctx context.Context) error
func (c *webhookChannel) sendDigest(ctx context.Context, entries []digestEntry) error
```

### 4.8 工具函数

```go
func buildNotifyBody(kind, level, title, content string) string
func maskSensitiveField(value string) string
func truncateNotifyContent(content string, maxLen int) string
```

---

## 5. 各渠道发送实现细节

### 5.1 Webhook

```go
// Send 实现
// 1. 构建 JSON body: {"type":"<kind>","level":"<level>","title":"<title>","content":"<content>","timestamp":"<RFC3339>"}
// 2. 如果 config.HMACSecret 非空，计算 HMAC-SHA256(body, secret)，放在 X-Signature-256 请求头
// 3. Content-Type: application/json
// 4. 调用 a.doHTTPWithTimeout(req, timeout=config.TimeoutSeconds)
// 5. timeoutSeconds clamp 到 [3, 60]，默认 10
// 6. HMACSecret 不在任何日志中输出
// 7. 发送前重定向检查：跟随标准重定向（http.Client 默认行为）

// digest 模式
// 1. dispatchNotification 检测到 digest 渠道时，不单独发送，而是把条目写入 c.digestEntries
// 2. StartDigestLoop 启动一个 goroutine，每 5 分钟 tick 一次
// 3. 积累满或 tick 到时，发 N 条 entry 的摘要
// 4. 摘要 payload: {"type":"digest","count":N,"entries":[...],"timestamp":"..."}
```

### 5.2 Telegram

```go
// Send 实现
// 1. 构建消息文本: <b>RelayCheck 通知</b>\n类型: <kind>\n等级: <level>\n标题: <title>\n内容: <content>
// 2. POST https://api.telegram.org/bot<token>/sendMessage
// 3. JSON body: {"chat_id":"<chatId>","text":"<message>","parse_mode":"HTML"}
// 4. 调用 a.doHTTPWithTimeout(req, 10*time.Second)
// 5. 响应非 200 只打日志，不重试
```

### 5.3 Bark

```go
// Send 实现
// 1. 构建 GET URL: <bark_url>/<url.PathEscape(title)>/<url.PathEscape(content)>?group=<group>&autoCopy=1
// 2. 调用 a.doHTTPWithTimeout(req, 10*time.Second)
// 3. 也可用 POST（参考 Bark 文档），选简单方案：GET
// 4. group 默认为 "RelayCheck"
```

### 5.4 ServerChan

```go
// Send 实现
// 1. POST https://sctapi.ftqq.com/<sendKey>.send
// 2. JSON body: {"title":"<title>","content":"<content>","channel":9}
//    channel=9 表示 Markdown 模式，保留换行和链接
// 3. 调用 a.doHTTPWithTimeout(req, 10*time.Second)
```

### 5.5 Email

```go
// Send 实现
// 1. 构建 MIME 消息:
//    From: <fromAddr>
//    To: <toAddr>
//    Subject: =?UTF-8?B?<base64(title)>?=
//    Content-Type: text/plain; charset="UTF-8"
//
//    <content>
// 2. 使用 net/smtp.SendMail 发送:
//    - SMTPTLS=false (587): smtp.SendMail(smtpHost:port, smtp.PlainAuth, fromAddr, []string{toAddr}, msg)
//    - SMTPTLS=true (465): 使用 crypto/tls 建立 TLS 连接再调用 smtp.SendMail
// 3. content 截断到最大 20000 字（防止一封邮件太大）
// 4. 需要验证 smtpHost 非空、fromAddr 非空、toAddr 非空
// 5. 不验证邮箱合法性，只验证非空
// 6. 使用 net.LookupMX 验证 smtpHost 可解析？不，太严格。仅验证 smtpHost 非空。
```

---

## 6. 加密处理

### 6.1 保存配置时（system.go）

在 `handleUpdateSystemSettings` 的 `network.proxy` 分支后（`system.go` 第 87 行之前），添加：

```go
if key == "notification.channels" {
	config, warnings := parseNotificationChannelsConfig(valueJSON)
	// 对各渠道的敏感字段进行加密
	for i := range config.Channels {
		if err := a.encryptChannelEntrySecrets(&config.Channels[i]); err != nil {
			writeError(w, http.StatusInternalServerError, "加密通知渠道密钥失败："+err.Error())
			return
		}
	}
	normalized, _ := json.Marshal(config)
	valueJSON = string(normalized)
	// 记录警告但不阻止保存
	if len(warnings) > 0 {
		log.Printf("[notification] 渠道配置验证告警: %v", warnings)
	}
}
```

### 6.2 加载配置时（reloadNotificationConfig）

```go
// 1. 调用 loadNotificationChannelsConfig 从数据库读原始 JSON
// 2. 对各渠道的加密字段调用 a.decryptText 解密
// 3. 解密失败的字段回退为空字符串（不阻塞启动）
// 4. 将结果赋值给 a.notificationConfig
```

### 6.3 encryptChannelEntrySecrets 实现

```go
func (a *App) encryptChannelEntrySecrets(entry *channelEntry) error {
	switch entry.Type {
	case "webhook":
		var cfg webhookConfig
		json.Unmarshal(entry.Config, &cfg)
		if cfg.HMACSecret != "" {
			enc, err := a.encryptText(cfg.HMACSecret)
			if err != nil { return err }
			cfg.HMACSecret = enc
		}
		entry.Config, _ = json.Marshal(cfg)
	case "telegram":
		var cfg telegramConfig
		json.Unmarshal(entry.Config, &cfg)
		if cfg.BotToken != "" {
			enc, err := a.encryptText(cfg.BotToken)
			if err != nil { return err }
			cfg.BotToken = enc
		}
		entry.Config, _ = json.Marshal(cfg)
	case "serverchan":
		var cfg serverchanConfig
		json.Unmarshal(entry.Config, &cfg)
		if cfg.SendKey != "" {
			enc, err := a.encryptText(cfg.SendKey)
			if err != nil { return err }
			cfg.SendKey = enc
		}
		entry.Config, _ = json.Marshal(cfg)
	case "email":
		var cfg emailConfig
		json.Unmarshal(entry.Config, &cfg)
		if cfg.Password != "" {
			enc, err := a.encryptText(cfg.Password)
			if err != nil { return err }
			cfg.Password = enc
		}
		entry.Config, _ = json.Marshal(cfg)
	}
	return nil
}
```

`decryptChannelEntrySecrets` 是对称操作，使用 `a.decryptText` 解密。

---

## 7. 模式匹配与分发逻辑

### 7.1 dispatchNotification

```go
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
		channel := a.buildChannelFromConfig(entry)
		if channel == nil {
			continue
		}
		// digest 模式的 webhook 特殊处理
		if c, ok := channel.(*webhookChannel); ok && c.config.Mode == "digest" {
			select {
			case digestEntries <- digestEntry{Kind: kind, Level: level, Title: title, Content: content, Time: time.Now()}:
			default:
				// channel full, drop
			}
			continue
		}
		// 普通发送（非阻塞 goroutine）
		go func(ch notificationChannel) {
			if err := ch.Send(context.Background(), kind, level, title, content); err != nil {
				log.Printf("[notification] %s 发送失败: %v", ch.Type(), err)
			}
		}(channel)
	}
}
```

### 7.2 shouldSendToChannel

```go
func shouldSendToChannel(entry channelEntry, kind, level string) bool {
	// types 过滤：若 entry.Types 不为空，kind 必须在列表中
	if len(entry.Types) > 0 {
		if !stringInSlice(kind, entry.Types) {
			return false
		}
	}
	// levels 过滤：优先用 entry.Levels，其次 DefaultLevels
	levels := entry.Levels
	if len(levels) == 0 {
		// 从 config 获取 DefaultLevels，这里需要传进来
		return true // 没有 level 限制则放行（由渠道的 mode 控制）
	}
	return stringInSlice(level, levels)
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
		return true // 未知 mode 放行
	}
}
```

---

## 8. 修改点细节（逐文件）

### 8.1 routes.go — notify() 函数（第 292-298 行）

**改动**：在 `a.invalidateReadCache()` 后添加分发调用。

```go
func (a *App) notify(kind, level, title, content, relatedType, relatedID string) {
	_, _ = a.db.Exec(`
		INSERT INTO app_notifications (id, type, level, title, content, read, related_type, related_id, created_at)
		VALUES (?, ?, ?, ?, ?, 0, ?, ?, ?)
	`, newID(), kind, level, title, content, relatedType, relatedID, now())
	a.invalidateReadCache()

	// 异步分发到外部通知渠道
	go a.dispatchNotification(kind, level, title, content)
}
```

### 8.2 app.go — App struct（第 22-41 行）

**改动**：在 `networkProxy NetworkProxyConfig` 后新增字段。

```go
type App struct {
	// ... 现有字段 ...
	networkProxy       NetworkProxyConfig
	notificationConfig notificationChannelsConfig   // 新增
	// ... 后续字段 ...
}
```

### 8.3 app.go — ensureDefaultSettings（第 173-191 行）

**改动**：在 defaults map 中新增一个键值对。

```go
defaults := map[string]string{
	// ... 现有键值 ...
	"notification.channels": `{"enabled":false,"defaultLevels":["warning","error"],"channels":[{"type":"webhook","name":"默认 Webhook","enabled":false,"config":{"url":"","hmacSecret":"","mode":"all","timeoutSeconds":10},"levels":["warning","error"],"types":["scheduled_checkin_failed","scheduled_sync_failed"]},{"type":"telegram","name":"Telegram Bot","enabled":false,"config":{"botToken":"","chatId":"","mode":"failure"},"levels":["warning","error"],"types":["scheduled_checkin_failed"]},{"type":"bark","name":"Bark","enabled":false,"config":{"url":"","mode":"failure","group":"RelayCheck"},"levels":["warning","error"]},{"type":"serverchan","name":"ServerChan","enabled":false,"config":{"sendKey":"","mode":"failure"},"levels":["warning","error"]},{"type":"email","name":"SMTP 邮件","enabled":false,"config":{"smtpHost":"","smtpPort":587,"smtpTls":true,"username":"","password":"","fromAddr":"","toAddr":"","mode":"failure"},"levels":["warning","error"]}]}`,
}
```

### 8.4 app.go — NewApp（第 72-127 行）

**改动**：在 `reloadNetworkProxyConfig` 调用后新增。

```go
if err := app.reloadNetworkProxyConfig(context.Background()); err != nil {
	_ = db.Close()
	return nil, err
}
// 新增：
if err := app.reloadNotificationConfig(context.Background()); err != nil {
	_ = db.Close()
	return nil, err
}
```

### 8.5 system.go — handleUpdateSystemSettings（第 52-104 行）

**改动**：在 `network.proxy` 分支后、`tx.ExecContext` 之前，添加 `notification.channels` 分支。

现有代码结构（第 79-87 行）：
```go
if key == "network.proxy" {
	config, err := parseNetworkProxyConfig(valueJSON)
	if err != nil {
		writeError(w, http.StatusBadRequest, "代理设置无效："+err.Error())
		return
	}
	normalized, _ := json.Marshal(config)
	valueJSON = string(normalized)
}
```

新增分支：
```go
if key == "network.proxy" {
	// ... 现有代码 ...
} else if key == "notification.channels" {
	config, warnings := parseNotificationChannelsConfig(valueJSON)
	// 对各渠道的敏感字段进行加密再保存
	for i := range config.Channels {
		if err := a.encryptChannelEntrySecrets(&config.Channels[i]); err != nil {
			writeError(w, http.StatusInternalServerError, "加密通知渠道密钥失败："+err.Error())
			return
		}
	}
	normalized, _ := json.Marshal(config)
	valueJSON = string(normalized)
	if len(warnings) > 0 {
		log.Printf("[notification] 渠道配置验证告警: %v", warnings)
	}
}
```

同时在第 101 行的 `reloadNetworkProxyConfig` 后增加 `reloadNotificationConfig`：

```go
_ = a.reloadNetworkProxyConfig(r.Context())
_ = a.reloadNotificationConfig(r.Context())  // 新增
```

### 8.6 health.go — healthStatus（第 18-37 行）

**改动**：在 `healthCheckScheduler()` 后增加一个检查项。

```go
func (a *App) healthStatus(ctx context.Context) HealthStatus {
	checks := []HealthCheck{
		// ... 现有检查 ...
		a.healthCheckScheduler(),
		a.healthCheckNotificationChannels(),  // 新增
	}
	// ... 后续逻辑 ...
}
```

新增方法实现：
```go
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
```

### 8.7 models.go — 新增结构体

```go
type NotificationChannelStatus struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Enabled     bool   `json:"enabled"`
	ConfigValid bool   `json:"configValid"`
	Levels      []string `json:"levels,omitempty"`
}
```

此结构体可用于后续（可选）添加到系统 API 响应中让前端展示通知渠道状态（当前项目不需要改前端，保留以备后续扩展）。

---

## 9. 边界情况与注意事项

| 编号 | 边界情况 | 处理方式 |
|------|---------|---------|
| B1 | 渠道 URL 为空且渠道已启用 | `Validate()` 返回 error，`buildChannelFromConfig` 返回 nil，跳过该渠道 |
| B2 | 加密字段为空 | 空字符串不加密，直接存空字符串 |
| B3 | 解密失败（密钥变化或数据损坏） | `decryptText` 返回 error，回退为空字符串，不影响启动 |
| B4 | HTTP 发送超时或网络错误 | 只打日志，不重试，不阻塞 |
| B5 | content 太长（>64KB） | 截断到 2000 中文字符（约 6000 字节 UTF-8） |
| B6 | 全局 Enabled: false | `dispatchNotification` 直接 return，不遍历渠道 |
| B7 | system_settings 无 `notification.channels` 行 | `loadNotificationChannelsConfig` 遇到 `sql.ErrNoRows` 返回默认配置 |
| B8 | digest 积累期间 app 退出 | 丢失积累的摘要（可接受，通知非关键日志，不要求持久化） |
| B9 | ServerChan sendKey 为空但启用 | `Validate()` 返回 error，跳过该渠道 |
| B10 | 单个渠道无效但不影响其他渠道 | 每个渠道独立创建和发送，失败不阻断后续渠道 |
| B11 | SSRF 防护 | 所有出站 URL 需通过 `validateOutboundHTTPURL` 检查（参考 network.go 用法） |
| B12 | 通知等级不匹配 mode | `levelMatchesMode` 在渠道 Send 内调用过滤 |

---

## 10. 测试计划

### 10.1 测试文件

`E:\zidqiandao\relaycheck-desktop\internal\core\notification_test.go`

`package core`

### 10.2 测试用例清单

#### TestParseNotificationChannelsConfig_Valid
- 用完整有效 JSON 初始化
- 验证所有字段正确解析
- 验证 channels 数组长度正确

#### TestParseNotificationChannelsConfig_Minimal
- 用 `{"enabled":true}` 初始化
- 验证回退默认值

#### TestParseNotificationChannelsConfig_InvalidJSON
- 用无效 JSON
- 验证返回 error

#### TestDefaultNotificationChannelsConfig (参考 `network_test.go` 的 `TestNetworkProxyConfig`)
- 验证默认配置结构
- 验证 enabled=false

#### TestValidateWebhookConfig
- url 非空 + HMAC 非空 -> nil
- url 空 -> error
- 完整有效配置

#### TestValidateTelegramConfig
- botToken 非空 + chatId 非空 -> nil
- botToken 空 -> error

#### TestValidateBarkConfig
- url 非空 -> nil
- url 空 -> error

#### TestValidateServerChanConfig
- sendKey 非空 -> nil
- sendKey 空 -> error

#### TestValidateEmailConfig
- 全部必填字段非空 -> nil
- smtpHost 空 -> error
- fromAddr 空 -> error
- toAddr 空 -> error

#### TestLevelMatchesMode
- "all" + 任意 level -> true
- "success" + "success" -> true
- "success" + "warning" -> false
- "failure" + "error" -> true
- "failure" + "info" -> false

#### TestWebhookSend_Success (使用 httptest.Server)
- 启动 httptest.Server 记录请求
- 验证 POST 包含正确 JSON body
- 验证 Content-Type: application/json
- 验证无 HMAC 时没有 X-Signature-256 头

#### TestWebhookSend_WithHMAC (使用 httptest.Server)
- 配置 hmacSecret
- 验证 X-Signature-256 头存在
- 验证签名值正确（用 test key 重新计算对比）

#### TestWebhookSend_DigestMode (使用 httptest.Server)
- 配置 digest 模式 + 短时间窗口
- 发送多个通知条目
- 验证最终收到一条 digest POST

#### TestDispatchNotification_Disabled (参考 `health_test.go` 模式)
- 配置 enabled=false
- 调用 dispatchNotification
- 验证无 HTTP 请求（httptest.Server 计数为 0）

#### TestDispatchNotification_LevelFilter
- 渠道只配 ["warning"]
- 发 info 级别通知
- 验证不触发渠道发送

#### TestReloadNotificationConfig_MissingRow
- 从 system_settings 删除 notification.channels 行
- 调用 reloadNotificationConfig
- 验证不 panic，配置回退到默认

#### TestEncryptDecryptChannelEntrySecrets
- 对包含明文敏感字段的 channelEntry 调用 encrypt
- 调用 decrypt
- 验证解密后与明文一致

### 10.3 测试注意事项

- **所有 HTTP 测试使用 `httptest.Server`**，不发出真实网络请求。参考 `health_test.go` 用法。
- Email 测试不连接真实 SMTP 服务器，仅测试 Validate。
- HMAC Secret 不打印到日志。
- 遵循现有测试风格：`app, err := NewApp(t.TempDir())` -> `defer app.Close()`。
- 注意 `notification.go` 新增的 `sync.Mutex` digest 相关字段的并发安全性（digest 可能在测试中需要加锁访问）。
- 测试 digest 时可以使用 `time.NewTimer` 控制 tick 触发（或提取 digest interval 为可配置参数以便测试注入）。

---

## 10. 依赖情况

**不新增 Go 模块依赖。** 所有渠道实现只需 Go 标准库：

| 能力 | 标准库包 |
|------|---------|
| Webhook POST | `net/http` |
| HMAC-SHA256 | `crypto/hmac` + `crypto/sha256` |
| Telegram Bot API | `net/http` |
| Bark | `net/http` + `net/url` |
| ServerChan | `net/http` |
| Email | `net/smtp` + `crypto/tls` |
| MIME 编码 | `net/mime` (用于 =?UTF-8?B? Subject) |

`go mod vendor` 无需更新。

---

## 11. 验收标准

1. `go test -mod=vendor ./internal/core/...` 全部通过 — 现有测试无回归，新测试全部通过
2. `notify()` 函数签名未改变：`notify(kind, level, title, content, relatedType, relatedID string)`
3. 所有敏感字段使用 `a.encryptText` 加密存储，不出现明文
4. HMAC Secret 不在日志中输出（`go vet` 不报警）
5. digest 模式仅 webhook 渠道实现，其他渠道不实现 `digestChannel` 接口
6. 配置保存时验证生效、无效渠道被跳过但不阻止保存
7. 健康检查可以反映通知渠道启用状态