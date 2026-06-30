package channels

import (
	"context"
	"sort"
	"strings"
)

// ChannelHealthOverview loads site and model rows from the database and
// aggregates them into a ChannelHealthOverview. Mirrors the body of the
// original core.channelHealthOverview (the cachedRead wrapping stays in the
// host so the host controls cache lifecycle).
func (s *Service) ChannelHealthOverview(ctx context.Context) (ChannelHealthOverview, error) {
	sites, err := s.loadChannelHealthSiteRows(ctx)
	if err != nil {
		return ChannelHealthOverview{}, err
	}
	models, err := s.loadChannelHealthModelRows(ctx)
	if err != nil {
		return ChannelHealthOverview{}, err
	}
	return buildChannelHealthOverview(sites, models, s.infra.Now()), nil
}

// loadChannelHealthSiteRows mirrors core.loadChannelHealthSiteRows. Returns
// up to 500 site-level health summary rows.
func (s *Service) loadChannelHealthSiteRows(ctx context.Context) ([]ChannelHealthSiteRow, error) {
	rows, err := s.infra.DB().QueryContext(ctx, `
		SELECT s.id, s.name, s.base_url, s.kind, s.health_status, COALESCE(s.last_health_check_at,''),
		       COUNT(a.id),
		       SUM(CASE WHEN COALESCE(a.api_key_fingerprint,'') <> '' AND COALESCE(a.api_key_status,'unchecked')='valid' THEN 1 ELSE 0 END),
		       SUM(CASE WHEN COALESCE(a.api_key_fingerprint,'') <> '' AND COALESCE(a.api_key_status,'unchecked') NOT IN ('valid','unchecked','untested','') THEN 1 ELSE 0 END),
		       SUM(CASE WHEN COALESCE(a.api_key_fingerprint,'') <> '' AND COALESCE(a.api_key_status,'unchecked') IN ('unchecked','untested','') THEN 1 ELSE 0 END)
		FROM upstream_sites s
		LEFT JOIN channel_accounts a ON a.upstream_site_id = s.id
		WHERE s.id <> ?
		GROUP BY s.id
		ORDER BY s.updated_at DESC
		LIMIT 500
	`, GlobalScheduleSiteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []ChannelHealthSiteRow{}
	for rows.Next() {
		var item ChannelHealthSiteRow
		if err := rows.Scan(&item.SiteID, &item.SiteName, &item.BaseURL, &item.Kind, &item.HealthStatus, &item.LastCheckedAt, &item.AccountCount, &item.ValidKeyCount, &item.InvalidKeyCount, &item.UncheckedKeyCount); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// loadChannelHealthModelRows mirrors core.loadChannelHealthModelRows. Returns
// up to 500 channel-level model sync rows joined to upstream_sites.
func (s *Service) loadChannelHealthModelRows(ctx context.Context) ([]ChannelHealthModelRow, error) {
	rows, err := s.infra.DB().QueryContext(ctx, `
		SELECT COALESCE(s.id,''), COALESCE(c.base_url,''), COALESCE(c.models_status,'unchecked'),
		       COALESCE(c.model_count,0), COALESCE(c.models_message,''), COALESCE(c.models_last_synced_at,''), c.name
		FROM imported_channels c
		LEFT JOIN upstream_sites s
		  ON (s.channel_id = c.id OR (COALESCE(s.channel_id,'') = '' AND COALESCE(s.base_url,'') <> '' AND s.base_url = COALESCE(c.base_url,'')))
		WHERE COALESCE(c.source_sync_status,'active') <> 'archived'
		  AND c.upstream_kind IN ('newapi','oneapi','sub2api','modified_relay')
		LIMIT 500
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []ChannelHealthModelRow{}
	for rows.Next() {
		var item ChannelHealthModelRow
		if err := rows.Scan(&item.SiteID, &item.BaseURL, &item.Status, &item.ModelCount, &item.Message, &item.LastSyncedAt, &item.ChannelName); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// buildChannelHealthOverview mirrors core.buildChannelHealthOverview.
// generatedAt is injected so callers control the timestamp (the host uses
// now() at cache-build time; tests can pin it).
func buildChannelHealthOverview(siteRows []ChannelHealthSiteRow, modelRows []ChannelHealthModelRow, generatedAt string) ChannelHealthOverview {
	overview := ChannelHealthOverview{
		GeneratedAt: generatedAt,
		Overall:     "success",
		Sites:       []ChannelHealthSite{},
	}
	sitesByID := map[string]*ChannelHealthSite{}
	sitesByBaseURL := map[string]*ChannelHealthSite{}

	for _, row := range siteRows {
		site := &ChannelHealthSite{
			SiteID:            row.SiteID,
			SiteName:          row.SiteName,
			BaseURL:           row.BaseURL,
			Kind:              row.Kind,
			HealthStatus:      firstNonEmpty(row.HealthStatus, "unknown"),
			AccountCount:      row.AccountCount,
			ValidKeyCount:     row.ValidKeyCount,
			InvalidKeyCount:   row.InvalidKeyCount,
			UncheckedKeyCount: row.UncheckedKeyCount,
			LastCheckedAt:     row.LastCheckedAt,
			Samples:           []string{},
		}
		sitesByID[site.SiteID] = site
		if site.BaseURL != "" {
			sitesByBaseURL[strings.TrimRight(site.BaseURL, "/")] = site
		}
		overview.SiteCount++
		switch strings.ToLower(site.HealthStatus) {
		case "healthy", "ok", "success":
			overview.HealthySiteCount++
		case "unreachable", "down", "failed", "error":
			overview.UnreachableSiteCount++
		}
		overview.ValidKeyCount += site.ValidKeyCount
		overview.InvalidKeyCount += site.InvalidKeyCount
		overview.UncheckedKeyCount += site.UncheckedKeyCount
	}

	for _, row := range modelRows {
		overview.ChannelCount++
		status := strings.ToLower(firstNonEmpty(row.Status, "unchecked"))
		site := sitesByID[row.SiteID]
		if site == nil && row.BaseURL != "" {
			site = sitesByBaseURL[strings.TrimRight(row.BaseURL, "/")]
		}
		if site == nil {
			continue
		}
		site.ModelChannelCount++
		site.ModelCount += row.ModelCount
		if row.LastSyncedAt != "" && (site.LastCheckedAt == "" || row.LastSyncedAt > site.LastCheckedAt) {
			site.LastCheckedAt = row.LastSyncedAt
		}
		appendUniqueString(&site.Samples, row.ChannelName, 4)
		switch status {
		case "live_key":
			overview.LiveModelChannelCount++
			site.LiveModelChannelCount++
		case "failed", "key_invalid", "empty":
			overview.FailedModelChannelCount++
			site.FailedModelChannelCount++
			if site.Message == "" {
				site.Message = row.Message
			}
		default:
			overview.UncheckedModelChannelCount++
			site.UncheckedModelChannelCount++
		}
	}

	for _, site := range sitesByID {
		site.Level, site.RecommendedAction = channelHealthSiteAdvice(*site)
		overview.Sites = append(overview.Sites, *site)
	}
	overview.Overall = channelHealthOverall(overview.Sites)
	sort.SliceStable(overview.Sites, func(i, j int) bool {
		left, right := overview.Sites[i], overview.Sites[j]
		if channelHealthLevelRank(left.Level) != channelHealthLevelRank(right.Level) {
			return channelHealthLevelRank(left.Level) > channelHealthLevelRank(right.Level)
		}
		if left.InvalidKeyCount != right.InvalidKeyCount {
			return left.InvalidKeyCount > right.InvalidKeyCount
		}
		if left.FailedModelChannelCount != right.FailedModelChannelCount {
			return left.FailedModelChannelCount > right.FailedModelChannelCount
		}
		return left.SiteName < right.SiteName
	})
	if len(overview.Sites) > 80 {
		overview.Sites = overview.Sites[:80]
	}
	return overview
}

func channelHealthSiteAdvice(site ChannelHealthSite) (string, string) {
	status := strings.ToLower(site.HealthStatus)
	if status == "unreachable" || status == "down" || status == "failed" || status == "error" {
		return "danger", "先重新探测站点；仍不可达时暂停该站点账号的自动签到、余额刷新和模型探活。"
	}
	if site.InvalidKeyCount > 0 {
		return "danger", "批量检测并替换无效 Key，再刷新模型覆盖。"
	}
	if site.FailedModelChannelCount > 0 {
		return "warning", "重新同步渠道模型；若持续失败，检查渠道 Key 权限和 /v1/models 可访问性。"
	}
	if site.UncheckedKeyCount > 0 || site.UncheckedModelChannelCount > 0 || status == "unknown" {
		return "warning", "补跑 Key 检测、模型同步和站点探测，补齐健康基线。"
	}
	return "success", "保持当前巡检节奏。"
}

func channelHealthOverall(sites []ChannelHealthSite) string {
	overall := "success"
	for _, site := range sites {
		switch site.Level {
		case "danger":
			return "danger"
		case "warning":
			overall = "warning"
		}
	}
	return overall
}

func channelHealthLevelRank(level string) int {
	switch level {
	case "danger":
		return 3
	case "warning":
		return 2
	case "success":
		return 1
	default:
		return 0
	}
}
