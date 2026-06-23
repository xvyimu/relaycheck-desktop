package core

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type apiCandidate struct {
	Method string
	Path   string
}

var checkinCandidates = []apiCandidate{
	{http.MethodPost, "/api/user/checkin"},
	{http.MethodGet, "/api/user/checkin"},
	{http.MethodPost, "/api/checkin"},
	{http.MethodGet, "/api/checkin"},
	{http.MethodPost, "/api/user/check_in"},
	{http.MethodGet, "/api/user/check_in"},
	{http.MethodPost, "/api/user/signin"},
	{http.MethodGet, "/api/user/signin"},
	{http.MethodPost, "/api/user/sign_in"},
	{http.MethodGet, "/api/user/sign_in"},
	{http.MethodPost, "/api/user/sign-in"},
	{http.MethodGet, "/api/user/sign-in"},
	{http.MethodPost, "/api/signin"},
	{http.MethodGet, "/api/signin"},
	{http.MethodPost, "/api/sign_in"},
	{http.MethodGet, "/api/sign_in"},
	{http.MethodPost, "/api/sign-in"},
	{http.MethodGet, "/api/sign-in"},
	{http.MethodPost, "/api/daily_checkin"},
	{http.MethodGet, "/api/daily_checkin"},
	{http.MethodPost, "/api/daily-checkin"},
	{http.MethodGet, "/api/daily-checkin"},
}

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
	AccountID       string
	AccountName     string
	UpstreamSiteID  string
	UpstreamSite    string
	ChannelID       string
	BaseURL         string
	LoginPath       string
	UserAgent       string
	LoginName       string
	Password        string
	AuthUserID      string
	Cookie          string
	AccessToken     string
	APIKey          string
	SupportsCheckin bool
	SupportsBalance bool
	CheckinRules    []apiCandidate
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
	today := time.Now().Format("2006-01-02")
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
	writeJSON(w, http.StatusOK, scanCheckinLogs(rows))
}

