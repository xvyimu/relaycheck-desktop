# ADR-004: Remove the global schedule ghost site

Status: Accepted
Date: 2026-07-05
Deciders: RelayCheck Desktop maintainers

## Context

ADR-002 consolidated schedule projection but deliberately left the global
checkin schedule represented as `upstream_sites.id='__global__'`. That kept
the slice low risk, but it also forced site listing, channel health, site task
selection, and tests to defend against a schedule-only row leaking into real
site workflows.

`channel_schedules` already has its own primary key. The global schedule can
be identified by `channel_schedules.id='__global__'` without pretending it is
an upstream site.

## Decision

Make `channel_schedules.upstream_site_id` nullable. Store the global checkin
schedule as `id='__global__', upstream_site_id=NULL`, while preserving
`upstreamSiteId="__global__"` in API responses as a compatibility identifier.

Add a migration that rebuilds the SQLite table when it still has the old
`NOT NULL` column, maps legacy `__global__` schedule rows to `NULL`, recreates
the schedule indexes, and removes the legacy `upstream_sites.__global__` row.

## Considered Alternatives

- Keep the virtual upstream site and keep filtering it out: rejected because it
  spreads schedule knowledge into unrelated site workflows.
- Move the global schedule only to `scheduler_runs`: rejected because calendar
  preview and per-schedule configuration still need a schedule row shape.
- Create a separate `global_schedules` table: rejected as unnecessary while
  there is exactly one global checkin schedule and one established schedule
  projection path.

## Consequences

- Positive: real upstream-site queries no longer need a generated row to
  satisfy global schedule persistence.
- Positive: old databases keep their schedule settings after migration.
- Positive: frontend/API compatibility is preserved for existing global
  schedule display logic.
- Negative: code that consumes schedule rows must understand that
  `upstream_site_id` can be NULL in storage.
- Risk: SQLite table rebuilds are more sensitive than additive migrations;
  mitigate with migration tests against a legacy table and full Go regression.

## Revisit triggers

- When adding multiple global schedule types.
- When introducing versioned database migrations.
- When merging scheduler status, calendar, and next-run responses into one API.

