# 发现记录：前端查询层

## 已确认上下文

- `useApi<T>` 已处理 abort、fallback、loading、error 和 refresh。
- `useNextRuns` 直接知道 `/api/scheduler/next-runs` endpoint 和 fallback。
- `HubRadar` 直接通过 `useApi` 请求 `/api/scheduler/calendar?days=2`，并在组件内分组 calendar items。
- `Dashboard` 同时使用 `useNextRuns`，并把 `onRefresh` 传给 `HubRadar`。
- 现有 Vitest 环境为 node，没有 hook testing library；适合把可测逻辑放在纯 `lib/` 模块。

## 执行方向

- 新增 `frontend/src/lib/schedulerPreview.ts`：endpoint、fallback、selector、calendar grouping。
- 新增 `frontend/src/hooks/useSchedulerPreview.ts`：组合 `useApi` 的 calendar 和 next-runs 查询。
- 更新 `Dashboard` 一处调用，向 `HubRadar` 注入 scheduler preview 数据。

## 执行后确认

- `HubRadar` 已不再直接请求 `/api/scheduler/calendar?days=2`。
- `useNextRuns` 与 `useSchedulerPreview` 共用 `schedulerPreview` lib 中的 endpoint/fallback/selector。
- 本轮未新增 npm 依赖，未改后端 API response shape，未改 Dashboard/HubRadar 文案与布局。
- 可测逻辑集中在纯模块测试中，hook 行为通过 TypeScript build 间接校验。

## 不做

- 不新增依赖。
- 不测试 React hook 运行时，只测试纯 selector/grouping。
- 不改后端 merged API。
