# HANDOFF.md

Authoritative handoff document for RelayCheck Desktop. Updated each session.
Read this first, then `CLAUDE.md` for architecture.

**Last updated:** 2026-07-02 (commit `0bd8c13`, local — not yet pushed)

---

## Current state

The project is stable. All verification gates pass:

```powershell
go build -mod=vendor ./...
go vet -mod=vendor ./...
go test -mod=vendor -count=1 ./internal/...
cd frontend; npm run build
cd frontend; npx tsc --noEmit
```

Working tree is clean. Local branch is **7 commits ahead of `origin/main`** — push is blocked by a git proxy TLS issue (see [Known blockers](#known-blockers)).

---

## What landed this session

### Commit `0bd8c13` — test coverage

Raised domain package coverage with table-driven tests for pure functions:

| Package | Before | After | New test files |
|---------|--------|-------|----------------|
| `internal/channels` | 14.0% | **60.7%** | `health_test.go`, `models_test.go`, `models_overview_test.go`, `pricing_test.go`, `schedules_test.go`, `channels_test.go` |
| `internal/accounts` | 19.6% | **25.4%** | `chrome_password_test.go` (CSV parsing, masking, account matching) |
| `internal/versioncheck` | 27.3% | **32.8%** | Extracted `decodeSettingString` from `getSettingString` (testable pure function); `service_test.go` extended |

**Key insight:** `priceLevelBySuffix("gemini-pro")` returns `"cheap"` because `"gemini"` contains the substring `"mini"` (ge**mini**). This is the original behavior — do not "fix" without a product decision.

---

## Recent fixes (commits `926ef7e`..`656c5dc`)

These landed in prior sessions and are reflected in current code. Read the commit messages for full detail.

| Commit | Theme |
|--------|-------|
| `926ef7e` | `rows.Err()` checks after all 18 `for rows.Next()` loops |
| `9a51aea` | Surface swallowed errors; remove DB paths from API responses; `http.NewRequestWithContext` error checks |
| `23cfb46` | `rootCtx`/`rootCancel` for background tasks; notification send goroutines use dedicated context + WG |
| `883e3dc` | Chrome password N+1 → batch `IN(...)`; CST timezone unification; SMTP `Close`/`Quit`; HTTP timeouts |
| `656c5dc` | 12 unhandled frontend Promise rejections wrapped in try/catch |

---

## Suggested next steps (priority order)

### 1. Push blocked commits (highest priority)

7 local commits are not on `origin/main`:

```
0bd8c13 test: raise domain package coverage for channels, accounts, versioncheck
656c5dc fix(frontend): handle promise rejections to prevent unhandled errors
883e3dc fix: surface swallowed errors, eliminate N+1 query, unify CST timezone
23cfb46 fix: cancel background tasks and notification sends on shutdown
9a51aea fix: surface swallowed errors and prevent panics in core handlers
926ef7e fix: enforce rows.Err() checks after all rows.Next() loops
3e581d1 refactor: simplify recently modified test and SSE handler code
```

`git push` fails with TLS handshake error through the local proxy. Options:
- Push from a network without the proxy
- Configure `git config --global http.proxy ""` (clears the proxy) — verify with the user first
- Use SSH remote if configured

### 2. Raise `internal/core` coverage (42.2%)

Low-hanging fruit — pure helpers worth testing:

- `filters.go` — `filterAccountsByStatus`, `filterChannelsByKind`, etc.
- `dry_run.go` — `dryRunLimitAccountIDs` (cap at 200), `partitionAccountIDs`
- `crypto.go` — `maskSecret`, `secretFingerprint`
- `models_pricing.go` — `extractModelPricingSources`, `applyPricingNumber`, `walkPricingJSON` (note: these are also mirrored in `channels` package; tests there already cover the mirror)
- `usage_overview.go` — aggregation pure functions

Integration handlers need an `Infra` mock — defer until a clear pattern emerges.

### 3. Raise `internal/accounts` coverage (25.4%)

Pure helpers in import paths worth testing:

- `import_admin_api.go` — pagination math, response parsing
- `import_sqlite.go` — schema introspection helpers
- `legacy_config.go` — config JSON parsing
- `local_newapi.go` — instance discovery
- `sync_preview.go` — diff computation
- `auto_detect.go` — path detection

The DB-coupled paths (`Service.ImportXxx`) need a temp DB fixture — higher effort.

### 4. Raise `internal/backup` coverage (32.1%)

`zip_crypto.go` is covered. `service.go` export/import round-trip needs a temp DB + temp dir — moderate effort.

### 5. Raise `internal/versioncheck` coverage (32.8%)

`CheckVersion` needs `httptest.Server` + a stub `Infra`:
- `DB()` returns a `*sql.DB` with a `system_settings` row
- `HTTPClient()` returns a default client
- `ProductVersion()` returns a fixed string
- `ValidateOutboundURLStrict()` passes through (or use a localhost URL with `AllowLocalOutbound`)

### 6. Frontend test coverage

Currently no automated tests. Consider adding Vitest for `frontend/src/lib/` pure helpers:
- `format.ts` — date/number formatting
- `labels.ts` — label lookups
- `tone.ts` — status color mapping
- `navigation.ts` — route helpers

### 7. E2E smoke wiring

`frontend/scripts/verify-navigation.mjs` + `npm run smoke` exist but require a running server + `RELAYCHECK_SMOKE_PASSWORD`. Worth wiring into a pre-release checklist once the push is unblocked.

---

## Known blockers

### Git push blocked (must resolve before next session)

`git push` fails with a TLS handshake error through the local proxy. All 7 commits since `3e581d1` are local-only.

**To verify:** `git log --oneline origin/main..HEAD` should list 7 commits.

### Race detector unavailable

Windows env has cgo disabled, so `go test -race` cannot run. Concurrency code in `notifications/hub.go` (Reload replaces WaitGroup) and `task_runner.go` (SSE subscriber atomic counter) is covered by targeted tests but not by the race detector.

If cgo becomes available, run `go test -race -mod=vendor -count=1 ./internal/notifications/... ./internal/core/...` to catch data races.

---

## File map for handoff

| File | Role |
|------|------|
| `CLAUDE.md` | Architecture guide, verification commands, conventions, hard constraints |
| `HANDOFF.md` | This file — current task state, pending items, blockers |
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

---

## Session log

| Date | Session | Outcome |
|------|---------|---------|
| 2026-07-02 | Domain coverage batch | +2035 lines across 7 test files + 1 extraction. channels 14%→60.7%, accounts 19.6%→25.4%, versioncheck 27.3%→32.8% |
| 2026-07-01 | Backend M/L-tier fixes | `883e3dc` + `656c5dc`: error propagation, CST unification, SMTP/HTTP timeouts, 12 frontend Promise rejections |
| 2026-07-01 | rows.Err + diagnostics + shutdown | `926ef7e` + `9a51aea` + `23cfb46`: 18 rows.Err checks, API path scrubbing, rootCtx shutdown, notification WG |
| 2026-07-01 | NotificationHub + SSE cap | `63420b0`: 25 hub boundary tests, 50-subscriber SSE cap, digest WG panic fix |
| 2026-07-01 | Performance batch 4 | `e9ab95f`: N+1 elimination, per-key cache invalidation, SSE lifecycle hardening |
