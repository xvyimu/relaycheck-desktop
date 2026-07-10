# RelayCheck Desktop 全域优化方案

日期：2026-07-06  
状态：工程优化项已执行，发布验收仍等待人工批准  
范围：`E:\zidqiandao\relaycheck-desktop`  
依据：`README.md`、`docs/PROJECT_STRUCTURE.md`、`DESIGN_SYSTEM.md`、`docs/adr/*`、`HANDOFF.md`、`GOALS.md`、本地运行监控记录与代码热点扫描

## 执行状态快照

更新时间：2026-07-06

| 方向 | 状态 | 已完成内容 | 验证 / 证据 |
|---|---|---|---|
| A1 发布验收闭环 | 人工阻塞 | 本地 package、launch、monitor 记录已存在；最终发布仍需要操作者确认目标机器、备份状态、验收结论和批准记录 | 人工项不能由 Codex 代签 |
| A2 前端资源查询 Module | 已完成 | 删除宽 `useAppData`；新增 `useSystemOverview`、`useInventoryData`、`useOpsHealth`、`useModelUsageOverview`；App/Dashboard props 改为资源组；修复带 `AbortSignal` 的 GET 请求被读缓存复用导致 StrictMode 下误报 abort 的问题；审查后补强刷新语义：失败保留旧数据、全局刷新包含 model/pricing/usage、首屏 loading 不在后续刷新时重现 | `npm test` 220 passed；`npm run build` passed |
| A3 后端高频 Module 深化 | 已完成 | 新增 scheduler tick/status registry；补 ADR-005；补 `docs/A3_BACKEND_MODULE_AUDIT_2026-07-06.md`；确认 accounts/checkin/scheduler 后续切片 | focused scheduler tests 12 passed；`internal/core` coverage 53.7% |
| A4 CSS 与设计系统分层 | 已完成 | `frontend/src/styles.css` 变为入口 import；规则按 tokens/base/layout/components/domains/layers 拆为 24 个文件，保持原级联顺序；后续瘦身移除未引用 UI primitive、未挂载 `ImportDialogs`、`smoke.mjs` 薄封装后 CSS 输出为 186.51 kB | `npm test` 220 passed；`npm run build` passed；`npm run smoke` 9 PASS；`npm run smoke:schedules` 1440/390 passed |
| A5 诊断与 Onboarding 打通 | 已完成 | Action Center 增加 setup 类下一步：扫描 NewAPI → 导入渠道 → 添加账号 → dry-run；Dashboard 展示接入类行动 | `TestActionCenter*` 与 `go test ./internal/core` passed |
| 最终工程验证 | 已完成 | Go、前端、构建、smoke、diff 检查均已执行；发布验收人工项单独保留 | 见下方“最终验证记录” |

剩余不可自动完成项：目标机器确认、生产数据备份确认、操作者人工验收记录、最终批准结论。

### 最终验证记录

- `rtk go test -mod=vendor -count=1 ./...`: 991 passed in 12 packages。
- `rtk go vet -mod=vendor ./...`: no issues。
- `cd frontend; rtk npm test`: 17 files / 220 tests passed。
- `cd frontend; rtk npm run build`: passed。
- `cd frontend; rtk npm run smoke:schedules`: 1440x900 与 390x900 均 passed，无 body/row overflow，无 console issue。
- `rtk go build -mod=vendor -o dist\relaycheck.exe .`: success。
- `rtk git diff --check`: clean。

## 0. 待补充信息清单

以下信息不足会影响优先级，但不阻塞初版方案：

