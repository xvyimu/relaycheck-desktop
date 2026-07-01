package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

type modelOverview struct {
	GeneratedAt      string                  `json:"generatedAt"`
	SyncedAccounts   int                     `json:"syncedAccounts,omitempty"`
	ModelCount       int                     `json:"modelCount"`
	AccountCount     int                     `json:"accountCount"`
	ValidKeyCount    int                     `json:"validKeyCount"`
	UsableModelCount int                     `json:"usableModelCount"`
	FastestLatencyMs int64                   `json:"fastestLatencyMs,omitempty"`
	Models           []modelCoverageItem     `json:"models"`
	Sites            []siteModelCoverageItem `json:"sites"`
	PriceHints       []modelPriceHint        `json:"priceHints"`
}

type modelCoverageItem struct {
	Model            string   `json:"model"`
	AccountCount     int      `json:"accountCount"`
	ValidKeyCount    int      `json:"validKeyCount"`
	UsableCount      int      `json:"usableCount"`
	FastestLatencyMs int64    `json:"fastestLatencyMs,omitempty"`
	Sites            []string `json:"sites,omitempty"`
	Fingerprints     []string `json:"fingerprints,omitempty"`
}

type siteModelCoverageItem struct {
	SiteID           string   `json:"siteId"`
	SiteName         string   `json:"siteName"`
	BaseURL          string   `json:"baseUrl"`
	Kind             string   `json:"kind"`
	ModelCount       int      `json:"modelCount"`
	ValidKeyCount    int      `json:"validKeyCount"`
	UsableKeyCount   int      `json:"usableKeyCount"`
	FastestLatencyMs int64    `json:"fastestLatencyMs,omitempty"`
	SampleModels     []string `json:"sampleModels,omitempty"`
}

type modelPriceHint struct {
	Model      string `json:"model"`
	Vendor     string `json:"vendor"`
	PriceLevel string `json:"priceLevel"`
	Notes      string `json:"notes"`
}

type modelPricingOverview struct {
	GeneratedAt      string                 `json:"generatedAt"`
	SourceCount      int                    `json:"sourceCount"`
	ModelCount       int                    `json:"modelCount"`
	ExactCount       int                    `json:"exactCount"`
	RatioCount       int                    `json:"ratioCount"`
	LiveCacheCount   int                    `json:"liveCacheCount"`
	FailedCacheCount int                    `json:"failedCacheCount"`
	Sources          []modelPricingSource   `json:"sources"`
	SiteCaches       []sitePricingCacheItem `json:"siteCaches"`
	Comparisons      []modelPriceComparison `json:"comparisons"`
}

type modelPricingSource struct {
	ChannelID       string   `json:"channelId"`
	ChannelName     string   `json:"channelName"`
	BaseURL         string   `json:"baseUrl,omitempty"`
	Kind            string   `json:"kind"`
	Model           string   `json:"model"`
	UpstreamModel   string   `json:"upstreamModel,omitempty"`
	Source          string   `json:"source"`
	FieldPath       string   `json:"fieldPath"`
	Price           *float64 `json:"price,omitempty"`
	PromptRatio     *float64 `json:"promptRatio,omitempty"`
	CompletionRatio *float64 `json:"completionRatio,omitempty"`
	Unit            string   `json:"unit,omitempty"`
	Currency        string   `json:"currency,omitempty"`
	Confidence      string   `json:"confidence"`
	Notes           string   `json:"notes,omitempty"`
	RawValueMasked  string   `json:"rawValueMasked,omitempty"`
}

type sitePricingCacheItem struct {
	SiteID       string `json:"siteId"`
	SiteName     string `json:"siteName"`
	BaseURL      string `json:"baseUrl"`
	Kind         string `json:"kind"`
	Status       string `json:"status"`
	HTTPStatus   int    `json:"httpStatus,omitempty"`
	LatencyMs    int64  `json:"latencyMs,omitempty"`
	SourcePath   string `json:"sourcePath"`
	SourceCount  int    `json:"sourceCount"`
	ModelCount   int    `json:"modelCount"`
	Message      string `json:"message,omitempty"`
	LastSyncedAt string `json:"lastSyncedAt,omitempty"`
}

