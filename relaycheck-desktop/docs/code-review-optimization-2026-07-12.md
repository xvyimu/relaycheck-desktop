# RelayCheck Desktop · 全栈代码审查与优化报告

**日期：** 2026-07-12  
**范围：** `E:\zidqiandao\relaycheck-desktop`（Go 1.24 + embed React 19 / Vite 8 + SQLite）  
**约束：** 无 Radix/shadcn；不自动登录 / 不绕 2FA；本机 loopback 运维台定位  
**方法：** 源码审读 + 关键路径核对（`account_api_client.go`、`ChannelsPanel` 尾部 import、CSS 体积、调度/HTTP 安全）+ 子域并行审查  

**栈摘要**

| 层 | 技术 |
|----|------|
| 后端 | Go `net/http`，`//go:embed frontend/dist`，`modernc.org/sqlite`，`internal/{core,accounts,sites,channels,...}` |
| 前端 | React 19、Vite 8、Tailwind v4、自研 `components/ui/*`、~10k 行自有 CSS |
| 发布 | `scripts/verify-release.ps1` → `package-release.ps1` → zip + SHA256 + operator-launch |

---

## 总览优先级

| 级别 | 数量 | 建议窗口 |
|------|------|----------|
| **P0** | 0（未发现可远程 RCE / 明文密码落日志类致命项） | — |
| **P1** | 12 | 1–2 个迭代内 |
| **P2** | 10+ | 随后打磨 |

**优先修（收益/成本比最高）**

1. 后端：`AccountAPIClient.Do` 与 `DoWithTimeout` 统一 SSRF 校验 + 出站 `CheckRedirect`  
2. 后端：`beginSchedulerJob` 原子互斥，防签到双跑  
3. 前端：尾部 `import { Button }` 归位 + 隐藏 Tab 暂停数据层  
4. 配置：`RELAYCHECK_DATA_DIR` / exe 旁 `data` + 打包 ldflags 版本元数据  
5. 门禁：`verify-release` 增加 `npm run lint` + embed HTML smoke  

---

## 1. 前端代码审查

### 1.1 已做得好的点

- 面板 `React.lazy` + Vite `manualChunks`（`panel-*`）；Dashboard 硬载、Analytics 默认折叠避免 idle 轮询  
- `idle-tabs` 5 min TTL + pinned dashboard，有单测  
- `useApi` AbortController；GET 读缓存对带 `signal` 请求不入缓存  
- 自研 `DialogShell`（Escape / Tab 环 / 焦点还原）；无 `dangerouslySetInnerHTML` / `eval`  
- 主题 / 更新横幅只用 localStorage 非密钥；密码控件不落盘  

### 1.2 发现项

#### FE-1 · `import { Button }` 挂在文件末尾（ghost 迁移残留）

| 项 | 内容 |
|----|------|
| **问题描述** | 多文件在 `export const X = memo(...)` **之后** 才 `import { Button }`。已核实 `ChannelsPanel.tsx:329-330`；同类约 20+ 文件（Settings / Sites / UpdateBanner / AccountForm 等）。ESM hoist 后运行可过，但违反模块惯例，易被 lint/工具链误伤。 |
| **影响评估** | **P1** — 可维护性；diff 噪音；后续静态分析可能报错。 |
| **推荐操作** | 1）`rg -n "import \{ Button" frontend/src -g'*.tsx'` 列出位置。<br>2）全部挪到文件顶部 import 区（脚本或批量编辑）。<br>3）ESLint 启用 `import/first`（或 `@typescript-eslint` 等价）。<br>4）`npm run lint && npx tsc -b && npm test`。 |
| **预期收益** | 迁移痕迹清除；CI/阅读成本下降。 |
| **验证** | 任意组件文件末尾无 import；lint 0 error。 |

#### FE-2 · 隐藏 Tab 用 `display:none` keep-alive，子树 effect 仍可能跑

