# RelayCheck 提示词总清单

> 用途：把用户桌面提示词沉淀为可持续验收清单。后续每完成一项，就在这里把对应条目标记为 `[x]`，并在 `progress.md` 记录验证结果。
>
> 范围：正式版优先为 `relaycheck-desktop`；`newapi_signin` 为冻结遗留；`relaycheck-hub` 为实验性 MVP。

## 总原则

- [x] 确立单一正式版：以 `relaycheck-desktop` 为唯一正式版推进。
- [x] 冻结 `newapi_signin`，不删除旧数据库。
- [x] 标记 `relaycheck-hub` 为实验性 MVP。
- [x] 小步、可验证、不破坏现有数据。
- [x] 不新增明文凭据、硬编码真实账号、零鉴权端点。
- [x] 遵守 Control Room 视觉方向：冷静、精准、紧凑、可信、低噪声。
- [x] 390px 与常见桌面宽度无横向滚动。
- [x] 每个剩余改动都补齐“改前 / 改后 / 手测步骤”三元组。

## 第一层：产品战略与信息架构 P0

### T1.1 顶层治理

- [x] 根目录新建 `README.md`，说明三套代码库的角色、端口、数据文件、启动器引用、维护状态。
- [x] 合并重复启动器，只保留 `启动RelayCheck.bat` / `静默启动RelayCheck.vbs`。
- [x] 删除重复 `启动.bat` / `静默启动.vbs` 前比对内容。
- [x] 将孤儿 `run.py` 移入 `legacy/` 并标记 `DEPRECATED`。
- [x] `relaycheck-desktop/README.md` 增加架构与路由总览 mermaid 图。

### T1.2 产品定位与命名收敛

- [x] 正式版显示名收敛为 `RelayCheck Desktop v1.0`。
- [x] 设置页展示版本号。
- [x] 设置页展示构建时间。
- [x] 设置页展示数据文件路径。
- [x] 设置页展示调度器状态。
- [x] 设置页展示上次自检结果。

### T1.3 路线图与优先级

- [x] 根目录输出 `ROADMAP.md`。
- [x] 根目录输出 `OPTIMIZATION_PLAN.md`。
- [x] 将任务按 Now / Next / Later 组织。

## 第二层：核心功能闭环 P0/P1

### T2.1 安全基线 P0

- [x] 本地 API 加鉴权：主要 `/api/*` 业务路由强制 `requireSession`。
- [x] 登录、会话、登出、健康检查作为明确例外。
- [x] Host 头校验只允许 loopback 主机名与当前监听端口。
- [x] API 响应增加基础安全头。
- [x] 防止 DNS rebinding 的本地 Host 校验。
- [x] SSRF 防护：外部 URL 限定 `http/https`。
- [x] SSRF 防护：默认拒绝 `localhost` / loopback。
- [x] SSRF 防护：默认拒绝 private IP。
- [x] SSRF 防护：默认拒绝 link-local。
- [x] SSRF 防护：默认拒绝 metadata `169.254.169.254`。
- [x] SSRF 防护：默认拒绝 IPv6 loopback / 非全局地址。
- [x] 本地可信探测路径显式 opt-in 允许 loopback/private。
- [x] 已保存 secrets 继续加密落盘。
- [x] 凭据加密落盘做一次端到端复核并形成文档规范。
- [x] 导出时一律指纹化，永不导出明文密钥的规范补进用户可见文档。
- [x] 新增 `audit_log` 表。
- [x] 审计登录成功。
- [x] 审计登录失败。
- [x] 审计登出。
- [x] 审计设置变更。
- [x] 审计备份创建。
- [x] 审计备份删除。
- [x] 审计备份恢复。
- [x] 审计账号创建。
- [x] 审计账号更新。
- [x] 审计账号删除。
- [x] 审计上游站点删除。
- [x] 审计连接 / 断开浏览器授权。
- [x] 审计导入 / 导出。
- [x] 提供只读 `/api/system/audit-log`。
- [x] 设置页提供只读“审计日志”卡片。
- [x] URL 限定 `http/https` scheme。
- [x] 批量外部动作 limit 统一 clamp 到 `1..10`。
- [x] Admin API pageSize clamp 到 `10..100`。
- [x] 清理/避免源码中真实邮箱 `2174274760@qq.com`。
- [x] 清理/避免源码中真实密码 `xie123456789`。

### T2.2 可靠性基线 P0

- [x] SQLite 启用 WAL。
- [x] SQLite 设置 `busy_timeout=5000`。
- [x] SQLite 连接池已配置。
- [x] 将 SQLite 调优迁移/同步到 `relaycheck-hub`。
- [x] 将 SQLite 调优补到冻结 Python 版文档或冻结说明。
- [x] 签到网络类失败自动指数退避重试，上限 3 次。
- [x] 签到结果标注“已自动重试 N 次”。
- [x] 探测/批量识别有并发上限。
- [x] 批量检测 / 批量刷新 / 模型同步等批量入口有数量上限。
- [x] 每站点最小间隔限流。
- [x] 新增 `/api/health`。
- [x] `/api/health` 检查 DB。
- [x] `/api/health` 检查 scheduler。
- [x] `/api/health` 检查数据路径。
- [x] 请求 ID 通过 `x-request-id` 响应头返回。
- [x] 客户端传入安全 `x-request-id` 时沿用。
- [x] 非法 `x-request-id` 会被替换。
- [x] 结构化 HTTP 访问日志包含 requestId、method、path、status、statusClass、durationMs。
- [x] 结构化 HTTP 访问日志不记录请求体、Cookie、Authorization。
- [x] 所有内部错误分类为稳定 error class。
- [x] 替换遗留 Python `print()` 为结构化 logging 或在冻结说明中明确不再维护。
- [x] 替换吞掉的 `except Exception: pass` 或在冻结说明中列为遗留风险。

