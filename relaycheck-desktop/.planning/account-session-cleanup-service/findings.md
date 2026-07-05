# Findings: Account Session Cleanup Service

## Source Findings

- `accounts.go` still owns clear-session execution:
  - deletes `BrowserSessionStore` entry
  - reads `browser_profile_path`
  - removes profile directory when under `dataDir`
  - clears session fields in `channel_accounts`
  - writes `browser_auth.disconnected` audit
- Existing coverage:
  - `audit_test.go` covers handler success and audit action.
  - `browser_session_store_test.go` covers store primitives.
- Missing coverage:
  - service-level DB/session/profile cleanup.
  - explicit protection for profile paths outside `dataDir`.

## Decisions

- Extract `AccountSessionCleanupService` inside `package core`.
- Keep `clearAccountSession` as thin HTTP method/write wrapper.
- Keep path protection semantics compatible with current `filepath.Clean` + `HasPrefix` guard.
- Add service-level tests before implementation.
