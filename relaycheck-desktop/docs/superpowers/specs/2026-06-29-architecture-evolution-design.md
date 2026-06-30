# 架构演进设计：从单包 god object 到领域化分层

**日期：** 2026-06-29
**状态：** 待批准
**作者：** 架构演进 brainstorming session

---

## 1. 背景与动机

### 1.1 现状

`internal/core` 是一个单 Go 包，包含 65+ 源文件、**281 个 `*App` 方法**、68 条路由全集中在 `routes.go`。`App` struct 同时承担四种角色：

1. **DI 容器** — 持有 `db`、`key`、`client` 等依赖
2. **有状态服务** — 持有 `browserSessions`、`readCache`、`digestChannels` 等运行时状态
3. **数据仓储** — 281 个方法中大部分是直接 SQL 查询
4. **HTTP handler receiver** — 几乎所有 handler 都是 `(a *App)` 方法

### 1.2 痛点

- **改动牵连广**：最近的 N+1 修复需触碰 10+ 文件，因为所有领域共享 `*App` 这个根
- **测试难写**：要测一个领域必须构造完整 `*App`，无法隔离依赖
- **云端迁移无抓手**：存储、调度、通知都焊死在 `*App` 上，替换任一组件需大改

### 1.3 动机（用户确认）

1. 降低改动牵连 — 改一个功能只动该功能的文件
2. 提升可测试性 — 通过接口隔离让单测更快更独立
3. 为多用户/云端铺路 — 预留架构扩展点，但不提前造云端的桥
4. 长期可维护性 — 渐进式收敛 god object

---

## 2. 硬约束（不可破坏）

| 约束 | 理由 |
|------|------|
| **单二进制分发** | `go:embed frontend/dist` + 单个 `.exe`，产品核心体验 |
| **本地优先 + 离线可用** | 产品定位，云端是"未来可选形态"非"替代形态" |
| **现有 DB schema 无缝迁移** | 用户有真实数据，要求重新导入 = 产品事故 |
| **对外 `/api/*` 契约稳定** | 重写前端成本高、价值低；重构只发生在后端内部 |

**推论：** 本次重构是"后端内部的渐进式重组"，不碰对外的壳。

---

## 3. 总体方向

### 3.1 终态：纵向按领域切片（方案 B）

把 `internal/core` 按业务领域拆成独立包，每个包自包含自己的 HTTP handler + 业务逻辑 + 数据查询。

```
internal/
  core/              ← 瘦身后的组装根 + 公共设施
    app.go           ← 瘦 App：只持有公共设施引用 + RegisterRoutes 装配
    infra.go         ← 公共设施接口定义
    http.go          ← writeJSON/writeError/requireSession 等共享 HTTP 工具
  accounts/          ← 领域包 1
    handlers.go      ← HTTP handler（接收注入的依赖）
    service.go       ← 业务逻辑
    store.go         ← 数据查询（实现 core.AccountStore 接口）
    types.go         ← 领域类型
    *_test.go
  checkin/           ← 领域包 2
    ...
  channels/
    ...
  notifications/
    ...
  （共 8 个领域包）
```

### 3.2 走法：用接口缝手法渐进落地（方案 C 的手法）

不一次性大改，而是：

1. **先在 `package core` 内部**为每个领域定义接口（`AccountStore`、`CheckinRunner` 等），让逻辑依赖接口而非 `*App`
2. 接口缝干净后，**再把整个领域文件群挪进独立包**
3. 每次挪一个领域，独立一次提交，全绿后继续

这样随时可停而不留半成品，每一步都能编译通过、测试通过、用户无感。

### 3.3 为什么不选其他方案

| 方案 | 为什么不选 |
|------|-----------|
| 纯水平分层（数据/逻辑/HTTP 三层） | 一个功能的改动要跨三层文件，**反而加剧**改动牵连痛点 |
| 纯接口缝不拆包（方案 C 原版） | 65 文件仍挤在一个包，物理隔离不足，长期可维护性提升有限 |
| 公共设施集中到 `infra` 包 | 设备井会越来越臃肿，重新长出新的 god object |

---

## 4. 公共设施安家：瘦 App 模式（选项 C）

### 4.1 现有公共设施清单

