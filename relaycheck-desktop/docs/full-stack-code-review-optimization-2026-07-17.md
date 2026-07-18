# RelayCheck Desktop 全栈代码审查与优化报告（2026-07-17）

- **审查日期：** 2026-07-17
- **仓库：** `E:\zidqiandao\relaycheck-desktop`

## 2026-07-18 增量收口：Settings / SiteSchedules typed API

Settings 与站点排程的 system/scheduler/sites 请求已收敛到 `frontend/src/api/system.ts`、`scheduler.ts`、`sites.ts`。只读加载（settings/backups/scheduler-status/audit-log/exports/version-check/port-check）与写路径（backup/restore/delete/settings PUT/proxy-test/export/import、channel-schedules PUT）均由 adapter 持有声明字段；UI 仍保留恢复/删除/导入确认框语义。契约测试覆盖 GET/POST body；Settings API ownership 测试锁定 confirm 取消零写请求。前端门禁：63 文件 / 387 项；覆盖率 66.99 / 58.80 / 59.73 / 68.18。未执行真实备份恢复、DB 迁移、commit/push/deploy。

## 2026-07-18 增量收口：B1–B3 剩余低风险前端纵切

按批准计划一次落地三批：

1. **channels 写路径**：`channelsApi.detect/restoreSourceStatus/archiveSourceStatus/bulkSourceStatus`；`ChannelTable` 与 `useChannelActions` 去掉裸 `api`；ChannelTable 行为测试覆盖识别/禁用/失败文案/源状态/加载更多。
2. **local-newapi 全表面**：`listInstances/excludeRules/autoDetectImport/sync`；`ScanPanel` 与 `LocalNewAPISyncPanel` 只消费 adapter；sync 默认不带令牌，draft 非空才附带。
3. **notifications 写路径**：`notificationsApi.markAllRead/clearRead/trim`；取消 confirm 零请求；收纳已读固定 `keep=10`。

前端门禁：59 文件 / 378 项；覆盖率 66.66 / 58.76 / 58.58 / 67.83；ChannelTable 行覆盖约 87.5%，NotificationsPanel 约 87.9%，ScanPanel 约 90.5%。format/lint/tsc/build/budget/diff-check 通过。未触碰数据库、未提交/推送/部署；Settings 备份恢复与真实上游仍未做。

## 2026-07-18 增量收口：ChannelsPanel 行为测试 + health path 收敛

`ChannelsPanel` 导出可测纯函数：`refreshChannelPanelData`、`syncChannelModelsAndHealth`、`shouldAutoloadChannelModels`、`healthToneClass`、`topHealthRisks`。健康概览路径收敛到 `channelsApi.healthOverviewPath`，面板不再手写 `/api/channels/health/overview`。新增纯函数与组件行为测试：三路刷新部分失败、sync 先于 health、inventory 注入不 dual-fetch、风险站点展示、探测 task 参数、刷新按钮并行三路。前端门禁：55 文件 / 357 项；覆盖率 63.70 / 55.76 / 54.22 / 64.89；ChannelsPanel 行覆盖约 74.6%。未触碰数据库、未提交/推送/部署。

## 2026-07-18 增量收口：OnboardingWizard typed API + 文案 schema 修正

`OnboardingWizard` 的 Admin 导入与渠道模型同步已收敛到 `frontend/src/api/local-newapi.ts`、`frontend/src/api/channels.ts`。顺带修复引导第 2 步误读 `total/synced/failed` 的问题，改为真实后端 schema：`channelCount/syncedChannels/failedCount`。`useChannelActions` 同步改用 `channelsApi`，避免同一端点双份 URL。行为测试覆盖空表单校验、导入成功文案、syncModels limit=10、第 4 步只导航 `checkinPreview`、API 失败 error。前端门禁：54 文件 / 343 项；覆盖率 61.86 / 54.35 / 51.69 / 63.10；OnboardingWizard 行覆盖约 67%。未触碰数据库、未提交/推送/部署。

## 2026-07-18 增量收口：AccountInsights models/keys typed API

`AccountInsights` 中模型概览、模型同步、价格概览/同步与 Key 脱敏导出预览的裸 URL 已收敛到 `frontend/src/api/models.ts` 与 `frontend/src/api/keys.ts`。组件只消费稳定 schema：`modelsApi.overview/pricing/sync/syncPricing`、`keysApi.exportPreview`。同步默认 `limit=50` 由 adapter 持有，组件不再拼接 `/api/models/*` 或 `/api/keys/*` 字符串。新增契约测试与 AccountInsights 行为测试：展开后加载、同步、导出预览、清理 409 清空已消费预览。前端门禁：51 文件 / 332 项；覆盖率 59.15 / 52.18 / 49.76 / 60.15；format/lint/tsc/build/budget/diff-check 通过。未触碰数据库、未提交/推送/部署。

## 2026-07-18 增量收口：账号清理候选同源

后续安全复核发现一个原报告未列出的高风险竞态：不支持签到账号的 dry-run 与真实删除会分别重新查询候选，可能出现“预览 A、删除 B”；空 body 也会因 `dryRun` 零值进入删除。当前已改为 5 分钟、一次性 `previewId`，确认只消费服务端冻结候选并在事务内复核；状态漂移、过期、重放均 409 且零删除，容量耗尽为 429。

前端新增 typed `account-cleanup` API owner 和受控清理面板，`AccountInsights` 不再持有裸 URL/`dryRun` 契约。数据库恢复重绑会清空旧数据集 preview；任意确认失败都会要求重新预览。新增 Go/前端测试及 Playwright 取消、确认、409/500 流程。最终门禁为 Go 13 包 1148 项、前端 48 文件 324 项，覆盖率 56.22 / 48.41 / 44.53 / 57.44。未执行数据库迁移、真实数据删除、提交、推送或部署。
- **提交基线：** `1b95b0d`，并包含当前未提交的 P1 覆盖率与错误契约改动
- **技术栈：** Go 1.25 module / release toolchain Go 1.26.5 · `net/http` · SQLite（modernc）· React 19 · TypeScript 5.9 · Vite 8 · Vitest 4
- **审查方法：** 源码与测试审读、API 路由交叉检查、SQL/索引检查、安全边界检查、构建与 CI 配置审查、静态搜索、现有门禁与包体结果复核
- **总体结论：** 当前架构适合“本机单用户运维控制台”，工程门禁和模块边界总体健康；未发现 SQL 注入、前端直接 XSS sink、明文密钥入库或已知 Go/npm 依赖漏洞。审查发现的 P1–P3 实施项已落地，并通过完整 release verifier 与精确工具链 package verifier；剩余项仅为需要部署环境的真实性能度量。

---

## 最终复核（当前工作树，2026-07-17 18:32）

> 本节是对报告初稿的二次实测校正，优先级高于下方早期基线。正文保留问题发现时的证据与演进记录；已修复项以路线图勾选状态为准，当前是否可合并以本节门禁结果为准。

### 当前结论

当前工作树的历史 P1 安全问题（账号 action 路由漂移、SSRF 拨号 pin、未认证 health 原始错误、浏览器 profile 路径、导入错误分类和多条业务 raw error）已有代码与回归测试保护。后续发现的 Hook lint、覆盖率、Go toolchain 和导航 E2E fixture 漂移也已修复。项目自带 `scripts/verify-release.ps1` 已在 Go 1.26.5 下完整通过，当前没有已知的合并/发布阻塞项。

### 当前实测门禁

