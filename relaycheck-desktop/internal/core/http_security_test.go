package core

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAllowedHostAcceptsLoopbackHostsOnRuntimePort(t *testing.T) {
	app := newTestApp(t)
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
	app := newTestApp(t)
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
	app := newTestApp(t)
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
	app := newTestApp(t)
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

func TestRequireSessionRejectsCrossOriginStateChangingRequests(t *testing.T) {
	app := newTestApp(t)
	app.SetRuntimeAddress("127.0.0.1", 3001)

	called := false
	handler := app.requireSession(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})

	cases := []struct {
		name       string
		method     string
		origin     string
		wantCalled bool
		wantStatus int
	}{
		{name: "POST without Origin (non-browser client)", method: http.MethodPost, origin: "", wantCalled: true, wantStatus: http.StatusNoContent},
		{name: "POST with same-origin Origin", method: http.MethodPost, origin: "http://127.0.0.1:3001", wantCalled: true, wantStatus: http.StatusNoContent},
		{name: "POST with localhost same-origin Origin", method: http.MethodPost, origin: "http://localhost:3001", wantCalled: true, wantStatus: http.StatusNoContent},
		{name: "POST with cross-origin Origin", method: http.MethodPost, origin: "http://evil.example:3001", wantCalled: false, wantStatus: http.StatusForbidden},
		{name: "POST with cross-port Origin", method: http.MethodPost, origin: "http://127.0.0.1:9999", wantCalled: false, wantStatus: http.StatusForbidden},
		{name: "POST with https Origin", method: http.MethodPost, origin: "https://127.0.0.1:3001", wantCalled: false, wantStatus: http.StatusForbidden},
		{name: "POST with malformed Origin", method: http.MethodPost, origin: "://bad", wantCalled: false, wantStatus: http.StatusForbidden},
		{name: "GET with cross-origin Origin (allowed; not state-changing)", method: http.MethodGet, origin: "http://evil.example:3001", wantCalled: true, wantStatus: http.StatusNoContent},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called = false
			req := httptest.NewRequest(tc.method, "http://127.0.0.1:3001/api/example", nil)
			// httptest defaults RemoteAddr to 192.0.2.1; force loopback so BE-3
			// remote check does not mask Origin assertions.
			req.RemoteAddr = "127.0.0.1:55123"
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if called != tc.wantCalled {
				t.Fatalf("handler invoked=%v, want %v (status %d)", called, tc.wantCalled, rec.Code)
			}
			if rec.Code != tc.wantStatus {
				t.Fatalf("status=%d, want %d", rec.Code, tc.wantStatus)
			}
		})
	}
}

func TestRequireSessionRejectsInvalidLocalTokenWhenEnabled(t *testing.T) {
	app := newTestApp(t)
	app.SetLocalToken("0123456789abcdef")

	called := false
	handler := app.requireSession(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})

	cases := []struct {
		name       string
		cookie     string
		wantCalled bool
		wantStatus int
	}{
		{name: "missing token", cookie: "", wantCalled: false, wantStatus: http.StatusUnauthorized},
		{name: "wrong token", cookie: "bad", wantCalled: false, wantStatus: http.StatusUnauthorized},
		{name: "matching token", cookie: "0123456789abcdef", wantCalled: true, wantStatus: http.StatusNoContent},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called = false
			req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:3001/api/example", nil)
			if tc.cookie != "" {
				req.AddCookie(&http.Cookie{Name: sessionTokenCookie, Value: tc.cookie})
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if called != tc.wantCalled {
				t.Fatalf("handler invoked=%v, want %v (status %d)", called, tc.wantCalled, rec.Code)
			}
			if rec.Code != tc.wantStatus {
				t.Fatalf("status=%d, want %d", rec.Code, tc.wantStatus)
			}
		})
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

func TestRequireSessionRejectsNonLoopbackRemoteOnWrites(t *testing.T) {
	app := newTestApp(t)
	app.SetRuntimeAddress("127.0.0.1", 3001)

	called := false
	handler := app.requireSession(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})

	cases := []struct {
		name       string
		method     string
		remote     string
		wantCalled bool
		wantStatus int
	}{
		{name: "POST loopback remote", method: http.MethodPost, remote: "127.0.0.1:55123", wantCalled: true, wantStatus: http.StatusNoContent},
		{name: "POST empty remote (in-process)", method: http.MethodPost, remote: "", wantCalled: true, wantStatus: http.StatusNoContent},
		{name: "POST non-loopback remote", method: http.MethodPost, remote: "192.168.1.10:55123", wantCalled: false, wantStatus: http.StatusForbidden},
		{name: "GET non-loopback remote allowed", method: http.MethodGet, remote: "192.168.1.10:55123", wantCalled: true, wantStatus: http.StatusNoContent},
		{name: "POST IPv6 loopback", method: http.MethodPost, remote: "[::1]:55123", wantCalled: true, wantStatus: http.StatusNoContent},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called = false
			req := httptest.NewRequest(tc.method, "http://127.0.0.1:3001/api/example", nil)
			req.RemoteAddr = tc.remote
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if called != tc.wantCalled {
				t.Fatalf("handler invoked=%v, want %v (status %d)", called, tc.wantCalled, rec.Code)
			}
			if rec.Code != tc.wantStatus {
				t.Fatalf("status=%d, want %d", rec.Code, tc.wantStatus)
			}
		})
	}
}

