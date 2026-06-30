package accounts

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// This file hosts pure helper functions duplicated from core so the accounts
// package can build without importing core. The originals live in
// core/{crypto,filters,sites,scanner,checkin_balance,scheduler,http}.go and
// must stay there because core still uses them; these copies must stay in
// sync.

// boolInt converts a bool to its SQLite integer representation.
func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

// maskSecret masks all but the last 4 characters of a secret.
func maskSecret(value string) string {
	if value == "" {
		return ""
	}
	if len(value) <= 4 {
		return strings.Repeat("*", len(value))
	}
	return strings.Repeat("*", max(4, len(value)-4)) + value[len(value)-4:]
}

// firstNonEmpty returns the first argument whose trimmed value is non-empty.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// mustJSON encodes value, returning "" on error.
func mustJSON(value interface{}) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

// errorsText wraps a plain message as an error.
func errorsText(message string) error {
	return fmt.Errorf("%s", message)
}

// normalizeBaseURL strips path/query/fragment from raw and trims trailing "/".
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

// hostLabel extracts the host portion of raw for display.
func hostLabel(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	return parsed.Host
}

// isHTTPURL reports whether value starts with http:// or https://.
func isHTTPURL(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(normalized, "http://") || strings.HasPrefix(normalized, "https://")
}

// pathFromMaybeURL returns the path (with optional query) of a URL-like string.
func pathFromMaybeURL(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "/") {
		return trimmed
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	if parsed.RawQuery != "" {
		return parsed.Path + "?" + parsed.RawQuery
	}
	return parsed.Path
}

// clampInt clamps value into [min, max] with a fallback.
func clampInt(value int, min int, max int, fallback int) int {
	if value < min || value > max {
		return fallback
	}
	return value
}

// clampBatchLimit clamps a batch limit to [1, fallback].
func clampBatchLimit(value int, fallback int) int {
	if value <= 0 {
		return fallback
	}
	if value > fallback {
		return fallback
	}
	return value
}

// intFromResult reads an int from a map[string]interface{} result.
func intFromResult(result map[string]interface{}, key string) int {
	switch value := result[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case json.Number:
		parsed, _ := value.Int64()
		return int(parsed)
	default:
		return 0
	}
}

// stringFromResult reads a string from a map[string]interface{} result.
func stringFromResult(result map[string]interface{}, key string) string {
	if value, ok := result[key].(string); ok {
		return value
	}
	return ""
}

// boolFromResult reads a bool from a map[string]interface{} result.
func boolFromResult(result map[string]interface{}, key string) bool {
	if value, ok := result[key].(bool); ok {
		return value
	}
	return false
}

// stringValue reads a string field from a record map.
func stringValue(record map[string]interface{}, key string) string {
	value, ok := record[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

// extractImportedBaseURL pulls base_url/baseUrl/url from a record or its
// nested config JSON.
func extractImportedBaseURL(record map[string]interface{}) string {
	for _, key := range []string{"base_url", "baseUrl", "url"} {
		if value := stringValue(record, key); value != "" {
			return strings.TrimRight(value, "/")
		}
	}
	config := stringValue(record, "config")
	if config != "" {
		var parsed map[string]interface{}
		if json.Unmarshal([]byte(config), &parsed) == nil {
			for _, key := range []string{"base_url", "baseUrl", "url"} {
				if value := stringValue(parsed, key); value != "" {
					return strings.TrimRight(value, "/")
				}
			}
		}
	}
	return ""
}

// inferImportedKind guesses the relay kind from a record + baseURL.
func inferImportedKind(record map[string]interface{}, baseURL string) string {
	combined := strings.ToLower(fmt.Sprint(record) + " " + baseURL)
	switch {
	case strings.Contains(combined, "sub2api"):
		return "sub2api"
	case strings.Contains(combined, "oneapi") || strings.Contains(combined, "one api"):
		return "oneapi"
	case strings.Contains(combined, "newapi") || strings.Contains(combined, "new api"):
		return "newapi"
	case strings.Contains(combined, "openai"):
		return "openai_compatible"
	default:
		return "unknown"
	}
}

// normalizeDBValue converts []byte to string for SQLite scan destinations.
func normalizeDBValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case nil:
		return nil
	case []byte:
		return string(typed)
	default:
		return typed
	}
}

