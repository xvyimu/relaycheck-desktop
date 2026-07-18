package core

import (
	"context"
	"database/sql"
	"log"
)

type CheckinTaskService struct {
	db               *sql.DB
	rootCtx          context.Context
	taskRunner       *TaskRunner
	accountAuth      *AccountAuthRepository
	checkinExecutor  *CheckinExecutor
	checkinRun       *CheckinRunStore
	balanceRefresher *BalanceRefresher
	scheduleConfig   func(context.Context) checkinScheduleConfig
}

func NewCheckinTaskService(app *App) *CheckinTaskService {
	return &CheckinTaskService{
		db:               app.db,
		rootCtx:          app.rootCtx,
		taskRunner:       app.taskRunner,
		accountAuth:      app.accountAuth,
		checkinExecutor:  app.checkinExecutor,
		checkinRun:       app.checkinRun,
		balanceRefresher: app.balanceRefresher,
		scheduleConfig:   app.loadCheckinScheduleConfig,
	}
}

func (s *CheckinTaskService) StartCheckin(taskID string, accounts []checkinRunAccount) error {
	if !s.checkinRun.begin("manual.task", len(accounts)) {
		return errCheckinRunBusy
	}
	task, taskCtx := s.taskRunner.start(taskID, TaskCheckin, len(accounts))
	go func() {
		defer s.checkinRun.finish()
		if len(accounts) == 0 {
			task.finish(nil)
			return
		}

		siteLimiter := newCheckinSiteLimiter(s.scheduleConfig(taskCtx))
		accountIDs := checkinRunAccountIDs(accounts)
		auths, err := s.accountAuth.LoadBatch(taskCtx, accountIDs)
		if err != nil {
			auths = map[string]accountAuthContext{}
		}
		for _, account := range accounts {
			if taskCtx.Err() != nil {
				task.finish(taskCtx.Err())
				return
			}
			if err := siteLimiter.wait(taskCtx, account.UpstreamSiteID); err != nil {
				task.finish(err)
				return
			}
			s.checkinRun.updateCurrent(account.ID, account.AccountName, account.SiteName, "正在签到...")
			var auth *accountAuthContext
			if loaded, ok := auths[account.ID]; ok {
				auth = &loaded
			}
			result, err := s.checkinExecutor.Run(taskCtx, account.ID, auth)
			item := ItemResult{ID: account.ID, Name: account.AccountName}
			if err != nil {
				item.Status = "failed"
				item.Message = publicOperationFailure("checkin", "task-run", account.ID, "签到失败，请稍后重试。", err)
			} else {
				item.Status = result.Status
				item.Message = result.Message
			}
			s.checkinRun.recordResult(item.Status, item.Message)
			task.update(item)
		}
		task.finish(nil)
	}()
	return nil
}

func (s *CheckinTaskService) StartRefreshBalances(taskID string, params map[string]interface{}) {
	go func() {
		ctx := s.rootContext()
		limit := 50
		missingOnly := false
		if l, ok := params["limit"].(float64); ok && l > 0 {
			limit = int(l)
		}
		if m, ok := params["missingOnly"].(bool); ok {
			missingOnly = m
		}

		jobs, err := s.loadBalanceRefreshJobs(ctx, limit, missingOnly)
		if err != nil {
			task, _ := s.taskRunner.start(taskID, TaskRefreshBalances, 0)
			task.finish(err)
			return
		}

		task, taskCtx := s.taskRunner.start(taskID, TaskRefreshBalances, len(jobs))
		jobIDs := balanceRefreshTaskJobIDs(jobs)
		auths, _ := s.accountAuth.LoadBatch(ctx, jobIDs)
		for _, job := range jobs {
			if taskCtx.Err() != nil {
				task.finish(taskCtx.Err())
				return
			}
			var auth *accountAuthContext
			if loaded, ok := auths[job.ID]; ok {
				auth = &loaded
			}
			item := s.refreshBalanceForTask(taskCtx, job.ID, auth)
			result := ItemResult{ID: job.ID, Name: job.Name, Status: item.Status}
			if item.Status != "success" {
				result.Message = item.Message
			}
			task.update(result)
		}
		task.finish(nil)
	}()
}

type balanceRefreshTaskJob struct {
	ID   string
	Name string
}

func (s *CheckinTaskService) loadBalanceRefreshJobs(ctx context.Context, limit int, missingOnly bool) ([]balanceRefreshTaskJob, error) {
	query := `
		SELECT a.id, COALESCE(a.display_name, a.username, a.id)
		FROM channel_accounts a
		JOIN upstream_sites s ON s.id = a.upstream_site_id
		WHERE s.supports_balance = 1
	`
	if missingOnly {
		query += ` AND a.balance IS NULL`
	}
	query += ` ORDER BY COALESCE(a.last_validated_at,''), a.updated_at DESC LIMIT ?`

	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := []balanceRefreshTaskJob{}
	for rows.Next() {
		var job balanceRefreshTaskJob
		if err := rows.Scan(&job.ID, &job.Name); err != nil {
			log.Printf("[task:refresh-balances] scan failed: %v", err)
			continue
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[task:refresh-balances] query iteration failed: %v", err)
	}
	return jobs, nil
}

func (s *CheckinTaskService) refreshBalanceForTask(ctx context.Context, id string, auth *accountAuthContext) bulkBalanceRefreshItem {
	item := bulkBalanceRefreshItem{AccountID: id, Status: "failed"}
	if auth != nil {
		item.AccountName = auth.AccountName
		item.SiteName = auth.UpstreamSite
	}
	result, err := s.balanceRefresher.Run(ctx, id, auth)
	if err != nil {
		item.Message = publicOperationFailure("balance", "task-refresh", id, "余额刷新失败，请稍后重试。", err)
		return item
	}
	item.Status = "success"
	item.Message = "余额已刷新。"
	item.Path = result.Path
	item.Balance = formatBalanceForMessage(result.Balance, result.Unit)
	return item
}

func (s *CheckinTaskService) rootContext() context.Context {
	if s.rootCtx != nil {
		return s.rootCtx
	}
	return context.Background()
}

func checkinRunAccountIDs(accounts []checkinRunAccount) []string {
	ids := make([]string, 0, len(accounts))
	for _, account := range accounts {
		ids = append(ids, account.ID)
	}
	return ids
}

func balanceRefreshTaskJobIDs(jobs []balanceRefreshTaskJob) []string {
	ids := make([]string, 0, len(jobs))
	for _, job := range jobs {
		ids = append(ids, job.ID)
	}
	return ids
}
