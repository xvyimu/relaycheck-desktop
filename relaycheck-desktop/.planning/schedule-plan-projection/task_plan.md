# 任务计划：收敛调度计划投影

## 目标

执行 ADR-002 的低风险切片：新增 core 内部 `ScheduleProjectionService`，集中日历预览和 next-runs 投影逻辑，保持 HTTP API、数据库 schema、`__global__` 记录策略和前端行为不变。

## 当前阶段

- Phase 1：恢复上下文与现状扫描 - complete
- Phase 2：TDD 实施 ScheduleProjectionService - complete
- Phase 3：验证与复查 - complete

## 任务清单

- [x] 读取 ADR-002、`channel_schedules.go`、`internal/channels/schedules.go`、现有调度测试。
- [x] RED：新增 focused 测试，引用未来 `app.scheduleProjection.BuildNextRuns`。
- [x] GREEN：新增 `ScheduleProjectionService`，搬迁 `handleScheduleCalendar` / `handleNextRuns` 的投影逻辑。
- [x] REFACTOR：handler 变薄，更新 `App` wiring、`PACKAGE_INDEX.md`、ADR 状态。
- [x] CHECK：运行 focused 调度测试、`internal/core`、全量 Go、vet/build、前端回归。
- [x] REVIEW：五轴复查，确认无行为/API/schema 变更。

## 范围边界

- 不改 `channel_schedules` schema。
- 不移除 `__global__` 虚拟站点记录。
- 不合并 `/api/scheduler/calendar` 与 `/api/scheduler/next-runs` 响应。
- 不改前端请求逻辑。
- 不改每站点调度 tick 行为。

## 验收标准

- `channel_schedules.go` 的 calendar / next-runs handler 只负责 HTTP method、参数、错误和 JSON 响应。
- `ScheduleProjectionService` 拥有 calendar items 与 next-runs items 构建逻辑。
- 现有 schedule/calendar/next-runs 测试通过。
- 新 focused service 测试通过。
- 全量验证通过。
