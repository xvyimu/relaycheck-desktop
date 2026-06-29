# 遗留项深化设计

**日期**：2026-06-29
**状态**：已确认，待实施
**优先级顺序**：低风险优先（Part 1 → 2 → 3 → 4）

## 背景

6 批代码审查修复完成后，遗留 4 项需要较大重构的改进。本 spec 覆盖这 4 项的设计，按低风险优先顺序实施。每项独立可测、独立可回滚，无跨项依赖。

## Part 1：删除 python_migration.go 死代码

### 决策依据

用户确认 Python 版数据已全部迁移完成，`python_migration.go` 不会再被执行。

### 范围

- 删除 `internal/core/python_migration.go` 整个文件
- 删除 `internal/core/python_migration_test.go`（如果存在）
- 移除 `routes.go` / `app.go` 中的路由注册和调用点
- 移除相关类型定义（如果有独立定义）

### 步骤

1. Grep `python_migration` / `importPython` / `verifyPython` / `PythonMigration` 确认所有引用点
2. 删除文件本身
3. 移除路由注册（`/api/migrate/python` 等）
4. 移除 handler 注册
5. `go build ./...` + `go test ./...` 验证无断裂

### 风险

极低 — 已确认迁移完成，代码不会执行。

## Part 2：AnalyticsPanel 钻取 effect fallback 清理

### 问题

`AnalyticsPanel.tsx:420-443` 的钻取 effect 中，`Promise.resolve(balanceSnapshots)` 和 `Promise.resolve(checkinLogs)` 的 fallback 值从未被消费 — 后续 `if (selectedDate) setBalanceSnapshots(snapshots)` 守卫阻止了 fallback 值的 setter 调用。eslint-disable 注释掩盖了这个问题。

### 修复

将 fallback 改为空数组占位，移除 eslint-disable，deps 完整化：

```tsx
useEffect(() => {
  if (!selectedDate && !selectedStatus) return;
  let cancelled = false;
  async function loadDrill() {
    setDrillLoading(true);
    try {
      const [snapshots, logs] = await Promise.all([
        selectedDate ? api<BalanceSnapshot[]>("/api/balances/snapshots") : Promise.resolve([] as BalanceSnapshot[]),
        selectedStatus ? api<CheckinLog[]>("/api/checkins/logs") : Promise.resolve([] as CheckinLog[]),
      ]);
      if (!cancelled) {
        if (selectedDate) setBalanceSnapshots(snapshots);
        if (selectedStatus) setCheckinLogs(logs);
      }
    } catch {
      // ignore
    } finally {
      if (!cancelled) setDrillLoading(false);
    }
  }
  void loadDrill();
  return () => { cancelled = true; };
}, [selectedDate, selectedStatus]);
```

### 风险

极低 — fallback 值原本就未被消费，行为不变。

## Part 3：错误信息统一为中文

### 范围

统一 `notification.go`、`checkin_balance.go` 中 HTTP 状态码相关的英文错误信息为中文。

### 改动清单

**`internal/core/notification.go`**（6 处）：

| 行号 | 原文 | 改为 |
|------|------|------|
| 865 | `"HTTP %d (不重试)"` | `"HTTP 状态码 %d（不重试）"` |
| 867 | `"HTTP %d"` | `"HTTP 状态码 %d"` |
| 993 | `"HTTP %d"` | `"HTTP 状态码 %d"` |
| 1043 | `"Telegram API HTTP %d"` | `"Telegram API 返回 HTTP 状态码 %d"` |
| 1096 | `"Bark HTTP %d"` | `"Bark 返回 HTTP 状态码 %d"` |
| 1143 | `"ServerChan API HTTP %d"` | `"ServerChan API 返回 HTTP 状态码 %d"` |

**`internal/core/checkin_balance.go`**（4 处）：

| 行号 | 原文 | 改为 |
|------|------|------|
| 1003 | `"%s 登录态不可用：HTTP %d"` | `"%s 登录态不可用：HTTP 状态码 %d"` |
| 1007 | `"%s 返回 HTTP %d"` | `"%s 返回 HTTP 状态码 %d"` |
| 1194 | `"HTTP %d: %s"` | `"HTTP 状态码 %d：%s"` |

### 前置检查

- Grep 前端代码确认无 `switch(err.message)` 或字符串匹配依赖这些文案
- 如有前端依赖，改为匹配 `errorClass` 字段而非文案

### 不改动项

- `db.go` / `app.go` 的安全校验错误（`"invalid origin"` 等）— 内部错误，不直接面向用户，保持英文符合安全惯例
- `log.Printf` 的日志行 — 日志保持英文便于工具处理

### 风险

