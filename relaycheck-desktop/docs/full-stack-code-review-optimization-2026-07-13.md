# RelayCheck Desktop 全栈代码审查与优化报告

- **日期：** 2026-07-13
- **审查范围：** `E:\zidqiandao\relaycheck-desktop`
- **技术栈：** Go 1.24 模块 / `net/http` / SQLite（modernc）/ React 19 / TypeScript 5.9 / Vite 8 / Vitest 4
- **审查方式：** 源码与测试审读、静态检查、依赖审计、构建与包体基线、数据库查询与 API 契约审查、发布与运维配置核对
- **结论：** 当前代码库整体健康，没有发现 P0 级远程代码执行、SQL 注入、明文密钥入库或前端直接 XSS sink。主要风险集中在质量门禁未闭环、账号列表扩展性、通知分页契约错误、内部错误泄露，以及 CI/运维文档与当前实现不一致。

---

## 0. 审查基线

### 0.1 已执行验证

| 检查项 | 结果 | 说明 |
|---|---:|---|
| `go test -mod=vendor -count=1 ./...` | 通过 | 1037 tests；当前 `./...` 还会包含 `frontend/node_modules/flatted/golang/pkg/flatted`，见 CF-1 |
| `go vet -mod=vendor ./...` | 通过 | 未发现 vet 问题 |
| `go test -mod=vendor -cover -count=1 ./internal/...` | 通过 | core 55.4%、accounts 45.5%、channels 60.8%、notifications 58.6% 等 |
| `npx tsc -b` | 通过 | TypeScript 无错误 |
| `npm test` | 通过 | 29 files / 268 tests |
| `npm run lint` | 通过 | ESLint 退出码 0，但规则集较窄，见 FE-3 |
| `npm run build` | 通过 | 生产构建成功 |
| `npm audit --audit-level=moderate` | 通过 | 0 个已知 npm 漏洞 |
| `npm run format:check` | **失败** | 93 个源文件不符合 Prettier，见 FE-1 |
| `npm run test:coverage` | **失败** | 缺少 `@vitest/coverage-v8`，见 FE-2 |
| `govulncheck` | 未独立执行 | 当前机器未安装；发布脚本在联网时可运行，但失败只告警，见 CF-5 |

### 0.2 前端包体基线

| 产物 | 原始大小 | gzip |
|---|---:|---:|
| 主 CSS | 205.36 kB | 34.44 kB |
| 主 JS | 225.56 kB | 70.76 kB |
| `panel-accounts` | 119.29 kB | 35.11 kB |

当前体积尚可，但没有自动预算，后续回归无法被 CI 及时阻断。

### 0.3 优先级定义

| 级别 | 含义 | 建议时限 |
|---|---|---|
| P0 | 可直接导致远程接管、严重数据损坏或密钥大规模泄露 | 立即阻断发布 |
| P1 | 已影响正确性、门禁可靠性、主要工作流或明显扩展性 | 当前迭代处理 |
| P2 | 中期会增加故障率、维护成本或安全暴露面 | 1-2 个迭代处理 |
| P3 | 纵深防御、体验或工程成熟度提升 | 排入持续优化 |

本轮共记录 **P0 0 项、P1 8 项、P2 16 项、P3 1 项**。

---

## 1. 前端代码审查

### 1.1 已验证的优点

- 面板使用 `React.lazy` 和 Vite `manualChunks`，非首屏功能已分块。
- `useApi` 使用 `AbortSignal`、短时读缓存和统一错误对象；写请求会清理缓存。
- 隐藏面板带 `inert` / `aria-hidden`，并支持 inactive 时暂停部分数据请求。
- 未发现 `dangerouslySetInnerHTML`、`eval()` 或 `new Function`。
- 外链统一经过 `safeExternalUrl`，并使用 `target="_blank"` + `rel="noopener noreferrer"`。
- `localStorage` 仅保存主题、引导状态、更新忽略版本和选中站点等非密钥偏好。
- React 测试、TypeScript、ESLint 和生产构建均通过。

### FE-1（P1）Prettier 门禁当前不可通过

**问题描述**

`frontend/package.json:18` 已定义 `format:check`，但实际执行发现 93 个文件不符合格式，涵盖 API、hooks、主要组件、测试和 CSS。CI 与发布脚本均未运行该命令，因此格式脚本长期处于“存在但不可作为门禁”的状态。

**影响评估**

- 大量无关格式 diff 增加代码审查噪音。
- 自动生成或跨模型修改时更容易形成风格漂移。
- 一旦未来把 `format:check` 直接加入 CI，会一次性阻断所有提交。

**推荐操作步骤**

1. 单独创建一次纯格式变更，运行 `rtk npm run format`，不要与功能修改混合。
2. 审查格式 diff，重点检查 JSX 换行、长字符串和 CSS 层顺序没有语义变化。
3. 在 `.github/workflows/ci.yml` 的前端 job 中加入 `rtk npm run format:check`。
4. 可选增加 `lint-staged`，但只在团队明确接受本地 hook 后启用。

**预期收益**

稳定 diff、降低审查成本，并让 Prettier 真正成为可依赖的质量门禁。

**验证方法**

```powershell
cd frontend
rtk npm run format:check
rtk npm run lint
rtk npx tsc -b
rtk npm test
```

