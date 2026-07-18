-- Read-only historical orphan precheck for RelayCheck Desktop.
-- Does NOT mutate data. Run after backing up if you plan a later cleanup.
--
-- Orphan definition: child rows whose upstream_site_id / site_id no longer
-- exists in upstream_sites (or is empty/null where not expected).
--
-- sqlite3 example:
--   sqlite3 data/relaycheck.db < scripts/sql/precheck-site-orphans.sql

.mode column
.headers on

SELECT 'channel_accounts' AS table_name,
       COUNT(*) AS orphan_rows
FROM channel_accounts a
WHERE COALESCE(TRIM(a.upstream_site_id), '') <> ''
  AND NOT EXISTS (SELECT 1 FROM upstream_sites s WHERE s.id = a.upstream_site_id)

UNION ALL
SELECT 'checkin_logs',
       COUNT(*)
FROM checkin_logs l
WHERE COALESCE(TRIM(l.upstream_site_id), '') <> ''
  AND NOT EXISTS (SELECT 1 FROM upstream_sites s WHERE s.id = l.upstream_site_id)

UNION ALL
SELECT 'balance_snapshots',
       COUNT(*)
FROM balance_snapshots b
WHERE COALESCE(TRIM(b.upstream_site_id), '') <> ''
  AND NOT EXISTS (SELECT 1 FROM upstream_sites s WHERE s.id = b.upstream_site_id)

UNION ALL
SELECT 'channel_schedules',
       COUNT(*)
FROM channel_schedules c
WHERE COALESCE(TRIM(c.upstream_site_id), '') <> ''
  AND c.upstream_site_id <> '__global__'
  AND NOT EXISTS (SELECT 1 FROM upstream_sites s WHERE s.id = c.upstream_site_id)

UNION ALL
SELECT 'site_pricing_cache',
       COUNT(*)
FROM site_pricing_cache p
WHERE COALESCE(TRIM(p.site_id), '') <> ''
  AND NOT EXISTS (SELECT 1 FROM upstream_sites s WHERE s.id = p.site_id);