| 项 | 内容 |
|----|------|
| **问题描述** | `main.tsx`：`visitedTabs` 内面板保持挂载，仅 `display` 切换。访问过且未过 TTL 的 Channels/Sites 等仍占用内存；内部 `useApi` / 轮询 / 刷新路径在不可见时仍可能工作。`PRUNE_INTERVAL_MS=30s` 才回收。 |
| **影响评估** | **P1** — 长会话内存与后台网络线性上升。 |
| **推荐操作** | 1）非 active 容器加 `inert` + `aria-hidden`。<br>2）数据 hook 接收 `active` prop，仅 active 时轮询/订阅。<br>3）可选：切走 pause 数据层、UI state 进 session map。<br>4）Performance：连点 5 Tab 后对比 heap / Network。 |
| **预期收益** | 长时间开着桌面更稳，后台噪音下降。 |
| **验证** | 切离渠道页后该页 API 轮询停止（DevTools）。 |

#### FE-3 · 渠道数据双重拉取

| 项 | 内容 |
|----|------|
| **问题描述** | `useInventoryData`（App 启动即拉 channels/sites/accounts）与 `ChannelsPanel` 内 `useChannelActions.refresh` 再拉 channels/models/accounts 重叠。 |
| **影响评估** | **P1** — 启动 + 进渠道重复 RTT；1.5s 读缓存只能部分抵消。 |
| **推荐操作** | 1）Channels 消费 `inventory.*` props，仅增量拉 health/models overview。<br>2）或抽 `useChannelsBundle` 单源。<br>3）集成测：mock fetch 断言 channels 请求次数。 |
| **预期收益** | 进渠道网络近减半，状态一致。 |
| **验证** | 冷启动→点渠道，channels/accounts 有效请求不双倍。 |

#### FE-4 · CSS 层过重 + recovery 垫底（~10k 行）

| 项 | 内容 |
|----|------|
| **问题描述** | 自有 CSS 合计约 **10076 行**；`control-room.css` ~30KB 已合并 redesign/layout/linear；`recovery.css` ~20KB 仍在 `styles.css` **最后**导入，与 v4 token 形成「谁最后谁赢」契约。stub 旧层文件易混淆。 |
| **影响评估** | **P1** — 包体/解析、改样式猜层序、视觉回归靠肉眼。 |
| **推荐操作** | 1）recovery 选择器引用审计：死规则删，能并入 base/domain 的并入。<br>2）文档唯一真相源：tokens → base → layout → domain；layers 标删除日期。<br>3）对比 build CSS 体积；保留 `smoke:layout`。 |
| **预期收益** | 样式可预期；CSS 体积与心智负担下降。 |
| **验证** | `npm run build` gzip CSS 下降或持平且 smoke:layout 过。 |

#### FE-5 · `Settings.tsx` 单体 ~900 行

| 项 | 内容 |
|----|------|
| **问题描述** | 单文件承载系统/代理/备份/导入导出/调度/版本等；任一 `setBusy` 整页重渲染；高危操作与展示混杂。 |
| **影响评估** | **P1** — 误改与不可测风险。 |
| **推荐操作** | 拆 `SettingsProxy` / `SettingsBackup` / `SettingsExportImport` / `SettingsSchedules`（已有 SiteSchedules 可对齐）；export/import 补 vitest（confirm + api mock）。 |
| **预期收益** | 设置域改动半径缩小；高危流可单测。 |
| **验证** | 备份/导入路径仍 confirm；单测覆盖取消/成功。 |

#### FE-6 · DialogShell 缺 scroll-lock / 标题关联；测试过薄

| 项 | 内容 |
|----|------|
| **问题描述** | 有 focus trap 与 Escape，但打开不锁 `body` 滚动；仅 `aria-label` 无 `aria-labelledby`；`dialog-shell.test.ts` 仅 export 契约。 |
| **影响评估** | **P1** — a11y/UX；回归无防护。 |
| **推荐操作** | 1）open → `body.overflow=hidden`，cleanup 还原。<br>2）`titleId` → `aria-labelledby`。<br>3）RTL + userEvent：Tab 环、Esc、backdrop、焦点还原。 |
| **预期收益** | 抽屉行为可证明；键盘体验达标。 |
| **验证** | 新单测绿；手动：抽屉开时滚轮不带动背后列表。 |

#### FE-7 · 大面板测试空洞

| 项 | 内容 |
|----|------|
| **问题描述** | ~27 测试文件偏 `lib/*` / 部分 accounts；**无** ChannelsPanel / SitesPanel / Settings 行为测。 |
| **影响评估** | **P1** — 导航 intent、健康探测完成刷新等高价值路径无护栏。 |
| **推荐操作** | 优先：Channels 探测 done→refreshAll；Sites `intent.accountsView`；Settings restore 取消。串联现有 Playwright smoke。 |
| **预期收益** | CSS/Dialog 重构不怕静默破坏主路径。 |
| **验证** | 新增测 + `npm test`；可选 `npm run smoke`。 |

