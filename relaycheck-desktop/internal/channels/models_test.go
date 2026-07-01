package channels

import "testing"

func TestBuildChannelModelOverview(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		ov := BuildChannelModelOverview(nil, 0, "2026-07-01")
		if ov.ChannelCount != 0 || ov.ModelCount != 0 {
			t.Errorf("expected zero overview, got %+v", ov)
		}
		if ov.GeneratedAt != "2026-07-01" {
			t.Errorf("GeneratedAt = %q", ov.GeneratedAt)
		}
		if len(ov.Items) != 0 || len(ov.Models) != 0 {
			t.Errorf("expected empty slices, got items=%d models=%d", len(ov.Items), len(ov.Models))
		}
	})

	t.Run("status_counts", func(t *testing.T) {
		items := []ChannelModelSyncItem{
			{ChannelID: "c1", ChannelName: "alpha", Status: "live_key", ModelCount: 3, SampleModels: []string{"gpt-4"}},
			{ChannelID: "c2", ChannelName: "beta", Status: "raw_only", ModelCount: 1},
			{ChannelID: "c3", ChannelName: "gamma", Status: "failed"},
			{ChannelID: "c4", ChannelName: "delta", Status: "key_invalid"},
			{ChannelID: "c5", ChannelName: "epsilon", Status: "unchecked"},
			{ChannelID: "c6", ChannelName: "zeta", Status: ""},
		}
		ov := BuildChannelModelOverview(items, 5, "now")
		if ov.ChannelCount != 6 {
			t.Errorf("ChannelCount = %d, want 6", ov.ChannelCount)
		}
		if ov.LiveKeyCount != 1 {
			t.Errorf("LiveKeyCount = %d, want 1", ov.LiveKeyCount)
		}
		if ov.RawOnlyCount != 1 {
			t.Errorf("RawOnlyCount = %d, want 1", ov.RawOnlyCount)
		}
		if ov.FailedCount != 2 {
			t.Errorf("FailedCount = %d, want 2 (failed + key_invalid)", ov.FailedCount)
		}
		if ov.UncheckedCount != 2 {
			t.Errorf("UncheckedCount = %d, want 2 (unchecked + empty)", ov.UncheckedCount)
		}
		if ov.SyncedChannels != 5 {
			t.Errorf("SyncedChannels = %d, want 5 (passthrough)", ov.SyncedChannels)
		}
	})

	t.Run("model_coverage_aggregation", func(t *testing.T) {
		items := []ChannelModelSyncItem{
			{ChannelID: "c1", ChannelName: "alpha", Status: "live_key", SampleModels: []string{"gpt-4", "claude-3-opus"}},
			{ChannelID: "c2", ChannelName: "beta", Status: "live_key", SampleModels: []string{"GPT-4"}},
			{ChannelID: "c3", ChannelName: "gamma", Status: "raw_only", SampleModels: []string{"claude-3-opus"}},
		}
		ov := BuildChannelModelOverview(items, 0, "now")
		if ov.ModelCount != 2 {
			t.Errorf("ModelCount = %d, want 2 (case-insensitive dedup)", ov.ModelCount)
		}
		// Find gpt-4 entry
		var gpt4 *ChannelModelCoverageItem
		for i := range ov.Models {
			if ov.Models[i].Model == "gpt-4" {
				gpt4 = &ov.Models[i]
				break
			}
		}
		if gpt4 == nil {
			t.Fatalf("gpt-4 model not in coverage: %+v", ov.Models)
		}
		if gpt4.ChannelCount != 2 {
			t.Errorf("gpt-4 ChannelCount = %d, want 2 (alpha + beta case-insensitive)", gpt4.ChannelCount)
		}
		if gpt4.LiveKeyCount != 2 {
			t.Errorf("gpt-4 LiveKeyCount = %d, want 2", gpt4.LiveKeyCount)
		}
		if len(gpt4.Channels) != 2 {
			t.Errorf("gpt-4 Channels len = %d, want 2", len(gpt4.Channels))
		}
	})

	t.Run("items_sorted_by_model_count_desc_then_name", func(t *testing.T) {
		items := []ChannelModelSyncItem{
			{ChannelID: "c1", ChannelName: "zeta", ModelCount: 5},
			{ChannelID: "c2", ChannelName: "alpha", ModelCount: 5},
			{ChannelID: "c3", ChannelName: "beta", ModelCount: 10},
		}
		ov := BuildChannelModelOverview(items, 0, "now")
		if ov.Items[0].ChannelName != "beta" {
			t.Errorf("first item should be beta (highest model count), got %s", ov.Items[0].ChannelName)
		}
		if ov.Items[1].ChannelName != "alpha" {
			t.Errorf("second item should be alpha (tie-break by name), got %s", ov.Items[1].ChannelName)
		}
	})

	t.Run("items_truncated_at_80", func(t *testing.T) {
		items := make([]ChannelModelSyncItem, 90)
		for i := range items {
			items[i] = ChannelModelSyncItem{ChannelID: "c" + itoa(i), ChannelName: "ch" + itoa(i), ModelCount: i}
		}
		ov := BuildChannelModelOverview(items, 0, "now")
		if len(ov.Items) != 80 {
			t.Errorf("len(Items) = %d, want 80", len(ov.Items))
		}
	})
}

