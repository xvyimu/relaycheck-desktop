# SQLite foreign keys & site-delete semantics

- **Date:** 2026-07-18
- **Status:** Application-level cascade **implemented** for site delete. Full schema FK migration for historical tables remains **deferred** (see risk).

## Current facts (code)

1. DSN enables `foreign_keys(1)` at open (`internal/core/app.go` `openAppDB`).
2. Only `channel_schedules.upstream_site_id` declares `REFERENCES upstream_sites(id) ON DELETE CASCADE` in `migrate()`.
3. `channel_accounts`, `checkin_logs`, `balance_snapshots`, `site_pricing_cache` historically **lack** FK clauses.
4. **Before this change:** `DeleteUpstreamSite` ran a single `DELETE FROM upstream_sites`, leaving orphan accounts/logs/balances/schedules (schedules may cascade only if FK active + declared).
5. **After this change:** `sites.Service.DeleteUpstreamSite` deletes, in one transaction:
   - `checkin_logs` by `upstream_site_id`
   - `balance_snapshots` by `upstream_site_id`
   - `channel_accounts` by `upstream_site_id`
   - `channel_schedules` by `upstream_site_id`
   - `site_pricing_cache` by `site_id`
   - `upstream_sites` by `id`
6. Response returns cascade counts; missing site → 404; `__global__` compatibility id rejected.
7. Frontend: `sitesApi.remove(id)` typed DELETE owner.

## Explicitly not done (still needs product confirm + backup)

- Adding FK constraints to existing tables via rebuild migration (SQLite rewrite).
- One-shot orphan cleanup of historical rows already left behind by old deletes.
- Soft-delete / archive-site product mode.
- Touching real `data/relaycheck.db` contents in this session.

## Operator checklist before enabling aggressive UI delete

1. Backup: Settings → 立即备份，或 copy `data\relaycheck.db*`.
2. Prefer API/UI delete only after cascade is deployed.
3. If orphans already exist, run a **future** maintenance query after backup (not shipped as auto-migration here).

## Tests

- `go test -mod=vendor ./internal/sites -run DeleteUpstream`
- Frontend `sitesApi.remove` contract test
