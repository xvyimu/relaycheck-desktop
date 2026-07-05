# Task Plan: Account Site Update Service

## Goal

Move account site update execution out of `internal/core/accounts.go` into a focused `AccountSiteUpdateService` in `package core`.

This slice preserves account update HTTP behavior, database schema, JSON response shape, audit/notification behavior, and frontend behavior. It reduces risk around editing account site URLs, manual login URLs, kind metadata, and shared-site updates.

## Current Phase

- Phase 1: Planning - complete
- Phase 2: TDD implementation - complete
- Phase 3: Review - complete

## Task List

- [x] OBSERVE: Re-read current site update functions and tests.
- [x] RED: Add a focused service-level test for shared-scope account site update behavior.
- [x] GREEN: Add `account_site_update_service.go`, wire `App.accountSiteUpdates`, and keep `App` wrappers.
- [x] REFACTOR: Move `resolveAccountSiteUpdate`, `updateSharedAccountSite`, `updateAccountSiteAddress`, and `updateAccountSiteMetadata` bodies behind the service.
- [x] DOCS: Update `internal/core/PACKAGE_INDEX.md`.
- [x] CHECK: Run focused site update tests, core tests, full Go tests, vet, build, frontend test/build, and diff check.
- [x] REVIEW: Inspect for account reassignment drift, SQL issues, manual login URL metadata regressions, and broad scope creep.

## Scope

In scope:

- `internal/core/account_site_update_service.go`
- `internal/core/accounts.go`
- `internal/core/app.go`
- focused tests in `sites_test.go` or a new account site update test file
- `internal/core/PACKAGE_INDEX.md`

Out of scope:

- Database schema changes
- Public HTTP API changes
- Frontend behavior changes
- Changing account CRUD fields unrelated to site updates
- Moving full account update handler
- Changing site detection/import behavior

## Acceptance Criteria

- `accounts.go` no longer owns the full account site update bodies.
- Existing `App` methods remain available as thin compatibility wrappers.
- Current-scope updates still create/choose a single account-specific site when the base URL changes.
- Shared-scope updates still update or merge all accounts from the old site into an existing site with the same base URL.
- Manual login URL updates still mark `login_url_source='manual'`, confidence `1`, and write `login_discovery_json`.
- Invalid base URLs and excluded relay sites still return the same validation errors.
- All SQL remains parameterized except fixed, internally generated SET clauses.

## Verification Commands

```powershell
rtk go test -mod=vendor -count=1 ./internal/core -run "AccountSiteUpdate|UpdateAccountSite|Sites"
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
| Shared-scope update reassigns the wrong accounts | High | Add focused service-level test around existing site merge |
| Manual login metadata regresses | High | Keep existing `TestUpdateAccountSiteMetadataMarksLoginURLManual` and route wrapper |
| Current-scope behavior changes | Medium | Preserve wrapper and existing helper flow |
| Broad account update churn | Medium | Limit this slice to site update functions only |
