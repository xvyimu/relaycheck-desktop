package channels

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// This file duplicates small pure helpers from the core package so the
// channels service can build without importing core. Each copy carries a
// pointer to its origin; the two must stay in sync. The duplication follows
// the same pattern already established by the sites package
// (normalizeBaseURL, hostLabel, isOfficialProviderBaseURL,
// isCheckinDisabledText).

// LooksLikeModelID mirrors core.looksLikeModelID. Reports whether value
// resembles a known model identifier. Used by SyncChannelModels,
// BuildModelOverview, and the pricing extraction engine to filter noise.
func LooksLikeModelID(value string) bool {
	return looksLikeModelID(value)
}

func looksLikeModelID(value string) bool {
	if value == "" || len(value) > 120 || strings.Contains(value, " ") {
		return false
	}
	lower := strings.ToLower(value)
	prefixes := []string{"gpt-", "claude-", "deepseek", "gemini", "qwen", "glm-", "yi-", "moonshot", "kimi", "doubao", "abab", "llama", "mistral", "mixtral"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return strings.Contains(lower, "-") && (strings.Contains(lower, "chat") || strings.Contains(lower, "turbo") || strings.Contains(lower, "model"))
}

// parseModelIDs mirrors core.parseModelIDs. Extracts model IDs from a
// /v1/models response body. Used by FetchChannelModelsWithKey.
func parseModelIDs(body string) []string {
	var payload interface{}
	if json.Unmarshal([]byte(body), &payload) != nil {
		return nil
	}
	seen := map[string]bool{}
	models := []string{}
	var walk func(interface{})
	walk = func(value interface{}) {
		switch typed := value.(type) {
		case map[string]interface{}:
			if id, ok := typed["id"]; ok {
				text := strings.TrimSpace(fmt.Sprint(id))
				if text != "" && !seen[text] {
					seen[text] = true
					models = append(models, text)
				}
			}
			if name, ok := typed["model"]; ok {
				text := strings.TrimSpace(fmt.Sprint(name))
				if text != "" && !seen[text] && !strings.Contains(text, "map[") {
					seen[text] = true
					models = append(models, text)
				}
			}
			for _, child := range typed {
				walk(child)
			}
		case []interface{}:
			for _, child := range typed {
				walk(child)
			}
		case string:
			text := strings.TrimSpace(typed)
			if looksLikeModelID(text) && !seen[text] {
				seen[text] = true
				models = append(models, text)
			}
		}
	}
	walk(payload)
	return models
}

// limitStrings mirrors core.limitStrings. Returns values truncated to limit.
func limitStrings(values []string, limit int) []string {
	if len(values) <= limit {
		return values
	}
	return append([]string{}, values[:limit]...)
}

// parsePersistedStringSlice mirrors core.parsePersistedStringSlice. Used by
// LoadChannelModelItems, LoadAccountModelRecords, and ListChannels to recover
// sample model slices persisted as JSON arrays.
func parsePersistedStringSlice(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil
	}
	return limitStrings(values, 8)
}

// extractMessage mirrors core.extractMessage. Pulls a human-readable message
// out of an arbitrary JSON body. Used by FetchChannelModelsWithKey and
// SyncSitePricing.
func extractMessage(body string) string {
	var payload interface{}
	if json.Unmarshal([]byte(body), &payload) != nil {
		return ""
	}
	for _, key := range []string{"message", "msg", "error", "detail"} {
		if value := findString(payload, key); value != "" {
			return value
		}
	}
	return ""
}

func findString(value interface{}, wanted string) string {
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, child := range typed {
			if strings.EqualFold(key, wanted) {
				return strings.TrimSpace(fmt.Sprint(child))
			}
		}
		for _, child := range typed {
			if found := findString(child, wanted); found != "" {
				return found
			}
		}
	case []interface{}:
		for _, child := range typed {
			if found := findString(child, wanted); found != "" {
				return found
			}
		}
	}
	return ""
}

// maskResponse mirrors core.maskResponse. Truncates a raw response body for
// safe display/storage. Used by SyncChannelModels and SaveSitePricingCache.
func maskResponse(body string) string {
	trimmed := strings.TrimSpace(body)
	if len(trimmed) > 2000 {
		trimmed = trimmed[:2000] + "...(truncated)"
	}
	return trimmed
}

