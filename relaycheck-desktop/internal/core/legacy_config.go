package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type legacySiteConfig struct {
	Name       string `json:"name"`
	SiteName   string `json:"site_name"`
	BaseURL    string `json:"base_url"`
	Username   string `json:"username"`
	Email      string `json:"email"`
	Password   string `json:"password"`
	LoginURL   string `json:"login_url"`
	CheckinURL string `json:"checkin_url"`
	BalanceURL string `json:"balance_url"`
}

func (a *App) handleLegacyConfigImport(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var input struct {
		ConfigContent string `json:"configContent"`
		FileName      string `json:"fileName"`
	}
	if err := decodeJSON(r, &input); err != nil || strings.TrimSpace(input.ConfigContent) == "" {
		writeError(w, http.StatusBadRequest, "旧配置内容不能为空。")
		return
	}
	result, err := a.importLegacyConfig(r.Context(), input.ConfigContent, input.FileName)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	a.audit("import.legacy_config", "info", "", "upstream_site", stringFromResult(result, "siteId"), "旧配置导入完成。", map[string]interface{}{
		"siteCreated":     boolFromResult(result, "siteCreated"),
		"accountImported": boolFromResult(result, "accountImported"),
		"hasCheckinRule":  boolFromResult(result, "hasCheckinRule"),
		"hasBalanceRule":  boolFromResult(result, "hasBalanceRule"),
	})
	writeJSON(w, http.StatusOK, result)
}

func (a *App) importLegacyConfig(ctx context.Context, content string, fileName string) (map[string]interface{}, error) {
	var cfg legacySiteConfig
	if err := json.Unmarshal([]byte(content), &cfg); err != nil {
		return nil, fmt.Errorf("旧配置 JSON 解析失败：%w", err)
	}

	baseURL := normalizeBaseURL(firstNonEmpty(cfg.BaseURL, cfg.LoginURL, cfg.CheckinURL, cfg.BalanceURL))
	if baseURL == "" {
		return nil, errorsText("旧配置里没有可识别的 base_url/login_url/checkin_url。")
	}
	siteName := firstNonEmpty(cfg.SiteName, cfg.Name, hostLabel(baseURL), fileName)
	loginURL := strings.TrimSpace(cfg.LoginURL)
	checkinConfig := ""
	if strings.TrimSpace(cfg.CheckinURL) != "" {
		checkinConfig = mustJSON(map[string]string{
			"method": http.MethodPost,
			"url":    strings.TrimSpace(cfg.CheckinURL),
			"path":   pathFromMaybeURL(cfg.CheckinURL),
		})
	}
	balanceConfig := ""
	if strings.TrimSpace(cfg.BalanceURL) != "" {
		balanceConfig = mustJSON(map[string]string{
			"method": http.MethodGet,
			"url":    strings.TrimSpace(cfg.BalanceURL),
			"path":   pathFromMaybeURL(cfg.BalanceURL),
		})
	}

	siteID, created, err := a.upsertLegacySite(ctx, siteName, baseURL, loginURL, checkinConfig, balanceConfig, content)
	if err != nil {
		return nil, err
	}

	accountID := ""
	accountImported := false
	loginName := firstNonEmpty(cfg.Email, cfg.Username)
	if loginName != "" && strings.TrimSpace(cfg.Password) != "" {
		accountID, accountImported, err = a.importLegacyAccount(ctx, siteID, siteName, cfg.Email, cfg.Username, cfg.Password)
		if err != nil {
			return nil, err
		}
	}

	a.notify("legacy_config_imported", "success", "旧配置导入完成", fmt.Sprintf("%s 已导入兼容配置。", siteName), "upstream_site", siteID)
	return map[string]interface{}{
		"siteId":          siteID,
		"siteCreated":     created,
		"accountId":       accountID,
		"accountImported": accountImported,
		"hasCheckinRule":  checkinConfig != "",
		"hasBalanceRule":  balanceConfig != "",
		"baseUrl":         baseURL,
	}, nil
}

