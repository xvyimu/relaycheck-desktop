# Findings: Account Cleanup Service

## Source Findings

- `accounts.go` still contains unsupported-checkin cleanup types and logic:
  - `unsupportedCheckinAccountItem`
  - `unsupportedCheckinCleanupResult`
  - `deleteUnsupportedCheckinAccounts`
  - `loadUnsupportedCheckinAccounts`
- Existing tests in `accounts_cleanup_test.go` cover:
  - dry-run preview
  - deleting site-unsupported accounts
  - deleting last-unsupported accounts when included
  - batch `hasMore` behavior
  - cleanup of related `checkin_logs` and `balance_snapshots`
- Handler-level notification and audit behavior is separate from cleanup execution and should stay in `accounts.go`.

## Decision

- Extract cleanup execution into `AccountCleanupService` inside `package core`.
- Keep `App` methods as thin wrappers for existing tests and call sites.
- Add a focused test for the `includeLastUnsupported=false` branch through the new service boundary.

