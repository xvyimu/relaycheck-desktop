package core

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

const accountProblemPredicate = `(
	LOWER(COALESCE(a.login_status, '')) IN ('expired', 'manual_required', 'captcha_required', 'two_factor_required')
	OR LOWER(COALESCE(a.last_checkin_status, '')) IN ('auth_expired', 'manual_required', 'failed')
)`

const accountPageSelect = `
	SELECT a.id, a.upstream_site_id, s.name, s.base_url, COALESCE(s.login_url,''), s.kind, a.display_name, COALESCE(a.email,''), COALESCE(a.username,''),
	       a.auth_type, COALESCE(a.browser_profile_path,''), a.login_status,
	       COALESCE(a.api_key_fingerprint,''), COALESCE(a.api_key_status,''), COALESCE(a.api_key_last_checked_at,''),
	       COALESCE(a.api_key_model_count,0), COALESCE(a.api_key_sample_models_json,''), COALESCE(a.api_key_test_model,''),
	       COALESCE(a.api_key_model_usable,0), COALESCE(a.api_key_latency_ms,0), COALESCE(a.api_key_test_http_status,0),
	       COALESCE(a.api_key_test_message,''), COALESCE(a.api_key_test_path,''),
	       COALESCE(a.balance_unit,'unknown'),
	       a.balance, COALESCE(a.last_checkin_at,''), COALESCE(a.last_checkin_status,''),
	       COALESCE((
	         SELECT l.message FROM checkin_logs l
	         WHERE l.account_id = a.id
	         ORDER BY l.started_at DESC, l.id DESC LIMIT 1
	       ), ''),
	       COALESCE(a.last_login_at,''), COALESCE(a.last_validated_at,''),
	       COALESCE(a.cookie_expiry_at,''), COALESCE(a.storage_state_expiry_at,''),
	       a.created_at, a.updated_at
	FROM channel_accounts a
	JOIN upstream_sites s ON s.id = a.upstream_site_id
`

type accountPageCursor struct {
	UpdatedAt string `json:"updatedAt"`
	ID        string `json:"id"`
}

type accountPageOptions struct {
	UpstreamSiteID string
	Query          string
	Status         string
	Limit          int
	Cursor         *accountPageCursor
}

func parseAccountPageOptions(r *http.Request) (accountPageOptions, error) {
	options := accountPageOptions{
		UpstreamSiteID: strings.TrimSpace(r.URL.Query().Get("upstreamSiteId")),
		Query:          strings.TrimSpace(r.URL.Query().Get("query")),
		Status:         strings.TrimSpace(strings.ToLower(r.URL.Query().Get("status"))),
		Limit:          clampListLimit(queryInt(r, "limit", 50), 50, 200),
	}
	if len(options.Query) > 200 {
		return options, errors.New("搜索关键词不能超过 200 个字符。")
	}
	if options.Status == "" {
		options.Status = "all"
	}
	if options.Status != "all" && options.Status != "problem" {
		return options, errors.New("账号状态筛选无效。")
	}
	if rawCursor := strings.TrimSpace(r.URL.Query().Get("cursor")); rawCursor != "" {
		cursor, err := decodeAccountPageCursor(rawCursor)
		if err != nil {
			return options, errors.New("账号分页游标无效。")
		}
		options.Cursor = &cursor
	}
	return options, nil
}

func encodeAccountPageCursor(updatedAt string, id string) string {
	body, _ := json.Marshal(accountPageCursor{UpdatedAt: updatedAt, ID: id})
	return base64.RawURLEncoding.EncodeToString(body)
}

func decodeAccountPageCursor(raw string) (accountPageCursor, error) {
	var cursor accountPageCursor
	body, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return cursor, err
	}
	if err := json.Unmarshal(body, &cursor); err != nil {
		return cursor, err
	}
	if strings.TrimSpace(cursor.ID) == "" || strings.TrimSpace(cursor.UpdatedAt) == "" {
		return cursor, errors.New("cursor fields are required")
	}
	return cursor, nil
}

func escapeAccountSearch(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	return strings.ReplaceAll(value, `_`, `\_`)
}

func buildAccountPageWhere(options accountPageOptions, includeCursor bool) (string, []interface{}) {
	where := ` WHERE 1=1`
	args := []interface{}{}
	if options.UpstreamSiteID != "" && options.UpstreamSiteID != "all" {
		where += ` AND a.upstream_site_id = ?`
		args = append(args, options.UpstreamSiteID)
	}
	if options.Query != "" {
		where += ` AND LOWER(
			COALESCE(a.display_name, '') || ' ' || COALESCE(a.email, '') || ' ' ||
			COALESCE(a.username, '') || ' ' || COALESCE(s.name, '') || ' ' ||
			COALESCE(a.login_status, '')
		) LIKE ? ESCAPE '\'`
		args = append(args, "%"+strings.ToLower(escapeAccountSearch(options.Query))+"%")
	}
	if options.Status == "problem" {
		where += ` AND ` + accountProblemPredicate
	}
	if includeCursor && options.Cursor != nil {
		where += ` AND (a.updated_at < ? OR (a.updated_at = ? AND a.id < ?))`
		args = append(args, options.Cursor.UpdatedAt, options.Cursor.UpdatedAt, options.Cursor.ID)
	}
	return where, args
}

