# 任务计划：抽取 API Key 测试任务服务

## 目标

继续收敛 `internal/core/task_runner.go` 的业务编排职责，将 `test_keys` SSE 任务体抽入独立服务，保持 HTTP API、SSE 进度结构、数据库写入和前端行为不变。

## 当前阶段

- Phase 1：规划 — complete
- Phase 2：TDD 实现 — complete
- Phase 3：Review — complete

## 任务清单

- [x] RED：新增 `AccountTaskService`/API Key 任务服务测试，证明未来服务能发布 `test_keys` 进度并持久化 API Key 测试结果。
- [x] GREEN：新增任务服务并接入 `App`，让 `startTestKeysTask` 委托服务执行。
- [x] REFACTOR：清理重复 helper，更新 `PACKAGE_INDEX.md` 与 handoff/计划进度。
- [x] CHECK：运行 focused、core、全量 Go 验证；必要时补前端回归。
- [x] REVIEW：以 `ec7357c` 为 fixed point 做 Standards/Spec 双轴审查。

## 范围边界

- 不修改数据库 schema。
- 不修改公开 HTTP API、SSE payload 或前端行为。
- 不移动 `testAPIKeyForAccount` 的单账号测试逻辑；本切片只抽任务编排层。
- 不改 `detect_sites` 与 `channel_health_probe` 任务体。

## 验收标准

- `task_runner.go` 中 `startTestKeysTask` 变为 thin delegation。
- 新任务服务具备 focused 测试。
- `go test ./internal/core`、`go test ./...`、`go vet ./...`、`go build ./...` 通过。
- 文档和 handoff 与实际边界一致。
