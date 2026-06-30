# 实施计划：架构演进 — 从单包 god object 到领域化分层

**关联设计：** [2026-06-29-architecture-evolution-design.md](2026-06-29-architecture-evolution-design.md)
**日期：** 2026-06-29
**状态：** 待批准

---

## 概述

把 `internal/core` 单包（65+ 文件、281 个 `*App` 方法）按 8 个业务领域拆成独立包，采用"瘦 App + 接口注入"模式，按风险三档顺序渐进推进。每个领域遵循"接口缝 → 移包 → 装配"三 commit 模式，每次提交全绿可独立回退。

## 架构决策

- **纵向领域切片**（非水平分层）：直接解决"改动牵连"痛点
- **瘦 App + 接口注入**：`*App` 只持有公共设施引用，领域包通过 `SharedInfra` 接口拿依赖
- **接口在消费方、实现在提供方**：接口定义放 `core`，领域包 import `core` 实现接口，避免循环依赖
- **每领域 3 commit**：接口缝 / 移包 / 装配，每个 commit 是原子回退单位
- **领域间无互相依赖**：跨领域协作通过 `*App` 组装层编排

## 通用验收标准（每个任务通用）

- [ ] `go build ./...` 全绿
- [ ] `go test ./internal/...` 全绿
- [ ] `go vet ./...` 全绿
- [ ] `cd frontend && npx tsc --noEmit && npm run build` 全绿
- [ ] 无循环依赖（`go build` 自动检测）
- [ ] 单次提交，commit message 形如 `refactor(<domain>): <action>`

## 通用验证命令

```powershell
cd e:\zidqiandao\relaycheck-desktop
go build -mod=vendor ./...
go test -mod=vendor ./internal/... -count=1 -timeout 120s
go vet -mod=vendor ./...
cd frontend ; npx tsc --noEmit ; npm run build ; cd ..
```

---

## Phase 0：Foundation — 搭建公共设施接口层

### Task 0.1：创建 infra.go + App 实现 SharedInfra

**描述：** 新建 `internal/core/infra.go`，定义 `SharedInfra` 接口及 4 个端口接口（`BrowserSessionStore`、`ReadCacheStore`、`TaskRunnerPort`、`NotificationHubPort`）。让 `*App` 实现 `SharedInfra`（添加 `DB()`/`HTTPClient()`/`Key()`/`DataDir()`/`Locker()`/`BrowserSessions()`/`ReadCache()`/`TaskRunner()`/`NotificationHub()` 方法）。此阶段不改动任何领域代码，只新增接口 + 装配方法。

**验收标准：**
- [ ] `infra.go` 存在，定义 `SharedInfra` + 4 个端口接口
- [ ] `*App` 实现 `SharedInfra`（编译期可通过 `var _ SharedInfra = (*App)(nil)` 断言验证）
- [ ] 通用验收标准全绿
- [ ] 现有所有测试不变（接口新增不影响现有代码）

**验证：** 通用验证命令 + 额外 `go vet -mod=vendor ./internal/core/` 确认接口断言通过

**依赖：** None

**可能触碰文件：**
- `internal/core/infra.go`（新增）
- `internal/core/app.go`（添加 9 个 getter 方法，~30 行）

**规模：** M（3-5 文件，但改动量小）

**提交：** `refactor(core): add SharedInfra interface and App getters`

---

## Phase 1：Tier 1 — 低风险领域练手（4 个领域）

目标：验证"接口缝 + 独立包 + 装配"三 commit 模式能跑通。每个领域独立推进，互不依赖。

### Task 1.1：notifications 领域拆包

**描述：** 把 `notification.go`（+ test）从 `core` 拆到 `internal/notifications/` 包。notification 仅依赖 `digestChannels`/`mu`，风险最低，作为模式验证首选。

**子步骤（3 commit）：**
1. **接口缝**：在 `infra.go` 补 `NotificationEvent`/`NotificationChannel` 类型（若不存在）；让 `notification.go` 内部函数通过 `NotificationHubPort` 接口而非 `*App` 字段访问 `digestChannels`；commit `refactor(notifications): extract interface seam`
2. **移包**：创建 `internal/notifications/`，移入 `notification.go` + `notification_test.go`；改 `package core` → `package notifications`；`*App` 方法改为 `Service` 方法；commit `refactor(notifications): move to own package`
3. **装配**：在 `app.go` 的 `NewApp` 中构造 `notifications.NewService(a)`；在 `routes.go` 把通知相关路由 handler 改为调用 `a.notifications.*`；commit `refactor(notifications): wire into App assembly`

