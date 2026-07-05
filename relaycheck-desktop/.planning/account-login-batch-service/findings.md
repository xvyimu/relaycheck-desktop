# Findings: Account Login Batch Service

## Source Findings

- `accounts.go` still owns bulk login orchestration:
  - `handleBulkPasswordLogin`
  - `retryPasswordLogin`
  - `handleBulkOpenBrowserLogin`
  - `handleBulkFinishBrowserLogin`
- Existing single-account services:
  - `BrowserLoginService.Open`
  - `BrowserLoginService.Save`
  - `AccountSessionService.LoginWithPassword`
- Existing result types are shared by handlers and services:
  - `bulkPasswordLoginResult`
  - `browserLoginOpenResult`
  - `browserLoginSaveResult`

## Decisions

- Extract `AccountLoginBatchService` inside `package core`.
- Keep HTTP handlers in `accounts.go` as thin request/response wrappers.
- Inject single-account open/save/password-login functions so service-level tests do not launch Chrome or call external login endpoints.
- Keep existing response top-level keys in handlers.

## Notes

- PowerShell passes `internal/core/*_test.go` globs literally in some `rg` invocations; prefer searching the directory path directly.
- Historical mojibake exists in nearby account text. This slice preserves existing text bytes instead of widening scope.
