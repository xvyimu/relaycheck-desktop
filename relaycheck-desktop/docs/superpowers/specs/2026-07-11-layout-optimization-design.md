# RelayCheck Desktop 布局优化设计

**日期：** 2026-07-11  
**状态：** 设计完成 · 方案 α 待实施  
**范围：** 桌面端壳层 + 仪表盘 / 站点 / 账号主运营面  
**作者：** 布局优化规划（守岸人会话）  
**关联：** `DESIGN_SYSTEM.md` · `docs/PROJECT_STRUCTURE.md` · `e12aca5`（accounts `upstreamSiteId` 过滤已上线）

---

## 0. 约束冲突检查（首要过滤）

| 约束 | 来源 | 方案必须满足 |
|------|------|----------------|
| Control Room：冷静、紧凑、精准、低噪 | `DESIGN_SYSTEM.md` | 不引入营销式大留白或装饰布局 |
| 中高密度桌面 + 390 不横滚 | 设计系统 A11y | 断点 900 / 820 / 560 行为可预期 |
| 不新增 Radix/shadcn 运行时依赖 | 设计系统 Implementation | 仅用现有 `components/ui/*` + CSS |
| 状态不只靠颜色 | 设计系统 Hierarchy | 继续用 `StatusLabel` + 文案 |
| 触控目标 ≥44×44（粗指针） | 设计系统 | 侧栏已 44px；卡片主按钮不低于 32px 桌面 / 粗指针区再抬升 |
| 后端已有 `?upstreamSiteId=` | `internal/core/accounts.go` | 前端必须接线，禁止重复造过滤 API |
| 本地单用户、无登录层 | `PROJECT_STRUCTURE.md` | 布局不引入鉴权壳 |
| 少即是多 | 本规格 Constrains | 不新增 Tab 数量；β 若合并站点/账号须单独评审 |
| 不写密钥/Cookie/密码进文档 | 安全规则 | 本文仅用匿名 ID 与计数 |

**目标视口（主）：** 1440×900  
**目标视口（辅）：** 1180、900、560、390  
**运行规模（2026-07-11 本机库）：** 上游站点 ~59 · 账号 25 · 多账号站点常见 2–3 条/站

---

## 1. 现状分析

### 1.1 空间骨架与尺度

| 区域 | 实现 | 尺度 / 节奏 | 数据来源 |
|------|------|-------------|----------|
| 应用壳 | `.app-shell` | `grid-template-columns: 245px minmax(0,1fr)` | `styles/layers/recovery.css` |
| 侧栏 | `.sidebar` sticky | 全高；nav 项 `min-height: 44px`；gap 6–7px | `styles/layout.css` + recovery |
| 主区 | `.main-panel` | padding `22px 24px` | recovery |
| Topbar | `.topbar` | 左：eyebrow+标题；右：主题+刷新 | `Topbar.tsx` |
| 卡片节奏 | `--rc-gap: 12px` · `--rc-gap-sm: 8px` | 账号卡 `minmax(≤284px, 304px)` | `layout-harmonization.css` |
| ≤820px | `.app-shell { display:block }` | 侧栏不再固定左列 | `drawers.css` |
| ≤900px | 多网格强制 1 列 | account/channel/site/settings | layout-harmonization |

**示意（当前骨架）：**

```text
┌──────────┬──────────────────────────────────────────────┐
│ 245px    │ main-panel                                   │
│ Sidebar  │ Topbar (标题 + 刷新)                          │
│ 8 Tab    │ 当前 Tab 内容（纵向长卷）                      │
│ 平级列表  │                                              │
└──────────┴──────────────────────────────────────────────┘
```

### 1.2 信息架构（IA）

| Tab key | 中文 | 角色 | 当前数据入口 |
|---------|------|------|--------------|
| dashboard | 仪表盘 | 决策台 | system + inventory + ops + modelUsage |
| channels | 渠道 | 资产 | `/api/channels` |
| sites | 站点 | 资产 | inventory.sites |
| accounts | 账号 | 执行台 | `/api/accounts`（**无 query**） |
| checkins | 签到 | 运营 | ops.checkins |
| scan | 本机扫描 | 工具 | 扫描 API |
| notifications | 通知 | 运营 | ops.notifications |
| settings | 设置 | 系统 | system.status |

