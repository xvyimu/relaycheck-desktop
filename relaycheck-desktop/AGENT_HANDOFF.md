# RelayCheck Hub Agent Handoff

## 0. Immediate Resume Packet

- Resume objective: continue phase 84 by extracting only `HubRadar` from `frontend/src/main.tsx`.
- Current status: phase 83 is complete; phase 84 is not complete.
- Target checklist state: `E:\zidqiandao\目标\TARGET_PROMPT_CHECKLIST.md` has `constants 提取到 lib/constants.ts` checked, but `HubRadar 拆分` remains unchecked.
- Current file state: `frontend/src/components/dashboard` exists, but `frontend/src/components/dashboard/HubRadar.tsx` does not exist yet.
- Do not mark complete until: `HubRadar.tsx` exists, `main.tsx` imports/renders it, `HubRadar` no longer lives in `main.tsx`, and `cd E:\zidqiandao\relaycheck-desktop\frontend; npm run build` passes.
- Recommended first edit: move the `HubRadar` component body from `frontend/src/main.tsx` to `frontend/src/components/dashboard/HubRadar.tsx`.
- Behavior boundary: preserve the current navigation behavior by passing `actionNavigationIntent` into `HubRadar` as a prop, or by extracting/importing the helper without changing its return shape.
- Known dependency wrinkle: `HubRadar` currently uses `LoadingSkeleton`, which still lives in `main.tsx`; either pass/import a minimally extracted skeleton component, or do a tiny safe extraction before moving `HubRadar`.
- Validation after this slice: run only the frontend build first; Go tests are not required for a frontend-only component move unless backend files change.
- Runtime note: as of 2026-06-21, port 3001 is occupied by `node.exe` PID 70024, not the rebuilt `relaycheck.exe`; do not rely on the old PID below as current runtime proof.

## 1. Current Snapshot

- Project path: `E:\zidqiandao\relaycheck-desktop`
- Runtime: Go backend + embedded React/Vite frontend + SQLite
- Frontend stack: React 19, TypeScript, Vite 8, plain CSS design-system layers
- Database: `data\relaycheck.db`
- Desktop binary: `dist\relaycheck.exe`
- Local URL: `http://127.0.0.1:3001`
- Default local login: `admin / <local bootstrap password>`; on a fresh database set `RELAYCHECK_BOOTSTRAP_PASSWORD` before first launch or read `data/bootstrap-admin-password.txt`.
- Latest confirmed desktop PID in older records: `51152` (stale; current 3001 listener observed as `node.exe` PID 70024)
- Current date of this handoff: 2026-06-21

This is a local personal tool for managing NewAPI / OneAPI / Sub2API / modified relay sites, accounts, check-ins, balances, model/key checks, pricing, notifications, backup/restore, and local NewAPI sync. It is not a commercial platform, proxy pool, or resale system.

## 2. Files To Read First

- `AGENT_HANDOFF.md`: this file, start here.
- `PROMPT_CHECKLIST.md`: master checklist converted from the user's prompt; update checkboxes as each item is completed.
- `DESIGN_SYSTEM.md`: visual/product rules. Keep the Control Room direction.
- `progress.md`: chronological validation log.
- `findings.md`: product/technical decisions and gotchas.
- `task_plan.md`: completed phases and remaining roadmap.
- `frontend/src/main.tsx`: main React UI.
- `frontend/src/styles.css`: all UI styling and responsive/card rules.
- `internal/core`: Go application, DB, APIs, scheduler, scanner, account/check-in/balance logic.

## 3. Recent Completed Work

### P0 Local API Security And Observability Pass

- Added local `Host` validation and base security headers through `SecureLocalHandler`.
- Added outbound URL safety checks for SSRF-sensitive paths; default external requests reject loopback/private/link-local/metadata targets, while trusted local probes opt in explicitly.
- Added `audit_log` migration, read-only `/api/system/audit-log`, and a compact Settings audit card.
- Audit events now cover login success/failure/logout, settings updates, backup create/delete/restore, account create/update/delete, and upstream site deletion.
- Added unauthenticated `/api/health` for startup/smoke checks. It verifies DB connectivity, database path, data directory, keys directory, and scheduler state.
- Clamped external batch action limits to `1..10`; Admin API page size is clamped to `10..100`.
- Latest verification for this pass:
  - `go test -mod=vendor ./...` passed.
  - `npm run build` in `frontend` passed.

### Important Current Validation Note

- The user asked to finish the prompt batch before final building; do not keep rebuilding after every small edit.
- This pass intentionally ran `npm run build` only after the P0 security/observability slice was complete.

### Code Health Pass

- Added `frontend/.npmrc` with `package-lock=true` so this project is not affected by the user's global npm config that disables lockfiles.
- Added/refreshed `frontend/package-lock.json` for reproducible frontend installs.
- Upgraded frontend build tooling:
  - `vite` to `8.0.16`
  - `@vitejs/plugin-react` to `6.0.2`
  - `esbuild` to `0.28.1`
- Moved build-only tooling into `devDependencies`; runtime dependencies now stay limited to React/React DOM.
- Cleared npm audit: `npm audit --audit-level=low` returns `found 0 vulnerabilities`.
- Replaced `newID()` random-source failure `panic` with a crypto-random-first fallback path using timestamp + atomic counter.
- Added `internal/core/app_test.go` to cover the ID fallback behavior.

### Latest UI/Layout Pass

- Operation and diagnostic cards stay compact on desktop/tablet.
- They switch to full-width only at `max-width: 560px`.
- Desktop smoke confirmed:
  - action cards: about `264px`
  - diagnostic cards: about `236px`
  - channel cards: about `324px`
  - account cards: about `304px`
- 390px mobile smoke confirmed no horizontal overflow.

### AI API Hub Radar Pass

