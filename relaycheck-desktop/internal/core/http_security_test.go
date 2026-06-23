package core

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAllowedHostAcceptsLoopbackHostsOnRuntimePort(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.SetRuntimeAddress("127.0.0.1", 3001)

	allowed := []string{
		"127.0.0.1:3001",
		"localhost:3001",
		"[::1]:3001",
		"127.0.0.1",
	}
	for _, host := range allowed {
		if !app.allowedHost(host) {
			t.Fatalf("expected host %q to be allowed", host)
		}
	}
}

func TestAllowedHostRejectsForeignHostsAndPorts(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.SetRuntimeAddress("127.0.0.1", 3001)

	rejected := []string{
		"evil.example:3001",
		"127.0.0.1:9999",
		"192.168.1.10:3001",
		"",
	}
	for _, host := range rejected {
		if app.allowedHost(host) {
			t.Fatalf("expected host %q to be rejected", host)
		}
	}
}

func TestSecureLocalHandlerRejectsBadHostAndSetsHeaders(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.SetRuntimeAddress("127.0.0.1", 3001)

	nextCalled := false
	handler := app.SecureLocalHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "http://evil.example:3001/api/system/status", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if nextCalled {
		t.Fatal("expected bad host to be rejected before next handler")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
	if rec.Header().Get("x-frame-options") != "DENY" {
		t.Fatalf("expected security headers, got %#v", rec.Header())
	}
	if rec.Header().Get("x-request-id") == "" {
		t.Fatal("expected request id header")
	}
}

func TestClampLimits(t *testing.T) {
	if got := clampBatchLimit(0, 30); got != 10 {
		t.Fatalf("expected fallback clamped to 10, got %d", got)
	}
	if got := clampBatchLimit(99, 5); got != 10 {
		t.Fatalf("expected oversized batch clamped to 10, got %d", got)
	}
	if got := clampBatchLimit(3, 5); got != 3 {
		t.Fatalf("expected explicit limit preserved, got %d", got)
	}
	if got := clampInt(999, 10, 100, 100); got != 100 {
		t.Fatalf("expected page size clamped to 100, got %d", got)
	}
	if got := clampInt(1, 10, 100, 100); got != 10 {
		t.Fatalf("expected page size clamped to 10, got %d", got)
	}
}

func TestSecureLocalHandlerRequestIDAndAccessLog(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.SetRuntimeAddress("127.0.0.1", 3001)

	var logs bytes.Buffer
	previousWriter := accessLogWriter
	accessLogWriter = &logs
	defer func() { accessLogWriter = previousWriter }()

	handler := app.SecureLocalHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := requestIDFromContext(r.Context()); got != "test-request-123" {
			t.Fatalf("expected request id in context, got %q", got)
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("ok"))
	}))
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:3001/api/example", strings.NewReader("password=secret"))
	req.RemoteAddr = "127.0.0.1:55123"
	req.Header.Set("x-request-id", "test-request-123")
	req.Header.Set("authorization", "Bearer should-not-be-logged")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status 202, got %d", rec.Code)
	}
	if rec.Header().Get("x-request-id") != "test-request-123" {
		t.Fatalf("expected propagated request id, got %q", rec.Header().Get("x-request-id"))
	}
	var entry map[string]interface{}
	if err := json.Unmarshal(bytes.TrimSpace(logs.Bytes()), &entry); err != nil {
		t.Fatalf("expected JSON access log, got %q: %v", logs.String(), err)
	}
	if entry["event"] != "http_request" || entry["requestId"] != "test-request-123" || entry["path"] != "/api/example" {
		t.Fatalf("unexpected access log: %#v", entry)
	}
	if logs.String() == "" || strings.Contains(logs.String(), "should-not-be-logged") || strings.Contains(logs.String(), "secret") {
		t.Fatalf("access log leaked sensitive content: %s", logs.String())
	}
}

func TestRequestIDRejectsUnsafeHeaderValue(t *testing.T) {
	if got := requestIDFromHeader("bad value with spaces"); got == "bad value with spaces" || got == "" {
		t.Fatalf("expected unsafe request id to be replaced, got %q", got)
	}
	if got := requestIDFromHeader("safe.id-123"); got != "safe.id-123" {
		t.Fatalf("expected safe request id to be preserved, got %q", got)
	}
}

func TestWriteErrorIncludesStableErrorClass(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, http.StatusBadRequest, "请求参数不完整。")

	var payload apiResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.OK {
		t.Fatal("expected error response")
	}
	if payload.Error != "请求参数不完整。" {
		t.Fatalf("unexpected error message: %q", payload.Error)
	}
	if payload.ErrorClass != "validation_error" {
		t.Fatalf("expected validation_error, got %q", payload.ErrorClass)
	}
	if got := errorClassForStatus(http.StatusInternalServerError); got != "server_error" {
		t.Fatalf("expected server_error, got %q", got)
	}
	if got := errorClassForStatus(http.StatusTooManyRequests); got != "rate_limited" {
		t.Fatalf("expected rate_limited, got %q", got)
	}
}
