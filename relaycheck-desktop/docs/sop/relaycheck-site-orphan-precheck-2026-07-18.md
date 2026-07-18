# Site orphan precheck (read-only)

- **Date:** 2026-07-18
- **Status:** Shipped as dry-run tooling only. **Does not delete or migrate.**

## Purpose

Historical site deletes (pre application-level cascade) may have left child rows whose `upstream_site_id` / `site_id` no longer exists in `upstream_sites`. This precheck counts those orphans so operators can decide whether a later cleanup is needed.

## What it checks

| Table | Join key | Notes |
|---|---|---|
| `channel_accounts` | `upstream_site_id` | empty id ignored |
| `checkin_logs` | `upstream_site_id` | empty id ignored |
| `balance_snapshots` | `upstream_site_id` | empty id ignored |
| `channel_schedules` | `upstream_site_id` | excludes `__global__` |
| `site_pricing_cache` | `site_id` | empty id ignored |

## How to run

```powershell
cd E:\zidqiandao\relaycheck-desktop

# Default DB: data\relaycheck.db
powershell -NoProfile -File .\scripts\precheck-site-orphans.ps1

# Explicit path + JSON (if sqlite3 supports .mode json)
powershell -NoProfile -File .\scripts\precheck-site-orphans.ps1 -DbPath "D:\backup\relaycheck.db"

# Raw SQL
sqlite3 -readonly data\relaycheck.db ".read scripts/sql/precheck-site-orphans.sql"
```

Requires `sqlite3` CLI on PATH (or common install paths). Opens DB with `-readonly`.

## Safety

- **Never** deletes rows.
- **Never** touches `data/` lifecycle (no create/remove of the data directory).
- Cleanup of orphans still requires: Settings backup → explicit product confirm → separate maintenance SQL (not shipped).

## Related

- Cascade delete (app-level): `docs/sop/relaycheck-site-delete-cascade-2026-07-18.md`
- UI delete entry: Sites 卡片 / 详情抽屉 / 主从「删除站点」→ `sitesApi.remove`
- Cleanup dry-run (checkin_logs only): `docs/sop/relaycheck-checkin-orphan-cleanup-dry-run-2026-07-18.md`
