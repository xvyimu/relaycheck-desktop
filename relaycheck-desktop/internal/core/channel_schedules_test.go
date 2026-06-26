package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// parseAPIResponse unwraps the {"ok":true,"data":...} layer and unmarshals the data field into target.
func parseAPIResponse(t *testing.T, body string, target interface{}) {
	t.Helper()
	var wrapper struct {
		OK   bool            `json:"ok"`
		Data json.RawMessage `json:"data,omitempty"`
	}
	if err := json.Unmarshal([]byte(body), &wrapper); err != nil {
		t.Fatalf("unmarshal apiResponse wrapper: %v\nbody=%s", err, body)
	}
	if !wrapper.OK {
		t.Fatalf("apiResponse.ok is false, body=%s", body)
	}
	if len(wrapper.Data) == 0 || string(wrapper.Data) == "null" {
		return // nil data — target stays zero
	}
	if err := json.Unmarshal(wrapper.Data, target); err != nil {
		t.Fatalf("unmarshal apiResponse.Data: %v\ndata=%s\nbody=%s", err, string(wrapper.Data), body)
	}
}

func TestListChannelSchedules_Empty(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	items, err := app.listChannelSchedules(context.Background())
	if err != nil {
		t.Fatalf("listChannelSchedules on empty DB: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty list, got %d items", len(items))
	}
}

