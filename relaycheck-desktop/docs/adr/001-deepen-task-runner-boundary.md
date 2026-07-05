# ADR-001: Deepen TaskRunner into a lifecycle module

Status: Accepted
Date: 2026-07-04
Deciders: RelayCheck Desktop maintainers

## Context

`internal/core/task_runner.go` currently owns two different interfaces: the generic task lifecycle interface (`start`, `cancel`, SSE stream, cleanup) and concrete task implementation for site detection and channel health probe. Recent slices already moved checkin, balance refresh, and API key testing into `CheckinTaskService` and `AccountTaskService`, leaving `detect_sites` and `channel_health_probe` as the remaining task bodies inside `TaskRunner`.

This makes the module shallow: callers see a task lifecycle interface, but maintainers must also understand DB queries, site detection, model sync, cache invalidation, and health result summarization in the same file.

## Decision

Create a `SiteTaskService` inside `package core` and move `detect_sites` plus `channel_health_probe` task bodies behind that service. Keep `TaskRunner` focused on lifecycle, cancellation, subscriber caps, heartbeat, cleanup, and HTTP/SSE wiring.

## Considered Alternatives

- Keep current layout: rejected because the task lifecycle interface and site probe implementation stay coupled in one shallow module.
- Create `internal/tasks` package now: rejected because task bodies still need core-only types and helpers, causing avoidable interface churn.
- Move only `detect_sites`: rejected as incomplete; `channel_health_probe` is the same task-body family and already has coverage.

## Consequences

- Positive: `task_runner.go` becomes a deeper module with smaller interface and stronger locality.
- Positive: site task orchestration gains a focused test surface next to `AccountTaskService` and `CheckinTaskService`.
- Negative: `App` gains one more service field.
- Risk: channel health probe behavior could change while moving code; mitigate with existing integration test and focused detect-sites task test.

## Revisit triggers

- If more task families appear, consider a task registry interface instead of another `switch`.
- If site task code no longer needs core-only types, reconsider moving it to `internal/sites`.
- If frontend task payloads need versioning, revisit the SSE payload contract separately.