func (a *App) upsertLegacySite(ctx context.Context, name string, baseURL string, loginURL string, checkinConfig string, balanceConfig string, raw string) (string, bool, error) {
	detectionJSON := mustJSON(UpstreamDetection{
		BaseURL:             baseURL,
		HomepageURL:         baseURL,
		LoginURL:            firstNonEmpty(loginURL, strings.TrimRight(baseURL, "/")+"/login"),
		Kind:                "unknown",
		HealthStatus:        "unknown",
		DetectionConfidence: 0.1,
		SupportsCheckin:     checkinConfig != "",
		SupportsBalance:     balanceConfig != "",
		MatchedSignals:      []string{"legacy-config-import"},
	})
	var siteID string
	err := a.db.QueryRowContext(ctx, `SELECT id FROM upstream_sites WHERE base_url=? ORDER BY updated_at DESC LIMIT 1`, baseURL).Scan(&siteID)
	if err == nil {
		_, err = a.db.ExecContext(ctx, `
			UPDATE upstream_sites
			SET name=CASE WHEN name='' THEN ? ELSE name END,
			    login_url=CASE WHEN ? <> '' THEN ? ELSE login_url END,
			    checkin_config_json=CASE WHEN ? <> '' THEN ? ELSE checkin_config_json END,
			    balance_config_json=CASE WHEN ? <> '' THEN ? ELSE balance_config_json END,
			    supports_checkin=CASE WHEN ? <> '' THEN 1 ELSE supports_checkin END,
			    supports_balance=CASE WHEN ? <> '' THEN 1 ELSE supports_balance END,
			    detection_json=CASE WHEN COALESCE(detection_json,'')='' THEN ? ELSE detection_json END,
			    updated_at=?
			WHERE id=?
		`, name, loginURL, loginURL, checkinConfig, checkinConfig, balanceConfig, balanceConfig, checkinConfig, balanceConfig, detectionJSON, now(), siteID)
		return siteID, false, err
	}
	if err != sql.ErrNoRows {
		return "", false, err
	}

	channelID := newID()
	rawJSON := mustJSON(map[string]interface{}{"source": "legacy_config", "raw": raw})
	_, err = a.db.ExecContext(ctx, `
		INSERT INTO imported_channels (id, source_channel_id, name, base_url, status, upstream_kind, supports_checkin, supports_balance, raw_json, detection_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, 'legacy', 'unknown', ?, ?, ?, ?, ?, ?)
	`, channelID, "legacy-"+channelID, name, baseURL, boolInt(checkinConfig != ""), boolInt(balanceConfig != ""), rawJSON, detectionJSON, now(), now())
	if err != nil {
		return "", false, err
	}

	siteID = newID()
	_, err = a.db.ExecContext(ctx, `
		INSERT INTO upstream_sites (id, channel_id, name, homepage_url, base_url, login_url, kind, health_status, supports_checkin, supports_balance, checkin_config_json, balance_config_json, detection_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 'unknown', 'unknown', ?, ?, ?, ?, ?, ?, ?)
	`, siteID, channelID, name, baseURL, baseURL, loginURL, boolInt(checkinConfig != ""), boolInt(balanceConfig != ""), checkinConfig, balanceConfig, detectionJSON, now(), now())
	return siteID, true, err
}

func (a *App) importLegacyAccount(ctx context.Context, siteID string, siteName string, email string, username string, password string) (string, bool, error) {
	loginName := firstNonEmpty(email, username)
	var existingID string
	err := a.db.QueryRowContext(ctx, `
		SELECT id FROM channel_accounts
		WHERE upstream_site_id=? AND (LOWER(COALESCE(email,''))=LOWER(?) OR LOWER(COALESCE(username,''))=LOWER(?))
		LIMIT 1
	`, siteID, loginName, loginName).Scan(&existingID)
	if err == nil {
		return existingID, false, nil
	}
	if err != sql.ErrNoRows {
		return "", false, err
	}

	passwordEncrypted, err := a.encryptText(password)
	if err != nil {
		return "", false, err
	}
	accountID := newID()
	if email == "" && strings.Contains(loginName, "@") {
		email = loginName
	}
	if username == "" && email == "" {
		username = loginName
	}
	_, err = a.db.ExecContext(ctx, `
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, email, username, auth_type, password_encrypted, login_status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 'email_password', ?, 'unknown', ?, ?)
	`, accountID, siteID, siteName+" · "+loginName, email, username, passwordEncrypted, now(), now())
	return accountID, true, err
}

func mustJSON(value interface{}) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