func TestUpsertChannelSchedule_CreatesAndUpdates(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	// First, create an upstream site to satisfy FK constraint
	_, err = app.db.ExecContext(context.Background(),
		`INSERT INTO upstream_sites (id, name, base_url, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		"site-1", "测试站点", "https://example.com", now(), now())
	if err != nil {
		t.Fatalf("create upstream site: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/scheduler/channel-schedules", strings.NewReader(
		`{"upstreamSiteId":"site-1","enabled":true,"checkinTime":"09:30","randomDelayMin":5,"randomDelayMax":15}`,
	))
	w := httptest.NewRecorder()
	app.handleChannelSchedules(w, req)

	body := w.Body.String()
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: body=%s", w.Code, body)
	}

	var data struct {
		OK        bool   `json:"ok"`
		NextRunAt string `json:"nextRunAt"`
	}
	parseAPIResponse(t, body, &data)
	if !data.OK {
		t.Fatalf("expected ok=true, got %v", data.OK)
	}
	if data.NextRunAt == "" {
		t.Fatalf("expected non-empty nextRunAt, data=%+v", data)
	}

	items, err := app.listChannelSchedules(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 schedule, got %d", len(items))
	}
	if items[0].CheckinTime != "09:30" {
		t.Fatalf("expected checkinTime 09:30, got %s", items[0].CheckinTime)
	}
	if !items[0].Enabled {
		t.Fatal("expected enabled=true")
	}

	// Update: change time
	req2 := httptest.NewRequest(http.MethodPut, "/api/scheduler/channel-schedules", strings.NewReader(
		`{"upstreamSiteId":"site-1","enabled":true,"checkinTime":"22:00"}`,
	))
	w2 := httptest.NewRecorder()
	app.handleChannelSchedules(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 on update, got %d: %s", w2.Code, w2.Body.String())
	}

	items2, _ := app.listChannelSchedules(context.Background())
	if len(items2) != 1 {
		t.Fatalf("expected 1 schedule after update, got %d", len(items2))
	}
	if items2[0].CheckinTime != "22:00" {
		t.Fatalf("expected checkinTime 22:00 after update, got %s", items2[0].CheckinTime)
	}
}

func TestHandleChannelSchedules_GET_Empty(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/scheduler/channel-schedules", nil)
	w := httptest.NewRecorder()
	app.handleChannelSchedules(w, req)

	body := w.Body.String()
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: body=%s", w.Code, body)
	}

	var data []ChannelSchedule
	parseAPIResponse(t, body, &data)
	if data != nil && len(data) != 0 {
		t.Fatalf("expected empty array, got %d items", len(data))
	}
}

func TestHandleChannelSchedules_GET_WithItems(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	// Create site + schedule
	_, err = app.db.ExecContext(context.Background(),
		`INSERT INTO upstream_sites (id, name, base_url, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		"s1", "数据站点", "https://example.com", now(), now())
	if err != nil {
		t.Fatalf("create upstream site: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/scheduler/channel-schedules", strings.NewReader(
		`{"upstreamSiteId":"s1","enabled":true,"checkinTime":"10:00"}`,
	))
	w := httptest.NewRecorder()
	app.handleChannelSchedules(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// GET should return 1 item
	getReq := httptest.NewRequest(http.MethodGet, "/api/scheduler/channel-schedules", nil)
	getW := httptest.NewRecorder()
	app.handleChannelSchedules(getW, getReq)

	var data []ChannelSchedule
	parseAPIResponse(t, getW.Body.String(), &data)
	if len(data) != 1 {
		t.Fatalf("expected 1 schedule, got %d", len(data))
	}
	if data[0].CheckinTime != "10:00" {
		t.Fatalf("expected checkinTime 10:00, got %s", data[0].CheckinTime)
	}
}

func TestHandleChannelSchedules_PUT_InvalidTime(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	req := httptest.NewRequest(http.MethodPut, "/api/scheduler/channel-schedules", strings.NewReader(
		`{"upstreamSiteId":"site-1","checkinTime":"25:00"}`,
	))
	w := httptest.NewRecorder()
	app.handleChannelSchedules(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid time, got %d", w.Code)
	}
}

func TestHandleNextRuns_ReturnsJobs(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/scheduler/next-runs", nil)
	w := httptest.NewRecorder()
	app.handleNextRuns(w, req)

	body := w.Body.String()
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: body=%s", w.Code, body)
	}

	var data struct {
		Items []NextRunItem `json:"items"`
	}
	parseAPIResponse(t, body, &data)
	if len(data.Items) < 2 {
		t.Fatalf("expected at least 2 scheduler jobs, got %d\nbody=%s", len(data.Items), body)
	}
	keys := make(map[string]bool)
	for _, item := range data.Items {
		keys[item.JobKey] = true
	}
	if !keys[schedulerJobCheckin] {
		t.Fatalf("missing checkin job, got keys: %v", keys)
	}
	if !keys[schedulerJobSync] {
		t.Fatalf("missing sync job, got keys: %v", keys)
	}
}

func TestHandleScheduleCalendar_ReturnsItems(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/scheduler/calendar", nil)
	w := httptest.NewRecorder()
	app.handleScheduleCalendar(w, req)

	body := w.Body.String()
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: body=%s", w.Code, body)
	}

	var data struct {
		Items []ScheduleCalendarItem `json:"items"`
	}
	parseAPIResponse(t, body, &data)
	if len(data.Items) < 14 {
		t.Fatalf("expected at least 14 calendar items (7 days x 2 jobs), got %d\nbody=%s", len(data.Items), body)
	}
}

func TestHandleScheduleCalendar_IncludesChannelSchedules(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	// Create an upstream site + schedule
	_, err = app.db.ExecContext(context.Background(),
		`INSERT INTO upstream_sites (id, name, base_url, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		"site-1", "测试站点", "https://example.com", now(), now())
	if err != nil {
		t.Fatalf("create upstream site: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/scheduler/channel-schedules", strings.NewReader(
		`{"upstreamSiteId":"site-1","enabled":true,"checkinTime":"09:30"}`,
	))
	w := httptest.NewRecorder()
	app.handleChannelSchedules(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("failed to create schedule: %d: %s", w.Code, w.Body.String())
	}

	// Get calendar
	calReq := httptest.NewRequest(http.MethodGet, "/api/scheduler/calendar", nil)
	calW := httptest.NewRecorder()
	app.handleScheduleCalendar(calW, calReq)

	body := calW.Body.String()
	var data struct {
		Items []ScheduleCalendarItem `json:"items"`
	}
	parseAPIResponse(t, body, &data)
	// Should have at least 14 global items + 7 channel schedule items = 21
	if len(data.Items) < 21 {
		t.Fatalf("expected at least 21 items (14 global + 7 per-site), got %d\nbody=%s", len(data.Items), body)
	}
}

func TestComputeNextRun_ReturnsFutureTime(t *testing.T) {
	nowTime := time.Now()
	result := computeNextRun("08:00", 0, 30)
	if result == "" {
		t.Fatal("expected non-empty next run")
	}

	parsed, err := time.Parse(time.RFC3339, result)
	if err != nil {
		t.Fatalf("parse next run: %v", err)
	}

	if parsed.Before(nowTime.Add(-24 * time.Hour)) {
		t.Fatalf("next run %s should not be more than 24h in the past", result)
	}
}

func TestComputeNextRun_DeterministicWithinDay(t *testing.T) {
	// Same inputs should give same result within the same second
	r1 := computeNextRun("14:30", 10, 20)
	r2 := computeNextRun("14:30", 10, 20)
	if r1 != r2 {
		t.Fatalf("expected deterministic result, got %s vs %s", r1, r2)
	}
}

func TestHandleChannelSchedules_RejectsEmptySiteID(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	req := httptest.NewRequest(http.MethodPut, "/api/scheduler/channel-schedules", strings.NewReader(
		`{"upstreamSiteId":"","checkinTime":"08:00"}`,
	))
	w := httptest.NewRecorder()
	app.handleChannelSchedules(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty site ID, got %d", w.Code)
	}
}

func TestHandleChannelSchedules_RejectsWrongMethod(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	req := httptest.NewRequest(http.MethodDelete, "/api/scheduler/channel-schedules", nil)
	w := httptest.NewRecorder()
	app.handleChannelSchedules(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for wrong method, got %d", w.Code)
	}
}
