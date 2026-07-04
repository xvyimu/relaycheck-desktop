package core

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

type CheckinBatchOrchestrator struct {
	db              *sql.DB
	accountAuth     *AccountAuthRepository
	checkinRun      *CheckinRunStore
	checkinExecutor *CheckinExecutor
	scheduleConfig  func(context.Context) checkinScheduleConfig
}

func NewCheckinBatchOrchestrator(app *App) *CheckinBatchOrchestrator {
	return &CheckinBatchOrchestrator{
		db:              app.db,
		accountAuth:     app.accountAuth,
		checkinRun:      app.checkinRun,
		checkinExecutor: app.checkinExecutor,
		scheduleConfig:  app.loadCheckinScheduleConfig,
	}
}

func (o *CheckinBatchOrchestrator) Run(ctx context.Context, mode string) ([]map[string]interface{}, error) {
	results, err := o.runWithFilter(ctx, mode, "", "正在签到...", "今天没有待签到账号。")
	if err == errCheckinRunBusy {
		return nil, errorsText("已有签到任务正在运行，请等待当前任务完成。")
	}
	return results, err
}

func (o *CheckinBatchOrchestrator) RunForSite(ctx context.Context, siteID string) ([]map[string]interface{}, error) {
	return o.runWithFilter(ctx, "channel."+siteID, siteID, "正在签到(独立排程)...", "")
}

func (o *CheckinBatchOrchestrator) runWithFilter(ctx context.Context, mode string, siteID string, currentMessage string, emptyMessage string) ([]map[string]interface{}, error) {
	accounts, err := o.LoadDueAccounts(ctx, siteID, 0)
	if err != nil {
		return nil, err
	}
	if siteID != "" && len(accounts) == 0 {
		return nil, nil
	}
	if !o.checkinRun.begin(mode, len(accounts)) {
		return nil, errCheckinRunBusy
	}
	defer o.checkinRun.finish()

	results := make([]map[string]interface{}, 0, len(accounts))
	if len(accounts) == 0 && emptyMessage != "" {
		o.checkinRun.updateMessage(emptyMessage)
	}
	siteLimiter := newCheckinSiteLimiter(o.scheduleConfig(ctx))
	accountIDs := make([]string, 0, len(accounts))
	for _, account := range accounts {
		accountIDs = append(accountIDs, account.ID)
	}
	auths, _ := o.accountAuth.LoadBatch(ctx, accountIDs)
	for _, account := range accounts {
		if ctx.Err() != nil {
			break
		}
		if err := siteLimiter.wait(ctx, account.UpstreamSiteID); err != nil {
			return results, err
		}
		o.checkinRun.updateCurrent(account.ID, account.AccountName, account.SiteName, currentMessage)
		var auth *accountAuthContext
		if loaded, ok := auths[account.ID]; ok {
			auth = &loaded
		}
		result, err := o.checkinExecutor.Run(ctx, account.ID, auth)
		entry := map[string]interface{}{
			"accountId":   account.ID,
			"accountName": account.AccountName,
			"siteName":    account.SiteName,
		}
		if err != nil {
			entry["status"] = "failed"
			entry["message"] = err.Error()
			o.checkinRun.recordResult("failed", err.Error())
		} else {
			entry["status"] = result.Status
			entry["message"] = result.Message
			entry["path"] = result.Path
			o.checkinRun.recordResult(result.Status, result.Message)
		}
		results = append(results, entry)
	}
	return results, nil
}

func (o *CheckinBatchOrchestrator) LoadDueAccounts(ctx context.Context, siteID string, limit int) ([]checkinRunAccount, error) {
	query := `
		SELECT a.id, a.display_name, s.id, s.name
		FROM channel_accounts a
		JOIN upstream_sites s ON s.id = a.upstream_site_id
		WHERE (COALESCE(a.last_checkin_status,'') NOT IN ('success','already_checked')
		   OR COALESCE(substr(a.last_checkin_at, 1, 10),'') <> ?)
	`
	args := []interface{}{todayCST()}
	if siteID != "" {
		query += ` AND a.upstream_site_id = ?`
		args = append(args, siteID)
	}
	query += ` ORDER BY a.updated_at DESC`
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := o.db.QueryContext(ctx, query, args...)
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
