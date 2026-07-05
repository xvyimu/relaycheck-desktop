# 发现记录：调度计划投影

## 已确认上下文

- ADR-002 建议先收敛调度计划投影，不做 nullable schema migration。
- `internal/core/channel_schedules.go` 当前同时承担 HTTP handler、calendar projection、next-runs projection 和 channel_schedules 写入校验。
- `internal/channels/schedules.go` 已拥有纯投影 helper：`CalendarItemsForSchedule`、`NextSyncCalendarItem`、`SortCalendarItemsByDateTime`、`ParseCalendarDays`。
- `handleNextRuns` 当前在 schedule 读取失败时静默忽略 per-site schedules，保留 scheduler jobs。
- `handleScheduleCalendar` 当前在 schedule 读取失败时返回 HTTP 500。

## 执行方向

新增 `ScheduleProjectionService` 放在 `internal/core`。理由：

- 需要使用 core 类型 `SchedulerStatus`、`NextRunList`、`NextRunItem`、`ScheduleCalendarItem`。
- 不扩大 `channels.Infra`。
- 不引入包循环。
- 能让 `channel_schedules.go` 的 HTTP handler 成为薄控制器。