func TestModelsFromRawChannelJSON(t *testing.T) {
	t.Run("invalid_json", func(t *testing.T) {
		if got := modelsFromRawChannelJSON("not-json"); got != nil {
			t.Errorf("expected nil for invalid JSON, got %v", got)
		}
	})
	t.Run("models_array", func(t *testing.T) {
		raw := `{"models":["gpt-4","claude-3-opus"],"name":"channel"}`
		got := modelsFromRawChannelJSON(raw)
		if len(got) != 2 {
			t.Fatalf("expected 2 models, got %d: %v", len(got), got)
		}
		if got[0] != "gpt-4" || got[1] != "claude-3-opus" {
			t.Errorf("models = %v", got)
		}
	})
	t.Run("model_mapping_keys", func(t *testing.T) {
		raw := `{"model_mapping":{"gpt-4":"gpt-4-0613","claude-3-opus":"claude-3-opus-20240229"}}`
		got := modelsFromRawChannelJSON(raw)
		if len(got) != 2 {
			t.Fatalf("expected 2 mapping keys, got %d: %v", len(got), got)
		}
	})
	t.Run("embedded_json_string", func(t *testing.T) {
		raw := `{"config":"{\"models\":[\"gpt-4\"]}"}`
		got := modelsFromRawChannelJSON(raw)
		if len(got) != 1 || got[0] != "gpt-4" {
			t.Errorf("expected [gpt-4] from embedded JSON, got %v", got)
		}
	})
	t.Run("filters_non_model_ids", func(t *testing.T) {
		raw := `{"models":["gpt-4","random","just-text"]}`
		got := modelsFromRawChannelJSON(raw)
		if len(got) != 1 || got[0] != "gpt-4" {
			t.Errorf("expected only gpt-4, got %v", got)
		}
	})
}

func TestNormalizeModelIDs(t *testing.T) {
	t.Run("dedup_case_insensitive", func(t *testing.T) {
		input := []string{"gpt-4", "GPT-4", "Claude-3-Opus", "claude-3-opus"}
		got := normalizeModelIDs(input)
		if len(got) != 2 {
			t.Errorf("expected 2 unique models, got %d: %v", len(got), got)
		}
	})
	t.Run("filters_empty_and_non_model", func(t *testing.T) {
		input := []string{"", "  ", "random", "gpt-4"}
		got := normalizeModelIDs(input)
		if len(got) != 1 || got[0] != "gpt-4" {
			t.Errorf("expected [gpt-4], got %v", got)
		}
	})
	t.Run("nil_input", func(t *testing.T) {
		if got := normalizeModelIDs(nil); len(got) != 0 {
			t.Errorf("expected empty, got %v", got)
		}
	})
}

func TestSplitModelList(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  int
	}{
		{"comma_separated", "gpt-4,claude-3-opus,deepseek-chat", 3},
		{"newline_separated", "gpt-4\nclaude-3-opus\ndeepseek-chat", 3},
		{"mixed_separators", "gpt-4 claude-3-opus,deepseek-chat", 3},
		{"filters_non_model", "gpt-4,random,just-text", 1},
		{"empty", "", 0},
		{"single", "gpt-4", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := splitModelList(tc.input)
			if len(got) != tc.want {
				t.Errorf("splitModelList(%q) = %v (len %d), want len %d", tc.input, got, len(got), tc.want)
			}
		})
	}
}

