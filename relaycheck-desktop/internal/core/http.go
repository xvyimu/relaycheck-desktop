package core

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type apiResponse struct {
	OK         bool        `json:"ok"`
	Data       interface{} `json:"data,omitempty"`
	Error      string      `json:"error,omitempty"`
	ErrorClass string      `json:"errorClass,omitempty"`
}

type requestIDContextKey struct{}

var (
	accessLogMu     sync.Mutex
	accessLogWriter io.Writer = os.Stderr
)

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("content-type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(apiResponse{OK: true, Data: data}); err != nil {
		// Most write failures are client disconnects; log so dropped connections
		// and encoding bugs are still visible without being silently swallowed.
		log.Printf("[http] writeJSON encode failed: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	if status >= http.StatusInternalServerError {
		requestID := strings.TrimSpace(w.Header().Get("x-request-id"))
		if requestID == "" {
			requestID = "unavailable"
		}
		log.Printf("[http] internal error requestId=%s status=%d: %s", requestID, status, message)
		message = "服务暂时不可用，请稍后重试。"
	}
	w.Header().Set("content-type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(apiResponse{OK: false, Error: message, ErrorClass: errorClassForStatus(status)}); err != nil {
		log.Printf("[http] writeError encode failed: %v", err)
	}
}

// writePublicError records request context without persisting a potentially
// sensitive upstream error, while preserving a stable client-facing contract.
func writePublicError(w http.ResponseWriter, status int, publicMessage string, err error) {
	requestID := strings.TrimSpace(w.Header().Get("x-request-id"))
	if requestID == "" {
		requestID = "unavailable"
	}
	log.Printf("[http] request error requestId=%s status=%d causeType=%T", requestID, status, err)
	writeError(w, status, publicMessage)
}

func errorClassForStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "validation_error"
	case http.StatusUnauthorized:
		return "auth_error"
	case http.StatusForbidden:
		return "permission_error"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusMethodNotAllowed:
		return "method_not_allowed"
	case http.StatusConflict:
		return "conflict"
	case http.StatusTooManyRequests:
		return "rate_limited"
	}
	if status >= 500 {
		return "server_error"
	}
	if status >= 400 {
		return "request_error"
	}
	return "unknown_error"
}

const defaultJSONBodyLimit = 8 << 20 // 8 MiB

func decodeJSON(r *http.Request, out interface{}) error {
	return decodeJSONBody(r, out, false)
}

// decodeOptionalJSON keeps the established empty-body defaults for bulk
// endpoints, but never turns malformed non-empty JSON into a zero-value
// request that could trigger an unintended write or outbound operation.
func decodeOptionalJSON(r *http.Request, out interface{}) error {
	return decodeJSONBody(r, out, true)
}

func decodeJSONBody(r *http.Request, out interface{}, allowEmpty bool) error {
	if r.Body == nil {
		if allowEmpty {
			return nil
		}
		return io.EOF
	}
	defer r.Body.Close()
	limited := http.MaxBytesReader(nil, r.Body, defaultJSONBodyLimit)
	decoder := json.NewDecoder(limited)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		if allowEmpty && errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	var extra struct{}
	if err := decoder.Decode(&extra); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return err
	}
	return errors.New("multiple JSON values")
}

func method(w http.ResponseWriter, r *http.Request, expected string) bool {
	if r.Method != expected {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return false
	}
	return true
}

func clampBatchLimit(value int, fallback int) int {
	if fallback < 1 {
		fallback = 1
	}
	if fallback > 10 {
		fallback = 10
	}
	if value <= 0 {
		return fallback
	}
	if value > 10 {
		return 10
	}
	return value
}

func clampListLimit(value int, fallback int, maximum int) int {
	if maximum < 1 {
		maximum = 1
	}
	if fallback < 1 {
		fallback = 1
	}
	if fallback > maximum {
		fallback = maximum
	}
	if value <= 0 {
		return fallback
	}
	if value > maximum {
		return maximum
	}
	return value
}

func clampInt(value int, min int, max int, fallback int) int {
	if min > max {
		min, max = max, min
	}
	if fallback < min || fallback > max {
		fallback = min
	}
	if value <= 0 {
		return fallback
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// SecureLocalHandler wraps an http.Handler with local-only access enforcement.
func (a *App) SecureLocalHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		requestID := requestIDFromHeader(r.Header.Get("x-request-id"))
		w.Header().Set("x-request-id", requestID)
		ctx := context.WithValue(r.Context(), requestIDContextKey{}, requestID)
		fields := correlationFromRequest(r)
		fields.RequestID = requestID
		r = r.WithContext(withCorrelation(ctx, fields))
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			logHTTPRequest(r, requestID, recorder.status, time.Since(started))
		}()

		setSecurityHeaders(recorder)
		if !a.allowedHost(r.Host) {
			writeError(recorder, http.StatusForbidden, "host not allowed")
			return
		}
		next.ServeHTTP(recorder, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(data []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(data)
}

func requestIDFromHeader(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 64 {
		return newID()
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '-' || char == '_' || char == '.' {
			continue
		}
		return newID()
	}
	return value
}

func requestIDFromContext(ctx context.Context) string {
	if value, ok := ctx.Value(requestIDContextKey{}).(string); ok {
		return value
	}
	return ""
}

func logHTTPRequest(r *http.Request, requestID string, status int, duration time.Duration) {
	entry := map[string]interface{}{
		"event":       "http_request",
		"requestId":   requestID,
		"method":      r.Method,
		"path":        r.URL.Path,
		"status":      status,
		"statusClass": strconv.Itoa(status/100) + "xx",
		"errorClass":  errorClassForStatus(status),
		"durationMs":  duration.Milliseconds(),
		"remoteAddr":  safeRemoteAddr(r.RemoteAddr),
	}
	addCorrelationToEntry(entry, correlationFromContext(r.Context()))
	writeStructuredLog(entry)
}

func safeRemoteAddr(value string) string {
	host, _, err := net.SplitHostPort(value)
	if err == nil {
		return host
	}
	return strings.TrimSpace(value)
}

func setSecurityHeaders(w http.ResponseWriter) {
	header := w.Header()
	header.Set("x-content-type-options", "nosniff")
	header.Set("x-frame-options", "DENY")
	header.Set("referrer-policy", "no-referrer")
	header.Set("content-security-policy", "default-src 'self'; script-src 'self'; style-src 'self'; style-src-attr 'none'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'")
}

func (a *App) allowedHost(rawHost string) bool {
	host, port, err := net.SplitHostPort(rawHost)
	if err != nil {
		host = rawHost
		port = ""
	}
	host = strings.Trim(strings.TrimSpace(strings.ToLower(host)), "[]")
	if host == "" {
		return false
	}
	if port != "" {
		expectedPort := a.runtimePort()
		parsedPort, err := strconv.Atoi(port)
		if err != nil || parsedPort != expectedPort {
			return false
		}
	}
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}
	a.mu.RLock()
	bind := strings.ToLower(a.bind)
	a.mu.RUnlock()
	return host == bind
}

func (a *App) runtimePort() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.port
}

func queryInt(r *http.Request, name string, fallback int) int {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return n
}