- Used `agent-reach` with GitHub CLI to review `qixing-jk/all-api-hub`.
- Reference repo: `https://github.com/qixing-jk/all-api-hub`
- Repository description highlights the same product direction requested here: New-API/Sub2API account hub, balance/usage dashboard, auto check-in, one-click keys, price comparison, health checks, and advanced channel management.
- Latest release observed during this pass: `v3.47.0`, published 2026-06-16.
- Added a Dashboard "AI API Hub Radar" section using existing local APIs only:
  - `/api/models/overview`
  - `/api/models/pricing`
  - `/api/usage/overview`
  - existing status/action-center/diagnostics data
- The radar has four compact soft cards:
  - assets: accounts, channels, local NewAPI, unread notifications
  - key/models: valid keys, known models, usable model accounts, fastest latency
  - cost/usage: pricing sources, model price coverage, low balance, declining accounts
  - automation/health: scheduler state, next check-in/sync, top action-center issue
- Cards include direct navigation actions to channels, scan/sync, accounts/Key library, balances, settings, and problem handling.
- No new backend tables or heavy UI dependencies were added.

### Performance Architecture Pass

- Kept the requested lightweight Go + SQLite + embedded React/Vite architecture, but upgraded the hot path instead of migrating to a heavier stack.
- SQLite startup now uses WAL with `busy_timeout`, `synchronous=NORMAL`, in-memory temp storage, a larger cache target, and a 4-connection pool for better concurrent local reads.
- Added migration-backed performance indexes for common list/overview paths:
  - imported channels by source status/kind and updated time
  - upstream sites by kind/updated time
  - accounts by updated time and key-check time
  - check-in logs and balance snapshots by account/site plus time
  - notifications by read state plus created time
- Added a small backend short-TTL read cache for repeated local dashboard/list/overview requests:
  - dashboard summary
  - channels list
  - upstream sites list
  - accounts list
  - model overview
  - model pricing overview
  - usage overview
- Cache is cleared when notification-producing write actions complete, and notification mark/clear actions explicitly clear it.
- Added frontend GET request coalescing with a 1.5s TTL; POST/PUT/DELETE success clears the client read cache.
- Excluded dynamic session and check-in status reads from frontend caching so login state and countdown/running state stay live.
- After the production build, `frontend\node_modules`, `frontend\tsconfig.tsbuildinfo`, and `E:\zidqiandao\.npm-cache` were removed because the running desktop binary only needs `frontend\dist` embedded at build time.

### Linear-Inspired Visual Pass

- Used Linear's public website as a visual reference, translating the feel rather than copying the page:
  - precise white surfaces
  - fine 1px borders
  - low-noise shadows
  - subtle grid background
  - restrained typography and high scanability
- Preserved the user's requested white/blue rounded-card "improved version" direction.
- Added a final CSS finishing layer in `frontend/src/styles.css` instead of restructuring business JSX.
- Kept the app as an operations console, not a marketing landing page.
- Fixed a mobile overflow regression from the visual layer by forcing the shell/sidebar/main layout to become a single-column block under 900px.
- Latest visual smoke:
  - desktop and 390px mobile have no horizontal overflow
  - console/page errors are empty
  - checked pages: 总览、渠道、上游站点、账号、签到、余额、通知、设置

### shadcn/Tailwind Dashboard Prototype Pass

- User rejected the earlier CSS-only Linear attempt as not close enough and requested a more aggressive shadcn-ui direction.
- Added Tailwind CSS via `@tailwindcss/vite` and kept the current Vite/React architecture.
- Added lightweight shadcn-style local components:
  - `frontend/src/components/ui/button.tsx`
  - `frontend/src/components/ui/card.tsx`
  - `frontend/src/components/ui/badge.tsx`
  - `frontend/src/lib/cn.ts`
- Rebuilt only the dashboard first as the visual sample:
  - Command Center hero
  - five-metric strip
  - four balanced quadrants: assets, keys/models, automation/check-in, cost/usage
  - right-side priority queue
  - compact diagnostics summary
- Preserved existing business behavior and API calls; this pass is a visual/information architecture prototype for the dashboard.
- Current production build embeds the new dashboard; `frontend\node_modules`, `frontend\tsconfig.tsbuildinfo`, and `E:\zidqiandao\.npm-cache` were removed after build.

## 4. Latest Verification

All of these passed after the latest changes:

```powershell
cd E:\zidqiandao\relaycheck-desktop\frontend
npm ci --cache E:\zidqiandao\.npm-cache
npm run build

cd E:\zidqiandao\relaycheck-desktop
go test -mod=vendor ./...
go build -mod=vendor -ldflags="-H windowsgui" -o dist\relaycheck.exe .
```

Runtime/API smoke:

- Hidden desktop binary rebuilt and restarted on `127.0.0.1:3001`.
- Latest PID: `51152`.
- Login smoke passed.
- API timing smoke after the performance pass returned:
  - `/api/system/status` average about `1ms`
  - `/api/channels` average about `4.6ms`
  - `/api/accounts` average about `1.6ms`
  - `/api/models/pricing` average about `8ms`
  - `/api/usage/overview` average about `0.4ms`

Browser smoke:

- Used Playwright without screenshots.
- Checked: 总览、渠道、上游站点、账号、签到、余额、通知、设置.
- Desktop and 390px mobile had no horizontal overflow.
- `consoleErrors=[]`, `pageErrors=[]`.
- Latest Hub Radar smoke confirmed 4 radar cards on dashboard:
  - desktop card width: about `276px`
  - mobile card width: about `343px`
  - no horizontal overflow

Security scan:

- Sensitive scan for previously shared token/password/email fragments returned no matches.
- Do not write real user passwords, tokens, cookies, API keys, or browser session data into docs, source, or temp files.

Blocked validation:

- `go test -race ./internal/core` was attempted but did not run because the current Go environment has cgo disabled: `-race requires cgo`. This is environment-related, not a test failure.

## 5. How To Build And Restart

From project root:

