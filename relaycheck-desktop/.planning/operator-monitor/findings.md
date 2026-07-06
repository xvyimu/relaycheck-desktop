# Findings: Operator Monitor

## Source Findings

- scripts/operator-launch.ps1 already starts a package-local relaycheck.exe, runs package verification, runs operator acceptance, and can leave the app running for follow-up monitoring.
- scripts/operator-acceptance.ps1 already proves the read-only health, system status, channel, next-runs, and calendar endpoints can be checked without storing secrets.
- docs/OPERATOR_RUNBOOK.md requires first-hour checks at 0, 5, 15, 30, and 60 minutes, but those checks were manual and did not yet produce a structured monitor record.

## Implementation Findings

- scripts/operator-monitor.ps1 defaults to 0/5/15/30/60 minute samples and supports quick smoke checks with -SampleCount 3 -IntervalSeconds 1 or explicit interval arrays.
- The monitor samples /api/health, /api/system/status, /api/scheduler/next-runs, /api/scheduler/calendar?days=2, and /api/system/action-center.
- The monitor writes Markdown and JSON records under launch-records/ and stores only status/count/failure summaries, not raw secrets or exported data.
- The monitor fails on health down, health degraded unless explicitly allowed, unexpected port binding, unaccepted port conflict, missing API shape, critical diagnostics, or critical Action Center items unless explicitly allowed.

## Verification Findings

- PowerShell parser checks passed for scripts/operator-monitor.ps1, scripts/package-release.ps1, and scripts/verify-package.ps1.
- Whitespace check passed with rtk git diff --check.
- Development package verification passed from both source tree and package root with manifest.gitDirty=true explicitly allowed.
- Package-local launch smoke passed on port 3218, then package-local monitor smoke sampled +0s/+1s/+2s and passed. Diagnostics reported warning status in the fresh runtime, which is recorded as a warning rather than a monitor failure.
- No relaycheck process remained after smoke cleanup.
- Clean package relaycheck-desktop-1.1.0-a1877b682202-20260706-025213.zip verified with manifest.gitDirty=false.
- Final package-local operator-launch and operator-monitor smoke passed on port 3219; only expected fresh-runtime diagnostics warnings were recorded.
