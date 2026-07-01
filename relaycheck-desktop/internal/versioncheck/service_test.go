package versioncheck

import "testing"

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"v1.0", "v1.0", 0},
		{"1.0.0", "1.0.0", 0},
		{"v1.0", "v2.0", -1},
		{"v2.0", "v1.0", 1},
		{"v1.0.1", "v1.0.0", 1},
		{"v1.0.0", "v1.0.1", -1},
		{"v1.1", "v1.0.9", 1},
		{"v1.0.9", "v1.1", -1},
		{"2.0.0", "2.0.1", -1},
		{"v1.0", "1.0", 0},
		{"", "", 0},
		{"v1.0.0", "v1.0.0.1", -1},
		{"v1.0.0.1", "v1.0.0", 1},
	}
	for _, tt := range tests {
		got := CompareVersions(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestCompareVersions_AdditionalCases(t *testing.T) {
	// Whitespace is trimmed.
	if got := CompareVersions("  v1.0  ", " v1.0 "); got != 0 {
		t.Errorf("whitespace should be trimmed, got %d", got)
	}
	// Non-numeric parts parse as 0.
	if got := CompareVersions("v1.0.0-rc1", "v1.0.0"); got != 0 {
		t.Errorf("non-numeric suffix should parse as 0, got %d", got)
	}
	// Empty vs version: empty parses as 0.0.0.
	if got := CompareVersions("", "v1.0"); got != -1 {
		t.Errorf("empty < v1.0, got %d", got)
	}
	// Mixed v prefix.
	if got := CompareVersions("v1.2.3", "1.2.3"); got != 0 {
		t.Errorf("v-prefix should be stripped, got %d", got)
	}
}

func TestDecodeSettingString(t *testing.T) {
	t.Run("json_string", func(t *testing.T) {
		// Properly JSON-encoded string.
		if got := decodeSettingString(`"https://example.com/manifest.json"`); got != "https://example.com/manifest.json" {
			t.Errorf("expected URL, got %q", got)
		}
	})
	t.Run("json_string_with_escapes", func(t *testing.T) {
		// JSON string with escaped quotes.
		if got := decodeSettingString(`"hello \"world\""`); got != `hello "world"` {
			t.Errorf("expected escaped quotes, got %q", got)
		}
	})
	t.Run("json_empty_string", func(t *testing.T) {
		if got := decodeSettingString(`""`); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})
	t.Run("raw_value_no_quotes", func(t *testing.T) {
		// Not valid JSON — falls back to raw value.
		if got := decodeSettingString("https://example.com/manifest.json"); got != "https://example.com/manifest.json" {
			t.Errorf("expected raw value, got %q", got)
		}
	})
	t.Run("raw_value_with_surrounding_quotes", func(t *testing.T) {
		// Not valid JSON (unescaped quotes) — strips surrounding quotes.
		if got := decodeSettingString(`"https://example.com`); got != "https://example.com" {
			t.Errorf("expected surrounding quotes stripped, got %q", got)
		}
	})
	t.Run("empty_input", func(t *testing.T) {
		if got := decodeSettingString(""); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})
	t.Run("json_number_falls_back", func(t *testing.T) {
		// A JSON number is not a string, so Unmarshal into *string fails;
		// fallback returns the raw value (no quotes to strip).
		if got := decodeSettingString("42"); got != "42" {
			t.Errorf("expected 42, got %q", got)
		}
	})
	t.Run("json_object_falls_back", func(t *testing.T) {
		// A JSON object is not a string — fallback returns raw.
		raw := `{"key":"value"}`
		if got := decodeSettingString(raw); got != raw {
			t.Errorf("expected raw object, got %q", got)
		}
	})
}
