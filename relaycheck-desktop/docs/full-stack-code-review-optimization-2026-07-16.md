# RelayCheck Desktop 全栈代码审查与优化报告（2026-07-16）

- **日期：** 2026-07-16
- **审查基线：** `ad227ad`（`main` / SSH 远端已对齐）
- **审查范围：** `E:\zidqiandao\relaycheck-desktop`
- **技术栈：** Go 1.24 module（本机 1.26.4 / gate 1.26.5）· `net/http` · SQLite（modernc）· React 19 · TypeScript 5.9 · Vite 8 · Vitest 4
- **审查方式：** 源码与契约审读、静态结构与安全路径检查、依赖与门禁复跑、构建产物体积、覆盖率与既有审查文档交叉核对
- **结论：** 代码库整体健康，**未发现 P0 远程代码执行、SQL 注入、前端直接 XSS sink 或明文密钥入库**。账号启动全量加载（FE-4/BE-2）与通知分页（BE-1）已在 `5147d2b` 落地。当前主要风险集中在：**前端覆盖率偏低与大型组件职责过重、Action Center / usage 查询扩展性、业务错误文本外泄、SSRF DNS rebinding 残余、本机 token 默认关闭与 Windows ACL、构建/发布门禁与本机 Go 版本不一致、包体无预算门禁、以及 FE-4 后 3 个文件未过 Prettier**。

---

## 0. 审查基线

### 0.1 已执行验证（本轮）

| 检查项 | 结果 | 说明 |
|---|---:|---|
| `go test -mod=vendor -count=1 ./internal/...` | 通过 | accounts / core / notifications 等全部 ok |
| `go test -mod=vendor -cover -count=1 ./internal/core/` | 通过 | **coverage 56.0%**（≥ CI 55% 门槛） |
| `npx tsc -b` | 通过 | 无 TS 错误 |
| `npm test` | 通过 | **31 files / 271 tests** |
| `npm run lint` | 通过 | ESLint max-warnings=0；已含 `no-floating-promises` / `no-misused-promises` / `exhaustive-deps=error` |
| `npm run format:check` | **失败** | 3 个文件：`AccountsPanel.tsx`、`useAccountsPage.ts`、`useAccountsPage.test.ts`（FE-4 新增后未 Prettier） |
| `npm run test:coverage` | 通过（低水位） | statements **41%** / branches **36%** / functions **31%** / lines **41%**（阈值 40/35/30/40） |
| `npm audit --audit-level=moderate` | 通过 | 0 vulnerabilities |
| `npm run build` | 通过 | 见 0.2 包体基线 |
| `govulncheck` | 未在本机独立全量跑 | CI + `verify-release.ps1` 已接入；本机 Go 1.26.4 低于 release 要求 1.26.5 |

### 0.2 前端包体基线（本轮 `npm run build`）

| 产物 | 原始 | gzip |
|---|---:|---:|
| 主 CSS `index-*.css` | 205.37 kB | 34.45 kB |
| 主 JS `index-*.js` | 225.65 kB | 70.76 kB |
| `panel-accounts` | 119.49 kB | 35.18 kB |
| `panel-settings` | 40.67 kB | 11.21 kB |
| `panel-channels` | 24.02 kB | 7.36 kB |
| `panel-sites` | 21.31 kB | 6.11 kB |

体积尚可；**仍无自动 gzip 预算门禁**。`panel-accounts` 已接近建议 panel 上限（45 kB gzip）。

### 0.3 优先级定义

| 级别 | 含义 | 建议时限 |
|---|---|---|
| P0 | 远程接管、严重数据损坏、密钥大规模泄露 | 立即阻断发布 |
| P1 | 正确性、门禁可靠性、主路径扩展性 | 当前迭代 |
| P2 | 中期故障率/维护成本/安全暴露面 | 1–2 迭代 |
| P3 | 纵深防御、体验、工程成熟度 | 持续优化 |

**本轮：** P0 **0** · P1 **6** · P2 **10** · P3 **3**

已关闭（相对 2026-07-13 报告）：FE-4 启动全量 accounts、BE-1 通知 limit 压 10、BE-2 账号分页契约（见 `5147d2b`）；FE-2 coverage 脚本、FE-3 ESLint 过弱已基本关闭。

---

## 1. 前端代码审查

### 1.1 已验证优点