### T2.3 功能补全 P1

- [x] `relaycheck-hub/README.md` 顶部标记“MVP，签到/调度未实现”警告。
- [ ] `relaycheck-hub` 接入 `node-cron` 调度器。
- [ ] `relaycheck-hub` 签到页接成真实功能。
- [ ] `relaycheck-hub` 余额页接成真实功能。
- [ ] 每渠道独立调度。
- [ ] 调度日历预览。
- [ ] 下次运行列表。
- [ ] Webhook 失败专用 / 成功专用 / 汇总三种模式。
- [ ] Webhook HMAC 签名。
- [ ] Webhook 重试。
- [ ] Telegram 通知渠道。
- [ ] Bark 通知渠道。
- [ ] Server 酱通知渠道。
- [ ] 邮件通知渠道。
- [ ] 桌面通知渠道。
- [ ] 站内通知静音 / 已读 / 分级增强。
- [ ] 一键导出加密 zip：渠道 + 凭据 + 历史 + 设置。
- [ ] 一键导入加密 zip。
- [ ] Python `zidqiandao.db` 到 Go `relaycheck.db` 单向迁移器。
- [ ] 余额增长曲线。
- [ ] 按站点可靠性分析。
- [ ] 失败原因分布饼图。
- [ ] 响应时间分布。
- [ ] 余额增量日/周图。
- [ ] Dashboard 日期范围选择器。
- [ ] Dashboard 图表点击下钻。
- [ ] Cookie/storage_state 预计过期时间记录。
- [ ] Cookie 临近过期 Action Center 提醒。
- [ ] 2FA 签到登录明确指引。
- [ ] 批量执行 Dry-run 预览。

## 第三层：交互体验 P1

### T3.1 状态体系三件套

- [x] 表格 loading 骨架。
- [x] 面板 loading 骨架。
- [x] 图表 loading 骨架。
- [x] 空状态区分“从未添加过渠道”和“筛选无结果”。
- [x] Dashboard “待处理告警”接真实数据或移除假指标。
- [x] fetch 失败显示持久错误条。
- [x] 错误条提供重试按钮。
- [x] 新增全局 `ErrorBoundary`。

### T3.2 大列表性能与可用性

- [x] Channels 虚拟列表或加载更多。
- [x] History 虚拟列表或加载更多。
- [x] Notifications 虚拟列表或加载更多。
- [ ] 500+ 行不卡顿验证。
- [x] 余额列排序。
- [x] 响应时间列排序。
- [x] 最近签到列排序。
- [x] ID 列排序。
- [x] 渠道搜索覆盖 `base_url`。
- [x] 渠道搜索覆盖 `note`。
- [x] 渠道搜索覆盖 `platform`。
- [x] 渠道搜索覆盖凭据邮箱/用户名。
- [x] 历史搜索覆盖 `message`。

### T3.3 破坏性操作与流程安全

- [x] 删除凭据弹确认。
- [x] 删除渠道弹确认。
- [x] 清除覆盖弹确认。
- [x] 批量删除弹确认。
- [x] 高风险操作二次输入名称或显式勾选。
- [x] 提供撤销或软删除可恢复。
- [x] 模态 Esc 关闭。
- [x] 点击背景不自动取消进行中的浏览器授权。
- [x] 模态 `role="dialog"`。
- [x] 模态 `aria-modal="true"`。
- [x] 模态焦点陷阱。
- [x] 关闭后焦点归还触发元素。

### T3.4 批量与进度

- [x] 一键签到已有内存运行状态。
- [ ] 批量签到 SSE 或轮询逐条上报。
- [ ] 批量测试 SSE 或轮询逐条上报。
- [ ] 批量识别 SSE 或轮询逐条上报。
- [ ] 进度条不再 0→100 假跳。
- [ ] `current/total` 实时更新。
- [ ] 批量测试尊重设置的并发数。

### T3.5 引导与帮助

- [ ] 首次运行空数据分步向导。
- [ ] 引导连接 NewAPI。
- [ ] 引导导入渠道。
- [ ] 引导配置凭据。
- [ ] 引导试签到一次。
- [x] 能力 chip 常驻 tooltip 或图例弹窗。
- [x] 新增帮助/文档入口。

## 第四层：视觉美化与设计系统 P1/P2

### T4.1 主题系统

- [ ] 深色模式。
- [ ] `prefers-color-scheme`。
- [ ] 手动主题切换。
- [ ] 主题持久化。
- [ ] 所有颜色走 CSS 变量，逐步消除硬编码十六进制色。
- [ ] Dashboard 图表 tooltip/grid 随主题切换。
- [ ] 浅色界面禁止深色 tooltip 块。