// marshalImportedRecord masks sensitive fields in a record and tags it with
// an import source.
func marshalImportedRecord(record map[string]interface{}, importSource string) (string, error) {
	safeRecord := map[string]interface{}{}
	for key, value := range record {
		safeRecord[key] = maskSensitiveImportedValue(key, value)
	}
	if importSource != "" {
		safeRecord["import_source"] = importSource
	}
	rawJSON, err := json.Marshal(safeRecord)
	if err != nil {
		return "", err
	}
	return string(rawJSON), nil
}

// maskSensitiveImportedValue masks sensitive top-level fields and recurses
// into JSON-string values.
func maskSensitiveImportedValue(key string, value interface{}) interface{} {
	if value == nil {
		return nil
	}
	if isSensitiveImportedField(key) {
		return maskSecret(fmt.Sprint(value))
	}
	text, ok := value.(string)
	if !ok {
		return value
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || !(strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")) {
		return value
	}
	var parsed interface{}
	if json.Unmarshal([]byte(trimmed), &parsed) != nil {
		return value
	}
	masked := maskSensitiveJSONValue(parsed)
	next, err := json.Marshal(masked)
	if err != nil {
		return value
	}
	return string(next)
}

// maskSensitiveJSONValue recurses into parsed JSON masking sensitive keys.
func maskSensitiveJSONValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		next := map[string]interface{}{}
		for key, child := range typed {
			if isSensitiveImportedField(key) {
				next[key] = maskSecret(fmt.Sprint(child))
			} else {
				next[key] = maskSensitiveJSONValue(child)
			}
		}
		return next
	case []interface{}:
		next := make([]interface{}, 0, len(typed))
		for _, child := range typed {
			next = append(next, maskSensitiveJSONValue(child))
		}
		return next
	default:
		return value
	}
}

// isSensitiveImportedField reports whether key names a secret field.
func isSensitiveImportedField(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(key), "-", "_"))
	switch normalized {
	case "key", "api_key", "apikey", "access_key", "secret", "password", "cookie", "authorization", "bearer", "token", "access_token", "refresh_token":
		return true
	default:
		return false
	}
}

// hostnameForMatch extracts the lowercased host (without www.) for matching.
func hostnameForMatch(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" {
		parsed, err = url.Parse("https://" + strings.TrimSpace(raw))
		if err != nil {
			return ""
		}
	}
	host := strings.ToLower(parsed.Hostname())
	host = strings.TrimPrefix(host, "www.")
	return host
}

// hostsMatch reports whether two hosts are equal or subdomains of each other.
func hostsMatch(left string, right string) bool {
	if left == "" || right == "" {
		return false
	}
	return left == right || strings.HasSuffix(left, "."+right) || strings.HasSuffix(right, "."+left)
}

// countUniqueMatchedSites counts distinct SiteIDs in a match list.
func countUniqueMatchedSites(matches []chromePasswordMatch) int {
	seen := map[string]bool{}
	for _, match := range matches {
		seen[match.SiteID] = true
	}
	return len(seen)
}

// findChromeRow returns the first row matching targetURL + username.
func findChromeRow(rows []chromePasswordRow, targetURL string, username string) chromePasswordRow {
	for _, row := range rows {
		if row.URL == targetURL && row.Username == username {
			return row
		}
	}
	return chromePasswordRow{}
}

// sourceIDSetFromRecords builds a set of source IDs from a record list.
func sourceIDSetFromRecords(records []map[string]interface{}) map[string]bool {
	seen := map[string]bool{}
	for index, record := range records {
		sourceID := stringValue(record, "id")
		if sourceID == "" {
			sourceID = fmt.Sprintf("row-%d", index+1)
		}
		seen[sourceID] = true
	}
	return seen
}

