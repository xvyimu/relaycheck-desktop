# HANDOFF.md

Authoritative handoff document for RelayCheck Desktop. Updated each session.
Read this first, then `CLAUDE.md` for architecture.

**Last updated:** 2026-07-11 (β + #8.3 + #9 Phase 2)

---

## Current state

Layout optimization **alpha** is complete; **beta MVP (IA-1 master-detail)**, **#8.3 channel-sync UI**, and **#9 Phase 2 bulk re-login** are implemented on branch `main`.

- **Verification (2026-07-11 evening):**
  - `cd frontend; npx tsc --noEmit` - PASS
  - `cd frontend; npm test` - 25 files / **250** tests PASS
- Branch may be ahead of `origin/main`; **do not push unless asked**.
- Preserved: `vendor/`, `data/`, `frontend/dist/`. Run `cd frontend; npm ci` if `node_modules` missing.
- Constraints still hold: no auto-login / 2FA bypass; no secrets in docs/logs; reuse `?upstreamSiteId=`; no Radix/shadcn.

### Alpha slice checklist

| Slice | Summary | Status |
|-------|---------|--------|
| S1–S6 | Action-first accounts/sites/dashboard/sidebar/CSS | done |

### Beta / #8 / #9 (this session)

| Track | Summary | Status |
|-------|---------|--------|
| β-MVP | `SiteAccountMasterDetail` dual-pane (≥1180) / stack (≤900); default layout on Sites; toggle 主从/卡片 | done |
| #8.3 | `LocalNewAPISyncPanel` + `syncFeedback` counters; empty vs excluded vs needs-token; Scan tab | done |
| #9 Phase 2 | `BulkReloginWizard` (open/save batch); `AccountDetailContent` re-login step strip + CTAs | done |

Still open (not this slice): **#8.4** exclude audit, **#8.2** last-sync persistence; β Tab merge / full IA-2.

### Key files (new / wired)

- `frontend/src/components/sites/SiteAccountMasterDetail.tsx` + `SitesPanel.tsx` (layoutMode)
- `frontend/src/components/scan/LocalNewAPISyncPanel.tsx` + `ScanPanel.tsx`
- `frontend/src/lib/syncFeedback.ts`
- `frontend/src/components/accounts/BulkReloginWizard.tsx` + `AccountsPanel.tsx`
- `frontend/src/components/accounts/AccountDetailContent.tsx` (parity CTAs)
- CSS: `frontend/src/styles/domains/accounts.css` (`master-detail-*`, `local-newapi-*`, `bulk-relogin-*`)
- Tests: `syncFeedback.test.ts`, `BulkReloginWizard.test.tsx`, `SiteAccountMasterDetail.test.tsx`

### Token save note (#8.3)

There is **no** `POST /api/local-newapi/{id}/sync-token`. Saving a system access token is done by posting `accessToken` + `saveAccessToken: true` on `POST …/{id}/sync`.

### Specs

- β review: `docs/superpowers/specs/2026-07-11-layout-beta-design-review-draft.md`
- #8: `docs/superpowers/specs/2026-07-11-newapi-channel-sync-exploration.md` (8.1/8.5/8.3 done)
- #9: `docs/superpowers/specs/2026-07-11-session-relogin-plan.md`

### Suggested verify after pull

```powershell
cd E:\zidqiandao\relaycheck-desktop
go test -mod=vendor -count=1 -run TestListAccounts ./internal/core/
cd frontend
npx tsc --noEmit
npm test
# optional: npm run build
```

---

## Historical sessions

See git log and earlier HANDOFF sections in history for alpha S1–S6, AccountTaskService, CheckinTaskService, and global optimization closure.