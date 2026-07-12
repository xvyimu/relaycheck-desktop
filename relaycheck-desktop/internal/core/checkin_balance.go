package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"relaycheck-desktop/internal/capabilities"
	"strings"
	"time"
)

type apiCandidate = capabilities.APICandidate

var balanceCandidates = []string{
	"/v1/dashboard/billing/subscription",
	"/v1/usage",
	"/api/usage/token/",
	"/api/log/token",
	"/api/user/self",
	"/api/user/quota",
	"/api/balance",
}

const (
	checkinMaxNetworkAttempts = 3
	checkinRetryBaseDelay     = 100 * time.Millisecond
)

type accountAuthContext struct {
	AccountID              string
	AccountName            string
	UpstreamSiteID         string
	UpstreamSite           string
	SiteKind               string
	ChannelID              string
	BaseURL                string
	LoginPath              string
	BrowserLoginURL        string
	BrowserLoginSource     string
	BrowserLoginConfidence float64
	BrowserLoginReason     string
	BrowserProfilePath     string
	UserAgent              string
	LoginName              string
	Password               string
	AuthUserID             string
	Cookie                 string
	AccessToken            string
	APIKey                 string
	SupportsCheckin        bool
	SupportsBalance        bool
	CheckinRules           []apiCandidate
}

type checkinResult struct {
	Status            string `json:"status"`
	Message           string `json:"message,omitempty"`
	Reward            string `json:"reward,omitempty"`
	HTTPStatus        int    `json:"httpStatus,omitempty"`
	Path              string `json:"path,omitempty"`
	RawResponseMasked string `json:"rawResponseMasked,omitempty"`
	RetryCount        int    `json:"retryCount,omitempty"`
}

type balanceResult struct {
	Balance           *float64 `json:"balance,omitempty"`
	UsedQuota         *float64 `json:"usedQuota,omitempty"`
	TotalQuota        *float64 `json:"totalQuota,omitempty"`
	Unit              string   `json:"unit"`
	HTTPStatus        int      `json:"httpStatus,omitempty"`
	Path              string   `json:"path,omitempty"`
	RawResponseMasked string   `json:"rawResponseMasked,omitempty"`
}

type bulkBalanceRefreshItem struct {
	AccountID   string `json:"accountId"`
	AccountName string `json:"accountName"`
	SiteName    string `json:"siteName"`
	Status      string `json:"status"`
	Message     string `json:"message"`
	Balance     string `json:"balance,omitempty"`
	Path        string `json:"path,omitempty"`
}

type checkinRunAccount struct {
	ID             string
	AccountName    string
	UpstreamSiteID string
	SiteName       string
}

func (a *App) handleTodayCheckins(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	today := todayCST()
	rows, err := a.db.QueryContext(r.Context(), `
		SELECT l.id, l.account_id, a.display_name, l.upstream_site_id, s.name, COALESCE(l.channel_id,''),
		       l.status, COALESCE(l.reward,''), COALESCE(l.message,''), COALESCE(l.raw_response_masked,''),
		       l.started_at, l.finished_at
		FROM checkin_logs l
		JOIN channel_accounts a ON a.id = l.account_id
		JOIN upstream_sites s ON s.id = l.upstream_site_id
		WHERE substr(l.started_at, 1, 10) = ?
		ORDER BY l.started_at DESC
	`, today)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	logs, err := scanCheckinLogs(rows)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, logs)
}