| 检查项 | 当前结果 | 可执行结论 |
|---|---:|---|
| `go test -mod=vendor -count=1 ./ ./internal/...` | **通过，1085 tests / 12 packages** | Go 回归门禁通过 |
| `go test -mod=vendor -cover -count=1 ./internal/core/` | **56.3%** | 高于 CI 55% floor |
| `go vet -mod=vendor ./ ./internal/...` | **通过** | 无 vet 问题 |
| `npm run format:check` | **通过** | 格式门禁通过 |
| `npm run lint` | **通过** | Hook 测试夹具符合 Rules of Hooks |
| `npm run test:coverage` | **通过，44 files / 309 tests** | 54.99 / 46.36 / 42.28 / 56.11，高于 53/45/40/54 floor |
| `npm run build` | **通过** | Vite 生产构建成功 |
| `npm run budget:check` | **通过** | 主 JS 67.27 kB gzip、accounts panel 35.61 kB gzip、初始 CSS 32.93 kB gzip，均在预算内 |
| `npm audit --audit-level=moderate` | **通过，0 vulnerabilities** | npm 已知漏洞门禁通过 |
| `govulncheck@v1.5.0` | **通过，0 vulnerabilities** | `x/sys` 已升级至 v0.44.0，`GO-2026-5024` 已消除 |
| Go release toolchain | **Go 1.26.5** | 通过官方按需 toolchain 与 `.go-version` 对齐 |
| Release binary / embedded UI | **通过** | 健康检查 200、嵌入资源和 fresh DB API shape 正常 |
| Browser smoke | **通过** | 调度布局桌面/390px、NavigationIntent 9/9、layout 1440/900/390px 全部通过且无横向溢出 |
| 完整 release verifier | **通过** | `-BrowserPort 5174`，未跳过 govulncheck 或任一浏览器 smoke |
| Package verifier | **通过** | Go 1.26.5 / Node 22.23.1 / npm 10.9.8；7 个源输入、内部文件与 zip SHA256 全部匹配 |

### 新增或校正后的操作项

#### FE-H（P1 · 门禁正确性）Hook 单测夹具违反 Rules of Hooks

**状态：已关闭。** 使用组件命名的 Hook harness 修复规则违规；lint 与定时行为测试均通过。

- **问题描述：** `useDebouncedValue.test.ts` 通过 mock React 并在普通异步函数中直接调用 Hook；ESLint 正确拒绝该模式。该测试也没有由真实 React 生命周期驱动 effect cleanup，存在“测试通过但行为模型失真”的风险。
- **影响评估：** CI lint 必然失败；定时器清理、重渲染与卸载行为没有得到可信验证。
- **推荐操作步骤：** 使用真实 Hook 渲染夹具（优先 `renderHook`，若不新增依赖则用现有 React DOM 测试基础设施构造最小 Harness）；断言初值、250ms 更新、连续输入只发布最后值、IME disabled、卸载清 timer。不要添加 `eslint-disable`，也不要仅靠函数改名绕过规则。
- **预期收益：** 恢复 lint 门禁，并让 debounce 回归测试与真实 React 调度一致。
- **验证方法：** `npm run lint` 与 `npm test -- --run useDebouncedValue` 同时通过；测试结束后 `vi.getTimerCount()` 为 0。

#### FE-I（P1 · 覆盖率门禁）新增搜索/筛选代码未配套行为测试

**状态：已关闭。** 已补 `useAccountSiteSearch`、`useChannelFilters` 与提取后账号逻辑的行为测试，当前覆盖率为 54.99 / 46.36 / 42.28 / 56.11，floor 为 53/45/40/54。

- **问题描述：** 当前覆盖率中 `useAccountSiteSearch.ts` 仅约 2.56% statements，`useChannelFilters.ts` 为 0%；全局四项指标均低于已配置阈值。测试总数增加并不等于门禁有效。
- **影响评估：** CI coverage 必然失败；debounce、abort、truncated 提示、筛选复位和站点匹配仍可无声回归。
- **推荐操作步骤：** 优先补这两个 Hook 的行为测试，再补 ChannelsPanel 请求次数与 partial failure；不要降低 50/45/35/50 阈值。覆盖正常、空查询、快速改词取消旧请求、服务端失败、`truncated=true`、intent/filter reset。
- **预期收益：** 恢复覆盖率门禁，同时保护本轮新增的搜索与资源 owner 逻辑。
- **验证方法：** `npm run test:coverage` 退出码为 0；故意删除 abort 或 truncated 分支时测试失败。

#### BE-K（P2 · 查询效率）账号分页的“最新签到消息”索引未完全匹配排序

**状态：已关闭。** 已增加 `(account_id, started_at DESC, id DESC)` 与 balance snapshot 对应复合索引，并由查询计划测试和 5000 账号 benchmark 保护。

- **问题描述：** `accounts_page.go` 对每页账号执行相关子查询，按 `checkin_logs(account_id, started_at DESC, id DESC)` 取最新消息；当前索引只有 `(account_id, started_at)`，缺少最终排序键 `id`。50 条页面会触发最多 50 次索引探测，时间戳相同时还需额外排序/回表。
- **影响评估：** 日志量增长后账号页 p95 会被相关子查询放大；相同 `started_at` 的稳定性依赖额外排序。
- **推荐操作步骤：** 用 `EXPLAIN QUERY PLAN` 验证后，将索引演进为 `(account_id, started_at DESC, id DESC)`；若账号页仍是热点，评估在账号表维护 `last_checkin_message` projection，或用窗口/CTE 一次 join 当前页最新日志。
- **预期收益：** 降低账号页每行最新日志查询成本，排序结果稳定。
- **验证方法：** 5k accounts / 100k logs fixture 下记录 handler p95；执行计划应使用覆盖排序的复合索引且不出现临时 B-tree。

#### CF-H（P2 · CI 可靠性）现有 workflow 缺少运行约束

**状态：已关闭。** workflow 已配置最小权限、并发取消、job 超时、固定 Action SHA、质量 artifact 与 nightly benchmark。

- **问题描述：** `.github/workflows/ci.yml` 未声明顶层 `permissions`、`concurrency` 或 job `timeout-minutes`，Actions 仍使用可变 major tag。
- **影响评估：** 默认权限可能高于所需；连续 push 浪费 Windows runner；网络型扫描卡住时缺少上限；供应链版本不可完全复现。
- **推荐操作步骤：** 设置 `permissions: contents: read`；按 workflow/ref 配置 `concurrency` 和 `cancel-in-progress`；为 Go/Frontend job 设置 20/15 分钟超时；将官方 Action pin 到完整 commit SHA，并由 Dependabot 更新。
- **预期收益：** CI 更安全、节省资源、失败边界明确且可复现。
- **验证方法：** 连续推送时旧 run 被取消；权限页面只读；故意挂起步骤会在设定时间退出。

---

## 0. 执行摘要与验证基线

### 0.1 当前验证结果

| 检查项 | 结果 | 说明 |
|---|---:|---|
| Go internal tests | 通过 | `./internal/...` 全部通过 |
| Core coverage | **56.3%** | 高于 CI 55% floor |
| Go vet | 通过 | `go vet -mod=vendor ./ ./internal/...` 无问题 |
| Frontend format/lint/tsc | 通过 | Prettier、ESLint、TypeScript 均绿 |
| Frontend tests | **44 files / 309 tests** | 全部通过 |
| Frontend coverage | **54.99 / 46.36 / 42.28 / 56.11** | statements / branches / functions / lines；通过 53/45/40/54 floor |
| Production build | 通过 | Vite 构建成功 |
| Bundle budget | 通过 | 主 JS 67.27 kB gzip；accounts panel 35.61 kB gzip；初始 CSS 32.93 kB gzip |
| npm audit | 通过 | 0 vulnerabilities（moderate threshold） |
| govulncheck | 通过 | v1.5.0；`x/sys v0.44.0`；0 vulnerabilities |
| Release toolchain | 通过 | Go 1.26.5 按需 toolchain |
| Complete release verifier | 通过 | 包含二进制、嵌入 UI、桌面/移动布局和导航 E2E |

