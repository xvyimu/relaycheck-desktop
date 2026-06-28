package core

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// createTestPythonDB builds a Python-format SQLite database at the given path
// with all 4 tables and representative test data.
func createTestPythonDB(t *testing.T, dbPath string) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(dbPath))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	schema := `
	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS local_channels (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		base_url TEXT NOT NULL,
		enabled INTEGER NOT NULL DEFAULT 1,
		detection_json TEXT DEFAULT '',
		source_type TEXT DEFAULT '',
		platform_override TEXT DEFAULT ''
	);
	CREATE TABLE IF NOT EXISTS channel_credentials (
		id INTEGER PRIMARY KEY,
		channel_id INTEGER NOT NULL,
		site_name TEXT NOT NULL DEFAULT '',
		checkin_password TEXT DEFAULT '',
		auth_type TEXT DEFAULT '',
		auth_config TEXT DEFAULT '',
		login_url TEXT DEFAULT '',
		checkin_url TEXT DEFAULT '',
		email TEXT DEFAULT '',
		username TEXT DEFAULT '',
		cookie TEXT DEFAULT '',
		access_token TEXT DEFAULT '',
		api_key TEXT DEFAULT ''
	);
	CREATE TABLE IF NOT EXISTS checkin_history (
		id INTEGER PRIMARY KEY,
		channel_id INTEGER NOT NULL,
		status TEXT DEFAULT '',
		balance TEXT DEFAULT '',
		message TEXT DEFAULT '',
		check_date TEXT DEFAULT '',
		credential_id INTEGER
	);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}

	// Insert 4 settings, 2 local_channels, 2 channel_credentials, 3 checkin_history
	data := []string{
		`INSERT INTO settings (key, value) VALUES
			('schedule_enabled', '1'),
			('delay_min', '5'),
			('delay_max', '60'),
			('concurrent', '3')`,
		`INSERT INTO local_channels (id, name, base_url, enabled, detection_json, source_type) VALUES
			(1, 'NewAPI 站点', 'https://newapi.example.com', 1, '{"checkin_enabled":true,"version":"1.0"}', 'newapi'),
			(2, 'Sub2API 站点', 'https://sub2api.example.com', 0, '', 'sub2api')`,
		`INSERT INTO channel_credentials (id, channel_id, site_name, checkin_password, auth_type, auth_config, login_url, checkin_url, email, username) VALUES
			(1, 1, 'NewAPI 站点 · admin', 'test123', 'password', '{"platform":"newapi"}', 'https://newapi.example.com/login', 'https://newapi.example.com/checkin', 'admin@test.com', 'admin'),
			(2, 2, 'Sub2API 站点 · token', '', 'token', '{"platform":"sub2api"}', '', 'https://sub2api.example.com/checkin', '', 'token-user')`,
		`INSERT INTO checkin_history (id, channel_id, status, balance, message, check_date, credential_id) VALUES
			(1, 1, '成功', '100', '签到成功', '2026-06-20', 1),
			(2, 2, '已签到', '0', '今日已签到', '2026-06-20', 2),
			(3, 2, '失败', '0', '网络错误', '2026-06-19', 2)`,
	}
	for _, stmt := range data {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
}

// createEmptyTestPythonDB creates a Python-format database with schema only.
func createEmptyTestPythonDB(t *testing.T, dbPath string) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(dbPath))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	schema := `
	CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value TEXT NOT NULL);
	CREATE TABLE IF NOT EXISTS local_channels (id INTEGER PRIMARY KEY, name TEXT NOT NULL, base_url TEXT NOT NULL, enabled INTEGER DEFAULT 1, detection_json TEXT DEFAULT '', source_type TEXT DEFAULT '', platform_override TEXT DEFAULT '');
	CREATE TABLE IF NOT EXISTS channel_credentials (id INTEGER PRIMARY KEY, channel_id INTEGER NOT NULL, site_name TEXT DEFAULT '', checkin_password TEXT DEFAULT '', auth_type TEXT DEFAULT '', auth_config TEXT DEFAULT '', login_url TEXT DEFAULT '', checkin_url TEXT DEFAULT '', email TEXT DEFAULT '', username TEXT DEFAULT '', cookie TEXT DEFAULT '', access_token TEXT DEFAULT '', api_key TEXT DEFAULT '');
	CREATE TABLE IF NOT EXISTS checkin_history (id INTEGER PRIMARY KEY, channel_id INTEGER NOT NULL, status TEXT DEFAULT '', balance TEXT DEFAULT '', message TEXT DEFAULT '', check_date TEXT DEFAULT '', credential_id INTEGER);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}
}

