package core

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

	"relaycheck-desktop/internal/channels"
)

const (
	schedulerJobCheckin       = "checkin.daily"
	schedulerJobSync          = "sync.local_newapi"
	schedulerJobChannelHealth = "channel.health_probe"

	schedulerTickInterval   = 30 * time.Second
	checkinJobTimeout       = 20 * time.Minute
	syncJobTimeout          = 10 * time.Minute
	channelHealthJobTimeout = 12 * time.Minute
	minScheduleInterval     = 5 * time.Minute
	defaultScheduleInterval = 30 * time.Minute
)

type schedulerRunRecord struct {
	JobKey         string
	Status         string
	PlannedRunKey  string
	NextRunAt      string
	LastRunKey     string
	LastStartedAt  string
	LastFinishedAt string
	LastSuccessAt  string
	LastError      string
	Summary        string
	UpdatedAt      string
}

type checkinScheduleConfig struct {
	Enabled                bool   `json:"enabled"`
	Time                   string `json:"time"`
	RandomDelayMinutes     []int  `json:"randomDelayMinutes"`
	SiteConcurrency        int    `json:"siteConcurrency"`
	GlobalConcurrency      int    `json:"globalConcurrency"`
	SiteMinIntervalSeconds int    `json:"siteMinIntervalSeconds"`
}

type syncScheduleConfig struct {
	Enabled         bool   `json:"enabled"`
	IntervalMinutes int    `json:"intervalMinutes"`
	Mode            string `json:"mode"`
	RunOnStartup    bool   `json:"runOnStartup"`
}

type channelHealthScheduleConfig struct {
	Enabled         bool `json:"enabled"`
	IntervalMinutes int  `json:"intervalMinutes"`
	RunOnStartup    bool `json:"runOnStartup"`
	Limit           int  `json:"limit"`
	OnlyRisky       bool `json:"onlyRisky"`
}

type checkinPlan struct {
	Enabled bool
	RunKey  string
	Start   time.Time
	End     time.Time
	RunAt   time.Time
	Message string
}

type scheduledSyncResult struct {
	TotalInstances     int
	ProcessedInstances int
	SkippedInstances   int
	FailedInstances    int
	ImportedChannels   int
	SitesCreated       int
	SitesMerged        int
	MissingChannels    int
	Messages           []string
}

func (r scheduledSyncResult) Summary() string {
	parts := []string{
		fmt.Sprintf("实例 %d/%d", r.ProcessedInstances, r.TotalInstances),
		fmt.Sprintf("跳过 %d", r.SkippedInstances),
		fmt.Sprintf("失败 %d", r.FailedInstances),
		fmt.Sprintf("渠道 %d", r.ImportedChannels),
		fmt.Sprintf("新站点 %d", r.SitesCreated),
		fmt.Sprintf("合并 %d", r.SitesMerged),
		fmt.Sprintf("源端移除 %d", r.MissingChannels),
	}
	if len(r.Messages) > 0 {
		parts = append(parts, strings.Join(r.Messages, "；"))
	}
	return strings.Join(parts, "，")
}

// StartSchedulers launches the checkin, sync, and channel-health scheduler goroutines.
// It derives its lifecycle from a.rootCtx so that app.Close() cancels all scheduler work.
func (a *App) StartSchedulers() {
	a.mu.Lock()
	if a.schedulerCancel != nil {
		a.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(a.rootCtx)
	a.schedulerCancel = cancel
	a.schedulerStartedAt = time.Now()
	a.mu.Unlock()

	a.schedulerWG.Add(1)
	go func() {
		defer a.schedulerWG.Done()
		a.schedulerLoop(ctx)
	}()
}

func (a *App) schedulerLoop(ctx context.Context) {
	_ = a.resetInterruptedSchedulerRuns(ctx)
	a.tickSchedulers(ctx)

	ticker := time.NewTicker(schedulerTickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.tickSchedulers(ctx)
		}
	}
}

func (a *App) tickSchedulers(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	// Use CST for scheduler ticks so that HH:MM config values like "08:00"
	// are interpreted as 08:00 CST regardless of the server's local timezone.
	// This also keeps runKey dates and "today" boundaries aligned with the
	// rest of the system (action center, diagnostics, checkin summaries).
	nowTime := nowCST()
	a.tickCheckinScheduler(ctx, nowTime)
	a.tickChannelScheduler(ctx, nowTime)
	a.tickSyncScheduler(ctx, nowTime)
	a.tickChannelHealthScheduler(ctx, nowTime)
}

