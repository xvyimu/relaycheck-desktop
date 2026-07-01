package accounts

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseChromePasswordCSV(t *testing.T) {
	t.Run("valid_csv", func(t *testing.T) {
		csv := "name,url,username,password\n" +
			"Alpha,https://a.com/login,user1,pass1\n" +
			"Beta,https://b.com/login,user2,pass2\n"
		rows, err := parseChromePasswordCSV(csv)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rows) != 2 {
			t.Fatalf("expected 2 rows, got %d", len(rows))
		}
		if rows[0].URL != "https://a.com/login" {
			t.Errorf("row0 URL = %q", rows[0].URL)
		}
		if rows[0].Username != "user1" {
			t.Errorf("row0 Username = %q", rows[0].Username)
		}
		if rows[0].Password != "pass1" {
			t.Errorf("row0 Password = %q", rows[0].Password)
		}
		if rows[0].Name != "Alpha" {
			t.Errorf("row0 Name = %q", rows[0].Name)
		}
	})

	t.Run("strips_bom", func(t *testing.T) {
		// UTF-8 BOM prefix should be stripped.
		csv := "\ufeffname,url,username,password\nA,https://a.com,u,p\n"
		rows, err := parseChromePasswordCSV(csv)
		if err != nil {
			t.Fatalf("BOM should be stripped, got error: %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("expected 1 row, got %d", len(rows))
		}
	})

	t.Run("missing_required_columns", func(t *testing.T) {
		csv := "name,url,username\nA,https://a.com,u\n"
		if _, err := parseChromePasswordCSV(csv); err == nil {
			t.Fatal("expected error for missing password column")
		}
	})

	t.Run("header_only_no_data", func(t *testing.T) {
		csv := "name,url,username,password\n"
		if _, err := parseChromePasswordCSV(csv); err == nil {
			t.Fatal("expected error for header-only CSV")
		}
	})

	t.Run("empty_csv", func(t *testing.T) {
		if _, err := parseChromePasswordCSV(""); err == nil {
			t.Fatal("expected error for empty CSV")
		}
	})

	t.Run("skips_rows_with_empty_fields", func(t *testing.T) {
		csv := "name,url,username,password\n" +
			",https://a.com,user1,pass1\n" + // empty name is OK (name optional)
			"A,,user2,pass2\n" + // empty URL → skip
			"B,https://b.com,,pass2\n" + // empty username → skip
			"C,https://c.com,user3,\n" + // empty password → skip
			"D,https://d.com,user4,pass4\n"
		rows, err := parseChromePasswordCSV(csv)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Only row 0 (empty name) and row 4 (D) should survive.
		if len(rows) != 2 {
			t.Fatalf("expected 2 rows (others skipped), got %d", len(rows))
		}
		if rows[0].Name != "" {
			t.Errorf("row0 Name should be empty, got %q", rows[0].Name)
		}
		if rows[1].Name != "D" {
			t.Errorf("row1 Name = %q, want D", rows[1].Name)
		}
	})

	t.Run("columns_in_different_order", func(t *testing.T) {
		// Columns should be mapped by header name, not position.
		csv := "password,url,username,name\n" +
			"secret,https://a.com,bob,Bob\n"
		rows, err := parseChromePasswordCSV(csv)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("expected 1 row, got %d", len(rows))
		}
		if rows[0].Password != "secret" {
			t.Errorf("Password = %q, want secret", rows[0].Password)
		}
		if rows[0].Username != "bob" {
			t.Errorf("Username = %q, want bob", rows[0].Username)
		}
		if rows[0].Name != "Bob" {
			t.Errorf("Name = %q, want Bob", rows[0].Name)
		}
	})

	t.Run("whitespace_trimmed", func(t *testing.T) {
		csv := "name,url,username,password\n" +
			"  Alpha  ,  https://a.com  ,  user1  ,  pass1  \n"
		rows, err := parseChromePasswordCSV(csv)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rows[0].URL != "https://a.com" {
			t.Errorf("URL should be trimmed, got %q", rows[0].URL)
		}
		if rows[0].Username != "user1" {
			t.Errorf("Username should be trimmed, got %q", rows[0].Username)
		}
	})

	t.Run("oversized_csv_rejected", func(t *testing.T) {
		// >8MB should be rejected. Build efficiently with strings.Repeat.
		header := "name,url,username,password\n"
		row := "A,https://a.com,u,p\n"
		// 9MB of row data + header exceeds the 8MB limit.
		big := header + strings.Repeat(row, (9*1024*1024)/len(row)+1)
		if _, err := parseChromePasswordCSV(big); err == nil {
			t.Fatal("expected error for oversized CSV")
		}
	})
}

