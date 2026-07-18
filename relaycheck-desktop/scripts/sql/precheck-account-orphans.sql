-- Read-only: account-level orphans that block Phase B FK
-- (checkin_logs / balance_snapshots → channel_accounts)

.mode column
.headers on

SELECT 'checkin_logs_missing_account' AS check_name, COUNT(*) AS orphan_rows
FROM checkin_logs l
WHERE NOT EXISTS (SELECT 1 FROM channel_accounts a WHERE a.id = l.account_id)

UNION ALL
SELECT 'balance_snapshots_missing_account', COUNT(*)
FROM balance_snapshots b
WHERE NOT EXISTS (SELECT 1 FROM channel_accounts a WHERE a.id = b.account_id)

UNION ALL
SELECT 'checkin_logs_site_account_mismatch', COUNT(*)
FROM checkin_logs l
JOIN channel_accounts a ON a.id = l.account_id
WHERE a.upstream_site_id <> l.upstream_site_id;
