package core

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListUpstreamSitesHidesGlobalScheduleRecord(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/upstream-sites", nil)
	w := httptest.NewRecorder()
	app.listUpstreamSites(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var items []UpstreamSite
	parseAPIResponse(t, w.Body.String(), &items)
	for _, item := range items {
		if item.ID == globalScheduleSiteID {
			t.Fatalf("global schedule record leaked into upstream sites: %#v", item)
		}
	}
}
