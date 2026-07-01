package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// taskCleanupTTL is how long a finished task's entry remains in the
// TaskRunner.tasks map before being garbage-collected. The TTL gives SSE
// clients a window to attach (or re-attach) and read the final snapshot
// after the task has completed, while bounding memory growth from
// accumulated finished entries.
const taskCleanupTTL = 5 * time.Minute

// sseHeartbeatInterval is how often the SSE handler emits a comment line
// to keep the connection alive through proxies and to detect dead clients
// (write failure returns the handler, releasing the subscriber).
const sseHeartbeatInterval = 15 * time.Second

// maxSSESubscribers caps the total number of concurrent SSE task-stream
// connections the process will accept. Each connection holds a goroutine
// and a subscriber channel; without a cap a misbehaving client (or a
// retry storm) can exhaust goroutines and memory. Desktop-scale app: 50
// is far above any legitimate usage.
const maxSSESubscribers = 50

// TaskType identifies a batch operation.
type TaskType string

const (
	TaskCheckin            TaskType = "checkin"
	TaskTestKeys           TaskType = "test_keys"
	TaskRefreshBalances    TaskType = "refresh_balances"
	TaskDetectSites        TaskType = "detect_sites"
	TaskChannelHealthProbe TaskType = "channel_health_probe"
)

// TaskStatus tracks the lifecycle of a task.
type TaskStatus string