func (a *App) loadAccountPage(ctx context.Context, options accountPageOptions) (AccountPage, error) {
	page := AccountPage{Items: []ChannelAccount{}}
	where, filterArgs := buildAccountPageWhere(options, false)
	countQuery := `SELECT
		(SELECT COUNT(*) FROM channel_accounts),
		(SELECT COUNT(*) FROM channel_accounts a WHERE ` + accountProblemPredicate + `),
		(SELECT COUNT(*) FROM channel_accounts a JOIN upstream_sites s ON s.id = a.upstream_site_id` + where + `)`
	if err := a.db.QueryRowContext(ctx, countQuery, filterArgs...).Scan(&page.AccountTotal, &page.ProblemTotal, &page.Total); err != nil {
		return page, err
	}

	pageWhere, pageArgs := buildAccountPageWhere(options, true)
	pageArgs = append(pageArgs, options.Limit+1)
	rows, err := a.db.QueryContext(ctx, accountPageSelect+pageWhere+` ORDER BY a.updated_at DESC, a.id DESC LIMIT ?`, pageArgs...)
	if err != nil {
		return page, err
	}
	defer rows.Close()

	for rows.Next() {
		item, err := scanAccountPageRow(rows)
		if err != nil {
			return page, err
		}
		page.Items = append(page.Items, item)
	}
	if err := rows.Err(); err != nil {
		return page, err
	}
	if len(page.Items) > options.Limit {
		page.Items = page.Items[:options.Limit]
		last := page.Items[len(page.Items)-1]
		page.NextCursor = encodeAccountPageCursor(last.UpdatedAt, last.ID)
	}
	return page, nil
}

type accountPageRowScanner interface {
	Scan(dest ...interface{}) error
}

func scanAccountPageRow(scanner accountPageRowScanner) (ChannelAccount, error) {
	var item ChannelAccount
	var balance sql.NullFloat64
	var sampleModelsJSON string
	var modelUsable int
	if err := scanner.Scan(
		&item.ID, &item.UpstreamSiteID, &item.UpstreamSiteName, &item.UpstreamSiteBaseURL, &item.UpstreamSiteLoginURL, &item.UpstreamSiteKind,
		&item.DisplayName, &item.Email, &item.Username, &item.AuthType, &item.BrowserProfilePath, &item.LoginStatus,
		&item.APIKeyFingerprint, &item.APIKeyStatus, &item.APIKeyLastCheckedAt, &item.APIKeyModelCount, &sampleModelsJSON, &item.APIKeyTestModel,
		&modelUsable, &item.APIKeyLatencyMs, &item.APIKeyTestHTTPStatus, &item.APIKeyTestMessage, &item.APIKeyTestPath, &item.BalanceUnit,
		&balance, &item.LastCheckinAt, &item.LastCheckinStatus, &item.LastCheckinMessage, &item.LastLoginAt, &item.LastValidatedAt,
		&item.CookieExpiryAt, &item.StorageStateExpiryAt, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return item, err
	}
	item.APIKeyModelUsable = modelUsable == 1
	item.APIKeySampleModels = parsePersistedStringSlice(sampleModelsJSON)
	item.Balance = nullableFloat(balance)
	return item, nil
}

func (a *App) handleAccountsPage(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	options, err := parseAccountPageOptions(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	page, err := a.loadAccountPage(r.Context(), options)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "加载账号分页失败。")
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (a *App) loadAccountSummary(ctx context.Context) (AccountSummary, error) {
	return cachedRead(a, "accounts-summary", shortReadCacheTTL, func() (AccountSummary, error) {
		var summary AccountSummary
		err := a.db.QueryRowContext(ctx, `SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN `+accountProblemPredicate+` THEN 1 ELSE 0 END), 0)
			FROM channel_accounts a`).Scan(&summary.AccountTotal, &summary.ProblemTotal)
		return summary, err
	})
}

func (a *App) handleAccountsSummary(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	summary, err := a.loadAccountSummary(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "加载账号摘要失败。")
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (a *App) loadAccountSearchIndex(ctx context.Context) ([]AccountSearchIndexItem, error) {
	return cachedRead(a, "accounts-search-index", shortReadCacheTTL, func() ([]AccountSearchIndexItem, error) {
		rows, err := a.db.QueryContext(ctx, `
			SELECT s.id, s.name, s.base_url,
			       COALESCE(SUBSTR(GROUP_CONCAT(
			         TRIM(COALESCE(a.display_name, '') || ' ' || COALESCE(a.email, '') || ' ' || COALESCE(a.username, '')),
			         ' '
			       ), 1, 4000), '')
			FROM upstream_sites s
			LEFT JOIN channel_accounts a ON a.upstream_site_id = s.id
			GROUP BY s.id, s.name, s.base_url
			ORDER BY s.name, s.id
		`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		items := []AccountSearchIndexItem{}
		for rows.Next() {
			var item AccountSearchIndexItem
			if err := rows.Scan(&item.UpstreamSiteID, &item.UpstreamSiteName, &item.UpstreamSiteBaseURL, &item.SearchText); err != nil {
				return nil, err
			}
			items = append(items, item)
		}
		return items, rows.Err()
	})
}

func (a *App) handleAccountSearchIndex(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	items, err := a.loadAccountSearchIndex(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "加载账号搜索索引失败。")
		return
	}
	writeJSON(w, http.StatusOK, items)
}