// tickChannelScheduler checks per-site channel schedules and triggers checkins for due sites.
// Each site with an enabled schedule gets checked independently of the global schedule.
func (a *App) tickChannelScheduler(ctx context.Context, currentTime time.Time) {
	schedules, err := a.listChannelSchedules(ctx)
	if err != nil || len(schedules) == 0 {
		return
	}

	nowStr := currentTime.Format(time.RFC3339)
	for _, sched := range schedules {
		if ctx.Err() != nil {
			return
		}
		if sched.ID == globalScheduleSiteID {
			continue
		}
		if !sched.Enabled || sched.NextRunAt == "" {
			continue
		}
		// Skip if next run is still in the future
		if sched.NextRunAt > nowStr {
			continue
		}
		// Execute per-site checkin. Do not advance the schedule if the run was
		// blocked or failed before completion; otherwise the due window is lost.
		if _, err := a.runDueCheckinsForSite(ctx, sched.UpstreamSiteID); err != nil {
			continue
		}

		// Recalculate next run after execution
		newNextRun := channels.ComputeNextRun(sched.CheckinTime, sched.CronExpr, sched.SkipDates, sched.RandomDelayMin, sched.RandomDelayMax)
		if _, execErr := a.db.ExecContext(ctx, `
			UPDATE channel_schedules
			SET last_run_at=?, next_run_at=?, updated_at=?
			WHERE id=?
		`, nowStr, newNextRun, nowStr, sched.ID); execErr != nil {
			log.Printf("[scheduler] channel schedule next_run_at update failed for %s: %v", sched.ID, execErr)
		}
	}
}

func (a *App) tickCheckinScheduler(ctx context.Context, currentTime time.Time) {
	config := a.loadCheckinScheduleConfig(ctx)
	plan := makeCheckinPlan(config, currentTime)
	record, _ := a.loadSchedulerRun(ctx, schedulerJobCheckin)
	if !plan.Enabled {
		if err := a.upsertSchedulerPlan(ctx, schedulerJobCheckin, "", "", "自动签到未启用。"); err != nil {
			log.Printf("[scheduler] upsert %s plan failed: %v", schedulerJobCheckin, err)
		}
		return
	}

	nextRunAt := record.NextRunAt
	if record.PlannedRunKey != plan.RunKey || !timeWithinPlan(nextRunAt, plan.Start, plan.End) {
		runAt := randomTimeInWindow(plan.Start, plan.End)
		plan.RunAt = runAt
		// Preserve the CST offset in the stored string instead of normalising
		// to UTC; the parsed value is later interpreted in CST via In(cstZone).
		nextRunAt = runAt.Format(time.RFC3339Nano)
		if err := a.upsertSchedulerPlan(ctx, schedulerJobCheckin, plan.RunKey, nextRunAt, "等待自动签到窗口。"); err != nil {
			log.Printf("[scheduler] upsert %s plan failed: %v", schedulerJobCheckin, err)
		}
	} else if parsed, err := time.Parse(time.RFC3339Nano, nextRunAt); err == nil {
		plan.RunAt = parsed.In(cstZone())
	}

	if record.LastRunKey == plan.RunKey || currentTime.Before(plan.RunAt) || currentTime.After(plan.End) {
		return
	}
	if !a.beginSchedulerJob(ctx, schedulerJobCheckin, plan.RunKey) {
		return
	}

	jobCtx, cancel := context.WithTimeout(ctx, checkinJobTimeout)
	defer cancel()
	results, err := a.runDueCheckins(jobCtx, "scheduled")
	status := "success"
	summary := fmt.Sprintf("自动签到处理 %d 个账号。", len(results))
	errMessage := ""
	if err != nil {
		status = "failed"
		errMessage = err.Error()
		summary = "自动签到失败：" + errMessage
		a.notify("scheduled_checkin_failed", "warning", "自动签到失败", errMessage, "scheduler", schedulerJobCheckin)
	}
	if err := a.finishSchedulerJob(ctx, schedulerJobCheckin, plan.RunKey, status, summary, errMessage); err != nil {
		log.Printf("[scheduler] finish %s failed: %v", schedulerJobCheckin, err)
	}
	a.syncGlobalScheduleRecord(ctx)
}

