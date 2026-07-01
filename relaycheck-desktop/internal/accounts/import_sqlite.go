package accounts

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
)

// ImportChannelsFromSQLite imports channels from a local NewAPI SQLite DB.
func (s *Service) ImportChannelsFromSQLite(ctx context.Context, dbPath string, importKeys bool, instanceName string, baseURL string, createSites bool, detectAfterImport bool) (map[string]interface{}, error) {
	return s.ImportChannelsFromSQLiteWithOptions(ctx, dbPath, importKeys, instanceName, baseURL, createSites, detectAfterImport, true)
}

// ImportChannelsFromSQLiteWithOptions imports channels from a local NewAPI
// SQLite DB, with a flag to suppress notifications (used by scheduled sync).
func (s *Service) ImportChannelsFromSQLiteWithOptions(ctx context.Context, dbPath string, importKeys bool, instanceName string, baseURL string, createSites bool, detectAfterImport bool, notify bool) (map[string]interface{}, error) {
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

	instanceID := s.infra.NewID()
	if instanceName == "" {
		instanceName = "SQLite 导入 " + filepath.Base(cleanPath)
	}
	if baseURL == "" {
		baseURL = "sqlite://" + filepath.ToSlash(cleanPath)
	}
	_, err = s.infra.DB().ExecContext(ctx, `
		INSERT INTO local_newapi_instances (id, name, base_url, detected_from, status, database_path, last_scanned_at, created_at, updated_at)
		VALUES (?, ?, ?, 'sqlite_import', 'unknown', ?, ?, ?, ?)
		ON CONFLICT(base_url) DO UPDATE SET name=excluded.name, database_path=excluded.database_path, last_scanned_at=excluded.last_scanned_at, updated_at=excluded.updated_at
	`, instanceID, instanceName, baseURL, cleanPath, s.infra.Now(), s.infra.Now(), s.infra.Now())
	if err != nil {
		return nil, err
	}
	if err := s.infra.DB().QueryRowContext(ctx, `SELECT id FROM local_newapi_instances WHERE base_url=?`, baseURL).Scan(&instanceID); err != nil {
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
			keyEncrypted, err = s.infra.EncryptText(keyValue)
			if err != nil {
				return nil, err
			}
			keyMasked = maskSecret(keyValue)
		}
		rawJSON, _ := marshalImportedRecord(record, "sqlite")
		kind := inferImportedKind(record, channelBaseURL)

		channelID := s.infra.NewID()
		_, err = s.infra.DB().ExecContext(ctx, `
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
		`, channelID, instanceID, sourceID, name, channelBaseURL, status, kind, keyEncrypted, keyMasked, rawJSON, s.infra.Now(), s.infra.Now())
		if err != nil {
			return nil, err
		}
		if err := s.infra.DB().QueryRowContext(ctx, `
			SELECT id FROM imported_channels
			WHERE local_instance_id=? AND source_channel_id=?
		`, instanceID, sourceID).Scan(&channelID); err != nil {
			return nil, err
		}

		if createSites && channelBaseURL != "" {
			var detection *Detection
			if detectAfterImport {
				nextDetection, detectErr := s.infra.DetectUpstreamForImport(ctx, channelBaseURL)
				if detectErr != nil {
					return nil, detectErr
				}
				detection = &nextDetection
				kind = nextDetection.Kind
				detected++
				_, err = s.infra.DB().ExecContext(ctx, `
					UPDATE imported_channels
					SET base_url=?, upstream_kind=?, supports_checkin=?, supports_balance=?, supports_models=?, supports_pricing=?, detection_json=?, last_detected_at=?, updated_at=?
					WHERE id=?
				`, nextDetection.BaseURL, nextDetection.Kind, boolInt(nextDetection.SupportsCheckin), boolInt(nextDetection.SupportsBalance), boolInt(nextDetection.SupportsModels), boolInt(nextDetection.SupportsPricing), mustJSON(nextDetection), s.infra.Now(), s.infra.Now(), channelID)
				if err != nil {
					return nil, err
				}
			}
			_, created, err := s.infra.EnsureChannelSiteForImport(ctx, channelID, name, channelBaseURL, kind, detection)
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
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if notify {
		s.infra.Notify("channels_imported", "success", "渠道导入完成", fmt.Sprintf("从 SQLite 导入 %d 条渠道，生成 %d 个站点，合并 %d 个站点。", imported, sitesCreated, sitesMerged), "local_newapi_instance", instanceID)
	}
	return map[string]interface{}{
		"instanceId":    instanceID,
		"importedCount": imported,
		"sitesCreated":  sitesCreated,
		"sitesMerged":   sitesMerged,
		"detectedCount": detected,
	}, nil
}

// readSQLiteChannelRecords reads all rows from the channels table of a local
// SQLite DB into record maps. Used by the sync-preview flow.
func readSQLiteChannelRecords(ctx context.Context, dbPath string) (string, []map[string]interface{}, error) {
	cleanPath, err := filepath.Abs(dbPath)
	if err != nil {
		return "", nil, err
	}
	source, err := sql.Open("sqlite", "file:"+filepath.ToSlash(cleanPath)+"?mode=ro")
	if err != nil {
		return "", nil, err
	}
	defer source.Close()

	var tableName string
	if err := source.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name='channels'`).Scan(&tableName); err != nil {
		return "", nil, fmt.Errorf("未找到 channels 表")
	}

	rows, err := source.QueryContext(ctx, `SELECT * FROM channels`)
	if err != nil {
		return "", nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return "", nil, err
	}

	records := []map[string]interface{}{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		dest := make([]interface{}, len(columns))
		for i := range values {
			dest[i] = &values[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return "", nil, err
		}
		record := map[string]interface{}{}
		for i, col := range columns {
			record[col] = normalizeDBValue(values[i])
		}
		records = append(records, record)
	}
	return cleanPath, records, rows.Err()
}

// probeSQLiteHasChannels reports whether the SQLite DB at dbPath has a
// "channels" table. Pure function: does not access *Service state.
func probeSQLiteHasChannels(ctx context.Context, dbPath string) bool {
	source, err := sql.Open("sqlite", "file:"+filepath.ToSlash(dbPath)+"?mode=ro")
	if err != nil {
		return false
	}
	defer source.Close()

	var tableName string
	err = source.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name='channels'`).Scan(&tableName)
	return err == nil && tableName == "channels"
}

// defaultNewAPISearchPaths lists common NewAPI SQLite DB locations.
var defaultNewAPISearchPaths = []string{
	`D:\newapi\data\one-api.db`,
	`D:\new-api\data\one-api.db`,
	`one-api.db`,
	`data\one-api.db`,
}

// defaultNewAPISearchDirs lists common NewAPI install directories.
var defaultNewAPISearchDirs = []string{
	`D:\newapi`,
	`D:\new-api`,
}
