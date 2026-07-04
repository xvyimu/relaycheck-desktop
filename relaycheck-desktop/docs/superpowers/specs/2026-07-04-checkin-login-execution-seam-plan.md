# Implementation Plan: 签到与登录执行缝架构优化

**关联设计：** [2026-07-04-checkin-login-execution-seam-design.md](2026-07-04-checkin-login-execution-seam-design.md)
**日期：** 2026-07-04
**状态：** 实施中；核心执行服务已落地，任务服务收尾切片待提交

---

## Overview

本计划把签到、余额、密码登录、账号 API 调用和浏览器授权链路从 `checkin_balance.go` / `accounts.go` 的大块逻辑中抽成三个 `internal/core` 内部服务：`BrowserLoginService`、`AccountAPIClient`、`AccountSessionService`。本阶段不拆新包、不改 DB schema、不改公开 API，只缩小执行边界并补足关键回归测试。

2026-07-04 进度备注：

- 已落地 `BrowserLoginService`、`AccountAPIClient`、`AccountSessionService`、`CheckinExecutor`、`BalanceRefresher`、`CheckinBatchOrchestrator`。
- 当前收尾切片新增 `CheckinTaskService`，将 `task_runner.go` 中 `checkin` / `refresh_balances` 两个 SSE 任务业务体迁出，只保留通用任务引擎和 HTTP/SSE 生命周期。
- 本切片不改数据库、HTTP API 或前端行为；验证通过 `go test ./...`、`go vet ./...`、`go build ./...`、前端 `npm test` 和 `npm run build`。

## Dependency Graph

```text
accountAuthContext / BrowserLoginSession / CryptoService / BrowserSessionStore
    │
    ├── BrowserLoginService
    │       └── open-browser-login / finish-browser-login thin wrappers
    │
    ├── AccountAPIClient
    │       ├── checkin API calls
    │       ├── balance API calls
    │       └── API key model test calls
    │
    └── AccountSessionService
            ├── password login
            ├── session save
            └── Ensure before checkin / balance
```

Implementation order follows the lowest-risk seam first:

1. Extract browser login URL/session behavior.
2. Extract account API request behavior.
3. Extract password login/session persistence.
4. Switch checkin, balance, and API key callers onto the services.

## Architecture Decisions

- Keep all new services in `package core` so they can reuse existing unexported types and helpers without import churn.
- Preserve existing `*App` methods as thin forwarders during the transition; this keeps scheduler, task runner, handlers, and tests stable.
- Do not change route paths, response JSON shapes, DB columns, or frontend behavior.
- Prefer behavior-preserving moves plus focused tests over broad cleanup.
- Update `internal/core/PACKAGE_INDEX.md` only after the service boundaries are real.

## Task List

### Task 1: Extract BrowserLoginService

**Description:** Move browser login target resolution, Chrome launch/session tracking, and browser cookie save logic from `accounts.go` into `browser_login_service.go`. Keep `openBrowserLogin`, `finishBrowserLogin`, `startBrowserLogin`, `saveBrowserLoginSession`, `resolveLoginTargetURL`, and `resolveManualLoginTargetURL` as compatible thin wrappers or moved helpers with equivalent names where tests require them.

**Acceptance criteria:**

- [ ] Automatic discovered/candidate login URLs remain same-origin guarded.
- [ ] Manual absolute login URLs remain allowed.
- [ ] `open-browser-login` and `finish-browser-login` responses are unchanged.
- [ ] Browser session cleanup by PID still prevents deleting a newer session.

**Verification:**

- [ ] `rtk go test -mod=vendor -count=1 ./internal/core -run "BrowserLogin|ResolveLogin|AccountAuth"`
- [ ] `rtk go test -mod=vendor -count=1 ./internal/core -run TestStartBrowserLogin`
- [ ] `rtk git diff --check`

**Dependencies:** None

**Files likely touched:**

- `internal/core/browser_login_service.go`
- `internal/core/accounts.go`
- `internal/core/app.go`
- `internal/core/balance_bulk_test.go`
- `internal/core/account_auth_repo_test.go`

**Estimated scope:** Medium: 5 files

### Task 2: Extract AccountAPIClient

**Description:** Move account API request construction and response reading from `checkin_balance.go` / `accounts.go` into `account_api_client.go`. The client owns auth header assembly and timeout variants, while callers continue parsing checkin, balance, model, and API key results.

**Acceptance criteria:**

- [ ] Cookie, Access Token, API key, and user ID headers match current behavior.
- [ ] API key and token values still receive `Bearer ` when needed.
- [ ] Response body read limit remains 256 KiB.
- [ ] `callAccountAPI` and `callAccountAPIWithTimeout` remain available as thin `App` forwarders during migration.

**Verification:**

- [ ] `rtk go test -mod=vendor -count=1 ./internal/core -run "APIKey|CallAccountAPI|BulkTestAPIKeys"`
- [ ] `rtk go test -mod=vendor -count=1 ./internal/core -run "Balance|Checkin"`
- [ ] `rtk git diff --check`

**Dependencies:** Task 1

**Files likely touched:**

- `internal/core/account_api_client.go`
- `internal/core/checkin_balance.go`
- `internal/core/accounts.go`
- `internal/core/app.go`
- `internal/core/bulk_test_api_keys_test.go`

**Estimated scope:** Medium: 5 files

### Task 3: Extract AccountSessionService