四条命令均应退出码为 0，格式提交不得包含行为变化。

### FE-2（P1）前端覆盖率脚本不可用

**问题描述**

`frontend/package.json:15` 定义了 `vitest run --coverage`，但 `devDependencies` 没有 `@vitest/coverage-v8`。`package-lock.json` 中出现的条目只是 Vitest 的 optional peer 声明，并不代表依赖已安装。当前命令直接报 `MISSING DEPENDENCY`。

**影响评估**

- 无法量化关键 hooks、API 客户端和高风险组件的回归保护。
- CI 虽有 268 个测试，但无法识别“测试数量增加、实际覆盖下降”。
- 重构 `AccountInsights`、`AccountCard` 等大型组件时缺少覆盖率基线。

**推荐操作步骤**

1. 安装与 Vitest 同版本的 provider：`@vitest/coverage-v8@4.1.9`。
2. 在 `vite.config.ts` 或独立 `vitest.config.ts` 中设置 `coverage.provider = "v8"`。
3. 首轮只建立基线，不盲目设置过高阈值；建议从 statements/lines 60%、branches 50% 起步。
4. 对 `api/client.ts`、`useApi.ts`、导航、设置导入导出、账号批处理设置更高的文件级目标。
5. 把覆盖率命令加入 CI，并上传文本或 LCOV artifact。

**预期收益**

把“有测试”升级为“关键行为被覆盖”，为后续拆组件和 API 改造提供可量化护栏。

**验证方法**

```powershell
cd frontend
rtk npm run test:coverage
```

命令应成功生成报告；CI 应在低于阈值时失败，并能看到未覆盖文件列表。

### FE-3（P2）ESLint 只覆盖 React Hooks，缺少 TypeScript 语义规则

**问题描述**

`frontend/eslint.config.js` 只加载 `@typescript-eslint/parser` 和 `react-hooks` 插件；`exhaustive-deps` 仍是 warning。未启用 `no-floating-promises`、`no-misused-promises`、类型感知的未使用变量检查等规则。

**影响评估**

- Promise 遗漏、异步事件处理和类型收窄错误主要依赖人工审查。
- 当前 lint 通过不能代表 TypeScript 异步正确性通过。
- 大量 `void action()` 调用需要规则区分“明确忽略”与“意外遗漏”。

**推荐操作步骤**

1. 增加与 parser 同版本的 `@typescript-eslint/eslint-plugin` 或 `typescript-eslint` 配置包。
2. 先启用非类型感知推荐集，再逐步启用 `no-floating-promises`、`no-misused-promises`。
3. 为测试文件、浏览器脚本和 Node 脚本配置不同 globals。
4. 清理 warning 后将 `react-hooks/exhaustive-deps` 提升为 error。
5. CI 使用 `eslint --max-warnings=0`，避免 warning 永久积累。

**预期收益**

在提交前捕获更多真实异步缺陷，减少依赖运行时或用户操作才能暴露的问题。

**验证方法**

```powershell
cd frontend
rtk npm run lint -- --max-warnings=0
rtk npx tsc -b
rtk npm test
```

### FE-4（P1）账号数据仍是“启动全量加载 + 客户端分页”

**问题描述**

- `useInventoryData.ts:9` 在应用启动时请求 `/api/accounts`。
- 后端默认返回 500 条、最多 1000 条（`internal/core/accounts.go:26-29`）。
- `AccountsPanel.tsx:21,110-113` 只在前端按 50 条切片。
- 搜索、登录状态和余额筛选仍在浏览器对全量数组执行。

这改善了渲染节点数量，但没有减少启动网络、JSON 解析、React 状态和服务端查询开销。

**影响评估**

- 账号数接近 500 时，用户即使只看 Dashboard，也会支付账号全量加载成本。
- 超过 1000 条后，前端永远看不到被截断的数据，且没有 `total` 或 `nextCursor` 提示。
- 前端“总数”可能只是本次响应数量，不是数据库真实总数。

**推荐操作步骤**

1. 为 `/api/accounts` 增加稳定游标分页：`cursor`、`limit`、`query`、`status`、`upstreamSiteId`。
2. 返回 `{ items, nextCursor, total }`，排序使用 `(updated_at DESC, id DESC)` 保证游标稳定。
3. 保留旧数组响应一段迁移期，或新增 `/api/accounts/page`，避免一次破坏所有调用点。
4. `useInventoryData` 不再启动加载完整账号；改为账号摘要或按活动面板加载。
5. Channels 只请求所需账号聚合信息，不复用完整账号详情 DTO。
6. 建立 5k/20k 账号 fixture，记录响应体、查询耗时和浏览器 heap。

**预期收益**

账号规模扩大时启动时间和内存保持稳定，消除 1000 条静默截断，并让分页总数可信。

**验证方法**

- 20k 账号时首屏不请求全量账号。
- 单页响应不超过配置的 `limit`，`nextCursor` 无重复/遗漏。
- p95 查询时间和响应体大小写入基准；前后翻页后 ID 集合完整。
- 新增后端分页测试和前端 hook 测试。

### FE-5（P2）渠道数据刷新存在隐式重复与所有权不清

**问题描述**

