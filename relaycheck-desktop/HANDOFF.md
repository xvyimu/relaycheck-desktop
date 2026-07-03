# HANDOFF.md

Authoritative handoff document for RelayCheck Desktop. Updated each session.
Read this first, then `CLAUDE.md` for architecture.

**Last updated:** 2026-07-03

---

## Current state

The project is stable. This session has uncommitted schedule UI/API hardening changes in:

- `frontend/src/components/settings/SiteSchedules.tsx`
- `frontend/src/styles.css`
- `internal/core/channel_schedules.go`
- `internal/core/channel_schedules_test.go`

All verification gates pass:

```powershell
go build -mod=vendor ./...
go vet -mod=vendor ./...
go test -mod=vendor -count=1 ./...               # 888 tests pass
cd frontend; npm run build                       # tsc + vite
cd frontend; npx vitest run                      # 187 tests pass
git diff --check
```

---

## What landed this session

### Schedule UI/API hardening (pending commit)

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

### 0. Commit the schedule hardening change

- Current hardening changes are verified, including `npm run smoke:schedules`.
- Commit with a message like `fix(scheduler): harden per-site schedule settings`.

### 1. Raise test coverage in core, accounts, channels, and notifications

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
| 2026-07-02 | rootCtx lifecycle + cleanup | `9fb28d4`: StartSchedulers derives from a.rootCtx. `ac8687e`: all remaining context.Background() in scheduler.go replaced with ctx. 861 tests pass. |
| 2026-07-02 | Coverage sprint G0–G2 | G0: 8 commits pushed. G1: weighted avg 61.4% (beat 55%). G2: 187 frontend tests. New: crypto_util_test, filters_test, helpers_test, versioncheck extended, backup service_test. |
| 2026-07-02 | Domain coverage batch | +2035 lines across 7 test files + 1 extraction. channels 14%→60.7%, accounts 19.6%→25.4%, versioncheck 27.3%→32.8% |
| 2026-07-02 | Handler + Service tests | `27288f7`: core handler_health_test.go (13 tests, 43.8%), accounts service_test.go (stubInfra pattern, 31.5%). Weighted avg 62.6%. |
| 2026-07-01 | Backend M/L-tier fixes | `883e3dc` + `656c5dc`: error propagation, CST unification, SMTP/HTTP timeouts, 12 frontend Promise rejections |
| 2026-07-01 | rows.Err + diagnostics + shutdown | `926ef7e` + `9a51aea` + `23cfb46`: 18 rows.Err checks, API path scrubbing, rootCtx shutdown, notification WG |
| 2026-07-01 | NotificationHub + SSE cap | `63420b0`: 25 hub boundary tests, 50-subscriber SSE cap, digest WG panic fix |
| 2026-07-01 | Performance batch 4 | `e9ab95f`: N+1 elimination, per-key cache invalidation, SSE lifecycle hardening |