### T4.2 设计系统收敛

- [x] 二选一并写入 `DESIGN_SYSTEM.md`：正式纳入 Tailwind/shadcn 或移除 Tailwind/shadcn 收尾层。
- [x] 清理另一方残留。
- [x] V4 token foundation + Tailwind `@theme` bridge first pass。
- [x] Active V4 hardcoded radius/type/status/shadow token sweep second pass。
- [ ] 颜色 token 单一来源。
- [ ] 圆角 token 单一来源。
- [ ] 阴影 token 单一来源。
- [ ] 间距 token 单一来源。
- [ ] 字号 token 单一来源。
- [ ] 抽出共享 `<StatCard>` 组件。

### T4.3 视觉细节

- [x] 去 emoji 化：状态文本改 lucide 图标 + 文字。
- [x] 所有可点击元素触摸目标 ≥44×44px。
- [x] Dashboard 图表网格改 `auto-fit/minmax`。
- [x] 表格列宽弹性。
- [x] 移动端单列。
- [x] 重复 keyframes 统一进全局 CSS。
- [x] `prefers-reduced-motion` 已有基础兼容。
- [x] 所有动画在 `prefers-reduced-motion` 下完整降级或关闭。

### T4.4 Control Room 信息密度

- [x] 重要数字更大、更易扫读。
- [x] 成功弱于失败。
- [x] 维护/破坏性操作次级化分组。
- [x] 逐页巡检并修正所有“状态只靠颜色”问题。
- [x] 逐页巡检并修正重要数字位置和等宽数字。

## 第五层：工程质量与架构 P1/P2

### T5.1 拆解巨型文件

- [ ] `newapi_signin/frontend/src/pages/Channels.jsx` 拆为子组件。（冻结遗留，不解冻）
- [ ] `newapi_signin/api.py` 拆为 routers + detection。（冻结遗留，不解冻）
- [x] `relaycheck-desktop/frontend/src/main.tsx` 拆分页面组件与 hooks：HubRadar、ChannelTable、ImportDialogs、useChannelActions、useChannelFilters 已抽出，Accounts 等页面逻辑保留内联。
- [x] 抽出 `ChannelTable` → `components/channels/ChannelTable.tsx`
- [ ] 抽出 `AuthModal` — 当前 main.tsx 无此组件；授权为账号卡内联按钮，不值得单独抽取。
- [ ] 抽出 `BrowserAuthPanel` — 当前 main.tsx 无此组件；浏览器授权为单按钮操作，不值得单独抽取。
- [x] 抽出 `ImportDialogs` → `components/import/ImportDialogs.tsx`
- [x] 抽出 `useChannelActions` → `hooks/useChannelActions.ts`
- [x] 抽出 `useChannelFilters` → `hooks/useChannelFilters.ts`

### T5.2 消除重复与提升健壮性

- [ ] 抽单一 `buildCredentialPayload()`。
- [ ] `Schedule.jsx` 不再绕过 `api.js` 直接用 `http`。
- [ ] 补齐 `scheduleApi`。
- [ ] `AppContext` value 用 `useMemo` 或拆分 context。
- [x] 修复/确认 `UnlockGate` 调用不存在的 `doUnlock`。
- [x] 修复/确认 `createAuthDraft` 死分支。
- [x] 修复/确认 `handleSaveBrowserAuth` 闭包捕获陈旧 `filteredIds`。
- [x] 修复/确认 `checkinApi.batch` body 结构不一致。
- [x] `.trim('|')` 改显式 `replace(/^\|/, '')`。

### T5.3 类型与校验

- [ ] 后端关键模型补完整类型注解/字段文档。
- [ ] 前端 TS 类型覆盖补齐。
- [ ] URL 语义校验完善。
- [ ] 邮箱语义校验完善。
- [ ] 数值范围语义校验完善。

### T5.4 测试与可观测性

- [ ] 检测引擎单元测试继续补充。
- [x] 签到临时失败重试与结果标注单元测试。
- [ ] 签到成功分类单元测试。
- [x] 并发测试或文档化 cgo/race 限制。
- [x] 结构化 HTTP 日志 + request ID。
- [x] `/api/health`。

## 第六层：运维与启动体验 P2

- [x] 单一可见启动器。
- [x] 单一静默启动器。
- [ ] 开机自启写入 `shell:startup` 快捷方式。
- [x] 已有系统自检诊断。
- [x] 首屏已有系统/诊断/建议信息。
- [x] 首屏徽章显示“系统健康：良好/N 项需关注”。
- [x] 点击健康徽章直达预筛选目标页。
- [ ] 版本检查。
- [ ] 一键更新提示。
- [ ] 启动前检测端口占用。
- [ ] 已运行时提示“是否打开窗口”。

## 执行约束与最终验收

- [x] 桌面版 `go test -mod=vendor ./...`。
- [x] 桌面版 `npm run build`。
- [x] 桌面版 `npm audit --audit-level=low` 0 漏洞。
- [x] 关键页 Playwright smoke：桌面 + 390px 无横向滚动、console 无错。
- [x] 遗留 Python `api.py` AST parse 通过。
- [ ] 遗留 Python 后端路由数检查。
- [ ] 遗留 Python DB init 幂等检查。
- [x] `relaycheck-hub npm run build`。
- [x] `relaycheck-hub prisma validate`。
- [ ] 每个主题更新对应 README / ROADMAP / DESIGN_SYSTEM / AGENT_HANDOFF。
- [ ] 最终交付“改前 vs 改后”关键截图或手测记录。

