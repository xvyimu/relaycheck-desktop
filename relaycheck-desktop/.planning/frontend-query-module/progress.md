# 进度记录：前端 Scheduler Preview 查询层

## 2026-07-04

- 选择 ADR-003 作为第三个执行方向。
- 读取 `frontend/package.json`、`useApi.ts`、`useNextRuns.ts`、`HubRadar.tsx`、`Dashboard.tsx`、`types/index.ts`、Vitest 配置。
- 确认无 hook testing library；本轮采用纯 lib 测试 + hook/组件类型检查。
- RED：新增 `frontend/src/lib/__tests__/schedulerPreview.test.ts`，先覆盖 endpoint、fallback selector、calendar grouping 这些可回归行为。
- GREEN：新增 `frontend/src/lib/schedulerPreview.ts` 与 `frontend/src/hooks/useSchedulerPreview.ts`，保持沿用现有 `useApi<T>`。
- REFACTOR：`Dashboard` 改为一次性获取 scheduler preview，`HubRadar` 改为接收 props，`useNextRuns` 复用 shared selector/endpoint。
- Focused CHECK 已通过：`cd frontend; rtk npm test -- schedulerPreview`。
- Build CHECK 已通过：`cd frontend; rtk npm run build`。
- 全量 CHECK 已通过：`cd frontend; rtk npm test`（14 files / 216 tests）、`rtk go test -mod=vendor -count=1 ./...`（12 packages / 971 tests）、`rtk go vet -mod=vendor ./...`、`rtk go build -mod=vendor ./...`、`rtk git diff --check`。
- 发布闸门补充已通过：`cd frontend; rtk npm audit --audit-level=low`（0 vulnerabilities）、`rtk go build -mod=vendor -ldflags="-H windowsgui" -o dist\relaycheck.exe .`。

## 错误记录

| 错误 | 处理 |
|------|------|
| 暂无 | - |
