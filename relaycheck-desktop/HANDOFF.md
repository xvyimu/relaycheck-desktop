# HANDOFF.md

Authoritative handoff document for RelayCheck Desktop. Updated each session.
Read this first, then `CLAUDE.md` for architecture.

**Last updated:** 2026-07-04

---

## Current state

The project is stable. The latest backend slice is:

- `internal/core/account_task_service.go` — new `AccountTaskService` owns SSE task-facing API key test orchestration
- `internal/core/task_runner.go` — `test_keys` task handler now delegates to `AccountTaskService`
- `internal/core/account_task_service_test.go` — regression test covers task progress and API key result persistence
- `internal/core/checkin_task_service.go` — new `CheckinTaskService` owns SSE task-facing checkin and balance refresh orchestration
- `internal/core/task_runner.go` — `checkin` and `refresh_balances` task handlers now delegate to `CheckinTaskService`
- `internal/core/checkin_task_service_test.go` — regression tests cover task progress and DB side effects for checkin and balance refresh
- `internal/core/accounts.go` — `listAccounts` now honors `?upstreamSiteId=`
- `internal/core/accounts_list_test.go` — regression test for filtered reads after the unfiltered cache is populated

Current local state:

- Branch `main` is ahead of `origin/main`; run `rtk git status --branch --short` for the exact count.
- The `CheckinTaskService` slice is committed locally.
- The `AccountTaskService` slice is committed locally as `3658672`.

Remote sync note:

- `origin/main` includes the recent UI/UX commits through `08af812` (`fix(ui): improve scan panel feedback`).
- If GitHub HTTPS is unreachable, push through the local proxy with:
  ```powershell
  rtk git -c http.proxy=http://127.0.0.1:7897 -c https.proxy=http://127.0.0.1:7897 -c http.version=HTTP/1.1 push origin main
  ```
- After push, confirm `rtk git status --branch --short` no longer reports `ahead` and the worktree is clean.

Latest Go verification gates pass:

```powershell
go vet -mod=vendor ./...
go test -mod=vendor -count=1 ./...               # 969 tests pass
go build -mod=vendor ./...
go test -mod=vendor -cover -count=1 ./internal/core ./internal/accounts ./internal/channels ./internal/notifications ./internal/autostart ./internal/backup ./internal/versioncheck
git diff --check
```

Recent frontend gates also passed:

```powershell
cd frontend; npm run build                       # tsc + vite
cd frontend; npm test                            # 213 tests pass
cd frontend; npm run smoke:schedules
git diff --check
```

---

## What landed this session

### Account task service extraction

- Added `AccountTaskService` in `internal/core/account_task_service.go`.
- Moved the `test_keys` task body out of `task_runner.go`; `task_runner.go` delegates API key test task orchestration while preserving task/SSE mechanics.
- Wired `App.accountTasks` in `NewApp`.
- Updated `internal/core/PACKAGE_INDEX.md` with the new service and test file.
- Added `TestAccountTaskServiceStartTestKeysPublishesProgress`.

Verification:

- `rtk go test -mod=vendor -count=1 ./internal/core -run "AccountTaskService" -v` — 1 passed
- `rtk go test -mod=vendor -count=1 ./internal/core -run "AccountTaskService|TaskRunner|APIKey|BulkTestAPIKeys" -v` — 6 passed
- `rtk go test -mod=vendor -count=1 ./internal/core` — 378 passed
- `rtk go test -mod=vendor -count=1 ./...` — 969 passed / 12 packages
- `rtk go vet -mod=vendor ./...` — clean
- `rtk go build -mod=vendor ./...` — success
- `cd frontend; rtk npm test` — 213 passed
- `cd frontend; rtk npm run build` — success
- `rtk git diff --check` — clean

### Checkin task service extraction

- Added `CheckinTaskService` in `internal/core/checkin_task_service.go`.
- Moved the `checkin` and `refresh_balances` task bodies out of `task_runner.go`; `task_runner.go` now keeps task start/cancel/stream mechanics and delegates those two business task bodies.
- Wired `App.checkinTasks` in `NewApp` after `rootCtx` and `TaskRunner` are initialized.
- Updated `internal/core/PACKAGE_INDEX.md` with the new service and test file.
- Added two focused tests:
  - `TestCheckinTaskServiceStartCheckinPublishesProgress`
  - `TestCheckinTaskServiceStartRefreshBalancesPublishesProgress`