func (a *App) handleCheckinStatus(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	status, err := cachedRead(a, "checkin-status", shortReadCacheTTL, func() (CheckinStatus, error) {
		return a.buildCheckinStatus(r.Context(), nowCST())
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (a *App) handleBulkRefreshBalances(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var input struct {
		Limit       int  `json:"limit"`
		MissingOnly bool `json:"missingOnly"`
	}
	_ = decodeJSON(r, &input)
	input.Limit = clampBatchLimit(input.Limit, 10)
	accountIDs, err := a.loadBalanceRefreshAccountIDs(r.Context(), input.Limit, input.MissingOnly)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	results := []bulkBalanceRefreshItem{}
	success := 0
	auths, _ := a.loadAccountAuths(r.Context(), accountIDs)
	for _, id := range accountIDs {
		var auth *accountAuthContext
		if loaded, ok := auths[id]; ok {
			auth = &loaded
		}
		item := a.refreshBalanceForBulk(r.Context(), id, auth)
		if item.Status == "success" {
			success++
		}
		results = append(results, item)
	}
	if len(results) > 0 {
		a.notify("bulk_balance_refresh", "info", "批量余额刷新完成", fmt.Sprintf("处理 %d 个账号，成功 %d 个。", len(results), success), "account", "")
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"processed": len(results),
		"success":   success,
		"failed":    len(results) - success,
		"results":   results,
	})
}

func (a *App) loadBalanceRefreshAccountIDs(ctx context.Context, limit int, missingOnly bool) ([]string, error) {
	query := `
		SELECT a.id
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
		return nil, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}

func (a *App) refreshBalanceForBulk(ctx context.Context, id string, auth *accountAuthContext) bulkBalanceRefreshItem {
	item := bulkBalanceRefreshItem{AccountID: id, Status: "failed"}
	if auth != nil {
		item.AccountName = auth.AccountName
		item.SiteName = auth.UpstreamSite
	}
	result, err := a.refreshAccountBalance(ctx, id, auth)
	if err != nil {
		item.Message = err.Error()
		return item
	}
	item.Status = "success"
	item.Message = "余额已刷新。"
	item.Path = result.Path
	item.Balance = formatBalanceForMessage(result.Balance, result.Unit)
	return item
}

func formatBalanceForMessage(value *float64, unit string) string {
	if value == nil {
		return ""
	}
	unit = strings.TrimSpace(unit)
	if unit == "" {
		unit = "unknown"
	}
	return fmt.Sprintf("%.4g %s", *value, unit)
}

func (a *App) handleCheckinLogs(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `
		SELECT l.id, l.account_id, a.display_name, l.upstream_site_id, s.name, COALESCE(l.channel_id,''),
		       l.status, COALESCE(l.reward,''), COALESCE(l.message,''), COALESCE(l.raw_response_masked,''),
		       l.started_at, l.finished_at
		FROM checkin_logs l
		JOIN channel_accounts a ON a.id = l.account_id
		JOIN upstream_sites s ON s.id = l.upstream_site_id
		ORDER BY l.started_at DESC
		LIMIT 200
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	items, err := scanCheckinLogs(rows)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func scanCheckinLogs(rows *sql.Rows) ([]CheckinLog, error) {
	items := []CheckinLog{}
	for rows.Next() {
		var item CheckinLog
		if err := rows.Scan(&item.ID, &item.AccountID, &item.AccountName, &item.UpstreamSiteID, &item.UpstreamSiteName, &item.ChannelID, &item.Status, &item.Reward, &item.Message, &item.RawResponseMasked, &item.StartedAt, &item.FinishedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (a *App) handleRunAllCheckins(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	results, err := a.runDueCheckins(r.Context(), "manual")
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func (a *App) runDueCheckins(ctx context.Context, mode string) ([]map[string]interface{}, error) {
	return a.checkinBatch.Run(ctx, mode)
}

// runDueCheckinsForSite runs checkins only for accounts belonging to the given site.
func (a *App) runDueCheckinsForSite(ctx context.Context, siteID string) ([]map[string]interface{}, error) {
	return a.checkinBatch.RunForSite(ctx, siteID)
}

var errCheckinRunBusy = errorsText("checkin run already in progress")

func (a *App) runDueCheckinsWithFilter(ctx context.Context, mode string, siteID string, currentMessage string, emptyMessage string) ([]map[string]interface{}, error) {
	return a.checkinBatch.runWithFilter(ctx, mode, siteID, currentMessage, emptyMessage)
}

func (a *App) loadDueCheckinAccounts(ctx context.Context, siteID string, limit int) ([]checkinRunAccount, error) {
	return a.checkinBatch.LoadDueAccounts(ctx, siteID, limit)
}

func (a *App) beginCheckinRun(mode string, total int) bool {
	return a.checkinRun.begin(mode, total)
}

func (a *App) updateCheckinRunCurrent(accountID string, accountName string, siteName string, message string) {
	a.checkinRun.updateCurrent(accountID, accountName, siteName, message)
}

func (a *App) updateCheckinRunMessage(message string) {
	a.checkinRun.updateMessage(message)
}

func (a *App) recordCheckinRunResult(status string, message string) {
	a.checkinRun.recordResult(status, message)
}

func (a *App) finishCheckinRun() {
	a.checkinRun.finish()
}

func (a *App) buildCheckinStatus(ctx context.Context, currentTime time.Time) (CheckinStatus, error) {
	run := a.checkinRun.Snapshot()
	status := CheckinStatus{
		GeneratedAt:       now(),
		Running:           run.Running,
		Mode:              firstNonEmpty(run.Mode, "idle"),
		CurrentAccountID:  run.CurrentAccountID,
		CurrentAccount:    run.CurrentAccount,
		CurrentSite:       run.CurrentSite,
		CurrentMessage:    run.CurrentMessage,
		TotalAccounts:     run.TotalAccounts,
		ProcessedAccounts: run.ProcessedAccounts,
		PendingAccounts:   maxInt(0, run.TotalAccounts-run.ProcessedAccounts),
		SuccessCount:      run.SuccessCount,
		AlreadyCount:      run.AlreadyCount,
		FailedCount:       run.FailedCount,
		UnsupportedCount:  run.UnsupportedCount,
		AuthExpiredCount:  run.AuthExpiredCount,
		StartedAt:         run.StartedAt,
		UpdatedAt:         run.UpdatedAt,
		FinishedAt:        run.FinishedAt,
		LastRunMessage:    run.LastRunMessage,
	}
	today, err := a.checkinTodaySummary(ctx)
	if err != nil {
		return status, err
	}
	status.Today = today
	schedule, err := a.checkinScheduleStatus(ctx, currentTime)
	if err != nil {
		return status, err
	}
	a.applySchedulerPlanToCheckinStatus(ctx, currentTime, &schedule)
	status.Schedule = schedule
	return status, nil
}

func (a *App) checkinTodaySummary(ctx context.Context) (CheckinTodaySummary, error) {
	summary := CheckinTodaySummary{}
	rows, err := a.db.QueryContext(ctx, `
		SELECT status, COUNT(*)
		FROM checkin_logs
		WHERE substr(started_at, 1, 10)=?
		GROUP BY status
	`, todayCST())
	if err != nil {
		return summary, err
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return summary, err
		}
		summary.TotalLogs += count
		switch status {
		case "success":
			summary.SuccessCount += count
		case "already_checked":
			summary.AlreadyCount += count
		case "unsupported":
			summary.UnsupportedCount += count
		case "auth_expired", "manual_required":
			summary.AuthExpiredCount += count
		default:
			summary.FailedCount += count
		}
	}
	dueAccounts, err := a.loadDueCheckinAccounts(ctx, "", 0)
	if err != nil {
		return summary, err
	}
	summary.DueAccounts = len(dueAccounts)
	return summary, rows.Err()
}

func (a *App) checkinScheduleStatus(ctx context.Context, currentTime time.Time) (CheckinScheduleStatus, error) {
	config := a.loadCheckinScheduleConfig(ctx)
	return computeCheckinScheduleStatus(config.Enabled, config.Time, config.RandomDelayMinutes, currentTime), nil
}

func computeCheckinScheduleStatus(enabled bool, scheduleTime string, randomDelayMinutes []int, currentTime time.Time) CheckinScheduleStatus {
	status := CheckinScheduleStatus{
		Enabled: enabled,
		Time:    firstNonEmpty(scheduleTime, "08:00"),
	}
	if len(randomDelayMinutes) >= 2 {
		status.RandomDelayMin = randomDelayMinutes[0]
		status.RandomDelayMax = randomDelayMinutes[1]
	}
	if status.RandomDelayMin < 0 {
		status.RandomDelayMin = 0
	}
	if status.RandomDelayMax < status.RandomDelayMin {
		status.RandomDelayMax = status.RandomDelayMin
	}
	if !enabled {
		status.Message = "自动签到未启用。"
		return status
	}
	parsedTime, err := time.Parse("15:04", status.Time)
	if err != nil {
		status.Message = "签到时间格式无效，请使用 HH:MM。"
		return status
	}
	base := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), parsedTime.Hour(), parsedTime.Minute(), 0, 0, currentTime.Location())
	start := base.Add(time.Duration(status.RandomDelayMin) * time.Minute)
	end := base.Add(time.Duration(status.RandomDelayMax) * time.Minute)
	if currentTime.After(end) {
		base = base.Add(24 * time.Hour)
		start = base.Add(time.Duration(status.RandomDelayMin) * time.Minute)
		end = base.Add(time.Duration(status.RandomDelayMax) * time.Minute)
	}
	status.NextWindowStartAt = start.UTC().Format(time.RFC3339Nano)
	status.NextWindowEndAt = end.UTC().Format(time.RFC3339Nano)
	if currentTime.Before(start) {
		status.NextRunInSeconds = int64(start.Sub(currentTime).Seconds())
		status.NextWindowInSeconds = int64(end.Sub(currentTime).Seconds())
		status.Message = "等待下一次自动签到窗口。"
		return status
	}
	status.NextRunInSeconds = 0
	status.NextWindowInSeconds = maxInt64(0, int64(end.Sub(currentTime).Seconds()))
	status.Message = "当前处于自动签到窗口。"
	return status
}

func (a *App) runAccountCheckin(ctx context.Context, id string, auth *accountAuthContext) (checkinResult, error) {
	return a.checkinExecutor.Run(ctx, id, auth)
}

func (a *App) callCheckinAPIWithRetry(ctx context.Context, auth accountAuthContext, candidate apiCandidate) (int, string, int, error) {
	return a.checkinExecutor.callAPIWithRetry(ctx, auth, candidate)
}

func shouldRetryCheckinAttempt(status int, err error) bool {
	if err != nil {
		return true
	}
	return status == http.StatusRequestTimeout || status == http.StatusTooManyRequests || status >= 500
}

func checkinRetryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	return checkinRetryBaseDelay * time.Duration(1<<(attempt-1))
}

func sleepWithContext(ctx context.Context, delay time.Duration) bool {
	if delay <= 0 {
		return true
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func annotateCheckinRetry(result checkinResult) checkinResult {
	if result.RetryCount <= 0 {
		return result
	}
	suffix := fmt.Sprintf("已自动重试 %d 次。", result.RetryCount)
	if strings.Contains(result.Message, suffix) {
		return result
	}
	result.Message = strings.TrimSpace(result.Message)
	if result.Message == "" {
		result.Message = suffix
		return result
	}
	result.Message = strings.TrimRight(result.Message, "。.!！") + "。" + suffix
	return result
}

func classifyCheckinResponse(status int, body string) checkinResult {
	text := strings.ToLower(body)
	message := extractMessage(body)
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return checkinResult{Status: "auth_expired", Message: firstNonEmpty(message, "登录态已失效。")}
	case isCheckinDisabledText(body):
		return checkinResult{Status: "unsupported", Message: firstNonEmpty(message, "该站点未开启签到。")}
	case isTurnstileRequired(body):
		return checkinResult{Status: "manual_required", Message: firstNonEmpty(message, "该站点签到需要人机验证（Turnstile），请手动签到。")}
	case status < 200 || status >= 300:
		return checkinResult{Status: "failed", Message: firstNonEmpty(message, fmt.Sprintf("签到接口返回 HTTP %d。", status))}
	case isAlreadyCheckedIn(text, body):
		return checkinResult{Status: "already_checked", Message: firstNonEmpty(message, "今日已签到。")}
	case strings.Contains(text, "login") && strings.Contains(text, "<html"):
		return checkinResult{Status: "auth_expired", Message: "接口返回登录页，请重新保存授权。"}
	case strings.Contains(text, `"success":false`) || strings.Contains(text, `"ok":false`):
		return checkinResult{Status: "failed", Message: firstNonEmpty(message, "签到失败。")}
	default:
		reward := extractCheckinReward(body)
		msg := firstNonEmpty(message, "签到成功。")
		if reward != "" {
			msg = msg + " 奖励：" + reward
		}
		return checkinResult{Status: "success", Message: msg, Reward: reward}
	}
}

// isAlreadyCheckedIn detects "already checked in" responses with broader keyword coverage.
func isAlreadyCheckedIn(text, body string) bool {
	alreadyKeywords := []string{
		"already", "today", "checked_in_today", "已经签到", "已签到",
		"今日已签", "今天已签", "不可重复", "重复签到", "已领取",
	}
	for _, kw := range alreadyKeywords {
		if strings.Contains(text, strings.ToLower(kw)) || strings.Contains(body, kw) {
			return true
		}
	}
	return false
}

// isTurnstileRequired detects Turnstile/Cloudflare challenge responses.
func isTurnstileRequired(body string) bool {
	text := strings.ToLower(body)
	turnstileKeywords := []string{
		"turnstile", "cf-turnstile", "cloudflare",
		"人机验证", "验证码", "captcha", "challenge",
	}
	for _, kw := range turnstileKeywords {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}

// extractCheckinReward parses reward amount from checkin response.
func extractCheckinReward(body string) string {
	var payload map[string]interface{}
	if json.Unmarshal([]byte(body), &payload) != nil {
		return ""
	}
	// Try common reward field names.
	for _, key := range []string{"reward", "quota", "amount", "bonus", "gift"} {
		if val, ok := payload[key]; ok {
			if num, ok := toFloat(val); ok && num > 0 {
				return fmt.Sprintf("%.2f", num)
			}
			if str, ok := val.(string); ok && str != "" {
				return str
			}
		}
	}
	// Try nested data.reward.
	if data, ok := payload["data"].(map[string]interface{}); ok {
		for _, key := range []string{"reward", "quota", "amount", "bonus"} {
			if val, ok := data[key]; ok {
				if num, ok := toFloat(val); ok && num > 0 {
					return fmt.Sprintf("%.2f", num)
				}
			}
		}
	}
	return ""
}

func (a *App) saveCheckinResult(ctx context.Context, auth accountAuthContext, result checkinResult, startedAt string, finishedAt string) error {
	return a.checkinExecutor.SaveResult(ctx, auth, result, startedAt, finishedAt)
}

func (a *App) handleBalanceSnapshots(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `
		SELECT b.id, b.account_id, a.display_name, b.upstream_site_id, s.name, COALESCE(b.channel_id,''),
		       b.balance, b.used_quota, b.total_quota, b.unit, COALESCE(b.raw_response_masked,''), b.created_at
		FROM balance_snapshots b
		JOIN channel_accounts a ON a.id = b.account_id
		JOIN upstream_sites s ON s.id = b.upstream_site_id
		ORDER BY b.created_at DESC
		LIMIT 200
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	items := []BalanceSnapshot{}
	for rows.Next() {
		var item BalanceSnapshot
		var balance, used, total sql.NullFloat64
		if err := rows.Scan(&item.ID, &item.AccountID, &item.AccountName, &item.UpstreamSiteID, &item.UpstreamSiteName, &item.ChannelID, &balance, &used, &total, &item.Unit, &item.RawResponseMasked, &item.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		item.Balance = nullableFloat(balance)
		item.UsedQuota = nullableFloat(used)
		item.TotalQuota = nullableFloat(total)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (a *App) refreshAccountBalance(ctx context.Context, id string, auth *accountAuthContext) (balanceResult, error) {
	return a.balanceRefresher.Run(ctx, id, auth)
}

func (a *App) saveBalanceResult(ctx context.Context, auth accountAuthContext, result balanceResult) error {
	return a.balanceRefresher.SaveResult(ctx, auth, result)
}

func (a *App) loadAccountAuth(ctx context.Context, id string) (accountAuthContext, error) {
	auth, err := a.accountAuth.Load(ctx, id)
	if err != nil {
		return accountAuthContext{}, err
	}
	return *auth, nil
}

// loadAccountAuths batch-loads accountAuthContext for multiple account IDs in
// a single query, eliminating N+1 lookups in bulk operations. Returns a map
// keyed by account ID. If a particular ID is not found, it is simply absent
// from the map; callers should fall back to loadAccountAuth for missing entries.
func (a *App) loadAccountAuths(ctx context.Context, ids []string) (map[string]accountAuthContext, error) {
	return a.accountAuth.LoadBatch(ctx, ids)
}

func parseCheckinRules(raw string) []apiCandidate {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var one struct {
		Method string `json:"method"`
		Path   string `json:"path"`
		URL    string `json:"url"`
	}
	if json.Unmarshal([]byte(raw), &one) == nil && (one.Path != "" || one.URL != "") {
		method := strings.ToUpper(firstNonEmpty(one.Method, http.MethodPost))
		path := firstNonEmpty(one.Path, pathFromMaybeURL(one.URL))
		if strings.HasPrefix(path, "/") {
			return []apiCandidate{{Method: method, Path: path}}
		}
	}
	var many []struct {
		Method string `json:"method"`
		Path   string `json:"path"`
		URL    string `json:"url"`
	}
	if json.Unmarshal([]byte(raw), &many) == nil {
		rules := []apiCandidate{}
		for _, item := range many {
			method := strings.ToUpper(firstNonEmpty(item.Method, http.MethodPost))
			path := firstNonEmpty(item.Path, pathFromMaybeURL(item.URL))
			if strings.HasPrefix(path, "/") {
				rules = append(rules, apiCandidate{Method: method, Path: path})
			}
		}
		return rules
	}
	return nil
}

func pathFromMaybeURL(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "/") {
		return trimmed
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	if parsed.RawQuery != "" {
		return parsed.Path + "?" + parsed.RawQuery
	}
	return parsed.Path
}

func (a *App) ensureAccountSession(ctx context.Context, auth *accountAuthContext) error {
	return a.accountSession.Ensure(ctx, auth)
}

func (a *App) loginWithPassword(ctx context.Context, auth *accountAuthContext) error {
	return a.accountSession.LoginWithPassword(ctx, auth)
}

func (a *App) doLoginHTTP(req *http.Request, jar *cookiejar.Jar) (*http.Response, error) {
	if a != nil && a.db == nil && a.client != nil {
		client := *a.client
		client.Jar = jar
		return client.Do(req)
	}
	policy := outboundURLPolicy{}
	if a != nil {
		policy = a.externalURLPolicy()
	}
	client := newNetworkHTTPClient(defaultHTTPTimeout, a.currentNetworkProxyConfig(), policy)
	client.Jar = jar
	return client.Do(req)
}

func (a *App) saveAccountSession(ctx context.Context, auth *accountAuthContext, cookie string, accessToken string, authUserID string) error {
	return a.accountSession.Save(ctx, auth, cookie, accessToken, authUserID)
}

func (a *App) callAccountAPI(ctx context.Context, auth accountAuthContext, method string, path string, body []byte) (int, string, error) {
	return a.accountAPI.Do(ctx, auth, method, path, body)
}

func parseBalance(body string) balanceResult {
	result := balanceResult{Unit: "unknown"}
	var payload interface{}
	if json.Unmarshal([]byte(body), &payload) != nil {
		return result
	}
	quotaBalance := findNumber(payload, "quota", "remaining_quota", "remain_quota")
	result.Balance = findNumber(payload, "balance", "quota", "remaining_quota", "remain_quota", "remaining", "available")
	result.UsedQuota = findNumber(payload, "used_quota", "used", "usage", "used_amount", "used_tokens")
	result.TotalQuota = findNumber(payload, "total_quota", "hard_limit_usd", "limit", "total", "quota_limit")
	if result.Balance != nil || result.UsedQuota != nil || result.TotalQuota != nil {
		if quotaBalance != nil || result.UsedQuota != nil || result.TotalQuota != nil {
			result.Unit = "quota"
		} else {
			result.Unit = inferBalanceUnit(body)
		}
	}
	return result
}

func findNumber(value interface{}, keys ...string) *float64 {
	switch typed := value.(type) {
	case map[string]interface{}:
		for _, wanted := range keys {
			for key, child := range typed {
				if strings.EqualFold(key, wanted) {
					if number, ok := toFloat(child); ok {
						return &number
					}
				}
			}
		}
		for _, child := range typed {
			if found := findNumber(child, keys...); found != nil {
				return found
			}
		}
	case []interface{}:
		for _, child := range typed {
			if found := findNumber(child, keys...); found != nil {
				return found
			}
		}
	}
	return nil
}

func toFloat(value interface{}) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, !math.IsNaN(typed)
	case int:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	case string:
		var parsed float64
		if _, err := fmt.Sscanf(strings.TrimSpace(typed), "%f", &parsed); err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func inferBalanceUnit(body string) string {
	text := strings.ToLower(body)
	switch {
	case strings.Contains(text, "quota"):
		return "quota"
	case strings.Contains(text, "usd") || strings.Contains(text, "dollar"):
		return "usd"
	case strings.Contains(text, "cny") || strings.Contains(body, "人民币"):
		return "cny"
	case strings.Contains(text, "token"):
		return "token"
	case strings.Contains(text, "quota"):
		return "quota"
	default:
		return "unknown"
	}
}

func extractMessage(body string) string {
	var payload interface{}
	if json.Unmarshal([]byte(body), &payload) != nil {
		return ""
	}
	for _, key := range []string{"message", "msg", "error", "detail"} {
		if value := findString(payload, key); value != "" {
			return value
		}
	}
	return ""
}

func findString(value interface{}, wanted string) string {
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, child := range typed {
			if strings.EqualFold(key, wanted) {
				return strings.TrimSpace(fmt.Sprint(child))
			}
		}
		for _, child := range typed {
			if found := findString(child, wanted); found != "" {
				return found
			}
		}
	case []interface{}:
		for _, child := range typed {
			if found := findString(child, wanted); found != "" {
				return found
			}
		}
	}
	return ""
}

func nullableFloat(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	return &value.Float64
}

func maskResponse(body string) string {
	trimmed := strings.TrimSpace(body)
	if len(trimmed) > 2000 {
		trimmed = trimmed[:2000] + "...(truncated)"
	}
	return trimmed
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func errorsText(message string) error {
	return fmt.Errorf("%s", message)
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}

func maxInt64(left int64, right int64) int64 {
	if left > right {
		return left
	}
	return right
}