#### FE-8 · 导出密码 state 生命周期与弱校验（P2）

| 项 | 内容 |
|----|------|
| **问题描述** | Settings 导出/导入密码 `length < 6`；成功后未见立即清空 state。 |
| **影响评估** | **P2** — 共用机/录屏小窗口；弱口令导出包。 |
| **推荐操作** | 成功/卸载清空密码；文案对齐后端策略。 |
| **预期收益** | 敏感操作后内存更干净。 |

#### FE-9 · 裸 `<button>` 与 `<Button>` 混用（P2）

| 项 | 内容 |
|----|------|
| **问题描述** | ghost 已迁；主操作/danger 仍大量原生 button，视觉与 focus ring 两套体系。 |
| **影响评估** | **P2** |
| **推荐操作** | 运营主路径 toolbar 统一 Button；destructive 映射 `.danger`。 |
| **预期收益** | Control Room 控件一致。 |

#### FE-10 · 切 Tab 焦点逃逸（P2）

| 项 | 内容 |
|----|------|
| **问题描述** | `handleTabChange` 不处理焦点；隐藏 `display:none` 控件上的焦点可能落到 body。 |
| **影响评估** | **P2** |
| **推荐操作** | 切 Tab 焦点移到 `main` 地标；非 active `inert`。 |
| **预期收益** | 键盘流连续。 |

#### FE-11 · CSP 与 FOUC（已修，记入资产）

| 项 | 内容 |
|----|------|
| **问题描述** | 曾内联 theme 脚本被 `script-src 'self'` 拦截。 |
| **影响评估** | 已 **P1→关闭**（`public/theme-bootstrap.js`）。 |
| **推荐操作** | 保持外链；`visual-smoke-theme.mjs` 回归。 |
| **预期收益** | 深浅色首屏无闪、无 CSP 报错。 |

---

## 2. 后端代码审查

### 2.1 已做得好的点

- 固定 `127.0.0.1` + Host 白名单 + CSP / `X-Frame-Options`  
- 状态变更 Origin 校验（CSRF 基线）+ 相关单测  
- AES-GCM 凭据落盘；密钥文件权限意识；访问日志不落 Authorization/body  
- SQLite WAL、`busy_timeout`、`foreign_keys`、参数化 SQL、性能索引  
- SSRF：`url_safety` / `ValidateOutboundURL` 存在（但路径不一致，见下）  
- 调度 `rootCtx` + WaitGroup；`AccountAuthRepository.LoadBatch` 消 N+1  
- 领域包 `accounts/sites/channels/...` 经接口挂到 `App`  

### 2.2 发现项

#### BE-1 · `AccountAPIClient.Do` 绕过出站 URL 校验

| 项 | 内容 |
|----|------|
| **问题描述** | `internal/core/account_api_client.go:28-45`：`Do` 用 `normalizeBaseURL(auth.BaseURL)+path`，**不**调用 `safeNormalizeBaseURL`。`DoWithTimeout`（47–70）则正确校验。签到主路径 `CheckinExecutor` 走 `Do`。污染的 `base_url`（导入/改库）可打到内网/metadata。 |
| **影响评估** | **P1** — SSRF 策略不一致；签到为高频自动路径。 |
| **推荐操作** | 1）`Do` 与 `DoWithTimeout` 统一 `safeNormalizeBaseURL`。<br>2）禁止 path 以 `//` 或绝对 URL 逃逸。<br>3）单测：私网 base_url 在 `Do` 上失败。 |
| **预期收益** | 签到/会话与 Key 探测同等 SSRF 防护。 |
| **验证** | `go test ./internal/core -run 'URL|AccountAPI|SafeNormalize' -count=1`（补测后）。 |

#### BE-2 · 出站 HTTP 无 `CheckRedirect` 限制

| 项 | 内容 |
|----|------|
| **问题描述** | `network.go` 客户端默认跟随重定向；公网 302 → `169.254.169.254` 可绕过首次 DNS/IP 检查。 |
| **影响评估** | **P1** |
| **推荐操作** | 自定义 `CheckRedirect`：每跳重校验 host/IP；或探测类禁止跟随。单测 302→私网拒绝。 |
| **预期收益** | 补齐 `url_safety` redirect 缺口。 |
| **验证** | 新增 redirect SSRF 单测绿。 |

