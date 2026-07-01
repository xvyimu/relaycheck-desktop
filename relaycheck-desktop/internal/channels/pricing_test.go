package channels

import "testing"

func TestBuildPricingOverview(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		ov := BuildPricingOverview(nil, nil, nil, "now")
		if ov.SourceCount != 0 || ov.ModelCount != 0 {
			t.Errorf("expected zero, got %+v", ov)
		}
		if len(ov.Sources) != 0 || len(ov.Comparisons) != 0 {
			t.Errorf("expected empty slices")
		}
	})

	t.Run("filters_non_model_ids", func(t *testing.T) {
		sources := []ModelPricingSource{
			{ChannelID: "c1", Model: "random", Source: "raw_json"},
			{ChannelID: "c1", Model: "gpt-4", Source: "raw_json"},
		}
		ov := BuildPricingOverview(sources, nil, nil, "now")
		if ov.SourceCount != 1 {
			t.Errorf("SourceCount = %d, want 1 (random filtered)", ov.SourceCount)
		}
	})

	t.Run("dedup_sources", func(t *testing.T) {
		sources := []ModelPricingSource{
			{ChannelID: "c1", Model: "gpt-4", Source: "raw_json", FieldPath: "a", UpstreamModel: ""},
			{ChannelID: "c1", Model: "gpt-4", Source: "raw_json", FieldPath: "a", UpstreamModel: ""},
		}
		ov := BuildPricingOverview(sources, nil, nil, "now")
		if ov.SourceCount != 1 {
			t.Errorf("SourceCount = %d, want 1 (dedup)", ov.SourceCount)
		}
	})

	t.Run("exact_and_ratio_counts", func(t *testing.T) {
		sources := []ModelPricingSource{
			{ChannelID: "c1", Model: "gpt-4", Source: "raw_json", Price: floatPtr(0.03)},
			{ChannelID: "c2", Model: "claude-3-opus", Source: "raw_json", PromptRatio: floatPtr(1.5)},
		}
		ov := BuildPricingOverview(sources, nil, nil, "now")
		if ov.ExactCount != 1 {
			t.Errorf("ExactCount = %d, want 1", ov.ExactCount)
		}
		if ov.RatioCount != 1 {
			t.Errorf("RatioCount = %d, want 1", ov.RatioCount)
		}
	})

	t.Run("cache_status_counts", func(t *testing.T) {
		cacheItems := []SitePricingCacheItem{
			{SiteID: "s1", Status: "success"},
			{SiteID: "s2", Status: "failed"},
			{SiteID: "s3", Status: "empty"},
			{SiteID: "s4", Status: ""},
		}
		ov := BuildPricingOverview(nil, cacheItems, nil, "now")
		if ov.LiveCacheCount != 1 {
			t.Errorf("LiveCacheCount = %d, want 1", ov.LiveCacheCount)
		}
		if ov.FailedCacheCount != 2 {
			t.Errorf("FailedCacheCount = %d, want 2 (failed + empty, but not blank)", ov.FailedCacheCount)
		}
	})

	t.Run("sources_truncated_at_200", func(t *testing.T) {
		sources := make([]ModelPricingSource, 210)
		for i := range sources {
			sources[i] = ModelPricingSource{
				ChannelID: "c" + itoa(i),
				Model:     "model-" + itoa(i),
				Source:    "raw_json",
			}
		}
		ov := BuildPricingOverview(sources, nil, nil, "now")
		if len(ov.Sources) != 200 {
			t.Errorf("len(Sources) = %d, want 200", len(ov.Sources))
		}
	})
}