func (a *App) tickSyncScheduler(ctx context.Context, currentTime time.Time) {
	config := a.loadSyncScheduleConfig(ctx)
	record, _ := a.loadSchedulerRun(ctx, schedulerJobSync)
	if !config.Enabled || config.Mode == "manual-only" {
		if err := a.upsertSchedulerPlan(ctx, schedulerJobSync, "", "", "定时同步未启用。"); err != nil {
			log.Printf("[scheduler] upsert %s plan failed: %v", schedulerJobSync, err)
		}
		return
	}

	interval := time.Duration(config.IntervalMinutes) * time.Minute
	if interval < minScheduleInterval {
		interval = defaultScheduleInterval
	}

	due := false
	nextRun := currentTime.Add(interval)
	runKey := currentTime.Format("20060102-1504")
	if record.LastFinishedAt == "" {
		if config.RunOnStartup {
			due = true
			nextRun = currentTime
		} else {
			startedAt := a.schedulerStartTime()
			nextRun = startedAt.Add(interval)
			if !currentTime.Before(nextRun) {
				due = true
			}
		}
	} else if lastFinished, err := time.Parse(time.RFC3339Nano, record.LastFinishedAt); err == nil {
		nextRun = lastFinished.Add(interval)
		if !currentTime.Before(nextRun) {
			due = true
		}
	}
	if err := a.upsertSchedulerPlan(ctx, schedulerJobSync, runKey, nextRun.Format(time.RFC3339Nano), "等待下一次 NewAPI 同步。"); err != nil {
		log.Printf("[scheduler] upsert %s plan failed: %v", schedulerJobSync, err)
	}
	if !due {
		return
	}
	if !a.beginSchedulerJob(ctx, schedulerJobSync, runKey) {
		return
	}

	jobCtx, cancel := context.WithTimeout(ctx, syncJobTimeout)
	defer cancel()
	result := a.runScheduledLocalNewAPISync(jobCtx)
	status := "success"
	errMessage := ""
	if result.FailedInstances > 0 {
		status = "warning"
		errMessage = fmt.Sprintf("%d 个实例同步失败。", result.FailedInstances)
		a.notify("scheduled_sync_failed", "warning", "定时同步有失败项", result.Summary(), "scheduler", schedulerJobSync)
	}
	if result.ProcessedInstances == 0 && result.SkippedInstances > 0 {
		status = "skipped"
	}
	if err := a.finishSchedulerJob(ctx, schedulerJobSync, runKey, status, result.Summary(), errMessage); err != nil {
		log.Printf("[scheduler] finish %s failed: %v", schedulerJobSync, err)
	}
}

