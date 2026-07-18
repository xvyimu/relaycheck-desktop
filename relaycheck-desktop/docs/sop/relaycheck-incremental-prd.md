# RelayCheck Desktop 增量 PRD（推进专项）

- **项目名称：** `relaycheck_desktop_incremental_advance`
- **文档日期：** 2026-07-18
- **语言：** 中文
- **现有技术栈：** Go `net/http` + React 19 + TypeScript + Vite 8 + Tailwind CSS v4/项目自有 UI primitives + SQLite
- **原始需求复述：** 深度研究 `relaycheck-desktop`，不做无依据的大重构，直接筛选并推进一组最小风险、可验证的高优先级增量改进。
- **本轮边界：** 本文仅定义增量需求和候选实施切片；不修改业务源码，不把历史文档声明等同于本次重新验证结果。

## 1. 产品定义与目标

### 1.1 产品定位

RelayCheck Desktop 是面向可信本机操作者的 **NewAPI / OneAPI / Sub2API 及兼容中转站本地运维控制台**。它以单 Go 二进制启动 loopback HTTP 服务，将 React UI 和 SQLite 嵌入本地运行，聚合渠道、站点、账号、签到、余额/用量、模型、调度、通知、诊断、备份和本机 NewAPI 同步。

它不是公开 SaaS、营销站或多租户管理后台；默认是无独立管理员登录的可信单用户模式，共享主机场景可选择启用每进程 HttpOnly session token。

### 1.2 主要用户角色

1. **日常操作者（主角色）：** 管理中转站资产、处理登录态过期、执行/观察签到和同步任务、排查异常。
2. **部署/交付操作者：** 在目标 Windows 机器验证包、备份数据、启动、首小时监控、签署验收结论。
3. **维护开发者（次角色）：** 通过 Go/Vitest/Playwright/CI/release gate 小步演进并保持安全、性能和兼容性。

### 1.3 本轮产品目标

1. **闭合首次接入与安全执行路径：** 让新操作者从“连接/扫描 → 渠道 → 凭据 → 预览 → 签到”获得一致、可恢复的指引，减少误操作。
2. **消除高频界面语义偏差：** 统一已合并的信息架构和操作文案，确保入口名称、按钮行为、状态反馈与真实产品结构一致。
3. **建立下一阶段可量化基线：** 用最小侵入方式补齐启动/关键 API 性能证据，为后续是否优化提供数据，而非提前重构。

## 2. 证据与现状摘要

### 2.1 读取证据

