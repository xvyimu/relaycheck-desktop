package core

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
)

type BalanceRefresher struct {
	db             *sql.DB
	accountAuth    *AccountAuthRepository
	accountAPI     *AccountAPIClient
	accountSession *AccountSessionService
	notify         func(string, string, string, string, string, string)
}

func NewBalanceRefresher(app *App) *BalanceRefresher {
	return &BalanceRefresher{
		db:             app.db,
		accountAuth:    app.accountAuth,
		accountAPI:     app.accountAPI,
		accountSession: app.accountSession,
		notify:         app.notify,
	}
}

func (r *BalanceRefresher) Run(ctx context.Context, id string, auth *accountAuthContext) (balanceResult, error) {
	if auth == nil {
		loaded, err := r.accountAuth.Load(ctx, id)
		if err != nil {
			return balanceResult{}, err
		}
		auth = loaded
	}
	if !auth.SupportsBalance {
		return balanceResult{Unit: "unknown", HTTPStatus: 0, Path: "", RawResponseMasked: "", Balance: nil}, errorsText("该站点未探测到余额接口。")
	}
	if err := r.accountSession.Ensure(ctx, auth); err != nil && auth.Cookie == "" && auth.AccessToken == "" && auth.APIKey == "" {
		return balanceResult{Unit: "unknown"}, fmt.Errorf("账号密码登录失败：%w", err)
	}

	var lastErr error
	for _, path := range balanceCandidates {
		status, body, err := r.accountAPI.Do(ctx, *auth, http.MethodGet, path, nil)
		if err != nil {
			lastErr = err
			continue
		}
		if status == http.StatusNotFound || status == http.StatusMethodNotAllowed {
			continue
		}
		if status == http.StatusUnauthorized || status == http.StatusForbidden {
			lastErr = fmt.Errorf("%s 登录态不可用：HTTP 状态码 %d", path, status)
			continue
		}
		if status < 200 || status >= 300 {
			lastErr = fmt.Errorf("%s 返回 HTTP 状态码 %d", path, status)
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
		if err := r.SaveResult(ctx, *auth, result); err != nil {
			return result, err
		}
		return result, nil
	}
	if lastErr != nil {
		return balanceResult{Unit: "unknown"}, lastErr
	}
	return balanceResult{Unit: "unknown"}, errorsText("未找到可用余额接口。")
}

func (r *BalanceRefresher) SaveResult(ctx context.Context, auth accountAuthContext, result balanceResult) error {
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
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO balance_snapshots (id, account_id, upstream_site_id, channel_id, balance, used_quota, total_quota, unit, raw_response_masked, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, newID(), auth.AccountID, auth.UpstreamSiteID, auth.ChannelID, balanceValue, usedValue, totalValue, result.Unit, result.RawResponseMasked, createdAt)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
		UPDATE channel_accounts
		SET balance=?, balance_unit=?, last_validated_at=?, login_status='valid', updated_at=?
		WHERE id=?
	`, balanceValue, result.Unit, createdAt, now(), auth.AccountID)
	if err == nil {
		r.notify("balance_refreshed", "success", "余额已刷新", auth.AccountName+" 余额信息已更新。", "account", auth.AccountID)
	}
	return err
}