- 面板 `React.lazy` + Vite `manualChunks` 分块；非 dashboard 面板按需加载。
- `useApi` 带 `AbortSignal`、短缓存、写后失效；keep-alive 面板 `inert` / `aria-hidden` / `enabled`。
- 未发现 `dangerouslySetInnerHTML` / `eval` / `new Function`。
- 外链经 `safeExternalUrl`；`localStorage` 仅存主题/引导/忽略版本等非密钥偏好。
- **FE-4 已落地：** inventory 只拉 channels + sites + `/api/accounts/summary`；账号列表走 `/api/accounts/page`；渠道筛选走 `/api/accounts/search-index`；站点主从走 page API（limit 200）。
- ESLint 已升级类型感知规则；覆盖率脚本与 `@vitest/coverage-v8` 可用。
- 测试 271、lint 0、tsc 0、build 0、npm audit 0。

### FE-A（P1）Prettier 门禁对 FE-4 新文件失败

**问题描述**

CI 已跑 `format:check`，但 `AccountsPanel.tsx` / `useAccountsPage.ts` / `useAccountsPage.test.ts` 当前不符合 Prettier。本地 `format:check` 失败，会直接阻断 GitHub Actions frontend job。

**影响评估**

- `main` 推送后 CI 可能红。
- 与“format 作为硬门禁”的设计冲突。

**推荐操作步骤**

1. `cd frontend && npm run format`（仅格式，无行为变化）。
2. 本地 `npm run format:check && npm run lint && npx tsc -b && npm test`。
3. 单独 commit：`style(frontend): prettier FE-4 account page files`。
4. 确认 CI Format 步骤通过。

**预期收益**

恢复可合并的 frontend CI；避免后续格式债累积。

**验证方法**

```powershell
cd E:\zidqiandao\relaycheck-desktop\frontend
npm run format:check   # exit 0
```

### FE-B（P1）前端覆盖率整体偏低，关键 hooks 几乎未测

**问题描述**

`test:coverage` 总览约 **41% statements / 36% branches**。关键路径覆盖差：

| 区域 | 约覆盖 | 风险 |
|---|---:|---|
| `hooks/useAccountsPage.ts` | ~18% | 分页/游标栈主路径 |
| `hooks/useChannelActions.ts` | 0% | 渠道写操作与刷新 |
| `hooks/useSiteAccounts.ts` | ~8% | 站点主从数据 |
| `components/settings/*Backup*Proxy*` | 0% | 导入导出与代理 |
| `dialog-shell.tsx` | ~15% | 焦点陷阱/滚动锁 |
| `lib/*` | ~88% | 相对健康 |

阈值仅 40/35/30/40，能绿但不能保护回归。

**影响评估**

- 重构 Accounts/Channels 时缺行为护栏。
- CI 覆盖率门禁几乎形同虚设。

**推荐操作步骤**

1. 为 `buildAccountsPageUrl`（已有）之外补游标栈/abort 竞态的纯函数或轻量 hook 测试。
2. `useChannelActions.refresh` / seed 路径：mock `api`，断言不请求 `/api/accounts` 全量、请求 search-index。
3. Settings 导入导出、DialogShell Escape/focus 各加 2–3 个契约测试。
4. 分两阶段抬阈值：先 statements/lines **50%**，再 **60%**；`api/client.ts`、`useApi.ts`、`navigation.ts` 设文件级更高目标。
5. 覆盖率 artifact 已上传；在 PR 模板中要求贴 `coverage-summary` 摘要。

**预期收益**

把“有测试”升级为“主路径被覆盖”，支撑大组件拆分。

**验证方法**

```powershell
cd frontend
npm run test:coverage
# 目标：statements/lines ≥ 50% 后提高 thresholds
```

### FE-C（P1）`panel-accounts` 体积偏大 + 超大组件职责过重

**问题描述**

- `panel-accounts` **119 kB / 35 kB gzip**，接近建议 panel 上限。
- `AccountInsights.tsx` **939 行**、`AccountCard.tsx` **643 行**、`SettingsCards.tsx` **663 行**、`SitesPanel.tsx` **553 行**。
- 组件内混合：请求、确认流、状态机、格式化、展示。
- Insights/批量重登目前只吃**当前页** accounts（设计如此），但 UI 若暗示“全局”会误导。

**影响评估**

- 账号域任何小改都易触发大 chunk 与宽回归。
- 用户可能对批量操作作用域产生错误预期。

**推荐操作步骤**

