# CLAUDE.md

Guidance for Claude Code working in this repository.

## Project

**RelayCheck Desktop** — local operations console for NewAPI/OneAPI/Sub2API relay sites. Single-binary Go backend + embedded React/Vite frontend + SQLite. Runs as a desktop app on `http://127.0.0.1:3001`.

- Go 1.24+, `net/http` server, `modernc.org/sqlite` (no cgo)
- React 19 + Vite, embedded via `//go:embed frontend/dist`
- Single-user local tool, no login layer (`requireSession` is passthrough)

## Verification (run from repo root)

```powershell
go build -mod=vendor ./...
go test -mod=vendor ./internal/core/ -count=1 -timeout 120s
go vet -mod=vendor ./internal/core/...
cd frontend; npm run build; cd ..
cd frontend; npx tsc --noEmit; cd ..
```

All five must pass before commit. Race detector (`-race`) is not used: requires cgo which is disabled in this Windows env.

## Architecture (post-refactor)

`internal/core` is a single `package core` with 75+ files. The `App` struct in `app.go` is the god object / assembly root. A completed architecture evolution (commits `8fc1975`..`1444e43`, June 2026) extracted state and services into dedicated types to reduce coupling and improve testability.

### Extracted types (each owns its own mutex, independently testable)

| Type | File | Replaces | Notes |
|------|------|----------|-------|
| `SharedInfra` (interface) | `infra.go` | — | `DB()/HTTPClient()/Key()/DataDir()/Locker()` getters; `*App` implements it |
| `CryptoService` | `crypto_service.go` | `a.encryptText/decryptText` bodies | AES-256-GCM, `v1.<nonce>.<ciphertext>` format; `*App` methods are thin forwarders |
| `AccountAuthRepository` | `account_auth_repo.go` | `a.loadAccountAuth(s)` bodies | `Load(ctx,id)` + `LoadBatch(ctx,ids)`; injects `db`+`crypto` |
| `CheckinRunStore` | `checkin_run_state.go` | `a.checkinRun` + 5 mutators | Independent `sync.RWMutex`; `Snapshot()` for reads |
| `NotificationHub` | `notification_hub.go` | 5 App fields + 7 methods | Holds `config`/`digestChannels`/`digestCancel`/`digestWG`/`channelRateLimits`; `Close()` stops digest goroutines |
| `NotificationHTTPPort` (interface) | `notification_hub.go` | `webhookChannel.app *App` | 2 methods (`externalURLPolicy`, `doHTTPWithTimeout`); all 6 channel types depend on this, not `*App` |
| `SyncJobRunStore` | `sync_job_run_store.go` | `a.localSyncRun`/`channelHealthRun` | `TryStart()`/`Finish()` for re-entrancy guard |
| `SchedulerRepo` | `scheduler_repo.go` | `a.loadSettingJSON`/`loadSchedulerRun`/`upsertSchedulerPlan` bodies | Pure db repository |
| `ReadCacheStore` | `read_cache_store.go` | `a.readCache`+`a.readCacheMu` | Generic `Get[T]` + `Invalidate()`; `cachedRead[T]` and `a.invalidateReadCache` are forwarders |
| `BrowserSessionStore` | `browser_session_store.go` | `a.browserSessions` | `Get/Set/Delete/DeleteIfPIDMatches/List/Range`; watchdog uses `DeleteIfPIDMatches` to avoid deleting replaced sessions |
| `NetworkProxyStore` | `network_proxy_store.go` | `a.networkProxy` | `Get()/Set()` |

### Forwarding method pattern

`*App` retains thin forwarding methods for all extracted logic so existing call sites need no changes. Example:

```go
func (a *App) encryptText(value string) (string, error) { return a.crypto.Encrypt(value) }
```

When adding new code, prefer calling the extracted type directly (`a.crypto.Encrypt(...)`, `a.accountAuth.Load(...)`) rather than the `*App` forwarder. The forwarders exist for migration safety, not as the preferred API.

### What stays on `*App` (by design)

