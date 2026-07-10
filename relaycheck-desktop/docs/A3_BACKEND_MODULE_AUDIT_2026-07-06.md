# A3 Backend Module Audit

Date: 2026-07-06
Scope: `internal/core/accounts.go`, `internal/core/checkin_balance.go`, `internal/core/scheduler.go`

## Summary

A3 is no longer starting from a large god-object state. The current backend
already has concrete service boundaries for browser login, account API calls,
session persistence, account validation, account cleanup, account creation,
account-site updates, batch login, checkin execution, balance refresh, batch
checkins, task-facing orchestration, schedule projection, and scheduler
repository access.

This slice completes the requested audit and adds a low-risk scheduler registry
so job tick order and status metadata share one local source of truth.

## `accounts.go` Audit

Remaining responsibilities:

- HTTP entrypoints and method dispatch:
  `handleAccounts`, `handleAccountByID`, bulk login handlers, API key handlers,
  clear-session/delete handlers.
- Read-model SQL for account list/detail:
  `listAccounts`, `loadAccountByID`.
- Update orchestration:
  `updateAccount` still owns request decoding, field-set assembly, credential
  encryption calls, audit field calculation, and response reload.
- Thin compatibility wrappers:
  account creation, account-site update, browser login, account validation,
  API key validation, account cleanup, and login batch services.
- Pure helpers used by tests and services:
  auth type inference, display-name defaults, model ID parsing, API key
  diagnostic sanitizing, updated-field audit mapping.

Recommended next backend slice:

1. Extract `AccountUpdateService` from `updateAccount`.
2. Move the bulk API-key ID query and result loop into `AccountValidationService`.
3. Consider an `AccountReadService` only after deciding whether the read model
   should stay in `core` with `ChannelAccount` or move behind the extracted
   `internal/accounts` package.

Deferred:

- Moving `accounts.go` fully into `internal/accounts`: still too wide because
  the HTTP API uses core response types and cross-domain checkin/balance
  wrappers.

## `checkin_balance.go` Audit

Remaining responsibilities:

- HTTP handlers for today logs, status, bulk balance refresh, checkin logs,
  run-all checkins, and balance snapshots.
- Checkin status and schedule read models:
  `buildCheckinStatus`, `checkinTodaySummary`, `checkinScheduleStatus`,
  `computeCheckinScheduleStatus`.
- Thin wrappers to `CheckinExecutor`, `BalanceRefresher`,
  `CheckinBatchOrchestrator`, `AccountSessionService`, and `AccountAPIClient`.
- Response parsing helpers:
  checkin response classification, reward extraction, balance parsing,
  message extraction, masking, and numeric conversion.

Recommended next backend slice:

1. Extract `CheckinStatusService` for status/today/schedule read models.
2. Move balance/checkin response parsers into focused parser files with unit
   tests, while keeping the types in `core`.
3. Keep `runDueCheckins` wrappers until scheduler and task tests no longer need
   the compatibility method names.

## `scheduler.go` Audit

Completed in this slice:

- Added `scheduler_jobs.go`.
- `tickSchedulers` now iterates `schedulerTickRegistry()`.
- `buildSchedulerStatus` now uses `schedulerStatusRegistry()`.
- Added test coverage for visible job labels, tick order, and status order.
- ADR-005 records why this is a lightweight registry rather than a full job
  interface.

Remaining responsibilities:

- Job-specific planning/execution for checkin, sync, and channel health.
- Per-site channel schedule ticking.
- Scheduler run begin/finish lifecycle persistence.
- Config loading and normalisation.

Recommended next backend slice:

1. Extract scheduler run lifecycle persistence from `beginSchedulerJob` and
   `finishSchedulerJob` into `SchedulerRepo` or a small `SchedulerRunService`.
2. Keep job-specific planners in `scheduler.go` until another scheduled job is
   added.
3. Revisit a full `SchedulerJob` interface only when at least one new scheduled
   job needs to share plan/run/result behavior.

## Validation

- `rtk go test -mod=vendor -count=1 ./internal/core -run "TestBuildSchedulerStatusIncludesKnownJobs|TestSchedulerRegistriesKeepTickAndStatusOrder|TestChannelHealthScheduler|TestSyncScheduler|TestTickChannelScheduler" -v`: 12 passed.
- `rtk go test -mod=vendor -count=1 -coverprofile='core-cover.out' ./internal/core`: passed, `internal/core` coverage 53.7% of statements.
- Temporary `core-cover.out` was removed after recording the coverage number.

## Requirement Mapping

- T301 `accounts.go` audit: completed in this document.
- T302 scheduler job registry ADR: completed by ADR-005.
- T303 core coverage 50%+: completed; measured at 53.7%.