1. 按操作拆分，而非按卡片：API Key / 登录会话 / 余额签到 / 清理 / 批量重登 → `useAccountActions` 或 command 模块。
2. `AccountInsights` 对“仅当前页”加明确文案；全局洞察改走 summary 或专用聚合 API。
3. 将 `BulkReloginWizard` / Insights 二级面板再 `lazy`，避免首进 accounts 一次加载全部向导。
4. 每次拆分 100–300 行 diff + 先补行为测试。
5. 加 gzip 预算：主 JS 80 kB、主 CSS 40 kB、单 panel 45 kB（见 CF-C）。

**预期收益**

缩小修改半径；控制 accounts chunk；降低误操作范围误解。

**验证方法**

Profiler 无关区提交不增；`npm run build` gzip 不超预算；关键操作测试仍绿。

### FE-D（P2）渠道数据所有权仍有隐式重复刷新

**问题描述**

`useChannelActions.refresh()` 拉 channels + models + search-index；`ChannelsPanel.refreshAll()` 再调父级 `onRefresh()` → inventory 再拉 channels/sites/summary。依赖短缓存掩盖重复，所有权不清晰。

**影响评估**

缓存失效时双写状态；后续扩展分页/筛选更易漂移。

**推荐操作步骤**

1. 约定 owner：inventory 管 channels/sites/summary；channels hook 只管 models/health/search-index。
2. 写操作按资源键失效，禁止“刷新全部”默认路径。
3. `refreshAll` 改为并行去重计划表。
4. fetch mock：进入渠道页 / 手动刷新 各 endpoint 次数断言。

**预期收益**

减少重复请求；缓存策略可演进。

**验证方法**

冷启动进渠道、手动刷新、模型同步、健康探测四条路径请求次数符合契约。

### FE-E（P2）站点主从 `limit=200` 静默截断

**问题描述**

`useSiteAccounts` 使用 `/api/accounts/page?limit=200&upstreamSiteId=`。单站账号大于 200 时无 nextCursor UI，用户看不到后续账号。

**影响评估**

大站账号管理静默丢数据（比历史全局 1000 截断范围小，但仍是正确性缺口）。

**推荐操作步骤**

1. 主从列表接入与 `AccountsPanel` 相同的 cursor 分页条。
2. 或显示“仅显示前 200，请到全部账号筛选该站”。
3. 测试：插入 201 账号断言 UI 有更多提示或可翻页。

**预期收益**

消除单站静默截断。

**验证方法**

201 账号 fixture 下 ID 集合完整可遍历。

### FE-F（P2）内联 `style={{...}}` 与 CSP `style-src 'unsafe-inline'` 耦合

**问题描述**

`http.go` CSP 允许 `style-src 'self' 'unsafe-inline'`。前端大量 React inline style（tab 显隐、进度条宽度、Settings/Scan 布局）。

**影响评估**

当前无直接 XSS sink，属纵深防御缺口；阻碍收紧 CSP。

**推荐操作步骤**

1. 新代码优先 class / CSS 变量。
2. tab 显隐改为 class `is-hidden` + CSS；进度条保留动态 width 或改 CSS variable。
3. 迁移完成后尝试去掉 `unsafe-inline`；若必须保留写入威胁模型。
4. 加 CSP 响应头测试。

**预期收益**

提升 XSS 纵深防御；样式策略可验证。

**验证方法**

release UI 无 CSP violation；主题/抽屉/进度正常。

### FE-G（P3）启动并行请求面仍偏宽

**问题描述**

首屏同时拉 system status、inventory（3）、ops（checkins/notifications/diagnostics/action-center）、随后 model/pricing/usage。账号全量已去掉，但 action-center + diagnostics 仍重。

**影响评估**

账号不多时体感尚可；运维数据增大后首屏 TTFB/JSON 解析仍抖。

**推荐操作步骤**

1. Dashboard 首屏仅 system + summary + action-center 摘要字段。
2. diagnostics / model / pricing / usage 延后到面板可见或 idle。
3. 记录启动请求 waterfall 基线。

**预期收益**

首屏更稳，弱机器更友好。

---

## 2. 后端代码审查

### 2.1 已验证优点

