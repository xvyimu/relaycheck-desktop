package channels

import "testing"

func TestBuildModelOverview(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		ov := BuildModelOverview(nil, "2026-07-01")
		if ov.ModelCount != 0 || ov.AccountCount != 0 {
			t.Errorf("expected zero overview, got %+v", ov)
		}
		if ov.GeneratedAt != "2026-07-01" {
			t.Errorf("GeneratedAt = %q", ov.GeneratedAt)
		}
		if len(ov.Models) != 0 || len(ov.Sites) != 0 || len(ov.PriceHints) != 0 {
			t.Errorf("expected empty slices")
		}
	})

	t.Run("account_and_key_counts", func(t *testing.T) {
		records := []AccountModelRecord{
			{AccountID: "a1", SiteID: "s1", Status: "valid", ModelUsable: true, LatencyMs: 100, TestModel: "gpt-4"},
			{AccountID: "a2", SiteID: "s1", Status: "invalid", ModelUsable: false, LatencyMs: 200},
			{AccountID: "a3", SiteID: "s2", Status: "valid", ModelUsable: true, LatencyMs: 50, TestModel: "claude-3-opus"},
		}
		ov := BuildModelOverview(records, "now")
		if ov.AccountCount != 3 {
			t.Errorf("AccountCount = %d, want 3", ov.AccountCount)
		}
		if ov.ValidKeyCount != 2 {
			t.Errorf("ValidKeyCount = %d, want 2", ov.ValidKeyCount)
		}
		if ov.UsableModelCount != 2 {
			t.Errorf("UsableModelCount = %d, want 2", ov.UsableModelCount)
		}
		if ov.FastestLatencyMs != 50 {
			t.Errorf("FastestLatencyMs = %d, want 50", ov.FastestLatencyMs)
		}
	})

	t.Run("model_coverage_with_usable_test_model", func(t *testing.T) {
		records := []AccountModelRecord{
			{
				AccountID:    "a1",
				SiteID:       "s1",
				SiteName:     "Alpha",
				Status:       "valid",
				ModelUsable:  true,
				LatencyMs:    100,
				TestModel:    "gpt-4",
				SampleModels: []string{"gpt-4", "claude-3-opus"},
			},
		}
		ov := BuildModelOverview(records, "now")
		if ov.ModelCount != 2 {
			t.Fatalf("ModelCount = %d, want 2", ov.ModelCount)
		}
		var gpt4 *ModelCoverageItem
		for i := range ov.Models {
			if ov.Models[i].Model == "gpt-4" {
				gpt4 = &ov.Models[i]
				break
			}
		}
		if gpt4 == nil {
			t.Fatalf("gpt-4 not in models: %+v", ov.Models)
		}
		if gpt4.UsableCount != 1 {
			t.Errorf("gpt-4 UsableCount = %d, want 1 (TestModel matches)", gpt4.UsableCount)
		}
		if gpt4.FastestLatencyMs != 100 {
			t.Errorf("gpt-4 FastestLatencyMs = %d, want 100", gpt4.FastestLatencyMs)
		}
		// claude-3-opus has no matching TestModel
		var claude *ModelCoverageItem
		for i := range ov.Models {
			if ov.Models[i].Model == "claude-3-opus" {
				claude = &ov.Models[i]
				break
			}
		}
		if claude == nil {
			t.Fatalf("claude-3-opus not in models")
		}
		if claude.UsableCount != 0 {
			t.Errorf("claude-3-opus UsableCount = %d, want 0 (TestModel mismatch)", claude.UsableCount)
		}
	})

	t.Run("case_insensitive_test_model_match", func(t *testing.T) {
		records := []AccountModelRecord{
			{AccountID: "a1", SiteID: "s1", Status: "valid", ModelUsable: true, LatencyMs: 50, TestModel: "GPT-4"},
		}
		ov := BuildModelOverview(records, "now")
		var gpt4 *ModelCoverageItem
		for i := range ov.Models {
			if ov.Models[i].Model == "GPT-4" {
				gpt4 = &ov.Models[i]
				break
			}
		}
		if gpt4 == nil {
			t.Fatalf("GPT-4 not in models: %+v", ov.Models)
		}
		if gpt4.UsableCount != 1 {
			t.Errorf("GPT-4 UsableCount = %d, want 1 (case-insensitive match)", gpt4.UsableCount)
		}
	})

	t.Run("price_hints_inferred", func(t *testing.T) {
		records := []AccountModelRecord{
			{AccountID: "a1", SiteID: "s1", Status: "valid", SampleModels: []string{"gpt-4o-mini", "claude-3-opus", "random-model"}},
		}
		ov := BuildModelOverview(records, "now")
		if len(ov.PriceHints) != 2 {
			t.Errorf("PriceHints len = %d, want 2 (gpt-4o-mini + claude-3-opus; random-model excluded)", len(ov.PriceHints))
		}
	})

	t.Run("models_sorted_by_usable_count_desc", func(t *testing.T) {
		records := []AccountModelRecord{
			{AccountID: "a1", SiteID: "s1", Status: "valid", ModelUsable: true, TestModel: "gpt-4", SampleModels: []string{"gpt-4"}},
			{AccountID: "a2", SiteID: "s1", Status: "valid", ModelUsable: true, TestModel: "gpt-4", SampleModels: []string{"gpt-4"}},
			{AccountID: "a3", SiteID: "s1", Status: "valid", ModelUsable: false, TestModel: "claude-3-opus", SampleModels: []string{"claude-3-opus"}},
		}
		ov := BuildModelOverview(records, "now")
		if len(ov.Models) < 2 {
			t.Fatalf("expected 2 models, got %d", len(ov.Models))
		}
		if ov.Models[0].Model != "gpt-4" {
			t.Errorf("expected gpt-4 first (UsableCount=2), got %s (UsableCount=%d)", ov.Models[0].Model, ov.Models[0].UsableCount)
		}
	})

	t.Run("models_truncated_at_80", func(t *testing.T) {
		records := make([]AccountModelRecord, 1)
		models := make([]string, 90)
		for i := range models {
			models[i] = "model-" + itoa(i)
		}
		records[0] = AccountModelRecord{AccountID: "a1", SiteID: "s1", SampleModels: models}
		ov := BuildModelOverview(records, "now")
		if len(ov.Models) != 80 {
			t.Errorf("len(Models) = %d, want 80", len(ov.Models))
		}
	})
}