| 设施 | 当前位置 | 用途 | 谁在用 |
|------|---------|------|--------|
| `mu sync.RWMutex` | App 字段 | 并发保护 | 几乎所有领域 |
| `taskRunner *TaskRunner` | App 字段 | 批量任务引擎 | 签到、密钥、余额 |
| `browserSessions map` | App 字段 | 浏览器登录态 | 账号、签到 |
| `readCache map` | App 字段 | 仪表盘读缓存 | 仪表盘、通知 |
| `digestChannels map` | App 字段 | 通知通道实例 | 通知、签到 |
| `schedulerCancel` | App 字段 | 调度器控制 | 调度器 |
| `db *sql.DB` | App 字段 | 数据库 | 所有领域 |
| `key []byte` | App 字段 | 加密密钥 | 账号、备份 |
| `client *http.Client` | App 字段 | HTTP 客户端 | 站点、检测 |

### 4.2 瘦 App 设计

`App` 瘦身成**只持有公共设施引用**的组装根，业务包通过**注入的接口**拿到它们，不再直接认识 `*App`。

```go
// internal/core/app.go（瘦身后）
type App struct {
    db     *sql.DB
    key    []byte
    dataDir string
    client *http.Client

    // 公共设施（通过接口暴露给领域包）
    mu               sync.RWMutex
    browserSessions  map[string]BrowserLoginSession
    readCache        map[string]readCacheEntry
    digestChannels   map[string]*webhookChannel
    channelRateLimits map[string]*channelRateLimiter
    taskRunner       *TaskRunner
    schedulerCancel  context.CancelFunc

    // 领域服务（在 NewApp 中装配）
    accounts     accounts.Service
    checkin      checkin.Service
    channels     channels.Service
    notifications notifications.Service
    // ... 其他领域
}
```

### 4.3 公共设施接口定义

在 `internal/core/infra.go` 定义领域包可用的接口。领域包依赖这些接口，不依赖 `*App`：

```go
// internal/core/infra.go
package core

import (
    "context"
    "database/sql"
    "net/http"
    "sync"
)

// SharedInfra 是各领域包可访问的公共设施抽象。
// 领域包通过此接口拿到 db / 锁 / http client 等，不直接依赖 *App。
type SharedInfra interface {
    DB() *sql.DB
    HTTPClient() *http.Client
    Key() []byte
    DataDir() string
    Locker() sync.Locker          // 暴露 mu（粗粒度锁）
    BrowserSessions() BrowserSessionStore
    ReadCache() ReadCacheStore
    TaskRunner() TaskRunnerPort
    NotificationHub() NotificationHubPort
}

// BrowserSessionStore 浏览器登录态存储接口（让账号包可单测）
type BrowserSessionStore interface {
    Get(accountID string) (BrowserLoginSession, bool)
    Set(accountID string, s BrowserLoginSession)
    Delete(accountID string)
}

// ReadCacheStore 读缓存接口
type ReadCacheStore interface {
    Get(key string) (interface{}, bool)
    Set(key string, value interface{}, ttl time.Duration)
    Invalidate(key string)
}

// TaskRunnerPort 任务引擎接口（让签到包可单测，未来可换云端队列）
type TaskRunnerPort interface {
    Start(ctx context.Context, kind string, total int) string
    ReportProgress(taskID string, processed, success, failed int)
    Stream(ctx context.Context, taskID string, w http.ResponseWriter)
    Cancel(taskID string) bool
}

// NotificationHubPort 通知中心接口
type NotificationHubPort interface {
    Send(ctx context.Context, event NotificationEvent) error
    RegisterChannel(name string, ch NotificationChannel)
}
```

### 4.4 领域包接收接口而非 *App

```go
// internal/accounts/service.go
package accounts

type Service struct {
    db    *sql.DB
    infra core.SharedInfra   // 通过接口拿公共设施
    // ... 其他依赖
}

func NewService(infra core.SharedInfra) *Service {
    return &Service{
        db:    infra.DB(),
        infra: infra,
    }
}

// handler 通过 s.infra.Locker() 拿锁，不直接碰 *App
```

### 4.5 为什么不选其他选项

| 选项 | 为什么不选 |
|------|-----------|
| 公共设施集中到 `infra` 包（选项 A） | `infra` 会重新长成 god object，治标不治本 |
| 每样设施独立成包（选项 B） | 包数量过多，对单机工具是过度设计 |
| **瘦 App + 接口注入（选项 C，已选）** | 唯一同时满足"可测试性"+"云端铺路"两个动机 |

---

## 5. 领域划分与推进顺序

### 5.1 领域画像