- SQLite WAL、`busy_timeout`、`foreign_keys`、连接池限制；关键表有组合索引（含 `idx_channel_accounts_updated`、`idx_checkin_logs_account_started`、`idx_balance_snapshots_account_created`）。
- 业务 API `requireSession`；写操作 Origin + loopback `RemoteAddr`；Host allowlist、CSP、nosniff、frame-ancestors、JSON 8MiB。
- 可选 session token：256-bit、HttpOnly、SameSite=Strict、常量时间比较。
- 出站 URL 拒绝 loopback/private/link-local/metadata；redirect 重校验。
- `writeError` 对 **HTTP ≥500** 已统一对外文案 + requestId 日志。
- **BE-2 已落地：** `/api/accounts/page|summary|search-index` + 测试。
- **BE-1 已落地：** 通知 `clampListLimit` + `/api/notifications/page` totals。
- core 覆盖率 **56%**；全 internal 包测试通过。

### BE-A（P1）`writeError` 500 已脱敏，但业务路径仍大量 `err.Error()` 直出

**问题描述**

`writeError` 仅在 `status >= 500` 时替换消息。大量 handler 对失败仍传入 `err.Error()`——500 对外已替换，但多处 400/业务结果把上游或驱动消息放进响应或 `result.Message`，前端当用户文案展示。

典型：`accounts.go` 创建/更新/校验、backup、autostart、audit 等。

**影响评估**

- 本机单用户风险低于公网，但仍可能把路径、SQL、上游 body 片段展示给 UI。
- 错误文案不稳定，驱动升级会改 UX。

**推荐操作步骤**

1. 定义 `AppError{Class, PublicMsg, Err}`；handler 只写 PublicMsg。
2. 4xx：审查过的可操作中文；5xx：稳定文案 + requestId（已有）。
3. 批量任务 `result.Message` 使用枚举/映射，不把 raw err 塞 UI。
4. 测试：构造 DB/备份/上游错误，断言响应无绝对路径、SQL、密钥片段。

**预期收益**

稳定错误契约；降低信息泄露面。

**验证方法**

集成测试抽检 accounts create fail、backup fail、proxy test fail。

### BE-B（P1）Action Center 多段独立 COUNT + sample 查询

**问题描述**

`action_center.go` 对十余类风险项各执行 `COUNT(*)` + sample `SELECT ... LIMIT 4`，外加若干总量 COUNT。缓存有 `overviewReadCacheTTL`，但冷启动/失效时 SQLite 往返次数高。

**影响评估**

账号与日志增大后，Dashboard 首屏与 `/api/system/action-center` p95 变差；与已做的 dashboard summary 单查询优化不对称。

**推荐操作步骤**

1. `EXPLAIN QUERY PLAN` 核对每条是否走索引。
2. 将无参数依赖的 COUNT 合并为单 SQL 多标量子查询（同 BE-10 手法）。
3. sample 查询可按需懒加载（UI 展开再取）。
4. 5k/20k fixture 压测 handler 耗时与响应体。

**预期收益**

首屏运维摘要稳定；减少 SQLite 锁竞争。

**验证方法**

20k 账号下 action-center p95 达团队预算（例本机缓存命中小于 100ms / 冷小于 300ms）。

### BE-C（P1）`usage/overview` 一次拉 1000 行快照再内存聚合

**问题描述**

`buildUsageOverview`：`ORDER BY account_id, created_at DESC LIMIT 1000` 全表片段进内存分组。账号多、快照密时偏差大且无分页。

**影响评估**

用量面板统计不完整或偏向最近写入账户；大库时 JSON/CPU 成本高。

**推荐操作步骤**

1. 改为按账号窗口函数/子查询取每个账号最近 N 条，或维护日聚合表。
2. 增加 site/account 过滤与 limit。
3. 性能测试覆盖该 endpoint（现有 large dataset 测试未覆盖）。

**预期收益**

统计正确性可预期；响应体可控。

**验证方法**

固定 fixture 下 daily use / low balance 与手算一致；LIMIT 边界有测试。

### BE-D（P2）旧 `GET /api/accounts` 仍默认 500/最大 1000 + 相关子查询

**问题描述**

兼容列表仍对每行相关子查询最新 checkin message；默认 500、最大 1000 静默截断。前端主路径已迁走，但任何遗漏调用方仍踩坑。

**影响评估**

外部脚本/旧 UI/调试工具误用导致不完整数据；DB 压力残留。

**推荐操作步骤**

1. 文档与注释标明 deprecated，推荐 `/page`。
2. 响应头 `Deprecation` / `Link` 指向 page。
3. 日志对无 site 过滤的全表 list 打 warning。
4. 中期：默认 limit 降到 100 或强制 `upstreamSiteId`。
5. 子查询与 page 路径统一 scan helper，避免双份 SQL 漂移。