### 0.2 优先级

| 级别 | 定义 | 审查发现数量（含已关闭） |
|---|---|---:|
| P0 | 可远程接管、严重数据损坏、密钥直接泄露 | 0 |
| P1 | 主路径正确性、安全边界、发布阻塞、门禁失真 | 8 |
| P2 | 规模化性能、维护成本、纵深防御 | 12 |
| P3 | 工程成熟度与持续优化 | 4 |

### 0.3 本轮确认已关闭的旧问题

- 启动路径不再请求全量 `GET /api/accounts`；账号主列表已使用 cursor page。
- 通知已提供分页 totals；账号摘要与 search-index 已拆分。
- Action Center COUNT 已批处理，sample 已改为独立端点按需加载。
- usage 已改为每账号最近两条快照的窗口查询，不再受全局 `LIMIT 1000` 正确性影响。
- 旧账号 list 已带 `Deprecation` 和 `Link` 响应头。
- 前端包体预算脚本与 CI gate 已存在并通过。
- 前端 statements/lines 已超过阶段性 50% 目标。

### 0.4 本轮实施结果（未提交）

| 范围 | 状态 | 可验证结果 |
|---|---|---|
| FE-A 路由契约 | 已完成 | AccountDetail 改用 `/test-login`，交互测试断言真实 POST URL |
| FE-B / CF-B 覆盖率 | 已完成 | floor 提至 53/45/40/54；实际 54.99/46.36/42.28/56.11 |
| BE-A 公共错误契约 | 已完成首批高风险路径 | accounts/import、browser/batch/validation、checkin/balance/task/scheduler/proxy 均不再返回 raw error；敏感上游签到消息回退为固定文案 |
| BE-B profile/health 泄露 | 已完成 | health 使用稳定中文错误；browser profile path 不再序列化到 JSON |
| BE-D import 分类 | 已完成 | path/format/upstream auth/upstream unavailable/storage typed errors；SQLite 缺表不会误报路径越界；Admin API 不回显 body |
| BE-E SSRF pin | 已完成 | 初始目标和 redirect 都 pin 已验证 IP；rebinding 测试通过 |
| 查询与 API 性能 | 已完成 | FTS5 搜索、精确复合索引、usage site/limit/truncated、Action Center sample 按需加载、Dashboard 约 10→3 个业务请求 |
| 前端拆分与 CSP | 已完成 | Analytics 与面板按需 chunk；大组件/类型/CSS 分拆；内联 style 39→0；`style-src-attr 'none'` |
| Windows ACL 与可观察性 | 已完成 | token 文件 DACL 写入后复核；失败即删除并禁用 enforcement；request/task/account/site 关联日志与敏感字段裁剪 |
| CI / 发布可追溯性 | 已完成 | Action SHA、最小权限、并发/超时、nightly 5k benchmark、质量 artifact、Node/npm 精确声明与 source SHA256 manifest |
| 全量门禁 | 已完成 | Go vet；1085 tests；core 56.3%；frontend 309；format/lint/tsc/coverage/build/budget/audit 全绿；release verifier 完整通过 |
| 依赖与发布包 | 已完成 | `x/sys v0.44.0` 后 govulncheck 0 漏洞；精确 Go/Node/npm 工具链打包与 manifest/checksum/zip verifier 全绿 |
| Browser E2E | 已完成 | 调度 2 视口、NavigationIntent 9/9、layout 3 视口全部通过；验证端口支持参数化 |

当前唯一无法在仓库内闭环的验收项：尚无代表性 RUM/线上 p95 数据可证明“首屏 <2 秒、API 延迟降低 50%”。开发机默认 Node/npm 虽与声明不同，但已用隔离的精确 Node 22.23.1/npm 10.9.8 完成真实打包，且打包脚本会对漂移版本 fail closed。

---

## 1. 前端代码审查

### 1.1 已验证优点

- 面板使用 `React.lazy` 与手工 chunk 分组，当前所有 chunk 均低于预算。
- `useApi`、`useAccountsPage`、`useSiteAccounts` 使用 AbortController，能抑制过期响应覆盖新状态。
- 未发现 `dangerouslySetInnerHTML`、`eval`、`new Function` 等直接执行 sink。
- 外链有专用安全归一化工具；本地存储只保存非密钥偏好。
- keep-alive 面板使用 `inert` / `aria-hidden`，并有 idle eviction。
- DialogShell 已有焦点陷阱、Escape、滚动锁与恢复焦点测试。
- 分页、渠道搜索索引与站点账号路径已有“不请求全量 `/api/accounts`”回归断言。

### FE-A（P1 · 正确性）账号详情调用不存在的登录态测试端点

**状态：已关闭。** 已集中账号 API URL builder，并以点击行为测试锁定 `/test-login` 契约。

**问题描述**

`frontend/src/components/accounts/AccountDetailContent.tsx:85` 请求：

```text
/api/accounts/{id}/test-login-status
```

后端 `internal/core/accounts.go:246` 只识别：

```text
/api/accounts/{id}/test-login
```

同一功能在 `AccountCard.tsx` 使用的是正确端点。`AccountDetailContent` 被 AccountsPanel 与 SiteAccountMasterDetail 实际使用，因此不是死代码。

**影响评估**

- 账号详情抽屉中的“测试登录态”必然落入未知子路由或错误 ID，主操作失败。
- 同一业务在两处表现不一致，用户会误判账号或后端故障。
- 当前 288 个测试未发现该问题，说明交互测试仍存在盲区。

**推荐操作步骤**

1. 将 AccountDetailContent 改为 `/test-login`。
2. 抽取 `accountEndpoint(id, action)` 或账号 API client，禁止组件手写路由字符串。
3. 新增交互测试：点击“测试登录态”，断言 POST URL、请求体和成功/失败文案。
4. 增加前后端路由契约测试，至少覆盖所有账号 action suffix。

**预期收益**

- 恢复账号详情核心操作。
- 避免同类端点漂移再次出现。

**验证方法**

```powershell
cd frontend
npm test -- --run AccountDetailContent
```

并通过浏览器 smoke 确认点击后后端返回 200，而不是 404/405。

### FE-B（P1 · 测试质量）覆盖率提升未完全转化为行为保护

**状态：已关闭阶段性目标。** coverage floor 和关键 Hook/路由行为测试已补齐；低覆盖大型组件仍列入后续逐步提升项。

**问题描述**

当前 coverage 已达到 51.87% statements，但部分新增测试只直接调用函数组件并断言 `toBeTruthy()`：

- `AccountDetailContent.test.ts`
- `settings-split.test.ts`

这些测试执行了渲染分支，却没有触发 click/change/async handler；因此 FE-A 的错误端点仍未被发现。`useAccountsPage` 目前也只覆盖首次请求和 disabled，未覆盖 cursor 栈、filter reset、abort/stale response。

同时 `frontend/vite.config.ts` 的阈值仍是 40/35/30/40，允许 coverage 从当前水平大幅回退。

**影响评估**

- 指标看似提升，但关键写操作、异步状态机和请求契约仍可能回归。
- CI 无法守住本轮已经达到的 50% 水位。

**推荐操作步骤**

1. 优先补行为测试，而不是继续用浅渲染扩行数。
2. `useAccountsPage` 增加 next/prev、filter reset、旧请求 abort 测试。
3. AccountDetail、Settings 导入导出、代理测试改为事件驱动断言。
4. 阈值立即提高到 statements/lines 50、branches 45、functions 35。
5. 为 `useAccountsPage`、`useSiteAccounts`、`api/client.ts` 设置文件级 floor。

