package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"strings"
)

func marshalDetection(detection *UpstreamDetection) string {
	if detection == nil {
		return ""
	}
	payload, err := json.Marshal(detection)
	if err != nil {
		return ""
	}
	return string(payload)
}

func parseDetectionJSON(raw string) (UpstreamDetection, bool) {
	if strings.TrimSpace(raw) == "" {
		return UpstreamDetection{}, false
	}
	var detection UpstreamDetection
	if err := json.Unmarshal([]byte(raw), &detection); err != nil {
		return UpstreamDetection{}, false
	}
	return detection, true
}

func sourceTypeFromChannel(channel ImportedChannel) string {
	raw := strings.ToLower(channel.RawJSON)
	sourceID := strings.ToLower(channel.SourceChannelID)
	switch {
	case strings.Contains(raw, `"source":"manual"`) || strings.HasPrefix(sourceID, "manual-"):
		return "manual"
	case strings.Contains(raw, `"source":"legacy"`) || strings.Contains(raw, `"source":"legacy_config"`) || strings.HasPrefix(sourceID, "legacy-"):
		return "legacy"
	case channel.LocalInstanceID != "":
		if strings.Contains(raw, `"source":"admin_api"`) || strings.Contains(raw, `"import_source":"admin_api"`) {
			return "admin_api"
		}
		return "sqlite"
	default:
		return "unknown"
	}
}

func siteSuggestions(site UpstreamSite, detection UpstreamDetection, accounts []ChannelAccount) []string {
	seen := map[string]bool{}
	suggestions := make([]string, 0, 8)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		suggestions = append(suggestions, value)
	}

	switch site.Kind {
	case "official_provider":
		add("该站点被识别为官方供应商接口，只建议做密钥有效性和余额检查，不做签到。")
	case "openai_compatible":
		add("该站点目前更像 OpenAI 兼容 API，不像 NewAPI/Sub2API 后台，默认不做签到。")
	case "sub2api":
		if !site.SupportsCheckin {
			add("识别为 Sub2API，但没有探测到标准签到接口，可能是未开启签到或使用了自定义路径。")
		}
	case "newapi", "oneapi":
		if !site.SupportsCheckin {
			add("识别为面板型中转站，但没有命中签到接口；可以补充手动签到规则后再测。")
		}
	default:
		add("该站点仍未稳定识别为目标后台，建议重新识别，或手动指定后台类型。")
	}

	switch site.HealthStatus {
	case "unreachable":
		add("站点当前不可达，请检查 Base URL、端口、代理或网络访问。")
	case "auth_required":
		add("探针命中了需要登录的接口，说明站点在线，但部分功能需要账号授权后才能进一步判断。")
	case "degraded":
		add("站点有响应但探针命中不足，建议打开详情查看命中的信号，再补充登录页或后台路径。")
	}

	if len(detection.MatchedSignals) == 0 {
		add("当前没有命中明显特征信号，可以尝试手动登录一次并保存授权后重新识别。")
	}

	if len(accounts) == 0 {
		add("该站点还没有绑定账号，绑定一个账号后可以继续做登录态、签到和余额检测。")
	}

	expired := 0
	manualRequired := 0
	for _, account := range accounts {
		switch account.LoginStatus {
		case "expired":
			expired++
		case "manual_required", "captcha_required", "two_factor_required":
			manualRequired++
		}
	}
	if expired > 0 {
		add("部分账号登录态已失效，建议重新打开浏览器标签页保存授权。")
	}
	if manualRequired > 0 {
		add("部分账号需要人工登录或二次验证，完成后再执行签到与余额刷新。")
	}
	if !site.SupportsBalance {
		add("尚未稳定识别到余额接口，后续可以补充站点自定义余额规则。")
	}
	return suggestions
}

