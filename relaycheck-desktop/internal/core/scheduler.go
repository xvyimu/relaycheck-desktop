package core

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"
)

const (
	schedulerJobCheckin = "checkin.daily"
	schedulerJobSync    = "sync.local_newapi"
)

type syncJobRunState struct {
	Running bool
}

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

func (a *App) StartSchedulers(parent context.Context) {
	a.mu.Lock()
	if a.schedulerCancel != nil {
		a.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(parent)
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
	_ = a.resetInterruptedSchedulerRuns(context.Background())
	a.tickSchedulers(ctx)

	ticker := time.NewTicker(30 * time.Second)
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
	nowTime := time.Now()
	a.tickCheckinScheduler(ctx, nowTime)
	a.tickSyncScheduler(ctx, nowTime)
}

func (a *App) tickCheckinScheduler(ctx context.Context, currentTime time.Time) {
	config := a.loadCheckinScheduleConfig(ctx)
	plan := makeCheckinPlan(config, currentTime)
	record, _ := a.loadSchedulerRun(ctx, schedulerJobCheckin)
	if !plan.Enabled {
		_ = a.upsertSchedulerPlan(ctx, schedulerJobCheckin, "", "", "自动签到未启用。")
		return
	}

	nextRunAt := record.NextRunAt
	if record.PlannedRunKey != plan.RunKey || !timeWithinPlan(nextRunAt, plan.Start, plan.End) {
		runAt := randomTimeInWindow(plan.Start, plan.End)
		plan.RunAt = runAt
		nextRunAt = runAt.UTC().Format(time.RFC3339Nano)
		_ = a.upsertSchedulerPlan(ctx, schedulerJobCheckin, plan.RunKey, nextRunAt, "等待自动签到窗口。")
	} else if parsed, err := time.Parse(time.RFC3339Nano, nextRunAt); err == nil {
		plan.RunAt = parsed.Local()
	}

	if record.LastRunKey == plan.RunKey || currentTime.Before(plan.RunAt) || currentTime.After(plan.End) {
		return
	}
	if !a.beginSchedulerJob(ctx, schedulerJobCheckin, plan.RunKey) {
		return
	}

	jobCtx, cancel := context.WithTimeout(ctx, 20*time.Minute)
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
	_ = a.finishSchedulerJob(context.Background(), schedulerJobCheckin, plan.RunKey, status, summary, errMessage)
}

func (a *App) tickSyncScheduler(ctx context.Context, currentTime time.Time) {
	config := a.loadSyncScheduleConfig(ctx)
	record, _ := a.loadSchedulerRun(ctx, schedulerJobSync)
	if !config.Enabled || config.Mode == "manual-only" {
		_ = a.upsertSchedulerPlan(ctx, schedulerJobSync, "", "", "定时同步未启用。")
		return
	}

	interval := time.Duration(config.IntervalMinutes) * time.Minute
	if interval < 5*time.Minute {
		interval = 30 * time.Minute
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
	_ = a.upsertSchedulerPlan(ctx, schedulerJobSync, runKey, nextRun.UTC().Format(time.RFC3339Nano), "等待下一次 NewAPI 同步。")
	if !due {
		return
	}
	if !a.beginSchedulerJob(ctx, schedulerJobSync, runKey) {
		return
	}

	jobCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
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
	_ = a.finishSchedulerJob(context.Background(), schedulerJobSync, runKey, status, result.Summary(), errMessage)
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
		return time.Now()
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
		a.mu.Lock()
		if a.localSyncRun.Running {
			a.mu.Unlock()
			return false
		}
		a.localSyncRun.Running = true
		a.mu.Unlock()
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
			a.mu.Lock()
			a.localSyncRun.Running = false
			a.mu.Unlock()
		}
		return false
	}
	return true
}

func (a *App) finishSchedulerJob(ctx context.Context, jobKey string, runKey string, status string, summary string, errMessage string) error {
	if jobKey == schedulerJobSync {
		a.mu.Lock()
		a.localSyncRun.Running = false
		a.mu.Unlock()
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

func (a *App) loadSettingJSON(ctx context.Context, key string, target interface{}) error {
	var valueJSON string
	if err := a.db.QueryRowContext(ctx, `SELECT value_json FROM system_settings WHERE key=?`, key).Scan(&valueJSON); err != nil {
		return err
	}
	return json.Unmarshal([]byte(valueJSON), target)
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
	local := parsed.Local()
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
