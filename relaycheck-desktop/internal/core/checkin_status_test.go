package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestComputeCheckinScheduleStatusUsesNextWindow(t *testing.T) {
	location := time.FixedZone("CST", 8*60*60)
	current := time.Date(2026, 6, 19, 7, 30, 0, 0, location)
	status := computeCheckinScheduleStatus(true, "08:00", []int{0, 120}, current)
	if status.NextRunInSeconds != int64(30*time.Minute/time.Second) {
		t.Fatalf("expected 30 minute countdown, got %d", status.NextRunInSeconds)
	}
	if status.NextWindowInSeconds != int64(150*time.Minute/time.Second) {
		t.Fatalf("expected 150 minute window countdown, got %d", status.NextWindowInSeconds)
	}

	insideWindow := time.Date(2026, 6, 19, 8, 30, 0, 0, location)
	status = computeCheckinScheduleStatus(true, "08:00", []int{0, 120}, insideWindow)
	if status.NextRunInSeconds != 0 {
		t.Fatalf("expected active window countdown to be 0, got %d", status.NextRunInSeconds)
	}
	if status.NextWindowInSeconds != int64(90*time.Minute/time.Second) {
		t.Fatalf("expected 90 minutes left in window, got %d", status.NextWindowInSeconds)
	}
}