func (a *App) tickChannelHealthScheduler(ctx context.Context, currentTime time.Time) {
	config := a.loadChannelHealthScheduleConfig(ctx)
	record, _ := a.loadSchedulerRun(ctx, schedulerJobChannelHealth)
	if !config.Enabled {
		if err := a.upsertSchedulerPlan(ctx, schedulerJobChannelHealth, "", "", "渠道健康探测未启用。"); err != nil {
			log.Printf("[scheduler] upsert %s plan failed: %v", schedulerJobChannelHealth, err)
		}
		return
	}

	interval := time.Duration(config.IntervalMinutes) * time.Minute
	if interval < minScheduleInterval {
		interval = defaultScheduleInterval
	}

	due := false
	nextRun := currentTime.Add(interval)
	runKey := currentTime.Format("20060102-1504")
	if record.LastFinishedAt == "" {
		if config.RunOnStartup {
			due = true
			nextRun = currentTime
		} else {
			startedAt := a.schedulerStartTime()
			nextRun = startedAt.Add(interval)
			if !currentTime.Before(nextRun) {
				due = true
			}
		}
	} else if lastFinished, err := time.Parse(time.RFC3339Nano, record.LastFinishedAt); err == nil {
		nextRun = lastFinished.Add(interval)
		if !currentTime.Before(nextRun) {
			due = true
		}
	}
	if err := a.upsertSchedulerPlan(ctx, schedulerJobChannelHealth, runKey, nextRun.Format(time.RFC3339Nano), "等待下一次渠道健康探测。"); err != nil {
		log.Printf("[scheduler] upsert %s plan failed: %v", schedulerJobChannelHealth, err)
	}
	if !due {
		return
	}
	if !a.beginSchedulerJob(ctx, schedulerJobChannelHealth, runKey) {
		return
	}

	jobCtx, cancel := context.WithTimeout(ctx, channelHealthJobTimeout)
	defer cancel()
	result, err := a.runScheduledChannelHealthProbe(jobCtx, config)
	status := "success"
	errMessage := ""
	summary := result.Summary()
	if err != nil {
		status = "failed"
		errMessage = err.Error()
		summary = "渠道健康探测失败：" + errMessage
		a.notify("scheduled_channel_health_probe_failed", "warning", "渠道健康探测失败", errMessage, "scheduler", schedulerJobChannelHealth)
	} else if result.Failed > 0 || result.Warning > 0 {
		status = "warning"
		errMessage = fmt.Sprintf("%d 个失败，%d 个预警", result.Failed, result.Warning)
		a.notify("scheduled_channel_health_probe_warning", "warning", "渠道健康探测发现风险", summary, "scheduler", schedulerJobChannelHealth)
	}
	if err := a.finishSchedulerJob(ctx, schedulerJobChannelHealth, runKey, status, summary, errMessage); err != nil {
		log.Printf("[scheduler] finish %s failed: %v", schedulerJobChannelHealth, err)
	}
}

func (a *App) runScheduledChannelHealthProbe(ctx context.Context, config channelHealthScheduleConfig) (channelHealthProbeResult, error) {
	jobs, err := a.loadChannelHealthProbeJobs(ctx, config.Limit, config.OnlyRisky)
	if err != nil {
		return channelHealthProbeResult{}, err
	}
	return a.runChannelHealthProbe(ctx, jobs, nil), nil
}

func (a *App) runScheduledLocalNewAPISync(ctx context.Context) scheduledSyncResult {
	instances, err := a.loadLocalNewAPIInstancesForScheduler(ctx)
	result := scheduledSyncResult{TotalInstances: len(instances)}
	if err != nil {
		result.FailedInstances = 1
		result.Messages = append(result.Messages, err.Error())
		return result
	}
	input := localNewAPISyncRunInput{
		UserID:            "1",
		ImportKeys:        false,
		SkipCreateSites:   false,
		DetectAfterImport: false,
		PageSize:          100,
	}
	for _, instance := range instances {
		if ctx.Err() != nil {
			result.FailedInstances++
			result.Messages = append(result.Messages, ctx.Err().Error())
			break
		}
		if strings.TrimSpace(instance.DatabasePath) == "" && (!isHTTPURL(instance.BaseURL) || !instance.HasSyncToken) {
			result.SkippedInstances++
			continue
		}
		syncResult, err := a.syncLocalNewAPIInstanceData(ctx, instance.ID, input, false)
		if err != nil {
			result.FailedInstances++
			result.Messages = append(result.Messages, instance.Name+"："+err.Error())
			continue
		}
		result.ProcessedInstances++
		result.ImportedChannels += intFromResult(syncResult, "importedCount")
		result.SitesCreated += intFromResult(syncResult, "sitesCreated")
		result.SitesMerged += intFromResult(syncResult, "sitesMerged")

		sourceInput := localNewAPISyncSourceInput{UserID: input.UserID, PageSize: input.PageSize}
		missingResult, err := a.reconcileMissingLocalNewAPIInstance(ctx, instance, sourceInput, false)
		if err != nil {
			result.FailedInstances++
			result.Messages = append(result.Messages, instance.Name+" 标记源端移除失败："+err.Error())
			continue
		}
		result.MissingChannels += intFromResult(missingResult, "missingCount")
	}
	return result
}

