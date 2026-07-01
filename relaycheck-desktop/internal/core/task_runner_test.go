package core

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

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