func TestNormalizedModelList(t *testing.T) {
	t.Run("dedup_sample_models_case_insensitive", func(t *testing.T) {
		record := AccountModelRecord{SampleModels: []string{"gpt-4", "GPT-4", "claude-3-opus"}}
		got := normalizedModelList(record)
		if len(got) != 2 {
			t.Errorf("expected 2 unique, got %v", got)
		}
	})
	t.Run("includes_test_model", func(t *testing.T) {
		record := AccountModelRecord{SampleModels: []string{"gpt-4"}, TestModel: "claude-3-opus"}
		got := normalizedModelList(record)
		if len(got) != 2 {
			t.Errorf("expected 2 (sample + test), got %v", got)
		}
	})
	t.Run("test_model_dedup_with_sample", func(t *testing.T) {
		record := AccountModelRecord{SampleModels: []string{"gpt-4"}, TestModel: "GPT-4"}
		got := normalizedModelList(record)
		if len(got) != 1 {
			t.Errorf("expected 1 (dedup case-insensitive), got %v", got)
		}
	})
	t.Run("filters_empty_strings", func(t *testing.T) {
		record := AccountModelRecord{SampleModels: []string{"", "  ", "gpt-4"}}
		got := normalizedModelList(record)
		if len(got) != 1 || got[0] != "gpt-4" {
			t.Errorf("expected [gpt-4], got %v", got)
		}
	})
}

func TestInferModelPriceHint(t *testing.T) {
	cases := []struct {
		model      string
		wantVendor string
		wantLevel  string
		wantOk     bool
	}{
		{"gpt-4o", "OpenAI", "unknown", true},
		{"gpt-4o-mini", "OpenAI", "cheap", true},
		{"gpt-4.1", "OpenAI", "unknown", true},
		{"o1", "OpenAI", "unknown", true},
		{"o3-mini", "OpenAI", "cheap", true},
		{"claude-3-opus", "Anthropic", "high", true},
		{"claude-3-sonnet", "Anthropic", "unknown", true},
		{"gemini-pro", "Google", "cheap", true},
		{"gemini-flash", "Google", "cheap", true},
		{"deepseek-chat", "DeepSeek", "low", true},
		{"qwen-turbo", "Qwen", "cheap", true},
		{"glm-4", "Zhipu", "unknown", true},
		{"doubao-pro", "ByteDance", "standard", true},
		{"moonshot-v1", "Moonshot", "unknown", true},
		{"kimi-latest", "Moonshot", "unknown", true},
		{"random-model", "unknown", "unknown", false},
		{"", "unknown", "unknown", false},
	}
	for _, tc := range cases {
		t.Run(tc.model, func(t *testing.T) {
			hint, ok := inferModelPriceHint(tc.model)
			if ok != tc.wantOk {
				t.Errorf("inferModelPriceHint(%q) ok = %v, want %v", tc.model, ok, tc.wantOk)
				return
			}
			if !ok {
				return
			}
			if hint.Vendor != tc.wantVendor {
				t.Errorf("Vendor = %q, want %q", hint.Vendor, tc.wantVendor)
			}
			if hint.PriceLevel != tc.wantLevel {
				t.Errorf("PriceLevel = %q, want %q", hint.PriceLevel, tc.wantLevel)
			}
		})
	}
}

func TestPriceLevelBySuffix(t *testing.T) {
	cases := map[string]string{
		"gpt-4o-mini":      "cheap",
		"gemini-flash":     "cheap",
		"claude-3-lite":    "cheap",
		"qwen-turbo":       "cheap",
		"claude-3-pro":     "standard",
		"gpt-4-plus":       "standard",
		"claude-3-opus":    "high",
		"gpt-4-32k":        "high",
		"gpt-4-128k":       "high",
		"claude-3-max":     "high",
		"gpt-4":            "unknown",
		"deepseek-chat":    "unknown",
	}
	for model, want := range cases {
		if got := priceLevelBySuffix(model); got != want {
			t.Errorf("priceLevelBySuffix(%q) = %q, want %q", model, got, want)
		}
	}
}