func (a *App) loadLocalNewAPIInstancesForScheduler(ctx context.Context) ([]LocalNewAPIInstance, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT id, name, base_url, COALESCE(detected_from,''), status,
		       COALESCE(version,''), COALESCE(database_path,''), COALESCE(last_scanned_at,''),
		       COALESCE(sync_access_token_masked,''), created_at, updated_at
		FROM local_newapi_instances
		ORDER BY updated_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []LocalNewAPIInstance{}
	for rows.Next() {
		var item LocalNewAPIInstance
		if err := rows.Scan(&item.ID, &item.Name, &item.BaseURL, &item.DetectedFrom, &item.Status, &item.Version, &item.DatabasePath, &item.LastScannedAt, &item.SyncTokenMasked, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.HasSyncToken = strings.TrimSpace(item.SyncTokenMasked) != ""
		item.SyncCapability = syncCapability(item)
		items = append(items, item)
	}
	return items, rows.Err()
}

func intFromResult(result map[string]interface{}, key string) int {
	switch value := result[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case json.Number:
		parsed, _ := value.Int64()
		return int(parsed)
	default:
		return 0
	}
}

func stringFromResult(result map[string]interface{}, key string) string {
	if value, ok := result[key].(string); ok {
		return value
	}
	return ""
}

func boolFromResult(result map[string]interface{}, key string) bool {
	if value, ok := result[key].(bool); ok {
		return value
	}
	return false
}

func (a *App) schedulerStartTime() time.Time {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.schedulerStartedAt.IsZero() {
		return nowCST()
	}
	return a.schedulerStartedAt
}

func (a *App) resetInterruptedSchedulerRuns(ctx context.Context) error {
	_, err := a.db.ExecContext(ctx, `
		UPDATE scheduler_runs
		SET status='idle',
		    summary=CASE WHEN COALESCE(summary,'')='' THEN '上次程序退出时任务未完成，已重置。' ELSE summary END,
		    updated_at=?
		WHERE status='running'
	`, now())
	return err
}

func (a *App) beginSchedulerJob(ctx context.Context, jobKey string, runKey string) bool {
	if jobKey == schedulerJobSync {
		if !a.localSyncRun.TryStart() {
			return false
		}
	}
	if jobKey == schedulerJobChannelHealth {
		if !a.channelHealthRun.TryStart() {
			return false
		}
	}
	_, err := a.db.ExecContext(ctx, `
		INSERT INTO scheduler_runs (job_key, status, planned_run_key, last_run_key, last_started_at, last_error, summary, updated_at)
		VALUES (?, 'running', ?, ?, ?, '', '', ?)
		ON CONFLICT(job_key) DO UPDATE SET
			status='running',
			planned_run_key=excluded.planned_run_key,
			last_run_key=excluded.last_run_key,
			last_started_at=excluded.last_started_at,
			last_error='',
			summary='',
			updated_at=excluded.updated_at
	`, jobKey, runKey, runKey, now(), now())
	if err != nil {
		if jobKey == schedulerJobSync {
			a.localSyncRun.Finish()
		}
		if jobKey == schedulerJobChannelHealth {
			a.channelHealthRun.Finish()
		}
		return false
	}
	return true
}

func (a *App) finishSchedulerJob(ctx context.Context, jobKey string, runKey string, status string, summary string, errMessage string) error {
	if jobKey == schedulerJobSync {
		a.localSyncRun.Finish()
	}
	if jobKey == schedulerJobChannelHealth {
		a.channelHealthRun.Finish()
	}
	finishedAt := now()
	lastSuccessAt := ""
	if status == "success" || status == "warning" || status == "skipped" {
		lastSuccessAt = finishedAt
	}
	_, err := a.db.ExecContext(ctx, `
		INSERT INTO scheduler_runs (job_key, status, last_run_key, last_finished_at, last_success_at, last_error, summary, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(job_key) DO UPDATE SET
			status=excluded.status,
			last_run_key=excluded.last_run_key,
			last_finished_at=excluded.last_finished_at,
			last_success_at=CASE WHEN excluded.last_success_at<>'' THEN excluded.last_success_at ELSE scheduler_runs.last_success_at END,
			last_error=excluded.last_error,
			summary=excluded.summary,
			updated_at=excluded.updated_at
	`, jobKey, status, runKey, finishedAt, lastSuccessAt, errMessage, summary, finishedAt)
	return err
}

func (a *App) upsertSchedulerPlan(ctx context.Context, jobKey string, plannedRunKey string, nextRunAt string, summary string) error {
	timestamp := now()
	_, err := a.db.ExecContext(ctx, `
		INSERT INTO scheduler_runs (job_key, status, planned_run_key, next_run_at, summary, updated_at)
		VALUES (?, 'scheduled', ?, ?, ?, ?)
		ON CONFLICT(job_key) DO UPDATE SET
			status=CASE WHEN scheduler_runs.status='running' THEN scheduler_runs.status ELSE excluded.status END,
			planned_run_key=excluded.planned_run_key,
			next_run_at=excluded.next_run_at,
			summary=CASE WHEN scheduler_runs.status='running' THEN scheduler_runs.summary ELSE excluded.summary END,
			updated_at=excluded.updated_at
	`, jobKey, plannedRunKey, nextRunAt, summary, timestamp)
	return err
}

func (a *App) loadSchedulerRun(ctx context.Context, jobKey string) (schedulerRunRecord, error) {
	var record schedulerRunRecord
	var planned, nextRun, lastRun, started, finished, success, errText, summary sql.NullString
	err := a.db.QueryRowContext(ctx, `
		SELECT job_key, status, planned_run_key, next_run_at, last_run_key,
		       last_started_at, last_finished_at, last_success_at, last_error, summary, updated_at
		FROM scheduler_runs
		WHERE job_key=?
	`, jobKey).Scan(&record.JobKey, &record.Status, &planned, &nextRun, &lastRun, &started, &finished, &success, &errText, &summary, &record.UpdatedAt)
	if err != nil {
		return record, err
	}
	record.PlannedRunKey = planned.String
	record.NextRunAt = nextRun.String
	record.LastRunKey = lastRun.String
	record.LastStartedAt = started.String
	record.LastFinishedAt = finished.String
	record.LastSuccessAt = success.String
	record.LastError = errText.String
	record.Summary = summary.String
	return record, nil
}

func (a *App) buildSchedulerStatus(ctx context.Context) SchedulerStatus {
	status := SchedulerStatus{
		GeneratedAt: now(),
		Jobs:        []SchedulerJobStatus{},
	}
	for _, job := range []struct {
		key   string
		label string
	}{
		{schedulerJobCheckin, "自动签到"},
		{schedulerJobSync, "NewAPI 定时同步"},
		{schedulerJobChannelHealth, "渠道健康探测"},
	} {
		record, err := a.loadSchedulerRun(ctx, job.key)
		item := SchedulerJobStatus{Key: job.key, Label: job.label, Status: "idle"}
		if err == nil {
			item.Status = firstNonEmpty(record.Status, "idle")
			item.PlannedRunKey = record.PlannedRunKey
			item.NextRunAt = record.NextRunAt
			item.LastRunKey = record.LastRunKey
			item.LastStartedAt = record.LastStartedAt
			item.LastFinishedAt = record.LastFinishedAt
			item.LastSuccessAt = record.LastSuccessAt
			item.LastError = record.LastError
			item.Summary = record.Summary
			item.UpdatedAt = record.UpdatedAt
		}
		status.Jobs = append(status.Jobs, item)
	}
	return status
}

func (a *App) loadCheckinScheduleConfig(ctx context.Context) checkinScheduleConfig {
	config := checkinScheduleConfig{
		Enabled:                true,
		Time:                   "08:00",
		RandomDelayMinutes:     []int{0, 120},
		SiteConcurrency:        1,
		GlobalConcurrency:      3,
		SiteMinIntervalSeconds: 2,
	}
	_ = a.loadSettingJSON(ctx, "checkin.schedule", &config)
	if config.Time == "" {
		config.Time = "08:00"
	}
	if config.SiteMinIntervalSeconds < 0 {
		config.SiteMinIntervalSeconds = 0
	}
	if config.SiteMinIntervalSeconds > 60 {
		config.SiteMinIntervalSeconds = 60
	}
	return config
}

func (a *App) loadSyncScheduleConfig(ctx context.Context) syncScheduleConfig {
	config := syncScheduleConfig{
		Enabled:         true,
		IntervalMinutes: 30,
		Mode:            "local-newapi",
		RunOnStartup:    false,
	}
	_ = a.loadSettingJSON(ctx, "sync.schedule", &config)
	if config.IntervalMinutes < 5 {
		config.IntervalMinutes = 30
	}
	if strings.TrimSpace(config.Mode) == "" {
		config.Mode = "local-newapi"
	}
	return config
}

func (a *App) loadChannelHealthScheduleConfig(ctx context.Context) channelHealthScheduleConfig {
	config := channelHealthScheduleConfig{
		Enabled:         true,
		IntervalMinutes: 60,
		RunOnStartup:    false,
		Limit:           20,
		OnlyRisky:       false,
	}
	_ = a.loadSettingJSON(ctx, "channel.health.schedule", &config)
	return normalizeChannelHealthScheduleConfig(config)
}

func normalizeChannelHealthScheduleConfig(config channelHealthScheduleConfig) channelHealthScheduleConfig {
	if config.IntervalMinutes < 5 {
		config.IntervalMinutes = 30
	}
	if config.IntervalMinutes > 1440 {
		config.IntervalMinutes = 1440
	}
	if config.Limit <= 0 {
		config.Limit = 20
	}
	if config.Limit > 50 {
		config.Limit = 50
	}
	return config
}

func (a *App) loadSettingJSON(ctx context.Context, key string, target interface{}) error {
	return a.schedulerRepo.LoadSettingJSON(ctx, key, target)
}

func makeCheckinPlan(config checkinScheduleConfig, currentTime time.Time) checkinPlan {
	plan := checkinPlan{Enabled: config.Enabled}
	if !config.Enabled {
		plan.Message = "自动签到未启用。"
		return plan
	}
	parsedTime, err := time.Parse("15:04", firstNonEmpty(config.Time, "08:00"))
	if err != nil {
		plan.Enabled = false
		plan.Message = "签到时间格式无效，请使用 HH:MM。"
		return plan
	}
	minDelay, maxDelay := normalizedRandomDelay(config.RandomDelayMinutes)
	base := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), parsedTime.Hour(), parsedTime.Minute(), 0, 0, currentTime.Location())
	start := base.Add(time.Duration(minDelay) * time.Minute)
	end := base.Add(time.Duration(maxDelay) * time.Minute)
	if currentTime.After(end) {
		base = base.Add(24 * time.Hour)
		start = base.Add(time.Duration(minDelay) * time.Minute)
		end = base.Add(time.Duration(maxDelay) * time.Minute)
	}
	plan.Start = start
	plan.End = end
	plan.RunAt = start
	plan.RunKey = start.Format("2006-01-02")
	plan.Message = "等待自动签到窗口。"
	return plan
}

