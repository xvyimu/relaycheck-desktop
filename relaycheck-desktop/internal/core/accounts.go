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
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	items, err := cachedRead(a, "accounts-list", shortReadCacheTTL, func() ([]ChannelAccount, error) {
		rows, err := a.db.QueryContext(r.Context(), `
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
		ORDER BY a.updated_at DESC
	`)
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
	var input struct {
		UpstreamSiteID string `json:"upstreamSiteId"`
		SiteName       string `json:"siteName"`
		BaseURL        string `json:"baseUrl"`
		LoginURL       string `json:"loginUrl"`
		Kind           string `json:"kind"`
		DisplayName    string `json:"displayName"`
		Email          string `json:"email"`
		Username       string `json:"username"`
		AuthType       string `json:"authType"`
		Password       string `json:"password"`
		Cookie         string `json:"cookie"`
		AccessToken    string `json:"accessToken"`
		RefreshToken   string `json:"refreshToken"`
		APIKey         string `json:"apiKey"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "账号参数不完整。")
		return
	}
	input.UpstreamSiteID = strings.TrimSpace(input.UpstreamSiteID)
	if input.UpstreamSiteID == "" && strings.TrimSpace(input.BaseURL) != "" {
		siteID, err := a.ensureManualAccountSite(r.Context(), input.SiteName, input.BaseURL, input.LoginURL, input.Kind)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		input.UpstreamSiteID = siteID
	}
	if input.UpstreamSiteID == "" {
		writeError(w, http.StatusBadRequest, "请选择已有站点，或填写自定义站点网址。")
		return
	}
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	if input.DisplayName == "" {
		input.DisplayName = defaultAccountDisplayName(input.Email, input.Username, input.APIKey)
	}
	if input.AuthType == "" {
		input.AuthType = inferAccountAuthType(input.Password, input.Cookie, input.AccessToken, input.RefreshToken, input.APIKey)
	}

	password, err := a.encryptText(input.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	cookie, err := a.encryptText(input.Cookie)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	access, err := a.encryptText(input.AccessToken)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	refresh, err := a.encryptText(input.RefreshToken)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	apiKey, err := a.encryptText(input.APIKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	id := newID()
	profilePath := ""
	status := "unknown"
	if input.AuthType == "browser_profile" || input.AuthType == "oauth_session" {
		profilePath = filepath.Join(a.dataDir, "browser-profiles", id)
		status = "manual_required"
	}
	apiKeyFingerprint := secretFingerprint(input.APIKey)
	_, err = a.db.ExecContext(r.Context(), `
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, email, username, auth_type, password_encrypted, cookie_encrypted, access_token_encrypted, refresh_token_encrypted, api_key_encrypted, api_key_fingerprint, api_key_status, browser_profile_path, login_status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, input.UpstreamSiteID, input.DisplayName, input.Email, input.Username, input.AuthType, password, cookie, access, refresh, apiKey, apiKeyFingerprint, statusFromKey(apiKeyFingerprint), profilePath, status, now(), now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.notify("account_created", "success", "账号已添加", input.DisplayName+" 已绑定。", "account", id)
	a.audit("account.created", "info", "", "account", id, "账号已添加："+input.DisplayName, map[string]interface{}{"authType": input.AuthType, "siteId": input.UpstreamSiteID, "apiKeyFingerprint": apiKeyFingerprint})
	writeJSON(w, http.StatusOK, map[string]string{"id": id})
}

func (a *App) ensureManualAccountSite(ctx context.Context, name string, rawBaseURL string, loginURL string, preferredKind string) (string, error) {
	baseURL := normalizeBaseURL(rawBaseURL)
	if baseURL == "" || (!strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://")) {
		return "", errorsText("请填写完整站点网址，例如 https://example.com。")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = firstNonEmpty(hostLabel(baseURL), baseURL)
	}
	if isExcludedRelaySite(name, baseURL) {
		return "", errorsText("该站点已被排除，不再作为中转站导入。")
	}

	loginURL = strings.TrimSpace(loginURL)
	var existingID string
	err := a.db.QueryRowContext(ctx, `SELECT id FROM upstream_sites WHERE base_url=? ORDER BY updated_at DESC LIMIT 1`, baseURL).Scan(&existingID)
	if err == nil {
		if loginURL != "" {
			_, err = a.db.ExecContext(ctx, `
				UPDATE upstream_sites
				SET login_url=?, updated_at=?
				WHERE id=?
			`, loginURL, now(), existingID)
			if err != nil {
				return "", err
			}
		}
		return existingID, nil
	}
	if err != sql.ErrNoRows {
		return "", err
	}

	detection := a.detectUpstream(ctx, baseURL)
	preferredKind = strings.ToLower(strings.TrimSpace(preferredKind))
	if isManagedRelayKind(preferredKind) {
		detection.Kind = preferredKind
		if detection.DetectionConfidence < 0.3 {
			detection.DetectionConfidence = 0.3
		}
	}
	if !isManagedRelayKind(detection.Kind) {
		return "", errorsText("该地址未识别为 NewAPI/OneAPI/Sub2API/魔改中转面板型中转站。可先在上游站点页查看识别详情，或手动指定后台类型后再添加。")
	}
	if loginURL != "" {
		detection.LoginURL = loginURL
	}

	channelID := newID()
	siteID := newID()
	detectionJSON := marshalDetection(&detection)
	createdAt := now()
	_, err = a.db.ExecContext(ctx, `
		INSERT INTO imported_channels (id, source_channel_id, name, base_url, status, upstream_kind, supports_checkin, supports_balance, supports_models, supports_pricing, raw_json, detection_json, last_detected_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, 'manual', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, channelID, "manual-"+channelID, name, detection.BaseURL, detection.Kind, boolInt(detection.SupportsCheckin), boolInt(detection.SupportsBalance), boolInt(detection.SupportsModels), boolInt(detection.SupportsPricing), `{"source":"manual-account"}`, detectionJSON, createdAt, createdAt, createdAt)
	if err != nil {
		return "", err
	}
	_, err = a.db.ExecContext(ctx, `
		INSERT INTO upstream_sites (id, channel_id, name, homepage_url, base_url, login_url, kind, detection_confidence, health_status, supports_checkin, supports_balance, supports_models, supports_pricing, detection_json, last_health_check_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, siteID, channelID, name, detection.HomepageURL, detection.BaseURL, detection.LoginURL, detection.Kind, detection.DetectionConfidence, detection.HealthStatus, boolInt(detection.SupportsCheckin), boolInt(detection.SupportsBalance), boolInt(detection.SupportsModels), boolInt(detection.SupportsPricing), detectionJSON, createdAt, createdAt, createdAt)
	if err != nil {
		return "", err
	}
	a.notify("upstream_site_created", "success", "上游站点已添加", name+" 已通过账号表单加入站点列表。", "upstream_site", siteID)
	return siteID, nil
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
	AccountID   string `json:"accountId"`
	AccountName string `json:"accountName,omitempty"`
	SiteName    string `json:"siteName,omitempty"`
	Status      string `json:"status"`
	Message     string `json:"message,omitempty"`
	URL         string `json:"url,omitempty"`
	DebugPort   int    `json:"debugPort,omitempty"`
	ProfilePath string `json:"profilePath,omitempty"`
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
	input.Limit = clampBatchLimit(input.Limit, 10)

	rows, err := a.db.QueryContext(r.Context(), `
		SELECT id FROM channel_accounts
		WHERE COALESCE(password_encrypted,'') <> ''
		  AND (
		    login_status IN ('expired','manual_required','unknown')
		    OR COALESCE(last_checkin_status,'') IN ('auth_expired','manual_required','failed')
		  )
		ORDER BY updated_at DESC
		LIMIT ?
	`, input.Limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	accountIDs := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			accountIDs = append(accountIDs, id)
		}
	}
	_ = rows.Close()

	results := []bulkPasswordLoginResult{}
	auths, _ := a.loadAccountAuths(r.Context(), accountIDs)
	for _, id := range accountIDs {
		var auth *accountAuthContext
		if loaded, ok := auths[id]; ok {
			auth = &loaded
		}
		results = append(results, a.retryPasswordLogin(r.Context(), id, auth))
	}
	successCount := 0
	for _, result := range results {
		if result.Status == "valid" {
			successCount++
		}
	}
	if len(results) > 0 {
		a.notify("bulk_password_login", "info", "批量密码重登完成", fmt.Sprintf("处理 %d 个账号，成功 %d 个。", len(results), successCount), "account", "")
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"processed": len(results),
		"success":   successCount,
		"failed":    len(results) - successCount,
		"results":   results,
	})
}

func (a *App) retryPasswordLogin(ctx context.Context, id string, auth *accountAuthContext) bulkPasswordLoginResult {
	if auth == nil {
		loaded, err := a.loadAccountAuth(ctx, id)
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
		if _, execErr := a.db.ExecContext(ctx, `UPDATE channel_accounts SET login_status='manual_required', last_validated_at=?, updated_at=? WHERE id=?`, now(), now(), id); execErr != nil {
			log.Printf("[accounts] password login status update to manual_required failed for account %s: %v", id, execErr)
		}
		return result
	}
	auth.Cookie = ""
	auth.AccessToken = ""
	auth.AuthUserID = ""
	if err := a.loginWithPassword(ctx, auth); err != nil {
		result.Status = "expired"
		result.Message = err.Error()
		if _, execErr := a.db.ExecContext(ctx, `UPDATE channel_accounts SET login_status='expired', last_validated_at=?, updated_at=? WHERE id=?`, now(), now(), id); execErr != nil {
			log.Printf("[accounts] password login status update to expired failed for account %s: %v", id, execErr)
		}
		return result
	}
	result.Status = "valid"
	result.Message = "密码登录成功，已保存新会话。"
	return result
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
	input.Limit = clampBatchLimit(input.Limit, 5)
	accountIDs := input.IDs
	if len(accountIDs) > input.Limit {
		accountIDs = accountIDs[:input.Limit]
	}
	if len(accountIDs) == 0 {
		rows, err := a.db.QueryContext(r.Context(), `
			SELECT id FROM channel_accounts
			WHERE login_status IN ('expired','manual_required','unknown')
			   OR COALESCE(last_checkin_status,'') IN ('auth_expired','manual_required','failed')
			ORDER BY updated_at DESC
			LIMIT ?
		`, input.Limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err == nil {
				accountIDs = append(accountIDs, id)
			}
		}
		_ = rows.Close()
	}

	results := []browserLoginOpenResult{}
	opened := 0
	auths, _ := a.loadAccountAuths(r.Context(), accountIDs)
	for _, id := range accountIDs {
		var auth *accountAuthContext
		if loaded, ok := auths[id]; ok {
			auth = &loaded
		}
		result := a.startBrowserLogin(r.Context(), id, auth)
		if result.Status == "opened" || result.Status == "already_open" {
			opened++
		}
		results = append(results, result)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"processed": len(results),
		"opened":    opened,
		"failed":    len(results) - opened,
		"results":   results,
	})
}

func (a *App) handleBulkFinishBrowserLogin(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var input struct {
		IDs []string `json:"ids"`
	}
	_ = decodeJSON(r, &input)
	accountIDs := input.IDs
	if len(accountIDs) == 0 {
		a.browserSessions.Range(func(id string, _ BrowserLoginSession) {
			accountIDs = append(accountIDs, id)
		})
	}
	results := []browserLoginSaveResult{}
	saved := 0
	if len(accountIDs) > 10 {
		accountIDs = accountIDs[:10]
	}
	auths, _ := a.loadAccountAuths(r.Context(), accountIDs)
	for _, id := range accountIDs {
		var auth *accountAuthContext
		if loaded, ok := auths[id]; ok {
			auth = &loaded
		}
		result := a.saveBrowserLoginSession(r.Context(), id, auth)
		if result.Status == "saved" {
			saved++
		}
		results = append(results, result)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"processed": len(results),
		"saved":     saved,
		"failed":    len(results) - saved,
		"results":   results,
	})
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
	siteName = strings.TrimSpace(siteName)
	baseURL := normalizeBaseURL(rawBaseURL)
	loginURL = strings.TrimSpace(loginURL)
	preferredKind = strings.ToLower(strings.TrimSpace(preferredKind))
	if preferredKind == "auto" {
		preferredKind = ""
	}
	siteScope = strings.ToLower(strings.TrimSpace(siteScope))
	if siteScope == "" {
		siteScope = "current"
	}

	currentBaseURL := normalizeBaseURL(current.UpstreamSiteBaseURL)
	if baseURL == "" {
		if siteName != "" || loginURL != "" || isManagedRelayKind(preferredKind) {
			return current.UpstreamSiteID, false, a.updateAccountSiteMetadata(ctx, current.UpstreamSiteID, siteName, loginURL, preferredKind)
		}
		return current.UpstreamSiteID, false, nil
	}
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		return "", false, errorsText("请填写完整站点网址，例如 https://example.com。")
	}
	if isExcludedRelaySite(siteName, baseURL) {
		return "", false, errorsText("该站点已被排除，不再作为中转站导入。")
	}
	if siteScope == "shared" {
		return a.updateSharedAccountSite(ctx, current, siteName, baseURL, loginURL, preferredKind)
	}
	if currentBaseURL != "" && baseURL == currentBaseURL {
		return current.UpstreamSiteID, false, a.updateAccountSiteMetadata(ctx, current.UpstreamSiteID, siteName, loginURL, preferredKind)
	}

	nextSiteName := firstNonEmpty(siteName, current.UpstreamSiteName, hostLabel(baseURL), baseURL)
	siteID, err := a.ensureManualAccountSite(ctx, nextSiteName, baseURL, loginURL, preferredKind)
	if err != nil {
		return "", false, err
	}
	if err := a.updateAccountSiteMetadata(ctx, siteID, nextSiteName, loginURL, preferredKind); err != nil {
		return "", false, err
	}
	return siteID, siteID != current.UpstreamSiteID, nil
}

func (a *App) updateSharedAccountSite(ctx context.Context, current ChannelAccount, siteName string, baseURL string, loginURL string, kind string) (string, bool, error) {
	currentBaseURL := normalizeBaseURL(current.UpstreamSiteBaseURL)
	nextSiteName := firstNonEmpty(strings.TrimSpace(siteName), current.UpstreamSiteName, hostLabel(baseURL), baseURL)
	if currentBaseURL != "" && baseURL == currentBaseURL {
		return current.UpstreamSiteID, false, a.updateAccountSiteMetadata(ctx, current.UpstreamSiteID, nextSiteName, loginURL, kind)
	}

	var existingID string
	err := a.db.QueryRowContext(ctx, `
		SELECT id
		FROM upstream_sites
		WHERE base_url=? AND id<>?
		ORDER BY updated_at DESC
		LIMIT 1
	`, baseURL, current.UpstreamSiteID).Scan(&existingID)
	if err == nil {
		if err := a.updateAccountSiteMetadata(ctx, existingID, nextSiteName, loginURL, kind); err != nil {
			return "", false, err
		}
		if _, err := a.db.ExecContext(ctx, `
			UPDATE channel_accounts
			SET upstream_site_id=?, updated_at=?
			WHERE upstream_site_id=?
		`, existingID, now(), current.UpstreamSiteID); err != nil {
			return "", false, err
		}
		return existingID, true, nil
	}
	if err != sql.ErrNoRows {
		return "", false, err
	}

	if err := a.updateAccountSiteAddress(ctx, current.UpstreamSiteID, nextSiteName, baseURL, loginURL, kind); err != nil {
		return "", false, err
	}
	return current.UpstreamSiteID, false, nil
}

