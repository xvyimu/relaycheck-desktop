# Task Plan: Account Session Cleanup Service

## Goal

Move browser account session cleanup execution out of `internal/core/accounts.go` into a focused `AccountSessionCleanupService` in `package core`.

This slice preserves `/api/accounts/{id}/clear-session` HTTP behavior, database schema, JSON response shape, audit behavior, browser session store behavior, and filesystem safety. It reduces risk around deleting browser profile directories and resetting session credentials.

## Current Phase

- Phase 1: Planning - complete
- Phase 2: TDD implementation - complete
- Phase 3: Review - complete

## Task List

- [x] OBSERVE: Re-read current `clearAccountSession`, browser session store, and audit tests.
- [x] RED: Add service-level tests for DB/session/profile cleanup and out-of-data-dir path protection.
- [x] GREEN: Add `account_session_cleanup_service.go`, wire `App.accountSessionCleanup`, and keep `clearAccountSession` as HTTP wrapper.
- [x] REFACTOR: Move cleanup execution behind the service without changing response shape or audit semantics.
- [x] DOCS: Update `internal/core/PACKAGE_INDEX.md`.
- [x] CHECK: Run focused cleanup tests, core tests, full Go tests, vet, build, frontend test/build, and diff check.
- [x] REVIEW: Inspect for unsafe filesystem deletion, audit drift, DB field reset drift, handler status drift, and broad scope creep.

## Scope

In scope:

- `internal/core/account_session_cleanup_service.go`
- `internal/core/account_session_cleanup_service_test.go`
- `internal/core/accounts.go`
- `internal/core/app.go`
- `internal/core/PACKAGE_INDEX.md`

Out of scope:

- BrowserLoginService open/save behavior
- Chrome runtime helpers
- DB schema changes
- Public HTTP route/JSON changes
- Account deletion or cleanup-service changes

## Acceptance Criteria

- `accounts.go` no longer owns the full clear-session execution body.
- `clearAccountSession` still requires POST and returns `{"cleared": true}` on success.
- In-memory browser session is deleted for the account.
- `cookie_encrypted`, `browser_profile_path`, and `user_agent` are cleared; `login_status` becomes `manual_required`.
- Browser profile directory is removed only when its path is strictly inside `App.dataDir`.
- Paths outside `App.dataDir`, including sibling paths sharing the same string prefix, are not deleted.
- Audit action remains `browser_auth.disconnected`.

## Verification Commands

```powershell
rtk go test -mod=vendor -count=1 ./internal/core -run "AccountSessionCleanup|ClearSession|BrowserSessionStore"
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
| Unsafe recursive delete outside dataDir | High | Add explicit out-of-data-dir protection test |
| DB reset behavior changes | High | Add service-level test for cleared fields/status |
| Audit behavior changes | Medium | Keep handler audit covered and service writes same action |
| Windows temp-dir cleanup flakes | Medium | Use existing `newTestApp` cleanup and small fixture directory |