```powershell
cd E:\zidqiandao\relaycheck-desktop

cd frontend
npm ci --cache E:\zidqiandao\.npm-cache
npm run build
cd ..

go test -mod=vendor ./...
go build -mod=vendor -ldflags="-H windowsgui" -o dist\relaycheck-next.exe .

$listeners = Get-NetTCPConnection -LocalPort 3001 -State Listen -ErrorAction SilentlyContinue
foreach ($listener in $listeners) {
  $proc = Get-Process -Id $listener.OwningProcess -ErrorAction SilentlyContinue
  if ($proc -and $proc.ProcessName -like 'relaycheck*') {
    Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
  }
}
Get-Process | Where-Object { $_.ProcessName -like 'relaycheck*' } | Stop-Process -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 1
Copy-Item -Force dist\relaycheck-next.exe dist\relaycheck.exe
$env:RELAYCHECK_PORT='3001'
$env:RELAYCHECK_NO_OPEN='1'
Start-Process -FilePath (Resolve-Path dist\relaycheck.exe) -WorkingDirectory (Resolve-Path .) -WindowStyle Hidden
```

Port check:

```powershell
Get-NetTCPConnection -LocalPort 3001 -State Listen -ErrorAction SilentlyContinue |
  Select-Object -First 1 OwningProcess,LocalAddress,LocalPort,State
```

## 6. Current Product State

### Local NewAPI Sync

- Local NewAPI discovery and sync are implemented.
- Supports saved access token import/sync.
- One-click sync exists per local instance and all available instances.
- Sync marks source-missing channels instead of deleting local data.
- Scheduled local NewAPI sync defaults to 30 minutes.

### Channels And Upstream Sites

- Channels focus on NewAPI / OneAPI / Sub2API / modified relay sites.
- Channel cards display support for check-in, balance, models, pricing, status, model counts, and sync status.
- Detection stores explainable signals and confidence.
- Pure official-provider or unsupported router-style channels should stay out of the default daily view.

### Accounts

- Same site can have multiple accounts.
- Account cards are compact by default.
- Daily actions stay visible; maintenance actions are under "更多".
- Each account can edit display name, account/email, auth type, password/API key/cookie/token fields, site URL, login URL, and backend kind.
- Editing site URL supports "current account only" vs shared site update semantics.
- Saved secrets are encrypted; blank sensitive edit fields preserve existing encrypted values.

### Browser Authorization

- Browser login/session save flow exists.
- Keep per-account browser profile/session behavior.
- Do not silently read/decrypt Chrome passwords or cookies.
- Do not bypass CAPTCHA, 2FA, passkeys, or platform risk controls.

### Check-ins

- Manual and automatic check-in are implemented.
- Scheduler persists daily run keys to avoid duplicate automatic runs after restart.
- Check-in page shows current run status, countdown, success rows, and failure/problem rows.
- Unsupported check-in should be clearly marked, not treated as a generic failure.

### Balances, Usage, Models, Pricing

- Balance refresh and snapshots exist.
- Site-level balance aggregation exists.
- Key validity/model detection exists using local direct upstream calls only.
- Model availability and latency are stored as summaries.
- Pricing radar merges imported NewAPI raw config, live pricing cache, Key usability, and latency.
- External modeloc/llmtest-style behavior should stay local-only; never submit user keys to third-party test sites.

### Notifications

- Important notifications are prioritized.
- Routine success/info notices are tucked away by default.
- Notification center supports read/unread handling.

### Backup/Restore

- Settings page supports backup creation, restore, and multi-select cleanup of old backups.
- Restore creates a current DB snapshot before replacing the database.

## 7. Architecture And Security Rules

- Keep the current lightweight architecture unless the user explicitly asks for a migration.
- Do not add heavy UI frameworks or component libraries just for styling.
- Do not migrate to Tailwind/shadcn/Radix; `shadcn/ui` is a visual reference only.
- Use existing Go + SQLite + React/Vite patterns.
- Use parameterized SQL. Existing dynamic SQL should only use whitelisted identifiers.
- `SELECT * FROM channels` in SQLite import/sync is intentional for schema introspection and rawJson preservation across NewAPI/OneAPI variants.
- Do not commit or write user secrets to docs/temp/source:
  - real passwords
  - real access tokens
  - cookies
  - API keys
  - bearer tokens
  - Chrome password/cookie store contents
- Key export remains safe by design: metadata/fingerprints/status only, no plaintext keys.

## 8. UI Design Rules

See `DESIGN_SYSTEM.md` for the full direction. Short version:

- RelayCheck is a local operations Control Room, not a landing page.
- Prioritize compact cards, high scanability, and problem-first workflows.
- Avoid long full-width banners unless on mobile.
- Cards should be rounded and soft, but not bloated.
- Important data gets larger type; secondary metadata gets smaller chips.
- Desktop can flow horizontally; mobile must be single-column with no horizontal overflow.
- Operation/diagnostic cards stay short on desktop and tablet; only full-width under 560px.
- Do not add decorative orbs, heavy gradients, or marketing-style hero sections.

## 9. Known Gotchas

- The frontend has no lint script; current quality gates are build, audit, Playwright smoke, and Go checks.
- `go test -race` needs cgo enabled.
- The repo path used here is not necessarily a Git repository root. Do not assume git commands work from the project folder.
- The user's machine may have global npm config `package-lock=false`; keep `frontend/.npmrc`.
- There can be multiple local servers on 3000/3001. RelayCheck target is `127.0.0.1:3001`.
- Use system Chrome/Playwright carefully and avoid screenshots unless the user asks. The user previously said no screenshots.
- 7897 proxy may be used for external checks. Local addresses should bypass proxy.
- Some sites are modified NewAPI/OneAPI and may return login pages or nonstandard API responses; diagnostics should explain what was tried.

## 10. Recommended Next Work

Highest-value next steps:

1. Add a compact "站点详情页/抽屉" if not already complete enough:
   - show detected kind
   - matched probe signals
   - health status
   - check-in support reason
   - balance/model/pricing support reason
   - recommended fix for each failed probe

2. Improve NewAPI sync review:
   - one-click sync summary already exists
   - next improvement is clearer conflict handling for renamed channels and duplicated base URLs

3. Improve account Key workspace:
   - batch retest stale keys
   - safe CSV export of fingerprints/status/model samples only
   - never export plaintext keys unless the user explicitly requests and confirms a secure path

