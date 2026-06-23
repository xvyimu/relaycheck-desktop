package core

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
)

type localNewAPISyncRunInput struct {
	AccessToken       string `json:"accessToken"`
	SaveAccessToken   bool   `json:"saveAccessToken"`
	ClearAccessToken  bool   `json:"clearAccessToken"`
	UserID            string `json:"userId"`
	ImportKeys        bool   `json:"importKeys"`
	SkipCreateSites   bool   `json:"skipCreateSites"`
	DetectAfterImport bool   `json:"detectAfterImport"`
	PageSize          int    `json:"pageSize"`
}

func (a *App) handleLocalNewAPIInstances(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.listLocalNewAPIInstances(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (a *App) listLocalNewAPIInstances(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.QueryContext(r.Context(), `
		SELECT i.id, i.name, i.base_url, COALESCE(i.detected_from,''), i.status,
		       COALESCE(i.version,''), COALESCE(i.database_path,''), COALESCE(i.last_scanned_at,''),
		       COALESCE(i.sync_access_token_masked,''), i.created_at, i.updated_at,
		       (SELECT COUNT(*) FROM imported_channels c WHERE c.local_instance_id = i.id)
		FROM local_newapi_instances i
		ORDER BY i.updated_at DESC
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	items := []LocalNewAPIInstance{}
	for rows.Next() {
		var item LocalNewAPIInstance
		if err := rows.Scan(&item.ID, &item.Name, &item.BaseURL, &item.DetectedFrom, &item.Status, &item.Version, &item.DatabasePath, &item.LastScannedAt, &item.SyncTokenMasked, &item.CreatedAt, &item.UpdatedAt, &item.ChannelCount); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		item.HasSyncToken = strings.TrimSpace(item.SyncTokenMasked) != ""
		item.SyncCapability = syncCapability(item)
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, items)
}

func (a *App) handleLocalNewAPIInstanceByID(w http.ResponseWriter, r *http.Request) {
	tail := pathTail(r.URL.Path, "/api/local-newapi/")
	if strings.HasSuffix(tail, "/sync-preview") {
		id := strings.TrimSuffix(tail, "/sync-preview")
		a.previewLocalNewAPIInstanceSync(w, r, id)
		return
	}
	if strings.HasSuffix(tail, "/mark-missing") {
		id := strings.TrimSuffix(tail, "/mark-missing")
		a.markMissingLocalNewAPIInstance(w, r, id)
		return
	}
	if strings.HasSuffix(tail, "/sync") {
		id := strings.TrimSuffix(tail, "/sync")
		a.syncLocalNewAPIInstance(w, r, id)
		return
	}
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func (a *App) syncLocalNewAPIInstance(w http.ResponseWriter, r *http.Request, id string) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var input localNewAPISyncRunInput
	_ = decodeJSON(r, &input)
	result, err := a.syncLocalNewAPIInstanceData(r.Context(), id, input, true)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *App) syncLocalNewAPIInstanceData(ctx context.Context, id string, input localNewAPISyncRunInput, notify bool) (map[string]interface{}, error) {
	if strings.TrimSpace(input.UserID) == "" {
		input.UserID = "1"
	}
	input.PageSize = clampInt(input.PageSize, 10, 100, 100)

	instance, err := a.getLocalNewAPIInstance(ctx, id)
	if err == sql.ErrNoRows {
		return nil, errorsText("NewAPI 实例不存在。")
	}
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if strings.TrimSpace(instance.DatabasePath) != "" {
		result, err = a.importChannelsFromSQLiteWithOptions(ctx, instance.DatabasePath, input.ImportKeys, instance.Name, instance.BaseURL, !input.SkipCreateSites, input.DetectAfterImport, notify)
	} else if isHTTPURL(instance.BaseURL) {
		accessToken, err := a.resolveLocalNewAPISyncToken(ctx, instance, input.AccessToken)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(accessToken) == "" {
			return nil, errorsText("该实例需要填写系统访问令牌后才能通过后台 API 同步。")
		}
		result, err = a.importChannelsFromAdminAPIWithOptions(ctx, instance.BaseURL, accessToken, input.UserID, instance.Name, input.ImportKeys, !input.SkipCreateSites, input.DetectAfterImport, input.PageSize, notify)
		if err == nil {
			err = a.updateLocalNewAPISyncToken(ctx, instance.ID, input.AccessToken, input.SaveAccessToken, input.ClearAccessToken)
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

func (a *App) getLocalNewAPIInstance(ctx context.Context, id string) (LocalNewAPIInstance, error) {
	var instance LocalNewAPIInstance
	err := a.db.QueryRowContext(ctx, `
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

func (a *App) resolveLocalNewAPISyncToken(ctx context.Context, instance LocalNewAPIInstance, inputToken string) (string, error) {
	if strings.TrimSpace(inputToken) != "" {
		return strings.TrimSpace(inputToken), nil
	}
	if strings.TrimSpace(instance.SyncTokenEncrypted) == "" {
		var encrypted string
		if err := a.db.QueryRowContext(ctx, `SELECT COALESCE(sync_access_token_encrypted,'') FROM local_newapi_instances WHERE id=?`, instance.ID).Scan(&encrypted); err != nil {
			return "", err
		}
		instance.SyncTokenEncrypted = encrypted
	}
	return a.decryptText(instance.SyncTokenEncrypted)
}

func (a *App) updateLocalNewAPISyncToken(ctx context.Context, instanceID string, token string, save bool, clear bool) error {
	if clear {
		_, err := a.db.ExecContext(ctx, `UPDATE local_newapi_instances SET sync_access_token_encrypted='', sync_access_token_masked='', updated_at=? WHERE id=?`, now(), instanceID)
		return err
	}
	if !save || strings.TrimSpace(token) == "" {
		return nil
	}
	encrypted, err := a.encryptText(strings.TrimSpace(token))
	if err != nil {
		return err
	}
	_, err = a.db.ExecContext(ctx, `UPDATE local_newapi_instances SET sync_access_token_encrypted=?, sync_access_token_masked=?, updated_at=? WHERE id=?`, encrypted, maskSecret(strings.TrimSpace(token)), now(), instanceID)
	return err
}

func syncCapability(item LocalNewAPIInstance) string {
	if strings.TrimSpace(item.DatabasePath) != "" {
		return "sqlite"
	}
	if isHTTPURL(item.BaseURL) {
		if item.HasSyncToken {
			return "admin_api_saved_token"
		}
		return "admin_api_token_required"
	}
	return "unsupported"
}

func isHTTPURL(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(normalized, "http://") || strings.HasPrefix(normalized, "https://")
}
