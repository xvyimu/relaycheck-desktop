# RelayCheck Desktop：安全与签到一致性增量验证（2026-07-18）

## 范围与结论

本报告是当前工作树的增量结论，补充 `full-stack-code-review-optimization-2026-07-17.md`。已完成的代码收口均通过回归、全量构建和浏览器烟测；经用户确认后，实例密钥 ACL、CI 增量烟测与破坏性账号清理同源契约均已收口。未执行数据库迁移、发布或真实上游签到。

## 已完成修复

### 1. JSON 写入请求 fail-closed

- 统一解码器现在拒绝未知字段、多个 JSON 值和尾随垃圾；仅显式使用 `decodeOptionalJSON` 的端点允许空请求体并保留原默认值。
- 覆盖批量密码/浏览器登录、API Key 检测、余额刷新、删除不支持签到账号、代理测试、模型同步、站点探测以及 Local NewAPI 同步/预览/标记缺失。
- 畸形请求在进入数据库写入或外部 HTTP 前返回 400。删除账号与代理探测的回归测试分别断言数据库记录和出站请求数不变。

证据：`internal/core/http.go`、`internal/core/accounts.go`、`internal/core/checkin_balance.go`、`internal/core/network.go`、`internal/core/write_json_validation_test.go`。

兼容性影响：未提供的第三方本地调用方若发送未声明字段，现会收到 400；仓内前端与全量测试未发现此类调用。

### 2. CST 业务日界一致性

- 将“今日”统一为 `Asia/Shanghai` 对应 UTC 半开区间 `[start, end)`，不再截取 UTC RFC3339 的日期前缀。
- 应用到待签到候选、今日签到列表、状态汇总、行动中心和系统诊断。
- 查询对 `started_at` 保持范围谓词，可使用已有时间索引。

证据：`internal/core/app.go`、`internal/core/checkin_batch_orchestrator.go`、`internal/core/checkin_balance.go`、`internal/core/action_center.go`、`internal/core/diagnostics.go`、`internal/core/checkin_time_consistency_test.go`、`internal/core/checkin_cst_reporting_test.go`。

已验证边界：CST `2026-07-18 00:01` 映射为 UTC `2026-07-17T16:01:00Z`；CST 当日 00:00 的成功签到不再被重新判为待签到，前一秒的记录仍属于前一业务日。

### 3. 签到日志与账号状态原子提交

- `CheckinExecutor.SaveResult` 以单一事务写入日志和账号最新签到状态。
- 账号更新必须且只能影响一行；失败时回滚日志，通知只在提交成功后发送。

证据：`internal/core/checkin_executor.go`、`internal/core/checkin_time_consistency_test.go`。

### 4. 不支持签到账号清理同源

- 原实现的 dry-run 与真实删除分别按 `updated_at DESC` 重新查询候选，存在“用户预览 A、实际删除 B”的竞态；空请求体还会因 `dryRun=false` 默认值进入删除路径。
- 预览现在冻结账号 ID，返回 5 分钟、一次性的 `previewId/expiresAt`；确认请求只接受 `previewId`，不能重新提交 limit、过滤范围或候选集合。
- 服务端以有界内存 store 执行原子 Claim；确认事务只复核冻结集合。候选缺失、不再符合原条件、过期或重放均返回 409，账号、签到日志、余额快照全部保持不变。
- 数据库恢复重绑会切换 `AccountCleanupService` 的连接并清空旧数据集 preview，防止 token 跨数据集生效。
- 前端通过 `api/account-cleanup.ts` 消费 typed 契约，受控 `UnsupportedCheckinCleanupPanel` 只负责展示冻结候选与二次确认；取消不会发确认请求，任意确认失败都会清空已消费预览并要求重新预览。

