package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
)

func (a *App) handleImportFromSQLite(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}

	var input struct {
		DatabasePath      string `json:"databasePath"`
		ImportKeys        bool   `json:"importKeys"`
		InstanceName      string `json:"instanceName"`
		BaseURL           string `json:"baseUrl"`
		SkipCreateSites   bool   `json:"skipCreateSites"`
		DetectAfterImport bool   `json:"detectAfterImport"`
	}
	if err := decodeJSON(r, &input); err != nil || strings.TrimSpace(input.DatabasePath) == "" {
		writeError(w, http.StatusBadRequest, "SQLite 数据库路径不能为空。")
		return
	}

	result, err := a.importChannelsFromSQLite(r.Context(), input.DatabasePath, input.ImportKeys, input.InstanceName, input.BaseURL, !input.SkipCreateSites, input.DetectAfterImport)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	a.audit("import.sqlite", "info", "", "local_newapi_instance", stringFromResult(result, "instanceId"), "SQLite 渠道导入完成。", map[string]interface{}{
		"importedCount": intFromResult(result, "importedCount"),
		"sitesCreated":  intFromResult(result, "sitesCreated"),
		"sitesMerged":   intFromResult(result, "sitesMerged"),
		"detectedCount": intFromResult(result, "detectedCount"),
		"importKeys":    input.ImportKeys,
	})
	writeJSON(w, http.StatusOK, result)
}

func (a *App) importChannelsFromSQLite(ctx context.Context, dbPath string, importKeys bool, instanceName string, baseURL string, createSites bool, detectAfterImport bool) (map[string]interface{}, error) {
	return a.importChannelsFromSQLiteWithOptions(ctx, dbPath, importKeys, instanceName, baseURL, createSites, detectAfterImport, true)
}

