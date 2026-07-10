# ADR-005: Use a lightweight scheduler job registry

Status: Accepted
Date: 2026-07-06
Deciders: RelayCheck Desktop maintainers

## Context

`internal/core/scheduler.go` owns several related concerns: the scheduler loop,
per-site channel schedules, global checkin planning, local NewAPI sync, channel
health probes, scheduler run persistence, and scheduler status projection.

ADR-002 and ADR-004 already moved schedule projection and global schedule
persistence in the right direction. The remaining scheduler risk is extension
cost: adding a new scheduled job requires changing tick order, status labels,
and status projection in separate places. A full job interface would be heavier
than the current code needs, because each job still has different planning and
execution rules.

## Decision

Introduce a lightweight in-process scheduler registry in `internal/core`.

The registry lists scheduler tick units in one ordered slice:

- automatic checkin
- per-site channel schedules
- local NewAPI sync
- channel health probe

The same registry exposes a status-visible subset for `/api/system/status` and
next-run projection. Per-site channel schedules remain ticked by the scheduler
loop but are not shown as one global scheduler job, because their user-facing
next-runs are projected from `channel_schedules`.

Do not introduce a generic job interface yet. Keep job-specific planning and
execution in the existing functions until another scheduled job appears or the
current functions need separate unit seams.

## Considered Alternatives

- Keep hard-coded tick calls plus a separate status list: rejected because the
  order and labels drift easily.
- Introduce a full `SchedulerJob` interface now: rejected as premature. The
  current jobs have different config formats, run-key semantics, and result
  summaries; a generic interface would mostly hide useful differences.
- Move all scheduler code into a new package: rejected for this slice because
  the jobs still depend on `App` services, DB-backed settings, notifications,
  and schedule projection.

## Consequences

- Positive: tick order and status metadata now have one local source of truth.
- Positive: a future job can be added by extending the registry first, then
  adding the job-specific planner/executor.
- Positive: per-site channel schedules remain explicit as a tick unit without
  pretending to be a single `scheduler_runs` job.
- Negative: `scheduler.go` still owns begin/finish lifecycle persistence.
- Risk: the registry can become shallow if every future job grows special cases;
  revisit before adding a second non-status tick unit.

## Revisit Triggers

- When adding another scheduled job type.
- When extracting scheduler run lifecycle into a service/repository.
- When merging scheduler status, calendar, and next-run APIs.
- When per-site schedules need their own aggregated status row.
