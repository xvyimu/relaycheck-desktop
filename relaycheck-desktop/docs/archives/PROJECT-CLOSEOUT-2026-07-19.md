# RelayCheck Desktop — PROJECT CLOSEOUT (2026-07-19)

**Status: ENDED（暂时结束）.** Code track fully closed. Reopen only when external materials arrive or Phase C is authorized.

## Final baseline

| Field | Value |
|---|---|
| Path | `E:\zidqiandao\relaycheck-desktop` |
| HEAD | **`0d400d4`** = `main` = `origin/main` (last feature **`42f21c8`**) |
| Remote | https://github.com/xvyimu/relaycheck-desktop |
| Data | `data/` preserved; backups `pre-orphan-cleanup-20260718-195632/`, `pre-fk-phase-ab-*/` |
| Local DB | FK Phase A+B migrated (`schema.fk_phase="2"`); `foreign_key_check` empty; logs 487 / accounts 25 |

## Everything shipped this closing arc (fd5a87a → 42f21c8)

| Commit | Delivery |
|---|---|
| `fd5a87a` | Site delete UI (card/drawer/master-detail confirm) + read-cache invalidate + orphan precheck/dry-run tooling |
| `4b51187` | Local representative API p95 (N=50, 5 endpoints) |
| `9d19352` | Local cleanup: 18 orphan `checkin_logs` (backed up first) |
| `3cadcaf` | Authenticode hard-block after exhaustive env/disk/cert-store search |
| `5ce688d` | Evening close archive |
| `e3e50e9` | accountApi residual migrate + cold-start sampler (`scripts/sample-cold-start.ps1`) |
| `2d824f3` | SQLite FK migration design (phases A/B/C, gates, risks) |
| `d702383` | **FK Phase A+B implemented**: rebuild 4 tables with ON DELETE CASCADE, orphan scrub, account cascade delete, `db_fk_test.go` |
| `9f7ba4e` | Handoff/archive sync for FK |
| `42f21c8` | `systemApi.statusPath` + `dashboardApi.*Path` owners; **UI first-interactive mark** (`lib/firstInteractive.ts`, local-only) |

## Gate snapshot at close

- Frontend: **69 files / 405 tests** green; `tsc` clean; vite build OK
- Go: `./internal/core` full green (incl. FK suite); `./internal/sites` green; `go build ./...` OK
- Cold start (rebuilt exe): spawn 631ms → firstHealth 1232ms
- Local perf samples (gitignored): `docs/perf/samples/local-api-p95-*.json`, `local-cold-start-*.json`

## Reopen triggers（三把钥匙）

| Key | Holder action | Then |
|---|---|---|
| Code Signing PFX + password | Set `RELAYCHECK_SIGN_PFX` / `_PASSWORD` (outside repo/chat) | `scripts/sign-release.ps1` → `Get-AuthenticodeSignature` Valid |
| Representative host(s) / larger DB | Provide machine + window | Follow `docs/perf/production-rum-collection-plan-2026-07-18.md`; UI mark readable via console `[perf]` / localStorage |
| “批准 Phase C” | Explicit authorization | FK plan §4.3 (channel/instance SET NULL) |

## Standing rules for any future agent

- Never delete `data/`; backup `relaycheck.db*` before first launch of an upgraded binary (auto FK migrate runs once).
- No self-signed “production” certs; no Phase C / destructive migration without explicit confirm.
- Do not redo: cascade deletes, FK A+B, orphan cleanups, typed API owners, perf instrumentation — all on main.

## Read order on reopen

1. `HANDOFF.md`
2. This file
3. `docs/sop/relaycheck-sqlite-fk-migration-plan-2026-07-18.md`
4. `docs/perf/production-rum-collection-plan-2026-07-18.md`
5. `docs/deploy/code-signing-readiness-2026-07-18.md`
