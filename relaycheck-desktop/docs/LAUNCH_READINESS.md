# Launch Readiness

Date: 2026-07-05
Status: Pre-launch verified locally with the release gate; production rollout still requires operator approval.

Latest local release-gate run:

```powershell
powershell -ExecutionPolicy Bypass -File scripts\verify-release.ps1 -ProxyUrl http://127.0.0.1:7897
```

## Verified Gates

| Gate | Result |
| --- | --- |
| Frontend tests | `npm test` via release gate - 14 files / 216 tests passed |
| Go tests | `go test -mod=vendor -count=1 ./...` via release gate - passed across 12 packages |
| Go vet | `go vet -mod=vendor ./...` via release gate - no issues |
| Frontend build | `npm run build` via release gate - TypeScript + Vite build passed |
| Windows binary build | `go build -mod=vendor -ldflags="-H windowsgui" -o dist\relaycheck.exe .` via release gate - passed |
| npm vulnerability scan | `npm audit --audit-level=low` via release gate - 0 vulnerabilities |
| Go vulnerability scan | `govulncheck` via release gate - current code affected by 0 vulnerabilities; one imported vulnerable package was not called |
| Scheduler layout smoke | `cd frontend; rtk npm run smoke:schedules` - 1440x900 and 390x900 passed |
| Navigation smoke | `cd frontend; rtk npm run smoke` - 9 PASS / 0 FAIL |
| Binary health smoke | Temporary `dist\relaycheck.exe` runtime returned `/api/health` 200 |
| Fresh DB API shape | `/api/channels` returned `ok=true` and non-null `data` |
| One-command release gate | `powershell -ExecutionPolicy Bypass -File scripts\verify-release.ps1 -ProxyUrl http://127.0.0.1:7897` - passed |
| Whitespace | `rtk git diff --check` - passed |

## One-Command Gate

Run from the repository root:

```powershell
powershell -ExecutionPolicy Bypass -File scripts\verify-release.ps1 -ProxyUrl http://127.0.0.1:7897
```

Omit `-ProxyUrl` when direct access to the Go module proxy works. The script starts temporary local processes for binary health and browser smoke, then stops them and removes generated smoke files. After the latest run, `.tmp`, `frontend\verify-canary.txt`, and `frontend\verify-nav-output.txt` were absent; `dist\relaycheck.exe` and `frontend\dist` remained as generated release/build artifacts.

## Launch Notes

- The application is a local desktop service bound to `127.0.0.1`.
- The release artifact is `dist\relaycheck.exe`, built from embedded `frontend/dist`.
- Startup creates a local SQLite database and key material under the process working directory's `data\` folder.
- On a fresh database, the bootstrap admin password is read from `RELAYCHECK_BOOTSTRAP_PASSWORD` or generated into `data\bootstrap-admin-password.txt`.
- Browser smoke now uses deterministic API fixtures for navigation intent coverage, so it does not depend on the operator's local database contents.
- Re-run the one-command gate after any production-code change. Documentation-only updates do not change the embedded binary, but should still be reviewed for operator accuracy.

## Operator Checklist

- Confirm target machine has a writable application working directory.
- Set `RELAYCHECK_BOOTSTRAP_PASSWORD` for first production launch if a deterministic initial password is required.
- Back up existing `data\relaycheck.db` before replacing an already-running installation.
- Start `dist\relaycheck.exe` from the intended working directory.
- Open `http://127.0.0.1:3001` and verify `/api/health` reports all checks as `ok`.
- Run one manual critical flow with non-secret test data: open dashboard, inspect scheduler preview, create or view a site, and trigger a dry-run task.

## Rollback Plan

Trigger rollback if startup health fails, UI cannot render, SQLite migration fails, or smoke/manual critical flow fails.

1. Stop the running `relaycheck.exe` process.
2. Restore the previous executable.
3. Restore the previous `data\relaycheck.db` backup if the new version touched production data.
4. Start the previous executable and verify `/api/health`.
5. Keep the failed build's logs and `data\backups` snapshot for diagnosis.

## Known Follow-Ups

- Add signed artifact packaging if this is distributed beyond a trusted local operator.
- Decide whether to store launch smoke fixtures in shared test utilities once more browser smoke suites are added.
- Add an explicit first-hour monitoring checklist if RelayCheck Desktop is handed to an operator who will run it outside this development workstation.
