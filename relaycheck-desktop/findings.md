# 发现与决策

## 2026-06-21 最新发现
- 2026-06-21 阶段 84 延续：T5.1 继续推进，从 main.tsx 提取出一批组件和 hooks：
  - `ChannelTable` → `components/channels/ChannelTable.tsx`，纯展示组件，由 props 接收 channels/loaded/message 和回调，filters 通过 `ChannelFiltersResult` 传入
  - `useChannelActions` → `hooks/useChannelActions.ts`，封装 channels/accounts/modelOverview 加载、模型同步、渠道状态变更（归档/恢复/批量）
  - `useChannelFilters` → `hooks/useChannelFilters.ts`，封装渠道搜索/筛选/分页，包含 rawJson 解析和账号关联搜索
  - `ImportDialogs` → `components/import/ImportDialogs.tsx`，包含 `LegacyConfigImport`（旧版 config_site*.json 导入）和 `ChromePasswordImport`（Chrome/Via 密码 CSV 匹配导入预览+确认）
  - `Channels` 函数从 ~280 行减到 ~70 行，复用 hooks + ChannelTable
  - 清理死代码 ~400 行，main.tsx 从 4822 行降至 4422 行
  - `npm run build` 通过，TypeScript 无错误
- 剩余 T5.1 项评估：`AuthModal`/`BrowserAuthPanel` 在 main.tsx 中无对应组件（授权为 AccountCard 内联按钮）；`AccountCard`（~310 行）依赖 Accounts 闭包和 15+ 辅助函数，提取代价较大且不属于原 list 命名项；`newapi_signin` 项属冻结遗留不解冻。当前 feasible 的 T5.1 项已全部完成。
- 2026-06-21 阶段 83：目标提示词 A3 第五项完成，`frontend/src/lib/constants.ts` 现集中管理导航元信息、状态集合、目标中转类型、rawJson 搜索白名单、重要通知关键词、Dialog focus selector、列表加载更多阈值和 API Key 过期阈值；`main.tsx` 继续保留页面状态与业务流程 helper，前端构建通过。下一步适合开始页面级拆分，优先从 Dashboard 或 HubRadar 这类边界相对清晰的区域切入。
- 2026-06-21 阶段 82：目标提示词 A3 第四项完成，`frontend/src/lib/labels.ts` 现集中管理错误分类、诊断等级、渠道/同步/审计/调度/签到/登录态/API Key/用量趋势/价格层级等纯标签映射，并迁移 `formatAPIKeyTestMessage`；`main.tsx` 保留导航意图和 Action Center 快捷动作等行为逻辑，前端构建通过。下一步适合继续提取 `lib/constants.ts`。
- 2026-06-21 阶段 81：目标提示词 A3 第三项完成，`frontend/src/lib/format.ts` 现集中管理时间/时长/字节/置信度/JSON 预览、余额与数字、价格来源和价格比较格式化；`formatAPIKeyTestMessage` 暂留 `main.tsx`，因为它依赖尚未抽离的 `apiKeyStatusLabel`，等 labels 工具抽离后再迁移更稳；前端构建通过。下一步适合继续提取 `lib/labels.ts`。
- 2026-06-21 阶段 80：目标提示词 A3 第二项完成，`frontend/src/api/client.ts` 现集中管理 `ApiError`、`api()`、读请求缓存和全局 API 错误订阅/发布；`main.tsx` 只导入 `api` 与 `subscribeApiErrors`，原 1500ms GET 缓存、非缓存前缀、非 GET 清缓存和 `credentials: "same-origin"` 行为保持不变；前端构建通过。下一步适合继续提取 `lib/format.ts`。
- 2026-06-21 阶段 79：目标提示词 A3 第一项完成，`frontend/src/types/index.ts` 现集中导出 65 个前端类型；`main.tsx` 改用 `import type { ... } from "@/types"`，`navItems` 使用 `satisfies readonly NavItem[]` 保持运行时数组和显式 `TabKey`/`LineIconName` 类型一致；前端构建通过。下一步适合继续提取 `api/client.ts`。
- 2026-06-21 阶段 77：根目录 `目标/` 文件夹包含两份新提示词（组件架构、UI/UX 美化），已新增 `目标/TARGET_PROMPT_CHECKLIST.md` 作为对应执行清单；后续完成这些目标项时也要同步打勾。
- 2026-06-21 阶段 77：正式前端 `cn.ts` 已升级为 `clsx + tailwind-merge`，并配置 `@/*` alias；本地 UI primitives 现包含 Button/Card/Badge/Input/Select/Skeleton/Dialog。
- 2026-06-21 阶段 78：目标提示词 A2 UI 原子组件已补齐，`components/ui/*` 现包含 Button、Card、Badge、Input、Select、Skeleton、Dialog、Progress、Tooltip、Switch；前端构建通过。
- 2026-06-21 阶段 77：`relaycheck-hub npm run build` 已通过。此前失败路径包括损坏的 `caniuse-lite` 传递依赖、Prisma Client 未生成、Prisma 7 enum/model 类型不再按旧方式导出、历史页面隐式 any；均已按最小范围修复。
- 2026-06-21 阶段 76：`relaycheck-hub` 已同步 SQLite 可靠性基线。主库通过 `src/lib/sqlite-tuning.ts` 集中应用 WAL、`busy_timeout=5000`、`synchronous=NORMAL`、`temp_store=MEMORY`、`cache_size=-20000`、`foreign_keys=ON`；Prisma better-sqlite3 adapter 显式传入 `timeout: 5000` 并保持进程级 singleton。外部 NewAPI SQLite 只读导入只继承 5000ms timeout，不强制修改用户外部数据库 WAL/pragma。
- 2026-06-21 阶段 74：Tailwind `@theme` 已桥接到 V4 token，V4 `:root` 已补齐语义色、状态背景、输入/focus、骨架、字号、字重、字距、间距、圆角和阴影 token；当前只完成活跃 V4 层第一批替换，历史 `--rc-*` / `--linear-*` / 早期 token 层仍需后续继续收敛，因此不提前勾选完整“单一来源”大项。
- 2026-06-21 阶段 75：V4 活跃层第二批硬编码收敛完成，导航激活色、移动密度覆盖、全局错误条、fatal error 卡和 JSON preview 已继续改用 V4 语义色、字号、圆角和阴影 token；前端构建通过。
- 渠道搜索已覆盖账号显示名、邮箱和用户名：通过渠道/账号站点 Base URL 或站点名关联，只索引非秘密标识字段，不读取密码、Cookie、Token 或 API Key 明文。
- 遗留前端缺陷项复核完成：`UnlockGate/doUnlock`、`createAuthDraft`、`handleSaveBrowserAuth/filteredIds`、`checkinApi.batch`、`.trim('|')` 在正式版源码未命中，属于提示词清单中的遗留/外部实现复核项，不适用于当前 `relaycheck-desktop`。
- 账号页已补关键字段排序：最近签到、余额、API Key 响应时间和 ID 均可正/倒序，空值有兜底排序，清空筛选恢复默认最近签到优先。
- Dashboard 首屏健康徽章已变为可点击入口：显示“系统健康：良好 / N 项需关注”，有问题时直达最高优先级 Action Center 目标页并携带筛选，无问题时刷新自检。
- 大列表加载更多已落地：渠道默认 24 条、签到历史默认 40 条、通知默认 30 条，筛选变化会重置显示数量，避免 500+ 行时一次性渲染全部匹配项。
- 前端通用 loading 骨架已落地：`LoadingSkeleton` 覆盖 panel/table/chart 三类场景，并用于启动页、Dashboard、Hub Radar、渠道列表、通知列表和签到状态；`prefers-reduced-motion` 下 shimmer 静态化。
- 通知页“清空已读”已补确认：未读通知保留，但已读历史删除不可恢复；这覆盖清理类误触保护，不等同于批量删除确认。
- 设置页已补充帮助入口和能力图例：用户可以在本地页面看到 README、主清单、设计系统、接力说明的位置，也能直接理解 NEW/ONE/SUB/MOD、Key 有效、模型可用、raw_json/live 等 chip 含义。
- 账号凭据和删除确认已补齐：清空 API Key 保存前会确认，账号卡和“本地地址疑似误匹配”快捷列表删除账号前会确认；当前正式版渠道主要是归档/恢复，没有真实渠道删除入口，因此不勾选“删除渠道弹确认”。
- 渠道搜索 `note` / `platform` 覆盖已完成：前端会解析 `rawJson`，白名单提取备注、说明、平台、供应商、分组和类型等描述性字段；刻意不做完整 `rawJson` 全文索引，避免 password/token/cookie/API key 等潜在敏感字段进入搜索文本。
- 前端全局错误体验已增强：API 失败会显示持久错误条并提供安全重试，后端 `errorClass` 会映射为中文分类；React 渲染异常由 `AppErrorBoundary` 接管，避免整页白屏。
- Python 遗留版冻结边界已文档化：`newapi_signin/DEPRECATED.md` 明确 SQLite 调优、`print()` 结构化日志改造、吞异常重构都不再回迁旧运行时，后续以正式版 `relaycheck-desktop` 为准。
- 审计缺口已补齐：浏览器授权打开/保存/断开、Key 脱敏导出预览、Admin API/SQLite/legacy/Chrome 密码导入都会写入 `audit_log`，且 metadata 只记录数量/布尔状态/资源 ID，不记录明文凭据。
- 凭据加密复核已完成：新增测试确认密码、Cookie、Access Token、Refresh Token、API Key 均以 AES-GCM `v1.<nonce>.<ciphertext>` 形式落盘，导出预览只包含指纹，不返回明文密钥或其他凭据。
- 签到每站点最小间隔限流已落地：批量手动签到和自动签到共用 `siteMinIntervalSeconds`，默认同站点连续账号至少间隔 2 秒；配置读取时夹取到 `0..60` 秒。
- API 错误响应已增加稳定 `errorClass`：现有中文 `error` 继续保留给 UI 展示，`errorClass` 提供机器可判定分类，避免后续前端/自动化只能解析错误文案。
- 签到临时失败重试已落地：网络错误、HTTP 408/429/5xx 会对同一候选签到接口最多尝试 3 次，并用 100ms、200ms 的指数退避间隔；401/403、404/405 和普通 4xx 不重试。
- 签到结果现在带 `retryCount`，结果消息和 `checkin_logs.message` 会标注“已自动重试 N 次”，方便用户区分最终失败和自动恢复的临时波动。
- 用户要求“把提示词弄进目录，每完成一个就对应标记完成”；已新增 `relaycheck-desktop/PROMPT_CHECKLIST.md` 作为提示词主清单，后续完成项必须同步勾选。
- 请求 ID 与结构化 HTTP 访问日志已落地：响应头返回 `x-request-id`，合法传入值会沿用，日志 JSON 包含 requestId/method/path/status/statusClass/durationMs，且测试确认不记录请求体/Authorization。
- P0 安全与可观测性收口已完成一批：本地 Host 校验、安全响应头、SSRF 默认拒绝内网/metadata、审计日志、设置页审计卡、`/api/health` 和批量 limit clamp 已落地。
- `/api/health` 是刻意免登录的轻量启动/烟测端点，但仍受 Host 校验保护；业务 `/api/*` 路由仍维持 session 保护。
- 外部上游请求默认使用安全 URL 策略；只有明确可信的本地探测路径允许 loopback/private 地址，避免误伤本地 NewAPI 扫描。
- 批量外部动作统一 clamp 到 1..10，减少误操作时对上游站点/账号的瞬时压力；Admin API `pageSize` 不是并发数，统一限制到 10..100。
- 设置页“审计日志”只展示最近 12 条，避免变成长日志页；完整只读 API 仍返回最近 100 条。
- Windows 上 `go test` 偶发出现 `TempDir RemoveAll cleanup`，定向复测通过，后续全量复测也通过；判断为临时目录/SQLite 句柄释放时序问题，不是业务断言失败。
- P0 顶层治理已完成第一批：根目录现在明确 `relaycheck-desktop` 是正式版，`newapi_signin` 是冻结遗留，`relaycheck-hub` 是实验性 MVP。
- 根目录重复启动器已清理：只保留 `启动RelayCheck.bat` 和 `静默启动RelayCheck.vbs`；静默启动器统一设置 `RELAYCHECK_NO_OPEN=1`。
- 遗留 Python `run.py` 已移入 `legacy/run.py` 并标记 deprecated，保留兼容和迁移参考，不删除旧数据库。
- 正式显示名已收敛为 `RelayCheck Desktop v1.0`，`/api/system/status` 和设置页“关于 / 版本”都会显示版本、构建时间、调度器和上次自检摘要。
- 当前同时检测到本机 `3000` 和 `3001` 服务，后续测试必须明确使用 `127.0.0.1:3001` 作为桌面正式版，避免误测实验性 Hub。
- 模型检测底座已经存在：账号 Key 检测会访问 `/v1/models`，解析模型 ID，并选取一个模型走 `/v1/chat/completions` 做可用性和延迟测试。
- 本轮把这个底座升级为可见产品功能：新增模型概览、模型同步、Key 脱敏导出预览。
- 安全边界：脱敏导出只返回 Key 指纹、站点、账号、模型和测速状态，不返回 `api_key_encrypted` 解密值，也不暴露真实 Key。
- 价格比较已从“模型名分层”升级到可提取 NewAPI 导入渠道 `raw_json` 内的真实配置来源：`model_ratio`、`completion_ratio`、`model_mapping`、`pricing`、嵌套 JSON 字符串等都会输出字段路径和置信度。
- 渠道模型同步已落地：`/api/channels/models/sync` 会优先用已加密保存的渠道 Key 请求 `/v1/models`；没有 Key 或实时接口不可用时，会回退解析 NewAPI channels 原始配置和 `model_mapping`。
- 用量分析已落地轻量版本：`/api/usage/overview` 只基于本地余额快照估算趋势、日消耗和低余额风险，不主动访问外部站点，适合本地工具长期轻量运行。
- 站点 `/api/pricing` 在线探测缓存已落地：由用户主动点击同步，缓存到 `site_pricing_cache`，并与账号 Key 可用性/延迟组合成模型价格雷达。
- `modeloc.com` 已作为产品参考处理：公开抓取/搜索没有稳定可读内容；继续吸收“模型检测、价格、延迟、测速”的检测维度，但不把用户 Key 发送到第三方站点。
- 仍不做真实 Key 明文导出；Key 导出继续只输出脱敏指纹、状态、模型和测速摘要，避免安全风险。
- 当前架构仍保持轻量：没有引入新依赖，没有更换技术栈，新增 Go 文件聚合逻辑和少量 React/CSS。

