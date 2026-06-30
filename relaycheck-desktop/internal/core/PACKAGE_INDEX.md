# internal/core Package Index

Last updated: 2026-06-30

The `internal/core` package contains all backend application logic for RelayCheck Desktop. It is a single Go package (`package core`) with 65+ source files organized by domain.

## File Groups

### Application Bootstrap

| File | Purpose |
|------|---------|
| `app.go` | `App` struct, `NewApp()` constructor, configuration defaults, lifecycle. |
| `db.go` | SQLite initialization, schema migrations, `channel_schedules` table. |
| `routes.go` | HTTP route registration, `RegisterRoutes()`, notification handlers. |
| `http.go` | HTTP helpers: `writeJSON`, `writeError`, `method()`, session middleware. |
| `models.go` | Core data structures: `SystemStatus`, `DashboardSummary`, `ChannelAccount`, `AutoStartStatus`, etc. |
| `infra.go` | `SharedInfra` interface (`DB()`/`HTTPClient()`/`Key()`/`DataDir()`/`Locker()` getters); `*App` implements it. Foundation for extracted services/stores. |

### Extracted Services & Stores (架构演进)

Extracted types from `*App` during the June 2026 architecture evolution (commits `8fc1975`..`1444e43`). Each owns its own mutex and is independently testable. `*App` retains thin forwarding methods for backward compatibility; new code should call the extracted types directly.

| File | Purpose |
|------|---------|
| `crypto_service.go` | `CryptoService` type: AES-256-GCM encryption with `v1.<nonce>.<ciphertext>` format. Extracted from `crypto.go` bodies of `encryptText`/`decryptText`. |
| `account_auth_repo.go` | `AccountAuthRepository`: `Load(ctx,id)` + `LoadBatch(ctx,ids)` for account authentication context. Injects `db`+`crypto`. |
| `checkin_run_state.go` | `CheckinRunStore`: checkin run state with independent `sync.RWMutex`; `Snapshot()` for reads. Replaces `a.checkinRun` + 5 mutators. |
| `notification_hub.go` | `NotificationHub` + `NotificationHTTPPort` interface. Holds 5 App fields (config/digestChannels/digestCancel/digestWG/channelRateLimits) + 7 methods; `Close()` stops digest goroutines. |
| `sync_job_run_store.go` | `SyncJobRunStore`: `TryStart()`/`Finish()` re-entrancy guard for scheduled jobs. Replaces `a.localSyncRun`/`channelHealthRun`. |
| `scheduler_repo.go` | `SchedulerRepo`: pure db repository for `loadSettingJSON`/`loadSchedulerRun`/`upsertSchedulerPlan`. |
| `read_cache_store.go` | `ReadCacheStore`: generic `Get[T]` + `Invalidate()`; `cachedRead[T]` and `a.invalidateReadCache` are forwarders. Replaces `read_cache.go`. |
| `browser_session_store.go` | `BrowserSessionStore`: Chrome login session management (`Get`/`Set`/`Delete`/`DeleteIfPIDMatches`/`List`/`Range`); watchdog uses `DeleteIfPIDMatches`. |
| `network_proxy_store.go` | `NetworkProxyStore`: proxy config `Get()`/`Set()`. |

### Accounts & Credentials

| File | Purpose |
|------|---------|
| `accounts.go` | Account CRUD, list/create/update/delete, login status management. |
| `crypto.go` | `loadOrCreateKey`, `maskSecret`, `secretFingerprint`, and `encryptText`/`decryptText` thin forwarders. Encryption logic now in `crypto_service.go` (`CryptoService`). |
| `chrome_password_import.go` | Chrome/Via password CSV import preview and matching. |
| `legacy_config.go` | Legacy `config_site*.json` import. |

### Channels & Sites

| File | Purpose |
|------|---------|
| `channels.go` | Channel CRUD, source status sync, archive/restore. |
| `channel_models.go` | Channel model list sync and overview. |
| `channel_schedules.go` | Per-site checkin scheduling, calendar preview, next-runs list. |
| `sites.go` | Upstream site CRUD, bulk detect, health check. |
| `scanner.go` | Local network scanner for NewAPI instances. |
| `detection_engine.go` | Site kind detection from headers/HTML/API responses. |
| `detection_detail.go` | Detailed detection result formatting. |

### Checkins & Balances

| File | Purpose |
|------|---------|
| `checkin_balance.go` | Checkin execution, balance refresh, API key testing. |
| `usage_overview.go` | Usage overview aggregation. |
| `balance_bulk_test.go` | Tests for bulk balance refresh. |

### Scheduling & Tasks

| File | Purpose |
|------|---------|
| `scheduler.go` | Global scheduler, job status, next-run computation. |
| `task_runner.go` | Unified task engine with SSE streaming progress. |
| `dry_run.go` | Dry-run preview for batch operations (200 account limit). |

### Analytics & Diagnostics

| File | Purpose |
|------|---------|
| `analytics.go` | Analytics endpoint: balance trend, checkin distribution, response times, site reliability, balance deltas. |
| `diagnostics.go` | System diagnostics, cookie expiry tracking, health checks. |
| `action_center.go` | Action Center: prioritized user-facing issues. |

### Notifications