- 侧栏 **无分组**（`Sidebar.tsx` 中 `TABS` 单数组平铺）。  
- 类型层已有 `NavItem`（含 icon/description），**UI 未使用分组/图标描述结构**。  
- `NavigationIntent` 字段：`accountStatus` / `siteHealth` / `query` 等；**无 `upstreamSiteId`**。

### 1.3 仪表盘纵向区块（自上而下）

| 序号 | 区块 | 组件 | 职责 |
|------|------|------|------|
| 0 | 更新条 | `UpdateBanner` | 版本提示（条件显示） |
| 1 | Hub Radar | `HubRadar` | 5 张雷达卡（资产/Key/成本/运营/排程） |
| 2 | 指标网格 | `.metric-grid` 5 卡 | NewAPI / 渠道 / 已识别 / 账号 / 未读 |
| 3 | 运营待办 | `dashboard-priority-card` | Action Center 列表 + 处理 |
| 4 | 三卡 | 系统 / 运营 / 调度器 | 元数据与调度摘要 |
| 5 | 分析 | `AnalyticsPanel` | SVG 图表 |

**问题摘要：** 决策主路径（待办「处理」）夹在 Radar 与多层统计之间；Radar 自身已含资产/运营摘要，与 metrics、运营三卡存在 **信息重复（冗余注视点）**。

### 1.4 账号页纵向区块

| 序号 | 区块 | 组件 | 默认状态 |
|------|------|------|----------|
| 1 | 洞察/批量 | `AccountInsights` | 常显，内容重 |
| 2 | 新建表单 | `AccountForm` | 常显完整表单 |
| 3 | 工具条 | 搜索 + 状态 select | **无站点筛选** |
| 4 | 卡片网格 | `AccountCard` × N | 一级操作约 6 个按钮 |

**过滤现状：**

| 能力 | 后端 | 前端 |
|------|------|------|
| `GET /api/accounts?upstreamSiteId=` | ✅ `listAccounts` + 缓存键隔离 + 单测 | ❌ `useInventoryData` 固定 `/api/accounts` |
| 状态「异常账号」 | — | ✅ client `isProblemAccount` + intent |
| 文本搜索 | — | ✅ client |
| intent 带 siteId | — | ❌ `NavigationIntent` 无字段 |

### 1.5 站点页要点

- 筛选：健康（unreachable）+ 文本搜索。  
- 摘要：total / healthy / checkinReady / linkedAccounts。  
- 详情抽屉；**无「查看该站账号」一键 intent**。

### 1.6 主用户任务与动线（当前）

| 任务 | 路径 | 步数（点击/关键决策） | 瓶颈 |
|------|------|----------------------|------|
| 看全局风险 | 打开 → 仪表盘 | 0–1 | 首屏层数 ≥5，待办非顶 |
| 处理异常账号 | 待办「处理」→ 账号 + problem | 1 + 扫视 | 无站点收敛 |
| 按站处理 2 账号 | 站点找站 → 账号页搜索站名 | ≥3 | 服务端过滤未用 |
| 重登过期会话 | 账号 → 卡上「网页登录」 | 2 | 一级按钮过多，目标密度高 |
| 批量测 Key/余额 | 账号页顶 Insights | 1 | 与列表抢首屏 |

### 1.7 相关代码锚点

| 路径 | 用途 |
|------|------|
| `frontend/src/main.tsx` | 壳层、Tab 挂载、intent 分发 |
| `frontend/src/components/layout/Sidebar.tsx` | 8 Tab 平铺 |
| `frontend/src/components/layout/Topbar.tsx` | 页标题 + 刷新 |
| `frontend/src/components/dashboard/Dashboard.tsx` | 仪表盘组装 |
| `frontend/src/components/dashboard/HubRadar.tsx` | 雷达 5 卡 |
| `frontend/src/components/accounts/AccountsPanel.tsx` | 账号列表与工具条 |
| `frontend/src/components/accounts/AccountCard.tsx` | 卡布局与动作分层（不彻底） |
| `frontend/src/hooks/useInventoryData.ts` | 账号全量拉取 |
| `frontend/src/lib/navigation.ts` | Action Center → intent |
| `frontend/src/types/index.ts` | `NavigationIntent` |
| `internal/core/accounts.go` | `upstreamSiteId` 服务端过滤 |
| `internal/core/accounts_list_test.go` | 过滤与缓存隔离测试 |
| `frontend/src/styles/layers/layout-harmonization.css` | 网格与卡片模数 |
| `frontend/src/styles/layers/mobile-density.css` | 小屏密度（偏 `.sidebar-v4`） |

