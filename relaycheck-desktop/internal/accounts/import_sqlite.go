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
// Result map includes the same diagnostic counters as Admin API import.
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

	fetched := 0
	imported := 0
	skippedExcluded := 0
	skippedNoBaseURL := 0
	sitesCreated := 0
	sitesMerged := 0
	detected := 0
	excludedSamples := make([]ExcludedChannelSample, 0, 8)
	excludedTruncated := false
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

		fetched++
		outcome, err := s.importChannelRecordOutcome(ctx, instanceID, record, importKeys, createSites, detectAfterImport, "sqlite")
		if err != nil {
			return nil, err
		}
		switch outcome.Skip {
		case importSkipExcluded:
			skippedExcluded++
			if len(excludedSamples) < MaxExcludedSamples {
				excludedSamples = append(excludedSamples, ExcludedChannelSample{
					SourceChannelID: outcome.SourceChannelID,
					Name:            outcome.Name,
					MatchedToken:    outcome.MatchedToken,
				})
			} else {
				excludedTruncated = true
			}
		case importSkipNone:
			imported++
			if outcome.NoBaseURL {
				skippedNoBaseURL++
			}
			if outcome.Created {
				sitesCreated++
			}
			if outcome.Merged {
				sitesMerged++
			}
			if outcome.DidDetect {
				detected++
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if notify {
		s.infra.Notify("channels_imported", "success", "渠道导入完成", fmt.Sprintf("从 SQLite 导入 %d 条渠道（拉取 %d，排除 %d，无地址 %d），生成 %d 个站点，合并 %d 个站点。", imported, fetched, skippedExcluded, skippedNoBaseURL, sitesCreated, sitesMerged), "local_newapi_instance", instanceID)
	}
	result := map[string]interface{}{
		"instanceId":       instanceID,
		"fetchedCount":     fetched,
		"importedCount":    imported,
		"skippedExcluded":  skippedExcluded,
		"skippedNoBaseURL": skippedNoBaseURL,
		"sitesCreated":     sitesCreated,
		"sitesMerged":      sitesMerged,
		"detectedCount":    detected,
	}
	if len(excludedSamples) > 0 {
		result["skippedExcludedSamples"] = excludedSamples
		result["skippedExcludedTruncated"] = excludedTruncated
	}
	return result, nil
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