func (a *App) loadSiteDetail(ctx context.Context, id string) (SiteDetail, error) {
	detail := SiteDetail{
		Accounts:         []ChannelAccount{},
		BalanceSnapshots: []BalanceSnapshot{},
		CheckinLogs:      []CheckinLog{},
		Suggestions:      []string{},
	}
	var checkin, balance, models, pricing int
	err := a.db.QueryRowContext(ctx, `
		SELECT s.id, COALESCE(s.channel_id,''), s.name, COALESCE(s.homepage_url,''), s.base_url,
		       COALESCE(s.login_url,''), s.kind, s.detection_confidence, s.health_status,
		       s.supports_checkin, s.supports_balance, s.supports_models, s.supports_pricing,
		       COALESCE(s.detection_json,''), COALESCE(s.last_health_check_at,''), s.created_at, s.updated_at,
		       (SELECT COUNT(*) FROM channel_accounts a WHERE a.upstream_site_id = s.id)
		FROM upstream_sites s
		WHERE s.id = ?
	`, id).Scan(
		&detail.Site.ID,
		&detail.Site.ChannelID,
		&detail.Site.Name,
		&detail.Site.HomepageURL,
		&detail.Site.BaseURL,
		&detail.Site.LoginURL,
		&detail.Site.Kind,
		&detail.Site.DetectionConfidence,
		&detail.Site.HealthStatus,
		&checkin,
		&balance,
		&models,
		&pricing,
		&detail.Site.DetectionJSON,
		&detail.Site.LastHealthCheckAt,
		&detail.Site.CreatedAt,
		&detail.Site.UpdatedAt,
		&detail.Site.AccountCount,
	)
	if err != nil {
		return detail, err
	}
	detail.Site.SupportsCheckin = checkin == 1
	detail.Site.SupportsBalance = balance == 1
	detail.Site.SupportsModels = models == 1
	detail.Site.SupportsPricing = pricing == 1
	normalizeOfficialProviderSite(&detail.Site)

	if detection, ok := parseDetectionJSON(detail.Site.DetectionJSON); ok {
		detail.Detection = detection
	} else {
		detection := a.detectUpstream(ctx, detail.Site.BaseURL)
		detail.Detection = detection
		detail.Site.DetectionJSON = marshalDetection(&detection)
		if _, execErr := a.db.ExecContext(ctx, `
			UPDATE upstream_sites
			SET detection_json=?, homepage_url=?, login_url=?, kind=?, detection_confidence=?, health_status=?,
			    supports_checkin=?, supports_balance=?, supports_models=?, supports_pricing=?, last_health_check_at=?, updated_at=?
			WHERE id=?
		`, detail.Site.DetectionJSON, detection.HomepageURL, detection.LoginURL, detection.Kind, detection.DetectionConfidence, detection.HealthStatus, boolInt(detection.SupportsCheckin), boolInt(detection.SupportsBalance), boolInt(detection.SupportsModels), boolInt(detection.SupportsPricing), now(), now(), id); execErr != nil {
			log.Printf("[detection] site detail cache update failed for site %s: %v", id, execErr)
		}
		detail.Site.Kind = detection.Kind
		detail.Site.HealthStatus = detection.HealthStatus
		detail.Site.SupportsCheckin = detection.SupportsCheckin
		detail.Site.SupportsBalance = detection.SupportsBalance
		detail.Site.SupportsModels = detection.SupportsModels
		detail.Site.SupportsPricing = detection.SupportsPricing
		detail.Site.HomepageURL = detection.HomepageURL
		detail.Site.LoginURL = detection.LoginURL
		detail.Site.DetectionConfidence = detection.DetectionConfidence
		normalizeOfficialProviderSite(&detail.Site)
	}
	if detail.Site.Kind == "official_provider" {
		detail.Detection.Kind = "official_provider"
		detail.Detection.SupportsCheckin = false
		if detail.Detection.HealthStatus == "" || detail.Detection.HealthStatus == "unknown" {
			detail.Detection.HealthStatus = detail.Site.HealthStatus
		}
	}

	accountRows, err := a.db.QueryContext(ctx, `
		SELECT a.id, a.upstream_site_id, ?, ?, a.display_name, COALESCE(a.email,''), COALESCE(a.username,''),
		       a.auth_type, COALESCE(a.browser_profile_path,''), a.login_status, COALESCE(a.api_key_fingerprint,''),
		       COALESCE(a.api_key_status,''), COALESCE(a.api_key_last_checked_at,''), COALESCE(a.balance_unit,'unknown'),
		       a.balance, COALESCE(a.last_checkin_at,''), COALESCE(a.last_checkin_status,''),
		       COALESCE((SELECT l.message FROM checkin_logs l WHERE l.account_id = a.id ORDER BY l.started_at DESC LIMIT 1), ''),
		       COALESCE(a.last_login_at,''), COALESCE(a.last_validated_at,''), a.created_at, a.updated_at
		FROM channel_accounts a
		WHERE a.upstream_site_id = ?
		ORDER BY a.updated_at DESC
	`, detail.Site.Name, detail.Site.BaseURL, id)
	if err != nil {
		return detail, err
	}
	defer accountRows.Close()
	for accountRows.Next() {
		var item ChannelAccount
		var accountBalance sql.NullFloat64
		if err := accountRows.Scan(
			&item.ID,
			&item.UpstreamSiteID,
			&item.UpstreamSiteName,
			&item.UpstreamSiteBaseURL,
			&item.DisplayName,
			&item.Email,
			&item.Username,
			&item.AuthType,
			&item.BrowserProfilePath,
			&item.LoginStatus,
			&item.APIKeyFingerprint,
			&item.APIKeyStatus,
			&item.APIKeyLastCheckedAt,
			&item.BalanceUnit,
			&accountBalance,
			&item.LastCheckinAt,
			&item.LastCheckinStatus,
			&item.LastCheckinMessage,
			&item.LastLoginAt,
			&item.LastValidatedAt,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err == nil {
			item.Balance = nullableFloat(accountBalance)
			detail.Accounts = append(detail.Accounts, item)
		}
	}

	balanceRows, err := a.db.QueryContext(ctx, `
		SELECT b.id, b.account_id, a.display_name, b.upstream_site_id, s.name, COALESCE(b.channel_id,''),
		       b.balance, b.used_quota, b.total_quota, b.unit, COALESCE(b.raw_response_masked,''), b.created_at
		FROM balance_snapshots b
		JOIN channel_accounts a ON a.id = b.account_id
		JOIN upstream_sites s ON s.id = b.upstream_site_id
		WHERE b.upstream_site_id = ?
		ORDER BY b.created_at DESC
		LIMIT 12
	`, id)
	if err != nil {
		return detail, err
	}
	defer balanceRows.Close()
	for balanceRows.Next() {
		var item BalanceSnapshot
		var current, used, total sql.NullFloat64
		if err := balanceRows.Scan(&item.ID, &item.AccountID, &item.AccountName, &item.UpstreamSiteID, &item.UpstreamSiteName, &item.ChannelID, &current, &used, &total, &item.Unit, &item.RawResponseMasked, &item.CreatedAt); err == nil {
			item.Balance = nullableFloat(current)
			item.UsedQuota = nullableFloat(used)
			item.TotalQuota = nullableFloat(total)
			detail.BalanceSnapshots = append(detail.BalanceSnapshots, item)
		}
	}

	logRows, err := a.db.QueryContext(ctx, `
		SELECT l.id, l.account_id, a.display_name, l.upstream_site_id, s.name, COALESCE(l.channel_id,''),
		       l.status, COALESCE(l.reward,''), COALESCE(l.message,''), COALESCE(l.raw_response_masked,''),
		       l.started_at, l.finished_at
		FROM checkin_logs l
		JOIN channel_accounts a ON a.id = l.account_id
		JOIN upstream_sites s ON s.id = l.upstream_site_id
		WHERE l.upstream_site_id = ?
		ORDER BY l.started_at DESC
		LIMIT 12
	`, id)
	if err != nil {
		return detail, err
	}
	defer logRows.Close()
	logs, err := scanCheckinLogs(logRows)
	if err != nil {
		return detail, err
	}
	detail.CheckinLogs = logs
	detail.Suggestions = siteSuggestions(detail.Site, detail.Detection, detail.Accounts)
	return detail, nil
}