---

## 2. 问题列表（按影响排序）

| ID | 优先级 | 问题 | 客观证据 | 理论依据 | 影响指标 |
|----|--------|------|----------|----------|----------|
| L1 | P0 | 站点轴未进入账号列表 UI | 后端过滤已上线；`useInventoryData` 无 query；工具条无站点控件 | 任务-界面映射缺失；能力闲置 | 按站定位步数；API 利用率=0 |
| L2 | P0 | 仪表盘纵向层数过多且信息重复 | Radar / metrics / 待办 / 三卡 / Analytics 串联；Radar 已含摘要 | 工作记忆 7±2；注视点竞争 | 首屏决策区占比；到「处理」的纵向距离 |
| L3 | P1 | 账号卡一级操作过密 | 常显约 6 个主按钮 +「更多」 | Fitts 定律：目标过密 → 时间与误点↑ | 一级 CTA 数；误点风险 |
| L4 | P1 | 导航 8 项平铺无任务分组 | `TABS` 无分区；`NavItem` 未用 | 格式塔接近性：应同组相邻 | 跨任务寻路时间 |
| L5 | P2 | 账号页顶置 Insights+全量表单 | 列表在第三块之后 | 使用频率分区：高频列表应优先露出 | 首屏可见账号卡行数 |
| L6 | P2 | 跨页筛选维度不一致 | 站点 health / 账号 problem / 渠道多过滤；无统一 siteId intent | 格式塔相似性 | 心智切换成本 |
| L7 | P3 | CSS 类双轨（`.sidebar` vs `.sidebar-v4`） | `mobile-density.css` 主写 v4 | 实现一致性 | 小屏规则命中率 |
| L8 | P3 | HubRadar「余额」导航 target 含 `balances` | `TabKey` 有 balances，侧栏无对应 Tab | 死链 / 空导航风险 | 无效点击率 |

---

## 3. 优化方案

### 3.1 方案 α · 行动优先流线式（**推荐默认实施**）

**侧重点：** 缩短「发现风险 → 执行」；仪表盘=决策台，账号页=执行台。  
**不改：** Tab 数量、路由 key、后端 API 契约（只消费已有 query）。

#### α-A 仪表盘重组

| 区块 | 调整 | 具体参数 | 理论 |
|------|------|----------|------|
| 运营待办 | 上移到 Radar 之后、metrics 之前 | 待办区 `max-height: min(48vh, 420px)`；内部滚动；sticky 标题行可选 | 主任务优先 |
| metrics | 由 5 大卡改为单行 pill / 压缩 metric | 行高 56–64px；gap 8–12px；数字 `tabular-nums` | 降低与 Radar 重复的垂直占用 |
| 系统/运营/调度三卡 | 默认折叠为 1 行摘要 | 折叠高 ≈ 48–56px；`aria-expanded` | 渐进披露 |
| Analytics | 默认折叠或「分析」次级开关 | 默认 `hidden`；本地 `localStorage` 可记展开偏好（非密钥） | 少即是多 |
| Radar | 保持；可选压缩 `hub-radar-grid` 在 ≥1180 为 3+2 | 不删卡，只控高度 | 信息保留 |

**结构示意：**

```text
[Sidebar] │ Topbar: 仪表盘
          │ UpdateBanner?
          │ HubRadar（可压缩）
          │ 运营待办 ← 主决策区
          │ 指标条（单行）
          │ ▸ 系统 / 调度摘要（折叠）
          │ ▸ 分析（折叠）
```

#### α-B 账号页：站点过滤 + 首屏重排

