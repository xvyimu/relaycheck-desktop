package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDashboardOpsCombinesInitialOperationalReads(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/ops", nil)
	rec := httptest.NewRecorder()
	app.handleDashboardOps(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var response struct {
		OK   bool                 `json:"ok"`
		Data DashboardOpsOverview `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.OK {
		t.Fatalf("expected ok response: %s", rec.Body.String())
	}
	if response.Data.ActionCenter.GeneratedAt == "" || len(response.Data.ActionCenter.Items) == 0 {
		t.Fatalf("action center missing from combined response: %#v", response.Data.ActionCenter)
	}
	if response.Data.Diagnostics.GeneratedAt == "" || len(response.Data.Diagnostics.Items) == 0 {
		t.Fatalf("diagnostics missing from combined response: %#v", response.Data.Diagnostics)
	}
	if response.Data.Checkins.GeneratedAt == "" {
		t.Fatalf("checkin status missing from combined response: %#v", response.Data.Checkins)
	}
	if response.Data.Notifications.Items == nil {
		t.Fatal("notification items must be an empty array, not null")
	}
}