## 改动验收三元组

> 后续每个新阶段都必须追加一条：改前 / 改后 / 手测步骤。更详细验证结果继续写入 `progress.md`。

### 阶段 40：签到临时失败重试

- 改前：签到接口遇到临时网络错误或 5xx 时直接记录失败或切换候选接口，结果不显示重试次数。
- 改后：网络错误、HTTP 408/429/5xx 对同一候选接口最多 3 次尝试，并在结果中写入 `retryCount` 与“已自动重试 N 次”。
- 手测步骤：运行 `go test -mod=vendor ./internal/core -run "TestRunAccountCheckinRetriesTemporaryFailures|TestShouldRetryCheckinAttemptOnlyRetriesTemporaryFailures" -count=1 -v` 和 `go test -mod=vendor ./...`。

### 阶段 41：统一 API 错误分类

- 改前：API 错误响应只有中文 `error` 文案，前端或自动化无法稳定按错误类型处理。
- 改后：错误响应保留 `error`，并新增稳定 `errorClass`。
- 手测步骤：运行 `go test -mod=vendor ./internal/core -run "TestWriteErrorIncludesStableErrorClass|TestSecureLocalHandlerRejectsBadHostAndSetsHeaders|TestSecureLocalHandlerRequestIDAndAccessLog" -count=1 -v` 和 `go test -mod=vendor ./...`。

### 阶段 42：签到每站点最小间隔

- 改前：批量签到同一站点多个账号会连续发起请求。
- 改后：批量手动签到和自动签到共用 `siteMinIntervalSeconds`，默认同站点连续账号至少间隔 2 秒。
- 手测步骤：运行 `go test -mod=vendor ./internal/core -run "TestCheckinSiteLimiterComputesPerSiteDelay|TestLoadCheckinScheduleConfigClampsSiteMinInterval" -count=1 -v` 和 `go test -mod=vendor ./...`。

### 阶段 43：凭据加密与指纹化导出

- 改前：已有加密和导出预览能力，但缺少端到端测试和用户可见规范。
- 改后：测试确认凭据加密落盘、导出预览不泄漏明文；README 增加凭据与导出安全规范。
- 手测步骤：运行 `go test -mod=vendor ./internal/core -run TestCredentialsAreEncryptedAtRestAndExportsAreFingerprinted -count=1 -v` 和 `go test -mod=vendor ./...`。

### 阶段 44：浏览器授权与导入导出审计

- 改前：部分浏览器授权和导入/导出路径没有审计记录。
- 改后：浏览器授权打开/保存/断开、Key 导出预览和主要导入路径写入 `audit_log`，且 metadata 不含明文凭据。
- 手测步骤：运行审计定向测试和 `go test -mod=vendor ./...`。

### 阶段 45：冻结 Python 版说明

- 改前：Python 遗留版冻结边界没有独立文档，SQLite 调优和 logging 风险容易被误判为仍需回迁。
- 改后：`newapi_signin/DEPRECATED.md` 明确冻结边界和遗留风险，根 README 增加入口。
- 手测步骤：搜索确认 `DEPRECATED.md` 包含 SQLite、`print()`、`except Exception` 和正式版替代路径。

### 阶段 46：全局 API 错误条与 ErrorBoundary

- 改前：API 失败主要散落到局部文案，渲染异常可能导致白屏。
- 改后：全局持久错误条展示错误分类/接口/状态并提供安全重试，React ErrorBoundary 接管渲染异常。
- 手测步骤：运行 `npm run build` 和 `go test -mod=vendor ./...`。

### 阶段 48：渠道空状态区分复核

- 改前：主清单未标记渠道空状态区分是否完成。
- 改后：复核源码确认渠道页已区分“还没有渠道”和“没有匹配渠道”，并同步勾选清单。
- 手测步骤：搜索 `frontend/src/main.tsx` 中 `还没有渠道` / `没有匹配渠道` 两个 `EmptyState` 分支。

### 阶段 49：真实告警与搜索覆盖复核

- 改前：主清单未标记 Dashboard 告警、渠道 `base_url` 搜索、历史 `message` 搜索是否完成。
- 改后：复核源码确认 Dashboard 使用真实 `/api/system/action-center` 和 `/api/system/diagnostics`，渠道搜索包含 `baseUrl`，签到历史搜索包含 `message`。
- 手测步骤：搜索 `actionCenter` / `baseUrl` / `log.message` 在 `frontend/src/main.tsx` 中的使用。

### 阶段 50：渠道搜索覆盖备注与平台

- 改前：渠道搜索只覆盖名称、Base URL、状态和后台类型等字段，NewAPI 原始配置里的 `note` / `platform` 无法作为关键词命中。
- 改后：渠道搜索安全解析 `rawJson`，白名单提取 `note`、`remark`、`description`、`platform`、`provider`、`group`、`type` 等非敏感描述字段。
- 手测步骤：运行 `npm run build`，确认前端 TypeScript 和生产构建通过。