| 证据路径 | 证据结论 |
|---|---|
| `README.md:1-7, 35-37, 57-67, 118-135` | 明确产品定位、单二进制/loopback/SQLite、可信单用户访问模型及核心 API 面。 |
| `DESIGN_SYSTEM.md:3-10, 21-27, 44-64` | UI 定位是低噪音 Control Room；状态不能只靠颜色；390px 无横向溢出；不新增 Radix/shadcn。 |
| `main.go:24-25, 48-88, 115-132` | 前端嵌入二进制；创建 App、单实例锁、注册路由、启动调度、端口回退并自动打开浏览器；数据根默认随可执行文件。 |
| `go.mod:1-10`、`frontend/package.json:1-49` | Go 1.25 module，SQLite/cron/websocket；React 19/Vite 8/Tailwind v4/Vitest/Playwright，已有 format/lint/build/budget/test 门禁。 |
| `internal/core/app.go:34-99, 130-270` | App 是 assembly root 且被冻结；已有领域服务、任务、调度、通知、备份、安全等真实实现，不宜再做 god-object 式大重构。 |
| `internal/core/routes.go:19-108` | 系统、Dashboard 聚合、任务、调度、渠道、站点、账号分页/搜索、签到、模型、通知、本机 NewAPI API 均已注册。 |
| `frontend/src/main.tsx:21-35, 41-55, 92-173, 176-264` | 7 个主导航面板；非 Dashboard 懒加载；已访问面板 keep-alive/空闲回收；数据按 4 组 hook 获取。 |
| `frontend/src/components/layout/Sidebar.tsx:1-44` | 当前 IA 为“仪表盘/渠道/站点与账号/签到/本机扫描/通知/设置”，账号已并入“站点与账号”。 |
| `frontend/src/components/dashboard/Dashboard.tsx:81-106, 157-260` | Dashboard 已按 Radar→运营待办→资产摘要→折叠次要区→按需分析组织，核心是 action-first 运维决策。 |
| `frontend/src/components/onboarding/OnboardingWizard.tsx:8-49, 174-221, 323-337` | 已有 4 步向导和真实 API 动作，但步骤 3仍显示“左侧账号页”旧文案；最后一步直接启动签到，没有 dry-run。 |
| `frontend/src/components/checkins/CheckinsPanel.tsx:201-217` | “执行全部签到”直接启动任务，UI 未调用已存在的 dry-run API。 |
| `internal/core/dry_run.go:11-17, 34-168` | 后端已实现最多 200 账号的签到/测试/识别预览，但前端源码未发现 `/api/tasks/dry-run` 调用，能力尚未形成用户闭环。 |
| `frontend/src/components/scan/ScanPanel.tsx:36-57, 97-147` | 扫描/导入完成后只刷新和展示结果，没有跳转“渠道/站点与账号”的下一步动作。 |
| `HANDOFF.md:10-17, 19-32, 101-105, 179-199` | 大部分工程 P1-P3 已完成；仅明确留下外部 RUM/启动 waterfall/API p95，且禁止重做已完成项、禁止自动登录/绕过 2FA。 |
| `docs/full-stack-code-review-optimization-2026-07-17.md:122-140, 1156-1167` | 本轮已完成安全、性能查询、拆分、CI/release gate；唯一无法在仓内闭环的是代表性 RUM/线上 p95；下一步建议补交互分支覆盖。 |
| `.github/workflows/ci.yml:19-139` | CI 已覆盖 Go test/vet/vuln/core 55%、前端 format/lint/type/test/coverage/build/budget/audit，以及 nightly 5k dataset benchmark。 |
| `docs/LAUNCH_READINESS.md:42-91`、`docs/OPERATOR_ACCEPTANCE_RECORD_DRAFT_2026-07-06.md:1-19, 58-109` | 发布脚本与回滚链齐全，但仓内验收草稿陈旧且仍为 Hold/Pending；真实目标机和人工批准不应由代码代理代签。 |
| `docs/OPERATOR_SESSION_EXPIRY_RUNBOOK.md:10-17, 38-66, 83-99` | 会话恢复必须由操作者在浏览器手工登录；工具只负责打开、保存、验证和批量编排，不得绕过 2FA/CAPTCHA。 |

### 2.2 核心能力与主要流程

**核心能力**

- 资产接入：扫描本机 NewAPI SQLite、Admin API 导入、渠道/站点识别与同步。
- 账号运营：账号分页/搜索、凭据/API Key、浏览器手工登录与批量重登、登录态验证。
- 自动化执行：签到、余额刷新、API Key 测试、站点识别、统一任务进度/SSE/取消、按站点调度。
- 运营决策：Dashboard Radar、Action Center、诊断、通知、分析、余额/用量/模型视图。
- 本地安全与恢复：loopback/Origin/RemoteAddr 防护、可选 session token、凭据加密、SSRF 防护、加密备份导入导出、审计。
- 工程交付：前后端测试、smoke、包体预算、漏洞扫描、release/package/operator scripts。

**主要用户流程**

1. 启动单二进制 → 健康/系统状态 → 首次向导。
2. 连接远端 NewAPI 或扫描本机数据库 → 导入渠道/站点 → 识别/模型同步。
3. 在“站点与账号”配置凭据/API Key或手工浏览器授权 → 测试登录态。
4. 预览待执行对象 → 执行签到/余额/测试任务 → 查看实时进度和结果。
5. Dashboard/通知/诊断发现风险 → 深链到渠道、站点与账号、签到页处理。
6. 配置调度/通知/代理/备份 → 长期本机运行；发布者通过 package/launch/monitor/acceptance 完成交付。

### 2.3 当前完成度判断

