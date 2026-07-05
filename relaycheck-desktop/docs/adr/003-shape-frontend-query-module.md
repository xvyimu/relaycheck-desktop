# ADR-003: Shape a frontend query module

Status: Accepted
Date: 2026-07-04
Deciders: RelayCheck Desktop maintainers

## Context

The frontend has a useful `useApi<T>` hook, but `useAppData` still coordinates many independent API calls and owns a large amount of loading/error state. Dashboard subviews also make scheduler-specific requests through `useNextRuns` and `HubRadar` calendar loading. This gives the dashboard leverage, but the query interface is still wide and tied to screen composition.

## Decision

Shape a frontend query module around stable resource groups, starting with scheduler preview data. Keep the current `useApi<T>` primitive and add a scheduler preview hook/module for calendar and next-runs data. Do not introduce a new query dependency.

## Considered Alternatives

- Introduce React Query/SWR: rejected for now because the existing stack already has a small `useApi<T>` and dependency discipline favors no new package.
- Leave all loading at screen level: rejected long term because UI panels must understand too many endpoint details.
- Merge all dashboard endpoints on the backend: rejected for now because it would couple unrelated resource freshness policies.

## Consequences

- Positive: frontend panels receive smaller props and fewer endpoint details.
- Positive: loading/error handling gains locality.
- Negative: query module must avoid becoming a generic pass-through.
- Risk: over-normalizing data can make local UI changes harder; keep resource groups domain-shaped.

## Revisit triggers

- When adding another dashboard panel with independent API loading.
- When task progress or scheduler preview requires shared refresh semantics.
- When frontend tests begin covering hook-level loading and abort behavior.