**预期收益**

降低误用；最终可删兼容层。

**验证方法**

CI 契约测试：list 带 Deprecation；page 为唯一无截断遍历方式。

### BE-E（P2）SSRF：校验 DNS 与拨号 DNS 分离（rebinding 时间窗）

**问题描述**

`url_safety.go` 先 `LookupIPAddr` 再由 `http.Transport` 自行解析。恶意 DNS 可在校验返回公网、连接切到私网/metadata。

**影响评估**

当前“用户自配上游 + 本机桌面”模型下为中等残余；未来若接受远端导入 URL 或非 loopback bind，风险上升。

**推荐操作步骤**

1. 自定义 `DialContext` 只连已验证 IP，保留 Host/SNI。
2. 多 IP 仅在已验证集合重试。
3. redirect 每跳重建允许集合。
4. 可注入 resolver 的 rebinding 单测。

**预期收益**

SSRF 防护落到真实连接目标。

**验证方法**

resolver 首次公网、二次 `127.0.0.1` / `169.254.169.254` 必须失败且测试 server 无连接。

### BE-F（P2）本机 token 默认关闭；Windows ACL 不等于 0600

**问题描述**

`RELAYCHECK_REQUIRE_TOKEN=1` 才启用。默认可信单用户：同机进程可调 API。token 文件 `0600` 在 Windows 上不保证 ACL。

**影响评估**

共享电脑/多用户场景横向访问；hardened 模式若 ACL 过宽可读 token。

**推荐操作步骤**

1. Runbook 明确两种模式。
2. 多用户机器默认建议 token=1。
3. 写 token 后用 Windows ACL API 收紧到当前用户 + SYSTEM。
4. 非 loopback bind 时强制 token。

**预期收益**

部署安全边界与文档一致。

**验证方法**

无 cookie → 401；`Get-Acl session-token.txt` 无不必要主体。

### BE-G（P2）`internal/core` 上帝包持续膨胀

**问题描述**

`models_pricing.go` 1197 行、`accounts.go` 890、`scheduler.go` 802、`checkin_balance.go` 910。虽已抽 accounts/channels/sites service，HTTP 与编排仍大量堆在 core。

**影响评估**

编译与测试反馈变慢；循环依赖风险；新人难定位。

**推荐操作步骤**

1. 按边界继续下沉：pricing、scheduler projection、checkin 编排 → 独立 package + interface。
2. `App` 只保留 wiring 与 cross-cutting（cache、audit、http security）。
3. 每迁一块同步 `PACKAGE_INDEX.md` 与测试包路径。
4. 评审约定：禁止新增超过约 400 行的 core 文件。

**预期收益**

可测试性与所有权清晰；覆盖率可按域提升。

**验证方法**

`go test` 包级时间下降；依赖图无新环。

### BE-H（P2）账号搜索索引 `GROUP_CONCAT` 全站拼接

**问题描述**

`loadAccountSearchIndex` 按站 `GROUP_CONCAT` 全部账号 display/email/username。站多账号密时单行极大、内存与 JSON 膨胀。

**影响评估**

渠道筛选搜索变慢；极端数据下响应体失控。

**推荐操作步骤**

1. 限制每站拼接长度或改 FTS/前缀索引表。
2. 渠道搜索改为服务端 query API，而非下发全量 searchText。
3. 缓存 TTL 保持短；写账号后精确失效该 key。

**预期收益**

渠道搜索可扩展。

**验证方法**

1k 账号/站 fixture 下 search-index 体积与耗时预算。

### BE-I（P3）性能测试未覆盖真实高成本 HTTP 路径

**问题描述**

`perf_large_dataset_test` 偏 COUNT/analytics；未系统覆盖 accounts page、action-center、usage overview、notifications page 的 JSON + handler。

**推荐操作步骤**

为上述 handler 加 benchmark 或 timeout 断言；CI 可只在 nightly 跑。

**预期收益**

扩展性回归可被机器发现。

---

## 3. 整体架构建议

### 3.1 当前架构评价