4. Improve pricing/usage analytics:
   - richer local snapshot trend charts
   - per-site burn rate
   - per-model cost comparison from cached/imported sources

5. Improve diagnostics:
   - every system self-check item should include a concrete solution
   - link actions should pre-filter the target page

## 11. Useful Commands

Build and test:

```powershell
cd E:\zidqiandao\relaycheck-desktop
go test -mod=vendor ./...
go vet ./...

cd frontend
npm ci --cache E:\zidqiandao\.npm-cache
npm run build
npm audit --audit-level=low
```

Sensitive scan:

```powershell
cd E:\zidqiandao\relaycheck-desktop
rg -n --glob '!frontend/node_modules/**' --glob '!frontend/dist/**' --glob '!dist/**' --glob '!data/**' "<known-sensitive-fragment-1>|<known-sensitive-fragment-2>|sk-[A-Za-z0-9]|Bearer\s+[A-Za-z0-9_\-.=]{12,}|api[_-]?key\s*[:=]\s*['""]" .
```

API smoke idea:

```powershell
$session = New-Object Microsoft.PowerShell.Commands.WebRequestSession
$body = @{ username = 'admin'; password = $env:RELAYCHECK_SMOKE_PASSWORD } | ConvertTo-Json
Invoke-RestMethod -Uri http://127.0.0.1:3001/api/auth/login -Method Post -ContentType 'application/json' -Body $body -WebSession $session | Out-Null
(Invoke-RestMethod -Uri http://127.0.0.1:3001/api/channels -WebSession $session).data.items.Count
(Invoke-RestMethod -Uri http://127.0.0.1:3001/api/accounts -WebSession $session).data.items.Count
```

## 12. Handoff Discipline For Future Agents

- Update this file after any meaningful architecture, dependency, runtime, or UX change.
- Put detailed step-by-step validation logs in `progress.md`.
- Put durable decisions and gotchas in `findings.md`.
- Keep `DESIGN_SYSTEM.md` as the UI source of truth.
- Before final response, verify the app still builds and the desktop process is either intentionally running or clearly reported as not restarted.

## 13. 2026-06-20 Radical V4 UI Rebuild

- Product direction: aggressive Linear/shadcn-style command workspace, still white/blue, rounded cards, compact density, clear primary vs secondary information.
- Main files changed:
  - `frontend/src/main.tsx`
  - `frontend/src/styles.css`
- App shell now uses `shell-v4`, `sidebar-v4`, `main-v4`, `topbar-v4`, and `workspace-canvas`.
- Sidebar now has richer nav copy, active pill styling, and a bottom local engine status card.
- Channels page now has a `page-brief`, `channels-panel`, `channel-card-v4`, source status pill, 3-metric card layout, and secondary chips that stay compact until hover.
- Accounts page now has a `page-brief`, `accounts-panel-v4`, `account-card-v4`, and a 4-metric row covering account, check-in, balance, and Key status.
- CSS V4 layer is appended near the end of `frontend/src/styles.css`; it intentionally overrides older accumulated layers instead of deleting them during this risky single-file refactor.
- Mobile density pass changes sidebar navigation to a horizontal compact strip under 560px so the content appears much earlier.
- Validation passed:
  - `npm run build`
  - `go test -mod=vendor ./...`
  - `go build -mod=vendor -ldflags="-H windowsgui" -o dist\relaycheck.exe .`
  - Playwright smoke on `http://127.0.0.1:3001`: login, 9 tabs, V4 shell, dashboard quadrants, channel/account cards, desktop no overflow, 390px mobile no overflow, `consoleErrors=[]`, `pageErrors=[]`.
- Runtime after this handoff: `dist\relaycheck.exe` restarted hidden on `127.0.0.1:3001`.
- Cleanup after validation removed `frontend/node_modules`, `frontend/tsconfig.tsbuildinfo`, `E:\zidqiandao\.npm-cache`, and temporary Playwright scripts/screenshots.

## 14. 2026-06-20 Product Loop Upgrade

- Product direction: move from "pretty dashboard" to a daily operations loop: task -> inspect -> act -> confirm.
- Main files changed:
  - `frontend/src/main.tsx`
  - `frontend/src/styles.css`
  - `AGENT_HANDOFF.md`
  - `progress.md`
- Dashboard priority queue is now a `TaskCenter`:
  - shows readiness score
  - separates high-priority and routine items
  - surfaces count, samples, recommended resolution, and quick actions
  - uses existing `/api/system/action-center`
- Channels now expose a `详情` action on each `channel-card-v4`.
- Accounts now expose a `详情` action on each `account-card-v4`.
- New generic `DetailDrawer` supports account and channel inspector views:
  - account inspector shows login, check-in, balance, Key, model samples, and suggested next actions
  - channel inspector shows detection conclusion, source status, model sync, cleanup advice, and truncated raw detection JSON
- Existing site detail drawer remains intact.
- Validation passed:
  - `npm run build`
  - `go test -mod=vendor ./...`
  - `go build -mod=vendor -ldflags="-H windowsgui" -o dist\relaycheck.exe .`
  - Playwright smoke on `http://127.0.0.1:3001`: login, task center, account drawer, channel drawer, mobile 390px no overflow, `consoleErrors=[]`, `pageErrors=[]`.
- Runtime after this handoff: `dist\relaycheck.exe` restarted hidden on `127.0.0.1:3001`.

## 15. 2026-06-20 C Drive Cleanup And E Drive Ownership

- Confirmed active project root is `E:\zidqiandao\relaycheck-desktop`.
- Confirmed C drive path `C:\Users\yuanjia\Documents\Codex\2026-06-17\e-zidqiandao` contained only empty `work`, `tmp`, and `outputs` directories; no unique code or artifacts needed to be migrated.
- Removed:
  - `C:\Users\yuanjia\Documents\Codex\2026-06-17\e-zidqiandao`
  - `C:\Users\yuanjia\Documents\Codex\tmp`
  - `C:\Users\yuanjia\AppData\Local\npm-cache`
  - `E:\zidqiandao\relaycheck-desktop\dist\relaycheck.exe~`
