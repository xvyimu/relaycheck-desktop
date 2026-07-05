# Task Plan: Account Cleanup Service

## Goal

Move unsupported-checkin account cleanup execution out of `internal/core/accounts.go` into a focused `AccountCleanupService` in `package core`.

This slice preserves the public cleanup endpoint, JSON response shape, database schema, audit/notification behavior, and frontend behavior. It improves launch operations by making destructive account cleanup easier to test and review.

## Current Phase

- Phase 1: Planning - complete
- Phase 2: TDD implementation - complete
- Phase 3: Review - complete

## Task List

- [x] OBSERVE: Re-read current cleanup functions and existing cleanup tests.
- [x] RED: Add a focused service-level test for unsupported-checkin cleanup behavior.
- [x] GREEN: Add `account_cleanup_service.go`, wire `App.accountCleanup`, and keep `App` wrappers.
- [x] REFACTOR: Move cleanup result types and deletion/loading bodies behind the service.
- [x] DOCS: Update `internal/core/PACKAGE_INDEX.md`.
- [x] CHECK: Run focused cleanup tests, core tests, full Go tests, vet, build, and diff check.
- [x] REVIEW: Inspect for accidental deletion scope drift, audit/notification changes, and SQL issues.

## Scope

In scope:

- `internal/core/account_cleanup_service.go`
- `internal/core/accounts.go`
- `internal/core/app.go`
- cleanup tests
- `internal/core/PACKAGE_INDEX.md`

Out of scope:

- Database schema changes
- Public HTTP API changes
- Frontend behavior changes
- Changing cleanup matching rules
- Moving CRUD/update-site logic
- Adding new destructive cleanup endpoints

## Acceptance Criteria

- `accounts.go` no longer owns the unsupported-checkin cleanup query/deletion body.
- `deleteUnsupportedCheckinAccounts` and `loadUnsupportedCheckinAccounts` remain available as thin compatibility wrappers.
- Dry-run never deletes accounts.
- Non-dry-run deletes only matched account rows plus their checkin logs and balance snapshots.
- `includeLastUnsupported=false` still excludes accounts that only have `last_checkin_status='unsupported'`.
- Handler-level notification and audit behavior remains unchanged.
- All SQL remains parameterized except fixed, internally generated placeholders.

## Verification Commands

```powershell
rtk go test -mod=vendor -count=1 ./internal/core -run "AccountCleanup|DeleteUnsupportedCheckinAccounts"
rtk go test -mod=vendor -count=1 ./internal/core
rtk go test -mod=vendor -count=1 ./...
rtk go vet -mod=vendor ./...
rtk go build -mod=vendor ./...
rtk git diff --check
```

## Risks

| Risk | Impact | Mitigation |
| --- | --- | --- |
| Cleanup deletes broader account set | High | Preserve SQL predicate and add includeLastUnsupported=false test |
| Dry-run mutates data | High | Keep existing dry-run tests and service-level test |
| Handler audit/notification changes | Medium | Leave handler ownership in `accounts.go` |
| Cache invalidation lost after delete | Medium | Inject and call existing `invalidateReadCache` |