| 问题 | 需要谁确认 | 影响 |
|---|---|---|
| 目标使用场景是个人本机长期运行，还是分发给多个可信操作者 | 项目负责人 / 操作者 | 决定是否优先做代码签名、安装器、迁移兼容和运维手册 |
| 首批真实 NewAPI / OneAPI / Sub2API 实例数量与数据规模 | 操作者 | 决定性能预算、分页策略、批量任务并发上限 |
| 是否必须保留无登录本地单用户模式 | 项目负责人 | 决定认证模块是否继续保持 passthrough，还是引入本地口令锁 |
| 视觉优化目标偏“控制台密集操作”还是“轻量新手引导” | 产品 / 设计 | 决定 Dashboard 和 Onboarding 的信息密度 |
| 是否计划发布到非开发机 | 项目负责人 | 决定 launch-ready 的最后一公里：代码签名、安装包、数据迁移、备份策略 |

## 1. 合理假设

- RelayCheck Desktop 的核心定位是本地运维控制台，不是公开 SaaS。
- 当前优先级应从“继续堆功能”转为“完成可持续运行、可维护、可交接的稳定产品”。
- 不新增大型运行时依赖；现有技术栈保持 Go `net/http`、React 19、Vite、SQLite、Tailwind v4 CSS layer。
- 后续实现仍按现有节奏：小切片、先测试、再实现、最后复查。

## 2. 项目目标设定

### 2.1 业务目标

| 目标 | 成功指标 |
|---|---|
| 从开发可用推进到操作者可长期使用 | 真实目标机器完成启动、备份、首小时监控、人工验收记录 |
| 降低操作者接入 NewAPI/OneAPI/Sub2API 的成本 | 首次导入路径有明确空状态、引导、错误恢复；关键路径一次完成率可人工验收 |
| 提升作为 AI API Hub 控制台的可信度 | Dashboard 能清楚呈现资产、Key、成本、调度、风险 |

### 2.2 用户目标

| 目标 | 成功指标 |
|---|---|
| 用户知道下一步该做什么 | 空数据状态下 Action Center / Onboarding 给出 1-3 个明确行动 |
| 用户能快速定位问题 | 账号失效、渠道识别失败、调度异常、余额风险可一键跳转到处理页 |
| 用户不需要理解内部实现也能完成发布操作 | package 内 operator docs、launch、monitor、acceptance record 可闭环 |

### 2.3 技术目标

| 目标 | 成功指标 |
|---|---|
| 加深高频 Module，减少宽 Interface | `useAppData` 拆成资源组 hook；`accounts.go` / `checkin_balance.go` / `scheduler.go` 继续收窄为调度或 HTTP 适配层 |
| 强化测试表面 | 高风险 Module 新增接口级测试；核心包覆盖率从 44.1% 提升到 50%+ |
| 降低 CSS 维护成本 | `frontend/src/styles.css` 从 9709 行单文件演进为 token/base/layout/domain 分层 |

### 2.4 设计目标

| 目标 | 成功指标 |
|---|---|
| 保持 Control Room 风格，不变成营销页 | Dashboard、Settings、Accounts 都是紧凑、清晰、低噪音的操作界面 |
| 统一状态表达 | 高频状态统一使用 `StatusLabel` / `Badge`，颜色必须配文字 |
| 移动端不溢出 | 390px 宽度 smoke 覆盖关键页，无横向滚动和文本重叠 |

### 2.5 执行目标

| 目标 | 成功指标 |
|---|---|
| 每个优化方向可被 Claude Code 或 Codex 接手 | 每项任务有文件范围、验收标准、回滚方式 |
| 文档先行，代码后置 | 大方向先写 ADR 或补充现有 ADR，再执行实现 |
| 发布状态可审计 | launch-records 保留本地记录，不提交运行产物；最终人工批准单独记录 |

## 3. 现状诊断

### 3.1 当前强项

| 维度 | 表现 |
|---|---|
| 架构 | 已完成 `*App` god object 的两阶段拆分，存在 8 个 `internal/<domain>` 包和多个 core 内服务/store |
| 发布 | 已有 `verify-release.ps1`、`package-release.ps1`、`verify-package.ps1`、`operator-launch.ps1`、`operator-monitor.ps1` |
| 安全 | 凭据加密、SSRF 规则、zip-bomb 保护、导出脱敏、HTTP 安全测试已经存在 |
| 测试 | Go / Frontend / Smoke / Package verification 均已有明确命令；前端已有 Vitest 与 Playwright smoke |
| 设计 | `DESIGN_SYSTEM.md` 明确 Control Room 方向，已有本地 UI primitives |