type modelPriceComparison struct {
	Model                 string   `json:"model"`
	SourceCount           int      `json:"sourceCount"`
	SiteCount             int      `json:"siteCount"`
	UsableAccountCount    int      `json:"usableAccountCount"`
	FastestLatencyMs      int64    `json:"fastestLatencyMs,omitempty"`
	LowestPrice           *float64 `json:"lowestPrice,omitempty"`
	LowestPromptRatio     *float64 `json:"lowestPromptRatio,omitempty"`
	LowestCompletionRatio *float64 `json:"lowestCompletionRatio,omitempty"`
	BestSource            string   `json:"bestSource,omitempty"`
	Sites                 []string `json:"sites,omitempty"`
	Notes                 string   `json:"notes,omitempty"`
}

type pricingSiteRecord struct {
	SiteID   string
	SiteName string
	BaseURL  string
	Kind     string
}

type keyExportPreview struct {
	GeneratedAt string                 `json:"generatedAt"`
	Total       int                    `json:"total"`
	Valid       int                    `json:"valid"`
	Usable      int                    `json:"usable"`
	Items       []keyExportPreviewItem `json:"items"`
	Notice      string                 `json:"notice"`
}

type keyExportPreviewItem struct {
	AccountID       string   `json:"accountId"`
	AccountName     string   `json:"accountName"`
	SiteName        string   `json:"siteName"`
	BaseURL         string   `json:"baseUrl"`
	Fingerprint     string   `json:"fingerprint"`
	Status          string   `json:"status"`
	ModelCount      int      `json:"modelCount"`
	SampleModels    []string `json:"sampleModels,omitempty"`
	TestModel       string   `json:"testModel,omitempty"`
	ModelUsable     bool     `json:"modelUsable"`
	LatencyMs       int64    `json:"latencyMs,omitempty"`
	LastCheckedAt   string   `json:"lastCheckedAt,omitempty"`
	MaskedExportRef string   `json:"maskedExportRef"`
}

type accountModelRecord struct {
	AccountID     string
	AccountName   string
	SiteID        string
	SiteName      string
	BaseURL       string
	Kind          string
	Fingerprint   string
	Status        string
	ModelCount    int
	SampleModels  []string
	TestModel     string
	ModelUsable   bool
	LatencyMs     int64
	LastCheckedAt string
}

