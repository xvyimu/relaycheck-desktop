# task_plan.md — RelayCheck Desktop 项目整理（2026-07-18）

## Goal
把工作树整理成「可接手、可继续」状态：清理无用本地产物、写清整理文档、同步 HANDOFF 到已推送的 4 个 commit，不破坏运行数据与业务代码。

## Success criteria
- [ ] 本地构建/覆盖率产物已删除（`dist/`、`frontend/dist/`、`frontend/coverage/`）
- [ ] 详细整理文档落盘（候选表、决策、回滚）
- [ ] `HANDOFF.md` 更新到 2026-07-18 安全闭环 + typed API 现状（去掉「未提交」过时描述）
- [ ] `task_plan.md` / `findings.md` / `progress.md` 记录本轮
- [ ] `git status` 干净或仅含明确文档改动；**未**误删 `data/`、`vendor/`、源码
- [ ] 不 push（除非用户另说）

## Hard constraints
- 禁止：`data/relaycheck.db*`、真实密钥、强制 push
- 禁止：删除 `.planning` 除非用户确认
- 禁止：格式化/重写无关源码
- 可删：已被 `.gitignore` 覆盖且 **未 tracked** 的构建产物

## Explicitly not doing
- 不部署、不真实上游签到
- 不删 `.planning/*` 历史会话（本轮默认保留）
- 不重构 SitesPanel 等剩余裸 API（另开任务）
- 不改 git 代理脚本（已在用户家目录单独优化）

## Phases

### Phase 1 — 盘点与文档（read-only → write docs）
- 状态：`complete`
- 产出：findings + 详细 housekeep 文档 + 本 task_plan

### Phase 2 — 安全清理本地产物
- 状态：`complete`
- 删除：`dist/`、`frontend/dist/`、`frontend/coverage/`（仅未跟踪）
- 验证：GONE ×3；`data/` 仍在；status 仅新文档 untracked

### Phase 3 — HANDOFF 同步
- 状态：`complete`
- 更新 Last updated、TODO、已完成节到 a8f372d 四提交链

### Phase 4 — 收尾
- 状态：`complete`
- progress 勾选；docs-only commit `c81f423`

## Dependency graph
Phase1 → Phase2 → Phase3 → Phase4

## Vertical slice acceptance
「整理后」：新人读 HANDOFF + housekeep 文档知道删了什么、为何保留 data/planning；磁盘上无 coverage/dist 垃圾；主分支与 origin 一致（已 push）。