### 阶段 51：账号凭据与删除二次确认

- 改前：账号卡清空 API Key 和删除账号可直接提交；误点可能删除账号及其保存的密码、Cookie、Token、API Key 等凭据。
- 改后：清空 API Key 保存前要求确认；账号卡删除与“本地地址疑似误匹配”删除入口均要求确认后才调用删除接口。
- 手测步骤：运行 `npm run build`，确认前端 TypeScript 和生产构建通过。

### 阶段 52：设置页帮助入口与能力图例

- 改前：帮助文档入口散落在目录文件中，能力 chip 缩写如 NEW/ONE/SUB/MOD、raw_json/live 需要用户自行猜测。
- 改后：设置页新增“帮助 / 文档”卡片，列出 README、PROMPT_CHECKLIST、DESIGN_SYSTEM、AGENT_HANDOFF；新增常驻能力图例解释后台类型、Key、模型来源和价格相关状态。
- 手测步骤：运行 `npm run build`，确认前端 TypeScript 和生产构建通过。

### 阶段 53：清空已读通知确认

- 改前：通知页“清空已读”会直接删除已读通知历史，误点后无法恢复。
- 改后：“清空已读”先弹出确认，明确未读通知会保留、已读通知历史删除后无法恢复。
- 手测步骤：运行 `npm run build`，确认前端 TypeScript 和生产构建通过。

### 阶段 54：通用 loading 骨架与减弱动画补强

- 改前：部分页面加载时只有文字提示或直接显示空状态，表格/面板/图表型区域缺少统一骨架；骨架 shimmer 没有专门静态降级。
- 改后：新增通用 `LoadingSkeleton`，覆盖启动页、Dashboard 自检/任务中心、Hub Radar、渠道列表、通知列表和签到状态；`prefers-reduced-motion` 下骨架动画静态化。
- 手测步骤：运行 `npm run build`，确认前端 TypeScript 和生产构建通过。

### 阶段 55：渠道、签到历史、通知加载更多

- 改前：渠道、签到历史和通知列表会一次性渲染全部匹配项，数据较多时首屏 DOM 压力更高。
- 改后：渠道默认显示 24 条、签到历史默认 40 条、通知默认 30 条，筛选变化时重置显示数量，并提供“加载更多”按钮逐步展开。
- 手测步骤：运行 `npm run build`，确认前端 TypeScript 和生产构建通过。

### 阶段 56：首屏系统健康徽章可直达问题

- 改前：Dashboard 首屏健康 Badge 只展示高优先级/需关注/运行健康，点击无法直接进入对应问题。
- 改后：健康徽章显示“系统健康：良好 / N 项需关注”；有问题时点击跳转到最高优先级 Action Center 目标页并携带筛选意图，无问题时点击刷新自检。
- 手测步骤：运行 `npm run build`，确认前端 TypeScript 和生产构建通过。

### 阶段 57：账号页关键字段排序

- 改前：账号页只能搜索和按登录状态筛选，不能按余额、响应时间、最近签到或 ID 排序。
- 改后：账号页新增排序下拉，支持最近签到正/倒序、余额高低、响应时间快慢、ID 正/倒序；清空筛选会恢复默认最近签到优先。
- 手测步骤：运行 `npm run build`，确认前端 TypeScript 和生产构建通过。

### 阶段 58：遗留前端缺陷项源码复核

- 改前：主清单保留 `UnlockGate/doUnlock`、`createAuthDraft`、`handleSaveBrowserAuth/filteredIds`、`checkinApi.batch`、`.trim('|')` 等待确认项。
- 改后：复核正式版源码未发现这些符号或 `.trim('|')` 用法，判定为遗留/外部实现风险项，不适用于当前 `relaycheck-desktop` 正式版。
- 手测步骤：使用 `rg` 搜索上述符号和固定字符串；当前仅命中主清单自身记录，源码未命中。

### 阶段 59：渠道搜索覆盖账号邮箱与用户名

- 改前：渠道搜索可命中渠道名称、Base URL、状态、后台类型、备注和平台字段，但不能通过绑定账号的邮箱/用户名找到相关渠道。
- 改后：渠道页额外读取账号摘要，按站点 Base URL/站点名关联渠道，只把账号显示名、邮箱、用户名加入搜索索引，不读取密码、Cookie、Token 或 API Key 明文。
- 手测步骤：运行 `npm run build`，确认前端 TypeScript 和生产构建通过。

### 阶段 60：cgo/race 限制文档化

- 改前：清单要求并发测试或文档化 cgo/race 限制，但 README 未明确当前 Windows Go 环境下 race detector 的阻塞原因。
- 改后：README 新增 Race / cgo Note，说明当前环境未启用 cgo，`go test -race ./internal/core` 会因 `-race requires cgo` 阻塞；本地必跑回归门槛仍是 `go test -mod=vendor ./...`。
- 手测步骤：阅读 `README.md` 的 Race / cgo Note，并运行 `go test -mod=vendor ./...`。

### 阶段 61：渠道归档确认