## 需求
- 用户希望继续优化升级 RelayCheck Hub。
- 工具必须轻量、稳定、可直接使用。
- NewAPI 后台同步、渠道合并、识别解释、账号/授权管理是当前核心链路。
- 敏感数据不能明文落入源码或临时文件。

## 研究发现
- 当前项目是 Go 后端 + embedded React/Vite 前端 + SQLite。
- 已有 NewAPI 实例列表、同步接口、同步预览接口。
- 同步预览目前能区分 new / changed / unchanged / skipped，但还不能识别本地存在而源端已经不存在的渠道。
- `imported_channels` 通过 `(local_instance_id, source_channel_id)` 保持同一来源渠道唯一。
- 本轮升级后，同步预览可额外识别 removed：本地存在但本次源端 channels 没有返回的渠道。
- 使用当前本地 NewAPI 实例实测：预览返回 51 条，其中新增 0、变更 37、不变 0、跳过 2、源端已移除 12。
- removed 渠道现在可通过 mark-missing 持久化标记为 missing；被源端重新返回的渠道会在后续导入/同步时恢复为 active。
- 实测 mark-missing：37 条 active，12 条 missing，源端本次返回 39 条。
- 渠道页现在支持搜索、同步状态筛选、后台类型筛选、单个恢复/归档、批量归档 missing、批量恢复 archived。
- 实测批量状态切换闭环：12 条 missing -> archived，再 12 条 archived -> active，最后 mark-missing 恢复为 12 条 missing。
- 渠道页默认筛选已改为日常视图 not_archived；仍可切换到全部含归档或只看已归档。
- Playwright UI 验证：默认 current=49，切换 missing 后 current=12，missing 卡片=12，批量归档按钮可见。
- 系统自检接口 /api/system/diagnostics 已可返回数据库、实例、渠道、账号、签到和通知诊断项。
- 实测自检接口：11 个诊断项，整体 danger，3 个 warning/danger 项；总览页 UI 自动化显示 11 张诊断卡。
- 自检卡片现在支持点击定位；missing-channels 会跳到渠道页并自动筛选 sourceStatus=missing。
- Playwright 验证：点击“存在源端已移除渠道”后进入渠道页，filterValue=missing，missingCards=12。
- 账号页支持 problem 筛选，签到页支持 problem 筛选，通知页支持 unread 筛选；自检卡片可带对应意图跳转。
- Playwright 验证：点击“存在需要处理的账号登录态”进入账号页 accountFilter=problem；点击“存在未读通知”进入通知页 notificationFilter=unread。
- 设置页与备份恢复已完成：可查看数据库路径、备份目录、运行架构、备份数量、设置 JSON。
- 备份会先执行 WAL checkpoint，再复制 relaycheck.db 到 data/backups，文件名格式为 relaycheck-YYYYMMDD-HHMMSS-reason.db。
- 恢复只允许使用 data/backups 目录中的 .db 文件；恢复前会自动创建 before-restore 快照，恢复后重新打开数据库并执行迁移、默认管理员、默认设置检查。
- 恢复过程已增加失败自动回滚原数据库文件的保险，避免新库打开/迁移失败时工具失去当前数据库。
- API 验证：创建备份成功，恢复成功，恢复后 /api/system/status 继续返回 port=3001。
- Playwright 验证：设置页 title=设置，备份行=2，设置编辑器=3，页面不是空白。
- 处理建议中心已完成：/api/system/action-center 返回按优先级排序的只读建议项。
- 当前实测处理建议：overall=danger，共 4 类；授权失效账号 10，余额缺失账号 17，源端已移除渠道 12，不可达站点 6。
- Playwright 验证：总览页处理建议中心显示 4 张卡片，点击“优先处理失效授权”跳转到账号页并自动筛选 problem。
- Debug 发现：核心只读列表接口均正常，但 GET /api/channels/:id 和 GET /api/accounts/:id 返回 405；已补齐详情 handler。
- Debug 数据一致性：channels=49，sites=40，accounts=26；orphan_accounts=0，orphan_sites_channel=0，duplicate_site_base=0，unknown_channels=0。
- Playwright 导航 debug：总览、渠道、上游站点、账号、签到、余额、通知、本机扫描、设置 9 页均可打开；consoleErrors=[]，pageErrors=[]。
- 当前最新签到日志日期为 2026-06-18，/api/checkins/today 返回 0 属于当前数据状态，不是接口错误。
- 正式回归测试通过：go test、npm build、14 个只读 API、渠道/账号详情接口、9 个 UI 页面、关键筛选、站点详情抽屉、移动端基础布局均通过。
- Playwright 回归结果：actionCards=4，diagnosticCards=11，missingCards=12，problem accounts visibleItems=10，balance cards=5，settings backups=2。
- 当前最新构建已隐藏重启：ProcessId=37872，端口 3001，渠道 49，账号 26，处理建议 4 类。
- 2026-06-19 本轮联网参考：NN/g Visual Hierarchy 指出层级主要来自色彩/对比、尺寸、分组；本轮对应到账号卡的左侧状态轨、56px 主头像、分区数据和操作层。
- 2026-06-19 本轮联网参考：Material Design 3 Elevation 建议少量 elevation 层级并保持一致；本轮只在账号卡、操作 deck、编辑面板使用有限阴影/表面层。
- 2026-06-19 本轮联网参考：Atlassian Elevation 强调 surface + shadow 用于引导焦点，且 elevated UI 要克制；本轮没有继续堆叠强阴影，而是用边框、背景和分组。
- 2026-06-19 本轮联网参考：Material Design/Atlassian 等设计规范倾向于固定图标尺寸层级，主品牌/对象头像大于导航小图标，状态/操作图标作为次级辅助。
- 2026-06-19 本轮联网参考：表单编辑应避免隐藏关键上下文，字段说明和错误提示应靠近输入项；敏感字段留空保留原值，降低误覆盖风险。
- 2026-06-19 agent-reach 状态：Exa 搜索未配置，doctor 显示 web/Jina Reader、GitHub、YouTube、B站/V2EX/RSS 可用；本轮使用内置网页搜索兜底获取设计参考。
- 2026-06-19 账号卡片层次感加强完成：身份层、数据层、操作层分开；左侧状态轨用于扫读异常账号；余额作为更高权重指标显示。
- 2026-06-19 当前视觉尺寸层级：账号主头像 56px，主操作按钮 42px，次级操作按钮 36px，登录状态徽章约 30px。
- 2026-06-19 Playwright 验证最新账号页：cards=26，identities=26，metricBalance=26，primaryGroups=26，secondaryGroups=26，dangerZones=26，desktop/mobile overflowX=false，errors=[]。
- 2026-06-19 产品痛点：账号卡长期显示太多维护按钮会干扰日常使用；已改为默认紧凑，只保留签到、刷新余额、网页登录和更多。
- 2026-06-19 产品痛点：同一中转站有多个账号，改 URL 的影响范围必须明确；已增加 current/shared 修改范围，默认 current 避免连带改坏其他账号。
- 2026-06-19 产品痛点：同步 NewAPI 需要多步点击；已增加“一键同步”流程，自动完成导入/更新和源端移除标记。
- 2026-06-19 产品痛点：重要信息和不重要信息缺少大小差；当前账号名 21px、余额 17px、密钥/时间芯片 10.5px，扫读优先级更明确。
- 2026-06-19 GitHub 参考：`qixing-jk/all-api-hub` 定位为 New-API/Sub2API account hub，核心卖点是余额/用量、自动签到、密钥使用、价格比较、健康检查和渠道管理；本轮只借鉴产品信息架构，不复制代码。
- 2026-06-19 产品痛点：签到页不应让成功日志淹没失败日志；已将成功/今日已签折叠成汇总卡，失败/需授权/不支持置顶显示。
- 2026-06-19 产品痛点：余额要按站点看总量，同一中转站多个账号需要汇总；已新增站点余额汇总，单位不同分开显示。
- 2026-06-19 产品痛点：通知中心未读太多会掩盖重要失败；已默认只显示重要通知，普通成功/info 通知收纳。
- 2026-06-19 产品痛点：倒计时和同步频率只有配置还不够，必须有真实后台任务；已新增轻量 scheduler，启动后自动计划签到和 NewAPI 同步。
- 2026-06-19 实测调度状态：`/api/system/scheduler-status` 返回 `checkin.daily` 和 `sync.local_newapi` 两个任务，均为 scheduled，并有 nextRunAt。
- 2026-06-19 实测签到状态：`/api/checkins/status` 当前 running=false，dueAccounts=18，schedule.nextRunAt 已使用 scheduler 随机后的真实执行时间。
- 2026-06-19 Playwright 验证：设置页“后台调度器”显示 2 个任务卡；桌面和 390px 移动端无横向溢出，consoleErrors=[]，pageErrors=[]。
- 2026-06-20 用户反馈：账号卡和信息条幅仍偏大、偏长；已将账号卡桌面列宽固定到 330px，头像 48px，标题 18px，操作按钮 34px，顶部信息条最大 680px。
- 2026-06-20 `qixing-jk/all-api-hub` README 确认其功能参考点：多站点资产总览、余额/用量、自动签到、Key 库、模型价格比较、模型/Key 可用性测试、渠道/模型同步和导出集成。
- 2026-06-20 GitHub CLI 复核 `qixing-jk/all-api-hub`：仓库描述继续强调 balance/usage dashboard、auto check-in、one-click keys、price comparison、health checks、advanced channel management；最新 release 观察为 v3.47.0，发布于 2026-06-16。本轮据此把总览升级为 `AI API Hub Radar`，但只复用本地现有 API，不引入重依赖或第三方 Key 提交。

