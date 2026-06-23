package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	_ "modernc.org/sqlite"
)

// pythonMigrateReport holds the summary of a Python-to-Go migration.
type pythonMigrateReport struct {
	Mode             string `json:"mode"`
	SettingsImported int    `json:"settingsImported"`
	SitesImported    int    `json:"sitesImported"`
	AccountsImported int    `json:"accountsImported"`
	LogsImported     int    `json:"logsImported"`
	BackupFileName   string `json:"backupFileName,omitempty"`
	Error            string `json:"error,omitempty"`
}

type pythonMigrateBody struct {
	DatabasePath string `json:"databasePath"`
	Mode         string `json:"mode"`
}

// handleMigrateFromPythonDB is the HTTP handler for POST /api/system/migrate-from-python-db.
func (a *App) handleMigrateFromPythonDB(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}

	var input pythonMigrateBody
	if err := decodeJSON(r, &input); err != nil || strings.TrimSpace(input.DatabasePath) == "" {
		writeError(w, http.StatusBadRequest, "缺少 databasePath 参数。")
		return
	}

	if input.Mode != "dry_run" && input.Mode != "live" {
		writeError(w, http.StatusBadRequest, "mode 必须是 dry_run 或 live。")
		return
	}

	report, err := a.migrateFromPythonDB(r.Context(), input.DatabasePath, input.Mode)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if input.Mode == "live" {
		a.audit("migrate.python_db", "info", "", "system", "", "Python 数据库迁移完成。", map[string]interface{}{
			"settingsImported": report.SettingsImported,
			"sitesImported":    report.SitesImported,
			"accountsImported": report.AccountsImported,
			"logsImported":     report.LogsImported,
			"backupFileName":   report.BackupFileName,
		})
		a.notify("migrate_python_db", "success", "Python 数据库迁移完成",
			fmt.Sprintf("迁移了 %d 个设置, %d 个站点, %d 个账号, %d 条签到记录。", report.SettingsImported, report.SitesImported, report.AccountsImported, report.LogsImported),
			"system", "python-migration")
	}

	writeJSON(w, http.StatusOK, report)
}

// migrateFromPythonDB is the core migration function.
func (a *App) migrateFromPythonDB(ctx context.Context, dbPath string, mode string) (*pythonMigrateReport, error) {
	writeMode := mode == "live"

	// 1. Validate DB file exists
	cleanPath, err := filepath.Abs(dbPath)
	if err != nil {
		return nil, fmt.Errorf("数据库路径无效: %w", err)
	}
	if _, err := os.Stat(cleanPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("Python 数据库文件不存在")
		}
		return nil, err
	}

	// 2. Validate SQLite header
	if err := validateSQLiteFile(cleanPath); err != nil {
		return nil, fmt.Errorf("无效的 SQLite 文件: %w", err)
	}

	// 3. Open read-only
	source, err := openPythonDBReadOnly(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("无法打开 Python 数据库: %w", err)
	}
	defer source.Close()

	// 4. Verify required tables exist
	if err := verifyPythonTables(ctx, source); err != nil {
		return nil, err
	}

	// 5. Live mode: create backup first
	backupFileName := ""
	if writeMode {
		backup, err := a.createBackup("before-python-migration")
		if err != nil {
			return nil, fmt.Errorf("备份失败: %w", err)
		}
		backupFileName = backup.FileName
	}

	// 6. Import settings
	settingsCount, err := a.importPythonSettings(ctx, source, writeMode)
	if err != nil {
		return nil, fmt.Errorf("设置导入失败: %w", err)
	}

	// 7. Import sites (local_channels -> upstream_sites)
	channelIDMap, sitesCount, err := a.importPythonSites(ctx, source, writeMode)
	if err != nil {
		return nil, fmt.Errorf("站点导入失败: %w", err)
	}

	// 8. Import accounts (channel_credentials -> channel_accounts)
	credentialIDMap, accountsCount, err := a.importPythonAccounts(ctx, source, channelIDMap, writeMode)
	if err != nil {
		return nil, fmt.Errorf("账号导入失败: %w", err)
	}

	// 9. Import checkin logs
	logsCount, err := a.importPythonCheckinLogs(ctx, source, credentialIDMap, channelIDMap, writeMode)
	if err != nil {
		return nil, fmt.Errorf("签到记录导入失败: %w", err)
	}

	return &pythonMigrateReport{
		Mode:             mode,
		SettingsImported: settingsCount,
		SitesImported:    sitesCount,
		AccountsImported: accountsCount,
		LogsImported:     logsCount,
		BackupFileName:   backupFileName,
	}, nil
}

