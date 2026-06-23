package core

import (
	"context"
	"fmt"
)

func (a *App) migrate(ctx context.Context) error {
	_, err := a.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS app_users (
	id TEXT PRIMARY KEY,
	username TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	display_name TEXT,
	role TEXT NOT NULL DEFAULT 'admin',
	must_change_pass INTEGER NOT NULL DEFAULT 1,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS system_settings (
	id TEXT PRIMARY KEY,
	key TEXT NOT NULL UNIQUE,
	value_json TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS local_newapi_instances (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	base_url TEXT NOT NULL UNIQUE,
	detected_from TEXT,
	status TEXT NOT NULL DEFAULT 'unknown',
	version TEXT,
	database_path TEXT,
	last_scanned_at TEXT,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS imported_channels (
	id TEXT PRIMARY KEY,
	local_instance_id TEXT,
	source_channel_id TEXT NOT NULL,
	name TEXT NOT NULL,
	base_url TEXT,
	status TEXT,
	upstream_kind TEXT NOT NULL DEFAULT 'unknown',
	supports_checkin INTEGER NOT NULL DEFAULT 0,
	supports_balance INTEGER NOT NULL DEFAULT 0,
	supports_models INTEGER NOT NULL DEFAULT 0,
	supports_pricing INTEGER NOT NULL DEFAULT 0,
	channel_key_encrypted TEXT,
	channel_key_masked TEXT,
	raw_json TEXT NOT NULL,
	source_sync_status TEXT NOT NULL DEFAULT 'active',
	source_missing_at TEXT,
	last_detected_at TEXT,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE(local_instance_id, source_channel_id)
);

CREATE TABLE IF NOT EXISTS upstream_sites (
	id TEXT PRIMARY KEY,
	channel_id TEXT,
	name TEXT NOT NULL,
	homepage_url TEXT,
	base_url TEXT NOT NULL,
	login_url TEXT,
	kind TEXT NOT NULL DEFAULT 'unknown',
	detection_confidence REAL NOT NULL DEFAULT 0,
	health_status TEXT NOT NULL DEFAULT 'unknown',
	supports_checkin INTEGER NOT NULL DEFAULT 0,
	supports_balance INTEGER NOT NULL DEFAULT 0,
	supports_models INTEGER NOT NULL DEFAULT 0,
	supports_pricing INTEGER NOT NULL DEFAULT 0,
	checkin_config_json TEXT,
	balance_config_json TEXT,
	last_health_check_at TEXT,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_upstream_sites_base_url ON upstream_sites(base_url);

CREATE TABLE IF NOT EXISTS channel_accounts (
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
	updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_channel_accounts_site ON channel_accounts(upstream_site_id);

CREATE TABLE IF NOT EXISTS checkin_logs (
	id TEXT PRIMARY KEY,
	account_id TEXT NOT NULL,
	upstream_site_id TEXT NOT NULL,
	channel_id TEXT,
	status TEXT NOT NULL,
	reward TEXT,
	message TEXT,
	raw_response_masked TEXT,
	started_at TEXT NOT NULL,
	finished_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_checkin_logs_account ON checkin_logs(account_id);
CREATE INDEX IF NOT EXISTS idx_checkin_logs_started ON checkin_logs(started_at);

CREATE TABLE IF NOT EXISTS balance_snapshots (
	id TEXT PRIMARY KEY,
	account_id TEXT NOT NULL,
	upstream_site_id TEXT NOT NULL,
	channel_id TEXT,
	balance REAL,
	used_quota REAL,
	total_quota REAL,
	unit TEXT NOT NULL DEFAULT 'unknown',
	raw_response_masked TEXT,
	created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_balance_snapshots_account ON balance_snapshots(account_id);
CREATE INDEX IF NOT EXISTS idx_balance_snapshots_created ON balance_snapshots(created_at);

CREATE TABLE IF NOT EXISTS app_notifications (
	id TEXT PRIMARY KEY,
	type TEXT NOT NULL,
	level TEXT NOT NULL DEFAULT 'info',
	title TEXT NOT NULL,
	content TEXT NOT NULL,
	read INTEGER NOT NULL DEFAULT 0,
	related_type TEXT,
	related_id TEXT,
	created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_app_notifications_read ON app_notifications(read);

CREATE TABLE IF NOT EXISTS audit_log (
	id TEXT PRIMARY KEY,
	action TEXT NOT NULL,
	level TEXT NOT NULL DEFAULT 'info',
	actor TEXT,
	resource_type TEXT,
	resource_id TEXT,
	summary TEXT NOT NULL,
	metadata_json TEXT,
	created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_audit_log_created ON audit_log(created_at);
CREATE INDEX IF NOT EXISTS idx_audit_log_action ON audit_log(action, created_at);

CREATE TABLE IF NOT EXISTS scheduler_runs (
	job_key TEXT PRIMARY KEY,
	status TEXT NOT NULL DEFAULT 'idle',
	planned_run_key TEXT,
	next_run_at TEXT,
	last_run_key TEXT,
	last_started_at TEXT,
	last_finished_at TEXT,
	last_success_at TEXT,
	last_error TEXT,
	summary TEXT,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS site_pricing_cache (
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
	UNIQUE(site_id, source_path)
);
CREATE INDEX IF NOT EXISTS idx_site_pricing_cache_site ON site_pricing_cache(site_id);
CREATE INDEX IF NOT EXISTS idx_site_pricing_cache_synced ON site_pricing_cache(last_synced_at);
`)
	if err != nil {
		return err
	}
	for _, column := range []struct {
		table      string
		name       string
		columnType string
	}{
		{"channel_accounts", "auth_user_id", "TEXT"},
		{"channel_accounts", "api_key_fingerprint", "TEXT"},
		{"channel_accounts", "api_key_status", "TEXT"},
		{"channel_accounts", "api_key_last_checked_at", "TEXT"},
		{"channel_accounts", "api_key_model_count", "INTEGER NOT NULL DEFAULT 0"},
		{"channel_accounts", "api_key_sample_models_json", "TEXT"},
		{"channel_accounts", "api_key_test_model", "TEXT"},
		{"channel_accounts", "api_key_model_usable", "INTEGER NOT NULL DEFAULT 0"},
		{"channel_accounts", "api_key_latency_ms", "INTEGER NOT NULL DEFAULT 0"},
		{"channel_accounts", "api_key_test_http_status", "INTEGER NOT NULL DEFAULT 0"},
		{"channel_accounts", "api_key_test_message", "TEXT"},
		{"channel_accounts", "api_key_test_path", "TEXT"},
		{"upstream_sites", "detection_json", "TEXT"},
		{"imported_channels", "detection_json", "TEXT"},
		{"imported_channels", "source_sync_status", "TEXT NOT NULL DEFAULT 'active'"},
		{"imported_channels", "source_missing_at", "TEXT"},
		{"imported_channels", "model_count", "INTEGER NOT NULL DEFAULT 0"},
		{"imported_channels", "sample_models_json", "TEXT"},
		{"imported_channels", "models_source", "TEXT"},
		{"imported_channels", "models_status", "TEXT"},
		{"imported_channels", "models_last_synced_at", "TEXT"},
		{"imported_channels", "models_message", "TEXT"},
		{"local_newapi_instances", "sync_access_token_encrypted", "TEXT"},
		{"local_newapi_instances", "sync_access_token_masked", "TEXT"},
	} {
		if err := a.ensureColumn(ctx, column.table, column.name, column.columnType); err != nil {
			return err
		}
	}
	if err := a.ensurePerformanceIndexes(ctx); err != nil {
		return err
	}
	return nil
}

func (a *App) ensurePerformanceIndexes(ctx context.Context) error {
	_, err := a.db.ExecContext(ctx, `
CREATE INDEX IF NOT EXISTS idx_imported_channels_source_status_updated ON imported_channels(source_sync_status, updated_at);
CREATE INDEX IF NOT EXISTS idx_imported_channels_kind_updated ON imported_channels(upstream_kind, updated_at);
CREATE INDEX IF NOT EXISTS idx_upstream_sites_kind_updated ON upstream_sites(kind, updated_at);
CREATE INDEX IF NOT EXISTS idx_upstream_sites_updated ON upstream_sites(updated_at);
CREATE INDEX IF NOT EXISTS idx_channel_accounts_updated ON channel_accounts(updated_at);
CREATE INDEX IF NOT EXISTS idx_channel_accounts_key_check ON channel_accounts(api_key_last_checked_at, updated_at);
CREATE INDEX IF NOT EXISTS idx_checkin_logs_account_started ON checkin_logs(account_id, started_at);
CREATE INDEX IF NOT EXISTS idx_balance_snapshots_account_created ON balance_snapshots(account_id, created_at);
CREATE INDEX IF NOT EXISTS idx_balance_snapshots_site_created ON balance_snapshots(upstream_site_id, created_at);
CREATE INDEX IF NOT EXISTS idx_app_notifications_read_created ON app_notifications(read, created_at);
`)
	return err
}

func (a *App) ensureColumn(ctx context.Context, table string, column string, columnType string) error {
	rows, err := a.db.QueryContext(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull int
		var defaultValue interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	_, err = a.db.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, columnType))
	return err
}