### 3.2 当前发布状态

| 项目 | 结论 |
|---|---|
| 最终包 | 历史包已在瘦身清理中移除；最终交付前需重新运行 `scripts\package-release.ps1` |
| Git commit | `338870bc315456b231194043abb045a15937c996` |
| Package dirty | `false` |
| 本机启动 | 端口 `3001`，`relaycheck.exe` PID `33900`，启动验收通过 |
| 首小时监控 | `operator-monitor-local-operation-3001-rerun.md` 结果 `pass` |
| 监控警告 | diagnostics overall 为 warning，原因是空数据状态：未记录 NewAPI 实例、未导入渠道、未添加账号 |
| 仍缺 | 目标机器确认、生产数据备份确认、人工验收记录、操作者最终批准 |

### 3.3 主要问题清单（实施前诊断）

| 优先级 | 维度 | 问题表现 | 原因 | 影响 |
|---|---|---|---|---|
| P0 | 发布闭环 | 本地首小时监控已通过，但没有人工验收记录和最终批准 | 当前仍是开发机本地运行 | 不能宣称正式交付完成 |
| P1 | 前端架构 | `useAppData` 一次拉 11 个资源，App 向 Dashboard 传递大量 props | 查询 Interface 偏宽，页面组合和数据获取耦合 | 加新面板时加载状态、错误处理、刷新策略容易扩散 |
| P1 | 后端架构 | `accounts.go` 825 行、`checkin_balance.go` 837 行、`scheduler.go` 733 行 | 仍有 HTTP handler、业务 orchestration、兼容 forwarder 混在同一文件 | 变更 Locality 不够集中，复查成本高 |
| P1 | 视觉系统 | `styles.css` 9709 行，存在 V4 token 与旧样式层并存 | 历史 CSS 持续叠加，未按 token/base/layout/domain 分层 | 小 UI 改动可能产生跨页回归，移动端验证成本高 |
| P1 | 体验 | 空数据 warning 已能诊断，但首次接入路径仍分散在扫描、导入、账号、渠道页 | Action Center、Onboarding、Dashboard 尚未形成一个闭环向导 | 新用户可能不知道先扫实例、再导入渠道、再添加账号 |
| P2 | 架构文档 | ADR 已覆盖任务、调度、查询、global schedule，但缺少“下一阶段优先级总线” | 多轮优化形成局部成果，缺少统一路线图 | 交接时容易重复讨论方向 |
| P2 | 质量度量 | coverage 目标局部完成，但 core/accounts/channels/notifications 仍有缺口 | DB/HTTP Infra mock 还不够统一 | 深层业务回归主要靠全量测试，定位慢 |
| P3 | 分发能力 | 未见代码签名、安装器、自动升级闭环 | 当前定位仍是可信本地 operator 包 | 多机器分发时会遇到信任、路径、备份、升级问题 |

## 4. 架构优化方案

### A1. 发布验收闭环优先完成

| 项 | 内容 |
|---|---|
| 目标 | 把“本地可运行”升级为“可被操作者确认接收” |
| 范围 | `docs/OPERATOR_ACCEPTANCE_RECORD.md`、`docs/LAUNCH_READINESS.md`、package `launch-records` |
| 方案 | 复制或填写验收记录，记录目标机器、工作目录、备份状态、启动记录、监控记录、人工结论 |
| 收益 | 发布状态从推测变成可审计事实 |
| 风险 | 误把开发机运行当生产验收 |
| 验收 | 人工批准记录存在，且明确“通过 / 暂缓 / 拒绝” |

### A2. 前端资源查询 Module 加深

