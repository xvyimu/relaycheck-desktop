package core

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// schemaFKPhaseKey records completed FK rebuild phases in system_settings.
// 2 = Phase A (site subtree) + Phase B (account subtree) applied.
const schemaFKPhaseKey = "schema.fk_phase"
const schemaFKPhaseAB = "2"

// ensureSiteAndAccountForeignKeys rebuilds child tables with SQLite FK clauses
// for Phase A (site subtree) and Phase B (account subtree). Idempotent via
// schema.fk_phase and PRAGMA foreign_key_list. Scrubs orphans that would block
// FK enablement (site orphans + checkin_logs missing parent account).
//
// Pattern mirrors ensureChannelSchedulesNullableSiteID: dedicated conn,
// PRAGMA foreign_keys=OFF, table rewrite, then foreign_key_check.
func (a *App) ensureSiteAndAccountForeignKeys(ctx context.Context) error {
	done, err := a.schemaFKPhaseAtLeast(ctx, schemaFKPhaseAB)
	if err != nil {
		return err
	}
	if done {
		return nil
	}
	// Fast path: already rewritten (e.g. settings wiped but tables have FKs).
	hasAccSite, err := a.tableHasFK(ctx, "channel_accounts", "upstream_site_id", "upstream_sites")
	if err != nil {
		return err
	}
	hasLogAcc, err := a.tableHasFK(ctx, "checkin_logs", "account_id", "channel_accounts")
	if err != nil {
		return err
	}
	if hasAccSite && hasLogAcc {
		return a.setSchemaFKPhase(ctx, schemaFKPhaseAB)
	}

	if err := a.scrubOrphansBeforeFK(ctx); err != nil {
		return err
	}
	if err := a.assertNoBlockingOrphansForFK(ctx); err != nil {
		return err
	}

	conn, err := a.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys=OFF`); err != nil {
		return err
	}
	defer conn.ExecContext(context.Background(), `PRAGMA foreign_keys=ON`)

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	// FTS content='channel_accounts' must be dropped before rewriting accounts.
	if err := dropAccountSearchFTS(ctx, tx); err != nil {
		return err
	}

	if err := rebuildChannelAccountsWithSiteFK(ctx, tx); err != nil {
		return fmt.Errorf("rebuild channel_accounts FK: %w", err)
	}
	if err := rebuildCheckinLogsWithFKs(ctx, tx); err != nil {
		return fmt.Errorf("rebuild checkin_logs FK: %w", err)
	}
	if err := rebuildBalanceSnapshotsWithFKs(ctx, tx); err != nil {
		return fmt.Errorf("rebuild balance_snapshots FK: %w", err)
	}
	if err := rebuildSitePricingCacheWithSiteFK(ctx, tx); err != nil {
		return fmt.Errorf("rebuild site_pricing_cache FK: %w", err)
	}

	// Recreate FTS against new accounts table (full rebuild).
	if err := createAccountSearchFTS(ctx, tx, true); err != nil {
		return fmt.Errorf("recreate account_search_fts: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true

	if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys=ON`); err != nil {
		return err
	}
	if err := a.assertForeignKeyCheck(ctx); err != nil {
		return err
	}
	return a.setSchemaFKPhase(ctx, schemaFKPhaseAB)
}

func (a *App) schemaFKPhaseAtLeast(ctx context.Context, want string) (bool, error) {
	var value string
	err := a.db.QueryRowContext(ctx, `
		SELECT value_json FROM system_settings WHERE key = ?
	`, schemaFKPhaseKey).Scan(&value)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	// value_json stores a JSON string, e.g. "2"
	clean := stringsTrimJSONString(value)
	return clean >= want, nil
}

func (a *App) setSchemaFKPhase(ctx context.Context, phase string) error {
	ts := now()
	_, err := a.db.ExecContext(ctx, `
		INSERT INTO system_settings (id, key, value_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value_json=excluded.value_json, updated_at=excluded.updated_at
	`, newID(), schemaFKPhaseKey, `"`+phase+`"`, ts, ts)
	return err
}