- `db`, `dataDir`, `key`, `client` — infrastructure, exposed via `SharedInfra`
- `mu` — now only protects `bind`/`port`/`preferredPort`/`portConflict`/`schedulerCancel`/`schedulerStartedAt` (runtime address + scheduler lifecycle)
- `schedulerCancel`/`schedulerStartedAt`/`schedulerWG`/`taskRunner` — App is the assembly root for lifecycle
- `bind`/`port`/`preferredPort`/`portConflict`/`allowLocalOutbound` — runtime address state

### Cross-cutting concerns that stay in `package core` (do NOT attempt to split into sub-packages)

- `audit.go` — `a.audit(...)` called from 22 sites across all domains
- `crypto.go` — `encryptText`/`decryptText` (forwarders) + `loadOrCreateKey` + `maskSecret` + `secretFingerprint`
- `notification.go` — channel type definitions + forwarding methods; hub logic is in `notification_hub.go`

These were evaluated for extraction during the architecture review and intentionally kept in `core` because the call-site churn (98+ import changes) outweighed the benefit. See `docs/superpowers/specs/2026-06-29-architecture-evolution-design.md` for the full rationale.

## Hard constraints (from project memory — do not violate)

- Zip exports: PBKDF2-SHA256 with 200,000 iterations + random 32-byte salt (no raw SHA-256)
- Zip imports: max 256MB total, 200MB per entry (zip-bomb protection)
- API responses must not include absolute filesystem paths
- Batch account ID operations: use `IN (...)` clauses, not N+1 queries (max 200 IDs for dry-run)
- Scheduled tasks: use `time.FixedZone("CST", 8*3600)`, never server local time
- All write endpoints: `requireSession` + CSRF/same-origin validation
- EventSource (SSE): close before creating new; prevents connection leaks
- SQL: parameterized statements only; check `rows.Err()` after iteration
- Balance aggregation: `AVG(NULLIF(balance, 0))` with `sql.NullFloat64`, not `AVG(COALESCE(balance, 0))`
- Errors: propagate to UI, no silent failure; log + return meaningful feedback
- Promises: handle rejections; no unhandled rejections

## File navigation

- `internal/core/PACKAGE_INDEX.md` — file-by-file map of the `core` package (authoritative)
- `docs/PROJECT_STRUCTURE.md` — source tree, generated paths, verification order
- `README.md` — product overview, route table, commands
- `DESIGN_SYSTEM.md` — Control Room visual direction
- `docs/superpowers/specs/` — architecture evolution design + plan + remaining-items deepening

## Conventions

- HTTP handlers: `handleXxx(w http.ResponseWriter, r *http.Request)`, registered in `routes.go`
- Method check: `if !method(w, r, http.MethodPost) { return }` (direct `r.Method` check, not multiple `method()` calls)
- Time: `now()` returns `time.Now().UTC().Format(time.RFC3339)`; "today" uses `todayCST()` helper in `app.go`
- IDs: `newID()` returns UUID-like string
- JSON: `writeJSON(w, status, data)` / `writeError(w, status, msg)`
- Encryption: `a.crypto.Encrypt/Decrypt` for new code; `a.encryptText/decryptText` forwarders exist for legacy call sites
- Auth loading: `a.accountAuth.Load(ctx, id)` / `LoadBatch(ctx, ids)` for new code; `a.loadAccountAuth(s)` forwarders exist
- Comments/commit messages: English. User-facing error messages: Chinese (unified during remaining-items deepening)

## Working directory

Primary: `e:\zidqiandao\relaycheck-desktop`
Git root: `e:\zidqiandao` (the `relaycheck-desktop` subdirectory is the active project; `e:\zidqiandao\_archive\` holds retired Python/Next.js implementations)

## Before you start

1. Read `internal/core/PACKAGE_INDEX.md` for the file map
2. Read `internal/core/app.go` lines 24-50 for the current `App` struct
3. Run the verification commands to confirm a clean baseline
4. Check `git log --oneline -20` for recent refactor history