**预期收益**

- 覆盖率与真实回归保护一致。
- 下一轮拆 AccountInsights/AccountCard 时有可信护栏。

**验证方法**

- 故意把 `/test-login` 改错，测试必须失败。
- 故意移除 abort，cursor/stale 测试必须失败。
- 将全局 coverage 降至 49%，CI 必须失败。

### FE-C（P1 · 性能）账号搜索每次按键触发请求，后端同时执行三类 COUNT

**状态：已关闭。** 已实现 250ms/IME-aware debounce、服务端 FTS5、summary/page 解耦和 cursor 复合索引。

**问题描述**

`AccountsPanel.tsx:120` 直接把输入值写入 query；`useAccountsPage` 随依赖变化立即请求。服务端 `accounts_page.go` 每次请求执行：

1. 全账号 COUNT；
2. 全局问题账号 COUNT；
3. 当前筛选 COUNT；
4. 当前页 SELECT。

文本搜索使用 `LOWER(concat(...)) LIKE '%query%'`，无法利用普通 B-tree 索引。

**影响评估**

- 连续输入会产生多次已发往服务端的 SQLite 查询；Abort 只能丢弃客户端结果，不能保证服务端已停止计算。
- 账号数量增长后，搜索响应和 SQLite CPU 使用会明显抖动。

**推荐操作步骤**

1. 前端增加 200–300ms debounce，并在 IME composing 时不发请求。
2. global account/problem totals 只由 `/api/accounts/summary` 提供，不在每页重复计算。
3. page 只计算 filtered total；或在无 UI 需求时取消 total，使用 `limit+1` 判断下一页。
4. 为 `(updated_at DESC, id DESC)` 增加复合索引。
5. 账号规模达到数万时，使用 FTS5 或维护 normalized search column。

**预期收益**

- 输入请求数下降 70% 以上（取决于打字速度）。
- page 冷查询更接近一次 COUNT + 一次 SELECT。

**验证方法**

- Playwright 输入 10 个字符，网络请求应不超过 1–2 次。
- 5k/20k 账号 fixture 下记录 p50/p95 与 `EXPLAIN QUERY PLAN`。

### FE-D（P2 · 数据所有权）渠道刷新仍存在重复请求与串行等待

**状态：已关闭。** channel hooks 已明确 owner、并行刷新并增加请求行为测试。

**问题描述**

`ChannelsPanel.refreshAll()` 依次执行：

1. `refreshActions()`：channels + models + account search-index；
2. `refreshHealth()`；
3. 父级 `onRefresh()`：system + inventory + ops + model usage，其中 inventory 再请求 channels。

当前 client 1.5s read cache 可能掩盖一部分重复，但资源 owner 仍不清晰。

**影响评估**

- 手动刷新可能形成重复 channels 请求和宽范围刷新。
- 串行 await 增加总等待时间；写后全量 cache clear 会放大问题。
- 后续分页化 channels 时容易出现双源状态漂移。

**推荐操作步骤**

1. 明确 owner：inventory 管 channels/sites/summary；channel hook 只管 models/health/search-index。
2. 写操作返回变更对象或 invalidation keys，不默认刷新整个应用。
3. 必须共同刷新时使用 `Promise.all`，并对单项失败使用可见的 partial error。
4. 增加 ChannelsPanel 级请求次数测试，而不仅测试 `useChannelActions`。

**预期收益**

- 降低请求扇出和刷新延迟。
- 资源状态归属清晰，便于后续分页与缓存演进。

**验证方法**

冷启动、手动刷新、模型同步、健康探测完成四条路径分别记录 endpoint 次数。

### FE-E（P2 · 可维护性）大型组件、类型文件与 CSS 层继续集中

**状态：已完成本轮拆分目标。** 已拆 AccountCardEditor、ActionPriorityItem、accountInsightsUtils、dashboard/navigation types 与按需 CSS；更深的命令 Hook 拆分不再作为本轮阻塞项。

**问题描述**

当前主要大文件：

| 文件 | 行数（约） |
|---|---:|
| `AccountInsights.tsx` | 933 |
| `SettingsCards.tsx` | 657 |
| `AccountCard.tsx` | 642 |
| `AnalyticsPanel.tsx` | 640 |
| `SitesPanel.tsx` | 550 |
| `types/index.ts` | 977 |
| `accounts.css` | 1193 |
| `control-room.css` | 1505 |

AccountInsights/AccountCard 混合请求、状态机、确认流、格式化与展示；types 单文件会成为跨域改动冲突点。

**影响评估**

- 修改半径宽，评审难度高。
- 组件测试需要大量 mock，促使团队写浅渲染测试。
- CSS 层叠与重复选择器难以追踪。

**推荐操作步骤**

1. 按业务命令拆 `useAccountLoginActions`、`useApiKeyActions`、`useAccountCleanupActions`。
2. AccountInsights 保留组合与展示，异步命令进入 hooks/command 模块。
3. `types/index.ts` 拆为 accounts/channels/system/tasks 等域文件，由 index 只 re-export。
4. CSS 按 domain/component 收敛；先用重复选择器检查，避免大规模纯格式改写。
5. 每次控制在 100–300 行 diff，持续守住 panel-accounts 45 kB gzip。

**预期收益**

- 缩小改动和回归范围。
- 行为测试可针对命令模块，不再依赖重组件 mock。

**验证方法**

每个拆分包独立测试，构建后 accounts panel gzip 不增长；无循环 import。

### FE-F（P2 · 首屏性能）Dashboard 初始请求瀑布仍偏宽

**状态：已关闭代码侧目标。** Dashboard 业务读请求约从 10 个收敛为 3 个聚合端点，Analytics 和非首屏面板按需加载；真实首屏时延仍需 RUM 验证。

**问题描述**

初始阶段同时加载：

- system status；
- inventory 3 项；
- ops 4 项（checkins、notifications、diagnostics、action-center）；
- system loaded 后再拉 model、pricing、usage 3 项。

当前 loading 判定等待 system + inventory + ops，意味着 diagnostics/action-center 等较重读模型会阻塞首屏。

**影响评估**

- SQLite 冷缓存或弱机器上启动时间由最慢请求决定。
- 多个摘要端点同时争用 4 个 DB connection。

**推荐操作步骤**

1. 首屏只等待 system + inventory summary + 最小 Dashboard summary。
2. diagnostics、完整 notification page、model/pricing/usage 在 idle 或对应卡片可见时加载。
3. Action Center 首屏返回 counts/priority，samples 展开时再加载。
4. 记录冷启动 waterfall 和 Time-to-Useful-Dashboard。

**预期收益**

- 首屏更快、更稳定。
- 冷启动数据库竞争降低。

**验证方法**

使用 Playwright trace 或浏览器 Performance 面板记录优化前后请求数、最慢请求和首屏可交互时间。

### FE-G（P3 · 安全纵深）内联样式阻塞 CSP 收紧

**状态：已关闭。** React inline style 由 39 处降为 0，CSP 使用 `style-src 'self'` 与 `style-src-attr 'none'`。

**问题描述**

主 tab 显隐、图表、进度条、Settings、Scan 等仍有数十处 `style={{...}}`。后端 CSP 因此保留 `style-src 'self' 'unsafe-inline'`。

**影响评估**

当前没有直接 XSS sink，但 CSP 的样式注入防线无法完全启用。

**推荐操作步骤**

1. 静态布局改 class；动态数字改受控 CSS custom property。
2. tab 显隐改 `.is-hidden` 或 `hidden` 属性。
3. 迁移后去掉 `unsafe-inline`，加入 CSP 响应头测试。