## 技术决策
| 决策 | 理由 |
|------|------|
| 增加 removed 预览状态 | 让用户知道后台已移除但本地仍保留的渠道 |
| 不在同步中自动删除 removed 渠道 | 避免误删账号、日志、余额快照等本地历史 |
| 继续复用现有 sync_preview.go | 改动集中，风险小 |
| 使用 missing 标记而不是删除 | 保留本地历史和账号绑定，后续可做归档/恢复 |
| 批量管理用后端单次 UPDATE | 比前端循环请求更快、更稳定，也更容易限制安全状态流转 |
| 默认隐藏 archived | 让日常渠道列表更干净，同时保留可恢复入口 |
| 自检项返回 action 文本 | 先用轻量文字建议，后续可升级为点击跳转和自动筛选 |
| 前端用 NavigationIntent 传递筛选意图 | 不增加路由库，保持轻量，同时能跨页面定位 |
| 工作台筛选先前端实现 | 当前数据量小，前端筛选足够快，避免新增复杂后端分页 |
| 备份恢复用标准库文件复制和 SQLite checkpoint | 不引入新依赖，符合轻量化目标 |
| 恢复只允许备份目录文件 | 降低误恢复任意路径或恶意路径的风险 |
| 设置保存前后端都校验 JSON | 避免坏配置写入导致后续读取失败 |
| 处理建议中心只读，不自动操作账号/渠道 | 当前阶段先提升定位效率，避免误触发网页登录、签到或渠道归档 |
| 建议卡片后端只返回 target/filter，前端映射 NavigationIntent | 保持后端轻量，不把前端状态细节写死到数据库聚合层 |
| 补齐详情接口时复用列表字段结构 | 保证 API 兼容，同时减少前端或外部工具处理差异 |
| 账号编辑站点 URL 采用“改 URL 则迁移当前账号” | 避免同一上游站点的多个账号被一次编辑连带改坏 |
| 图标层级采用品牌/对象头像 > 导航图标 > 状态徽章 | 提升视觉主次，保持轻量，不引入图标依赖 |
| 账号卡片用三段层级而不是继续堆按钮 | 让用户先识别站点和账号，再看状态/余额，最后处理操作 |
| 主操作和危险操作物理分区 | 降低误删概率，也让日常签到/余额/网页登录更容易扫到 |
| 账号卡默认紧凑 | 日常场景高频动作只有签到、余额和网页登录，其他维护项折叠可减少认知负担 |
| 站点 URL 修改默认只影响当前账号 | 同一站点多账号是核心场景，默认安全范围应最小 |
| NewAPI 同步提供一键路径，同时保留预览差异 | 新手需要快，熟练用户需要可控；两条路径并存 |
| API Key 模型检测只直连用户配置的上游 | 避免把 Key 交给第三方检测网站；本地工具只请求对应中转站的 `/v1/models` 和最小 chat 测试 |
| Key 检测结果只保存摘要，不保存完整模型响应 | 控制 SQLite 体积，也减少上游响应里潜在敏感内容长期留存 |
| Key 修改后清空旧模型检测摘要 | 同一账号可能替换 Key，旧模型数量/延迟继续显示会误导用户 |
| 当前没有账号级保存 API Key | 页面“有密钥 0”是数据状态，不是功能失败；新增 Key 账号后卡片才会显示 Key 摘要 |
| 全局代理作为 system_settings 保存 | 不增加新表和新依赖，符合轻量化目标，同时可被设置页 JSON 与结构化 UI 同时管理 |
| 代理默认绕过 localhost/127.0.0.1 | 避免本地 NewAPI 扫描、桌面端 API、Chrome CDP 被错误转发到外部代理 |
| 代理地址不支持用户名密码 | 避免凭据出现在 Chrome 启动参数、进程列表或诊断输出中 |
| 站点探针使用有限并发 | 保持识别速度，同时避免对上游站点瞬时请求过多 |
| 设置页移动端必须检查横向滚动 | 桌面看起来正常时，grid item 的 min-content 仍可能在 390px 宽度撑破布局 |
| 登录失败信息应该服务于下一步操作 | 单个 `HTTP 404` 不足以判断是路径不兼容还是账号密码错误；列出全部候选路径能让用户知道该改登录 URL 还是切换网页登录授权 |
| 批量动作验证优先小样本 | 余额刷新、签到、识别都会访问外部站点；调试时先用 `limit=1`，避免长时间真实请求阻塞整个验证 |
| 渠道页默认聚焦 NewAPI/Sub2API 体系 | 用户明确只关心基于 NewAPI/Sub2API/魔改 NewAPI 搭建的中转站，官方供应商和纯兼容 API 不应占据日常视图 |
| 备份恢复和备份清理要分离 | 恢复保持单个备份操作，删除旧备份才提供多选，避免误触发多个恢复动作 |
| 同步频率先落设置再接调度器 | `sync.schedule` 默认 30 分钟先可视化保存，后续后台 scheduler 可直接读取该配置 |
| scheduler 不引入 cron 依赖 | 30 秒轻量轮询足够满足本地工具需求，减少依赖和包体积 |
| 自动签到计划时间落库 | 随机延迟需要跨重启稳定，否则重启后可能改变执行点并重复触发 |
| 定时 NewAPI 同步不导入 Key | 定时任务应低风险、低噪音；渠道 Key 导入继续留给用户手动确认 |
| 账号卡不再自动撑满整行 | 多账号场景应优先提升扫读密度，卡片固定上限比等分拉伸更稳定 |
| `all-api-hub` 作为功能参考而非 UI 模板 | 当前项目是本地桌面 Web 面板，保留轻量架构，同时借鉴其资产、Key、模型、价格和同步能力 |
| 模型测速排行必须由用户主动触发检测后展示 | 自动测速可能消耗额度或触发上游限制；页面只展示已有检测摘要，单账号/批量检测由用户点击触发 |
| 能力卡应短宽协调而不是铺满页面 | 用户明确要求不要无脑拉长；账号页顶部应采用短卡片、轻边框、柔和状态色来突出关键数字 |
| 成功状态需要与问题状态并列展示 | 只展示异常会造成“全是问题”的感知；有效 Key、可用模型和成功测速是日常判断工具是否可用的关键信息 |
| 桌面信息区适合横向流式布局 | 用户明确希望信息多时横放，不要全部往下；桌面 flex-wrap 能保持短卡片同时提高首屏信息量 |
| 签到成功与失败应使用一致行节奏 | 成功、失败都需要适当展开，用同款短行能快速对比账号、站点、时间和状态，同时避免列表过重 |
| shadcn/ui 适合作为视觉系统参考而非依赖迁移 | 当前项目追求轻量和稳定，追加 CSS token/组件覆盖比引入 Tailwind/Radix 依赖风险更低 |
| 长筛选条会削弱后台卡片层次 | 筛选条最大 820px、通知行最大 640px、设置标题最大 560px 后，桌面首屏更像卡片工作台而不是表单堆叠 |
| 用系统 Chrome 做 UI smoke 更轻量 | Playwright 自带 Chromium 未安装，不下载额外浏览器，改用本机 Chrome 能验证真实桌面环境并控制空间占用 |
| RelayCheck 更适合 Control Room 而不是官网风格 | 用户管理的是本地 NewAPI/账号/签到/余额状态，核心是排障和扫读，不是营销展示；视觉应低噪音、高密度、问题优先 |
| 设计系统必须写入项目文档 | 多轮 UI 迭代容易变成零散覆盖，`DESIGN_SYSTEM.md` 能约束后续 agent 保持短卡片、状态优先和轻量架构 |
| 顶部栏应承担运行状态而不只是标题 | 本地工具最关键的是当前端口、数据库/架构和重要通知，把这些放在 topbar 能减少去设置页确认的频率 |
| 中等屏不能过早把操作卡拉满 | 操作卡和诊断卡应在桌面/平板宽度保持短卡片扫读节奏，只在 560px 以下切换单列全宽；1180px 全宽只适合签到、通知、同步等长内容块 |
| 前端项目必须覆盖全局 package-lock 配置 | 用户机器全局 npm 配置禁用了 package-lock，导致 `npm audit fix` 无法工作；项目级 `.npmrc` 能保证 RelayCheck 自己可复现安装和审计 |
| SQLite 渠道导入保留 `SELECT *` | 这是为了读取 NewAPI/OneAPI 魔改 schema 的所有字段并保存 rawJson，符合“schema introspection、不要写死字段”的需求；常规业务查询仍应显式列字段 |
| ID 生成不能因为随机源异常直接崩溃 | `crypto/rand` 仍是主路径，但本地工具在极端环境下应降级而不是 panic；时间戳+原子计数兜底保持 32 位十六进制格式 |
| 总览需要资产雷达而不是继续堆列表 | all-api-hub 类工具的核心价值是资产、Key、成本、自动化一眼看清；Dashboard 新增四张短卡能把已有功能入口聚合起来，同时不增加后端复杂度 |
| 当前 Windows Go 环境下 race 测试需文档化为阻塞项 | `go test -race` 在当前未启用 cgo 的 Windows 工具链中会返回 `-race requires cgo`；README 已写明本地回归门槛为 `go test -mod=vendor ./...`，后续只有显式启用 cgo 后才把 race 重新纳入必跑项 |
| 触屏尺寸用粗指针媒体查询保护 | 桌面端需要高密度控制台，不应全局放大所有小按钮；在 `@media (pointer: coarse)` 下统一提升 44x44px 能兼顾手机/平板可点性和桌面扫读密度 |
| Dashboard 网格不应固定 2/3 列 | 中等宽度下固定列容易挤压卡片；`auto-fit/minmax` 能让 Dashboard 图表和诊断卡自然换行，同时保留大屏多列 |
| 移动端单列只应用到主要内容 | 侧边导航已经有横向紧凑条以减少首屏占用；主要内容网格单列能降低横向溢出风险，但导航不应被同一规则误伤 |
| 正式版表格多为 grid 行 | 当前前端没有原生 `<table>`；“表格列宽弹性”应落在详情、通知、审计、备份、同步结果和日志等 grid 行的子项收缩与长文本换行上 |
| 动画 keyframes 应集中维护 | 当前正式版只需要 `panel-in` 和 `skeletonShimmer` 两个共享关键帧；集中在全局 motion 区域能避免后续 finishing layer 重复定义 |
| 图标升级不需要立即引入 lucide 依赖 | 当前前端依赖保持轻量，未安装 `lucide-react`；阶段 70 用内联线性 SVG 达成“非 emoji 图标 + 文字”，避免为了单个视觉项增加包体和维护面 |
| 状态表达不能只依赖颜色 | 阶段 71 将渠道、账号、调度、审计、同步和设置页高频状态统一接入 `StatusLabel`，用线性图标形状 + 中文文字承载成功/警告/失败语义，颜色只作为辅助 |
| 重要数字要统一等宽并靠前扫描 | 阶段 72 用集中 CSS 覆盖层把 Dashboard、Radar、渠道、账号、签到、通知、余额、详情、同步和调度指标纳入 `tabular-nums`，降低数字跳动和横向扫读成本 |
| Tailwind 正式保留，shadcn 不作为运行时依赖 | 阶段 73 复核正式版已有 Tailwind v4 构建链，因此保留 Tailwind；同时清理 CSS 中 shadcn 注释残留，并在设计系统中明确不新增 Radix/shadcn 运行时依赖 |
| hub 外部 SQLite 导入不强制 WAL | 用户选择的 NewAPI SQLite 是外部数据源，hub 只读导入时可使用 5000ms timeout 改善锁等待，但不应改写外部库的 journal/synchronous/temp/cache 等运行策略 |
| 前端 `cn()` 使用 `clsx + tailwind-merge` | 目标提示词要求 Tailwind 类冲突可解算；这是低风险基础设施升级，也为后续 UI primitives 和页面拆分铺路 |
| UI primitives 先补底座再接业务页面 | 先补 Progress/Tooltip/Switch 等低风险组件并构建验证，避免在拆 main.tsx 时同时引入新组件和页面逻辑变化，降低回归定位难度 |
| hub Prisma enum 类型本地化 | 当前 Prisma 7 生成客户端在本环境未从 `@prisma/client` 导出旧 enum 类型名；用 `src/lib/prisma-enums.ts` 集中声明字符串 union，避免散落 `any` 或降低 strict |

