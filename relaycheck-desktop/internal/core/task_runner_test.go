package core

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func taskStartRequest(previewID string) *http.Request {
	body := `{"type":"checkin","params":{"previewId":"` + previewID + `"}}`
	return httptest.NewRequest(http.MethodPost, "/api/tasks/start", strings.NewReader(body))
}

func decodeTaskStartID(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var response struct {
		Data struct {
			TaskID string `json:"taskId"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode task start response: %v", err)
	}
	return response.Data.TaskID
}

func TestHandleTaskStartCheckinRequiresPreviewID(t *testing.T) {
	app := newTestApp(t)
	req := httptest.NewRequest(http.MethodPost, "/api/tasks/start", strings.NewReader(`{"type":"checkin","params":{}}`))
	rec := httptest.NewRecorder()

	app.handleTaskStart(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing previewId status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleTaskStartClaimsPreviewOnceAndRegistersBeforeResponse(t *testing.T) {
	app := newTestApp(t)
	insertCheckinPlanFixture(t, app, checkinPlanFixture{
		id: "claim-once", name: "Claim once", supportsCheckin: 1, apiKey: "saved-key", updatedAt: "2026-07-18T01:00:00Z",
	})
	preview, err := app.checkinPlans.BuildAllDue(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	const callers = 2
	recorders := make([]*httptest.ResponseRecorder, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		recorders[i] = httptest.NewRecorder()
		wg.Add(1)
		go func(rec *httptest.ResponseRecorder) {
			defer wg.Done()
			app.handleTaskStart(rec, taskStartRequest(preview.PreviewID))
		}(recorders[i])
	}
	wg.Wait()

	statusCounts := map[int]int{}
	var success *httptest.ResponseRecorder
	for _, rec := range recorders {
		statusCounts[rec.Code]++
		if rec.Code == http.StatusOK {
			success = rec
		}
	}
	if statusCounts[http.StatusOK] != 1 || statusCounts[http.StatusConflict] != 1 {
		t.Fatalf("same preview must start once: statuses=%v bodies=%q / %q", statusCounts, recorders[0].Body.String(), recorders[1].Body.String())
	}
	taskID := decodeTaskStartID(t, success)
	if taskID == "" || app.taskRunner.get(taskID) == nil {
		t.Fatalf("task must be registered before start response: taskId=%q", taskID)
	}

	streamRec := httptest.NewRecorder()
	app.handleTaskStream(streamRec, httptest.NewRequest(http.MethodGet, "/api/tasks/"+taskID+"/stream", nil))
	if streamRec.Code == http.StatusNotFound {
		t.Fatalf("immediate task stream returned 404: %s", streamRec.Body.String())
	}
}

func TestHandleTaskStartConsumesPreviewWhenGlobalCheckinRunIsBusy(t *testing.T) {
	app := newTestApp(t)
	insertCheckinPlanFixture(t, app, checkinPlanFixture{
		id: "busy-preview", name: "Busy preview", supportsCheckin: 1, apiKey: "saved-key", updatedAt: "2026-07-18T01:00:00Z",
	})
	preview, err := app.checkinPlans.BuildAllDue(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !app.checkinRun.begin("scheduler", 1) {
		t.Fatal("failed to arrange busy checkin run")
	}
	defer app.checkinRun.finish()

	first := httptest.NewRecorder()
	app.handleTaskStart(first, taskStartRequest(preview.PreviewID))
	if first.Code != http.StatusConflict {
		t.Fatalf("busy start status = %d, want 409: %s", first.Code, first.Body.String())
	}
	second := httptest.NewRecorder()
	app.handleTaskStart(second, taskStartRequest(preview.PreviewID))
	if second.Code != http.StatusConflict {
		t.Fatalf("busy claim must still consume token, got %d: %s", second.Code, second.Body.String())
	}
}

// TestHandleTaskStream_SSESubscriberCap verifies that handleTaskStream
// rejects new SSE connections once the per-process subscriber cap is
// reached, returning 503 instead of accepting the connection.
func TestHandleTaskStream_SSESubscriberCap(t *testing.T) {
	app := newTestApp(t)

	// Create a task so the lookup would succeed if not for the cap.
	task, _ := app.taskRunner.start("cap-test", TaskCheckin, 1)
	defer app.taskRunner.cancelTask("cap-test")
	defer task.finish(nil)

	// Simulate the cap being saturated by real SSE connections.
	app.taskRunner.sseSubscribers.Store(maxSSESubscribers)
	defer app.taskRunner.sseSubscribers.Store(0)

	req := httptest.NewRequest(http.MethodGet, "/api/tasks/cap-test/stream", nil)
	rr := httptest.NewRecorder()
	app.handleTaskStream(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when subscriber cap reached, got %d", rr.Code)
	}
}

// TestHandleTaskStream_SSESubscriberCap_ReleasesOnMissingTask verifies the
// counter is released when the handler exits early (task not found), so a
// rejected lookup doesn't leak a subscriber slot.
func TestHandleTaskStream_SSESubscriberCap_ReleasesOnMissingTask(t *testing.T) {
	app := newTestApp(t)

	app.taskRunner.sseSubscribers.Store(0)

	req := httptest.NewRequest(http.MethodGet, "/api/tasks/nonexistent/stream", nil)
	rr := httptest.NewRecorder()
	app.handleTaskStream(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing task, got %d", rr.Code)
	}
	if got := app.taskRunner.sseSubscribers.Load(); got != 0 {
		t.Fatalf("subscriber counter leaked: got %d, want 0", got)
	}
}

// TestHandleTaskStream_SSESubscriberCap_AcceptsBelowLimitAndReleases verifies
// that a connection below the cap is accepted and the counter is released
// after the stream completes. To avoid blocking the test on the SSE select
// loop, the task is finished before the handler is called, so the handler
// immediately sends the final snapshot and returns via the task.done case.
func TestHandleTaskStream_SSESubscriberCap_AcceptsBelowLimitAndReleases(t *testing.T) {
	app := newTestApp(t)

	task, _ := app.taskRunner.start("below-cap-test", TaskCheckin, 1)
	// Finish the task BEFORE calling the handler so the select loop hits
	// case <-task.done immediately and returns without blocking.
	task.finish(nil)
	defer app.taskRunner.cancelTask("below-cap-test")

	app.taskRunner.sseSubscribers.Store(maxSSESubscribers - 1)

	req := httptest.NewRequest(http.MethodGet, "/api/tasks/below-cap-test/stream", nil)
	rr := httptest.NewRecorder()
	app.handleTaskStream(rr, req)

	if rr.Code == http.StatusServiceUnavailable {
		t.Fatalf("connection below cap should not be rejected with 503, got %d", rr.Code)
	}
	// Counter must have been released back to its pre-request value.
	if got := app.taskRunner.sseSubscribers.Load(); got != maxSSESubscribers-1 {
		t.Fatalf("subscriber counter leaked: got %d, want %d", got, maxSSESubscribers-1)
	}
}

func TestRunningTaskFinishDoesNotExposeInternalCause(t *testing.T) {
	runner := newTaskRunner()
	task, _ := runner.start("secret-error", TaskCheckin, 1)
	task.finish(errors.New(`open C:\secret\relaycheck.db: token=TOP_SECRET`))

	snapshot := task.snapshot()
	if snapshot.Error != "任务执行失败，请稍后重试。" {
		t.Fatalf("unexpected public task error: %q", snapshot.Error)
	}
	for _, forbidden := range []string{"C:\\secret", "relaycheck.db", "TOP_SECRET"} {
		if strings.Contains(snapshot.Error, forbidden) {
			t.Fatalf("task error leaked %q: %q", forbidden, snapshot.Error)
		}
	}
}

func TestRunningTaskFinishPreservesCancellationMeaning(t *testing.T) {
	runner := newTaskRunner()
	task, _ := runner.start("cancelled", TaskCheckin, 1)
	task.finish(context.Canceled)

	if got := task.snapshot().Error; got != "任务已取消。" {
		t.Fatalf("cancelled task error = %q", got)
	}
}

func TestTaskRunnerPropagatesRequestCorrelationIntoTaskContext(t *testing.T) {
	runner := newTaskRunner()
	runner.setPendingCorrelation("task-1", correlationFields{
		RequestID: "request-1",
		AccountID: "account-1",
		SiteID:    "site-1",
	})
	task, ctx := runner.start("task-1", TaskTestKeys, 1)
	defer task.cancel()

	fields := correlationFromContext(ctx)
	if fields.RequestID != "request-1" || fields.TaskID != "task-1" || fields.AccountID != "account-1" || fields.SiteID != "site-1" {
		t.Fatalf("unexpected task correlation: %#v", fields)
	}
}