**预期收益**

增强 XSS 纵深防御并减少 JSX 样式噪声。

**验证方法**

生产构建浏览器控制台无 CSP violation；主题、图表、进度和抽屉显示正常。

---

## 2. 后端代码审查

### 2.1 已验证优点

- SQLite 使用 WAL、busy timeout、foreign keys、有限连接池；关键列表已有组合索引。
- SQL 参数整体使用占位符；动态 SQL 片段来自受控字段集合，未发现用户输入直接拼接。
- rows iteration 基本都有 `rows.Err()` 检查。
- API 有统一 JSON envelope 和 `errorClass`。
- 所有业务路由使用 `requireSession`；写请求有 Origin 和 loopback RemoteAddr 防护。
- Host allowlist、CSP、nosniff、frame deny 已启用。
- JSON body 有 8 MiB 上限；列表 limit 有 clamp。
- 账号、通知已开始采用 page/summary/index 读模型。
- Action Center COUNT 批处理和 usage window 查询已修复上一轮明显扩展性问题。

### BE-A（P1 · 安全）SSRF 初始请求没有使用已校验 IP 拨号

**状态：已关闭。** 初始请求和 redirect 均绑定已校验 IP，包含 rebinding 与 allowLocal 回归测试。

**问题描述**

`internal/core/network.go:147` 调用 `resolveOutboundHTTPURL`，但只检查 error，丢弃了返回的 IP 集合。`newNetworkHTTPClient` 创建的 `pinned` map 初始为空；只有 redirect hook 才写入，而且仅在 `!policy.AllowLocal` 时写入。

因此：

- 初始请求仍由默认 resolver 在拨号时再次解析，存在 DNS rebinding 时间窗。
- 当应用开启 allowLocal 以支持本机 NewAPI 时，远端域名的 redirect 也不会 pin，因为判断跳过的是整个 policy，而不是只跳过 loopback 目标。

**影响评估**

- 恶意域名可在校验和拨号之间切换到 `127.0.0.1`、私网或 metadata IP。
- allowLocal 是正常本机 NewAPI 场景，不能把它等价为“所有远端目标都不需要 pin”。

**推荐操作步骤**

1. `doHTTPWithTimeout` 保留 `resolvedOutboundURL`，把初始 host→IPs 注入 transport。
2. DialContext 对所有已解析主机只连接允许集合；loopback 被明确允许时同样可以安全 pin 到 loopback。
3. redirect 每跳替换该 host 的允许 IP 集合。
4. 对使用 HTTP proxy 的路径明确安全模型：代理是受信任边界，或安全敏感请求绕过代理。
5. resolver/dialer 可注入，增加 rebinding 单测。

**预期收益**

SSRF 校验约束真实 TCP 连接目标，而不只约束第一次 DNS 查询。

**验证方法**

- resolver 第一次返回公网 IP、拨号前返回 `127.0.0.1`，必须拒绝且内网测试 server 收不到连接。
- allowLocal=true 时 localhost 成功，远端恶意域名仍只能连接首次允许 IP。

### BE-B（P1 · 信息泄露）未认证 health API 返回 SQLite 原始错误

**状态：已关闭。** 公共 health 只返回稳定错误，原始原因留在受控日志路径。

**问题描述**

`/api/health` 不经过 session token。`healthCheckDB` 在 Ping/SELECT 失败时直接返回 `err.Error()`。SQLite 错误可能包含文件路径、驱动文本或数据库状态信息。

**影响评估**

- 与“API 不返回绝对路径和 OS/SQLite 原始错误”的项目约束冲突。
- 即使当前只绑定 loopback，同机低权限进程也可以读取这些诊断。

**推荐操作步骤**

1. health 响应固定为“数据库连接失败”或“数据库查询失败”。
2. 服务端日志只记录 request ID、错误类型与经过审查的摘要。
3. 详细诊断放到受保护的 `/api/system/diagnostics`，同样禁止绝对路径。
4. 添加包含绝对路径/SQL/token sentinel 的 health 回归测试。

**预期收益**

公共健康检查保持可监控，同时不暴露内部实现信息。

**验证方法**