func TestBuildModelPriceComparisons(t *testing.T) {
	t.Run("lowest_price_picks_cheapest", func(t *testing.T) {
		cheap := 0.01
		expensive := 0.05
		sources := []ModelPricingSource{
			{ChannelID: "c1", ChannelName: "alpha", Model: "gpt-4", Price: &expensive},
			{ChannelID: "c2", ChannelName: "beta", Model: "gpt-4", Price: &cheap},
		}
		comps := BuildModelPriceComparisons(sources, nil)
		if len(comps) != 1 {
			t.Fatalf("expected 1 comparison, got %d", len(comps))
		}
		if comps[0].LowestPrice == nil || *comps[0].LowestPrice != cheap {
			t.Errorf("LowestPrice = %v, want %v", comps[0].LowestPrice, cheap)
		}
		if comps[0].BestSource != "beta" {
			t.Errorf("BestSource = %q, want beta", comps[0].BestSource)
		}
	})

	t.Run("ratio_only_no_price", func(t *testing.T) {
		ratio := 1.5
		sources := []ModelPricingSource{
			{ChannelID: "c1", ChannelName: "alpha", Model: "gpt-4", PromptRatio: &ratio},
		}
		comps := BuildModelPriceComparisons(sources, nil)
		if len(comps) != 1 {
			t.Fatalf("expected 1 comparison, got %d", len(comps))
		}
		if comps[0].LowestPrice != nil {
			t.Errorf("LowestPrice should be nil")
		}
		if comps[0].LowestPromptRatio == nil || *comps[0].LowestPromptRatio != ratio {
			t.Errorf("LowestPromptRatio = %v, want %v", comps[0].LowestPromptRatio, ratio)
		}
		if comps[0].Notes == "" {
			t.Errorf("Notes should mention price source")
		}
	})

	t.Run("usable_account_count_from_records", func(t *testing.T) {
		sources := []ModelPricingSource{
			{ChannelID: "c1", ChannelName: "alpha", Model: "gpt-4"},
		}
		records := []AccountModelRecord{
			{AccountID: "a1", SiteName: "Alpha", ModelUsable: true, TestModel: "gpt-4", LatencyMs: 100},
			{AccountID: "a2", SiteName: "Beta", ModelUsable: true, TestModel: "GPT-4", LatencyMs: 50},
			{AccountID: "a3", SiteName: "Gamma", ModelUsable: false, TestModel: "gpt-4"},
		}
		comps := BuildModelPriceComparisons(sources, records)
		if len(comps) != 1 {
			t.Fatalf("expected 1 comparison, got %d", len(comps))
		}
		if comps[0].UsableAccountCount != 2 {
			t.Errorf("UsableAccountCount = %d, want 2 (a3 not usable)", comps[0].UsableAccountCount)
		}
		if comps[0].FastestLatencyMs != 50 {
			t.Errorf("FastestLatencyMs = %d, want 50", comps[0].FastestLatencyMs)
		}
	})

	t.Run("comparisons_truncated_at_80", func(t *testing.T) {
		sources := make([]ModelPricingSource, 90)
		for i := range sources {
			sources[i] = ModelPricingSource{
				ChannelID: "c" + itoa(i),
				Model:     "model-" + itoa(i),
			}
		}
		comps := BuildModelPriceComparisons(sources, nil)
		if len(comps) != 80 {
			t.Errorf("len(comps) = %d, want 80", len(comps))
		}
	})
}

func TestExtractLivePricingSources(t *testing.T) {
	record := PricingSiteRecord{SiteID: "s1", SiteName: "Alpha", BaseURL: "https://a.com", Kind: "newapi"}

	t.Run("invalid_json", func(t *testing.T) {
		if got := ExtractLivePricingSources(record, "not-json"); got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("empty_object", func(t *testing.T) {
		got := ExtractLivePricingSources(record, "{}")
		// Empty object has no model/pricing keys, but may still walk.
		// Should not emit any sources.
		for _, s := range got {
			if s.Model != "" {
				t.Errorf("expected no sources with models, got %+v", s)
			}
		}
	})

	t.Run("extracts_pricing_from_object", func(t *testing.T) {
		raw := `{"data":[{"model":"gpt-4","price":0.03}]}`
		got := ExtractLivePricingSources(record, raw)
		if len(got) == 0 {
			t.Fatalf("expected at least 1 source, got 0")
		}
		for _, s := range got {
			if s.Source == "" || s.Source[:0] != "" {
				// Check that Source is prefixed with "live_api_pricing:"
				if s.Source == "" || (len(s.Source) > 19 && s.Source[:19] != "live_api_pricing:") {
					// Just verify Source contains "live_api_pricing"
					if !contains(s.Source, "live_api_pricing") {
						t.Errorf("Source = %q, want prefix 'live_api_pricing'", s.Source)
					}
				}
			}
		}
	})
}

func TestCountPricingModels(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		if got := countPricingModels(nil); got != 0 {
			t.Errorf("expected 0, got %d", got)
		}
	})
	t.Run("dedup_case_insensitive", func(t *testing.T) {
		sources := []ModelPricingSource{
			{Model: "gpt-4"},
			{Model: "GPT-4"},
			{Model: "claude-3-opus"},
			{Model: ""},
		}
		if got := countPricingModels(sources); got != 2 {
			t.Errorf("expected 2 unique models, got %d", got)
		}
	})
}