func stringsTrimJSONString(raw string) string {
	s := raw
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

func (a *App) tableHasFK(ctx context.Context, table, fromCol, parentTable string) (bool, error) {
	if !identifierPattern.MatchString(table) {
		return false, fmt.Errorf("tableHasFK: invalid table %q", table)
	}
	rows, err := a.db.QueryContext(ctx, "PRAGMA foreign_key_list("+table+")")
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, seq int
		var parent, from, to, onUpdate, onDelete, match string
		if err := rows.Scan(&id, &seq, &parent, &from, &to, &onUpdate, &onDelete, &match); err != nil {
			return false, err
		}
		if from == fromCol && parent == parentTable {
			return true, nil
		}
	}
	return false, rows.Err()
}

func (a *App) scrubOrphansBeforeFK(ctx context.Context) error {
	// Site-level orphans (should already be 0 on maintained hosts).
	stmts := []string{
		`DELETE FROM channel_accounts WHERE COALESCE(TRIM(upstream_site_id),'') <> '' AND NOT EXISTS (SELECT 1 FROM upstream_sites s WHERE s.id = channel_accounts.upstream_site_id)`,
		`DELETE FROM checkin_logs WHERE COALESCE(TRIM(upstream_site_id),'') <> '' AND NOT EXISTS (SELECT 1 FROM upstream_sites s WHERE s.id = checkin_logs.upstream_site_id)`,
		`DELETE FROM balance_snapshots WHERE COALESCE(TRIM(upstream_site_id),'') <> '' AND NOT EXISTS (SELECT 1 FROM upstream_sites s WHERE s.id = balance_snapshots.upstream_site_id)`,
		`DELETE FROM site_pricing_cache WHERE COALESCE(TRIM(site_id),'') <> '' AND NOT EXISTS (SELECT 1 FROM upstream_sites s WHERE s.id = site_pricing_cache.site_id)`,
		`DELETE FROM channel_schedules WHERE upstream_site_id IS NOT NULL AND COALESCE(TRIM(upstream_site_id),'') <> '' AND upstream_site_id <> ? AND NOT EXISTS (SELECT 1 FROM upstream_sites s WHERE s.id = channel_schedules.upstream_site_id)`,
		// Phase B blockers: logs/balances pointing at deleted accounts.
		`DELETE FROM checkin_logs WHERE NOT EXISTS (SELECT 1 FROM channel_accounts a WHERE a.id = checkin_logs.account_id)`,
		`DELETE FROM balance_snapshots WHERE NOT EXISTS (SELECT 1 FROM channel_accounts a WHERE a.id = balance_snapshots.account_id)`,
	}
	for i, q := range stmts {
		var err error
		if i == 4 {
			_, err = a.db.ExecContext(ctx, q, globalScheduleSiteID)
		} else {
			_, err = a.db.ExecContext(ctx, q)
		}
		if err != nil {
			return fmt.Errorf("scrub orphans step %d: %w", i, err)
		}
	}
	return nil
}

func (a *App) assertNoBlockingOrphansForFK(ctx context.Context) error {
	checks := []struct {
		name  string
		query string
		args  []interface{}
	}{
		{"channel_accounts→site", `SELECT COUNT(*) FROM channel_accounts a WHERE NOT EXISTS (SELECT 1 FROM upstream_sites s WHERE s.id=a.upstream_site_id)`, nil},
		{"checkin_logs→site", `SELECT COUNT(*) FROM checkin_logs l WHERE NOT EXISTS (SELECT 1 FROM upstream_sites s WHERE s.id=l.upstream_site_id)`, nil},
		{"balance_snapshots→site", `SELECT COUNT(*) FROM balance_snapshots b WHERE NOT EXISTS (SELECT 1 FROM upstream_sites s WHERE s.id=b.upstream_site_id)`, nil},
		{"site_pricing_cache→site", `SELECT COUNT(*) FROM site_pricing_cache p WHERE NOT EXISTS (SELECT 1 FROM upstream_sites s WHERE s.id=p.site_id)`, nil},
		{"checkin_logs→account", `SELECT COUNT(*) FROM checkin_logs l WHERE NOT EXISTS (SELECT 1 FROM channel_accounts a WHERE a.id=l.account_id)`, nil},
		{"balance_snapshots→account", `SELECT COUNT(*) FROM balance_snapshots b WHERE NOT EXISTS (SELECT 1 FROM channel_accounts a WHERE a.id=b.account_id)`, nil},
	}
	for _, c := range checks {
		var n int
		if err := a.db.QueryRowContext(ctx, c.query, c.args...).Scan(&n); err != nil {
			return err
		}
		if n != 0 {
			return fmt.Errorf("FK migrate blocked: %s still has %d orphan row(s); backup DB and repair before retry", c.name, n)
		}
	}
	return nil
}

