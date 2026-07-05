# Task Plan: Final Launch Audit

## Goal

Run the final launch-readiness audit on current `main`: review the release scripts/docs for blockers, run the full release gate, create a clean final package, and refresh the launch evidence.

This slice should not change product behavior. Any generated artifacts stay under ignored `dist/` or `frontend/dist/`.

## Current Phase

- Phase 1: Planning - complete
- Phase 2: Review - complete
- Phase 3: Full verification - complete
- Phase 4: Evidence refresh - in_progress
- Phase 5: Commit - pending

## Task List

- [x] OBSERVE: Confirm clean `main...origin/main`, latest launch/package commits, and applicable review/launch skills.
- [x] REVIEW: Inspect release scripts, operator runbook, launch readiness docs, package manifest logic, and secret/runtime-data boundaries.
- [x] CHECK: Run the full release gate with the 7897 proxy.
- [ ] CHECK: Create a clean release package after the full gate.
- [ ] CHECK: Verify final package hash, manifest, package contents, and package-local acceptance.
- [ ] DOCS: Refresh launch readiness and final audit evidence.
- [ ] REVIEW: Confirm no release-blocking findings remain.
- [ ] COMMIT: Commit and push final launch audit evidence.

## Verification Target

- `rtk powershell -NoProfile -ExecutionPolicy Bypass -File scripts\verify-release.ps1 -ProxyUrl http://127.0.0.1:7897`
- `rtk powershell -NoProfile -ExecutionPolicy Bypass -File scripts\package-release.ps1`
- Package-local `scripts\operator-acceptance.ps1 -StartReleaseExe -ExpectedPort 3102 -TimeoutSeconds 30`
