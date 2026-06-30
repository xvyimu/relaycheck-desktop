package accounts

import (
	"context"
	"fmt"
	"strings"
)

// ReconcileMissingLocalNewAPIInstance marks imported_channels rows as
// active/missing based on whether they still appear in the source, and
// optionally saves/clears the sync token.
func (s *Service) ReconcileMissingLocalNewAPIInstance(ctx context.Context, instance LocalNewAPIInstance, input SyncSourceInput, notify bool) (map[string]interface{}, error) {
	_, records, err := s.SourceChannelRecordsForLocalNewAPI(ctx, instance, input)
	if err != nil {
		return nil, err
	}
	seenSourceIDs := sourceIDSetFromRecords(records)
	existing, err := s.existingImportedChannels(ctx, instance.ID)
	if err != nil {
		return nil, err
	}

	tx, err := s.infra.DB().BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	activeCount := 0
	missingCount := 0
	markedAt := s.infra.Now()
	for sourceID := range existing {
		if seenSourceIDs[sourceID] {
			_, err = tx.ExecContext(ctx, `
				UPDATE imported_channels
				SET source_sync_status='active', source_missing_at='', updated_at=?
				WHERE local_instance_id=? AND source_channel_id=?
			`, markedAt, instance.ID, sourceID)
			activeCount++
		} else {
			_, err = tx.ExecContext(ctx, `
				UPDATE imported_channels
				SET source_sync_status='missing',
				    source_missing_at=CASE WHEN COALESCE(source_missing_at,'')='' THEN ? ELSE source_missing_at END,
				    updated_at=?
				WHERE local_instance_id=? AND source_channel_id=?
			`, markedAt, markedAt, instance.ID, sourceID)
			missingCount++
		}
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	if err := s.UpdateLocalNewAPISyncToken(ctx, instance.ID, input.AccessToken, input.SaveAccessToken, input.ClearAccessToken); err != nil {
		return nil, err
	}

	if notify {
		s.infra.Notify("channels_reconciled", "info", "渠道状态已标记", fmt.Sprintf("%s：%d 条保持活跃，%d 条标记为源端已移除。", instance.Name, activeCount, missingCount), "local_newapi_instance", instance.ID)
	}
	return map[string]interface{}{
		"instanceId":   instance.ID,
		"sourceCount":  len(seenSourceIDs),
		"activeCount":  activeCount,
		"missingCount": missingCount,
		"markedAt":     markedAt,
	}, nil
}

// SourceChannelRecordsForLocalNewAPI reads source channel records from either
// the SQLite path or the admin API for the given instance. Exported so the
// host's *App forwarder (previewLocalNewAPIInstanceSync handler) can delegate
// to it.
func (s *Service) SourceChannelRecordsForLocalNewAPI(ctx context.Context, instance LocalNewAPIInstance, input SyncSourceInput) (string, []map[string]interface{}, error) {
	if strings.TrimSpace(instance.DatabasePath) != "" {
		_, records, err := readSQLiteChannelRecords(ctx, instance.DatabasePath)
		return "sqlite", records, err
	}
	if isHTTPURL(instance.BaseURL) {
		accessToken, err := s.resolveLocalNewAPISyncToken(ctx, instance, input.AccessToken)
		if err != nil {
			return "", nil, err
		}
		if strings.TrimSpace(accessToken) == "" {
			return "", nil, fmt.Errorf("该实例需要填写系统访问令牌后才能读取后台 API 渠道。")
		}
		records, err := s.fetchAllAdminAPIChannelRecords(ctx, instance.BaseURL, accessToken, input.UserID, input.PageSize)
		return "admin_api", records, err
	}
	return "", nil, fmt.Errorf("该实例没有可用的 SQLite 路径或后台 API 地址，无法读取渠道。")
}

// BuildLocalNewAPISyncPreview builds a diff preview of what would change if
// the given instance is synced from source.
func (s *Service) BuildLocalNewAPISyncPreview(ctx context.Context, instance LocalNewAPIInstance, source string, records []map[string]interface{}) (SyncPreview, error) {
	existing, err := s.existingImportedChannels(ctx, instance.ID)
	if err != nil {
		return SyncPreview{}, err
	}

	preview := SyncPreview{
		InstanceID:   instance.ID,
		InstanceName: instance.Name,
		Source:       source,
		Items:        []SyncPreviewItem{},
		GeneratedAt:  s.infra.Now(),
	}
	seenSourceIDs := map[string]bool{}
	for index, record := range records {
		prepared := prepareSyncRecord(record, source, index)
		seenSourceIDs[prepared.SourceID] = true
		item := SyncPreviewItem{
			SourceChannelID: prepared.SourceID,
			Name:            prepared.Name,
			BaseURL:         prepared.BaseURL,
			Status:          prepared.Status,
			UpstreamKind:    prepared.Kind,
		}

		if token, matched := excludedRelaySiteMatch(prepared.Name, prepared.BaseURL); matched {
			item.Action = "skipped"
			item.Reason = fmt.Sprintf("匹配排除规则 %q，已按你的要求跳过该类路由站。", token)
			preview.SkippedCount++
			preview.Items = append(preview.Items, item)
			continue
		}

		current, ok := existing[prepared.SourceID]
		if !ok {
			item.Action = "new"
			item.Reason = "本地还没有这个 source_channel_id，同步时会新增。"
			preview.NewCount++
			preview.Items = append(preview.Items, item)
			continue
		}

		changedFields := compareImportedChannelFields(current, prepared)
		if len(changedFields) == 0 {
			item.Action = "unchanged"
			item.Reason = "本地记录和上游 channels 当前字段一致。"
			preview.UnchangedCount++
		} else {
			item.Action = "changed"
			item.ChangedFields = changedFields
			item.Reason = "同步时会更新：" + strings.Join(changedFields, "、")
			preview.ChangedCount++
		}
		preview.Items = append(preview.Items, item)
	}
	for sourceID, current := range existing {
		if seenSourceIDs[sourceID] {
			continue
		}
		preview.RemovedCount++
		preview.Items = append(preview.Items, SyncPreviewItem{
			SourceChannelID: sourceID,
			Name:            current.Name,
			BaseURL:         current.BaseURL,
			Status:          current.Status,
			UpstreamKind:    current.Kind,
			Action:          "removed",
			Reason:          "本地存在，但本次源端 channels 没有返回。为避免误删，同步不会自动删除，请确认是否为后台已移除或筛选条件变化。",
		})
	}
	preview.Total = len(preview.Items)
	return preview, nil
}

// existingImportedChannels reads the current imported_channels rows for an
// instance, keyed by source_channel_id.
func (s *Service) existingImportedChannels(ctx context.Context, instanceID string) (map[string]existingImportedChannel, error) {
	rows, err := s.infra.DB().QueryContext(ctx, `
		SELECT source_channel_id, name, COALESCE(base_url,''), COALESCE(status,''), upstream_kind,
		       COALESCE(raw_json,''), COALESCE(channel_key_masked,'')
		FROM imported_channels
		WHERE local_instance_id = ?
	`, instanceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := map[string]existingImportedChannel{}
	for rows.Next() {
		var sourceID string
		var item existingImportedChannel
		if err := rows.Scan(&sourceID, &item.Name, &item.BaseURL, &item.Status, &item.Kind, &item.RawJSON, &item.KeyMasked); err != nil {
			return nil, err
		}
		items[sourceID] = item
	}
	return items, rows.Err()
}