**验收标准：**
- [ ] `internal/notifications/` 包存在，含 `notification.go` + test
- [ ] `package core` 不再含 `notification.go`
- [ ] notification handler 不再是 `*App` 方法
- [ ] 通用验收标准全绿
- [ ] 手动冒烟：触发签到看通知是否发出（Webhook/Telegram/Bark 任一已配置通道）

**验证：** 通用验证命令 + 手动触发一次签到任务，确认通知正常送达

**依赖：** Task 0.1

**可能触碰文件：**
- `internal/notifications/notification.go`（从 core 移入）
- `internal/notifications/notification_test.go`（从 core 移入）
- `internal/core/infra.go`（补通知类型）
- `internal/core/app.go`（装配 + 删原文件引用）
- `internal/core/routes.go`（路由 handler 改装配）

**规模：** M（5 文件，机械搬运 + import 修正）

---

### Task 1.2：audit 领域拆包

**描述：** 把 `audit.go`（+ test）拆到 `internal/audit/`。audit 仅依赖 `db`，被多处调用记日志，但调用方都通过 `*App` 间接调，移包后调用方改为通过 `SharedInfra.DB()` 自查或保留 `*App` 转发。

**子步骤（3 commit）：** 同 Task 1.1 模式

**验收标准：**
- [ ] `internal/audit/` 包存在
- [ ] `package core` 不再含 `audit.go`
- [ ] 通用验收标准全绿
- [ ] 手动冒烟：触发一次需审计的操作（如账号增删），确认审计日志正常写入

**验证：** 通用验证命令 + 手动操作后查 `/api/system/audit-log`

**依赖：** Task 0.1（与 Task 1.1 互不依赖，但串行执行避免 `routes.go`/`app.go` 合并冲突）

**可能触碰文件：**
- `internal/audit/audit.go` + `audit_test.go`
- `internal/core/app.go`、`routes.go`

**规模：** M（4 文件）

**提交：** 3 个 commit，前缀 `refactor(audit):`

---

### Task 1.3：system 领域拆包

**描述：** 把系统设置相关文件（`system.go`、`autostart.go`、`platform_windows.go`、`platform_other.go`、`legacy_check.go`、`version_check.go` + 对应 test）拆到 `internal/system/`。仅依赖 `db`。

**子步骤（3 commit）：** 同前。注意 `platform_windows.go`/`platform_other.go` 用 build tag，移包后需保留 tag。

**验收标准：**
- [ ] `internal/system/` 包存在，含 6 源文件 + 3 test
- [ ] `package core` 不再含上述文件
- [ ] build tag 正确保留（`//go:build windows` / `//go:build !windows`）
- [ ] 通用验收标准全绿
- [ ] 手动冒烟：打开设置页，确认系统设置/自启/版本检查/诊断正常显示

**验证：** 通用验证命令 + 启动应用访问设置页

**依赖：** Task 1.2（串行）

**可能触碰文件：**
- `internal/system/` 下 9 文件
- `internal/core/app.go`、`routes.go`

**规模：** M（11 文件，但机械搬运）

**提交：** 3 个 commit，前缀 `refactor(system):`

---

### Task 1.4：backup 领域拆包

**描述：** 把 `backup_zip.go` + `crypto.go`（+ `key_export_security_test.go`）拆到 `internal/backup/`。依赖 `db` + `key`。`crypto.go` 的 `encryptText`/`decryptText` 被账号领域调用，需在 `infra.go` 暴露 `CryptoPort` 或保留 `core` 转发。

**子步骤（3 commit）：** 同前。关键决策：`crypto.go` 是否整个移走？若账号领域仍需 `encryptText`，在 `infra.go` 加 `CryptoPort` 接口让账号通过接口调用，避免领域间直接依赖。

**验收标准：**
- [ ] `internal/backup/` 包存在
- [ ] `package core` 不再含 `backup_zip.go`/`crypto.go`
- [ ] `infra.go` 含 `CryptoPort` 接口，`*App` 实现
- [ ] 通用验收标准全绿
- [ ] 手动冒烟：执行一次加密导出 + 导入，确认往返正常

**验证：** 通用验证命令 + 手动导出导入 zip

**依赖：** Task 1.3（串行）

**可能触碰文件：**
- `internal/backup/` 下 3 文件
- `internal/core/infra.go`（加 CryptoPort）
- `internal/core/app.go`、`routes.go`

**规模：** M（5 文件）