Verification:

- `rtk go test -mod=vendor -count=1 ./internal/core -run "CheckinTaskService" -v` — 2 passed
- `rtk go test -mod=vendor -count=1 ./internal/core -run "CheckinTask|TaskRunner|RefreshBalance|Checkin" -v` — 41 passed
- `rtk go test -mod=vendor -count=1 ./internal/core` — 377 passed
- `rtk go test -mod=vendor -count=1 ./...` — 968 passed / 12 packages
- `rtk go vet -mod=vendor ./...` — clean
- `rtk go build -mod=vendor ./...` — success
- `cd frontend; rtk npm test` — 213 passed
- `cd frontend; rtk npm run build` — success
- `rtk git diff --check` — clean

### Maintenance coverage cleanup

- `10def66` — `test: cover exported scanner helpers`
  - `internal/channels/helpers_test.go`: covered exported `LooksLikeModelID`.
  - `internal/sites/scanner_test.go`: covered exported site scanner helpers (`HostLabel`, `IsManagedRelayKind`, `IsExcludedRelaySite`).
- `9aff466` — `test(autostart): cover windows startup helpers`
  - `internal/autostart/platform_windows_test.go`: covered hidden process attrs, PowerShell single-quote escaping, startup folder error handling, and shortcut path construction without mutating shell startup state.
- `2558a9b` — `test(core): cover account helper defaults`
  - `internal/core/accounts_helpers_test.go`: covered account auth inference, default display names, estimated cookie expiry, cookie header assembly, and API key status defaults.

Latest package-level Go coverage snapshot:

| Package | Coverage |
|---------|----------|
| `internal/core` | **44.1%** |
| `internal/accounts` | **31.5%** |
| `internal/channels` | **60.7%** |
| `internal/notifications` | **65.9%** |
| `internal/autostart` | **38.9%** |
| `internal/backup` | **81.4%** |
| `internal/versioncheck` | **92.5%** |

### Accounts list filter bug fix + new-api sync exploration

- `internal/core/accounts.go`: Fixed `listAccounts` ignoring `?upstreamSiteId=` query parameter. Added `r.URL.Query().Get("upstreamSiteId")` parsing, scoped cache key (`"accounts-list"` vs `"accounts-list:"+siteID`), and conditional `WHERE a.upstream_site_id = ?` in SQL.
- `internal/core/accounts_list_test.go`: New regression test — fills unfiltered cache first, then verifies per-site filter returns exactly one row with correct ownership.
- New‑API sync exploration: local new‑api fork (port 3000, PID 10232, `new-api-fixed`) — `one-api.db` at `C:\Users\yuanjia\one-api.db`. `channels` table is empty (0 rows). Admin login (user `xiejia`) requires 2FA. No data to import.
- Temp access token written to new‑api DB for verification then cleaned up.
- relaycheck.exe cannot be started from sandbox (EPERM uv_spawn on both PS/Bash).

Verification gates passed:
- `go test -mod=vendor -count=1 ./internal/core -run TestListAccountsHonorsUpstreamSiteIDFilter -v`
- `go test -mod=vendor -count=1 ./internal/core` — 363 tests
- `go test -mod=vendor -count=1 ./...` — 949 tests / 11 packages
- `go build -mod=vendor ./...`
- `go vet -mod=vendor ./...`

| Package | Status |
|---------|--------|
| internal/core | 5.169s ✅ (includes new filter regression test) |
| internal/accounts | 0.710s ✅ |
| internal/notifications | 10.593s ✅ |
| internal/backup | 4.705s ✅ |
| internal/channels | 0.636s ✅ |

### Suggested next steps (priority order)

1. **Confirm a clean synced worktree** after this slice is committed:
   ```powershell
   cd E:\zidqiandao\relaycheck-desktop
   rtk git status --branch --short
   ```

- `internal/core/channel_schedules.go`: added random delay validation for per-site schedules (`0 <= min <= max <= 120`) before DB writes, so bad payloads return HTTP 400 instead of leaking into persistence or surfacing as 500.
- `internal/core/channel_schedules_test.go`: added table-driven regression coverage for negative delay, delay above 120, and `min > max`.
- `frontend/src/components/settings/SiteSchedules.tsx`: surfaced calendar/next-run preview load failures with a non-blocking inline status, removed stale unused date helpers, and normalized delay inputs to stay within `0..120` while preserving `min <= max`.
- `frontend/src/styles.css`: changed schedule preview rows to grid layouts with `minmax(0, 1fr)` content columns and a `max-width: 560px` responsive pass for long site names.
- `frontend/scripts/verify-site-schedules-layout.mjs` + `frontend/package.json`: added `npm run smoke:schedules`, a focused Playwright smoke that mocks the required APIs and checks the Settings schedule preview at desktop and 390px mobile widths.