#### BE-3 · 无真实本地会话：`requireSession` ≈ 信任本机全体进程

| 项 | 内容 |
|----|------|
| **问题描述** | 通过 Origin/Host 后会话恒为本地信任模型；能连 `127.0.0.1:port` 即可调 export/import/账号等 API。 |
| **影响评估** | **P1**（多用户机/本机恶意软件）；与产品「信任本机」定位相关，需文档+可选加固。 |
| **推荐操作** | 1）可选 bootstrap 解锁口令 + HttpOnly session。<br>2）敏感写操作二次确认已在 UI，后端可再加确认 token。<br>3）RUNBOOK 写明威胁模型。 |
| **预期收益** | 缩小本机横向面；预期与实现一致。 |
| **验证** | 未解锁时写 API 401；解锁后 Origin 仍强制。 |

#### BE-4 · SQLite 导入接受任意本机路径

| 项 | 内容 |
|----|------|
| **问题描述** | import 路径 `Abs` 后只读打开，无 scan-root allowlist；本机 API 可读任意可达 `.db`。 |
| **影响评估** | **P1** |
| **推荐操作** | 限制在配置的 scan roots；`EvalSymlinks` + 前缀检查；`importKeys=true` 显式确认。 |
| **预期收益** | 降低任意 SQLite 读取面。 |
| **验证** | 拒绝 `C:\Windows\...` 类路径单测。 |

#### BE-5 · 系统设置 GET 返回含通知密钥密文的 `value_json`

| 项 | 内容 |
|----|------|
| **问题描述** | GET settings 全量 `value_json`；通知 botToken 等为密文，与 `instance.key` 同盘时近于可解密材料。 |
| **影响评估** | **P1** |
| **推荐操作** | GET 仅 masked + `configured:true`；PUT 空字段=保留旧值。 |
| **预期收益** | 减小设置 API secret 暴露。 |
| **验证** | GET 响应无 raw 密文字段；前端仍显示「已配置」。 |

#### BE-6 · `beginSchedulerJob` 非 CAS，签到可重叠

| 项 | 内容 |
|----|------|
| **问题描述** | `scheduler.go`：`beginSchedulerJob` 对 running 未做「仅非 running 才抢锁」；sync 有内存 TryStart，**checkin 弱**。全局 tick 与 per-site 可能叠加。 |
| **影响评估** | **P1** — 重复签到、锁竞争、通知风暴。 |
| **推荐操作** | `UPDATE ... WHERE status <> 'running'`，`RowsAffected==0` 跳过；全局与 per-site 共用互斥；并发单测。 |
| **预期收益** | 调度幂等。 |
| **验证** | `go test ./internal/core -run Scheduler -count=1` + 新并发测。 |

#### BE-7 · Chrome CDP 调试端口暴露于 loopback（P2）

| 项 | 内容 |
|----|------|
| **问题描述** | browser login 使用 remote-debugging-port；窗口期本机其它进程可窃 cookie。 |
| **影响评估** | **P2**（桌面自动化固有风险） |
| **推荐操作** | 缩短端口存活；Save 后立即关浏览器；文档警告。 |
| **预期收益** | 缩短窃取窗口。 |

#### BE-8 · `Decrypt` 非法密文返回 `("", nil)`（P2）

| 项 | 内容 |
|----|------|
| **问题描述** | 损坏密文被当成「未配置」，调度反复失败难诊断。 |
| **影响评估** | **P2** |
| **推荐操作** | 区分空字段 vs 非法密文；非法 → `manual_required` + 通知。 |
| **预期收益** | 可运维的凭据诊断。 |

#### BE-9 · 无 graceful HTTP shutdown（P2）

| 项 | 内容 |
|----|------|
| **问题描述** | `main.go` `Serve` 阻塞，无 signal → `Shutdown`。 |
| **影响评估** | **P2** |
| **推荐操作** | SIGINT/SIGTERM → Shutdown → `app.Close()`。 |
| **预期收益** | 减少半写任务/WAL 残留。 |

#### BE-10 · Dashboard 多次 COUNT（P2）

