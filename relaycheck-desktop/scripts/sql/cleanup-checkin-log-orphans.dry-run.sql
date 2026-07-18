-- DRY-RUN only: list orphan checkin_logs that a future cleanup would remove.
-- Does NOT delete. Review output, backup DB, then request explicit confirm
-- before any mutating SQL.
--
-- Orphan rule: upstream_site_id non-empty AND no matching upstream_sites.id
--
-- sqlite3 example:
--   sqlite3 -readonly data/relaycheck.db < scripts/sql/cleanup-checkin-log-orphans.dry-run.sql

.mode column
.headers on
.nullvalue NULL

-- Summary counts
SELECT 'orphan_checkin_logs' AS metric, COUNT(*) AS n
FROM checkin_logs l
WHERE COALESCE(TRIM(l.upstream_site_id), '') <> ''
  AND NOT EXISTS (SELECT 1 FROM upstream_sites s WHERE s.id = l.upstream_site_id);

-- Sample rows (cap 50) for operator review
SELECT
  l.id,
  l.account_id,
  l.upstream_site_id AS missing_site_id,
  COALESCE(l.status, '') AS status,
  COALESCE(l.started_at, '') AS started_at,
  COALESCE(l.finished_at, '') AS finished_at
FROM checkin_logs l
WHERE COALESCE(TRIM(l.upstream_site_id), '') <> ''
  AND NOT EXISTS (SELECT 1 FROM upstream_sites s WHERE s.id = l.upstream_site_id)
ORDER BY l.started_at DESC, l.id
LIMIT 50;

-- Distinct missing site ids (how many historical sites left debris)
SELECT
  l.upstream_site_id AS missing_site_id,
  COUNT(*) AS orphan_logs
FROM checkin_logs l
WHERE COALESCE(TRIM(l.upstream_site_id), '') <> ''
  AND NOT EXISTS (SELECT 1 FROM upstream_sites s WHERE s.id = l.upstream_site_id)
GROUP BY l.upstream_site_id
ORDER BY orphan_logs DESC, missing_site_id;

-- Mutating form (NOT RUN by tooling; keep commented):
-- BEGIN;
-- DELETE FROM checkin_logs
-- WHERE COALESCE(TRIM(upstream_site_id), '') <> ''
--   AND NOT EXISTS (SELECT 1 FROM upstream_sites s WHERE s.id = checkin_logs.upstream_site_id);
-- -- verify COUNT before COMMIT
-- COMMIT;
