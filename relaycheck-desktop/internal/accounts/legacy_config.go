package accounts

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// ImportLegacyConfig imports a legacy config_site*.json file, upserting the
// site and (optionally) the account.
func (s *Service) ImportLegacyConfig(ctx context.Context, content string, fileName string) (map[string]interface{}, error) {
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

	siteID, created, err := s.upsertLegacySite(ctx, siteName, baseURL, loginURL, checkinConfig, balanceConfig, content)
	if err != nil {
		return nil, err
	}

	accountID := ""
	accountImported := false
	loginName := firstNonEmpty(cfg.Email, cfg.Username)
	if loginName != "" && strings.TrimSpace(cfg.Password) != "" {
		accountID, accountImported, err = s.importLegacyAccount(ctx, siteID, siteName, cfg.Email, cfg.Username, cfg.Password)
		if err != nil {
			return nil, err
		}
	}

	s.infra.Notify("legacy_config_imported", "success", "旧配置导入完成", fmt.Sprintf("%s 已导入兼容配置。", siteName), "upstream_site", siteID)
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

// upsertLegacySite inserts or updates an upstream_sites row for a legacy
// config import, also creating an imported_channels row when new.
func (s *Service) upsertLegacySite(ctx context.Context, name string, baseURL string, loginURL string, checkinConfig string, balanceConfig string, raw string) (string, bool, error) {
	detectionJSON := mustJSON(Detection{
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
	err := s.infra.DB().QueryRowContext(ctx, `SELECT id FROM upstream_sites WHERE base_url=? ORDER BY updated_at DESC LIMIT 1`, baseURL).Scan(&siteID)
	if err == nil {
		_, err = s.infra.DB().ExecContext(ctx, `
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
		`, name, loginURL, loginURL, checkinConfig, checkinConfig, balanceConfig, balanceConfig, checkinConfig, balanceConfig, detectionJSON, s.infra.Now(), siteID)
		return siteID, false, err
	}
	if err != sql.ErrNoRows {
		return "", false, err
	}

	channelID := s.infra.NewID()
	rawJSON := mustJSON(map[string]interface{}{"source": "legacy_config", "raw": raw})
	_, err = s.infra.DB().ExecContext(ctx, `
		INSERT INTO imported_channels (id, source_channel_id, name, base_url, status, upstream_kind, supports_checkin, supports_balance, raw_json, detection_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, 'legacy', 'unknown', ?, ?, ?, ?, ?, ?)
	`, channelID, "legacy-"+channelID, name, baseURL, boolInt(checkinConfig != ""), boolInt(balanceConfig != ""), rawJSON, detectionJSON, s.infra.Now(), s.infra.Now())
	if err != nil {
		return "", false, err
	}

	siteID = s.infra.NewID()
	_, err = s.infra.DB().ExecContext(ctx, `
		INSERT INTO upstream_sites (id, channel_id, name, homepage_url, base_url, login_url, kind, health_status, supports_checkin, supports_balance, checkin_config_json, balance_config_json, detection_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 'unknown', 'unknown', ?, ?, ?, ?, ?, ?, ?)
	`, siteID, channelID, name, baseURL, baseURL, loginURL, boolInt(checkinConfig != ""), boolInt(balanceConfig != ""), checkinConfig, balanceConfig, detectionJSON, s.infra.Now(), s.infra.Now())
	return siteID, true, err
}

// importLegacyAccount inserts a channel_accounts row from a legacy config,
// skipping if an account with the same login name already exists.
func (s *Service) importLegacyAccount(ctx context.Context, siteID string, siteName string, email string, username string, password string) (string, bool, error) {
	loginName := firstNonEmpty(email, username)
	var existingID string
	err := s.infra.DB().QueryRowContext(ctx, `
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

	passwordEncrypted, err := s.infra.EncryptText(password)
	if err != nil {
		return "", false, err
	}
	accountID := s.infra.NewID()
	if email == "" && strings.Contains(loginName, "@") {
		email = loginName
	}
	if username == "" && email == "" {
		username = loginName
	}
	_, err = s.infra.DB().ExecContext(ctx, `
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, email, username, auth_type, password_encrypted, login_status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 'email_password', ?, 'unknown', ?, ?)
	`, accountID, siteID, siteName+" · "+loginName, email, username, passwordEncrypted, s.infra.Now(), s.infra.Now())
	return accountID, true, err
}
