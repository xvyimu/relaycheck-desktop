package core

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"

	"relaycheck-desktop/internal/capabilities"
)

type CheckinExecutor struct {
	db             *sql.DB
	accountAuth    *AccountAuthRepository
	accountAPI     *AccountAPIClient
	accountSession *AccountSessionService
	notify         func(string, string, string, string, string, string)
}

func NewCheckinExecutor(app *App) *CheckinExecutor {
	return &CheckinExecutor{
		db:             app.db,
		accountAuth:    app.accountAuth,
		accountAPI:     app.accountAPI,
		accountSession: app.accountSession,
		notify:         app.notify,
	}
}

func (e *CheckinExecutor) Run(ctx context.Context, id string, auth *accountAuthContext) (checkinResult, error) {
	if auth == nil {
		loaded, err := e.accountAuth.Load(ctx, id)
		if err != nil {
			return checkinResult{}, err
		}
		auth = loaded
	}
	if !auth.SupportsCheckin {
		result := checkinResult{Status: "unsupported", Message: "该站点未探测到签到接口。"}
		if err := e.SaveResult(ctx, *auth, result, now(), now()); err != nil {
			log.Printf("[checkin] save result failed for account %s: %v", id, err)
		}
		return result, nil
	}
	if !canAttemptCheckin(*auth) {
		result := checkinResult{Status: "auth_expired", Message: "缺少可用的本地认证信息，请重新登录或补充凭据。"}
		if err := e.SaveResult(ctx, *auth, result, now(), now()); err != nil {
			log.Printf("[checkin] save result failed for account %s: %v", id, err)
		}
		return result, nil
	}
	if err := e.accountSession.Ensure(ctx, auth); err != nil && auth.Cookie == "" && auth.AccessToken == "" && auth.APIKey == "" {
		result := checkinResult{Status: "auth_expired", Message: publicOperationFailure("checkin", "ensure-session", id, "账号登录状态不可用，请重新登录。", err)}
		if err := e.SaveResult(ctx, *auth, result, now(), now()); err != nil {
			log.Printf("[checkin] save result failed for account %s: %v", id, err)
		}
		return result, nil
	}

	startedAt := now()
	lastUnsupported := checkinResult{Status: "unsupported", Message: "未找到可用签到接口。"}
	candidates := capabilities.CheckinCandidatesForKind(auth.SiteKind, auth.CheckinRules)
	for _, candidate := range candidates {
		status, body, retries, err := e.callAPIWithRetry(ctx, *auth, candidate)
		if err != nil {
			message := publicOperationFailure("checkin", "request", id, "签到请求失败，请稍后重试。", err)
			lastUnsupported = annotateCheckinRetry(checkinResult{Status: "failed", Message: message, Path: candidate.Path, RetryCount: retries})
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
			if loginErr := e.accountSession.LoginWithPassword(ctx, auth); loginErr != nil {
				result.Message = publicOperationFailure("checkin", "password-login", id, "账号登录状态不可用，请重新登录。", loginErr)
				result.HTTPStatus = 0
				result.Path = ""
				result.RawResponseMasked = ""
				result.RetryCount = retries
				result = annotateCheckinRetry(result)
				if err := e.SaveResult(ctx, *auth, result, startedAt, now()); err != nil {
					log.Printf("[checkin] save result failed for account %s: %v", id, err)
				}
				return result, nil
			}
			var retryAfterLogin int
			status, body, retryAfterLogin, err = e.callAPIWithRetry(ctx, *auth, candidate)
			retries += retryAfterLogin
			if err != nil {
				message := publicOperationFailure("checkin", "retry-request", id, "签到请求失败，请稍后重试。", err)
				lastUnsupported = annotateCheckinRetry(checkinResult{Status: "failed", Message: message, Path: candidate.Path, RetryCount: retries})
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
		if err := e.SaveResult(ctx, *auth, result, startedAt, now()); err != nil {
			log.Printf("[checkin] save result failed for account %s: %v", id, err)
		}
		return result, nil
	}
	if err := e.SaveResult(ctx, *auth, lastUnsupported, startedAt, now()); err != nil {
		log.Printf("[checkin] save result failed for account %s: %v", id, err)
	}
	return lastUnsupported, nil
}

func (e *CheckinExecutor) callAPIWithRetry(ctx context.Context, auth accountAuthContext, candidate apiCandidate) (int, string, int, error) {
	var status int
	var body string
	var err error
	attempts := checkinMaxNetworkAttempts
	if attempts < 1 {
		attempts = 1
	}
	// Send empty JSON body for POST requests (AI API Hub convention).
	var postBody []byte
	if candidate.Method == http.MethodPost {
		postBody = []byte("{}")
	}
	for attempt := 1; attempt <= attempts; attempt++ {
		status, body, err = e.accountAPI.Do(ctx, auth, candidate.Method, candidate.Path, postBody)
		if !shouldRetryCheckinAttempt(status, err) || attempt == attempts {
			return status, body, attempt - 1, err
		}
		if !sleepWithContext(ctx, checkinRetryDelay(attempt)) {
			return status, body, attempt - 1, ctx.Err()
		}
	}
	return status, body, attempts - 1, err
}

func (e *CheckinExecutor) SaveResult(ctx context.Context, auth accountAuthContext, result checkinResult, startedAt string, finishedAt string) error {
	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO checkin_logs (id, account_id, upstream_site_id, channel_id, status, reward, message, raw_response_masked, started_at, finished_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, newID(), auth.AccountID, auth.UpstreamSiteID, auth.ChannelID, result.Status, result.Reward, result.Message, result.RawResponseMasked, startedAt, finishedAt)
	if err != nil {
		return err
	}
	updated, err := tx.ExecContext(ctx, `
		UPDATE channel_accounts
		SET last_checkin_at=?, last_checkin_status=?, updated_at=?
		WHERE id=?
	`, finishedAt, result.Status, now(), auth.AccountID)
	if err != nil {
		return err
	}
	rowsAffected, err := updated.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected != 1 {
		return fmt.Errorf("checkin result account update affected %d rows", rowsAffected)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if e.notify != nil {
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
		e.notify("checkin_"+result.Status, level, title, auth.AccountName+"： "+result.Message, "account", auth.AccountID)
	}
	return nil
}
