# 签到与登录执行缝架构优化设计

**日期：** 2026-07-04
**状态：** 待实施
**作者：** Codex brainstorming session

---

## 1. 背景

RelayCheck Desktop 后端已经完成多轮领域化收敛：`internal/accounts`、`internal/channels`、`internal/sites`、`internal/notifications` 等包已经从 `internal/core` 中拆出，`core` 目前主要承担组装根、HTTP handler、横切能力和少量高耦合执行逻辑。

当前剩余架构压力最大的区域是签到、余额、登录态和浏览器授权链路：

- `internal/core/checkin_balance.go` 同时负责签到执行、余额刷新、密码登录、会话保存、账号 API 调用、结果持久化和通知。
- `internal/core/accounts.go` 中的网页登录入口打开和保存授权逻辑依赖同一套 `accountAuthContext`、浏览器 session store、加密和 DB 更新能力。
- `scheduler.go`、`task_runner.go`、账号 handler 和签到 handler 都会调用这些执行逻辑，使它们成为后续优化和缺陷修复的热点。

近期登录入口识别、网页登录跳转和同源保护已经落地。下一步不应继续做大范围拆包，而应先给“单账号执行链路”抽出稳定内部服务缝，让后续签到、余额、API key 和浏览器登录修复可以落在更小的边界内。

## 2. 目标

本阶段目标是降低 `checkin_balance.go` 与 `accounts.go` 之间的执行耦合，而不是改变产品行为。

验收重点：

- 把登录态保障、账号 API 调用、浏览器登录执行从大文件中抽成清晰内部服务。
- 保留 `internal/core` 作为组装根，暂不新建 `internal/checkin` 包。
- 保持现有数据库 schema、HTTP 路由、响应字段和前端动作链不变。
- 让单账号执行逻辑更容易单测，尤其是 URL 解析、请求头组装、会话保存和登录失败处理。
- 让 `runDueCheckinsWithFilter`、调度器和任务引擎继续作为编排层调用，不在本阶段重写。

## 3. 候选方案

| 方案 | 内容 | 优点 | 缺点 | 结论 |
|------|------|------|------|------|
| A. 内部执行服务缝 | 在 `package core` 内抽 `AccountSessionService`、`AccountAPIClient`、`BrowserLoginService`，原 handler 和编排函数转为调用服务 | 改动可控，贴合当前登录/签到热点，不改变包边界 | `core` 文件数不会立刻显著减少 | 采用 |
| B. 直接拆 `internal/checkin` 包 | 把签到、余额、任务和调度相关文件搬到新包 | 物理边界最清楚 | 依赖 `App`、DB、通知、任务、浏览器 session 和账号认证，容易引发大范围 import 与测试 churn | 暂缓 |
| C. Handler 薄控制器统一整理 | 先把所有账号/签到 handler 改成薄控制器，再抽服务 | 接口层更整齐 | 范围偏广，容易变成机械整理，不能优先解决执行耦合 | 暂缓 |

本阶段采用方案 A。它是后续拆 `checkin` 包前的低风险前置步骤，也能直接降低当前登录跳转和签到执行问题的维护成本。

## 4. 范围

### 4.1 本阶段包含

- 新增或收敛 `AccountSessionService`：
  - `Ensure(ctx, auth)`：确保账号有 Cookie、Access Token 或 API key。
  - `LoginWithPassword(ctx, auth)`：保留现有多 login path、多 payload 尝试逻辑。
  - `Save(ctx, auth, cookie, accessToken, authUserID)`：保存加密会话并更新账号登录状态。
- 新增或收敛 `AccountAPIClient`：
  - `Do(ctx, auth, method, path, body)`：复用现有账号 API 调用逻辑。
  - `DoWithTimeout(ctx, auth, method, path, body, timeout)`：服务 API key 模型测速等短超时调用。
  - 统一请求头组装：User-Agent、Cookie、New-Api-User 或站点特定用户 ID header、Authorization。
- 新增或收敛 `BrowserLoginService`：
  - `Open(ctx, accountID, auth)`：打开或复用网页登录窗口。
  - `Save(ctx, accountID, auth)`：读取浏览器 Cookie，保存授权。
  - `ResolveTarget(baseURL, loginURL, source)`：统一自动发现 URL 与手动 URL 的策略。
- 保持现有 `App` 方法作为薄转发器，避免一次性修改所有调用方。
- 补充聚焦测试，锁定 URL 同源策略、手动 URL 例外、请求头、密码登录和会话保存行为。

### 4.2 本阶段不包含

- 不创建 `internal/checkin` 新包。
- 不重写 `scheduler.go`、`task_runner.go` 或 SSE 任务流。
- 不调整前端页面结构和交互文案。
- 不修改数据库表结构或迁移。
- 不新增公开 HTTP API。
- 不引入新依赖或新浏览器自动化框架。
- 不改变通知、审计和日志语义。

## 5. 架构设计

### 5.1 组件边界

本阶段新增的服务仍在 `internal/core` 包内，保持访问现有未导出类型和 helper 的能力。每个服务通过构造函数接收最小依赖，避免继续把所有操作挂在 `*App` 上。

