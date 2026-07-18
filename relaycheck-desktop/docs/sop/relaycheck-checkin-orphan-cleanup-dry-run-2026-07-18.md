# checkin_logs orphan cleanup — dry-run only

- **Date:** 2026-07-18
- **Status:** Dry-run SQL shipped. **No mutating cleanup executed.**

## Context

Read-only precheck on local `data/relaycheck.db` (2026-07-18) found:

| table | orphan_rows |
|---|---:|
| channel_accounts | 0 |
| **checkin_logs** | **18** |
| balance_snapshots | 0 |
| channel_schedules | 0 |
| site_pricing_cache | 0 |

These are historical debris from site deletes before application-level cascade.

## Dry-run (safe)

```powershell
cd E:\zidqiandao\relaycheck-desktop

# Prefer sqlite3 if available
sqlite3 -readonly data\relaycheck.db ".read scripts/sql/cleanup-checkin-log-orphans.dry-run.sql"

# Or Python readonly (no sqlite3 CLI)
python -c "import sqlite3,os; uri='file:'+os.path.abspath('data/relaycheck.db').replace('\\\\','/')+'?mode=ro'; c=sqlite3.connect(uri,uri=True); print(c.execute('''SELECT COUNT(*) FROM checkin_logs l WHERE COALESCE(TRIM(l.upstream_site_id),'')<>'' AND NOT EXISTS (SELECT 1 FROM upstream_sites s WHERE s.id=l.upstream_site_id)''').fetchone()[0])"
```

SQL file: `scripts/sql/cleanup-checkin-log-orphans.dry-run.sql`  
Lists summary count, up to 50 sample rows (`started_at`/`finished_at`), and distinct missing site ids.

Local dry-run snapshot (2026-07-18, readonly): **18** orphan logs for **1** missing `upstream_site_id`; sample JSON under gitignored `docs/perf/samples/orphan-checkin-logs-dry-run-2026-07-18.json` when regenerated.

## Mutating cleanup (blocked until explicit confirm)

1. Settings → 立即备份，或 copy `data\relaycheck.db*`.
2. Operator says clearly: allow cleanup of orphan checkin_logs only.
3. Then run the commented `DELETE` inside a transaction after re-counting.
4. Do **not** broaden to other tables without a fresh precheck.

## Non-goals

- Full FK migration / table rebuild
- Auto-run on app start
- Deleting accounts, balances, schedules, pricing cache (already 0 orphans here)
