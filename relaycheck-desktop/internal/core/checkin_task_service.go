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
	checkinBatch     *CheckinBatchOrchestrator
	checkinExecutor  *CheckinExecutor
	balanceRefresher *BalanceRefresher
	scheduleConfig   func(context.Context) checkinScheduleConfig
}

func NewCheckinTaskService(app *App) *CheckinTaskService {
	return &CheckinTaskService{
		db:               app.db,
		rootCtx:          app.rootCtx,
		taskRunner:       app.taskRunner,
		accountAuth:      app.accountAuth,
		checkinBatch:     app.checkinBatch,
		checkinExecutor:  app.checkinExecutor,
		balanceRefresher: app.balanceRefresher,
		scheduleConfig:   app.loadCheckinScheduleConfig,
	}
}

func (s *CheckinTaskService) StartCheckin(taskID string) {
	go func() {
		ctx := s.rootContext()
		accounts, err := s.checkinBatch.LoadDueAccounts(ctx, "", 0)
		if err != nil {
			task, _ := s.taskRunner.start(taskID, TaskCheckin, 0)
			task.finish(err)
			return
		}
		total := len(accounts)
		task, taskCtx := s.taskRunner.start(taskID, TaskCheckin, total)

		if total == 0 {
			task.finish(nil)
			return
		}

		siteLimiter := newCheckinSiteLimiter(s.scheduleConfig(ctx))
		accountIDs := checkinRunAccountIDs(accounts)
		auths, _ := s.accountAuth.LoadBatch(ctx, accountIDs)
		for _, account := range accounts {
			if taskCtx.Err() != nil {
				task.finish(taskCtx.Err())
				return
			}
			_ = siteLimiter.wait(taskCtx, account.UpstreamSiteID)
			var auth *accountAuthContext
			if loaded, ok := auths[account.ID]; ok {
				auth = &loaded
			}
			result, err := s.checkinExecutor.Run(taskCtx, account.ID, auth)
			item := ItemResult{ID: account.ID, Name: account.AccountName}
			if err != nil {
				item.Status = "failed"
				item.Message = err.Error()
			} else {
				item.Status = result.Status
				item.Message = result.Message
			}
			task.update(item)
		}
		task.finish(nil)
	}()
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
		item.Message = err.Error()
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
