package core

import (
	"context"
	"database/sql"
	"fmt"
	"log"
)

type bulkPasswordLoginBatchResult struct {
	Processed int                       `json:"processed"`
	Success   int                       `json:"success"`
	Failed    int                       `json:"failed"`
	Results   []bulkPasswordLoginResult `json:"results"`
}

type bulkBrowserOpenBatchResult struct {
	Processed int                      `json:"processed"`
	Opened    int                      `json:"opened"`
	Failed    int                      `json:"failed"`
	Results   []browserLoginOpenResult `json:"results"`
}

type bulkBrowserSaveBatchResult struct {
	Processed int                      `json:"processed"`
	Saved     int                      `json:"saved"`
	Failed    int                      `json:"failed"`
	Results   []browserLoginSaveResult `json:"results"`
}

type AccountLoginBatchService struct {
	db            *sql.DB
	sessions      *BrowserSessionStore
	loadAuth      func(context.Context, string) (accountAuthContext, error)
	loadAuths     func(context.Context, []string) (map[string]accountAuthContext, error)
	passwordLogin func(context.Context, *accountAuthContext) error
	openBrowser   func(context.Context, string, *accountAuthContext) browserLoginOpenResult
	saveBrowser   func(context.Context, string, *accountAuthContext) browserLoginSaveResult
	notify        func(kind, level, title, content, relatedType, relatedID string)
}

func NewAccountLoginBatchService(app *App) *AccountLoginBatchService {
	return &AccountLoginBatchService{
		db:            app.db,
		sessions:      app.browserSessions,
		loadAuth:      app.loadAccountAuth,
		loadAuths:     app.loadAccountAuths,
		passwordLogin: app.accountSession.LoginWithPassword,
		openBrowser:   app.browserLogin.Open,
		saveBrowser:   app.browserLogin.Save,
		notify:        app.notify,
	}
}

func (s *AccountLoginBatchService) PasswordLogin(ctx context.Context, limit int) (bulkPasswordLoginBatchResult, error) {
	limit = clampBatchLimit(limit, 10)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id FROM channel_accounts
		WHERE COALESCE(password_encrypted,'') <> ''
		  AND (
		    login_status IN ('expired','manual_required','unknown')
		    OR COALESCE(last_checkin_status,'') IN ('auth_expired','manual_required','failed')
		  )
		ORDER BY updated_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return bulkPasswordLoginBatchResult{}, err
	}
	defer rows.Close()

	accountIDs := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			log.Printf("[accounts] bulk open browser scan failed: %v", err)
			continue
		}
		accountIDs = append(accountIDs, id)
	}
	if err := rows.Err(); err != nil {
		return bulkPasswordLoginBatchResult{}, err
	}

	results := []bulkPasswordLoginResult{}
	auths, _ := s.loadAuths(ctx, accountIDs)
	for _, id := range accountIDs {
		var auth *accountAuthContext
		if loaded, ok := auths[id]; ok {
			auth = &loaded
		}
		results = append(results, s.RetryPasswordLogin(ctx, id, auth))
	}

	successCount := 0
	for _, result := range results {
		if result.Status == "valid" {
			successCount++
		}
	}
	if len(results) > 0 {
		s.notify("bulk_password_login", "info", "批量密码重登完成", fmt.Sprintf("处理 %d 个账号，成功 %d 个。", len(results), successCount), "account", "")
	}
	return bulkPasswordLoginBatchResult{
		Processed: len(results),
		Success:   successCount,
		Failed:    len(results) - successCount,
		Results:   results,
	}, nil
}

