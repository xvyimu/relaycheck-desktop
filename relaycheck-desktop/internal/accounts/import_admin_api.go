package accounts

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// ImportChannelsFromAdminAPI imports channels from a NewAPI admin API endpoint.
func (s *Service) ImportChannelsFromAdminAPI(ctx context.Context, rawBaseURL string, accessToken string, userID string, instanceName string, importKeys bool, createSites bool, detectAfterImport bool, pageSize int) (map[string]interface{}, error) {
	return s.ImportChannelsFromAdminAPIWithOptions(ctx, rawBaseURL, accessToken, userID, instanceName, importKeys, createSites, detectAfterImport, pageSize, true)
}

// ImportChannelsFromAdminAPIWithOptions imports channels from a NewAPI admin
// API endpoint, with a flag to suppress notifications (used by scheduled sync).
func (s *Service) ImportChannelsFromAdminAPIWithOptions(ctx context.Context, rawBaseURL string, accessToken string, userID string, instanceName string, importKeys bool, createSites bool, detectAfterImport bool, pageSize int, notify bool) (map[string]interface{}, error) {
	baseURL := normalizeBaseURL(rawBaseURL)
	if instanceName == "" {
		instanceName = "NewAPI 后台 " + hostLabel(baseURL)
	}

	instanceID := s.infra.NewID()
	_, err := s.infra.DB().ExecContext(ctx, `
		INSERT INTO local_newapi_instances (id, name, base_url, detected_from, status, last_scanned_at, created_at, updated_at)
		VALUES (?, ?, ?, 'admin_api_import', 'healthy', ?, ?, ?)
		ON CONFLICT(base_url) DO UPDATE SET name=excluded.name, status=excluded.status, last_scanned_at=excluded.last_scanned_at, updated_at=excluded.updated_at
	`, instanceID, instanceName, baseURL, s.infra.Now(), s.infra.Now(), s.infra.Now())
	if err != nil {
		return nil, err
	}
	if err := s.infra.DB().QueryRowContext(ctx, `SELECT id FROM local_newapi_instances WHERE base_url=?`, baseURL).Scan(&instanceID); err != nil {
		return nil, err
	}

	imported := 0
	sitesCreated := 0
	sitesMerged := 0
	detected := 0
	for page := 0; page < 200; page++ {
		items, err := s.fetchAdminAPIChannels(ctx, baseURL, accessToken, userID, page, pageSize)
		if err != nil {
			return nil, err
		}
		if len(items) == 0 {
			break
		}
		for _, record := range items {
			channelID, created, merged, didDetect, err := s.importChannelRecord(ctx, instanceID, record, importKeys, createSites, detectAfterImport)
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
		s.infra.Notify("channels_imported", "success", "NewAPI 后台导入完成", fmt.Sprintf("从 %s 导入 %d 条渠道，生成 %d 个站点，合并 %d 个站点。", baseURL, imported, sitesCreated, sitesMerged), "local_newapi_instance", instanceID)
	}
	return map[string]interface{}{
		"instanceId":    instanceID,
		"importedCount": imported,
		"sitesCreated":  sitesCreated,
		"sitesMerged":   sitesMerged,
		"detectedCount": detected,
	}, nil
}

// fetchAdminAPIChannels fetches one page of channels from the NewAPI admin API.
func (s *Service) fetchAdminAPIChannels(ctx context.Context, baseURL string, accessToken string, userID string, page int, pageSize int) ([]map[string]interface{}, error) {
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
	resp, err := s.infra.DoHTTP(req)
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

// importChannelRecord inserts/updates a single imported_channels row from an
// admin API record, optionally creating an upstream_site.
func (s *Service) importChannelRecord(ctx context.Context, instanceID string, record map[string]interface{}, importKeys bool, createSites bool, detectAfterImport bool) (string, bool, bool, bool, error) {
	sourceID := stringValue(record, "id")
	if sourceID == "" {
		sourceID = s.infra.NewID()
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
		keyEncrypted, err = s.infra.EncryptText(keyValue)
		if err != nil {
			return "", false, false, false, err
		}
		keyMasked = maskSecret(keyValue)
	}
	rawJSON, _ := marshalImportedRecord(record, "admin_api")
	kind := inferImportedKind(record, channelBaseURL)

	channelID := s.infra.NewID()
	_, err := s.infra.DB().ExecContext(ctx, `
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
		return "", false, false, false, err
	}
	if err := s.infra.DB().QueryRowContext(ctx, `
		SELECT id FROM imported_channels
		WHERE local_instance_id=? AND source_channel_id=?
	`, instanceID, sourceID).Scan(&channelID); err != nil {
		return "", false, false, false, err
	}

	created := false
	merged := false
	didDetect := false
	if createSites && channelBaseURL != "" {
		var detection *Detection
		if detectAfterImport {
			nextDetection, detectErr := s.infra.DetectUpstreamForImport(ctx, channelBaseURL)
			if detectErr != nil {
				return "", false, false, false, detectErr
			}
			detection = &nextDetection
			didDetect = true
			_, err = s.infra.DB().ExecContext(ctx, `
				UPDATE imported_channels
				SET base_url=?, upstream_kind=?, supports_checkin=?, supports_balance=?, supports_models=?, supports_pricing=?, detection_json=?, last_detected_at=?, updated_at=?
				WHERE id=?
			`, nextDetection.BaseURL, nextDetection.Kind, boolInt(nextDetection.SupportsCheckin), boolInt(nextDetection.SupportsBalance), boolInt(nextDetection.SupportsModels), boolInt(nextDetection.SupportsPricing), mustJSON(nextDetection), s.infra.Now(), s.infra.Now(), channelID)
			if err != nil {
				return "", false, false, false, err
			}
		}
		_, wasCreated, err := s.infra.EnsureChannelSiteForImport(ctx, channelID, name, channelBaseURL, kind, detection)
		if err != nil {
			return "", false, false, false, err
		}
		created = wasCreated
		merged = !wasCreated
	}
	return channelID, created, merged, didDetect, nil
}

// fetchAllAdminAPIChannelRecords paginates through all admin API channel
// records. Used by the sync-preview flow.
func (s *Service) fetchAllAdminAPIChannelRecords(ctx context.Context, baseURL string, accessToken string, userID string, pageSize int) ([]map[string]interface{}, error) {
	records := []map[string]interface{}{}
	for page := 0; page < 200; page++ {
		items, err := s.fetchAdminAPIChannels(ctx, baseURL, accessToken, userID, page, pageSize)
		if err != nil {
			return nil, err
		}
		if len(items) == 0 {
			break
		}
		records = append(records, items...)
		if len(items) < pageSize {
			break
		}
	}
	return records, nil
}
