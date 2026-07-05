# ADR-002: Consolidate schedule plan projection

Status: Accepted
Date: 2026-07-04
Deciders: RelayCheck Desktop maintainers

## Context

Scheduling data is projected through `internal/core/channel_schedules.go`, `internal/channels/schedules.go`, and `internal/core/scheduler.go`. Calendar preview, next-run lists, global checkin projection, and per-site schedule ticks all depend on the same conceptual module but are currently split across HTTP handlers, domain service methods, and scheduler ticks.

The `__global__` schedule is also represented as a virtual upstream site to satisfy persistence constraints, which gives UI projection leverage but creates ghost data in `upstream_sites`.

## Decision

Do not change the schema in this slice. Introduce a deeper schedule projection module that owns calendar items and next-run items. Treat removing `__global__`, global checkin projection, and per-site due checks as later decisions.

## Considered Alternatives

- Make `channel_schedules.upstream_site_id` nullable immediately: rejected for this slice because it is a schema migration and requires careful compatibility checks.
- Keep projection logic scattered: rejected long term because schedule bugs require jumping between multiple modules.
- Move all scheduler code to `internal/channels`: rejected because sync and channel-health scheduler jobs are not channel schedule domain logic.

## Consequences

- Positive: future schedule changes get one interface for plan projection.
- Positive: calendar and next-runs can share one source of truth.
- Negative: the `__global__` record remains until a migration is explicitly planned.
- Risk: planner abstraction can become shallow if it only forwards existing calls; the follow-up must move real behavior, not just names.

## Revisit triggers

- When adding another scheduled job type.
- When changing `channel_schedules` schema.
- When calendar and next-runs need merged API responses.
