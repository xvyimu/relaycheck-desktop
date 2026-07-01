# internal/core Package Index

Last updated: 2026-07-02 (commit `0bd8c13`, local)

The `internal/core` package is the assembly root for RelayCheck Desktop's backend. It holds the `App` struct (`app.go`), HTTP handlers, cross-cutting concerns (audit/crypto/network/url_safety), and forwarding methods to 8 extracted domain packages under `internal/<domain>/`. See `CLAUDE.md` for the architecture overview.

## Extracted domain packages

Domain logic lives outside `core` in dedicated packages. Dependency direction: `core` → domain (one-way). Each domain package declares a local `Infra` interface that `*App` satisfies via exported adapter methods.

| Package | Files | Purpose |
|---------|-------|---------|
| `internal/notifications` | `hub.go`, `channels.go`, `channels_internal_test.go` | `NotificationHub` + 6 channel implementations (webhook/telegram/bark/serverchan/email/desktop). `NotificationHTTPPort` interface. |
| `internal/backup` | `service.go`, `zip_crypto.go` | Encrypted zip export/import. `Infra` interface (DB, DatabasePath, BackupsDir, ReopenDatabase, ReloadNotificationConfig, ProductVersion). |
| `internal/versioncheck` | `service.go`, `service_test.go` | Remote version manifest check + semver compare. `Infra` interface (DB, HTTPClient, ProductVersion, ValidateOutboundURLStrict). |
| `internal/legacycheck` | `service.go` | Legacy Python code detection. `Infra` interface (DataDir). |
| `internal/autostart` | `service.go`, `platform_windows.go`, `platform_other.go` | OS auto-start shortcut management. No Infra (pure). |
| `internal/sites` | `service.go`, `types.go`, `detection.go`, `detection_test.go`, `scanner.go`, `scanner_test.go` | Upstream site CRUD, detection (headers/HTML/API), local network scanner. |
| `internal/channels` | `service.go`, `types.go`, `helpers.go`, `channels.go`, `models.go`, `health.go`, `schedules.go`, `pricing.go`, `models_overview.go` + 7 test files (`helpers_test.go`, `health_test.go`, `models_test.go`, `models_overview_test.go`, `pricing_test.go`, `schedules_test.go`, `channels_test.go`) | Channel CRUD, model sync, health overview, schedules, pricing, model overview. Coverage 60.7%. |
| `internal/accounts` | `service.go`, `types.go`, `helpers.go`, `chrome_password.go`, `import_admin_api.go`, `import_sqlite.go`, `legacy_config.go`, `local_newapi.go`, `sync_preview.go`, `auto_detect.go` + 2 test files (`helpers_test.go`, `chrome_password_test.go`) | Account CRUD + 5 import paths + sync preview + auto-detect. Coverage 25.4%. |

## File Groups (within `internal/core`)

### Application Bootstrap

| File | Purpose |
|------|---------|
| `app.go` | `App` struct, `NewApp()` constructor, configuration defaults, lifecycle, two-phase init for domain services. |
| `db.go` | SQLite initialization, schema migrations, `channel_schedules` table. |
| `routes.go` | HTTP route registration, `RegisterRoutes()`. |
| `http.go` | HTTP helpers: `writeJSON`, `writeError`, `method()`, session middleware. |
| `models.go` | Core data structures: `SystemStatus`, `DashboardSummary`, `ChannelAccount`, `CheckinLog`, `BalanceSnapshot`, etc. |
| `infra.go` | `SharedInfra` interface (`DB()`/`HTTPClient()`/`Key()`/`DataDir()`/`Locker()` getters); `*App` implements it. Foundation for extracted services/stores. |

### Extracted Services & Stores (Phase 1 — within `core`)

Each owns its own mutex and is independently testable. `*App` retains thin forwarding methods for backward compatibility; new code should call the extracted types directly.

| File | Purpose |
|------|---------|
| `crypto_service.go` | `CryptoService` type: AES-256-GCM encryption with `v1.<nonce>.<ciphertext>` format. Extracted from `crypto.go` bodies of `encryptText`/`decryptText`. |
| `account_auth_repo.go` | `AccountAuthRepository`: `Load(ctx,id)` + `LoadBatch(ctx,ids)` for account authentication context. Injects `db`+`crypto`. |
| `checkin_run_state.go` | `CheckinRunStore`: checkin run state with independent `sync.RWMutex`; `Snapshot()` for reads. Replaces `a.checkinRun` + 5 mutators. |
| `sync_job_run_store.go` | `SyncJobRunStore`: `TryStart()`/`Finish()` re-entrancy guard for scheduled jobs. Replaces `a.localSyncRun`/`channelHealthRun`. |
| `scheduler_repo.go` | `SchedulerRepo`: pure db repository for `loadSettingJSON`/`loadSchedulerRun`/`upsertSchedulerPlan`. |
| `read_cache_store.go` | `ReadCacheStore`: generic `Get[T]` + `Invalidate()`; `cachedRead[T]` and `a.invalidateReadCache` are forwarders. Replaces `read_cache.go`. |
| `browser_session_store.go` | `BrowserSessionStore`: Chrome login session management (`Get`/`Set`/`Delete`/`DeleteIfPIDMatches`/`List`/`Range`); watchdog uses `DeleteIfPIDMatches`. |
| `network_proxy_store.go` | `NetworkProxyStore`: proxy config `Get()`/`Set()`. |

