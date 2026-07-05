# 进度记录：移除全局调度幽灵站点

## 2026-07-05

- 选择下一阶段推荐方向：调度系统深度收口。
- 读取 `superpower`、`planning-with-files-zh` 技能说明。
- 核对工作树：`main...origin/main`，干净。
- 读取并确认上轮 `schedule-plan-projection` 已完成，边界是不改 schema、不移除 `__global__`。
- 读取 `internal/core/db.go`、`internal/core/channel_schedules.go`、`internal/channels/schedules.go`、`internal/core/scheduler.go`、`internal/core/schedule_projection_service.go`、调度相关测试。
- 建立本计划，准备进入 RED 测试阶段。
- RED：在 `internal/core/channel_schedules_test.go` 新增新库/旧库迁移测试。
- RED 结果：`rtk go test -mod=vendor -count=1 ./internal/core -run "TestListChannelSchedules_Empty|TestNewAppMigratesLegacyGlobalScheduleWithoutGhostSite" -v` 失败，原因是当前启动/迁移后仍存在 `upstream_sites.id='__global__'`。
- GREEN：新增 `ensureChannelSchedulesNullableSiteID` 表重建迁移，把旧 `__global__` schedule 映射为 `upstream_site_id=NULL`；`EnsureGlobalScheduleRecord` 改为不再插入 upstream site，并清理旧虚拟站点。
- GREEN 验证：同一 focused 测试命令通过，2 passed。
- 回归验证：
  - `rtk go test -mod=vendor -count=1 ./internal/core -run "ChannelSchedules|ScheduleCalendar|NextRuns|Scheduler|ComputeNextRun|TickChannelScheduler" -v`：42 passed。
  - `rtk go test -mod=vendor -count=1 ./internal/channels -v`：264 passed。
  - `rtk go test -mod=vendor -count=1 ./internal/core`：381 passed。
  - `rtk go test -mod=vendor -count=1 ./...`：972 passed / 12 packages。
  - `rtk go vet -mod=vendor ./...`：clean。
  - `cd frontend; rtk npm test`：216 passed / 14 files。
  - `cd frontend; rtk npm run build`：success。
  - `rtk go build -mod=vendor -ldflags="-H windowsgui" -o dist\relaycheck.exe .`：success。
  - `rtk git diff --check`：clean。
  - `rtk powershell -ExecutionPolicy Bypass -File scripts\verify-release.ps1 -ProxyUrl http://127.0.0.1:7897`：Release verification passed；包含 govulncheck、临时二进制 health、fresh DB `/api/channels`、scheduler layout smoke、navigation smoke。
- 清理确认：3001 无监听；5173 仅 TIME_WAIT，无残留 dev server。
- 复查增强：补充断言全局 schedule 原始存储 `upstream_site_id IS NULL`；补充旧库真实站点 schedule 迁移保留和删除站点后 cascade 清理断言。
- 最终验证：
  - `rtk go test -mod=vendor -count=1 ./internal/core -run "TestListChannelSchedules_Empty|TestNewAppMigratesLegacyGlobalScheduleWithoutGhostSite" -v`：2 passed。
  - `rtk go test -mod=vendor -count=1 ./internal/core`：381 passed。
  - `rtk go test -mod=vendor -count=1 ./...`：972 passed / 12 packages。
  - `rtk go vet -mod=vendor ./...`：clean。
  - `rtk git diff --check`：clean。
- Review：
  - Standards 轴：未发现违反 `CLAUDE.md`/`PACKAGE_INDEX.md` 架构约定的问题；SQL 仍参数化，`rows.Err()` 已检查，domain package 仍不 import core。
  - Spec 轴：ADR-004 与计划验收项均有测试或 release gate 证据；无缺项。

## 错误记录

| 错误 | 处理 |
|------|------|
| PowerShell 文件规模统计命令两次因 `$_` 被外层引号吞掉而失败 | 放弃该辅助统计，改用已读取的调度热点文件推进。 |