## 遇到的问题
| 问题 | 解决方案 |
|------|---------|
| Playwright 等待通知页标题时误写“通知中心” | 实际标题为“通知”，修改测试等待条件后通过 |
| 本机没有 sqlite3 CLI | 改用临时 Go 脚本读取 SQLite，检查后删除临时文件 |
| GET 详情接口 405 | 为渠道和账号补齐 GET 单条详情接口 |
| Playwright 等待隐藏 option 超时 | 改为等待页面标题和筛选 select 可见，再读取 select value |
| Playwright 找不到“账号”导航 | 导航按钮实际文本含图标前缀 `AC账号`，测试从精确匹配改为包含“账号” |
| Playwright 自带浏览器未安装 | 不下载大浏览器，改用本机 Chrome 可执行文件验证，保持轻量 |
| `wxls.ccwu.cc` 直连易被误判不可达 | 增加全局代理后，`https://wxls.ccwu.cc/` 通过 127.0.0.1:7897 返回 HTTP 200，并能识别为 modified_relay |
| `wxls.ccwu.cc` 串行探针过慢 | 将探针改为有限并发后，真实识别耗时从约 37s 降至约 4.1s |
| 设置页移动端 hero/路径/备份行横向溢出 | 给 panel 子项、settings-grid 子项、detail-list、backup-row 添加 min-width/max-width/换行约束后 Playwright 验证 mobileOverflow=false |
| 处理建议“查看问题”Playwright 初测误判未跳转 | 测试脚本用 `innerText` 查 placeholder，placeholder 不属于 innerText；改用真实输入框 locator 后确认跳转正常 |
| 批量余额刷新单账号失败只显示最后一个 404 | 改为聚合所有尝试过的登录路径，返回 `/api/user/login`、`/api/login`、`/api/auth/login` 的失败原因，并提示修正账号卡登录地址或使用网页登录授权 |
| Playwright 点击“渠道”匹配到自检卡片 | 改用 `aside nav button` 精准定位导航 |
| Playwright 等待 select option 可见超时 | 原生 option 是隐藏节点，改为等待搜索框并读取 select value |
| Playwright 点击“设置”匹配到诊断卡片 | 改用 `nav button` 且 filter 文本为“设置”，避免严格模式多匹配 |
| shadcn 紧凑覆盖首次大 patch 上下文不匹配 | 拆成较小 patch，先定位 finishing layer，再只追加覆盖规则，避免误改原样式 |
| Playwright 默认 Chromium 缺失 | 不安装大浏览器，使用 `C:\Program Files\Google\Chrome\Application\chrome.exe` 作为 executablePath 完成巡检 |
| `ui-ux-pro-max` 脚本路径不可直接执行 | `scripts` 在当前安装中表现为文件/不可进入路径；改为使用已读取的设计原则，并把落地规则写入 `DESIGN_SYSTEM.md` |
| PowerShell 当前找不到 `tar` | 改用系统临时目录 `npm install --ignore-scripts` 检查 Prisma adapter 和 better-sqlite3 包源码，确认 adapter config 会透传 `timeout` 给 better-sqlite3 |
| hub build 缺 `caniuse-lite/data/agents.js` | 普通 `npm install` 未补回缺文件；单包重装 `caniuse-lite@latest` 后恢复，随后 Next build 进入正常 TypeScript 检查 |