### Domain Forwarders (Phase 2 — `*App` adapter methods + handlers)

Each file in `core` forwards to the corresponding extracted domain package. Conversion between `core` types and mirror types lives in `<domain>_convert.go` (where present).

| File | Purpose |
|------|---------|
| `notification.go` | Forwarders to `*notifications.NotificationHub`: `notify()`, `dispatchNotification()`, channel CRUD handlers, `healthCheckNotificationChannels()`, `ReloadNotificationConfig()` adapter. `NotificationHTTPPort` is satisfied via `ValidateOutboundURL` (`url_safety.go`) + `DoHTTPWithTimeout` (`network.go`). |
| `backup_zip.go` | Forwarders to `*backup.Service`: `handleBackupExport`/`handleBackupImport`/`handleBackupList` HTTP handlers. |
| `version_check.go` | Forwarders to `*versioncheck.Service`: `handleVersionCheck` HTTP handler. |
| `legacy_check.go` | Forwarders to `*legacycheck.Service`: `handleLegacyCheck` HTTP handler. |
| `autostart.go` | Forwarders to `*autostart.Service`: `handleAutoStartStatus`/`handleAutoStartEnable`/`handleAutoStartDisable` HTTP handlers. |
| `sites.go` | Forwarders to `*sites.Service`: site CRUD HTTP handlers. |
| `scanner.go` | Site scan/probe handlers (`ProbeResult`, `UpstreamDetection` types remain in `core` for API compatibility; forwarders call `sitesService`). |
| `detection_detail.go` | Cross-domain aggregation: `loadSiteDetail` joins site + channel + account data. `marshalDetection` is a pure `json.Marshal` helper called from 8 sites. |
| `channels.go` | Forwarders to `*channels.Service`: channel CRUD HTTP handlers. |
| `channels_convert.go` | Type converters between `core` and `channels` mirror types (pricing sources, site pricing cache items). |
| `channels_infra.go` | `*App` adapter methods implementing `channels.Infra` (14 methods). |
| `channel_health.go` | Channel health probe task and scheduled probe orchestration (uses `channelsService`). |
| `channel_models.go` | Channel model sync HTTP handlers (uses `channelsService`). |
| `channel_schedules.go` | Per-site checkin scheduling, calendar preview, next-runs list (uses `channelsService`). |
| `models_pricing.go` | Model overview, pricing sync, key export preview HTTP handlers. Pricing pure functions (`extractModelPricingSources`, etc.) remain here for test access. |
| `accounts.go` | Forwarders to `*accounts.Service`: account CRUD HTTP handlers. |
| `accounts_infra.go` | `*App` adapter methods implementing `accounts.Infra` (`EncryptText`, `DetectUpstreamForImport`, `EnsureChannelSiteForImport`). |
| `import_sqlite.go` | SQLite import HTTP handlers (forwards to `accountsService`). |
| `import_admin_api.go` | Admin API import HTTP handlers (forwards to `accountsService`). |
| `local_newapi.go` | Local NewAPI instance management HTTP handlers (forwards to `accountsService`). |
| `legacy_config.go` | Legacy `config_site*.json` import HTTP handlers (forwards to `accountsService`). |
| `chrome_password_import.go` | Chrome password CSV import HTTP handlers (forwards to `accountsService`). |
| `sync_preview.go` | Sync diff preview HTTP handlers (forwards to `accountsService`). |
| `auto_detect.go` | Auto-detect SQLite HTTP handlers (forwards to `accountsService`). |

### Checkins & Balances (stays in `core` — see CLAUDE.md rationale)

| File | Purpose |
|------|---------|
| `checkin_balance.go` | Checkin execution, balance refresh, login flow, API key testing. Highly coupled to `*App` (db, notify, checkinRun, accountAuth, callAccountAPI, loginWithPassword, saveAccountSession, encryptText, currentNetworkProxyConfig). Evaluated for extraction in Phase 2 and intentionally kept in `core`. |
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

### Cross-cutting concerns (stay in `core` — see CLAUDE.md rationale)

| File | Purpose |
|------|---------|
| `audit.go` | Audit log: append, query, redact. Called from 22+ sites across all domains. |
| `crypto.go` | `loadOrCreateKey`, `maskSecret`, `secretFingerprint`, and `encryptText`/`decryptText` thin forwarders. Encryption logic in `crypto_service.go`. |
| `network.go` | HTTP client with proxy support, `doHTTP`/`DoHTTPWithTimeout`/`doLoginHTTP` shared by `accounts`/`channels`/`sites`/`checkin_balance`. |
| `url_safety.go` | URL safety validation: SSRF blocklist, loopback/private address checks. `ValidateOutboundURL`/`ValidateOutboundURLStrict` adapters called from multiple domain packages. |