- C drive free space after cleanup: about 29.20GB.
- E drive free space after cleanup: about 1.35GB; most E usage is protected browser auth/data under `E:\zidqiandao\data` and should not be deleted casually.
- Runtime remained healthy after cleanup: `dist\relaycheck.exe` still listening on `127.0.0.1:3001`.

## 16. 2026-06-20 Touch Targets And Responsive Dashboard Grids

- Master checklist phase: 66.
- Main file changed:
  - `frontend/src/styles.css`
- Dashboard grids now avoid fixed column counts:
  - `.dashboard-main-grid` uses `repeat(auto-fit, minmax(min(100%, 320px), 1fr))`.
  - `.dashboard-diagnostics-grid` uses `repeat(auto-fit, minmax(min(100%, 210px), 1fr))`.
  - `.hub-radar-grid` uses `auto-fit/minmax` instead of `auto-fill`.
- Touch targets:
  - CSS now has a final `@media (pointer: coarse)` layer.
  - It raises native buttons, `[role="button"]`, submit/reset/button inputs, and known compact action button groups to at least 44x44px.
  - This intentionally does not change desktop mouse density.
- Validation passed:
  - `npm run build`
- Documentation updated:
  - `PROMPT_CHECKLIST.md`
  - `task_plan.md`
  - `progress.md`
  - `findings.md`
  - `DESIGN_SYSTEM.md`

## 17. 2026-06-20 Mobile Main Content Single Column Guard

- Master checklist phase: 67.
- Main file changed:
  - `frontend/src/styles.css`
- Added a final `@media (max-width: 760px)` guard for main content grids:
  - stats, Hub Radar, action/diagnostic grids
  - channel/account/balance/settings grids
  - dashboard layout/main/diagnostics/rail
  - check-in priority, capability, usage, scheduler, drawer detail, JSON preview grids
- Intentional exception:
  - `.sidebar-v4 nav` is not included because the mobile navigation is intentionally a compact horizontal strip.
- Validation passed:
  - `npm run build`
- Documentation updated:
  - `PROMPT_CHECKLIST.md`
  - `task_plan.md`
  - `progress.md`
  - `findings.md`
  - `DESIGN_SYSTEM.md`

## 18. 2026-06-20 Table-Like Grid Row Elastic Widths

- Master checklist phase: 68.
- Main file changed:
  - `frontend/src/styles.css`
- Finding:
  - The current frontend has no native `<table>` elements.
  - Table-like density is implemented with grid rows such as detail rows, notification rows, audit rows, backup rows, sync result rows, balance snapshot rows, and check-in log rows.
- CSS guard added:
  - row-like grids get `min-width: 0` and `max-width: 100%`
  - direct children get `min-width: 0`
  - long strings get `overflow-wrap: anywhere`
  - mobile rows collapse to one column
- Validation passed:
  - `npm run build`
- Documentation updated:
  - `PROMPT_CHECKLIST.md`
  - `task_plan.md`
  - `progress.md`
  - `findings.md`
  - `DESIGN_SYSTEM.md`

## 19. 2026-06-20 Global Keyframes Consolidation

- Master checklist phase: 69.
- Main file changed:
  - `frontend/src/styles.css`
- Keyframes state:
  - `panel-in` and `skeletonShimmer` are now colocated in the global motion/keyframes area near the top of the CSS.
  - Existing animation names and call sites were preserved.
  - `prefers-reduced-motion` still disables skeleton shimmer.
- Validation passed:
  - `npm run build`
  - `Select-String` confirmed the remaining `@keyframes` definitions are the global `panel-in` and `skeletonShimmer` definitions.
- Documentation updated:
  - `PROMPT_CHECKLIST.md`
  - `task_plan.md`
  - `progress.md`
  - `findings.md`
  - `DESIGN_SYSTEM.md`

## 20. 2026-06-21 Non-Emoji Linear Icons And Status Text

- Master checklist phase: 70.
- Main files changed:
  - `frontend/src/main.tsx`
  - `frontend/src/styles.css`
  - `PROMPT_CHECKLIST.md`
  - `task_plan.md`
  - `progress.md`
  - `findings.md`
  - `DESIGN_SYSTEM.md`
- Finding:
  - The current formal frontend did not have obvious emoji icon remnants.
  - `frontend/package.json` does not include `lucide-react`.
- Decision:
  - Do not add an icon dependency for this single visual checklist item.
  - Use a lightweight inline `LineIcon` SVG component with a lucide-like line style.
- UI changes:
  - Sidebar navigation now uses object line icons instead of `OV/CH/...` letter abbreviations.
  - Dashboard health, diagnostics, and task-center status badges now render `StatusLabel`: line icon plus visible Chinese status text.
- Validation passed:
  - `npm run build`

## 21. 2026-06-21 Status Cues Beyond Color

- Master checklist phase: 71.
- Main files changed:
  - `frontend/src/main.tsx`
  - `frontend/src/styles.css`
  - `PROMPT_CHECKLIST.md`
  - `task_plan.md`
  - `progress.md`
  - `findings.md`
  - `DESIGN_SYSTEM.md`
- UI changes:
  - Extended `statusIconName` so active/valid/scheduled/enabled, missing/archived/manual-required, and failed/expired/unreachable states get stable line-icon semantics.
  - Channel source status, account login status, scheduler jobs, audit levels, sync summaries, sync instance results, settings state pills, and proxy test results now use `StatusLabel` or explicit success/failure text.
  - Added small CSS alignment support for status icons inside pills and compact rows.
- Validation passed:
  - `npm run build`
- Scope note:
  - This was frontend UI/documentation only. Go backend tests were not rerun because no backend logic, database, credential, or external request path changed.
- Suggested next slice:
  - Continue T4.4 with “important numbers position and tabular numerals” inspection.

## 22. 2026-06-21 Important Numbers And Tabular Numerals

- Master checklist phase: 72.
- Main files changed:
  - `frontend/src/styles.css`
  - `PROMPT_CHECKLIST.md`
  - `task_plan.md`
  - `progress.md`
  - `findings.md`
  - `DESIGN_SYSTEM.md`
