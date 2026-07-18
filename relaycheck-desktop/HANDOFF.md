# HANDOFF.md

Authoritative handoff document for RelayCheck Desktop. Updated each session.
Read this first, then `CLAUDE.md` for architecture.

> **PROJECT ENDED 2026-07-19.** Code track closed at `42f21c8`. Reopen keys: PFX 签名材料 / 代表主机 RUM / 「批准 Phase C」。Full closeout: `docs/archives/PROJECT-CLOSEOUT-2026-07-19.md`.

**Last updated:** 2026-07-19 project closeout  
**HEAD:** `42f21c8` on `main` / `origin/main`  
**Worktree policy:** local `dist/` / `frontend/dist/` / `frontend/coverage/` may be deleted anytime (gitignored). Never delete `data/`.

---

## TODO (next session)

**Code track for this product loop is closed.** Only external materials / optional residual polish remain.

**Optional residual code:** **done this pass** — `useApi` consumers now take `systemApi.statusPath` / `dashboardApi.*Path`; UI first-interactive instrumented (`lib/firstInteractive.ts`, local mark + localStorage). Remaining polish: none mandatory.

**Still external / needs materials:**

| Item | Status | Unlock |
|---|---|---|
| Authenticode | **Hard block** — no PFX / no Code Signing EKU | Operator sets `RELAYCHECK_SIGN_PFX` + password, then `scripts/sign-release.ps1` |
| Multi-host / large-DB RUM | Local p95 + cold-start + **UI first-interactive mark shipped** | Representative extra hosts + larger data volume（外部材料） |
| FK Phase C | Deferred | Explicit product confirm (channel/instance weak FKs SET NULL) |

**Do not without explicit confirm:** Phase C FK, delete `data/*`, force-push, cloud deploy, real upstream checkin blasts, minting self-signed “production” certs.

---

## Done this close-out (2026-07-18 evening — pushed)

| Commit | Summary |
|---|---|
| `fd5a87a` | Site delete UI + read-cache invalidate + orphan precheck |
| `4b51187` | Local representative API p95 (N=50) |
| `9d19352` | Local cleanup of 18 orphan `checkin_logs` |
| `3cadcaf` | Signing hard-block after exhaustive cert search |
| `5ce688d` | Evening session close archive |
| `e3e50e9` | accountApi residual migrate + cold-start sampler |
| `2d824f3` | SQLite FK migration design |
| `d702383` | **FK Phase A+B rebuild implemented + local migrate** |

**Also earlier same day:** cascade delete service (`081285c`), typed APIs, planning archive, security loops — `docs/archives/session-close-2026-07-18.md` + `session-close-2026-07-18-evening.md`.

### Site delete

- Backend: transactional cascade (accounts / checkin_logs / balances / schedules / pricing_cache)
- Handler: `invalidateReadCache()` after DELETE
- UI: Sites card + detail drawer + master-detail; `sitesApi.remove` + `lib/siteDelete.ts`
- SOP: `docs/sop/relaycheck-site-delete-cascade-2026-07-18.md`

### Orphans

- Precheck: `scripts/precheck-site-orphans.ps1` + `scripts/sql/precheck-site-orphans.sql`
- Dry-run cleanup SQL: `scripts/sql/cleanup-checkin-log-orphans.dry-run.sql`
- **Local data:** 18 orphan logs deleted; backup `data/backups/pre-orphan-cleanup-20260718-195632/`; precheck all tables **0**
- SOP: `docs/sop/relaycheck-site-orphan-precheck-2026-07-18.md`, `docs/sop/relaycheck-checkin-orphan-cleanup-dry-run-2026-07-18.md`

### Local API p95 (operator host)

- Plan + table: `docs/perf/production-rum-collection-plan-2026-07-18.md`
- Sample JSON (gitignored): `docs/perf/samples/local-api-p95-20260718-195231.json`
- Caveat: loopback, ~1.1MB DB; `accounts-page` p95 ~496ms

### Signing

- Scaffold: `scripts/sign-release.ps1` + `docs/deploy/code-signing-readiness-2026-07-18.md`
- `dist\relaycheck.exe` may exist locally → **NotSigned**

---

## Done (do not redo)

### 2026-07-18 — security closed loops + typed API (pushed)

| Commit | Summary |
|---|---|
| `f5c10de` | Checkin + unsupported-cleanup `previewId` freeze/claim; JSON fail-closed; CST day bounds; atomic checkin save; instance key ACL; public errors; incremental smoke + CI |
| `b98218e` | Typed API owners: accounts, models, keys, channels, local-newapi, notifications, system, scheduler, sites; panel wiring + behavior tests |
| `5f5c556` | SOP architecture/QA/security docs + handoff notes |
| `a8f372d` | Ignore local `frontend/verify-canary.txt` / `verify-nav-output.txt` |

### Contract owners (frontend)

| Owner | Path |
|---|---|
| checkin / cleanup preview | `frontend/src/api/checkins.ts`, `account-cleanup.ts` |
| models / keys | `frontend/src/api/models.ts`, `keys.ts` |
| channels | `frontend/src/api/channels.ts` |
| local-newapi | `frontend/src/api/local-newapi.ts` |
| notifications | `frontend/src/api/notifications.ts` |
| system / scheduler / sites / analytics | `frontend/src/api/system.ts`, `scheduler.ts`, `sites.ts`, `analytics.ts` |

### Local deploy & perf

- Playbook: `docs/deploy/local-desktop-playbook-2026-07-18.md`
- Local sampler: `scripts/sample-local-perf.ps1`
- RUM plan (partial local fill): `docs/perf/production-rum-collection-plan-2026-07-18.md`

### 2026-07-17 and earlier

See `docs/full-stack-code-review-optimization-2026-07-17.md`.

---

## Read order for a new agent

1. This file  
2. `docs/archives/session-close-2026-07-18-evening.md`  
3. `docs/housekeep/project-cleanup-plan-2026-07-18.md`  
4. `docs/sop/relaycheck-site-delete-cascade-2026-07-18.md`  
5. `docs/deploy/code-signing-readiness-2026-07-18.md`  
6. `CLAUDE.md` + user `~\CLAUDE.md` workstyle  

---

## Local hygiene

| Path | Policy |
|---|---|
| `data/` | **Never delete** (DB + keys + local backups); gitignored |
| `dist/`, `frontend/dist/`, `frontend/coverage/` | Safe to delete; regenerate via build/test/package |
| `frontend/verify-*.txt` | gitignored canaries |
| `.planning/` | Archived; gitignored if recreated |
| `docs/perf/samples/*.json` | gitignored local samples |
| `vendor/` | Keep |

---

## Commands (smoke)

```powershell
cd E:\zidqiandao\relaycheck-desktop\frontend
npm test
npm run build

cd E:\zidqiandao\relaycheck-desktop
go test -mod=vendor -count=1 ./internal/sites -run DeleteUpstream
go test -mod=vendor -count=1 ./internal/core -timeout 120s
```

---

## Open product risks

- SQLite FK Phase C (channel/instance weak links) still deferred  
- Full backup restore service rebind matrix  
- Proxy-mode DNS rebinding  
- Real upstream checkin blasts / Authenticode / multi-host RUM / UI first-interactive  
