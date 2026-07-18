# RelayCheck 增量专项 QA 报告

- **日期：** 2026-07-18
- **范围：** P0 批量签到 `previewId` 安全闭环、Onboarding/Scan 导航，以及不支持签到账号清理的 `previewId` 同源删除契约。
- **方式：** TDD 回归、临时 SQLite handler/service 测试、typed API/受控组件测试和 Playwright route-mock 真实点击；未操作实际数据库。
- **结论：** **通过（NoOne）**。当前实现满足功能、回归、破坏性操作 fail-closed、390px 和基础可访问性验收。

## 1. 验收结论

| 验收项 | 独立证据 | 结论 |
|---|---|---|
| 预览先于任务启动，确认后仅启动一次 | `frontend/scripts/verify-incremental-flows.mjs` 在真实浏览器中断言 `dry-run -> start -> stream`，本轮运行通过（`dryRun=8`、`starts=2`） | 通过 |
| 取消预览不启动任务 | 同一 smoke 在取消后断言 task start 计数不变 | 通过 |
| `willRun=0` | 对话框禁用确认并提供“前往站点与账号”；smoke 验证确认禁用且不启动任务 | 通过 |
| 预览错误和重试 | 对话框使用 `role=alert`，smoke 模拟 500 后验证保留对话框、重试不启动任务 | 通过 |
| 预览与执行候选集合一致 | 后端从 `BuildAllDue` 构造一次性计划并在 start 时 `Claim` 固定的 `RunAccounts`；测试验证 `T=R`、预览后新增账号不被执行、重放失败、超过 200 个 fail closed | 通过 |
| 账号清理预览与删除同源 | 清理预览冻结账号 ID 并签发一次性 `previewId`；确认只消费冻结集合。测试验证新增候选不扩展删除集合，漂移/过期/重放 409 且三表零删除 | 通过 |
| 账号清理前端确认 | typed API 确认只提交 `previewId`；组件测试与 Playwright 验证取消零请求、确认同源、409/500 清空旧预览 | 通过 |
| Onboarding 第 4 步 | 第 4 步只发送 `checkinPreview: "open"` 导航意图，不直接调用 task start；smoke 验证旧入口文案消失、导航到同一预览、重开回到第 1 步 | 通过 |
| Scan 导航 | 成功/混合导入才显示“查看渠道”和“前往站点与账号”；失败时不显示；smoke 验证两个导航目标 | 通过 |
| 既有任务行为 | Go 测试覆盖即时 SSE 可连接、取消、任务完成及冻结计划成员；浏览器 smoke 验证任务流结束 | 通过 |
| 390px 与基础可访问性 | smoke 验证无水平溢出、预览操作最小 44px、Escape 关闭后焦点返回；源码提供状态/错误的 `role=status`、`role=alert` 和 `aria-live` | 通过 |

## 2. 关键源码与测试核验

### 2.1 `previewId` 同源安全闭环

- [checkin_plan.go](E:/zidqiandao/relaycheck-desktop/internal/core/checkin_plan.go:106) 使用 `LoadDueAccounts` 构建候选，并只把 `will_run` 的账户冻结到 `RunAccounts`。
- [dry_run.go](E:/zidqiandao/relaycheck-desktop/internal/core/dry_run.go:65) 的 checkin dry-run 固定调用 `BuildAllDue`；可执行项才发放 5 分钟、一次性的 `previewId`。
- [task_runner.go](E:/zidqiandao/relaycheck-desktop/internal/core/task_runner.go:290) 强制 checkin task 提交 `previewId`，随后 [task_runner.go](E:/zidqiandao/relaycheck-desktop/internal/core/task_runner.go:363) 只消费 `Claim` 返回的冻结计划。
- [checkin_plan_test.go](E:/zidqiandao/relaycheck-desktop/internal/core/checkin_plan_test.go:50) 验证预览中的有序 `will_run` 集合与任务集合严格相等、重放失败、0 可执行不发放 token、201 个账号被拒绝。
- [task_runner_test.go](E:/zidqiandao/relaycheck-desktop/internal/core/task_runner_test.go:28) 验证缺少预览 ID 返回 400、并发重复确认最多一次成功，以及返回 taskId 前 SSE 任务已注册。
- [checkin_task_service_test.go](E:/zidqiandao/relaycheck-desktop/internal/core/checkin_task_service_test.go:13) 验证预览后新增账户不被扩大执行，取消会停止剩余冻结计划。