| 领域 | 包含文件 | 共享状态依赖 | 被别人依赖 | 风险 | 推进档位 |
|------|---------|-------------|-----------|------|---------|
| ① 通知 notifications | `notification.go` | 仅 `digestChannels`/`mu` | 被签到调用 | 低 | 第一档 |
| ② 审计 audit | `audit.go` | 仅 `db` | 被多处调用记日志 | 低 | 第一档 |
| ③ 系统设置 system | `system.go`, `autostart.go`, `platform_*`, `legacy_check.go`, `version_check.go` | `db` | 被前端设置页 | 低 | 第一档 |
| ④ 备份导出 backup | `backup_zip.go`, `crypto.go` | `db`, `key` | 独立 | 低 | 第一档 |
| ⑤ 站点/检测 site+detect | `sites.go`, `scanner.go`, `detection_*.go`, `url_safety.go`, `network.go` | `db`, `client` | 被导入用 | 中 | 第二档 |
| ⑥ 渠道 channels | `channels.go`, `channel_*.go`, `models_pricing.go` | `db`, `mu` | 被仪表盘/调度 | 中 | 第二档 |
| ⑦ 账号 accounts（含导入） | `accounts.go`, `chrome_password_import.go`, `legacy_config.go`, `import_*.go`, `local_newapi.go`, `sync_preview.go`, `auto_detect.go` | `db`, `mu`, `browserSessions` | 被签到 | 高 | 第三档 |
| ⑧ 签到 checkin | `checkin_balance.go`, `task_runner.go`, `scheduler.go`, `dry_run.go` | 全部 | 被调度器驱动 | 最高 | 第三档 |

### 5.2 推进顺序

**第一档（先拆，低风险练手）：① → ② → ③ → ④**

这 4 个领域几乎不碰核心共享状态，搬走对其他领域零影响。用来验证"接口缝 + 独立包"这套打法能否跑通。每搬完一个就能立刻编译测试，发现问题好回退。

**第二档（中风险，拆完收益大）：⑤ → ⑥**

依赖稍多但边界清晰，搬完能让仪表盘和调度的依赖更干净。此时"瘦 App"模式已跑通，有经验可复用。

**第三档（最后拆，核心硬骨头）：⑦ → ⑧**

账号被签到依赖、签到依赖所有公共设施——这是耦合最重的地方。等前面 6 个领域搬完、公共设施接口稳定后，再动这两个。此时 `*App` 已大幅瘦身，动核心的风险反而比现在小。

### 5.3 每个领域一次提交

- 每次提交必须通过 `go build && go test && go vet && npx tsc --noEmit && npm run build`
- 每次提交是一个原子回退单位
- 每次提交后做一次该领域的手动冒烟

### 5.4 共享文件归属

部分文件跨领域共享，按"最自然的使用方"安家：

| 文件 | 归属 | 理由 |
|------|------|------|
| `models.go` | `internal/core`（留在组装根） | 跨领域共享类型 |
| `http.go` | `internal/core` | `writeJSON` 等共享工具 |
| `routes.go` | `internal/core` | 装配入口，调用各领域 `RegisterRoutes` |
| `db.go` | `internal/core` | schema 迁移，启动期一次性 |
| `filters.go` | `internal/core` | 通用查询解析 |
| `read_cache.go` | `internal/core` | 公共设施实现 |
| `action_center.go` | `internal/core` | 跨领域聚合视图 |
| `analytics.go` | `internal/core` | 跨领域聚合查询 |
| `diagnostics.go` | `internal/core` | 跨领域聚合诊断 |
| `health.go` | `internal/core` | 健康检查端点 |
| `usage_overview.go` | `internal/core` | 跨领域聚合（用量总览，跨账号/渠道） |

---

## 6. 测试策略

### 6.1 三道安全网

**第一道：接口契约测试（搬动前补）**

拆包本质是给业务逻辑换依赖（从 `*App` 换成注入的接口）。换之前，先给该领域补 1-2 个**通过接口调用**的测试，锁住行为。搬完后跑同一套测试，行为不变即安全。

**第二道：每次提交可独立回退**

一个领域 = 一次 commit = 一次全量构建测试全绿。任何一步红了，`git revert` 单个 commit 即可恢复。

**第三道：手动冒烟**

启动 `relaycheck.exe`，点一遍该领域功能。自动化测试覆盖不到 SSE 流、浏览器登录等桌面特有行为，手动冒烟补这个盲区。复用 `docs/manual-test-record.md` 模板。

### 6.2 测试缺口补偿

对拆包顺序里的低覆盖领域，搬动时顺手补测：