func TestBuildCheckinStatusIncludesDueAccountsAndRunProgress(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()
	var err error

	siteID := newID()
	accountID := newID()
	_, err = app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, supports_checkin, created_at, updated_at)
		VALUES (?, 'Relay', 'https://relay.example', 'newapi', 'healthy', 1, ?, ?)
	`, siteID, now(), now())
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.db.Exec(`
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, login_status, created_at, updated_at)
		VALUES (?, ?, 'Account A', 'cookie', 'valid', ?, ?)
	`, accountID, siteID, now(), now())
	if err != nil {
		t.Fatal(err)
	}

	if !app.beginCheckinRun("manual", 1) {
		t.Fatal("expected run to start")
	}
	app.updateCheckinRunCurrent(accountID, "Account A", "Relay", "正在签到...")
	app.recordCheckinRunResult("success", "签到成功")

	status, err := app.buildCheckinStatus(context.Background(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !status.Running {
		t.Fatal("expected status to show running")
	}
	if status.CurrentAccount != "Account A" || status.CurrentSite != "Relay" {
		t.Fatalf("unexpected current target: %#v", status)
	}
	if status.ProcessedAccounts != 1 || status.SuccessCount != 1 {
		t.Fatalf("unexpected progress counts: %#v", status)
	}
	if status.Today.DueAccounts != 1 {
		t.Fatalf("expected one due account, got %d", status.Today.DueAccounts)
	}
}

func TestRunAccountCheckinRetriesTemporaryFailures(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/checkin" {
			http.NotFound(w, r)
			return
		}
		call := calls.Add(1)
		if call <= 2 {
			http.Error(w, `{"message":"temporary upstream error"}`, http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"checked in"}`))
	}))
	defer server.Close()

	app := newTestApp(t)
	defer app.Close()
	app.client = server.Client()
	app.allowLocalOutbound = true

	siteID := newID()
	accountID := newID()
	cookieEncrypted, err := app.encryptText("session=valid")
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, supports_checkin, checkin_config_json, created_at, updated_at)
		VALUES (?, 'Retry Relay', ?, 'newapi', 'healthy', 1, '{"method":"POST","path":"/checkin"}', ?, ?)
	`, siteID, server.URL, now(), now())
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.db.Exec(`
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, cookie_encrypted, login_status, created_at, updated_at)
		VALUES (?, ?, 'Retry Account', 'cookie', ?, 'valid', ?, ?)
	`, accountID, siteID, cookieEncrypted, now(), now())
	if err != nil {
		t.Fatal(err)
	}

	result, err := app.runAccountCheckin(context.Background(), accountID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "success" {
		t.Fatalf("expected success after retries, got %+v", result)
	}
	if result.RetryCount != 2 {
		t.Fatalf("expected two retries, got %+v", result)
	}
	if !strings.Contains(result.Message, "已自动重试 2 次") {
		t.Fatalf("expected retry annotation, got %q", result.Message)
	}
	if calls.Load() != 3 {
		t.Fatalf("expected three checkin attempts, got %d", calls.Load())
	}

	var storedMessage string
	if err := app.db.QueryRow(`SELECT message FROM checkin_logs WHERE account_id=?`, accountID).Scan(&storedMessage); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(storedMessage, "已自动重试 2 次") {
		t.Fatalf("expected persisted retry annotation, got %q", storedMessage)
	}
}

func TestShouldRetryCheckinAttemptOnlyRetriesTemporaryFailures(t *testing.T) {
	if !shouldRetryCheckinAttempt(0, context.DeadlineExceeded) {
		t.Fatal("expected request errors to be retried")
	}
	for _, status := range []int{http.StatusRequestTimeout, http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway} {
		if !shouldRetryCheckinAttempt(status, nil) {
			t.Fatalf("expected HTTP %d to be retried", status)
		}
	}
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusMethodNotAllowed, http.StatusBadRequest} {
		if shouldRetryCheckinAttempt(status, nil) {
			t.Fatalf("did not expect HTTP %d to be retried", status)
		}
	}
}

func TestCheckinSiteLimiterComputesPerSiteDelay(t *testing.T) {
	limiter := newCheckinSiteLimiter(checkinScheduleConfig{SiteMinIntervalSeconds: 2})
	startedAt := time.Date(2026, 6, 20, 8, 0, 0, 0, time.UTC)
	limiter.lastStarted["site-a"] = startedAt

	if delay := limiter.delayFor("site-a", startedAt.Add(500*time.Millisecond)); delay != 1500*time.Millisecond {
		t.Fatalf("expected 1.5s delay for same site, got %s", delay)
	}
	if delay := limiter.delayFor("site-b", startedAt.Add(500*time.Millisecond)); delay != 0 {
		t.Fatalf("expected no delay for different site, got %s", delay)
	}
	if delay := limiter.delayFor("site-a", startedAt.Add(3*time.Second)); delay != 0 {
		t.Fatalf("expected no delay after interval elapsed, got %s", delay)
	}
}

func TestLoadCheckinScheduleConfigClampsSiteMinInterval(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()
	var err error

	config := app.loadCheckinScheduleConfig(context.Background())
	if config.SiteMinIntervalSeconds != 2 {
		t.Fatalf("expected default site min interval 2s, got %+v", config)
	}

	_, err = app.db.Exec(`UPDATE system_settings SET value_json=? WHERE key='checkin.schedule'`, `{"enabled":true,"time":"08:00","siteMinIntervalSeconds":999}`)
	if err != nil {
		t.Fatal(err)
	}
	config = app.loadCheckinScheduleConfig(context.Background())
	if config.SiteMinIntervalSeconds != 60 {
		t.Fatalf("expected clamped site min interval 60s, got %+v", config)
	}
}

func TestClassifyCheckinResponse_401ReturnsAuthExpired(t *testing.T) {
	result := classifyCheckinResponse(http.StatusUnauthorized, `{"message":"invalid token"}`)
	if result.Status != "auth_expired" {
		t.Fatalf("expected auth_expired, got %q", result.Status)
	}
	if result.Message == "" {
		t.Fatal("expected non-empty message")
	}
}

func TestClassifyCheckinResponse_403ReturnsAuthExpired(t *testing.T) {
	result := classifyCheckinResponse(http.StatusForbidden, `not allowed`)
	if result.Status != "auth_expired" {
		t.Fatalf("expected auth_expired, got %q", result.Status)
	}
}

func TestClassifyCheckinResponse_CheckinDisabledReturnsUnsupported(t *testing.T) {
	body := `{"error":"签到功能未启用"}`
	result := classifyCheckinResponse(http.StatusOK, body)
	if result.Status != "unsupported" {
		t.Fatalf("expected unsupported, got %q", result.Status)
	}
}

func TestClassifyCheckinResponse_CheckinDisabledEnglishReturnsUnsupported(t *testing.T) {
	body := `{"error":"checkin disabled for this account"}`
	result := classifyCheckinResponse(http.StatusOK, body)
	if result.Status != "unsupported" {
		t.Fatalf("expected unsupported, got %q", result.Status)
	}
}

func TestClassifyCheckinResponse_404ReturnsFailed(t *testing.T) {
	result := classifyCheckinResponse(http.StatusNotFound, `not found`)
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %q", result.Status)
	}
}

func TestClassifyCheckinResponse_503ReturnsFailed(t *testing.T) {
	result := classifyCheckinResponse(http.StatusServiceUnavailable, `service unavailable`)
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %q", result.Status)
	}
}

func TestClassifyCheckinResponse_AlreadyKeywordReturnsAlreadyChecked(t *testing.T) {
	result := classifyCheckinResponse(http.StatusOK, `{"message":"already checked in"}`)
	if result.Status != "already_checked" {
		t.Fatalf("expected already_checked, got %q", result.Status)
	}
	if result.Message != "already checked in" {
		t.Fatalf("expected message from JSON body, got %q", result.Message)
	}
}

func TestClassifyCheckinResponse_TodayKeywordReturnsAlreadyChecked(t *testing.T) {
	result := classifyCheckinResponse(http.StatusOK, `checked in today`)
	if result.Status != "already_checked" {
		t.Fatalf("expected already_checked, got %q", result.Status)
	}
}

func TestClassifyCheckinResponse_ChineseYiqianDaoReturnsAlreadyChecked(t *testing.T) {
	result := classifyCheckinResponse(http.StatusOK, `已签到`)
	if result.Status != "already_checked" {
		t.Fatalf("expected already_checked, got %q", result.Status)
	}
}

func TestClassifyCheckinResponse_ChineseZhongfuReturnsAlreadyChecked(t *testing.T) {
	result := classifyCheckinResponse(http.StatusOK, `重复签到`)
	if result.Status != "already_checked" {
		t.Fatalf("expected already_checked, got %q", result.Status)
	}
}

func TestClassifyCheckinResponse_LoginPageReturnsAuthExpired(t *testing.T) {
	body := `<html><form action="/api/user/login">login</form></html>`
	result := classifyCheckinResponse(http.StatusOK, body)
	if result.Status != "auth_expired" {
		t.Fatalf("expected auth_expired, got %q", result.Status)
	}
	if result.Message == "" {
		t.Fatal("expected non-empty message")
	}
}

func TestClassifyCheckinResponse_JSONSuccessFalseReturnsFailed(t *testing.T) {
	result := classifyCheckinResponse(http.StatusOK, `{"success":false,"message":"签到失败"}`)
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %q", result.Status)
	}
	if result.Message != "签到失败" {
		t.Fatalf("expected message from JSON, got %q", result.Message)
	}
}

func TestClassifyCheckinResponse_JSONOkFalseReturnsFailed(t *testing.T) {
	result := classifyCheckinResponse(http.StatusOK, `{"ok":false}`)
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %q", result.Status)
	}
}

func TestClassifyCheckinResponse_NonMatchingReturnsSuccess(t *testing.T) {
	result := classifyCheckinResponse(http.StatusOK, `{"message":"签到成功","reward":"100"}`)
	if result.Status != "success" {
		t.Fatalf("expected success, got %q", result.Status)
	}
	if result.Message != "签到成功 奖励：100.00" {
		t.Fatalf("expected message with reward, got %q", result.Message)
	}
	if result.Reward != "100.00" {
		t.Fatalf("expected reward 100.00, got %q", result.Reward)
	}
}

func TestClassifyCheckinResponse_EmptyBodyReturnsSuccess(t *testing.T) {
	result := classifyCheckinResponse(http.StatusOK, ``)
	if result.Status != "success" {
		t.Fatalf("expected success for empty body, got %q", result.Status)
	}
	if result.Message == "" {
		t.Fatal("expected fallback message for empty response")
	}
}

func TestExtractMessage_JSONWithMessageField(t *testing.T) {
	if got := extractMessage(`{"message":"hello"}`); got != "hello" {
		t.Fatalf("expected hello, got %q", got)
	}
}

func TestExtractMessage_JSONWithMsgFallback(t *testing.T) {
	if got := extractMessage(`{"msg":"world"}`); got != "world" {
		t.Fatalf("expected world, got %q", got)
	}
}

func TestExtractMessage_JSONWithErrorField(t *testing.T) {
	if got := extractMessage(`{"error":"something went wrong"}`); got != "something went wrong" {
		t.Fatalf("expected something went wrong, got %q", got)
	}
}

func TestExtractMessage_NonJSONReturnsEmpty(t *testing.T) {
	if got := extractMessage(`not json`); got != "" {
		t.Fatalf("expected empty for non-JSON, got %q", got)
	}
}

func TestExtractMessage_EmptyBody(t *testing.T) {
	if got := extractMessage(``); got != "" {
		t.Fatalf("expected empty for empty body, got %q", got)
	}
}

func TestFirstNonEmpty_ReturnsFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "second", "third"); got != "second" {
		t.Fatalf("expected second, got %q", got)
	}
}

func TestFirstNonEmpty_AllEmptyReturnsEmpty(t *testing.T) {
	if got := firstNonEmpty("", "", ""); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestFirstNonEmpty_TrimsWhitespace(t *testing.T) {
	if got := firstNonEmpty("  ", "a"); got != "a" {
		t.Fatalf("expected a, got %q", got)
	}
}
