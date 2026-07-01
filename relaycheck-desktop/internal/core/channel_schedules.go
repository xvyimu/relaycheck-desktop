package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"relaycheck-desktop/internal/channels"
)

// ChannelSchedule represents per-site checkin scheduling configuration.
type ChannelSchedule struct {
	ID             string   `json:"id"`
	UpstreamSiteID string   `json:"upstreamSiteId"`
	SiteName       string   `json:"siteName,omitempty"`
	Enabled        bool     `json:"enabled"`
	CheckinTime    string   `json:"checkinTime"` // "HH:MM" — fallback when cron_expr is empty
	CronExpr       string   `json:"cronExpr"`    // cron expression, e.g. "0 8 * * *" — overrides checkinTime when set
	SkipDates      []string `json:"skipDates"`   // ISO dates "YYYY-MM-DD" to skip
	RandomDelayMin int      `json:"randomDelayMin"`
	RandomDelayMax int      `json:"randomDelayMax"`
	LastRunAt      string   `json:"lastRunAt,omitempty"`
	NextRunAt      string   `json:"nextRunAt,omitempty"`
	CreatedAt      string   `json:"createdAt"`
	UpdatedAt      string   `json:"updatedAt"`
}

// ScheduleCalendarItem represents one upcoming scheduled run in the calendar view.
type ScheduleCalendarItem struct {
	Date     string `json:"date"` // "YYYY-MM-DD"
	Time     string `json:"time"` // "HH:MM"
	SiteName string `json:"siteName"`
	SiteID   string `json:"siteId"`
	JobType  string `json:"jobType"` // "checkin" | "sync"
	Enabled  bool   `json:"enabled"`
}

// NextRunList is the response for /api/scheduler/next-runs.
type NextRunList struct {
	GeneratedAt string        `json:"generatedAt"`
	Items       []NextRunItem `json:"items"`
}

type NextRunItem struct {
	JobKey       string `json:"jobKey"`
	Label        string `json:"label"`
	NextRunAt    string `json:"nextRunAt"`
	NextRunInSec int64  `json:"nextRunInSeconds"`
	Status       string `json:"status"`
	SiteID       string `json:"siteId,omitempty"`
	SiteName     string `json:"siteName,omitempty"`
}

const (
	// Virtual site ID for the global checkin schedule stored in channel_schedules.
	globalScheduleSiteID = channels.GlobalScheduleSiteID
)