func (a *App) importChannelsFromSQLiteWithOptions(ctx context.Context, dbPath string, importKeys bool, instanceName string, baseURL string, createSites bool, detectAfterImport bool, notify bool) (map[string]interface{}, error) {
	cleanPath, err := filepath.Abs(dbPath)
	if err != nil {
		return nil, err
	}
	source, err := sql.Open("sqlite", "file:"+filepath.ToSlash(cleanPath)+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer source.Close()

	var tableName string
	if err := source.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name='channels'`).Scan(&tableName); err != nil {
		return nil, fmt.Errorf("未找到 channels 表")
	}

	instanceID := newID()
	if instanceName == "" {
		instanceName = "SQLite 导入 " + filepath.Base(cleanPath)
	}
	if baseURL == "" {
		baseURL = "sqlite://" + filepath.ToSlash(cleanPath)
	}
	_, err = a.db.ExecContext(ctx, `
		INSERT INTO local_newapi_instances (id, name, base_url, detected_from, status, database_path, last_scanned_at, created_at, updated_at)
		VALUES (?, ?, ?, 'sqlite_import', 'unknown', ?, ?, ?, ?)
		ON CONFLICT(base_url) DO UPDATE SET name=excluded.name, database_path=excluded.database_path, last_scanned_at=excluded.last_scanned_at, updated_at=excluded.updated_at
	`, instanceID, instanceName, baseURL, cleanPath, now(), now(), now())
	if err != nil {
		return nil, err
	}
	if err := a.db.QueryRowContext(ctx, `SELECT id FROM local_newapi_instances WHERE base_url=?`, baseURL).Scan(&instanceID); err != nil {
		return nil, err
	}

	rows, err := source.QueryContext(ctx, `SELECT * FROM channels`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	imported := 0
	sitesCreated := 0
	sitesMerged := 0
	detected := 0
	for rows.Next() {
		values := make([]interface{}, len(columns))
		dest := make([]interface{}, len(columns))
		for i := range values {
			dest[i] = &values[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}

		record := map[string]interface{}{}
		for i, col := range columns {
			record[col] = normalizeDBValue(values[i])
		}

		sourceID := stringValue(record, "id")
		if sourceID == "" {
			sourceID = fmt.Sprintf("row-%d", imported+1)
		}
		name := stringValue(record, "name")
		if name == "" {
			name = "渠道 " + sourceID
		}
		channelBaseURL := extractImportedBaseURL(record)
		if isExcludedRelaySite(name, channelBaseURL) {
			continue
		}
		status := stringValue(record, "status")
		keyValue := stringValue(record, "key")
		keyEncrypted := ""
		keyMasked := ""
		if importKeys && keyValue != "" {
			keyEncrypted, err = a.encryptText(keyValue)
			if err != nil {
				return nil, err
			}
			keyMasked = maskSecret(keyValue)
		}
		rawJSON, _ := marshalImportedRecord(record, "sqlite")
		kind := inferImportedKind(record, channelBaseURL)

		channelID := newID()
		_, err = a.db.ExecContext(ctx, `
			INSERT INTO imported_channels (id, local_instance_id, source_channel_id, name, base_url, status, upstream_kind, channel_key_encrypted, channel_key_masked, raw_json, detection_json, source_sync_status, source_missing_at, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', 'active', '', ?, ?)
			ON CONFLICT(local_instance_id, source_channel_id) DO UPDATE SET
				name=excluded.name,
				base_url=excluded.base_url,
				status=excluded.status,
				upstream_kind=excluded.upstream_kind,
				channel_key_encrypted=CASE WHEN excluded.channel_key_encrypted='' THEN imported_channels.channel_key_encrypted ELSE excluded.channel_key_encrypted END,
				channel_key_masked=CASE WHEN excluded.channel_key_masked='' THEN imported_channels.channel_key_masked ELSE excluded.channel_key_masked END,
				raw_json=excluded.raw_json,
				detection_json=CASE WHEN excluded.detection_json='' THEN imported_channels.detection_json ELSE excluded.detection_json END,
				source_sync_status='active',
				source_missing_at='',
				updated_at=excluded.updated_at
		`, channelID, instanceID, sourceID, name, channelBaseURL, status, kind, keyEncrypted, keyMasked, rawJSON, now(), now())
		if err != nil {
			return nil, err
		}
		if err := a.db.QueryRowContext(ctx, `
			SELECT id FROM imported_channels
			WHERE local_instance_id=? AND source_channel_id=?
		`, instanceID, sourceID).Scan(&channelID); err != nil {
			return nil, err
		}

		if createSites && channelBaseURL != "" {
			var detection *UpstreamDetection
			if detectAfterImport {
				nextDetection := a.detectUpstream(ctx, channelBaseURL)
				detection = &nextDetection
				kind = nextDetection.Kind
				detected++
				_, err = a.db.ExecContext(ctx, `
					UPDATE imported_channels
					SET base_url=?, upstream_kind=?, supports_checkin=?, supports_balance=?, supports_models=?, supports_pricing=?, detection_json=?, last_detected_at=?, updated_at=?
					WHERE id=?
				`, nextDetection.BaseURL, nextDetection.Kind, boolInt(nextDetection.SupportsCheckin), boolInt(nextDetection.SupportsBalance), boolInt(nextDetection.SupportsModels), boolInt(nextDetection.SupportsPricing), marshalDetection(&nextDetection), now(), now(), channelID)
				if err != nil {
					return nil, err
				}
			}
			_, created, err := a.ensureUpstreamSiteForChannel(ctx, channelID, name, channelBaseURL, kind, detection)
			if err != nil {
				return nil, err
			}
			if created {
				sitesCreated++
			} else {
				sitesMerged++
			}
		}
		imported++
	}

	if notify {
		a.notify("channels_imported", "success", "渠道导入完成", fmt.Sprintf("从 SQLite 导入 %d 条渠道，生成 %d 个站点，合并 %d 个站点。", imported, sitesCreated, sitesMerged), "local_newapi_instance", instanceID)
	}
	return map[string]interface{}{
		"instanceId":    instanceID,
		"importedCount": imported,
		"sitesCreated":  sitesCreated,
		"sitesMerged":   sitesMerged,
		"detectedCount": detected,
	}, nil
}

func normalizeDBValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case nil:
		return nil
	case []byte:
		return string(typed)
	default:
		return typed
	}
}

func stringValue(record map[string]interface{}, key string) string {
	value, ok := record[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func extractImportedBaseURL(record map[string]interface{}) string {
	for _, key := range []string{"base_url", "baseUrl", "url"} {
		if value := stringValue(record, key); value != "" {
			return strings.TrimRight(value, "/")
		}
	}
	config := stringValue(record, "config")
	if config != "" {
		var parsed map[string]interface{}
		if json.Unmarshal([]byte(config), &parsed) == nil {
			for _, key := range []string{"base_url", "baseUrl", "url"} {
				if value := stringValue(parsed, key); value != "" {
					return strings.TrimRight(value, "/")
				}
			}
		}
	}
	return ""
}

func inferImportedKind(record map[string]interface{}, baseURL string) string {
	combined := strings.ToLower(fmt.Sprint(record) + " " + baseURL)
	switch {
	case strings.Contains(combined, "sub2api"):
		return "sub2api"
	case strings.Contains(combined, "oneapi") || strings.Contains(combined, "one api"):
		return "oneapi"
	case strings.Contains(combined, "newapi") || strings.Contains(combined, "new api"):
		return "newapi"
	case strings.Contains(combined, "openai"):
		return "openai_compatible"
	default:
		return "unknown"
	}
}
