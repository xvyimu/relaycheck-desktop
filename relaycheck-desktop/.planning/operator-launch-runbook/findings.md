# Findings: Operator Launch Runbook

## Source Findings

- `scripts\verify-release.ps1` is the full local release gate and already starts a temporary runtime for binary health and browser smoke.
- `docs\LAUNCH_READINESS.md` records release-gate evidence, basic operator checklist, and rollback plan, but it was still short on first-hour monitoring detail.
- `/api/health` returns wrapped JSON with `data.status` values `ok`, `degraded`, or `down`, plus per-check statuses.
- `/api/system/status` returns wrapped JSON with product identity, port, preferred port, port conflict flag, database path, backup dir, scheduler status, diagnostics, and dashboard summary.
- `/api/channels`, `/api/scheduler/next-runs`, and `/api/scheduler/calendar?days=2` are useful read-only API shape checks after startup.

## Decisions

- Keep the new acceptance script read-only: no backup creation, restore, login mutation, or task execution.
- Put destructive or operator-specific actions in the manual runbook instead of automating them.
- Treat port conflict as a launch blocker unless the operator explicitly allows it with a script switch.
