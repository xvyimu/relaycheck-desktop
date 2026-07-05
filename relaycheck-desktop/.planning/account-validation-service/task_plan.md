# Task Plan: Account Validation Service

## Goal

Move account login validation and API key validation execution out of `internal/core/accounts.go` into a focused `AccountValidationService` in `package core`.

This slice preserves public HTTP routes, response JSON shapes, database schema, task payloads, and frontend behavior. It should make `accounts.go` thinner while keeping existing account tests green.

## Current Phase

- Phase 1: Planning - complete
- Phase 2: TDD implementation - complete
- Phase 3: Review - complete

## Task List

- [x] OBSERVE: Re-read current `accounts.go`, `account_api_client.go`, and existing API key/login tests.
- [x] RED: Add focused tests for `AccountValidationService` behavior, initially failing because the service does not exist.
- [x] GREEN: Add `account_validation_service.go`, wire it into `App`, and keep old `App` methods as compatibility wrappers.
- [x] REFACTOR: Move `testAccountLogin`, `testAPIKeyForAccount`, and `speedTestAPIKeyModel` bodies behind the service without changing response shapes.
- [x] DOCS: Update `internal/core/PACKAGE_INDEX.md` with the new service boundary.
- [x] CHECK: Run focused core tests, full Go tests, vet, and build gates.
- [x] REVIEW: Inspect the diff for leaks, behavior drift, and accidental broad refactors.

## Scope

In scope:

- `internal/core/account_validation_service.go`
- `internal/core/accounts.go`
- `internal/core/app.go`
- focused tests around account login validation and API key validation
- `internal/core/PACKAGE_INDEX.md`

Out of scope:

- Database schema changes
- Public HTTP API changes
- Frontend behavior changes
- Moving code to a new non-core package
- Rewriting scheduler or task runner behavior
- Changing API key scoring, login status strings, audit semantics, or notification behavior

## Acceptance Criteria

- `accounts.go` no longer owns the full login validation and API key validation bodies.
- `testAccountLogin`, `testAPIKeyForAccount`, and `speedTestAPIKeyModel` remain available as thin wrappers or equivalent call sites.
- `/api/accounts/{id}/test-login` behavior remains compatible.
- `/api/accounts/{id}/test-api-key` and bulk API key testing behavior remain compatible.
- Account API calls continue to use `AccountAPIClient` for request construction.
- Secret values are not exposed in errors, logs, or test output.
- `PACKAGE_INDEX.md` matches the actual service boundary.

## Verification Commands

```powershell
rtk go test -mod=vendor -count=1 ./internal/core -run "AccountValidation|AccountAPIClient|APIKey|BulkTestAPIKeys|TestAccountLogin|Secrets"
rtk go test -mod=vendor -count=1 ./internal/core
rtk go test -mod=vendor -count=1 ./...
rtk go vet -mod=vendor ./...
rtk go build -mod=vendor ./...
rtk git diff --check
```

## Risks

| Risk | Impact | Mitigation |
| --- | --- | --- |
| Login validation response shape changes | High | Keep handler response structs and wrappers unchanged |
| API key result persistence drifts | High | Run existing `APIKey` and `BulkTestAPIKeys` tests |
| Secrets leak into error messages | High | Keep existing masking helpers and run `Secrets` tests |
| Extraction becomes broad CRUD cleanup | Medium | Limit this slice to validation paths only |
