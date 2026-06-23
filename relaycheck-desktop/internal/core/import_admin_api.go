package core

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func (a *App) handleImportFromAdminAPI(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var input struct {
		BaseURL           string `json:"baseUrl"`
		AccessToken       string `json:"accessToken"`
		SaveAccessToken   bool   `json:"saveAccessToken"`
		UserID            string `json:"userId"`
		InstanceName      string `json:"instanceName"`
		ImportKeys        bool   `json:"importKeys"`
		SkipCreateSites   bool   `json:"skipCreateSites"`
		DetectAfterImport bool   `json:"detectAfterImport"`
		PageSize          int    `json:"pageSize"`
	}
	if err := decodeJSON(r, &input); err != nil || strings.TrimSpace(input.BaseURL) == "" || strings.TrimSpace(input.AccessToken) == "" {
		writeError(w, http.StatusBadRequest, "NewAPI 地址和访问令牌不能为空。")
		return
	}
	if strings.TrimSpace(input.UserID) == "" {
		input.UserID = "1"
	}
	input.PageSize = clampInt(input.PageSize, 10, 100, 100)
	result, err := a.importChannelsFromAdminAPI(r.Context(), input.BaseURL, input.AccessToken, input.UserID, input.InstanceName, input.ImportKeys, !input.SkipCreateSites, input.DetectAfterImport, input.PageSize)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if input.SaveAccessToken {
		if instanceID, ok := result["instanceId"].(string); ok {
			if err := a.updateLocalNewAPISyncToken(r.Context(), instanceID, input.AccessToken, true, false); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			result["syncTokenSaved"] = true
		}
	}
	a.audit("import.admin_api", "info", "", "local_newapi_instance", stringFromResult(result, "instanceId"), "NewAPI 后台导入完成。", map[string]interface{}{
		"importedCount":  intFromResult(result, "importedCount"),
		"sitesCreated":   intFromResult(result, "sitesCreated"),
		"sitesMerged":    intFromResult(result, "sitesMerged"),
		"detectedCount":  intFromResult(result, "detectedCount"),
		"importKeys":     input.ImportKeys,
		"syncTokenSaved": input.SaveAccessToken,
	})
	writeJSON(w, http.StatusOK, result)
}

func (a *App) importChannelsFromAdminAPI(ctx context.Context, rawBaseURL string, accessToken string, userID string, instanceName string, importKeys bool, createSites bool, detectAfterImport bool, pageSize int) (map[string]interface{}, error) {
	return a.importChannelsFromAdminAPIWithOptions(ctx, rawBaseURL, accessToken, userID, instanceName, importKeys, createSites, detectAfterImport, pageSize, true)
}

func (a *App) importChannelsFromAdminAPIWithOptions(ctx context.Context, rawBaseURL string, accessToken string, userID string, instanceName string, importKeys bool, createSites bool, detectAfterImport bool, pageSize int, notify bool) (map[string]interface{}, error) {
	baseURL := normalizeBaseURL(rawBaseURL)
	if instanceName == "" {
		instanceName = "NewAPI 后台 " + hostLabel(baseURL)
	}

	instanceID := newID()
	_, err := a.db.ExecContext(ctx, `
		INSERT INTO local_newapi_instances (id, name, base_url, detected_from, status, last_scanned_at, created_at, updated_at)
		VALUES (?, ?, ?, 'admin_api_import', 'healthy', ?, ?, ?)
		ON CONFLICT(base_url) DO UPDATE SET name=excluded.name, status=excluded.status, last_scanned_at=excluded.last_scanned_at, updated_at=excluded.updated_at
	`, instanceID, instanceName, baseURL, now(), now(), now())
	if err != nil {
		return nil, err
	}
	if err := a.db.QueryRowContext(ctx, `SELECT id FROM local_newapi_instances WHERE base_url=?`, baseURL).Scan(&instanceID); err != nil {
		return nil, err
	}

	imported := 0
	sitesCreated := 0
	sitesMerged := 0
	detected := 0
	for page := 0; page < 200; page++ {
		items, err := a.fetchAdminAPIChannels(ctx, baseURL, accessToken, userID, page, pageSize)
		if err != nil {
			return nil, err
		}
		if len(items) == 0 {
			break
		}
		for _, record := range items {
			channelID, created, merged, didDetect, err := a.importChannelRecord(ctx, instanceID, record, importKeys, createSites, detectAfterImport)
			if err != nil {
				return nil, err
			}
			if channelID == "" {
				continue
			}
			imported++
			if created {
				sitesCreated++
			}
			if merged {
				sitesMerged++
			}
			if didDetect {
				detected++
			}
		}
		if len(items) < pageSize {
			break
		}
	}
	if notify {
		a.notify("channels_imported", "success", "NewAPI 后台导入完成", fmt.Sprintf("从 %s 导入 %d 条渠道，生成 %d 个站点，合并 %d 个站点。", baseURL, imported, sitesCreated, sitesMerged), "local_newapi_instance", instanceID)
	}
	return map[string]interface{}{
		"instanceId":    instanceID,
		"importedCount": imported,
		"sitesCreated":  sitesCreated,
		"sitesMerged":   sitesMerged,
		"detectedCount": detected,
	}, nil
}