func TestPythonMigrationDryRun(t *testing.T) {
	pythonDBPath := filepath.Join(t.TempDir(), "zidqiandao.db")
	createTestPythonDB(t, pythonDBPath)

	app := newTestApp(t)
	defer app.Close()

	report, err := app.migrateFromPythonDB(context.Background(), pythonDBPath, "dry_run")
	if err != nil {
		t.Fatal(err)
	}

	if report.Mode != "dry_run" {
		t.Fatalf("expected mode dry_run, got %s", report.Mode)
	}
	// 4 Python settings -> 1 composite checkin.schedule (dry_run doesn't check existence)
	if report.SettingsImported != 1 {
		t.Fatalf("expected 1 setting (checkin.schedule from 4 Python settings), got %d", report.SettingsImported)
	}
	if report.SitesImported != 2 {
		t.Fatalf("expected 2 sites, got %d", report.SitesImported)
	}
	if report.AccountsImported != 2 {
		t.Fatalf("expected 2 accounts, got %d", report.AccountsImported)
	}
	if report.LogsImported != 3 {
		t.Fatalf("expected 3 logs, got %d", report.LogsImported)
	}
	if report.BackupFileName != "" {
		t.Fatalf("expected no backup file in dry_run, got %s", report.BackupFileName)
	}

	// Verify no data was written to Go tables
	var count int
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM system_settings`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	// 8 default settings (including channel.health.schedule), none from migration
	if count != 8 {
		t.Fatalf("expected 8 default settings (no migration writes), got %d", count)
	}
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM upstream_sites`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 upstream_sites (__global__), got %d", count)
	}
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM channel_accounts`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected 0 channel_accounts, got %d", count)
	}
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM checkin_logs`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected 0 checkin_logs, got %d", count)
	}
}

func TestPythonMigrationLive(t *testing.T) {
	pythonDBPath := filepath.Join(t.TempDir(), "zidqiandao.db")
	createTestPythonDB(t, pythonDBPath)

	app := newTestApp(t)
	defer app.Close()

	report, err := app.migrateFromPythonDB(context.Background(), pythonDBPath, "live")
	if err != nil {
		t.Fatal(err)
	}

	if report.Mode != "live" {
		t.Fatalf("expected mode live, got %s", report.Mode)
	}
	// Default settings for checkin.schedule and network.proxy already exist, so 0 new
	if report.SettingsImported != 0 {
		t.Fatalf("expected 0 settings (defaults already exist), got %d", report.SettingsImported)
	}
	if report.SitesImported != 2 {
		t.Fatalf("expected 2 sites, got %d", report.SitesImported)
	}
	if report.AccountsImported != 2 {
		t.Fatalf("expected 2 accounts, got %d", report.AccountsImported)
	}
	if report.LogsImported != 3 {
		t.Fatalf("expected 3 logs, got %d", report.LogsImported)
	}
	if report.BackupFileName == "" {
		t.Fatal("expected backup file in live mode")
	}

	backupPath := filepath.Join(app.backupsDir(), report.BackupFileName)
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("expected backup to exist: %v", err)
	}

	// Verify Go tables have data
	var count int
	// 8 defaults + 0 imported = 8
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM system_settings`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 8 {
		t.Fatalf("expected 8 settings (all defaults), got %d", count)
	}

	if err := app.db.QueryRow(`SELECT COUNT(*) FROM upstream_sites`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("expected 3 upstream_sites, got %d", count)
	}
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM channel_accounts`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected 2 channel_accounts, got %d", count)
	}
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM checkin_logs`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("expected 3 checkin_logs, got %d", count)
	}
}

func TestPythonMigrationIdempotent(t *testing.T) {
	pythonDBPath := filepath.Join(t.TempDir(), "zidqiandao.db")
	createTestPythonDB(t, pythonDBPath)

	app := newTestApp(t)
	defer app.Close()

	var err error

	// First run
	_, err = app.migrateFromPythonDB(context.Background(), pythonDBPath, "live")
	if err != nil {
		t.Fatal(err)
	}

	// Second run
	report2, err := app.migrateFromPythonDB(context.Background(), pythonDBPath, "live")
	if err != nil {
		t.Fatal(err)
	}

	if report2.SettingsImported != 0 {
		t.Fatalf("expected 0 settings on re-run, got %d", report2.SettingsImported)
	}
	if report2.SitesImported != 2 {
		t.Fatalf("expected 2 sites on re-run, got %d", report2.SitesImported)
	}
	if report2.AccountsImported != 2 {
		t.Fatalf("expected 2 accounts on re-run, got %d", report2.AccountsImported)
	}
	if report2.LogsImported != 3 {
		t.Fatalf("expected 3 logs on re-run, got %d", report2.LogsImported)
	}

	// Verify row counts are unchanged (no duplicates)
	var count int
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM upstream_sites`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("expected 3 upstream_sites, got %d", count)
	}
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM channel_accounts`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected 2 channel_accounts (no duplicates), got %d", count)
	}
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM checkin_logs`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("expected 3 checkin_logs (no duplicates), got %d", count)
	}
}

