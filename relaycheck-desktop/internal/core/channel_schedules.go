package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
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
	globalScheduleSiteID = "__global__"
)

var standardCronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

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
		if _, err := standardCronParser.Parse(body.CronExpr); err != nil {
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

	nextRun := computeNextRun(body.CheckinTime, body.CronExpr, body.SkipDates, rdMin, rdMax)

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

func (a *App) listChannelSchedules(ctx context.Context) ([]ChannelSchedule, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT cs.id, cs.upstream_site_id, COALESCE(s.name,''), cs.enabled, cs.checkin_time,
		       COALESCE(cs.cron_expr,''), COALESCE(cs.skip_dates_json,'[]'),
		       cs.random_delay_min, cs.random_delay_max, COALESCE(cs.last_run_at,''),
		       COALESCE(cs.next_run_at,''), cs.created_at, cs.updated_at
		FROM channel_schedules cs
		LEFT JOIN upstream_sites s ON s.id = cs.upstream_site_id
		ORDER BY cs.checkin_time
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []ChannelSchedule
	for rows.Next() {
		var item ChannelSchedule
		var enabled int
		var skipDatesJSON string
		if err := rows.Scan(&item.ID, &item.UpstreamSiteID, &item.SiteName, &enabled, &item.CheckinTime,
			&item.CronExpr, &skipDatesJSON,
			&item.RandomDelayMin, &item.RandomDelayMax, &item.LastRunAt, &item.NextRunAt,
			&item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.Enabled = enabled != 0
		if skipDatesJSON != "" && skipDatesJSON != "[]" {
			json.Unmarshal([]byte(skipDatesJSON), &item.SkipDates)
		}
		items = append(items, item)
	}
	return items, nil
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
	days := parseCalendarDays(r, 7)
	windowEnd := now.AddDate(0, 0, days)
	items := make([]ScheduleCalendarItem, 0, len(schedules)*days)

	for _, sched := range schedules {
		items = append(items, calendarItemsForSchedule(sched, now, windowEnd, days)...)
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

func parseCalendarDays(r *http.Request, fallback int) int {
	if fallback <= 0 {
		fallback = 7
	}
	if r == nil {
		return fallback
	}
	raw := r.URL.Query().Get("days")
	if raw == "" {
		return fallback
	}
	days, err := strconv.Atoi(raw)
	if err != nil || days <= 0 {
		return fallback
	}
	if days > 31 {
		return 31
	}
	return days
}

func calendarItemsForSchedule(sched ChannelSchedule, now time.Time, windowEnd time.Time, days int) []ScheduleCalendarItem {
	if !sched.Enabled {
		return nil
	}
	if sched.CronExpr != "" {
		return cronCalendarItemsForSchedule(sched, now, windowEnd)
	}
	items := make([]ScheduleCalendarItem, 0, days)
	for day := 0; day < days; day++ {
		date := now.AddDate(0, 0, day)
		dateStr := date.Format("2006-01-02")
		if isDateSkipped(dateStr, sched.SkipDates) {
			continue
		}
		items = append(items, ScheduleCalendarItem{
			Date:     dateStr,
			Time:     sched.CheckinTime,
			SiteName: sched.SiteName,
			SiteID:   sched.UpstreamSiteID,
			JobType:  "checkin",
			Enabled:  true,
		})
	}
	return items
}

func cronCalendarItemsForSchedule(sched ChannelSchedule, now time.Time, windowEnd time.Time) []ScheduleCalendarItem {
	parsed, err := standardCronParser.Parse(sched.CronExpr)
	if err != nil {
		return nil
	}
	var items []ScheduleCalendarItem
	next := parsed.Next(now.Add(-1 * time.Second))
	for len(items) < 366 && next.Before(windowEnd) {
		dateStr := next.Format("2006-01-02")
		if !isDateSkipped(dateStr, sched.SkipDates) {
			items = append(items, ScheduleCalendarItem{
				Date:     dateStr,
				Time:     next.Format("15:04"),
				SiteName: sched.SiteName,
				SiteID:   sched.UpstreamSiteID,
				JobType:  "checkin",
				Enabled:  true,
			})
		}
		next = parsed.Next(next)
	}
	return items
}

func (a *App) nextSyncCalendarItem(ctx context.Context, now time.Time, windowEnd time.Time) (ScheduleCalendarItem, bool) {
	record, err := a.loadSchedulerRun(ctx, schedulerJobSync)
	if err != nil || strings.TrimSpace(record.NextRunAt) == "" {
		return ScheduleCalendarItem{}, false
	}
	nextRun, err := time.Parse(time.RFC3339Nano, record.NextRunAt)
	if err != nil {
		return ScheduleCalendarItem{}, false
	}
	nextRun = nextRun.In(now.Location())
	if nextRun.Before(now) || !nextRun.Before(windowEnd) {
		return ScheduleCalendarItem{}, false
	}
	return ScheduleCalendarItem{
		Date:     nextRun.Format("2006-01-02"),
		Time:     nextRun.Format("15:04"),
		SiteName: "本地 NewAPI 同步",
		SiteID:   "",
		JobType:  "sync",
		Enabled:  true,
	}, true
}

func (a *App) handleNextRuns(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	ctx := r.Context()
	status := a.buildSchedulerStatus(ctx)

	var items []NextRunItem
	nowTime := time.Now()
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

// isDateSkipped checks if a date string is in the skip list.
func isDateSkipped(date string, skipDates []string) bool {
	for _, d := range skipDates {
		if d == date {
			return true
		}
	}
	return false
}

// computeNextRun returns the next run time as RFC3339 string.
// Uses Asia/Shanghai (UTC+8) timezone for consistent scheduling regardless
// of the server's local timezone.
//
// Priority:
//  1. cron_expr (when set) — compute from cron expression
//  2. checkin_time (fallback) — "HH:MM" daily at that time
//
// Both skip dates in the skip list.
func computeNextRun(checkinTime string, cronExpr string, skipDates []string, delayMin, delayMax int) string {
	cst := time.FixedZone("CST", 8*3600)
	now := time.Now().In(cst)

	var next time.Time
	var cronSchedule cron.Schedule

	if cronExpr != "" {
		sched, err := standardCronParser.Parse(cronExpr)
		if err == nil {
			cronSchedule = sched
			next = sched.Next(now)
		} else {
			// Fallback to checkinTime on parse error
		}
	}

	if next.IsZero() {
		hour, minute := 8, 0
		fmt.Sscanf(checkinTime, "%d:%d", &hour, &minute)
		next = time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, cst)
		if !next.After(now) {
			next = next.AddDate(0, 0, 1)
		}
	}

	// Skip dates in the skip list — advance by 1 day and retry (cron) or advance by 1 day (daily)
	dateStr := next.Format("2006-01-02")
	maxIter := 366 // safety limit
	for i := 0; i < maxIter && isDateSkipped(dateStr, skipDates); i++ {
		if cronSchedule != nil {
			next = cronSchedule.Next(next.Add(time.Minute)) // advance past current match
		} else {
			next = next.AddDate(0, 0, 1)
		}
		dateStr = next.Format("2006-01-02")
	}

	if delayMax > delayMin {
		// Use midpoint for deterministic preview
		delay := (delayMin + delayMax) / 2
		next = next.Add(time.Duration(delay) * time.Minute)
	}
	return next.Format(time.RFC3339)
}

// ensureGlobalScheduleRecord creates or updates the __global__ channel_schedule record
// so that listChannelSchedules returns it as a regular schedule entry. This allows
// the global checkin schedule to appear in calendar views and next-run lists without
// a separate code path. The record is a projection of system_settings checkin.schedule.
func (a *App) ensureGlobalScheduleRecord(ctx context.Context) error {
	// Ensure __global__ upstream_site exists (FK constraint)
	_, err := a.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO upstream_sites (id, name, base_url, kind, created_at, updated_at)
		VALUES (?, ?, '', 'unknown', ?, ?)
	`, globalScheduleSiteID, "全局签到", now(), now())
	if err != nil {
		return err
	}
	config := a.loadCheckinScheduleConfig(ctx)
	delayMin, delayMax := normalizedRandomDelay(config.RandomDelayMinutes)
	nextRun := computeNextRun(config.Time, "", nil, delayMin, delayMax)
	_, err = a.db.ExecContext(ctx, `
		INSERT INTO channel_schedules (id, upstream_site_id, enabled, checkin_time, cron_expr, skip_dates_json, random_delay_min, random_delay_max, next_run_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, '', '[]', ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			enabled=excluded.enabled,
			checkin_time=excluded.checkin_time,
			cron_expr=excluded.cron_expr,
			skip_dates_json=excluded.skip_dates_json,
			random_delay_min=excluded.random_delay_min,
			random_delay_max=excluded.random_delay_max,
			next_run_at=excluded.next_run_at,
			updated_at=excluded.updated_at
	`, globalScheduleSiteID, globalScheduleSiteID, config.Enabled, config.Time, delayMin, delayMax, nextRun, now(), now())
	return err
}

// syncGlobalScheduleRecord updates the __global__ channel_schedule record to reflect
// the current checkin.schedule config. Called at the end of each tickCheckinScheduler.
func (a *App) syncGlobalScheduleRecord(ctx context.Context) {
	config := a.loadCheckinScheduleConfig(ctx)
	delayMin, delayMax := normalizedRandomDelay(config.RandomDelayMinutes)
	nextRun := computeNextRun(config.Time, "", nil, delayMin, delayMax)
	a.db.ExecContext(ctx, `
		UPDATE channel_schedules
		SET enabled=?, checkin_time=?, random_delay_min=?, random_delay_max=?, next_run_at=?, updated_at=?
		WHERE id=?
	`, config.Enabled, config.Time, delayMin, delayMax, nextRun, now(), globalScheduleSiteID)
}
