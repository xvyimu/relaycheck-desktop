-- DRY-RUN: list checkin_logs whose account_id no longer exists.
-- Does NOT delete. Required before Phase B FK (account_id → channel_accounts).

.mode column
.headers on

SELECT 'orphan_checkin_logs_missing_account' AS metric, COUNT(*) AS n
FROM checkin_logs l
WHERE NOT EXISTS (SELECT 1 FROM channel_accounts a WHERE a.id = l.account_id);

SELECT
  l.id,
  l.account_id AS missing_account_id,
  l.upstream_site_id,
  COALESCE(l.status, '') AS status,
  COALESCE(l.started_at, '') AS started_at
FROM checkin_logs l
WHERE NOT EXISTS (SELECT 1 FROM channel_accounts a WHERE a.id = l.account_id)
ORDER BY l.started_at DESC, l.id
LIMIT 50;

-- Mutating form (NOT RUN):
-- BEGIN;
-- DELETE FROM checkin_logs
-- WHERE NOT EXISTS (SELECT 1 FROM channel_accounts a WHERE a.id = checkin_logs.account_id);
-- COMMIT;