| File | Purpose |
|------|---------|
| `notification.go` | Notification channels: Webhook (HMAC + retry), Telegram, Bark, ServerChan, Email, Desktop. `levelMatchesMode` supports all/failure/success/warning+. |
| `notification_hub.go` | `NotificationHub` + `NotificationHTTPPort` interface (see Extracted Services & Stores). |
| `notification_test.go` | Notification channel tests. |

### System & Platform

| File | Purpose |
|------|---------|
| `system.go` | System settings CRUD, proxy config, backup/restore. |
| `version_check.go` | Remote version check against manifest URL. |
| `autostart.go` | Auto-start API: get/set shell:startup shortcut. |
| `platform_windows.go` | Windows: `CreateStartupShortcut()` via PowerShell COM, `IsStartupShortcutPresent()`, `RemoveStartupShortcut()`. |
| `platform_other.go` | Non-Windows platform stubs. |
| `legacy_check.go` | Legacy Python code check API (route count, DB init idempotency). |
| `health.go` | Health endpoint (unauthenticated). |
| `network.go` | HTTP client with proxy support, SSRF validation. |
| `url_safety.go` | URL safety validation: SSRF blocklist, loopback/private address checks. |
| `filters.go` | Query parameter parsing and filtering helpers. |

### Import & Migration

| File | Purpose |
|------|---------|
| `import_sqlite.go` | Import from local NewAPI SQLite. |
| `import_admin_api.go` | Import from local NewAPI admin API. |
| `local_newapi.go` | Local NewAPI instance management. |
| `sync_preview.go` | Sync diff preview. |

### Backup & Export

| File | Purpose |
|------|---------|
| `backup_zip.go` | AES-256-GCM encrypted zip export/import. PBKDF2-SHA256 key derivation (200,000 iterations + 32-byte salt). RCZIP2 format; RCZIP1 backward-compatible. Zip-bomb protection (256MB total, 200MB per entry). |

### Audit & Security

| File | Purpose |
|------|---------|
| `audit.go` | Audit log: append, query, redact. |
| `secrets_security_test.go` | Security tests: no plaintext secrets in responses. |
| `http_security_test.go` | HTTP security tests: CORS, headers, session. |

### Models & Pricing

| File | Purpose |
|------|---------|
| `models_pricing.go` | Model pricing sync and comparison. |

### Test Files

| File | Purpose |
|------|---------|
| `accounts_cleanup_test.go` | Account cleanup tests. |
| `accounts_key_test.go` | API key management tests. |
| `action_center_test.go` | Action Center tests. |
| `app_test.go` | App bootstrap tests. |
| `audit_test.go` | Audit log tests. |
| `auto_detect_test.go` | Auto-detect tests. |
| `bulk_test_api_keys_test.go` | Bulk API key tests. |
| `channel_health_probe_task_test.go` | Channel health probe task tests. |
| `channel_health_test.go` | Channel health tests. |
| `channel_models_test.go` | Channel model sync tests. |
| `channel_schedules_test.go` | Channel schedule tests. |
| `checkin_status_test.go` | Checkin status tests. |
| `db_ensure_column_test.go` | DB column-ensure tests. |
| `db_performance_test.go` | Database performance tests. |
| `detection_engine_test.go` | Detection engine tests (headers, HTML, API, confidence). |
| `dry_run_test.go` | Dry-run preview tests. |
| `encoding_test.go` | Encoding tests. |
| `health_test.go` | Health endpoint tests. |
| `key_export_security_test.go` | Key export security tests. |
| `models_pricing_test.go` | Model pricing tests. |
| `network_test.go` | Network/SSRF tests. |
| `perf_large_dataset_test.go` | 500+ account performance test. |
| `read_cache_test.go` | Read cache tests. |
| `scanner_test.go` | Scanner tests. |
| `scheduler_test.go` | Scheduler tests. |
| `sites_test.go` | Sites tests. |
| `sync_preview_test.go` | Sync preview tests. |
| `system_backup_test.go` | System backup tests. |
| `system_restore_test.go` | System restore tests. |
| `system_status_test.go` | System status tests. |
| `testhelper_test.go` | Shared test helpers. |
| `url_safety_test.go` | URL safety tests. |
| `usage_overview_test.go` | Usage overview tests. |
| `version_check_test.go` | Version check tests. |

## Conventions

- All files use `package core` — no sub-packages.
- Database access via `a.db` (the `*sql.DB` instance on `App`). 领域服务/仓储通过 `SharedInfra` 接口（`infra.go`）访问 db/client/key。
- HTTP handlers follow the `handleXxx(w http.ResponseWriter, r *http.Request)` pattern.
- Route registration in `routes.go` via `mux.HandleFunc`.
- Session protection via `a.requireSession(handler)`.
- Credential encryption via `CryptoService` (`crypto_service.go`); `*App.encryptText`/`decryptText` are thin forwarders.
- 新增代码应直接调用提取的类型（`a.crypto.Encrypt`, `a.accountAuth.Load`），而非 `*App` 转发方法。
- Time helpers: `now()` returns `time.Now().UTC().Format(time.RFC3339)`.
- ID generation: `newID()` returns a UUID-like string.
- Error responses: `writeError(w, statusCode, message)`.
- JSON responses: `writeJSON(w, statusCode, data)`.
