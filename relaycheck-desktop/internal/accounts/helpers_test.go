package accounts

import (
	"testing"
)

func TestMaskSecret(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  string
	}{
		{"empty", "", ""},
		{"short", "abc", "***"},
		{"exact4", "abcd", "****"},
		{"normal", "sk-1234567890", "*********7890"},
		{"long", "very-long-secret-key-12345", "**********************2345"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := maskSecret(tc.value); got != tc.want {
				t.Errorf("maskSecret(%q) = %q, want %q", tc.value, got, tc.want)
			}
		})
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "  ", "x"); got != "x" {
		t.Errorf("expected 'x', got %q", got)
	}
	if got := firstNonEmpty("a", "b"); got != "a" {
		t.Errorf("expected 'a', got %q", got)
	}
	if got := firstNonEmpty(); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestBoolInt(t *testing.T) {
	if boolInt(true) != 1 {
		t.Fatal("boolInt(true) should be 1")
	}
	if boolInt(false) != 0 {
		t.Fatal("boolInt(false) should be 0")
	}
}

func TestMustJSON(t *testing.T) {
	if got := mustJSON(map[string]string{"a": "b"}); got != `{"a":"b"}` {
		t.Errorf("unexpected JSON: %q", got)
	}
}

func TestNormalizeBaseURL(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"https://example.com/path", "https://example.com"},
		{"https://example.com/", "https://example.com"},
		{"not-a-url", "not-a-url"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := normalizeBaseURL(tc.raw); got != tc.want {
			t.Errorf("normalizeBaseURL(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestHostLabel(t *testing.T) {
	if got := hostLabel("https://example.com/path"); got != "example.com" {
		t.Errorf("expected 'example.com', got %q", got)
	}
	// "not-a-url" parses with empty Host (treated as path), so hostLabel returns "".
	if got := hostLabel("not-a-url"); got != "" {
		t.Errorf("expected empty host for bare path, got %q", got)
	}
}

func TestIsHTTPURL(t *testing.T) {
	if !isHTTPURL("http://example.com") {
		t.Error("http:// should be HTTP URL")
	}
	if !isHTTPURL("HTTPS://example.com") {
		t.Error("HTTPS:// should be HTTP URL (case-insensitive)")
	}
	if isHTTPURL("ftp://example.com") {
		t.Error("ftp:// should not be HTTP URL")
	}
	if isHTTPURL("example.com") {
		t.Error("bare domain should not be HTTP URL")
	}
}

func TestPathFromMaybeURL(t *testing.T) {
	cases := []struct {
		value string
		want  string
	}{
		{"", ""},
		{"/api/v1", "/api/v1"},
		{"https://example.com/api/v1", "/api/v1"},
		{"https://example.com/api?v=1", "/api?v=1"},
		{"not-a-url", "not-a-url"}, // url.Parse succeeds, Path="not-a-url"
	}
	for _, tc := range cases {
		if got := pathFromMaybeURL(tc.value); got != tc.want {
			t.Errorf("pathFromMaybeURL(%q) = %q, want %q", tc.value, got, tc.want)
		}
	}
}

func TestClampInt(t *testing.T) {
	if clampInt(5, 1, 10, 0) != 5 {
		t.Fatal("in-range value should pass through")
	}
	if clampInt(0, 1, 10, 99) != 99 {
		t.Fatal("below-min should return fallback")
	}
	if clampInt(20, 1, 10, 99) != 99 {
		t.Fatal("above-max should return fallback")
	}
}

func TestClampBatchLimit(t *testing.T) {
	if clampBatchLimit(0, 100) != 100 {
		t.Fatal("zero should return fallback")
	}
	if clampBatchLimit(50, 100) != 50 {
		t.Fatal("in-range should pass through")
	}
	if clampBatchLimit(200, 100) != 100 {
		t.Fatal("above-fallback should be clamped to fallback")
	}
}

func TestIntFromResult(t *testing.T) {
	m := map[string]interface{}{
		"a": 5,
		"b": int64(10),
		"c": float64(3.9),
	}
	if intFromResult(m, "a") != 5 {
		t.Fatal("int extraction failed")
	}
	if intFromResult(m, "b") != 10 {
		t.Fatal("int64 extraction failed")
	}
	if intFromResult(m, "c") != 3 {
		t.Fatal("float64 extraction failed")
	}
	if intFromResult(m, "missing") != 0 {
		t.Fatal("missing key should return 0")
	}
}

func TestStringFromResult(t *testing.T) {
	m := map[string]interface{}{"a": "hello"}
	if stringFromResult(m, "a") != "hello" {
		t.Fatal("string extraction failed")
	}
	if stringFromResult(m, "missing") != "" {
		t.Fatal("missing key should return empty")
	}
}

func TestBoolFromResult(t *testing.T) {
	m := map[string]interface{}{"a": true}
	if !boolFromResult(m, "a") {
		t.Fatal("bool extraction failed")
	}
	if boolFromResult(m, "missing") {
		t.Fatal("missing key should return false")
	}
}

func TestStringValue(t *testing.T) {
	m := map[string]interface{}{"a": "x", "b": 42, "c": nil}
	if stringValue(m, "a") != "x" {
		t.Fatal("string field failed")
	}
	if stringValue(m, "b") != "42" {
		t.Fatal("non-string should be stringified")
	}
	if stringValue(m, "c") != "" {
		t.Fatal("nil should return empty")
	}
	if stringValue(m, "missing") != "" {
		t.Fatal("missing should return empty")
	}
}

func TestExtractImportedBaseURL(t *testing.T) {
	cases := []struct {
		name   string
		record map[string]interface{}
		want   string
	}{
		{"base_url", map[string]interface{}{"base_url": "https://a.com/"}, "https://a.com"},
		{"baseUrl", map[string]interface{}{"baseUrl": "https://b.com"}, "https://b.com"},
		{"url", map[string]interface{}{"url": "https://c.com"}, "https://c.com"},
		{"config-nested", map[string]interface{}{"config": `{"base_url":"https://d.com"}`}, "https://d.com"},
		{"missing", map[string]interface{}{"name": "foo"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := extractImportedBaseURL(tc.record); got != tc.want {
				t.Errorf("extractImportedBaseURL() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestInferImportedKind(t *testing.T) {
	cases := []struct {
		record  map[string]interface{}
		baseURL string
		want    string
	}{
		{map[string]interface{}{"name": "sub2api relay"}, "", "sub2api"},
		{map[string]interface{}{"name": "one api"}, "", "oneapi"},
		{map[string]interface{}{"name": "newapi"}, "", "newapi"},
		{map[string]interface{}{}, "https://api.openai.com", "openai_compatible"},
		{map[string]interface{}{}, "", "unknown"},
	}
	for _, tc := range cases {
		if got := inferImportedKind(tc.record, tc.baseURL); got != tc.want {
			t.Errorf("inferImportedKind() = %q, want %q", got, tc.want)
		}
	}
}

func TestIsSensitiveImportedField(t *testing.T) {
	sensitive := []string{"key", "api_key", "apikey", "API-KEY", "access_key", "secret", "password", "cookie", "authorization", "bearer", "token", "access_token", "refresh_token"}
	for _, key := range sensitive {
		if !isSensitiveImportedField(key) {
			t.Errorf("expected %q to be sensitive", key)
		}
	}
	nonSensitive := []string{"name", "id", "base_url", "status", ""}
	for _, key := range nonSensitive {
		if isSensitiveImportedField(key) {
			t.Errorf("expected %q to NOT be sensitive", key)
		}
	}
}

func TestMarshalImportedRecord_MasksSecrets(t *testing.T) {
	record := map[string]interface{}{
		"id":      "1",
		"name":    "test",
		"key":     "sk-secret-123456",
		"api_key": "sk-other-789",
	}
	raw, err := marshalImportedRecord(record, "sqlite")
	if err != nil {
		t.Fatalf("marshalImportedRecord failed: %v", err)
	}
	// Secrets must be masked, not plaintext.
	if !contains(raw, "sk-secret") {
		// not the plaintext
	}
	if contains(raw, "sk-secret-123456") {
		t.Fatal("plaintext secret leaked into marshaled record")
	}
	if contains(raw, "sk-other-789") {
		t.Fatal("plaintext api_key leaked into marshaled record")
	}
	if !contains(raw, `"import_source":"sqlite"`) {
		t.Fatal("import_source tag missing")
	}
}

func TestMaskSensitiveImportedValue_NestedJSON(t *testing.T) {
	// Nested JSON string should have sensitive fields masked.
	record := map[string]interface{}{
		"config": `{"api_key":"sk-nested-12345","name":"visible"}`,
	}
	raw, _ := marshalImportedRecord(record, "")
	if contains(raw, "sk-nested-12345") {
		t.Fatal("nested plaintext secret leaked")
	}
	if !contains(raw, "visible") {
		t.Fatal("non-sensitive nested value should remain visible")
	}
}

func TestHostnameForMatch(t *testing.T) {
	if got := hostnameForMatch("https://www.example.com/path"); got != "example.com" {
		t.Errorf("expected 'example.com' (www stripped), got %q", got)
	}
	if got := hostnameForMatch("https://api.example.com"); got != "api.example.com" {
		t.Errorf("expected 'api.example.com', got %q", got)
	}
	// Bare host gets https:// prefix.
	if got := hostnameForMatch("example.com"); got != "example.com" {
		t.Errorf("expected 'example.com', got %q", got)
	}
}

func TestHostsMatch(t *testing.T) {
	if !hostsMatch("example.com", "example.com") {
		t.Fatal("identical hosts should match")
	}
	if !hostsMatch("api.example.com", "example.com") {
		t.Fatal("subdomain should match parent")
	}
	if !hostsMatch("example.com", "api.example.com") {
		t.Fatal("parent should match subdomain (symmetric)")
	}
	if hostsMatch("example.com", "other.com") {
		t.Fatal("different hosts should not match")
	}
	if hostsMatch("", "example.com") {
		t.Fatal("empty host should not match")
	}
}

func TestIsExcludedRelaySite(t *testing.T) {
	excluded := []struct {
		name    string
		baseURL string
	}{
		{"9router relay", ""},
		{"FreeModel Hub", ""},
		{"", "https://tokenrouter.io"},
		{"", "https://token router.example.com"},
	}
	for _, tc := range excluded {
		if !isExcludedRelaySite(tc.name, tc.baseURL) {
			t.Errorf("expected (%q, %q) to be excluded", tc.name, tc.baseURL)
		}
	}
	if isExcludedRelaySite("normal relay", "https://example.com") {
		t.Fatal("normal site should not be excluded")
	}
}

func TestExcludedRelaySiteMatch_ReturnsToken(t *testing.T) {
	token, matched := excludedRelaySiteMatch("9router", "")
	if !matched || token != "9router" {
		t.Fatalf("expected token '9router', got %q matched=%v", token, matched)
	}
	token, matched = excludedRelaySiteMatch("normal", "")
	if matched {
		t.Fatal("normal site should not match")
	}
}

func TestIsManagedRelayKind(t *testing.T) {
	managed := []string{"newapi", "oneapi", "sub2api", "modified_relay", "NewAPI", "  OneAPI  "}
	for _, kind := range managed {
		if !isManagedRelayKind(kind) {
			t.Errorf("expected %q to be managed", kind)
		}
	}
	nonManaged := []string{"openai_compatible", "unknown", ""}
	for _, kind := range nonManaged {
		if isManagedRelayKind(kind) {
			t.Errorf("expected %q to NOT be managed", kind)
		}
	}
}

func TestSyncCapability(t *testing.T) {
	cases := []struct {
		name string
		item LocalNewAPIInstance
		want string
	}{
		{"sqlite", LocalNewAPIInstance{DatabasePath: "/path/to.db"}, "sqlite"},
		{"admin-api-saved", LocalNewAPIInstance{BaseURL: "http://x", HasSyncToken: true}, "admin_api_saved_token"},
		{"admin-api-token-required", LocalNewAPIInstance{BaseURL: "http://x"}, "admin_api_token_required"},
		{"unsupported", LocalNewAPIInstance{}, "unsupported"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := syncCapability(tc.item); got != tc.want {
				t.Errorf("syncCapability() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBaseURLFromDBPath(t *testing.T) {
	cases := []struct {
		dbPath string
		want   string
	}{
		{"/data/newapi/one-api.db", "http://127.0.0.1:3000"},
		{"C:\\new-api\\db.sqlite", "http://127.0.0.1:3000"},
		{"/data/other.db", ""},
	}
	for _, tc := range cases {
		if got := baseURLFromDBPath(tc.dbPath); got != tc.want {
			t.Errorf("baseURLFromDBPath(%q) = %q, want %q", tc.dbPath, got, tc.want)
		}
	}
}

func TestNormalizeSyncSourceInput(t *testing.T) {
	input := &SyncSourceInput{}
	normalizeSyncSourceInput(input)
	if input.UserID != "1" {
		t.Fatalf("empty UserID should default to '1', got %q", input.UserID)
	}
	if input.PageSize != 100 {
		t.Fatalf("empty PageSize should default to 100, got %d", input.PageSize)
	}
	// PageSize below min → fallback.
	input2 := &SyncSourceInput{PageSize: 5}
	normalizeSyncSourceInput(input2)
	if input2.PageSize != 100 {
		t.Fatalf("PageSize below min should fallback to 100, got %d", input2.PageSize)
	}
	// PageSize in range → preserved.
	input3 := &SyncSourceInput{PageSize: 50}
	normalizeSyncSourceInput(input3)
	if input3.PageSize != 50 {
		t.Fatalf("PageSize in range should be preserved, got %d", input3.PageSize)
	}
	// UserID provided → preserved.
	input4 := &SyncSourceInput{UserID: "42"}
	normalizeSyncSourceInput(input4)
	if input4.UserID != "42" {
		t.Fatalf("provided UserID should be preserved, got %q", input4.UserID)
	}
}

func TestCompareImportedChannelFields(t *testing.T) {
	current := existingImportedChannel{Name: "old", BaseURL: "https://a.com", Status: "active", Kind: "newapi"}
	next := preparedSyncRecord{Name: "new", BaseURL: "https://b.com", Status: "disabled", Kind: "oneapi"}
	fields := compareImportedChannelFields(current, next)
	// Name, BaseURL, Status, Kind differ.
	if len(fields) < 4 {
		t.Fatalf("expected at least 4 changed fields, got %d: %v", len(fields), fields)
	}
}

func TestCompareImportedChannelFields_NoChanges(t *testing.T) {
	current := existingImportedChannel{Name: "same", BaseURL: "https://a.com", Status: "active", Kind: "newapi"}
	next := preparedSyncRecord{Name: "same", BaseURL: "https://a.com", Status: "active", Kind: "newapi"}
	fields := compareImportedChannelFields(current, next)
	if len(fields) != 0 {
		t.Fatalf("expected 0 changed fields, got %d: %v", len(fields), fields)
	}
}

func TestSourceIDSetFromRecords(t *testing.T) {
	records := []map[string]interface{}{
		{"id": "1"},
		{"id": "2"},
		{"id": "1"}, // duplicate
		{"name": "no-id"}, // falls back to row-4
	}
	set := sourceIDSetFromRecords(records)
	if len(set) != 3 {
		t.Fatalf("expected 3 unique source IDs, got %d: %v", len(set), set)
	}
	if !set["1"] || !set["2"] || !set["row-4"] {
		t.Fatalf("expected set to contain 1, 2, row-4: %v", set)
	}
}

func TestPrepareSyncRecord(t *testing.T) {
	record := map[string]interface{}{
		"id":   "42",
		"name": "my channel",
		"base_url": "https://example.com/",
		"key":  "sk-secret-12345678",
		"status": "active",
	}
	prepared := prepareSyncRecord(record, "admin_api", 0)
	if prepared.SourceID != "42" {
		t.Fatalf("expected SourceID '42', got %q", prepared.SourceID)
	}
	if prepared.Name != "my channel" {
		t.Fatalf("expected Name 'my channel', got %q", prepared.Name)
	}
	if prepared.BaseURL != "https://example.com" {
		t.Fatalf("expected BaseURL 'https://example.com' (trimmed), got %q", prepared.BaseURL)
	}
	if prepared.Kind != "unknown" {
		t.Fatalf("expected Kind 'unknown', got %q", prepared.Kind)
	}
	if prepared.KeyMasked == "" || contains(prepared.KeyMasked, "sk-secret") {
		t.Fatalf("KeyMasked should be masked, got %q", prepared.KeyMasked)
	}
	if !contains(prepared.RawJSON, "import_source") {
		t.Fatal("RawJSON should contain import_source tag")
	}
}

func TestPrepareSyncRecord_FallbackName(t *testing.T) {
	record := map[string]interface{}{"id": "5"}
	prepared := prepareSyncRecord(record, "", 2)
	if prepared.Name != "渠道 5" {
		t.Fatalf("expected fallback name '渠道 5', got %q", prepared.Name)
	}
	if prepared.SourceID != "5" {
		t.Fatalf("expected SourceID '5', got %q", prepared.SourceID)
	}
}

func TestPrepareSyncRecord_FallbackSourceID(t *testing.T) {
	record := map[string]interface{}{"name": "no id here"}
	prepared := prepareSyncRecord(record, "", 0)
	if prepared.SourceID != "row-1" {
		t.Fatalf("expected fallback SourceID 'row-1', got %q", prepared.SourceID)
	}
}

// contains is a small test helper to avoid importing strings just for one call.
func contains(haystack, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