`useChannelActions.refresh()` 会请求 channels、models overview、accounts；`ChannelsPanel.refreshAll()` 随后还会调用父级 `onRefresh()`，父级又刷新 inventory 的 channels/sites/accounts。当前 1.5 秒客户端缓存可能掩盖重复请求，但正确性依赖调用顺序和 TTL，而不是明确的数据所有权。

**影响评估**

- 缓存失效或请求超过 TTL 时产生重复网络和状态覆盖。
- 同一资源在 parent inventory 与 local hook 中有两份 source of truth。
- 后续接入服务端分页后，两份账号状态更难保持一致。

**推荐操作步骤**

1. 明确 owner：inventory 负责 channels/accounts，渠道 hook 只负责 models/health；或反过来由渠道域独占。
2. 写操作后按资源键失效，而不是调用“刷新全部”。
3. 将 `refreshAll()` 改为并行、去重的资源刷新计划。
4. 用 fetch mock 断言“进入渠道页”和“手动刷新”各 endpoint 的请求次数。

**预期收益**

减少隐式重复请求，避免局部状态覆盖全局状态，使分页和缓存策略更容易演进。

**验证方法**

冷启动进入渠道页、手动刷新、模型同步、健康探测完成四条路径中，每个资源请求次数应符合明确契约。

### FE-6（P2）大型组件仍承担过多领域职责

**问题描述**

当前行数基线：`AccountInsights.tsx` 705 行、`AccountCard.tsx` 498 行、`SettingsCards.tsx` 458 行。它们同时承担数据请求、确认流程、状态机、格式化和展示。

**影响评估**

- 修改单个操作时容易触发整块组件重渲染和大范围回归。
- 单测需要构造过多无关状态，导致测试更倾向只测 export 或浅层行为。
- 同类账号操作在卡片、详情和批量向导间容易出现文案或错误处理不一致。

**推荐操作步骤**

1. 以用户操作拆分，而不是按视觉卡片机械拆分：API Key、登录会话、余额/签到、清理、批量重登。
2. 将命令逻辑提到 `useAccountActions` 或领域 command 模块，组件只消费状态与结果。
3. 统一确认、busy、错误反馈模式，避免每个组件自建 `runAction`。
4. 每次拆分控制在约 100-300 行 diff，并先补行为测试。

**预期收益**

缩小修改半径、提升测试可读性，并减少账号操作在不同视图间的行为漂移。

**验证方法**

拆分前后关键操作测试保持通过；React Profiler 中无关区域提交次数不增加；组件文件职责可由名称清晰表达。

### FE-7（P2）包体有基线但没有预算门禁

**问题描述**

Vite 已手动分块且当前 gzip 体积可接受，但构建只报告大小，不会因主包或 panel 突增而失败。

**影响评估**

- 新增依赖或错误 import 可能让主包快速膨胀。
- 动态面板可能被意外拉回入口 chunk，只有用户感知到首屏变慢后才发现。

**推荐操作步骤**

1. 保存当前构建产物尺寸为基线。
2. 增加无第三方依赖的 Node 检查脚本，读取 `dist/assets` 并计算 gzip。
3. 初始预算建议：主 JS 80 kB gzip、主 CSS 40 kB gzip、任一 panel 45 kB gzip。
4. CI 上传 bundle manifest；超过预算时输出具体 chunk 和增量。

**预期收益**

把前端性能从人工观察变成可回归指标，阻止依赖或分块策略的无意退化。

**验证方法**

故意引入超预算 fixture 时 CI 应失败；恢复后 `npm run build` 与预算检查均通过。

### FE-8（P3）CSP 仍依赖 `style-src 'unsafe-inline'`

**问题描述**

`internal/core/http.go:214-220` 的 CSP 禁止内联脚本，但允许内联样式；前端存在较多 React `style={{...}}`。当前未发现直接 XSS sink，因此这属于纵深防御缺口，不是已证实漏洞。

**影响评估**

如果未来引入可控 HTML/属性注入，`unsafe-inline` 会降低样式层面的 CSP 防护；同时也阻碍进一步收紧策略。

**推荐操作步骤**

1. 新代码优先使用 CSS class 或受控 CSS custom properties。
2. 逐步迁移固定 inline style，保留确需动态计算的属性。
3. 建立 CSP 响应头测试和嵌入式 UI smoke。
4. 完成迁移后尝试移除 `style-src 'unsafe-inline'`，若必须保留则在威胁模型中明确原因。

**预期收益**

提升 XSS 纵深防御，并让安全策略与前端实现保持可验证的一致性。

**验证方法**

嵌入式 release UI 无 CSP violation；主题、抽屉和动态布局正常；响应头不再包含不必要的 `unsafe-inline`。

---

## 2. 后端代码审查

### 2.1 已验证的优点

- SQLite 使用 WAL、`busy_timeout=5000`、`foreign_keys=1`、`synchronous=NORMAL`，连接池限制为 4。
- SQL 主要使用参数化语句；关键表已有组合索引，如 `idx_checkin_logs_account_started`。
- 所有业务 API 经 `requireSession`；状态变更请求校验 Origin 和 loopback `RemoteAddr`。
- HTTP 层设置 Host allowlist、CSP、`X-Frame-Options`、`nosniff`、`Referrer-Policy`，JSON body 默认限制 8 MiB。
- 可选 session token 使用 256-bit 随机值、HttpOnly、SameSite=Strict 和常量时间比较。
- 出站 URL 会拒绝 loopback/private/link-local/multicast/metadata 地址，每次 redirect 也重新校验。
- 访问日志已有 requestId、状态码和 durationMs，且不会记录 Authorization 或请求 body。
- 后端全部测试、vet 和主要包覆盖率门槛通过。