func (a *App) fetchAdminAPIChannels(ctx context.Context, baseURL string, accessToken string, userID string, page int, pageSize int) ([]map[string]interface{}, error) {
	endpoint, err := url.Parse(strings.TrimRight(baseURL, "/") + "/api/channel/")
	if err != nil {
		return nil, err
	}
	query := endpoint.Query()
	query.Set("p", fmt.Sprint(page))
	query.Set("page", fmt.Sprint(page+1))
	query.Set("page_size", fmt.Sprint(pageSize))
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("authorization", "Bearer "+accessToken)
	req.Header.Set("New-Api-User", userID)
	req.Header.Set("accept", "application/json")
	resp, err := a.doHTTP(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("读取 NewAPI 渠道失败：HTTP %d，%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	data, _ := payload["data"].(map[string]interface{})
	rawItems, ok := data["items"].([]interface{})
	if !ok {
		if direct, ok := payload["data"].([]interface{}); ok {
			rawItems = direct
		}
	}
	items := []map[string]interface{}{}
	for _, raw := range rawItems {
		if item, ok := raw.(map[string]interface{}); ok {
			items = append(items, item)
		}
	}
	return items, nil
}

func (a *App) importChannelRecord(ctx context.Context, instanceID string, record map[string]interface{}, importKeys bool, createSites bool, detectAfterImport bool) (string, bool, bool, bool, error) {
	sourceID := stringValue(record, "id")
	if sourceID == "" {
		sourceID = newID()
	}
	name := stringValue(record, "name")
	if name == "" {
		name = "渠道 " + sourceID
	}
	channelBaseURL := extractImportedBaseURL(record)
	if isExcludedRelaySite(name, channelBaseURL) {
		return "", false, false, false, nil
	}
	status := stringValue(record, "status")
	keyValue := stringValue(record, "key")
	keyEncrypted := ""
	keyMasked := ""
	if importKeys && keyValue != "" {
		var err error
		keyEncrypted, err = a.encryptText(keyValue)
		if err != nil {
			return "", false, false, false, err
		}
		keyMasked = maskSecret(keyValue)
	}
	rawJSON, _ := marshalImportedRecord(record, "admin_api")
	kind := inferImportedKind(record, channelBaseURL)

	channelID := newID()
	_, err := a.db.ExecContext(ctx, `
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
		return "", false, false, false, err
	}
	if err := a.db.QueryRowContext(ctx, `
		SELECT id FROM imported_channels
		WHERE local_instance_id=? AND source_channel_id=?
	`, instanceID, sourceID).Scan(&channelID); err != nil {
		return "", false, false, false, err
	}

	created := false
	merged := false
	didDetect := false
	if createSites && channelBaseURL != "" {
		var detection *UpstreamDetection
		if detectAfterImport {
			nextDetection := a.detectUpstream(ctx, channelBaseURL)
			detection = &nextDetection
			didDetect = true
			_, err = a.db.ExecContext(ctx, `
				UPDATE imported_channels
				SET base_url=?, upstream_kind=?, supports_checkin=?, supports_balance=?, supports_models=?, supports_pricing=?, detection_json=?, last_detected_at=?, updated_at=?
				WHERE id=?
			`, nextDetection.BaseURL, nextDetection.Kind, boolInt(nextDetection.SupportsCheckin), boolInt(nextDetection.SupportsBalance), boolInt(nextDetection.SupportsModels), boolInt(nextDetection.SupportsPricing), marshalDetection(&nextDetection), now(), now(), channelID)
			if err != nil {
				return "", false, false, false, err
			}
		}
		_, wasCreated, err := a.ensureUpstreamSiteForChannel(ctx, channelID, name, channelBaseURL, kind, detection)
		if err != nil {
			return "", false, false, false, err
		}
		created = wasCreated
		merged = !wasCreated
	}
	return channelID, created, merged, didDetect, nil
}
