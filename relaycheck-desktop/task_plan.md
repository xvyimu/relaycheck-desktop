# 任务计划：RelayCheck Hub 持续优化升级

## 目标
把 RelayCheck Hub 打磨成轻量、稳定、可解释、适合个人长期管理 NewAPI / OneAPI / Sub2API 中转站的本地工具。

## 当前阶段
阶段 84（准备中）

## 各阶段

### 阶段 37：P0 顶层治理与正式版命名收敛
- [x] 根目录新增 `README.md`，说明三套实现的角色、端口、数据文件、启动器引用和维护状态
- [x] 根目录新增 `OPTIMIZATION_PLAN.md`，把提示词拆成 P0/P1/P2 可验证任务清单
- [x] 根目录新增 `ROADMAP.md`，按 Now/Next/Later 组织后续升级路线
- [x] 去重根目录启动器，只保留 `启动RelayCheck.bat` 和 `静默启动RelayCheck.vbs`
- [x] 将遗留 Python `run.py` 移入 `legacy/run.py` 并标记 deprecated
- [x] `relaycheck-desktop/README.md` 新增正式版说明、架构图、路由总览和验证命令
- [x] `relaycheck-hub/README.md` 改为实验性 MVP 警告
- [x] 后端 `/api/system/status` 返回 `RelayCheck Desktop v1.0`、构建时间和上次自检摘要
- [x] 设置页新增“关于 / 版本”卡片，展示版本、构建时间、数据路径、调度器和自检摘要
- [x] 前端标题、登录页和工作台标题收敛为 `RelayCheck Desktop`
- [x] Go 测试、前端构建、npm audit、Python legacy 入口编译、API smoke、桌面/390px 浏览器 smoke
- **状态：** complete

### 阶段 38：P0 本地 API 安全与可观测性收口
- [x] 为静态资源和 API 增加 Host 校验，默认只允许 loopback 主机名和当前监听端口
- [x] 为 API 响应增加基础安全头，降低本地页面被嵌入或嗅探的风险
- [x] 所有主要 `/api/*` 业务路由继续强制 requireSession，新增 `/api/health` 作为免登录健康检查
- [x] 增加 SSRF 出站 URL 安全校验，外部请求默认拒绝 localhost、loopback、private、link-local、metadata 等地址
- [x] 批量外部动作 limit 统一 clamp 到 1..10，Admin API pageSize clamp 到 10..100
- [x] 新增 `audit_log` 表、只读 `/api/system/audit-log` 和设置页“审计日志”卡片
- [x] 审计登录、登出、登录失败、设置保存、备份创建/删除/恢复、账号增删改、上游站点删除
- [x] 新增 `/api/health`，检查 DB、数据库文件、数据目录、密钥目录和 scheduler 状态
- [x] 补充 Host、SSRF、审计、健康检查、限流 clamp 单元测试
- [x] Go 测试、前端构建
- **状态：** complete

### 阶段 39：提示词总清单落盘与逐项勾选
- [x] 新增 `PROMPT_CHECKLIST.md`，把桌面提示词拆成可持续打勾的主清单
- [x] 当前已完成的 P0/P1 部分按真实状态标记 `[x]`
- [x] 未完成事项保留 `[ ]`，作为后续推进依据
- [x] `README.md` 和 `AGENT_HANDOFF.md` 增加该清单入口
- **状态：** complete

### 阶段 40：签到临时失败重试与结果标注
- [x] 签到请求遇到网络错误、HTTP 408/429/5xx 时，对同一候选接口自动重试
- [x] 签到临时失败使用指数退避，上限 3 次尝试
- [x] 签到结果返回并持久化 `retryCount`
- [x] 签到结果消息标注“已自动重试 N 次”
- [x] 授权失败、接口不存在和普通 4xx 不作为临时失败重试
- [x] 新增单元测试覆盖重试成功、重试标注持久化和不可重试状态分类
- **状态：** complete

### 阶段 41：统一 API 错误分类
- [x] API 错误响应增加稳定 `errorClass` 字段
- [x] HTTP 状态映射为 `validation_error`、`auth_error`、`permission_error`、`not_found`、`method_not_allowed`、`conflict`、`rate_limited`、`server_error` 等稳定分类
- [x] 保留原 `error` 文本，兼容现有前端错误展示
- [x] 新增单元测试覆盖错误分类响应和关键状态映射
- **状态：** complete

### 阶段 42：签到每站点最小间隔限流
- [x] `checkin.schedule` 新增 `siteMinIntervalSeconds`，默认 2 秒
- [x] 配置读取时将站点间隔夹取到 `0..60` 秒
- [x] 批量手动签到与自动签到共用站点最小间隔限流
- [x] 同一站点连续账号签到会等待最小间隔，不同站点不互相阻塞
- [x] 单账号手动签到不额外延迟
- [x] 新增单元测试覆盖站点间隔计算和配置夹取
- **状态：** complete

### 阶段 43：凭据加密复核与指纹化导出规范
- [x] 端到端复核密码、Cookie、Access Token、Refresh Token、API Key 加密落盘
- [x] 测试确认数据库加密字段不包含明文且可解密回读
- [x] 测试确认 Key 导出预览不泄漏明文密钥或其他凭据
- [x] 用户可见 README 增加凭据存储和导出安全规范
- [x] 明确导出端点只能返回指纹、遮罩引用、模型状态和诊断元数据
- **状态：** complete

### 阶段 44：浏览器授权与导入导出审计补齐
- [x] 浏览器授权打开写入 `browser_auth.opened` 审计
- [x] 浏览器授权保存写入 `browser_auth.connected` 审计
- [x] 浏览器授权断开/清除写入 `browser_auth.disconnected` 审计
- [x] Key 脱敏导出预览写入 `keys.export_preview` 审计
- [x] NewAPI Admin API / SQLite / legacy config / Chrome 密码 CSV 导入写入审计
- [x] 审计 metadata 只记录数量、布尔状态和资源 ID，不记录 Token、Cookie、密码或 API Key 明文
- [x] 新增单元测试覆盖导出审计不泄密、断开浏览器授权审计
- **状态：** complete

### 阶段 45：冻结 Python 版遗留风险说明
- [x] `newapi_signin/DEPRECATED.md` 新增冻结说明
- [x] 明确 SQLite WAL、busy_timeout、连接池等调优不再回迁冻结 Python 版
- [x] 明确遗留 `print()` 输出作为冻结风险，不再逐项改造为结构化 logging
- [x] 明确遗留吞异常代码作为冻结风险，不再逐项重构
- [x] 根目录 `README.md` 增加冻结说明入口
- **状态：** complete

### 阶段 46：全局 API 错误条与 ErrorBoundary
- [x] 前端 API 客户端读取后端 `errorClass`
- [x] fetch/API 失败发布全局错误事件
- [x] App shell 显示持久错误条，包含错误分类、HTTP 状态、接口路径和时间
- [x] 错误条提供“重试”和“关闭”
- [x] 新增 React `AppErrorBoundary`，渲染异常时显示可恢复错误页
- [x] 保持现有页面错误文案兼容，不改各业务页面数据流
- **状态：** complete

### 阶段 47：改前/改后/手测步骤三元组
- [x] `PROMPT_CHECKLIST.md` 新增“改动验收三元组”记录区
- [x] 为阶段 40 到阶段 46 补齐改前 / 改后 / 手测步骤摘要
- [x] 总原则勾选每个剩余改动补齐三元组
- **状态：** complete