### BE-1（P1）通知列表的 `limit=100` 实际被强制压成 10

**问题描述**

`internal/core/notification.go` 先将默认 `limit` 设为 100，随后调用 `clampBatchLimit(limit, 100)`；而 `internal/core/http.go:91-103` 的 helper 无条件把 fallback 和 value 最大限制为 10。结果是 `/api/notifications` 默认最多返回 10 条，即使调用方传 `limit=100` 也只能得到 10 条。前端摘要又把当前数组长度显示为“总数”，因此会产生错误的用户认知。

**影响评估**

- API 参数与实际行为不一致，属于可复现的正确性问题。
- 第 11 条之后的通知无法在 UI 查看，但数据库仍保留。
- “总数/未读/已读”只是当前 10 条统计，不是数据库总量。

**推荐操作步骤**

1. 将批处理并发限制与列表分页限制拆成两个 helper，例如 `clampBatchLimit(max=10)` 和 `clampListLimit(fallback,max)`。
2. 通知列表使用明确最大值，例如默认 50、最大 200。
3. 返回 `{ items, total, unreadTotal, nextOffset }` 或游标分页结构。
4. 前端摘要使用服务端统计，不再从当前页数组推导全局总数。
5. 为默认值、`limit=1/100/999`、offset 和过滤组合增加 handler 测试。

**预期收益**

修复通知可见性和统计正确性，并形成可复用、语义清晰的列表分页契约。

**验证方法**

插入 25 条通知后，默认请求和 `limit=20` 应返回契约规定数量；UI 总数应显示 25，而不是当前页长度。

### BE-2（P1）账号列表存在相关子查询与静默截断扩展风险

**问题描述**

`internal/core/accounts.go:45-48` 为每个账号执行相关子查询取得最新签到消息。已有 `(account_id, started_at)` 索引，因此当前 500 条规模未必慢，但查询成本仍随账号行数增加。接口只支持 site filter 和 limit，没有 cursor/offset/total；默认 500、最大 1000，并静默截断。

现有 `perf_large_dataset_test.go` 只在 500 账号下测 `COUNT(*)` 和 analytics，不测实际账号列表、JSON 编码或 HTTP handler。

**影响评估**

- 账号和签到日志增长后，列表可能成为 p95 热点。
- 超过最大 limit 的账号不可见且调用方无法知道数据不完整。
- 当前性能测试不能捕获该路径回归。

**推荐操作步骤**

1. 使用 `EXPLAIN QUERY PLAN` 验证当前索引命中，并记录 5k/20k fixture 的查询时间。
2. 优先实现游标分页和服务端过滤，先降低单次参与查询的账号数。
3. 若仍超预算，改用窗口函数/CTE 一次选出每个账号最新日志，或在账号表维护 last-checkin projection。
4. 稳定排序增加 `id` tie-breaker，避免相同 `updated_at` 下漏项。
5. 增加 handler benchmark，覆盖 DB 查询、scan、JSON 编码和响应体大小。

**预期收益**

把列表性能从“依赖当前数据规模”变成可预测、可压测的契约，并消除静默丢数据。

**验证方法**

- `EXPLAIN QUERY PLAN` 必须命中 `idx_channel_accounts_updated` 和 `idx_checkin_logs_account_started`。
- 20k 账号 fixture 下单页 p95 达到团队设定预算，例如本机 <100 ms。
- 分页遍历后 ID 无重复、无遗漏，`total` 与数据库一致。

### BE-3（P2）内部错误文本广泛直接返回客户端

**问题描述**

`writeError` 已提供稳定 `errorClass`，但仍原样序列化调用方传入的 message。`internal/core` 多个 handler 在 500 响应中直接传 `err.Error()`，包括数据库、备份、导入、网络和系统设置路径。

**影响评估**

- 本机单用户模式风险低于公网服务，但错误可能暴露 SQL、文件名、上游响应或系统细节。
- 前端依赖不稳定的底层错误文本，未来替换数据库/驱动会改变用户体验。
- requestId 已存在，却没有成为“客户端稳定错误 + 服务端详细日志”的关联键。

**推荐操作步骤**

1. 定义应用错误类型：validation、not_found、conflict、upstream、internal。
2. 4xx 只返回经过审查的用户可操作信息；5xx 返回稳定文案和 requestId。
3. 完整 `err` 仅写服务端日志，敏感路径先脱敏。
4. 将 `writeError` 扩展为接受 error code/requestId，逐域迁移，避免一次大改。
5. 增加测试，断言 500 响应不含绝对路径、SQL、密钥片段和原始上游 body。

**预期收益**

减少信息泄露，稳定前后端错误契约，并通过 requestId 保留排障能力。

**验证方法**

构造数据库错误、无效备份和上游 500；客户端只看到稳定 code/message/requestId，日志能用 requestId 找到完整错误。

