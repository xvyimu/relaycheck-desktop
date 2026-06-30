package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"
)

// LoadChannelModelSyncRecords returns up to limit channels that are eligible
// for model sync (non-archived, managed-relay kinds). Mirrors the original
// core.loadChannelModelSyncRecords.
func (s *Service) LoadChannelModelSyncRecords(ctx context.Context, limit int) ([]ChannelModelSyncRecord, error) {
	rows, err := s.infra.DB().QueryContext(ctx, `
		SELECT id, name, COALESCE(base_url,''), upstream_kind, COALESCE(raw_json,''),
		       COALESCE(channel_key_encrypted,''), COALESCE(model_count,0),
		       COALESCE(sample_models_json,''), COALESCE(models_source,''), COALESCE(models_status,''),
		       COALESCE(models_last_synced_at,''), COALESCE(models_message,'')
		FROM imported_channels
		WHERE COALESCE(source_sync_status,'active') <> 'archived'
		  AND upstream_kind IN ('newapi','oneapi','sub2api','modified_relay')
		ORDER BY CASE WHEN COALESCE(models_last_synced_at,'')='' THEN 0 ELSE 1 END,
		         models_last_synced_at ASC,
		         updated_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := []ChannelModelSyncRecord{}
	for rows.Next() {
		var record ChannelModelSyncRecord
		if err := rows.Scan(&record.ID, &record.Name, &record.BaseURL, &record.Kind, &record.RawJSON, &record.ChannelKeyEncrypted, &record.ModelCount, &record.SampleModelsJSON, &record.ModelsSource, &record.ModelsStatus, &record.ModelsLastSyncedAt, &record.ModelsMessage); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

// LoadChannelModelItems returns up to 500 channel model sync items used by
// the model overview endpoint. Mirrors the original core.loadChannelModelItems.
func (s *Service) LoadChannelModelItems(ctx context.Context) ([]ChannelModelSyncItem, error) {
	rows, err := s.infra.DB().QueryContext(ctx, `
		SELECT id, name, COALESCE(base_url,''), upstream_kind, COALESCE(channel_key_encrypted,''),
		       COALESCE(model_count,0), COALESCE(sample_models_json,''), COALESCE(models_source,''),
		       COALESCE(models_status,''), COALESCE(models_last_synced_at,''), COALESCE(models_message,'')
		FROM imported_channels
		WHERE COALESCE(source_sync_status,'active') <> 'archived'
		  AND upstream_kind IN ('newapi','oneapi','sub2api','modified_relay')
		ORDER BY model_count DESC, updated_at DESC
		LIMIT 500
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []ChannelModelSyncItem{}
	for rows.Next() {
		var item ChannelModelSyncItem
		var sampleJSON, keyEncrypted string
		if err := rows.Scan(&item.ChannelID, &item.ChannelName, &item.BaseURL, &item.Kind, &keyEncrypted, &item.ModelCount, &sampleJSON, &item.Source, &item.Status, &item.LastSyncedAt, &item.Message); err != nil {
			return nil, err
		}
		item.HasKey = keyEncrypted != ""
		item.SampleModels = parsePersistedStringSliceLimit(sampleJSON, 40)
		if item.Status == "" {
			item.Status = "unchecked"
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// SyncChannelModels re-extracts models for one channel, optionally probing
// /v1/models with the channel's decrypted key, and persists the result.
// Mirrors the original core.syncChannelModels.
func (s *Service) SyncChannelModels(ctx context.Context, record ChannelModelSyncRecord) ChannelModelSyncItem {
	item := ChannelModelSyncItem{
		ChannelID:    record.ID,
		ChannelName:  record.Name,
		BaseURL:      record.BaseURL,
		Kind:         record.Kind,
		HasKey:       record.ChannelKeyEncrypted != "",
		LastSyncedAt: s.infra.Now(),
	}
	rawModels := modelsFromRawChannelJSON(record.RawJSON)
	models := rawModels
	item.Source = "raw_json"
	item.Status = "raw_only"
	item.Message = fmt.Sprintf("从 channels 原始配置识别到 %d 个模型。", len(rawModels))

	key, _ := s.infra.DecryptText(record.ChannelKeyEncrypted)
	if key != "" && normalizeBaseURL(record.BaseURL) != "" {
		liveModels, latencyMs, httpStatus, message := s.fetchChannelModelsWithKey(ctx, record.BaseURL, key)
		item.LatencyMs = latencyMs
		if len(liveModels) > 0 {
			models = liveModels
			item.Source = "channel_key_api"
			item.Status = "live_key"
			item.Message = fmt.Sprintf("/v1/models 返回 HTTP %d，实时识别 %d 个模型。", httpStatus, len(liveModels))
		} else if httpStatus == http.StatusUnauthorized || httpStatus == http.StatusForbidden {
			item.Status = "key_invalid"
			item.Message = firstNonEmpty(message, "渠道 Key 无权访问 /v1/models。")
		} else if len(rawModels) == 0 {
			item.Status = "failed"
			item.Source = "channel_key_api"
			item.Message = firstNonEmpty(message, "未能从实时接口或原始配置识别模型。")
		} else if message != "" {
			item.Message += " 实时接口未采用：" + message
		}
	}

	models = normalizeModelIDs(models)
	item.ModelCount = len(models)
	item.SampleModels = limitStrings(models, 40)
	if item.ModelCount == 0 && item.Status != "key_invalid" {
		item.Status = "empty"
		item.Message = "没有在 raw_json、model_mapping 或 /v1/models 中识别到模型。"
	}
	if item.Status == "key_invalid" {
		item.SampleModels = limitStrings(rawModels, 40)
		item.ModelCount = len(item.SampleModels)
	}

	maskedMessage := maskResponse(item.Message)
	if _, execErr := s.infra.DB().ExecContext(ctx, `
		UPDATE imported_channels
		SET model_count=?, sample_models_json=?, models_source=?, models_status=?,
		    models_last_synced_at=?, models_message=?, supports_models=CASE WHEN ? > 0 THEN 1 ELSE supports_models END,
		    updated_at=?
		WHERE id=?
	`, item.ModelCount, marshalStringSliceLimit(item.SampleModels, 40), item.Source, item.Status, item.LastSyncedAt, maskedMessage, item.ModelCount, s.infra.Now(), record.ID); execErr != nil {
		log.Printf("[channel_models] model sync update failed for channel %s: %v", record.ID, execErr)
	}
	item.Message = maskedMessage
	return item
}

// fetchChannelModelsWithKey probes baseURL/v1/models with the given API key.
// Returns (models, latencyMs, httpStatus, message). Mirrors the original
// core.fetchChannelModelsWithKey.
func (s *Service) fetchChannelModelsWithKey(ctx context.Context, baseURL string, apiKey string) ([]string, int64, int, string) {
	requestCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	safeBaseURL, err := s.infra.SafeNormalizeBaseURL(requestCtx, baseURL)
	if err != nil {
		return nil, 0, 0, err.Error()
	}
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, strings.TrimRight(safeBaseURL, "/")+"/v1/models", nil)
	if err != nil {
		return nil, 0, 0, err.Error()
	}
	req.Header.Set("authorization", "Bearer "+strings.TrimSpace(apiKey))
	req.Header.Set("accept", "application/json, text/plain, */*")
	req.Header.Set("user-agent", "RelayCheck-Desktop/0.1")
	started := time.Now()
	resp, err := s.infra.DoHTTPWithTimeout(req, 13*time.Second)
	latencyMs := time.Since(started).Milliseconds()
	if err != nil {
		return nil, latencyMs, 0, err.Error()
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	message := firstNonEmpty(extractMessage(string(body)), fmt.Sprintf("/v1/models 返回 HTTP %d。", resp.StatusCode))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, latencyMs, resp.StatusCode, message
	}
	return parseModelIDs(string(body)), latencyMs, resp.StatusCode, message
}

// BuildChannelModelOverview aggregates per-channel model sync outcomes.
// Mirrors the original core.buildChannelModelOverview.
func BuildChannelModelOverview(items []ChannelModelSyncItem, synced int, generatedAt string) ChannelModelSyncOverview {
	overview := ChannelModelSyncOverview{
		GeneratedAt:    generatedAt,
		SyncedChannels: synced,
		Items:          []ChannelModelSyncItem{},
		Models:         []ChannelModelCoverageItem{},
	}
	models := map[string]*ChannelModelCoverageItem{}
	for _, item := range items {
		overview.ChannelCount++
		switch item.Status {
		case "live_key":
			overview.LiveKeyCount++
		case "raw_only":
			overview.RawOnlyCount++
		case "failed", "key_invalid":
			overview.FailedCount++
		case "unchecked", "":
			overview.UncheckedCount++
		}
		for _, model := range item.SampleModels {
			key := strings.ToLower(model)
			entry := models[key]
			if entry == nil {
				entry = &ChannelModelCoverageItem{Model: model, Channels: []string{}}
				models[key] = entry
			}
			entry.ChannelCount++
			if item.Status == "live_key" {
				entry.LiveKeyCount++
			}
			appendUniqueString(&entry.Channels, item.ChannelName, 6)
		}
		overview.Items = append(overview.Items, item)
	}
	for _, item := range models {
		overview.Models = append(overview.Models, *item)
	}
	overview.ModelCount = len(overview.Models)
	sort.SliceStable(overview.Items, func(i, j int) bool {
		left := overview.Items[i]
		right := overview.Items[j]
		if left.ModelCount != right.ModelCount {
			return left.ModelCount > right.ModelCount
		}
		return left.ChannelName < right.ChannelName
	})
	sort.SliceStable(overview.Models, func(i, j int) bool {
		left := overview.Models[i]
		right := overview.Models[j]
		if left.ChannelCount != right.ChannelCount {
			return left.ChannelCount > right.ChannelCount
		}
		return left.Model < right.Model
	})
	overview.Items = limitChannelModelItems(overview.Items, 80)
	overview.Models = limitChannelModelCoverageItems(overview.Models, 80)
	return overview
}

// modelsFromRawChannelJSON mirrors core.modelsFromRawChannelJSON.
func modelsFromRawChannelJSON(raw string) []string {
	root, ok := parseJSONLike(raw)
	if !ok {
		return nil
	}
	expanded := expandJSONStrings(root, 0)
	models := extractModelsFromJSON(expanded)
	models = append(models, extractModelMappingKeys(expanded)...)
	return normalizeModelIDs(models)
}

// normalizeModelIDs mirrors core.normalizeModelIDs.
func normalizeModelIDs(values []string) []string {
	seen := map[string]bool{}
	models := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if !looksLikeModelID(value) {
			continue
		}
		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		models = append(models, value)
	}
	return models
}

// extractModelsFromJSON mirrors core.extractModelsFromJSON. Walks value
// looking for keys named "models"/"model" and collects their model IDs.
func extractModelsFromJSON(value interface{}) []string {
	seen := map[string]bool{}
	models := []string{}
	var walk func(interface{}, string)
	walk = func(current interface{}, parentKey string) {
		current = expandJSONStrings(current, 0)
		switch typed := current.(type) {
		case map[string]interface{}:
			for key, child := range typed {
				lowerKey := strings.ToLower(key)
				if lowerKey == "models" || lowerKey == "model" {
					for _, model := range modelsFromAny(child) {
						if !seen[strings.ToLower(model)] {
							seen[strings.ToLower(model)] = true
							models = append(models, model)
						}
					}
				}
				walk(child, lowerKey)
			}
		case []interface{}:
			for _, child := range typed {
				walk(child, parentKey)
			}
		case string:
			if parentKey == "models" || parentKey == "model" {
				for _, model := range splitModelList(typed) {
					if !seen[strings.ToLower(model)] {
						seen[strings.ToLower(model)] = true
						models = append(models, model)
					}
				}
			}
		}
	}
	walk(value, "")
	return models
}

// modelsFromAny mirrors core.modelsFromAny. Extracts model IDs from any
// JSON-shaped value.
func modelsFromAny(value interface{}) []string {
	value = expandJSONStrings(value, 0)
	switch typed := value.(type) {
	case string:
		return splitModelList(typed)
	case []interface{}:
		models := []string{}
		for _, child := range typed {
			models = append(models, modelsFromAny(child)...)
		}
		return models
	case map[string]interface{}:
		if model := firstNonEmpty(stringFromAny(typed["id"]), stringFromAny(typed["model"]), stringFromAny(typed["name"])); looksLikeModelID(model) {
			return []string{model}
		}
	}
	return nil
}

// splitModelList mirrors core.splitModelList. Splits a comma/newline/tab/
// space-separated model list, keeping only values that look like model IDs.
func splitModelList(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
	models := []string{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if looksLikeModelID(part) {
			models = append(models, part)
		}
	}
	return models
}

// extractModelMappingKeys mirrors core.extractModelMappingKeys.
func extractModelMappingKeys(value interface{}) []string {
	models := []string{}
	var walk func(interface{})
	walk = func(current interface{}) {
		current = expandJSONStrings(current, 0)
		switch typed := current.(type) {
		case map[string]interface{}:
			for key, child := range typed {
				if isModelMappingKey(strings.ToLower(key)) {
					if mapping, ok := expandJSONStrings(child, 0).(map[string]interface{}); ok {
						for model := range mapping {
							models = append(models, model)
						}
					}
					continue
				}
				walk(child)
			}
		case []interface{}:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(value)
	return models
}

// marshalStringSliceLimit mirrors core.marshalStringSliceLimit.
func marshalStringSliceLimit(values []string, limit int) string {
	if len(values) == 0 {
		return ""
	}
	if len(values) > limit {
		values = values[:limit]
	}
	body, err := json.Marshal(values)
	if err != nil {
		return ""
	}
	return string(body)
}

// parsePersistedStringSliceLimit mirrors core.parsePersistedStringSliceLimit.
func parsePersistedStringSliceLimit(raw string, limit int) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil
	}
	return limitStrings(values, limit)
}

func limitChannelModelItems(values []ChannelModelSyncItem, limit int) []ChannelModelSyncItem {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}

func limitChannelModelCoverageItems(values []ChannelModelCoverageItem, limit int) []ChannelModelCoverageItem {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}