| 维度 | 判断 | 依据 |
|---|---|---|
| 产品功能 | **高（约 85%-90%）** | 核心资产、账号、签到、调度、诊断、备份、通知、分析和同步均有前后端实现。百分比是产品判断，不是代码统计。 |
| 工程质量 | **高** | 最新交接记录声明 1085 Go tests、前端 309 tests、coverage floor、全门禁和 package verifier 已通过；当前任务未重新执行全量门禁。 |
| 核心操作闭环 | **中高** | Action Center/重登/任务进度已闭环；dry-run 能力未进入实际签到 UI，扫描/向导下一步仍有断点。 |
| 信息架构一致性 | **中高** | 账号页已合并，但向导仍有一处旧“左侧账号页”文案。 |
| 发布/真实运营验证 | **中** | 工程包验证已完成；真实目标机人工验收、代表性 RUM/API p95 仍依赖外部环境。 |

**结论：** 项目已不是“从 0 到 1”或“大重构”阶段，而是 **上线/长期运行前的闭环校准与证据补齐阶段**。本轮应优先修正确性明确、范围小、能由现有测试验证的界面闭环；不重复已完成的架构拆分、查询收敛、安全加固和 CSS 大整理。

## 3. 用户故事

1. **As a 首次使用的操作者, I want 按当前信息架构完成连接、渠道、凭据、预览和首次签到, so that 我无需猜测入口且不会未经确认直接批量执行。**
2. **As a 日常操作者, I want 在执行全部签到前看到会执行/跳过的账号及原因, so that 我能发现过期凭据或不支持站点并避免意外任务。**
3. **As a 完成本机扫描的操作者, I want 从扫描结果直接前往渠道或站点与账号, so that 我能顺着结果完成下一步配置。**
4. **As a 维护开发者, I want 关键引导/扫描/签到交互有行为测试, so that 后续优化不会让入口、API 或状态反馈悄然回退。**
5. **As a 产品/维护负责人, I want 获取真实启动与关键 API 延迟基线, so that 下一轮性能投入由数据驱动而不是主观猜测。**

## 4. 增量需求池与验收标准

### P0（Must have）

#### P0-1：高风险批量签到前置 dry-run 预览

**需求**

- “执行全部签到”在真正启动前必须先获取候选账号并调用现有 `/api/tasks/dry-run`，展示 `willRun/skipped/items/reason`。
- 用户必须在预览对话框中二次确认后才调用现有 task start；取消不得产生任务。
- 预览失败必须保留页面状态并显示稳定错误，不得降级为静默直接执行。
- 单账号“签到”可维持直接操作；本需求仅收口高影响批量入口。

**验收标准**

1. 点击“执行全部签到”后，网络顺序为预览请求 → 用户确认 → task start；未确认时 task start 请求数为 0。
2. UI 显示总数、将执行数、跳过数，以及前若干条跳过原因；大于展示上限时明确“另有 N 条”。
3. `willRun=0` 时确认按钮禁用，并提供处理凭据/能力的下一步建议。
4. 预览接口错误时显示 `role=alert`，任务不启动；重试成功后可继续。
5. 组件/交互测试覆盖确认、取消、0 可执行、错误四个分支；现有签到任务进度与取消行为不回退。

#### P0-2：修复首次向导的信息架构与执行语义

**需求**

- 将向导步骤 3所有“左侧账号页”旧文案统一为“站点与账号 → 全部账号”。
- 最后一步不得把“首次验证”描述成无预览的直接批量签到；应复用 P0-1 预览，或清楚引导用户进入签到页完成预览确认。
- 向导的“完成”必须表示引导结束，不得暗示所有账号已验证成功。

**验收标准**

1. 源码和渲染结果中不再出现“左侧账号页”旧入口；与 Sidebar 的“站点与账号”一致。
2. 第 4 步未完成预览/确认时，不直接启动签到任务。
3. 成功、失败、跳过、返回上一步/重开引导的状态不会串步残留。
4. 新增行为测试覆盖 4 步切换、连接请求体不泄露到 DOM、凭据步骤文案、最终步不越权启动。
5. 保持“手工登录/不绕过 2FA/CAPTCHA”的既有硬边界。

### P1（Should have）