- 改前：渠道卡的“归档保留”可直接执行，虽然不物理删除数据，但会让渠道从日常视图隐藏，误点仍可能造成困惑。
- 改后：单个渠道归档前弹出确认，明确不会删除账号、余额或签到日志，只会从日常视图隐藏；取消时不发起请求。
- 手测步骤：运行 `npm run build`，确认前端 TypeScript 和生产构建通过；源码复核 `updateChannelSourceStatus` 中归档动作先调用 `window.confirm`。

### 阶段 62：批量删除确认复核

- 改前：主清单未确认批量删除入口是否都需要二次确认。
- 改后：复核正式版源码，真实批量删除入口为设置页“删除选中”备份，`deleteSelectedBackups` 已先弹确认；批量渠道操作为归档/恢复状态切换，不物理删除，并已有确认。
- 手测步骤：源码复核 `frontend/src/main.tsx` 中 `deleteSelectedBackups`、`bulkUpdateSourceStatus` 和 Action Center `archive-missing-channels` 均先调用 `window.confirm`。

### 阶段 63：渠道软删除可恢复复核

- 改前：主清单未确认高频渠道清理是否具备撤销或软删除路径。
- 改后：复核确认渠道清理采用 `missing -> archived -> active` 状态流转；归档不会删除账号、余额或签到日志，用户可筛选“已归档”并恢复活跃。
- 手测步骤：源码复核 `source_sync_status`、`archive-source-status`、`restore-source-status`、`恢复全部归档` 和归档提示文案；账号删除仍按物理删除处理并保留二次确认，不纳入本阶段软删除范围。

### 阶段 64：详情抽屉模态可访问性

- 改前：详情抽屉没有统一 `role="dialog"` / `aria-modal`，Esc 关闭、焦点陷阱和关闭后焦点归还缺失。
- 改后：新增 `useDialogBehavior`，详情抽屉打开后聚焦内部元素，Tab 在抽屉内循环，Esc 可关闭，卸载时焦点归还触发元素；站点/账号/渠道详情抽屉补齐 `role="dialog"`、`aria-modal="true"` 和可读标签。
- 手测步骤：运行 `npm run build`；源码复核 `DetailDrawer` / `SiteDetailDrawer` 使用 `useDialogBehavior` 并包含 `role="dialog"`、`aria-modal="true"`。

### 阶段 65：浏览器授权背景点击复核

- 改前：主清单未确认背景点击是否会取消正在进行的网页登录授权。
- 改后：复核确认网页登录授权没有可点击背景取消路径；前端背景点击只关闭详情抽屉 UI，不调用保存、断开或删除授权；后端 `browserSessions` 只在保存授权或断开授权时移除。
- 手测步骤：源码复核 `open-browser-login`、`finish-browser-login`、`browserSessions` 删除点和 `drawer-backdrop` 点击处理。

### 阶段 66：触屏目标与 Dashboard 自适应网格

- 改前：Dashboard 主图表/诊断网格仍有固定 2/3 列；触屏移动端存在为桌面密度压到 28-36px 的按钮规则。
- 改后：Dashboard 主图表、诊断网格和 Hub Radar 均使用 `auto-fit/minmax`；粗指针设备下按钮和按钮型控件统一至少 44x44px，桌面鼠标密度保持不变。
- 手测步骤：运行 `npm run build`，确认前端 TypeScript 和生产构建通过；源码复核 `frontend/src/styles.css` 的 `.dashboard-main-grid`、`.dashboard-diagnostics-grid` 和 `@media (pointer: coarse)`。

### 阶段 67：移动端主要内容单列保护

- 改前：多个后追加的 V4/Control Room CSS 覆盖层仍保留多列网格，移动端单列依赖分散的局部媒体查询。
- 改后：CSS 末尾新增 `@media (max-width: 760px)` 主要内容单列保护层，覆盖 Dashboard、Hub Radar、诊断、渠道、账号、余额、设置、调度和详情抽屉等内容网格；移动端导航继续保留横向紧凑条。
- 手测步骤：运行 `npm run build`，确认前端 TypeScript 和生产构建通过；源码复核 `frontend/src/styles.css` 末尾移动端单列覆盖层。

### 阶段 68：表格感行列宽弹性保护

- 改前：正式版前端没有原生 `<table>`，但详情、通知、审计、备份、同步结果、签到日志等 grid 行承担表格式信息密度；长 URL、文件名或摘要仍可能撑开列。
- 改后：为表格感 grid 行统一补 `min-width: 0` / `max-width: 100%`，子项可收缩，长文本允许 `overflow-wrap: anywhere`；移动端这些行统一单列。
- 手测步骤：运行 `npm run build`，确认前端 TypeScript 和生产构建通过；源码复核 `frontend/src/main.tsx` 无 `<table>`，以及 `frontend/src/styles.css` 末尾弹性行保护层。

### 阶段 69：重复 keyframes 收敛

- 改前：`panel-in` 与 `skeletonShimmer` 两个动画关键帧分散在 CSS 不同功能段，后续维护时容易重复追加或漏看全局动画。
- 改后：`panel-in` 与 `skeletonShimmer` 集中放在全局 keyframes 区域，动画引用名称与行为保持不变。
- 手测步骤：运行 `npm run build`，确认前端 TypeScript 和生产构建通过；源码搜索 `@keyframes` 确认 keyframes 集中在 CSS 前部。

### 阶段 70：非 emoji 线性图标与状态文字