| 项 | 内容 |
|---|---|
| 目标 | 缩窄 `useAppData` Interface，让页面只消费对应资源组 |
| 实施前 | `useSchedulerPreview` 已是好的起点，`useAppData` 曾集中拉取 status/channels/sites/accounts/checkins/notifications/diagnostics/actionCenter/model/pricing/usage |
| 实施后 | `useAppData` 已删除；App 使用 `useSystemOverview`、`useInventoryData`、`useOpsHealth`、`useModelUsageOverview` 四个资源组，并保留全局刷新语义 |
| 方案 | 按资源组拆成 `useSystemOverview`、`useInventoryData`、`useOpsHealth`、`useModelUsageOverview`，App 只持有导航状态 |
| 收益 | Dashboard、Accounts、Sites、Settings 的加载错误有 Locality；后续刷新更精确 |
| 风险 | 过早抽象成通用 query 框架会变浅 |
| 验收 | Dashboard props 数量下降；每个资源 hook 有测试或至少纯 mapper 测试；不新增 React Query/SWR |

### A3. 后端高频业务 Module 继续收窄

| 文件 | 优化方向 | 推荐动作 |
|---|---|---|
| `internal/core/accounts.go` | 保留 HTTP handler + compatibility wrapper，继续把创建、更新、清理、登录、验证路径下沉到已有 service | 优先检查剩余业务函数是否已有 service 可承接 |
| `internal/core/checkin_balance.go` | 让 `CheckinExecutor`、`BalanceRefresher`、`CheckinBatchOrchestrator` 成为主要 Interface | 将剩余 schedule/status/query helper 按读模型与执行模型拆分 |
| `internal/core/scheduler.go` | 分离 tick orchestration、plan persistence、job execution adapter | 先写 ADR：scheduler job registry 是否值得引入 |
| `internal/core/models_pricing.go` | 模型、价格、Key export 仍耦合 | 拆出 `ModelPricingService` 前先补测试，避免只搬文件 |

### A4. CSS 与设计系统分层

| 层 | 内容 | 目标 |
|---|---|---|
| `styles/tokens.css` | V4 token、dark mode token、semantic color | 单一 token 来源 |
| `styles/base.css` | reset、body、button/input 基础语义 | 控制全局副作用 |
| `styles/layout.css` | shell、sidebar、topbar、grid | 稳定页面骨架 |
| `styles/components.css` | card、badge、status、dialog、empty、progress | 复用 UI primitives |
| `styles/domains/*.css` | accounts/sites/channels/settings/dashboard | 领域样式可局部修改 |

验收标准：拆分后 `npm run build`、`npm test`、`npm run smoke:schedules`、390px 关键页截图或 smoke 通过；样式行为不变。

### A5. 诊断与 Onboarding 打通

| 项 | 内容 |
|---|---|
| 目标 | 把当前 diagnostics warning 变成清晰的新手路径 |
| 方案 | Action Center 对空数据状态输出“下一步”：扫描本机 NewAPI → 导入渠道 → 添加账号 → 执行 dry-run |
| 前端 | Dashboard / Onboarding / ScanPanel 共用一个 `setupProgress` mapper |
| 后端 | `/api/system/action-center` 可保留，但增加 setup 类 action 的稳定分类 |
| 验收 | 空 DB 首次打开时用户 30 秒内能知道下一步；warning 不再让人误判为系统故障 |

## 5. 体验优化方案

### 5.1 核心用户路径

| 路径 | 当前风险 | 优化动作 |
|---|---|---|
| 首次启动 | 空数据 warning 容易被理解为故障 | Dashboard 顶部显示“待接入数据源”，把 warning 分类为 setup |
| 扫描后台 | 扫描、保存、导入、识别分布在多个页面 | ScanPanel 输出下一步按钮：导入 channels / 前往账号 |
| 导入渠道 | 成功后还要理解识别与站点关系 | 导入结果页给出“识别未知渠道”入口 |
| 添加账号 | 登录方式复杂：密码、API Key、浏览器授权 | Account form 按站点能力显示推荐登录方式 |
| 调度运行 | 日历与 next-runs 已有预览，但失败信息可更明确 | `useSchedulerPreview` 暴露 error 给 Dashboard 可见区域 |
| 发布操作 | 文档和脚本齐全，但人工记录还未闭环 | 把 operator acceptance record 填写作为正式发布 gate |