## 资源
- 本地应用地址：http://127.0.0.1:3001
- 桌面端路径：dist/relaycheck.exe
- 数据库路径：data/relaycheck.db

## 视觉/浏览器发现
- 上一轮 Playwright 验证同步预览 UI 可正常显示新增、变更、不变、跳过数量。

---
*每执行2次查看/浏览器/搜索操作后更新此文件*
*防止视觉信息丢失*


---

## 2026-06-24 调研与实现发现：签到支持识别、Sub2API 识别、账号清理

- 外部项目调研来源：QuantumNous/new-api、songquanpeng/one-api、Wei-Shaw/sub2api 官方仓库源码。结论只作为识别策略依据，不复制外部项目代码。
- New API 识别结论：
  - 可使用 /api/about、/api/status、/api/user/self 等后台 API 信号辅助识别。
  - 签到相关接口以 /api/user/checkin 为核心候选；JSON 字段如 checked_in_today、quota_awarded、checkin_date、min_quota、max_quota、total_checkins 可作为“这是签到接口”的强信号。
  - 若返回“签到功能未启用 / 未开启签到 / 未启用签到 / 不支持签到”，应识别为 NewAPI 面板但 supportsCheckin=false。
- One API 识别结论：
  - 主要信号来自 /api/user/self、/api/user/available_models、登录和模型相关 API。
  - 默认不应因为 One API 面板存在而推断支持签到。
