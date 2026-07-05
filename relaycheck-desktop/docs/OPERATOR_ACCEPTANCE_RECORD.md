# Operator Acceptance Record

Date:
Operator:
Target machine:
Working directory:

Do not record real passwords, cookies, bearer tokens, API keys, exported `.rczip` passwords, or private database contents in this file.

## Release Package

| Field | Value |
| --- | --- |
| Release commit | |
| Package zip path | |
| Package SHA256 | |
| Manifest `version` | |
| Manifest `gitCommit` | |
| Manifest `gitDirty` | |
| Install type | Fresh install / Upgrade |
| Previous version or package | |

## Pre-Launch Approval

| Check | Result | Notes |
| --- | --- | --- |
| Release gate passed | Pending / Pass / Fail | |
| Package SHA256 matched sidecar | Pending / Pass / Fail | |
| Operator reviewed runbook | Pending / Pass / Fail | |
| Target working directory writable | Pending / Pass / Fail | |
| Bootstrap password source chosen | Env var / Generated local file / Existing admin | Do not paste the password |
| Existing database backed up | N/A / Pending / Pass / Fail | Path only, no contents |
| Previous executable backed up | N/A / Pending / Pass / Fail | |
| Rollback path confirmed | Pending / Pass / Fail | |

## Launch Verification

| Check | Result | Evidence / Notes |
| --- | --- | --- |
| `relaycheck.exe` started from intended working directory | Pending / Pass / Fail | |
| `/api/health` returned HTTP 200 and `data.status=ok` | Pending / Pass / Fail | |
| Settings shows expected port | Pending / Pass / Fail | |
| Settings shows expected database path | Pending / Pass / Fail | |
| Settings shows expected backup directory | Pending / Pass / Fail | |
| `scripts\operator-acceptance.ps1` passed | Pending / Pass / Fail | |
| Accepted warnings recorded below | N/A / Yes / No | |

## Manual Critical Flow

Use non-secret test data only.

| Flow | Result | Notes |
| --- | --- | --- |
| Admin sign-in works | Pending / Pass / Fail | |
| Dashboard renders summary, Action Center, scheduler preview, and notifications | Pending / Pass / Fail | |
| Settings runtime, diagnostics, database, and backup information render | Pending / Pass / Fail | |
| Manual backup can be created and found under `data\backups\` | Pending / Pass / Fail | |
| Sites or Channels can create or inspect one non-secret relay site | Pending / Pass / Fail | |
| Dry-run task completes before any real batch action | Pending / Pass / Fail | |
| Browser login URL opens without redirect loop, if tested | N/A / Pass / Fail | Use a disposable account |

## First-Hour Monitoring

| Time | Health | UI / API status | Logs / warnings | Operator initials |
| --- | --- | --- | --- | --- |
| 0 min | | | | |
| 5 min | | | | |
| 15 min | | | | |
| 30 min | | | | |
| 60 min | | | | |

## Accepted Warnings

| Warning | Owner | Follow-up | Due date |
| --- | --- | --- | --- |
| | | | |

## Rollback Readiness

| Check | Result | Notes |
| --- | --- | --- |
| Previous executable available | N/A / Pass / Fail | |
| Previous database backup available | N/A / Pass / Fail | |
| Rollback health check command known | Pass / Fail | |
| Rollback trigger owner assigned | Pass / Fail | |

## Final Decision

Decision: Approved for operation / Hold / Roll back

Operator signature:
Decision time:
Follow-up owner:

