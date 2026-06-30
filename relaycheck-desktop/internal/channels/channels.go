package channels

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ListChannels returns all imported channels ordered so missing-source
// channels sink to the bottom. Each channel's sample_models_json is decoded
// and official-provider channels are normalized to kind=official_provider.
// Mirrors the build function originally inlined in core.handleChannels.
func (s *Service) ListChannels(ctx context.Context) ([]ImportedChannel, error) {
	rows, err := s.infra.DB().QueryContext(ctx, `
		SELECT id, COALESCE(local_instance_id,''), source_channel_id, name, COALESCE(base_url,''),
		       COALESCE(status,''), upstream_kind, supports_checkin, supports_balance,
		       supports_models, supports_pricing, COALESCE(channel_key_masked,''),
		       COALESCE(model_count,0), COALESCE(sample_models_json,''), COALESCE(models_source,''),
		       COALESCE(models_status,''), COALESCE(models_last_synced_at,''), COALESCE(models_message,''),
		       COALESCE(source_sync_status,'active'), COALESCE(source_missing_at,''),
		       COALESCE(raw_json,''), COALESCE(detection_json,''), COALESCE(last_detected_at,''), created_at, updated_at
		FROM imported_channels
		ORDER BY CASE WHEN COALESCE(source_sync_status,'active')='missing' THEN 1 ELSE 0 END, updated_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []ImportedChannel{}
	for rows.Next() {
		var item ImportedChannel
		var checkin, balance, models, pricing int
		var sampleModelsJSON string
		if err := rows.Scan(&item.ID, &item.LocalInstanceID, &item.SourceChannelID, &item.Name, &item.BaseURL, &item.Status, &item.UpstreamKind, &checkin, &balance, &models, &pricing, &item.ChannelKeyMasked, &item.ModelCount, &sampleModelsJSON, &item.ModelsSource, &item.ModelsStatus, &item.ModelsLastSyncedAt, &item.ModelsMessage, &item.SourceSyncStatus, &item.SourceMissingAt, &item.RawJSON, &item.DetectionJSON, &item.LastDetectedAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.SupportsCheckin = checkin == 1
		item.SupportsBalance = balance == 1
		item.SupportsModels = models == 1
		item.SupportsPricing = pricing == 1
		item.SampleModels = parsePersistedStringSlice(sampleModelsJSON)
		item.SourceType = sourceTypeFromChannel(item)
		normalizeOfficialProviderChannel(&item)
		items = append(items, item)
	}
	return items, rows.Err()
}

// LoadChannelByID returns a single imported channel by ID. Mirrors the
// original core.loadChannelByID.
func (s *Service) LoadChannelByID(ctx context.Context, id string) (ImportedChannel, error) {
	var item ImportedChannel
	var checkin, balance, models, pricing int
	row := s.infra.DB().QueryRowContext(ctx, `
		SELECT id, COALESCE(local_instance_id,''), source_channel_id, name, COALESCE(base_url,''),
		       COALESCE(status,''), upstream_kind, supports_checkin, supports_balance,
		       supports_models, supports_pricing, COALESCE(channel_key_masked,''),
		       COALESCE(model_count,0), COALESCE(sample_models_json,''), COALESCE(models_source,''),
		       COALESCE(models_status,''), COALESCE(models_last_synced_at,''), COALESCE(models_message,''),
		       COALESCE(source_sync_status,'active'), COALESCE(source_missing_at,''),
		       COALESCE(raw_json,''), COALESCE(detection_json,''), COALESCE(last_detected_at,''), created_at, updated_at
		FROM imported_channels
		WHERE id=?
	`, id)
	var sampleModelsJSON string
	err := row.Scan(&item.ID, &item.LocalInstanceID, &item.SourceChannelID, &item.Name, &item.BaseURL, &item.Status, &item.UpstreamKind, &checkin, &balance, &models, &pricing, &item.ChannelKeyMasked, &item.ModelCount, &sampleModelsJSON, &item.ModelsSource, &item.ModelsStatus, &item.ModelsLastSyncedAt, &item.ModelsMessage, &item.SourceSyncStatus, &item.SourceMissingAt, &item.RawJSON, &item.DetectionJSON, &item.LastDetectedAt, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return item, err
	}
	item.SupportsCheckin = checkin == 1
	item.SupportsBalance = balance == 1
	item.SupportsModels = models == 1
	item.SupportsPricing = pricing == 1
	item.SampleModels = parsePersistedStringSlice(sampleModelsJSON)
	item.SourceType = sourceTypeFromChannel(item)
	normalizeOfficialProviderChannel(&item)
	return item, nil
}

// SetChannelSourceSyncStatus updates one channel's source_sync_status. Returns
// the previous status, name, and changedAt timestamp so the host handler can
// build its JSON response without re-querying. Mirrors the body of
// core.setChannelSourceSyncStatus (the HTTP handler stays in core).
func (s *Service) SetChannelSourceSyncStatus(ctx context.Context, id string, nextStatus string) (SetChannelSourceSyncStatusResult, error) {
	result := SetChannelSourceSyncStatusResult{ID: id}
	var name, currentStatus string
	err := s.infra.DB().QueryRowContext(ctx, `
		SELECT name, COALESCE(source_sync_status,'active')
		FROM imported_channels
		WHERE id = ?
	`, id).Scan(&name, &currentStatus)
	if err == sql.ErrNoRows {
		return result, sql.ErrNoRows
	}
	if err != nil {
		return result, err
	}
	result.Name = name
	result.PreviousStatus = currentStatus

	changedAt := s.infra.Now()
	if nextStatus == "active" {
		_, err = s.infra.DB().ExecContext(ctx, `
			UPDATE imported_channels
			SET source_sync_status='active', source_missing_at='', updated_at=?
			WHERE id=?
		`, changedAt, id)
	} else {
		_, err = s.infra.DB().ExecContext(ctx, `
			UPDATE imported_channels
			SET source_sync_status='archived',
			    source_missing_at=CASE WHEN COALESCE(source_missing_at,'')='' THEN ? ELSE source_missing_at END,
			    updated_at=?
			WHERE id=?
		`, changedAt, changedAt, id)
	}
	if err != nil {
		return result, err
	}
	result.SourceStatus = nextStatus
	result.ChangedAt = changedAt

	title := "渠道状态已恢复"
	content := name + " 已恢复为源端存在。"
	if nextStatus == "archived" {
		title = "渠道已归档"
		content = name + " 已归档保留，不会删除账号、余额或签到日志。"
	}
	s.infra.Notify("channel_source_status_changed", "info", title, content, "channel", id)
	s.infra.InvalidateReadCache()
	return result, nil
}

// BulkSetChannelSourceSyncStatus updates all channels whose current
// source_sync_status matches fromStatus. Returns the affected row count and
// the shared changedAt timestamp. Mirrors the body of
// core.handleBulkChannelSourceSyncStatus (the HTTP handler stays in core).
func (s *Service) BulkSetChannelSourceSyncStatus(ctx context.Context, fromStatus, toStatus string) (int64, string, error) {
	changedAt := s.infra.Now()
	var result sql.Result
	var err error
	if toStatus == "active" {
		result, err = s.infra.DB().ExecContext(ctx, `
			UPDATE imported_channels
			SET source_sync_status='active', source_missing_at='', updated_at=?
			WHERE COALESCE(source_sync_status,'active')=?
		`, changedAt, fromStatus)
	} else {
		result, err = s.infra.DB().ExecContext(ctx, `
			UPDATE imported_channels
			SET source_sync_status='archived',
			    source_missing_at=CASE WHEN COALESCE(source_missing_at,'')='' THEN ? ELSE source_missing_at END,
			    updated_at=?
			WHERE COALESCE(source_sync_status,'active')=?
		`, changedAt, changedAt, fromStatus)
	}
	if err != nil {
		return 0, changedAt, err
	}
	affected, _ := result.RowsAffected()
	s.infra.Notify("channel_source_status_bulk_changed", "info", "渠道批量状态已更新", fmt.Sprintf("已将 %d 条渠道从 %s 切换为 %s。", affected, fromStatus, toStatus), "channel", "")
	s.infra.InvalidateReadCache()
	return affected, changedAt, nil
}

// DetectChannel re-probes a channel's base URL, persists the detection, and
// ensures an upstream_sites row exists for it. Returns the detection plus the
// site ID and whether the site was newly created. Mirrors the body of
// core.detectChannel (the HTTP handler stays in core).
func (s *Service) DetectChannel(ctx context.Context, id string) (DetectChannelResult, error) {
	result := DetectChannelResult{}
	var name, baseURL string
	err := s.infra.DB().QueryRowContext(ctx, `
		SELECT name, COALESCE(base_url,'')
		FROM imported_channels
		WHERE id = ?
	`, id).Scan(&name, &baseURL)
	if err == sql.ErrNoRows {
		return result, sql.ErrNoRows
	}
	if err != nil {
		return result, err
	}
	if strings.TrimSpace(baseURL) == "" {
		return result, ErrEmptyChannelBaseURL
	}

	detection, detectErr := s.infra.DetectUpstream(ctx, baseURL)
	if detectErr != nil {
		return result, detectErr
	}
	_, err = s.infra.DB().ExecContext(ctx, `
		UPDATE imported_channels
		SET base_url=?, upstream_kind=?, supports_checkin=?, supports_balance=?, supports_models=?, supports_pricing=?, detection_json=?, last_detected_at=?, updated_at=?
		WHERE id=?
	`, detection.BaseURL, detection.Kind, boolInt(detection.SupportsCheckin), boolInt(detection.SupportsBalance), boolInt(detection.SupportsModels), boolInt(detection.SupportsPricing), marshalDetection(&detection), s.infra.Now(), s.infra.Now(), id)
	if err != nil {
		return result, err
	}

	var detectionPtr *Detection = &detection
	siteID, created, err := s.infra.EnsureUpstreamSiteForChannel(ctx, EnsureSiteInput{
		ChannelID:  id,
		Name:       name,
		RawBaseURL: detection.BaseURL,
		Kind:       detection.Kind,
		Detection:  detectionPtr,
	})
	if err != nil {
		return result, err
	}
	level := "success"
	content := "已识别渠道并同步到上游站点。"
	if detection.HealthStatus == "unreachable" {
		level = "warning"
		content = "渠道已同步，但当前不可达。"
	}
	s.infra.Notify("channel_detected", level, "渠道识别完成", name+"： "+content, "channel", id)
	s.infra.InvalidateReadCache()
	result.Detection = detection
	result.SiteID = siteID
	result.Created = created
	return result, nil
}

// ErrEmptyChannelBaseURL mirrors the "该渠道没有 Base URL" branch of
// core.detectChannel. Returned as a sentinel so the host handler can map it
// to a 400 without re-checking the base URL.
var ErrEmptyChannelBaseURL = errors.New("该渠道没有 Base URL，无法识别为上游站点。")

// IsSafeBulkSourceStatusTransition mirrors core.isSafeBulkSourceStatusTransition.
// Reports whether a bulk source-status transition is allowed. Only
// missing->archived, missing->active, and archived->active are permitted.
func IsSafeBulkSourceStatusTransition(fromStatus string, toStatus string) bool {
	switch fromStatus + "->" + toStatus {
	case "missing->archived", "missing->active", "archived->active":
		return true
	default:
		return false
	}
}

// normalizeOfficialProviderChannel mirrors core.normalizeOfficialProviderChannel.
// Forces official-provider channels to kind=official_provider and disables
// checkin support.
func normalizeOfficialProviderChannel(item *ImportedChannel) {
	if !isOfficialProviderBaseURL(item.BaseURL) {
		return
	}
	item.UpstreamKind = "official_provider"
	item.SupportsCheckin = false
}