- UI changes:
  - Added a centralized `Numeric scan pass` CSS layer.
  - Important operational numbers now consistently use `font-variant-numeric: tabular-nums` and `font-feature-settings: "tnum" 1, "lnum" 1`.
  - Coverage includes Dashboard, Hub Radar, channel/account metrics, check-in and notification counts, balances, detail metrics, sync results, and scheduler/status rows.
  - High-priority number surfaces get slightly tighter letter spacing and start alignment for faster scanning.
- Validation passed:
  - `npm run build`
- Scope note:
  - CSS/documentation only. Go backend tests were not rerun because no backend logic, database, credential, or external request path changed.
- Suggested next slice:
  - Continue with the next unchecked prompt item, likely T4.1 theme system or T4.2 design-system consolidation, unless prioritizing functional P1/P2 items first.

## 23. 2026-06-21 Tailwind/shadcn Design-System Convergence

- Master checklist phase: 73.
- Main files changed:
  - `frontend/src/styles.css`
  - `DESIGN_SYSTEM.md`
  - `PROMPT_CHECKLIST.md`
  - `task_plan.md`
  - `progress.md`
  - `findings.md`
- Decision:
  - Keep Tailwind v4 as the build-time CSS import/compiler layer.
  - Do not add Radix/shadcn runtime dependencies unless explicitly approved.
  - Treat `frontend/src/components/ui/*` as local project-owned primitives, not an installed shadcn system.
- Cleanup:
  - Renamed CSS comments from `shadcn-inspired` / `shadcn/Linear` to Control Room / Linear control-room wording.
- Validation passed:
  - `npm run build`
- Scope note:
  - CSS comments/documentation only. Go backend tests were not rerun because no backend logic, database, credential, or external request path changed.
- Suggested next slice:
  - Continue T4.2 token consolidation: color/radius/shadow/spacing/font-size single-source cleanup, or extract shared `<StatCard>` if staying in UI architecture work.

## 24. 2026-06-21 V4 Token Foundation And Tailwind Bridge

- Master checklist phase: 74.
- Main files changed:
  - `frontend/src/styles.css`
  - `DESIGN_SYSTEM.md`
  - `PROMPT_CHECKLIST.md`
  - `task_plan.md`
  - `progress.md`
  - `findings.md`
- UI/system changes:
  - Tailwind `@theme` now bridges to active V4 tokens for background, foreground, card, primary, destructive, border, input, ring, radius, and card shadow values.
  - V4 `:root` now includes semantic color extensions, status backgrounds/borders, input/focus tokens, skeleton tokens, type scale, font weights, tracking, spacing, radius scale, and shadow scale.
  - First active-layer replacements moved sidebar, page brief, metric, status pill, action dock, toolbar, and related V4 hard-coded values toward token references.
- Scope note:
  - This is a first-pass foundation. Do not mark the full color/radius/shadow/spacing/font-size single-source checklist items complete yet; legacy `--rc-*`, `--linear-*`, and early CSS layers still need follow-up cleanup.
- Validation passed:
  - `npm run build`
- Suggested next slice:
  - Continue T4.2 by replacing remaining active V4 hard-coded radii, type sizes, shadows, and status colors, then decide whether to safely delete old overridden CSS layers.

## 25. 2026-06-21 Active V4 Token Sweep Second Pass

- Master checklist phase: 75.
- Main files changed:
  - `frontend/src/styles.css`
  - `PROMPT_CHECKLIST.md`
  - `task_plan.md`
  - `progress.md`
  - `findings.md`
  - `AGENT_HANDOFF.md`
- UI/system changes:
  - Added amber text semantic tokens for warning/error surfaces.
  - Moved navigation active text, mobile density overrides, global error bar, fatal error card, and JSON preview toward V4 token references.
  - This is still not a full single-source cleanup; older CSS layers continue to contain hard-coded values and historical token namespaces.
- Validation passed:
  - `npm run build`
- Suggested next slice:
  - Continue scanning active V4 and dashboard layers for hard-coded type/radius/shadow values, then plan a safe CSS layer deletion/reduction pass.

## 26. 2026-06-21 relaycheck-hub SQLite Reliability Baseline

- Master checklist phase: 76.
- Scope:
  - Experimental `E:\zidqiandao\relaycheck-hub`, not the official desktop runtime.
- Main files changed:
  - `..\relaycheck-hub\src\lib\sqlite-tuning.ts`
  - `..\relaycheck-hub\src\lib\sqlite-init.ts`
  - `..\relaycheck-hub\src\lib\prisma.ts`
  - `..\relaycheck-hub\src\lib\local-newapi.ts`
  - `..\relaycheck-hub\scripts\verify-sqlite-tuning.mjs`
  - `..\relaycheck-hub\package.json`
  - `..\relaycheck-hub\README.md`
  - `PROMPT_CHECKLIST.md`
  - `task_plan.md`
  - `progress.md`
  - `findings.md`
- Runtime changes:
  - Hub main SQLite DB now centralizes tuning in `sqlite-tuning.ts`.
  - Applied pragmas: WAL, `busy_timeout=5000`, `synchronous=NORMAL`, `temp_store=MEMORY`, `cache_size=-20000`, and `foreign_keys=ON`.
  - Prisma better-sqlite3 adapter now explicitly passes `timeout: 5000` and still uses the global singleton client.
  - External NewAPI SQLite import remains read-only and only inherits 5000ms timeout; it does not force WAL or other pragmas onto user-provided databases.
- Validation command added:
  - `cd E:\zidqiandao\relaycheck-hub; npm run verify:sqlite`
- Note:
  - Current PowerShell lacked `tar`; package API was verified via temporary `npm install --ignore-scripts` outside the repo.
- Validation now passed:
  - `npm run verify:sqlite`
  - `npx prisma validate`
  - `npx prisma generate`
  - `npm run build`