func TestCSVField(t *testing.T) {
	record := []string{"a", "b", "c"}
	if got := csvField(record, 0); got != "a" {
		t.Errorf("csvField(record,0) = %q, want a", got)
	}
	if got := csvField(record, 2); got != "c" {
		t.Errorf("csvField(record,2) = %q, want c", got)
	}
	// Out of bounds.
	if got := csvField(record, 5); got != "" {
		t.Errorf("csvField(record,5) should be empty, got %q", got)
	}
	if got := csvField(record, -1); got != "" {
		t.Errorf("csvField(record,-1) should be empty, got %q", got)
	}
	// Whitespace trimmed.
	if got := csvField([]string{"  x  "}, 0); got != "x" {
		t.Errorf("csvField should trim, got %q", got)
	}
}

func TestCountUniqueMatchedSites(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		if got := countUniqueMatchedSites(nil); got != 0 {
			t.Errorf("expected 0, got %d", got)
		}
	})
	t.Run("dedup", func(t *testing.T) {
		matches := []chromePasswordMatch{
			{SiteID: "s1"},
			{SiteID: "s1"}, // duplicate
			{SiteID: "s2"},
			{SiteID: "s3"},
			{SiteID: "s2"}, // duplicate
		}
		if got := countUniqueMatchedSites(matches); got != 3 {
			t.Errorf("expected 3 unique sites, got %d", got)
		}
	})
}

func TestFindChromeRow(t *testing.T) {
	rows := []chromePasswordRow{
		{Name: "A", URL: "https://a.com", Username: "user1", Password: "p1"},
		{Name: "B", URL: "https://b.com", Username: "user2", Password: "p2"},
	}
	t.Run("found", func(t *testing.T) {
		row := findChromeRow(rows, "https://b.com", "user2")
		if row.Name != "B" {
			t.Errorf("expected B, got %q", row.Name)
		}
		if row.Password != "p2" {
			t.Errorf("expected p2, got %q", row.Password)
		}
	})
	t.Run("not_found_returns_empty", func(t *testing.T) {
		row := findChromeRow(rows, "https://x.com", "nobody")
		if row.URL != "" {
			t.Errorf("expected empty row, got %+v", row)
		}
	})
}

func TestExistingAccountIndex_Has(t *testing.T) {
	idx := existingAccountIndex{
		entries: map[string]bool{
			"s1\x00user1": true,
			"s2\x00user2": true,
		},
	}
	if !idx.has("s1", "user1") {
		t.Error("s1/user1 should exist")
	}
	if idx.has("s1", "user2") {
		t.Error("s1/user2 should not exist")
	}
	if idx.has("s3", "user1") {
		t.Error("s3/user1 should not exist")
	}
	// Empty index.
	empty := existingAccountIndex{entries: map[string]bool{}}
	if empty.has("s1", "user1") {
		t.Error("empty index should not have anything")
	}
}

func TestErrorsText(t *testing.T) {
	err := errorsText("something failed")
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if err.Error() != "something failed" {
		t.Errorf("Error() = %q, want 'something failed'", err.Error())
	}
}

func TestNormalizeDBValue(t *testing.T) {
	// nil stays nil.
	if got := normalizeDBValue(nil); got != nil {
		t.Errorf("nil should stay nil, got %v", got)
	}
	// []byte becomes string.
	if got := normalizeDBValue([]byte("hello")); got != "hello" {
		t.Errorf("[]byte should become string, got %v", got)
	}
	// Other types pass through.
	if got := normalizeDBValue(42); got != 42 {
		t.Errorf("int should pass through, got %v", got)
	}
	if got := normalizeDBValue("str"); got != "str" {
		t.Errorf("string should pass through, got %v", got)
	}
}

