# Findings: Account Validation Service

## Source Findings

- `internal/core/accounts.go` is the largest core file and still owns account CRUD plus validation paths.
- Already extracted services exist for nearby responsibilities:
  - `AccountAuthRepository` loads decrypted account auth context.
  - `AccountAPIClient` builds account API requests and handles timeout-aware calls.
  - `AccountSessionService` handles password login and session persistence.
  - `AccountTaskService` orchestrates SSE API key test tasks.
- Remaining hotspot in `accounts.go`:
  - `testAccountLogin`
  - `testAPIKeyForAccount`
  - `speedTestAPIKeyModel`
  - handler glue around single and bulk API key tests.

## Decisions

- Keep the new service in `package core` to avoid export churn around `accountAuthContext` and result structs.
- Preserve existing `App` wrappers so scheduler/task/handler call sites do not need a broad rewrite.
- Do not change public API response fields or database writes in this slice.