### 阶段 48：渠道空状态区分复核
- [x] 复核渠道页源码已区分“还没有渠道”和“没有匹配渠道”
- [x] 同步勾选 `PROMPT_CHECKLIST.md`
- [x] 在三元组记录区补充改前 / 改后 / 手测步骤
- **状态：** complete

### 阶段 49：真实告警与搜索覆盖复核
- [x] 复核 Dashboard 待处理告警来自真实 Action Center / Diagnostics
- [x] 复核渠道搜索覆盖 `base_url`
- [x] 复核签到历史搜索覆盖 `message`
- [x] 同步勾选 `PROMPT_CHECKLIST.md`
- [x] 在三元组记录区补充改前 / 改后 / 手测步骤
- **状态：** complete

### 阶段 50：渠道搜索覆盖备注与平台
- [x] 渠道搜索继续覆盖现有名称、Base URL、状态、后台类型和来源字段
- [x] 安全解析渠道 `rawJson`，白名单提取 `note` / `remark` / `description` 等备注字段
- [x] 安全解析渠道 `rawJson`，白名单提取 `platform` / `provider` / `group` / `type` 等平台字段
- [x] 避免全文索引 `rawJson`，不把 password/token/cookie/API key 等潜在敏感字段加入搜索文本
- [x] 前端生产构建通过
- [x] 同步勾选 `PROMPT_CHECKLIST.md` 并补充三元组
- **状态：** complete

### 阶段 51：账号凭据与删除二次确认
- [x] 清空账号 API Key 前弹出确认，避免误删已保存密钥
- [x] 账号卡“删除账号”前弹出确认，明确会删除保存的密码、Cookie、Token 和 API Key 等凭据
- [x] 账号洞察里的“本地地址疑似误匹配”删除入口也弹出确认
- [x] 前端生产构建通过
- [x] 同步勾选 `PROMPT_CHECKLIST.md` 并补充三元组
- **状态：** complete

### 阶段 52：设置页帮助入口与能力图例
- [x] 设置页新增“帮助 / 文档”卡片，集中指向 README、PROMPT_CHECKLIST、DESIGN_SYSTEM、AGENT_HANDOFF
- [x] 帮助卡提供本地新手路径：本机扫描、账号授权/Key、签到和余额验证
- [x] 设置页新增常驻能力图例，解释 NEW/ONE/SUB/MOD、Key 有效、模型可用、raw_json、live 等 chip 含义
- [x] 样式保持现有 Control Room 短卡风格
- [x] 前端生产构建通过
- [x] 同步勾选 `PROMPT_CHECKLIST.md` 并补充三元组
- **状态：** complete

### 阶段 53：清空已读通知确认
- [x] 通知页“清空已读”前弹出确认
- [x] 确认文案明确未读通知会保留，已读通知历史删除后无法恢复
- [x] 前端生产构建通过
- [x] 同步勾选 `PROMPT_CHECKLIST.md` 并补充三元组
- **状态：** complete

### 阶段 54：通用 loading 骨架与减弱动画补强
- [x] 新增通用 `LoadingSkeleton`，支持 panel/table/chart 三种展示
- [x] 启动页、签到状态、任务中心使用面板骨架
- [x] Dashboard 自检、渠道列表、通知列表使用表格/列表骨架
- [x] Hub Radar 使用图表型骨架
- [x] `prefers-reduced-motion` 下骨架 shimmer 静态化
- [x] 前端生产构建通过
- [x] 同步勾选 `PROMPT_CHECKLIST.md` 并补充三元组
- **状态：** complete

### 阶段 55：渠道、签到历史、通知加载更多
- [x] 渠道列表默认显示 24 条，支持逐步加载更多
- [x] 签到历史默认显示 40 条，支持逐步加载更多
- [x] 通知列表默认显示 30 条，支持逐步加载更多
- [x] 筛选条件变化时重置显示数量，避免隐藏新的筛选结果
- [x] 前端生产构建通过
- [x] 同步勾选 `PROMPT_CHECKLIST.md` 并补充三元组
- **状态：** complete

### 阶段 56：首屏系统健康徽章可直达问题
- [x] Dashboard 首屏徽章显示“系统健康：良好 / N 项需关注”
- [x] 健康徽章可聚焦、可点击，保留 Badge 视觉样式
- [x] 有 Action Center 问题时点击跳转最高优先级问题目标页并携带筛选意图
- [x] 无问题时点击刷新系统自检
- [x] 前端生产构建通过
- [x] 同步勾选 `PROMPT_CHECKLIST.md` 并补充三元组
- **状态：** complete

### 阶段 57：账号页关键字段排序
- [x] 账号页新增排序下拉
- [x] 支持最近签到正序/倒序
- [x] 支持余额从高到低/从低到高
- [x] 支持 API Key 响应时间最快/最慢
- [x] 支持 ID 正序/倒序
- [x] 清空筛选时恢复默认最近签到优先
- [x] 前端生产构建通过
- [x] 同步勾选 `PROMPT_CHECKLIST.md` 并补充三元组
- **状态：** complete

### 阶段 58：遗留前端缺陷项源码复核
- [x] 复核正式版源码未发现 `UnlockGate` / `doUnlock`
- [x] 复核正式版源码未发现 `createAuthDraft`
- [x] 复核正式版源码未发现 `handleSaveBrowserAuth` / `filteredIds`
- [x] 复核正式版源码未发现 `checkinApi.batch`
- [x] 复核正式版源码未发现 `.trim('|')` 用法
- [x] 同步勾选 `PROMPT_CHECKLIST.md` 并补充三元组
- **状态：** complete

### 阶段 59：渠道搜索覆盖账号邮箱与用户名
- [x] 渠道页额外读取账号摘要用于搜索索引
- [x] 按渠道 Base URL 与账号站点 Base URL 关联账号
- [x] 兜底按渠道名与账号站点名近似匹配
- [x] 搜索索引只包含账号显示名、邮箱、用户名，不包含密码、Cookie、Token、API Key
- [x] 前端生产构建通过
- [x] 同步勾选 `PROMPT_CHECKLIST.md` 并补充三元组
- **状态：** complete

### 阶段 60：cgo/race 限制文档化
- [x] README 新增 Race / cgo Note
- [x] 说明当前 Windows Go 环境未启用 cgo，`go test -race ./internal/core` 会因 `-race requires cgo` 阻塞
- [x] 明确当前本地必跑回归门槛仍是 `go test -mod=vendor ./...`
- [x] 同步勾选 `PROMPT_CHECKLIST.md` 并补充三元组
- **状态：** complete

### 阶段 61：渠道归档确认
- [x] 复核正式版没有物理删除渠道 API，现有安全替代路径为归档保留
- [x] 单个渠道“归档保留”前新增确认弹窗
- [x] 确认文案明确不会删除账号、余额或签到日志，只会从日常视图隐藏
- [x] 取消确认时不发起归档请求
- [x] 同步勾选 `PROMPT_CHECKLIST.md` 并补充三元组
- **状态：** complete

### 阶段 62：批量删除确认复核
- [x] 复核正式版真实批量删除入口
- [x] 确认设置页多选删除备份 `deleteSelectedBackups` 已弹确认
- [x] 确认批量渠道操作为归档/恢复状态切换，不物理删除，并已有确认
- [x] 同步勾选 `PROMPT_CHECKLIST.md` 并补充三元组
- **状态：** complete

