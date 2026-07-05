# Operator Runbook

Date: 2026-07-05
Scope: RelayCheck Desktop local Windows launch from a `scripts\package-release.ps1` zip package.

Use this runbook after the one-command release gate in `docs\LAUNCH_READINESS.md` has passed. It is written for a trusted local operator and does not replace the release gate.

## Required Inputs

- Release package: `dist\releases\relaycheck-desktop-<version>-<commit>-<timestamp>.zip`
- Release executable inside the package: `relaycheck.exe`
- Intended working directory: the folder that should own `data\relaycheck.db`
- Local URL: `http://127.0.0.1:3001` unless `RELAYCHECK_PORT` is changed
- Bootstrap account: `admin`
- Bootstrap password source: `RELAYCHECK_BOOTSTRAP_PASSWORD` on first launch, or `data\bootstrap-admin-password.txt` if the env var is not set
- Existing installation backup: a copy of the previous executable and `data\relaycheck.db`

Do not paste real passwords, cookies, bearer tokens, API keys, or exported `.rczip` passwords into tickets, screenshots, logs, or handoff notes.

## Pre-Launch Gate

Run from the repository root:

```powershell
powershell -ExecutionPolicy Bypass -File scripts\verify-release.ps1 -ProxyUrl http://127.0.0.1:7897
```

Omit `-ProxyUrl` when direct module access works. Continue only if the script passes.

Then create the handoff package from a clean Git tree:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\package-release.ps1
```

Before copying the package to the target machine, compare the zip SHA256 printed by the script and written to the sibling `.zip.sha256` file with the handoff record.

## First Launch

1. Confirm the working directory is writable and has enough disk space for `data\`, `data\backups\`, logs, and exports.
2. If this is a fresh install and a deterministic password is required, set `RELAYCHECK_BOOTSTRAP_PASSWORD` before starting the app.
3. If this is an upgrade, stop the previous `relaycheck.exe`, copy the previous executable aside, and back up `data\relaycheck.db`.
4. Extract the release package into the intended working directory.
5. Confirm `manifest.json` and `checksums.sha256` are present beside `relaycheck.exe`.
6. Start `relaycheck.exe` from the intended working directory.
7. Open `http://127.0.0.1:3001`.
8. Run the read-only acceptance script from the extracted package:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\operator-acceptance.ps1 -BaseUrl http://127.0.0.1:3001
```

If the app is intentionally running on another port, pass `-BaseUrl` and `-ExpectedPort` with that port. If a port fallback is expected and accepted, add `-AllowPortConflict`.

For an isolated fresh-runtime smoke that starts and stops the release executable automatically, use a non-default port:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\operator-acceptance.ps1 -StartReleaseExe -ExpectedPort 3101
```

## Manual Critical Flow

Use non-secret test data only.

1. Sign in with `admin` and the bootstrap password.
2. Open Dashboard and confirm summary cards, Action Center, scheduler preview, and notifications render.
3. Open Settings and confirm runtime port, database path, backup dir, scheduler status, and diagnostics are visible.
4. Create a manual backup from Settings and confirm the new file appears under `data\backups\`.
5. Open Sites or Channels and create or inspect one relay site with non-secret test values.
6. Run a dry-run task before any real batch action.
7. If testing browser login, use a disposable account and verify the detected login URL opens without redirect loops.
8. Do not run real check-ins, balance refreshes, imports, restores, or encrypted exports during acceptance unless the operator has approved that data change.

## Port Conflict Check

Expected behavior:

- If `127.0.0.1:3001` is free, the app should bind to port `3001`.
- If the preferred port is occupied and fallback is enabled by the launcher, Settings should show `portConflict=true`, the preferred port, and the actual bound port.
- If the operator requires port `3001`, any fallback is a launch blocker.

When a conflict is unexpected, stop the extra process, restart RelayCheck Desktop, and rerun `scripts\operator-acceptance.ps1`.

## First-Hour Monitoring

Record checks at launch, 5 minutes, 15 minutes, 30 minutes, and 60 minutes.

| Time | Required check | Pass condition |
| --- | --- | --- |
| 0 min | `/api/health` | HTTP 200 and `data.status` is `ok` |
| 0 min | Settings status | Port, database path, and backup dir match the intended working directory |
| 5 min | UI navigation | Dashboard, Channels, Sites, Accounts, Checkins, Notifications, and Settings render without blank screens |
| 15 min | Action Center | No new critical action item |
| 30 min | Scheduler preview | Next runs and calendar preview load |
| 60 min | Logs and notifications | No repeated startup, scheduler, database, or notification failures |

Use `degraded` health as a hold condition unless the warning is already understood and documented. Use `down` health as a rollback trigger.

## Rollback Triggers

Roll back immediately if any of these happen:

- `/api/health` returns `down`, fails to respond, or reports a database/key-path error.
- UI cannot render the main dashboard after refresh.
- SQLite migration or database reopen fails.
- Expected port binding fails and the operator requires that port.
- Manual critical flow fails with a reproducible product issue.
- Any real data mutation produces unexpected data loss or credential exposure.

## Rollback Steps

1. Stop `relaycheck.exe`.
2. Restore the previous executable.
3. Restore the previous `data\relaycheck.db` if the failed run changed production data.
4. Start the previous executable from the same working directory.
5. Verify `http://127.0.0.1:3001/api/health` returns HTTP 200 and `data.status` is `ok`.
6. Keep the failed build, logs, `data\backups\` snapshot, and exact failing step for diagnosis.

## Acceptance Record

Use `docs\OPERATOR_ACCEPTANCE_RECORD.md` from the extracted package as the operator record template. Keep the completed copy with the release note or handoff, and do not paste secrets into it.

Record at least the following:

- Release commit, package path, and zip SHA256
- Working directory
- Whether this was a fresh install or upgrade
- `scripts\verify-release.ps1` result
- `scripts\operator-acceptance.ps1` result
- Manual critical-flow result
- First-hour monitoring result
- Any accepted warnings, with owner and follow-up