### BE-4（P2）SSRF 校验仍存在 DNS rebinding 时间窗

**问题描述**

`url_safety.go` 在发请求前通过 `LookupIPAddr` 校验 DNS；`http.Transport` 真正连接时会再次解析。攻击者控制 DNS 时，可能在校验阶段返回公网 IP、连接阶段切换为私网 IP。redirect 已逐跳重校验，但没有消除“校验解析与拨号解析分离”的 TOCTOU。

**影响评估**

在当前“用户主动配置上游地址、本机桌面应用”模型下为中等残余风险；如果未来接受远端导入的 URL 或开放非本机访问，风险会明显上升。

**推荐操作步骤**

1. 将校验结果传入自定义 `DialContext`，连接已验证 IP，同时保留原 Host/SNI。
2. 若有多个公网 IP，只在已验证集合内重试。
3. redirect 每跳重新生成允许 IP 集合。
4. 添加可注入 resolver/dialer，编写 DNS rebinding 单测。

**预期收益**

把 SSRF 防护落实到实际连接目标，而不仅是请求前字符串和 DNS 快照校验。

**验证方法**

测试 resolver 首次返回公网、第二次返回 `127.0.0.1`/`169.254.169.254`；请求必须失败且测试服务不得收到连接。

### BE-5（P2）本机 token 为 opt-in，Windows 文件权限需显式验证

**问题描述**

`RELAYCHECK_REQUIRE_TOKEN=1` 才启用 token；默认仍按可信单用户 loopback 模型运行。token 文件使用 `0600` 写入，但 Windows ACL 并不等价于 POSIX mode，实际访问权限取决于目录继承。该项不是当前默认威胁模型下的直接漏洞，但共享电脑/多用户安装需要更强保证。

**影响评估**

- 默认模式下，同机原生进程可以调用读写 API；Host/Origin/loopback 主要阻断远端访问和浏览器 CSRF，不能隔离本机进程。
- hardened 模式若 token 文件 ACL 过宽，其他本机用户可能读取 token。

**推荐操作步骤**

1. 运维文档明确“默认可信单用户”和“hardened token”两种模式。
2. 多用户机器默认设置 `RELAYCHECK_REQUIRE_TOKEN=1`。
3. Windows 创建 token/key 文件后验证并收紧 DACL，仅当前用户和 SYSTEM 可读。
4. 若未来允许非 loopback bind，必须自动强制 token，不允许仅靠配置自觉。

**预期收益**

让实际部署安全边界与文档一致，并降低共享 Windows 主机上的本地横向访问风险。

**验证方法**

- token 模式下无 cookie 的 `/api/system/status` 返回 401，正确 cookie 成功。
- 使用 `Get-Acl data\session-token.txt` 验证无不必要主体拥有读取权限。
- 非 loopback 配置测试必须证明 token 自动强制。

### BE-6（P2）性能测试没有覆盖真实高成本端点

**问题描述**

现有大数据测试只插入 500 账号/日志，并检查简单 COUNT 和 analytics 查询；未覆盖账号列表、通知过滤分页、Action Center、模型概览、JSON 编码和 HTTP handler。

**影响评估**

索引存在并不等于端点满足延迟预算；查询、scan、转换和编码任一环节回归都可能绕过当前测试。

**推荐操作步骤**

1. 建立可复用 5k/20k 账号、100k 日志、10k 通知 fixture builder。
2. 为热点 handler 添加 Go benchmark 和普通上限测试。
3. 分别记录 DB-only、handler、响应体大小，避免只测 COUNT。
4. CI 跑小规模上限测试；较大 benchmark 放 nightly 或手工性能门禁。

**预期收益**

提前发现查询计划和序列化回归，为分页、索引和 projection 决策提供数据。

**验证方法**

保存 benchmark 基线；同一 runner 上 p95/allocs 超过容忍阈值时失败或告警。

### BE-7（P2）路由和 API 契约仍靠手工同步

**问题描述**

`internal/core/routes.go` 集中注册大量字符串路由，方法校验散落在 handler；前端另行手写 URL 和 TypeScript DTO。当前没有统一的 route metadata、分页 schema 或可机器验证的错误 code 表。

**影响评估**

- 路由、方法、权限包装和前端调用容易漂移。
- 新增分页后，需要同时维护 Go struct、JSON、TS 类型、hooks 和测试。
- 直接一次迁移 `/api/v1` 成本高且当前本机产品收益有限，不应作为首要方案。

**推荐操作步骤**

1. 先建立 typed route table：path、method、handler、session policy、request/response type。
2. 从 metadata 生成 OpenAPI 或最小 JSON schema，再生成/校验 TypeScript 类型。
3. 为所有路由增加契约测试：方法错误、鉴权、Content-Type、errorClass。
4. 保持现有 URL；只有出现真实兼容需求时再引入版本前缀。

**预期收益**

减少前后端契约漂移，在不进行高成本 URL 迁移的前提下获得可生成、可测试的 API 描述。

**验证方法**

CI 中生成物无 diff；route contract test 覆盖所有注册项；删除或改名一个 DTO 字段时类型检查能捕获调用方。

---

## 3. 整体架构建议

### 3.1 当前架构评估