// firstNonEmpty mirrors core.firstNonEmpty. Returns the first trimmed
// non-empty value. Used pervasively across the channels domain.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// boolInt mirrors core.boolInt. Encodes a bool as 0/1 for SQLite storage.
func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

// normalizeBaseURL mirrors core.normalizeBaseURL and sites.normalizeBaseURL.
// Strips path/query/fragment from raw and trims the trailing slash.
func normalizeBaseURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return strings.TrimRight(raw, "/")
	}
	parsed.Path = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/")
}

// isOfficialProviderBaseURL mirrors core.isOfficialProviderBaseURL and
// sites.isOfficialProviderBaseURL. Reports whether raw points at a known
// official provider. Used by NormalizeOfficialProviderChannel.
func isOfficialProviderBaseURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return false
	}
	officialDomains := []string{
		"api.openai.com",
		"api.anthropic.com",
		"api.mistral.ai",
		"generativelanguage.googleapis.com",
		"aiplatform.googleapis.com",
		"dashscope.aliyuncs.com",
		"open.bigmodel.cn",
		"api.moonshot.cn",
		"api.deepseek.com",
		"api.siliconflow.cn",
		"api.minimax.chat",
		"ark.cn-beijing.volces.com",
		"maas-api.ml-platform-cn-beijing.volces.com",
		"token.sensenova.cn",
	}
	for _, domain := range officialDomains {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	officialSuffixes := []string{
		".sensenova.cn",
		".sensecore.cn",
	}
	for _, suffix := range officialSuffixes {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}
	return false
}

// marshalDetection serializes a channels.Detection to its JSON form. Returns
// "" for a nil detection so callers can store the empty string directly.
// Mirrors core.marshalDetection but operates on the channels.Detection mirror
// so the channels service can persist detection_json without importing core.
func marshalDetection(detection *Detection) string {
	if detection == nil {
		return ""
	}
	payload, err := json.Marshal(detection)
	if err != nil {
		return ""
	}
	return string(payload)
}

// sourceTypeFromChannel mirrors core.sourceTypeFromChannel but operates on
// the channels.ImportedChannel mirror. Classifies a channel's import source
// (manual / legacy / sqlite / admin_api / unknown) from its raw JSON and
// source channel ID.
func sourceTypeFromChannel(channel ImportedChannel) string {
	raw := strings.ToLower(channel.RawJSON)
	sourceID := strings.ToLower(channel.SourceChannelID)
	switch {
	case strings.Contains(raw, `"source":"manual"`) || strings.HasPrefix(sourceID, "manual-"):
		return "manual"
	case strings.Contains(raw, `"source":"legacy"`) || strings.Contains(raw, `"source":"legacy_config"`) || strings.HasPrefix(sourceID, "legacy-"):
		return "legacy"
	case channel.LocalInstanceID != "":
		if strings.Contains(raw, `"source":"admin_api"`) || strings.Contains(raw, `"import_source":"admin_api"`) {
			return "admin_api"
		}
		return "sqlite"
	default:
		return "unknown"
	}
}

// parseJSONLike mirrors core.parseJSONLike. Decodes raw as generic JSON with
// json.Number support so the pricing extraction engine can preserve numeric
// precision.
func parseJSONLike(raw string) (interface{}, bool) {
	var value interface{}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, false
	}
	return value, true
}

// expandJSONStrings mirrors core.expandJSONStrings. Recursively parses
// string-encoded JSON values so the pricing extraction engine can walk a
// unified tree.
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

// appendUniqueString mirrors core.appendUniqueString. Appends value to values
// (case-insensitive dedupe) up to limit entries.
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

// normalizedRandomDelay mirrors core.normalizedRandomDelay. Clamps the
// randomDelayMinutes slice to a (min, max) pair. Used by
// EnsureGlobalScheduleRecord and SyncGlobalScheduleRecord.
func normalizedRandomDelay(values []int) (int, int) {
	minDelay := 0
	maxDelay := 0
	if len(values) >= 2 {
		minDelay = values[0]
		maxDelay = values[1]
	}
	if minDelay < 0 {
		minDelay = 0
	}
	if maxDelay < minDelay {
		maxDelay = minDelay
	}
	return minDelay, maxDelay
}
