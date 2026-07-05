package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

// standardCronParser mirrors core.standardCronParser. Supports minute/hour/
// dom/month/dow plus cron descriptors (e.g. "@daily"). Used by
// CalendarItemsForSchedule, CronCalendarItemsForSchedule, and ComputeNextRun.
var standardCronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

// ValidateCronExpr reports whether expr is a valid cron expression accepted
// by the standard parser. Exposed so the host's HTTP handler can validate
// user input without re-declaring the parser.
func ValidateCronExpr(expr string) error {
	_, err := standardCronParser.Parse(expr)
	return err
}

// ListChannelSchedules returns every channel_schedules row joined with
// upstream_sites for display. The global checkin schedule is stored with a
// NULL upstream_site_id, but is projected with the __global__ compatibility ID
// so callers and existing JSON contracts stay stable.
func (s *Service) ListChannelSchedules(ctx context.Context) ([]ChannelSchedule, error) {
	rows, err := s.infra.DB().QueryContext(ctx, `
		SELECT cs.id,
		       CASE WHEN cs.id=? THEN ? ELSE COALESCE(cs.upstream_site_id,'') END,
		       CASE WHEN cs.id=? THEN '全局签到' ELSE COALESCE(s.name,'') END,
		       cs.enabled, cs.checkin_time,
		       COALESCE(cs.cron_expr,''), COALESCE(cs.skip_dates_json,'[]'),
		       cs.random_delay_min, cs.random_delay_max, COALESCE(cs.last_run_at,''),
		       COALESCE(cs.next_run_at,''), cs.created_at, cs.updated_at
		FROM channel_schedules cs
		LEFT JOIN upstream_sites s ON s.id = cs.upstream_site_id
		ORDER BY CASE WHEN cs.id=? THEN 0 ELSE 1 END, cs.checkin_time
	`, GlobalScheduleSiteID, GlobalScheduleSiteID, GlobalScheduleSiteID, GlobalScheduleSiteID)
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
			if err := json.Unmarshal([]byte(skipDatesJSON), &item.SkipDates); err != nil {
				log.Printf("[schedule] skip_dates unmarshal failed for %s: %v", item.ID, err)
			}
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// NextSyncCalendarItem reads the next sync.local_newapi run from
// scheduler_runs and converts it to a calendar item if it falls within
// [now, windowEnd). Mirrors the body of core.nextSyncCalendarItem.
func (s *Service) NextSyncCalendarItem(ctx context.Context, now time.Time, windowEnd time.Time) (ScheduleCalendarItem, bool) {
	record, err := s.infra.LoadSchedulerRun(ctx, SchedulerJobSync)
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

// EnsureGlobalScheduleRecord creates or updates the __global__
// channel_schedule record. It no longer creates an upstream_sites row: the
// global checkin schedule is not a real site, so it is stored with a NULL
// upstream_site_id and only projected as __global__ for API compatibility.
func (s *Service) EnsureGlobalScheduleRecord(ctx context.Context) error {
	config := s.infra.LoadCheckinScheduleConfig(ctx)
	delayMin, delayMax := normalizedRandomDelay(config.RandomDelayMinutes)
	nextRun := ComputeNextRun(config.Time, "", nil, delayMin, delayMax)
	if _, err := s.infra.DB().ExecContext(ctx, `
		INSERT INTO channel_schedules (id, upstream_site_id, enabled, checkin_time, cron_expr, skip_dates_json, random_delay_min, random_delay_max, next_run_at, created_at, updated_at)
		VALUES (?, NULL, ?, ?, '', '[]', ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			upstream_site_id=NULL,
			enabled=excluded.enabled,
			checkin_time=excluded.checkin_time,
			cron_expr=excluded.cron_expr,
			skip_dates_json=excluded.skip_dates_json,
			random_delay_min=excluded.random_delay_min,
			random_delay_max=excluded.random_delay_max,
			next_run_at=excluded.next_run_at,
			updated_at=excluded.updated_at
	`, GlobalScheduleSiteID, config.Enabled, config.Time, delayMin, delayMax, nextRun, s.infra.Now(), s.infra.Now()); err != nil {
		return err
	}
	_, err := s.infra.DB().ExecContext(ctx, `DELETE FROM upstream_sites WHERE id=?`, GlobalScheduleSiteID)
	return err
}

// SyncGlobalScheduleRecord updates the __global__ channel_schedule record to
// reflect the current checkin.schedule config. Called at the end of each
// tickCheckinScheduler. Mirrors the body of core.syncGlobalScheduleRecord.
func (s *Service) SyncGlobalScheduleRecord(ctx context.Context) {
	config := s.infra.LoadCheckinScheduleConfig(ctx)
	delayMin, delayMax := normalizedRandomDelay(config.RandomDelayMinutes)
	nextRun := ComputeNextRun(config.Time, "", nil, delayMin, delayMax)
	if _, err := s.infra.DB().ExecContext(ctx, `
		UPDATE channel_schedules
		SET enabled=?, checkin_time=?, random_delay_min=?, random_delay_max=?, next_run_at=?, updated_at=?
		WHERE id=?
	`, config.Enabled, config.Time, delayMin, delayMax, nextRun, s.infra.Now(), GlobalScheduleSiteID); err != nil {
		log.Printf("[schedule] sync global schedule record failed: %v", err)
	}
}

// ParseCalendarDays mirrors core.parseCalendarDays. Extracts the "days" query
// parameter from r, clamped to [1, 31]. Returns fallback when the parameter is
// absent or invalid.
func ParseCalendarDays(r *http.Request, fallback int) int {
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

// CalendarItemsForSchedule mirrors core.calendarItemsForSchedule. Expands a
// per-site schedule into calendar items across [now, now+days). For cron-based
// schedules, defers to CronCalendarItemsForSchedule.
func CalendarItemsForSchedule(sched ChannelSchedule, now time.Time, windowEnd time.Time, days int) []ScheduleCalendarItem {
	if !sched.Enabled {
		return nil
	}
	if sched.CronExpr != "" {
		return CronCalendarItemsForSchedule(sched, now, windowEnd)
	}
	items := make([]ScheduleCalendarItem, 0, days)
	for day := 0; day < days; day++ {
		date := now.AddDate(0, 0, day)
		dateStr := date.Format("2006-01-02")
		if IsDateSkipped(dateStr, sched.SkipDates) {
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

// CronCalendarItemsForSchedule mirrors core.cronCalendarItemsForSchedule.
// Walks the cron schedule forward from now, emitting items until windowEnd.
func CronCalendarItemsForSchedule(sched ChannelSchedule, now time.Time, windowEnd time.Time) []ScheduleCalendarItem {
	parsed, err := standardCronParser.Parse(sched.CronExpr)
	if err != nil {
		return nil
	}
	var items []ScheduleCalendarItem
	next := parsed.Next(now.Add(-1 * time.Second))
	for len(items) < 366 && next.Before(windowEnd) {
		dateStr := next.Format("2006-01-02")
		if !IsDateSkipped(dateStr, sched.SkipDates) {
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

// IsDateSkipped mirrors core.isDateSkipped. Reports whether date appears in
// skipDates.
func IsDateSkipped(date string, skipDates []string) bool {
	for _, d := range skipDates {
		if d == date {
			return true
		}
	}
	return false
}

// ComputeNextRun mirrors core.computeNextRun. Returns the next run time as an
// RFC3339 string. Uses Asia/Shanghai (UTC+8) timezone for consistent
// scheduling regardless of the server's local timezone.
//
// Priority:
//  1. cronExpr (when set) — compute from cron expression
//  2. checkinTime (fallback) — "HH:MM" daily at that time
//
// Both skip dates in the skip list.
func ComputeNextRun(checkinTime string, cronExpr string, skipDates []string, delayMin, delayMax int) string {
	cst := time.FixedZone("CST", 8*3600)
	now := time.Now().In(cst)

	var next time.Time
	var cronSchedule cron.Schedule

	if cronExpr != "" {
		sched, err := standardCronParser.Parse(cronExpr)
		if err == nil {
			cronSchedule = sched
			next = sched.Next(now)
		}
		// On parse error, fall through to checkinTime.
	}

	if next.IsZero() {
		hour, minute := 8, 0
		fmt.Sscanf(checkinTime, "%d:%d", &hour, &minute)
		next = time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, cst)
		if !next.After(now) {
			next = next.AddDate(0, 0, 1)
		}
	}

	// Skip dates in the skip list — advance by 1 day and retry (cron) or
	// advance by 1 day (daily).
	dateStr := next.Format("2006-01-02")
	maxIter := 366 // safety limit
	for i := 0; i < maxIter && IsDateSkipped(dateStr, skipDates); i++ {
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

// SortCalendarItemsByDateTime sorts items in place by Date+Time ascending.
// Mirrors the sort.Slice call inlined in core.handleScheduleCalendar so the
// host handler can stay thin.
func SortCalendarItemsByDateTime(items []ScheduleCalendarItem) {
	sort.Slice(items, func(i, j int) bool {
		return items[i].Date+items[i].Time < items[j].Date+items[j].Time
	})
}
