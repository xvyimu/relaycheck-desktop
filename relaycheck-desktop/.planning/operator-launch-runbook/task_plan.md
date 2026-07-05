# Task Plan: Operator Launch Runbook

## Goal

Turn the latest local release-gate evidence into operator-ready launch material: a runbook, a read-only local acceptance script, and first-hour monitoring guidance.

This slice should not change product behavior or touch runtime data. The acceptance script must only inspect local API endpoints and filesystem paths returned by the app.

## Current Phase

- Phase 1: Planning - complete
- Phase 2: Documentation and script update - complete
- Phase 3: Verification - complete
- Phase 4: Commit - complete

## Task List

- [x] OBSERVE: Review launch readiness docs, release verification script, manual test notes, health/status/port API shapes, and repository state.
- [x] DOCS: Add an operator runbook with first-launch, existing-DB, port-conflict, manual critical-flow, first-hour monitoring, and rollback steps.
- [x] SCRIPT: Add a read-only operator acceptance script for local health/status/API-shape checks.
- [x] DOCS: Link the runbook and script from launch readiness and README.
- [x] CHECK: Validate script syntax and run it against a temporary local desktop process.
- [x] REVIEW: Check docs for launch-safety accuracy and no secret-handling drift.
- [x] COMMIT: Commit and push the runbook slice.

## Verification Target

- `rtk powershell -NoProfile -ExecutionPolicy Bypass -File scripts\operator-acceptance.ps1 -BaseUrl http://127.0.0.1:3001`
- `rtk git diff --check`