### 阶段 63：渠道软删除可恢复复核
- [x] 复核渠道清理采用 `missing -> archived -> active` 状态流转
- [x] 确认归档不会删除账号、余额或签到日志
- [x] 确认渠道页可筛选已归档并恢复活跃
- [x] 明确账号删除仍是物理删除并依靠二次确认，不纳入本阶段软删除范围
- [x] 同步勾选 `PROMPT_CHECKLIST.md` 并补充三元组
- **状态：** complete

### 阶段 64：详情抽屉模态可访问性
- [x] 新增 `useDialogBehavior` 管理详情抽屉键盘行为
- [x] Esc 可关闭详情抽屉
- [x] Tab / Shift+Tab 焦点限制在抽屉内循环
- [x] 关闭抽屉后焦点归还触发元素
- [x] `DetailDrawer` / `SiteDetailDrawer` 补齐 `role="dialog"` 与 `aria-modal="true"`
- [x] 同步勾选 `PROMPT_CHECKLIST.md` 并补充三元组
- **状态：** complete

### 阶段 65：浏览器授权背景点击复核
- [x] 复核前端没有浏览器授权弹层背景取消路径
- [x] 确认详情抽屉背景点击只关闭 UI，不调用保存、断开或删除授权
- [x] 复核后端 `browserSessions` 只在保存授权或断开授权时移除
- [x] 同步勾选 `PROMPT_CHECKLIST.md` 并补充三元组
- **状态：** complete

### 阶段 66：触屏目标与 Dashboard 自适应网格
- [x] Dashboard 主图表网格改为 `repeat(auto-fit, minmax(...))`
- [x] Dashboard 自检诊断网格改为 `repeat(auto-fit, minmax(...))`
- [x] Hub Radar 网格由 `auto-fill` 收敛为 `auto-fit/minmax`
- [x] 触屏粗指针设备下统一提升可点击控件到至少 44x44px
- [x] 保持桌面鼠标环境紧凑密度不变
- [x] 前端生产构建通过
- [x] 同步勾选 `PROMPT_CHECKLIST.md` 并补充三元组
- **状态：** complete

### 阶段 67：移动端主要内容单列保护
- [x] 在 CSS 末尾新增移动端主要内容单列保护层
- [x] 覆盖 Dashboard、Hub Radar、诊断、渠道、账号、余额、设置、调度、详情抽屉等主要内容网格
- [x] 保留移动端侧边导航紧凑横向条，不强行改为单列
- [x] 前端生产构建通过
- [x] 同步勾选 `PROMPT_CHECKLIST.md` 并补充三元组
- **状态：** complete

### 阶段 68：表格感行列宽弹性保护
- [x] 复核正式版前端没有原生 `<table>` 结构
- [x] 为详情、通知、审计、备份、同步结果、余额快照、签到日志等表格感 grid 行补充弹性列宽保护
- [x] 行内子元素统一 `min-width: 0`，长 URL/文件名/摘要允许 `overflow-wrap: anywhere`
- [x] 移动端表格感行统一单列，避免横向撑破
- [x] 前端生产构建通过
- [x] 同步勾选 `PROMPT_CHECKLIST.md` 并补充三元组
- **状态：** complete

### 阶段 69：重复 keyframes 收敛
- [x] 复核当前 CSS 动画定义和使用点
- [x] 将 `skeletonShimmer` 从组件局部区域移动到 `panel-in` 附近的全局 keyframes 区域
- [x] 保持 `panel-in` / `skeletonShimmer` 动画名称和行为不变
- [x] 前端生产构建通过
- [x] 同步勾选 `PROMPT_CHECKLIST.md` 并补充三元组
- **状态：** complete

### 阶段 70：非 emoji 线性图标与状态文字
- [x] 复核正式版前端无明显 emoji 图标残留
- [x] 确认当前前端未引入 `lucide-react`，本阶段不新增依赖
- [x] 新增轻量内联 `LineIcon` 线性 SVG 图标组件
- [x] 导航从字母缩写改为线性对象图标
- [x] Dashboard 健康、自检和任务中心状态 Badge 改为“线性状态图标 + 文字”
- [x] 前端生产构建通过
- [x] 同步勾选 `PROMPT_CHECKLIST.md` 并补充三元组
- **状态：** complete

### 阶段 71：状态不只靠颜色巡检与修正
- [x] 扩展 `StatusLabel` / `statusIconName`，覆盖 active、valid、scheduled、missing、archived、expired、failed、unreachable 等非诊断类状态
- [x] 渠道源端状态 pill 改为“图标 + 中文状态文字”
- [x] 账号登录态改为“图标 + 中文状态文字”
- [x] 调度任务、审计日志、同步摘要、同步实例结果改为“图标 + 中文状态文字”
- [x] 设置页正式版、代理开关、同步开关、代理测试结果改为“图标 + 中文状态文字”
- [x] 补充状态 pill 内图标对齐样式
- [x] 前端生产构建通过
- [x] 同步勾选 `PROMPT_CHECKLIST.md` 并补充三元组
- **状态：** complete

### 阶段 72：重要数字位置与等宽数字巡检
- [x] 巡检 Dashboard、Hub Radar、渠道/账号、签到/通知、余额、详情、同步和调度等指标数字样式
- [x] 新增集中 `Numeric scan pass` CSS 覆盖层
- [x] 为主要指标数字统一启用 `font-variant-numeric: tabular-nums`
- [x] 为主要指标数字补充 `font-feature-settings: "tnum" 1, "lnum" 1`
- [x] 为 Dashboard/Radar/同步/签到/余额等高优先级数字补充更稳定的字距与靠前对齐
- [x] 前端生产构建通过
- [x] 同步勾选 `PROMPT_CHECKLIST.md` 并补充三元组
- **状态：** complete

### 阶段 73：Tailwind/shadcn 设计系统收敛
- [x] 复核正式版确实使用 Tailwind v4 CSS import、`@tailwindcss/vite` 和 `tailwindcss`
- [x] 复核正式版没有 Radix/shadcn 运行时依赖
- [x] `DESIGN_SYSTEM.md` 明确保留 Tailwind v4 作为构建期 CSS 层
- [x] `DESIGN_SYSTEM.md` 明确不新增 Radix/shadcn 运行时依赖，`components/ui/*` 是项目自有轻量封装
- [x] 清理 `styles.css` 中 `shadcn-inspired` / `shadcn/Linear` 注释残留
- [x] 前端生产构建通过
- [x] 同步勾选 `PROMPT_CHECKLIST.md` 并补充三元组
- **状态：** complete

### 阶段 74：V4 token foundation 与 Tailwind bridge 第一批
- [x] `@theme` 从硬编码颜色/圆角/阴影桥接到 V4 token
- [x] `:root` 补齐 V4 语义色、状态背景、输入、骨架、字号、字重、字距、间距、圆角和阴影 token
- [x] 侧边栏、页面摘要、指标卡、状态 pill、工具条等活跃 V4 覆盖层第一批硬编码改用 token
- [x] 不提前勾选完整“颜色/圆角/阴影/间距/字号单一来源”大项，保留给后续历史层清理
- [x] 前端生产构建通过
- [x] 同步勾选 `PROMPT_CHECKLIST.md` 并补充三元组
- **状态：** complete

