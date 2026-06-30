package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// LoadRawChannelPricingSources extracts pricing sources from the raw_json
// column of every non-archived imported channel. Mirrors the body of
// core.loadRawChannelPricingSources.
func (s *Service) LoadRawChannelPricingSources(ctx context.Context) ([]ModelPricingSource, error) {
	rows, err := s.infra.DB().QueryContext(ctx, `
		SELECT id, name, COALESCE(base_url,''), upstream_kind, COALESCE(raw_json,'')
		FROM imported_channels
		WHERE COALESCE(raw_json,'') <> ''
		  AND COALESCE(source_sync_status,'active') <> 'archived'
		ORDER BY updated_at DESC
		LIMIT 500
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sources := []ModelPricingSource{}
	seenSources := map[string]bool{}
	for rows.Next() {
		var channelID, channelName, baseURL, kind, rawJSON string
		if err := rows.Scan(&channelID, &channelName, &baseURL, &kind, &rawJSON); err != nil {
			return nil, err
		}
		for _, source := range extractModelPricingSources(channelID, channelName, baseURL, kind, rawJSON) {
			key := source.ChannelID + "|" + strings.ToLower(source.Model) + "|" + source.Source + "|" + source.FieldPath + "|" + source.UpstreamModel
			if seenSources[key] {
				continue
			}
			seenSources[key] = true
			sources = append(sources, source)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return sources, nil
}

// LoadPricingSiteRecords returns up to limit upstream_sites rows that look
// like NewAPI/OneAPI/Sub2API/modified_relay panels (i.e. sites that expose
// /api/pricing). Mirrors the body of core.loadPricingSiteRecords.
func (s *Service) LoadPricingSiteRecords(ctx context.Context, limit int) ([]PricingSiteRecord, error) {
	rows, err := s.infra.DB().QueryContext(ctx, `
		SELECT id, name, base_url, kind
		FROM upstream_sites
		WHERE kind IN ('newapi','oneapi','sub2api','modified_relay')
		  AND COALESCE(base_url,'') <> ''
		ORDER BY COALESCE(last_health_check_at,''), updated_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := []PricingSiteRecord{}
	for rows.Next() {
		var record PricingSiteRecord
		if err := rows.Scan(&record.SiteID, &record.SiteName, &record.BaseURL, &record.Kind); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

// SyncSitePricing probes one site's /api/pricing endpoint, extracts live
// pricing sources, and persists the result to site_pricing_cache. Mirrors
// the body of core.syncSitePricing.
func (s *Service) SyncSitePricing(ctx context.Context, record PricingSiteRecord) SitePricingCacheItem {
	item := SitePricingCacheItem{
		SiteID:       record.SiteID,
		SiteName:     record.SiteName,
		BaseURL:      record.BaseURL,
		Kind:         record.Kind,
		SourcePath:   "/api/pricing",
		Status:       "failed",
		LastSyncedAt: s.infra.Now(),
	}
	baseURL, err := s.infra.SafeNormalizeBaseURL(ctx, record.BaseURL)
	if err != nil {
		item.Message = err.Error()
		s.saveSitePricingCache(ctx, item, nil, "")
		return item
	}
	baseURL = strings.TrimRight(baseURL, "/")

	requestCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, baseURL+"/api/pricing", nil)
	if err != nil {
		item.Message = err.Error()
		s.saveSitePricingCache(ctx, item, nil, "")
		return item
	}
	req.Header.Set("accept", "application/json, text/plain, */*")
	req.Header.Set("user-agent", "RelayCheck-Desktop/0.1")
	started := time.Now()
	resp, err := s.infra.DoHTTPWithTimeout(req, 9*time.Second)
	item.LatencyMs = time.Since(started).Milliseconds()
	if err != nil {
		item.Message = err.Error()
		s.saveSitePricingCache(ctx, item, nil, "")
		return item
	}
	defer resp.Body.Close()
	item.HTTPStatus = resp.StatusCode
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 200*1024))
	body := string(bodyBytes)
	item.Message = firstNonEmpty(extractMessage(body), fmt.Sprintf("/api/pricing 返回 HTTP %d。", resp.StatusCode))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		item.Status = "failed"
		s.saveSitePricingCache(ctx, item, nil, body)
		return item
	}
	sources := ExtractLivePricingSources(record, body)
	item.SourceCount = len(sources)
	item.ModelCount = countPricingModels(sources)
	if len(sources) == 0 {
		item.Status = "empty"
		item.Message = "接口可访问，但未识别到模型价格、倍率或 quota 字段。"
	} else {
		item.Status = "success"
		item.Message = fmt.Sprintf("识别到 %d 条价格来源，覆盖 %d 个模型。", item.SourceCount, item.ModelCount)
	}
	s.saveSitePricingCache(ctx, item, sources, body)
	return item
}

