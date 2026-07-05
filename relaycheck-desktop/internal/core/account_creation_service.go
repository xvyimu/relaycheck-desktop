package core

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
)

type accountCreationInput struct {
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

type accountCreationBadRequest struct {
	err error
}

func (e accountCreationBadRequest) Error() string {
	return e.err.Error()
}

func accountCreationBadRequestError(err error) error {
	return accountCreationBadRequest{err: err}
}

func isAccountCreationBadRequest(err error) bool {
	_, ok := err.(accountCreationBadRequest)
	return ok
}

type AccountCreationService struct {
	db             *sql.DB
	dataDir        string
	encrypt        func(string) (string, error)
	detectUpstream func(context.Context, string) UpstreamDetection
	notify         func(kind, level, title, content, relatedType, relatedID string)
	audit          func(action, level, actor, resourceType, resourceID, summary string, metadata map[string]interface{})
}

func NewAccountCreationService(app *App) *AccountCreationService {
	return &AccountCreationService{
		db:             app.db,
		dataDir:        app.dataDir,
		encrypt:        app.encryptText,
		detectUpstream: app.detectUpstream,
		notify:         app.notify,
		audit:          app.audit,
	}
}

func (s *AccountCreationService) Create(ctx context.Context, input accountCreationInput) (string, error) {
	input.UpstreamSiteID = strings.TrimSpace(input.UpstreamSiteID)
	if input.UpstreamSiteID == "" && strings.TrimSpace(input.BaseURL) != "" {
		siteID, err := s.EnsureManualSite(ctx, input.SiteName, input.BaseURL, input.LoginURL, input.Kind)
		if err != nil {
			return "", accountCreationBadRequestError(err)
		}
		input.UpstreamSiteID = siteID
	}
	if input.UpstreamSiteID == "" {
		return "", accountCreationBadRequestError(errorsText("请选择已有站点，或填写自定义站点网址。"))
	}
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	if input.DisplayName == "" {
		input.DisplayName = defaultAccountDisplayName(input.Email, input.Username, input.APIKey)
	}
	if input.AuthType == "" {
		input.AuthType = inferAccountAuthType(input.Password, input.Cookie, input.AccessToken, input.RefreshToken, input.APIKey)
	}

	password, err := s.encrypt(input.Password)
	if err != nil {
		return "", err
	}
	cookie, err := s.encrypt(input.Cookie)
	if err != nil {
		return "", err
	}
	access, err := s.encrypt(input.AccessToken)
	if err != nil {
		return "", err
	}
	refresh, err := s.encrypt(input.RefreshToken)
	if err != nil {
		return "", err
	}
	apiKey, err := s.encrypt(input.APIKey)
	if err != nil {
		return "", err
	}

	id := newID()
	profilePath := ""
	status := "unknown"
	if input.AuthType == "browser_profile" || input.AuthType == "oauth_session" {
		profilePath = filepath.Join(s.dataDir, "browser-profiles", id)
		status = "manual_required"
	}
	apiKeyFingerprint := secretFingerprint(input.APIKey)
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, email, username, auth_type, password_encrypted, cookie_encrypted, access_token_encrypted, refresh_token_encrypted, api_key_encrypted, api_key_fingerprint, api_key_status, browser_profile_path, login_status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, input.UpstreamSiteID, input.DisplayName, input.Email, input.Username, input.AuthType, password, cookie, access, refresh, apiKey, apiKeyFingerprint, statusFromKey(apiKeyFingerprint), profilePath, status, now(), now())
	if err != nil {
		return "", err
	}
	s.notify("account_created", "success", "账号已添加", input.DisplayName+" 已绑定。", "account", id)
	s.audit("account.created", "info", "", "account", id, "账号已添加："+input.DisplayName, map[string]interface{}{"authType": input.AuthType, "siteId": input.UpstreamSiteID, "apiKeyFingerprint": apiKeyFingerprint})
	return id, nil
}

func (s *AccountCreationService) EnsureManualSite(ctx context.Context, name string, rawBaseURL string, loginURL string, preferredKind string) (string, error) {
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
	err := s.db.QueryRowContext(ctx, `SELECT id FROM upstream_sites WHERE base_url=? ORDER BY updated_at DESC LIMIT 1`, baseURL).Scan(&existingID)
	if err == nil {
		if loginURL != "" {
			manualDiscovery := manualLoginDiscoveryForURL(loginURL, nil)
			_, err = s.db.ExecContext(ctx, `
				UPDATE upstream_sites
				SET login_url=?, login_url_source='manual', login_url_confidence=1, login_discovery_json=?, updated_at=?
				WHERE id=?
			`, loginURL, marshalLoginDiscovery(manualDiscovery), now(), existingID)
			if err != nil {
				return "", err
			}
		}
		return existingID, nil
	}
	if err != sql.ErrNoRows {
		return "", err
	}

	detection := s.detectUpstream(ctx, baseURL)
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
	storedLoginURL := detection.LoginURL
	storedLoginSource := ""
	storedLoginConfidence := 0.0
	storedLoginDiscoveryJSON := ""
	if detection.LoginDiscovery != nil {
		storedLoginURL = detection.LoginDiscovery.URL
		storedLoginSource = detection.LoginDiscovery.Source
		storedLoginConfidence = detection.LoginDiscovery.Confidence
		storedLoginDiscoveryJSON = marshalLoginDiscovery(detection.LoginDiscovery)
	}
	if loginURL != "" {
		manualDiscovery := manualLoginDiscoveryForURL(loginURL, detection.LoginDiscovery)
		storedLoginURL = manualDiscovery.URL
		storedLoginSource = manualDiscovery.Source
		storedLoginConfidence = manualDiscovery.Confidence
		storedLoginDiscoveryJSON = marshalLoginDiscovery(manualDiscovery)
	}

	channelID := newID()
	siteID := newID()
	detectionJSON := marshalDetection(&detection)
	createdAt := now()
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO imported_channels (id, source_channel_id, name, base_url, status, upstream_kind, supports_checkin, supports_balance, supports_models, supports_pricing, raw_json, detection_json, last_detected_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, 'manual', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, channelID, "manual-"+channelID, name, detection.BaseURL, detection.Kind, boolInt(detection.SupportsCheckin), boolInt(detection.SupportsBalance), boolInt(detection.SupportsModels), boolInt(detection.SupportsPricing), `{"source":"manual-account"}`, detectionJSON, createdAt, createdAt, createdAt)
	if err != nil {
		return "", err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO upstream_sites (id, channel_id, name, homepage_url, base_url, login_url, login_url_source, login_url_confidence, login_discovery_json, kind, detection_confidence, health_status, supports_checkin, supports_balance, supports_models, supports_pricing, detection_json, last_health_check_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, siteID, channelID, name, detection.HomepageURL, detection.BaseURL, storedLoginURL, storedLoginSource, storedLoginConfidence, storedLoginDiscoveryJSON, detection.Kind, detection.DetectionConfidence, detection.HealthStatus, boolInt(detection.SupportsCheckin), boolInt(detection.SupportsBalance), boolInt(detection.SupportsModels), boolInt(detection.SupportsPricing), detectionJSON, createdAt, createdAt, createdAt)
	if err != nil {
		return "", err
	}
	s.notify("upstream_site_created", "success", "上游站点已添加", name+" 已通过账号表单加入站点列表。", "upstream_site", siteID)
	return siteID, nil
}
