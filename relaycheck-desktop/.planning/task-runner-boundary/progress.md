# 进度记录：TaskRunner 边界深化

## 2026-07-04

- 读取技能：`improve-codebase-architecture`、`architect`、`superpower`、`code-review-and-quality`，并补读 planning/TDD/review 依赖技能。
- 读取项目上下文：`README.md`、`HANDOFF.md`、`GOALS.md`、`CLAUDE.md`、`docs/PROJECT_STRUCTURE.md`、`internal/core/PACKAGE_INDEX.md`、既有架构设计与执行缝计划。
- 确认 codebase graph 工具不可用，`rg/grep` 在当前 PATH 不可用；降级使用 `rtk read` 和 PowerShell `Select-String`。
- 选定本轮执行方向：新增 `SiteTaskService`，把 `detect_sites` 与 `channel_health_probe` 任务 body 从 `task_runner.go` 迁出。
- 创建 ADR 草案：
  - `docs/adr/001-deepen-task-runner-boundary.md`
  - `docs/adr/002-consolidate-schedule-plan-projection.md`
  - `docs/adr/003-shape-frontend-query-module.md`
- 生成并打开 HTML 架构报告：`C:\Users\yuanjia\AppData\Local\Temp\relaycheck-architecture-review-2026-07-04.html`。
- RED：新增 `internal/core/site_task_service_test.go`，运行 `rtk go test -mod=vendor -count=1 ./internal/core -run SiteTaskServiceStartDetectSites -v`，预期失败：`app.siteTasks undefined`。
- GREEN：新增 `internal/core/site_task_service.go`，接管 `detect_sites` / `channel_health_probe` 任务 body；`task_runner.go` 入口改成 thin delegation。
- 修正迁移中一个非必要行为差异：还原 detect-sites 查询形状，避免新增 `id <> __global__` 条件。
- 验证：
  - `rtk go test -mod=vendor -count=1 ./internal/core -run "SiteTaskService|ChannelHealthProbeTask|CheckinTaskService|AccountTaskService|TaskRunner|TaskStart|TaskStream" -v`：8 passed。
  - `rtk go test -mod=vendor -count=1 ./internal/core`：379 passed。
  - `rtk go test -mod=vendor -count=1 ./...`：970 passed / 12 packages。
  - `rtk go vet -mod=vendor ./...`：clean。
  - `rtk go build -mod=vendor ./...`：success。
  - `rtk git diff --check`：clean。
  - `cd frontend; rtk npm test`：213 passed / 13 files。
  - `cd frontend; rtk npm run build`：success。
- Review：确认 `task_runner.go` 不再包含站点任务 DB 查询、站点探测或模型同步 helper；修正 `channel_models.go` 的旧注释引用；未发现阻塞问题。

## 错误记录

| 错误 | 处理 |
|------|------|
| codebase-memory graph tools 未暴露 | 降级到本地文件扫描 |
| `rg` / `grep` 不在 PATH | 使用 PowerShell `Select-String` |
