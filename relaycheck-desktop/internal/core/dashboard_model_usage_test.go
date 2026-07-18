package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDashboardModelUsageCombinesRadarReads(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/model-usage", nil)
	rec := httptest.NewRecorder()
	app.handleDashboardModelUsage(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var response struct {
		OK   bool                        `json:"ok"`
		Data DashboardModelUsageOverview `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.OK {
		t.Fatalf("expected ok response: %s", rec.Body.String())
	}
	if response.Data.Model.GeneratedAt == "" || response.Data.Pricing.GeneratedAt == "" || response.Data.Usage.GeneratedAt == "" {
		t.Fatalf("combined radar response is incomplete: %#v", response.Data)
	}
	if response.Data.Usage.Accounts == nil || response.Data.Usage.Sites == nil {
		t.Fatalf("usage arrays must not be null: %#v", response.Data.Usage)
	}
}