当前“Go 单二进制 + embedded React + SQLite + loopback”与本地运维台场景匹配，部署面小、备份简单、无需独立数据库。后端已经把 accounts、channels、sites、notifications、backup 等域提取到 `internal/<domain>`，`core` 作为 assembly root；该方向正确。主要问题不是需要推倒重构，而是需要把既有边界、数据所有权和质量门禁进一步固化。

### AR-1（P2）继续冻结 `core`，用可执行规则守住领域边界

**问题描述**

`internal/core` 仍包含大量 handler、forwarder、跨域聚合和生命周期逻辑。`PACKAGE_INDEX.md` 已规定不得继续向 `App` 堆业务字段，但目前主要依赖文档自觉。

**影响评估**

长期开发中最容易出现“为了方便再加一个 App 方法”，逐步逆转领域拆分成果。

**推荐操作步骤**

1. 保持 `App` 只负责 DB/client/lifecycle/domain service wiring。
2. 新业务优先进入现有 domain service，跨域读取使用窄接口或只读 projection。
3. 增加 architecture test，限制 domain 包不得 import `internal/core`，并检查新增 `App` 字段。
4. 每季度更新 `PACKAGE_INDEX.md`，删除已完成迁移的兼容 forwarder，而不是永久保留。

**预期收益**

防止核心包重新膨胀，降低跨域回归和测试 fixture 复杂度。

**验证方法**

`go list -deps`/architecture test 证明依赖单向；新增业务不需要扩展 `App` 公共面。

### AR-2（P2）建立前端资源所有权与按需加载模型

**问题描述**

当前 inventory、panel hooks 和局部 action hooks 会持有同一资源副本。随着账号分页和资源级缓存加入，双 source of truth 会成为主要复杂度来源。

**影响评估**

状态覆盖、重复请求、失效范围过大，并增加“刷新后某个面板仍显示旧数据”的概率。

**推荐操作步骤**

1. 为 system、inventory、ops health、accounts page、channel detail 定义明确 owner。
2. 继续使用项目现有 hooks，不必立即引入大型状态库。
3. 将缓存键、失效键和分页参数集中到 query module。
4. 写一页 ADR，记录哪些资源全局、哪些资源按 tab/详情加载。

**预期收益**

在不增加重依赖的前提下获得可预测的数据生命周期，为服务端分页铺路。

**验证方法**

资源请求次数测试通过；写操作只失效受影响资源；切换 tab 后没有重复全量加载。

### AR-3（P2）以契约生成替代手工 Go/TS 双维护

**问题描述**

Go API struct、前端 TypeScript 类型、路由和错误 class 分别维护。字段重命名或分页改造的风险会跨多个目录传播。

**影响评估**

编译只能覆盖各自语言内部，无法证明 JSON 契约一致。

**推荐操作步骤**

1. 从 route metadata/Go schema 生成 OpenAPI 或 JSON Schema。
2. 生成 TS 类型到明确的 generated 目录，并在 CI 检查无未提交 diff。
3. 对核心响应保留 golden JSON contract test。
4. 对 breaking change 采用显式迁移记录，而不是静默改字段。

**预期收益**

减少跨语言契约错误，并使 API 演进可以被代码审查和 CI 证明。

**验证方法**

修改 Go 字段后生成检查必须提示 TS diff；golden test 能捕获缺字段和类型变化。

### AR-4（P2）把现有日志升级为可操作的性能与故障预算

**问题描述**

HTTP 已记录 requestId/status/duration，但没有 endpoint 维度的慢请求预算、SQLite 慢查询采样或趋势汇总。

**影响评估**

性能问题只能在用户感知后手工复现，难以判断回归来自 DB、网络还是序列化。

**推荐操作步骤**

1. 记录规范化 route 名，不把 ID 直接作为高基数标签。
2. 对超过阈值的 DB/handler 输出结构化 slow log，包含 requestId 和 duration。
3. 在 diagnostics 中只暴露聚合统计，不暴露绝对路径或敏感参数。
4. 为账号列表、Dashboard、Action Center、模型概览设 p95 和响应体预算。

**预期收益**

用数据定位热点，并能验证优化是否真实改善用户路径。

**验证方法**

压测时慢请求日志可关联 requestId；优化前后报告能对比 p50/p95、allocs 和响应体。

---

## 4. 配置、构建与部署优化

### CF-1（P1）CI Go 版本与模块声明不一致，且测试范围不完整

**问题描述**

- `go.mod:3` 声明 Go 1.24，`.github/workflows/ci.yml:16` 固定 1.22.x。
- `GOTOOLCHAIN=auto` 时 runner 可能额外下载 1.24；受限网络下可能直接失败，构建也不再完全可复现。
- CI vet 只覆盖 accounts/core/notifications/versioncheck，test 只覆盖 accounts/core；本地全量测试实际还有 backup、channels、sites 等包。
- `go list ./...` 当前会包含 `frontend/node_modules/flatted/golang/pkg/flatted`，说明 Go package pattern 被前端依赖污染。

**影响评估**

CI 可能与本地验证使用不同编译器、漏测多个后端域，并把无关 npm 依赖当 Go 包处理。

**推荐操作步骤**