- Additional build fixes:
  - Added `relaycheck-hub/.npmrc` with `package-lock=true`.
  - Reinstalled damaged `caniuse-lite` package after Next build could not find `data/agents.js`.
  - Added local Prisma enum string-union types in `relaycheck-hub/src/lib/prisma-enums.ts`.
  - Added page-local structure types to satisfy strict TypeScript checks without lowering strictness.

## 27. 2026-06-21 Target Prompt Checklist And Frontend UI Primitives

- The root `E:\zidqiandao\目标` folder contains additional prompt files:
  - `COMPONENT_ARCHITECTURE_PROMPT.md`
  - `UI_UX_BEAUTIFICATION_PROMPT.md`
- Added `E:\zidqiandao\目标\TARGET_PROMPT_CHECKLIST.md` as the execution checklist for those prompts.
- Completed target checklist items:
  - `cn.ts` now uses `clsx + tailwind-merge`.
  - `Button`, `Card`, and `Badge` import `cn()` through `@/lib/cn`.
  - `frontend/tsconfig.json` and `frontend/vite.config.ts` support `@/*` alias.
  - Added high-priority primitives: `Input`, `Select`, `Skeleton`, `Dialog`.
- Dependencies added to `relaycheck-desktop/frontend`:
  - `clsx`
  - `tailwind-merge`
- Documentation updated:
  - `DESIGN_SYSTEM.md` now records local UI primitives, `cn()` behavior, and `@/*` alias usage.
  - `PROMPT_CHECKLIST.md`, `task_plan.md`, `progress.md`, and `findings.md` updated for phase 77.
- Validation passed:
  - `cd E:\zidqiandao\relaycheck-desktop\frontend; npm run build`
- Suggested next slice:
  - A2 is now complete.
  - Start A3 with type/API/format/labels extraction before attempting page-level splits.

## 28. 2026-06-21 Target Prompt A2 UI Primitive Completion

- Master checklist phase: 78.
- Added remaining UI primitives:
  - `frontend/src/components/ui/progress.tsx`
  - `frontend/src/components/ui/tooltip.tsx`
  - `frontend/src/components/ui/switch.tsx`
- `Progress` uses `role="progressbar"` and clamps `value/max`.
- `Tooltip` is CSS-only and exposes hover/focus helper text without adding dependencies.
- `Switch` uses a native button with `role="switch"` and `aria-checked`.
- Updated:
  - `E:\zidqiandao\目标\TARGET_PROMPT_CHECKLIST.md`
  - `PROMPT_CHECKLIST.md`
  - `task_plan.md`
  - `progress.md`
  - `findings.md`
  - `DESIGN_SYSTEM.md`
- Validation passed:
  - `cd E:\zidqiandao\relaycheck-desktop\frontend; npm run build`
- Suggested next slice:
  - Begin `目标/TARGET_PROMPT_CHECKLIST.md` A3 with `types/index.ts` extraction, then `api/client.ts`.

## 29. 2026-06-21 Target Prompt A3 Type Extraction

- Master checklist phase: 79.
- Completed target checklist item:
  - `目标/TARGET_PROMPT_CHECKLIST.md` A3: 类型提取到 `types/index.ts`.
- Added:
  - `frontend/src/types/index.ts`
- Updated:
  - `frontend/src/main.tsx`
  - `E:\zidqiandao\目标\TARGET_PROMPT_CHECKLIST.md`
  - `PROMPT_CHECKLIST.md`
  - `task_plan.md`
  - `progress.md`
  - `findings.md`
- Notes:
  - `frontend/src/types/index.ts` now centralizes 65 frontend types for DTOs, navigation, API results/errors, and icon names.
  - `main.tsx` imports these with `import type { ... } from "@/types"`.
  - `TabKey` is now an explicit union; `navItems` uses `satisfies readonly NavItem[]` so runtime nav data is still checked against the extracted type contract.
  - No business logic, API behavior, rendering structure, or styles were intentionally changed.
- Validation passed:
  - `cd E:\zidqiandao\relaycheck-desktop\frontend; npm run build`
- Build note:
  - First build caught a missing `SyncRunItem` import; after adding the type import, the build passed.
- Suggested next slice:
  - Continue A3 with `api/client.ts` extraction. Move `ApiError`, `api()`, client read cache, `subscribeApiErrors`, and related API error publishing together, then run `npm run build`.

## 30. 2026-06-21 Target Prompt A3 API Client Extraction

- Master checklist phase: 80.
- Completed target checklist item:
  - `目标/TARGET_PROMPT_CHECKLIST.md` A3: API client 提取到 `api/client.ts`.
- Added:
  - `frontend/src/api/client.ts`
- Updated:
  - `frontend/src/main.tsx`
  - `E:\zidqiandao\目标\TARGET_PROMPT_CHECKLIST.md`
  - `PROMPT_CHECKLIST.md`
  - `task_plan.md`
  - `progress.md`
  - `findings.md`
- Notes:
  - `frontend/src/api/client.ts` now owns `ApiError`, `api()`, read caching, global API error listeners, and error publishing.
  - `main.tsx` imports `api` and `subscribeApiErrors` from `@/api/client`.
  - Preserved behavior: 1500ms GET cache TTL, uncached `/api/checkins/status` and `/api/auth/session`, cache clear after non-GET requests, `credentials: "same-origin"`, and `GlobalApiError` event shape.
  - `main.tsx` no longer contains local `ApiError`, `api()`, `clientReadCache`, `ApiResult`, or `ClientReadCacheEntry` references.
- Validation passed:
  - `cd E:\zidqiandao\relaycheck-desktop\frontend; npm run build`
- Suggested next slice:
  - Continue A3 with `lib/format.ts` extraction. Good first candidates are `formatTime`, `formatBuildTime`, `formatDuration`, `formatDurationShort`, `formatBytes`, `formatBalanceValue`, `formatBalanceMeta`, `formatBalanceGroup`, `formatCompactNumber`, `formatUSD`, `formatDecimal`, and `trimDecimal`.

## 31. 2026-06-21 Target Prompt A3 Format Extraction

- Master checklist phase: 81.
- Completed target checklist item:
  - `目标/TARGET_PROMPT_CHECKLIST.md` A3: format 工具提取到 `lib/format.ts`.
