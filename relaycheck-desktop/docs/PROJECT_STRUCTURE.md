# RelayCheck Desktop Project Structure

Last updated: 2026-06-30

## Active Source

| Path | Purpose |
|------|---------|
| `main.go` | Desktop entry, embeds `frontend/dist`, starts the local HTTP server. |
| `internal/core/` | Assembly root: `App` struct, HTTP handlers, routes, SQLite, scheduler, audit, crypto, network, URL safety, checkin/balance execution, system settings, analytics, diagnostics, and forwarding methods to all domain packages. See `internal/core/PACKAGE_INDEX.md` for the file-by-file map. |
| `internal/notifications/` | Notification hub + 6 channel implementations (webhook/telegram/bark/serverchan/email/desktop). |
| `internal/backup/` | Encrypted zip export/import (PBKDF2-SHA256 + RCZIP2/RCZIP1). |
| `internal/versioncheck/` | Remote version manifest check + semver compare. |
| `internal/legacycheck/` | Legacy Python code detection. |
| `internal/autostart/` | OS auto-start shortcut management (Windows + non-Windows stub). |
| `internal/sites/` | Upstream site CRUD, detection (headers/HTML/API), local network scanner. |
| `internal/channels/` | Channel CRUD, model sync, health overview, schedules, pricing, model overview. |
| `internal/accounts/` | Account CRUD + 5 import paths (Chrome password, admin API, SQLite, legacy config, local NewAPI) + sync preview + auto-detect. |
| `internal/lock/` | Single-instance lock implementation for Windows and Unix. |
| `frontend/src/main.tsx` | React application shell and page orchestration. |
| `frontend/src/components/` | Domain panels (dashboard, channels, sites, accounts, checkins, notifications, settings, onboarding) and shared UI primitives (ThemeToggle, UpdateBanner, TwoFactorGuide, AnalyticsPanel, Empty). |
| `frontend/src/api/`, `frontend/src/hooks/`, `frontend/src/lib/`, `frontend/src/types/` | Frontend client, hooks (useTaskProgress/useChannelActions), formatting/labels/constants/theme, and shared TypeScript types. |
| `frontend/scripts/smoke.mjs` | Browser smoke test for desktop and mobile surfaces. |
| `vendor/` | Vendored Go dependencies used by `go test -mod=vendor` and `go build -mod=vendor`. |

## Architecture Notes