func (a *App) handleChannelSchedules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	ctx := r.Context()

	if r.Method == http.MethodGet {
		schedules, err := a.listChannelSchedules(ctx)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, schedules)
		return
	}

	// PUT: upsert schedule for a site
	var body struct {
		UpstreamSiteID string   `json:"upstreamSiteId"`
		Enabled        *bool    `json:"enabled"`
		CheckinTime    string   `json:"checkinTime"`
		CronExpr       string   `json:"cronExpr"`
		SkipDates      []string `json:"skipDates"`
		RandomDelayMin *int     `json:"randomDelayMin"`
		RandomDelayMax *int     `json:"randomDelayMax"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败")
		return
	}
	if body.UpstreamSiteID == "" {
		writeError(w, http.StatusBadRequest, "缺少站点 ID")
		return
	}
	if body.CheckinTime == "" {
		body.CheckinTime = "08:00"
	}
	// Validate checkinTime format "HH:MM" when cron_expr is empty
	if body.CronExpr == "" {
		var hour, minute int
		if _, err := fmt.Sscanf(body.CheckinTime, "%d:%d", &hour, &minute); err != nil {
			writeError(w, http.StatusBadRequest, "签到时间格式无效，应为 HH:MM")
			return
		}
		if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
			writeError(w, http.StatusBadRequest, "签到时间范围无效，小时 0-23，分钟 0-59")
			return
		}
	} else {
		// Validate cron expression
		if err := channels.ValidateCronExpr(body.CronExpr); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("Cron 表达式无效: %s", err.Error()))
			return
		}
	}

	// Validate skip dates
	skipDatesJSON := "[]"
	if len(body.SkipDates) > 0 {
		for _, d := range body.SkipDates {
			if _, err := time.Parse("2006-01-02", d); err != nil {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("跳过日期格式无效 %q，应为 YYYY-MM-DD", d))
				return
			}
		}
		data, err := json.Marshal(body.SkipDates)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "序列化跳过日期失败")
			return
		}
		skipDatesJSON = string(data)
	}

	enabled := true
	if body.Enabled != nil {
		enabled = *body.Enabled
	}
	rdMin := 0
	if body.RandomDelayMin != nil {
		rdMin = *body.RandomDelayMin
	}
	rdMax := 30
	if body.RandomDelayMax != nil {
		rdMax = *body.RandomDelayMax
	}

	nextRun := channels.ComputeNextRun(body.CheckinTime, body.CronExpr, body.SkipDates, rdMin, rdMax)

	_, err := a.db.ExecContext(ctx, `
		INSERT INTO channel_schedules (id, upstream_site_id, enabled, checkin_time, cron_expr, skip_dates_json, random_delay_min, random_delay_max, next_run_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			enabled=excluded.enabled,
			checkin_time=excluded.checkin_time,
			cron_expr=excluded.cron_expr,
			skip_dates_json=excluded.skip_dates_json,
			random_delay_min=excluded.random_delay_min,
			random_delay_max=excluded.random_delay_max,
			next_run_at=excluded.next_run_at,
			updated_at=excluded.updated_at
	`, body.UpstreamSiteID, body.UpstreamSiteID, enabled, body.CheckinTime, body.CronExpr, skipDatesJSON, rdMin, rdMax, nextRun, now(), now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "nextRunAt": nextRun})
}

// listChannelSchedules is the *App forwarder for
// channels.Service.ListChannelSchedules. Converts the channels mirror type
// back to core.ChannelSchedule so existing callers (handleChannelSchedules,
// handleScheduleCalendar, handleNextRuns) are unchanged.
func (a *App) listChannelSchedules(ctx context.Context) ([]ChannelSchedule, error) {
	items, err := a.channelsService.ListChannelSchedules(ctx)
	if err != nil {
		return nil, err
	}
	return schedulesToCore(items), nil
}

func (a *App) handleScheduleCalendar(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	ctx := r.Context()

	schedules, err := a.listChannelSchedules(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	cst := time.FixedZone("CST", 8*3600)
	now := time.Now().In(cst)
	days := channels.ParseCalendarDays(r, 7)
	windowEnd := now.AddDate(0, 0, days)
	items := make([]ScheduleCalendarItem, 0, len(schedules)*days)

	for _, sched := range schedules {
		mirrorItems := channels.CalendarItemsForSchedule(scheduleToMirror(sched), now, windowEnd, days)
		items = append(items, calendarItemsToCore(mirrorItems)...)
	}
	if item, ok := a.nextSyncCalendarItem(ctx, now, windowEnd); ok {
		items = append(items, item)
	}

	// Sort by date+time
	sort.Slice(items, func(i, j int) bool {
		return items[i].Date+items[i].Time < items[j].Date+items[j].Time
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"generatedAt": now.Format(time.RFC3339),
		"items":       items,
	})
}

// nextSyncCalendarItem is the *App forwarder for
// channels.Service.NextSyncCalendarItem. Converts the channels mirror type
// back to core.ScheduleCalendarItem so handleScheduleCalendar is unchanged.
func (a *App) nextSyncCalendarItem(ctx context.Context, now time.Time, windowEnd time.Time) (ScheduleCalendarItem, bool) {
	item, ok := a.channelsService.NextSyncCalendarItem(ctx, now, windowEnd)
	if !ok {
		return ScheduleCalendarItem{}, false
	}
	return calendarItemFromMirror(item), true
}

func (a *App) handleNextRuns(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	ctx := r.Context()
	status := a.buildSchedulerStatus(ctx)

	var items []NextRunItem
	nowTime := nowCST()
	for _, job := range status.Jobs {
		var nextRunInSec int64 = -1
		if job.NextRunAt != "" {
			if t, err := time.Parse(time.RFC3339, job.NextRunAt); err == nil {
				nextRunInSec = int64(t.Sub(nowTime).Seconds())
				if nextRunInSec < 0 {
					nextRunInSec = 0
				}
			}
		}
		items = append(items, NextRunItem{
			JobKey:       job.Key,
			Label:        job.Label,
			NextRunAt:    job.NextRunAt,
			NextRunInSec: nextRunInSec,
			Status:       job.Status,
		})
	}

	// Add per-channel schedules
	schedules, _ := a.listChannelSchedules(ctx)
	for _, sched := range schedules {
		if !sched.Enabled || sched.NextRunAt == "" {
			continue
		}
		var nextRunInSec int64 = -1
		if t, err := time.Parse(time.RFC3339, sched.NextRunAt); err == nil {
			nextRunInSec = int64(t.Sub(nowTime).Seconds())
			if nextRunInSec < 0 {
				nextRunInSec = 0
			}
		}
		label := sched.SiteName + " 签到"
		if sched.CronExpr != "" {
			label = sched.SiteName + " 签到(" + sched.CronExpr + ")"
		}
		items = append(items, NextRunItem{
			JobKey:       "channel." + sched.UpstreamSiteID,
			Label:        label,
			NextRunAt:    sched.NextRunAt,
			NextRunInSec: nextRunInSec,
			Status:       "scheduled",
			SiteID:       sched.UpstreamSiteID,
			SiteName:     sched.SiteName,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].NextRunAt < items[j].NextRunAt
	})

	writeJSON(w, http.StatusOK, NextRunList{
		GeneratedAt: nowTime.Format(time.RFC3339),
		Items:       items,
	})
}

// ensureGlobalScheduleRecord is the *App forwarder for
// channels.Service.EnsureGlobalScheduleRecord. Delegates to the channels
// service so the SQL round-tripping lives in one place.
func (a *App) ensureGlobalScheduleRecord(ctx context.Context) error {
	return a.channelsService.EnsureGlobalScheduleRecord(ctx)
}

// syncGlobalScheduleRecord is the *App forwarder for
// channels.Service.SyncGlobalScheduleRecord. Delegates to the channels
// service so the SQL round-tripping lives in one place.
func (a *App) syncGlobalScheduleRecord(ctx context.Context) {
	a.channelsService.SyncGlobalScheduleRecord(ctx)
}
