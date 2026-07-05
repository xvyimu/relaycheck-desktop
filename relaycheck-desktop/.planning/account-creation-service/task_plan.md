# Task Plan: Account Creation Service

## Goal

Move account creation and manual account-site creation execution out of `internal/core/accounts.go` into a focused `AccountCreationService` in `package core`.

This slice preserves account creation HTTP behavior, database schema, JSON response shape, audit/notification behavior, site detection behavior, manual login URL metadata, and frontend behavior. It reduces risk around credential encryption, auth-type inference, browser-profile defaults, and manual upstream-site creation.

## Current Phase

- Phase 1: Planning - complete
- Phase 2: TDD implementation - complete
- Phase 3: Review - complete

## Task List

- [x] OBSERVE: Re-read current account creation, manual site creation, helper tests, and site metadata tests.
- [x] RED: Add focused service-level tests for account creation and injected manual-site detection.
- [x] GREEN: Add `account_creation_service.go`, wire `App.accountCreation`, and keep HTTP/compatibility wrappers.
- [x] REFACTOR: Move `createAccount` execution and `ensureManualAccountSite` body behind the service.
- [x] DOCS: Update `internal/core/PACKAGE_INDEX.md`.
- [x] CHECK: Run focused account creation tests, core tests, full Go tests, vet, build, frontend test/build, and diff check.
- [x] REVIEW: Inspect for credential persistence drift, bad-request/internal-error mapping drift, manual login metadata regression, SQL issues, and broad scope creep.

## Scope

In scope:

- `internal/core/account_creation_service.go`
- `internal/core/account_creation_service_test.go`
- `internal/core/accounts.go`
- `internal/core/app.go`
- `internal/core/PACKAGE_INDEX.md`

Out of scope:

- Database schema changes
- Public HTTP API changes
- Frontend behavior changes
- Site detection algorithm changes
- Existing account update/delete/list handlers
- Fixing unrelated historical mojibake text

## Acceptance Criteria

- `accounts.go` no longer owns the full account creation and manual account-site creation bodies.
- `createAccount` remains the HTTP boundary and returns the same success shape: `{"id": "<id>"}`.
- Decode errors remain HTTP 400.
- Missing/invalid manual site inputs still return HTTP 400.
- Credential encryption, API-key fingerprint/status, default display name, inferred auth type, browser-profile path/status, audit, and notification behavior are preserved.
- Manual site creation still stores manual login URL metadata with `login_url_source='manual'`, confidence `1`, and `login_discovery_json`.
- Existing `ensureManualAccountSite` remains available as a thin compatibility wrapper for other services/tests.
- All SQL remains parameterized.

## Verification Commands

```powershell
rtk go test -mod=vendor -count=1 ./internal/core -run "AccountCreation|EnsureManualAccountSite|DefaultAccountDisplayName|InferAccountAuthType"
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
| Credential encryption/persistence changes | High | Add service-level create test and reuse CryptoService encryption path |
| Manual site detection/metadata changes | High | Inject detection in tests and keep SQL/body unchanged |
| HTTP status mapping drift | Medium | Keep handler responsible for decode/write and map service bad-request errors explicitly |
| Existing wrappers break downstream services | Medium | Preserve `ensureManualAccountSite` wrapper |