- **No login layer:** The app runs as a local single-user tool. `requireSession`/`withSession` are passthrough middlewares kept for route registration uniformity. The `app_users` table, bcrypt import, and auth routes have been removed.
- **Unified task engine:** `internal/core/task_runner.go` drives all batch operations (checkin, test_keys, refresh_balances, detect_sites) with SSE streaming progress via `/api/tasks/`.
- **Theme system:** Three-state toggle (system/light/dark) via `frontend/src/lib/theme.ts`, persisted in localStorage, applied via `html.dark` class.
- **Onboarding wizard:** 4-step first-run guide in `frontend/src/components/onboarding/`, controlled by localStorage flag.
- **Analytics engine:** `internal/core/analytics.go` provides balance trend, checkin distribution, response times, site reliability, and balance deltas via `/api/analytics?days=N`. Frontend uses pure SVG charts (no chart library) with drilldown support.
- **Encrypted export/import:** `internal/backup/` implements AES-256-GCM encrypted zip with PBKDF2-SHA256 key derivation (200,000 iterations + 32-byte salt). RCZIP2 format; RCZIP1 supported for backward-compatible decryption. Zip-bomb protection caps total decompressed at 256 MB. `internal/core/backup_zip.go` is a thin forwarder.
- **Per-channel scheduling:** `internal/core/channel_schedules.go` allows per-site checkin time and random delay configuration. Calendar preview and next-runs list available via API. Uses `time.FixedZone("CST", 8*3600)` for timezone consistency.
- **Notification channels:** `internal/notifications/` implements Webhook (with HMAC + exponential backoff retry), Telegram, Bark, ServerChan, Email (SMTP), and Desktop (in-app + browser Notification API push). `levelMatchesMode` supports `all`/`failure`/`success`/`warning+` modes. `internal/core/notification.go` is a thin forwarder.
- **Detection engine:** `internal/sites/detection.go` identifies site kind (newapi/oneapi/sub2api) from HTTP headers, HTML content, and API responses with confidence scoring. `internal/core/detection_detail.go` does cross-domain aggregation.
- **Dry-run preview:** `internal/core/dry_run.go` previews batch operations without executing them, with a 200-account limit and single batch query.
- **Auto-start:** `internal/autostart/` creates Windows shell:startup shortcuts via PowerShell COM. `internal/core/autostart.go` is a thin forwarder.
- **Cookie expiry tracking:** ChannelAccount model tracks `cookie_expiry_at` and `storage_state_expiry_at`; diagnostics and Action Center surface upcoming expirations.
- **Architecture evolution (June 2026):** Two-phase decomposition of the `*App` god object in `internal/core/app.go`. **Phase 1** (commits `8fc1975`..`1444e43`) extracted 11 service/store types within `core` (CryptoService, AccountAuthRepository, CheckinRunStore, NotificationHub, SyncJobRunStore, SchedulerRepo, ReadCacheStore, BrowserSessionStore, NetworkProxyStore + SharedInfra interface), each with its own mutex. **Phase 2** (commits `2a32506`..`e80578c`) split 8 domain packages out of `core` (notifications, backup, versioncheck, legacycheck, autostart, sites, channels, accounts) using the Infra-interface + mirror-type + `*App`-forwarder pattern. Dependency direction is one-way: `core` → domain. See `CLAUDE.md` and `internal/core/PACKAGE_INDEX.md` for details. Design spec: `docs/superpowers/specs/2026-06-29-architecture-evolution-design.md`.

## Active Documents

| Path | Purpose |
|------|---------|
| `README.md` | Main product, runtime, command, and safety overview. |
| `DESIGN_SYSTEM.md` | Control Room UI direction and visual rules. |
| `CLAUDE.md` | AI agent / Claude Code onboarding: architecture, verification, conventions. |
| `docs/manual-test-record.md` | Manual test record with before/after comparisons. |
| `docs/reports/` | Dated progress reports and recovery records. |

## Local And Generated Paths

| Path | Treatment |
|------|-----------|
| `data/` | Real local runtime data. Do not delete or mutate without backup and rollback steps. |
| `frontend/dist/` | Generated by `npm run build`; required by `//go:embed frontend/dist` when compiling Go. |
| `dist/` | Generated desktop binaries. Keep `relaycheck.exe` for local launching; old variant binaries can be removed. |
| `frontend/node_modules/` | Local dependency install, ignored by Git. |
| `frontend/src/hooks/` | Custom React hooks: `useAppData.ts` (data loading + state), `useChannelFilters.ts`, `useTaskProgress.ts`. |
| `frontend/src/components/layout/` | Layout components: `Sidebar.tsx`, `Topbar.tsx` (extracted from main.tsx). |

## Archived Workspace Material

The old Python implementation, old standalone Vite frontend, Next.js experiment, and superseded workspace notes were moved to:

```text
E:\zidqiandao\_archive\2026-06-24-workspace-cleanup
```

This archive is not active source. Restore files from it only when a task explicitly targets legacy migration or historical comparison.

## Verification Order

Run from `E:\zidqiandao\relaycheck-desktop`:

```powershell
cd frontend
npm run build
npx tsc --noEmit
cd ..
go vet -mod=vendor ./...
go test -mod=vendor ./... -count=1 -timeout 120s
go build -mod=vendor -ldflags="-H windowsgui" -o dist\relaycheck.exe .
```

If browser smoke is needed, start the desktop server first, set `RELAYCHECK_SMOKE_PASSWORD`, then run:

```powershell
cd frontend
npm run smoke
```
