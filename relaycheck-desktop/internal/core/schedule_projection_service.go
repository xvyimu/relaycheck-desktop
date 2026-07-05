package core

import (
	"context"
	"sort"
	"time"

	"relaycheck-desktop/internal/channels"
)

type ScheduleProjectionService struct {
	schedulerStatus      func(context.Context) SchedulerStatus
	listSchedules        func(context.Context) ([]ChannelSchedule, error)
	nextSyncCalendarItem func(context.Context, time.Time, time.Time) (ScheduleCalendarItem, bool)
}

func NewScheduleProjectionService(app *App) *ScheduleProjectionService {
	return &ScheduleProjectionService{
		schedulerStatus:      app.buildSchedulerStatus,
		listSchedules:        app.listChannelSchedules,
		nextSyncCalendarItem: app.nextSyncCalendarItem,
	}
}

type ScheduleCalendarProjection struct {
	GeneratedAt string                 `json:"generatedAt"`
	Items       []ScheduleCalendarItem `json:"items"`
}

func (s *ScheduleProjectionService) BuildCalendar(ctx context.Context, currentTime time.Time, days int) (ScheduleCalendarProjection, error) {
	schedules, err := s.listSchedules(ctx)
	if err != nil {
		return ScheduleCalendarProjection{}, err
	}

	windowEnd := currentTime.AddDate(0, 0, days)
	items := make([]ScheduleCalendarItem, 0, len(schedules)*days)
	for _, sched := range schedules {
		mirrorItems := channels.CalendarItemsForSchedule(scheduleToMirror(sched), currentTime, windowEnd, days)
		items = append(items, calendarItemsToCore(mirrorItems)...)
	}
	if item, ok := s.nextSyncCalendarItem(ctx, currentTime, windowEnd); ok {
		items = append(items, item)
	}

	sortCalendarItemsByDateTime(items)
	return ScheduleCalendarProjection{
		GeneratedAt: currentTime.Format(time.RFC3339),
		Items:       items,
	}, nil
}

func (s *ScheduleProjectionService) BuildNextRuns(ctx context.Context, currentTime time.Time) NextRunList {
	status := s.schedulerStatus(ctx)
	items := make([]NextRunItem, 0, len(status.Jobs))
	for _, job := range status.Jobs {
		var nextRunInSec int64 = -1
		if job.NextRunAt != "" {
			if t, err := time.Parse(time.RFC3339, job.NextRunAt); err == nil {
				nextRunInSec = int64(t.Sub(currentTime).Seconds())
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

	// Preserve the previous endpoint behavior: scheduler jobs are still
	// returned even if per-site schedule loading fails.
	schedules, _ := s.listSchedules(ctx)
	for _, sched := range schedules {
		if !sched.Enabled || sched.NextRunAt == "" {
			continue
		}
		var nextRunInSec int64 = -1
		if t, err := time.Parse(time.RFC3339, sched.NextRunAt); err == nil {
			nextRunInSec = int64(t.Sub(currentTime).Seconds())
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
	return NextRunList{
		GeneratedAt: currentTime.Format(time.RFC3339),
		Items:       items,
	}
}

func sortCalendarItemsByDateTime(items []ScheduleCalendarItem) {
	sort.Slice(items, func(i, j int) bool {
		return items[i].Date+items[i].Time < items[j].Date+items[j].Time
	})
}