前端中，[CheckinsPanel.tsx](E:/zidqiandao/relaycheck-desktop/frontend/src/components/checkins/CheckinsPanel.tsx:50) 先请求预览；[CheckinsPanel.tsx](E:/zidqiandao/relaycheck-desktop/frontend/src/components/checkins/CheckinsPanel.tsx:71) 仅在存在 `previewId` 且 `willRun > 0` 时才启动任务。对话框的确认禁用、错误恢复、12 条展示上限和“另有 N 条”在 [CheckinDryRunDialog.tsx](E:/zidqiandao/relaycheck-desktop/frontend/src/components/checkins/CheckinDryRunDialog.tsx:35) 与 [CheckinDryRunDialog.tsx](E:/zidqiandao/relaycheck-desktop/frontend/src/components/checkins/CheckinDryRunDialog.tsx:130) 实现。

### 2.2 Onboarding 与 ScanPanel

- [OnboardingWizard.tsx](E:/zidqiandao/relaycheck-desktop/frontend/src/components/onboarding/OnboardingWizard.tsx:214) 第 4 步关闭引导并发送同一 `checkinPreview` intent；没有 `/api/tasks/start` 调用。
- [OnboardingWizard.tsx](E:/zidqiandao/relaycheck-desktop/frontend/src/components/onboarding/OnboardingWizard.tsx:334) 已统一为“站点与账号 → 全部账号”。
- [ScanPanel.tsx](E:/zidqiandao/relaycheck-desktop/frontend/src/components/scan/ScanPanel.tsx:32) 仅当至少一个导入结果有实际变更时认定成功；[ScanPanel.tsx](E:/zidqiandao/relaycheck-desktop/frontend/src/components/scan/ScanPanel.tsx:148) 才呈现导航操作。
- [verify-incremental-flows.mjs](E:/zidqiandao/relaycheck-desktop/frontend/scripts/verify-incremental-flows.mjs:1) 覆盖确认、取消、0 可执行、500 重试、409 失效预览、Onboarding、Scan 成功/混合/失败和 390px 实际点击路径。

### 2.3 破坏性账号清理

- [account_cleanup_service.go](E:/zidqiandao/relaycheck-desktop/internal/core/account_cleanup_service.go:75) 预览冻结候选并写入最多 64 条、5 分钟 TTL 的内存 store；确认原子 Claim 后在事务中复核完整集合。
- [accounts.go](E:/zidqiandao/relaycheck-desktop/internal/core/accounts.go:850) 将预览和确认拆为显式契约；空 body、缺失 ID、确认阶段重交筛选参数均 400，过期/重放/漂移均 409，容量耗尽为 429。
- [system.go](E:/zidqiandao/relaycheck-desktop/internal/core/system.go:367) 重开数据库时重绑清理服务并清空旧 preview，防止恢复后访问已关闭 DB 或跨数据集消费 token。
- [account_cleanup_preview_contract_test.go](E:/zidqiandao/relaycheck-desktop/internal/core/account_cleanup_preview_contract_test.go:1) 覆盖候选新增、状态漂移、空 body、参数重选、TTL/容量、并发双确认、DB 重绑与事务中途失败回滚。
- [account-cleanup.ts](E:/zidqiandao/relaycheck-desktop/frontend/src/api/account-cleanup.ts:1) 是前端唯一 HTTP 契约 owner；[UnsupportedCheckinCleanupPanel.tsx](E:/zidqiandao/relaycheck-desktop/frontend/src/components/accounts/UnsupportedCheckinCleanupPanel.tsx:1) 是受控展示与二次确认 owner。

