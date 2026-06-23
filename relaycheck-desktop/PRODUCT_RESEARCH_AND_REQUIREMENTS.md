# RelayCheck Desktop Product Research And Requirements

Last updated: 2026-06-23

This document turns the June 2026 online research pass into concrete product requirements for RelayCheck Desktop. It is intentionally product-facing and implementation-aware: every requirement below should map back to either the current Go/React codebase or a clearly marked future phase.

## 1. Strategic Decision

RelayCheck Desktop should be a local operations console for NewAPI, OneAPI, Sub2API, and modified relay sites.

It should not become another public API gateway. New API, One API, Sub2API, LiteLLM, Portkey Gateway, and AxonHub already compete in that layer. RelayCheck wins by staying local, quiet, encrypted, single-binary, and focused on day-to-day account and channel operations.

The short version:

- Manage many relay sites and accounts.
- Detect account, key, model, balance, check-in, and channel health problems.
- Keep credentials local and encrypted.
- Offer exports and integration helpers, but avoid hosting a public relay service.
- Prefer fast local diagnosis over heavy multi-tenant gateway features.

## 2. External Project Landscape

The table uses GitHub API data checked on 2026-06-23. Star counts are rounded for readability.

| Project | Positioning | Stars | License | Activity Signal | RelayCheck Lesson |
|---|---:|---:|---|---|---|
| [New API](https://github.com/QuantumNous/new-api) | Unified AI model aggregation and distribution gateway | 39.8k | AGPL-3.0 | Updated 2026-06-23 | Treat as a primary upstream object to detect, import, sync, and diagnose. Do not copy AGPL code. |
| [One API](https://github.com/songquanpeng/one-api) | Mature LLM API management and redistribution system | 35.2k | MIT | Updated 2026-01-09 | Keep compatibility because many relay sites still inherit its schema or behavior. |
| [Sub2API](https://github.com/Wei-Shaw/sub2api) | Subscription-to-API relay platform | 28.8k | LGPL-3.0 | Updated 2026-06-23 | Account authorization, subscription validity, cost sharing, and failed login states are first-class signals. |
| [all-api-hub](https://github.com/qixing-jk/all-api-hub) | NewAPI/Sub2API account hub | 4.3k | AGPL-3.0 | Updated 2026-06-23 | Closest functional reference: balance, usage, auto check-in, key use, price comparison, health checks. |
| [Metapi](https://github.com/cita-777/metapi) | Aggregate many relay sites into one API key and one entry point | 2.9k | MIT | Updated 2026-06-21 | Good reference for model discovery, cost-aware routing, and site aggregation. RelayCheck should integrate with this pattern, not replace it. |
| [one-api-hub](https://github.com/fxaxg/one-api-hub) | Browser extension for relay account balance, model, and key management | 355 | MIT | Updated 2025-09-05 | Confirms demand for no-repeat-login account visibility. RelayCheck can provide the local desktop version of this job. |
| [LiteLLM](https://github.com/BerriAI/litellm) | Python SDK and AI gateway proxy for 100+ LLM APIs | 51.3k | No assertion in API response | Updated 2026-06-23 | Borrow ideas from budgets, logging, routing, and cost tracking, but avoid enterprise gateway complexity. |
| [Portkey Gateway](https://github.com/Portkey-AI/gateway) | AI gateway with routing and guardrails | 12.2k | MIT | Updated 2026-05-25 | Guardrails and route policies are useful references for future exports, not core local operations. |
| [AxonHub](https://github.com/looplj/axonhub) | AI gateway with failover, load balancing, cost control, tracing | 4.4k | No assertion in API response | Updated 2026-06-23 | Tracing and failover language is useful for diagnostics, but RelayCheck should keep requests user-triggered where credentials or cost are involved. |

## 3. Product Boundary

RelayCheck owns the local operator layer.

Owned:

- Local site inventory.
- Local account inventory.
- Encrypted credential storage.
- Check-in status and scheduling.
- Balance and usage snapshots.
- API key model testing and summary storage.
- Channel import, status, model sync, and archive/restore.
- Model pricing and comparison.
- Notifications, audit log, backup, restore, diagnostics, and action center.
- Browser smoke and local health verification.

Not owned:

- Public API redistribution.
- Multi-tenant billing.
- Hosted SaaS dashboard.
- Third-party key custody.
- Automatic model speed testing without explicit user action.
- Sending user keys to third-party benchmark sites.
- Deep enterprise gateway controls such as policy engines, guardrail marketplaces, and tenant quotas.

Integration targets:

- New API / One API compatible admin APIs.
- New API / One API SQLite databases.
- Sub2API account and subscription surfaces.
- Local NewAPI instances discovered on loopback.
- Future export or handoff surfaces for Metapi, LiteLLM, and similar gateways.

## 4. Primary Users

### 4.1 Personal Relay Operator

This user runs or uses multiple relay sites and wants one local place to see:

- Which accounts still work.
- Which accounts need login or browser authorization.
- Which balances are low.
- Which sites can check in today.
- Which keys have usable models.
- Which channels are stale, missing, archived, or active.

Success means they can open RelayCheck in the morning and know what to fix first.

### 4.2 Small Team Maintainer

This user manages a shared set of relay accounts for a small internal team. They need:

- A single machine-local source of truth.
- Audit records for sensitive operations.
- Backup and restore.
- Clear export previews that do not leak real keys.
- Health status that can be explained to non-specialists.

Success means the maintainer can answer "what broke, when, and what should I do" without opening five admin panels.

### 4.3 Migration User

This user has old Python scripts, NewAPI SQLite data, browser password CSVs, or manual config files. They need:

- Safe import preview.
- Explicit confirmation before writing credentials.
- Clear skipped/changed/new/removed counts.
- No accidental deletion of existing data.

Success means they can migrate without losing accounts, keys, or site history.

## 5. Current Verified Capability Map

The current codebase already exposes the following route groups in `internal/core/routes.go`:

- Auth: `/api/auth/login`, `/api/auth/logout`, `/api/auth/session`
- Health: `/api/health`
- System: status, settings, scheduler, proxy test, diagnostics, action center, audit log, backups, restore, Python DB migration
- Local NewAPI: list, scan, SQLite import, Admin API import, instance operations
- Channels: list, bulk source status, model overview, model sync, item operations
- Upstream sites: list, bulk detect, item operations
- Accounts: list, browser login, password login, key tests, balance refresh, legacy import, Chrome password CSV import, item operations
- Models: overview, sync, pricing, pricing sync
- Key export preview
- Check-ins: today, logs, status, run all
- Usage overview
- Balance snapshots
- Notifications: list, mark all read, clear read

The active React shell currently mounts:

- Dashboard with `HubRadar`
- Channels with `ChannelTable`, `useChannelActions`, and `useChannelFilters`
- Sites as a lightweight recovery view
- Accounts with insights, form, cards, and detail content
- Check-ins as a lightweight recovery view
- Notifications as a lightweight recovery view
- Settings with diagnostics, backup, scheduler, audit, proxy, and help surfaces

This means the next product work should enrich Sites, Check-ins, and Notifications first before expanding into new domains.

## 6. Detailed Requirements

### R1. Dashboard And Action Center

Goal: make the first screen answer "what needs attention now".

Current data:

- `/api/system/status`
- `/api/system/diagnostics`
- `/api/system/action-center`
- `/api/models/overview`
- `/api/models/pricing`
- `/api/usage/overview`
- `/api/checkins/status`

Required details:

- Show overall system health, not only raw counts.
- Separate "healthy" signals from "needs action" signals.
- Link each action item to the correct target tab and filter.
- Prioritize auth expired, failed check-ins, invalid keys, low balance, stale model sync, missing channels, backup risk, and scheduler problems.
- Keep cards compact on desktop and readable at 390px mobile width.

Acceptance:

- Smoke test can load Dashboard after login.
- Dashboard has no horizontal overflow on desktop or 390px viewport.
- Each action item has a stable target and does not depend on parsing display text.
- No secret value appears in action item descriptions.

### R2. Sites

Goal: turn Sites from a lightweight list into the control surface for upstream relay sites.

Required fields:

- Display name
- Base URL
- Login URL
- Kind or platform, including NewAPI, OneAPI, Sub2API, modified relay, and unknown
- Detection status
- Check-in support
- Balance support
- Admin API or SQLite import status
- Last detected time
- Linked account count
- Linked channel count

Required interactions:

- Add site.
- Edit site.
- Detect site kind.
- View linked accounts.
- View linked channels.
- Filter by kind, status, support flags, and missing diagnostics.
- Show explicit empty states for no sites, no filtered result, and detection failure.

Acceptance:

- Site detection must not send credentials to third-party services.
- External URL probing must keep SSRF protections from `url_safety.go`.
- Local loopback probing is allowed only for explicit trusted local scans.
- User can distinguish "unsupported", "not checked", and "failed".

### R3. Accounts

Goal: make account health, balance, check-in, and key state scannable without opening every site.

Required fields:

- Site
- Display name
- Email or username
- Auth type
- Login status
- Check-in support
- Last check-in result
- Balance amount and unit
- API key fingerprint and key status
- Model count and last model test time
- Last browser authorization time

Required interactions:

- Add account.
- Edit account.
- Clear stored key or credential with confirmation.
- Open browser login.
- Finish browser login.
- Run password login.
- Refresh balance.
- Run API key model test.
- Import legacy config.
- Preview and import Chrome password CSV.

Acceptance:

- Password, cookies, tokens, and real API keys are never shown after save.
- Search may index display name, email, username, site name, and base URL, but not encrypted secret fields.
- Deleting an account requires explicit confirmation and explains data impact.
- Key export preview only returns fingerprints, model state, and diagnostic metadata.

### R4. Channels

Goal: make imported NewAPI/OneAPI/Sub2API channels maintainable over time.

Required fields:

- Channel ID
- Source instance
- Source channel ID
- Name
- Base URL
- Status: active, missing, archived
- Backend kind
- Provider or platform
- Model count
- Key status summary
- Last source sync
- Last model sync
- Linked account hints

Required interactions:

- Import from local NewAPI SQLite.
- Import from Admin API.
- Preview new, changed, unchanged, skipped, and removed channels before writing.
- Mark missing channels.
- Archive and restore channels.
- Sync channel models.
- Filter by status, backend kind, platform, source, account hints, and search text.

Acceptance:

- Removed upstream channels become missing or archived; they are not physically deleted by default.
- `rawJson` parsing must use a whitelist for searchable note/platform fields.
- Secret-looking raw fields must not enter search text.
- Bulk actions need confirmation when hiding or restoring many channels.

### R5. Check-ins

Goal: make daily check-in operations reliable and explainable.

Required fields:

- Running status
- Current account and site
- Total, processed, pending
- Success, already checked, failed, unsupported, auth expired
- Next scheduled run
- Last run message
- Today summary
- Retry count per result

Required interactions:

- Run all check-ins.
- Run one account check-in from account context.
- Filter history by site, account, result, date, and message.
- Show retry attempts when temporary network failures recover or fail.

Acceptance:

- Temporary failures retry only for network errors and HTTP 408, 429, and 5xx.
- Auth failures and ordinary 4xx must not retry blindly.
- Same-site minimum interval must be respected.
- Check-in logs should explain candidate endpoint failures without leaking credentials.

### R6. Models, Pricing, And Keys

Goal: show which keys and channels can actually serve useful models at what likely cost.

Required fields:

- Model ID
- Channel
- Account or key fingerprint
- Provider or model family
- Availability status
- Latency summary when user-triggered test exists
- Prompt and completion price when available
- Pricing source and confidence

Required interactions:

- Sync model overview.
- Sync pricing.
- Test one key.
- Bulk test selected keys with safe concurrency.
- Preview export.

Acceptance:

- Model tests only call the user-configured upstream relay site.
- No third-party benchmark service receives user keys.
- Test result storage keeps summary data, not full upstream response bodies.
- Price comparison must label unknown or inferred prices instead of pretending precision.

### R7. Settings, Backup, Restore, And Security

Goal: keep the local tool recoverable and auditable.

Required fields:

- Product version
- Data path
- Backup path
- Scheduler status
- Network proxy status
- Diagnostics summary
- Audit log summary
- Bootstrap login note

Required interactions:

- Save structured settings.
- Test proxy.
- Create backup.
- Delete selected backups.
- Restore one backup.
- View audit log.
- Run diagnostics.

Acceptance:

- Backup restore only accepts files from the backup directory unless a future import flow explicitly validates external paths.
- Audit metadata records counts, resource IDs, and booleans, not passwords, cookies, tokens, or API keys.
- Bootstrap admin password is read from `RELAYCHECK_BOOTSTRAP_PASSWORD` or generated into ignored local data.
- README and handoff docs must never include real local passwords.

### R8. Notifications

Goal: make operational events visible without turning successful routine events into noise.

Required fields:

- Type
- Level
- Title
- Content
- Related resource type and ID
- Read status
- Created time

Required interactions:

- List important notifications first.
- Expand or include ordinary success/info history separately.
- Mark all read.
- Clear read notifications with confirmation.
- Route scheduled failures to configured notification channels.

Acceptance:

- Warning and error notifications are never hidden behind ordinary success noise.
- Clearing read notifications must state that unread notifications remain.
- External notification channel configs must encrypt secret fields.

## 7. Roadmap

### P0. Stabilized Baseline

Status: complete.

Scope:

- Restore frontend build.
- Exclude damaged archives.
- Mount recovered Dashboard, Settings, Accounts, and Channels components.
- Add repeatable Playwright smoke.
- Keep Go tests and build green.
- Create Git baseline.

### P1. Complete Active Domain Surfaces

Scope:

- Rebuild Sites as a full domain component.
- Rebuild Check-ins as a full domain component.
- Rebuild Notifications as a full domain component.
- Add smoke selectors for these three pages.
- Add focused unit tests for new backend behavior only when backend changes are made.

Why:

- These are currently the remaining lightweight recovery views.
- They map directly to the strongest external project lesson: operators need issue-first account and site visibility.

### P2. Adapter And Import Maturity

Scope:

- Harden NewAPI/OneAPI SQLite import.
- Harden Admin API import.
- Add Sub2API-specific detection and status fields where confirmed by source behavior.
- Improve removed/missing channel workflows.
- Add import dry-run reports that can be copied into issue reports without secrets.

Why:

- New API, One API, and Sub2API are the primary upstream ecosystems.
- Import reliability is more valuable than another generic dashboard widget.

### P3. Operational Intelligence

Scope:

- Improve action center scoring.
- Add low-balance thresholds.
- Add stale model sync warnings.
- Add failed check-in triage.
- Add auth-expired grouping.
- Add model-price confidence labels.

Why:

- all-api-hub and Metapi show users care about balance, model, key, and cost surfaces.
- RelayCheck should turn those into local "what to fix next" views.

### P4. Gateway Interop Without Becoming A Gateway

Scope:

- Export sanitized channel/account/model summaries.
- Generate integration hints for New API, Metapi, LiteLLM, or similar gateways.
- Keep real credentials behind explicit encrypted export flows only.

Why:

- Gateway projects are strong and numerous.
- RelayCheck should help feed or audit them, not duplicate their runtime.

### P5. Local Reliability And Maintenance

Scope:

- Add more smoke assertions.
- Add backup restore smoke.
- Add migration fixture tests.
- Add docs for troubleshooting port conflicts, cgo race-test limitations, and Chrome executable selection.
- Keep package-lock and vendored Go dependencies reproducible.

Why:

- This is a personal local operations tool. Reliability beats feature breadth.

## 8. Design Principles

- Issue-first, not marketing-first.
- Compact cards over large hero panels.
- Stable dimensions for scan-heavy surfaces.
- Status text plus shape, not color alone.
- User-triggered network actions where credentials or cost may be involved.
- No secret values in documentation, logs, screenshots, audit metadata, or export previews.
- Local default: bind to `127.0.0.1`.
- Keep dependencies minimal unless a feature proves real operational value.

## 9. Risk Register

| Risk | Impact | Mitigation |
|---|---|---|
| AGPL projects used as references | License contamination if code is copied | Treat New API and all-api-hub as product references only; do not copy code. |
| Upstream schemas vary | Import bugs and wrong channel states | Prefer schema introspection, raw JSON snapshots, and explicit confidence labels. |
| Credential leakage | Severe local trust failure | Keep AES-GCM encrypted storage, masked previews, secret scans, and audit metadata limits. |
| Automatic checks consume balance | User cost surprise | Keep model speed and key tests user-triggered; batch limits remain clamped. |
| Public gateway feature creep | Product becomes too heavy | Keep gateway interop as export/integration, not runtime serving. |
| Windows file locking | Flaky tests or backup restore issues | Keep explicit app close paths, SQLite checkpointing, and repeated regression runs. |
| CJK encoding damage | Documentation and UI corruption | Keep active source clean, ignore `_archive`, and avoid editing damaged historical files unless restoring from known-good source. |

## 10. Verification Contract

Required local verification before calling a change complete:

```powershell
cd E:\zidqiandao\relaycheck-desktop\frontend
node --check scripts\smoke.mjs
npm run build
npm audit --audit-level=low

cd E:\zidqiandao\relaycheck-desktop
go test -mod=vendor ./...
go build -mod=vendor -ldflags="-H windowsgui" -o dist\relaycheck.exe .
```

Browser smoke:

```powershell
cd E:\zidqiandao\relaycheck-desktop\frontend
$env:RELAYCHECK_SMOKE_PASSWORD='<local password>'
npm run smoke
```

Security scan scope:

- Include active source and project docs.
- Exclude `node_modules`, `dist`, `data`, `_archive`, `vendor`, and `.pipeline/test-results`.
- Scan for real passwords, API keys, private keys, cookies, access tokens, and known token prefixes.

## 11. Source Notes

External sources checked on 2026-06-23:

- https://github.com/QuantumNous/new-api
- https://github.com/songquanpeng/one-api
- https://github.com/Wei-Shaw/sub2api
- https://github.com/qixing-jk/all-api-hub
- https://github.com/cita-777/metapi
- https://github.com/fxaxg/one-api-hub
- https://github.com/BerriAI/litellm
- https://github.com/Portkey-AI/gateway
- https://github.com/looplj/axonhub

Local source files checked before writing:

- `README.md`
- `internal/core/routes.go`
- `internal/core/models.go`
- `frontend/package.json`
- `frontend/src/main.tsx`

