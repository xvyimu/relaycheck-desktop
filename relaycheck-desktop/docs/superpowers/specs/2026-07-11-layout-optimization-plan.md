# RelayCheck Desktop 布局优化实施计划

**日期：** 2026-07-11  
**状态：** 待实施  
**设计规格：** `docs/superpowers/specs/2026-07-11-layout-optimization-design.md`  
**默认方案：** α（行动优先流线式）  
**β 门禁：** 仅当 α 上线后按站运维路径仍为主路径时开启

---

## 1. 目标

将设计规格中的方案 α 拆成可独立合并、可回退的实现切片，并给出每切片的验收命令与回归面。

**产品结果：**

1. 账号页能按站点收敛列表（先客户端，后服务端 query）。  
2. 账号页首屏优先露出列表与执行动作。  
3. 账号卡一级 CTA ≤3。  
4. 仪表盘首屏决策块 ≤3 层有效信息。  
5. 侧栏按运营/资产/工具分组。  
6. 消除 `balances` 空跳。

---

## 2. 非目标

见设计规格 §5.5。本计划不包含 β 主从布局编码，仅在 §8 保留触发条件。

---

## 3. 切片与任务

### Slice S1 — 账号执行台（最高优先）

| # | 任务 | 文件 | 验收 |
|---|------|------|------|
| S1.1 | 工具条增加站点 `select`（全部 + sites） | `AccountsPanel.tsx` | 选站后可见条数变化 |
| S1.2 | `filteredAccounts` 增加 `upstreamSiteId` 条件 | 同上 | 与手动数卡一致 |
| S1.3 | `AccountForm` 默认折叠 | `AccountForm.tsx` 或 Panel 外包 | 默认不占满首屏 |
| S1.4 | `AccountInsights` 默认可折叠，折叠态保留批量入口 | `AccountInsights.tsx` | 仍可一点展开批量 |
| S1.5 | `AccountCard` 一级仅：网页登录 / 签到 / 详情 | `AccountCard.tsx` | 单测或 DOM 断言 |
| S1.6 | 工具条 CSS grid 三控件 | `accounts.css` / harmonization | 1440 单行；900 可折行 |

**测试：**

```text
cd frontend
npx tsc --noEmit
npm test -- AccountCard AccountsPanel
```

**手工：** 1440 打开账号页 → 首屏见卡片；选一多账号站 → 条数=2 或 3。

---

### Slice S2 — Intent 与站点跳转

| # | 任务 | 文件 | 验收 |
|---|------|------|------|
| S2.1 | `NavigationIntent` 增加 `upstreamSiteId?: string` | `types/index.ts` | 类型编译通过 |
| S2.2 | `AccountsPanel` 消费 intent.site | `AccountsPanel.tsx` | 导航后自动选站 |
| S2.3 | 站点卡/详情「查看账号」 | `SitesPanel.tsx` | 一跳进入账号+筛选 |
| S2.4 | `navigation.ts` / 单测扩展 | `navigation.ts` + tests | 单测绿 |

```text
npm test -- navigation
```

---

### Slice S3 — 服务端过滤接线

| # | 任务 | 文件 | 验收 |
|---|------|------|------|
| S3.1 | 账号列表支持可选 query（专用 hook 或扩展 useApi 调用方） | `useInventoryData.ts` 或新建 `useAccounts.ts` | 请求 URL 含 upstreamSiteId |
| S3.2 | 切换站点时 refresh；「全部」回全量 | 同上 | 网络面板可见 |
| S3.3 | 与后端缓存隔离不回归 | 已有 Go 测 | `go test -run TestListAccountsHonorsUpstreamSiteIDFilter` |

**注意：** 全页 inventory 三资源并行时，避免仅因选站而重拉 channels/sites；优先账号独立 hook。

```text
go test -mod=vendor -count=1 -run TestListAccountsHonorsUpstreamSiteIDFilter ./internal/core/
```

---

### Slice S4 — 仪表盘决策台