### 阶段 75：Active V4 token sweep 第二批
- [x] 补充 V4 amber 文字语义 token
- [x] 导航激活色改用 V4 blue token
- [x] 移动密度覆盖的圆角和字号改用 V4 radius/type token
- [x] 全局错误条、fatal error 卡、JSON preview 改用 V4 状态色、字号、圆角和阴影 token
- [x] 前端生产构建通过
- [x] 同步勾选 `PROMPT_CHECKLIST.md` 并补充三元组
- **状态：** complete

### 阶段 76：relaycheck-hub SQLite 调优同步
- [x] 新增 `relaycheck-hub/src/lib/sqlite-tuning.ts`，集中定义 WAL、busy_timeout、synchronous、temp_store、cache_size、foreign_keys 调优
- [x] `relaycheck-hub` schema 初始化改用统一调优入口
- [x] Prisma better-sqlite3 adapter 显式使用 `timeout: 5000` 并继续保持进程级 singleton
- [x] 外部 NewAPI SQLite 只读导入继承 5000ms timeout，但不强制修改用户外部数据库 WAL/pragma
- [x] 新增 `npm run verify:sqlite` 验证 WAL、busy_timeout、foreign_keys、synchronous、temp_store、cache_size
- [x] `relaycheck-hub/README.md` 记录实验版 SQLite 可靠性基线
- [x] 同步勾选 `PROMPT_CHECKLIST.md` 并补充三元组
- **状态：** complete

### 阶段 77：目标提示词清单与组件基础设施
- [x] 读取 `目标/COMPONENT_ARCHITECTURE_PROMPT.md`
- [x] 读取 `目标/UI_UX_BEAUTIFICATION_PROMPT.md`
- [x] 新增 `目标/TARGET_PROMPT_CHECKLIST.md`，将两份提示词拆成可打勾清单
- [x] `frontend/src/lib/cn.ts` 升级为 `clsx + tailwind-merge`
- [x] `Button` / `Card` / `Badge` 使用升级后的 `cn()` 与 `@/*` alias
- [x] `frontend/tsconfig.json` 和 `frontend/vite.config.ts` 配置 `@/*` alias
- [x] 新增 `Input`、`Select`、`Skeleton`、`Dialog` 四个高优先级 UI primitives
- [x] `DESIGN_SYSTEM.md` 同步本地 UI primitives 与 alias 规则
- [x] 前端生产构建通过
- [x] 同步勾选 `目标/TARGET_PROMPT_CHECKLIST.md` 与 `PROMPT_CHECKLIST.md` 并补充三元组
- **状态：** complete

### 阶段 78：目标提示词 A2 UI 原子组件补齐
- [x] 新增 `frontend/src/components/ui/progress.tsx`
- [x] 新增 `frontend/src/components/ui/tooltip.tsx`
- [x] 新增 `frontend/src/components/ui/switch.tsx`
- [x] 三个组件均使用 `cn()`、支持外部 `className`，并避免新增外部 UI 依赖
- [x] `Progress` 使用 `role="progressbar"` 与 `aria-valuenow`
- [x] `Tooltip` 使用 CSS-only hover/focus 展示，避免隐藏关键业务信息
- [x] `Switch` 使用原生 button 与 `role="switch"` / `aria-checked`
- [x] 前端生产构建通过
- [x] 同步勾选 `目标/TARGET_PROMPT_CHECKLIST.md`
- **状态：** complete

### 阶段 79：目标提示词 A3 类型定义抽离
- [x] 新增 `frontend/src/types/index.ts`，集中承载前端 DTO、导航、API 错误与 UI 图标类型
- [x] `frontend/src/main.tsx` 改为 `import type { ... } from "@/types"`
- [x] `TabKey` 从 `main.tsx` 本地推导改为显式 union，并让 `navItems` 使用 `satisfies readonly NavItem[]` 做反向校验
- [x] 不改业务逻辑、API 调用、渲染结构或样式
- [x] 前端生产构建通过
- [x] 同步勾选 `目标/TARGET_PROMPT_CHECKLIST.md`
- **状态：** complete

### 阶段 80：目标提示词 A3 API client 抽离
- [x] 新增 `frontend/src/api/client.ts`
- [x] 将 `ApiError`、`api()`、读缓存、错误订阅与错误发布逻辑从 `main.tsx` 移入 API client
- [x] `frontend/src/main.tsx` 改为从 `@/api/client` 导入 `api` 与 `subscribeApiErrors`
- [x] 保留原缓存 TTL、非缓存前缀、`credentials: "same-origin"`、错误事件结构和非 GET 清缓存行为
- [x] 前端生产构建通过
- [x] 同步勾选 `目标/TARGET_PROMPT_CHECKLIST.md`
- **状态：** complete

### 阶段 81：目标提示词 A3 format 工具抽离
- [x] 新增 `frontend/src/lib/format.ts`
- [x] 抽离时间、构建时间、时长、短时长、字节、置信度、JSON 预览格式化工具
- [x] 抽离余额、紧凑数字、USD、十进制与价格来源/价格比较格式化工具
- [x] `frontend/src/main.tsx` 改为从 `@/lib/format` 导入这些工具
- [x] `formatAPIKeyTestMessage` 暂留 `main.tsx`，等待 labels 工具抽离后再迁移
- [x] 前端生产构建通过
- [x] 同步勾选 `目标/TARGET_PROMPT_CHECKLIST.md`
- **状态：** complete

### 阶段 82：目标提示词 A3 labels 工具抽离
- [x] 新增 `frontend/src/lib/labels.ts`
- [x] 抽离错误分类、诊断等级、渠道来源/状态、同步来源/动作、后台类型、审计、调度、签到、登录态、API Key、用量趋势和价格层级等纯标签映射
- [x] 将 `formatAPIKeyTestMessage` 从 `main.tsx` 迁移到 `lib/labels.ts`
- [x] `frontend/src/main.tsx` 改为从 `@/lib/labels` 导入上述标签工具
- [x] 保留 `diagnosticNavigationIntent`、`actionNavigationIntent`、`actionCenterQuickActions` 等行为逻辑在 `main.tsx`，不混入 label 工具
- [x] 前端生产构建通过
- [x] 同步勾选 `目标/TARGET_PROMPT_CHECKLIST.md`
- **状态：** complete

### 阶段 83：目标提示词 A3 constants 工具抽离
- [x] 新增 `frontend/src/lib/constants.ts`
- [x] 抽离导航元信息 `NAV_ITEMS`
- [x] 抽离状态图标分类集合、目标中转类型集合、签到成功/问题状态集合、渠道 rawJson 搜索白名单、重要通知等级/关键词
- [x] 抽离 Dialog 可聚焦选择器、列表加载更多初始数量/递增数量和 API Key 过期阈值
- [x] `frontend/src/main.tsx` 改为从 `@/lib/constants` 导入这些静态配置
- [x] 保留页面状态、业务流程函数和 richer explanation helpers 在 `main.tsx`
- [x] 前端生产构建通过
- [x] 同步勾选 `目标/TARGET_PROMPT_CHECKLIST.md`
- **状态：** complete