#### P1-1：扫描/导入结果提供明确下一步导航

**需求**

- ScanPanel 成功后提供“查看渠道”和“前往站点与账号”按钮；部分失败时保留成功结果和修复建议。
- 通过现有 `NavigationIntent`/`onNavigate` 传递导航，不引入新路由框架。

**验收标准**

1. `found=true` 且至少一项成功时出现两个下一步按钮；点击分别切到正确面板。
2. 全失败时不展示误导性的成功导航，仅展示权限/文件完整性/重试建议。
3. 成功+失败混合结果同时表达“已导入”和“需处理”，状态不只靠颜色。
4. 390px 和桌面宽度无横向溢出；交互测试覆盖成功、混合、失败、导航。

#### P1-2：首次接入进度来源统一

**需求**

- 以已有系统摘要/Action Center setup 分类为事实来源，形成轻量 `setupProgress` mapper，供 Dashboard、Onboarding、ScanPanel 复用。
- 进度至少覆盖：已连接实例、已导入渠道、已配置账号、已完成首次安全验证（dry-run 或任务结果，具体事实口径待确认）。
- 不新增数据库表，不引入工作流引擎。

**验收标准**

1. 相同数据状态在 Dashboard、向导、扫描页显示同一步骤和下一动作，不互相矛盾。
2. 空库、仅实例、已有渠道、已有账号四组 fixture 下 mapper 输出稳定且有单元测试。
3. setup warning 与真实故障在文案/状态上明确区分。
4. 已完成用户可重开向导，但不被强制弹窗。

#### P1-3：交互覆盖的最小增量提升

**需求**

- 优先补 Onboarding、ScanPanel、CheckinsPanel 的真实点击/API/失败恢复测试，而不是仅静态 markup helper 测试。
- 不以追求总体百分比为唯一目标，覆盖 P0/P1 新行为和高风险回归点。

**验收标准**

1. 每个目标组件至少覆盖 1 条成功路径、1 条错误路径、1 条用户取消/跳过路径。
2. 测试断言真实 API URL、method 和关键非秘密字段；不记录 token/cookie/password。
3. `npm test`、coverage floor、lint、typecheck 全绿；不得降低现有阈值。

### P2（Nice to have）

#### P2-1：真实性能测量方案与外部基线

**需求**

- 先定义、后采集：启动到 Dashboard 可操作时间、`/api/system/status`、`/api/dashboard/inventory`、`/api/dashboard/ops`、`/api/dashboard/model-usage` 的 p50/p95、失败率和样本环境。
- 优先复用后端已有 `http_request` 结构化日志中的 `durationMs/requestId/path/status`；前端仅在明确批准后加入不含业务数据的本地性能标记。
- 数据只保存在本地/CI artifact，不新增遥测上报服务。

**验收标准**

1. 文档记录机器、数据规模、冷/热启动定义、样本数，避免把开发机单次结果称为生产结论。
2. 至少采集 30 次冷启动或等价代表性样本，以及每个关键 API ≥100 次样本（若环境不允许，应记录阻塞而非伪造数据）。
3. 形成基线表和阈值建议；未达到阈值才进入下一轮性能优化。
4. 任何采集内容不得包含 URL token、凭据、账号名、绝对 profile 路径或数据库内容。

#### P2-2：发布/验收文档现状刷新

**需求**

- 将 2026-07-05/06 的 Launch Readiness 与 acceptance draft 同最新 HANDOFF/release verifier 状态对齐；历史记录必须标注历史，不覆盖真实签署记录。

**验收标准**

1. 工具链、测试数、包 SHA/commit 等动态数据只引用可验证产物，不手填虚构值。
2. 保留“目标机、备份、人工结论、签名”必须由操作者确认。
3. 文档不再出现与当前无登录单用户模型冲突的“Admin sign-in/bootstrap password”未解释字段。

## 5. 本轮推荐可落地候选

### 推荐实施组合（按顺序）