### 2.4 AccountInsights models/keys typed API（2026-07-18 续）

- [models.ts](E:/zidqiandao/relaycheck-desktop/frontend/src/api/models.ts:1) 持有 `/api/models/overview|pricing|sync|pricing/sync`；同步默认 `limit=50`。
- [keys.ts](E:/zidqiandao/relaycheck-desktop/frontend/src/api/keys.ts:1) 持有只读 `/api/keys/export-preview`；夹具与响应断言不得出现真实密钥形态。
- [AccountInsights.tsx](E:/zidqiandao/relaycheck-desktop/frontend/src/components/accounts/AccountInsights.tsx:1) 展开/同步/导出均只调用 typed adapter；清理确认失败仍清空 preview。
- 定向测试：`models.test.ts`、`keys.test.ts`、`AccountInsights.behavior.test.ts`（8 项）通过。

### 2.5 OnboardingWizard typed API + schema 文案修正（2026-07-18 续）

- [local-newapi.ts](E:/zidqiandao/relaycheck-desktop/frontend/src/api/local-newapi.ts:1) 持有 Admin 导入；trim 地址/令牌；默认 `importKeys/detectAfterImport=false`。
- [channels.ts](E:/zidqiandao/relaycheck-desktop/frontend/src/api/channels.ts:1) 持有渠道模型 overview/sync；默认 sync `limit=100`，Onboarding 覆盖 `10`。
- [OnboardingWizard.tsx](E:/zidqiandao/relaycheck-desktop/frontend/src/components/onboarding/OnboardingWizard.tsx:1) 不再拼裸 URL；第 2 步文案改用 `channelCount/syncedChannels/failedCount`。
- [useChannelActions.ts](E:/zidqiandao/relaycheck-desktop/frontend/src/hooks/useChannelActions.ts:1) 与 Onboarding 共用 `channelsApi`。
- 定向测试：`local-newapi.test.ts`、`channels.test.ts`、`OnboardingWizard.behavior.test.ts`、更新后的 `useChannelActions.test.ts` 通过。

### 2.6 ChannelsPanel 行为测试 + health path 收敛（2026-07-18 续）

- [ChannelsPanel.tsx](E:/zidqiandao/relaycheck-desktop/frontend/src/components/channels/ChannelsPanel.tsx:1) 导出 `refreshChannelPanelData` / `syncChannelModelsAndHealth` / `shouldAutoloadChannelModels` / `healthToneClass` / `topHealthRisks`。
- 健康概览只读路径：`channelsApi.healthOverviewPath`（`/api/channels/health/overview`）。
- 纯函数测试覆盖三路刷新部分失败、sync 失败不刷 health、inventory dual-fetch 防护、风险站点截断。
- 组件行为测试覆盖刷新/同步/探测、风险展示、inactive 不 autoload。
- 定向：`ChannelsPanel.test.ts`、`ChannelsPanel.behavior.test.ts`、扩展 `channels.test.ts` 共 19 项通过。

### 2.7 B1–B3 写路径全收敛（2026-07-18 续）

- [channels.ts](E:/zidqiandao/relaycheck-desktop/frontend/src/api/channels.ts:1)：`detect` / `restoreSourceStatus` / `archiveSourceStatus` / `bulkSourceStatus`；ID encode。
- [ChannelTable.tsx](E:/zidqiandao/relaycheck-desktop/frontend/src/components/channels/ChannelTable.tsx:1) + 行为测试：识别成功/失败、无 baseUrl 禁用、源状态回调、加载更多。
- [local-newapi.ts](E:/zidqiandao/relaycheck-desktop/frontend/src/api/local-newapi.ts:1)：`listInstances` / `excludeRules` / `autoDetectImport` / `sync`。
- [notifications.ts](E:/zidqiandao/relaycheck-desktop/frontend/src/api/notifications.ts:1) + NotificationsPanel：markAllRead / clearRead / trim(10)。