- Sub2API 识别结论：
  - Sub2API 更像订阅转 OpenAI/Gemini 网关和 /api/v1 后台，不应误判为支持签到。
  - 即使页面没有 sub2api 品牌文本，也可通过 /api/v1/auth/login、/api/v1/settings/public、/api/v1/user/profile、/v1/models、/v1beta/models 等路由组合识别。
  - Sub2API 可支持模型/余额类能力探测，但签到支持默认保持 false，除非未来发现明确且可验证的签到接口。
- 产品决策：
  - “删除不支持签到的账号”必须先提供 dry-run 预览，不直接操作真实库。
  - 后端删除范围默认包含站点级 supports_checkin=0，也支持包含账号上次签到状态为 unsupported 的记录。
  - 删除账号时同步删除该账号的 checkin_logs 和 balance_snapshots，避免列表和统计残留。
  - 真实数据清理必须走备份、预览、确认、执行四步，不允许直接 SQL 删除真实库。
- 当前风险：
  - 前端入口尚未接入，用户暂时无法在 UI 中确认预览/执行。
  - 当前接口按批量 limit 执行，适合分批清理；如果真实库中有大量账号，需要 UI 明确展示“本次最多清理 N 个”。
  - 上游项目接口可能继续变化；后续新增识别规则必须以官方仓库或真实响应样本交叉验证。