| 顺序 | 候选 | 优先级 | 预计范围 | 风险 | 验证方式 |
|---|---|---:|---|---|---|
| 1 | 修复 Onboarding 旧入口文案并补行为测试 | P0-2 子切片 | `OnboardingWizard.tsx` + tests | 极低 | Vitest；字符串/点击/API 契约 |
| 2 | 批量签到 dry-run 确认闭环 | P0-1 | `CheckinsPanel.tsx`、可能新增轻量预览组件 + tests；复用现有 API | 低-中 | Vitest + 现有 Go dry-run tests + smoke |
| 3 | ScanPanel 成功后的下一步导航 | P1-1 | `main.tsx` prop、`ScanPanel.tsx` + tests | 低 | Vitest + navigation smoke + 390px layout |
| 4 | setupProgress 纯 mapper | P1-2 的最小切片 | `frontend/src/lib/*` + tests，先不改后端/DB | 低 | 4 组 fixture 单元测试 |
| 5 | 性能采集 runbook/脚本方案 | P2-1 | 文档/已有日志分析，不改业务行为 | 低 | 可重复采样；无秘密扫描 |

### 为什么推荐此组合

- **证据明确：** 旧文案和 dry-run UI 缺失可由源码直接证明；不是主观“觉得应该重构”。
- **复用现有能力：** 后端 dry-run、NavigationIntent、Action Center/setup、结构化 HTTP 日志都已存在。
- **可独立回滚：** 每项可以单独提交、测试、回退，不触碰 SQLite schema、加密格式、scheduler 或领域服务边界。
- **用户价值直接：** 优先减少误执行和首次接入断点，再采集性能数据。

### 本轮不建议直接做

- 重写 App/领域包、换前端 query 框架、引入路由库、微服务化、数据库迁移。
- 再做一次全站 CSS/视觉重构。
- 未有真实 p95 证据前引入物化投影、缓存层或大规模 SQL 重写。
- 自动填写密码、自动通过 2FA/CAPTCHA、无人工确认的自动签到。

## 6. UI / 交互关注点

1. **控制室优先：** 预览必须紧凑展示“将执行/跳过/原因”，不做营销式大卡片。
2. **主次动作：** “确认执行”是明确主动作；“取消/返回修复”次级；破坏性或批量动作不得与普通查看同权重。
3. **状态可理解：** 成功/警告/失败同时使用文本、图标/标签和颜色，不能只靠红黄绿。
4. **错误可恢复：** API 失败后保留已输入/筛选/当前步骤，给出重试或去修复凭据的路径。
5. **敏感信息：** Access Token 输入保持 password 类型；反馈不得回显 token/cookie/password/API Key。
6. **响应式与可访问性：** 390px 单列、44×44 触控目标、可见 focus、DialogShell 焦点陷阱、`aria-live`、`prefers-reduced-motion`。
7. **一致命名：** 全局统一“站点与账号”“全部账号”“本机扫描”“执行全部签到”，避免历史“账号页”等幽灵入口。

## 7. 非功能需求

### 安全

- 必须保持 loopback/Host/Origin/RemoteAddr/session token 现有防护。
- 不得把凭据、Cookie、API Key、token、绝对浏览器 profile 路径写入 UI、日志、测试 fixture、文档或性能样本。
- 不改变 RCZIP2/RCZIP1、实例密钥、SSRF pin 和导入路径白名单契约。

### 性能

- P0/P1 不得增加 Dashboard 首屏固定请求数；向导/扫描/预览请求仅在用户操作时触发。
- dry-run 单次继续遵守后端 200 账号上限；UI 对超限给出明确提示。
- 不以同步加载 Analytics 或未访问面板换取实现便利。

### 可靠性

- 批量任务启动必须幂等地受用户确认控制；双击/忙碌态不得重复启动。
- 请求失败不得自动降级为危险操作。
- 保留已有任务进度、取消、完成后刷新语义。

### 兼容性与可维护性

- 保持 Go + embedded React + SQLite 和项目自有 UI primitives；不新增大型运行时依赖。
- 保持已有 API 兼容路径；新增前端闭环优先复用已有 endpoint。
- 任何业务源码变更需通过：Go tests/vet（若涉及后端）、frontend format/lint/typecheck/test/coverage/build/budget，以及相关 smoke。

### 可观测性

