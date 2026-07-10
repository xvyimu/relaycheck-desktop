package core

import (
	"context"
	"time"
)

type schedulerTickUnit struct {
	key             string
	label           string
	visibleInStatus bool
	tick            func(context.Context, time.Time)
}

func (a *App) schedulerTickRegistry() []schedulerTickUnit {
	return []schedulerTickUnit{
		{
			key:             schedulerJobCheckin,
			label:           "自动签到",
			visibleInStatus: true,
			tick:            a.tickCheckinScheduler,
		},
		{
			key:             "channel.site_schedules",
			label:           "渠道独立排程",
			visibleInStatus: false,
			tick:            a.tickChannelScheduler,
		},
		{
			key:             schedulerJobSync,
			label:           "NewAPI 定时同步",
			visibleInStatus: true,
			tick:            a.tickSyncScheduler,
		},
		{
			key:             schedulerJobChannelHealth,
			label:           "渠道健康探测",
			visibleInStatus: true,
			tick:            a.tickChannelHealthScheduler,
		},
	}
}

func (a *App) schedulerStatusRegistry() []schedulerTickUnit {
	ticks := a.schedulerTickRegistry()
	jobs := make([]schedulerTickUnit, 0, len(ticks))
	for _, tick := range ticks {
		if tick.visibleInStatus {
			jobs = append(jobs, tick)
		}
	}
	return jobs
}