---

## 2026-06-24 Implementation finding: cleanup UI must stay dry-run first

- Accounts cleanup is now reachable from the Accounts capability area, but the first-class path remains preview-first. This matches the product decision that unsupported-check-in account deletion is irreversible enough to require explicit confirmation.
- Browser smoke now includes a structural assertion for .unsupported-cleanup-panel, so future refactors that accidentally remove the cleanup entry should fail fast.
- Full Go regression exposed a known Windows sqlite/temp-directory cleanup flake once; the affected test passed in isolation and the final full run passed. Treat this as environment-level flakiness unless it repeats in three consecutive runs or starts failing assertions.
- Sensitive scan result remains clean for active source and docs except the deliberate fake key fixture in internal/core/secrets_security_test.go.

## 2026-06-24 Cleanup finding: archive inactive implementations, keep active runtime data

- The active product boundary is now explicit: `relaycheck-desktop/` is the maintained implementation; old Python, old standalone Vite, and Next.js experiment material is archived under `_archive/2026-06-24-workspace-cleanup/`.
- Root launchers are not junk: both point at `relaycheck-desktop/dist/relaycheck.exe`, so they remain as user-facing convenience entry points.
- `frontend/dist/` is generated but operationally important for Go compilation because `main.go` uses `//go:embed frontend/dist`; do not remove it unless the next step is to run `npm run build` before Go compilation.
- `data/relaycheck.db` remains outside cleanup. Any destructive real-data operation still needs backup, dry-run preview, explicit confirmation, and API-mediated execution.
- Generated archive caches such as Next `.next`, `node_modules`, local `data`, and Python `__pycache__` provide no source value after archival and can be deleted to avoid Git traversal warnings and reduce clutter.
- Post-cleanup verification passed with `npm run build`, `npm audit --audit-level=low`, `go test -mod=vendor ./...`, and `go build -mod=vendor -ldflags="-H windowsgui" -o dist\relaycheck.exe .`.
- The Windows reserved-name `nul` entries could not be removed through normal PowerShell path APIs, but were successfully deleted with Node's long-path file API.

## 2026-06-24 Cleanup finding: destructive batches need server-side "has more" truth

- A fixed `limit=10` cleanup is safe, but not enough operationally: after deleting one batch the operator needs to know whether another preview is expected.
- Returning `hasMore` from the same candidate query is lower risk than adding an exact total count. The cleanup query already knows whether row 11 exists, and the UI only needs a continuation hint rather than an expensive exact total.
- `matched` remains the current batch count, not the global match count. This keeps delete confirmation text honest: the user confirms the visible batch, then repeats preview/delete if `hasMore=true`.
- The dry-run-first rule still stands. Real cleanup should continue to be backup -> dry-run preview -> explicit confirmation -> API delete, and never direct SQL against `data/relaycheck.db`.
- Browser smoke is useful for this surface because the cleanup card is dense. The latest smoke verified that the added batch hints do not create horizontal overflow on desktop or 390px mobile.