func (a *App) assertForeignKeyCheck(ctx context.Context) error {
	rows, err := a.db.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		var table string
		var rowid int64
		var parent string
		var fkid int
		_ = rows.Scan(&table, &rowid, &parent, &fkid)
		return fmt.Errorf("PRAGMA foreign_key_check failed: table=%s rowid=%d parent=%s fkid=%d", table, rowid, parent, fkid)
	}
	return rows.Err()
}

func dropAccountSearchFTS(ctx context.Context, tx *sql.Tx) error {
	stmts := []string{
		`DROP TRIGGER IF EXISTS channel_accounts_search_ai`,
		`DROP TRIGGER IF EXISTS channel_accounts_search_ad`,
		`DROP TRIGGER IF EXISTS channel_accounts_search_au`,
		`DROP TABLE IF EXISTS account_search_fts`,
	}
	for _, s := range stmts {
		if _, err := tx.ExecContext(ctx, s); err != nil {
			return err
		}
	}
	return nil
}

func createAccountSearchFTS(ctx context.Context, tx *sql.Tx, rebuild bool) error {
	if _, err := tx.ExecContext(ctx, `
CREATE VIRTUAL TABLE IF NOT EXISTS account_search_fts USING fts5(
	display_name,
	email,
	username,
	login_status,
	content='channel_accounts',
	content_rowid='rowid',
	tokenize='unicode61 remove_diacritics 2'
);
CREATE TRIGGER IF NOT EXISTS channel_accounts_search_ai AFTER INSERT ON channel_accounts BEGIN
	INSERT INTO account_search_fts(rowid, display_name, email, username, login_status)
	VALUES (new.rowid, new.display_name, new.email, new.username, new.login_status);
END;
CREATE TRIGGER IF NOT EXISTS channel_accounts_search_ad AFTER DELETE ON channel_accounts BEGIN
	INSERT INTO account_search_fts(account_search_fts, rowid, display_name, email, username, login_status)
	VALUES ('delete', old.rowid, old.display_name, old.email, old.username, old.login_status);
END;
CREATE TRIGGER IF NOT EXISTS channel_accounts_search_au AFTER UPDATE ON channel_accounts BEGIN
	INSERT INTO account_search_fts(account_search_fts, rowid, display_name, email, username, login_status)
	VALUES ('delete', old.rowid, old.display_name, old.email, old.username, old.login_status);
	INSERT INTO account_search_fts(rowid, display_name, email, username, login_status)
	VALUES (new.rowid, new.display_name, new.email, new.username, new.login_status);
END;
`); err != nil {
		return err
	}
	if rebuild {
		_, err := tx.ExecContext(ctx, `INSERT INTO account_search_fts(account_search_fts) VALUES('rebuild')`)
		return err
	}
	return nil
}