func (a *App) handleCheckinStatus(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	status, err := a.buildCheckinStatus(r.Context(), time.Now())
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
	for _, id := range accountIDs {
		item := a.refreshBalanceForBulk(r.Context(), id)
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
	return ids, nil
}

func (a *App) refreshBalanceForBulk(ctx context.Context, id string) bulkBalanceRefreshItem {
	item := bulkBalanceRefreshItem{AccountID: id, Status: "failed"}
	_ = a.db.QueryRowContext(ctx, `
		SELECT a.display_name, s.name
		FROM channel_accounts a
		JOIN upstream_sites s ON s.id = a.upstream_site_id
		WHERE a.id = ?
	`, id).Scan(&item.AccountName, &item.SiteName)
	result, err := a.refreshAccountBalance(ctx, id)
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
	writeJSON(w, http.StatusOK, scanCheckinLogs(rows))
}

func scanCheckinLogs(rows *sql.Rows) []CheckinLog {
	items := []CheckinLog{}
	for rows.Next() {
		var item CheckinLog
		_ = rows.Scan(&item.ID, &item.AccountID, &item.AccountName, &item.UpstreamSiteID, &item.UpstreamSiteName, &item.ChannelID, &item.Status, &item.Reward, &item.Message, &item.RawResponseMasked, &item.StartedAt, &item.FinishedAt)
		items = append(items, item)
	}
	return items
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
	accounts, err := a.loadDueCheckinAccounts(ctx, 0)
	if err != nil {
		return nil, err
	}
	if !a.beginCheckinRun(mode, len(accounts)) {
		return nil, errorsText("已有签到任务正在运行，请等待当前任务完成。")
	}
	defer a.finishCheckinRun()

	results := []map[string]interface{}{}
	if len(accounts) == 0 {
		a.updateCheckinRunMessage("今天没有待签到账号。")
	}
	siteLimiter := newCheckinSiteLimiter(a.loadCheckinScheduleConfig(ctx))
	for _, account := range accounts {
		if err := siteLimiter.wait(ctx, account.UpstreamSiteID); err != nil {
			return results, err
		}
		a.updateCheckinRunCurrent(account.ID, account.AccountName, account.SiteName, "正在签到...")
		result, err := a.runAccountCheckin(ctx, account.ID)
		entry := map[string]interface{}{"accountId": account.ID, "accountName": account.AccountName, "siteName": account.SiteName}
		if err != nil {
			entry["status"] = "failed"
			entry["message"] = err.Error()
			a.recordCheckinRunResult("failed", err.Error())
		} else {
			entry["status"] = result.Status
			entry["message"] = result.Message
			entry["path"] = result.Path
			a.recordCheckinRunResult(result.Status, result.Message)
		}
		results = append(results, entry)
	}
	return results, nil
}

func (a *App) loadDueCheckinAccounts(ctx context.Context, limit int) ([]checkinRunAccount, error) {
	query := `
		SELECT a.id, a.display_name, s.id, s.name
		FROM channel_accounts a
		JOIN upstream_sites s ON s.id = a.upstream_site_id
		WHERE COALESCE(a.last_checkin_status,'') NOT IN ('success','already_checked')
		   OR COALESCE(substr(a.last_checkin_at, 1, 10),'') <> ?
		ORDER BY a.updated_at DESC
	`
	args := []interface{}{time.Now().UTC().Format("2006-01-02")}
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := a.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	accounts := []checkinRunAccount{}
	for rows.Next() {
		var account checkinRunAccount
		if err := rows.Scan(&account.ID, &account.AccountName, &account.UpstreamSiteID, &account.SiteName); err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	return accounts, rows.Err()
}

type checkinSiteLimiter struct {
	minInterval time.Duration
	lastStarted map[string]time.Time
}

func newCheckinSiteLimiter(config checkinScheduleConfig) *checkinSiteLimiter {
	interval := time.Duration(config.SiteMinIntervalSeconds) * time.Second
	if interval < 0 {
		interval = 0
	}
	return &checkinSiteLimiter{
		minInterval: interval,
		lastStarted: map[string]time.Time{},
	}
}

func (l *checkinSiteLimiter) wait(ctx context.Context, siteID string) error {
	if l == nil || l.minInterval <= 0 || strings.TrimSpace(siteID) == "" {
		return nil
	}
	nowTime := time.Now()
	delay := l.delayFor(siteID, nowTime)
	if delay > 0 && !sleepWithContext(ctx, delay) {
		return ctx.Err()
	}
	l.lastStarted[siteID] = time.Now()
	return nil
}

func (l *checkinSiteLimiter) delayFor(siteID string, nowTime time.Time) time.Duration {
	lastStarted, exists := l.lastStarted[siteID]
	if !exists {
		return 0
	}
	elapsed := nowTime.Sub(lastStarted)
	if elapsed >= l.minInterval {
		return 0
	}
	return l.minInterval - elapsed
}

func (a *App) beginCheckinRun(mode string, total int) bool {
	timestamp := now()
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.checkinRun.Running {
		return false
	}
	a.checkinRun = checkinRunState{
		Running:       true,
		Mode:          mode,
		TotalAccounts: total,
		StartedAt:     timestamp,
		UpdatedAt:     timestamp,
	}
	if total == 0 {
		a.checkinRun.CurrentMessage = "今天没有待签到账号。"
	}
	return true
}

func (a *App) updateCheckinRunCurrent(accountID string, accountName string, siteName string, message string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.checkinRun.Running {
		return
	}
	a.checkinRun.CurrentAccountID = accountID
	a.checkinRun.CurrentAccount = accountName
	a.checkinRun.CurrentSite = siteName
	a.checkinRun.CurrentMessage = message
	a.checkinRun.UpdatedAt = now()
}

func (a *App) updateCheckinRunMessage(message string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.checkinRun.CurrentMessage = message
	a.checkinRun.LastRunMessage = message
	a.checkinRun.UpdatedAt = now()
}

func (a *App) recordCheckinRunResult(status string, message string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.checkinRun.Running {
		return
	}
	a.checkinRun.ProcessedAccounts++
	a.checkinRun.CurrentMessage = firstNonEmpty(message, status)
	a.checkinRun.LastRunMessage = a.checkinRun.CurrentMessage
	switch status {
	case "success":
		a.checkinRun.SuccessCount++
	case "already_checked":
		a.checkinRun.AlreadyCount++
	case "unsupported":
		a.checkinRun.UnsupportedCount++
	case "auth_expired", "manual_required":
		a.checkinRun.AuthExpiredCount++
	default:
		a.checkinRun.FailedCount++
	}
	a.checkinRun.UpdatedAt = now()
}

func (a *App) finishCheckinRun() {
	a.mu.Lock()
	defer a.mu.Unlock()
	timestamp := now()
	a.checkinRun.Running = false
	a.checkinRun.FinishedAt = timestamp
	a.checkinRun.UpdatedAt = timestamp
	a.checkinRun.CurrentAccountID = ""
	a.checkinRun.CurrentAccount = ""
	a.checkinRun.CurrentSite = ""
	if a.checkinRun.LastRunMessage == "" {
		a.checkinRun.LastRunMessage = fmt.Sprintf("本轮处理 %d 个账号。", a.checkinRun.ProcessedAccounts)
	}
}

func (a *App) buildCheckinStatus(ctx context.Context, currentTime time.Time) (CheckinStatus, error) {
	a.mu.RLock()
	run := a.checkinRun
	a.mu.RUnlock()
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
	`, time.Now().UTC().Format("2006-01-02"))
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
	dueAccounts, err := a.loadDueCheckinAccounts(ctx, 0)
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

func (a *App) runAccountCheckin(ctx context.Context, id string) (checkinResult, error) {
	auth, err := a.loadAccountAuth(ctx, id)
	if err != nil {
		return checkinResult{}, err
	}
	if !auth.SupportsCheckin {
		result := checkinResult{Status: "unsupported", Message: "该站点未探测到签到接口。"}
		_ = a.saveCheckinResult(ctx, auth, result, now(), now())
		return result, nil
	}
	if err := a.ensureAccountSession(ctx, &auth); err != nil && auth.Cookie == "" && auth.AccessToken == "" && auth.APIKey == "" {
		result := checkinResult{Status: "auth_expired", Message: "账号密码登录失败：" + err.Error()}
		_ = a.saveCheckinResult(ctx, auth, result, now(), now())
		return result, nil
	}

	startedAt := now()
	lastUnsupported := checkinResult{Status: "unsupported", Message: "未找到可用签到接口。"}
	candidates := append([]apiCandidate{}, auth.CheckinRules...)
	candidates = append(candidates, checkinCandidates...)
	for _, candidate := range candidates {
		status, body, retries, err := a.callCheckinAPIWithRetry(ctx, auth, candidate)
		if err != nil {
			lastUnsupported = annotateCheckinRetry(checkinResult{Status: "failed", Message: err.Error(), Path: candidate.Path, RetryCount: retries})
			continue
		}
		if status == http.StatusNotFound || status == http.StatusMethodNotAllowed {
			continue
		}
		result := classifyCheckinResponse(status, body)
		if result.Status == "auth_expired" && auth.Password != "" {
			auth.Cookie = ""
			auth.AccessToken = ""
			auth.AuthUserID = ""
			if loginErr := a.loginWithPassword(ctx, &auth); loginErr != nil {
				result.Message = "账号密码登录失败：" + loginErr.Error()
				result.HTTPStatus = 0
				result.Path = ""
				result.RawResponseMasked = ""
				result.RetryCount = retries
				result = annotateCheckinRetry(result)
				_ = a.saveCheckinResult(ctx, auth, result, startedAt, now())
				return result, nil
			}
			var retryAfterLogin int
			status, body, retryAfterLogin, err = a.callCheckinAPIWithRetry(ctx, auth, candidate)
			retries += retryAfterLogin
			if err != nil {
				lastUnsupported = annotateCheckinRetry(checkinResult{Status: "failed", Message: err.Error(), Path: candidate.Path, RetryCount: retries})
				continue
			}
			result = classifyCheckinResponse(status, body)
		}
		result.HTTPStatus = status
		result.Path = candidate.Path
		result.RawResponseMasked = maskResponse(body)
		result.RetryCount = retries
		if result.Message == "" {
			result.Message = fmt.Sprintf("%s %s 返回 HTTP %d", candidate.Method, candidate.Path, status)
		}
		result = annotateCheckinRetry(result)
		_ = a.saveCheckinResult(ctx, auth, result, startedAt, now())
		return result, nil
	}
	_ = a.saveCheckinResult(ctx, auth, lastUnsupported, startedAt, now())
	return lastUnsupported, nil
}

func (a *App) callCheckinAPIWithRetry(ctx context.Context, auth accountAuthContext, candidate apiCandidate) (int, string, int, error) {
	var status int
	var body string
	var err error
	attempts := checkinMaxNetworkAttempts
	if attempts < 1 {
		attempts = 1
	}
	for attempt := 1; attempt <= attempts; attempt++ {
		status, body, err = a.callAccountAPI(ctx, auth, candidate.Method, candidate.Path, nil)
		if !shouldRetryCheckinAttempt(status, err) || attempt == attempts {
			return status, body, attempt - 1, err
		}
		if !sleepWithContext(ctx, checkinRetryDelay(attempt)) {
			return status, body, attempt - 1, ctx.Err()
		}
	}
	return status, body, attempts - 1, err
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
	case status < 200 || status >= 300:
		return checkinResult{Status: "failed", Message: firstNonEmpty(message, fmt.Sprintf("签到接口返回 HTTP %d。", status))}
	case strings.Contains(text, "already") || strings.Contains(text, "today") || strings.Contains(body, "已签到") || strings.Contains(body, "重复"):
		return checkinResult{Status: "already_checked", Message: firstNonEmpty(message, "今日已签到。")}
	case strings.Contains(text, "login") && strings.Contains(text, "<html"):
		return checkinResult{Status: "auth_expired", Message: "接口返回登录页，请重新保存授权。"}
	case strings.Contains(text, `"success":false`) || strings.Contains(text, `"ok":false`):
		return checkinResult{Status: "failed", Message: firstNonEmpty(message, "签到失败。")}
	default:
		return checkinResult{Status: "success", Message: firstNonEmpty(message, "签到成功。")}
	}
}

func (a *App) saveCheckinResult(ctx context.Context, auth accountAuthContext, result checkinResult, startedAt string, finishedAt string) error {
	_, err := a.db.ExecContext(ctx, `
		INSERT INTO checkin_logs (id, account_id, upstream_site_id, channel_id, status, reward, message, raw_response_masked, started_at, finished_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, newID(), auth.AccountID, auth.UpstreamSiteID, auth.ChannelID, result.Status, result.Reward, result.Message, result.RawResponseMasked, startedAt, finishedAt)
	if err != nil {
		return err
	}
	_, err = a.db.ExecContext(ctx, `
		UPDATE channel_accounts
		SET last_checkin_at=?, last_checkin_status=?, updated_at=?
		WHERE id=?
	`, finishedAt, result.Status, now(), auth.AccountID)
	if err == nil {
		level := "info"
		title := "签到完成"
		if result.Status == "success" || result.Status == "already_checked" {
			level = "success"
		} else if result.Status == "auth_expired" || result.Status == "manual_required" {
			level = "warning"
			title = "需要重新登录"
		} else if result.Status != "unsupported" {
			level = "error"
			title = "签到失败"
		}
		a.notify("checkin_"+result.Status, level, title, auth.AccountName+"： "+result.Message, "account", auth.AccountID)
	}
	return err
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
	writeJSON(w, http.StatusOK, items)
}

func (a *App) refreshAccountBalance(ctx context.Context, id string) (balanceResult, error) {
	auth, err := a.loadAccountAuth(ctx, id)
	if err != nil {
		return balanceResult{}, err
	}
	if !auth.SupportsBalance {
		return balanceResult{Unit: "unknown", HTTPStatus: 0, Path: "", RawResponseMasked: "", Balance: nil}, errorsText("该站点未探测到余额接口。")
	}
	if err := a.ensureAccountSession(ctx, &auth); err != nil && auth.Cookie == "" && auth.AccessToken == "" && auth.APIKey == "" {
		return balanceResult{Unit: "unknown"}, fmt.Errorf("账号密码登录失败：%w", err)
	}

	var lastErr error
	for _, path := range balanceCandidates {
		status, body, err := a.callAccountAPI(ctx, auth, http.MethodGet, path, nil)
		if err != nil {
			lastErr = err
			continue
		}
		if status == http.StatusNotFound || status == http.StatusMethodNotAllowed {
			continue
		}
		if status == http.StatusUnauthorized || status == http.StatusForbidden {
			lastErr = fmt.Errorf("%s 登录态不可用：HTTP %d", path, status)
			continue
		}
		if status < 200 || status >= 300 {
			lastErr = fmt.Errorf("%s 返回 HTTP %d", path, status)
			continue
		}
		result := parseBalance(body)
		result.HTTPStatus = status
		result.Path = path
		result.RawResponseMasked = maskResponse(body)
		if result.Balance == nil && result.UsedQuota == nil && result.TotalQuota == nil {
			lastErr = fmt.Errorf("%s 未解析到余额字段", path)
			continue
		}
		if err := a.saveBalanceResult(ctx, auth, result); err != nil {
			return result, err
		}
		return result, nil
	}
	if lastErr != nil {
		return balanceResult{Unit: "unknown"}, lastErr
	}
	return balanceResult{Unit: "unknown"}, errorsText("未找到可用余额接口。")
}

func (a *App) saveBalanceResult(ctx context.Context, auth accountAuthContext, result balanceResult) error {
	var balanceValue interface{}
	if result.Balance != nil {
		balanceValue = *result.Balance
	}
	var usedValue interface{}
	if result.UsedQuota != nil {
		usedValue = *result.UsedQuota
	}
	var totalValue interface{}
	if result.TotalQuota != nil {
		totalValue = *result.TotalQuota
	}
	createdAt := now()
	_, err := a.db.ExecContext(ctx, `
		INSERT INTO balance_snapshots (id, account_id, upstream_site_id, channel_id, balance, used_quota, total_quota, unit, raw_response_masked, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, newID(), auth.AccountID, auth.UpstreamSiteID, auth.ChannelID, balanceValue, usedValue, totalValue, result.Unit, result.RawResponseMasked, createdAt)
	if err != nil {
		return err
	}
	_, err = a.db.ExecContext(ctx, `
		UPDATE channel_accounts
		SET balance=?, balance_unit=?, last_validated_at=?, login_status='valid', updated_at=?
		WHERE id=?
	`, balanceValue, result.Unit, createdAt, now(), auth.AccountID)
	if err == nil {
		a.notify("balance_refreshed", "success", "余额已刷新", auth.AccountName+" 余额信息已更新。", "account", auth.AccountID)
	}
	return err
}

func (a *App) loadAccountAuth(ctx context.Context, id string) (accountAuthContext, error) {
	var auth accountAuthContext
	var email, username, cookieEncrypted, accessEncrypted, apiKeyEncrypted, passwordEncrypted, loginURL, checkinConfigJSON string
	var supportsCheckin, supportsBalance int
	err := a.db.QueryRowContext(ctx, `
		SELECT a.id, a.display_name, s.id, s.name, COALESCE(s.channel_id,''), s.base_url,
		       COALESCE(s.login_url,''), COALESCE(a.user_agent,''), COALESCE(a.email,''), COALESCE(a.username,''),
		       COALESCE(a.password_encrypted,''), COALESCE(a.cookie_encrypted,''),
		       COALESCE(a.access_token_encrypted,''), COALESCE(a.api_key_encrypted,''),
		       COALESCE(a.auth_user_id,''), s.supports_checkin, s.supports_balance,
		       COALESCE(s.checkin_config_json,'')
		FROM channel_accounts a
		JOIN upstream_sites s ON s.id = a.upstream_site_id
		WHERE a.id = ?
	`, id).Scan(&auth.AccountID, &auth.AccountName, &auth.UpstreamSiteID, &auth.UpstreamSite, &auth.ChannelID, &auth.BaseURL, &loginURL, &auth.UserAgent, &email, &username, &passwordEncrypted, &cookieEncrypted, &accessEncrypted, &apiKeyEncrypted, &auth.AuthUserID, &supportsCheckin, &supportsBalance, &checkinConfigJSON)
	if err == sql.ErrNoRows {
		return auth, errorsText("账号不存在。")
	}
	if err != nil {
		return auth, err
	}
	auth.LoginName = firstNonEmpty(email, username)
	auth.LoginPath = pathFromMaybeURL(loginURL)
	auth.Password, _ = a.decryptText(passwordEncrypted)
	auth.Cookie, _ = a.decryptText(cookieEncrypted)
	auth.AccessToken, _ = a.decryptText(accessEncrypted)
	auth.APIKey, _ = a.decryptText(apiKeyEncrypted)
	auth.SupportsCheckin = supportsCheckin == 1
	auth.SupportsBalance = supportsBalance == 1
	auth.CheckinRules = parseCheckinRules(checkinConfigJSON)
	if len(auth.CheckinRules) > 0 {
		auth.SupportsCheckin = true
	}
	return auth, nil
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
	if auth.Cookie != "" || auth.APIKey != "" || (auth.AccessToken != "" && auth.AuthUserID != "") {
		return nil
	}
	if auth.LoginName == "" || auth.Password == "" {
		return errorsText("没有可用的 Cookie、Token 或账号密码。")
	}
	return a.loginWithPassword(ctx, auth)
}

func (a *App) loginWithPassword(ctx context.Context, auth *accountAuthContext) error {
	payloads := []map[string]string{
		{"username": auth.LoginName, "password": auth.Password},
		{"email": auth.LoginName, "password": auth.Password},
		{"account": auth.LoginName, "password": auth.Password},
	}
	loginPaths := candidateLoginPaths(auth.LoginPath)
	var lastErr error
	pathFailures := []string{}
	for _, loginPath := range loginPaths {
		var pathErr error
		for _, payload := range payloads {
			body, _ := json.Marshal(payload)
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, normalizeBaseURL(auth.BaseURL)+loginPath, bytes.NewReader(body))
			if err != nil {
				return err
			}
			req.Header.Set("content-type", "application/json")
			req.Header.Set("accept", "application/json, text/plain, */*")
			req.Header.Set("user-agent", firstNonEmpty(auth.UserAgent, "RelayCheck-Desktop/0.1"))
			resp, err := a.doHTTP(req)
			if err != nil {
				lastErr = err
				pathErr = err
				continue
			}
			content, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
			cookies := cookiesToHeader(resp.Cookies())
			_ = resp.Body.Close()
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				lastErr = fmt.Errorf("%s HTTP %d: %s", loginPath, resp.StatusCode, firstNonEmpty(extractMessage(string(content)), maskResponse(string(content))))
				pathErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, firstNonEmpty(extractMessage(string(content)), maskResponse(string(content))))
				continue
			}
			if responseExplicitlyFailed(string(content)) {
				lastErr = errorsText(firstNonEmpty(extractMessage(string(content)), "登录失败。"))
				pathErr = lastErr
				continue
			}
			accessToken := extractToken(string(content))
			authUserID := extractUserID(string(content))
			if cookies == "" && accessToken == "" {
				lastErr = fmt.Errorf("%s 未返回 Cookie 或 Token", loginPath)
				pathErr = errorsText("未返回 Cookie 或 Token")
				continue
			}
			if err := a.saveAccountSession(ctx, auth, cookies, accessToken, authUserID); err != nil {
				return err
			}
			return nil
		}
		if pathErr != nil {
			pathFailures = append(pathFailures, formatLoginPathFailure(loginPath, pathErr))
		}
	}
	if len(pathFailures) > 0 {
		return loginFailuresError(pathFailures)
	}
	if lastErr != nil {
		return lastErr
	}
	return errorsText("登录接口不可用。")
}

func candidateLoginPaths(customPath string) []string {
	paths := []string{}
	if strings.Contains(customPath, "/api/") {
		paths = append(paths, customPath)
	}
	paths = append(paths, "/api/user/login", "/api/login", "/api/auth/login")
	seen := map[string]bool{}
	result := []string{}
	for _, path := range paths {
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		result = append(result, path)
	}
	return result
}

func formatLoginPathFailure(path string, err error) string {
	message := "未知错误"
	if err != nil {
		message = err.Error()
	}
	message = strings.Join(strings.Fields(message), " ")
	if len(message) > 120 {
		message = message[:120] + "..."
	}
	return fmt.Sprintf("%s %s", path, message)
}

func loginFailuresError(pathFailures []string) error {
	return fmt.Errorf("登录接口全部失败：%s；建议在账号卡片修正站点登录地址，或改用网页登录授权保存会话。", strings.Join(pathFailures, "；"))
}

func responseExplicitlyFailed(body string) bool {
	var payload map[string]interface{}
	if json.Unmarshal([]byte(body), &payload) != nil {
		return false
	}
	for _, key := range []string{"success", "ok"} {
		if value, exists := payload[key]; exists {
			if boolValue, ok := value.(bool); ok {
				return !boolValue
			}
		}
	}
	if value, exists := payload["code"]; exists {
		if code, ok := toFloat(value); ok {
			return code != 0
		}
	}
	return false
}

func (a *App) saveAccountSession(ctx context.Context, auth *accountAuthContext, cookie string, accessToken string, authUserID string) error {
	cookieEncrypted, err := a.encryptText(cookie)
	if err != nil {
		return err
	}
	accessEncrypted, err := a.encryptText(accessToken)
	if err != nil {
		return err
	}
	_, err = a.db.ExecContext(ctx, `
		UPDATE channel_accounts
		SET cookie_encrypted=CASE WHEN ? <> '' THEN ? ELSE cookie_encrypted END,
		    access_token_encrypted=CASE WHEN ? <> '' THEN ? ELSE access_token_encrypted END,
		    auth_user_id=CASE WHEN ? <> '' THEN ? ELSE auth_user_id END,
		    login_status='valid',
		    last_login_at=?,
		    last_validated_at=?,
		    updated_at=?
		WHERE id=?
	`, cookie, cookieEncrypted, accessToken, accessEncrypted, authUserID, authUserID, now(), now(), now(), auth.AccountID)
	if err != nil {
		return err
	}
	if cookie != "" {
		auth.Cookie = cookie
	}
	if accessToken != "" {
		auth.AccessToken = accessToken
	}
	if authUserID != "" {
		auth.AuthUserID = authUserID
	}
	return nil
}

func cookiesToHeader(cookies []*http.Cookie) string {
	parts := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		if cookie.Name != "" {
			parts = append(parts, cookie.Name+"="+cookie.Value)
		}
	}
	return strings.Join(parts, "; ")
}

func extractToken(body string) string {
	var payload interface{}
	if json.Unmarshal([]byte(body), &payload) != nil {
		return ""
	}
	for _, key := range []string{"access_token", "accessToken", "token", "session_token"} {
		if value := findString(payload, key); value != "" {
			return value
		}
	}
	return ""
}

func extractUserID(body string) string {
	var payload interface{}
	if json.Unmarshal([]byte(body), &payload) != nil {
		return ""
	}
	if root, ok := payload.(map[string]interface{}); ok {
		if data, ok := root["data"]; ok {
			if found := findNumber(data, "id", "user_id", "userId"); found != nil {
				return fmt.Sprintf("%.0f", *found)
			}
		}
	}
	for _, key := range []string{"user_id", "userId", "id"} {
		if found := findNumber(payload, key); found != nil {
			return fmt.Sprintf("%.0f", *found)
		}
	}
	return ""
}

func (a *App) callAccountAPI(ctx context.Context, auth accountAuthContext, method string, path string, body []byte) (int, string, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, normalizeBaseURL(auth.BaseURL)+path, reader)
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("user-agent", firstNonEmpty(auth.UserAgent, "RelayCheck-Desktop/0.1"))
	req.Header.Set("accept", "application/json, text/plain, */*")
	if body != nil {
		req.Header.Set("content-type", "application/json")
	}
	if auth.Cookie != "" {
		req.Header.Set("cookie", auth.Cookie)
	}
	if auth.AuthUserID != "" {
		req.Header.Set("New-Api-User", auth.AuthUserID)
	}
	if token := firstNonEmpty(auth.AccessToken, auth.APIKey); token != "" {
		if !strings.HasPrefix(strings.ToLower(token), "bearer ") {
			token = "Bearer " + token
		}
		req.Header.Set("authorization", token)
	}
	resp, err := a.doHTTP(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	content, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	return resp.StatusCode, string(content), nil
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
