package core

import (
	"context"
	"database/sql"
	"strings"
)

type AccountSiteUpdateService struct {
	db               *sql.DB
	ensureManualSite func(context.Context, string, string, string, string) (string, error)
}

func NewAccountSiteUpdateService(app *App) *AccountSiteUpdateService {
	return &AccountSiteUpdateService{
		db:               app.db,
		ensureManualSite: app.ensureManualAccountSite,
	}
}

func (s *AccountSiteUpdateService) Resolve(ctx context.Context, current ChannelAccount, siteName string, rawBaseURL string, loginURL string, preferredKind string, siteScope string) (string, bool, error) {
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
			return current.UpstreamSiteID, false, s.UpdateMetadata(ctx, current.UpstreamSiteID, siteName, loginURL, preferredKind)
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
		return s.UpdateShared(ctx, current, siteName, baseURL, loginURL, preferredKind)
	}
	if currentBaseURL != "" && baseURL == currentBaseURL {
		return current.UpstreamSiteID, false, s.UpdateMetadata(ctx, current.UpstreamSiteID, siteName, loginURL, preferredKind)
	}

	nextSiteName := firstNonEmpty(siteName, current.UpstreamSiteName, hostLabel(baseURL), baseURL)
	siteID, err := s.ensureManualSite(ctx, nextSiteName, baseURL, loginURL, preferredKind)
	if err != nil {
		return "", false, err
	}
	if err := s.UpdateMetadata(ctx, siteID, nextSiteName, loginURL, preferredKind); err != nil {
		return "", false, err
	}
	return siteID, siteID != current.UpstreamSiteID, nil
}

func (s *AccountSiteUpdateService) UpdateShared(ctx context.Context, current ChannelAccount, siteName string, baseURL string, loginURL string, kind string) (string, bool, error) {
	currentBaseURL := normalizeBaseURL(current.UpstreamSiteBaseURL)
	nextSiteName := firstNonEmpty(strings.TrimSpace(siteName), current.UpstreamSiteName, hostLabel(baseURL), baseURL)
	if currentBaseURL != "" && baseURL == currentBaseURL {
		return current.UpstreamSiteID, false, s.UpdateMetadata(ctx, current.UpstreamSiteID, nextSiteName, loginURL, kind)
	}

	var existingID string
	err := s.db.QueryRowContext(ctx, `
		SELECT id
		FROM upstream_sites
		WHERE base_url=? AND id<>?
		ORDER BY updated_at DESC
		LIMIT 1
	`, baseURL, current.UpstreamSiteID).Scan(&existingID)
	if err == nil {
		if err := s.UpdateMetadata(ctx, existingID, nextSiteName, loginURL, kind); err != nil {
			return "", false, err
		}
		if _, err := s.db.ExecContext(ctx, `
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

	if err := s.UpdateAddress(ctx, current.UpstreamSiteID, nextSiteName, baseURL, loginURL, kind); err != nil {
		return "", false, err
	}
	return current.UpstreamSiteID, false, nil
}

func (s *AccountSiteUpdateService) UpdateAddress(ctx context.Context, siteID string, siteName string, baseURL string, loginURL string, kind string) error {
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
		sets = append(sets, "login_url=?", "login_url_source=?", "login_url_confidence=?", "login_discovery_json=?")
		args = append(args, loginURL, "manual", 1, marshalLoginDiscovery(manualLoginDiscoveryForURL(loginURL, nil)))
	}
	if isManagedRelayKind(kind) {
		sets = append(sets, "kind=?")
		args = append(args, kind)
	}
	args = append(args, siteID)
	if _, err := s.db.ExecContext(ctx, "UPDATE upstream_sites SET "+strings.Join(sets, ", ")+" WHERE id=?", args...); err != nil {
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
	_, err := s.db.ExecContext(ctx, "UPDATE imported_channels SET "+strings.Join(channelSets, ", ")+" WHERE id=(SELECT channel_id FROM upstream_sites WHERE id=?)", channelArgs...)
	return err
}

func (s *AccountSiteUpdateService) UpdateMetadata(ctx context.Context, siteID string, siteName string, loginURL string, kind string) error {
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
		sets = append(sets, "login_url=?", "login_url_source=?", "login_url_confidence=?", "login_discovery_json=?")
		args = append(args, loginURL, "manual", 1, marshalLoginDiscovery(manualLoginDiscoveryForURL(loginURL, nil)))
	}
	if isManagedRelayKind(kind) {
		sets = append(sets, "kind=?")
		args = append(args, kind)
	}
	args = append(args, siteID)
	if _, err := s.db.ExecContext(ctx, "UPDATE upstream_sites SET "+strings.Join(sets, ", ")+" WHERE id=?", args...); err != nil {
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
	_, err := s.db.ExecContext(ctx, "UPDATE imported_channels SET "+strings.Join(channelSets, ", ")+" WHERE id=(SELECT channel_id FROM upstream_sites WHERE id=?)", channelArgs...)
	return err
}