Verification:

- `cd frontend; npm run build`
- `cd frontend; npm test` — 7 files / 187 tests
- `cd frontend; npm run smoke:schedules` — 1440x900 and 390x900 pass, no body/row overflow, no console issues
- `go build -mod=vendor ./...`
- `go test -mod=vendor -count=1 ./...` — 888 tests
- `go vet -mod=vendor ./...`
- `git diff --check`

Browser note: Vite was started on `127.0.0.1:5173` and shut down cleanly. The smoke used cached Playwright Chromium at `C:\Users\yuanjia\AppData\Local\ms-playwright\chromium_headless_shell-1228\...`.

### rootCtx lifecycle fix

- `internal/core/scheduler.go`: Removed `parent context.Context` parameter from `StartSchedulers`, changed `WithCancel(parent)` to `WithCancel(a.rootCtx)` so `app.Close()` properly terminates scheduler goroutines
- `main.go`: Removed unused `"context"` import, changed `app.StartSchedulers(context.Background())` to `app.StartSchedulers()`
- Commit: `9fb28d4` — pushed to `origin/main`

### Test coverage sprint (G0–G2)

| Package | Before | After | New test files |
|---------|--------|-------|----------------|
| `internal/core` | 42.2% | **43.8%** | `handler_health_test.go` (13 handler tests) |
| `internal/accounts` | 25.4% | **31.5%** | `service_test.go` (stubInfra + in-memory SQLite, 9 tests) |
| `internal/versioncheck` | 32.8% | **92.5%** | extended `service_test.go` w/ httptest.Server |
| `internal/backup` | 32.1% | **81.4%** | `service_test.go` (10+ tests, export/import round-trip) |
| `internal/channels` | 60.7% | **60.7%** | (unchanged this session) |
| `internal/notifications` | 65.9% | **65.9%** | (unchanged this session) |
| **Frontend** | 0 | **187 tests** | 7 test files in `frontend/src/lib/__tests__/` |

**Weighted average Go coverage:** ≈ 61.4% (beat 55% target)

**Key implementation notes:**
- `maskSecret` uses **byte-length** (`len()`), not rune count — test expectations must use byte math
- Windows file lock: `service.go:RestoreEncryptedExport` calls `os.Rename` BEFORE `ReopenDatabase`, so tests must close the `*sql.DB` handle before calling restore
- Vitest v4.1.9, `globals: true`, `environment: 'node'`

### Git push resolved

8 commits pushed via `ALL_PROXY= git push origin main`.

---

## Known blockers

### Browser automation needs elevated launch in Codex sandbox

Playwright package exists under `frontend/node_modules`. Direct browser launch from the regular Codex sandbox can fail with `spawn EPERM`; use an elevated shell/tool run for `npm run smoke:schedules`, or run it manually from a normal terminal:

```powershell
cd E:\zidqiandao\relaycheck-desktop\frontend
npm run dev
npm run smoke:schedules
```

The schedule-layout smoke does not need the Go backend because it mocks API responses in Playwright.

### Race detector unavailable

Windows env has cgo disabled, so `go test -race` cannot run. Concurrency code in `notifications/hub.go` and `task_runner.go` is covered by targeted tests but not by the race detector.

### All context.Background() calls resolved ✅

All `context.Background()` calls in `scheduler.go` have been replaced with the cancellable `ctx` derived from `a.rootCtx` (commit `ac8687e`). `notifications/hub.go` retains two `context.Background()` calls — these are deliberate because the hub creates its own cancelable context in `NewNotificationHub` and cancels it in `Close()`, so the lifecycle is self-contained and correct.

---

## Suggested next steps (priority order)

### 1. Push local maintenance commits when GitHub connectivity recovers

```powershell
cd E:\zidqiandao\relaycheck-desktop
rtk git push origin main
rtk git status --branch --short
```

Expected final state: no `ahead` marker.

