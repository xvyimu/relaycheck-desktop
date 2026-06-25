# RelayCheck Desktop

**RelayCheck Desktop** is a local operations console for [NewAPI](https://github.com/Calcium-Ion/new-api), OneAPI, Sub2API, and compatible relay sites. Manage accounts, check-ins, balances, upstream detection, notifications, encrypted backups, and local NewAPI synchronization — all from a single local desktop app.

**Stack:** Go `net/http` backend + React 19 / Vite frontend + SQLite (embedded via modernc.org/sqlite). Single binary, no external server required.

> Official display name: `RelayCheck Desktop v1.1.0`.

---

## Quick Start

### Prerequisites

- Go 1.24+
- Node.js 20+
- npm

### Build & Run

```bash
# 1. Build frontend assets
cd frontend
npm ci
npm run build
cd ..

# 2. Build desktop binary
go build -mod=vendor -o dist/relaycheck.exe .

# 3. Run
./dist/relaycheck.exe
```

Then open **http://127.0.0.1:3001** in your browser.

> On first launch, the bootstrap admin password is printed to the console or written to `data/bootstrap-admin-password.txt`.

### Run Tests

```bash
go test -mod=vendor ./...
go vet ./...
cd frontend && npx tsc --noEmit
```

## Core Documents

| Document | Purpose |
|----------|---------|
| `docs/PROJECT_STRUCTURE.md` | Current source tree, generated paths, archive boundary, and verification order. |
| `DESIGN_SYSTEM.md` | Control Room visual direction and UI rules. |
| `PRODUCT_RESEARCH_AND_REQUIREMENTS.md` | Product research, requirements, roadmap, and risk register. |

## Runtime

| Layer | Technology |
|------|------------|
| Desktop/server | Go `net/http`, single embedded executable |
| Frontend | React 19 + Vite, embedded into the Go binary |
| Storage | SQLite at `data/relaycheck.db` |
| Default URL | `http://127.0.0.1:3001` |
| Bootstrap login | `admin` plus `RELAYCHECK_BOOTSTRAP_PASSWORD`; if unset on a fresh DB, read `data/bootstrap-admin-password.txt` |
| Design direction | Control Room: calm, compact, precise, low-noise |

## Architecture

```mermaid
flowchart TD
    User["User"]
    Launcher["Root launchers"]
    Desktop["relaycheck.exe"]
    Router["Go HTTP router"]
    Frontend["Embedded React/Vite UI"]
    DB["SQLite relaycheck.db"]
    Scheduler["Scheduler"]
    BrowserAuth["Browser auth runtime"]
    Upstream["NewAPI / OneAPI / Sub2API sites"]
    Notifications["Notifications"]
    Analytics["Analytics engine"]

    User --> Launcher
    Launcher --> Desktop
    Desktop --> Router
    Router --> Frontend
    Router --> DB
    Router --> Scheduler
    Router --> BrowserAuth
    Router --> Notifications
    Router --> Analytics
    Scheduler --> DB
    Scheduler --> Upstream
    BrowserAuth --> Upstream
    Router --> Upstream
```

## Route Overview

| Group | Endpoints |
|-------|-----------|
| System | `/api/system/status`, `/version-check`, `/autostart`, `/legacy-check`, `/port-check`, `/settings`, `/scheduler-status`, `/proxy-test`, `/diagnostics`, `/action-center`, `/audit-log`, `/backups`, `/backup`, `/export`, `/import`, `/exports`, `/restore`, `/migrate-from-python-db`, `/migrate-python-db` |
| Scheduler | `/api/scheduler/channel-schedules`, `/calendar`, `/next-runs` |
| Tasks | `/api/tasks/start`, `/tasks/{id}/stream`, `/tasks/{id}/cancel`, `/tasks/dry-run` |
| Analytics | `/api/analytics` (balance trend, checkin distribution, response times, site reliability, balance deltas) |
| Auth | `/api/auth/login`, `/logout`, `/session` |
| Sites | `/api/upstream-sites`, `/bulk-detect` |
| Channels | `/api/channels`, `/bulk-source-status`, `/models/overview`, `/models/sync` |
| Accounts | `/api/accounts`, `/bulk-open-browser-login`, `/bulk-finish-browser-login`, `/bulk-password-login`, `/bulk-test-api-keys`, `/bulk-refresh-balances`, `/import-legacy-config`, `/import-chrome-passwords/preview`, `/import-chrome-passwords/import` |
| Checkins | `/api/checkins/today`, `/logs`, `/status`, `/run-all` |
| Balances | `/api/balances/snapshots`, `/api/usage/overview` |
| Models | `/api/models/overview`, `/sync`, `/pricing`, `/pricing/sync` |
| Keys | `/api/keys/export-preview` |
| Notifications | `/api/notifications`, `/mark-all-read`, `/clear-read`, `/mark-read` |
| Local NewAPI | `/api/local-newapi`, `/scan`, `/import-from-sqlite`, `/import-from-admin-api` |
| Health | `/api/health` (unauthenticated) |

## Commands

Run from `E:\zidqiandao\relaycheck-desktop`.

| Command | Purpose |
|---------|---------|
| `cd frontend; npm ci --cache E:\zidqiandao\.npm-cache; npm run build` | Install frontend dependencies and build embedded assets. |
| `cd frontend; $env:RELAYCHECK_SMOKE_PASSWORD='<local password>'; npm run smoke` | Run the browser smoke test against a running local desktop server. |
| `go test -mod=vendor ./...` | Run Go test suite using vendored dependencies, including security, audit, health, and SSRF checks. |
| `go build -mod=vendor -ldflags="-H windowsgui" -o dist\relaycheck.exe .` | Build the Windows desktop executable. |
| `go vet ./...` | Run static analysis (zero warnings expected). |
| `cd frontend; npx tsc --noEmit` | TypeScript type check (zero errors expected). |

Run `npm run build` before Go compilation if `frontend/dist/` is missing; `main.go` embeds that directory at compile time.

## Race / cgo Note

The Windows Go environment used for this workspace currently does not enable cgo. Because Go's race detector requires cgo on this platform, `go test -race ./internal/core` is documented as blocked here with `-race requires cgo`. Use `go test -mod=vendor ./...` as the required local regression gate unless cgo is explicitly enabled in a future toolchain setup.

## Verification Checklist

- `go test -mod=vendor ./...`
- `go vet ./...`
- `cd frontend && npm run build`
- `cd frontend && npx tsc --noEmit`
- `cd frontend && npm audit --audit-level=low`
- Browser smoke on desktop and 390px mobile width: set `RELAYCHECK_SMOKE_PASSWORD`, start the desktop server, then run `cd frontend && npm run smoke`
- No real secrets, passwords, tokens, cookies, or API keys in diffs

## Credential And Export Safety

- Credentials are stored locally in encrypted columns such as `password_encrypted`, `cookie_encrypted`, `access_token_encrypted`, `refresh_token_encrypted`, and `api_key_encrypted`.
- The encryption envelope is AES-GCM with a local instance key stored under `data/keys/instance.key`; encrypted values use the `v1.<nonce>.<ciphertext>` format.
- Encrypted zip export/import uses AES-256-GCM with PBKDF2-SHA256 key derivation (200,000 iterations + random 32-byte salt). The RCZIP2 format is current; RCZIP1 (legacy raw SHA-256) is supported for backward-compatible decryption only.
- Zip import is protected against zip-bomb attacks: total decompressed content is capped at 256 MB, individual entries at 200 MB.
- On a fresh database, the bootstrap admin password is taken from `RELAYCHECK_BOOTSTRAP_PASSWORD`; if that is not set, a generated local password is written under `data/bootstrap-admin-password.txt`, which is ignored by Git.
- API key sharing/export surfaces must only expose fingerprints, masked references, model status, and diagnostic metadata.
- Real passwords, cookies, access tokens, refresh tokens, sync tokens, channel keys, and API keys must never be returned by export endpoints or written into documentation, logs, screenshots, or temporary handoff files.

## Notification Channels

| Channel | Modes | Config Fields |
|---------|-------|---------------|
| Webhook | all / failure / success | URL, HMAC secret, timeout, max retries (exp. backoff: 1s/2s/4s/8s/16s) |
| Telegram | all / failure | Bot token, chat ID |
| Bark | all / failure | URL, group |
| ServerChan | all / failure | SendKey |
| Email (SMTP) | all / failure | SMTP host/port/TLS, username, password, from/to |
| Desktop | all / failure / warning+ | In-app notification with browser Notification API push |

## Analytics

The `/api/analytics?days=N` endpoint provides:

- **Balance trend**: daily average balance (excludes NULL/zero via `AVG(NULLIF(balance, 0))`)
- **Checkin distribution**: 7-day status breakdown (success/already/failed/unsupported/expired)
- **Response times**: API key latency per account
- **Site reliability**: per-site success rate and average latency
- **Balance deltas**: day-over-day change with cumulative total
- **Date range**: selectable 7/30/90 days
- **Drilldown**: click chart points to see per-day details

## Maintenance Notes

- Keep changes focused on this directory unless a task explicitly targets the legacy Python or experimental Next.js implementations.
- Preserve the existing SQLite data file unless a migration task includes a backup and rollback plan.
- Use `docs/PROJECT_STRUCTURE.md` as the source-tree map before deleting or moving files.
- Follow `DESIGN_SYSTEM.md` when changing visual surfaces.
- Update `AGENT_HANDOFF.md`, `progress.md`, and `task_plan.md` after substantial implementation work.
- Use `PROMPT_CHECKLIST.md` as the master checklist for the original optimization prompt; mark each item complete there as work lands.
- `/api/health` is intentionally unauthenticated for local startup/smoke checks; business `/api/*` routes remain session-protected.
- External outbound URLs are validated against SSRF rules by default. Only explicit trusted local probes may opt into loopback/private addresses.
- The scheduler uses `time.FixedZone("CST", 8*3600)` (UTC+8) for consistent scheduling regardless of server timezone.
