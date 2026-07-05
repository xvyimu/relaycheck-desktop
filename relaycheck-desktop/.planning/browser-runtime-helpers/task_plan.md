# Task Plan: Browser Runtime Helpers

## Goal

Move Chrome DevTools/browser runtime helper code out of `internal/core/accounts.go` into a focused `browser_runtime.go` file in `package core`.

This slice preserves browser login behavior, cookie saving behavior, Chrome discovery behavior, debug-port selection, public HTTP/API behavior, database schema, and frontend behavior. It reduces `accounts.go` to account HTTP forwarding logic and places browser runtime details near `BrowserLoginService`.

## Current Phase

- Phase 1: Planning - complete
- Phase 2: TDD implementation - complete
- Phase 3: Review - complete

## Task List

- [x] OBSERVE: Re-read remaining browser runtime helpers and existing helper tests.
- [x] RED: Add focused test coverage for debug-port selection fallback.
- [x] GREEN: Move `estimateCookieExpiry`, CDP cookie/response types, Chrome session reading, cookie header building, debug-port selection, and Chrome path discovery into `browser_runtime.go`.
- [x] REFACTOR: Keep function signatures unchanged so BrowserLoginService and existing tests continue to call the same helpers.
- [x] DOCS: Update `internal/core/PACKAGE_INDEX.md`.
- [x] CHECK: Run focused browser runtime/helper tests, core tests, full Go tests, vet, build, frontend test/build, and diff check.
- [x] REVIEW: Inspect for browser-login behavior drift, OS-specific path drift, timeout regression, test fragility, and broad scope creep.

## Scope

In scope:

- `internal/core/browser_runtime.go`
- focused helper tests in `internal/core/accounts_helpers_test.go` or new `browser_runtime_test.go`
- `internal/core/accounts.go`
- `internal/core/PACKAGE_INDEX.md`

Out of scope:

- BrowserLoginService behavior changes
- Chrome launch arguments
- DB schema changes
- HTTP route/JSON changes
- Clear-session service extraction
- API key validation refactors

## Acceptance Criteria

- Browser runtime helper bodies no longer live in `accounts.go`.
- `BrowserLoginService` continues using the same helper names.
- Existing cookie header and cookie expiry tests still pass.
- Debug-port selection skips used ports and returns an available local port.
- `findPageWebSocket` still uses a bounded 10-second HTTP client.
- No frontend/API response shape changes.

## Verification Commands

```powershell
rtk go test -mod=vendor -count=1 ./internal/core -run "BuildCookieHeader|EstimateCookieExpiry|FreeDebugPort|BrowserRuntime"
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
| Moving imports breaks accounts.go or browser login build | Medium | Keep package core names unchanged and run full Go build |
| Port-selection test flakes if port range is exhausted | Medium | Mark only one used port and assert returned port is different/nonzero |
| Chrome DevTools timeout behavior changes | High | Move `findPageWebSocket` body unchanged |