func (s *AccountLoginBatchService) RetryPasswordLogin(ctx context.Context, id string, auth *accountAuthContext) bulkPasswordLoginResult {
	if auth == nil {
		loaded, err := s.loadAuth(ctx, id)
		if err != nil {
			return bulkPasswordLoginResult{AccountID: id, Status: "failed", Message: err.Error()}
		}
		auth = &loaded
	}
	result := bulkPasswordLoginResult{
		AccountID:   auth.AccountID,
		AccountName: auth.AccountName,
		SiteName:    auth.UpstreamSite,
	}
	if auth.LoginName == "" || auth.Password == "" {
		result.Status = "manual_required"
		result.Message = "没有可用账号密码，请网页登录授权。"
		if _, execErr := s.db.ExecContext(ctx, `UPDATE channel_accounts SET login_status='manual_required', last_validated_at=?, updated_at=? WHERE id=?`, now(), now(), id); execErr != nil {
			log.Printf("[accounts] password login status update to manual_required failed for account %s: %v", id, execErr)
		}
		return result
	}
	auth.Cookie = ""
	auth.AccessToken = ""
	auth.AuthUserID = ""
	if err := s.passwordLogin(ctx, auth); err != nil {
		result.Status = "expired"
		result.Message = err.Error()
		if _, execErr := s.db.ExecContext(ctx, `UPDATE channel_accounts SET login_status='expired', last_validated_at=?, updated_at=? WHERE id=?`, now(), now(), id); execErr != nil {
			log.Printf("[accounts] password login status update to expired failed for account %s: %v", id, execErr)
		}
		return result
	}
	result.Status = "valid"
	result.Message = "密码登录成功，已保存新会话。"
	return result
}

func (s *AccountLoginBatchService) OpenBrowser(ctx context.Context, limit int, ids []string) (bulkBrowserOpenBatchResult, error) {
	limit = clampBatchLimit(limit, 5)
	accountIDs := ids
	if len(accountIDs) > limit {
		accountIDs = accountIDs[:limit]
	}
	if len(accountIDs) == 0 {
		rows, err := s.db.QueryContext(ctx, `
			SELECT id FROM channel_accounts
			WHERE login_status IN ('expired','manual_required','unknown')
			   OR COALESCE(last_checkin_status,'') IN ('auth_expired','manual_required','failed')
			ORDER BY updated_at DESC
			LIMIT ?
		`, limit)
		if err != nil {
			return bulkBrowserOpenBatchResult{}, err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err == nil {
				accountIDs = append(accountIDs, id)
			}
		}
		if err := rows.Err(); err != nil {
			return bulkBrowserOpenBatchResult{}, err
		}
	}

	results := []browserLoginOpenResult{}
	opened := 0
	auths, _ := s.loadAuths(ctx, accountIDs)
	for _, id := range accountIDs {
		var auth *accountAuthContext
		if loaded, ok := auths[id]; ok {
			auth = &loaded
		}
		result := s.openBrowser(ctx, id, auth)
		if result.Status == "opened" || result.Status == "already_open" {
			opened++
		}
		results = append(results, result)
	}
	return bulkBrowserOpenBatchResult{
		Processed: len(results),
		Opened:    opened,
		Failed:    len(results) - opened,
		Results:   results,
	}, nil
}

func (s *AccountLoginBatchService) FinishBrowser(ctx context.Context, ids []string) (bulkBrowserSaveBatchResult, error) {
	accountIDs := ids
	if len(accountIDs) == 0 {
		s.sessions.Range(func(id string, _ BrowserLoginSession) {
			accountIDs = append(accountIDs, id)
		})
	}
	if len(accountIDs) > 10 {
		accountIDs = accountIDs[:10]
	}

	results := []browserLoginSaveResult{}
	saved := 0
	auths, _ := s.loadAuths(ctx, accountIDs)
	for _, id := range accountIDs {
		var auth *accountAuthContext
		if loaded, ok := auths[id]; ok {
			auth = &loaded
		}
		result := s.saveBrowser(ctx, id, auth)
		if result.Status == "saved" {
			saved++
		}
		results = append(results, result)
	}
	return bulkBrowserSaveBatchResult{
		Processed: len(results),
		Saved:     saved,
		Failed:    len(results) - saved,
		Results:   results,
	}, nil
}
