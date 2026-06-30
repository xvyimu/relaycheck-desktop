package channels

import (
	"sort"
	"strings"
)

// BuildModelOverview mirrors core.buildModelOverview. Aggregates per-account
// model records into a coverage overview (models, sites, price hints).
// generatedAt is injected so callers can control the timestamp (host passes
// s.infra.Now()).
func BuildModelOverview(records []AccountModelRecord, generatedAt string) ModelOverview {
	overview := ModelOverview{
		GeneratedAt: generatedAt,
		Models:      []ModelCoverageItem{},
		Sites:       []SiteModelCoverageItem{},
		PriceHints:  []ModelPriceHint{},
	}
	models := map[string]*ModelCoverageItem{}
	sites := map[string]*SiteModelCoverageItem{}
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
			site = &SiteModelCoverageItem{
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
				item = &ModelCoverageItem{Model: model, Sites: []string{}, Fingerprints: []string{}}
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

// normalizedModelList mirrors core.normalizedModelList. Returns the deduped
// union of record.SampleModels and record.TestModel.
func normalizedModelList(record AccountModelRecord) []string {
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

// inferModelPriceHint mirrors core.inferModelPriceHint. Returns a lightweight
// vendor/level hint inferred from the model name.
func inferModelPriceHint(model string) (ModelPriceHint, bool) {
	lower := strings.ToLower(strings.TrimSpace(model))
	if lower == "" {
		return ModelPriceHint{}, false
	}
	hint := ModelPriceHint{Model: model, Vendor: "unknown", PriceLevel: "unknown", Notes: "未获取官方价格，仅按模型名称给出粗略分层。"}
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

// priceLevelBySuffix mirrors core.priceLevelBySuffix. Maps model-name
// suffixes to a coarse price-level bucket.
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

// limitModelCoverageItems mirrors core.limitModelCoverageItems. Returns the
// first limit items of values.
func limitModelCoverageItems(values []ModelCoverageItem, limit int) []ModelCoverageItem {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}

// limitSiteCoverageItems mirrors core.limitSiteCoverageItems. Returns the
// first limit items of values.
func limitSiteCoverageItems(values []SiteModelCoverageItem, limit int) []SiteModelCoverageItem {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}

// limitModelPriceHints mirrors core.limitModelPriceHints. Returns the first
// limit items of values.
func limitModelPriceHints(values []ModelPriceHint, limit int) []ModelPriceHint {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}