| 维度 | 评价 |
|---|---|
| 形态 | 单二进制桌面控制面：Go 嵌入 `frontend/dist`，loopback HTTP，SQLite 本地状态 |
| 边界 | 模块化 service（accounts/channels/sites/backup/notifications）方向正确，core 仍厚 |
| 安全模型 | 默认可信本机单用户 + 可选 token；适合个人运维工具，不适合裸奔局域网多租户 |
| 数据面 | 读缓存 + 写后失效；列表已开始分页化，分析/用量仍偏“一次聚合” |
| 前端 | 无路由库的 tab shell + keep-alive；领域面板 lazy，状态以 hooks 为中心 |

**总体：** 对“本机运维桌面端”目标架构匹配；下一阶段重点是**扩展性（分页/聚合）与 core 瘦身**，而不是上微服务。

### AR-A（P1）API 契约分层：List / Page / Summary / Index

**问题描述**

账号与通知已出现 page/summary；channels、checkin logs、audit、balances 仍偏“数组即真理”。

**推荐操作步骤**

1. 立约：所有可能大于 100 的集合必须提供 `page` 或 cursor，并返回 `total`。
2. 启动路径只允许 summary/status/health。
3. 用集成测试锁定“首屏禁止全量 accounts/notifications”。
4. 文档 `PROJECT_STRUCTURE` + 简短 API 表。

**预期收益**

规模增长时启动时间近似常数。

### AR-B（P2）读模型（projection）与写模型分离

**问题描述**

Action Center、usage、search-index、dashboard summary 均在读时重算。部分已有 schedule projection 思路。

**推荐操作步骤**

1. 对高频摘要维护 projection 表（如 account_problem_bit、last_checkin_message）。
2. 写路径更新 projection；读路径变 O(1) 或轻量。
3. 先选 problem totals + last checkin message（去掉相关子查询）。

**预期收益**

大库下 p95 稳定；SQL 简化。

### AR-C（P2）前端领域状态容器收敛

**问题描述**

inventory / ops / channel local state / site scoped 多源；刷新扇出靠人工约定。

**推荐操作步骤**

1. 定义资源键与 owner 表（可放 `docs/adr`）。
2. 写操作只 `invalidate(keys)` + owner refresh。
3. 避免 panel `refreshAll` 调全局 reload。

**预期收益**

减少双源真相；分页演进更容易。

### AR-D（P3）不要过早拆进程/上远端 DB

**问题描述**

单机 SQLite + embed UI 是产品优势。

**建议**

在明确多机共享/远程访问需求前，**不做**前后端分离部署或 Postgres 迁移；优先 loopback 加固与备份/导出。

---

## 4. 配置 / 构建 / 部署优化

### CF-A（P1）本机 Go 1.26.4 vs release/CI 要求 1.26.5

**问题描述**

`.go-version` = **1.26.5**；`verify-release.ps1` 硬性要求 ≥1.26.5（GO-2026-5856）。本机 `go version` = **1.26.4**。CI `GOTOOLCHAIN: local` + `go-version-file: .go-version`，runner 需能提供 1.26.5。

**影响评估**

本机无法完整跑 release 验证；安全补丁版本不一致。

**推荐操作步骤**

1. 开发机升级 Go **1.26.5+**。
2. CI 确认 windows-latest 能解析 `.go-version`；必要时 `GOTOOLCHAIN=auto`。
3. release note 记录构建 toolchain。
4. 可选：`go env GOTOOLCHAIN=local` 文档化。

**预期收益**

发布门禁可本地复现；漏洞修复版本对齐。

**验证方法**

```powershell
go version   # ≥ 1.26.5
pwsh scripts/verify-release.ps1 -SkipBrowserSmoke
```

### CF-B（P1）CI Format 将被 3 个脏 Prettier 文件阻断

**问题描述**

见 FE-A。`ad227ad` 推送后 frontend job 的 Format 步骤预期失败。

**推荐操作步骤**

同 FE-A；优先 hotfix 合并。

**预期收益**

main CI 转绿。

### CF-C（P2）缺少包体 gzip 预算门禁

**问题描述**

构建只打印体积；主 JS 70.76 kB gzip、accounts panel 35.18 kB gzip 无自动失败阈值。

**推荐操作步骤**

1. 增加无依赖 Node 脚本扫描 `frontend/dist/assets` gzip。
2. 初始预算：主 JS **80 kB**、主 CSS **40 kB**、任一 `panel-*` **45 kB**。
3. CI build 后跑脚本；上传 manifest artifact。

**预期收益**

依赖/错误 import 导致的包体回归可阻断。

**验证方法**