建议文件布局：

| 文件 | 职责 |
|------|------|
| `account_session_service.go` | 密码登录、会话保障、Cookie/Token 保存 |
| `account_api_client.go` | 账号 API 请求构造、认证 header、响应读取 |
| `browser_login_service.go` | 浏览器登录打开、保存授权、目标 URL 解析 |

`App` 持有这些服务：

```go
type App struct {
    accountSession *AccountSessionService
    accountAPI     *AccountAPIClient
    browserLogin   *BrowserLoginService
}
```

现有方法保留薄转发：

```go
func (a *App) loginWithPassword(ctx context.Context, auth *accountAuthContext) error {
    return a.accountSession.LoginWithPassword(ctx, auth)
}
```

这样可以先稳定边界，再逐步把 `runAccountCheckin`、`refreshAccountBalance`、`testAPIKeyForAccount` 等调用点切到服务字段。

### 5.2 `AccountSessionService`

职责：

- 判断账号是否已有可用认证材料。
- 使用现有 `capabilities.LoginAPIPaths(auth.LoginPath)` 构造候选登录接口。
- 尝试 username/email/account 三种 payload。
- 从响应中提取 Cookie、Access Token 和用户 ID。
- 调用加密服务保存会话。
- 更新传入的 `accountAuthContext`，让后续签到或余额刷新立即复用新会话。

依赖：

- `*sql.DB`
- `*CryptoService` 或等价加解密端口
- 登录 HTTP 执行函数，沿用现有代理和网络策略

边界：

- 不负责签到响应分类。
- 不负责余额解析。
- 不负责浏览器 DevTools 读取。

### 5.3 `AccountAPIClient`

职责：

- 基于 `accountAuthContext` 构造账号侧 API 请求。
- 统一处理认证 header：
  - Cookie
  - Access Token
  - API key
  - `New-Api-User` 或 `capabilities.UserIDHeaderForKind`
- 读取最多 256 KiB 响应体，维持现有内存上限。
- 提供普通调用和带 timeout 调用。

依赖：

- 常规 HTTP 执行函数 `doHTTP`
- 带超时 HTTP 执行函数 `doHTTPWithTimeout`
- outbound URL policy，用于需要安全归一化的调用

边界：

- 不解析业务响应。
- 不写数据库。
- 不发通知。
- 不吞掉 HTTP 状态码。

### 5.4 `BrowserLoginService`

职责：

- 统一决定网页登录目标地址。
- 自动发现或候选 URL 必须与站点 baseURL 同源；不安全时回退到 `/login`。
- 手动配置的绝对 URL 保持允许，延续当前兼容行为。
- 打开 Chrome 调试端口并记录 `BrowserLoginSession`。
- 保存浏览器 Cookie、User-Agent、cookie 过期估算和账号登录状态。
- 守护 Chrome 进程退出后清理 session store。

依赖：

- `*sql.DB`
- `*CryptoService`
- `*BrowserSessionStore`
- 浏览器启动 helper、`readChromeSession`、`hiddenProcessAttr`
- audit 记录函数

边界：

- 不负责密码登录。
- 不负责调用签到或余额接口。
- 不改变现有 profile path 策略。

## 6. 数据流

### 6.1 密码登录与账号 API

```text
handler / task / scheduler
  -> loadAccountAuth / loadAccountAuths
  -> AccountSessionService.Ensure
      -> LoginWithPassword if needed
      -> Save encrypted cookie/token
  -> AccountAPIClient.Do
  -> runAccountCheckin / refreshAccountBalance / testAPIKeyForAccount parses result
  -> save checkin log / balance snapshot / API key test state
```

### 6.2 浏览器登录

```text
POST /api/accounts/{id}/open-browser-login
  -> loadAccountAuth
  -> BrowserLoginService.ResolveTarget
  -> BrowserLoginService.Open
  -> BrowserSessionStore.Set
  -> account login_status = manual_required

POST /api/accounts/{id}/finish-browser-login
  -> BrowserSessionStore.Get
  -> readChromeSession
  -> encrypt cookie
  -> account login_status = valid
  -> BrowserSessionStore.Delete
```

### 6.3 签到与余额编排

```text
runDueCheckinsWithFilter
  -> loadDueCheckinAccounts
  -> loadAccountAuths
  -> runAccountCheckin
      -> AccountSessionService.Ensure
      -> AccountAPIClient.Do
      -> classifyCheckinResponse
      -> saveCheckinResult

refreshAccountBalance
  -> AccountSessionService.Ensure
  -> AccountAPIClient.Do
  -> parseBalance
  -> saveBalanceResult
```

## 7. 错误处理

本阶段不引入新的错误模型，保留现有中文错误和状态字段：

- `auth_expired`
- `unsupported`
- `failed`
- `missing`
- `manual_required`
- `valid`
- `expired`

服务层返回原始错误，调用方继续决定 HTTP status、任务结果和用户可见消息。

需要特别保持的行为：

