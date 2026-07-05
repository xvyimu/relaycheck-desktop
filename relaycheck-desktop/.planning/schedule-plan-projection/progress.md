# 进度记录：调度计划投影

## 2026-07-04

- 从上轮架构报告继续，选择 ADR-002 作为下一执行方向。
- 读取当前未提交状态，确认已有 TaskRunner/SiteTaskService 变更未提交。
- 读取 `channel_schedules.go`、`internal/channels/schedules.go`、`channel_schedules_test.go`、`scheduler_test.go`。
- 设定小切片边界：只收敛 calendar 和 next-runs 投影，不做 DB schema 或 API 改动。
- RED：新增 `TestScheduleProjectionServiceBuildNextRunsIncludesSiteSchedules`，运行 focused 测试预期失败：`app.scheduleProjection undefined`。
- GREEN：新增 `internal/core/schedule_projection_service.go`，集中 `BuildCalendar` 与 `BuildNextRuns`。
- `handleScheduleCalendar` 与 `handleNextRuns` 已变为 thin delegation；`ADR-002` 状态更新为 Accepted。
- 验证：
  - `rtk go test -mod=vendor -count=1 ./internal/core -run "ScheduleProjectionServiceBuildNextRunsIncludesSiteSchedules|HandleNextRuns|HandleScheduleCalendar" -v`：6 passed。
  - `rtk go test -mod=vendor -count=1 ./internal/core -run "ChannelSchedules|ScheduleCalendar|NextRuns|Scheduler|ComputeNextRun|TickChannelScheduler" -v`：42 passed。
  - `rtk go test -mod=vendor -count=1 ./internal/core`：380 passed。
  - `rtk go test -mod=vendor -count=1 ./...`：971 passed / 12 packages。
  - `rtk go vet -mod=vendor ./...`：clean。
  - `rtk go build -mod=vendor ./...`：success。
  - `rtk git diff --check`：clean。
  - `cd frontend; rtk npm test`：213 passed / 13 files。
  - `cd frontend; rtk npm run build`：success。
- Review：`ScheduleProjectionService` 只读调度状态和 schedules，无 DB 写入、schema 改动、公开响应结构改动或凭据处理。

## 错误记录

| 错误 | 处理 |
|------|------|
| 暂无 | - |