func TestPythonMigrationEmptyDB(t *testing.T) {
	pythonDBPath := filepath.Join(t.TempDir(), "python_empty.db")
	createEmptyTestPythonDB(t, pythonDBPath)

	app := newTestApp(t)
	defer app.Close()

	report, err := app.migrateFromPythonDB(context.Background(), pythonDBPath, "live")
	if err != nil {
		t.Fatal(err)
	}

	if report.SettingsImported != 0 {
		t.Fatalf("expected 0 settings, got %d", report.SettingsImported)
	}
	if report.SitesImported != 0 {
		t.Fatalf("expected 0 sites, got %d", report.SitesImported)
	}
	if report.AccountsImported != 0 {
		t.Fatalf("expected 0 accounts, got %d", report.AccountsImported)
	}
	if report.LogsImported != 0 {
		t.Fatalf("expected 0 logs, got %d", report.LogsImported)
	}
}

func TestPythonMigrationMissingFile(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()
	var err error

	_, err = app.migrateFromPythonDB(context.Background(), "/nonexistent/path/zidqiandao.db", "dry_run")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "文件不存在") {
		t.Fatalf("expected error about 文件不存在, got: %v", err)
	}
}

func TestPythonMigrationInvalidFile(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	invalidPath := filepath.Join(t.TempDir(), "not-a-db.txt")
	if err := os.WriteFile(invalidPath, []byte("this is not a sqlite file"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := app.migrateFromPythonDB(context.Background(), invalidPath, "live")
	if err == nil {
		t.Fatal("expected error for invalid sqlite file")
	}
}

func TestPythonMigrationStatusMapping(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"成功", "success"},
		{"已签到", "already_checked"},
		{"失败", "failed"},
		{"", "unknown"},
		{"random", "unknown"},
		{"未知状态", "unknown"},
	}

	for _, tt := range tests {
		result := mapPythonStatus(tt.input)
		if result != tt.expected {
			t.Errorf("mapPythonStatus(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestPythonMigrationGBKStatusMapping(t *testing.T) {
	gbkSuccess := string([]byte{0xB3, 0xC9, 0xB9, 0xA6})
	gbkAlreadyChecked := string([]byte{0xD2, 0xD1, 0xC7, 0xA9, 0xB5, 0xBD})
	gbkFailed := string([]byte{0xCA, 0xA7, 0xB0, 0xDC})

	tests := []struct {
		input    string
		expected string
	}{
		{gbkSuccess, "success"},
		{gbkAlreadyChecked, "already_checked"},
		{gbkFailed, "failed"},
	}

	for _, tt := range tests {
		result := mapPythonStatus(tt.input)
		if result != tt.expected {
			t.Errorf("mapPythonStatus(GBK) = %q, want %q (input bytes: % x)", result, tt.expected, []byte(tt.input))
		}
	}
}

func TestPythonMigrationEncryption(t *testing.T) {
	pythonDBPath := filepath.Join(t.TempDir(), "zidqiandao.db")
	createTestPythonDB(t, pythonDBPath)

	app := newTestApp(t)
	defer app.Close()
	var err error

	_, err = app.migrateFromPythonDB(context.Background(), pythonDBPath, "live")
	if err != nil {
		t.Fatal(err)
	}

	var passwordEncrypted string
	err = app.db.QueryRow(`SELECT password_encrypted FROM channel_accounts WHERE display_name = 'NewAPI 站点 · admin'`).Scan(&passwordEncrypted)
	if err != nil {
		t.Fatal(err)
	}
	if passwordEncrypted == "" {
		t.Fatal("expected non-empty encrypted password")
	}
	if passwordEncrypted == "test123" {
		t.Fatal("password should not be stored in plaintext")
	}
	if strings.Contains(passwordEncrypted, "test123") {
		t.Fatal("encrypted password should not contain plaintext")
	}

	decrypted, err := app.decryptText(passwordEncrypted)
	if err != nil {
		t.Fatal(err)
	}
	if decrypted != "test123" {
		t.Fatalf("expected decrypted password 'test123', got %q", decrypted)
	}
}

func TestPythonMigrationApiKeyEncryption(t *testing.T) {
	pythonDBPath := filepath.Join(t.TempDir(), "zidqiandao.db")
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(pythonDBPath))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT NOT NULL);
		CREATE TABLE local_channels (id INTEGER PRIMARY KEY, name TEXT NOT NULL, base_url TEXT NOT NULL, enabled INTEGER DEFAULT 1, detection_json TEXT DEFAULT '', source_type TEXT DEFAULT '', platform_override TEXT DEFAULT '');
		CREATE TABLE channel_credentials (id INTEGER PRIMARY KEY, channel_id INTEGER NOT NULL, site_name TEXT DEFAULT '', checkin_password TEXT DEFAULT '', auth_type TEXT DEFAULT '', auth_config TEXT DEFAULT '', login_url TEXT DEFAULT '', checkin_url TEXT DEFAULT '', email TEXT DEFAULT '', username TEXT DEFAULT '', cookie TEXT DEFAULT '', access_token TEXT DEFAULT '', api_key TEXT DEFAULT '');
		CREATE TABLE checkin_history (id INTEGER PRIMARY KEY, channel_id INTEGER NOT NULL, status TEXT DEFAULT '', balance TEXT DEFAULT '', message TEXT DEFAULT '', check_date TEXT DEFAULT '', credential_id INTEGER);
		INSERT INTO local_channels (id, name, base_url, enabled) VALUES (1, 'API 站点', 'https://api.example.com', 1);
		INSERT INTO channel_credentials (id, channel_id, site_name, auth_type, api_key) VALUES (1, 1, 'API 站点 · key', 'token', 'sk-test-api-key-12345');
	`)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	app := newTestApp(t)
	defer app.Close()

	_, err = app.migrateFromPythonDB(context.Background(), pythonDBPath, "live")
	if err != nil {
		t.Fatal(err)
	}

	var apiKeyEncrypted string
	err = app.db.QueryRow(`SELECT api_key_encrypted FROM channel_accounts WHERE display_name = 'API 站点 · key'`).Scan(&apiKeyEncrypted)
	if err != nil {
		t.Fatal(err)
	}
	if apiKeyEncrypted == "" {
		t.Fatal("expected non-empty encrypted api key")
	}
	if strings.Contains(apiKeyEncrypted, "sk-test") {
		t.Fatal("api key should not contain plaintext")
	}

	decrypted, err := app.decryptText(apiKeyEncrypted)
	if err != nil {
		t.Fatal(err)
	}
	if decrypted != "sk-test-api-key-12345" {
		t.Fatalf("expected decrypted api key 'sk-test-api-key-12345', got %q", decrypted)
	}
}

func TestPythonMigrationSettingsMerge(t *testing.T) {
	pythonDBPath := filepath.Join(t.TempDir(), "zidqiandao.db")
	createTestPythonDB(t, pythonDBPath)

	app := newTestApp(t)
	defer app.Close()
	var err error

	// Pre-populate checkin.schedule with a user-configured value
	existingSchedule := `{"enabled":false,"time":"10:00","randomDelayMinutes":[30,60],"siteConcurrency":2,"globalConcurrency":5}`
	_, err = app.db.Exec(`UPDATE system_settings SET value_json = ?, updated_at = ? WHERE key = 'checkin.schedule'`, existingSchedule, now())
	if err != nil {
		t.Fatal(err)
	}

	report, err := app.migrateFromPythonDB(context.Background(), pythonDBPath, "live")
	if err != nil {
		t.Fatal(err)
	}

	if report.SettingsImported != 0 {
		t.Fatalf("expected 0 new settings (user already configured), got %d", report.SettingsImported)
	}

	var valueJSON string
	err = app.db.QueryRow(`SELECT value_json FROM system_settings WHERE key = 'checkin.schedule'`).Scan(&valueJSON)
	if err != nil {
		t.Fatal(err)
	}
	if valueJSON != existingSchedule {
		t.Fatalf("expected existing schedule to be unchanged, got %q", valueJSON)
	}
}

func TestPythonMigrationMappedStatusesInDB(t *testing.T) {
	pythonDBPath := filepath.Join(t.TempDir(), "zidqiandao.db")
	createTestPythonDB(t, pythonDBPath)

	app := newTestApp(t)
	defer app.Close()
	var err error

	_, err = app.migrateFromPythonDB(context.Background(), pythonDBPath, "live")
	if err != nil {
		t.Fatal(err)
	}

	expectedRows := []struct {
		reward string
		status string
	}{
		{"100", "success"},
		{"0", "already_checked"},
		{"0", "failed"},
	}

	rows, err := app.db.Query(`SELECT reward, status FROM checkin_logs ORDER BY reward DESC, status`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	idx := 0
	for rows.Next() {
		var reward, status string
		if err := rows.Scan(&reward, &status); err != nil {
			t.Fatal(err)
		}
		if idx >= len(expectedRows) {
			t.Errorf("unexpected extra row: reward=%s, status=%s", reward, status)
			continue
		}
		if reward != expectedRows[idx].reward || status != expectedRows[idx].status {
			t.Errorf("row %d: expected (reward=%s, status=%s), got (reward=%s, status=%s)",
				idx, expectedRows[idx].reward, expectedRows[idx].status, reward, status)
		}
		idx++
	}
	if idx != len(expectedRows) {
		t.Errorf("expected %d rows, got %d", len(expectedRows), idx)
	}
}

func TestPythonMigrationDateConversion(t *testing.T) {
	tests := []struct {
		input string
		check string
	}{
		{"2026-06-20", "2026-06-20"},
		{"2026-06-20 15:04:05", "2026-06-20"},
		{"", ""},
		{"invalid", ""},
	}

	for _, tt := range tests {
		result := parsePythonDate(tt.input)
		if tt.check == "" {
			if result == "" {
				t.Errorf("parsePythonDate(%q) returned empty string", tt.input)
			}
		} else if !strings.Contains(result, tt.check) {
			t.Errorf("parsePythonDate(%q) = %q, want it to contain %q", tt.input, result, tt.check)
		}
	}
}

func TestPythonMigrationCreateSiteForMissingChannel(t *testing.T) {
	pythonDBPath := filepath.Join(t.TempDir(), "zidqiandao.db")

	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(pythonDBPath))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT NOT NULL);
		CREATE TABLE local_channels (id INTEGER PRIMARY KEY, name TEXT NOT NULL, base_url TEXT NOT NULL, enabled INTEGER DEFAULT 1, detection_json TEXT DEFAULT '', source_type TEXT DEFAULT '', platform_override TEXT DEFAULT '');
		CREATE TABLE channel_credentials (id INTEGER PRIMARY KEY, channel_id INTEGER NOT NULL, site_name TEXT DEFAULT '', checkin_password TEXT DEFAULT '', auth_type TEXT DEFAULT '', auth_config TEXT DEFAULT '', login_url TEXT DEFAULT '', checkin_url TEXT DEFAULT '', email TEXT DEFAULT '', username TEXT DEFAULT '', cookie TEXT DEFAULT '', access_token TEXT DEFAULT '', api_key TEXT DEFAULT '');
		CREATE TABLE checkin_history (id INTEGER PRIMARY KEY, channel_id INTEGER NOT NULL, status TEXT DEFAULT '', balance TEXT DEFAULT '', message TEXT DEFAULT '', check_date TEXT DEFAULT '', credential_id INTEGER);
		INSERT INTO channel_credentials (id, channel_id, site_name, checkin_password, auth_type) VALUES (1, -15, '孤立的站点', 'secret', 'password');
	`)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	app := newTestApp(t)
	defer app.Close()

	report, err := app.migrateFromPythonDB(context.Background(), pythonDBPath, "live")
	if err != nil {
		t.Fatal(err)
	}

	// 0 sites from local_channels (the table is empty); accounts auto-creates a minimal site
	if report.SitesImported != 0 {
		t.Fatalf("expected 0 sites (no local_channels), got %d", report.SitesImported)
	}
	if report.AccountsImported != 1 {
		t.Fatalf("expected 1 account, got %d", report.AccountsImported)
	}
}