### 阶段 84：目标提示词 A3 HubRadar 拆分
- [ ] 从 `frontend/src/main.tsx` 抽离 `HubRadar`
- [ ] 新增目标组件文件，优先使用 `frontend/src/components/dashboard/HubRadar.tsx`
- [ ] 保持现有导航回调、API 数据形状、格式化和调度状态展示不变
- [ ] 前端生产构建通过
- [ ] 同步勾选 `目标/TARGET_PROMPT_CHECKLIST.md`
- **状态：** pending
- **当前断点：** 已创建空目录 `frontend/src/components/dashboard`；尚未创建 `HubRadar.tsx`，尚未迁移 `HubRadar` 代码，尚未勾选目标清单。
- **接手入口：** 先读 `AGENT_HANDOFF.md` 顶部 `Immediate Resume Packet`，再只做 `HubRadar` 这一刀。
- **下一步唯一动作：** 把 `main.tsx` 中的 `HubRadar` 迁移到 `frontend/src/components/dashboard/HubRadar.tsx`，处理 `LoadingSkeleton` 依赖，保留 `actionNavigationIntent` 行为。
- **完成判定：** `HubRadar.tsx` 存在、`main.tsx` 导入并渲染它、`main.tsx` 不再定义 `HubRadar`、`frontend npm run build` 通过后，才能勾选 `目标/TARGET_PROMPT_CHECKLIST.md` 的 `HubRadar 拆分`。

### 阶段 36：AI API Hub Radar 与总览柔和协调
- [x] 使用 agent-reach/GitHub CLI 复核 `qixing-jk/all-api-hub` 产品方向
- [x] 将 all-api-hub 的资产、Key、价格、用量、健康检查方向映射到本地轻量工具
- [x] Dashboard 新增 `AI API Hub Radar` 四张柔和短卡
- [x] 雷达复用现有模型、价格、用量、系统状态、自检和处理建议 API
- [x] 增加渠道/同步、Key/待检测、余额/价格雷达、处理问题/调度设置快捷入口
- [x] 保持桌面短卡、移动端全宽，无长条横幅
- [x] 前端构建、Go 测试、依赖审计、隐藏重启 3001、Playwright 无截图 smoke、敏感扫描
- **状态：** complete

### 阶段 31：模型同步、价格层级、Key 安全导出
- [x] 新增模型概览 API
- [x] 新增模型同步 API，复用现有 Key 检测和测速逻辑
- [x] 新增 Key 脱敏导出预览 API
- [x] 账号页展示模型覆盖、价格层级和安全导出短卡片
- [x] 验证 Go 测试和前端构建
- [x] 重建并隐藏启动桌面端到 3001
- **状态：** complete

### 阶段 32：完整价格/用量分析增强
- [x] 从 NewAPI channel config、model_mapping、倍率字段中提取价格/倍率数据
- [x] 增加渠道模型同步，优先使用渠道 Key `/v1/models`，回退 NewAPI raw_json
- [x] 增加渠道模型覆盖短卡片
- [x] 增加用量趋势和余额消耗估算
- [x] 增加余额页用量分析短卡片
- [x] 探测站点 `/api/pricing` 并缓存可解释的价格来源
- [x] 按模型、站点、账号做价格/延迟/可用性对比
- [x] 参考 modeloc 类检测维度，但 Key 检测仍本地直连上游，不提交第三方
- [x] all-api-hub 风格高级用量/价格矩阵已落地轻量 MVP，后续可继续扩展更细报表
- **状态：** complete

### 阶段 1：需求与发现
- [x] 理解用户希望持续优化升级，而不是只做规划
- [x] 确定当前重点在 NewAPI 同步、渠道合并、识别解释、数据安全
- [x] 将发现记录到 findings.md
- **状态：** complete

### 阶段 2：规划与结构
- [x] 确定本轮先做同步质量升级
- [x] 保持 Go + embedded React + SQLite 轻量架构
- [x] 记录决策及理由
- **状态：** complete

### 阶段 3：实现
- [x] 增强同步预览，识别源端已移除渠道
- [x] 在前端显示 removed 差异和风险提示
- [x] 保持同步本身默认不删除本地数据
- **状态：** complete

### 阶段 4：测试与验证
- [x] 运行前端构建
- [x] 运行 Go 测试
- [x] 重建并重启桌面端
- [x] 用 API 验证新功能
- **状态：** complete

### 阶段 5：交付
- [x] 总结修改点
- [x] 说明测试结果
- [x] 给出下一步优化建议
- **状态：** complete

### 阶段 6：removed 渠道安全标记
- [x] 增加 source_sync_status / source_missing_at 字段
- [x] 增加 mark-missing 后端确认接口
- [x] 导入/同步时自动恢复 active 状态
- [x] 渠道页显示源端已移除状态
- [x] 扫描页提供确认标记按钮
- **状态：** complete

### 阶段 7：missing/archived 渠道管理
- [x] 渠道页增加搜索和同步状态筛选
- [x] 渠道页增加后台类型筛选
- [x] 单个渠道支持恢复活跃和归档保留
- [x] 批量归档全部 missing
- [x] 批量恢复全部 archived
- **状态：** complete

### 阶段 8：渠道页日常视图与 UI 验证
- [x] 默认隐藏 archived 渠道
- [x] 保留“全部含归档”和“已归档”筛选入口
- [x] 清空筛选恢复日常视图
- [x] Playwright 验证渠道筛选 UI
- **状态：** complete

### 阶段 9：系统自检诊断
- [x] 新增 /api/system/diagnostics
- [x] 检查数据库、实例、渠道、账号、签到、通知
- [x] 总览页展示诊断卡片和建议动作
- [x] Playwright 验证自检 UI
- **状态：** complete

### 阶段 10：自检卡片点击定位
- [x] 自检卡片支持点击跳转
- [x] 渠道类自检自动带筛选条件
- [x] missing 渠道自检跳到渠道页并筛选 missing
- [x] Playwright 验证点击定位
- **状态：** complete

### 阶段 11：账号/签到/通知工作台筛选
- [x] 账号页支持问题账号筛选
- [x] 签到页支持异常记录筛选
- [x] 通知页支持未读筛选
- [x] 自检卡片可跳转到账号/签到/通知并自动筛选
- [x] Playwright 验证账号异常和未读通知跳转
- **状态：** complete

### 阶段 12：设置页与数据库备份恢复
- [x] 新增系统设置 API
- [x] 新增备份列表、创建备份、安全恢复 API
- [x] 恢复前自动创建当前数据库快照
- [x] 设置页展示运行路径、备份列表、设置 JSON
- [x] 构建、Go 测试、API 验证、Playwright UI 验证
- **状态：** complete

### 阶段 13：处理建议中心
- [x] 新增 /api/system/action-center
- [x] 聚合授权失效、密钥异常、签到异常、余额缺失、低余额、未知渠道、missing 渠道、不可达站点
- [x] 总览页新增处理建议中心卡片
- [x] 建议卡片支持跳转到对应页面并携带筛选意图
- [x] 构建、Go 测试、API 验证、Playwright UI 验证
- **状态：** complete

### 阶段 14：系统 debug 与详情接口修复
- [x] 核心只读 API 冒烟测试
- [x] 数据一致性检查
- [x] 修复 GET /api/channels/:id 返回 405
- [x] 修复 GET /api/accounts/:id 返回 405
- [x] Playwright 遍历全部导航页面并检查前端错误
- [x] 构建、重启、复测、敏感信息扫描
- **状态：** complete

### 阶段 15：正式回归测试
- [x] 后端 go test
- [x] 前端生产构建
- [x] 核心只读 API 合同测试
- [x] 渠道/账号详情接口回归
- [x] Playwright 覆盖总览、渠道、站点、账号、签到、余额、通知、扫描、设置
- [x] 移动端基础布局检查
- [x] 重建并隐藏重启桌面端
- [x] 敏感信息扫描
- **状态：** complete