func (a *App) handleModelOverview(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	overview, err := cachedRead(a, "models-overview", overviewReadCacheTTL, func() (modelOverview, error) {
		records, err := a.loadAccountModelRecords(r)
		if err != nil {
			return modelOverview{}, err
		}
		return buildModelOverview(records), nil
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, overview)
}

func (a *App) handleModelSync(w http.ResponseWriter, r *http.Request) {
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
	rows, err := a.db.QueryContext(r.Context(), `
		SELECT id FROM channel_accounts
		WHERE COALESCE(api_key_encrypted,'') <> ''
		ORDER BY COALESCE(api_key_last_checked_at,''), updated_at DESC
		LIMIT ?
	`, input.Limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = rows.Close()
	auths, _ := a.loadAccountAuths(r.Context(), ids)
	for _, id := range ids {
		var auth *accountAuthContext
		if loaded, ok := auths[id]; ok {
			auth = &loaded
		}
		_ = a.testAPIKeyForAccount(r.Context(), id, auth)
	}
	records, err := a.loadAccountModelRecords(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	overview := buildModelOverview(records)
	overview.SyncedAccounts = len(ids)
	if len(ids) > 0 {
		a.notify("model_sync_completed", "success", "模型同步完成", "已检测并同步 "+strconv.Itoa(len(ids))+" 个 Key 的模型状态。", "model", "")
	}
	writeJSON(w, http.StatusOK, overview)
}

func (a *App) handleKeyExportPreview(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	records, err := a.loadAccountModelRecords(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	preview := keyExportPreview{
		GeneratedAt: now(),
		Items:       []keyExportPreviewItem{},
		Notice:      "安全模式只导出账号、站点、Key 指纹、模型和测速状态，不导出真实 API Key。如需真实密钥，请在原平台重新生成或手动复制。",
	}
	for _, record := range records {
		if record.Fingerprint == "" {
			continue
		}
		preview.Total++
		if record.Status == "valid" {
			preview.Valid++
		}
		if record.ModelUsable {
			preview.Usable++
		}
		preview.Items = append(preview.Items, keyExportPreviewItem{
			AccountID:       record.AccountID,
			AccountName:     record.AccountName,
			SiteName:        record.SiteName,
			BaseURL:         record.BaseURL,
			Fingerprint:     record.Fingerprint,
			Status:          firstNonEmpty(record.Status, "unchecked"),
			ModelCount:      record.ModelCount,
			SampleModels:    limitStrings(record.SampleModels, 6),
			TestModel:       record.TestModel,
			ModelUsable:     record.ModelUsable,
			LatencyMs:       record.LatencyMs,
			LastCheckedAt:   record.LastCheckedAt,
			MaskedExportRef: record.SiteName + " · " + record.Fingerprint,
		})
	}
	sort.SliceStable(preview.Items, func(i, j int) bool {
		left := preview.Items[i]
		right := preview.Items[j]
		if left.Status != right.Status {
			return left.Status == "valid"
		}
		if left.ModelUsable != right.ModelUsable {
			return left.ModelUsable
		}
		return left.SiteName < right.SiteName
	})
	a.audit("keys.export_preview", "info", "", "api_key", "", fmt.Sprintf("Key 脱敏导出预览：%d 个指纹。", preview.Total), map[string]interface{}{"total": preview.Total, "valid": preview.Valid, "usable": preview.Usable})
	writeJSON(w, http.StatusOK, preview)
}

func (a *App) handleModelPricing(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	overview, err := cachedRead(a, "models-pricing", overviewReadCacheTTL, func() (modelPricingOverview, error) {
		rawSources, err := a.loadRawChannelPricingSources(r.Context())
		if err != nil {
			return modelPricingOverview{}, err
		}
		cacheSources, cacheItems, err := a.loadSitePricingCache(r.Context())
		if err != nil {
			return modelPricingOverview{}, err
		}
		accountRecords, err := a.loadAccountModelRecords(r)
		if err != nil {
			return modelPricingOverview{}, err
		}
		return buildPricingOverview(append(rawSources, cacheSources...), cacheItems, accountRecords), nil
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, overview)
}

func (a *App) handleModelPricingSync(w http.ResponseWriter, r *http.Request) {
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
	records, err := a.loadPricingSiteRecords(r.Context(), input.Limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	failedSites := []string{}
	for _, record := range records {
		item := a.syncSitePricing(r.Context(), record)
		if item.Status != "" && item.Status != "success" {
			log.Printf("[pricing] sync site %s (%s) returned status=%s: %s", record.SiteID, record.SiteName, item.Status, item.Message)
			failedSites = append(failedSites, record.SiteName)
		}
	}
	rawSources, err := a.loadRawChannelPricingSources(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	cacheSources, cacheItems, err := a.loadSitePricingCache(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	accountRecords, err := a.loadAccountModelRecords(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	overview := buildPricingOverview(append(rawSources, cacheSources...), cacheItems, accountRecords)
	if len(records) > 0 {
		msg := fmt.Sprintf("已探测 %d 个上游站点的 /api/pricing。", len(records))
		level := "success"
		if len(failedSites) > 0 {
			level = "warning"
			msg += fmt.Sprintf(" %d 个站点同步失败：%s。", len(failedSites), strings.Join(failedSites, "、"))
		}
		a.notify("pricing_sync_completed", level, "价格同步完成", msg, "pricing", "")
	}
	writeJSON(w, http.StatusOK, overview)
}

// loadRawChannelPricingSources forwards to the channels service. The host
// keeps this method so handlers in core can call it with core types; the
// channels service owns the SQL and the extraction engine.
func (a *App) loadRawChannelPricingSources(ctx context.Context) ([]modelPricingSource, error) {
	mirror, err := a.channelsService.LoadRawChannelPricingSources(ctx)
	if err != nil {
		return nil, err
	}
	return pricingSourcesToCore(mirror), nil
}

func buildPricingOverview(sources []modelPricingSource, cacheItems []sitePricingCacheItem, accountRecords []accountModelRecord) modelPricingOverview {
	overview := modelPricingOverview{
		GeneratedAt: now(),
		Sources:     []modelPricingSource{},
		SiteCaches:  cacheItems,
		Comparisons: []modelPriceComparison{},
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
	overview.Comparisons = buildModelPriceComparisons(overview.Sources, accountRecords)
	if len(overview.Sources) > 200 {
		overview.Sources = overview.Sources[:200]
	}
	return overview
}

// loadPricingSiteRecords forwards to the channels service so the host handler
// gets the same []pricingSiteRecord without owning the SQL.
func (a *App) loadPricingSiteRecords(ctx context.Context, limit int) ([]pricingSiteRecord, error) {
	mirror, err := a.channelsService.LoadPricingSiteRecords(ctx, limit)
	if err != nil {
		return nil, err
	}
	if len(mirror) == 0 {
		return nil, nil
	}
	out := make([]pricingSiteRecord, 0, len(mirror))
	for _, m := range mirror {
		out = append(out, pricingSiteRecord{
			SiteID:   m.SiteID,
			SiteName: m.SiteName,
			BaseURL:  m.BaseURL,
			Kind:     m.Kind,
		})
	}
	return out, nil
}

// syncSitePricing forwards to the channels service. The channels service owns
// the HTTP probe, live-source extraction, and site_pricing_cache upsert; the
// host handler just needs the resulting cache item in core's type.
func (a *App) syncSitePricing(ctx context.Context, record pricingSiteRecord) sitePricingCacheItem {
	mirror := a.channelsService.SyncSitePricing(ctx, pricingSiteRecordToMirror(record))
	return sitePricingCacheItemFromMirror(mirror)
}

// loadSitePricingCache forwards to the channels service. Returns the cached
// /api/pricing sources and items in core types so the pricing-overview builder
// can aggregate them without a type change.
func (a *App) loadSitePricingCache(ctx context.Context) ([]modelPricingSource, []sitePricingCacheItem, error) {
	mirrorSources, mirrorItems, err := a.channelsService.LoadSitePricingCache(ctx)
	if err != nil {
		return nil, nil, err
	}
	return pricingSourcesToCore(mirrorSources), sitePricingCacheItemsFromMirror(mirrorItems), nil
}

func extractLivePricingSources(record pricingSiteRecord, rawJSON string) []modelPricingSource {
	root, ok := parseJSONLike(rawJSON)
	if !ok {
		return nil
	}
	root = expandJSONStrings(root, 0)
	context := pricingChannelContext{channelID: "site:" + record.SiteID, channelName: record.SiteName, baseURL: record.BaseURL, kind: record.Kind}
	sources := []modelPricingSource{}
	models := extractModelsFromJSON(root)
	walkPricingJSON(context, root, []string{"api_pricing"}, models, &sources)
	for index := range sources {
		sources[index].Source = "live_api_pricing:" + sources[index].Source
		sources[index].Notes = firstNonEmpty(sources[index].Notes, "来自上游站点 /api/pricing 在线探测缓存。")
	}
	return sources
}

func buildModelPriceComparisons(sources []modelPricingSource, accountRecords []accountModelRecord) []modelPriceComparison {
	items := map[string]*modelPriceComparison{}
	for _, source := range sources {
		key := strings.ToLower(source.Model)
		if key == "" {
			continue
		}
		item := items[key]
		if item == nil {
			item = &modelPriceComparison{Model: source.Model, Sites: []string{}}
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
	comparisons := []modelPriceComparison{}
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

// loadAccountModelRecords forwards to the channels service. The channels
// service owns the joined channel_accounts + upstream_sites query; the host
// keeps this method so handlers (model overview, pricing, key export preview)
// can call it with the original *http.Request signature and get back core
// accountModelRecord values.
func (a *App) loadAccountModelRecords(r *http.Request) ([]accountModelRecord, error) {
	mirror, err := a.channelsService.LoadAccountModelRecords(r.Context())
	if err != nil {
		return nil, err
	}
	return accountModelRecordsToCore(mirror), nil
}

func extractModelPricingSources(channelID string, channelName string, baseURL string, kind string, rawJSON string) []modelPricingSource {
	root, ok := parseJSONLike(rawJSON)
	if !ok {
		return nil
	}
	root = expandJSONStrings(root, 0)
	context := pricingChannelContext{channelID: channelID, channelName: channelName, baseURL: baseURL, kind: kind}
	sources := []modelPricingSource{}
	models := extractModelsFromJSON(root)
	walkPricingJSON(context, root, []string{"raw_json"}, models, &sources)
	return sources
}

type pricingChannelContext struct {
	channelID   string
	channelName string
	baseURL     string
	kind        string
}

func walkPricingJSON(context pricingChannelContext, value interface{}, path []string, knownModels []string, sources *[]modelPricingSource) {
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

func sourcesFromModelMapping(context pricingChannelContext, value interface{}, path []string) []modelPricingSource {
	value = expandJSONStrings(value, 0)
	typed, ok := value.(map[string]interface{})
	if !ok {
		return nil
	}
	sources := []modelPricingSource{}
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

func sourcesFromPricingContainer(context pricingChannelContext, value interface{}, path []string, knownModels []string) []modelPricingSource {
	value = expandJSONStrings(value, 0)
	sources := []modelPricingSource{}
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

func sourceFromPricingValue(context pricingChannelContext, model string, sourceKey string, path []string, value interface{}) (modelPricingSource, bool) {
	if number, ok := numericFromAny(value); ok {
		source := basePricingSource(context, model, sourceKey, strings.Join(path, "."), "medium", "从渠道配置的数值字段提取。", value)
		applyPricingNumber(&source, sourceKey, number)
		return source, true
	}
	if typed, ok := expandJSONStrings(value, 0).(map[string]interface{}); ok {
		return sourceFromPricingMap(context, model, sourceKey, strings.ToLower(strings.Join(path, ".")), path, typed)
	}
	return modelPricingSource{}, false
}

func sourceFromPricingMap(context pricingChannelContext, model string, sourceKey string, lowerPath string, path []string, value map[string]interface{}) (modelPricingSource, bool) {
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
		return modelPricingSource{}, false
	}
	return source, true
}

func basePricingSource(context pricingChannelContext, model string, source string, fieldPath string, confidence string, notes string, raw interface{}) modelPricingSource {
	return modelPricingSource{
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

func applyPricingNumber(source *modelPricingSource, sourceKey string, number float64) {
	switch {
	case strings.Contains(sourceKey, "completion") || strings.Contains(sourceKey, "output"):
		source.CompletionRatio = floatPtr(number)
	case strings.Contains(sourceKey, "price") || strings.Contains(sourceKey, "pricing") || strings.Contains(sourceKey, "quota"):
		source.Price = floatPtr(number)
	default:
		source.PromptRatio = floatPtr(number)
	}
}

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

func parseJSONLike(raw string) (interface{}, bool) {
	var value interface{}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, false
	}
	return value, true
}

func expandJSONStrings(value interface{}, depth int) interface{} {
	if depth > 3 {
		return value
	}
	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		if !strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, "[") {
			return typed
		}
		if parsed, ok := parseJSONLike(trimmed); ok {
			return expandJSONStrings(parsed, depth+1)
		}
		return typed
	case map[string]interface{}:
		next := map[string]interface{}{}
		for key, child := range typed {
			next[key] = expandJSONStrings(child, depth+1)
		}
		return next
	case []interface{}:
		next := make([]interface{}, 0, len(typed))
		for _, child := range typed {
			next = append(next, expandJSONStrings(child, depth+1))
		}
		return next
	default:
		return value
	}
}

func isModelMappingKey(key string) bool {
	return strings.Contains(key, "model_mapping") || strings.Contains(key, "modelmap") || strings.Contains(key, "model_map")
}

func isPricingContainerKey(key string) bool {
	return strings.Contains(key, "price") ||
		strings.Contains(key, "pricing") ||
		strings.Contains(key, "ratio") ||
		strings.Contains(key, "quota") ||
		strings.Contains(key, "model_ratio") ||
		strings.Contains(key, "completion_ratio")
}

func numericFromAny(value interface{}) (float64, bool) {
	switch typed := value.(type) {
	case json.Number:
		number, err := typed.Float64()
		return number, err == nil
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case string:
		number, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return number, err == nil
	default:
		return 0, false
	}
}

func stringFromAny(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	default:
		if value == nil {
			return ""
		}
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func floatPtr(value float64) *float64 {
	return &value
}

func confidenceRank(confidence string) int {
	switch confidence {
	case "high":
		return 3
	case "medium":
		return 2
	case "mapping":
		return 1
	default:
		return 0
	}
}

func buildModelOverview(records []accountModelRecord) modelOverview {
	overview := modelOverview{
		GeneratedAt: now(),
		Models:      []modelCoverageItem{},
		Sites:       []siteModelCoverageItem{},
		PriceHints:  []modelPriceHint{},
	}
	models := map[string]*modelCoverageItem{}
	sites := map[string]*siteModelCoverageItem{}
	seenPriceHints := map[string]bool{}
	for _, record := range records {
		overview.AccountCount++
		if record.Status == "valid" {
			overview.ValidKeyCount++
		}
		if record.ModelUsable {
			overview.UsableModelCount++
		}
		if record.LatencyMs > 0 && (overview.FastestLatencyMs == 0 || record.LatencyMs < overview.FastestLatencyMs) {
			overview.FastestLatencyMs = record.LatencyMs
		}
		site := sites[record.SiteID]
		if site == nil {
			site = &siteModelCoverageItem{
				SiteID:       record.SiteID,
				SiteName:     record.SiteName,
				BaseURL:      record.BaseURL,
				Kind:         record.Kind,
				SampleModels: []string{},
			}
			sites[record.SiteID] = site
		}
		if record.Status == "valid" {
			site.ValidKeyCount++
		}
		if record.ModelUsable {
			site.UsableKeyCount++
		}
		if record.LatencyMs > 0 && (site.FastestLatencyMs == 0 || record.LatencyMs < site.FastestLatencyMs) {
			site.FastestLatencyMs = record.LatencyMs
		}
		for _, model := range normalizedModelList(record) {
			item := models[model]
			if item == nil {
				item = &modelCoverageItem{Model: model, Sites: []string{}, Fingerprints: []string{}}
				models[model] = item
				if hint, ok := inferModelPriceHint(model); ok && !seenPriceHints[model] {
					overview.PriceHints = append(overview.PriceHints, hint)
					seenPriceHints[model] = true
				}
			}
			item.AccountCount++
			if record.Status == "valid" {
				item.ValidKeyCount++
			}
			if record.ModelUsable && strings.EqualFold(model, record.TestModel) {
				item.UsableCount++
			}
			if record.LatencyMs > 0 && strings.EqualFold(model, record.TestModel) && (item.FastestLatencyMs == 0 || record.LatencyMs < item.FastestLatencyMs) {
				item.FastestLatencyMs = record.LatencyMs
			}
			appendUniqueString(&item.Sites, record.SiteName, 6)
			appendUniqueString(&item.Fingerprints, record.Fingerprint, 6)
			appendUniqueString(&site.SampleModels, model, 8)
		}
		site.ModelCount = len(site.SampleModels)
	}
	for _, item := range models {
		overview.Models = append(overview.Models, *item)
	}
	for _, site := range sites {
		overview.Sites = append(overview.Sites, *site)
	}
	overview.ModelCount = len(overview.Models)
	sort.SliceStable(overview.Models, func(i, j int) bool {
		left := overview.Models[i]
		right := overview.Models[j]
		if left.UsableCount != right.UsableCount {
			return left.UsableCount > right.UsableCount
		}
		if left.AccountCount != right.AccountCount {
			return left.AccountCount > right.AccountCount
		}
		return left.Model < right.Model
	})
	sort.SliceStable(overview.Sites, func(i, j int) bool {
		left := overview.Sites[i]
		right := overview.Sites[j]
		if left.UsableKeyCount != right.UsableKeyCount {
			return left.UsableKeyCount > right.UsableKeyCount
		}
		if left.ValidKeyCount != right.ValidKeyCount {
			return left.ValidKeyCount > right.ValidKeyCount
		}
		return left.SiteName < right.SiteName
	})
	sort.SliceStable(overview.PriceHints, func(i, j int) bool {
		return overview.PriceHints[i].Model < overview.PriceHints[j].Model
	})
	overview.Models = limitModelCoverageItems(overview.Models, 80)
	overview.Sites = limitSiteCoverageItems(overview.Sites, 40)
	overview.PriceHints = limitModelPriceHints(overview.PriceHints, 40)
	return overview
}

func normalizedModelList(record accountModelRecord) []string {
	models := append([]string{}, record.SampleModels...)
	if record.TestModel != "" {
		models = append(models, record.TestModel)
	}
	seen := map[string]bool{}
	normalized := []string{}
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" || seen[strings.ToLower(model)] {
			continue
		}
		seen[strings.ToLower(model)] = true
		normalized = append(normalized, model)
	}
	return normalized
}

func inferModelPriceHint(model string) (modelPriceHint, bool) {
	lower := strings.ToLower(strings.TrimSpace(model))
	if lower == "" {
		return modelPriceHint{}, false
	}
	hint := modelPriceHint{Model: model, Vendor: "unknown", PriceLevel: "unknown", Notes: "未获取官方价格，仅按模型名称给出粗略分层。"}
	switch {
	case strings.HasPrefix(lower, "gpt-4.1") || strings.HasPrefix(lower, "gpt-4o") || strings.HasPrefix(lower, "o1") || strings.HasPrefix(lower, "o3"):
		hint.Vendor = "OpenAI"
		hint.PriceLevel = priceLevelBySuffix(lower)
	case strings.HasPrefix(lower, "claude-"):
		hint.Vendor = "Anthropic"
		hint.PriceLevel = priceLevelBySuffix(lower)
	case strings.HasPrefix(lower, "gemini"):
		hint.Vendor = "Google"
		hint.PriceLevel = priceLevelBySuffix(lower)
	case strings.HasPrefix(lower, "deepseek"):
		hint.Vendor = "DeepSeek"
		hint.PriceLevel = "low"
	case strings.HasPrefix(lower, "qwen"):
		hint.Vendor = "Qwen"
		hint.PriceLevel = priceLevelBySuffix(lower)
	case strings.HasPrefix(lower, "glm"):
		hint.Vendor = "Zhipu"
		hint.PriceLevel = priceLevelBySuffix(lower)
	case strings.HasPrefix(lower, "doubao"):
		hint.Vendor = "ByteDance"
		hint.PriceLevel = priceLevelBySuffix(lower)
	case strings.HasPrefix(lower, "moonshot") || strings.Contains(lower, "kimi"):
		hint.Vendor = "Moonshot"
		hint.PriceLevel = priceLevelBySuffix(lower)
	default:
		return hint, false
	}
	hint.Notes = "轻量价格层级：cheap/low/standard/high/unknown；完整价格仍需站点 /api/pricing 或后台倍率同步。"
	return hint, true
}

func priceLevelBySuffix(model string) string {
	switch {
	case strings.Contains(model, "mini"), strings.Contains(model, "flash"), strings.Contains(model, "lite"), strings.Contains(model, "turbo"):
		return "cheap"
	case strings.Contains(model, "pro"), strings.Contains(model, "plus"):
		return "standard"
	case strings.Contains(model, "opus"), strings.Contains(model, "max"), strings.Contains(model, "32k"), strings.Contains(model, "128k"):
		return "high"
	default:
		return "unknown"
	}
}

func appendUniqueString(values *[]string, value string, limit int) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	for _, existing := range *values {
		if strings.EqualFold(existing, value) {
			return
		}
	}
	if len(*values) >= limit {
		return
	}
	*values = append(*values, value)
}

func limitModelCoverageItems(values []modelCoverageItem, limit int) []modelCoverageItem {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}

func limitSiteCoverageItems(values []siteModelCoverageItem, limit int) []siteModelCoverageItem {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}

func limitModelPriceHints(values []modelPriceHint, limit int) []modelPriceHint {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}

func cloneModelOverviewForTest(records []accountModelRecord) modelOverview {
	body, _ := json.Marshal(buildModelOverview(records))
	var cloned modelOverview
	_ = json.Unmarshal(body, &cloned)
	return cloned
}