func TestPythonMigrationEmptyPasswordHandling(t *testing.T) {
	pythonDBPath := filepath.Join(t.TempDir(), "zidqiandao.db")

	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(pythonDBPath))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT NOT NULL);
		CREATE TABLE local_channels (id INTEGER PRIMARY KEY, name TEXT NOT NULL, base_url TEXT NOT NULL, enabled INTEGER DEFAULT 1, detection_json TEXT DEFAULT '', source_type TEXT DEFAULT '', platform_override TEXT DEFAULT '');
		CREATE TABLE channel_credentials (id INTEGER PRIMARY KEY, channel_id INTEGER NOT NULL, site_name TEXT DEFAULT '', checkin_password TEXT DEFAULT '', auth_type TEXT DEFAULT '', auth_config TEXT DEFAULT '', login_url TEXT DEFAULT '', checkin_url TEXT DEFAULT '', email TEXT DEFAULT '', username TEXT DEFAULT '', cookie TEXT DEFAULT '', access_token TEXT DEFAULT '', api_key TEXT DEFAULT '');
		CREATE TABLE checkin_history (id INTEGER PRIMARY KEY, channel_id INTEGER NOT NULL, status TEXT DEFAULT '', balance TEXT DEFAULT '', message TEXT DEFAULT '', check_date TEXT DEFAULT '', credential_id INTEGER);
		INSERT INTO local_channels (id, name, base_url, enabled) VALUES (1, '站点', 'https://site.example.com', 1);
		INSERT INTO channel_credentials (id, channel_id, site_name, checkin_password, auth_type) VALUES (1, 1, '空密码账号', '', 'password');
	`)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	app := newTestApp(t)
	defer app.Close()

	_, err = app.migrateFromPythonDB(context.Background(), pythonDBPath, "live")
	if err != nil {
		t.Fatal(err)
	}

	var passwordEncrypted string
	err = app.db.QueryRow(`SELECT COALESCE(password_encrypted,'') FROM channel_accounts WHERE display_name = '空密码账号'`).Scan(&passwordEncrypted)
	if err != nil {
		t.Fatal(err)
	}
	if passwordEncrypted != "" {
		t.Fatalf("expected empty password_encrypted for empty password, got %q", passwordEncrypted)
	}
}

func TestPythonMigrationVerifyTables(t *testing.T) {
	// Create a DB with only some tables to verify table-check fails
	pythonDBPath := filepath.Join(t.TempDir(), "partial.db")
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(pythonDBPath))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT NOT NULL)`)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	app := newTestApp(t)
	defer app.Close()

	_, err = app.migrateFromPythonDB(context.Background(), pythonDBPath, "live")
	if err == nil {
		t.Fatal("expected error for missing tables")
	}
}

