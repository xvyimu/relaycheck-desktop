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
go test -mod=vendor ./... -count=1 -timeout 120s
go vet -mod=vendor ./...
cd frontend; npm run build; cd ..
cd frontend; npx tsc --noEmit; cd ..
```

All five must pass before commit. Race detector (`-race`) is not used: requires cgo which is disabled in this Windows env.

## Architecture (post-refactor)

`internal/core` is a single `package core` (60+ files) that serves as the assembly root. Surrounding it are 8 extracted domain packages under `internal/<domain>/` that own pure business logic. Dependency direction is **one-way**: `core` imports domain packages; domain packages never import `core`. The `App` struct in `app.go` is the god object / assembly root.

Two phases of architecture evolution (June 2026):
1. **Phase 1** (commits `8fc1975`..`1444e43`) — extracted state and services into dedicated types within `core` to reduce coupling and improve testability.
2. **Phase 2** (commits `2a32506`..`e80578c`) — split domain logic into independent `internal/<domain>/` packages using the Infra-interface + mirror-type + `*App` forwarder pattern.

### Domain packages (Phase 2 — extracted from `core`)

Each domain package defines a local `Infra` interface that `*App` satisfies via exported adapter methods. Domain packages own pure business logic and SQL; `core` retains HTTP handlers and forwarding methods so call sites need no changes.

| Package | Files | Domain | Infra interface |
|---------|-------|--------|-----------------|
| `internal/notifications` | 3 | Notification hub + 6 channel implementations (webhook/telegram/bark/serverchan/email/desktop) | `NotificationHTTPPort` (ValidateOutboundURL, DoHTTPWithTimeout) |
| `internal/backup` | 2 | Encrypted zip export/import (PBKDF2-SHA256 + RCZIP2/RCZIP1) | `Infra` (DB, DatabasePath, BackupsDir, ReopenDatabase, ReloadNotificationConfig, ProductVersion) |
| `internal/versioncheck` | 2 | Remote version manifest check + semver compare | `Infra` (DB, HTTPClient, ProductVersion, ValidateOutboundURLStrict) |
| `internal/legacycheck` | 1 | Legacy Python code detection | `Infra` (DataDir) |
| `internal/autostart` | 3 | OS auto-start shortcut management (Windows + non-Windows stub) | (none — pure) |
| `internal/sites` | 6 | Upstream site CRUD, detection (headers/HTML/API), local network scanner | `Infra` (DB, DoHTTP, ValidateOutboundURL, ValidateLocalURL, AllowLocalOutbound, Notify, Audit, Now, NewID) |
| `internal/channels` | 9 | Channel CRUD, model sync, health overview, schedules, pricing, model overview | `Infra` (14 methods) |
| `internal/accounts` | 10 | Account CRUD, Chrome password import, admin API import, SQLite import, legacy config import, local NewAPI management, sync preview, auto-detect | `Infra` (DB, EncryptText, DetectUpstreamForImport, EnsureChannelSiteForImport, Notify, Audit, Now, NewID) |

### Domain extraction pattern (Phase 2)

Three cooperating patterns make the split safe and reversible:

1. **Infra interface**: domain package declares `type Infra interface { ... }` listing only what it needs from the host. `*App` satisfies it via exported adapter methods (`DatabasePath()`, `ValidateOutboundURL()`, `DoHTTPWithTimeout()`, etc.). Compile-time assertion: `var _ domain.Infra = (*App)(nil)`.
2. **Mirror types**: domain package defines local mirror types with JSON tags identical to the corresponding `core` types. `core` keeps the original types (for API compatibility) and converts at the boundary in `<domain>_convert.go`.
3. **`*App` forwarder methods**: HTTP handlers and forwarders stay in `core` so call sites (29 `a.notify()` calls, scheduler hooks, etc.) need zero changes. `core` holds a `xxxService *xxx.Service` field, initialized in two phases: `app := NewApp(...)` then `app.xxxService = xxx.NewService(app)`.

### Extracted types within `core` (Phase 1 — each owns its own mutex, independently testable)

| Type | File | Replaces | Notes |
|------|------|----------|-------|
| `SharedInfra` (interface) | `infra.go` | — | `DB()/HTTPClient()/Key()/DataDir()/Locker()` getters; `*App` implements it |
| `CryptoService` | `crypto_service.go` | `a.encryptText/decryptText` bodies | AES-256-GCM, `v1.<nonce>.<ciphertext>` format; `*App` methods are thin forwarders |
| `AccountAuthRepository` | `account_auth_repo.go` | `a.loadAccountAuth(s)` bodies | `Load(ctx,id)` + `LoadBatch(ctx,ids)`; injects `db`+`crypto` |
| `CheckinRunStore` | `checkin_run_state.go` | `a.checkinRun` + 5 mutators | Independent `sync.RWMutex`; `Snapshot()` for reads |
| `NotificationHub` (type alias for `*notifications.NotificationHub`) | `internal/notifications/hub.go` | 5 App fields + 7 methods | Holds `config`/`digestChannels`/`digestCancel`/`digestWG`/`channelRateLimits`; `Close()` stops digest goroutines |
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
- Domain service fields (`notificationHub`, `backupService`, `versionCheckService`, `legacyCheckService`, `autostartService`, `sitesService`, `channelsService`, `accountsService`) — wired in `NewApp` via two-phase init

### Cross-cutting concerns that stay in `package core` (do NOT attempt to split into sub-packages)

These files were evaluated for extraction and intentionally kept in `core` because the call-site churn and Infra interface surface area outweighed the benefit.

- `audit.go` — `a.audit(...)` called from 22+ sites across all domains
- `crypto.go` — `encryptText`/`decryptText` (forwarders) + `loadOrCreateKey` + `maskSecret` + `secretFingerprint`
- `notification.go` — forwarding methods to `notifications.NotificationHub`; hub logic is in `internal/notifications/hub.go`
- `url_safety.go` — `ValidateOutboundURL`/`ValidateOutboundURLStrict` (SSRF blocklist + loopback/private checks) called from multiple domain packages
- `network.go` — `doHTTP`/`DoHTTPWithTimeout`/`doLoginHTTP` shared by `accounts`, `channels`, `sites`, `checkin_balance`
- `checkin_balance.go` — checkin/balance execution is highly coupled to `*App` (db, notify, checkinRun, accountAuth, callAccountAPI, loginWithPassword, saveAccountSession, encryptText, currentNetworkProxyConfig). Splitting would require an Infra interface exposing 10+ methods, with type-conversion overhead exceeding the benefit. Pure helpers (`classifyCheckinResponse`, `parseBalance`, etc.) are also referenced by `accounts.go`/`account_auth_repo.go`, so they cannot move either.
- `system.go` — system settings CRUD is tightly coupled to `notification`/`network`/`channel_health` config reload paths.

See `docs/superpowers/specs/2026-06-29-architecture-evolution-design.md` for the full rationale.

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

### Known inherited behaviors (not bugs — preserved from pre-refactor code)

These behaviors were uncovered by unit tests for the extracted types. They are inherited from the original `*App` methods and intentionally preserved to avoid behavior changes during the refactor. Do not "fix" them without a product decision.

- `AccountAuthRepository.Load` converts `sql.ErrNoRows` to a Chinese error message (`"账号不存在。"`) rather than propagating `sql.ErrNoRows`. This differs from `SchedulerRepo.LoadSettingJSON/LoadSchedulerRun` which propagate `sql.ErrNoRows` directly. Callers cannot use `errors.Is(err, sql.ErrNoRows)` on `Load`.
- `AccountAuthRepository.Load`/`LoadBatch` silently discard decryption errors (`auth.Password, _ = r.crypto.Decrypt(...)`). Corrupt ciphertext in the DB yields an empty string for that field, not an error. This is the original behavior.

## Conventions (continued)

- Comments/commit messages: English. User-facing error messages: Chinese (unified during remaining-items deepening)

## Working directory

Primary: `e:\zidqiandao\relaycheck-desktop`
Git root: `e:\zidqiandao` (the `relaycheck-desktop` subdirectory is the active project; `e:\zidqiandao\_archive\` holds retired Python/Next.js implementations)

## Before you start

1. Read `internal/core/PACKAGE_INDEX.md` for the file map
2. Read `internal/core/app.go` lines 24-50 for the current `App` struct
3. Run the verification commands to confirm a clean baseline
4. Check `git log --oneline -20` for recent refactor history