- 改前：正式版源码未发现明显 emoji 图标残留，但导航仍使用 `OV/CH/...` 字母缩写，关键状态 Badge 只有文字，未形成“图标 + 文字”的一致状态语言。
- 改后：新增轻量内联 `LineIcon` 线性 SVG 图标组件，不新增 `lucide-react` 依赖；导航改为对象线性图标，Dashboard 健康、自检和任务中心 Badge 改为线性状态图标 + 中文状态文字。
- 手测步骤：运行 `npm run build`，确认前端 TypeScript 和生产构建通过；源码复核 `navItems`、`LineIcon`、`StatusLabel` 和关键 `Badge` 调用。

### 阶段 71：状态不只靠颜色巡检与修正

- 改前：渠道源端状态、账号登录态、调度任务、审计日志、同步结果、代理/同步开关等状态位虽然有中文文本，但视觉层仍主要依赖 `status-*` / `level-*` 色彩类来区分成功、警告和失败。
- 改后：扩展 `StatusLabel` 状态映射，并把上述高频状态位改为“线性状态图标 + 中文状态文字”；同步成功/失败结果显式写入状态文案，避免仅靠绿色/红色判断。
- 手测步骤：运行 `npm run build`，确认前端 TypeScript 和生产构建通过；源码复核 `statusIconName`、`StatusLabel`、`status-pill`、账号/渠道/调度/审计/同步结果调用。

### 阶段 72：重要数字位置与等宽数字巡检

- 改前：Dashboard 顶部指标已有等宽数字，但 Hub Radar、渠道/账号卡片、签到/通知计数、能力卡、同步结果、详情指标等数字样式分散，部分重要数字未统一启用 `tabular-nums`。
- 改后：在 CSS finishing layer 新增集中数字扫描覆盖层，为主要指标数字统一启用 `font-variant-numeric: tabular-nums` 和 `font-feature-settings: "tnum" 1, "lnum" 1`，并为高优先级数字轻微收紧字距和靠前对齐。
- 手测步骤：运行 `npm run build`，确认前端 TypeScript 和生产构建通过；源码复核 `Numeric scan pass` 覆盖 Dashboard、Radar、渠道、账号、签到、通知、余额、详情、同步和调度指标。

### 阶段 73：Tailwind/shadcn 设计系统收敛

- 改前：正式版已经使用 Tailwind v4 CSS import 和 `@tailwindcss/vite`，但 `DESIGN_SYSTEM.md` 仍笼统写着不新增 Tailwind/Radix/shadcn，CSS 注释里也残留 `shadcn-inspired` / `shadcn/Linear` 表述。
- 改后：`DESIGN_SYSTEM.md` 明确正式保留 Tailwind v4 作为构建期 CSS 层，不新增 Radix/shadcn 运行时依赖；CSS 注释改为 Control Room / Linear control-room 表述，清掉 shadcn 实现残留。
- 手测步骤：运行 `npm run build`，确认前端 TypeScript 和生产构建通过；源码复核 `package.json` 保留 Tailwind v4，`styles.css` 不再出现 `shadcn` 注释残留。

### 阶段 74：V4 token foundation 与 Tailwind bridge 第一批

- 改前：Tailwind `@theme` 仍直接写入部分硬编码颜色/圆角/阴影，V4 活跃层只有少量 `--v4-*` 变量，状态背景、常用字号和部分圆角仍散落在覆盖层里。
- 改后：`@theme` 桥接到 V4 token；`:root` 补齐颜色、状态背景、输入、骨架、字号、字重、字距、间距、圆角和阴影 token；活跃 V4 层的侧边栏、摘要、指标、状态 pill、工具条等第一批样式改用 token。
- 手测步骤：运行 `npm run build`，确认前端 TypeScript 和生产构建通过；下一轮继续收敛剩余历史层硬编码，不提前勾选完整“单一来源”大项。

### 阶段 75：Active V4 token sweep 第二批

- 改前：V4 活跃层仍有导航激活色、移动密度覆盖、全局错误条、fatal error 卡和 JSON preview 的硬编码字号、圆角、状态色与阴影。
- 改后：补充 amber 语义文字 token，并将上述区域继续改为 `--v4-*` 字号、圆角、阴影和语义色引用。
- 手测步骤：运行 `npm run build`，确认前端 TypeScript 和生产构建通过；继续保留完整“单一来源”大项未完成，等待历史层清理和全量硬编码扫描。

### 阶段 76：relaycheck-hub SQLite 调优同步

- 改前：`relaycheck-hub` schema 初始化只设置 WAL/foreign_keys，Prisma adapter 未显式同步 `busy_timeout=5000`，外部 SQLite 导入也未统一锁等待策略。
- 改后：新增 `src/lib/sqlite-tuning.ts` 作为 hub SQLite 调优入口，主库统一应用 WAL、busy_timeout、synchronous、temp_store、cache_size、foreign_keys；Prisma adapter 显式传入 `timeout: 5000` 并保持进程级 singleton；外部 NewAPI SQLite 只读导入仅继承 5000ms timeout，不强改用户外部库。
- 手测步骤：在 `relaycheck-hub` 运行 `npm install`、`npm run verify:sqlite`、`npm run build` 和 `npx prisma validate`。

