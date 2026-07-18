package core

import (
	"context"
	"encoding/json"
	"errors"
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
	tasks          map[string]*runningTask
	mu             sync.RWMutex
	pendingContext map[string]correlationFields
	sseSubscribers atomic.Int64
	rootCtx        context.Context
}

func newTaskRunner() *TaskRunner {
	return &TaskRunner{
		tasks:          map[string]*runningTask{},
		pendingContext: map[string]correlationFields{},
		rootCtx:        context.Background(),
	}
}

// setRootCtx links the TaskRunner's per-task cancellation contexts to the
// App's process-level rootCtx so that App.Close() interrupts in-flight tasks
// in addition to the prelude work executed by the start*Task handlers.
func (tr *TaskRunner) setRootCtx(ctx context.Context) {
	tr.mu.Lock()
	tr.rootCtx = ctx
	tr.mu.Unlock()
}

func (tr *TaskRunner) start(id string, taskType TaskType, total int) (*runningTask, context.Context) {
	tr.mu.Lock()
	root := tr.rootCtx
	fields := tr.pendingContext[id]
	delete(tr.pendingContext, id)
	tr.mu.Unlock()
	fields.TaskID = id
	ctx, cancel := context.WithCancel(withCorrelation(root, fields))
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

func (tr *TaskRunner) setPendingCorrelation(id string, fields correlationFields) {
	fields.TaskID = id
	tr.mu.Lock()
	tr.pendingContext[id] = fields
	tr.mu.Unlock()
}

func (tr *TaskRunner) clearPendingCorrelation(id string) {
	tr.mu.Lock()
	delete(tr.pendingContext, id)
	tr.mu.Unlock()
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
		switch {
		case errors.Is(err, context.Canceled):
			t.progress.Error = publicOperationFailure("tasks", "cancel", t.progress.ID, "任务已取消。", err)
		case errors.Is(err, context.DeadlineExceeded):
			t.progress.Error = publicOperationFailure("tasks", "timeout", t.progress.ID, "任务执行超时。", err)
		default:
			t.progress.Error = publicOperationFailure("tasks", "finish", t.progress.ID, "任务执行失败，请稍后重试。", err)
		}
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
	a.taskRunner.setPendingCorrelation(taskID, correlationFromContext(r.Context()))

	switch taskType {
	case TaskCheckin:
		if err := a.startCheckinTask(taskID, input.Params); err != nil {
			a.taskRunner.clearPendingCorrelation(taskID)
			switch {
			case errors.Is(err, errCheckinPreviewRequired):
				writeError(w, http.StatusBadRequest, "缺少签到预览 ID，请重新预览。")
			case errors.Is(err, errCheckinPreviewUnavailable):
				writeError(w, http.StatusConflict, "签到预览已过期或已使用，请重新预览。")
			case errors.Is(err, errCheckinRunBusy):
				writeError(w, http.StatusConflict, "已有签到任务正在运行，请等待完成后重新预览。")
			default:
				writePublicError(w, http.StatusInternalServerError, "签到任务启动失败，请稍后重试。", err)
			}
			return
		}
	case TaskTestKeys:
		a.startTestKeysTask(taskID, input.Params)
	case TaskRefreshBalances:
		a.startRefreshBalancesTask(taskID, input.Params)
	case TaskDetectSites:
		a.startDetectSitesTask(taskID, input.Params)
	case TaskChannelHealthProbe:
		a.startChannelHealthProbeTask(taskID, input.Params)
	default:
		a.taskRunner.clearPendingCorrelation(taskID)
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
	writeSSEEvent(w, flusher, task.snapshot())

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
			writeSSEEvent(w, flusher, p)
			if p.Status != TaskStatusRunning {
				return
			}
		case <-task.done:
			writeSSEEvent(w, flusher, task.snapshot())
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

// writeSSEEvent marshals payload as an SSE data event and flushes. Marshal
// failures are silently skipped (consistent with the prior inline pattern);
// a nil payload produces no output.
func writeSSEEvent(w http.ResponseWriter, flusher http.Flusher, payload interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}

// --- Task handlers ---

var errCheckinPreviewRequired = errors.New("checkin preview id required")

func (a *App) startCheckinTask(taskID string, params map[string]interface{}) error {
	previewID, _ := params["previewId"].(string)
	previewID = strings.TrimSpace(previewID)
	if previewID == "" {
		return errCheckinPreviewRequired
	}
	plan, err := a.checkinPlans.Claim(previewID)
	if err != nil {
		return err
	}
	return a.checkinTasks.StartCheckin(taskID, plan.RunAccounts)
}

func (a *App) startTestKeysTask(taskID string, params map[string]interface{}) {
	a.accountTasks.StartTestKeys(taskID, params)
}

func (a *App) startRefreshBalancesTask(taskID string, params map[string]interface{}) {
	a.checkinTasks.StartRefreshBalances(taskID, params)
}

func (a *App) startDetectSitesTask(taskID string, params map[string]interface{}) {
	a.siteTasks.StartDetectSites(taskID, params)
}

func (a *App) startChannelHealthProbeTask(taskID string, params map[string]interface{}) {
	a.siteTasks.StartChannelHealthProbe(taskID, params)
}
