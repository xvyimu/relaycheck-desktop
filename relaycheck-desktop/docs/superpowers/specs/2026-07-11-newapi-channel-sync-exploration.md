# #8 NewAPI 渠道同步 · 探查与方案

**日期：** 2026-07-11  
**状态：** 8.1 结构化计数 + 8.5 skip 文案 + 8.3 前端同步反馈已实现 · 8.4/8.2 仍待  
**范围：** 本机 / 远程 NewAPI 实例 → `imported_channels`（及可选上游站点）同步链路  
**关联代码：** `internal/accounts/import_admin_api.go`、`local_newapi.go`、`import_sqlite.go`、`sync_preview.go`；`internal/core/local_newapi.go`、`import_admin_api.go`、`scheduler.go`（job `sync.local_newapi`）  
**安全：** 本文不出现真实 token / 密钥 / 库路径中的敏感段；示例仅用占位符

---

## 1. 背景与问题陈述

运营侧历史痛点（产品 backlog **#8**）：

- 渠道列表偶发 **空/偏少**，或与 NewAPI 后台不一致；  
- 同步依赖 **系统访问令牌** 或 **本机 SQLite 路径**；令牌缺失、2FA/权限不足、实例不可达时失败形态不统一；  
- 定时任务 `sync.local_newapi` 已存在，但失败时用户难定位「该补 token / 换 SQLite / 查排除规则」。

**#8 目标（本方案）：** 摸清现网能力边界 → 给出可诊断的改进切片与验收，**不**在本期做大规模导入重写。

---

## 2. 现网能力地图（探查结论）

### 2.1 HTTP 面

| 方法 | 路径 | 作用 |
|------|------|------|
| GET | `/api/local-newapi` | 实例列表（含 channelCount、HasSyncToken、SyncCapability） |
| POST | `/api/local-newapi/scan` | 扫描本机 NewAPI |
| POST | `/api/local-newapi/import-from-sqlite` | 从 SQLite 导入渠道 |
| POST | `/api/local-newapi/import-from-admin-api` | 从后台 Admin API 导入 |
| POST | `/api/local-newapi/auto-detect-import` | 自动发现并导入 |
| POST | `/api/local-newapi/{id}/sync` | 按实例能力再同步 |
| POST | `/api/local-newapi/{id}/sync-preview` | 同步预览（diff 字段） |
| POST | `/api/local-newapi/{id}/mark-missing` | 标记源端已消失渠道 |
| GET/其他 | `/api/channels*` | 已导入渠道消费侧 |

### 2.2 同步决策（`SyncLocalNewAPIInstanceData`）

```text
instance 存在？
  ├─ database_path 非空 → ImportChannelsFromSQLite*
  ├─ base_url 为 http(s)
  │     └─ resolve token（请求体 accessToken 或库内加密 token）
  │           空 → 错误：「需要填写系统访问令牌…」
  │           有 → ImportChannelsFromAdminAPI*
  └─ 否则 → 错误：无 SQLite 且无可用 API
```

定时任务特点（`runScheduledLocalNewAPISync`）：

- 遍历 **全部** `local_newapi_instances`；  
- 默认 `ImportKeys=false`，`DetectAfterImport=false`，`UserID=1`，`PageSize=100`；  
- 成功后再 `reconcileMissing…` 标记源端移除；  
- 失败计入 `FailedInstances`，可发 `scheduled_sync_failed` 通知。

### 2.3 Admin API 拉取

- 端点：`{base}/api/channel/`，query `p` / `page` / `page_size`；  
- Header：`Authorization: Bearer <token>`，`New-Api-User: <userId>`；  
- 分页上限循环 200 页；解析 `data.items` 或 `data` 为数组；  
- 单条 `importChannelRecord`：写 `imported_channels`，可选 `importKeys` 加密 key，可选建站 + detect。

### 2.4 空结果的合法原因（非必 bug）

| 原因 | 机制 |
|------|------|
| API 返回 0 条 | 源实例本身无渠道 / 权限只看到空集 |
| `extractImportedBaseURL` 为空且后续逻辑跳过建站 | 记录仍可能入库，但站点侧「像空」 |
| `isExcludedRelaySite(name, baseURL)` | 命中排除 token → **整条跳过**（返回空 id） |
| token 错误 / HTTP 非 2xx | 整页失败，importedCount 不增加 |
| 仅 SQLite 路径失效 | 走 SQLite 分支失败 |
| 定时未存 token 且无 database_path | 该实例每次失败 |

### 2.5 与「2FA」关系

- **渠道同步**走 Admin API token 或 SQLite，**不**走账号网页登录 / 2FA。  
- 用户口语里的「2FA 导致同步失败」通常是：  
  - 拿不到长期 **系统访问令牌**；或  
  - 把 **账号登录 2FA**（#9）与 **NewAPI 系统 token** 混淆。  
- 方案上应在 UI/文案拆开两条错误语义，避免混谈。

---

## 3. 问题分层（#8  backlog 拆解）

| ID | 层级 | 描述 | 优先级 |
|----|------|------|--------|
| C1 | 可观测 | 同步结果只有 importedCount，缺少「跳过/排除/无 base_url/鉴权失败」细分 | P0 |
| C2 | 配置 | 实例无 token 且无 db 时，定时任务静默失败堆通知 | P0 |
| C3 | 数据 | 排除规则过宽导致「后台有、本地无」 | P1 |
| C4 | 数据 | base_url 抽不出 → 站点不建，渠道「有记录难用」 | P1 |
| C5 | UX | 扫描/导入/同步入口分散，失败下一步不清晰 | P1 |
| C6 | 正确性 | 分页/字段兼容不同 NewAPI 版本 | P2 |
| C7 | 安全 | 响应/日志/预览不得回传明文 key（现网已有 mask；需守住） | 持续 |