强制 DB 关闭或返回错误，断言 `/api/health` body 不包含 `C:\`、`SELECT`、数据库文件名。

### BE-C（P1 · 信息泄露）浏览器 profile 绝对路径进入账号 API

**状态：已关闭。** profile path 已从公共 JSON 契约移除并有回归扫描。

**问题描述**

`ChannelAccount.BrowserProfilePath` 以 `browserProfilePath` 输出；browser login open result 也返回 `profilePath`。路径由 `dataDir/browser-profiles/{id}` 生成，通常是绝对路径。前端当前没有实际读取该字段。

**影响评估**

- 暴露 Windows 用户名、安装路径和数据目录结构。
- 与项目明确的 API 路径脱敏约束冲突。
- 该字段没有前端价值，属于不必要的数据面。

**推荐操作步骤**

1. API DTO 移除 `browserProfilePath/profilePath`，内部模型继续保留。
2. UI 若需要状态，只返回 `hasBrowserProfile: boolean`。
3. 避免直接复用数据库模型作为 HTTP response；建立 public DTO converter。
4. 添加所有账号/list/page/browser-open 响应的绝对路径扫描测试。

**预期收益**

减少信息泄露面，推动存储模型与公共契约分离。

**验证方法**

对响应 JSON 全文断言不包含 `browser-profiles`、盘符路径和 dataDir。

### BE-D（P1 · 错误契约）200/业务结果中仍大量携带 raw error

**状态：已关闭首批高风险路径。** browser/batch/checkin/proxy/import 等响应已使用稳定公共错误，日志不记录敏感正文。

**问题描述**

`writeError` 只保护 HTTP 5xx。以下业务结果仍把 `err.Error()` 放进 JSON `message`：

- BrowserLoginService：目录创建、Chrome 查找/启动、DevTools 读取、加密和 DB 写入；
- AccountLoginBatchService：密码登录错误；
- AccountValidationService：上游请求错误与模型测速错误；
- checkin/balance/task result；
- proxy test 的 HTTP 200 failure message。

这些消息可能包含绝对路径、上游 response body、网络地址和驱动细节。

**影响评估**

- API Key 专用 sanitizer 只能替换当前 key，不能覆盖所有凭据和路径。
- 批处理结果通常以 200 返回，不会经过 `writeError` 的 5xx 脱敏。
- 错误文案依赖 OS/驱动版本，前端 UX 不稳定。

**推荐操作步骤**

1. 定义业务错误码：`chrome_not_found`、`browser_start_failed`、`upstream_unreachable`、`auth_rejected`、`storage_failed` 等。
2. response 只返回稳定 PublicMsg + code；raw cause 不进入 JSON。
3. 日志记录错误类型、request/account ID，不记录可能含 token 的上游 body。
4. 为 browser/bulk/checkin/proxy result 增加敏感片段回归测试。

**预期收益**

统一错误 UX，避免 200 response 绕过错误脱敏层。

**验证方法**

注入 `open C:\secret...`、`token=TOP_SECRET`、SQL 错误，断言所有 `data.results[].message` 均无敏感片段。

### BE-E（P1 · 正确性）新 PublicMsg 映射过度具体，可能误导用户

**状态：已关闭。** import domain 使用 typed errors 区分路径、格式、上游认证/可用性与存储失败。

**问题描述**

当前未提交改动把 `ImportChannelsFromSQLite` 的所有错误映射为“路径不在允许目录”；但服务可能因文件损坏、SQLite schema 不兼容、查询失败或事务失败而报错。Admin API 所有错误也统一提示地址/token/权限，legacy import 所有错误统一提示 JSON 格式。

**影响评估**

- 安全性改善，但可操作性下降；用户会按错误方向排查。
- 测试只覆盖路径拒绝，未覆盖其他错误类别。

**推荐操作步骤**

1. domain package 定义 typed/sentinel errors：path rejected、invalid format、unsupported schema、auth rejected、upstream unavailable、storage failure。
2. handler 使用 `errors.Is/errors.As` 映射精确 PublicMsg。
3. 未知错误使用中性文案“导入失败，请检查文件并重试”，不猜测原因。
4. 每种 error class 至少一个 handler 契约测试。

**预期收益**

同时满足安全脱敏和用户可操作性。

**验证方法**

路径越界、损坏 DB、缺表、401、超时、DB 写失败 fixture 分别返回正确 class/message。

### BE-F（P2 · 查询效率）账号 page 的统计与搜索仍会全表扫描

**状态：已关闭。** page/summary 已解耦，空查询走 cursor 索引，文本查询走 FTS5，并有大数据查询计划测试。

**问题描述**

账号 page 每次请求计算全局总数、全局问题数、筛选总数；文本搜索是多字段拼接后的前后通配 LIKE。现有 `idx_channel_accounts_updated` 不是 `(updated_at,id)` 复合 cursor 索引。

**影响评估**

- 翻页和搜索成本随账号总量线性增加。
- 页面请求由 counts 而非 page SELECT 主导。

**推荐操作步骤**

1. 全局 totals 走 cached summary；page 不重复计算。
2. 为 cursor 添加 `(updated_at DESC, id DESC)` 索引。
3. query 为空时确保走 cursor index；query 非空时使用 FTS5/搜索投影。
4. 为 5k/20k 账号增加 handler benchmark 与 query plan 快照。

**预期收益**

稳定分页 p95，减少输入搜索时的 SQLite CPU。

**验证方法**

`EXPLAIN QUERY PLAN` 显示 cursor query 走复合索引；20k fixture 冷查询达到团队预算。

### BE-G（P2 · 正确性/扩展性）search-index 4k 截断会产生静默搜索假阴性

**状态：已关闭。** 搜索改为服务端 FTS5 与限定结果契约，不再依赖 4k 聚合字符串。

**问题描述**

`loadAccountSearchIndex` 对每站 `SUBSTR(GROUP_CONCAT(...),1,4000)`，但 response 没有 `truncated` 标志。`GROUP_CONCAT` 也没有明确账号排序。渠道页使用这段文本判断账号名/邮箱是否命中。

**影响评估**

- 大站后部账号无法被搜到，UI 没有任何提示。
- SQLite 执行计划或数据写入顺序变化可能改变“前 4k”包含哪些账号。

**推荐操作步骤**

1. 最佳方案：渠道搜索改为服务端 query endpoint，返回匹配 channel IDs。
2. 过渡方案：response 增加 `truncated` 与稳定排序，并在 UI 提示搜索不完整。
3. 若保留聚合，限制的是账号条数而不是字符串字节，并明确采样策略。

**预期收益**

消除静默假阴性，响应体仍可控。

**验证方法**

单站 1000 账号 fixture，搜索首、中、末账号均有稳定结果；截断时 UI 明示。

### BE-H（P2 · 查询效率）usage 窗口正确但仍遍历整个快照历史

**状态：已关闭阶段性目标。** 已加入 `siteId/limit/truncated`，先限制目标账号再执行窗口查询，并增加精确时间/ID 索引。

**问题描述**

ROW_NUMBER 修复了全局 LIMIT 的正确性，但 SQLite 仍需对所有 `balance_snapshots` 分区排序后筛 `rn <= 2`。已有 `(account_id, created_at)` 索引能帮助，但 endpoint 无 site/account/limit 参数，并返回全部账号与站点汇总。

**影响评估**

- 快照历史持续增长时，缓存 miss 成本持续上升。
- 前端通常只展示有限区域，却获取全量 usage accounts。

**推荐操作步骤**

1. 加 `siteId`、`limit` 或 cursor 参数。
2. 索引调整为 `(account_id, created_at DESC, id DESC)`。
3. 长期维护 `latest_balance` / daily usage projection，写快照时更新。
4. nightly benchmark 覆盖 10万/100万快照。

**预期收益**

usage 成本由全历史规模转为活跃账号规模。

**验证方法**

固定 fixture 手算结果一致；历史快照扩大 10 倍时 p95 不线性增长。

### BE-I（P2 · 冷路径性能）Action Center sample 查询仍为多次往返

**状态：已关闭。** 首屏仅返回 count/priority，sample 通过 `/api/system/action-center/samples?id=...` 展开后加载。

**问题描述**

COUNT 已批处理，但每个 count>0 的风险项仍独立执行 sample query，最坏情况下为十余次 SQLite 往返。

**影响评估**

风险项较多的真实故障场景，恰好是 Dashboard 最需要快速显示的时候。

**推荐操作步骤**

1. 首屏 Action Center 只返回 count/priority。
2. 展开条目或点击后再请求 samples。
3. 或把同类 sample 用 UNION ALL 批量返回，并带 action ID。

**预期收益**

降低冷启动和故障态 Dashboard 延迟。

**验证方法**

制造全部风险项为非零的 fixture，记录 SQL 次数与 p95；首屏目标控制在固定 2–4 次查询。

### BE-J（P2 · 本机安全）session token 默认关闭且 Windows ACL 仅人工检查

**状态：已关闭 hardened 路径。** Windows token 文件自动设置并复核 DACL；复核失败即删除文件且不启用 token enforcement。默认 token 仍为产品信任模型选择，而非 ACL 实现缺口。

**问题描述**

token 为 opt-in，文件使用 `0600` 写入；Windows 上 POSIX mode 不等于 DACL。runbook 仅要求人工 `Get-Acl`/`icacls`。

**影响评估**

- 共享机器上其他同机进程可调用默认无 token 的读 API。
- hardened 模式仍依赖文件 ACL 是否继承过宽。

**推荐操作步骤**

1. 写 token 后自动设置当前用户 + SYSTEM DACL。
2. 启动时检查 ACL，不合格则高亮 warning 或拒绝 hardened mode。
3. 保持 loopback 固定；未来若增加非 loopback bind，必须强制 token。
4. threat model 明确“本机其他进程”是否在信任边界内。

**预期收益**

hardened mode 从文档约定升级为机器可验证的安全边界。

**验证方法**

Windows 集成测试或验收脚本检查 ACL 主体；无 cookie 请求返回 401。

---

## 3. 整体架构建议

### 3.1 当前架构评价

| 维度 | 评价 |
|---|---|
| 产品形态 | Go 单二进制 + embedded React + loopback HTTP + SQLite，非常适合本机运维工具 |
| 后端边界 | accounts/channels/sites/backup/notifications 等 domain service 方向正确；core 仍承担较多 HTTP 与 orchestration |
| 前端边界 | hooks + lazy panels 清晰；账号 API builder、资源 owner、组件/类型/CSS 拆分已推进，少数大型交互组件仍适合继续小步拆分 |
| 数据模型 | 写模型稳定；page/summary/FTS/聚合端点已形成读模型层，长期仍可评估 latest-balance projection |
| 安全模型 | loopback + Host/Origin/RemoteAddr、SSRF IP pin、公共错误脱敏与 hardened token DACL 已形成纵深；默认 token 仍为 opt-in |

**总体建议：不要拆微服务，不要迁远端数据库。** 当前问题都可在单进程架构内通过契约、projection、索引和模块边界解决。

### AR-A（P1）建立单一 API 契约来源

**状态：已完成最小方案。** 已新增 accounts/action-center API builders 与契约测试；生成式 OpenAPI 仍是可选长期演进。

**问题描述**

FE-A 说明路由字符串散落在组件中，TypeScript 类型与 Go handler 手工同步，编译器无法发现端点后缀错误。

**影响评估**

API 改名、参数变化或兼容路由很容易只更新一侧。

**推荐操作步骤**

1. 最小方案：前端集中 `api/accounts.ts`，组件不再写 URL literal。
2. 后端增加 route contract test；前端对 action endpoints 建常量与参数 builder。
3. 中期生成轻量 OpenAPI/JSON schema，仅覆盖公共 request/response，不必引入完整框架。
4. CI 增加契约 smoke：关键 endpoint method/path 必须存在。

**预期收益**

路由与 payload 漂移在 CI 阶段被发现。

**验证方法**

任意修改后端 action suffix 而不改前端，contract test 必须失败。

### AR-B（P2）读模型与写模型进一步分离

**状态：已完成本轮读模型收敛。** Dashboard 聚合端点、FTS 与 sample 懒加载已落地；物化 projection 仅在真实 p95 证明需要时再引入。

**问题描述**

Action Center、usage、search-index、dashboard totals 都属于读模型，目前部分仍在请求时从明细表聚合。

**影响评估**

历史数据增长后，读路径成本不断上升，并让 Dashboard 请求竞争 SQLite connection。

**推荐操作步骤**

1. 优先投影：latest balance、account problem bit、last checkin message。
2. 写入明细时同步更新 projection，或在事务后异步更新并记录版本。
3. projection 增加 rebuild command，避免迁移不可恢复。
4. 缓存 key 与 projection version 对齐。

**预期收益**

高频读趋近 O(1)，SQL 更短、更易测试。

**验证方法**

projection 与明细 fixture 手算一致；rebuild 后结果相同。

### AR-C（P2）明确前端资源 owner 与 invalidation 表

**状态：已关闭主要重复刷新路径。** inventory/ops/model usage 与 channel 本地动作的 owner 已收敛，并由 Hook/浏览器行为测试保护。

**问题描述**

inventory、ops、channel local state、site scoped state 都可能刷新同一资源，当前依赖调用顺序和短缓存去重。

**影响评估**

双源状态、刷新风暴和分页状态重置难以预测。

**推荐操作步骤**

1. 在 ADR 中列出资源 key → owner hook → consumers。
2. mutation 返回 invalidation keys，只由 owner 执行 refresh。
3. 全局 reload 仅用于手动“刷新全部”或恢复操作。
4. 对 endpoint 次数写测试。

**预期收益**

数据流可解释，分页与缓存演进更安全。

**验证方法**

渠道更新、账号删除、导入恢复后，只有相关资源刷新且 UI 状态一致。

### AR-D（P2）继续瘦身，但遵守现有 core freeze 边界

**状态：已完成本轮边界拆分。** `models_pricing_handlers.go` 已拆出，未扩大 App business method；后续只按真实维护成本继续演进。

**问题描述**

`models_pricing.go` 约 1197 行；App 作为 assembly root 已拥有较多 service。`checkin_balance.go` 等文件已在架构文档中明确为有意保留的 cross-cutting orchestration，不应机械拆包。

**影响评估**

无边界拆分会制造大 Infra interface；完全不拆则 models/pricing 继续膨胀。

**推荐操作步骤**

1. 优先拆 `models_pricing.go` 的纯查询、远端同步和 response projection。
2. 不新增 App business method；使用已有 domain service 或 core 内独立 service。
3. 对明确保留在 core 的 checkin orchestration 只做文件内模块化，不强拆 package。
4. 同步 `PACKAGE_INDEX.md` 和 ADR。

**预期收益**

降低 core 认知负担，同时避免为了“行数好看”扩大依赖面。

**验证方法**

依赖方向仍为 core → domain；App 字段不无序增长；包级测试时间可控。

### AR-E（P3）统一可观察性字段

**状态：已关闭。** request/task/account/site 关联字段已进入结构化日志；慢 SQL 与外呼日志不记录 SQL、URL 查询串或错误正文。

**问题描述**

HTTP access log 有 requestId/status/errorClass，但业务任务、出站 HTTP、SQLite 慢查询没有统一关联字段。

**影响评估**

当用户报告“某账号操作失败”时，难以从 request → task → upstream probe 串联定位。

**推荐操作步骤**

1. context 传播 requestId/taskId/accountId/siteId。
2. 仅记录结构化元数据，不记录 cookie/token/response body。
3. 对 >100ms SQLite 和 >1s outbound request 记录采样日志。

**预期收益**

诊断效率提高，同时不扩大敏感日志面。

**验证方法**

一次账号检测可通过 requestId 串联 HTTP、task 和 outbound 日志。

---

## 4. 配置、构建与部署优化

### CF-A（P1 · 发布阻塞）本机 Go 低于 release gate

**状态：发布验证阻塞已关闭。** 使用官方按需 Go 1.26.5 完成全量验证；系统默认 `go` 仍为 1.26.4，因此发布命令必须显式使用声明工具链。

**问题描述**

- `.go-version`：1.26.5
- `verify-release.ps1`：要求 ≥1.26.5
- 本机：Go 1.26.4

**影响评估**

无法在本机完整复现 release verification；缺少 GO-2026-5856 对应补丁版本。

**推荐操作步骤**

1. 升级本机 Go 到 1.26.5 或更高兼容补丁版。
2. 保持 CI `go-version-file` 与 release gate 单一来源。
3. 升级后完整跑 `verify-release.ps1`，不要仅跑单元测试。

**预期收益**

发布结果可本地复现，安全 toolchain 对齐。

**验证方法**

```powershell
go version
pwsh scripts/verify-release.ps1
```

### CF-B（P1 · 门禁）前端 coverage threshold 仍停留在旧水位

**状态：已关闭。** floor 已提升为 53/45/40/54，当前四项均通过。

**问题描述**

当前实际 statements/lines 为 51.87/52.98，但配置仍是 40/40；branches/functions 也低于当前实际约 2–8 个百分点。

**影响评估**

本轮新增测试可以被后续删除，而 CI 仍然通过。

**推荐操作步骤**

1. 立即设为 50/45/35/50。
2. 后续每次只提高 2–5%，避免一次性制造大量低质量测试。
3. 对高风险 hooks 设 per-file thresholds。

**预期收益**

把已经获得的覆盖率固化为长期资产。

**验证方法**

删除一个关键测试后 coverage 低于 floor，CI 必须失败。

### CF-C（P2 · CI 安全与资源）GitHub Actions 缺少最小权限、并发与超时

**状态：已关闭。** 最小权限、并发取消、超时与官方 Action 完整 SHA 均已配置。

**问题描述**

CI 使用 `actions/checkout@v4`、`setup-go@v5`、`setup-node@v4` 的可变 major tag；workflow 未显式设置 `permissions`、`concurrency`、`timeout-minutes`。

**影响评估**

- 第三方 action 供应链 pin 不够强。
- 连续 push 会重复占用 Windows runner。
- 卡住的 govulncheck/npm audit 可能长期占用资源。

**推荐操作步骤**

1. workflow 顶层设置 `permissions: contents: read`。
2. actions pin 到完整 commit SHA，并由 Dependabot/Renovate 更新。
3. 增加 branch/PR concurrency，cancel-in-progress。
4. job 设置合理 timeout（如 Go 20 分钟、frontend 15 分钟）。

**预期收益**

降低 CI 供应链与资源浪费风险。

**验证方法**

连续推两次只保留最新 run；workflow 权限页面显示 read-only。

### CF-D（P2 · 运行时一致性）本机 Node 与项目钉扎版本不一致

**状态：已关闭发布路径。** 项目固定 Node 22.23.1、npm 10.9.8，已用精确隔离工具链成功打包并校验；当前默认 shell 仍为 Node 24.16.0/npm 11.13.0，但不会进入 release 产物。

**问题描述**

- `frontend/.node-version`：22
- 本机 Node：24.16.0
- package.json 未声明 `packageManager`

虽然 engines 允许 Node ≥22.12，但开发机与 CI 的实际运行时不同。

**影响评估**

Vite/Vitest/ESLint 在 Node 22 与 24 上可能出现边界行为差异；npm 主版本也可能漂移。

**推荐操作步骤**

1. 本地使用 Node 22 LTS 对齐 CI，或明确升级 `.node-version` 并验证全套门禁。
2. 添加 `packageManager: npm@<exact-version>`。
3. runbook 写明 corepack/npm 版本策略。

**预期收益**

减少“本机绿、CI 红”的运行时差异。

**验证方法**

Node 22 环境跑 format/lint/tsc/test/build/budget 全绿。

### CF-E（P2 · 安全扫描）本地 govulncheck 不可直接执行

**状态：已关闭。** 已升级 `x/sys` 至 v0.44.0，并通过 `go run ...@v1.5.0` 与 release verifier 验证为 0 vulnerabilities；离线复现可使用已预热缓存。

**问题描述**

本机没有 govulncheck 命令；CI/release verifier 会通过 `go run ...@v1.5.0` 获取工具。npm audit 本轮为 0 vulnerabilities，Go 静态漏洞结果仅依赖 CI/发布环境。

**影响评估**

离线或 CI 网络异常时，开发机不能独立复现 Go 漏洞扫描。

**推荐操作步骤**

1. 在受控工具目录缓存固定版本 govulncheck，或由 bootstrap 脚本安装并校验版本。
2. 正式 release 禁止 SkipGoVulnCheck；离线跳过必须在 release 记录中明确审批。
3. CI 保存 govulncheck 输出 artifact。

**预期收益**

漏洞门禁更可复现，网络故障更易诊断。

**验证方法**

断网前预热工具后仍可扫描；输出包含工具版本和数据库时间。

### CF-F（P2 · 性能门禁）缺少真实高成本 endpoint 的 nightly benchmark

**状态：已关闭阶段性目标。** 已增加 5000 账号 benchmark、nightly workflow 与质量 artifact；线上 p95 趋势仍需部署后采集。

**问题描述**

当前包体与 coverage 有 gate，但 accounts page、usage、Action Center、notification page 缺少统一的大数据 handler benchmark。

**影响评估**

SQL 扩展性回归可能在功能测试全绿时进入 main。

**推荐操作步骤**

1. 建立固定 5k accounts / 100k snapshots fixture。
2. benchmark 完整 handler，包括 JSON 编码，不只测 SQL helper。
3. nightly 执行并保存历史趋势；PR 只跑轻量 smoke。

**预期收益**

性能回归可量化、可追踪。

**验证方法**

人为移除关键索引后 nightly 明显变慢或触发阈值。

### CF-G（P3 · 发布完整性）保持现有包体与 release verifier 优势

**状态：已关闭。** CI 上传质量产物，release manifest 记录工具链与 7 个源输入 SHA256，package verifier 校验实际值与声明值一致。

**问题描述**

当前 bundle budget、package verify、embedded UI smoke、端口归属清理都已实现，是值得保留的发布资产。

**影响评估**

如果这些结果只停留在控制台输出，后续排查某个发布包时仍缺少统一的 toolchain、包体和校验记录。

**推荐操作步骤**

1. bundle budget JSON 作为 CI artifact 上传。
2. release 元数据记录 Go/Node/npm/govulncheck 版本。
3. package verify 输出 zip SHA256 和内部文件 manifest。

**预期收益**

发布可追溯性更完整。

**验证方法**

任意 release zip 可反查 commit、toolchain、验证日志与 SHA256。

---

## 5. 建议落地路线图

### S0 · 合并前必须完成

1. [x] 修复 AccountDetailContent `/test-login-status` 路由错误并补交互测试。
2. [x] 修复 SSRF 初始请求/allowLocal pin 逻辑并加 rebinding 测试。
3. [x] health DB 错误改稳定公共文案。
4. [x] 删除 API 中 browser profile 绝对路径字段。
5. [x] 把 frontend coverage floor 提到 53/45/40/54。
6. [x] 修正 import PublicMsg 分类，避免所有 SQLite 错误都提示路径越界。

### S1 · 当前迭代

1. [x] 收敛 browser/batch/checkin/proxy 的业务 result error contract。
2. [x] 账号搜索 debounce + page totals 解耦 + cursor 复合索引。
3. [x] 渠道刷新 owner 去重。
4. [x] Go 1.26.5 对齐并跑完整 release verifier。

### S2 · 下一迭代

1. [x] search-index 改服务端搜索或返回 truncated 状态。
2. [x] usage 增加 site/limit/truncated，并用受限窗口查询与精确索引控制成本。
3. [x] Action Center samples 改为展开后按需加载。
4. [x] 拆 AccountInsights/AccountCard 子组件、工具函数与 domain types。
5. [x] token Windows DACL 自动化并执行写后复核。

### S3 · 持续优化

1. [x] 去除 CSP `style-src 'unsafe-inline'`，禁止 style attribute。
2. [x] Dashboard 聚合请求与 Analytics/面板分层按需加载。
3. [x] 增加 5000 账号 nightly 大数据 benchmark 与 artifact。
4. [x] CI action SHA pin、最小权限、并发取消与 timeout。

---

## 6. 每包验证清单

```powershell
cd E:\zidqiandao\relaycheck-desktop
go vet -mod=vendor ./ ./internal/...
go test -mod=vendor -count=1 ./internal/...
go test -mod=vendor -cover -count=1 ./internal/core/   # >=55%