// prepareSyncRecord normalises a raw record into a preparedSyncRecord.
func prepareSyncRecord(record map[string]interface{}, importSource string, index int) preparedSyncRecord {
	sourceID := stringValue(record, "id")
	if sourceID == "" {
		sourceID = fmt.Sprintf("row-%d", index+1)
	}
	name := stringValue(record, "name")
	if name == "" {
		name = "渠道 " + sourceID
	}
	baseURL := extractImportedBaseURL(record)
	keyMasked := ""
	if keyValue := stringValue(record, "key"); keyValue != "" {
		keyMasked = maskSecret(keyValue)
	}
	rawJSON, _ := marshalImportedRecord(record, importSource)
	return preparedSyncRecord{
		SourceID:  sourceID,
		Name:      name,
		BaseURL:   baseURL,
		Status:    stringValue(record, "status"),
		Kind:      inferImportedKind(record, baseURL),
		RawJSON:   rawJSON,
		KeyMasked: keyMasked,
	}
}

// compareImportedChannelFields returns the human-readable field names that
// differ between current and next.
func compareImportedChannelFields(current existingImportedChannel, next preparedSyncRecord) []string {
	fields := []string{}
	if current.Name != next.Name {
		fields = append(fields, "名称")
	}
	if strings.TrimRight(current.BaseURL, "/") != strings.TrimRight(next.BaseURL, "/") {
		fields = append(fields, "API 地址")
	}
	if current.Status != next.Status {
		fields = append(fields, "状态")
	}
	if current.Kind != next.Kind {
		fields = append(fields, "识别类型")
	}
	if next.KeyMasked != "" && current.KeyMasked != "" && current.KeyMasked != next.KeyMasked {
		fields = append(fields, "渠道 Key")
	}
	if current.RawJSON != "" && next.RawJSON != "" && current.RawJSON != next.RawJSON {
		fields = append(fields, "原始配置")
	}
	return fields
}

// excludedRelaySiteTokens lists site name/URL fragments that are blocked
// from import. Mirrors core.excludedRelaySiteTokens.
var excludedRelaySiteTokens = []string{
	"9router",
	"freemodel",
	"free model",
	"tokenrouter",
	"token router",
}

// isExcludedRelaySite reports whether name/baseURL matches an excluded token.
func isExcludedRelaySite(name string, baseURL string) bool {
	_, matched := excludedRelaySiteMatch(name, baseURL)
	return matched
}

// excludedRelaySiteMatch returns the matched token (if any) for name/baseURL.
func excludedRelaySiteMatch(name string, baseURL string) (string, bool) {
	combined := strings.ToLower(strings.TrimSpace(name) + " " + strings.TrimSpace(baseURL))
	for _, token := range excludedRelaySiteTokens {
		if strings.Contains(combined, token) {
			return token, true
		}
	}
	return "", false
}

// isManagedRelayKind reports whether kind is a managed relay panel type.
func isManagedRelayKind(kind string) bool {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "newapi", "oneapi", "sub2api", "modified_relay":
		return true
	default:
		return false
	}
}

// syncCapability returns the sync capability label for an instance.
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

// baseURLFromDBPath maps common DB paths to their likely NewAPI base URL.
func baseURLFromDBPath(dbPath string) string {
	normalized := strings.ToLower(dbPath)
	if strings.Contains(normalized, "newapi") || strings.Contains(normalized, "new-api") || strings.Contains(normalized, "one-api") {
		return "http://127.0.0.1:3000"
	}
	return ""
}

// normalizeSyncSourceInput applies defaults to a SyncSourceInput.
func normalizeSyncSourceInput(input *SyncSourceInput) {
	if strings.TrimSpace(input.UserID) == "" {
		input.UserID = "1"
	}
	input.PageSize = clampInt(input.PageSize, 10, 100, 100)
}