---

## 4. 方案选项

| 方案 | 内容 | 优点 | 缺点 | 结论 |
|------|------|------|------|------|
| **A. 诊断增强优先** | 同步/预览响应增加 counters：fetched / imported / skippedExcluded / skippedNoURL / authError；实例列表暴露 lastSyncError 摘要 | 小改动、先解释「空」 | 不直接增加导入成功率 | **推荐先做** |
| **B. 配置向导** | 无 capability 时强制引导：贴 token 或选 sqlite；sync-preview 必经 | 降低误用 | 前端工作量 | A 后 |
| **C. 同步引擎重写** | 新客户端、变更 feed、增量 API | 长期 | 超范围、风险高 | 否 |
| **D. 仅文档 runbook** | 运维手册 | 零代码 | 不治本 | 作 A 的附件 |

**本阶段推荐：A → 局部 B；不做 C。**

---

## 5. 推荐切片（仍不编码，仅计划形状）

### Slice 8.1 — 同步结果结构化（后端）

- 扩展 `ImportChannelsFromAdminAPIWithOptions` / SQLite 等价路径的返回 map：  
  - `fetchedCount` / `importedCount` / `skippedExcluded` / `skippedNoBaseURL` / `sitesCreated` / `sitesMerged`  
- 鉴权失败保持 4xx + **稳定错误码或前缀文案**（中文可执行提示，无 token 原文）。  
- 单测：mock 一页含「排除名」「无 URL」「正常」三条 → counters 断言。

### Slice 8.2 — 实例 last sync 摘要（存储可选）

- 最小：不改 schema，仅在 API 响应与通知里带本次 counters。  
- 增强（可选列）：`last_sync_at` / `last_sync_summary`（无密钥）——单独立项。

### Slice 8.3 — 前端同步反馈

- 本机扫描 / 实例卡：展示 HasSyncToken、SyncCapability、上次结果摘要。  
- 空导入：区分「源为空」vs「全被排除」vs「需要 token」。  
- 文案：**禁止**写「请关闭 2FA」；改为「需要 NewAPI 系统访问令牌或本机数据库路径」。

### Slice 8.4 — 排除规则可审计

- 管理端或诊断接口返回「因排除跳过的 source_channel_id 列表（可截断）」；  
- 或设置页只读展示 `excludedRelaySiteTokens` 语义说明（不在文档写具体敏感域名列表若有隐私问题则写「见 helpers 常量」）。

### Slice 8.5 — 定时任务友善跳过

- 无 `database_path` 且无 sync token 的实例：计 `SkippedInstances`，**不**计 Failed，避免假警告风暴。  
- 通知文案区分 skip vs fail。

---

## 6. 验收标准（实现阶段）

| # | 标准 |
|---|------|
| 1 | 人为空源：fetched=0，imported=0，UI 说明「源端无渠道」 |
| 2 | 人为全排除：skippedExcluded>0，UI 说明排除而非失败 |
| 3 | 无 token：明确错误，不写 token 内容 |
| 4 | 有 token 正常同步：imported 与后台条数在排除规则后一致（允许说明差集） |
| 5 | 定时：无凭证实例 skip，不刷 failed 通知 |
| 6 | `go test` 覆盖 counters；前端无明文 key |
| 7 | 不引入新依赖；不改加密算法 |

---

## 7. 探查清单（人工一次即可）

在目标机器（有真实 NewAPI 时）：

1. GET `/api/local-newapi` → 记录实例数、HasSyncToken、ChannelCount（可截图打码）。  
2. 对一实例 POST `…/sync-preview` → 看将增/改/缺。  
3. POST `…/sync` → 对照 NewAPI 后台渠道数与本地 `imported_channels`。  
4. 查调度器 job `sync.local_newapi` 最近结果与通知。  
5. 若 imported=0：抓（脱敏）HTTP 状态与是否全排除——**不要**把 Authorization 写入 issue。

---

## 8. 与 #9 / β 的边界

| 项 | #8 | #9 | 布局 β |
|----|----|----|--------|
| 对象 | NewAPI 实例与渠道资产 | 上游站账号会话 | 站/号 UI |
| 鉴权 | 系统 token / SQLite | 浏览器 Cookie / 密码登录 | 无 |
| 可并行 | 是 | 是 | 是 |

---

## 9. 明确不做

- 绕过 NewAPI 登录或 2FA 去「偷」渠道；  
- 在日志/HANDOFF 粘贴 accessToken；  
- 自动对生产 NewAPI 爆破分页；  
- 与布局 β 绑同一 PR。

---

## 10. 建议优先级

```text
8.1 结构化结果 → 8.5 定时 skip 语义 → 8.3 UI 文案 → 8.4 排除审计 → 8.2 持久摘要（可选）
```

**批准后：** 另开实施 plan 再动代码。

---

## 11. 变更记录

| 日期 | 说明 |
|------|------|
| 2026-07-11 | 初稿：基于 import_admin_api / local_newapi / scheduler 源码探查；docs-only |
| 2026-07-11 | 8.1：Admin/SQLite 导入返回 fetched/skippedExcluded/skippedNoBaseURL；8.5：定时无凭证 skip 写入 Messages；单测 import_counters_test |
| 2026-07-11 | 8.3：LocalNewAPISyncPanel + syncFeedback（空源/排除/需令牌）；Scan 页挂载；禁止关闭 2FA 文案 |
