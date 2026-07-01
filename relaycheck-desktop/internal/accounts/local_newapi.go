package accounts

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
)

// ListLocalNewAPIInstances returns all local_newapi_instances rows with their
// channel counts, for the instance-list endpoint.
func (s *Service) ListLocalNewAPIInstances(ctx context.Context) ([]LocalNewAPIInstance, error) {
	rows, err := s.infra.DB().QueryContext(ctx, `
		SELECT i.id, i.name, i.base_url, COALESCE(i.detected_from,''), i.status,
		       COALESCE(i.version,''), COALESCE(i.database_path,''), COALESCE(i.last_scanned_at,''),
		       COALESCE(i.sync_access_token_masked,''), i.created_at, i.updated_at,
		       (SELECT COUNT(*) FROM imported_channels c WHERE c.local_instance_id = i.id)
		FROM local_newapi_instances i
		ORDER BY i.updated_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []LocalNewAPIInstance{}
	for rows.Next() {
		var item LocalNewAPIInstance
		if err := rows.Scan(&item.ID, &item.Name, &item.BaseURL, &item.DetectedFrom, &item.Status, &item.Version, &item.DatabasePath, &item.LastScannedAt, &item.SyncTokenMasked, &item.CreatedAt, &item.UpdatedAt, &item.ChannelCount); err != nil {
			return nil, err
		}
		item.HasSyncToken = strings.TrimSpace(item.SyncTokenMasked) != ""
		item.SyncCapability = syncCapability(item)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

// SyncLocalNewAPIInstanceData syncs a local NewAPI instance by either
// re-importing from SQLite or calling the admin API, depending on the
// instance's capability.
func (s *Service) SyncLocalNewAPIInstanceData(ctx context.Context, id string, input SyncRunInput, notify bool) (map[string]interface{}, error) {
	if strings.TrimSpace(input.UserID) == "" {
		input.UserID = "1"
	}
	input.PageSize = clampInt(input.PageSize, 10, 100, 100)

	instance, err := s.GetLocalNewAPIInstance(ctx, id)
	if err == sql.ErrNoRows {
		return nil, errorsText("NewAPI 实例不存在。")
	}
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if strings.TrimSpace(instance.DatabasePath) != "" {
		result, err = s.ImportChannelsFromSQLiteWithOptions(ctx, instance.DatabasePath, input.ImportKeys, instance.Name, instance.BaseURL, !input.SkipCreateSites, input.DetectAfterImport, notify)
	} else if isHTTPURL(instance.BaseURL) {
		accessToken, err := s.resolveLocalNewAPISyncToken(ctx, instance, input.AccessToken)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(accessToken) == "" {
			return nil, errorsText("该实例需要填写系统访问令牌后才能通过后台 API 同步。")
		}
		result, err = s.ImportChannelsFromAdminAPIWithOptions(ctx, instance.BaseURL, accessToken, input.UserID, instance.Name, input.ImportKeys, !input.SkipCreateSites, input.DetectAfterImport, input.PageSize, notify)
		if err == nil {
			err = s.UpdateLocalNewAPISyncToken(ctx, instance.ID, input.AccessToken, input.SaveAccessToken, input.ClearAccessToken)
		}
	} else {
		return nil, errorsText("该实例没有可用的 SQLite 路径或后台 API 地址，无法同步。")
	}
	if err != nil {
		return nil, err
	}
	result["synced"] = true
	result["sourceInstanceId"] = id
	return result, nil
}

// GetLocalNewAPIInstance reads a single local_newapi_instances row by ID.
func (s *Service) GetLocalNewAPIInstance(ctx context.Context, id string) (LocalNewAPIInstance, error) {
	var instance LocalNewAPIInstance
	err := s.infra.DB().QueryRowContext(ctx, `
		SELECT id, name, base_url, COALESCE(detected_from,''), status,
		       COALESCE(version,''), COALESCE(database_path,''), COALESCE(last_scanned_at,''),
		       COALESCE(sync_access_token_encrypted,''), COALESCE(sync_access_token_masked,''), created_at, updated_at
		FROM local_newapi_instances
		WHERE id = ?
	`, id).Scan(&instance.ID, &instance.Name, &instance.BaseURL, &instance.DetectedFrom, &instance.Status, &instance.Version, &instance.DatabasePath, &instance.LastScannedAt, &instance.SyncTokenEncrypted, &instance.SyncTokenMasked, &instance.CreatedAt, &instance.UpdatedAt)
	if err == nil {
		instance.HasSyncToken = strings.TrimSpace(instance.SyncTokenMasked) != ""
		instance.SyncCapability = syncCapability(instance)
	}
	return instance, err
}

// resolveLocalNewAPISyncToken returns the access token to use for an admin
// API sync: the explicit input token if given, otherwise the stored encrypted
// token decrypted.
func (s *Service) resolveLocalNewAPISyncToken(ctx context.Context, instance LocalNewAPIInstance, inputToken string) (string, error) {
	if strings.TrimSpace(inputToken) != "" {
		return strings.TrimSpace(inputToken), nil
	}
	if strings.TrimSpace(instance.SyncTokenEncrypted) == "" {
		var encrypted string
		if err := s.infra.DB().QueryRowContext(ctx, `SELECT COALESCE(sync_access_token_encrypted,'') FROM local_newapi_instances WHERE id=?`, instance.ID).Scan(&encrypted); err != nil {
			return "", err
		}
		instance.SyncTokenEncrypted = encrypted
	}
	return s.infra.DecryptText(instance.SyncTokenEncrypted)
}

// UpdateLocalNewAPISyncToken saves, clears, or no-ops the sync access token
// for a local NewAPI instance.
func (s *Service) UpdateLocalNewAPISyncToken(ctx context.Context, instanceID string, token string, save bool, clear bool) error {
	if clear {
		_, err := s.infra.DB().ExecContext(ctx, `UPDATE local_newapi_instances SET sync_access_token_encrypted='', sync_access_token_masked='', updated_at=? WHERE id=?`, s.infra.Now(), instanceID)
		return err
	}
	if !save || strings.TrimSpace(token) == "" {
		return nil
	}
	encrypted, err := s.infra.EncryptText(strings.TrimSpace(token))
	if err != nil {
		return err
	}
	_, err = s.infra.DB().ExecContext(ctx, `UPDATE local_newapi_instances SET sync_access_token_encrypted=?, sync_access_token_masked=?, updated_at=? WHERE id=?`, encrypted, maskSecret(strings.TrimSpace(token)), s.infra.Now(), instanceID)
	return err
}

// BaseURLForAutoDetectedDB returns the base URL for an auto-detected DB path,
// preferring a previously-stored base_url, falling back to path heuristics.
func (s *Service) BaseURLForAutoDetectedDB(ctx context.Context, dbPath string) string {
	cleanPath, err := filepath.Abs(dbPath)
	if err != nil {
		cleanPath = dbPath
	}
	var baseURL string
	err = s.infra.DB().QueryRowContext(ctx, `
		SELECT base_url
		FROM local_newapi_instances
		WHERE database_path=?
		ORDER BY updated_at DESC
		LIMIT 1
	`, cleanPath).Scan(&baseURL)
	if err == nil && strings.TrimSpace(baseURL) != "" {
		return baseURL
	}
	return baseURLFromDBPath(cleanPath)
}