1. `actions/setup-go` 使用 `go-version-file: go.mod`，或固定经过验证的 1.24.x。
2. CI 改为 `go vet -mod=vendor ./ ./internal/...`。
3. CI 改为 `go test -mod=vendor -count=1 ./ ./internal/...`。
4. 覆盖率分别对需要门槛的包执行，不使用根目录 `./...` 生成单一 profile。
5. 保留 core 55% 门槛，并逐步为 accounts/notifications 设置基线。

**预期收益**

CI 与模块声明一致，覆盖全部自有 Go 代码，避免网络自动下载和 node_modules 污染。

**验证方法**

GitHub Actions 日志显示预期 Go 版本；package 列表只包含根包和 `internal/*`；所有域测试/vet 通过。

### CF-2（P1）CI/发布门禁没有覆盖格式与前端覆盖率

**问题描述**

CI 已运行 lint、tsc、test、build；`verify-release.ps1` 已运行前端测试/lint、全量 Go test/vet、build 和 npm audit。但两者都没有 `format:check`，也没有可工作的前端覆盖率门槛。

**影响评估**

当前两个已知红灯不会阻止合并或发布，导致“门禁全绿”与实际工程状态不一致。

**推荐操作步骤**

1. 先完成 FE-1 的独立格式提交和 FE-2 的 coverage provider 配置。
2. CI 顺序建议：format -> lint -> tsc -> unit/coverage -> build。
3. 发布脚本复用同一 npm script，避免 CI 与本地门禁分叉。
4. 将 coverage 报告和 bundle manifest 作为 artifact 保存。

**预期收益**

所有声明存在的质量门禁都可运行、可阻断，减少发布前才发现工程债的情况。

**验证方法**

故意制造格式错误或降低覆盖率时 CI/verify-release 应失败；恢复后全绿。

### CF-3（P1）运维文档仍描述不存在的 bootstrap 登录

**问题描述**

当前实现和 `docs/PROJECT_STRUCTURE.md` 明确：应用是可信本机单用户模型，没有多用户 unlock password；可选 `RELAYCHECK_REQUIRE_TOKEN=1`。但以下文档仍要求 `admin`、`RELAYCHECK_BOOTSTRAP_PASSWORD`、`bootstrap-admin-password.txt` 或 `RELAYCHECK_SMOKE_PASSWORD`：

- `README.md:37,65,166`
- `docs/OPERATOR_RUNBOOK.md:15,42,118`
- `docs/LAUNCH_READINESS.md:58,65`
- `docs/OPERATOR_ACCEPTANCE_RECORD.md:34`

源码中没有对应环境变量或文件逻辑。

**影响评估**

- 操作员会等待不存在的密码文件或尝试不存在的登录流程。
- 安全文档同时声明“可信本机”与“admin 登录”，威胁模型自相矛盾。
- 发布验收记录可能填写虚假或无意义字段。

**推荐操作步骤**

1. 全局删除 bootstrap admin/password 说明。
2. 增加统一环境变量表：`RELAYCHECK_PORT`、`RELAYCHECK_DATA_DIR`、`RELAYCHECK_NO_OPEN`、`RELAYCHECK_REQUIRE_TOKEN`。
3. Runbook 分成默认模式和 hardened token 模式，说明 token 文件位置和 cookie 行为。
4. 更新 acceptance record，不再要求登录；改为验证 loopback、Host/Origin、token 可选模式。
5. 对打包进 zip 的 operator docs 再跑一次内容检查。

**预期收益**

消除上线操作误导，使实现、威胁模型、验收和发布包文档一致。

**验证方法**

```powershell
rtk grep -n "RELAYCHECK_BOOTSTRAP_PASSWORD|bootstrap-admin-password|Bootstrap login|RELAYCHECK_SMOKE_PASSWORD" README.md docs\OPERATOR_RUNBOOK.md docs\LAUNCH_READINESS.md docs\OPERATOR_ACCEPTANCE_RECORD.md scripts frontend
```

应无残留；默认启动和 token 模式的 operator acceptance 均应可按文档完成。

### CF-4（P2）本地与 CI 工具链版本未统一

**问题描述**

CI 使用 Node 22，当前本地为 Node 24.16.0；`package.json` 没有 `engines` 或 `.node-version`。本地 Go 为 1.26.4，模块声明 1.24，CI 又固定 1.22.x。

**影响评估**

不同开发机可能得到不同 lint、构建或 lockfile 行为，问题难以复现。

**推荐操作步骤**

1. 选定支持 Vite 8 的 Node 22 LTS 作为项目基线。
2. 增加 `.node-version`/`.nvmrc` 和 `package.json.engines`。
3. setup-node 读取同一版本文件。
4. Go CI 读取 `go.mod`；本地文档明确最低版本和推荐版本。

**预期收益**

提升构建可复现性，减少“只在某台机器失败”。

**验证方法**

干净环境按版本文件执行 `npm ci`、lint、test、build，产物 hash/尺寸应稳定在可接受范围。

### CF-5（P2）Go 漏洞扫描不确定且失败不阻断

**问题描述**

`verify-release.ps1` 在找不到本地工具时运行 `go run ...@latest`，依赖网络且版本不固定；扫描失败只输出 warning 后继续。当前机器未安装 `govulncheck`，本轮无法提供确定的 Go 漏洞扫描结果。