- 密码登录所有候选接口失败时，继续返回“登录接口全部失败”类提示，并建议修正站点登录地址或使用网页登录授权。
- 余额接口未探测到时仍返回“该站点未探测到余额接口”。
- 签到接口未探测到时仍记录 `unsupported`。
- 浏览器 session 不存在时仍返回 `missing`。
- 任何错误响应、日志和通知都不得包含 Cookie、Token、API key、Authorization header、密码或浏览器 profile 绝对路径。

## 8. 测试策略

### 8.1 后端单元测试

新增或迁移聚焦测试：

- `BrowserLoginService.ResolveTarget`
  - 自动发现的绝对 URL 必须同源。
  - 跨源、`javascript:`、协议相对 URL 回退到 `/login`。
  - 手动绝对 URL 保持允许。
  - 相对路径与 query string 保持正确。
- `AccountSessionService.LoginWithPassword`
  - 多 login path 依次尝试。
  - 多 payload 依次尝试。
  - Cookie 登录成功写入加密字段并更新 `login_status='valid'`。
  - Token 登录成功写入 access token 和 auth user id。
  - 登录失败消息不泄露响应中的敏感内容。
- `AccountAPIClient.Do`
  - Cookie header 正确传递。
  - Access Token 和 API key 自动补 `Bearer `。
  - 站点特定用户 ID header 正确选择。
  - 响应体读取上限保持 256 KiB。
- 现有回归测试继续覆盖：
  - `balance_bulk_test.go`
  - `checkin_status_test.go`
  - `bulk_test_api_keys_test.go`
  - `account_auth_repo_test.go`

### 8.2 集成验证

实施完成后运行：

```powershell
rtk git diff --check
rtk go test -mod=vendor -count=1 ./...
rtk go vet -mod=vendor ./...
rtk go build -mod=vendor ./...
cd frontend; rtk npm run build
cd frontend; rtk npm test
```

若本阶段没有前端改动，前端测试仍作为回归门，而不是本阶段主要变更验证。

### 8.3 手动冒烟

在已有本地数据或 mock 站点上确认：

- 单账号打开网页登录。
- 保存网页登录授权。
- 测试登录态。
- 执行单账号签到。
- 刷新单账号余额。
- 批量签到和批量余额刷新仍能运行。

## 9. 安全与隐私

- 保留现有加密机制，不新增明文凭据落盘路径。
- 服务边界内的测试 fixture 使用假 Cookie、假 Token 和本地 httptest server。
- 日志和错误消息继续使用 `maskResponse`、`maskSecret` 等既有脱敏 helper。
- 自动发现登录 URL 的同源保护必须保留；只有用户手动配置的绝对登录 URL 可以跨源。
- 不把真实凭据写入 handoff、文档、截图、日志或长期记忆。

## 10. 迁移与兼容

本阶段是内部结构迁移，对外兼容要求如下：

- `/api/accounts/{id}/open-browser-login` 响应结构不变。
- `/api/accounts/{id}/finish-browser-login` 响应结构不变。
- `/api/accounts/{id}/checkin` 响应结构不变。
- `/api/accounts/{id}/refresh-balance` 响应结构不变。
- `/api/accounts/{id}/test-api-key` 与批量 API key 测试行为不变。
- `checkin_logs`、`balance_snapshots`、`channel_accounts` 写入语义不变。
- 调度器和任务引擎调用方式可以通过薄转发保持兼容。

## 11. 实施顺序建议

1. 抽 `BrowserLoginService`，因为近期登录跳转问题最集中，且已有 URL 同源回归测试可迁移。
2. 抽 `AccountAPIClient`，先把请求头组装和响应读取从业务逻辑中隔离。
3. 抽 `AccountSessionService`，收敛密码登录和会话保存。
4. 将 `runAccountCheckin`、`refreshAccountBalance`、`testAPIKeyForAccount` 改为调用服务字段。
5. 更新 `internal/core/PACKAGE_INDEX.md`，记录新的服务边界。
6. 跑全量 Go/前端验证。

每一步应尽量保持单独 commit；如果某一步发现行为测试不足，先补测试再移动逻辑。

## 12. 验收标准

- `checkin_balance.go` 中密码登录、会话保存和账号 API 调用逻辑已迁出到服务文件或变成薄转发。
- `accounts.go` 中浏览器登录打开、保存授权和 URL 解析逻辑已迁出到服务文件或变成薄转发。
- 现有 handler、调度器、任务引擎和前端 API 行为保持不变。
- Go test、go vet、go build 全绿。
- 前端 build/test 全绿。
- 登录 URL 同源保护、手动 URL 兼容、Cookie/Token 保存、API 认证 header 组装均有测试覆盖。
- `PACKAGE_INDEX.md` 与实际文件边界一致。

## 13. 后续阶段

本阶段完成后，再评估是否继续：

- 把签到执行与余额刷新进一步抽成 `CheckinExecutor` 和 `BalanceRefresher`。
- 整理 `scheduler.go`、`task_runner.go`、`channel_schedules.go` 的调度/任务边界。
- 在服务边界稳定后，再考虑是否创建 `internal/checkin` 独立包。
- 增加真实浏览器登录的半自动手动测试记录模板。

---

**批准后下一步：** 将本设计转成分步实施计划，再开始编码。