**Description:** Move `ensureAccountSession`, `loginWithPassword`, `saveAccountSession`, and closely related login response parsing/persistence glue into `account_session_service.go`. Keep parsing helpers that are shared by account API tests in `core` or move them with compatibility wrappers.

**Acceptance criteria:**

- [ ] Password login still tries existing login path candidates and payload variants.
- [ ] Cookie login success encrypts and saves `cookie_encrypted`.
- [ ] Token login success encrypts and saves `access_token_encrypted` plus `auth_user_id`.
- [ ] Successful login mutates the in-memory `accountAuthContext` so later API calls reuse the new session.
- [ ] Failure messages preserve current Chinese wording and do not leak secrets.

**Verification:**

- [ ] `rtk go test -mod=vendor -count=1 ./internal/core -run "LoginWithPassword|BalanceBulk|CheckinStatus"`
- [ ] `rtk go test -mod=vendor -count=1 ./internal/core -run "Secrets|Audit"`
- [ ] `rtk git diff --check`

**Dependencies:** Task 2

**Files likely touched:**

- `internal/core/account_session_service.go`
- `internal/core/checkin_balance.go`
- `internal/core/app.go`
- `internal/core/balance_bulk_test.go`
- `internal/core/secrets_security_test.go`

**Estimated scope:** Medium: 5 files

### Task 4: Switch Execution Callers and Update Package Index

**Description:** Convert `runAccountCheckin`, `refreshAccountBalance`, `testAPIKeyForAccount`, `speedTestAPIKeyModel`, bulk balance refresh, and task runner balance paths to use the extracted services directly where it improves clarity. Leave thin `App` wrappers only for backward compatibility or tests. Update `PACKAGE_INDEX.md` to document the new service files.

**Acceptance criteria:**

- [ ] `checkin_balance.go` no longer contains full password login, session save, or account API request bodies.
- [ ] `accounts.go` no longer contains full browser login open/save bodies.
- [ ] `PACKAGE_INDEX.md` lists the new service files and their responsibilities.
- [ ] Scheduler, task runner, account handlers, and frontend API behavior remain unchanged.

**Verification:**

- [ ] `rtk go test -mod=vendor -count=1 ./internal/core`
- [ ] `rtk go test -mod=vendor -count=1 ./...`
- [ ] `rtk go vet -mod=vendor ./...`
- [ ] `rtk go build -mod=vendor ./...`
- [ ] `cd frontend; rtk npm run build`
- [ ] `cd frontend; rtk npm test`
- [ ] `rtk git diff --check`

**Dependencies:** Tasks 1-3

**Files likely touched:**

- `internal/core/checkin_balance.go`
- `internal/core/accounts.go`
- `internal/core/task_runner.go`
- `internal/core/app.go`
- `internal/core/PACKAGE_INDEX.md`

**Estimated scope:** Medium: 5 files

## Checkpoints

### Checkpoint A: After Task 1

- [ ] Browser login tests pass.
- [ ] Login URL same-origin protection remains covered.
- [ ] No checkin, balance, or API key behavior changed.

### Checkpoint B: After Task 2

- [ ] Account API header behavior is covered by focused tests.
- [ ] Checkin and balance tests still pass through compatibility wrappers.
- [ ] API key model test path still uses timeout-aware calls.

### Checkpoint C: After Task 3

- [ ] Password login and session persistence behavior is covered.
- [ ] Existing bulk balance and checkin tests still pass.
- [ ] No secret leakage regressions.

### Checkpoint D: Complete

- [ ] All design acceptance criteria are met.
- [ ] Full Go and frontend verification passes.
- [ ] `PACKAGE_INDEX.md` reflects the new boundaries.
- [ ] Work is in reviewable commits.

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Service extraction accidentally changes API response shape | High | Keep handlers and response structs unchanged; use thin wrappers first |
| Browser login URL resolver regresses manual cross-origin behavior | High | Preserve `resolveManualLoginTargetURL` semantics and keep tests |
| Account API client changes auth header priority | High | Add tests for Cookie, Access Token, API key, and user ID header combinations |
| Password login move breaks in-memory session reuse | Medium | Test that `accountAuthContext` receives Cookie/Token after successful login |
| Extraction expands beyond five-file tasks | Medium | Stop at thin wrappers and defer deeper cleanup to a follow-up task |
| Full frontend tests are slow or environment-sensitive | Low | Run them as final regression gates; no frontend production changes expected |

## Open Questions

- None blocking. The plan assumes no DB schema changes, no public API changes, and no frontend behavior changes.

## Parallelization Opportunities

- Tasks 1-3 should be sequential because they share `accounts.go`, `checkin_balance.go`, and `app.go`.
- Additional tests for already-extracted helpers can be written in parallel only after each task lands.
- Documentation updates should wait until Task 4 so `PACKAGE_INDEX.md` matches real code.

## Implementation Order

1. Task 1: Extract `BrowserLoginService`.
2. Task 2: Extract `AccountAPIClient`.
3. Task 3: Extract `AccountSessionService`.
4. Task 4: Switch callers, update `PACKAGE_INDEX.md`, run full verification.

Each task should leave the repo buildable and testable. Prefer one commit per task when the diff is coherent; split further if tests reveal hidden coupling.

---

**批准后下一步：** 从 Task 1 开始实现 `BrowserLoginService`，先补/保留 URL 解析和浏览器登录回归测试，再移动代码。