### 阶段 16：账号卡片可编辑站点 URL 与视觉层级
- [x] 账号编辑卡支持修改站点名称、Base URL、登录页和后台类型
- [x] Base URL 改变时复用或创建上游站点，并只迁移当前账号绑定
- [x] Base URL 未改变时允许更新站点显示信息和登录页
- [x] 账号卡增加主头像/站点标识，小状态图标/徽章作为次级信息
- [x] 根据设计参考优化图标尺寸、卡片扫描性、表单提示
- [x] 更新 AGENT_HANDOFF.md，方便其他 agent 接力
- [x] 构建、测试、重启并验证 3001
- **状态：** complete

### 阶段 17：账号卡片层次感加强
- [x] 账号卡片改为身份层、数据层、操作层三段结构
- [x] 增加左侧状态轨，让异常账号扫读时更醒目
- [x] 主头像提升到 56px，状态徽章保持小尺寸，形成图标主次
- [x] 余额指标加重，账号/签到指标保持次级权重
- [x] 主操作、维护操作、危险操作分区显示
- [x] 编辑区增加内嵌配置面板层级
- [x] 构建、Go 测试、隐藏重启 3001、Playwright 桌面/移动端验证
- **状态：** complete

### 阶段 18：按产品痛点优化账号与 NewAPI 同步
- [x] 账号卡默认紧凑，只保留签到、刷新余额、网页登录和更多
- [x] 维护操作和危险操作收进“更多”，降低误操作和视觉噪音
- [x] 编辑账号支持“只改当前账号 / 同步同站点全部账号”
- [x] 后端支持 shared 范围更新站点地址，并同步更新对应渠道显示字段
- [x] 头像改为域名缩写，叠加后台类型 NewAPI/OneAPI/Sub2API 小标
- [x] 重要信息和次要信息拉开字号层级：账号名 > 余额 > 签到/账号 > 密钥/时间
- [x] 本机扫描页增加每实例“一键同步”和顶部“一键同步全部可用实例”
- [x] 构建、Go 测试、隐藏重启 3001、Playwright 桌面/移动端验证
- **状态：** complete

### 阶段 19：NewAPI 同步结果反馈升级
- [x] 本机扫描页实例列表改为小卡片样式
- [x] 增加同步结果摘要卡，显示更新渠道、新站点、合并站点、探测、源端移除
- [x] 单实例同步、标记源端移除、单实例一键同步均写入结构化结果
- [x] 全部一键同步支持单实例失败后继续执行，并汇总失败原因
- [x] 修复一键同步清除保存令牌的顺序，避免 mark-missing 阶段丢失令牌
- [x] 构建、Go 测试、隐藏重启 3001、Playwright 模拟同步验证桌面/移动端
- **状态：** complete

### 阶段 20：API Key 模型检测、有效性与测速
- [x] 保持本地直连上游检测，不上传 Key 到第三方检测站
- [x] `/api/accounts/:id/test-api-key` 获取 `/v1/models`
- [x] 从模型列表中选择轻量优先模型，发起最小 chat completion 测试
- [x] 返回并持久化 Key 状态、模型数量、样例模型、测试模型、模型可用性、测速延迟和诊断消息
- [x] 批量检测继续统计有效 Key 和可用模型数量
- [x] 账号卡在有保存 Key 时显示模型检测摘要
- [x] API Key 修改或清空时重置旧检测摘要
- [x] 构建、Go 测试、隐藏重启 3001、Playwright 账号页冒烟验证
- **状态：** complete

### 阶段 21：全局代理、魔改站点探测提速与设置页验证
- [x] 增加 `network.proxy` 默认设置，预置 `http://127.0.0.1:7897`
- [x] 后端校验代理 URL，并拒绝带用户名密码的代理地址
- [x] 外部网络请求统一接入代理配置，本地地址默认绕过代理
- [x] 新增 `/api/system/proxy-test`，用于测试当前代理访问目标站点
- [x] 设置页增加代理卡片，支持启用、绕过本地地址、保存并测试
- [x] Playwright/Chrome 网页登录启动时跟随代理配置
- [x] 上游站点探针改为有限并发，降低超时站点检测耗时
- [x] 当前数据库启用 `127.0.0.1:7897` 代理，并验证 `wxls.ccwu.cc` HTTP 200
- [x] 验证 `wxls.ccwu.cc` 被识别为 `modified_relay`，健康状态 `auth_required`，置信度 `0.98`
- [x] 修复设置页移动端横向溢出
- [x] 构建、Go 测试、隐藏重启 3001、Playwright 桌面/移动端设置页验证
- **状态：** complete

### 阶段 22：处理建议动作回归与登录失败诊断
- [x] 确认 3001 服务状态和管理员登录链路
- [x] 轻量验证 `/api/system/action-center`
- [x] 使用 `limit=1` 验证批量余额刷新接口，避免大批量外部请求阻塞
- [x] Playwright 验证处理建议“查看问题”跳转账号问题列表
- [x] Playwright 验证上游站点统计、健康状态筛选、桌面和移动端无横向溢出
- [x] 改进账号密码登录失败信息，列出所有候选登录路径和下一步处理建议
- [x] 新增后端单测覆盖登录失败诊断
- [x] 前端构建、Go 测试、Windows GUI exe 构建、隐藏重启 3001
- [x] 敏感信息扫描不命中用户提供的密码/token/邮箱片段
- **状态：** complete

### 阶段 23：签到状态、问题优先信息架构和维护体验
- [x] 参考 `qixing-jk/all-api-hub` 的 New-API/Sub2API 账号中心产品方向
- [x] 新增 `/api/checkins/status`，返回当前签到、今日统计、待签数量和倒计时
- [x] 一键签到全部写入运行进度，并防止重复启动
- [x] 总览页显示紧凑签到倒计时
- [x] 签到页显示完整签到状态，成功/已签折叠，失败/授权/不支持重点展示
- [x] 渠道页默认只展示 NewAPI/OneAPI/Sub2API/魔改中转站
- [x] 渠道卡片明确显示能否签到
- [x] 余额页按上游站点汇总多个账号余额，并保留账号明细
- [x] 通知页默认只展示重要通知，普通通知收纳
- [x] 设置页备份默认只显示最新一个，支持多选删除旧备份
- [x] 设置页新增同步频率配置，默认 30 分钟
- [x] 统一提升通知、备份、签到、列表卡片圆角
- [x] 构建、Go 测试、隐藏重启 3001、Playwright 桌面/移动端验证
- **状态：** complete

### 阶段 24：后台自动签到与 NewAPI 定时同步调度器
- [x] 新增 `scheduler_runs` 轻量状态表，记录任务状态、计划时间、上次运行和摘要
- [x] 桌面端启动时挂载后台 scheduler，普通测试创建 App 不自动启动后台任务
- [x] 自动签到读取 `checkin.schedule`，在每日窗口内选择随机执行时间，并用 run key 避免同一天重复自动触发
- [x] 一键签到和自动签到复用同一套 `runDueCheckins` 逻辑
- [x] NewAPI 定时同步读取 `sync.schedule`，默认 30 分钟间隔，不导入渠道 Key，不做重探测
- [x] 后台同步默认静默，只在失败/部分失败时发重要通知
- [x] 新增 `/api/system/scheduler-status`，系统状态也返回 scheduler 摘要
- [x] 签到状态卡显示随机后的真实下次执行时间
- [x] 设置页新增“后台调度器”卡片，展示自动签到和 NewAPI 同步的下次/上次状态
- [x] 构建、Go 测试、隐藏重启 3001、API 冒烟、Playwright 桌面/移动端验证、敏感扫描
- **状态：** complete