证据：`internal/core/account_cleanup_service.go`、`internal/core/accounts.go`、`internal/core/system.go`、`internal/core/account_cleanup_preview_contract_test.go`、`frontend/src/api/account-cleanup.ts`、`frontend/src/components/accounts/UnsupportedCheckinCleanupPanel.tsx`。

兼容性影响：仓内只有 `AccountInsights` 使用此端点，已同步迁移；仓外脚本是否调用该破坏性端点**不确定**。旧的空 body 或无 `previewId` 删除请求现在返回 400，这是有意的 fail-closed 收紧。

## 当前验证结果

| 检查 | 结果 |
|---|---|
| 新增安全/时区/事务针对性回归 | 通过，27 项 |
| Go core | 534 项通过 |
| Go 全量 | 13 包、1148 项通过 |
| `go vet -mod=vendor ./...` | 通过 |
| `git diff --check` | 通过 |
| 前端格式与 lint | 通过 |
| 前端测试与覆盖率 | 48 文件、324 测试通过；56.22 / 48.41 / 44.53 / 57.44 |
| 前端生产构建与包体预算 | 通过 |
| 增量浏览器烟测 | 通过：签到 dry-run 8 次、start 2 次；账号清理 preview 3 次、confirm 3 次 |
| 导航/排程/布局烟测 | 通过：导航 9/9，1440/900/390px 无错误或横向溢出 |

烟测所需 Vite 仅在本机 `127.0.0.1:5173` 临时运行；验证后已停止，并确认端口无监听。

## 已确认执行的高风险收口

### Windows 实例密钥 ACL

- 已将实际 `data\keys\instance.key` 从继承式宽权限 DACL 改为受保护 DACL，仅保留当前用户、SYSTEM 和 Administrators 的完全控制。
- 启动时的 `loadOrCreateKey` 现在对新旧实例密钥都执行同一策略并验证；新建密钥加固失败时会删除刚创建的文件，已有密钥加固失败时拒绝继续使用。
- 代码与回归证据：`internal/core/crypto.go`、`internal/core/session_token_file_windows.go`、`internal/core/crypto_test.go`。

### CI 增量浏览器烟测

- `.github/workflows/ci.yml` 已在前端构建与包体预算后安装 Playwright Chromium，并运行增量浏览器烟测。
- `frontend/scripts/run-incremental-smoke.ps1` 负责启动 Vite、等待 `127.0.0.1:5173`、运行烟测并在成功或失败时终止整个进程树。
- 该脚本已在本机 Windows PowerShell 5.1 验证通过，且端口清理通过验证；远程 GitHub Actions 尚未触发，因为当前工作树未提交或推送。

最新后端验证：13 包、1148 项测试通过；`go vet`、`go build -mod=vendor ./...` 与 `git diff --check` 通过。

## 保留风险与需要确认的事项

下列事项不在本次直接修改范围内，原因是会改变真实数据、凭据可用性、CI 或运行环境：

1. SQLite 外键/站点删除语义迁移与历史孤儿清理：本轮只收紧既有“清理不支持签到账号”命令，没有改变站点删除语义。预检确认实际数据库仍有 WAL 文件；仍需确认删除站点时账号、日志、余额快照的保留策略，并备份真实数据。
2. 备份恢复与 WAL/后台任务协调：需设计维护窗口、恢复回滚和全部 service 的数据库重绑验收。
3. 代理模式下的 DNS rebinding 防护：仓内仅有 HTTP 代理配置示例，未提供生产代理协议、DNS 解析位置及兼容性要求，不能安全实现 pinned CONNECT/目标解析策略。
4. 真实上游签到、桌面打包签名、提交、推送、部署和生产性能/RUM 验收：未提供目标、签名材料或运行权限；本次均未执行。

## 审查结论

本轮代码审查未发现新增的阻断性正确性、安全性、可维护性或性能问题。当前工作树仍是大规模未提交集合，建议在任何提交前按逻辑拆分并再次审查；不要把本地模拟烟测等同于真实上游或生产环境验收。