故意超预算 fixture → CI 红；恢复后绿。

### CF-D（P2）覆盖率阈值过低 + 无包级趋势

**问题描述**

前端 thresholds 约等于当前水位；后端仅 core 55%。

**推荐操作步骤**

1. 前端分阶段 +5%~10%。
2. 后端对 `internal/accounts` 等关键包设 floor。
3. PR 评论贴 coverage diff（可选）。

### CF-E（P2）`verify-release` / package 路径与代理环境

**问题描述**

历史问题：HTTPS GitHub + 死代理 7897；已用 SSH 别名绕过。govulncheck 需网络。

**推荐操作步骤**

1. 文档固定：`git push ssh://git@github.com-obsidian/xvyimu/relaycheck-desktop.git main`。
2. `verify-release -SkipGoVulnCheck` 仅离线应急，正式 release 必须扫描。
3. 代理白名单或 `NO_PROXY` 说明写入 OPERATOR_RUNBOOK。

### CF-F（P3）Node/引擎钉扎已做，建议锁 packageManager

**问题描述**

已有 `frontend/.node-version` 22 与 engines。可补 `packageManager` 字段或 corepack 说明，避免 npm 主版本漂移。

---

## 5. 建议落地顺序（可执行路线图）

### S0 · 立即（小时级）

1. Prettier 修复 3 文件并推送 → 救 CI Format。
2. 开发机升级 Go 1.26.5。

### S1 · 本迭代（P1）

1. FE 覆盖率：accounts page / channel actions / dialog-shell 补测，阈值 → 50%。
2. Action Center COUNT 合并 + usage overview 查询改写。
3. 错误公共文案映射（accounts/backup 优先）。
4. 包体 gzip 预算脚本进 CI。

### S2 · 下一迭代（P2）

1. 大组件拆分 AccountInsights/AccountCard。
2. 站点主从 cursor 分页。
3. SSRF DialContext 钉 IP。
4. token Windows ACL + runbook。
5. search-index 改服务端搜索或限长。
6. core 再拆 pricing/scheduler。

### S3 · 持续（P3）

1. 收紧 CSP style-src。
2. 启动请求瀑布优化。
3. nightly 性能套件。

---

## 6. 回归验证清单（每完成一包必跑）

```powershell
cd E:\zidqiandao\relaycheck-desktop
go test -mod=vendor -count=1 ./internal/...
go test -mod=vendor -cover -count=1 ./internal/core/   # ≥ 55%

cd frontend
npm run format:check
npm run lint
npx tsc -b
npm test
npm run test:coverage
npm run build
npm audit --audit-level=moderate
```

发布前额外：

```powershell
# 需 Go ≥ 1.26.5
pwsh scripts/verify-release.ps1
pwsh scripts/package-release.ps1
pwsh scripts/verify-package.ps1 -ZipPath <zip>
```

---

## 7. 与上一轮审查的关系

| 上轮 ID | 状态（相对 `ad227ad`） |
|---|---|
| FE-1 Prettier 大面积失败 | **大部分已修**；残留 FE-A 3 文件 |
| FE-2 coverage 脚本不可用 | **已关闭**（provider + thresholds 在） |
| FE-3 ESLint 过弱 | **已关闭**（floating/misused promises 等） |
| FE-4 账号全量加载 | **已关闭**（`5147d2b`） |
| BE-1 通知 limit=10 | **已关闭** |
| BE-2 账号分页 | **已关闭** |
| BE-3 错误直出 | **部分关闭**（500 脱敏）；见 BE-A |
| BE-4 SSRF rebinding | **仍开** → BE-E |
| BE-5 token/ACL | **仍开** → BE-F |
| 包体预算 | **仍开** → CF-C |

---

## 8. 总结

RelayCheck Desktop 在**本机单用户控制面**定位下结构清晰、门禁齐全、近期分页化改造方向正确。优先做三件事即可显著提升可发布性与可扩展性：

1. **救 CI：** Prettier 3 文件 + Go 1.26.5 对齐。
2. **护回归：** 抬前端覆盖率与包体预算，拆 accounts 大组件。
3. **稳读模型：** Action Center / usage / search-index 查询与投影化，避免下一个“静默 1000 条”热点。

本报告建议均可在现有单仓单二进制架构内落地，**无需**引入微服务或远端数据库。

---

*报告路径：`docs/full-stack-code-review-optimization-2026-07-16.md` · 基线 `ad227ad`*