低 — 需确认前端不依赖字面量匹配。

## Part 4：I3 N+1 批量化（loadAccountAuths 预加载）

### 当前状况

`loadAccountAuth(ctx, id)` 被以下 6 个原子函数调用：
- `runAccountCheckin`、`testAPIKeyForAccount`、`refreshAccountBalance`
- `retryPasswordLogin`、`startBrowserLogin`、`saveBrowserLoginSession`

这些原子函数又被 9+ 个批量函数循环调用，形成 N+1。已知痛点：500+ 账号时显著性能退化。

### 设计

**1. 新增批量预加载函数**：

```go
func (a *App) loadAccountAuths(ctx context.Context, ids []string) (map[string]accountAuthContext, error) {
    // 用 WHERE a.id IN (?,?,...) 一次加载所有账号的 auth context
    // 返回 map[accountID]accountAuthContext
}
```

SQL 与现有 `loadAccountAuth` 相同，仅 WHERE 子句改为 `IN (...)`。

**2. 原子函数加 `auth *accountAuthContext` 参数**（nil 时内部加载，保持向后兼容）：

```go
func (a *App) runAccountCheckin(ctx context.Context, id string, auth *accountAuthContext) (checkinResult, error) {
    if auth == nil {
        loaded, err := a.loadAccountAuth(ctx, id)
        if err != nil { return checkinResult{}, err }
        auth = &loaded
    }
    // ... 原有逻辑用 *auth
}
```

同样修改：`testAPIKeyForAccount`、`refreshAccountBalance`、`retryPasswordLogin`、`startBrowserLogin`、`saveBrowserLoginSession`。

**3. 批量函数开头预加载**：

```go
func (a *App) runDueCheckinsWithFilter(...) {
    ids := make([]string, 0, len(accounts))
    for _, acc := range accounts { ids = append(ids, acc.ID) }
    auths, _ := a.loadAccountAuths(ctx, ids) // 失败则 auths 为 nil，原子函数内部 fallback
    for _, account := range accounts {
        var auth *accountAuthContext
        if loaded, ok := auths[account.ID]; ok {
            auth = &loaded
        }
        // auth 为 nil 时，原子函数内部调 loadAccountAuth 单查（降级为 N+1）
        result, err := a.runAccountCheckin(ctx, account.ID, auth)
        // ...
    }
}
```

需修改的批量函数清单：
- `runDueCheckinsWithFilter` (checkin_balance.go)
- `handleBulkRefreshBalances` (checkin_balance.go)
- `handleBulkPasswordLogin` (accounts.go)
- `handleBulkOpenBrowserLogin` (accounts.go)
- `handleBulkFinishBrowserLogin` (accounts.go)
- `handleBulkTestAPIKeys` (accounts.go)
- `startCheckinTask` (task_runner.go)
- `startTestKeysTask` (task_runner.go)
- `startRefreshBalancesTask` (task_runner.go)
- `handleModelSync` (models_pricing.go)

**4. 错误处理策略**：

`loadAccountAuths` 失败时，批量函数不中断 — 各原子函数内部 fallback 到单查 `loadAccountAuth`。这样即使预加载失败，功能仍正常（降级为原 N+1 行为）。

**5. 测试**：

- 新增 `TestLoadAccountAuthsBatchLoadsAllAccounts` — 验证批量加载正确性
- 新增 `TestLoadAccountAuthsHandlesEmptyIDs` — 空列表边界
- 修改原子函数的现有测试调用 — 加 `nil` 参数
- 修改批量函数的现有测试 — 验证预加载后行为不变

### 风险控制

- 原子函数 `auth *accountAuthContext` 为 nil 时行为完全等同原逻辑
- 批量函数预加载失败时降级为原行为
- 现有测试加 `nil` 参数即可通过，无断言变化

## 验收标准

1. `go build ./...` 通过
2. `go test ./... -count=1` 全部通过
3. `go vet ./...` 无告警
4. `npx tsc --noEmit` 通过
5. `npm run build` 通过
6. Part 1：`grep -r "python_migration" internal/core/` 无结果
7. Part 2：AnalyticsPanel 钻取 effect 无 eslint-disable
8. Part 3：notification.go / checkin_balance.go 无裸 `"HTTP %d"` 模式
9. Part 4：`grep -rn "loadAccountAuth(" internal/core/ | grep -v "_test.go"` 中，直接调用 `loadAccountAuth` 的仅剩 `loadAccountAuths` 内部的 fallback 和原子函数内的 nil 检查路径

## 不在本 spec 范围

- 其他 Important/Suggestion 项（已在 6 批修复中处理）
- 前端其他 race condition / memoize 改进
- 架构层重构（如 Repository 模式）
