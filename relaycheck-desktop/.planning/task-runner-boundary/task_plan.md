# 任务计划：深化 TaskRunner 任务边界

## 目标

按“架构建议 -> ADR -> TDD 实施 -> 复查”的流程，先完成 RelayCheck Desktop 下一阶段架构升级建议，再选择一个低风险高杠杆方向执行到测试通过。

本轮执行方向：把 `detect_sites` 和 `channel_health_probe` 任务 body 从 `internal/core/task_runner.go` 迁入新的 `SiteTaskService`，让 `TaskRunner` 的 interface 回到任务生命周期、取消和 SSE 流。

## 当前阶段

- Phase 1：架构扫描与候选建议 - complete
- Phase 2：ADR 草案 - complete
- Phase 3：TDD 实施 SiteTaskService - complete
- Phase 4：验证与复查 - complete

## 任务清单

- [x] 输出全面架构升级建议报告。
- [x] 创建 2-3 个 ADR 草案。
- [x] RED：新增 `SiteTaskService` detect-sites 任务进度测试，先确认当前服务不存在导致失败。
- [x] GREEN：新增 `SiteTaskService` 并接管 `detect_sites` / `channel_health_probe`。
- [x] REFACTOR：瘦身 `task_runner.go`，更新 `App` wiring 和 `PACKAGE_INDEX.md`。
- [x] CHECK：运行 focused Go 测试、全量 Go 测试、vet/build，必要时跑前端回归。
- [x] REVIEW：按 correctness/readability/architecture/security/performance 复查本次 diff。

## 范围边界

- 不改数据库 schema。
- 不改公开 HTTP API、SSE payload 或前端行为。
- 不改站点探测、模型同步和健康状态判定语义。
- 不迁移 `TaskRunner` 的生命周期/SSE 实现。
- 不删除 `data/`、`vendor/`、`frontend/dist/` 等运行或构建目录。

## 验收标准

- `task_runner.go` 不再包含 `detect_sites` / `channel_health_probe` 的 DB 查询和探测执行 body。
- 新的 `SiteTaskService` 有 focused 测试覆盖 detect-sites 任务进度。
- 现有 channel health probe task 测试继续通过。
- `internal/core/PACKAGE_INDEX.md` 与新边界一致。
- `rtk git diff --check`、focused Go 测试、`go test ./internal/core`、`go test ./...`、`go vet`、`go build` 通过。
