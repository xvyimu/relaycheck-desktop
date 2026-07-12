# HANDOFF.md

Authoritative handoff document for RelayCheck Desktop. Updated each session.
Read this first, then `CLAUDE.md` for architecture.

**Last updated:** 2026-07-12 (S2 review polish + FE/BE)

---

## ⏳ TODO (next session)

- [x] **Push local commits** — S0/S1 + S2 `91b9f40` → `origin/main` via `ssh://git@github.com-obsidian/...` (https 443/proxy dead)
- [x] **Release zip (post CSP bootstrap)** — `dist/releases/relaycheck-desktop-1.1.0-00529ee1ca70-20260712-145035.zip`  
  Zip SHA256 `8b8b7efe3e018e9c6b0026043f30ecf44de2441a151fe7b6c3cb7a7d674a01a3` · `verify-package` PASS  
  (supersedes `…-2611e3a5c9c4-…` / `a7990685…`)
- [x] **Visual smoke** — `scripts/visual-smoke-theme.mjs` PASS (light/dark token L, 6 nav tabs, no hard console/CSP after external bootstrap). Screenshots local `.tmp/visual-smoke/` only.
- [x] **Operator session-expiry runbook** — `docs/OPERATOR_SESSION_EXPIRY_RUNBOOK.md` tracked; linked from `OPERATOR_RUNBOOK.md`.
- [x] **Full-stack review report** — `docs/code-review-optimization-2026-07-12.md`（FE/BE/Arch/Config；P0=0 / P1≈12）
- [x] **S0 review fixes** — FE-1 trailing Button import；BE-1/BE-2 SSRF+redirect；BE-6 scheduler CAS
- [x] **S1 review fixes** — RELAYCHECK_DATA_DIR / exe-dir data root；ldflags version；verify-release lint+embed smoke；graceful shutdown
- [x] **S2 review polish** — BE-4/5 + FE-2/3/4/5/6/7（见下方 Current state）

---

## Current state

Layout optimization **alpha** is complete; **beta MVP (IA-1 master-detail)**, **IA-2** (accounts tab physically merged into 站点与账号), **#8.3 channel-sync UI**, **#9 Phase 2 bulk re-login**, **#8.4/#8.2**, **Action Center site sample deep-links**, **#9 R2 session indicator**, and **2026-07-12 frontend optimization** are on branch `main` (local; may be unpushed).

### S2 review polish (2026-07-12) — done

| ID | Summary | Status |
|----|---------|--------|
| BE-5 | GET settings mask notification secrets; PUT empty preserves ciphertext | done |
| BE-4 | SQLite import path allowlist (Abs+EvalSymlinks+roots) | done |
| FE-2 | keep-alive `inert`/`aria-hidden`; `useApi({enabled})` pause | done |
| FE-3 | ChannelsPanel seeds from inventory; models-only when seeded | done |
| FE-6 | DialogShell body scroll-lock + `titleId`/aria-labelledby | done |
| FE-5 | SettingsProxy / SettingsBackup / SettingsExportImport; clear export password | done |
| FE-4 | recovery cascade note + drop duplicate `.muted` | done |
| FE-7 | settings-split + dialog-shell contract tests | done |

**Verify (S2):**

```powershell
cd E:\zidqiandao\relaycheck-desktop
go test -mod=vendor -count=1 ./internal/accounts/ ./internal/core/
cd frontend
npx tsc -b
npm test          # 264 passed
npm run lint
```

### Frontend optimization (2026-07-12) — done

Report: `docs/frontend-optimization-report-2026-07-12.md`

| Track | Summary | Status |
|-------|---------|--------|
| Theme integrity | Fix circular `--surface-solid`; class-only `html.dark`; FOUC bootstrap | done |
| Primitives | Tokenized Button/Card/Badge/Empty; **no Radix/shadcn** | done |
| DialogShell | Shared Escape + focus trap/cycle/restore; Sites / Accounts / master-detail / Channels / Onboarding / 2FA | done |
| Idle tabs | `lib/idle-tabs.ts` 5 min TTL; dashboard pinned | done |
| Lazy + chunks | Non-dashboard panels `React.lazy`; Vite `manualChunks` | done |
| CSS merge | `layers/control-room.css` = redesign → layout-harmonization → linear; old files stubbed | done |
| Button ghost | `variant="ghost"` → CSS class `ghost`; product ghost call sites migrated; 0 residual `button.ghost` natives | done |
| exhaustive-deps | Stable refresh/destructure + Settings `DEFAULT_*` module constants | done |
| Tooling | ESLint 9 flat + react-hooks; Prettier | done |

**Verify (frontend close-out):**

```powershell
cd E:\zidqiandao\relaycheck-desktop\frontend
npx tsc -b
npm test          # 264 passed
npm run build
npm run lint      # 0 errors, 0 warns
```

### Product tracks

| Track | Summary | Status |
|-------|---------|--------|
| S1–S6 | Action-first accounts/sites/dashboard/sidebar/CSS | done |
| β-MVP | `SiteAccountMasterDetail` dual-pane (≥1180) / stack (≤900); default layout on Sites | done |
| IA-2 | Physically merge accounts tab into 站点与账号 (`c517226`) | done |
| #8.3 | `LocalNewAPISyncPanel` + `syncFeedback`; Scan tab | done |
| #8.4 | exclude samples + `GET /api/local-newapi/exclude-rules` | done |
| #8.2 | last_sync_at / last_sync_summary on instances | done |
| #9 Phase 2 | `BulkReloginWizard` + detail re-login strip | done |
| #9 R2 / 9.3 | Session chip while `browser_open` | done |
| Action Center deep-link | `ActionSample` → sites master-detail | done |

### Constraints (still hold)

- No auto-login / 2FA bypass; never “请关闭 2FA”
- No secrets in docs/logs
- Reuse `?upstreamSiteId=`
- No Radix/shadcn (project-owned `components/ui/*` only)
- **Do not push unless asked** (or after proxy is up and user expects publish)

### Token save note (#8.3)

There is **no** `POST /api/local-newapi/{id}/sync-token`. Saving a system access token is done by posting `accessToken` + `saveAccessToken: true` on `POST …/{id}/sync`.

### Specs

- Frontend opt report: `docs/frontend-optimization-report-2026-07-12.md`
- **Full-stack review:** `docs/code-review-optimization-2026-07-12.md`（2026-07-12）
- β review: `docs/superpowers/specs/2026-07-11-layout-beta-design-review-draft.md`
- #8: `docs/superpowers/specs/2026-07-11-newapi-channel-sync-exploration.md`
- #9: `docs/superpowers/specs/2026-07-11-session-relogin-plan.md`

### Suggested verify after pull

```powershell
cd E:\zidqiandao\relaycheck-desktop
go test -mod=vendor -count=1 ./internal/accounts/ ./internal/core/
cd frontend
npx tsc -b
npm test
npm run lint
# optional: npm run build
```

Preserved: `vendor/`, `data/`, `frontend/dist/`. Run `cd frontend; npm ci` if `node_modules` missing.

---

## Historical sessions

See git log and earlier HANDOFF history for alpha S1–S6, AccountTaskService, CheckinTaskService, and global optimization closure.
