package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

// TaskType identifies a batch operation.
type TaskType string

const (
	TaskCheckin         TaskType = "checkin"
	TaskTestKeys        TaskType = "test_keys"
	TaskRefreshBalances TaskType = "refresh_balances"
	TaskDetectSites     TaskType = "detect_sites"
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
	tasks map[string]*runningTask
	mu    sync.RWMutex
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
		}
	}
}

// --- Task handlers ---

func (a *App) startCheckinTask(taskID string, _ map[string]interface{}) {
	go func() {
		ctx := context.Background()
		accounts, err := a.loadDueCheckinAccounts(ctx, 0)
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
		for _, account := range accounts {
			if taskCtx.Err() != nil {
				task.finish(taskCtx.Err())
				return
			}
			_ = siteLimiter.wait(taskCtx, account.UpstreamSiteID)
			result, err := a.runAccountCheckin(taskCtx, account.ID)
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
		for _, j := range jobs {
			if taskCtx.Err() != nil {
				task.finish(taskCtx.Err())
				return
			}
			result := a.testAPIKeyForAccount(taskCtx, j.ID)
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
		for _, j := range jobs {
			if taskCtx.Err() != nil {
				task.finish(taskCtx.Err())
				return
			}
			item := a.refreshBalanceForBulk(taskCtx, j.ID)
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