- Added:
  - `frontend/src/lib/format.ts`
- Updated:
  - `frontend/src/main.tsx`
  - `E:\zidqiandao\目标\TARGET_PROMPT_CHECKLIST.md`
  - `PROMPT_CHECKLIST.md`
  - `task_plan.md`
  - `progress.md`
  - `findings.md`
- Notes:
  - `frontend/src/lib/format.ts` now exports time, duration, byte, confidence, JSON preview, balance, compact number, USD, decimal, pricing source, and price comparison formatting helpers.
  - `main.tsx` imports these helpers from `@/lib/format`.
  - `formatAPIKeyTestMessage` intentionally remains in `main.tsx` because it depends on `apiKeyStatusLabel`; move it after `lib/labels.ts` exists.
- Validation passed:
  - `cd E:\zidqiandao\relaycheck-desktop\frontend; npm run build`
- Suggested next slice:
  - Continue A3 with `lib/labels.ts` extraction. Good candidates are `errorClassLabel`, `diagnosticLevelLabel`, `channelSourceLabel`, `channelSourceSyncLabel`, `localNewAPISourceLabel`, `syncCapabilityLabel`, `syncSourceLabel`, `syncActionLabel`, `syncSummaryScopeLabel`, `upstreamKindLabel`, `channelStatusLabel`, `channelModelStatusLabel`, `auditActionLabel`, `auditLevelLabel`, `schedulerStatusLabel`, `statusLabel`, `loginStatusLabel`, `apiKeyStatusLabel`, `usageTrendLabel`, `priceLevelLabel`, `priceLevelShort`, `pricingCacheStatusLabel`, and then `formatAPIKeyTestMessage`.

## 32. 2026-06-21 Target Prompt A3 Labels Extraction

- Master checklist phase: 82.
- Completed target checklist item:
  - `目标/TARGET_PROMPT_CHECKLIST.md` A3: labels 工具提取到 `lib/labels.ts`.
- Added:
  - `frontend/src/lib/labels.ts`
- Updated:
  - `frontend/src/main.tsx`
  - `E:\zidqiandao\目标\TARGET_PROMPT_CHECKLIST.md`
  - `PROMPT_CHECKLIST.md`
  - `task_plan.md`
  - `progress.md`
  - `findings.md`
- Notes:
  - `frontend/src/lib/labels.ts` now exports pure label helpers for API errors, diagnostics, channel/source status, NewAPI sync, upstream kinds, audit, scheduler, check-in status, login status, API key status, usage trends, pricing levels, pricing cache badges, and `formatAPIKeyTestMessage`.
  - `main.tsx` imports these helpers from `@/lib/labels`.
  - Behavior/navigation helpers intentionally remain in `main.tsx`: `diagnosticNavigationIntent`, `actionNavigationIntent`, `actionCenterQuickActions`, plus richer business explanation helpers such as `checkinCapabilityLabel` and `signalLabel`.
  - No business logic, API behavior, rendering structure, or styles were intentionally changed.
- Validation passed:
  - `cd E:\zidqiandao\relaycheck-desktop\frontend; npm run build`
  - `rg` confirmed the extracted target label function definitions now live in `frontend/src/lib/labels.ts`, not `main.tsx`.
- Suggested next slice:
  - Continue A3 with `lib/constants.ts` extraction. Good candidates include navigation metadata and shared static sets/configs, but keep behavior helpers and page state inside `main.tsx` until the page split begins.

## 33. 2026-06-21 Target Prompt A3 Constants Extraction

- Master checklist phase: 83.
- Completed target checklist item:
  - `目标/TARGET_PROMPT_CHECKLIST.md` A3: constants 提取到 `lib/constants.ts`.
- Added:
  - `frontend/src/lib/constants.ts`
- Updated:
  - `frontend/src/main.tsx`
  - `E:\zidqiandao\目标\TARGET_PROMPT_CHECKLIST.md`
  - `PROMPT_CHECKLIST.md`
  - `task_plan.md`
  - `progress.md`
  - `findings.md`
- Notes:
  - `frontend/src/lib/constants.ts` now exports `NAV_ITEMS`, status icon level sets, relay kind sets, problem/success status sets, raw channel search keys, important notification levels/keywords, dialog focus selector, load-more limits, and `API_KEY_STALE_MS`.
  - `main.tsx` imports these constants from `@/lib/constants`.
  - Page state, API flows, navigation intent helpers, Action Center quick actions, and explanation helpers still stay in `main.tsx` for now.
  - No business logic, API behavior, rendering structure, or styles were intentionally changed.
- Validation passed:
  - `cd E:\zidqiandao\relaycheck-desktop\frontend; npm run build`
  - `Select-String` confirmed old local constants/magic numbers no longer remain in `main.tsx`.
- Suggested next slice:
  - Start page-level extraction. The safest first candidates are `HubRadar` or `Dashboard`, but extract one component/page at a time and run `npm run build` after each slice.

## 34. 2026-06-21 Current Stop Point

- Current task-plan phase: 84 (preparing `HubRadar` extraction).
- Completed before interruption:
  - Phase 83 is complete and recorded.
  - `目标/TARGET_PROMPT_CHECKLIST.md` has `constants 提取到 lib/constants.ts` checked.
  - `frontend/src/lib/constants.ts` exists and the frontend build passed after it.
- Started but not completed:
  - Created empty directory `frontend/src/components/dashboard`.
  - Did not create `HubRadar.tsx`.
  - Did not move `HubRadar` out of `main.tsx`.
  - Did not check `目标/TARGET_PROMPT_CHECKLIST.md` item `HubRadar 拆分`.
- Next safe action:
  - Extract only `HubRadar` into `frontend/src/components/dashboard/HubRadar.tsx`.
  - Keep `Dashboard` and other pages in `main.tsx` for that slice.
  - Preserve navigation behavior by either passing `actionNavigationIntent` as a prop or exporting/importing a helper with no behavior change.
  - Run `cd E:\zidqiandao\relaycheck-desktop\frontend; npm run build`.