| 元素 | 位置 | 尺寸 / 行为 | 理论 |
|------|------|-------------|------|
| 站点筛选 | 工具条第 1 控件 | `select` 或可搜索下拉；宽 200–280px；选项=`sites`；首项「全部站点」 | 接近性：筛选项同带 |
| 状态 | 第 2 | 宽 120–160px | 与现网一致 |
| 搜索 | 第 3 | `flex:1`；`max-width: 320px` | 次于站+状态 |
| 摘要计数 | 工具条上沿 | 全部 / 异常 / 可见 | 现有保留 |
| AccountForm | 默认折叠 | 触发器「+ 添加账号」；展开后现有表单 | 频率分区 |
| AccountInsights | 默认可折叠 | 折叠态仅保留 1 行批量按钮（测 Key / 刷余额 / …） | 少即是多 |

**数据策略（两档，可连续交付）：**

| 档 | 行为 | 适用 |
|----|------|------|
| α-B1 客户端 | 仍拉全量；`filteredAccounts` 增加 `upstreamSiteId` 条件 | 账号 ≤100 可先上 |
| α-B2 服务端 | `useInventoryData` 或账号专用 hook：`/api/accounts?upstreamSiteId=` | 与缓存键隔离一致；规模增长后默认 |

**Intent 扩展：**

```ts
// NavigationIntent 增量（设计）
upstreamSiteId?: string;
```

| 来源 | 目标行为 |
|------|----------|
| 站点卡「查看账号」 | `onNavigate("accounts", { upstreamSiteId })` |
| Action Center 若未来带 site | 同字段 |
| 清除筛选 | 去掉 siteId + status + query |

#### α-C 账号卡动作分层

| 层级 | 操作 | UI |
|------|------|-----|
| 一级（常显 ≤3） | 网页登录 · 签到 · 详情 | primary 组 |
| 二级（更多） | 保存授权 · 测试登录态 · 刷新余额 · 编辑 · 检测密钥 · 2FA | secondary |
| 三级 | 删除 | danger-zone（保持） |

**参数：** 一级按钮间距 ≥8px；`min-height` ≥32px 桌面；粗指针媒体查询可抬到 40–44px。  
**依据：** Fitts；操作频率分层；服务会话重登主路径（与项目 #9 对齐）。

#### α-D 侧栏轻分组（不增 Tab）

```text
运营
  仪表盘 · 签到 · 通知
资产
  渠道 · 站点 · 账号
工具
  本机扫描 · 设置
```

| 参数 | 值 |
|------|-----|
| 组标题 | 11–12px uppercase / muted；padding-left 与 nav 对齐 |
| 组间距 | 12–16px |
| 组内 gap | 6px（保持） |

**依据：** 格式塔接近性 + 相似任务聚类。

#### α-E 死链修正（随 α 一并）

- HubRadar「余额用量」：导航到 **账号**（query/余额相关）或 **仪表盘分析展开**，在 `balances` Tab 未实现前禁止 `onNavigate("balances")` 空跳。  
- 或：实现最小 balances 面板（**超出 α 默认范围，单独立项**）。

---

### 3.2 方案 β · 资产分区式（站点中轴）

**侧重点：** 以站点为库存主键，账号挂在站下。  
**触发条件：** α 上线后，按站运维占比仍显著高于跨站扫异常；或站点/账号规模继续上升。

#### β-A 主从布局（≥1180px）

```text
[Sidebar] │ Topbar: 站点与账号（可合并 Tab 文案）
          │ ┌─────────────┬──────────────────────────┐
          │ │ 左 32%      │ 右 68%                     │
          │ │ 站点列表    │ 选中站的账号列表            │
          │ │ 筛选+摘要   │ GET ?upstreamSiteId=       │
          │ │ 选中左边框  │ 未选：空态引导              │
          │ │ 3px primary │                            │
          │ └─────────────┴──────────────────────────┘
```

| 参数 | 值 | 说明 |
|------|-----|------|
| 左栏 min-width | 280px | 站名 + health + accountCount |
| 右栏 | `min-width: 0` + flex | 防横溢（设计系统强制） |
| 选中指示 | `box-shadow: inset 3px 0 0 var(--primary)` | 与 nav active 一致 |
| ≤900px | 站点全宽 → 点入账号子视图 | 避免双栏挤压 |

#### β-B IA 调整

| 项 | 内容 |
|----|------|
| Tab | 「站点」升级为「站点与账号」；「账号」降为「全部账号」次入口或合并 |
| 仪表盘 | 仅跨站聚合；「处理」intent 预选 siteId 进入主从右栏 |
| 风险 | 习惯总表用户的学习成本；回归面大于 α |