- 复用 `requestId/path/status/durationMs` 结构化日志；新增日志必须裁剪敏感字段。
- 性能结论必须区分本地 synthetic、CI benchmark、真实目标机/RUM。

## 8. 范围外事项

- 多用户/RBAC/云端 SaaS 化。
- 微服务化、替换 SQLite、引入消息队列或通用工作流引擎。
- 自动登录、保存/填写明文密码、绕过 2FA/CAPTCHA/TOTP。
- macOS/Linux 安装器、Windows 代码签名和自动升级（除非明确转向多机分发）。
- 解冻旧 Python/Next.js/relaycheck-hub 工作区。
- 在本轮替换现有设计系统、Radix/shadcn 或全站路由架构。
- 由开发代理代替操作者签署生产验收或编造真实 p95/RUM 数据。

## 9. 风险与缓解

| 风险 | 等级 | 缓解/回滚 |
|---|---|---|
| dry-run 与实际任务候选集合口径不同 | 中 | 先审计 task start 候选查询；测试相同 fixture；若无法完全一致，UI 明示“预估”并阻止 0 可执行启动。 |
| 在 Onboarding 重复实现一套签到预览 | 中 | 抽轻量共享 service/hook 或引导到签到页；不复制后端规则。 |
| 预览增加一步使熟练用户效率下降 | 低 | 仅约束“全部签到”；单账号操作不变；预览对话框支持键盘确认但不默认越过。 |
| ScanPanel 改 prop 影响懒加载面板 | 低 | 复用现有 `handleNavigate`；增加组件和 navigation smoke。 |
| setupProgress 事实口径不清导致误报完成 | 中 | 先用纯 mapper和明确已有字段；“首次安全验证”的事实源未确认前不落库。 |
| 性能采集本身泄露业务数据 | 中 | 只采 request path/status/duration/requestId；不采 query/body/实体名；本地保存。 |
| 陈旧文档被误认为当前发布状态 | 中 | 所有动态数字标注日期/来源；最新 HANDOFF优先；人工验收继续 Pending 直到真实签署。 |
| 工作区存在未提交改动，误覆盖他人工作 | 高 | 实施前先查看 status/diff；只改分配文件；小提交；不得清理 `data/`、`vendor/`、`frontend/dist/`。 |

## 10. 待确认问题

1. “执行全部签到”是否要求 **每次** dry-run 确认，还是可提供仅本机记忆的“本次会话内不再提示”？推荐每次确认，避免跨数据状态误执行。
2. dry-run 的账号集合应取“今日 dueAccounts”、全部可签到账号，还是当前筛选结果？推荐与实际 task start 完全同源。
3. Onboarding 第 4 步应内嵌预览，还是导航到签到页完成？推荐导航/复用共享预览，避免两套逻辑。
4. setupProgress 的“首次验证完成”应以 dry-run 成功、首次 task done，还是至少一条签到成功为准？需要产品负责人确认。
5. 扫描后“查看渠道”是否需要自动加 `sourceStatus=not_archived` 或仅打开默认渠道页？推荐先默认页，避免过度筛选隐藏导入结果。
6. 性能基线的代表环境是什么：开发机、目标运营机，还是两者都要？真实目标应以运营机为准，开发机仅作回归参考。
7. 真实目标机发布是否已发生？若没有，P2-1 只能先交付采集方案，不能声称获得 RUM/线上 p95。
8. 是否明确授权刷新 `LAUNCH_READINESS`/acceptance draft？文档存在旧数字和“Admin sign-in/bootstrap password”字段，与当前访问模型可能冲突。

## 11. 建议实施与验收顺序

1. **文案/行为契约小修：** P0-2 入口文案 + Onboarding 行为测试。
2. **安全执行闭环：** P0-1 dry-run 预览；先核对实际 task candidate 口径。
3. **接入下一步：** P1-1 ScanPanel 导航。
4. **一致性收敛：** P1-2 纯 mapper，先不改数据库。
5. **外部证据：** P2-1 在目标环境采样；数据未证明问题前不做性能重构。
6. 每个切片独立运行相关门禁并可独立回滚；任何失败不得以跳过测试或放宽安全边界解决。