### System & Platform

| File | Purpose |
|------|---------|
| `system.go` | System settings CRUD, proxy config, backup/restore. Tightly coupled to `notification`/`network`/`channel_health` config reload paths (stays in `core`). |
| `platform_windows.go` | Windows: `netListen`, `hiddenProcessAttr` (used by `accounts.go`). Auto-start logic moved to `internal/autostart/`. |
| `platform_other.go` | Non-Windows platform stubs for `netListen`/`hiddenProcessAttr`. |
| `health.go` | Health endpoint (unauthenticated). |
| `filters.go` | Query parameter parsing and filtering helpers. |

### Test Files

| File | Purpose |
|------|---------|
| `account_auth_repo_test.go` | `AccountAuthRepository` unit tests. |
| `accounts_cleanup_test.go` | Account cleanup tests. |
| `accounts_key_test.go` | API key management tests. |
| `action_center_test.go` | Action Center tests. |
| `app_test.go` | App bootstrap tests. |
| `audit_test.go` | Audit log tests. |
| `auto_detect_test.go` | Auto-detect tests. |
| `balance_bulk_test.go` | Bulk balance refresh tests. |
| `browser_session_store_test.go` | `BrowserSessionStore` unit tests. |
| `bulk_test_api_keys_test.go` | Bulk API key tests. |
| `channel_health_probe_task_test.go` | Channel health probe task tests. |
| `channel_health_test.go` | Channel health tests. |
| `channel_models_test.go` | Channel model sync tests. |
| `channel_schedules_test.go` | Channel schedule tests. |
| `checkin_run_state_test.go` | `CheckinRunStore` unit tests. |
| `checkin_status_test.go` | Checkin status tests. |
| `crypto_service_test.go` | `CryptoService` unit tests. |
| `db_ensure_column_test.go` | DB column-ensure tests. |
| `db_performance_test.go` | Database performance tests. |
| `detection_engine_test.go` | Detection engine tests (headers, HTML, API, confidence). |
| `dry_run_test.go` | Dry-run preview tests. |
| `encoding_test.go` | Encoding tests. |
| `health_test.go` | Health endpoint tests. |
| `http_security_test.go` | HTTP security tests: CORS, headers, session. |
| `key_export_security_test.go` | Key export security tests. |
| `models_pricing_test.go` | Model pricing tests. |
| `network_proxy_store_test.go` | `NetworkProxyStore` unit tests. |
| `network_test.go` | Network/SSRF tests. |
| `notification_test.go` | Notification forwarding tests (14 tests covering dispatch, reload, encrypt/decrypt, build channel, health check). |
| `perf_large_dataset_test.go` | 500+ account performance test. |
| `read_cache_store_test.go` | `ReadCacheStore` unit tests. |
| `read_cache_test.go` | Read cache tests. |
| `scanner_test.go` | Scanner tests. |
| `scheduler_repo_test.go` | `SchedulerRepo` unit tests. |
| `scheduler_test.go` | Scheduler tests. |
| `secrets_security_test.go` | Security tests: no plaintext secrets in responses. |
| `sites_test.go` | Sites tests. |
| `sync_job_run_state_test.go` | `SyncJobRunStore` unit tests. |
| `sync_preview_test.go` | Sync preview tests. |
| `system_backup_test.go` | System backup tests. |
| `system_restore_test.go` | System restore tests. |
| `system_status_test.go` | System status tests. |
| `testhelper_test.go` | Shared test helpers. |
| `url_safety_test.go` | URL safety tests. |
| `usage_overview_test.go` | Usage overview tests. |

## Conventions

- All files use `package core` — no sub-packages within `core`.
- Database access via `a.db` (the `*sql.DB` instance on `App`). 领域服务/仓储通过 `SharedInfra` 接口（`infra.go`）或领域 `Infra` 接口（`<domain>_infra.go`）访问 db/client/key。
- HTTP handlers follow the `handleXxx(w http.ResponseWriter, r *http.Request)` pattern.
- Route registration in `routes.go` via `mux.HandleFunc`.
- Session protection via `a.requireSession(handler)`.
- Credential encryption via `CryptoService` (`crypto_service.go`); `*App.encryptText`/`decryptText` are thin forwarders.
- 新增代码应直接调用提取的类型（`a.crypto.Encrypt`, `a.accountAuth.Load`, `a.channelsService.XXX`, `a.sitesService.XXX` 等），而非 `*App` 转发方法。
- Domain service fields on `*App`: `notificationHub`, `backupService`, `versionCheckService`, `legacyCheckService`, `autostartService`, `sitesService`, `channelsService`, `accountsService`. Two-phase init in `NewApp`.
- Time helpers: `now()` returns `time.Now().UTC().Format(time.RFC3339)`.
- ID generation: `newID()` returns a UUID-like string.
- Error responses: `writeError(w, statusCode, message)`.
- JSON responses: `writeJSON(w, statusCode, data)`.