// saveSitePricingCache upserts one site_pricing_cache row. Mirrors the body
// of core.saveSitePricingCache.
func (s *Service) saveSitePricingCache(ctx context.Context, item SitePricingCacheItem, sources []ModelPricingSource, rawBody string) {
	sourcesJSON := marshalPricingSourcesLimit(sources, 200)
	rawMasked := ""
	if strings.TrimSpace(rawBody) != "" {
		rawMasked = maskResponse(rawBody)
	}
	if _, execErr := s.infra.DB().ExecContext(ctx, `
		INSERT INTO site_pricing_cache (id, site_id, site_name, base_url, kind, status, http_status, latency_ms, source_path, raw_response_masked, sources_json, model_count, source_count, message, last_synced_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(site_id, source_path) DO UPDATE SET
			site_name=excluded.site_name,
			base_url=excluded.base_url,
			kind=excluded.kind,
			status=excluded.status,
			http_status=excluded.http_status,
			latency_ms=excluded.latency_ms,
			raw_response_masked=excluded.raw_response_masked,
			sources_json=excluded.sources_json,
			model_count=excluded.model_count,
			source_count=excluded.source_count,
			message=excluded.message,
			last_synced_at=excluded.last_synced_at,
			updated_at=excluded.updated_at
	`, s.infra.NewID(), item.SiteID, item.SiteName, item.BaseURL, item.Kind, item.Status, item.HTTPStatus, item.LatencyMs, item.SourcePath, rawMasked, sourcesJSON, item.ModelCount, item.SourceCount, maskResponse(item.Message), item.LastSyncedAt, s.infra.Now(), s.infra.Now()); execErr != nil {
		log.Printf("[pricing] site pricing cache save failed for site %s: %v", item.SiteID, execErr)
	}
}