#### β-C 成本

| 维度 | 相对 α |
|------|--------|
| 组件新建 | 高（MasterDetail 壳） |
| 响应式两态 | 必须 |
| 与现网 AccountsPanel 复用 | 右栏可复用卡片/工具条子集 |

---

### 3.3 方案对照（选型）

| 维度 | 方案 α | 方案 β |
|------|--------|--------|
| 核心隐喻 | 决策流 → 执行列表 | 对象树（站→号） |
| 破坏性 | 低 | 中 |
| 吃到 upstreamSiteId | 是 | 是（主路径） |
| 交付切片 | 可 1–2 个 PR | 需独立里程碑 |
| **默认选择** | **是** | α 后按数据再开 |

---

## 4. 预期效果对比

任务基准：**处理某站 2 个异常账号**（本机库存在多账号站点，如某 siteId 下 2–3 条）。

| 指标 | 现状 | 方案 α 目标 | 方案 β 目标 | 测量方式 |
|------|------|-------------|-------------|----------|
| 决策→可见目标列表点击步数 | 1（intent）+ 搜索/扫视 | 1 + 站点筛选 0–1 | 1（带 siteId） | 任务走查计数 |
| 服务端站点过滤使用 | 否 | 是（α-B2）或客户端等价（α-B1） | 是 | 网络面板 / 单测 |
| 过滤后条数正确性 | N/A | = DB `count by site` | 同左 | API/UI 对照（已有后端 PASS） |
| 账号页首屏是否露出 ≥1 行卡片（1440） | 常被 Insights/表单挤掉 | 是（表单/洞察折叠） | 右栏即列表 | 1440 截图 |
| 单卡一级按钮数 | ~6 | ≤3 | ≤3 | 代码审查 |
| 仪表盘首屏主要决策块数 | ≥5 | ≤3（Radar+待办+指标条） | ≤3 | 结构审查 |
| 侧栏任务分组 | 0 | 3 组 | 3 组 + 可能合并 Tab | UI |
| 无效 `balances` 导航 | 存在 | 0 | 0 | 点击测试 |
| 390 横滚 | 禁止回归 | 禁止回归 | 禁止回归 | smoke / 手工 |

**不采用主观词（好看/舒服）**；验收以步数、块数、按钮数、过滤一致性、断点溢出为准。

---

## 5. 实施建议

### 5.1 阶段划分

| 阶段 | 内容 | 预估改动面 | 依赖 |
|------|------|------------|------|
| **S0** | 本文档评审冻结；锁定 α | 文档 | — |
| **S1** | 账号工具条站点筛选（先 α-B1 客户端）+ Form/Insights 折叠 + 卡动作 ≤3 | `AccountsPanel` `AccountForm` `AccountInsights` `AccountCard` + 少量 CSS | S0 |
| **S2** | `NavigationIntent.upstreamSiteId` + 站点页「查看账号」+ navigation 单测 | `types` `navigation.ts` `SitesPanel` `AccountsPanel` | S1 |
| **S3** | α-B2 服务端过滤接线（可选专用 hook，避免整页 inventory 重拉风暴） | `useInventoryData` 或 `useAccounts` | S1 |
| **S4** | 仪表盘折叠与待办上移 | `Dashboard.tsx` + dashboard CSS | S0 |
| **S5** | 侧栏分组 + 修正 balances 死链 | `Sidebar.tsx` `HubRadar.tsx` | S0 |
| **S6** | class 归一（sidebar-v4 与现网） | CSS layers | 可并行低优 |
| **S7** | 仅当指标要求时启动 β | 新布局组件 | S1–S5 稳定后 |

### 5.2 S1 文件级清单（最小可测试）

| 文件 | 变更要点 |
|------|----------|
| `AccountsPanel.tsx` | `siteFilter` state；工具条站点 select；`filteredAccounts` 增加 site 条件；intent 读 `upstreamSiteId` |
| `AccountForm.tsx` | 外包折叠：默认收起，按钮展开 |
| `AccountInsights.tsx` | 默认折叠或 `details/summary` 等价模式 |
| `AccountCard.tsx` | 一级仅 网页登录/签到/详情；其余进 more |
| `domains/accounts.css` 或 harmonization | 工具条 grid：`minmax(200px,280px) minmax(120px,160px) minmax(0,320px)` |
| 测试 | 扩展 AccountsPanel 相关测试；AccountCard 动作可见性断言 |

