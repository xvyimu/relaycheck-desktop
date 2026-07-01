package channels

import (
	"encoding/json"
	"testing"
)

func TestLooksLikeModelID(t *testing.T) {
	cases := []struct {
		value string
		want  bool
	}{
		{"", false},
		{"gpt-4", true},
		{"claude-3-opus", true},
		{"deepseek-chat", true},
		{"gemini-pro", true},
		{"qwen-turbo", true},
		{"glm-4", true},
		{"yi-34b", true},
		{"moonshot-v1", true},
		{"kimi-latest", true},
		{"doubao-pro", true},
		{"abab-6", true},
		{"llama-3", true},
		{"mistral-7b", true},
		{"mixtral-8x7b", true},
		{"something-chat", true},
		{"foo-turbo", true},
		{"foo-model", true},
		{"random-string", false}, // has "-" but no chat/turbo/model keyword
		{"random", false},        // no prefix, no dash+keyword
		{"has space", false},
		{string([]byte{0x68, 0x69}), false}, // "hi" no prefix
	}

	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			if got := looksLikeModelID(tc.value); got != tc.want {
				t.Errorf("looksLikeModelID(%q) = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}

func TestLooksLikeModelID_TooLong(t *testing.T) {
	long := ""
	for i := 0; i < 121; i++ {
		long += "a"
	}
	if looksLikeModelID(long) {
		t.Fatal("120+ char string should not be a model ID")
	}
}

func TestParseModelIDs(t *testing.T) {
	body := `{"data":[{"id":"gpt-4"},{"id":"claude-3"},{"model":"gpt-4o"}],"object":"list"}`
	models := parseModelIDs(body)
	if len(models) != 3 {
		t.Fatalf("expected 3 models, got %d: %v", len(models), models)
	}
	// Dedup: gpt-4 appears twice (id + model), should only count once.
	body2 := `{"data":[{"id":"gpt-4"},{"model":"gpt-4"}]}`
	models2 := parseModelIDs(body2)
	if len(models2) != 1 {
		t.Fatalf("expected 1 deduped model, got %d: %v", len(models2), models2)
	}
}

func TestParseModelIDs_InvalidJSON(t *testing.T) {
	if got := parseModelIDs("not json"); got != nil {
		t.Fatalf("expected nil for invalid JSON, got %v", got)
	}
}

func TestParseModelIDs_StringRoot(t *testing.T) {
	// String root: "gpt-4" looksLikeModelID → included.
	models := parseModelIDs(`"gpt-4"`)
	if len(models) != 1 || models[0] != "gpt-4" {
		t.Fatalf("expected [gpt-4], got %v", models)
	}
	// String root: "random" does not lookLikeModelID → excluded.
	models = parseModelIDs(`"random"`)
	if len(models) != 0 {
		t.Fatalf("expected empty for non-model string, got %v", models)
	}
}

func TestLimitStrings(t *testing.T) {
	values := []string{"a", "b", "c", "d", "e"}
	if got := limitStrings(values, 3); len(got) != 3 {
		t.Fatalf("expected 3 items, got %d", len(got))
	}
	if got := limitStrings(values, 10); len(got) != 5 {
		t.Fatalf("expected 5 items (all), got %d", len(got))
	}
	// Ensure returned slice doesn't alias original.
	got := limitStrings(values, 2)
	got[0] = "x"
	if values[0] == "x" {
		t.Fatal("limitStrings should return a copy, not alias the original")
	}
}

func TestParsePersistedStringSlice(t *testing.T) {
	if got := parsePersistedStringSlice(""); got != nil {
		t.Fatalf("expected nil for empty, got %v", got)
	}
	if got := parsePersistedStringSlice("  "); got != nil {
		t.Fatalf("expected nil for whitespace, got %v", got)
	}
	if got := parsePersistedStringSlice("invalid"); got != nil {
		t.Fatalf("expected nil for invalid JSON, got %v", got)
	}
	got := parsePersistedStringSlice(`["a","b","c"]`)
	if len(got) != 3 || got[0] != "a" {
		t.Fatalf("expected 3 items, got %v", got)
	}
	// Limit to 8.
	long := `["a","b","c","d","e","f","g","h","i","j"]`
	got = parsePersistedStringSlice(long)
	if len(got) != 8 {
		t.Fatalf("expected 8 items (limit), got %d", len(got))
	}
}

func TestExtractMessage(t *testing.T) {
	cases := []struct {
		body string
		want string
	}{
		{`{"message":"hello"}`, "hello"},
		{`{"msg":"hi"}`, "hi"},
		{`{"error":"bad"}`, "bad"},
		{`{"detail":"info"}`, "info"},
		{`{"data":{"message":"nested"}}`, "nested"},
		{`{"items":[{"error":"deep"}]}`, "deep"},
		{`{"nothing":"here"}`, ""},
		{`invalid`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := extractMessage(tc.body); got != tc.want {
				t.Errorf("extractMessage(%q) = %q, want %q", tc.body, got, tc.want)
			}
		})
	}
}

func TestMaskResponse(t *testing.T) {
	short := "hello"
	if got := maskResponse(short); got != short {
		t.Fatalf("short response should be unchanged, got %q", got)
	}
	long := ""
	for i := 0; i < 2500; i++ {
		long += "x"
	}
	got := maskResponse(long)
	if len(got) >= len(long) {
		t.Fatalf("long response should be truncated, got len %d", len(got))
	}
	if got == "" {
		t.Fatal("masked response should not be empty")
	}
}

func TestFirstNonEmpty(t *testing.T) {
	cases := []struct {
		values []string
		want   string
	}{
		{[]string{"", "", "c"}, "c"},
		{[]string{"  ", "x"}, "x"},
		{[]string{"a", "b"}, "a"},
		{[]string{}, ""},
		{[]string{"", ""}, ""},
	}
	for _, tc := range cases {
		if got := firstNonEmpty(tc.values...); got != tc.want {
			t.Errorf("firstNonEmpty(%v) = %q, want %q", tc.values, got, tc.want)
		}
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

func TestNormalizeBaseURL(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"https://example.com/path", "https://example.com"},
		{"https://example.com/path?q=1", "https://example.com"},
		{"https://example.com/", "https://example.com"},
		{"https://example.com", "https://example.com"},
		{"  https://example.com/  ", "https://example.com"},
		{"not-a-url", "not-a-url"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := normalizeBaseURL(tc.raw); got != tc.want {
			t.Errorf("normalizeBaseURL(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestIsOfficialProviderBaseURL(t *testing.T) {
	official := []string{
		"https://api.openai.com/v1",
		"https://api.anthropic.com",
		"https://api.deepseek.com",
		"https://dashscope.aliyuncs.com",
		"https://sub.api.openai.com",
	}
	for _, raw := range official {
		if !isOfficialProviderBaseURL(raw) {
			t.Errorf("expected %q to be official", raw)
		}
	}
	nonOfficial := []string{
		"https://example.com",
		"localhost",
		"",
		"https://my-relay.com",
	}
	for _, raw := range nonOfficial {
		if isOfficialProviderBaseURL(raw) {
			t.Errorf("expected %q to NOT be official", raw)
		}
	}
}

func TestMarshalDetection(t *testing.T) {
	if got := marshalDetection(nil); got != "" {
		t.Fatalf("nil detection should produce empty string, got %q", got)
	}
	d := &Detection{BaseURL: "https://example.com", Kind: "newapi"}
	got := marshalDetection(d)
	if got == "" {
		t.Fatal("non-nil detection should produce JSON")
	}
	var parsed Detection
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("marshalDetection output is invalid JSON: %v", err)
	}
	if parsed.BaseURL != "https://example.com" || parsed.Kind != "newapi" {
		t.Fatalf("round-trip mismatch: %+v", parsed)
	}
}

func TestSourceTypeFromChannel(t *testing.T) {
	cases := []struct {
		name    string
		channel ImportedChannel
		want    string
	}{
		{"manual-raw", ImportedChannel{RawJSON: `{"source":"manual"}`, SourceChannelID: "manual-1"}, "manual"},
		{"manual-id", ImportedChannel{RawJSON: `{}`, SourceChannelID: "manual-abc"}, "manual"},
		{"legacy-raw", ImportedChannel{RawJSON: `{"source":"legacy"}`, SourceChannelID: "x"}, "legacy"},
		{"legacy-config", ImportedChannel{RawJSON: `{"source":"legacy_config"}`, SourceChannelID: "x"}, "legacy"},
		{"legacy-id", ImportedChannel{RawJSON: `{}`, SourceChannelID: "legacy-5"}, "legacy"},
		{"admin-api", ImportedChannel{LocalInstanceID: "1", RawJSON: `{"source":"admin_api"}`}, "admin_api"},
		{"admin-api-import", ImportedChannel{LocalInstanceID: "1", RawJSON: `{"import_source":"admin_api"}`}, "admin_api"},
		{"sqlite-default", ImportedChannel{LocalInstanceID: "1", RawJSON: `{}`}, "sqlite"},
		{"unknown", ImportedChannel{RawJSON: `{}`}, "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sourceTypeFromChannel(tc.channel); got != tc.want {
				t.Errorf("sourceTypeFromChannel() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseJSONLike(t *testing.T) {
	value, ok := parseJSONLike(`{"a":1}`)
	if !ok {
		t.Fatal("expected ok for valid JSON")
	}
	m, ok := value.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", value)
	}
	// json.Number preserved.
	if _, ok := m["a"].(json.Number); !ok {
		t.Fatalf("expected json.Number, got %T", m["a"])
	}
	if _, ok := parseJSONLike("invalid"); ok {
		t.Fatal("expected not-ok for invalid JSON")
	}
}

func TestExpandJSONStrings(t *testing.T) {
	// String-encoded JSON should be expanded.
	input := map[string]interface{}{
		"config": `{"nested":"value"}`,
		"plain":  "hello",
	}
	expanded := expandJSONStrings(input, 0)
	m := expanded.(map[string]interface{})
	nested, ok := m["config"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected config to be expanded to map, got %T", m["config"])
	}
	if nested["nested"] != "value" {
		t.Fatalf("unexpected nested value: %v", nested["nested"])
	}
	if m["plain"] != "hello" {
		t.Fatalf("plain string should be unchanged, got %v", m["plain"])
	}
}

func TestExpandJSONStrings_DepthLimit(t *testing.T) {
	// Depth > 3 should stop expanding.
	nested := `{"a":"{\"b\":\"c\"}"}`
	value, _ := parseJSONLike(nested)
	expanded := expandJSONStrings(value, 5).(map[string]interface{})
	if _, ok := expanded["a"].(map[string]interface{}); ok {
		t.Fatal("depth > 3 should not expand string-encoded JSON")
	}
}

func TestNumericFromAny(t *testing.T) {
	cases := []struct {
		name  string
		value interface{}
		want  float64
		ok    bool
	}{
		{"float64", float64(3.14), 3.14, true},
		{"int", 42, 42, true},
		{"int64", int64(100), 100, true},
		{"string-num", "3.5", 3.5, true},
		{"string-bad", "abc", 0, false},
		{"nil", nil, 0, false},
		{"bool", true, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := numericFromAny(tc.value)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}
			if ok && got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestStringFromAny(t *testing.T) {
	if got := stringFromAny("  hello  "); got != "hello" {
		t.Errorf("expected trimmed 'hello', got %q", got)
	}
	if got := stringFromAny(nil); got != "" {
		t.Errorf("expected empty for nil, got %q", got)
	}
	if got := stringFromAny(42); got != "42" {
		t.Errorf("expected '42', got %q", got)
	}
}

func TestConfidenceRank(t *testing.T) {
	if confidenceRank("high") != 3 {
		t.Fatal("high should be 3")
	}
	if confidenceRank("medium") != 2 {
		t.Fatal("medium should be 2")
	}
	if confidenceRank("mapping") != 1 {
		t.Fatal("mapping should be 1")
	}
	if confidenceRank("low") != 0 {
		t.Fatal("unknown should be 0")
	}
	if confidenceRank("") != 0 {
		t.Fatal("empty should be 0")
	}
}

func TestAppendUniqueString(t *testing.T) {
	values := []string{}
	appendUniqueString(&values, "a", 3)
	appendUniqueString(&values, "b", 3)
	appendUniqueString(&values, "A", 3) // case-insensitive dedupe
	appendUniqueString(&values, "c", 3)
	appendUniqueString(&values, "d", 3) // exceeds limit
	if len(values) != 3 {
		t.Fatalf("expected 3 items (limit), got %d: %v", len(values), values)
	}
	if values[0] != "a" || values[1] != "b" || values[2] != "c" {
		t.Fatalf("unexpected values: %v", values)
	}
	// Empty value is ignored.
	appendUniqueString(&values, "", 5)
	if len(values) != 3 {
		t.Fatal("empty value should be ignored")
	}
}

func TestNormalizedRandomDelay(t *testing.T) {
	cases := []struct {
		name    string
		values  []int
		minWant int
		maxWant int
	}{
		{"empty", []int{}, 0, 0},
		{"single", []int{5}, 0, 0},
		{"normal", []int{3, 10}, 3, 10},
		{"negative-min", []int{-1, 5}, 0, 5},
		{"max-below-min", []int{10, 3}, 10, 10},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			minD, maxD := normalizedRandomDelay(tc.values)
			if minD != tc.minWant || maxD != tc.maxWant {
				t.Errorf("normalizedRandomDelay(%v) = (%d, %d), want (%d, %d)",
					tc.values, minD, maxD, tc.minWant, tc.maxWant)
			}
		})
	}
}

func TestIsModelMappingKey(t *testing.T) {
	positive := []string{"model_mapping", "modelmap", "model_map", "config.model_mapping"}
	for _, key := range positive {
		if !isModelMappingKey(key) {
			t.Errorf("expected %q to be a model mapping key", key)
		}
	}
	if isModelMappingKey("random") {
		t.Error("expected 'random' to not be a model mapping key")
	}
}

func TestIsPricingContainerKey(t *testing.T) {
	positive := []string{"price", "pricing", "ratio", "quota", "model_ratio", "completion_ratio"}
	for _, key := range positive {
		if !isPricingContainerKey(key) {
			t.Errorf("expected %q to be a pricing key", key)
		}
	}
	if isPricingContainerKey("random") {
		t.Error("expected 'random' to not be a pricing key")
	}
}
