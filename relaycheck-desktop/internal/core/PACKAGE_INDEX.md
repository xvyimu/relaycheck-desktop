# internal/core Package Index

Last updated: 2026-06-25

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

### Accounts & Credentials

| File | Purpose |
|------|---------|
| `accounts.go` | Account CRUD, list/create/update/delete, login status management. |
| `crypto.go` | AES-GCM encryption/decryption for credential fields, instance key management. |
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
| `read_cache.go` | Read-side cache for dashboard and notification counts. |

### Notifications

| File | Purpose |
|------|---------|
| `notification.go` | Notification channels: Webhook (HMAC + retry), Telegram, Bark, ServerChan, Email, Desktop. `levelMatchesMode` supports all/failure/success/warning+. |
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
| `app_test.go` | App bootstrap tests. |
| `accounts_cleanup_test.go` | Account cleanup tests. |
| `accounts_key_test.go` | API key management tests. |
| `audit_test.go` | Audit log tests. |
| `channel_models_test.go` | Channel model sync tests. |
| `checkin_status_test.go` | Checkin status tests. |
| `db_performance_test.go` | Database performance tests. |
| `detection_engine_test.go` | Detection engine tests (headers, HTML, API, confidence). |
| `health_test.go` | Health endpoint tests. |
| `network_test.go` | Network/SSRF tests. |
| `perf_large_dataset_test.go` | 500+ account performance test. |
| `models_pricing_test.go` | Model pricing tests. |
| `read_cache_test.go` | Read cache tests. |
| `scanner_test.go` | Scanner tests. |
| `scheduler_test.go` | Scheduler tests. |
| `system_backup_test.go` | System backup tests. |
| `system_status_test.go` | System status tests. |
| `url_safety_test.go` | URL safety tests. |
| `usage_overview_test.go` | Usage overview tests. |
| `version_check_test.go` | Version check tests. |

## Conventions

- All files use `package core` — no sub-packages.
- Database access via `a.db` (the `*sql.DB` instance on `App`).
- HTTP handlers follow the `handleXxx(w http.ResponseWriter, r *http.Request)` pattern.
- Route registration in `routes.go` via `mux.HandleFunc`.
- Session protection via `a.requireSession(handler)`.
- Credential encryption via `encryptText`/`decryptText` from `crypto.go`.
- Time helpers: `now()` returns `time.Now().UTC().Format(time.RFC3339)`.
- ID generation: `newID()` returns a UUID-like string.
- Error responses: `writeError(w, statusCode, message)`.
- JSON responses: `writeJSON(w, statusCode, data)`.
