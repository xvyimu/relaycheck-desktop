package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type correlationContextKey struct{}

type correlationFields struct {
	RequestID string
	TaskID    string
	AccountID string
	SiteID    string
}

var slowOperationThreshold = 250 * time.Millisecond

func withCorrelation(ctx context.Context, fields correlationFields) context.Context {
	current := correlationFromContext(ctx)
	if fields.RequestID != "" {
		current.RequestID = fields.RequestID
	}
	if fields.TaskID != "" {
		current.TaskID = fields.TaskID
	}
	if fields.AccountID != "" {
		current.AccountID = fields.AccountID
	}
	if fields.SiteID != "" {
		current.SiteID = fields.SiteID
	}
	return context.WithValue(ctx, correlationContextKey{}, current)
}

func correlationFromContext(ctx context.Context) correlationFields {
	if ctx == nil {
		return correlationFields{}
	}
	if fields, ok := ctx.Value(correlationContextKey{}).(correlationFields); ok {
		return fields
	}
	return correlationFields{RequestID: requestIDFromContext(ctx)}
}

func safeCorrelationID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 128 {
		return ""
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '-' || char == '_' || char == '.' || char == ':' {
			continue
		}
		return ""
	}
	return value
}

func correlationFromRequest(r *http.Request) correlationFields {
	fields := correlationFields{
		TaskID:    safeCorrelationID(firstNonEmpty(r.Header.Get("x-task-id"), r.URL.Query().Get("taskId"))),
		AccountID: safeCorrelationID(firstNonEmpty(r.Header.Get("x-account-id"), r.URL.Query().Get("accountId"))),
		SiteID:    safeCorrelationID(firstNonEmpty(r.Header.Get("x-site-id"), r.URL.Query().Get("siteId"), r.URL.Query().Get("upstreamSiteId"))),
	}
	segments := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(segments) >= 3 && segments[0] == "api" {
		resourceID := safeCorrelationID(segments[2])
		switch segments[1] {
		case "accounts":
			if !reservedCorrelationPathSegment(resourceID, "page", "summary", "search-sites", "search-index", "bulk-open-browser-login", "bulk-finish-browser-login", "bulk-password-login", "bulk-test-api-keys", "bulk-refresh-balances", "delete-unsupported-checkins", "import-legacy-config", "import-chrome-passwords") {
				fields.AccountID = resourceID
			}
		case "upstream-sites":
			if !reservedCorrelationPathSegment(resourceID, "bulk-detect") {
				fields.SiteID = resourceID
			}
		case "tasks":
			if !reservedCorrelationPathSegment(resourceID, "start", "dry-run") {
				fields.TaskID = resourceID
			}
		}
	}
	return fields
}

func reservedCorrelationPathSegment(value string, reserved ...string) bool {
	if value == "" {
		return true
	}
	for _, item := range reserved {
		if value == item {
			return true
		}
	}
	return false
}

func addCorrelationToEntry(entry map[string]interface{}, fields correlationFields) {
	if fields.RequestID != "" {
		entry["requestId"] = fields.RequestID
	}
	if fields.TaskID != "" {
		entry["taskId"] = fields.TaskID
	}
	if fields.AccountID != "" {
		entry["accountId"] = fields.AccountID
	}
	if fields.SiteID != "" {
		entry["siteId"] = fields.SiteID
	}
}

func writeStructuredLog(entry map[string]interface{}) {
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	accessLogMu.Lock()
	defer accessLogMu.Unlock()
	_, _ = accessLogWriter.Write(append(data, '\n'))
}

func logSlowOperation(ctx context.Context, kind string, operation string, started time.Time, operationErr error, extra map[string]interface{}) {
	duration := time.Since(started)
	if operationErr == nil && duration < slowOperationThreshold {
		return
	}
	entry := map[string]interface{}{
		"event":      "slow_operation",
		"kind":       safeCorrelationID(kind),
		"operation":  safeCorrelationID(operation),
		"durationMs": duration.Milliseconds(),
	}
	if operationErr != nil {
		entry["errorType"] = fmt.Sprintf("%T", operationErr)
	}
	for key, value := range extra {
		entry[key] = value
	}
	addCorrelationToEntry(entry, correlationFromContext(ctx))
	writeStructuredLog(entry)
}