func TestRequireLoopbackRemoteHelper(t *testing.T) {
	if err := requireLoopbackRemote(""); err != nil {
		t.Fatalf("empty remote should pass: %v", err)
	}
	if err := requireLoopbackRemote("127.0.0.1:1"); err != nil {
		t.Fatalf("loopback should pass: %v", err)
	}
	if err := requireLoopbackRemote("[::1]:1"); err != nil {
		t.Fatalf("ipv6 loopback should pass: %v", err)
	}
	if err := requireLoopbackRemote("10.0.0.1:1"); err == nil {
		t.Fatal("non-loopback should fail")
	}
	if err := requireLoopbackRemote("not-an-ip"); err == nil {
		t.Fatal("invalid host should fail")
	}
}

func TestClampIntAndErrorClassBranches(t *testing.T) {
	if got := clampInt(5, 10, 1, 3); got != 5 {
		// min/max swap path (min=1,max=10); value in range
		t.Fatalf("swap min/max expected 5, got %d", got)
	}
	if got := clampInt(0, 10, 1, 0); got != 1 {
		// fallback 0 out of range after swap → min
		t.Fatalf("fallback clamp after swap expected 1, got %d", got)
	}
	if got := clampInt(0, 2, 8, 4); got != 4 {
		t.Fatalf("zero uses fallback, got %d", got)
	}
	if got := clampInt(1, 2, 8, 4); got != 2 {
		t.Fatalf("below min clamps to min, got %d", got)
	}
	if got := clampInt(99, 2, 8, 4); got != 8 {
		t.Fatalf("above max clamps to max, got %d", got)
	}
	if got := clampInt(5, 2, 8, 4); got != 5 {
		t.Fatalf("in range preserved, got %d", got)
	}
	if got := clampBatchLimit(-1, 0); got != 1 {
		t.Fatalf("fallback floor, got %d", got)
	}
	if got := clampBatchLimit(1, 20); got != 1 {
		t.Fatalf("valid value under max, got %d", got)
	}

	cases := map[int]string{
		http.StatusUnauthorized:   "auth_error",
		http.StatusForbidden:      "permission_error",
		http.StatusNotFound:       "not_found",
		http.StatusMethodNotAllowed: "method_not_allowed",
		http.StatusConflict:       "conflict",
		http.StatusTeapot:         "request_error",
		http.StatusOK:             "unknown_error",
	}
	for status, want := range cases {
		if got := errorClassForStatus(status); got != want {
			t.Fatalf("status %d: got %q want %q", status, got, want)
		}
	}
}

func TestAnalyticsDaysBounds(t *testing.T) {
	if got := analyticsDaysBounds(""); got != 30 {
		t.Fatalf("empty -> 30, got %d", got)
	}
	if got := analyticsDaysBounds("abc"); got != 30 {
		t.Fatalf("invalid -> 30, got %d", got)
	}
	if got := analyticsDaysBounds("0"); got != 30 {
		t.Fatalf("zero -> 30, got %d", got)
	}
	if got := analyticsDaysBounds("7"); got != 7 {
		t.Fatalf("7 preserved, got %d", got)
	}
	if got := analyticsDaysBounds("999"); got != 365 {
		t.Fatalf("over max -> 365, got %d", got)
	}
}

func TestSafeRemoteAddrAndRequestIDContext(t *testing.T) {
	if got := safeRemoteAddr("127.0.0.1:9999"); got != "127.0.0.1" {
		t.Fatalf("split host, got %q", got)
	}
	if got := safeRemoteAddr("bare-host"); got != "bare-host" {
		t.Fatalf("bare host, got %q", got)
	}
	if got := requestIDFromContext(context.Background()); got != "" {
		t.Fatalf("empty context id, got %q", got)
	}
}

func TestAppDataDirAndPortConflictAccessors(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()
	if app.DataDir() == "" {
		t.Fatal("DataDir should be non-empty")
	}
	app.SetPortConflict(3001, true)
	// runtimePort / conflict are private; just ensure SetPortConflict does not panic
	app.SetRuntimeAddress("127.0.0.1", 3002)
	if !app.allowedHost("127.0.0.1:3002") {
		t.Fatal("expected new runtime port allowed")
	}
}

