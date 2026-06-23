# Changes

## T3.3 Notification Channel Expansion

### Files Created

#### `internal/core/notification.go` (NEW)
Single-file notification channel implementation containing:

- **Data structures**: `notificationChannelsConfig` / `channelEntry` / per-channel configs (`webhookConfig` / `telegramConfig` / `barkConfig` / `serverchanConfig` / `emailConfig`) / `digestEntry`
- **Interfaces**: `notificationChannel` (Type/Validate/Send/EncryptedFields) / `digestChannel` (extends notificationChannel with StartDigestLoop/FlushDigest)
- **Config loading**: `defaultNotificationChannelsConfig()` / `parseNotificationChannelsConfig()` / `validateNotificationChannelsConfig()` / `reloadNotificationConfig()` / `loadNotificationChannelsConfig()` / `currentNotificationChannelsConfig()` — follows the same three-tier lock pattern as `network.go`
- **Channel implementations**: webhookChannel / telegramChannel / barkChannel / serverchanChannel / emailChannel, each with Type/Validate/Send/EncryptedFields
- **Dispatch engine**: `dispatchNotification()` — checks global Enabled, filters by channel Types/Levels, dispatches to digestCh or async goroutine
- **Digest mode** (webhook only): `StartDigestLoop()` (5-min ticker) / `FlushDigest()` / `sendDigest()` with HMAC-SHA256 signing
- **Encryption**: `encryptChannelEntrySecrets()` / `decryptChannelEntrySecrets()` using `a.encryptText()`/`a.decryptText()` (AES-256-GCM v1 prefix); decrypt failure falls back to empty string
- **Utility functions**: `buildNotifyBody()` / `maskSensitiveField()` / `truncateNotifyContent()` / `stringInSlice()` / `buildEmailMessage()`
- **SSRF protection**: webhook/bark Send methods call `validateOutboundHTTPURL()` with `externalURLPolicy()`

#### `internal/core/notification_test.go` (NEW)
30+ test functions covering:

| Category | Tests |
|----------|-------|
| Config parsing | Valid JSON, minimal config, invalid JSON, default config |
| Per-channel Validate | webhook (url+hmac), telegram (token+chatId), bark (url), serverchan (sendKey), email (smtpHost/from/to) |
| Level/Mode matching | `levelMatchesMode` all/success/failure/unknown combos |
| Type/Level filtering | `shouldSendToChannel` with types, levels, empty lists, edge cases |
| HTTP send | webhook success (httptest.Server), HMAC signature verification, digest mode (FlushDigest), mode filter |
| Dispatch | disabled config (0 calls), level filter (info vs warning) |
| Config lifecycle | reload missing row (no panic, default fallback), decrypt failure fallback |
| Encrypt/Decrypt roundtrip | webhook hmacSecret, telegram botToken, serverchan sendKey, email password |
| Utilities | maskSensitiveField, truncateNotifyContent, stringInSlice, buildNotifyBody, buildEmailMessage |
| Channel factory | valid webhook, digest init, empty Config → nil, unknown type → nil |
| Channel metadata | EncryptedFields, Type() for all 5 channels |
| Health check | disabled/ok, enabled-no-channels/warning, all-disabled/warning, some-enabled/ok |
| Integration | notify() → dispatchNotification triggers HTTP call, ensureDefaultSettings includes notification.channels |

### Files Modified

#### `internal/core/routes.go` (EDIT)
- `notify()` function: appended `go a.dispatchNotification(kind, level, title, content)` after `a.invalidateReadCache()` to asynchronously distribute notifications to external channels.

#### `internal/core/app.go` (EDIT)
- **App struct** (line ~33): Added `notificationConfig notificationChannelsConfig` field
- **ensureDefaultSettings()** (line ~179): Added `"notification.channels"` key with default config containing 5 disabled channels (webhook/telegram/bark/serverchan/email)
- **NewApp()** (line ~126): Added `reloadNotificationConfig()` call after `reloadNetworkProxyConfig()`

#### `internal/core/system.go` (EDIT)
- **handleUpdateSystemSettings()**: Added `notification.channels` branch between `network.proxy` branch and `tx.ExecContext` — parses config, encrypts secrets, normalizes JSON, logs warnings
- Added `_ = a.reloadNotificationConfig(r.Context())` after `reloadNetworkProxyConfig()` to refresh in-memory config after save
- Added `"log"` import

#### `internal/core/health.go` (EDIT)
- **healthStatus()**: Added `a.healthCheckNotificationChannels()` to checks slice
- New `healthCheckNotificationChannels()`: disabled→ok, enabled-but-no-channels→warning, enabled-with-channels→ok with count
- Added `"fmt"` import

#### `internal/core/models.go` (EDIT)
- Added `NotificationChannelStatus` struct (Type/Name/Enabled/ConfigValid/Levels) for future frontend exposure

#### `internal/core/python_migration_test.go` (EDIT)
- Updated default settings count assertions from `5` to `6` (new `notification.channels` default key)

### Dependencies
- **Zero new Go module dependencies**. All channel implementations use only the Go standard library (`net/http`, `crypto/hmac`, `crypto/sha256`, `crypto/tls`, `net/smtp`, `mime`, `encoding/json`, `encoding/hex`, etc.)

### Verification
- [x] `go build -mod=vendor ./...` — compiles without errors
- [x] `go vet ./internal/core/...` — no warnings
- [x] `go test -mod=vendor ./internal/core/...` — all new notification tests pass; 2 pre-existing TempDir cleanup failures unrelated to this change
- [x] All sensitive fields use `a.encryptText` for storage, `a.decryptText` for loading
- [x] HMAC Secret never logged (no string interpolation of secret in log.Printf)
- [x] Digest mode implemented only on webhookChannel (not on other channels)
- [x] Config validation on save rejects malformed entries but doesn't block saving
- [x] Health check reflects notification channel enable status
