package core

import (
	"strings"
	"testing"
)

func TestBuildModelOverviewAggregatesKeyModelCoverage(t *testing.T) {
	overview := cloneModelOverviewForTest([]accountModelRecord{
		{
			AccountID:     "acc-1",
			AccountName:   "fast key",
			SiteID:        "site-1",
			SiteName:      "Alpha Relay",
			BaseURL:       "https://alpha.example",
			Kind:          "newapi",
			Fingerprint:   "key_fast",
			Status:        "valid",
			ModelCount:    2,
			SampleModels:  []string{"gpt-4o-mini", "deepseek-chat"},
			TestModel:     "gpt-4o-mini",
			ModelUsable:   true,
			LatencyMs:     420,
			LastCheckedAt: "2026-06-20T10:00:00Z",
		},
		{
			AccountID:    "acc-2",
			AccountName:  "slow key",
			SiteID:       "site-2",
			SiteName:     "Beta Relay",
			BaseURL:      "https://beta.example",
			Kind:         "sub2api",
			Fingerprint:  "key_slow",
			Status:       "expired",
			ModelCount:   1,
			SampleModels: []string{"gpt-4o-mini"},
			TestModel:    "gpt-4o-mini",
			LatencyMs:    900,
		},
	})

	if overview.AccountCount != 2 {
		t.Fatalf("expected 2 key accounts, got %d", overview.AccountCount)
	}
	if overview.ValidKeyCount != 1 {
		t.Fatalf("expected 1 valid key, got %d", overview.ValidKeyCount)
	}
	if overview.UsableModelCount != 1 {
		t.Fatalf("expected 1 usable model account, got %d", overview.UsableModelCount)
	}
	if overview.FastestLatencyMs != 420 {
		t.Fatalf("expected fastest latency 420, got %d", overview.FastestLatencyMs)
	}
	if len(overview.Models) == 0 || overview.Models[0].Model != "gpt-4o-mini" {
		t.Fatalf("expected gpt-4o-mini to be first, got %+v", overview.Models)
	}
	if overview.Models[0].AccountCount != 2 || overview.Models[0].ValidKeyCount != 1 || overview.Models[0].UsableCount != 1 {
		t.Fatalf("unexpected model aggregation: %+v", overview.Models[0])
	}
	if len(overview.PriceHints) == 0 || overview.PriceHints[0].Vendor == "" {
		t.Fatalf("expected price hints for known models, got %+v", overview.PriceHints)
	}
}

func TestExtractModelPricingSourcesFromImportedChannelRawJSON(t *testing.T) {
	raw := `{
		"id": 7,
		"name": "relay",
		"models": "gpt-4o-mini,deepseek-chat",
		"config": "{\"model_ratio\":{\"gpt-4o-mini\":0.25,\"deepseek-chat\":0.1},\"completion_ratio\":{\"gpt-4o-mini\":4}}",
		"model_mapping": "{\"gpt-4o-mini\":\"openai/gpt-4o-mini\"}"
	}`

	sources := extractModelPricingSources("ch-1", "Relay", "https://relay.example", "newapi", raw)
	if len(sources) < 3 {
		t.Fatalf("expected pricing and mapping sources, got %+v", sources)
	}

	foundPromptRatio := false
	foundCompletionRatio := false
	foundMapping := false
	for _, source := range sources {
		if source.Model == "gpt-4o-mini" && source.PromptRatio != nil && *source.PromptRatio == 0.25 {
			foundPromptRatio = true
		}
		if source.Model == "gpt-4o-mini" && source.CompletionRatio != nil && *source.CompletionRatio == 4 {
			foundCompletionRatio = true
		}
		if source.Model == "gpt-4o-mini" && source.UpstreamModel == "openai/gpt-4o-mini" {
			foundMapping = true
		}
	}
	if !foundPromptRatio || !foundCompletionRatio || !foundMapping {
		t.Fatalf("missing expected pricing sources: prompt=%v completion=%v mapping=%v sources=%+v", foundPromptRatio, foundCompletionRatio, foundMapping, sources)
	}
}

func TestExtractLivePricingSourcesFromModelKeyedPricing(t *testing.T) {
	raw := `{
		"gpt-4o-mini": {"prompt_ratio": 0.2, "completion_ratio": 0.8, "currency": "quota"},
		"deepseek-chat": 0.1
	}`

	sources := extractLivePricingSources(pricingSiteRecord{
		SiteID:   "site-1",
		SiteName: "Live Relay",
		BaseURL:  "https://relay.example",
		Kind:     "newapi",
	}, raw)

	if len(sources) < 2 {
		t.Fatalf("expected live pricing sources, got %+v", sources)
	}
	foundMini := false
	foundDeepSeek := false
	for _, source := range sources {
		if source.Model == "gpt-4o-mini" && source.PromptRatio != nil && *source.PromptRatio == 0.2 && strings.HasPrefix(source.Source, "live_api_pricing:") {
			foundMini = true
		}
		if source.Model == "deepseek-chat" && source.PromptRatio != nil && *source.PromptRatio == 0.1 {
			foundDeepSeek = true
		}
	}
	if !foundMini || !foundDeepSeek {
		t.Fatalf("missing expected live sources: mini=%v deepseek=%v sources=%+v", foundMini, foundDeepSeek, sources)
	}
}

func TestBuildPricingOverviewComparesPriceLatencyAndUsability(t *testing.T) {
	promptRatio := 0.25
	overview := buildPricingOverview([]modelPricingSource{
		{
			ChannelID:   "site:1",
			ChannelName: "Alpha",
			Kind:        "newapi",
			Model:       "gpt-4o-mini",
			Source:      "live_api_pricing:prompt_ratio",
			FieldPath:   "api_pricing.gpt-4o-mini",
			PromptRatio: &promptRatio,
			Confidence:  "high",
		},
	}, []sitePricingCacheItem{{SiteID: "site-1", SiteName: "Alpha", Status: "success", SourceCount: 1, ModelCount: 1}}, []accountModelRecord{
		{
			AccountID:    "acc-1",
			AccountName:  "usable",
			SiteID:       "site-1",
			SiteName:     "Alpha",
			ModelUsable:  true,
			TestModel:    "gpt-4o-mini",
			SampleModels: []string{"gpt-4o-mini"},
			LatencyMs:    320,
		},
	})

	if overview.LiveCacheCount != 1 {
		t.Fatalf("expected live cache count, got %+v", overview)
	}
	if len(overview.Comparisons) == 0 {
		t.Fatalf("expected comparison rows, got %+v", overview)
	}
	row := overview.Comparisons[0]
	if row.Model != "gpt-4o-mini" || row.UsableAccountCount != 1 || row.FastestLatencyMs != 320 {
		t.Fatalf("unexpected comparison row: %+v", row)
	}
	if row.LowestPromptRatio == nil || *row.LowestPromptRatio != 0.25 {
		t.Fatalf("expected lowest prompt ratio 0.25, got %+v", row)
	}
}
