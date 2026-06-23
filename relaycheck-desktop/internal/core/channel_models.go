package core

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

type channelModelSyncOverview struct {
	GeneratedAt    string                     `json:"generatedAt"`
	SyncedChannels int                        `json:"syncedChannels,omitempty"`
	ChannelCount   int                        `json:"channelCount"`
	ModelCount     int                        `json:"modelCount"`
	LiveKeyCount   int                        `json:"liveKeyCount"`
	RawOnlyCount   int                        `json:"rawOnlyCount"`
	FailedCount    int                        `json:"failedCount"`
	UncheckedCount int                        `json:"uncheckedCount"`
	Items          []channelModelSyncItem     `json:"items"`
	Models         []channelModelCoverageItem `json:"models"`
}

type channelModelSyncItem struct {
	ChannelID    string   `json:"channelId"`
	ChannelName  string   `json:"channelName"`
	BaseURL      string   `json:"baseUrl,omitempty"`
	Kind         string   `json:"kind"`
	HasKey       bool     `json:"hasKey"`
	Status       string   `json:"status"`
	Source       string   `json:"source,omitempty"`
	ModelCount   int      `json:"modelCount"`
	SampleModels []string `json:"sampleModels,omitempty"`
	LatencyMs    int64    `json:"latencyMs,omitempty"`
	Message      string   `json:"message,omitempty"`
	LastSyncedAt string   `json:"lastSyncedAt,omitempty"`
}

type channelModelCoverageItem struct {
	Model        string   `json:"model"`
	ChannelCount int      `json:"channelCount"`
	LiveKeyCount int      `json:"liveKeyCount"`
	Channels     []string `json:"channels,omitempty"`
}

type channelModelSyncRecord struct {
	ID                  string
	Name                string
	BaseURL             string
	Kind                string
	RawJSON             string
	ChannelKeyEncrypted string
	ModelCount          int
	SampleModelsJSON    string
	ModelsSource        string
	ModelsStatus        string
	ModelsLastSyncedAt  string
	ModelsMessage       string
}

func (a *App) handleChannelModelsOverview(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	items, err := a.loadChannelModelItems(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, buildChannelModelOverview(items, 0))
}

func (a *App) handleChannelModelsSync(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var input struct {
		Limit int `json:"limit"`
	}
	if r.ContentLength != 0 {
		_ = decodeJSON(r, &input)
	}
	input.Limit = clampBatchLimit(input.Limit, 10)
	records, err := a.loadChannelModelSyncRecords(r.Context(), input.Limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	items := make([]channelModelSyncItem, 0, len(records))
	for _, record := range records {
		items = append(items, a.syncChannelModels(r.Context(), record))
	}
	overview := buildChannelModelOverview(items, len(records))
	if len(records) > 0 {
		a.notify("channel_models_synced", "success", "渠道模型同步完成", fmt.Sprintf("已同步 %d 个渠道的模型覆盖。", len(records)), "channel", "")
	}
	writeJSON(w, http.StatusOK, overview)
}

func (a *App) loadChannelModelSyncRecords(ctx context.Context, limit int) ([]channelModelSyncRecord, error) {
	rows, err := a.db.QueryContext(ctx, `
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

	records := []channelModelSyncRecord{}
	for rows.Next() {
		var record channelModelSyncRecord
		if err := rows.Scan(&record.ID, &record.Name, &record.BaseURL, &record.Kind, &record.RawJSON, &record.ChannelKeyEncrypted, &record.ModelCount, &record.SampleModelsJSON, &record.ModelsSource, &record.ModelsStatus, &record.ModelsLastSyncedAt, &record.ModelsMessage); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (a *App) loadChannelModelItems(ctx context.Context) ([]channelModelSyncItem, error) {
	rows, err := a.db.QueryContext(ctx, `
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

	items := []channelModelSyncItem{}
	for rows.Next() {
		var item channelModelSyncItem
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

func (a *App) syncChannelModels(ctx context.Context, record channelModelSyncRecord) channelModelSyncItem {
	item := channelModelSyncItem{
		ChannelID:    record.ID,
		ChannelName:  record.Name,
		BaseURL:      record.BaseURL,
		Kind:         record.Kind,
		HasKey:       record.ChannelKeyEncrypted != "",
		LastSyncedAt: now(),
	}
	rawModels := modelsFromRawChannelJSON(record.RawJSON)
	models := rawModels
	item.Source = "raw_json"
	item.Status = "raw_only"
	item.Message = fmt.Sprintf("从 channels 原始配置识别到 %d 个模型。", len(rawModels))

	key, _ := a.decryptText(record.ChannelKeyEncrypted)
	if key != "" && normalizeBaseURL(record.BaseURL) != "" {
		liveModels, latencyMs, httpStatus, message := a.fetchChannelModelsWithKey(ctx, record.BaseURL, key)
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

	message := maskResponse(item.Message)
	_, _ = a.db.ExecContext(ctx, `
		UPDATE imported_channels
		SET model_count=?, sample_models_json=?, models_source=?, models_status=?,
		    models_last_synced_at=?, models_message=?, supports_models=CASE WHEN ? > 0 THEN 1 ELSE supports_models END,
		    updated_at=?
		WHERE id=?
	`, item.ModelCount, marshalStringSliceLimit(item.SampleModels, 40), item.Source, item.Status, item.LastSyncedAt, message, item.ModelCount, now(), record.ID)
	item.Message = message
	return item
}

func (a *App) fetchChannelModelsWithKey(ctx context.Context, baseURL string, apiKey string) ([]string, int64, int, string) {
	requestCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	safeBaseURL, err := safeNormalizeBaseURL(requestCtx, baseURL, a.externalURLPolicy())
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
	resp, err := a.doHTTPWithTimeout(req, 13*time.Second)
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

func buildChannelModelOverview(items []channelModelSyncItem, synced int) channelModelSyncOverview {
	overview := channelModelSyncOverview{
		GeneratedAt:    now(),
		SyncedChannels: synced,
		Items:          []channelModelSyncItem{},
		Models:         []channelModelCoverageItem{},
	}
	models := map[string]*channelModelCoverageItem{}
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
				entry = &channelModelCoverageItem{Model: model, Channels: []string{}}
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

func limitChannelModelItems(values []channelModelSyncItem, limit int) []channelModelSyncItem {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}

func limitChannelModelCoverageItems(values []channelModelCoverageItem, limit int) []channelModelCoverageItem {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}
