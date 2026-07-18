# RelayCheck Desktop 项目整理规划与执行说明（2026-07-18）

## 1. 目的与成功标准

**目的：** 在已完成安全闭环 + typed API 并推送到 `origin/main` 之后，把本地工作区与交接文档整理到可长期接手状态。

**成功标准：**

1. 磁盘上无过期构建/覆盖率垃圾（`dist/`、`frontend/dist/`、`frontend/coverage/`）。
2. 有一份**细节齐全**的整理文档（本文件）说明：删什么、留什么、为什么、怎么回滚。
3. `HANDOFF.md` 反映 **2026-07-18 已推送** 状态，不再写「大量未提交」。
4. 项目根有 `task_plan.md` / `findings.md` / `progress.md`（superpower / planning-with-files）。
5. **绝不**删除 `data/`（数据库与密钥）、**不**擅自删除 `.planning/` 历史。
6. 不部署、不强制 push（主分支已同步则无需 push）。

---

## 2. 当前权威基线

| 项 | 值 |
|---|---|
| 路径 | `E:\zidqiandao\relaycheck-desktop` |
| 分支 | `main` = `origin/main` |
| HEAD | `a8f372d` |
| 技术栈 | Go 单二进制 + React/Vite + SQLite 本机 loopback |

### 2.1 已推送提交链（整理时点）

| Commit | 摘要 |
|---|---|
| `f5c10de` | 签到/清理 previewId 安全闭环 + Go 契约 |
| `b98218e` | 前端 typed API adapter 全量收敛 |
| `5f5c556` | SOP/审查/交接文档 |
| `a8f372d` | ignore 本地 verify canary |

### 2.2 产品硬约束（不可在整理时破坏）

- 本机单用户；前后端代码所有权分离，不拆微服务。
- 破坏性操作：preview → confirm 同源；取消零请求。
- 禁止操作：`data/relaycheck.db*`；无确认不 commit/push/deploy（本轮文档 commit 需明确）。

---

## 3. 文件分类总表

### 3.1 必须保留（生产/源码/依赖）

| 类别 | 路径 | 原因 |
|---|---|---|
| 源码 | `main.go` `internal/` `frontend/src/` | 产品本体 |
| 依赖 | `vendor/` `frontend/node_modules/` | 构建与 CI 可复现 |
| 配置 | `go.mod` `.go-version` `frontend/package.json` `vite.config.ts` | 工具链 |
| 文档 | `docs/` `README.md` `CLAUDE.md` `HANDOFF.md` | 交接 |
| CI | `.github/workflows/` `scripts/` | 门禁 |
| 运行时数据 | `data/` | **真实 DB/密钥**；仅 gitignore，永不 rm |

### 3.2 应删除的本地产物（本轮执行）

| 路径 | 约大小 | 是否 tracked | 是否 ignore | 删除理由 |
|---|---:|---|---|---|
| `dist/` | ~25.5 MB | 否 | 是 | 发布/打包输出，可 `verify-release`/`package-release` 再生 |
| `frontend/dist/` | ~0.7 MB | 否 | 是 | `npm run build` 可再生 |
| `frontend/coverage/` | ~1.8 MB | 否 | 是 | `npm run test:coverage` 可再生 |

**删除命令（PowerShell）：**

```powershell
Set-Location E:\zidqiandao\relaycheck-desktop
Remove-Item -Recurse -Force dist, frontend\dist, frontend\coverage -ErrorAction SilentlyContinue
```

**回滚：** 重新跑构建/覆盖率即可，无源码损失。

### 3.3 默认保留、需用户确认才动

| 路径 | 原因 | 若清理 |
|---|---|---|
| `.planning/**` | 历史 planning-with-files（曾 tracked） | **已归档** `docs/archives/planning-history-2026-07-18.tar.gz` 并 gitignore |
| `frontend/node_modules` | 开发必需 | 仅磁盘极度紧张时 `npm ci` 重装 |
| `docs/sop` 全量 | 已提交权威增量文档 | 不删 |

### 3.4 已处理过的垃圾

| 路径 | 状态 |
|---|---|
| `frontend/verify-canary.txt` | 已删 + gitignore |
| `frontend/verify-nav-output.txt` | 已删 + gitignore |
| 仓外 `E:\zidqiandao\overview.md` | 已删（内容进 docs/sop） |

---

## 4. 文档体系应如何读（整理后）

**新会话建议顺序：**

1. `HANDOFF.md` — 当前 TODO 与最近完成  
2. `CLAUDE.md` / 用户 `~\CLAUDE.md` — 工作风格  
3. `docs/sop/relaycheck-incremental-architecture.md` — 契约边界  
4. `docs/sop/relaycheck-incremental-qa-report.md` — 门禁数字  
5. `docs/sop/relaycheck-security-consistency-verification-2026-07-18.md` — 安全纵切  
6. `docs/full-stack-code-review-optimization-2026-07-17.md` — 全栈审查演进  

**本轮新增：**

- `docs/housekeep/project-cleanup-plan-2026-07-18.md`（本文件）  
- 根目录 `task_plan.md` / `findings.md` / `progress.md`  

---

## 5. HANDOFF 应写到的状态（目标文案要点）

```
Last updated: 2026-07-18
HEAD: a8f372d on origin/main

Done:
- previewId checkin + unsupported cleanup closed loops
- typed API owners: accounts/models/keys/channels/local-newapi/notifications/system/scheduler/sites
- frontend gates ~63 files / 387 tests; coverage ~67/59/60/68
- canary outputs gitignored

Next (safe, no migration):
- SitesPanel / remaining bare API 收敛
- LocalNewAPISyncPanel 更深行为测试
- RUM / production p95（需部署环境）

Never without confirm:
- data/* delete, DB migration, force push, deploy
```

---

## 6. 执行步骤（checklist）

### Phase A — 文档
- [x] 写本规划文档  
- [x] 写 task_plan / findings / progress  

### Phase B — 清理
- [ ] 删除 `dist` `frontend/dist` `frontend/coverage`  
- [ ] 确认 `git status` 干净或仅文档改动  
- [ ] 确认 `data/` 仍在  

### Phase C — HANDOFF
- [ ] 重写过时 TODO / 去掉 uncommitted 误导  
- [ ] 修正乱码段落或改为指向 sop 文档  

### Phase D — 可选提交
- [ ] 若仅文档 + 无大文件：`docs(housekeep): ...`  
- [ ] 不 push 除非用户要求  

---

## 7. 风险与不做什么

| 风险 | 缓解 |
|---|---|
| 误删 data | 脚本只点名 dist/coverage；删前 Test-Path 列表 |
| 误删 .planning | 默认不进删除列表 |
| HANDOFF 编码再次损坏 | 用 UTF-8 无 BOM 或与仓库一致编码写入 |
| 把 node_modules 当垃圾删 | 明确保留 |

**明确不做：** 全仓 prettier、vendor 重写、真实 DB vacuum、生产部署、Sites 大重构。

---

## 8. 后续推荐任务（不在本轮）

1. `SitesPanel` / `ChannelTable` 以外残留裸 API 扫描与收敛  
2. LocalNewAPISyncPanel 行为测试加深  
3. HANDOFF 与 README 双语文案统一编码  
4. 可选：`.planning` 归档 zip  

---

## 9. 回滚命令

```powershell
# 仅恢复构建产物
cd E:\zidqiandao\relaycheck-desktop\frontend
npm run build
npm run test:coverage
# 根 dist 由 scripts/package-release.ps1 或 verify-release 再生
```