func normalizedRandomDelay(values []int) (int, int) {
	minDelay := 0
	maxDelay := 0
	if len(values) >= 2 {
		minDelay = values[0]
		maxDelay = values[1]
	}
	if minDelay < 0 {
		minDelay = 0
	}
	if maxDelay < minDelay {
		maxDelay = minDelay
	}
	return minDelay, maxDelay
}

func randomTimeInWindow(start time.Time, end time.Time) time.Time {
	if !end.After(start) {
		return start
	}
	seconds := int64(end.Sub(start).Seconds())
	if seconds <= 0 {
		return start
	}
	offset, err := rand.Int(rand.Reader, big.NewInt(seconds+1))
	if err != nil {
		return start
	}
	return start.Add(time.Duration(offset.Int64()) * time.Second)
}

func timeWithinPlan(value string, start time.Time, end time.Time) bool {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return false
	}
	// Interpret the stored timestamp in CST so the comparison is correct
	// regardless of the server's local timezone.
	local := parsed.In(cstZone())
	return !local.Before(start) && !local.After(end)
}

func (a *App) applySchedulerPlanToCheckinStatus(ctx context.Context, currentTime time.Time, status *CheckinScheduleStatus) {
	record, err := a.loadSchedulerRun(ctx, schedulerJobCheckin)
	if err != nil || record.NextRunAt == "" || !status.Enabled {
		return
	}
	nextRun, err := time.Parse(time.RFC3339Nano, record.NextRunAt)
	if err != nil || nextRun.Before(currentTime.Add(-1*time.Minute)) {
		return
	}
	status.NextRunAt = record.NextRunAt
	status.NextRunInSeconds = maxInt64(0, int64(nextRun.Sub(currentTime).Seconds()))
}