func (a *App) updateAccountSiteAddress(ctx context.Context, siteID string, siteName string, baseURL string, loginURL string, kind string) error {
	siteName = strings.TrimSpace(siteName)
	loginURL = strings.TrimSpace(loginURL)
	kind = strings.ToLower(strings.TrimSpace(kind))

	sets := []string{"base_url=?", "homepage_url=?", "updated_at=?"}
	args := []interface{}{baseURL, baseURL, now()}
	if siteName != "" {
		sets = append(sets, "name=?")
		args = append(args, siteName)
	}
	if loginURL != "" {
		sets = append(sets, "login_url=?")
		args = append(args, loginURL)
	}
	if isManagedRelayKind(kind) {
		sets = append(sets, "kind=?")
		args = append(args, kind)
	}
	args = append(args, siteID)
	if _, err := a.db.ExecContext(ctx, "UPDATE upstream_sites SET "+strings.Join(sets, ", ")+" WHERE id=?", args...); err != nil {
		return err
	}

	channelSets := []string{"base_url=?", "updated_at=?"}
	channelArgs := []interface{}{baseURL, now()}
	if siteName != "" {
		channelSets = append(channelSets, "name=?")
		channelArgs = append(channelArgs, siteName)
	}
	if isManagedRelayKind(kind) {
		channelSets = append(channelSets, "upstream_kind=?")
		channelArgs = append(channelArgs, kind)
	}
	channelArgs = append(channelArgs, siteID)
	_, err := a.db.ExecContext(ctx, "UPDATE imported_channels SET "+strings.Join(channelSets, ", ")+" WHERE id=(SELECT channel_id FROM upstream_sites WHERE id=?)", channelArgs...)
	return err
}

func (a *App) updateAccountSiteMetadata(ctx context.Context, siteID string, siteName string, loginURL string, kind string) error {
	siteName = strings.TrimSpace(siteName)
	loginURL = strings.TrimSpace(loginURL)
	kind = strings.ToLower(strings.TrimSpace(kind))
	if siteName == "" && loginURL == "" && !isManagedRelayKind(kind) {
		return nil
	}

	sets := []string{"updated_at=?"}
	args := []interface{}{now()}
	if siteName != "" {
		sets = append(sets, "name=?")
		args = append(args, siteName)
	}
	if loginURL != "" {
		sets = append(sets, "login_url=?")
		args = append(args, loginURL)
	}
	if isManagedRelayKind(kind) {
		sets = append(sets, "kind=?")
		args = append(args, kind)
	}
	args = append(args, siteID)
	if _, err := a.db.ExecContext(ctx, "UPDATE upstream_sites SET "+strings.Join(sets, ", ")+" WHERE id=?", args...); err != nil {
		return err
	}

	channelSets := []string{"updated_at=?"}
	channelArgs := []interface{}{now()}
	if siteName != "" {
		channelSets = append(channelSets, "name=?")
		channelArgs = append(channelArgs, siteName)
	}
	if isManagedRelayKind(kind) {
		channelSets = append(channelSets, "upstream_kind=?")
		channelArgs = append(channelArgs, kind)
	}
	channelArgs = append(channelArgs, siteID)
	_, err := a.db.ExecContext(ctx, "UPDATE imported_channels SET "+strings.Join(channelSets, ", ")+" WHERE id=(SELECT channel_id FROM upstream_sites WHERE id=?)", channelArgs...)
	return err
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
	result := a.startBrowserLogin(r.Context(), id, nil)
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
	result := a.saveBrowserLoginSession(r.Context(), id, nil)
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
	if auth == nil {
		loaded, err := a.loadAccountAuth(ctx, id)
		if err != nil {
			return browserLoginOpenResult{Status: "failed", Message: err.Error()}
		}
		auth = &loaded
	}
	var accountName, siteName, baseURL, loginURL, profilePath string
	err := a.db.QueryRowContext(ctx, `
		SELECT a.display_name, s.name, s.base_url, COALESCE(s.login_url,''), COALESCE(a.browser_profile_path,'')
		FROM channel_accounts a
		JOIN upstream_sites s ON s.id = a.upstream_site_id
		WHERE a.id = ?
	`, id).Scan(&accountName, &siteName, &baseURL, &loginURL, &profilePath)
	result := browserLoginOpenResult{AccountID: id, AccountName: accountName, SiteName: siteName}
	if err == sql.ErrNoRows {
		result.Status = "failed"
		result.Message = "账号不存在。"
		return result
	}
	if err != nil {
		result.Status = "failed"
		result.Message = err.Error()
		return result
	}

	if session, ok := a.browserSessions.Get(id); ok {
		result.Status = "already_open"
		result.Message = "该账号网页登录窗口已经打开。"
		result.DebugPort = session.Port
		result.ProfilePath = profilePath
		return result
	}
	usedPorts := map[int]bool{}
	for _, session := range a.browserSessions.List() {
		usedPorts[session.Port] = true
	}

	if profilePath == "" {
		profilePath = filepath.Join(a.dataDir, "browser-profiles", id)
	}
	if err := os.MkdirAll(profilePath, 0o700); err != nil {
		result.Status = "failed"
		result.Message = err.Error()
		return result
	}

	targetURL := loginURL
	if targetURL == "" {
		targetURL = strings.TrimRight(baseURL, "/") + "/login"
	}
	targetURL = resolveLoginTargetURL(baseURL, targetURL)
	port, err := freeDebugPort(usedPorts)
	if err != nil {
		result.Status = "failed"
		result.Message = err.Error()
		return result
	}
	chromePath, err := findChrome()
	if err != nil {
		result.Status = "failed"
		result.Message = err.Error()
		return result
	}

	chromeArgs := []string{
		"--remote-debugging-port=" + strconv.Itoa(port),
		"--user-data-dir=" + profilePath,
		"--no-first-run",
		"--no-default-browser-check",
		targetURL,
	}
	if proxyArgs := a.chromeProxyArgs(); len(proxyArgs) > 0 {
		chromeArgs = append(chromeArgs[:len(chromeArgs)-1], append(proxyArgs, targetURL)...)
	}
	cmd := exec.Command(chromePath, chromeArgs...)
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = hiddenProcessAttr()
	}
	if err := cmd.Start(); err != nil {
		result.Status = "failed"
		result.Message = err.Error()
		return result
	}

	a.browserSessions.Set(id, BrowserLoginSession{AccountID: id, Port: port, StartedAt: time.Now(), PID: cmd.Process.Pid})

	// Watchdog: clean up the session entry when the Chrome process exits so
	// the in-memory map doesn't leak entries for crashed or user-closed
	// browser windows.
	go func(accountID string, proc *os.Process) {
		_, _ = proc.Wait()
		a.browserSessions.DeleteIfPIDMatches(accountID, proc.Pid)
	}(id, cmd.Process)

	if _, execErr := a.db.ExecContext(ctx, `
		UPDATE channel_accounts
		SET auth_type='browser_profile', browser_profile_path=?, login_status='manual_required', updated_at=?
		WHERE id=?
	`, profilePath, now(), id); execErr != nil {
		log.Printf("[accounts] browser login profile path update failed for account %s: %v", id, execErr)
	}
	a.audit("browser_auth.opened", "info", "", "account", id, "网页登录授权窗口已打开。", map[string]interface{}{"accountName": accountName, "siteName": siteName})

	result.Status = "opened"
	result.Message = "网页登录窗口已打开，请完成登录后保存授权。"
	result.URL = targetURL
	result.DebugPort = port
	result.ProfilePath = profilePath
	return result
}