| 领域 | 现有测试数 | 补测目标 |
|------|-----------|---------|
| sites | 1 | 搬前补 2-3 个 CRUD 测试 |
| accounts | 仅 cleanup+key | 搬前补 CRUD + 批量操作测试 |

拆包本来就要碰这些代码，补测是顺手保险。

### 6.3 不做的事（YAGNI）

- ❌ 不引入 E2E 测试框架（单机工具，手动冒烟足够）
- ❌ 不追求覆盖率指标（重构期目标是"行为不变"，不是"提高覆盖率"）
- ❌ 不为公共设施接口写 mock 框架（先用真实现，云端迁移时再换）

---

## 7. 实施规范

### 7.1 单领域拆包步骤（以 notifications 为例）

```
Step 1: 在 internal/core/infra.go 定义 NotificationHubPort 接口
Step 2: 让 notification.go 的函数依赖接口而非 *App（仍在 package core）
        → go build && go test（此时还在 core 包内，验证接口缝正确）
Step 3: git commit "refactor(notifications): extract interface seam"
Step 4: 创建 internal/notifications/ 包，把 notification.go 挪进去
        → 修正 import，类型从 *App 方法改为 Service 方法
        → go build && go test
Step 5: git commit "refactor(notifications): move to own package"
Step 6: 在 routes.go 中用 notifications.Service 装配
        → go build && go test && go vet && npm run build
Step 7: 手动冒烟：触发签到看通知是否发出
Step 8: git commit "refactor(notifications): wire into App assembly"（如有独立变更）
```

### 7.2 包间依赖规则

- 领域包**可以**依赖 `internal/core`（拿接口定义 + 共享类型）
- 领域包**不能**互相依赖（accounts 不能 import checkin）
- 跨领域协作通过 `*App` 在组装层编排，或通过事件接口（如签到完成后调 `NotificationHub.Send`）
- 若发现领域包必须互相依赖，说明领域划错了，需重新划界

### 7.3 命名规范

- 包名：单数小写（`accounts` 而非 `account`，与 Go 惯例一致）
- 服务类型：`Service`（每个领域一个）
- 存储接口：`Store`（如 `AccountStore`，定义在 core 供领域实现）
- Handler 类型：直接用包级函数或 `Handler` 类型

### 7.4 循环依赖处理

若 `core` 定义接口、领域包实现接口、`core` 又 import 领域包 → 循环依赖。

**解法：** 接口定义放在 `core`，领域包 import `core` 实现接口；`core` 的 `App` 字段类型用 `core` 自己定义的接口（而非领域包的 `Service` 类型），在 `NewApp` 中把领域 `Service` 实例赋给接口字段。Go 的结构化类型系统支持这种"接口在消费方、实现在提供方"模式。

---

## 8. 云端迁移路径（未来预留，不现在实现）

本次重构完成后，云端迁移有以下抓手：

| 云端需求 | 本次预留的缝 |
|---------|------------|
| 多用户数据隔离 | `AccountStore` 接口可换多租户实现 |
| 远程数据库 | `SharedInfra.DB()` 可换为连接池 |
| 分布式任务调度 | `TaskRunnerPort` 可换云端队列 |
| 多渠道通知聚合 | `NotificationHubPort` 可换 SaaS 通知服务 |
| 对象存储备份 | `backup` 领域的 `StorageBackend` 接口（未来抽） |

**本次不实现任何云端代码**，只确保接口形状不阻挡未来。

---

## 9. 验收标准

每个领域拆包完成的验收清单：

- [ ] 领域文件已移入独立包，`package core` 文件数减少
- [ ] 领域代码不再直接依赖 `*App`，通过 `SharedInfra` 接口拿依赖
- [ ] `go build && go test && go vet` 全绿
- [ ] `npx tsc --noEmit && npm run build` 全绿（前端不受影响）
- [ ] 该领域功能手动冒烟通过
- [ ] 单次提交，commit message 形如 `refactor(<domain>): <action>`

整体重构完成的验收标准：

- [ ] `internal/core` 文件数从 65+ 降到 ~15（组装根 + 公共设施 + 跨领域聚合）
- [ ] 8 个领域包各自独立，包间无互相依赖
- [ ] `*App` 方法数从 281 降到 ~30（只剩装配 + 聚合视图）
- [ ] 现有 DB 数据无缝迁移（schema 不变）
- [ ] `/api/*` 路径与响应结构完全不变
- [ ] 单二进制分发保持（`go:embed` 仍工作）

