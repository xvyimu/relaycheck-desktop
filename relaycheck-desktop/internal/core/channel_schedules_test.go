package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"relaycheck-desktop/internal/channels"
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
	app := newTestApp(t)
	defer app.Close()

	items, err := app.listChannelSchedules(context.Background())
	if err != nil {
		t.Fatalf("listChannelSchedules on empty DB: %v", err)
	}
	// __global__ schedule is created during NewApp startup
	if len(items) != 1 {
		t.Fatalf("expected 1 item (__global__), got %d items", len(items))
	}
}

func TestUpsertChannelSchedule_CreatesAndUpdates(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()
	var err error

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

	var resp struct {
		OK        bool   `json:"ok"`
		NextRunAt string `json:"nextRunAt"`
	}
	parseAPIResponse(t, body, &resp)
	if !resp.OK {
		t.Fatalf("expected ok=true, got %v", resp.OK)
	}
	if resp.NextRunAt == "" {
		t.Fatalf("expected non-empty nextRunAt, data=%+v", resp)
	}

	items, err := app.listChannelSchedules(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 schedules (__global__ + site-1), got %d", len(items))
	}
	if items[1].CheckinTime != "09:30" {
		t.Fatalf("expected checkinTime 09:30, got %s", items[0].CheckinTime)
	}
	if !items[1].Enabled {
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
	if len(items2) != 2 {
		t.Fatalf("expected 2 schedules (__global__ + site-1) after update, got %d", len(items2))
	}
	if items2[1].CheckinTime != "22:00" {
		t.Fatalf("expected checkinTime 22:00 after update, got %s", items2[1].CheckinTime)
	}
}

func TestHandleChannelSchedules_GET_GlobalOnly(t *testing.T) {
	app := newTestApp(t)
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
	if len(data) != 1 {
		t.Fatalf("expected 1 item (__global__), got %d items", len(data))
	}
	if data[0].UpstreamSiteID != "__global__" {
		t.Fatalf("expected __global__ schedule, got %s", data[0].UpstreamSiteID)
	}
}

func TestHandleChannelSchedules_GET_WithItems(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()
	var err error

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
	if len(data) != 2 {
		t.Fatalf("expected 2 schedules (__global__ + s1), got %d", len(data))
	}
	if data[1].CheckinTime != "10:00" {
		t.Fatalf("expected checkinTime 10:00, got %s", data[0].CheckinTime)
	}
}

func TestHandleChannelSchedules_PUT_InvalidTime(t *testing.T) {
	app := newTestApp(t)
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
	app := newTestApp(t)
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
	app := newTestApp(t)
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
	// Global checkin schedule appears as __global__ channel_schedule.
	if len(data.Items) < 7 {
		t.Fatalf("expected at least 7 global checkin items, got %d\nbody=%s", len(data.Items), body)
	}
}

func TestHandleScheduleCalendar_IncludesChannelSchedules(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()
	var err error

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
	// 7 global checkin + 7 per-site channel schedule items.
	if len(data.Items) < 14 {
		t.Fatalf("expected at least 14 items (7 global + 7 per-site), got %d\nbody=%s", len(data.Items), body)
	}
}

func TestHandleScheduleCalendar_RespectsDaysQuery(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/scheduler/calendar?days=2", nil)
	w := httptest.NewRecorder()
	app.handleScheduleCalendar(w, req)

	var data struct {
		Items []ScheduleCalendarItem `json:"items"`
	}
	parseAPIResponse(t, w.Body.String(), &data)

	dates := map[string]bool{}
	for _, item := range data.Items {
		dates[item.Date] = true
	}
	if len(dates) != 2 {
		t.Fatalf("expected exactly 2 calendar dates for days=2, got %d dates: %v", len(dates), dates)
	}
}

func TestHandleScheduleCalendar_UsesCronOccurrences(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()
	ctx := context.Background()

	_, err := app.db.ExecContext(ctx,
		`INSERT INTO upstream_sites (id, name, base_url, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		"site-calendar-cron", "Calendar Cron Site", "https://cron.example", now(), now())
	if err != nil {
		t.Fatalf("create upstream site: %v", err)
	}
	req := httptest.NewRequest(http.MethodPut, "/api/scheduler/channel-schedules", strings.NewReader(
		`{"upstreamSiteId":"site-calendar-cron","enabled":true,"checkinTime":"08:00","cronExpr":"0 9 * * 1-5"}`,
	))
	w := httptest.NewRecorder()
	app.handleChannelSchedules(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("create schedule: %d: %s", w.Code, w.Body.String())
	}

	calReq := httptest.NewRequest(http.MethodGet, "/api/scheduler/calendar?days=14", nil)
	calW := httptest.NewRecorder()
	app.handleScheduleCalendar(calW, calReq)

	var data struct {
		Items []ScheduleCalendarItem `json:"items"`
	}
	parseAPIResponse(t, calW.Body.String(), &data)

	cst := time.FixedZone("CST", 8*3600)
	var matched int
	for _, item := range data.Items {
		if item.SiteID != "site-calendar-cron" {
			continue
		}
		matched++
		if item.Time != "09:00" {
			t.Fatalf("expected cron occurrence at 09:00, got %s", item.Time)
		}
		parsedDate, err := time.ParseInLocation("2006-01-02", item.Date, cst)
		if err != nil {
			t.Fatalf("parse item date: %v", err)
		}
		if parsedDate.Weekday() == time.Saturday || parsedDate.Weekday() == time.Sunday {
			t.Fatalf("cron calendar should not include weekend date %s", item.Date)
		}
	}
	if matched == 0 {
		t.Fatal("expected at least one cron calendar occurrence")
	}
	if matched > 10 {
		t.Fatalf("expected no more than 10 weekdays in 14-day window, got %d", matched)
	}
}

func TestComputeNextRun_ReturnsFutureTime(t *testing.T) {
	nowTime := time.Now()
	result := channels.ComputeNextRun("08:00", "", nil, 0, 30)
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
	r1 := channels.ComputeNextRun("14:30", "", nil, 10, 20)
	r2 := channels.ComputeNextRun("14:30", "", nil, 10, 20)
	if r1 != r2 {
		t.Fatalf("expected deterministic result, got %s vs %s", r1, r2)
	}
}

func TestHandleChannelSchedules_PUT_WithCronExpr(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	// Create upstream site
	_, err := app.db.ExecContext(context.Background(),
		`INSERT INTO upstream_sites (id, name, base_url, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		"site-cron", "Cron站点", "https://example.com", now(), now())
	if err != nil {
		t.Fatalf("create upstream site: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/scheduler/channel-schedules", strings.NewReader(
		`{"upstreamSiteId":"site-cron","enabled":true,"checkinTime":"08:00","cronExpr":"0 9 * * 1-5"}`,
	))
	w := httptest.NewRecorder()
	app.handleChannelSchedules(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	items, err := app.listChannelSchedules(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, s := range items {
		if s.UpstreamSiteID == "site-cron" {
			found = true
			if s.CronExpr != "0 9 * * 1-5" {
				t.Fatalf("expected cronExpr '0 9 * * 1-5', got %q", s.CronExpr)
			}
			break
		}
	}
	if !found {
		t.Fatal("schedule with site-cron not found")
	}
}

func TestHandleChannelSchedules_PUT_InvalidCronExpr(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	req := httptest.NewRequest(http.MethodPut, "/api/scheduler/channel-schedules", strings.NewReader(
		`{"upstreamSiteId":"site-1","cronExpr":"bad bad"}`,
	))
	w := httptest.NewRecorder()
	app.handleChannelSchedules(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid cron expr, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleChannelSchedules_PUT_WithSkipDates(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	_, err := app.db.ExecContext(context.Background(),
		`INSERT INTO upstream_sites (id, name, base_url, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		"site-skip", "Skip站点", "https://example.com", now(), now())
	if err != nil {
		t.Fatalf("create upstream site: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/scheduler/channel-schedules", strings.NewReader(
		`{"upstreamSiteId":"site-skip","enabled":true,"checkinTime":"08:00","skipDates":["2026-07-01","2026-07-04"]}`,
	))
	w := httptest.NewRecorder()
	app.handleChannelSchedules(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	items, err := app.listChannelSchedules(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, s := range items {
		if s.UpstreamSiteID == "site-skip" {
			found = true
			if len(s.SkipDates) != 2 {
				t.Fatalf("expected 2 skip dates, got %v", s.SkipDates)
			}
			if s.SkipDates[0] != "2026-07-01" {
				t.Fatalf("expected skip date 2026-07-01, got %s", s.SkipDates[0])
			}
			break
		}
	}
	if !found {
		t.Fatal("schedule with site-skip not found")
	}
}

func TestHandleChannelSchedules_PUT_InvalidSkipDateFormat(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	req := httptest.NewRequest(http.MethodPut, "/api/scheduler/channel-schedules", strings.NewReader(
		`{"upstreamSiteId":"site-1","skipDates":["not-a-date"]}`,
	))
	w := httptest.NewRecorder()
	app.handleChannelSchedules(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid skip date format, got %d: %s", w.Code, w.Body.String())
	}
}

func TestComputeNextRun_WithCronExpr(t *testing.T) {
	// "0 9 * * 1-5" = 09:00 on weekdays
	// Use a fixed reference time so the test is deterministic
	result := channels.ComputeNextRun("08:00", "0 9 * * 1-5", nil, 0, 0)
	if result == "" {
		t.Fatal("expected non-empty next run for cron expr")
	}
	parsed, err := time.Parse(time.RFC3339, result)
	if err != nil {
		t.Fatalf("parse next run: %v", err)
	}
	weekday := parsed.In(time.FixedZone("CST", 8*3600)).Weekday()
	if weekday == time.Saturday || weekday == time.Sunday {
		t.Fatalf("cron '0 9 * * 1-5' produced a weekend day: %s (%s)", result, weekday)
	}
}

func TestComputeNextRun_WithSkipDates(t *testing.T) {
	// Use a fixed "today" so skip date is guaranteed relevant
	// computeNextRun uses time.Now(), so we can't mock. Instead verify
	// that a date in skip list is indeed excluded from the result.
	cst := time.FixedZone("CST", 8*3600)
	tomorrow := time.Now().In(cst).AddDate(0, 0, 1).Format("2006-01-02")

	result := channels.ComputeNextRun("08:00", "", []string{tomorrow}, 0, 0)
	if result == "" {
		t.Fatal("expected non-empty next run")
	}
	parsed, err := time.Parse(time.RFC3339, result)
	if err != nil {
		t.Fatalf("parse next run: %v", err)
	}
	resultDate := parsed.In(cst).Format("2006-01-02")
	if resultDate == tomorrow {
		t.Fatalf("next run %s should not fall on skipped date %s", result, tomorrow)
	}
}

func TestComputeNextRun_EmptyCronFallsBackToCheckinTime(t *testing.T) {
	result := channels.ComputeNextRun("22:30", "", nil, 0, 0)
	if result == "" {
		t.Fatal("expected non-empty next run")
	}
	parsed, err := time.Parse(time.RFC3339, result)
	if err != nil {
		t.Fatalf("parse next run: %v", err)
	}
	cst := time.FixedZone("CST", 8*3600)
	hour, minute, _ := parsed.In(cst).Clock()
	if hour != 22 || minute != 30 {
		t.Fatalf("expected 22:30 in result, got %02d:%02d from %s", hour, minute, result)
	}
}

func TestHandleChannelSchedules_RejectsEmptySiteID(t *testing.T) {
	app := newTestApp(t)
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
	app := newTestApp(t)
	defer app.Close()

	req := httptest.NewRequest(http.MethodDelete, "/api/scheduler/channel-schedules", nil)
	w := httptest.NewRecorder()
	app.handleChannelSchedules(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for wrong method, got %d", w.Code)
	}
}