func resolveLoginTargetURL(baseURL string, loginURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	loginURL = strings.TrimSpace(loginURL)
	if loginURL == "" {
		loginURL = "/login"
	}
	if strings.HasPrefix(loginURL, "http://") || strings.HasPrefix(loginURL, "https://") {
		return loginURL
	}
	base, err := url.Parse(strings.TrimRight(baseURL, "/") + "/")
	if err != nil || base.Scheme == "" || base.Host == "" {
		return loginURL
	}
	parsed, err := url.Parse(loginURL)
	if err != nil {
		return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(loginURL, "/")
	}
	resolved := base.ResolveReference(parsed)
	if resolved.Scheme != "http" && resolved.Scheme != "https" {
		return base.ResolveReference(&url.URL{Path: "/login"}).String()
	}
	if !strings.EqualFold(resolved.Host, base.Host) {
		return base.ResolveReference(&url.URL{Path: "/login"}).String()
	}
	return resolved.String()
}

func (a *App) saveBrowserLoginSession(ctx context.Context, id string, auth *accountAuthContext) browserLoginSaveResult {
	if auth == nil {
		loaded, err := a.loadAccountAuth(ctx, id)
		if err != nil {
			return browserLoginSaveResult{Status: "failed", Message: err.Error()}
		}
		auth = &loaded
	}
	var accountName, siteName string
	_ = a.db.QueryRowContext(ctx, `
		SELECT a.display_name, s.name
		FROM channel_accounts a
		JOIN upstream_sites s ON s.id = a.upstream_site_id
		WHERE a.id = ?
	`, id).Scan(&accountName, &siteName)
	result := browserLoginSaveResult{AccountID: id, AccountName: accountName, SiteName: siteName}

	session, ok := a.browserSessions.Get(id)
	if !ok {
		result.Status = "missing"
		result.Message = "没有正在进行的网页登录会话，请先点击网页登录。"
		return result
	}

	cookies, userAgent, err := readChromeSession(session.Port)
	if err != nil {
		result.Status = "failed"
		result.Message = err.Error()
		return result
	}
	if len(cookies) == 0 {
		result.Status = "failed"
		result.Message = "未检测到 Cookie，请先在浏览器中完成登录。"
		return result
	}

	cookieHeader := buildCookieHeader(cookies)
	encryptedCookie, err := a.encryptText(cookieHeader)
	if err != nil {
		result.Status = "failed"
		result.Message = err.Error()
		return result
	}
	_, err = a.db.ExecContext(ctx, `
		UPDATE channel_accounts
		SET cookie_encrypted=?, user_agent=?, login_status='valid', last_login_at=?, last_validated_at=?, cookie_expiry_at=?, updated_at=?
		WHERE id=?
	`, encryptedCookie, userAgent, now(), now(), estimateCookieExpiry(), now(), id)
	if err != nil {
		result.Status = "failed"
		result.Message = err.Error()
		return result
	}

	a.browserSessions.Delete(id)
	a.notify("browser_login_saved", "success", "网页登录态已保存", fmt.Sprintf("%s 已保存 %d 个 Cookie。", firstNonEmpty(accountName, id), len(cookies)), "account", id)
	a.audit("browser_auth.connected", "info", "", "account", id, "网页登录授权已保存。", map[string]interface{}{"accountName": accountName, "siteName": siteName, "cookieCount": len(cookies)})

	result.Status = "saved"
	result.Message = "网页登录态已保存。"
	result.CookieCount = len(cookies)
	result.CookiePreview = maskSecret(cookieHeader)
	return result
}

func (a *App) testAccountLogin(w http.ResponseWriter, r *http.Request, id string) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var baseURL, cookieEncrypted, accessEncrypted, apiKeyEncrypted, userAgent string
	err := a.db.QueryRowContext(r.Context(), `
		SELECT s.base_url, COALESCE(a.cookie_encrypted,''), COALESCE(a.access_token_encrypted,''), COALESCE(a.api_key_encrypted,''), COALESCE(a.user_agent,'')
		FROM channel_accounts a
		JOIN upstream_sites s ON s.id = a.upstream_site_id
		WHERE a.id = ?
	`, id).Scan(&baseURL, &cookieEncrypted, &accessEncrypted, &apiKeyEncrypted, &userAgent)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "账号不存在。")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	req, _ := http.NewRequestWithContext(r.Context(), http.MethodGet, normalizeBaseURL(baseURL)+"/api/user/self", nil)
	if userAgent != "" {
		req.Header.Set("user-agent", userAgent)
	}
	if cookie, _ := a.decryptText(cookieEncrypted); cookie != "" {
		req.Header.Set("cookie", cookie)
	}
	if token, _ := a.decryptText(accessEncrypted); token != "" {
		if !strings.HasPrefix(strings.ToLower(token), "bearer ") {
			token = "Bearer " + token
		}
		req.Header.Set("authorization", token)
	}
	if key, _ := a.decryptText(apiKeyEncrypted); key != "" && req.Header.Get("authorization") == "" {
		req.Header.Set("authorization", "Bearer "+key)
	}

	status := "unknown"
	httpStatus := 0
	if resp, err := a.doHTTP(req); err == nil {
		httpStatus = resp.StatusCode
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			status = "valid"
		} else if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			status = "expired"
		}
	}
	if _, execErr := a.db.ExecContext(r.Context(), `UPDATE channel_accounts SET login_status=?, last_validated_at=?, updated_at=? WHERE id=?`, status, now(), now(), id); execErr != nil {
		log.Printf("[accounts] test login status update failed for account %s: %v", id, execErr)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": status, "httpStatus": httpStatus})
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
	if auth == nil {
		loaded, err := a.loadAccountAuth(ctx, id)
		if err != nil {
			return apiKeyTestResult{AccountID: id, Status: "failed", Message: err.Error()}
		}
		auth = &loaded
	}
	result := apiKeyTestResult{
		AccountID:   auth.AccountID,
		AccountName: auth.AccountName,
		SiteName:    auth.UpstreamSite,
		Fingerprint: secretFingerprint(auth.APIKey),
	}
	if strings.TrimSpace(auth.APIKey) == "" {
		result.Status = "missing"
		result.Message = "该账号没有保存 API Key。"
		return result
	}
	auth.Cookie = ""
	auth.AccessToken = ""
	auth.AuthUserID = ""

	modelsStatus, modelsBody, modelsErr := a.callAccountAPI(ctx, *auth, http.MethodGet, "/v1/models", nil)
	result.HTTPStatus = modelsStatus
	result.Path = "/v1/models"
	if modelsErr != nil {
		result.Status = "unknown"
		result.Message = modelsErr.Error()
	} else if modelsStatus == http.StatusUnauthorized || modelsStatus == http.StatusForbidden {
		result.Status = "expired"
		result.Message = firstNonEmpty(extractMessage(modelsBody), "API Key 无权访问 /v1/models。")
	} else if modelsStatus >= 200 && modelsStatus < 300 {
		models := parseModelIDs(modelsBody)
		result.Status = "valid"
		result.ModelCount = len(models)
		result.SampleModels = limitStrings(models, 8)
		result.Message = fmt.Sprintf("/v1/models 返回 HTTP %d，识别到 %d 个模型。", modelsStatus, len(models))
		if len(models) > 0 {
			result.TestedModel = chooseModelForSpeedTest(models)
			a.speedTestAPIKeyModel(ctx, auth, &result)
		} else {
			result.ModelTestMessage = "模型列表为空，未执行可用性测速。"
		}
	} else if modelsStatus == http.StatusNotFound || modelsStatus == http.StatusMethodNotAllowed {
		result.Status = "unknown"
		result.Message = firstNonEmpty(extractMessage(modelsBody), "/v1/models 不可用，继续用面板接口判断 Key。")
	} else {
		result.Status = "unknown"
		result.Message = firstNonEmpty(extractMessage(modelsBody), fmt.Sprintf("/v1/models 返回 HTTP %d。", modelsStatus))
	}

	if result.Status == "unknown" {
		probes := []string{"/api/user/self", "/api/token/"}
		for _, path := range probes {
			status, body, err := a.callAccountAPI(ctx, *auth, http.MethodGet, path, nil)
			if err != nil {
				result.Path = path
				result.Message = err.Error()
				continue
			}
			result.HTTPStatus = status
			result.Path = path
			result.Message = firstNonEmpty(extractMessage(body), fmt.Sprintf("%s 返回 HTTP %d", path, status))
			if status == http.StatusOK {
				result.Status = "valid"
				break
			}
			if status == http.StatusUnauthorized || status == http.StatusForbidden {
				result.Status = "expired"
				break
			}
			if status == http.StatusNotFound || status == http.StatusMethodNotAllowed {
				continue
			}
			if status >= 200 && status < 300 {
				result.Status = "valid"
				break
			}
		}
	}
	if result.Status == "" {
		result.Status = "unknown"
		result.Message = "没有找到可判断 API Key 的接口。"
	}
	if result.Status == "valid" && result.ModelCount > 0 && result.TestedModel != "" {
		if result.ModelUsable {
			result.Message = fmt.Sprintf("密钥有效，模型 %s 可用，测速 %dms。", result.TestedModel, result.ModelTestLatencyMs)
		} else if result.ModelTestMessage != "" {
			result.Message = "密钥可读取模型，但模型调用未通过：" + result.ModelTestMessage
		}
	}
	result.Message = sanitizeAPIKeyDiagnostic(result.Message, auth.APIKey)
	result.ModelTestMessage = sanitizeAPIKeyDiagnostic(result.ModelTestMessage, auth.APIKey)
	sampleModelsJSON := marshalStringSlice(limitStrings(result.SampleModels, 8))
	if _, execErr := a.db.ExecContext(ctx, `
		UPDATE channel_accounts
		SET api_key_fingerprint=?, api_key_status=?, api_key_last_checked_at=?,
		    api_key_model_count=?, api_key_sample_models_json=?, api_key_test_model=?,
		    api_key_model_usable=?, api_key_latency_ms=?, api_key_test_http_status=?,
		    api_key_test_message=?, api_key_test_path=?,
		    login_status=CASE WHEN ?='valid' THEN 'valid' WHEN ?='expired' THEN 'expired' ELSE login_status END,
		    last_validated_at=?, updated_at=?
		WHERE id=?
	`, result.Fingerprint, result.Status, now(), result.ModelCount, sampleModelsJSON, result.TestedModel, boolInt(result.ModelUsable), result.ModelTestLatencyMs, result.ModelTestHTTPStatus, result.ModelTestMessage, result.ModelTestPath, result.Status, result.Status, now(), now(), id); execErr != nil {
		log.Printf("[accounts] api key test result update failed for account %s: %v", id, execErr)
	}
	return result
}

func (a *App) speedTestAPIKeyModel(ctx context.Context, auth *accountAuthContext, result *apiKeyTestResult) {
	if strings.TrimSpace(result.TestedModel) == "" {
		return
	}
	payload := map[string]interface{}{
		"model":       result.TestedModel,
		"messages":    []map[string]string{{"role": "user", "content": "ping"}},
		"max_tokens":  1,
		"temperature": 0,
		"stream":      false,
	}
	body, _ := json.Marshal(payload)
	started := time.Now()
	status, responseBody, err := a.callAccountAPIWithTimeout(ctx, *auth, http.MethodPost, "/v1/chat/completions", body, 12*time.Second)
	result.ModelTestLatencyMs = time.Since(started).Milliseconds()
	result.ModelTestHTTPStatus = status
	result.ModelTestPath = "/v1/chat/completions"
	if err != nil {
		result.ModelTestMessage = err.Error()
		return
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		result.Status = "expired"
		result.ModelTestMessage = firstNonEmpty(extractMessage(responseBody), "模型调用未授权。")
		return
	}
	if status < 200 || status >= 300 {
		result.ModelTestMessage = firstNonEmpty(extractMessage(responseBody), fmt.Sprintf("模型调用返回 HTTP %d。", status))
		return
	}
	if responseExplicitlyFailed(responseBody) {
		result.ModelTestMessage = firstNonEmpty(extractMessage(responseBody), "模型调用返回失败。")
		return
	}
	result.ModelUsable = true
	result.ModelTestMessage = firstNonEmpty(extractMessage(responseBody), "模型调用成功。")
}

