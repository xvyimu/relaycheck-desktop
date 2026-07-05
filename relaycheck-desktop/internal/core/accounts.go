package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

func (a *App) handleAccounts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.listAccounts(w, r)
	case http.MethodPost:
		a.createAccount(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (a *App) listAccounts(w http.ResponseWriter, r *http.Request) {
	// Honor the optional ?upstreamSiteId= filter. The previous implementation
	// ignored the query parameter and always returned the full table, which
	// made per-site account views impossible. Cache key is scoped by siteId so
	// filtered and unfiltered reads do not poison each other.
	siteID := strings.TrimSpace(r.URL.Query().Get("upstreamSiteId"))
	cacheKey := "accounts-list"
	if siteID != "" {
		cacheKey = "accounts-list:" + siteID
	}

	items, err := cachedRead(a, cacheKey, shortReadCacheTTL, func() ([]ChannelAccount, error) {
		const selectColumns = `
			SELECT a.id, a.upstream_site_id, s.name, s.base_url, COALESCE(s.login_url,''), s.kind, a.display_name, COALESCE(a.email,''), COALESCE(a.username,''),
			       a.auth_type, COALESCE(a.browser_profile_path,''), a.login_status,
			       COALESCE(a.api_key_fingerprint,''), COALESCE(a.api_key_status,''), COALESCE(a.api_key_last_checked_at,''),
			       COALESCE(a.api_key_model_count,0), COALESCE(a.api_key_sample_models_json,''), COALESCE(a.api_key_test_model,''),
			       COALESCE(a.api_key_model_usable,0), COALESCE(a.api_key_latency_ms,0), COALESCE(a.api_key_test_http_status,0),
			       COALESCE(a.api_key_test_message,''), COALESCE(a.api_key_test_path,''),
			       COALESCE(a.balance_unit,'unknown'),
			       a.balance, COALESCE(a.last_checkin_at,''), COALESCE(a.last_checkin_status,''),
			       COALESCE((SELECT l.message FROM checkin_logs l WHERE l.account_id = a.id ORDER BY l.started_at DESC LIMIT 1), ''),
			       COALESCE(a.last_login_at,''), COALESCE(a.last_validated_at,''),
			       COALESCE(a.cookie_expiry_at,''), COALESCE(a.storage_state_expiry_at,''),
			       a.created_at, a.updated_at
			FROM channel_accounts a
			JOIN upstream_sites s ON s.id = a.upstream_site_id
		`
		var rows *sql.Rows
		var err error
		if siteID != "" {
			rows, err = a.db.QueryContext(r.Context(), selectColumns+`
				WHERE a.upstream_site_id = ?
				ORDER BY a.updated_at DESC
			`, siteID)
		} else {
			rows, err = a.db.QueryContext(r.Context(), selectColumns+`
				ORDER BY a.updated_at DESC
			`)
		}
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		items := []ChannelAccount{}
		for rows.Next() {
			var item ChannelAccount
			var balance sql.NullFloat64
			var sampleModelsJSON string
			var modelUsable int
			if err := rows.Scan(&item.ID, &item.UpstreamSiteID, &item.UpstreamSiteName, &item.UpstreamSiteBaseURL, &item.UpstreamSiteLoginURL, &item.UpstreamSiteKind, &item.DisplayName, &item.Email, &item.Username, &item.AuthType, &item.BrowserProfilePath, &item.LoginStatus, &item.APIKeyFingerprint, &item.APIKeyStatus, &item.APIKeyLastCheckedAt, &item.APIKeyModelCount, &sampleModelsJSON, &item.APIKeyTestModel, &modelUsable, &item.APIKeyLatencyMs, &item.APIKeyTestHTTPStatus, &item.APIKeyTestMessage, &item.APIKeyTestPath, &item.BalanceUnit, &balance, &item.LastCheckinAt, &item.LastCheckinStatus, &item.LastCheckinMessage, &item.LastLoginAt, &item.LastValidatedAt, &item.CookieExpiryAt, &item.StorageStateExpiryAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
				return nil, err
			}
			item.APIKeyModelUsable = modelUsable == 1
			item.APIKeySampleModels = parsePersistedStringSlice(sampleModelsJSON)
			item.Balance = nullableFloat(balance)
			items = append(items, item)
		}
		return items, rows.Err()
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (a *App) createAccount(w http.ResponseWriter, r *http.Request) {
	var input accountCreationInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "账号参数不完整。")
		return
	}
	id, err := a.accountCreation.Create(r.Context(), input)
	if err != nil {
		if isAccountCreationBadRequest(err) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": id})
}

func (a *App) ensureManualAccountSite(ctx context.Context, name string, rawBaseURL string, loginURL string, preferredKind string) (string, error) {
	return a.accountCreation.EnsureManualSite(ctx, name, rawBaseURL, loginURL, preferredKind)
}

func inferAccountAuthType(password string, cookie string, accessToken string, refreshToken string, apiKey string) string {
	switch {
	case strings.TrimSpace(apiKey) != "":
		return "api_key"
	case strings.TrimSpace(cookie) != "":
		return "cookie"
	case strings.TrimSpace(accessToken) != "":
		return "access_token"
	case strings.TrimSpace(refreshToken) != "":
		return "refresh_token"
	case strings.TrimSpace(password) != "":
		return "email_password"
	default:
		return "browser_profile"
	}
}

func defaultAccountDisplayName(email string, username string, apiKey string) string {
	if loginName := firstNonEmpty(strings.TrimSpace(email), strings.TrimSpace(username)); loginName != "" {
		return loginName
	}
	if fingerprint := secretFingerprint(apiKey); fingerprint != "" {
		return "API Key " + fingerprint
	}
	return "网页登录账号"
}

type bulkPasswordLoginResult struct {
	AccountID   string `json:"accountId"`
	AccountName string `json:"accountName"`
	SiteName    string `json:"siteName"`
	Status      string `json:"status"`
	Message     string `json:"message"`
}

type browserLoginOpenResult struct {
	AccountID          string  `json:"accountId"`
	AccountName        string  `json:"accountName,omitempty"`
	SiteName           string  `json:"siteName,omitempty"`
	Status             string  `json:"status"`
	Message            string  `json:"message,omitempty"`
	URL                string  `json:"url,omitempty"`
	LoginURLSource     string  `json:"loginUrlSource,omitempty"`
	LoginURLConfidence float64 `json:"loginUrlConfidence,omitempty"`
	LoginURLReason     string  `json:"loginUrlReason,omitempty"`
	DebugPort          int     `json:"debugPort,omitempty"`
	ProfilePath        string  `json:"profilePath,omitempty"`
}

type browserLoginSaveResult struct {
	AccountID     string `json:"accountId"`
	AccountName   string `json:"accountName,omitempty"`
	SiteName      string `json:"siteName,omitempty"`
	Status        string `json:"status"`
	Message       string `json:"message,omitempty"`
	CookieCount   int    `json:"cookieCount,omitempty"`
	CookiePreview string `json:"cookiePreview,omitempty"`
}

func (a *App) handleBulkPasswordLogin(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var input struct {
		Limit int `json:"limit"`
	}
	_ = decodeJSON(r, &input)
	result, err := a.accountLoginBatch.PasswordLogin(r.Context(), input.Limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *App) retryPasswordLogin(ctx context.Context, id string, auth *accountAuthContext) bulkPasswordLoginResult {
	return a.accountLoginBatch.RetryPasswordLogin(ctx, id, auth)
}

func (a *App) handleBulkOpenBrowserLogin(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var input struct {
		Limit int      `json:"limit"`
		IDs   []string `json:"ids"`
	}
	_ = decodeJSON(r, &input)
	result, err := a.accountLoginBatch.OpenBrowser(r.Context(), input.Limit, input.IDs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *App) handleBulkFinishBrowserLogin(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var input struct {
		IDs []string `json:"ids"`
	}
	_ = decodeJSON(r, &input)
	result, err := a.accountLoginBatch.FinishBrowser(r.Context(), input.IDs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *App) handleAccountByID(w http.ResponseWriter, r *http.Request) {
	tail := pathTail(r.URL.Path, "/api/accounts/")
	if strings.HasSuffix(tail, "/open-browser-login") {
		a.openBrowserLogin(w, r, strings.TrimSuffix(tail, "/open-browser-login"))
		return
	}
	if strings.HasSuffix(tail, "/finish-browser-login") {
		a.finishBrowserLogin(w, r, strings.TrimSuffix(tail, "/finish-browser-login"))
		return
	}
	if strings.HasSuffix(tail, "/test-login") {
		a.testAccountLogin(w, r, strings.TrimSuffix(tail, "/test-login"))
		return
	}
	if strings.HasSuffix(tail, "/test-api-key") {
		a.testAccountAPIKey(w, r, strings.TrimSuffix(tail, "/test-api-key"))
		return
	}
	if strings.HasSuffix(tail, "/checkin") {
		a.checkinAccount(w, r, strings.TrimSuffix(tail, "/checkin"))
		return
	}
	if strings.HasSuffix(tail, "/refresh-balance") {
		a.refreshBalanceAccount(w, r, strings.TrimSuffix(tail, "/refresh-balance"))
		return
	}
	if strings.HasSuffix(tail, "/clear-session") {
		a.clearAccountSession(w, r, strings.TrimSuffix(tail, "/clear-session"))
		return
	}
	if r.Method == http.MethodGet {
		item, err := a.loadAccountByID(r.Context(), tail)
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "账号不存在。")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, item)
		return
	}
	if r.Method == http.MethodPut {
		a.updateAccount(w, r, tail)
		return
	}
	if r.Method == http.MethodDelete {
		a.deleteAccount(w, r, tail)
		return
	}
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func (a *App) loadAccountByID(ctx context.Context, id string) (ChannelAccount, error) {
	var item ChannelAccount
	var balance sql.NullFloat64
	var sampleModelsJSON string
	var modelUsable int
	err := a.db.QueryRowContext(ctx, `
		SELECT a.id, a.upstream_site_id, s.name, s.base_url, COALESCE(s.login_url,''), s.kind, a.display_name, COALESCE(a.email,''), COALESCE(a.username,''),
		       a.auth_type, COALESCE(a.browser_profile_path,''), a.login_status,
		       COALESCE(a.api_key_fingerprint,''), COALESCE(a.api_key_status,''), COALESCE(a.api_key_last_checked_at,''),
		       COALESCE(a.api_key_model_count,0), COALESCE(a.api_key_sample_models_json,''), COALESCE(a.api_key_test_model,''),
		       COALESCE(a.api_key_model_usable,0), COALESCE(a.api_key_latency_ms,0), COALESCE(a.api_key_test_http_status,0),
		       COALESCE(a.api_key_test_message,''), COALESCE(a.api_key_test_path,''),
		       COALESCE(a.balance_unit,'unknown'),
		       a.balance, COALESCE(a.last_checkin_at,''), COALESCE(a.last_checkin_status,''),
		       COALESCE((SELECT l.message FROM checkin_logs l WHERE l.account_id = a.id ORDER BY l.started_at DESC LIMIT 1), ''),
		       COALESCE(a.last_login_at,''), COALESCE(a.last_validated_at,''),
		       COALESCE(a.cookie_expiry_at,''), COALESCE(a.storage_state_expiry_at,''),
		       a.created_at, a.updated_at
		FROM channel_accounts a
		JOIN upstream_sites s ON s.id = a.upstream_site_id
		WHERE a.id=?
	`, id).Scan(&item.ID, &item.UpstreamSiteID, &item.UpstreamSiteName, &item.UpstreamSiteBaseURL, &item.UpstreamSiteLoginURL, &item.UpstreamSiteKind, &item.DisplayName, &item.Email, &item.Username, &item.AuthType, &item.BrowserProfilePath, &item.LoginStatus, &item.APIKeyFingerprint, &item.APIKeyStatus, &item.APIKeyLastCheckedAt, &item.APIKeyModelCount, &sampleModelsJSON, &item.APIKeyTestModel, &modelUsable, &item.APIKeyLatencyMs, &item.APIKeyTestHTTPStatus, &item.APIKeyTestMessage, &item.APIKeyTestPath, &item.BalanceUnit, &balance, &item.LastCheckinAt, &item.LastCheckinStatus, &item.LastCheckinMessage, &item.LastLoginAt, &item.LastValidatedAt, &item.CookieExpiryAt, &item.StorageStateExpiryAt, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return item, err
	}
	item.APIKeyModelUsable = modelUsable == 1
	item.APIKeySampleModels = parsePersistedStringSlice(sampleModelsJSON)
	item.Balance = nullableFloat(balance)
	return item, nil
}

func (a *App) updateAccount(w http.ResponseWriter, r *http.Request, id string) {
	var input struct {
		DisplayName  string `json:"displayName"`
		SiteName     string `json:"siteName"`
		BaseURL      string `json:"baseUrl"`
		LoginURL     string `json:"loginUrl"`
		Kind         string `json:"kind"`
		Email        string `json:"email"`
		Username     string `json:"username"`
		AuthType     string `json:"authType"`
		Password     string `json:"password"`
		APIKey       string `json:"apiKey"`
		Cookie       string `json:"cookie"`
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		SiteScope    string `json:"siteUpdateScope"`
		ClearAPIKey  bool   `json:"clearApiKey"`
		ClearCookie  bool   `json:"clearCookie"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "账号参数不完整。")
		return
	}

	current, err := a.loadAccountByID(r.Context(), id)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "账号不存在。")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	displayName := strings.TrimSpace(input.DisplayName)
	if displayName == "" {
		displayName = defaultAccountDisplayName(input.Email, input.Username, input.APIKey)
	}
	if displayName == "网页登录账号" {
		displayName = current.DisplayName
	}
	authType := strings.TrimSpace(input.AuthType)
	if authType == "" {
		authType = current.AuthType
	}
	siteID, siteChanged, err := a.resolveAccountSiteUpdate(r.Context(), current, input.SiteName, input.BaseURL, input.LoginURL, input.Kind, input.SiteScope)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updatedAt := now()

	sets := []string{
		"display_name=?",
		"email=?",
		"username=?",
		"auth_type=?",
		"updated_at=?",
	}
	args := []interface{}{
		displayName,
		strings.TrimSpace(input.Email),
		strings.TrimSpace(input.Username),
		authType,
		updatedAt,
	}
	if siteChanged {
		sets = append(sets, "upstream_site_id=?")
		args = append(args, siteID)
	}

	if strings.TrimSpace(input.Password) != "" {
		encrypted, err := a.encryptText(input.Password)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		sets = append(sets, "password_encrypted=?")
		args = append(args, encrypted)
	}
	if strings.TrimSpace(input.APIKey) != "" || input.ClearAPIKey {
		encrypted := ""
		fingerprint := ""
		status := "missing"
		if strings.TrimSpace(input.APIKey) != "" {
			var err error
			encrypted, err = a.encryptText(input.APIKey)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			fingerprint = secretFingerprint(input.APIKey)
			status = statusFromKey(fingerprint)
		}
		sets = append(sets,
			"api_key_encrypted=?", "api_key_fingerprint=?", "api_key_status=?", "api_key_last_checked_at=''",
			"api_key_model_count=0", "api_key_sample_models_json=''", "api_key_test_model=''",
			"api_key_model_usable=0", "api_key_latency_ms=0", "api_key_test_http_status=0",
			"api_key_test_message=''", "api_key_test_path=''",
		)
		args = append(args, encrypted, fingerprint, status)
	}
	if strings.TrimSpace(input.Cookie) != "" || input.ClearCookie {
		encrypted := ""
		if strings.TrimSpace(input.Cookie) != "" {
			var err error
			encrypted, err = a.encryptText(input.Cookie)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
		sets = append(sets, "cookie_encrypted=?", "login_status=?")
		args = append(args, encrypted, map[bool]string{true: "manual_required", false: "valid"}[input.ClearCookie])
	}
	if strings.TrimSpace(input.AccessToken) != "" {
		encrypted, err := a.encryptText(input.AccessToken)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		sets = append(sets, "access_token_encrypted=?")
		args = append(args, encrypted)
	}
	if strings.TrimSpace(input.RefreshToken) != "" {
		encrypted, err := a.encryptText(input.RefreshToken)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		sets = append(sets, "refresh_token_encrypted=?")
		args = append(args, encrypted)
	}

	args = append(args, id)
	query := "UPDATE channel_accounts SET " + strings.Join(sets, ", ") + " WHERE id=?"
	if _, err := a.db.ExecContext(r.Context(), query, args...); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.notify("account_updated", "success", "账号已更新", displayName+" 的账号信息已保存。", "account", id)
	a.audit("account.updated", "info", "", "account", id, "账号已更新："+displayName, map[string]interface{}{"updatedFields": auditUpdatedAccountFields(
		input.SiteName,
		input.BaseURL,
		input.LoginURL,
		input.Kind,
		input.DisplayName,
		input.Email,
		input.Username,
		input.Password,
		false,
		input.APIKey,
		input.ClearAPIKey,
		input.Cookie,
		input.ClearCookie,
		input.AccessToken,
		input.RefreshToken,
	)})
	item, err := a.loadAccountByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *App) resolveAccountSiteUpdate(ctx context.Context, current ChannelAccount, siteName string, rawBaseURL string, loginURL string, preferredKind string, siteScope string) (string, bool, error) {
	return a.accountSiteUpdates.Resolve(ctx, current, siteName, rawBaseURL, loginURL, preferredKind, siteScope)
}

func (a *App) updateSharedAccountSite(ctx context.Context, current ChannelAccount, siteName string, baseURL string, loginURL string, kind string) (string, bool, error) {
	return a.accountSiteUpdates.UpdateShared(ctx, current, siteName, baseURL, loginURL, kind)
}

func (a *App) updateAccountSiteAddress(ctx context.Context, siteID string, siteName string, baseURL string, loginURL string, kind string) error {
	return a.accountSiteUpdates.UpdateAddress(ctx, siteID, siteName, baseURL, loginURL, kind)
}

func (a *App) updateAccountSiteMetadata(ctx context.Context, siteID string, siteName string, loginURL string, kind string) error {
	return a.accountSiteUpdates.UpdateMetadata(ctx, siteID, siteName, loginURL, kind)
}

func (a *App) checkinAccount(w http.ResponseWriter, r *http.Request, id string) {
	if !method(w, r, http.MethodPost) {
		return
	}
	result, err := a.runAccountCheckin(r.Context(), id, nil)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *App) refreshBalanceAccount(w http.ResponseWriter, r *http.Request, id string) {
	if !method(w, r, http.MethodPost) {
		return
	}
	result, err := a.refreshAccountBalance(r.Context(), id, nil)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *App) openBrowserLogin(w http.ResponseWriter, r *http.Request, id string) {
	if !method(w, r, http.MethodPost) {
		return
	}
	result := a.browserLogin.Open(r.Context(), id, nil)
	if result.Status == "failed" {
		writeError(w, http.StatusInternalServerError, result.Message)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *App) finishBrowserLogin(w http.ResponseWriter, r *http.Request, id string) {
	if !method(w, r, http.MethodPost) {
		return
	}
	result := a.browserLogin.Save(r.Context(), id, nil)
	if result.Status == "failed" {
		writeError(w, http.StatusBadRequest, result.Message)
		return
	}
	if result.Status == "missing" {
		writeError(w, http.StatusBadRequest, result.Message)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *App) startBrowserLogin(ctx context.Context, id string, auth *accountAuthContext) browserLoginOpenResult {
	return a.browserLogin.Open(ctx, id, auth)
}

func resolveLoginTargetURL(baseURL string, loginURL string) string {
	return resolveBrowserLoginTargetURL(baseURL, loginURL)
}

func resolveManualLoginTargetURL(baseURL string, loginURL string) string {
	return resolveManualBrowserLoginTargetURL(baseURL, loginURL)
}

func (a *App) saveBrowserLoginSession(ctx context.Context, id string, auth *accountAuthContext) browserLoginSaveResult {
	return a.browserLogin.Save(ctx, id, auth)
}

func (a *App) testAccountLogin(w http.ResponseWriter, r *http.Request, id string) {
	if !method(w, r, http.MethodPost) {
		return
	}
	result, err := a.accountValidation.TestLogin(r.Context(), id)
	if err != nil {
		status, message := accountValidationHTTPErrorStatus(err)
		writeError(w, status, message)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

type apiKeyTestResult struct {
	AccountID           string   `json:"accountId"`
	AccountName         string   `json:"accountName,omitempty"`
	SiteName            string   `json:"siteName,omitempty"`
	Fingerprint         string   `json:"fingerprint,omitempty"`
	Status              string   `json:"status"`
	HTTPStatus          int      `json:"httpStatus,omitempty"`
	Path                string   `json:"path,omitempty"`
	Message             string   `json:"message,omitempty"`
	ModelCount          int      `json:"modelCount,omitempty"`
	SampleModels        []string `json:"sampleModels,omitempty"`
	TestedModel         string   `json:"testedModel,omitempty"`
	ModelUsable         bool     `json:"modelUsable"`
	ModelTestHTTPStatus int      `json:"modelTestHttpStatus,omitempty"`
	ModelTestLatencyMs  int64    `json:"modelTestLatencyMs,omitempty"`
	ModelTestMessage    string   `json:"modelTestMessage,omitempty"`
	ModelTestPath       string   `json:"modelTestPath,omitempty"`
}

func (a *App) testAccountAPIKey(w http.ResponseWriter, r *http.Request, id string) {
	if !method(w, r, http.MethodPost) {
		return
	}
	result := a.testAPIKeyForAccount(r.Context(), id, nil)
	if result.Status == "missing" {
		writeError(w, http.StatusBadRequest, result.Message)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *App) handleBulkTestAPIKeys(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var input struct {
		Limit int `json:"limit"`
	}
	_ = decodeJSON(r, &input)
	input.Limit = clampBatchLimit(input.Limit, 10)
	rows, err := a.db.QueryContext(r.Context(), `
		SELECT id FROM channel_accounts
		WHERE COALESCE(api_key_encrypted,'') <> ''
		ORDER BY COALESCE(api_key_last_checked_at,''), updated_at DESC
		LIMIT ?
	`, input.Limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = rows.Close()

	results := []apiKeyTestResult{}
	valid := 0
	usable := 0
	auths, _ := a.loadAccountAuths(r.Context(), ids)
	for _, id := range ids {
		var auth *accountAuthContext
		if loaded, ok := auths[id]; ok {
			auth = &loaded
		}
		result := a.testAPIKeyForAccount(r.Context(), id, auth)
		if result.Status == "valid" {
			valid++
		}
		if result.ModelUsable {
			usable++
		}
		results = append(results, result)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"processed": len(results),
		"valid":     valid,
		"usable":    usable,
		"invalid":   len(results) - valid,
		"results":   results,
	})
}

func (a *App) testAPIKeyForAccount(ctx context.Context, id string, auth *accountAuthContext) apiKeyTestResult {
	return a.accountValidation.TestAPIKey(ctx, id, auth)
}

func (a *App) speedTestAPIKeyModel(ctx context.Context, auth *accountAuthContext, result *apiKeyTestResult) {
	a.accountValidation.SpeedTestAPIKeyModel(ctx, auth, result)
}

func (a *App) callAccountAPIWithTimeout(ctx context.Context, auth accountAuthContext, method string, path string, body []byte, timeout time.Duration) (int, string, error) {
	return a.accountAPI.DoWithTimeout(ctx, auth, method, path, body, timeout)
}

func parseModelIDs(body string) []string {
	var payload interface{}
	if json.Unmarshal([]byte(body), &payload) != nil {
		return nil
	}
	seen := map[string]bool{}
	models := []string{}
	var walk func(interface{})
	walk = func(value interface{}) {
		switch typed := value.(type) {
		case map[string]interface{}:
			if id, ok := typed["id"]; ok {
				text := strings.TrimSpace(fmt.Sprint(id))
				if text != "" && !seen[text] {
					seen[text] = true
					models = append(models, text)
				}
			}
			if name, ok := typed["model"]; ok {
				text := strings.TrimSpace(fmt.Sprint(name))
				if text != "" && !seen[text] && !strings.Contains(text, "map[") {
					seen[text] = true
					models = append(models, text)
				}
			}
			for _, child := range typed {
				walk(child)
			}
		case []interface{}:
			for _, child := range typed {
				walk(child)
			}
		case string:
			text := strings.TrimSpace(typed)
			if looksLikeModelID(text) && !seen[text] {
				seen[text] = true
				models = append(models, text)
			}
		}
	}
	walk(payload)
	return models
}

func looksLikeModelID(value string) bool {
	if value == "" || len(value) > 120 || strings.Contains(value, " ") {
		return false
	}
	lower := strings.ToLower(value)
	prefixes := []string{"gpt-", "claude-", "deepseek", "gemini", "qwen", "glm-", "yi-", "moonshot", "kimi", "doubao", "abab", "llama", "mistral", "mixtral"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return strings.Contains(lower, "-") && (strings.Contains(lower, "chat") || strings.Contains(lower, "turbo") || strings.Contains(lower, "model"))
}

func chooseModelForSpeedTest(models []string) string {
	preferred := []string{
		"gpt-4o-mini", "gpt-4.1-mini", "gpt-3.5-turbo", "deepseek-chat",
		"qwen-turbo", "qwen-plus", "glm-4-flash", "doubao-lite", "moonshot-v1-8k",
	}
	lowerToOriginal := map[string]string{}
	for _, model := range models {
		lowerToOriginal[strings.ToLower(model)] = model
	}
	for _, wanted := range preferred {
		if original := lowerToOriginal[wanted]; original != "" {
			return original
		}
	}
	for _, model := range models {
		lower := strings.ToLower(model)
		if strings.Contains(lower, "chat") || strings.Contains(lower, "turbo") || strings.Contains(lower, "mini") || strings.Contains(lower, "flash") {
			return model
		}
	}
	return models[0]
}

func limitStrings(values []string, limit int) []string {
	if len(values) <= limit {
		return values
	}
	return append([]string{}, values[:limit]...)
}

func parsePersistedStringSlice(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil
	}
	return limitStrings(values, 8)
}

func marshalStringSlice(values []string) string {
	if len(values) == 0 {
		return ""
	}
	body, err := json.Marshal(limitStrings(values, 8))
	if err != nil {
		return ""
	}
	return string(body)
}

func sanitizeAPIKeyDiagnostic(message string, apiKey string) string {
	message = maskResponse(message)
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" || message == "" {
		return message
	}
	message = strings.ReplaceAll(message, apiKey, maskSecret(apiKey))
	message = strings.ReplaceAll(message, "Bearer "+apiKey, "Bearer "+maskSecret(apiKey))
	message = strings.ReplaceAll(message, "bearer "+apiKey, "bearer "+maskSecret(apiKey))
	return message
}

// estimateCookieExpiry returns an ISO 8601 timestamp approximately 30 days
// from now, representing the estimated cookie expiry for most relay sites.
func estimateCookieExpiry() string {
	return time.Now().UTC().Add(30 * 24 * time.Hour).Format(time.RFC3339)
}

func (a *App) clearAccountSession(w http.ResponseWriter, r *http.Request, id string) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var profilePath string
	a.browserSessions.Delete(id)
	if err := a.db.QueryRowContext(r.Context(), `SELECT COALESCE(browser_profile_path,'') FROM channel_accounts WHERE id=?`, id).Scan(&profilePath); err != nil && err != sql.ErrNoRows {
		log.Printf("[accounts] clearAccountSession load profile path failed for %s: %v", id, err)
	}
	if profilePath != "" && strings.HasPrefix(filepath.Clean(profilePath), filepath.Clean(a.dataDir)) {
		if rmErr := os.RemoveAll(profilePath); rmErr != nil {
			log.Printf("[accounts] clearAccountSession: remove profile %s failed: %v", profilePath, rmErr)
		}
	}
	_, err := a.db.ExecContext(r.Context(), `
		UPDATE channel_accounts
		SET cookie_encrypted='', browser_profile_path='', user_agent='', login_status='manual_required', updated_at=?
		WHERE id=?
	`, now(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.audit("browser_auth.disconnected", "warning", "", "account", id, "网页登录授权已断开。", nil)
	writeJSON(w, http.StatusOK, map[string]bool{"cleared": true})
}

type cdpCookie struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type cdpResponse struct {
	ID     int `json:"id"`
	Result struct {
		Cookies []cdpCookie `json:"cookies"`
		Result  struct {
			Value string `json:"value"`
		} `json:"result"`
	} `json:"result"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func readChromeSession(port int) ([]cdpCookie, string, error) {
	pageWS, err := findPageWebSocket(port)
	if err != nil {
		return nil, "", err
	}
	conn, _, err := websocket.DefaultDialer.Dial(pageWS, nil)
	if err != nil {
		return nil, "", err
	}
	defer conn.Close()

	if err := conn.WriteJSON(map[string]interface{}{"id": 1, "method": "Network.getAllCookies"}); err != nil {
		return nil, "", err
	}
	var cookieResp cdpResponse
	if err := conn.ReadJSON(&cookieResp); err != nil {
		return nil, "", err
	}
	if cookieResp.Error != nil {
		return nil, "", errors.New(cookieResp.Error.Message)
	}

	userAgent := ""
	_ = conn.WriteJSON(map[string]interface{}{
		"id":     2,
		"method": "Runtime.evaluate",
		"params": map[string]interface{}{"expression": "navigator.userAgent", "returnByValue": true},
	})
	var uaResp cdpResponse
	if err := conn.ReadJSON(&uaResp); err == nil {
		userAgent = uaResp.Result.Result.Value
	}

	return cookieResp.Result.Cookies, userAgent, nil
}

func findPageWebSocket(port int) (string, error) {
	// Use a bounded-time HTTP client so a stuck Chrome DevTools endpoint
	// cannot hang the entire saveBrowserLoginSession request indefinitely.
	// The default http.DefaultClient has no timeout.
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/json/list")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var pages []struct {
		Type                 string `json:"type"`
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.Unmarshal(body, &pages); err != nil {
		return "", err
	}
	for _, page := range pages {
		if page.Type == "page" && page.WebSocketDebuggerURL != "" {
			return page.WebSocketDebuggerURL, nil
		}
	}
	return "", errors.New("未找到可读取的浏览器页面，请确认登录页仍然打开。")
}

func buildCookieHeader(cookies []cdpCookie) string {
	parts := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		if cookie.Name != "" {
			parts = append(parts, cookie.Name+"="+cookie.Value)
		}
	}
	return strings.Join(parts, "; ")
}

func statusFromKey(fingerprint string) string {
	if fingerprint == "" {
		return ""
	}
	return "unchecked"
}

func freeDebugPort(used map[int]bool) (int, error) {
	for port := 9222; port < 9250; port++ {
		if used[port] {
			continue
		}
		listener, err := netListen("127.0.0.1:" + strconv.Itoa(port))
		if err == nil {
			_ = listener.Close()
			return port, nil
		}
	}
	return 0, errors.New("没有可用的浏览器调试端口。")
}

func findChrome() (string, error) {
	candidates := []string{
		filepath.Join(os.Getenv("ProgramFiles"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("LocalAppData"), "Google", "Chrome", "Application", "chrome.exe"),
	}
	for _, candidate := range candidates {
		if candidate != "" {
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}
	}
	return exec.LookPath("chrome")
}

func (a *App) deleteAccount(w http.ResponseWriter, r *http.Request, id string) {
	_, err := a.db.ExecContext(r.Context(), `DELETE FROM channel_accounts WHERE id=?`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.audit("account.deleted", "warning", "", "account", id, "账号已删除", nil)
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (a *App) handleDeleteUnsupportedCheckinAccounts(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var input struct {
		Limit                  int   `json:"limit"`
		DryRun                 bool  `json:"dryRun"`
		IncludeLastUnsupported *bool `json:"includeLastUnsupported"`
	}
	_ = decodeJSON(r, &input)
	includeLastUnsupported := true
	if input.IncludeLastUnsupported != nil {
		includeLastUnsupported = *input.IncludeLastUnsupported
	}
	result, err := a.deleteUnsupportedCheckinAccounts(r.Context(), input.Limit, includeLastUnsupported, input.DryRun)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !result.DryRun && result.Deleted > 0 {
		a.notify("unsupported_checkin_accounts_deleted", "warning", "Unsupported check-in accounts deleted", fmt.Sprintf("Deleted %d accounts that cannot run check-ins.", result.Deleted), "account", "")
		a.audit("account.bulk_deleted_unsupported_checkin", "warning", "", "account", "", "Deleted unsupported check-in accounts.", map[string]interface{}{
			"matched":                result.Matched,
			"deleted":                result.Deleted,
			"limit":                  result.Limit,
			"hasMore":                result.HasMore,
			"includeLastUnsupported": includeLastUnsupported,
		})
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *App) deleteUnsupportedCheckinAccounts(ctx context.Context, limit int, includeLastUnsupported bool, dryRun bool) (unsupportedCheckinCleanupResult, error) {
	return a.accountCleanup.DeleteUnsupportedCheckinAccounts(ctx, limit, includeLastUnsupported, dryRun)
}

func (a *App) loadUnsupportedCheckinAccounts(ctx context.Context, limit int, includeLastUnsupported bool) ([]unsupportedCheckinAccountItem, bool, error) {
	return a.accountCleanup.LoadUnsupportedCheckinAccounts(ctx, limit, includeLastUnsupported)
}

func auditUpdatedAccountFields(siteName, baseURL, loginURL, kind, displayName, email, username, password string, clearPassword bool, apiKey string, clearAPIKey bool, cookie string, clearCookie bool, accessToken string, refreshToken string) []string {
	fields := []string{}
	add := func(name string, changed bool) {
		if changed {
			fields = append(fields, name)
		}
	}
	add("site", strings.TrimSpace(siteName) != "" || strings.TrimSpace(baseURL) != "" || strings.TrimSpace(loginURL) != "" || strings.TrimSpace(kind) != "")
	add("displayName", strings.TrimSpace(displayName) != "")
	add("email", strings.TrimSpace(email) != "")
	add("username", strings.TrimSpace(username) != "")
	add("password", strings.TrimSpace(password) != "" || clearPassword)
	add("apiKey", strings.TrimSpace(apiKey) != "" || clearAPIKey)
	add("cookie", strings.TrimSpace(cookie) != "" || clearCookie)
	add("accessToken", strings.TrimSpace(accessToken) != "")
	add("refreshToken", strings.TrimSpace(refreshToken) != "")
	return fields
}
