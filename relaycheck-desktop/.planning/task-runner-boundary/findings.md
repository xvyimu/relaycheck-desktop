# 发现记录：TaskRunner 边界深化

## 已确认上下文

- `README.md`、`HANDOFF.md`、`CLAUDE.md` 和 `internal/core/PACKAGE_INDEX.md` 说明当前架构：`internal/core` 是 assembly root，已有 `AccountTaskService` 和 `CheckinTaskService`。
- `internal/core/task_runner.go` 仍同时包含任务生命周期/SSE、HTTP handlers、`detect_sites` 任务 body、`channel_health_probe` 任务 body、channel health probe helper。
- `internal/core/account_task_service.go` 与 `internal/core/checkin_task_service.go` 已形成可复用模式：服务持有 `db/rootCtx/taskRunner` 和具体执行端口，`task_runner.go` 只做 thin delegation。
- `internal/core/channel_health_probe_task_test.go` 已覆盖 channel health probe 的任务进度、站点检测、模型状态和缓存影响。
- 目前缺少 detect-sites task 的 focused 服务测试。

## 架构候选

1. Strong：深化 TaskRunner。新增 `SiteTaskService` 接管站点探测和渠道健康探测任务 body。
2. Worth exploring：调度计划投影。收敛 `channel_schedules.go`、`internal/channels/schedules.go`、`scheduler.go` 的 calendar/next-runs/plan 逻辑，并后续评估 `__global__` 虚拟站点记录。
3. Worth exploring：前端查询层。`useAppData` 是 11 路并发加载的大 hook，`HubRadar` 与 `useNextRuns` 仍有独立 scheduler 请求；可以引入更深的 query module，集中 loading/error/abort/reload 语义。

## 当前推荐

优先执行候选 1。理由：已有相邻模式和测试基础，scope 小，能直接减少 `task_runner.go` 的浅职责混合，并保持 public API 不变。