| 项 | 内容 |
|----|------|
| **问题描述** | summary / action center 多轮独立 COUNT；索引已有，大库仍多 RTT。 |
| **影响评估** | **P2** |
| **推荐操作** | 单 SQL 多列聚合或拉长 `cachedRead` TTL + 写路径 invalidate。 |
| **预期收益** | 大库仪表盘延迟下降。 |

#### BE-11 · API 无版本前缀、错误消息不稳定（P2）

| 项 | 内容 |
|----|------|
| **问题描述** | `/api/...` 无 `/v1`；部分 `err.Error()` 直出前端。 |
| **影响评估** | **P2** |
| **推荐操作** | 新接口 `/api/v1`；对外错误码表 + 内部 log 细节。 |
| **预期收益** | 契约与排障成本下降。 |

---

## 3. 整体架构建议

### 3.1 架构优点

- **单二进制桌面形态正确**：embed SPA + loopback + 纯 Go SQLite，贴合本地运维台。  
- **领域拆分方向对**：`internal/{sites,channels,accounts,...}` + core 装配；任务/SSE 集中。  
- **前端 IA 已收敛**：7 Tab；IA-2 账号并入「站点与账号」；lazy + idle + chunks。  
- **发布链完整**：verify → package（manifest/SHA256）→ operator-launch/acceptance。  

### 3.2 建议项

#### AR-1 · `*App` 仍是超大装配根

| 项 | 内容 |
|----|------|
| **问题描述** | `internal/core/app.go` 挂大量 service；新逻辑易继续堆在 `App`。 |
| **影响评估** | **P1** — 回归面大。 |
| **推荐操作** | 冻结：`App` 不再加业务字段；新逻辑进 domain Service；core 覆盖率目标 ≥55%。 |
| **预期收益** | 变更半径可控。 |

#### AR-2 · 数据目录绑定进程 CWD

| 项 | 内容 |
|----|------|
| **问题描述** | `NewApp(".")` → `data/` 相对 CWD；双击 exe 或错误工作目录会「像丢库」。 |
| **影响评估** | **P1** |
| **推荐操作** | 默认 `filepath.Join(exeDir, "data")`；`RELAYCHECK_DATA_DIR` 覆盖；启动日志打绝对路径；RUNBOOK 写死启动方式。 |
| **预期收益** | 安装/升级路径稳定。 |

#### AR-3 · 发布门禁未充分验证 **embed 后 UI**

| 项 | 内容 |
|----|------|
| **问题描述** | binary smoke 偏 API；浏览器 smoke 常打 Vite dev，而非 exe 内嵌 dist。 |
| **影响评估** | **P1** — embed/CSP/index fallback 易漏（已有 CSP 前科）。 |
| **推荐操作** | health 后 GET `/` 断言 assets；可选 `RELAYCHECK_SMOKE_BASE` 跑 `visual-smoke-theme.mjs`。 |
| **预期收益** | 发布物 = 用户所见 UI。 |

#### AR-4 · 遗留 navigation intent 名

| 项 | 内容 |
|----|------|
| **问题描述** | `accounts`/`balances` 仍映射到 sites；须只经 `navigation.ts`。 |
| **影响评估** | **P2** |
| **推荐操作** | 全局搜非法 `setTab("accounts")`；TabKey 单一来源；smoke 覆盖 Action sample → master-detail。 |
| **预期收益** | Deep-link 一致。 |

---

## 4. 配置优化

#### CF-1 · 版本未走 ldflags

| 项 | 内容 |
|----|------|
| **问题描述** | `productVersion` 源码常量 + 正则抠版本；`go build` 仅 `-H windowsgui`。 |
| **影响评估** | **P1** — 版本漂移；`local build` 无追溯。 |
| **推荐操作** | `var productVersion/buildTime/gitCommit` + `-X` 注入；`-s -w`；package-release 与 manifest 同源。 |
| **预期收益** | 可追溯发布；体积略降。 |

#### CF-2 · 门禁缺 lint；govulncheck 依赖外网代理

| 项 | 内容 |
|----|------|
| **问题描述** | `verify-release` 有 test/build/audit，**未**强制 `npm run lint`；govulncheck 常需代理。无项目级 CI workflow。 |
| **影响评估** | **P1** |
| **推荐操作** | Frontend build 前 `npm run lint`；govulncheck 失败显式 WARN；可选 GHA windows job。 |
| **预期收益** | 本地与远程门禁一致。 |

