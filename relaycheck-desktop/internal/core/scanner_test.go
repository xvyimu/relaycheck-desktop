package core

import "testing"

func TestParseModelIDsAndChooseSpeedTestModel(t *testing.T) {
	models := parseModelIDs(`{"object":"list","data":[{"id":"embedding-1"},{"id":"deepseek-chat"},{"id":"gpt-4o-mini"}]}`)
	if len(models) != 3 {
		t.Fatalf("expected 3 models, got %d: %v", len(models), models)
	}
	if got := chooseModelForSpeedTest(models); got != "gpt-4o-mini" {
		t.Fatalf("expected preferred speed test model, got %s", got)
	}
}