**提交：** 3 个 commit，前缀 `refactor(backup):`

---

## Checkpoint 1：Tier 1 完成

- [ ] 4 个领域包独立存在，`package core` 文件数减少 ~14
- [ ] 所有通用验收标准持续全绿
- [ ] 手动冒烟 4 个领域功能均正常
- [ ] `*App` 方法数从 281 减少（预估 ~240）
- [ ] **人工 review 后再进入 Phase 2**

**验证：** 全量构建测试 + 启动应用点一遍 4 个领域功能

---

## Phase 2：Tier 2 — 中风险领域（2 个领域）

### Task 2.1：site + detect 领域拆包

**描述：** 把站点/检测相关 11 文件（`sites.go`、`scanner.go`、`detection_engine.go`、`detection_detail.go`、`url_safety.go`、`network.go` + 5 test）拆到 `internal/sitedetect/`。依赖 `db` + `client`。被导入子领域调用（Phase 3 才拆），暂通过 `*App` 转发。

**子步骤（3 commit）：** 同前。`network.go` 的 HTTP client 通过 `SharedInfra.HTTPClient()` 拿。

**验收标准：**
- [ ] `internal/sitedetect/` 包存在，含 6 源文件 + 5 test
- [ ] `package core` 不再含上述文件
- [ ] 通用验收标准全绿
- [ ] 手动冒烟：站点列表/批量检测/扫描本地 NewAPI 正常

**验证：** 通用验证命令 + 启动应用访问站点页 + 扫描功能

**依赖：** Checkpoint 1

**可能触碰文件：**
- `internal/sitedetect/` 下 11 文件
- `internal/core/app.go`、`routes.go`

**规模：** M（13 文件，机械搬运）

**提交：** 3 个 commit，前缀 `refactor(sitedetect):`

---

### Task 2.2：channels 领域拆包

**描述：** 把渠道相关 10 文件（`channels.go`、`channel_models.go`、`channel_schedules.go`、`channel_health.go`、`models_pricing.go` + 5 test）拆到 `internal/channels/`。依赖 `db` + `mu`。被仪表盘/调度调用，通过 `*App` 转发。

**子步骤（3 commit）：** 同前。

**验收标准：**
- [ ] `internal/channels/` 包存在，含 5 源文件 + 5 test
- [ ] `package core` 不再含上述文件
- [ ] 通用验收标准全绿
- [ ] 手动冒烟：渠道列表/模型同步/健康概览/排期日历正常

**验证：** 通用验证命令 + 启动应用访问渠道页 + 触发模型同步

**依赖：** Task 2.1（串行）

**可能触碰文件：**
- `internal/channels/` 下 10 文件
- `internal/core/app.go`、`routes.go`

**规模：** M（12 文件，机械搬运）

**提交：** 3 个 commit，前缀 `refactor(channels):`

---

## Checkpoint 2：Tier 2 完成

- [ ] 6 个领域包独立，`package core` 文件数减少 ~25
- [ ] `*App` 方法数预估 ~180
- [ ] **人工 review 后再进入 Phase 3（核心硬骨头）**

**验证：** 全量构建测试 + 完整功能冒烟

---

## Phase 3：Tier 3 — 核心硬骨头（2 个领域，拆子任务）

这两个领域文件多、耦合重，按子任务拆分以控制单任务规模 ≤5 文件。

### Task 3.1：accounts 接口缝

**描述：** 在 `infra.go` 加 `AccountStore` 接口（CRUD + 批量操作签名）；让 `accounts.go` 内部函数通过接口而非 `*App` 字段访问 `browserSessions`（用 `BrowserSessionStore`）；仍在 `package core` 内。

**验收标准：**
- [ ] `infra.go` 含 `AccountStore` 接口
- [ ] `accounts.go` 不再直接访问 `a.browserSessions`，改用 `a.BrowserSessions()` 接口
- [ ] 通用验收标准全绿

**依赖：** Checkpoint 2

**可能触碰文件：**
- `internal/core/infra.go`
- `internal/core/accounts.go`

**规模：** S（2 文件）

**提交：** `refactor(accounts): extract interface seam`

---

### Task 3.2：accounts 核心移包

**描述：** 创建 `internal/accounts/`，移入 `accounts.go` + `accounts_cleanup_test.go` + `accounts_key_test.go` + `chrome_password_import.go` + `legacy_config.go`。改 package + Service 化。

**验收标准：**
- [ ] `internal/accounts/` 包存在，含 5 文件
- [ ] `package core` 不再含上述文件
- [ ] 通用验收标准全绿