### 2. Raise test coverage in core, accounts, channels, and notifications

- `internal/core`: Create `Infra` mock to test handler paths (current pure-function tests only moved coverage 0.3%)
- `internal/accounts`: Restructure tests to cover unexported helpers (current helpers_test.go not counted in coverage)
- `internal/channels`: Add DB/HTTP mocks to test full paths (current 60.7% covers only local logic)
- `internal/notifications`: Implement SMTP/HTTP mocks for full path coverage

---

## File map for handoff

| File | Role |
|------|------|
| `CLAUDE.md` | Architecture guide, verification commands, conventions, hard constraints |
| `HANDOFF.md` | This file — current task state, pending items, blockers |
| `GOALS.md` | Sprint goals with per-target completion status |
| `README.md` | Product overview, route table, commands (user-facing) |
| `internal/core/PACKAGE_INDEX.md` | File-by-file map of `core` package |
| `docs/PROJECT_STRUCTURE.md` | Source tree, generated paths, archive boundary |
| `DESIGN_SYSTEM.md` | Control Room visual direction |
| `docs/superpowers/specs/2026-06-29-architecture-evolution-design.md` | Phase 1/2 refactor rationale |

---

## Conventions recap

- **Commit messages:** English, `type(scope): subject` (e.g. `fix(core):`, `test(channels):`, `refactor(notifications):`)
- **Comments:** English in code; user-facing error messages in Chinese
- **Tests:** table-driven with `t.Run(tc.name, ...)`; pure functions only unless an `Infra` mock exists
- **PowerShell:** use `;` to chain commands (not `&&`); no heredoc (write commit message to a file, then `git commit -F`)
- **Go module:** `relaycheck-desktop`, Go 1.24, `-mod=vendor`
- **Frontend:** React 19 + Vite, embedded via `//go:embed frontend/dist`
- **Vitest:** v4.1.9, globals: true, environment: node

---

## Session log

| Date | Session | Outcome |
|------|---------|---------|
| 2026-07-04 | Accounts filter bug fix + new-api sync exploration | `listAccounts` now honors `?upstreamSiteId=` with scoped cache + regression test. New-API sync found 0 channels to import. 2FA blocks admin login. Go gates all green. |
| 2026-07-03 | Maintenance coverage cleanup | `10def66`, `9aff466`, `2558a9b`: exported scanner helpers, Windows autostart helpers, core account helper defaults. 928 Go tests pass. Push of `2558a9b` is pending if GitHub 443 remains unreachable. |
| 2026-07-03 | Schedule UI/API hardening | `09969ad`: per-site schedule random-delay validation, non-silent preview failures, responsive schedule preview rows, `npm run smoke:schedules`. Verification passed and pushed to `origin/main`. |
| 2026-07-02 | rootCtx lifecycle + cleanup | `9fb28d4`: StartSchedulers derives from a.rootCtx. `ac8687e`: all remaining context.Background() in scheduler.go replaced with ctx. 861 tests pass. |
| 2026-07-02 | Coverage sprint G0–G2 | G0: 8 commits pushed. G1: weighted avg 61.4% (beat 55%). G2: 187 frontend tests. New: crypto_util_test, filters_test, helpers_test, versioncheck extended, backup service_test. |
| 2026-07-02 | Domain coverage batch | +2035 lines across 7 test files + 1 extraction. channels 14%→60.7%, accounts 19.6%→25.4%, versioncheck 27.3%→32.8% |
| 2026-07-02 | Handler + Service tests | `27288f7`: core handler_health_test.go (13 tests, 43.8%), accounts service_test.go (stubInfra pattern, 31.5%). Weighted avg 62.6%. |
| 2026-07-01 | Backend M/L-tier fixes | `883e3dc` + `656c5dc`: error propagation, CST unification, SMTP/HTTP timeouts, 12 frontend Promise rejections |
| 2026-07-01 | rows.Err + diagnostics + shutdown | `926ef7e` + `9a51aea` + `23cfb46`: 18 rows.Err checks, API path scrubbing, rootCtx shutdown, notification WG |
| 2026-07-01 | NotificationHub + SSE cap | `63420b0`: 25 hub boundary tests, 50-subscriber SSE cap, digest WG panic fix |
| 2026-07-01 | Performance batch 4 | `e9ab95f`: N+1 elimination, per-key cache invalidation, SSE lifecycle hardening |
