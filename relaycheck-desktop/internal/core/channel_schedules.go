package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"
)

// ChannelSchedule represents per-site checkin scheduling configuration.
type ChannelSchedule struct {
	ID             string `json:"id"`
	UpstreamSiteID string `json:"upstreamSiteId"`
	SiteName       string `json:"siteName,omitempty"`
	Enabled        bool   `json:"enabled"`
	CheckinTime    string `json:"checkinTime"` // "HH:MM"
	RandomDelayMin int   `json:"randomDelayMin"`
	RandomDelayMax int   `json:"randomDelayMax"`
	LastRunAt      string `json:"lastRunAt,omitempty"`
	NextRunAt      string `json:"nextRunAt,omitempty"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

// ScheduleCalendarItem represents one upcoming scheduled run in the calendar view.
type ScheduleCalendarItem struct {
	Date         string `json:"date"`         // "YYYY-MM-DD"
	Time         string `json:"time"`         // "HH:MM"
	SiteName     string `json:"siteName"`
	SiteID       string `json:"siteId"`
	JobType      string `json:"jobType"`      // "checkin" | "sync"
	Enabled      bool   `json:"enabled"`
}

// NextRunList is the response for /api/scheduler/next-runs.
type NextRunList struct {
	GeneratedAt string                 `json:"generatedAt"`
	Items       []NextRunItem          `json:"items"`
}

type NextRunItem struct {
	JobKey       string `json:"jobKey"`
	Label        string `json:"label"`
	NextRunAt    string `json:"nextRunAt"`
	NextRunInSec int64  `json:"nextRunInSeconds"`
	Status       string `json:"status"`
	SiteName     string `json:"siteName,omitempty"`
}

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
		UpstreamSiteID string `json:"upstreamSiteId"`
		Enabled        *bool  `json:"enabled"`
		CheckinTime    string `json:"checkinTime"`
		RandomDelayMin *int  `json:"randomDelayMin"`
		RandomDelayMax *int  `json:"randomDelayMax"`
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
	// Validate checkinTime format "HH:MM"
	var hour, minute int
	if _, err := fmt.Sscanf(body.CheckinTime, "%d:%d", &hour, &minute); err != nil {
		writeError(w, http.StatusBadRequest, "签到时间格式无效，应为 HH:MM")
		return
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		writeError(w, http.StatusBadRequest, "签到时间范围无效，小时 0-23，分钟 0-59")
		return
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

	nextRun := computeNextRun(body.CheckinTime, rdMin, rdMax)

	_, err := a.db.ExecContext(ctx, `
		INSERT INTO channel_schedules (id, upstream_site_id, enabled, checkin_time, random_delay_min, random_delay_max, next_run_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			enabled=excluded.enabled,
			checkin_time=excluded.checkin_time,
			random_delay_min=excluded.random_delay_min,
			random_delay_max=excluded.random_delay_max,
			next_run_at=excluded.next_run_at,
			updated_at=excluded.updated_at
	`, body.UpstreamSiteID, body.UpstreamSiteID, enabled, body.CheckinTime, rdMin, rdMax, nextRun, now(), now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "nextRunAt": nextRun})
}

func (a *App) listChannelSchedules(ctx context.Context) ([]ChannelSchedule, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT cs.id, cs.upstream_site_id, COALESCE(s.name,''), cs.enabled, cs.checkin_time,
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
		if err := rows.Scan(&item.ID, &item.UpstreamSiteID, &item.SiteName, &enabled, &item.CheckinTime,
			&item.RandomDelayMin, &item.RandomDelayMax, &item.LastRunAt, &item.NextRunAt,
			&item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.Enabled = enabled != 0
		items = append(items, item)
	}
	return items, nil
}

func (a *App) handleScheduleCalendar(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	ctx := r.Context()

	// Get channel schedules for next 7 days
	schedules, err := a.listChannelSchedules(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var items []ScheduleCalendarItem
	cst := time.FixedZone("CST", 8*3600)
	now := time.Now().In(cst)

	// Load configs once outside the loop to avoid repeated DB queries
	checkinCfg := a.loadCheckinScheduleConfig(ctx)
	syncCfg := a.loadSyncScheduleConfig(ctx)

	for day := 0; day < 7; day++ {
		date := now.AddDate(0, 0, day)
		dateStr := date.Format("2006-01-02")

		for _, sched := range schedules {
			if !sched.Enabled {
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

		// Add global checkin schedule
		if checkinCfg.Enabled {
			items = append(items, ScheduleCalendarItem{
				Date:     dateStr,
				Time:     checkinCfg.Time,
				SiteName: "全部站点",
				SiteID:   "",
				JobType:  "checkin",
				Enabled:  true,
			})
		}

		// Add sync schedule on each day at the configured sync time
		if syncCfg.Enabled {
			items = append(items, ScheduleCalendarItem{
				Date:     dateStr,
				Time:     date.Format("15:04"),
				SiteName: "本地 NewAPI 同步",
				SiteID:   "",
				JobType:  "sync",
				Enabled:  true,
			})
		}
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

func (a *App) handleNextRuns(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	ctx := r.Context()
	status := a.buildSchedulerStatus(ctx)

	var items []NextRunItem
	now := time.Now()
	for _, job := range status.Jobs {
		var nextRunInSec int64 = -1
		if job.NextRunAt != "" {
			if t, err := time.Parse(time.RFC3339, job.NextRunAt); err == nil {
				nextRunInSec = int64(t.Sub(now).Seconds())
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
			nextRunInSec = int64(t.Sub(now).Seconds())
			if nextRunInSec < 0 {
				nextRunInSec = 0
			}
		}
		items = append(items, NextRunItem{
			JobKey:       "channel." + sched.UpstreamSiteID,
			Label:        sched.SiteName + " 签到",
			NextRunAt:    sched.NextRunAt,
			NextRunInSec: nextRunInSec,
			Status:       "scheduled",
			SiteName:     sched.SiteName,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].NextRunAt < items[j].NextRunAt
	})

	writeJSON(w, http.StatusOK, NextRunList{
		GeneratedAt: now.Format(time.RFC3339),
		Items:       items,
	})
}

// computeNextRun returns the next run time as RFC3339 string.
// Uses Asia/Shanghai (UTC+8) timezone for consistent scheduling regardless
// of the server's local timezone.
func computeNextRun(checkinTime string, delayMin, delayMax int) string {
	hour, minute := 8, 0
	fmt.Sscanf(checkinTime, "%d:%d", &hour, &minute)

	// Use fixed CST timezone to avoid ambiguity when server runs in UTC
	cst := time.FixedZone("CST", 8*3600)
	now := time.Now().In(cst)
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, cst)
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}

	if delayMax > delayMin {
		// Use midpoint for deterministic preview
		delay := (delayMin + delayMax) / 2
		next = next.Add(time.Duration(delay) * time.Minute)
	}
	return next.Format(time.RFC3339)
}
