# 任务计划：前端 Scheduler Preview 查询层收敛

## 目标

执行 ADR-003 的低风险切片：把 Dashboard 中 scheduler calendar / next-runs 的端点、fallback、selector 和分组逻辑收敛到稳定的 scheduler preview 查询模块与 hook。保持后端 API、UI 文案和交互不变，不引入新依赖。

## 当前阶段

- Phase 1：恢复上下文与前端查询现状扫描 - complete
- Phase 2：TDD 实施 scheduler preview query module - complete
- Phase 3：验证与复查 - complete

## 任务清单

- [x] 读取 ADR-003、`useApi`、`useNextRuns`、Dashboard/HubRadar 和现有测试结构。
- [x] RED：新增 `schedulerPreview` 纯模块测试，先确认模块不存在导致失败。
- [x] GREEN：新增 `lib/schedulerPreview.ts` 与 `hooks/useSchedulerPreview.ts`。
- [x] REFACTOR：Dashboard 使用 `useSchedulerPreview`，HubRadar 改为消费 schedulerPreview props，`useNextRuns` 复用共享端点/selector。
- [x] CHECK：运行 focused Vitest、全量前端 test/build、后端回归。
- [x] REVIEW：确认无 API/UI 行为变化，无新依赖。

## 范围边界

- 不引入 React Query/SWR 等依赖。
- 不改后端 API。
- 不改 Dashboard 页面文案、布局、CSS。
- 不重写 `useAppData` 的 11 路加载；本轮只做 scheduler preview resource group。

## 验收标准

- Scheduler calendar 和 next-runs 的 endpoint/fallback/selector 在一个 lib 模块中。
- Dashboard 只调用一个 scheduler preview hook 获取 calendar 与 next-runs。
- HubRadar 不再直接知道 `/api/scheduler/calendar?days=2`。
- 前端 test/build 通过，后端回归通过。