**依赖：** Task 3.1

**可能触碰文件：**
- `internal/accounts/` 下 5 文件

**规模：** M（5 文件）

**提交：** `refactor(accounts): move core to own package`

---

### Task 3.3：accounts 导入子领域移包

**描述：** 把导入相关文件移入 `internal/accounts/`（作为 accounts 包的 import 子目录或同包文件）：`import_sqlite.go`、`import_admin_api.go`、`local_newapi.go`、`sync_preview.go`、`auto_detect.go` + 2 test。

**验收标准：**
- [ ] 上述文件在 `internal/accounts/`（或其 `import` 子包）
- [ ] `package core` 不再含上述文件
- [ ] 通用验收标准全绿
- [ ] 手动冒烟：从本地 NewAPI 导入账号 + Chrome 密码导入预览正常

**依赖：** Task 3.2

**可能触碰文件：**
- `internal/accounts/` 下 7 文件

**规模：** M（7 文件，机械搬运）

**提交：** `refactor(accounts): move import subdomain`

---

### Task 3.4：accounts 装配

**描述：** 在 `app.go` 构造 `accounts.NewService(a)`；`routes.go` 把账号/导入路由 handler 改为调用 `a.accounts.*`。

**验收标准：**
- [ ] 账号路由全部走 `a.accounts.*`
- [ ] 通用验收标准全绿
- [ ] 手动冒烟：账号 CRUD + 批量登录/测试/刷新 + 浏览器登录全流程

**依赖：** Task 3.3

**可能触碰文件：**
- `internal/core/app.go`、`routes.go`

**规模：** S（2 文件）

**提交：** `refactor(accounts): wire into App assembly`

---

### Task 3.5：checkin 接口缝

**描述：** 在 `infra.go` 确认 `TaskRunnerPort` 完整（Start/ReportProgress/Stream/Cancel）；让 `checkin_balance.go`/`task_runner.go`/`scheduler.go` 通过接口而非 `*App` 访问 `taskRunner`；仍在 `package core`。

**验收标准：**
- [ ] `checkin_balance.go` 不再直接访问 `a.taskRunner`，改用 `a.TaskRunner()` 接口
- [ ] 通用验收标准全绿

**依赖：** Task 3.4

**可能触碰文件：**
- `internal/core/infra.go`
- `internal/core/checkin_balance.go`、`task_runner.go`、`scheduler.go`

**规模：** M（4 文件）

**提交：** `refactor(checkin): extract interface seam`

---

### Task 3.6：task_runner + scheduler 移包

**描述：** 创建 `internal/checkin/`，移入 `task_runner.go` + `scheduler.go` + `scheduler_test.go` + `dry_run.go` + `dry_run_test.go`。这些是签到的调度基础设施。

**验收标准：**
- [ ] `internal/checkin/` 包存在，含上述文件
- [ ] `package core` 不再含上述文件
- [ ] 通用验收标准全绿
- [ ] 手动冒烟：调度器启动 + 手动触发签到任务 + SSE 进度流正常

**依赖：** Task 3.5

**可能触碰文件：**
- `internal/checkin/` 下 5 文件

**规模：** M（5 文件）

**提交：** `refactor(checkin): move task_runner and scheduler`

---

### Task 3.7：checkin_balance + 测试移包

**描述：** 把 `checkin_balance.go` + `checkin_status_test.go` + `balance_bulk_test.go` + `bulk_test_api_keys_test.go` 移入 `internal/checkin/`。

**验收标准：**
- [ ] 上述文件在 `internal/checkin/`
- [ ] `package core` 不再含上述文件
- [ ] 通用验收标准全绿
- [ ] 手动冒烟：单账号签到 + 余额刷新 + API key 测试正常

**依赖：** Task 3.6

**可能触碰文件：**
- `internal/checkin/` 下 4 文件

**规模：** M（4 文件）

**提交：** `refactor(checkin): move checkin_balance`

---

### Task 3.8：checkin 装配

**描述：** 在 `app.go` 构造 `checkin.NewService(a)`；`routes.go` 把任务/调度路由 handler 改为调用 `a.checkin.*`；`StartSchedulers` 改为委托给 `a.checkin`。

**验收标准：**
- [ ] 任务/调度路由全部走 `a.checkin.*`
- [ ] 通用验收标准全绿
- [ ] 手动冒烟：完整签到流程（调度触发 + 进度流 + 通知 + 余额刷新）

**依赖：** Task 3.7