// openPythonDBReadOnly opens a Python SQLite database in read-only mode.
func openPythonDBReadOnly(dbPath string) (*sql.DB, error) {
	cleanPath, err := filepath.Abs(dbPath)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(cleanPath)+"?mode=ro")
	if err != nil {
		return nil, err
	}
	return db, nil
}

// verifyPythonTables checks that all 4 required tables exist in the Python DB.
func verifyPythonTables(ctx context.Context, source *sql.DB) error {
	tables := []string{"settings", "local_channels", "channel_credentials", "checkin_history"}
	for _, table := range tables {
		var name string
		err := source.QueryRowContext(ctx,
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table,
		).Scan(&name)
		if err != nil {
			return fmt.Errorf("Python 数据库中缺少 %s 表", table)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Settings import
// ---------------------------------------------------------------------------

func (a *App) importPythonSettings(ctx context.Context, source *sql.DB, writeMode bool) (int, error) {
	rows, err := source.QueryContext(ctx, `SELECT key, value FROM settings`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	pyValues := map[string]string{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return 0, err
		}
		pyValues[key] = value
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	count := 0

	// Build checkin.schedule JSON from Python values
	if _, hasEnabled := pyValues["schedule_enabled"]; hasEnabled || pyValues["delay_min"] != "" || pyValues["delay_max"] != "" || pyValues["concurrent"] != "" {
		enabled := pyValues["schedule_enabled"] == "1"
		delayMin := intVal(pyValues["delay_min"], 0)
		delayMax := intVal(pyValues["delay_max"], 120)
		concurrent := intVal(pyValues["concurrent"], 3)
		scheduleJSON := fmt.Sprintf(`{"enabled":%v,"randomDelayMinutes":[%d,%d],"globalConcurrency":%d,"siteConcurrency":1}`,
			enabled, delayMin, delayMax, concurrent)
		imported, err := a.importSettingIfNotExists(ctx, "checkin.schedule", scheduleJSON, writeMode)
		if err != nil {
			return 0, err
		}
		count += imported
	}

	// Build network.proxy JSON
	if _, hasProxy := pyValues["enable_proxy"]; hasProxy || pyValues["proxy_list"] != "" {
		proxyEnabled := pyValues["enable_proxy"] == "1"
		proxyURL := pyValues["proxy_list"]
		proxyJSON := fmt.Sprintf(`{"enabled":%v,"url":"%s","bypassLocal":true}`, proxyEnabled, proxyURL)
		imported, err := a.importSettingIfNotExists(ctx, "network.proxy", proxyJSON, writeMode)
		if err != nil {
			return 0, err
		}
		count += imported
	}

	return count, nil
}

func (a *App) importSettingIfNotExists(ctx context.Context, goKey string, valueJSON string, writeMode bool) (int, error) {
	if !writeMode {
		return 1, nil
	}

	var existing int
	if err := a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM system_settings WHERE key = ?`, goKey).Scan(&existing); err != nil {
		return 0, err
	}
	if existing > 0 {
		return 0, nil
	}

	_, err := a.db.ExecContext(ctx, `
		INSERT INTO system_settings (id, key, value_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`, newID(), goKey, valueJSON, now(), now())
	if err != nil {
		return 0, err
	}
	return 1, nil
}

func intVal(value string, defaultVal int) int {
	if value == "" {
		return defaultVal
	}
	var result int
	if _, err := fmt.Sscanf(value, "%d", &result); err != nil {
		return defaultVal
	}
	return result
}

// ---------------------------------------------------------------------------
// Sites import: local_channels -> upstream_sites
// ---------------------------------------------------------------------------

func (a *App) importPythonSites(ctx context.Context, source *sql.DB, writeMode bool) (map[int64]string, int, error) {
	channelIDMap := map[int64]string{}

	rows, err := source.QueryContext(ctx, `SELECT id, name, base_url, enabled, COALESCE(detection_json,''), COALESCE(source_type,''), COALESCE(platform_override,'') FROM local_channels`)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id int64
		var name, baseURL, detectionJSON, sourceType, platformOverride string
		var enabled int
		if err := rows.Scan(&id, &name, &baseURL, &enabled, &detectionJSON, &sourceType, &platformOverride); err != nil {
			return nil, 0, err
		}

		baseURL = strings.TrimRight(baseURL, "/")
		if baseURL == "" {
			continue
		}

		if !writeMode {
			channelIDMap[id] = ""
			count++
			continue
		}

		// Idempotency: check by base_url
		var existingID string
		err := a.db.QueryRowContext(ctx, `SELECT id FROM upstream_sites WHERE base_url = ?`, baseURL).Scan(&existingID)
		if err == nil {
			channelIDMap[id] = existingID
			count++
			continue
		}

		// Determine kind
		kind := platformOverride
		if kind == "" {
			kind = sourceType
		}
		if kind == "" && detectionJSON != "" {
			kind = inferKindFromDetection(detectionJSON)
		}
		if kind == "" {
			kind = "unknown"
		}

		// Check if detection_json contains checkin_enabled
		supportsCheckin := 0
		if detectionJSON != "" {
			var parsed map[string]interface{}
			if json.Unmarshal([]byte(detectionJSON), &parsed) == nil {
				if checkinEnabled, ok := parsed["checkin_enabled"].(bool); ok && checkinEnabled {
					supportsCheckin = 1
				}
			}
		}

		// Mark detection_json as migrated from Python
		if detectionJSON != "" {
			var parsed map[string]interface{}
			if json.Unmarshal([]byte(detectionJSON), &parsed) == nil {
				parsed["migrated_from"] = "python_zidqiandao"
				if data, err := json.Marshal(parsed); err == nil {
					detectionJSON = string(data)
				}
			}
		}

		healthStatus := "unknown"
		if enabled == 1 {
			healthStatus = "healthy"
		}

		siteID := newID()
		_, err = a.db.ExecContext(ctx, `
			INSERT INTO upstream_sites (id, name, homepage_url, base_url, kind, health_status, supports_checkin, detection_json, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, siteID, name, baseURL, baseURL, kind, healthStatus, supportsCheckin, detectionJSON, now(), now())
		if err != nil {
			return nil, 0, err
		}

		channelIDMap[id] = siteID
		count++
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return channelIDMap, count, nil
}

func inferKindFromDetection(jsonStr string) string {
	lower := strings.ToLower(jsonStr)
	switch {
	case strings.Contains(lower, "newapi"):
		return "newapi"
	case strings.Contains(lower, "sub2api"):
		return "sub2api"
	case strings.Contains(lower, "oneapi"):
		return "oneapi"
	default:
		return "unknown"
	}
}

// ---------------------------------------------------------------------------
// Accounts import: channel_credentials -> channel_accounts
// ---------------------------------------------------------------------------

func (a *App) importPythonAccounts(ctx context.Context, source *sql.DB, channelIDMap map[int64]string, writeMode bool) (map[int64]string, int, error) {
	credentialIDMap := map[int64]string{}

	rows, err := source.QueryContext(ctx, `SELECT id, channel_id, COALESCE(site_name,''), COALESCE(checkin_password,''), COALESCE(auth_type,''), COALESCE(auth_config,''), COALESCE(login_url,''), COALESCE(checkin_url,''), COALESCE(email,''), COALESCE(username,''), COALESCE(cookie,''), COALESCE(access_token,''), COALESCE(api_key,'') FROM channel_credentials`)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id, channelID int64
		var siteName, password, authType, authConfig, loginURL, checkinURL, email, username, cookie, accessToken, apiKey string
		if err := rows.Scan(&id, &channelID, &siteName, &password, &authType, &authConfig, &loginURL, &checkinURL, &email, &username, &cookie, &accessToken, &apiKey); err != nil {
			return nil, 0, err
		}

		if !writeMode {
			credentialIDMap[id] = ""
			count++
			continue
		}

		// Lookup or create upstream_site for this channel_id
		siteID := channelIDMap[channelID]
		if siteID == "" {
			siteID = newID()
			baseURL := fmt.Sprintf("python-import://channel-%d", channelID)
			_, err := a.db.ExecContext(ctx, `
				INSERT INTO upstream_sites (id, name, homepage_url, base_url, kind, health_status, created_at, updated_at)
				VALUES (?, ?, ?, ?, 'unknown', 'unknown', ?, ?)
			`, siteID, siteName, baseURL, baseURL, now(), now())
			if err != nil {
				return nil, 0, err
			}
			channelIDMap[channelID] = siteID
		}

		// Idempotency check
		var existingID string
		err := a.db.QueryRowContext(ctx, `SELECT id FROM channel_accounts WHERE upstream_site_id = ? AND display_name = ?`, siteID, siteName).Scan(&existingID)
		if err == nil {
			credentialIDMap[id] = existingID
			count++
			continue
		}

		// Encrypt secrets
		passwordEncrypted, err := a.encryptText(password)
		if err != nil {
			return nil, 0, err
		}
		cookieEncrypted, err := a.encryptText(cookie)
		if err != nil {
			return nil, 0, err
		}
		accessTokenEncrypted, err := a.encryptText(accessToken)
		if err != nil {
			return nil, 0, err
		}
		apiKeyEncrypted, err := a.encryptText(apiKey)
		if err != nil {
			return nil, 0, err
		}

		mappedAuthType := mapPythonAuthType(authType)

		accountID := newID()
		_, err = a.db.ExecContext(ctx, `
			INSERT INTO channel_accounts (id, upstream_site_id, display_name, email, username, auth_type, password_encrypted, cookie_encrypted, access_token_encrypted, api_key_encrypted, auth_user_id, login_status, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'unknown', ?, ?)
		`, accountID, siteID, siteName, email, username, mappedAuthType, passwordEncrypted, cookieEncrypted, accessTokenEncrypted, apiKeyEncrypted, "python_migration", now(), now())
		if err != nil {
			return nil, 0, err
		}

		// Update parent upstream_site with login_url, checkin config, and platform
		updateFields := []string{}
		var updateArgs []interface{}
		if loginURL != "" {
			updateFields = append(updateFields, "login_url=?")
			updateArgs = append(updateArgs, loginURL)
		}
		if checkinURL != "" {
			checkinConfig := fmt.Sprintf(`{"method":"POST","url":"%s"}`, checkinURL)
			updateFields = append(updateFields, "checkin_config_json=?")
			updateArgs = append(updateArgs, checkinConfig)
			updateFields = append(updateFields, "supports_checkin=1")
		}
		if authConfig != "" {
			var parsed map[string]interface{}
			if json.Unmarshal([]byte(authConfig), &parsed) == nil {
				if platform, ok := parsed["platform"].(string); ok && platform != "" {
					updateFields = append(updateFields, "kind=CASE WHEN kind='unknown' THEN ? ELSE kind END")
					updateArgs = append(updateArgs, platform)
				}
			}
		}

		if len(updateFields) > 0 {
			updateArgs = append(updateArgs, now(), siteID)
			sqlStr := fmt.Sprintf(`UPDATE upstream_sites SET %s, updated_at=? WHERE id=?`, strings.Join(updateFields, ", "))
			_, err = a.db.ExecContext(ctx, sqlStr, updateArgs...)
			if err != nil {
				return nil, 0, err
			}
		}

		credentialIDMap[id] = accountID
		count++
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return credentialIDMap, count, nil
}

func mapPythonAuthType(authType string) string {
	switch strings.TrimSpace(authType) {
	case "password":
		return "email_password"
	case "cookie":
		return "browser_profile"
	case "token":
		return "api_key"
	default:
		return "email_password"
	}
}

// ---------------------------------------------------------------------------
// Checkin logs import: checkin_history -> checkin_logs
// ---------------------------------------------------------------------------

func (a *App) importPythonCheckinLogs(ctx context.Context, source *sql.DB, credentialIDMap map[int64]string, channelIDMap map[int64]string, writeMode bool) (int, error) {
	rows, err := source.QueryContext(ctx, `SELECT id, channel_id, COALESCE(status,''), COALESCE(balance,''), COALESCE(message,''), COALESCE(check_date,''), COALESCE(credential_id,0) FROM checkin_history`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id, channelID int64
		var status, balance, message, checkDate string
		var credentialID int64
		if err := rows.Scan(&id, &channelID, &status, &balance, &message, &checkDate, &credentialID); err != nil {
			return 0, err
		}

		mappedStatus := mapPythonStatus(status)
		startedAt := parsePythonDate(checkDate)
		accountID := credentialIDMap[credentialID]
		siteID := channelIDMap[channelID]
		channelIDStr := ""
		if channelID != 0 {
			channelIDStr = fmt.Sprint(channelID)
		}

		if !writeMode {
			count++
			continue
		}

		// Idempotency check
		if accountID != "" {
			var existing string
			err := a.db.QueryRowContext(ctx, `SELECT id FROM checkin_logs WHERE account_id = ? AND started_at = ?`, accountID, startedAt).Scan(&existing)
			if err == nil {
				count++
				continue
			}
		}

		logID := newID()
		_, err = a.db.ExecContext(ctx, `
			INSERT INTO checkin_logs (id, account_id, upstream_site_id, channel_id, status, reward, message, started_at, finished_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, logID, accountID, siteID, channelIDStr, mappedStatus, balance, message, startedAt, startedAt)
		if err != nil {
			return 0, err
		}

		count++
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	return count, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// mapPythonStatus converts Chinese status strings to Go status values.
func mapPythonStatus(status string) string {
	if utf8.ValidString(status) {
		switch {
		case status == "":
			return "unknown"
		case strings.Contains(status, "成功"):
			return "success"
		case strings.Contains(status, "已签到"):
			return "already_checked"
		case strings.Contains(status, "失败"):
			return "failed"
		default:
			return "unknown"
		}
	}
	return mapPythonStatusGBK(status)
}

// mapPythonStatusGBK handles GBK-encoded status strings via raw byte matching.
func mapPythonStatusGBK(status string) string {
	raw := []byte(status)
	switch {
	case containsBytes(raw, gbkSuccess):
		return "success"
	case containsBytes(raw, gbkAlreadyChecked):
		return "already_checked"
	case containsBytes(raw, gbkFailed):
		return "failed"
	default:
		return "unknown"
	}
}

// GBK byte patterns for known Chinese status values.
var (
	gbkSuccess        = []byte{0xB3, 0xC9, 0xB9, 0xA6}          // 成功
	gbkAlreadyChecked = []byte{0xD2, 0xD1, 0xC7, 0xA9, 0xB5, 0xBD} // 已签到
	gbkFailed         = []byte{0xCA, 0xA7, 0xB0, 0xDC}          // 失败
)

func containsBytes(haystack, needle []byte) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) && strings.Contains(string(haystack), string(needle))
}

// decodePythonChineseText decodes text that may be GBK-encoded (Python 2 legacy).
// Valid UTF-8 is returned as-is; non-UTF-8 raw bytes are returned unchanged
// (callers should use mapPythonStatus for status fields, which handles GBK bytes).
func decodePythonChineseText(raw string) string {
	if utf8.ValidString(raw) {
		return raw
	}
	return raw
}

// parsePythonDate converts "YYYY-MM-DD" or "YYYY-MM-DD HH:MM:SS" to RFC3339Nano.
func parsePythonDate(date string) string {
	date = strings.TrimSpace(date)
	if date == "" {
		return now()
	}
	if t, err := time.Parse("2006-01-02 15:04:05", date); err == nil {
		return t.UTC().Format(time.RFC3339Nano)
	}
	if t, err := time.Parse("2006-01-02", date); err == nil {
		return t.UTC().Format(time.RFC3339Nano)
	}
	return now()
}

// pythonSettingToJSON converts a plain Python setting value to JSON representation.
func pythonSettingToJSON(value string) string {
	if value == "" {
		return ""
	}
	// Try numeric
	if _, err := fmt.Sscanf(value, "%d", new(int)); err == nil {
		return value
	}
	if _, err := fmt.Sscanf(value, "%f", new(float64)); err == nil {
		return value
	}
	// Boolean-like
	if value == "0" || value == "1" {
		return value
	}
	// String: wrap in quotes
	data, err := json.Marshal(value)
	if err != nil {
		return `""`
	}
	return string(data)
}