**影响评估**

- “release gate 通过”不一定意味着 Go 漏洞扫描成功。
- `@latest` 让同一提交在不同时间使用不同扫描器版本。
- 断网环境容易长期跳过而不被察觉。

**推荐操作步骤**

1. 固定一个经过验证的 govulncheck 版本，不使用 `@latest`。
2. 在线 CI 中扫描失败应失败；离线 release 必须显式 `-SkipGoVulnCheck` 并生成“未扫描”记录。
3. 保存扫描器版本、时间和输出摘要为 artifact。
4. 对 vendor 依赖同时保留 `go list -m`/SBOM 清单，便于后续追踪。

**预期收益**

让漏洞扫描结果可重复、可审计，避免网络问题被误认为安全通过。

**验证方法**

在线 job 能输出固定版本并成功扫描；断网且未显式 skip 时门禁失败；显式 skip 时发布记录标记风险。

### CF-6（P2）打包验证未进入自动化发布工作流

**问题描述**

项目已有成熟的 `package-release.ps1`、`verify-package.ps1`、manifest 和 SHA256，但当前 CI 只到前端 build，不验证最终 Windows zip、embedded UI 和 package-local scripts。

**影响评估**

源码门禁全绿仍不能证明最终交付包完整；打包路径、manifest、checksum 或嵌入资源问题可能只在手工发布时出现。

**推荐操作步骤**

1. PR CI 保持快速；新增 tag/manual release workflow。
2. 运行 verify-release、package-release、verify-package。
3. 对生成的 exe 做 `/api/health`、`GET /` 和静态 assets smoke。
4. 上传 zip、`.sha256`、manifest 和验证日志 artifact。
5. 只有该 workflow 全绿才允许标记正式 release。

**预期收益**

把“源码可构建”提升为“用户实际收到的包可启动、可验证、可追溯”。

**验证方法**

从 CI artifact 下载到干净 Windows 环境，`verify-package.ps1`、health、首页和关键导航 smoke 均通过。

---

## 5. 推荐落地顺序

| 阶段 | 建议项 | 目标 |
|---|---|---|
| S0：1-2 天 | BE-1、CF-1、CF-3 | 修复通知正确性、CI 版本/范围、运维文档误导 |
| S1：2-4 天 | FE-1、FE-2、CF-2、CF-5 | 让格式、覆盖率和安全扫描门禁真实可用 |
| S2：3-5 天 | FE-4、BE-2、BE-6 | 服务端分页、真实列表基准、消除静默截断 |
| S3：1-2 周 | FE-3、FE-5、FE-6、BE-3、BE-4 | 强化静态规则、数据所有权、错误与 SSRF 边界 |
| 持续优化 | FE-7、FE-8、BE-5、BE-7、AR-1 至 AR-4、CF-4、CF-6 | 固化架构、性能预算和可重复发布 |

建议每个阶段独立提交，格式化、依赖变更、行为修复和架构重构不要混在同一个大 diff 中。

---

## 6. 完成后的统一验证清单

```powershell
cd E:\zidqiandao\relaycheck-desktop

# 后端自有代码，避免 frontend/node_modules 污染
rtk go vet -mod=vendor ./ ./internal/...
rtk go test -mod=vendor -count=1 ./ ./internal/...
rtk go test -mod=vendor -cover -count=1 ./internal/...

cd frontend
rtk npm ci
rtk npm run format:check
rtk npm run lint -- --max-warnings=0
rtk npx tsc -b
rtk npm run test:coverage
rtk npm run build
rtk npm audit --audit-level=moderate

cd ..
rtk powershell -NoProfile -ExecutionPolicy Bypass -File scripts\verify-release.ps1
rtk powershell -NoProfile -ExecutionPolicy Bypass -File scripts\package-release.ps1
rtk powershell -NoProfile -ExecutionPolicy Bypass -File scripts\verify-package.ps1
rtk git diff --check
rtk git status --short --branch
```

### 验收标准

- 所有 P1 项有代码/配置/文档变更及对应测试。
- CI 与本地使用一致的 Go/Node 基线。
- `format:check` 和 `test:coverage` 可执行且可阻断。
- 账号和通知列表均有真实服务端分页、总数和边界测试。
- 500 响应不泄露底层错误细节，日志可用 requestId 关联。
- 运维文档不再出现不存在的 bootstrap 登录流程。
- 最终 zip 在干净 Windows 环境通过 package、health、embedded UI 验证。

---

## 7. 审查限制与剩余风险

- 本轮没有启动桌面 UI 做 Chrome runtime profiling，因此 bundle 基线已验证，LCP/INP/heap 属待执行项。
- Windows 当前未启用 cgo，`go test -race` 无法运行；并发安全主要依赖单测和代码审读。
- 当前机器未安装 govulncheck，且未为本次审查联网安装工具；Go 依赖漏洞状态不能声明为“已扫描无漏洞”。
- DNS rebinding 风险属于基于网络栈行为的已知 TOCTOU 推断，需用可注入 resolver/dialer 测试验证修复。
- 账号相关子查询已具备组合索引，是否达到瓶颈必须通过 5k/20k fixture 和 `EXPLAIN QUERY PLAN` 定量确认；本报告不把推断写成已发生的性能故障。
