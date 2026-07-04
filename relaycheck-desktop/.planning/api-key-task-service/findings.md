# 发现记录：API Key 测试任务服务

## 本地源码发现

- `task_runner.go` 当前保留 `startTestKeysTask`、`startDetectSitesTask`、`startChannelHealthProbeTask` 三个业务任务体。
- `test_keys` 任务从 `channel_accounts` 读取 `api_key_encrypted` 非空账号，按 `api_key_last_checked_at` 和 `updated_at` 排序，批量加载 auth 后逐个调用 `testAPIKeyForAccount`。
- API Key 单账号测试逻辑已经在 `accounts.go` 的 `testAPIKeyForAccount` / `speedTestAPIKeyModel` 中，已有 `accounts_key_test.go` 和 `bulk_test_api_keys_test.go` 覆盖。
- 上一提交已新增 `CheckinTaskService`，可作为任务服务接入模式参考。

## 决策

- 本切片只抽任务编排，不移动 API Key 单账号测试逻辑，避免扩大 blast radius。
- 新服务暂命名 `AccountTaskService`，为后续账号相关任务留扩展位。