**可能触碰文件：**
- `internal/core/app.go`、`routes.go`

**规模：** S（2 文件）

**提交：** `refactor(checkin): wire into App assembly`

---

## Checkpoint 3：Tier 3 完成 = 整体重构完成

- [ ] 8 个领域包全部独立，包间无互相依赖
- [ ] `internal/core` 文件数从 65+ 降到 ~25（组装根 + 公共设施 + 跨领域聚合）
- [ ] `*App` 方法数从 281 降到 ~30（只剩装配 + 聚合视图）
- [ ] 现有 DB 数据无缝迁移（schema 不变）
- [ ] `/api/*` 路径与响应结构完全不变
- [ ] 单二进制分发保持（`go:embed` 仍工作）
- [ ] **完整人工 review + 全功能冒烟**

**验证：** 全量构建测试 + 启动应用走完所有功能（仪表盘/账号/渠道/站点/签到/通知/设置/导入/备份）

---

## 任务依赖图

```
Task 0.1 (infra.go)
    │
    ├── Task 1.1 (notifications) ─┐
    │     ├── 1.1a seam            │
    │     ├── 1.1b move            │ Tier 1 串行
    │     └── 1.1c wire            │（routes.go/app.go 合并冲突）
    ├── Task 1.2 (audit) ──────────┤
    ├── Task 1.3 (system) ─────────┤
    └── Task 1.4 (backup) ─────────┘
                │
          Checkpoint 1
                │
    ├── Task 2.1 (sitedetect) ─────┐ Tier 2 串行
    └── Task 2.2 (channels) ───────┘
                │
          Checkpoint 2
                │
    ├── Task 3.1 (accounts seam)   │
    ├── Task 3.2 (accounts core)   │
    ├── Task 3.3 (accounts import) │ Tier 3 串行
    ├── Task 3.4 (accounts wire)   │（核心硬骨头）
    ├── Task 3.5 (checkin seam)    │
    ├── Task 3.6 (checkin runner)  │
    ├── Task 3.7 (checkin balance) │
    └── Task 3.8 (checkin wire)    │
                │
          Checkpoint 3 (完成)
```

## 风险与缓解

| 风险 | 影响 | 缓解 |
|------|------|------|
| `routes.go`/`app.go` 多任务串行合并冲突 | 中 | 严格串行执行领域任务，每任务独立提交 |
| `crypto.go` 移包后账号领域断链 | 高 | Task 1.4 在 `infra.go` 加 `CryptoPort` 接口，账号通过接口调用 |
| `browserSessions` 跨 accounts/checkin 共享 | 高 | Task 3.1 接口缝用 `BrowserSessionStore`，两领域都通过接口访问 |
| 循环依赖（core import 领域包） | 高 | 接口定义留 core，App 字段用 core 接口类型，NewApp 装配时赋领域 Service |
| 中途想停 | 低 | 每个 commit 独立可用，可在任意 Checkpoint 暂停 |
| 跨领域聚合（analytics/action_center）拆不动 | 低 | 这些留 core，spec 5.4 已明确 |

## 开放问题

- **Q1：** `internal/sitedetect` 包名是否过长？可考虑 `sitedetect` / `sites` / `detection`。建议 `sitedetect`（语义清晰）。
- **Q2：** accounts 导入子领域是同包还是子包 `internal/accounts/import`？建议先同包（YAGNI），文件多到难管再拆子包。
- **Q3：** Task 3.6/3.7 是否可合并？若 checkin 总文件数（9）可控，可合并为单 commit。建议先按拆分执行，机械搬运顺畅再考虑合并。

## 并行化机会

- **可并行：** 各领域 test 文件的搬运（但与源文件绑定，实际串行更安全）
- **必须串行：** 所有领域任务（共享 `routes.go`/`app.go`）
- **可并行：** 文档更新（PACKAGE_INDEX.md）可在最后统一更新，不阻塞代码任务

## 提交规范

所有 commit message 前缀：
- `refactor(core):` — Phase 0
- `refactor(notifications):` / `refactor(audit):` / `refactor(system):` / `refactor(backup):` — Phase 1
- `refactor(sitedetect):` / `refactor(channels):` — Phase 2
- `refactor(accounts):` / `refactor(checkin):` — Phase 3

每个 commit body 可附一行说明改动要点。

## 不在本计划范围

- ❌ 不改前端代码
- ❌ 不改 DB schema
- ❌ 不实现云端代码
- ❌ 不引入新框架/库
- ❌ 不做性能优化
- ❌ 不删功能、不加功能