### 阶段 25：账号页紧凑化与功能参考校准
- [x] 缩小账号卡桌面列宽，避免 grid 自动撑满
- [x] 缩小头像、标题、指标、芯片、按钮和编辑区间距
- [x] 将账号页顶部信息条改为短块，不再横跨整页
- [x] 保持移动端 100% 宽度和无横向溢出
- [x] 使用 `qixing-jk/all-api-hub` README 校准后续功能参考方向
- [x] 构建、Go 测试、隐藏重启 3001、Playwright 验证
- **状态：** complete

### 阶段 26：短圆角信息卡与 Key/模型能力总览
- [x] 将账号页顶部信息从长条框拆成短圆角卡片组
- [x] 新增 Key/模型能力总览：有效 Key、异常/未测、建议重测、模型能力、平均测速、模型样例
- [x] 将同步结果、签到/通知重点卡、通用 note、wide item 改成桌面内容宽度优先
- [x] 保持小屏自动铺满，避免按钮和文字挤压
- [x] 按 `qixing-jk/all-api-hub` 功能方向落地轻量版 Key 库/模型可用性检测入口
- [x] 构建、Go 测试、隐藏重启 3001、Playwright 验证、敏感扫描
- **状态：** complete

### 阶段 27：模型测速排行与 Key 问题短卡片
- [x] 在账号页新增“模型测速排行”短卡片，展示已测速账号的最快模型调用延迟
- [x] 在账号页新增“待处理 Key”短卡片，聚合异常、未测、超过 24 小时未重测的 Key
- [x] 待处理 Key 支持单账号检测，复用已有 `/api/accounts/:id/test-api-key`
- [x] 没有保存 Key 时也显示空状态，引导用户添加或导入 API Key
- [x] 保持桌面 320px 短卡片、移动端单列铺满
- [x] 构建、Go 测试、隐藏重启 3001、Playwright 验证
- **状态：** complete

### 阶段 28：能力卡柔和布局与模型覆盖筛选
- [x] 将账号页能力卡从可能拉长的三列改为柔和短卡，桌面单卡约 272px
- [x] 将“待处理 Key”升级为“Key 状态”，同时展示成功/有效 Key 和问题 Key
- [x] 新增“模型覆盖”短卡，按模型聚合账号数量
- [x] 点击模型覆盖 chip 可自动筛选账号
- [x] 账号搜索支持模型名、测试模型、Key 指纹和 Key 状态
- [x] 优化行高、字号、背景、边框，成功行柔和绿底，问题行轻微黄底
- [x] 构建、Go 测试、隐藏重启 3001、Playwright 验证、敏感扫描
- **状态：** complete

### 阶段 29：账号顶部横排与签到成功/失败同款展开
- [x] 账号页顶部信息改为 desktop flex-wrap 横向流式排列，避免全部往下堆
- [x] 保持能力卡短宽，卡片横向容纳，屏幕不足时再换行
- [x] 签到页成功记录默认展开最近 5 条
- [x] 签到页失败/需授权/不支持记录改为和成功同款短行设计
- [x] 失败记录增加状态胶囊，并在记录过多时显示“还有 N 条”提示
- [x] 560px 以下成功/失败行自动变为单列，避免挤压
- [x] 构建、Go 测试、隐藏重启 3001、端口确认、敏感扫描
- **状态：** complete

### 阶段 31：shadcn/ui 参考视觉增强
- [x] 读取当前 React/Vite/CSS 结构，确认不迁移技术栈
- [x] 参考 `shadcn-ui/ui` 的 token、圆角、边框、状态和可访问组件思路
- [x] 抽样参考本地 Astral、Blog Home、SB Admin 2 模板的背景层次、卡片节奏和后台密度
- [x] 用系统 Chrome 巡检真实页面，定位长筛选条、通知长行、设置标题偏长等问题
- [x] 在 `frontend/src/styles.css` finishing layer 追加轻量覆盖，不改业务逻辑
- [x] 筛选条最大 820px，通知行最大 640px，设置标题最大 560px
- [x] 账号卡收至 318px，渠道卡约 348px，余额卡约 272px
- [x] 增加 `prefers-reduced-motion` 兼容
- [x] 前端构建、Go 测试、隐藏重启 3001、API smoke、Chrome 桌面/移动 UI smoke、敏感扫描
- **状态：** complete

### 阶段 32：Control Room 深度设计改造
- [x] 学习 UI/UX 设计原则，明确本项目是本地运维控制台，不是营销官网
- [x] 新增 `DESIGN_SYSTEM.md`，固化短卡片、问题优先、高密度、低噪音、轻量架构规则
- [x] 顶部栏改成工作台状态区，展示本地运行、端口、架构、SQLite 和重要通知
- [x] 新增深度 CSS 覆盖层：gridded 背景、紧凑侧边栏、低噪音卡片、tabular 数字、统一状态色
- [x] 核心卡片进一步压缩：stats 196px、action 276px、channel 336px、account 310px、notification 610px
- [x] 保留现有业务逻辑、路由、API 和轻量依赖
- [x] 前端构建、Go 测试、隐藏重启 3001、API smoke、Chrome 桌面/移动 UI smoke、敏感扫描
- **状态：** complete

## 关键问题
1. 源端移除的渠道是否直接删除本地数据？本轮决定默认只提示，不自动删除。
2. 是否继续保留官方供应商渠道？本轮不改变导入范围，避免破坏现有数据。

