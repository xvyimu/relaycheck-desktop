# RelayCheck Desktop — Session Close Archive (2026-07-18)

## Snapshot

| Field | Value |
|---|---|
| Path | `E:\zidqiandao\relaycheck-desktop` |
| Branch | `main` = `origin/main` (after push of this batch) |
| Pre-batch HEAD | `1eb6c28` |
| Gates | frontend **65 files / 395 tests**, `tsc` clean |

## Delivered this mega-session (already on remote before final micro-patch)

1. Security: checkin + cleanup `previewId` closed loops (`f5c10de`)
2. Typed API panel convergence (`b98218e` … sites/analytics `1eb6c28`)
3. Docs/SOP/QA/security reports (`5f5c556`)
4. Canary ignore (`a8f372d`)
5. Housekeep: dist/coverage deleted, HANDOFF rewrite, planning files (`c81f423`–`ddbe497`)
6. `.planning` → `docs/archives/planning-history-2026-07-18.tar.gz` (`126101a`–`177488f`)
7. Local deploy playbook + local perf sampler (not production RUM)

## Micro-patch in this close-out

- `accountApi.page/postAction/remove/postBulk` request helpers
- `useAccountsPage` uses `accountApi.page`
- `systemApi.health/status` + `useSystemOverview` health via systemApi
- Tests extended; full suite 395 green

## Residual issues (documented, not fake-closed)

| Item | Status | Why blocked / deferred |
|---|---|---|
| AccountCard/Form/Insights still call `api()` with `accountApi` URLs | Accept residual | URL ownership is centralized; full call-site rewrite is optional next slice |
| Production RUM / multi-host p95 | External | Needs installed host + real data volume |
| Authenticode signing | External | No cert materials |
| DB migration / site-delete semantics | Confirm required | Data risk |
| `useApi("/api/system/status")` path string | Accept | Shared hook; status path also on systemApi |

## Hygiene

- `dist/`, `frontend/dist/`, `frontend/coverage/`: gone (regenerable)
- `data/`: preserved
- `.planning/`: archived + gitignored

## Read order next session

1. `HANDOFF.md`
2. `docs/housekeep/project-cleanup-plan-2026-07-18.md`
3. `docs/sop/*`
4. This archive: `docs/archives/session-close-2026-07-18.md`

## Authorization batch (RUM / signing / migration / site-delete)

| Item | Outcome |
|---|---|
| Site-delete semantics | **Code closed:** transactional cascade delete (accounts/logs/balances/schedules/pricing) + tests + `sitesApi.remove` |
| Schema FK migration | **Deferred** (documented; no rewrite of historical tables; no real DB touch) |
| Code signing | **Scaffold only:** `scripts/sign-release.ps1` + readiness doc; **blocked without PFX** |
| Production RUM/p95 | **Plan only:** `docs/perf/production-rum-collection-plan-2026-07-18.md`; local sampler remains non-RUM |

No `data/relaycheck.db` contents modified. No force-push. Signing secrets never written to repo.
