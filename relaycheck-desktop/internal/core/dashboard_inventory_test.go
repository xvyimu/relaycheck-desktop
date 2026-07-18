package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDashboardInventoryCombinesInitialInventoryReads(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/inventory", nil)
	rec := httptest.NewRecorder()
	app.handleDashboardInventory(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var response struct {
		OK   bool                       `json:"ok"`
		Data DashboardInventoryOverview `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.OK {
		t.Fatalf("expected ok response: %s", rec.Body.String())
	}
	if response.Data.Channels == nil || response.Data.Sites == nil {
		t.Fatalf("inventory arrays must not be null: %#v", response.Data)
	}
	if response.Data.AccountSummary.AccountTotal != 0 || response.Data.AccountSummary.ProblemTotal != 0 {
		t.Fatalf("unexpected empty summary: %#v", response.Data.AccountSummary)
	}
}