### 2.8 Settings / SiteSchedules typed API（2026-07-18 续）

- [system.ts](E:/zidqiandao/relaycheck-desktop/frontend/src/api/system.ts:1)：settings/backups/scheduler-status/audit/exports/version/port + backup/restore/delete/save/proxy/export/import。
- [scheduler.ts](E:/zidqiandao/relaycheck-desktop/frontend/src/api/scheduler.ts:1)：channel-schedules list/save、calendar、next-runs。
- [sites.ts](E:/zidqiandao/relaycheck-desktop/frontend/src/api/sites.ts:1)：`GET /api/upstream-sites`。
- [Settings.tsx](E:/zidqiandao/relaycheck-desktop/frontend/src/components/settings/Settings.tsx:1)、[SiteSchedules.tsx](E:/zidqiandao/relaycheck-desktop/frontend/src/components/settings/SiteSchedules.tsx:1) 不再拼 system/scheduler 裸 URL；恢复/删除/导入确认语义保留。
- 定向：`system.test.ts`、`scheduler.test.ts`、`sites.test.ts`、`Settings.api-ownership.test.ts` 通过。

## 3. 质量门禁结果

| 命令 | 结果 |
|---|---|
| `rtk go test -mod=vendor -count=1 ./internal/core` | 通过 |
| `rtk go test -mod=vendor ./... -count=1 -timeout 120s` | 通过：13 个包、1148 项测试 |
| `rtk go vet -mod=vendor ./...` | 通过：0 项问题 |
| `rtk npm run format:check` | 通过 |
| `rtk npm run lint` | 通过：0 warnings |
| `rtk npx tsc -b` | 通过：0 errors |
| `rtk npm run test:coverage` | 通过：63 个文件、387 项测试；语句/分支/函数/行覆盖率为 66.99% / 58.80% / 59.73% / 68.18%，高于 53% / 45% / 40% / 54% 门槛 |
| `rtk npm run build` | 通过：Vite production build |
| `rtk npm run budget:check` | 通过：全部 chunk 在预算内；入口 JS gzip 65.0kB（预算 80.0kB）；settings panel gzip 约 11.5kB |
| `rtk npm run smoke:incremental` | 上轮通过：签到 dry-run 8/start 2，账号清理 preview 3/confirm 3；本轮未改签到/清理路径，未机械重跑 browser smoke |
| `rtk git diff --check` | 通过：无 whitespace error |

## 4. 工作树与安全检查

- 当前工作树包含大量已跟踪与未跟踪改动，超出本专项文件范围；本报告没有将无关改动视作本轮实现，也没有修改或清理它们。
- 本专项目标路径除 Checkins、Onboarding、Scan 外，还包含账号清理 service/handler、typed API、受控面板、DB 重绑和对应测试。`git diff --check` 通过。
- 对全量 diff 进行了不回显内容的高风险标记扫描，未发现私钥头、`Authorization: Bearer` 或 `sk-` 格式标记。测试中的 `TOP_SECRET`、`saved-key` 等仅为防泄露断言和 fixture 占位符，不能作为真实凭据结论。

## 5. 非阻塞观察与限制

1. `.github/workflows/ci.yml` 已安装 Playwright Chromium 并执行增量 smoke；本轮本地也实际通过，但远程 GitHub Actions 尚未触发，因为工作树未提交或推送。
2. 本轮未运行真实桌面二进制、目标 Windows 机人工验收、真实上游签到或线上性能采样；这些不应从 route-mock smoke 推断为已验收。
3. 本报告不取代全工作树的安全、发布或人工生产验收结论。

## 6. 智能路由

| QA 轮次 | 判定 | 原因 |
|---|---|---|
| 第 1 轮 | **NoOne** | 所有专项验收路径与质量门禁均通过；未发现源码缺陷，测试本身未需修复。 |

**最终路由：NoOne。**