// LoadSitePricingCache returns the cached /api/pricing sources and items
// from site_pricing_cache. Mirrors the body of core.loadSitePricingCache.
func (s *Service) LoadSitePricingCache(ctx context.Context) ([]ModelPricingSource, []SitePricingCacheItem, error) {
	rows, err := s.infra.DB().QueryContext(ctx, `
		SELECT site_id, site_name, base_url, kind, status, http_status, latency_ms,
		       source_path, COALESCE(sources_json,''), model_count, source_count,
		       COALESCE(message,''), last_synced_at
		FROM site_pricing_cache
		ORDER BY last_synced_at DESC
		LIMIT 200
	`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	sources := []ModelPricingSource{}
	items := []SitePricingCacheItem{}
	for rows.Next() {
		var item SitePricingCacheItem
		var sourcesJSON string
		if err := rows.Scan(&item.SiteID, &item.SiteName, &item.BaseURL, &item.Kind, &item.Status, &item.HTTPStatus, &item.LatencyMs, &item.SourcePath, &sourcesJSON, &item.ModelCount, &item.SourceCount, &item.Message, &item.LastSyncedAt); err != nil {
			return nil, nil, err
		}
		items = append(items, item)
		sources = append(sources, parsePricingSourcesJSON(sourcesJSON)...)
	}
	return sources, items, rows.Err()
}

// LoadAccountModelRecords returns the joined channel_accounts + upstream_sites
// rows used by model overview, pricing overview, and key export preview.
// Mirrors the body of core.loadAccountModelRecords (takes context.Context
// instead of *http.Request so the host handler can pass r.Context() through).
func (s *Service) LoadAccountModelRecords(ctx context.Context) ([]AccountModelRecord, error) {
	rows, err := s.infra.DB().QueryContext(ctx, `
		SELECT a.id, a.display_name, s.id, s.name, s.base_url, s.kind,
		       COALESCE(a.api_key_fingerprint,''), COALESCE(a.api_key_status,''), COALESCE(a.api_key_model_count,0),
		       COALESCE(a.api_key_sample_models_json,''), COALESCE(a.api_key_test_model,''),
		       COALESCE(a.api_key_model_usable,0), COALESCE(a.api_key_latency_ms,0), COALESCE(a.api_key_last_checked_at,'')
		FROM channel_accounts a
		JOIN upstream_sites s ON s.id = a.upstream_site_id
		WHERE COALESCE(a.api_key_fingerprint,'') <> ''
		ORDER BY s.name ASC, a.display_name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := []AccountModelRecord{}
	for rows.Next() {
		var record AccountModelRecord
		var sampleJSON string
		var modelUsable int
		if err := rows.Scan(&record.AccountID, &record.AccountName, &record.SiteID, &record.SiteName, &record.BaseURL, &record.Kind, &record.Fingerprint, &record.Status, &record.ModelCount, &sampleJSON, &record.TestModel, &modelUsable, &record.LatencyMs, &record.LastCheckedAt); err != nil {
			return nil, err
		}
		record.ModelUsable = modelUsable == 1
		record.SampleModels = parsePersistedStringSlice(sampleJSON)
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

// BuildPricingOverview aggregates raw + cached pricing sources into the
// pricing-overview response. Mirrors core.buildPricingOverview (pure function
// so the host handler can call it after assembling the inputs).
func BuildPricingOverview(sources []ModelPricingSource, cacheItems []SitePricingCacheItem, accountRecords []AccountModelRecord, generatedAt string) ModelPricingOverview {
	overview := ModelPricingOverview{
		GeneratedAt: generatedAt,
		Sources:     []ModelPricingSource{},
		SiteCaches:  cacheItems,
		Comparisons: []ModelPriceComparison{},
	}
	seenModels := map[string]bool{}
	seenSources := map[string]bool{}
	for _, source := range sources {
		if !looksLikeModelID(source.Model) {
			continue
		}
		key := source.ChannelID + "|" + strings.ToLower(source.Model) + "|" + source.Source + "|" + source.FieldPath + "|" + source.UpstreamModel
		if seenSources[key] {
			continue
		}
		seenSources[key] = true
		overview.Sources = append(overview.Sources, source)
		seenModels[strings.ToLower(source.Model)] = true
		if source.Price != nil {
			overview.ExactCount++
		}
		if source.PromptRatio != nil || source.CompletionRatio != nil {
			overview.RatioCount++
		}
	}
	for _, item := range cacheItems {
		if item.Status == "success" {
			overview.LiveCacheCount++
		} else if item.Status != "" {
			overview.FailedCacheCount++
		}
	}
	sort.SliceStable(overview.Sources, func(i, j int) bool {
		left := overview.Sources[i]
		right := overview.Sources[j]
		if left.Confidence != right.Confidence {
			return confidenceRank(left.Confidence) > confidenceRank(right.Confidence)
		}
		if left.Model != right.Model {
			return left.Model < right.Model
		}
		return left.ChannelName < right.ChannelName
	})
	overview.SourceCount = len(overview.Sources)
	overview.ModelCount = len(seenModels)
	overview.Comparisons = BuildModelPriceComparisons(overview.Sources, accountRecords)
	if len(overview.Sources) > 200 {
		overview.Sources = overview.Sources[:200]
	}
	return overview
}

// ExtractLivePricingSources mirrors core.extractLivePricingSources. Walks a
// raw /api/pricing response body and extracts pricing sources tagged with
// "live_api_pricing".
func ExtractLivePricingSources(record PricingSiteRecord, rawJSON string) []ModelPricingSource {
	root, ok := parseJSONLike(rawJSON)
	if !ok {
		return nil
	}
	root = expandJSONStrings(root, 0)
	context := pricingChannelContext{channelID: "site:" + record.SiteID, channelName: record.SiteName, baseURL: record.BaseURL, kind: record.Kind}
	sources := []ModelPricingSource{}
	models := extractModelsFromJSON(root)
	walkPricingJSON(context, root, []string{"api_pricing"}, models, &sources)
	for index := range sources {
		sources[index].Source = "live_api_pricing:" + sources[index].Source
		sources[index].Notes = firstNonEmpty(sources[index].Notes, "来自上游站点 /api/pricing 在线探测缓存。")
	}
	return sources
}

// BuildModelPriceComparisons mirrors core.buildModelPriceComparisons. Cross-
// references pricing sources against account model records to surface the
// cheapest usable option per model.
func BuildModelPriceComparisons(sources []ModelPricingSource, accountRecords []AccountModelRecord) []ModelPriceComparison {
	items := map[string]*ModelPriceComparison{}
	for _, source := range sources {
		key := strings.ToLower(source.Model)
		if key == "" {
			continue
		}
		item := items[key]
		if item == nil {
			item = &ModelPriceComparison{Model: source.Model, Sites: []string{}}
			items[key] = item
		}
		item.SourceCount++
		appendUniqueString(&item.Sites, source.ChannelName, 8)
		if source.Price != nil && (item.LowestPrice == nil || *source.Price < *item.LowestPrice) {
			item.LowestPrice = source.Price
			item.BestSource = source.ChannelName
		}
		if source.PromptRatio != nil && (item.LowestPromptRatio == nil || *source.PromptRatio < *item.LowestPromptRatio) {
			item.LowestPromptRatio = source.PromptRatio
			if item.BestSource == "" {
				item.BestSource = source.ChannelName
			}
		}
		if source.CompletionRatio != nil && (item.LowestCompletionRatio == nil || *source.CompletionRatio < *item.LowestCompletionRatio) {
			item.LowestCompletionRatio = source.CompletionRatio
		}
	}
	for _, record := range accountRecords {
		for _, model := range normalizedModelList(record) {
			item := items[strings.ToLower(model)]
			if item == nil {
				continue
			}
			if record.ModelUsable && strings.EqualFold(model, record.TestModel) {
				item.UsableAccountCount++
			}
			if record.LatencyMs > 0 && strings.EqualFold(model, record.TestModel) && (item.FastestLatencyMs == 0 || record.LatencyMs < item.FastestLatencyMs) {
				item.FastestLatencyMs = record.LatencyMs
			}
			appendUniqueString(&item.Sites, record.SiteName, 8)
		}
	}
	comparisons := []ModelPriceComparison{}
	for _, item := range items {
		item.SiteCount = len(item.Sites)
		if item.LowestPrice == nil && item.LowestPromptRatio == nil {
			item.Notes = "只有映射或模型覆盖信息，尚无可比较的价格/倍率。"
		} else if item.UsableAccountCount == 0 {
			item.Notes = "已有价格来源，但还没有账号 Key 证明该模型可调用。"
		} else {
			item.Notes = "已有价格来源和可用性检测，可作为优先候选。"
		}
		comparisons = append(comparisons, *item)
	}
	sort.SliceStable(comparisons, func(i, j int) bool {
		left := comparisons[i]
		right := comparisons[j]
		if left.UsableAccountCount != right.UsableAccountCount {
			return left.UsableAccountCount > right.UsableAccountCount
		}
		if left.LowestPromptRatio != nil && right.LowestPromptRatio != nil && *left.LowestPromptRatio != *right.LowestPromptRatio {
			return *left.LowestPromptRatio < *right.LowestPromptRatio
		}
		if left.SourceCount != right.SourceCount {
			return left.SourceCount > right.SourceCount
		}
		return left.Model < right.Model
	})
	if len(comparisons) > 80 {
		return comparisons[:80]
	}
	return comparisons
}

// marshalPricingSourcesLimit mirrors core.marshalPricingSourcesLimit. JSON-
// encodes up to limit sources; returns "" on error.
func marshalPricingSourcesLimit(values []ModelPricingSource, limit int) string {
	if len(values) > limit {
		values = values[:limit]
	}
	body, err := json.Marshal(values)
	if err != nil {
		return ""
	}
	return string(body)
}

// parsePricingSourcesJSON mirrors core.parsePricingSourcesJSON. Decodes the
// sources_json column from site_pricing_cache.
func parsePricingSourcesJSON(raw string) []ModelPricingSource {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var sources []ModelPricingSource
	if err := json.Unmarshal([]byte(raw), &sources); err != nil {
		return nil
	}
	return sources
}

// countPricingModels mirrors core.countPricingModels. Counts distinct model
// names (case-insensitive) across sources.
func countPricingModels(sources []ModelPricingSource) int {
	seen := map[string]bool{}
	for _, source := range sources {
		if source.Model != "" {
			seen[strings.ToLower(source.Model)] = true
		}
	}
	return len(seen)
}

// extractModelPricingSources mirrors core.extractModelPricingSources. Walks
// a channel's raw_json and extracts pricing sources tagged with "raw_json".
func extractModelPricingSources(channelID string, channelName string, baseURL string, kind string, rawJSON string) []ModelPricingSource {
	root, ok := parseJSONLike(rawJSON)
	if !ok {
		return nil
	}
	root = expandJSONStrings(root, 0)
	context := pricingChannelContext{channelID: channelID, channelName: channelName, baseURL: baseURL, kind: kind}
	sources := []ModelPricingSource{}
	models := extractModelsFromJSON(root)
	walkPricingJSON(context, root, []string{"raw_json"}, models, &sources)
	return sources
}

// pricingChannelContext mirrors core.pricingChannelContext. Carries the
// channel identity through the pricing-JSON walk so each source can be
// tagged with its origin.
type pricingChannelContext struct {
	channelID   string
	channelName string
	baseURL     string
	kind        string
}

// walkPricingJSON mirrors core.walkPricingJSON. Recursively walks value,
// emitting pricing sources for model-keyed entries.
func walkPricingJSON(context pricingChannelContext, value interface{}, path []string, knownModels []string, sources *[]ModelPricingSource) {
	switch typed := value.(type) {
	case map[string]interface{}:
		lowerPath := strings.ToLower(strings.Join(path, "."))
		if model := stringFromAny(typed["model"]); looksLikeModelID(model) {
			if source, ok := sourceFromPricingMap(context, model, "", lowerPath, path, typed); ok {
				*sources = append(*sources, source)
			}
		}
		for key, child := range typed {
			childPath := append(path, key)
			lowerKey := strings.ToLower(key)
			if looksLikeModelID(key) {
				if source, ok := sourceFromPricingValue(context, key, lowerKey, childPath, child); ok {
					*sources = append(*sources, source)
					continue
				}
			}
			if isModelMappingKey(lowerKey) {
				for _, source := range sourcesFromModelMapping(context, child, childPath) {
					*sources = append(*sources, source)
				}
				continue
			}
			if isPricingContainerKey(lowerKey) {
				for _, source := range sourcesFromPricingContainer(context, child, childPath, knownModels) {
					*sources = append(*sources, source)
				}
				continue
			}
			walkPricingJSON(context, child, childPath, knownModels, sources)
		}
	case []interface{}:
		for index, child := range typed {
			walkPricingJSON(context, child, append(path, strconv.Itoa(index)), knownModels, sources)
		}
	}
}

// sourcesFromModelMapping mirrors core.sourcesFromModelMapping. Extracts
// sources from a model_mapping object keyed by model name.
func sourcesFromModelMapping(context pricingChannelContext, value interface{}, path []string) []ModelPricingSource {
	value = expandJSONStrings(value, 0)
	typed, ok := value.(map[string]interface{})
	if !ok {
		return nil
	}
	sources := []ModelPricingSource{}
	for model, upstream := range typed {
		model = strings.TrimSpace(model)
		if !looksLikeModelID(model) {
			continue
		}
		sources = append(sources, basePricingSource(context, model, "model_mapping", strings.Join(append(path, model), "."), "mapping", "映射关系来自 NewAPI channel model_mapping。", map[string]interface{}{"upstream": upstream}))
		sources[len(sources)-1].UpstreamModel = stringFromAny(upstream)
	}
	return sources
}

// sourcesFromPricingContainer mirrors core.sourcesFromPricingContainer.
// Extracts sources from a pricing container keyed by model name.
func sourcesFromPricingContainer(context pricingChannelContext, value interface{}, path []string, knownModels []string) []ModelPricingSource {
	value = expandJSONStrings(value, 0)
	sources := []ModelPricingSource{}
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, child := range typed {
			model := strings.TrimSpace(key)
			childPath := append(path, key)
			if looksLikeModelID(model) {
				if source, ok := sourceFromPricingValue(context, model, strings.ToLower(path[len(path)-1]), childPath, child); ok {
					sources = append(sources, source)
				}
				continue
			}
			if childMap, ok := expandJSONStrings(child, 0).(map[string]interface{}); ok {
				if nestedModel := stringFromAny(childMap["model"]); looksLikeModelID(nestedModel) {
					if source, ok := sourceFromPricingMap(context, nestedModel, "", strings.ToLower(strings.Join(childPath, ".")), childPath, childMap); ok {
						sources = append(sources, source)
					}
				}
			}
		}
	case []interface{}:
		for index, child := range typed {
			childPath := append(path, strconv.Itoa(index))
			if childMap, ok := expandJSONStrings(child, 0).(map[string]interface{}); ok {
				if model := stringFromAny(childMap["model"]); looksLikeModelID(model) {
					if source, ok := sourceFromPricingMap(context, model, "", strings.ToLower(strings.Join(childPath, ".")), childPath, childMap); ok {
						sources = append(sources, source)
					}
				}
			}
		}
	case float64, int, int64, json.Number:
		for _, model := range knownModels {
			if source, ok := sourceFromPricingValue(context, model, strings.ToLower(path[len(path)-1]), append(path, model), typed); ok {
				sources = append(sources, source)
			}
		}
	}
	return sources
}

// sourceFromPricingValue mirrors core.sourceFromPricingValue. Builds a
// source from a scalar or map pricing value.
func sourceFromPricingValue(context pricingChannelContext, model string, sourceKey string, path []string, value interface{}) (ModelPricingSource, bool) {
	if number, ok := numericFromAny(value); ok {
		source := basePricingSource(context, model, sourceKey, strings.Join(path, "."), "medium", "从渠道配置的数值字段提取。", value)
		applyPricingNumber(&source, sourceKey, number)
		return source, true
	}
	if typed, ok := expandJSONStrings(value, 0).(map[string]interface{}); ok {
		return sourceFromPricingMap(context, model, sourceKey, strings.ToLower(strings.Join(path, ".")), path, typed)
	}
	return ModelPricingSource{}, false
}

// sourceFromPricingMap mirrors core.sourceFromPricingMap. Builds a source
// from a map containing price/ratio/quota fields.
func sourceFromPricingMap(context pricingChannelContext, model string, sourceKey string, lowerPath string, path []string, value map[string]interface{}) (ModelPricingSource, bool) {
	source := basePricingSource(context, model, firstNonEmpty(sourceKey, lowerPath), strings.Join(path, "."), "high", "从渠道配置对象提取。", value)
	found := false
	for key, child := range value {
		lowerKey := strings.ToLower(strings.TrimSpace(key))
		number, ok := numericFromAny(child)
		if !ok {
			continue
		}
		switch {
		case strings.Contains(lowerKey, "completion") || strings.Contains(lowerKey, "output"):
			source.CompletionRatio = floatPtr(number)
			found = true
		case strings.Contains(lowerKey, "prompt") || strings.Contains(lowerKey, "input"):
			source.PromptRatio = floatPtr(number)
			found = true
		case strings.Contains(lowerKey, "ratio"):
			source.PromptRatio = floatPtr(number)
			found = true
		case strings.Contains(lowerKey, "price") || strings.Contains(lowerKey, "quota"):
			source.Price = floatPtr(number)
			found = true
		}
	}
	if currency := firstNonEmpty(stringFromAny(value["currency"]), stringFromAny(value["unit"])); currency != "" {
		if len(currency) <= 8 {
			source.Currency = currency
		} else {
			source.Unit = currency
		}
	}
	if !found {
		return ModelPricingSource{}, false
	}
	return source, true
}

// basePricingSource mirrors core.basePricingSource. Constructs a
// ModelPricingSource with the common fields filled in.
func basePricingSource(context pricingChannelContext, model string, source string, fieldPath string, confidence string, notes string, raw interface{}) ModelPricingSource {
	return ModelPricingSource{
		ChannelID:      context.channelID,
		ChannelName:    context.channelName,
		BaseURL:        context.baseURL,
		Kind:           context.kind,
		Model:          model,
		Source:         source,
		FieldPath:      fieldPath,
		Confidence:     confidence,
		Notes:          notes,
		RawValueMasked: maskResponse(fmt.Sprint(raw)),
	}
}

// applyPricingNumber mirrors core.applyPricingNumber. Maps a scalar pricing
// value onto the appropriate source field based on its key.
func applyPricingNumber(source *ModelPricingSource, sourceKey string, number float64) {
	switch {
	case strings.Contains(sourceKey, "completion") || strings.Contains(sourceKey, "output"):
		source.CompletionRatio = floatPtr(number)
	case strings.Contains(sourceKey, "price") || strings.Contains(sourceKey, "pricing") || strings.Contains(sourceKey, "quota"):
		source.Price = floatPtr(number)
	default:
		source.PromptRatio = floatPtr(number)
	}
}