cd frontend
npm run format:check
npm run lint
npx tsc -b
npm test
npm run test:coverage
npm run build
npm run budget:check
npm audit --audit-level=moderate
```

安全修复额外验证：

```text
- route contract：账号详情 test-login 必须命中后端 handler
- SSRF：DNS rebinding / redirect / allowLocal 三组 resolver 测试
- error contract：response 不含绝对路径、SQL、token、cookie
- health：未认证响应只含稳定状态，不含驱动错误
```

发布前：

```powershell
$env:GOTOOLCHAIN = "go1.26.5"
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\verify-release.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\package-release.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\verify-package.ps1 -ZipPath <zip>
```

---

## 7. 最终评价

RelayCheck Desktop 的单二进制、loopback HTTP、SQLite 和 embedded React 组合与产品定位匹配，近期分页、错误 envelope、包体预算、domain service 抽取都在正确方向。当前不需要微服务化或数据库迁移。

本轮已完成正确性与安全边界、关键行为测试、FTS/索引与聚合 API、资源 owner、代码/CSS 拆分、coverage floor、Windows token DACL、结构化关联日志、CI 固化和完整 release verification。当前 release gate 无已知阻塞项。

下一阶段只保留需要外部或长期数据支撑的工作：

1. **建立真实性能基线：** 用 RUM、启动 waterfall 和 API p95 验证首屏 <2 秒及 API 延迟降低 50%，不能用本机单次 benchmark 替代线上结论。
2. **持续提高测试质量：** 优先覆盖 AccountInsights、ChannelsPanel 和 onboarding 的交互分支，再小步提高 coverage floor；AccountCardEditor 已达到 100% branches。

这些残余项不阻塞当前工作树通过 release gate；依赖升级和门槛提升仍应采用小批次、可测量、可回滚的方式。
