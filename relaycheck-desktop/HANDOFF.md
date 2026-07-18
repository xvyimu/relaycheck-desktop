# HANDOFF.md

Authoritative handoff document for RelayCheck Desktop. Updated each session.
Read this first, then `CLAUDE.md` for architecture.

**Last updated:** 2026-07-18 (site delete UI + cache invalidate + orphan precheck)  
**HEAD:** see `git rev-parse --short HEAD` on `main` / `origin/main`  
**Worktree policy:** local `dist/` / `frontend/dist/` / `frontend/coverage/` may be deleted anytime (gitignored). Never delete `data/`.

---

## TODO (next session)

1. [x] Sites/analytics typed APIs + LocalNewAPISyncPanel tests + local deploy/perf docs  
2. [x] Archive `.planning/**`  
3. [x] Session close archive (`docs/archives/session-close-2026-07-18.md`) + memory  
4. [x] Site delete UI (confirm + cascade copy) + read-cache invalidate + **read-only** orphan precheck  
5. [x] checkin_logs orphan **dry-run** SQL (`scripts/sql/cleanup-checkin-log-orphans.dry-run.sql`) — mutate still blocked  

**Optional residual code (non-blocking):**

- AccountCard / AccountForm / AccountInsights / BulkRelogin still use `api()` + `accountApi` URL builders; `accountApi.page/postAction/remove` helpers exist for incremental migration.
- `useApi` path-string consumers (system status) can later take constants from `systemApi`.

**Still external / needs materials or real host:**

- Authenticode / store signing materials — scaffold: `scripts/sign-release.ps1` + `docs/deploy/code-signing-readiness-2026-07-18.md` (**blocked without PFX**)
- Production RUM + multi-host API p95 — plan: `docs/perf/production-rum-collection-plan-2026-07-18.md` (local sampler only)
- Full schema FK migration / historical orphan **cleanup** — **deferred** (precheck only: `scripts/precheck-site-orphans.ps1`); cascade delete + UI are live (`docs/sop/relaycheck-site-delete-cascade-2026-07-18.md`)

**Do not without explicit confirm:** DB migration, delete `data/*`, orphan cleanup SQL that mutates, force-push, cloud deploy, real upstream checkin blasts.
---

## Done (do not redo)

### 2026-07-18 — security closed loops + typed API (pushed)

| Commit | Summary |
|---|---|
| `f5c10de` | Checkin + unsupported-cleanup `previewId` freeze/claim; JSON fail-closed; CST day bounds; atomic checkin save; instance key ACL; public errors; incremental smoke + CI |
| `b98218e` | Typed API owners: accounts, models, keys, channels, local-newapi, notifications, system, scheduler, sites; panel wiring + behavior tests |
| `5f5c556` | SOP architecture/QA/security docs + handoff notes |
| `a8f372d` | Ignore local `frontend/verify-canary.txt` / `verify-nav-output.txt` |

**Frontend gate snapshot (typed API batch):** ~63 files / 387 tests; coverage ~66.99 / 58.80 / 59.73 / 68.18 (floors 53/45/40/54).  
**Go gate snapshot (security batch):** prior full run 13 packages / 1148 tests; re-run only if backend files change.

### Contract owners (frontend)

| Owner | Path |
|---|---|
| checkin / cleanup preview | `frontend/src/api/checkins.ts`, `account-cleanup.ts` |
| models / keys | `frontend/src/api/models.ts`, `keys.ts` |
| channels | `frontend/src/api/channels.ts` |
| local-newapi | `frontend/src/api/local-newapi.ts` |
| notifications | `frontend/src/api/notifications.ts` |
| system / scheduler / sites / analytics | `frontend/src/api/system.ts`, `scheduler.ts`, `sites.ts`, `analytics.ts` |

### Local deploy & perf (desktop meaning of deploy)

- Playbook: `docs/deploy/local-desktop-playbook-2026-07-18.md` (`verify-release` → `package-release` → `verify-package`)
- Local perf harness: `docs/perf/README.md` + `scripts/sample-local-perf.ps1` (writes gitignored JSON under `docs/perf/samples/`)
- Production RUM/p95 still requires a representative installed host; local samples **do not** prove those targets.

### 2026-07-17 and earlier

See `docs/full-stack-code-review-optimization-2026-07-17.md` for P1–P3 close-out, SSRF pin, import typed errors, coverage floors, release verifier history. Older session bullets remain valid as historical evidence in that report; they are **already on main**, not uncommitted.

---

## Read order for a new agent

1. This file  
2. `docs/housekeep/project-cleanup-plan-2026-07-18.md`  
3. `docs/sop/relaycheck-security-consistency-verification-2026-07-18.md`  
4. `docs/sop/relaycheck-incremental-architecture.md`  
5. `docs/sop/relaycheck-incremental-qa-report.md`  
6. `CLAUDE.md` + user `~\CLAUDE.md` workstyle  

Housekeep session files: `task_plan.md`, `findings.md`, `progress.md`.

---

## Local hygiene

| Path | Policy |
|---|---|
| `data/` | **Never delete** (DB + keys); gitignored |
| `dist/`, `frontend/dist/`, `frontend/coverage/` | Safe to delete; regenerate via build/test/package scripts |
| `frontend/verify-*.txt` | gitignored canaries |
| `.planning/` | **Archived** to `docs/archives/planning-history-2026-07-18.tar.gz`; gitignored if recreated |
| `vendor/` | Keep; Go modules vendored |

---

## Commands (smoke)

```powershell
cd E:\zidqiandao\relaycheck-desktop\frontend
npm test
npm run test:coverage
npm run build

cd E:\zidqiandao\relaycheck-desktop
go test -mod=vendor -count=1 ./internal/core
go test -mod=vendor ./... -count=1 -timeout 120s
```

---

## Open product risks (updated 2026-07-18)

- SQLite full-table FK migration + historical orphan **cleanup** (precheck exists; mutate still needs backup + confirm)  
- Full backup restore service rebind matrix  
- Proxy-mode DNS rebinding  
- Real upstream checkin / desktop signing / production RUM  