#### CF-3 · Env 面窄且文档不齐；git 代理卡住 push

| 项 | 内容 |
|----|------|
| **问题描述** | 代码：`RELAYCHECK_PORT` / `NO_OPEN`；脚本：`PROXY` / smoke password；文档另有 bootstrap password。无 `DATA_DIR`。本机 git `http.proxy=7897` 常死，提交易只留本地。 |
| **影响评估** | **P1**（协作/备份风险） |
| **推荐操作** | README Env 表；实现 `RELAYCHECK_DATA_DIR`；push：`git -c http.proxy= -c https.proxy= push` 或先起 Clash。 |
| **预期收益** | 启动可文档化；解除 commits 积压。 |

#### CF-4 · embed 依赖先 `npm run build`

| 项 | 内容 |
|----|------|
| **问题描述** | `frontend/dist` gitignore；裸 `go build` 易嵌旧/失败。 |
| **影响评估** | **P2** |
| **推荐操作** | `scripts/build-desktop.ps1`：`npm ci && npm run build && go build`；正式 zip 禁止随意 `-SkipBuild`。 |
| **预期收益** | 消除半套构建。 |

#### CF-5 · 当前发布物（本地已打）

| 项 | 内容 |
|----|------|
| **产物** | `dist/releases/relaycheck-desktop-1.1.0-00529ee1ca70-20260712-145035.zip` |
| **SHA256** | `8b8b7efe3e018e9c6b0026043f30ecf44de2441a151fe7b6c3cb7a7d674a01a3` |
| **校验** | `verify-package` PASS |
| **Git** | 本地 `main` 可能 **ahead origin**（网络/7897）；以 `HANDOFF.md` 为准 |

---

## 5. 建议落地路线图

| 阶段 | 项 | 预估 |
|------|----|------|
| **S0 立刻** | FE-1 尾部 import；BE-1/BE-2 SSRF；BE-6 调度 CAS | 0.5–1.5d |
| **S1 本周** | AR-2 DATA_DIR；CF-1 ldflags；CF-2 lint 入门禁；AR-3 embed smoke | 1–2d |
| **S2 随后** | FE-2/3 keep-alive+双拉；FE-5/6/7 Settings 拆分与测；BE-4/5 路径与 settings mask | 3–5d |
| **S3 中期** | AR-1 App 冻结+覆盖率；BE-3 可选解锁；FE-4 recovery 瘦身；CI | 按排期 |

---

## 6. 统一验证命令

```powershell
cd E:\zidqiandao\relaycheck-desktop

# 后端
go test -mod=vendor -count=1 ./internal/core/ ./internal/accounts/ ./internal/channels/ ./internal/sites/
go test -mod=vendor ./internal/core -run "Security|SecureLocal|URL|Scheduler" -count=1

# 前端
cd frontend
npx tsc -b
npm test
npm run lint
npm run build

# 发布门禁（完整，需时较长）
cd ..
powershell -NoProfile -File scripts\verify-release.ps1 -SkipBrowserSmoke   # 或完整跑
powershell -NoProfile -File scripts\package-release.ps1
powershell -NoProfile -File scripts\verify-package.ps1

# 视觉（需 dist\relaycheck.exe + frontend playwright）
# 隔离 runtime 起 exe 后：
node scripts\visual-smoke-theme.mjs --base http://127.0.0.1:3015 --out .tmp\visual-smoke
```

---

## 7. 结论

RelayCheck 作为 **本机 loopback 运维台**，绑定/Host/CSRF 基线/凭据加密/SQLite pragma/发布拉链已经明显用心，前端 lazy+idle+DialogShell 方向正确。  

当前主要短板不在「框架选型」，而在：

1. **安全路径一致性**（`Do` vs `DoWithTimeout`、redirect、导入路径、settings 密文面）  
2. **调度互斥与桌面长会话性能税**（双拉、keep-alive effect）  
3. **工程卫生**（尾部 import、巨型 Settings、recovery CSS、ldflags/DATA_DIR/门禁）  

按 S0→S1 执行即可在 **不大改产品行为** 的前提下抬高安全底线与可维护性；S2 起再拆面板与瘦 CSS。

---

*报告路径：`docs/code-review-optimization-2026-07-12.md`*  
*未自动改业务代码；需要落地某条时指定 ID（如 `BE-1`）即可。*