## 已做决策
| 决策 | 理由 |
|------|------|
| 同步预览先增加 removed 状态 | 用户需要知道 NewAPI 后台和本地渠道差异，风险低、收益高 |
| 不自动删除本地 removed 渠道 | 本地账号、余额、签到日志可能仍有价值，自动删除风险高 |
| 保持轻量架构 | 用户明确要求占用小、启动快、稳定 |
| removed 后先标记 missing，不直接删除 | 方便后续归档、恢复、审计，避免误删 |
| 批量操作只做状态切换 | 提升效率，同时保留可回退能力 |
| 渠道页默认隐藏归档 | 降低日常使用干扰，同时仍可通过筛选恢复查看 |
| 总览增加系统自检 | 让打不开、空白、签到失败、授权失效等问题更容易定位 |
| 自检卡片可点击定位 | 从“发现问题”直接进入“处理问题”，减少来回找页面 |
| 账号/签到/通知使用前端筛选 | 不改后端接口，轻量、低风险、响应快 |
| 恢复前自动备份当前数据库 | 让恢复操作可回退，避免误恢复造成不可逆数据损失 |
| 处理建议中心只读聚合，不自动执行修复 | 避免误操作账号授权、渠道状态或余额记录，同时让用户更快定位问题 |
| 详情接口补齐但不改变列表结构 | 满足 API 兼容需求，同时避免影响现有前端页面 |
| 账号编辑 URL 改变时迁移当前账号，不直接改共享站点 | 同一站点可能绑定多个账号，直接改共享站点会影响其他账号；迁移当前账号更安全 |
| 同步结果用摘要卡而不是单行消息 | 用户需要知道新增、更新、合并、源端移除和失败原因，结构化结果更适合排查 |
| 全部同步遇到单个失败继续执行 | 多个本地 NewAPI 实例之间相互独立，单个失败不应阻断其它实例同步 |
| 一键同步清令牌延后到 mark-missing 后 | 使用已保存令牌时，提前清除会导致第二步无法读取源端 channels |
| Key 模型检测只保存摘要 | 轻量化、减少数据库膨胀，同时避免长期保存完整上游响应 |
| Key 可读模型不等于模型可调用 | UI 和结果明确区分“能读取 /v1/models”和“最小 chat 调用可用” |
| 全局代理必须可视化配置 | 用户已经明确使用 7897 代理，放在设置页能降低不可达误判排查成本 |
| 本地地址默认绕过代理 | 否则本机 NewAPI 扫描和桌面 API 可能被代理影响 |
| 探针有限并发而不是无限并发 | 在提升速度和避免压上游之间取平衡 |
| 登录失败要给出可执行方案 | 密码登录路径不兼容时，用户需要知道尝试了哪些接口，以及下一步是修正登录 URL 还是使用网页登录授权 |
| 日常渠道页只聚焦目标中转站 | 用户只需要 NewAPI/Sub2API 体系中转站，官方供应商和纯 OpenAI 兼容 API 放到“全部后台类型”里按需查看 |
| 通知默认重要优先 | 普通成功类通知数量大，默认收纳能避免掩盖授权失效、失败和低余额问题 |
| 备份多选只用于删除清理 | 恢复数据库是高风险操作，仍保持单个备份恢复 |
| 后台同步默认静默 | 30 分钟一次的成功通知会制造噪音，只有失败/部分失败需要提醒 |
| 自动签到使用每日 run key | 即使程序重启，也不会在同一个计划日重复自动跑全量签到 |
| 账号卡桌面列宽设上限 | 多账号管理需要密度，固定上限比拉伸到整列更容易扫读 |
| `all-api-hub` 参考功能实现路线 | 优先吸收资产总览、Key 库、价格比较、模型/延迟检测、渠道模型同步，而不是照搬扩展 UI |
| 长条信息框默认改短卡片 | 用户明确不喜欢横向拖长的信息条；桌面按内容包裹，移动端再全宽更符合实际使用 |
| Key/模型总览先用现有字段聚合 | 后端已保存 Key 检测摘要，前端聚合最轻量、最快稳定，不需要新表或重型依赖 |
| 模型测速排行先展示已有测试结果 | 避免页面加载时自动消耗上游额度，测速必须由用户主动点击批量或单账号检测触发 |
| 没有 Key 也显示能力卡空状态 | 功能入口应该可见，否则用户不知道下一步要保存 API Key 才能启用检测 |
| 能力卡不追求铺满整行 | 多账号工具的顶部信息应一眼看清关键数字，卡片按内容短宽展示比拉满更稳 |
| 成功状态也要展示 | 只展示问题会让用户误以为系统整体失败；有效 Key 和可用模型需要提供正向确认 |
| 桌面端信息可以横排 | 用户明确指出信息多时可以横放，不要全部往下；桌面使用 flex-wrap，移动端再单列 |
| 签到成功和失败使用同款行节奏 | 成功也需要适当展开，失败也不应笨重堆叠；同一行节奏更容易对比状态 |
| shadcn/ui 只作为设计参考 | 当前项目轻量优先，不引入 Tailwind/Radix，使用 CSS token 和组件覆盖即可获得一致视觉 |
| 长条信息框继续收敛到内容宽 | 用户多次要求不要无脑拉长，桌面短卡优先，移动端再全宽 |
| Control Room 是长期视觉方向 | 这个工具需要像运维控制台一样快速扫状态、处理失败、同步渠道，而不是做大幅营销化排版 |

## 遇到的错误
| 错误 | 尝试次数 | 解决方案 |
|------|---------|---------|
| 本机没有 sqlite3 CLI | 1 | 改用项目 Go + modernc sqlite 驱动创建临时只读检查脚本，检查后删除 |
| GET /api/channels/:id 和 GET /api/accounts/:id 返回 405 | 1 | 补齐详情查询 handler，复用列表字段结构返回单条记录 |

## 备注
- 做重大决策前重新读取此计划。
- 避免把访问令牌、密码、Cookie 写进源码或临时文件。


---

### Phase 85: P1 active domain panels and check-in cleanup backend
- [x] Extract Sites page into frontend/src/components/sites/SitesPanel.tsx.
- [x] Extract Check-ins page into frontend/src/components/checkins/CheckinsPanel.tsx.
- [x] Extract Notifications page into frontend/src/components/notifications/NotificationsPanel.tsx.
- [x] Wire the new panels from frontend/src/main.tsx.
- [x] Extend smoke coverage for desktop and 390px mobile tab navigation / overflow checks.
- [x] Add backend cleanup route POST /api/accounts/delete-unsupported-checkins.
- [x] Add backend dry-run/delete logic for accounts that cannot run check-ins.
- [x] Add Go tests for cleanup behavior and strengthened upstream detection.
- [x] Record targeted test pass for cleanup and detection tests.
- [x] Add Accounts UI entry for previewing and confirming unsupported-check-in account cleanup.
- [x] Run full frontend build, Go test suite, Windows GUI build, and browser smoke after UI hookup.
- [x] Run sensitive-information scan after final changes.

### Phase 86: Stronger NewAPI / OneAPI / Sub2API recognition
- [x] Add New API /api/about and check-in JSON response signals.
- [x] Treat disabled check-in messages as supportsCheckin=false.
- [x] Add One API model/self API signals without inferring check-in support.
- [x] Add Sub2API /api/v1 and /v1beta gateway route signals.
- [x] Prevent Sub2API / OneAPI / OpenAI-compatible from being treated as check-in-capable by default.
- [x] Re-run full regression after frontend cleanup entry lands.
- [x] Keep future upstream claims tied to official source or captured real response samples.

### Phase 87: Final cleanup-entry verification and handoff
- [x] Fix Accounts capability-card nesting so Key export and check-in cleanup are sibling panels.
- [x] Add smoke assertion that the Accounts page renders .unsupported-cleanup-panel.
- [x] Verify frontend build, targeted cleanup/detection tests, full Go regression, npm audit, Windows GUI build, browser smoke, and sensitive scan.
- [x] Stop temporary smoke-test RelayCheck process and keep real data/relaycheck.db untouched.
- **Status:** complete

### Phase 88: Workspace cleanup and project structure整理
- [x] Archive inactive root-level implementations and old workspace notes to `_archive/2026-06-24-workspace-cleanup/`.
- [x] Preserve root launchers `启动RelayCheck.bat` and `静默启动RelayCheck.vbs` as the user-facing desktop entry points.
- [x] Move dated progress reports to `docs/reports/`.
- [x] Remove generated smoke runtimes, screenshot test-results, old `dist/relaycheck-next.exe`, frontend `tsconfig.tsbuildinfo`, root npm cache, and generated archive caches.
- [x] Add root `README.md` cleanup boundary and active project entry.
- [x] Add `docs/PROJECT_STRUCTURE.md` and link it from `README.md`.
- [x] Re-run frontend build and Go regression after cleanup.
- [x] Record final cleanup verification results.
- **Status:** complete