### 5.3 验收清单（Definition of Done）

**文档 DoD（本文件）：**

- [x] 现状分析（尺寸、分区、动线、代码锚点）  
- [x] 问题列表含优先级与理论依据  
- [x] ≥2 套方案 + 参数 + 示意  
- [x] 量化对比表  
- [x] 实施阶段与文件级建议  
- [x] 约束冲突检查  

**实现 DoD（α S1–S5，后续 PR）：**

- [ ] 选站点后列表仅含该站账号；「全部」恢复全量  
- [ ] 与 `GET /api/accounts?upstreamSiteId=` 条数一致（S3 后）  
- [ ] 账号页 1440 首屏可见卡片行  
- [ ] 单卡一级 ≤3 CTA  
- [ ] 仪表盘默认不展开 Analytics  
- [ ] 侧栏三组标签可见  
- [ ] 无 balances 空跳  
- [ ] `npm test` / 相关组件测通过；`npx tsc --noEmit`  
- [ ] 390 与 1440 无横向溢出（smoke 或手工记录）

### 5.4 风险与回退

| 风险 | 缓解 | 回退 |
|------|------|------|
| 折叠隐藏批量入口 | 折叠态保留图标按钮行 | 默认展开 Insights |
| 客户端过滤大数据变慢 | S3 切服务端 | 保持全量 + 虚拟列表（另项） |
| 待办上移打乱用户习惯 | 分 PR；可先做账号页 | 还原 Dashboard 顺序 |
| β 过早 | 门禁：α 指标未达标且运维反馈按站主路径 | 不合并 Tab |

### 5.5 明确不在本期

- 自动登录 / 绕过 2FA  
- 新图表库、新组件库依赖  
- 重做设置页信息架构  
- 物理打印/导出 PDF 布局  
- 未评审前的站点+账号 Tab 强行合并（β）

---

## 6. 理论依据索引（速查）

| 原则 | 在本方案中的用法 |
|------|------------------|
| 格式塔 · 接近性 | 筛选项同工具带；侧栏任务分组 |
| 格式塔 · 相似性 | 跨页统一 site/status 语义与 intent 字段 |
| 格式塔 · 连续性 | 待办 → 账号列表 → 卡内登录/签到 主路径不断裂 |
| Fitts 定律 | 减少一级按钮数、保证间距与最小高度 |
| 使用频率分区 | 列表与主 CTA 优先于新建表单与分析 |
| 渐进披露 | 表单/Insights/Analytics/系统三卡默认折叠 |
| 工作记忆限制 | 仪表盘首屏决策块 ≤3 |
| 模数 / 既有 token | 沿用 `--rc-gap`、`--rc-card-*`、245px 侧栏，避免新模数体系 |

---

## 7. 示意图汇总

### 7.1 现状仪表盘流

```text
Radar(5) → Metrics(5) → 待办 → 三卡 → Analytics
         ↑ 重复摘要      ↑ 主任务偏后
```

### 7.2 α 仪表盘流

```text
Radar → 待办(主) → 指标条 → ▸系统/调度 → ▸分析
```

### 7.3 α 账号页流

```text
[▸洞察批量] [▸添加账号]
工具条: [站点▼] [状态▼] [搜索……]  计数
卡片网格 (一级: 登录|签到|详情)
```

### 7.4 β 主从（≥1180）

```text
站点列表 | 账号列表(?upstreamSiteId=)
```

---

## 8. 终选与下一步

| 决议 | 内容 |
|------|------|
| **采用** | 方案 α 为默认实施路径 |
| **搁置** | 方案 β，直至 α 指标与用户反馈触发 |
| **文档状态** | 设计完成（本文件） |
| **下一代码切片** | S1：站点筛选（客户端）+ 表单/洞察折叠 + 卡动作分层 |

---

## 9. 变更记录

| 日期 | 说明 |
|------|------|
| 2026-07-11 | 初版：基于源码与本机 25 账号/59 站点实测；对齐 accounts 服务端过滤已上线事实 |
