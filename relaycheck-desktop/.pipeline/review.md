# T3.3 Review

## VERDICT: NEEDS WORK

## Findings

### Critical (must fix)

1. **Goroutine leak in digest mode** — `notification.go:294`
   `buildChannelFromConfig` 中，每次 mode=="digest" 的 webhook 都会执行：
   ```go
   ch.digestCh = make(chan digestEntry, 100)
   go ch.StartDigestLoop(context.Background(), ch.digestCh)
   ```
   `dispatchNotification` 每次通知都会调用 `buildChannelFromConfig`，即每次通知都创建一个新的 goroutine + channel。旧 goroutine 持有对旧 webhookChannel 的引用（闭包捕获 `c`），永远不会退出。高频通知场景下 goroutine 线性增长，最终导致内存泄漏。
   
   修复建议：digest loop 应由 App 生命周期管理，而非每次 dispatch 时新建。参考 `schedulerCancel` + `schedulerWG` 模式，在 App 级别持有一个 `digestCancel context.CancelFunc`，`buildChannelFromConfig` 只创建 channel 不启动 goroutine，由 `reloadNotificationConfig` / `App.Close` 负责取消和等待。

### Major (should fix)

2. **Digest mode bypasses level/mode filter** — `notification.go:516-521` vs `notification.go:682-739`
   `dispatchNotification` 检测到 digest 渠道时直接把条目写入 `digestCh`，跳过了 `levelMatchesMode` 检查。而 `sendDigest` 内部也没有调用 `levelMatchesMode`。结果是：即使 webhook 配置 mode="failure"，digest 中仍会包含 success/info 级别的通知。这违背了 mode 过滤的设计意图。

   修复建议：在写入 `digestCh` 前或在 `sendDigest` 中调用 `levelMatchesMode(c.config.Mode, entry.Level)` 过滤。

3. **`sendDigest` 不检查 HTTP 状态码** — `notification.go:732-738`
   对比 `webhookChannel.Send`（第 623 行有 `if resp.StatusCode < 200 || resp.StatusCode >= 300` 检查），`sendDigest` 只做了 `defer resp.Body.Close()` 和 drain，对非 2xx 响应直接返回 nil。这意味着 webhook 服务器返回 500/404 时，digest 投递被静默忽略，调用方完全不知情。

   修复建议：在 `sendDigest` 中增加与 `Send` 相同的 status code 检查。

### Minor (nice to have)

4. **非 digest 渠道缺少 content 截断** — `webhookChannel.Send`、`telegramChannel.Send`、`barkChannel.Send`、`serverchanChannel.Send` 均直接使用原始 `content` 参数，未调用 `truncateNotifyContent`。仅 `sendDigest`（截断到 2000）和 `buildEmailMessage`（截断到 20000）做了截断。如果通知内容非常大（如完整错误堆栈），可能导致 webhook 接收方拒绝或 SMTP 邮件过大。

5. **Telegram/ServerChan 未做 SSRF URL 验证** — `notification.go:772` 和 `notification.go:872`
   webhook 和 bark 正确调用了 `validateOutboundHTTPURL`，但 Telegram（`https://api.telegram.org/bot<token>/sendMessage`）和 ServerChan（`https://sctapi.ftqq.com/<sendKey>.send`）的 URL 由用户可控字段拼接而成，未经过 `validateOutboundHTTPURL`。风险较低（host 被硬编码，token/sendKey 在 path 中），但属于 spec 第 8.9 B11 条要求的偏离。

6. **`shouldSendToChannel` 未检查 mode，导致不必要的 channel 构建** — `notification.go:533-544`
   mode 过滤被推迟到各渠道 `Send()` 内执行，但 `dispatchNotification` 已经调用了 `buildChannelFromConfig`（digest 模式还会启动 goroutine）。如果 mode 过滤掉了所有通知，仍然创建了 channel 实例（和可能的 digest goroutine）。

7. **`maskSensitiveField` 泄露最后 4 位** — `notification.go:999`
   对于 HMAC secret 这类凭证，泄露最后 4 位降低了安全边际。虽然当前仅在测试中使用，但如果未来用于日志/UI 展示，建议对敏感凭证完全掩码（返回 `""` 或 `"******"`）。

### Positive

- **加密体系完整**：敏感字段（HMACSecret/BotToken/SendKey/Password）保存前加密、加载后解密，解密失败优雅降级为空字符串，不阻塞启动。
- **并发安全**：`notificationConfig` 的读写遵循与 `networkProxy` 一致的三级锁模式（`reloadNotificationConfig` 写锁、`currentNotificationChannelsConfig` 读锁）。
- **notify() 签名未变**：`routes.go:292` 签名保持 `notify(kind, level, title, content, relatedType, relatedID string)`，尾部追加 `go a.dispatchNotification(...)`，不影响现有通知流程。
- **单渠道失败不阻塞**：每个渠道在独立 goroutine 中发送，失败仅打日志。
- **SSRF 防护到位**：webhook 和 bark 正确使用了 `validateOutboundHTTPURL` + `externalURLPolicy`，覆盖了 loopback/private/metadata 地址。
- **测试覆盖全面**：32 个测试用例覆盖了配置解析、验证、加密往返、HTTP 发送（含 HMAC 验证）、digest flush、dispatch 过滤、健康检查、integration 链路。
- **build/vet 全部通过**，无回归。

## Summary

整体实现质量高，与现有代码库风格一致，加密和并发模型正确。但存在 3 个需要修复的问题：

1. **Critical**：digest goroutine 泄漏——每次 dispatch 都创建永不退出的 goroutine，高频场景下内存持续增长。
2. **Major**：digest 模式绕过 level/mode 过滤——配置了 failure 模式的 digest 仍会积累和发送 success 通知。
3. **Major**：`sendDigest` 静默忽略 HTTP 错误——投递失败无感知。

建议修复后重新跑测试和 vet 再合入。