func TestExtractModelsFromJSON(t *testing.T) {
	t.Run("nested_models_key", func(t *testing.T) {
		root := map[string]interface{}{
			"data": []interface{}{
				map[string]interface{}{"models": []interface{}{"gpt-4", "claude-3-opus"}},
				map[string]interface{}{"model": "deepseek-chat"},
			},
		}
		got := extractModelsFromJSON(root)
		if len(got) != 3 {
			t.Errorf("expected 3 models, got %d: %v", len(got), got)
		}
	})
	t.Run("models_as_string", func(t *testing.T) {
		root := map[string]interface{}{
			"models": "gpt-4,claude-3-opus",
		}
		got := extractModelsFromJSON(root)
		if len(got) != 2 {
			t.Errorf("expected 2 models from string, got %v", got)
		}
	})
	t.Run("dedup_case_insensitive", func(t *testing.T) {
		root := map[string]interface{}{
			"models": []interface{}{"gpt-4", "GPT-4"},
		}
		got := extractModelsFromJSON(root)
		if len(got) != 1 {
			t.Errorf("expected 1 unique model, got %v", got)
		}
	})
}

func TestModelsFromAny(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		got := modelsFromAny("gpt-4,claude-3-opus")
		if len(got) != 2 {
			t.Errorf("expected 2 models, got %v", got)
		}
	})
	t.Run("array", func(t *testing.T) {
		got := modelsFromAny([]interface{}{"gpt-4", "claude-3-opus"})
		if len(got) != 2 {
			t.Errorf("expected 2 models, got %v", got)
		}
	})
	t.Run("map_with_id", func(t *testing.T) {
		got := modelsFromAny(map[string]interface{}{"id": "gpt-4"})
		if len(got) != 1 || got[0] != "gpt-4" {
			t.Errorf("expected [gpt-4], got %v", got)
		}
	})
	t.Run("map_with_model_field", func(t *testing.T) {
		got := modelsFromAny(map[string]interface{}{"model": "claude-3-opus"})
		if len(got) != 1 || got[0] != "claude-3-opus" {
			t.Errorf("expected [claude-3-opus], got %v", got)
		}
	})
	t.Run("map_with_non_model_id", func(t *testing.T) {
		got := modelsFromAny(map[string]interface{}{"id": "random"})
		if len(got) != 0 {
			t.Errorf("expected empty (non-model id), got %v", got)
		}
	})
}

func TestExtractModelMappingKeys(t *testing.T) {
	t.Run("extracts_keys", func(t *testing.T) {
		root := map[string]interface{}{
			"model_mapping": map[string]interface{}{
				"gpt-4":         "gpt-4-0613",
				"claude-3-opus": "claude-3-opus-20240229",
			},
		}
		got := extractModelMappingKeys(root)
		if len(got) != 2 {
			t.Errorf("expected 2 mapping keys, got %v", got)
		}
	})
	t.Run("nested_in_array", func(t *testing.T) {
		root := map[string]interface{}{
			"channels": []interface{}{
				map[string]interface{}{
					"model_mapping": map[string]interface{}{"gpt-4": "gpt-4-0613"},
				},
			},
		}
		got := extractModelMappingKeys(root)
		if len(got) != 1 || got[0] != "gpt-4" {
			t.Errorf("expected [gpt-4], got %v", got)
		}
	})
	t.Run("no_mapping_key", func(t *testing.T) {
		root := map[string]interface{}{"other": "value"}
		if got := extractModelMappingKeys(root); len(got) != 0 {
			t.Errorf("expected empty, got %v", got)
		}
	})
}

func TestMarshalStringSliceLimit(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		if got := marshalStringSliceLimit(nil, 10); got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})
	t.Run("under_limit", func(t *testing.T) {
		got := marshalStringSliceLimit([]string{"a", "b"}, 10)
		if got != `["a","b"]` {
			t.Errorf("got %q", got)
		}
	})
	t.Run("over_limit_truncates", func(t *testing.T) {
		got := marshalStringSliceLimit([]string{"a", "b", "c"}, 2)
		if got != `["a","b"]` {
			t.Errorf("expected truncated, got %q", got)
		}
	})
}

func TestParsePersistedStringSliceLimit(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		if got := parsePersistedStringSliceLimit("", 10); got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})
	t.Run("whitespace_only", func(t *testing.T) {
		if got := parsePersistedStringSliceLimit("   ", 10); got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})
	t.Run("invalid_json", func(t *testing.T) {
		if got := parsePersistedStringSliceLimit("not-json", 10); got != nil {
			t.Errorf("expected nil for invalid JSON, got %v", got)
		}
	})
	t.Run("valid_with_limit", func(t *testing.T) {
		got := parsePersistedStringSliceLimit(`["a","b","c"]`, 2)
		if len(got) != 2 || got[0] != "a" || got[1] != "b" {
			t.Errorf("expected [a b], got %v", got)
		}
	})
}