---

## 10. 风险与缓解

| 风险 | 缓解 |
|------|------|
| 拆包过程中引入行为变化 | 每步接口契约测试 + 全量构建 + 手动冒烟 |
| 循环依赖卡住 | 接口放消费方、实现放提供方；必要时重新划界 |
| 公共设施接口设计不当导致反复改 | 先在低风险领域（通知/审计）跑通 2 个，稳定后再推广 |
| 重构周期长、中途想停 | 每次提交独立可用，随时可停在任一领域 |
| 跨领域聚合视图（analytics/action_center）拆不动 | 这些留在 core，作为组装根的聚合层 |

---

## 11. 不在本次范围

- ❌ 不改前端代码（除非接口响应结构调整，本次承诺不改）
- ❌ 不改 DB schema
- ❌ 不实现云端代码
- ❌ 不引入新框架/库
- ❌ 不做性能优化（除非拆包顺手暴露的问题）
- ❌ 不删功能、不加功能

---

## 12. 时间线估算（非承诺）

按"低风险优先"顺序，每档作为一个里程碑：

- **里程碑 1**（第一档 4 个领域）：作为"脚手架验证"，跑通接口缝 + 独立包 + 装配流程
- **里程碑 2**（第二档 2 个领域）：中风险领域，验证模式可扩展
- **里程碑 3**（第三档 2 个领域）：核心硬骨头，此时风险已大幅降低

每个领域独立提交，可在任意里程碑后暂停而不影响系统可用性。

---

## 附录 A：领域文件清单（拆包时参考）

### ① notifications
- `notification.go`
- `notification_test.go`

### ② audit
- `audit.go`
- `audit_test.go`

### ③ system
- `system.go`
- `autostart.go`
- `platform_windows.go`
- `platform_other.go`
- `legacy_check.go`
- `version_check.go`
- `version_check_test.go`
- `system_backup_test.go`
- `system_restore_test.go`
- `system_status_test.go`

### ④ backup
- `backup_zip.go`
- `crypto.go`
- `key_export_security_test.go`

### ⑤ site + detect
- `sites.go`
- `sites_test.go`
- `scanner.go`
- `scanner_test.go`
- `detection_engine.go`
- `detection_engine_test.go`
- `detection_detail.go`
- `url_safety.go`
- `url_safety_test.go`
- `network.go`
- `network_test.go`

### ⑥ channels
- `channels.go`
- `channel_models.go`
- `channel_models_test.go`
- `channel_schedules.go`
- `channel_schedules_test.go`
- `channel_health.go`
- `channel_health_test.go`
- `channel_health_probe_task_test.go`
- `models_pricing.go`（渠道模型同步）
- `models_pricing_test.go`

### ⑦ accounts（含导入子领域）
- `accounts.go`
- `accounts_cleanup_test.go`
- `accounts_key_test.go`
- `chrome_password_import.go`
- `legacy_config.go`
- `import_sqlite.go`（账号导入）
- `import_admin_api.go`（账号导入）
- `local_newapi.go`（本地实例管理，导入入口）
- `sync_preview.go`（导入同步预览）
- `sync_preview_test.go`
- `auto_detect.go`（自动检测导入）
- `auto_detect_test.go`

### ⑧ checkin
- `checkin_balance.go`
- `checkin_status_test.go`
- `balance_bulk_test.go`
- `bulk_test_api_keys_test.go`
- `task_runner.go`
- `scheduler.go`
- `scheduler_test.go`
- `dry_run.go`
- `dry_run_test.go`

### 留在 core（组装根 + 公共设施 + 跨领域聚合）
- `app.go`
- `db.go`
- `routes.go`
- `http.go`
- `models.go`
- `filters.go`
- `read_cache.go`
- `read_cache_test.go`
- `infra.go`（新增）
- `action_center.go`
- `action_center_test.go`
- `analytics.go`
- `diagnostics.go`
- `health.go`
- `health_test.go`
- `usage_overview.go`（跨领域聚合）
- `usage_overview_test.go`
- `http_security_test.go`
- `secrets_security_test.go`
- `db_ensure_column_test.go`
- `db_performance_test.go`
- `perf_large_dataset_test.go`
- `app_test.go`
- `testhelper_test.go`
- `encoding_test.go`
- `PACKAGE_INDEX.md`

---

**批准后下一步：** 调用 `writing-plans` skill 把本设计转化为分步实施计划。