### 5.2 信息架构建议

- Dashboard：只放风险、资产、调度、成本、下一步；减少重复卡片。
- Scan：聚焦“发现实例”和“导入渠道”两件事。
- Accounts：聚焦登录态、Key 可用性、余额与签到能力。
- Settings：聚焦系统设置、调度、备份、审计，不承载太多运营数据。
- Notifications：默认收起 success/info，突出失败、warning、需要处理的通知。

## 6. 视觉设计优化方案

### 6.1 当前视觉判断

当前方向正确：控制台、低噪音、密集信息、状态明确。主要问题不是风格错误，而是 CSS 历史层叠太多，导致设计规则难以稳定执行。

### 6.2 视觉规范建议

| 项 | 建议 |
|---|---|
| 色彩 | 保持 blue/teal 主色，green/amber/red 作为状态色；避免页面被单一蓝色填满 |
| 字体 | 数字统一 `tabular-nums`；卡片内标题不要使用 hero 级字号 |
| 圆角 | 常规卡片 8px；大型 shell 区域可略大，但避免层层卡片嵌套 |
| 图标 | 保持 line-icon 或本地图标，不引入新图标依赖，禁止 emoji icon |
| 动效 | 仅保留轻量 hover/加载动画，遵守 `prefers-reduced-motion` |
| 空状态 | 空数据不是错误；使用 setup 类型文案和明确行动按钮 |

## 7. 执行计划

### 阶段 1：发布收口（P0，0.5 天）

| 编号 | 任务 | 角色 | 交付物 | 验收 |
|---|---|---|---|---|
| T001 | 确认目标机器、工作目录、数据备份状态 | Operator | 验收记录 | 有明确目标和备份结论 |
| T002 | 填写 operator acceptance record | Operator / Engineer | `OPERATOR_ACCEPTANCE_RECORD` 副本 | 包含 launch record、monitor record、人工结论 |
| T003 | 决定是否停止当前开发机进程 | Engineer | 运行状态说明 | 不遗留未知进程 |

### 阶段 2：体验闭环（P1，1-2 天）

| 编号 | 任务 | 角色 | 交付物 | 验收 |
|---|---|---|---|---|
| T101 | 定义 setup progress 数据模型 | Product / Engineer | ADR 或 spec | 覆盖 NewAPI、channels、accounts、dry-run |
| T102 | Dashboard 显示 setup 下一步 | Frontend | UI + 测试 | 空 DB 不再只显示 warning |
| T103 | ScanPanel 导入后提供下一步导航 | Frontend | UI + smoke | 扫描到导入路径闭环 |

### 阶段 3：前端查询 Module（P1，1-2 天）

| 编号 | 任务 | 角色 | 交付物 | 验收 |
|---|---|---|---|---|
| T201 | 拆 `useSystemOverview` / `useOpsHealth` | Frontend | hooks + tests | Dashboard props 明显减少 |
| T202 | 将 model/pricing/usage 合并为资源组 hook | Frontend | `useModelUsageOverview` | 加载/错误状态局部化 |
| T203 | 保留 `useApi<T>`，不新增依赖 | Frontend | 代码约束 | package.json 无新增 query 库 |

### 阶段 4：后端高频 Module 加深（P1/P2，2-4 天）

| 编号 | 任务 | 角色 | 交付物 | 验收 |
|---|---|---|---|---|
| T301 | 审计 `accounts.go` 剩余业务函数 | Backend | 候选清单 | 标注可移动到已有 service 的函数 |
| T302 | 为 scheduler job registry 写 ADR | Architect | ADR | 决定是否引入 registry Interface |
| T303 | 补 core/accounts/channels Infra mock 测试 | Backend | 测试 | core 覆盖率 50%+，关键路径测试可读 |

