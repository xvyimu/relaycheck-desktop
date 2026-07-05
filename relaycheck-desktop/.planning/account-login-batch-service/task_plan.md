# Task Plan: Account Login Batch Service

## Goal

Move bulk account login orchestration out of `internal/core/accounts.go` into a focused `AccountLoginBatchService` in `package core`.

This slice preserves the public HTTP routes, JSON response shapes, database schema, browser login behavior, notification behavior, and frontend behavior. It reduces risk around bulk password re-login, bulk browser-login window opening, and bulk browser-login session saving.

## Current Phase

- Phase 1: Planning - complete
- Phase 2: TDD implementation - complete
- Phase 3: Review - complete

## Task List

- [x] OBSERVE: Re-read current bulk login handlers and existing login services.
- [x] RED: Add focused service-level tests for bulk account selection and injected single-account actions.
- [x] GREEN: Add `account_login_batch_service.go`, wire `App.accountLoginBatch`, and keep account handlers/wrappers compatible.
- [x] REFACTOR: Move bulk password login, bulk browser open, bulk browser finish, and retry-password-login bodies behind the service.
- [x] DOCS: Update `internal/core/PACKAGE_INDEX.md`.
- [x] CHECK: Run focused bulk login tests, core tests, full Go tests, vet, build, frontend test/build, and diff check.
- [x] REVIEW: Inspect for API shape drift, Chrome launch side effects, login status persistence regressions, SQL issues, and broad scope creep.

## Scope

In scope:

- `internal/core/account_login_batch_service.go`
- `internal/core/account_login_batch_service_test.go`
- `internal/core/accounts.go`
- `internal/core/app.go`
- `internal/core/PACKAGE_INDEX.md`

Out of scope:

- Database schema changes
- Public HTTP API changes
- Frontend behavior changes
- BrowserLoginService internals
- AccountSessionService login protocol behavior
- Account creation/update handlers
- Fixing unrelated historical mojibake text

## Acceptance Criteria

- `accounts.go` no longer owns the full bulk login orchestration bodies.
- Existing HTTP handlers remain available and return the same top-level JSON keys.
- Bulk password login still selects password-backed accounts with expired/manual/unknown login state or failed/auth-expired checkin state.
- Bulk password login still marks missing credentials as `manual_required` and login failures as `expired`.
- Bulk browser open still respects explicit IDs, limit clamping, and fallback account selection.
- Bulk browser finish still defaults to active browser sessions and caps processing at 10.
- Existing BrowserLoginService and AccountSessionService behavior is reused instead of duplicated.
- All SQL remains parameterized.

## Verification Commands

```powershell
rtk go test -mod=vendor -count=1 ./internal/core -run "AccountLoginBatch|BulkPassword|BulkOpenBrowser|BulkFinishBrowser"
rtk go test -mod=vendor -count=1 ./internal/core
rtk go test -mod=vendor -count=1 ./...
rtk go vet -mod=vendor ./...
rtk go build -mod=vendor ./...
cd frontend; rtk npm test
cd frontend; rtk npm run build
rtk git diff --check
```

## Risks

| Risk | Impact | Mitigation |
| --- | --- | --- |
| Bulk endpoint response shape changes | High | Keep handlers writing the same maps and add service tests for counts/results |
| Browser windows start during tests | High | Inject open/save functions and test orchestration without launching Chrome |
| Password login status persistence regresses | High | Move existing DB updates unchanged and test missing-credential path |
| Account selection changes | Medium | Keep existing SQL predicates and limit handling |
