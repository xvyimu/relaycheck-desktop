package core

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
)

func (a *App) handleChannels(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	items, err := cachedRead(a, "channels-list", shortReadCacheTTL, func() ([]ImportedChannel, error) {
		rows, err := a.db.QueryContext(r.Context(), `
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
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (a *App) handleBulkChannelSourceSyncStatus(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var input struct {
		FromStatus string `json:"fromStatus"`
		ToStatus   string `json:"toStatus"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "请求参数不完整。")
		return
	}
	fromStatus := strings.TrimSpace(input.FromStatus)
	toStatus := strings.TrimSpace(input.ToStatus)
	if !isSafeBulkSourceStatusTransition(fromStatus, toStatus) {
		writeError(w, http.StatusBadRequest, "仅支持 missing->archived、missing->active、archived->active 这几种安全批量状态切换。")
		return
	}

	changedAt := now()
	var result sql.Result
	var err error
	if toStatus == "active" {
		result, err = a.db.ExecContext(r.Context(), `
			UPDATE imported_channels
			SET source_sync_status='active', source_missing_at='', updated_at=?
			WHERE COALESCE(source_sync_status,'active')=?
		`, changedAt, fromStatus)
	} else {
		result, err = a.db.ExecContext(r.Context(), `
			UPDATE imported_channels
			SET source_sync_status='archived',
			    source_missing_at=CASE WHEN COALESCE(source_missing_at,'')='' THEN ? ELSE source_missing_at END,
			    updated_at=?
			WHERE COALESCE(source_sync_status,'active')=?
		`, changedAt, changedAt, fromStatus)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	affected, _ := result.RowsAffected()
	a.notify("channel_source_status_bulk_changed", "info", "渠道批量状态已更新", fmt.Sprintf("已将 %d 条渠道从 %s 切换为 %s。", affected, fromStatus, toStatus), "channel", "")
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"fromStatus": fromStatus,
		"toStatus":   toStatus,
		"affected":   affected,
		"changedAt":  changedAt,
	})
}

func isSafeBulkSourceStatusTransition(fromStatus string, toStatus string) bool {
	switch fromStatus + "->" + toStatus {
	case "missing->archived", "missing->active", "archived->active":
		return true
	default:
		return false
	}
}

func normalizeOfficialProviderChannel(item *ImportedChannel) {
	if !isOfficialProviderBaseURL(item.BaseURL) {
		return
	}
	item.UpstreamKind = "official_provider"
	item.SupportsCheckin = false
}

func (a *App) handleChannelByID(w http.ResponseWriter, r *http.Request) {
	tail := pathTail(r.URL.Path, "/api/channels/")
	if r.Method == http.MethodGet {
		item, err := a.loadChannelByID(r.Context(), tail)
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "渠道不存在。")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, item)
		return
	}
	if strings.HasSuffix(tail, "/detect") {
		a.detectChannel(w, r, strings.TrimSuffix(tail, "/detect"))
		return
	}
	if strings.HasSuffix(tail, "/restore-source-status") {
		a.setChannelSourceSyncStatus(w, r, strings.TrimSuffix(tail, "/restore-source-status"), "active")
		return
	}
	if strings.HasSuffix(tail, "/archive-source-status") {
		a.setChannelSourceSyncStatus(w, r, strings.TrimSuffix(tail, "/archive-source-status"), "archived")
		return
	}
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func (a *App) loadChannelByID(ctx context.Context, id string) (ImportedChannel, error) {
	var item ImportedChannel
	var checkin, balance, models, pricing int
	row := a.db.QueryRowContext(ctx, `
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

func (a *App) setChannelSourceSyncStatus(w http.ResponseWriter, r *http.Request, id string, nextStatus string) {
	if !method(w, r, http.MethodPost) {
		return
	}
	if nextStatus != "active" && nextStatus != "archived" {
		writeError(w, http.StatusBadRequest, "不支持的渠道同步状态。")
		return
	}

	var name string
	var currentStatus string
	err := a.db.QueryRowContext(r.Context(), `
		SELECT name, COALESCE(source_sync_status,'active')
		FROM imported_channels
		WHERE id = ?
	`, id).Scan(&name, &currentStatus)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "渠道不存在。")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	changedAt := now()
	if nextStatus == "active" {
		_, err = a.db.ExecContext(r.Context(), `
			UPDATE imported_channels
			SET source_sync_status='active', source_missing_at='', updated_at=?
			WHERE id=?
		`, changedAt, id)
	} else {
		_, err = a.db.ExecContext(r.Context(), `
			UPDATE imported_channels
			SET source_sync_status='archived',
			    source_missing_at=CASE WHEN COALESCE(source_missing_at,'')='' THEN ? ELSE source_missing_at END,
			    updated_at=?
			WHERE id=?
		`, changedAt, changedAt, id)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	title := "渠道状态已恢复"
	content := name + " 已恢复为源端存在。"
	if nextStatus == "archived" {
		title = "渠道已归档"
		content = name + " 已归档保留，不会删除账号、余额或签到日志。"
	}
	a.notify("channel_source_status_changed", "info", title, content, "channel", id)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":             id,
		"name":           name,
		"previousStatus": currentStatus,
		"sourceStatus":   nextStatus,
		"changedAt":      changedAt,
	})
}

func (a *App) detectChannel(w http.ResponseWriter, r *http.Request, id string) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var name, baseURL string
	err := a.db.QueryRowContext(r.Context(), `
		SELECT name, COALESCE(base_url,'')
		FROM imported_channels
		WHERE id = ?
	`, id).Scan(&name, &baseURL)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "渠道不存在。")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if strings.TrimSpace(baseURL) == "" {
		writeError(w, http.StatusBadRequest, "该渠道没有 Base URL，无法识别为上游站点。")
		return
	}

	detection := a.detectUpstream(r.Context(), baseURL)
	_, err = a.db.ExecContext(r.Context(), `
		UPDATE imported_channels
		SET base_url=?, upstream_kind=?, supports_checkin=?, supports_balance=?, supports_models=?, supports_pricing=?, detection_json=?, last_detected_at=?, updated_at=?
		WHERE id=?
	`, detection.BaseURL, detection.Kind, boolInt(detection.SupportsCheckin), boolInt(detection.SupportsBalance), boolInt(detection.SupportsModels), boolInt(detection.SupportsPricing), marshalDetection(&detection), now(), now(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	siteID, created, err := a.ensureUpstreamSiteForChannel(r.Context(), id, name, detection.BaseURL, detection.Kind, &detection)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	level := "success"
	content := "已识别渠道并同步到上游站点。"
	if detection.HealthStatus == "unreachable" {
		level = "warning"
		content = "渠道已同步，但当前不可达。"
	}
	a.notify("channel_detected", level, "渠道识别完成", name+"： "+content, "channel", id)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"detection": detection,
		"siteId":    siteID,
		"created":   created,
	})
}

// ensureUpstreamSiteForChannel is now a *App forwarder for
// sites.Service.EnsureUpstreamSiteForChannel — see scanner.go.