### 阶段 5：CSS 分层（P2，1-2 天）

| 编号 | 任务 | 角色 | 交付物 | 验收 |
|---|---|---|---|---|
| T401 | 建立 CSS 分层文件并保持 import 顺序 | Frontend | `styles/*.css` | 构建和 smoke 通过 |
| T402 | 迁移 token/base/layout 第一批规则 | Frontend | 小 diff | 不改变视觉快照 |
| T403 | Dashboard / Settings 域样式分离 | Frontend | domain css | 390px 无横向溢出 |

## 8. 风险评估

| 风险 | 等级 | 预防 | 回滚 |
|---|---|---|---|
| 把本地开发机监控误认为生产验收 | 高 | 验收记录必须写明机器、目录、备份、操作者 | 标记为 local-only，不发布 |
| CSS 分层引入视觉回归 | 中 | 小批迁移，每批跑 build/smoke，必要时截图 | 单独 revert CSS 分层 commit |
| 前端 hook 拆分导致刷新语义变化 | 中 | 保持 `reload` 行为，先补 mapper/hook 测试 | 回滚到 `useAppData` 聚合 |
| 后端 Module 迁移改变业务行为 | 中 | 先测试后移动，保留 thin forwarder | 回滚单个 service extraction |
| 引入过多抽象变浅 | 中 | 通过 deletion test：删除 Module 后复杂性是否会在 callers 复现 | 拒绝该抽象或合并回原处 |
| 真实数据迁移风险 | 高 | 操作前备份 `data\relaycheck.db` 和 `data\keys` | 停进程、恢复备份、重新 health check |

## 9. 验收标准

### 9.1 发布验收

- package verification 通过。
- `operator-launch.ps1` 记录存在且 health ok。
- `operator-monitor.ps1` 首小时记录 result 为 pass。
- diagnostics warning 若存在，必须分类为 accepted warning 或 setup warning。
- 操作者最终批准记录存在。

### 9.2 架构验收

- 新 Module 的 Interface 比原调用面更小，且后方承载真实行为。
- 业务测试围绕 Interface，而不是只测 implementation 细节。
- `internal/core/PACKAGE_INDEX.md` 与实际结构同步。
- 新 ADR 记录高影响架构决策。

### 9.3 体验验收

- 空 DB 首次打开有明确下一步。
- 账号、渠道、站点、调度问题可从 Dashboard 一步跳转。
- 批量任务必须有进度、失败摘要、可恢复建议。

### 9.4 视觉验收

- 390px 和 1440px 下关键页无横向滚动。
- 状态不只靠颜色表达。
- 按钮、卡片、表单、badge 使用统一 tokens。
- CSS 分层后没有重复定义造成的覆盖混乱。

### 9.5 质量验收

- `go test -mod=vendor -count=1 ./...`
- `go vet -mod=vendor ./...`
- `cd frontend; npm test`
- `cd frontend; npm run build`
- `cd frontend; npm run smoke:schedules`
- 发布前运行 `scripts\verify-release.ps1`

## 10. 下一步行动建议

推荐按以下顺序推进：

1. **先完成发布验收闭环**：目标机器、备份、人工验收记录、最终批准。
2. **写 setup progress 小 spec 或 ADR**：把 diagnostics warning 与 onboarding/action-center 统一。
3. **继续一个 P1 小切片**：优先补 Dashboard 空状态体验闭环，或为资源组 hook 增加更多组件级回归测试。
4. **再做后端 Module 加深**：优先选择已有测试基础的 accounts/checkin/scheduler 子方向。
5. **最后做 CSS 分层**：只做机械拆分，不顺手改视觉；每批都跑 smoke。

推荐首个实现方向：**A5 诊断与 Onboarding 打通**。它直接解决当前本地监控里的 warning 解释问题，用户价值高，技术范围小，风险低，并能为后续真实操作者验收提供更好的第一屏体验。