const (
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusDone      TaskStatus = "done"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// ItemResult is the per-item result pushed via SSE.
type ItemResult struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// TaskProgress is the SSE payload.
type TaskProgress struct {
	ID        string       `json:"id"`
	Type      TaskType     `json:"type"`
	Status    TaskStatus   `json:"status"`
	Current   int          `json:"current"`
	Total     int          `json:"total"`
	Results   []ItemResult `json:"results"`
	StartedAt string       `json:"startedAt"`
	UpdatedAt string       `json:"updatedAt"`
	Error     string       `json:"error,omitempty"`
}

type channelHealthProbeJob struct {
	ID      string
	Name    string
	BaseURL string
}

type channelHealthProbeResult struct {
	Total     int
	Processed int
	Failed    int
	Warning   int
	Items     []ItemResult
	Messages  []string
}

func (r channelHealthProbeResult) Summary() string {
	parts := []string{
		fmt.Sprintf("processed %d/%d", r.Processed, r.Total),
		fmt.Sprintf("warnings %d", r.Warning),
		fmt.Sprintf("failed %d", r.Failed),
	}
	if len(r.Messages) > 0 {
		parts = append(parts, strings.Join(r.Messages, "; "))
	}
	samples := r.RiskSamples(3)
	if len(samples) > 0 {
		parts = append(parts, "samples: "+strings.Join(samples, "; "))
	}
	return strings.Join(parts, "; ")
}

func (r channelHealthProbeResult) RiskSamples(limit int) []string {
	if limit <= 0 {
		return nil
	}
	samples := []string{}
	for _, item := range r.Items {
		if item.Status != "warning" && item.Status != "failed" {
			continue
		}
		sample := strings.TrimSpace(item.Name)
		if sample == "" {
			sample = item.ID
		}
		if item.Message != "" {
			sample += " (" + item.Message + ")"
		}
		samples = append(samples, sample)
		if len(samples) >= limit {
			break
		}
	}
	return samples
}

type runningTask struct {
	progress TaskProgress
	mu       sync.RWMutex
	cancel   context.CancelFunc
	subs     []chan TaskProgress
	subMu    sync.Mutex
	done     chan struct{}
}

// TaskRunner manages all running batch tasks.
type TaskRunner struct {
	tasks           map[string]*runningTask
	mu              sync.RWMutex
	sseSubscribers  atomic.Int64
}

func newTaskRunner() *TaskRunner {
	return &TaskRunner{tasks: map[string]*runningTask{}}
}

func (tr *TaskRunner) start(id string, taskType TaskType, total int) (*runningTask, context.Context) {
	ctx, cancel := context.WithCancel(context.Background())
	task := &runningTask{
		progress: TaskProgress{
			ID:        id,
			Type:      taskType,
			Status:    TaskStatusRunning,
			Total:     total,
			StartedAt: now(),
			UpdatedAt: now(),
			Results:   []ItemResult{},
		},
		cancel: cancel,
		done:   make(chan struct{}),
	}
	tr.mu.Lock()
	tr.tasks[id] = task
	tr.mu.Unlock()
	// S1: Schedule deferred cleanup so finished task entries don't leak the
	// tasks map. We wait for task.done (closed by finish()), then sleep the
	// TTL, then remove the entry. If a new task reuses the same id before
	// cleanup, the map already points to the new entry; only delete when
	// the entry still matches this task pointer.
	go func() {
		<-task.done
		time.Sleep(taskCleanupTTL)
		tr.mu.Lock()
		if current := tr.tasks[id]; current == task {
			delete(tr.tasks, id)
		}
		tr.mu.Unlock()
	}()
	return task, ctx
}

func (tr *TaskRunner) get(id string) *runningTask {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	return tr.tasks[id]
}

func (tr *TaskRunner) cancelTask(id string) bool {
	task := tr.get(id)
	if task == nil {
		return false
	}
	task.cancel()
	return true
}

func (t *runningTask) update(item ItemResult) {
	t.mu.Lock()
	t.progress.Current++
	t.progress.Results = append(t.progress.Results, item)
	t.progress.UpdatedAt = now()
	p := t.progress
	t.mu.Unlock()
	t.broadcast(p)
}

func (t *runningTask) finish(err error) {
	t.mu.Lock()
	if err != nil {
		t.progress.Status = TaskStatusCancelled
		t.progress.Error = err.Error()
	} else {
		t.progress.Status = TaskStatusDone
	}
	t.progress.UpdatedAt = now()
	p := t.progress
	t.mu.Unlock()
	t.broadcast(p)
	close(t.done)
}

func (t *runningTask) snapshot() TaskProgress {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.progress
}

func (t *runningTask) subscribe() chan TaskProgress {
	ch := make(chan TaskProgress, 32)
	t.subMu.Lock()
	t.subs = append(t.subs, ch)
	t.subMu.Unlock()
	return ch
}

func (t *runningTask) unsubscribe(ch chan TaskProgress) {
	t.subMu.Lock()
	for i, sub := range t.subs {
		if sub == ch {
			t.subs = append(t.subs[:i], t.subs[i+1:]...)
			close(ch)
			break
		}
	}
	t.subMu.Unlock()
}

func (t *runningTask) broadcast(p TaskProgress) {
	t.subMu.Lock()
	for _, ch := range t.subs {
		select {
		case ch <- p:
		default:
		}
	}
	t.subMu.Unlock()
}

// --- HTTP handlers ---

func (a *App) handleTaskStart(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var input struct {
		Type   string                 `json:"type"`
		Params map[string]interface{} `json:"params"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "请求参数无效。")
		return
	}

	taskType := TaskType(input.Type)
	taskID := newID()

	switch taskType {
	case TaskCheckin:
		a.startCheckinTask(taskID, input.Params)
	case TaskTestKeys:
		a.startTestKeysTask(taskID, input.Params)
	case TaskRefreshBalances:
		a.startRefreshBalancesTask(taskID, input.Params)
	case TaskDetectSites:
		a.startDetectSitesTask(taskID, input.Params)
	case TaskChannelHealthProbe:
		a.startChannelHealthProbeTask(taskID, input.Params)
	default:
		writeError(w, http.StatusBadRequest, "未知的任务类型。")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"taskId": taskID})
}

func (a *App) handleTaskCancel(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	taskID := pathTail(r.URL.Path, "/api/tasks/")
	taskID = strings.TrimSuffix(taskID, "/cancel")
	if a.taskRunner.cancelTask(taskID) {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	} else {
		writeError(w, http.StatusNotFound, "任务不存在或已完成。")
	}
}

func (a *App) handleTaskStream(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	taskID = strings.TrimSuffix(taskID, "/stream")

	// S3: Enforce a per-process SSE subscriber cap. Each connection holds a
	// goroutine and a subscriber channel for the lifetime of the stream;
	// without a cap a client retry storm can exhaust goroutines. We count
	// before looking up the task so rejected connections don't need to
	// touch the tasks map at all.
	if a.taskRunner.sseSubscribers.Add(1) > maxSSESubscribers {
		a.taskRunner.sseSubscribers.Add(-1)
		writeError(w, http.StatusServiceUnavailable, "SSE 连接数已达上限，请稍后重试。")
		return
	}
	defer a.taskRunner.sseSubscribers.Add(-1)

	task := a.taskRunner.get(taskID)
	if task == nil {
		writeError(w, http.StatusNotFound, "任务不存在。")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "当前服务器不支持流式推送。")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Send initial snapshot.
	snapshot := task.snapshot()
	if data, err := json.Marshal(snapshot); err == nil {
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	ch := task.subscribe()
	defer task.unsubscribe(ch)

	// S2: Heartbeat ticker. SSE comment lines (": ...") are ignored by
	// EventSource clients but keep the connection alive through idle
	// proxies and let us detect dead clients when the write fails.
	ticker := time.NewTicker(sseHeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case p, ok := <-ch:
			if !ok {
				return
			}
			if data, err := json.Marshal(p); err == nil {
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			}
			if p.Status != TaskStatusRunning {
				return
			}
		case <-task.done:
			final := task.snapshot()
			if data, err := json.Marshal(final); err == nil {
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			}
			return
		case <-ticker.C:
			// Write failure indicates the client has disconnected; bail
			// out so the subscriber is released and the goroutine exits.
			if _, err := fmt.Fprintf(w, ": heartbeat\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// --- Task handlers ---

func (a *App) startCheckinTask(taskID string, _ map[string]interface{}) {
	go func() {
		ctx := context.Background()
		accounts, err := a.loadDueCheckinAccounts(ctx, "", 0)
		if err != nil {
			task, _ := a.taskRunner.start(taskID, TaskCheckin, 0)
			task.finish(err)
			return
		}
		total := len(accounts)
		task, taskCtx := a.taskRunner.start(taskID, TaskCheckin, total)

		if total == 0 {
			task.finish(nil)
			return
		}

		siteLimiter := newCheckinSiteLimiter(a.loadCheckinScheduleConfig(ctx))
		accountIDs := make([]string, 0, len(accounts))
		for _, account := range accounts {
			accountIDs = append(accountIDs, account.ID)
		}
		auths, _ := a.loadAccountAuths(ctx, accountIDs)
		for _, account := range accounts {
			if taskCtx.Err() != nil {
				task.finish(taskCtx.Err())
				return
			}
			_ = siteLimiter.wait(taskCtx, account.UpstreamSiteID)
			var auth *accountAuthContext
			if loaded, ok := auths[account.ID]; ok {
				auth = &loaded
			}
			result, err := a.runAccountCheckin(taskCtx, account.ID, auth)
			item := ItemResult{ID: account.ID, Name: account.AccountName}
			if err != nil {
				item.Status = "failed"
				item.Message = err.Error()
			} else {
				item.Status = result.Status
				item.Message = result.Message
			}
			task.update(item)
		}
		task.finish(nil)
	}()
}

func (a *App) startTestKeysTask(taskID string, params map[string]interface{}) {
	go func() {
		ctx := context.Background()
		limit := 50
		if l, ok := params["limit"].(float64); ok && l > 0 {
			limit = int(l)
		}

		rows, err := a.db.QueryContext(ctx, `
			SELECT id, COALESCE(display_name, username, id)
			FROM channel_accounts
			WHERE COALESCE(api_key_encrypted,'') <> ''
			ORDER BY COALESCE(api_key_last_checked_at,''), updated_at DESC
			LIMIT ?
		`, limit)
		if err != nil {
			task, _ := a.taskRunner.start(taskID, TaskTestKeys, 0)
			task.finish(err)
			return
		}
		type job struct{ ID, Name string }
		jobs := []job{}
		for rows.Next() {
			var j job
			_ = rows.Scan(&j.ID, &j.Name)
			jobs = append(jobs, j)
		}
		_ = rows.Close()

		task, taskCtx := a.taskRunner.start(taskID, TaskTestKeys, len(jobs))
		jobIDs := make([]string, 0, len(jobs))
		for _, j := range jobs {
			jobIDs = append(jobIDs, j.ID)
		}
		auths, _ := a.loadAccountAuths(ctx, jobIDs)
		for _, j := range jobs {
			if taskCtx.Err() != nil {
				task.finish(taskCtx.Err())
				return
			}
			var auth *accountAuthContext
			if loaded, ok := auths[j.ID]; ok {
				auth = &loaded
			}
			result := a.testAPIKeyForAccount(taskCtx, j.ID, auth)
			item := ItemResult{ID: j.ID, Name: j.Name, Status: result.Status}
			if result.Status != "valid" {
				item.Message = result.Message
			}
			task.update(item)
		}
		task.finish(nil)
	}()
}

func (a *App) startRefreshBalancesTask(taskID string, params map[string]interface{}) {
	go func() {
		ctx := context.Background()
		limit := 50
		missingOnly := false
		if l, ok := params["limit"].(float64); ok && l > 0 {
			limit = int(l)
		}
		if m, ok := params["missingOnly"].(bool); ok {
			missingOnly = m
		}

		query := `
			SELECT a.id, COALESCE(a.display_name, a.username, a.id)
			FROM channel_accounts a
			JOIN upstream_sites s ON s.id = a.upstream_site_id
			WHERE s.supports_balance = 1
		`
		if missingOnly {
			query += ` AND a.balance IS NULL`
		}
		query += ` ORDER BY COALESCE(a.last_validated_at,''), a.updated_at DESC LIMIT ?`

		rows, err := a.db.QueryContext(ctx, query, limit)
		if err != nil {
			task, _ := a.taskRunner.start(taskID, TaskRefreshBalances, 0)
			task.finish(err)
			return
		}
		type job struct{ ID, Name string }
		jobs := []job{}
		for rows.Next() {
			var j job
			_ = rows.Scan(&j.ID, &j.Name)
			jobs = append(jobs, j)
		}
		_ = rows.Close()

		task, taskCtx := a.taskRunner.start(taskID, TaskRefreshBalances, len(jobs))
		jobIDs := make([]string, 0, len(jobs))
		for _, j := range jobs {
			jobIDs = append(jobIDs, j.ID)
		}
		auths, _ := a.loadAccountAuths(ctx, jobIDs)
		for _, j := range jobs {
			if taskCtx.Err() != nil {
				task.finish(taskCtx.Err())
				return
			}
			var auth *accountAuthContext
			if loaded, ok := auths[j.ID]; ok {
				auth = &loaded
			}
			item := a.refreshBalanceForBulk(taskCtx, j.ID, auth)
			result := ItemResult{ID: j.ID, Name: j.Name, Status: item.Status}
			if item.Status != "success" {
				result.Message = item.Message
			}
			task.update(result)
		}
		task.finish(nil)
	}()
}

func (a *App) startDetectSitesTask(taskID string, params map[string]interface{}) {
	go func() {
		ctx := context.Background()
		limit := 50
		onlyUnknownOrOpenAI := false
		if l, ok := params["limit"].(float64); ok && l > 0 {
			limit = int(l)
		}
		if u, ok := params["onlyUnknownOrOpenAI"].(bool); ok {
			onlyUnknownOrOpenAI = u
		}

		query := `
			SELECT id, name, base_url
			FROM upstream_sites
			WHERE COALESCE(base_url,'') <> ''
			  AND lower(name) NOT LIKE '%9router%'
			  AND lower(base_url) <> 'http://localhost:20128'
		`
		args := []interface{}{}
		if onlyUnknownOrOpenAI {
			query += ` AND kind IN ('unknown','openai_compatible')`
		}
		query += ` ORDER BY updated_at DESC LIMIT ?`
		args = append(args, limit)

		rows, err := a.db.QueryContext(ctx, query, args...)
		if err != nil {
			task, _ := a.taskRunner.start(taskID, TaskDetectSites, 0)
			task.finish(err)
			return
		}
		type job struct{ ID, Name, BaseURL string }
		jobs := []job{}
		for rows.Next() {
			var j job
			_ = rows.Scan(&j.ID, &j.Name, &j.BaseURL)
			jobs = append(jobs, j)
		}
		_ = rows.Close()

		task, taskCtx := a.taskRunner.start(taskID, TaskDetectSites, len(jobs))
		for _, j := range jobs {
			if taskCtx.Err() != nil {
				task.finish(taskCtx.Err())
				return
			}
			result := a.detectAndSaveSite(taskCtx, j.ID, j.Name, j.BaseURL)
			item := ItemResult{ID: j.ID, Name: j.Name, Status: "success"}
			if result.Kind != "" {
				item.Message = result.Kind
			}
			task.update(item)
		}
		task.finish(nil)
	}()
}

func (a *App) startChannelHealthProbeTask(taskID string, params map[string]interface{}) {
	go func() {
		ctx := context.Background()
		limit := 20
		onlyRisky := false
		if l, ok := params["limit"].(float64); ok && l > 0 {
			limit = int(l)
		}
		if limit > 50 {
			limit = 50
		}
		if r, ok := params["onlyRisky"].(bool); ok {
			onlyRisky = r
		}
		jobs, err := a.loadChannelHealthProbeJobs(ctx, limit, onlyRisky)
		if err != nil {
			task, _ := a.taskRunner.start(taskID, TaskChannelHealthProbe, 0)
			task.finish(err)
			return
		}

		task, taskCtx := a.taskRunner.start(taskID, TaskChannelHealthProbe, len(jobs))
		_ = a.runChannelHealthProbe(taskCtx, jobs, func(item ItemResult) {
			task.update(item)
		})
		if taskCtx.Err() != nil {
			task.finish(taskCtx.Err())
			return
		}
		task.finish(nil)
	}()
}

func (a *App) loadChannelHealthProbeJobs(ctx context.Context, limit int, onlyRisky bool) ([]channelHealthProbeJob, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	query := `
		SELECT s.id, s.name, s.base_url
		FROM upstream_sites s
		WHERE COALESCE(s.base_url,'') <> ''
		  AND s.id <> ?
		  AND lower(s.name) NOT LIKE '%9router%'
		  AND lower(s.base_url) <> 'http://localhost:20128'
	`
	args := []interface{}{globalScheduleSiteID}
	if onlyRisky {
		query += ` AND (
			s.health_status IN ('unknown','unreachable','down','failed','error')
			OR EXISTS (
				SELECT 1 FROM imported_channels c
				WHERE COALESCE(c.source_sync_status,'active') <> 'archived'
				  AND c.upstream_kind IN ('newapi','oneapi','sub2api','modified_relay')
				  AND (s.channel_id = c.id OR (COALESCE(s.channel_id,'') = '' AND COALESCE(s.base_url,'') <> '' AND s.base_url = COALESCE(c.base_url,'')))
				  AND COALESCE(c.models_status,'unchecked') IN ('unchecked','failed','key_invalid','empty','')
			)
			OR EXISTS (
				SELECT 1 FROM channel_accounts account
				WHERE account.upstream_site_id = s.id
				  AND COALESCE(account.api_key_fingerprint,'') <> ''
				  AND COALESCE(account.api_key_status,'unchecked') NOT IN ('valid')
			)
		)`
	}
	query += ` ORDER BY s.updated_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := a.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := []channelHealthProbeJob{}
	for rows.Next() {
		var job channelHealthProbeJob
		if err := rows.Scan(&job.ID, &job.Name, &job.BaseURL); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (a *App) runChannelHealthProbe(ctx context.Context, jobs []channelHealthProbeJob, onItem func(ItemResult)) channelHealthProbeResult {
	result := channelHealthProbeResult{Total: len(jobs), Items: []ItemResult{}}
	for _, job := range jobs {
		if ctx.Err() != nil {
			result.Messages = append(result.Messages, ctx.Err().Error())
			break
		}
		item := a.probeChannelHealthSite(ctx, job.ID, job.Name, job.BaseURL)
		result.Processed++
		switch item.Status {
		case "failed":
			result.Failed++
		case "warning":
			result.Warning++
		}
		result.Items = append(result.Items, item)
		if onItem != nil {
			onItem(item)
		}
	}
	// Per-key invalidation: channel health probe changes channel health data,
	// which affects channel-health-overview, channels-list, action-center
	// (channel health risks), and dashboard-summary (counts).
	a.invalidateReadCacheKeys("channel-health-overview", "channels-list", "action-center", "dashboard-summary")
	return result
}

func (a *App) probeChannelHealthSite(ctx context.Context, id, name, baseURL string) ItemResult {
	detection := a.detectAndSaveSite(ctx, id, name, baseURL)
	item := ItemResult{ID: id, Name: name, Status: "success", Message: detection.HealthStatus}
	if detection.Error != "" {
		item.Status = "failed"
		item.Message = detection.Error
		return item
	}
	a.syncModelsForHealthSite(ctx, id, detection.BaseURL)
	switch strings.ToLower(detection.HealthStatus) {
	case "unreachable", "down", "failed", "error":
		item.Status = "failed"
	case "unknown", "degraded", "auth_required", "blocked":
		item.Status = "warning"
	}
	return item
}

func (a *App) syncModelsForHealthSite(ctx context.Context, siteID string, baseURL string) {
	records, err := a.loadChannelModelSyncRecordsForHealthSite(ctx, siteID, baseURL)
	if err != nil {
		return
	}
	for _, record := range records {
		if ctx.Err() != nil {
			return
		}
		a.syncChannelModels(ctx, record)
	}
}

func (a *App) loadChannelModelSyncRecordsForHealthSite(ctx context.Context, siteID string, baseURL string) ([]channelModelSyncRecord, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	rows, err := a.db.QueryContext(ctx, `
		SELECT c.id, c.name, COALESCE(c.base_url,''), c.upstream_kind, COALESCE(c.raw_json,''),
		       COALESCE(c.channel_key_encrypted,''), COALESCE(c.model_count,0),
		       COALESCE(c.sample_models_json,''), COALESCE(c.models_source,''), COALESCE(c.models_status,''),
		       COALESCE(c.models_last_synced_at,''), COALESCE(c.models_message,'')
		FROM imported_channels c
		LEFT JOIN upstream_sites s
		  ON (s.channel_id = c.id OR (COALESCE(s.channel_id,'') = '' AND COALESCE(s.base_url,'') <> '' AND s.base_url = COALESCE(c.base_url,'')))
		WHERE COALESCE(c.source_sync_status,'active') <> 'archived'
		  AND c.upstream_kind IN ('newapi','oneapi','sub2api','modified_relay')
		  AND (s.id = ? OR (? <> '' AND COALESCE(c.base_url,'') = ?))
		ORDER BY c.updated_at DESC
		LIMIT 10
	`, siteID, baseURL, baseURL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := []channelModelSyncRecord{}
	for rows.Next() {
		var record channelModelSyncRecord
		if err := rows.Scan(&record.ID, &record.Name, &record.BaseURL, &record.Kind, &record.RawJSON, &record.ChannelKeyEncrypted, &record.ModelCount, &record.SampleModelsJSON, &record.ModelsSource, &record.ModelsStatus, &record.ModelsLastSyncedAt, &record.ModelsMessage); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}
