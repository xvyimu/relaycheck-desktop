# Operator Acceptance Record Draft

Date: 2026-07-06
Operator: Pending human confirmation
Target machine: Pending human confirmation
Working directory: Pending human confirmation

Do not record real passwords, cookies, bearer tokens, API keys, exported `.rczip`
passwords, or private database contents in this file.

## Status

Decision: Hold

Reason: engineering verification is complete, but this worktree has uncommitted
optimization changes. The latest packaged zip was built from commit
`338870bc315456b231194043abb045a15937c996` before the current optimization
diff. A final handoff package must be rebuilt from the intended final commit or
explicitly approved dirty build before production operation.

## Current Engineering Verification

| Check | Result | Evidence |
| --- | --- | --- |
| Go tests | Pass | `rtk go test -mod=vendor -count=1 ./...` - 991 passed in 12 packages |
| Go vet | Pass | `rtk go vet -mod=vendor ./...` - no issues |
| Frontend tests | Pass | `cd frontend; rtk npm test` - 17 files / 220 tests passed |
| Frontend build | Pass | `cd frontend; rtk npm run build` - passed |
| Schedule layout smoke | Pass | `cd frontend; rtk npm run smoke:schedules` - 1440x900 and 390x900 passed |
| Windows binary build | Pass | `rtk go build -mod=vendor -o dist\relaycheck.exe .` - success |
| Whitespace | Pass | `rtk git diff --check` - clean |

## Latest Historical Package

This package is useful as historical local-launch evidence, not as the final
package for the current uncommitted optimization work.

| Field | Value |
| --- | --- |
| Release commit | `338870bc315456b231194043abb045a15937c996` |
| Package zip path | Cleaned from `dist\releases`; rebuild final package before handoff |
| Package SHA256 | `67f342e52e5c7aa081cf8fd20b9ea8a0bed7dd89a747e9b22b6b481bc45cb83d` |
| Manifest `version` | `v1.1.0` |
| Manifest `gitCommit` | `338870bc315456b231194043abb045a15937c996` |
| Manifest `gitDirty` | `false` |
| Install type | Pending: Fresh install / Upgrade |
| Previous version or package | Pending human confirmation |

## Historical Local Launch Evidence

| Check | Result | Evidence / Notes |
| --- | --- | --- |
| Operator launch record exists | Historical pass | Old extracted package directory was cleaned during slimming; rerun on the final package |
| Operator monitor record exists | Historical pass | Old extracted package directory was cleaned during slimming; rerun on the final package |
| First-hour monitor result | Pass | Monitor record result `pass` |
| Monitor warning classification | Pending | Diagnostics warning was observed; operator must record whether this is an accepted setup warning |

## Pre-Launch Approval

| Check | Result | Notes |
| --- | --- | --- |
| Final release gate passed on intended final tree | Pending | Re-run after committing or explicitly approving dirty package build |
| Final package SHA256 matched sidecar | Pending | Requires rebuilt final package |
| Final package verifier passed | Pending | Requires rebuilt final package |
| Operator launch helper passed on target package | Pending | Run from extracted final package |
| Operator launch record path | Pending | |
| Operator reviewed runbook | Pending | |
| Target working directory writable | Pending | Human/operator confirmation |
| Bootstrap password source chosen | Pending | Env var / Generated local file / Existing admin; do not paste password |
| Existing database backed up | Pending | Path only, no contents |
| Previous executable backed up | Pending | |
| Rollback path confirmed | Pending | |

## Manual Critical Flow

Use non-secret test data only.

| Flow | Result | Notes |
| --- | --- | --- |
| Admin sign-in works | Pending | |
| Dashboard renders summary, Action Center, scheduler preview, and notifications | Pending | |
| Settings runtime, diagnostics, database, and backup information render | Pending | |
| Manual backup can be created and found under `data\backups\` | Pending | |
| Sites or Channels can create or inspect one non-secret relay site | Pending | |
| Dry-run task completes before any real batch action | Pending | |
| Browser login URL opens without redirect loop, if tested | Pending | Use a disposable account |

## Accepted Warnings

| Warning | Owner | Follow-up | Due date |
| --- | --- | --- | --- |
| Diagnostics overall warning from local monitor | Pending | Confirm whether this is expected empty/setup state or a real launch blocker | Pending |

## Rollback Readiness

| Check | Result | Notes |
| --- | --- | --- |
| Previous executable available | Pending | |
| Previous database backup available | Pending | |
| Rollback health check command known | Pending | `scripts\operator-acceptance.ps1 -BaseUrl http://127.0.0.1:3001` |
| Rollback trigger owner assigned | Pending | |

## Final Decision

Decision: Hold

Operator signature: Pending
Decision time: Pending
Follow-up owner: Pending
