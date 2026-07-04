package core

import (
	"context"
	"database/sql"
	"log"
)

type AccountTaskService struct {
	db           *sql.DB
	rootCtx      context.Context
	taskRunner   *TaskRunner
	accountAuth  *AccountAuthRepository
	apiKeyTester func(context.Context, string, *accountAuthContext) apiKeyTestResult
}

func NewAccountTaskService(app *App) *AccountTaskService {
	return &AccountTaskService{
		db:           app.db,
		rootCtx:      app.rootCtx,
		taskRunner:   app.taskRunner,
		accountAuth:  app.accountAuth,
		apiKeyTester: app.testAPIKeyForAccount,
	}
}

func (s *AccountTaskService) StartTestKeys(taskID string, params map[string]interface{}) {
	go func() {
		ctx := s.rootContext()
		limit := 50
		if l, ok := params["limit"].(float64); ok && l > 0 {
			limit = int(l)
		}

		jobs, err := s.loadAPIKeyTestJobs(ctx, limit)
		if err != nil {
			task, _ := s.taskRunner.start(taskID, TaskTestKeys, 0)
			task.finish(err)
			return
		}

		task, taskCtx := s.taskRunner.start(taskID, TaskTestKeys, len(jobs))
		jobIDs := apiKeyTestTaskJobIDs(jobs)
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
			result := s.apiKeyTester(taskCtx, job.ID, auth)
			item := ItemResult{ID: job.ID, Name: job.Name, Status: result.Status}
			if result.Status != "valid" {
				item.Message = result.Message
			}
			task.update(item)
		}
		task.finish(nil)
	}()
}

type apiKeyTestTaskJob struct {
	ID   string
	Name string
}

func (s *AccountTaskService) loadAPIKeyTestJobs(ctx context.Context, limit int) ([]apiKeyTestTaskJob, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, COALESCE(display_name, username, id)
		FROM channel_accounts
		WHERE COALESCE(api_key_encrypted,'') <> ''
		ORDER BY COALESCE(api_key_last_checked_at,''), updated_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := []apiKeyTestTaskJob{}
	for rows.Next() {
		var job apiKeyTestTaskJob
		if err := rows.Scan(&job.ID, &job.Name); err != nil {
			log.Printf("[task:test-keys] scan failed: %v", err)
			continue
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[task:test-keys] query iteration failed: %v", err)
	}
	return jobs, nil
}

func (s *AccountTaskService) rootContext() context.Context {
	if s.rootCtx != nil {
		return s.rootCtx
	}
	return context.Background()
}

func apiKeyTestTaskJobIDs(jobs []apiKeyTestTaskJob) []string {
	ids := make([]string, 0, len(jobs))
	for _, job := range jobs {
		ids = append(ids, job.ID)
	}
	return ids
}