// copyTableByIntersection copies every column present on both src and dest tables.
func copyTableByIntersection(ctx context.Context, tx *sql.Tx, src, dest string) error {
	srcCols, err := listTableColumnsTx(ctx, tx, src)
	if err != nil {
		return err
	}
	destCols, err := listTableColumnsTx(ctx, tx, dest)
	if err != nil {
		return err
	}
	destSet := map[string]bool{}
	for _, c := range destCols {
		destSet[c] = true
	}
	var cols []string
	for _, c := range srcCols {
		if destSet[c] {
			cols = append(cols, c)
		}
	}
	if len(cols) == 0 {
		return fmt.Errorf("no overlapping columns between %s and %s", src, dest)
	}
	list := strings.Join(cols, ", ")
	_, err = tx.ExecContext(ctx, "INSERT INTO "+dest+" ("+list+") SELECT "+list+" FROM "+src)
	return err
}

func listTableColumnsTx(ctx context.Context, tx *sql.Tx, table string) ([]string, error) {
	if !identifierPattern.MatchString(table) {
		return nil, fmt.Errorf("invalid table %q", table)
	}
	rows, err := tx.QueryContext(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var cid, notNull, pk int
		var name, ctype string
		var dflt interface{}
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dflt, &pk); err != nil {
			return nil, err
		}
		cols = append(cols, name)
	}
	return cols, rows.Err()
}

func rebuildChannelAccountsWithSiteFK(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `
CREATE TABLE channel_accounts_new (
	id TEXT PRIMARY KEY,
	upstream_site_id TEXT NOT NULL,
	display_name TEXT NOT NULL,
	email TEXT,
	username TEXT,
	auth_type TEXT NOT NULL,
	password_encrypted TEXT,
	cookie_encrypted TEXT,
	access_token_encrypted TEXT,
	refresh_token_encrypted TEXT,
	api_key_encrypted TEXT,
	browser_profile_path TEXT,
	user_agent TEXT,
	login_status TEXT NOT NULL DEFAULT 'unknown',
	balance REAL,
	balance_unit TEXT DEFAULT 'unknown',
	last_login_at TEXT,
	last_validated_at TEXT,
	last_checkin_at TEXT,
	last_checkin_status TEXT,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	auth_user_id TEXT,
	api_key_fingerprint TEXT,
	api_key_status TEXT,
	api_key_last_checked_at TEXT,
	api_key_model_count INTEGER NOT NULL DEFAULT 0,
	api_key_sample_models_json TEXT,
	api_key_test_model TEXT,
	api_key_model_usable INTEGER NOT NULL DEFAULT 0,
	api_key_latency_ms INTEGER NOT NULL DEFAULT 0,
	api_key_test_http_status INTEGER NOT NULL DEFAULT 0,
	api_key_test_message TEXT,
	api_key_test_path TEXT,
	cookie_expiry_at TEXT,
	storage_state_expiry_at TEXT,
	FOREIGN KEY (upstream_site_id) REFERENCES upstream_sites(id) ON DELETE CASCADE
);
`); err != nil {
		return err
	}
	if err := copyTableByIntersection(ctx, tx, "channel_accounts", "channel_accounts_new"); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DROP TABLE channel_accounts`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `ALTER TABLE channel_accounts_new RENAME TO channel_accounts`); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
CREATE INDEX IF NOT EXISTS idx_channel_accounts_site ON channel_accounts(upstream_site_id);
CREATE INDEX IF NOT EXISTS idx_channel_accounts_updated ON channel_accounts(updated_at);
CREATE INDEX IF NOT EXISTS idx_channel_accounts_updated_id ON channel_accounts(updated_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_channel_accounts_key_check ON channel_accounts(api_key_last_checked_at, updated_at);
`)
	return err
}