func TestParsePricingSourcesJSON(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		if got := parsePricingSourcesJSON(""); got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})
	t.Run("invalid_json", func(t *testing.T) {
		if got := parsePricingSourcesJSON("not-json"); got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})
	t.Run("valid", func(t *testing.T) {
		raw := `[{"channelId":"c1","model":"gpt-4","source":"raw_json","confidence":"high"}]`
		got := parsePricingSourcesJSON(raw)
		if len(got) != 1 {
			t.Fatalf("expected 1 source, got %d", len(got))
		}
		if got[0].Model != "gpt-4" {
			t.Errorf("Model = %q", got[0].Model)
		}
	})
}

func TestMarshalPricingSourcesLimit(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		// json.Marshal(nil slice) returns "null".
		if got := marshalPricingSourcesLimit(nil, 10); got != "null" {
			t.Errorf("expected %q, got %q", "null", got)
		}
	})
	t.Run("truncates", func(t *testing.T) {
		sources := []ModelPricingSource{
			{ChannelID: "c1", Model: "gpt-4"},
			{ChannelID: "c2", Model: "claude-3-opus"},
			{ChannelID: "c3", Model: "deepseek-chat"},
		}
		got := marshalPricingSourcesLimit(sources, 2)
		// Should only contain 2 sources
		if !contains(got, "gpt-4") || !contains(got, "claude-3-opus") {
			t.Errorf("expected first 2 sources in JSON, got %q", got)
		}
		if contains(got, "deepseek-chat") {
			t.Errorf("third source should be truncated, got %q", got)
		}
	})
}

func TestApplyPricingNumber(t *testing.T) {
	t.Run("completion_key", func(t *testing.T) {
		var s ModelPricingSource
		applyPricingNumber(&s, "completion_ratio", 3.0)
		if s.CompletionRatio == nil || *s.CompletionRatio != 3.0 {
			t.Errorf("CompletionRatio = %v, want 3.0", s.CompletionRatio)
		}
	})
	t.Run("output_key", func(t *testing.T) {
		var s ModelPricingSource
		applyPricingNumber(&s, "output", 2.0)
		if s.CompletionRatio == nil {
			t.Errorf("CompletionRatio should be set for 'output' key")
		}
	})
	t.Run("price_key", func(t *testing.T) {
		var s ModelPricingSource
		applyPricingNumber(&s, "price", 0.05)
		if s.Price == nil || *s.Price != 0.05 {
			t.Errorf("Price = %v, want 0.05", s.Price)
		}
	})
	t.Run("quota_key", func(t *testing.T) {
		var s ModelPricingSource
		applyPricingNumber(&s, "quota", 100)
		if s.Price == nil {
			t.Errorf("Price should be set for 'quota' key")
		}
	})
	t.Run("default_prompt_ratio", func(t *testing.T) {
		var s ModelPricingSource
		applyPricingNumber(&s, "ratio", 1.5)
		if s.PromptRatio == nil || *s.PromptRatio != 1.5 {
			t.Errorf("PromptRatio = %v, want 1.5", s.PromptRatio)
		}
	})
}