// TestHandleMigratePythonDBLive verifies the /api/system/migrate-python-db
// handler accepts {"sourcePath": "..."} and performs an idempotent live migration.
func TestHandleMigratePythonDBLive(t *testing.T) {
	pythonDBPath := filepath.Join(t.TempDir(), "zidqiandao.db")
	createTestPythonDB(t, pythonDBPath)

	app := newTestApp(t)
	defer app.Close()

	body, _ := json.Marshal(map[string]string{"sourcePath": pythonDBPath})
	req := httptest.NewRequest(http.MethodPost, "/api/system/migrate-python-db", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	app.handleMigratePythonDB(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		OK   bool                `json:"ok"`
		Data pythonMigrateReport `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.OK {
		t.Fatalf("expected ok=true, got false: %s", rec.Body.String())
	}
	if resp.Data.Mode != "live" {
		t.Fatalf("expected mode live (default), got %s", resp.Data.Mode)
	}
	if resp.Data.SitesImported != 2 {
		t.Fatalf("expected 2 sites, got %d", resp.Data.SitesImported)
	}
	if resp.Data.AccountsImported != 2 {
		t.Fatalf("expected 2 accounts, got %d", resp.Data.AccountsImported)
	}
	if resp.Data.LogsImported != 3 {
		t.Fatalf("expected 3 logs, got %d", resp.Data.LogsImported)
	}
	if resp.Data.BackupFileName == "" {
		t.Fatal("expected backup file in live mode")
	}
}

// TestHandleMigratePythonDBDryRun verifies the handler honors mode=dry_run.
func TestHandleMigratePythonDBDryRun(t *testing.T) {
	pythonDBPath := filepath.Join(t.TempDir(), "zidqiandao.db")
	createTestPythonDB(t, pythonDBPath)

	app := newTestApp(t)
	defer app.Close()

	body, _ := json.Marshal(map[string]string{"sourcePath": pythonDBPath, "mode": "dry_run"})
	req := httptest.NewRequest(http.MethodPost, "/api/system/migrate-python-db", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	app.handleMigratePythonDB(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		OK   bool                `json:"ok"`
		Data pythonMigrateReport `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Data.Mode != "dry_run" {
		t.Fatalf("expected mode dry_run, got %s", resp.Data.Mode)
	}
	if resp.Data.BackupFileName != "" {
		t.Fatalf("expected no backup in dry_run, got %s", resp.Data.BackupFileName)
	}

	// dry_run must not write any data
	var count int
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM upstream_sites`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 upstream_sites (__global__) after dry_run, got %d", count)
	}
}

// TestHandleMigratePythonDBMissingSourcePath verifies the handler rejects
// requests without a sourcePath parameter.
func TestHandleMigratePythonDBMissingSourcePath(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	body, _ := json.Marshal(map[string]string{"mode": "live"})
	req := httptest.NewRequest(http.MethodPost, "/api/system/migrate-python-db", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	app.handleMigratePythonDB(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing sourcePath, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "sourcePath") {
		t.Fatalf("expected error mentioning sourcePath, got %s", rec.Body.String())
	}
}

// TestHandleMigratePythonDBIdempotent verifies that calling the handler twice
// does not create duplicate data.
func TestHandleMigratePythonDBIdempotent(t *testing.T) {
	pythonDBPath := filepath.Join(t.TempDir(), "zidqiandao.db")
	createTestPythonDB(t, pythonDBPath)

	app := newTestApp(t)
	defer app.Close()

	// First call
	body, _ := json.Marshal(map[string]string{"sourcePath": pythonDBPath})
	req := httptest.NewRequest(http.MethodPost, "/api/system/migrate-python-db", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	app.handleMigratePythonDB(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("first call expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Second call
	req2 := httptest.NewRequest(http.MethodPost, "/api/system/migrate-python-db", bytes.NewReader(body))
	rec2 := httptest.NewRecorder()
	app.handleMigratePythonDB(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("second call expected 200, got %d: %s", rec2.Code, rec2.Body.String())
	}

	var count int
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM upstream_sites`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("expected 3 upstream_sites, got %d", count)
	}
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM channel_accounts`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected 2 channel_accounts (no duplicates), got %d", count)
	}
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM checkin_logs`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("expected 3 checkin_logs (no duplicates), got %d", count)
	}
}