func TestMaskSensitiveJSONValue(t *testing.T) {
	t.Run("map_with_sensitive_keys", func(t *testing.T) {
		value := map[string]interface{}{
			"name":  "visible",
			"key":   "sk-secret-123456",
			"token": "tok-abc",
		}
		result := maskSensitiveJSONValue(value)
		m, ok := result.(map[string]interface{})
		if !ok {
			t.Fatalf("expected map, got %T", result)
		}
		if m["name"] != "visible" {
			t.Error("non-sensitive field should be visible")
		}
		masked, ok := m["key"].(string)
		if !ok || contains(masked, "sk-secret-123456") {
			t.Errorf("key should be masked, got %v", m["key"])
		}
		maskedTok, ok := m["token"].(string)
		if !ok || contains(maskedTok, "tok-abc") {
			t.Errorf("token should be masked, got %v", m["token"])
		}
	})

	t.Run("nested_map", func(t *testing.T) {
		value := map[string]interface{}{
			"config": map[string]interface{}{
				"password": "secret123",
				"label":    "ok",
			},
		}
		result := maskSensitiveJSONValue(value).(map[string]interface{})
		config := result["config"].(map[string]interface{})
		if config["label"] != "ok" {
			t.Error("nested non-sensitive should be visible")
		}
		if contains(fmt.Sprint(config["password"]), "secret123") {
			t.Error("nested password should be masked")
		}
	})

	t.Run("array", func(t *testing.T) {
		value := []interface{}{
			map[string]interface{}{"key": "k1", "name": "n1"},
			map[string]interface{}{"key": "k2", "name": "n2"},
		}
		result := maskSensitiveJSONValue(value).([]interface{})
		if len(result) != 2 {
			t.Fatalf("expected 2 elements, got %d", len(result))
		}
		m0 := result[0].(map[string]interface{})
		if m0["name"] != "n1" {
			t.Error("array element non-sensitive should be visible")
		}
		if contains(fmt.Sprint(m0["key"]), "k1") {
			t.Error("array element key should be masked")
		}
	})

	t.Run("scalar_passthrough", func(t *testing.T) {
		if got := maskSensitiveJSONValue(42); got != 42 {
			t.Errorf("int should pass through, got %v", got)
		}
		if got := maskSensitiveJSONValue("str"); got != "str" {
			t.Errorf("string should pass through, got %v", got)
		}
	})
}

func TestMaskSensitiveImportedValue_NonJSONString(t *testing.T) {
	// Non-JSON string with non-sensitive key should pass through unchanged.
	if got := maskSensitiveImportedValue("description", "plain text"); got != "plain text" {
		t.Errorf("non-sensitive non-JSON should pass through, got %v", got)
	}
	// Sensitive key should be masked.
	if got := maskSensitiveImportedValue("key", "sk-1234567890"); got == "sk-1234567890" {
		t.Error("sensitive key should be masked")
	}
	// nil passes through.
	if got := maskSensitiveImportedValue("key", nil); got != nil {
		t.Errorf("nil should pass through, got %v", got)
	}
	// Non-string value with non-sensitive key passes through.
	if got := maskSensitiveImportedValue("count", 42); got != 42 {
		t.Errorf("non-string non-sensitive should pass through, got %v", got)
	}
}

func TestInferImportedKind_AdditionalCases(t *testing.T) {
	// Cover "openai" and "one api" / "new api" variants.
	cases := []struct {
		record  map[string]interface{}
		baseURL string
		want    string
	}{
		{map[string]interface{}{"note": "new api relay"}, "", "newapi"},
		{map[string]interface{}{"note": "one api relay"}, "", "oneapi"},
		{map[string]interface{}{}, "https://openai.example.com", "openai_compatible"},
	}
	for _, tc := range cases {
		if got := inferImportedKind(tc.record, tc.baseURL); got != tc.want {
			t.Errorf("inferImportedKind() = %q, want %q", got, tc.want)
		}
	}
}

func TestCompareImportedChannelFields_KeyAndRawJSON(t *testing.T) {
	// Differing KeyMasked and RawJSON should be detected.
	current := existingImportedChannel{
		Name:      "same",
		BaseURL:   "https://a.com",
		Status:    "active",
		Kind:      "newapi",
		KeyMasked: "*********aaaa",
		RawJSON:   `{"id":"1"}`,
	}
	next := preparedSyncRecord{
		Name:      "same",
		BaseURL:   "https://a.com",
		Status:    "active",
		Kind:      "newapi",
		KeyMasked: "*********bbbb",
		RawJSON:   `{"id":"2"}`,
	}
	fields := compareImportedChannelFields(current, next)
	// Should include "渠道 Key" and "原始配置".
	foundKey := false
	foundRaw := false
	for _, f := range fields {
		if f == "渠道 Key" {
			foundKey = true
		}
		if f == "原始配置" {
			foundRaw = true
		}
	}
	if !foundKey {
		t.Error("expected '渠道 Key' in changed fields")
	}
	if !foundRaw {
		t.Error("expected '原始配置' in changed fields")
	}
}

func TestCompareImportedChannelFields_KeySkippedWhenEmpty(t *testing.T) {
	// When next.KeyMasked is empty, key comparison is skipped.
	current := existingImportedChannel{
		Name:      "same",
		BaseURL:   "https://a.com",
		Status:    "active",
		Kind:      "newapi",
		KeyMasked: "*********aaaa",
	}
	next := preparedSyncRecord{
		Name:      "same",
		BaseURL:   "https://a.com",
		Status:    "active",
		Kind:      "newapi",
		KeyMasked: "",
	}
	fields := compareImportedChannelFields(current, next)
	for _, f := range fields {
		if f == "渠道 Key" {
			t.Error("should not flag Key when next.KeyMasked is empty")
		}
	}
}
