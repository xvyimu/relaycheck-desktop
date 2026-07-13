package core

import (
	"database/sql"
	"testing"
)

func TestIsLowBalance(t *testing.T) {
	if isLowBalance(sql.NullFloat64{}, "usd") {
		t.Fatal("invalid balance should not be low")
	}
	if !isLowBalance(sql.NullFloat64{Float64: 1, Valid: true}, "usd") {
		t.Fatal("usd 1 should be low")
	}
	if isLowBalance(sql.NullFloat64{Float64: 10, Valid: true}, "cny") {
		t.Fatal("cny 10 should not be low")
	}
	if !isLowBalance(sql.NullFloat64{Float64: 100, Valid: true}, "quota") {
		t.Fatal("quota 100 should be low")
	}
	if !isLowBalance(sql.NullFloat64{Float64: 50, Valid: true}, "token") {
		t.Fatal("token 50 should be low")
	}
	if !isLowBalance(sql.NullFloat64{Float64: 1, Valid: true}, "other") {
		t.Fatal("default unit uses 5 threshold")
	}
}

func TestUsageTrend(t *testing.T) {
	if got := usageTrend(-1); got != "down" {
		t.Fatalf("down, got %q", got)
	}
	if got := usageTrend(1); got != "up" {
		t.Fatalf("up, got %q", got)
	}
	if got := usageTrend(0); got != "flat" {
		t.Fatalf("flat, got %q", got)
	}
}

func TestLimitUsageItems(t *testing.T) {
	accounts := []usageAccountItem{{}, {}, {}}
	if got := limitUsageAccountItems(accounts, 2); len(got) != 2 {
		t.Fatalf("limit accounts, got %d", len(got))
	}
	if got := limitUsageAccountItems(accounts, 10); len(got) != 3 {
		t.Fatalf("under limit accounts, got %d", len(got))
	}
	sites := []usageSiteItem{{}, {}}
	if got := limitUsageSiteItems(sites, 1); len(got) != 1 {
		t.Fatalf("limit sites, got %d", len(got))
	}
}

func TestPriceLevelAndModelHint(t *testing.T) {
	if got := priceLevelBySuffix("gpt-4o-mini"); got != "cheap" {
		t.Fatalf("mini -> cheap, got %q", got)
	}
	if got := priceLevelBySuffix("claude-3-pro"); got != "standard" {
		t.Fatalf("pro -> standard, got %q", got)
	}
	if got := priceLevelBySuffix("claude-opus"); got != "high" {
		t.Fatalf("opus -> high, got %q", got)
	}
	if got := priceLevelBySuffix("base"); got != "unknown" {
		t.Fatalf("default unknown, got %q", got)
	}

	if _, ok := inferModelPriceHint(""); ok {
		t.Fatal("empty model should fail")
	}
	if _, ok := inferModelPriceHint("totally-unknown-model"); ok {
		t.Fatal("unknown vendor should fail")
	}
	hint, ok := inferModelPriceHint("gpt-4o-mini")
	if !ok || hint.Vendor != "OpenAI" || hint.PriceLevel != "cheap" {
		t.Fatalf("openai mini hint: %+v ok=%v", hint, ok)
	}
	hint, ok = inferModelPriceHint("claude-3-5-sonnet")
	if !ok || hint.Vendor != "Anthropic" {
		t.Fatalf("anthropic hint: %+v ok=%v", hint, ok)
	}
	hint, ok = inferModelPriceHint("gemini-1.5-flash")
	if !ok || hint.Vendor != "Google" || hint.PriceLevel != "cheap" {
		t.Fatalf("google hint: %+v ok=%v", hint, ok)
	}
	hint, ok = inferModelPriceHint("deepseek-chat")
	if !ok || hint.Vendor != "DeepSeek" || hint.PriceLevel != "low" {
		t.Fatalf("deepseek hint: %+v ok=%v", hint, ok)
	}
	hint, ok = inferModelPriceHint("qwen-max")
	if !ok || hint.Vendor != "Qwen" {
		t.Fatalf("qwen hint: %+v ok=%v", hint, ok)
	}
	hint, ok = inferModelPriceHint("glm-4")
	if !ok || hint.Vendor != "Zhipu" {
		t.Fatalf("zhipu hint: %+v ok=%v", hint, ok)
	}
	hint, ok = inferModelPriceHint("doubao-pro")
	if !ok || hint.Vendor != "ByteDance" {
		t.Fatalf("doubao hint: %+v ok=%v", hint, ok)
	}
	hint, ok = inferModelPriceHint("moonshot-v1")
	if !ok || hint.Vendor != "Moonshot" {
		t.Fatalf("moonshot hint: %+v ok=%v", hint, ok)
	}
	hint, ok = inferModelPriceHint("kimi-k2")
	if !ok || hint.Vendor != "Moonshot" {
		t.Fatalf("kimi hint: %+v ok=%v", hint, ok)
	}
}

func TestAppendUniqueStringAndModelCoverageLimit(t *testing.T) {
	var values []string
	appendUniqueString(&values, " a ", 2)
	appendUniqueString(&values, "A", 2)
	appendUniqueString(&values, "", 2)
	appendUniqueString(&values, "b", 2)
	appendUniqueString(&values, "c", 2)
	if len(values) != 2 || values[0] != "a" || values[1] != "b" {
		t.Fatalf("append unique, got %#v", values)
	}
	items := []modelCoverageItem{{}, {}, {}}
	if got := limitModelCoverageItems(items, 1); len(got) != 1 {
		t.Fatalf("limit model coverage, got %d", len(got))
	}
}

func TestNormalizeOfficialProviderSite(t *testing.T) {
	item := &UpstreamSite{BaseURL: "https://example.com", HealthStatus: "unknown"}
	normalizeOfficialProviderSite(item)
	if item.Kind == "official_provider" {
		// only if base URL matches official list; example.com should not
		t.Fatal("example.com should not become official_provider")
	}
}