### 阶段 77：目标提示词清单与组件基础设施

- 改前：`目标/` 目录已有两份新提示词，但没有对应可打勾清单；正式前端 `cn.ts` 仍是简单 join，缺少 `@/*` alias，UI 原子组件只有 Button/Card/Badge。
- 改后：新增 `目标/TARGET_PROMPT_CHECKLIST.md`，将两份提示词拆成可持续验收清单；`cn.ts` 升级为 `clsx + tailwind-merge`；`tsconfig` 和 `vite` 配置 `@/*` alias；新增 `Input`、`Select`、`Skeleton`、`Dialog` 四个高优先级 UI primitives，并更新 `DESIGN_SYSTEM.md`。
- 手测步骤：在 `relaycheck-desktop/frontend` 运行 `npm install clsx tailwind-merge` 和 `npm run build`；构建通过后在 `目标/TARGET_PROMPT_CHECKLIST.md` 勾选 A1 与 A2 高优先级项。

### 阶段 78：目标提示词 A2 UI 原子组件补齐

- 改前：`目标/TARGET_PROMPT_CHECKLIST.md` 的 A2 仍剩 `Progress`、`Tooltip`、`Switch` 未完成，UI primitive 底座不完整。
- 改后：新增 `Progress`、`Tooltip`、`Switch` 三个轻量组件，均使用 `cn()` 和项目 Tailwind/V4 token，且不新增 UI 依赖；A2 UI 原子组件项全部勾选完成。
- 手测步骤：在 `relaycheck-desktop/frontend` 运行 `npm run build`，确认 TypeScript 与 Vite 生产构建通过。

### 阶段 79：目标提示词 A3 类型定义抽离

- 改前：`frontend/src/main.tsx` 顶部内嵌大量 DTO、导航、API 错误和 UI 图标类型，A3 的类型抽离项未完成。
- 改后：新增 `frontend/src/types/index.ts` 集中导出 65 个类型；`main.tsx` 使用 `import type { ... } from "@/types"`，并用 `satisfies readonly NavItem[]` 校验导航项结构；未改变业务逻辑或样式。
- 手测步骤：在 `relaycheck-desktop/frontend` 运行 `npm run build`，确认 TypeScript 与 Vite 生产构建通过。

### 阶段 80：目标提示词 A3 API client 抽离

- 改前：`frontend/src/main.tsx` 仍内嵌 `ApiError`、`api()`、读缓存和全局 API 错误发布逻辑，页面组件与请求基础设施耦合。
- 改后：新增 `frontend/src/api/client.ts`，集中管理 API 请求、读缓存、错误订阅和错误发布；`main.tsx` 只从 `@/api/client` 导入 `api` 与 `subscribeApiErrors`，原请求行为保持不变。
- 手测步骤：在 `relaycheck-desktop/frontend` 运行 `npm run build`，确认 TypeScript 与 Vite 生产构建通过；用源码搜索确认 `main.tsx` 不再包含本地 `ApiError` / `api()` / `clientReadCache`。

### 阶段 81：目标提示词 A3 format 工具抽离

- 改前：`frontend/src/main.tsx` 仍内嵌时间、时长、字节、JSON 预览、余额、紧凑数字和价格比较等格式化函数。
- 改后：新增 `frontend/src/lib/format.ts` 集中导出通用格式化工具；`main.tsx` 从 `@/lib/format` 导入使用；依赖 label 的 `formatAPIKeyTestMessage` 暂留主文件，等待 labels 抽离。
- 手测步骤：在 `relaycheck-desktop/frontend` 运行 `npm run build`，确认 TypeScript 与 Vite 生产构建通过；源码搜索确认 `main.tsx` 不再包含通用 `format*` / `trimDecimal` / `compactJSONPreview` 定义。

### 阶段 82：目标提示词 A3 labels 工具抽离

- 改前：`frontend/src/main.tsx` 仍内嵌大量纯标签映射函数，`formatAPIKeyTestMessage` 也因依赖 `apiKeyStatusLabel` 暂留主文件。
- 改后：新增 `frontend/src/lib/labels.ts` 集中导出错误分类、诊断等级、渠道/同步/审计/调度/签到/登录态/API Key/价格等标签工具，并迁移 `formatAPIKeyTestMessage`；导航意图和快捷动作等行为逻辑仍保留在 `main.tsx`。
- 手测步骤：在 `relaycheck-desktop/frontend` 运行 `npm run build`，确认 TypeScript 与 Vite 生产构建通过；源码搜索确认目标 label 函数只在 `lib/labels.ts` 定义。

### 阶段 83：目标提示词 A3 constants 工具抽离

- 改前：`frontend/src/main.tsx` 仍内嵌导航元信息、状态集合、渠道搜索白名单、通知关键词、Dialog focus selector 和列表加载更多阈值等静态配置。
- 改后：新增 `frontend/src/lib/constants.ts` 集中导出上述静态配置；`main.tsx` 从 `@/lib/constants` 引用，页面状态和业务流程函数仍留在主文件等待后续页面拆分。
- 手测步骤：在 `relaycheck-desktop/frontend` 运行 `npm run build`，确认 TypeScript 与 Vite 生产构建通过；源码搜索确认本地旧常量和魔法数字不再残留。