func rebuildCheckinLogsWithFKs(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `
CREATE TABLE checkin_logs_new (
	id TEXT PRIMARY KEY,
	account_id TEXT NOT NULL,
	upstream_site_id TEXT NOT NULL,
	channel_id TEXT,
	status TEXT NOT NULL,
	reward TEXT,
	message TEXT,
	raw_response_masked TEXT,
	started_at TEXT NOT NULL,
	finished_at TEXT NOT NULL,
	FOREIGN KEY (account_id) REFERENCES channel_accounts(id) ON DELETE CASCADE,
	FOREIGN KEY (upstream_site_id) REFERENCES upstream_sites(id) ON DELETE CASCADE
);
`); err != nil {
		return err
	}
	if err := copyTableByIntersection(ctx, tx, "checkin_logs", "checkin_logs_new"); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DROP TABLE checkin_logs`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `ALTER TABLE checkin_logs_new RENAME TO checkin_logs`); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
CREATE INDEX IF NOT EXISTS idx_checkin_logs_account ON checkin_logs(account_id);
CREATE INDEX IF NOT EXISTS idx_checkin_logs_started ON checkin_logs(started_at);
CREATE INDEX IF NOT EXISTS idx_checkin_logs_account_started ON checkin_logs(account_id, started_at);
CREATE INDEX IF NOT EXISTS idx_checkin_logs_account_started_id ON checkin_logs(account_id, started_at DESC, id DESC);
`)
	return err
}

func rebuildBalanceSnapshotsWithFKs(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `
CREATE TABLE balance_snapshots_new (
	id TEXT PRIMARY KEY,
	account_id TEXT NOT NULL,
	upstream_site_id TEXT NOT NULL,
	channel_id TEXT,
	balance REAL,
	used_quota REAL,
	total_quota REAL,
	unit TEXT NOT NULL DEFAULT 'unknown',
	raw_response_masked TEXT,
	created_at TEXT NOT NULL,
	FOREIGN KEY (account_id) REFERENCES channel_accounts(id) ON DELETE CASCADE,
	FOREIGN KEY (upstream_site_id) REFERENCES upstream_sites(id) ON DELETE CASCADE
);
`); err != nil {
		return err
	}
	if err := copyTableByIntersection(ctx, tx, "balance_snapshots", "balance_snapshots_new"); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DROP TABLE balance_snapshots`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `ALTER TABLE balance_snapshots_new RENAME TO balance_snapshots`); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
CREATE INDEX IF NOT EXISTS idx_balance_snapshots_account ON balance_snapshots(account_id);
CREATE INDEX IF NOT EXISTS idx_balance_snapshots_created ON balance_snapshots(created_at);
CREATE INDEX IF NOT EXISTS idx_balance_snapshots_account_created ON balance_snapshots(account_id, created_at);
CREATE INDEX IF NOT EXISTS idx_balance_snapshots_account_created_id ON balance_snapshots(account_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_balance_snapshots_site_created ON balance_snapshots(upstream_site_id, created_at);
`)
	return err
}

func rebuildSitePricingCacheWithSiteFK(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `
CREATE TABLE site_pricing_cache_new (
	id TEXT PRIMARY KEY,
	site_id TEXT NOT NULL,
	site_name TEXT NOT NULL,
	base_url TEXT NOT NULL,
	kind TEXT NOT NULL DEFAULT 'unknown',
	status TEXT NOT NULL DEFAULT 'unknown',
	http_status INTEGER NOT NULL DEFAULT 0,
	latency_ms INTEGER NOT NULL DEFAULT 0,
	source_path TEXT NOT NULL DEFAULT '/api/pricing',
	raw_response_masked TEXT,
	sources_json TEXT,
	model_count INTEGER NOT NULL DEFAULT 0,
	source_count INTEGER NOT NULL DEFAULT 0,
	message TEXT,
	last_synced_at TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE(site_id, source_path),
	FOREIGN KEY (site_id) REFERENCES upstream_sites(id) ON DELETE CASCADE
);
`); err != nil {
		return err
	}
	if err := copyTableByIntersection(ctx, tx, "site_pricing_cache", "site_pricing_cache_new"); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DROP TABLE site_pricing_cache`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `ALTER TABLE site_pricing_cache_new RENAME TO site_pricing_cache`); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
CREATE INDEX IF NOT EXISTS idx_site_pricing_cache_site ON site_pricing_cache(site_id);
CREATE INDEX IF NOT EXISTS idx_site_pricing_cache_synced ON site_pricing_cache(last_synced_at);
`)
	return err
}