func (a *App) callAccountAPIWithTimeout(ctx context.Context, auth accountAuthContext, method string, path string, body []byte, timeout time.Duration) (int, string, error) {
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	baseURL, err := safeNormalizeBaseURL(requestCtx, auth.BaseURL, a.externalURLPolicy())
	if err != nil {
		return 0, "", err
	}
	var reader io.Reader
	if body != nil {
		reader = strings.NewReader(string(body))
	}
	req, err := http.NewRequestWithContext(requestCtx, method, baseURL+path, reader)
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("user-agent", firstNonEmpty(auth.UserAgent, "RelayCheck-Desktop/0.1"))
	req.Header.Set("accept", "application/json, text/plain, */*")
	if body != nil {
		req.Header.Set("content-type", "application/json")
	}
	if auth.APIKey != "" {
		token := auth.APIKey
		if !strings.HasPrefix(strings.ToLower(token), "bearer ") {
			token = "Bearer " + token
		}
		req.Header.Set("authorization", token)
	}
	resp, err := a.doHTTPWithTimeout(req, timeout+time.Second)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	content, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	return resp.StatusCode, string(content), nil
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
	_ = a.db.QueryRowContext(r.Context(), `SELECT COALESCE(browser_profile_path,'') FROM channel_accounts WHERE id=?`, id).Scan(&profilePath)
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
	resp, err := http.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/json/list")
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

type unsupportedCheckinAccountItem struct {
	AccountID         string `json:"accountId"`
	AccountName       string `json:"accountName"`
	UpstreamSiteID    string `json:"upstreamSiteId"`
	UpstreamSiteName  string `json:"upstreamSiteName"`
	UpstreamSiteKind  string `json:"upstreamSiteKind"`
	LastCheckinStatus string `json:"lastCheckinStatus,omitempty"`
	Reason            string `json:"reason"`
}

type unsupportedCheckinCleanupResult struct {
	Matched                int                             `json:"matched"`
	Deleted                int                             `json:"deleted"`
	Limit                  int                             `json:"limit"`
	HasMore                bool                            `json:"hasMore"`
	DryRun                 bool                            `json:"dryRun"`
	IncludeLastUnsupported bool                            `json:"includeLastUnsupported"`
	Items                  []unsupportedCheckinAccountItem `json:"items"`
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
	limit = clampBatchLimit(limit, 10)
	items, hasMore, err := a.loadUnsupportedCheckinAccounts(ctx, limit, includeLastUnsupported)
	if err != nil {
		return unsupportedCheckinCleanupResult{}, err
	}
	result := unsupportedCheckinCleanupResult{
		Matched:                len(items),
		Limit:                  limit,
		HasMore:                hasMore,
		DryRun:                 dryRun,
		IncludeLastUnsupported: includeLastUnsupported,
		Items:                  items,
	}
	if dryRun || len(items) == 0 {
		return result, nil
	}

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if len(items) > 0 {
		ids := make([]interface{}, 0, len(items))
		for _, item := range items {
			ids = append(ids, item.AccountID)
		}
		placeholders := strings.Repeat("?,", len(ids))
		placeholders = placeholders[:len(placeholders)-1]
		if _, err := tx.ExecContext(ctx, `DELETE FROM checkin_logs WHERE account_id IN (`+placeholders+`)`, ids...); err != nil {
			return result, err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM balance_snapshots WHERE account_id IN (`+placeholders+`)`, ids...); err != nil {
			return result, err
		}
		deleted, err := tx.ExecContext(ctx, `DELETE FROM channel_accounts WHERE id IN (`+placeholders+`)`, ids...)
		if err != nil {
			return result, err
		}
		if affected, _ := deleted.RowsAffected(); affected > 0 {
			result.Deleted = int(affected)
		}
	}
	if err := tx.Commit(); err != nil {
		return result, err
	}
	committed = true
	a.invalidateReadCache()
	return result, nil
}

func (a *App) loadUnsupportedCheckinAccounts(ctx context.Context, limit int, includeLastUnsupported bool) ([]unsupportedCheckinAccountItem, bool, error) {
	limit = clampBatchLimit(limit, 10)
	where := `s.supports_checkin = 0`
	if includeLastUnsupported {
		where = `(` + where + ` OR lower(COALESCE(a.last_checkin_status,'')) = 'unsupported')`
	}
	rows, err := a.db.QueryContext(ctx, `
		SELECT a.id, a.display_name, s.id, s.name, s.kind, COALESCE(a.last_checkin_status,''),
		       CASE
		         WHEN s.supports_checkin = 0 THEN 'site_not_support_checkin'
		         ELSE 'last_checkin_unsupported'
		       END
		FROM channel_accounts a
		JOIN upstream_sites s ON s.id = a.upstream_site_id
		WHERE `+where+`
		ORDER BY CASE WHEN s.supports_checkin = 0 THEN 0 ELSE 1 END, a.updated_at DESC
		LIMIT ?
	`, limit+1)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	items := []unsupportedCheckinAccountItem{}
	for rows.Next() {
		var item unsupportedCheckinAccountItem
		if err := rows.Scan(&item.AccountID, &item.AccountName, &item.UpstreamSiteID, &item.UpstreamSiteName, &item.UpstreamSiteKind, &item.LastCheckinStatus, &item.Reason); err != nil {
			return nil, false, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	return items, hasMore, nil
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