| # | 任务 | 文件 | 验收 |
|---|------|------|------|
| S4.1 | 运营待办上移到 Radar 后 | `Dashboard.tsx` | 源码顺序 / 截图 |
| S4.2 | metrics 压缩为单行或降视觉权重 | Dashboard + CSS | 高度下降 |
| S4.3 | 系统/运营/调度默认折叠 | Dashboard | `aria-expanded=false` 默认 |
| S4.4 | Analytics 默认折叠 | Dashboard | 同上 |

```text
npm test -- Dashboard
# 若无单测，以 tsc + 手工 1440 为准
```

---

### Slice S5 — 导航与死链

| # | 任务 | 文件 | 验收 |
|---|------|------|------|
| S5.1 | 侧栏运营/资产/工具分组 | `Sidebar.tsx` + CSS | 三组标题可见 |
| S5.2 | HubRadar 去掉无效 `balances` 导航 | `HubRadar.tsx` | 点击落到有效 Tab |
| S5.3 | smoke 导航 | `npm run smoke`（若环境允许） | 通过或记录跳过原因 |

---

### Slice S6 — CSS 归一（低优并行）

| # | 任务 | 验收 |
|---|------|------|
| S6.1 | 核对 `.sidebar` vs `.sidebar-v4` 小屏规则命中 | 560 下 nav 可横滑或等价紧凑 |
| S6.2 | 不删历史层；只补桥接选择器 | 无样式回归 |

---

## 4. 建议 PR 顺序

```text
PR1: S1 账号执行台
PR2: S2 intent + 站点跳转
PR3: S4 仪表盘
PR4: S5 导航/死链
PR5: S3 服务端过滤（可与 PR2 合并若改动小）
PR6: S6 CSS 桥接（可选）
```

每个 PR 保持可独立回退；不在默认分支上直接做 β。

---

## 5. 验证矩阵

| 场景 | 1440 | 900 | 390 |
|------|------|-----|-----|
| 账号站点筛选 | 必测 | 必测 | 必测（控件可折行） |
| 卡一级 3 CTA | 必测 | 必测 | 必测 |
| 仪表盘折叠 | 必测 | 必测 | 必测 |
| 侧栏分组 | 必测 | 必测 | 横滑/块状 nav |
| 无横溢 | 必测 | 必测 | 必测 |

**命令基线：**

```powershell
cd E:\zidqiandao\relaycheck-desktop
go test -mod=vendor -count=1 ./internal/core/ -run TestListAccounts
cd frontend
npx tsc --noEmit
npm test
```

---

## 6. 指标回收（上线后 1 周）

| 指标 | 记录方式 |
|------|----------|
| 按站处理是否仍依赖搜索站名 | 用户反馈 / 自评 |
| 账号页是否还要先滚过表单 | 截图对比 |
| Action Center → 账号是否带对筛选 | 点选走查 |

若「按站」仍占主路径且站点数继续↑，打开设计规格 §3.2 方案 β 立项。

---

## 7. 风险清单（实施时）

| 风险 | 处理 |
|------|------|
| Insights 折叠后批量难找 | 折叠头保留主批量按钮 |
| intent 与本地 filter 双写冲突 | intent effect 只在 intent 变化时写入，避免锁死手动改筛 |
| 服务端过滤与全局 refresh 竞态 | 沿用 useApi AbortController |
| 测试不足 | S1 至少补 AccountCard 一级按钮查询 |

---

## 8. β 触发条件（不自动开工）

同时满足再开 β 设计评审：

1. α S1–S5 已合并；  
2. 运营反馈「先站后号」为默认心智；  
3. 站点 ≥50 且同站多账号成为常态；  
4. 有排期做 Master-Detail 响应式两态。

---

## 9. 文档完成度对照（专家模板）

| 模板模块 | 设计规格 | 本计划 |
|----------|----------|--------|
| 现状分析 | §1 | — |
| 问题列表 | §2 | — |
| 优化方案 + 示意 | §3 · §7 | — |
| 预期效果对比 | §4 | §6 回收 |
| 实施建议 | §5 | §3–§5 细化 |

**文档要求状态：已完成。**  
编码以本计划切片为准，另起实现会话/PR，不阻塞文档 DoD。
