# HANDOFF.md

Authoritative handoff document for RelayCheck Desktop. Updated each session.
Read this first, then `CLAUDE.md` for architecture.

**Last updated:** 2026-07-11 (Action Center site deep-links + #9 R2 session chip)

---

## Current state

Layout optimization **alpha** is complete; **beta MVP (IA-1 master-detail)**, **#8.3 channel-sync UI**, **#9 Phase 2 bulk re-login**, **#8.4/#8.2**, **Action Center site sample deep-links**, and **#9 R2 session indicator** are implemented on branch `main`.

- **Verification (2026-07-11 night):**
  - `go test -mod=vendor ./internal/accounts/ ./internal/core/` PASS
  - `go test -mod=vendor ./internal/core/ -run ActionCenter` PASS (incl. site/channel sample entities)
  - `cd frontend; npx tsc --noEmit` PASS
  - `cd frontend; npx vitest run` navigation + accountActions + AccountCard **45/45** PASS
- Branch may be ahead of `origin/main`; **do not push unless asked**.
- Preserved: `vendor/`, `data/`, `frontend/dist/`. Run `cd frontend; npm ci` if `node_modules` missing.
- Constraints still hold: no auto-login / 2FA bypass; no secrets in docs/logs; reuse `?upstreamSiteId=`; no Radix/shadcn; never “请关闭 2FA”.

### Alpha slice checklist

| Slice | Summary | Status |
|-------|---------|--------|
| S1–S6 | Action-first accounts/sites/dashboard/sidebar/CSS | done |

### Beta / #8 / #9 / Action Center

| Track | Summary | Status |
|-------|---------|--------|
| β-MVP | `SiteAccountMasterDetail` dual-pane (≥1180) / stack (≤900); default layout on Sites; toggle 主从/卡片 | done |
| #8.3 | `LocalNewAPISyncPanel` + `syncFeedback` counters; empty vs excluded vs needs-token; Scan tab | done |
| #8.4 | `skippedExcludedSamples` (+truncated) on Admin/SQLite import; `ListExcludedRelaySiteRules`; `GET /api/local-newapi/exclude-rules`; panel 只读规则 | done |
| #8.2 | `last_sync_at` / `last_sync_summary` ensureColumn; List/Get SELECT; Sync 成功后 `SaveLocalNewAPILastSyncSummary`; 实例卡展示上次摘要 | done |
| #9 Phase 2 | `BulkReloginWizard` (open/save batch); `AccountDetailContent` re-login step strip + CTAs | done |
| #9 R2 / 9.3 | Persistent session chip while `reloginPhase === browser_open` (`opened` / `already_open`); helpers in `accountActions` | done |
| Action Center deep-link | `ActionSample` with entityType/entityId; unreachable-sites + channel-health-risks return site ids; sample click → sites master-detail | done |

Still open: β Tab merge / full IA-2 (out of #8 scope). Accounts tab still accepts legacy upstreamSiteId filter.

### Key files (this slice)

**Action Center site deep-links**
- Backend: `internal/core/models.go` (`ActionSample`), `action_center.go` (`sampleEntityType`, 2-col sample SQL for unreachable-sites / channel-health-risks), `action_center_test.go`
- Frontend: `types/index.ts`, `lib/navigation.ts` (`actionSampleNavigationIntent`), `Dashboard.tsx` (clickable `task-sample-link`), `styles/domains/dashboard.css`
- Tests: `navigation.test.ts`

**#9 R2 session indicator**
- Frontend: `lib/accountActions.ts` (`browserSessionOpenKind`, `browserSessionRunningLabel`), `AccountCard.tsx`, `AccountDetailContent.tsx`, `styles/domains/accounts.css` (`.account-session-chip`)
- Tests: `accountActions.test.ts`

### Token save note (#8.3)

There is **no** `POST /api/local-newapi/{id}/sync-token`. Saving a system access token is done by posting `accessToken` + `saveAccessToken: true` on `POST …/{id}/sync`.

### Specs

- β review: `docs/superpowers/specs/2026-07-11-layout-beta-design-review-draft.md`
- #8: `docs/superpowers/specs/2026-07-11-newapi-channel-sync-exploration.md` (8.1/8.5/8.3/8.4/8.2 done)
- #9: `docs/superpowers/specs/2026-07-11-session-relogin-plan.md` (9.1–9.6 + R2/9.3 + Phase 2 bulk; no auto-login)

### Suggested verify after pull

```powershell
cd E:\zidqiandao\relaycheck-desktop
go test -mod=vendor -count=1 ./internal/accounts/ ./internal/core/
cd frontend
npx tsc --noEmit
npm test
# optional: npm run build
```

---

## Historical sessions

See git log and earlier HANDOFF sections in history for alpha S1–S6, AccountTaskService, CheckinTaskService, and global optimization closure.
