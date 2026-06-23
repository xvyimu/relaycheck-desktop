package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
)

type existingImportedChannel struct {
	Name      string
	BaseURL   string
	Status    string
	Kind      string
	RawJSON   string
	KeyMasked string
}

type preparedSyncRecord struct {
	SourceID  string
	Name      string
	BaseURL   string
	Status    string
	Kind      string
	RawJSON   string
	KeyMasked string
}

type localNewAPISyncSourceInput struct {
	AccessToken      string `json:"accessToken"`
	SaveAccessToken  bool   `json:"saveAccessToken"`
	ClearAccessToken bool   `json:"clearAccessToken"`
	UserID           string `json:"userId"`
	PageSize         int    `json:"pageSize"`
}

func (a *App) previewLocalNewAPIInstanceSync(w http.ResponseWriter, r *http.Request, id string) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var input localNewAPISyncSourceInput
	_ = decodeJSON(r, &input)
	normalizeLocalNewAPISyncSourceInput(&input)

	instance, err := a.getLocalNewAPIInstance(r.Context(), id)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "NewAPI 实例不存在。")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	source, records, err := a.sourceChannelRecordsForLocalNewAPI(r.Context(), instance, input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	preview, err := a.buildLocalNewAPISyncPreview(r.Context(), instance, source, records)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (a *App) markMissingLocalNewAPIInstance(w http.ResponseWriter, r *http.Request, id string) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var input localNewAPISyncSourceInput
	_ = decodeJSON(r, &input)
	normalizeLocalNewAPISyncSourceInput(&input)

	instance, err := a.getLocalNewAPIInstance(r.Context(), id)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "NewAPI 实例不存在。")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	result, err := a.reconcileMissingLocalNewAPIInstance(r.Context(), instance, input, true)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *App) reconcileMissingLocalNewAPIInstance(ctx context.Context, instance LocalNewAPIInstance, input localNewAPISyncSourceInput, notify bool) (map[string]interface{}, error) {
	_, records, err := a.sourceChannelRecordsForLocalNewAPI(ctx, instance, input)
	if err != nil {
		return nil, err
	}
	seenSourceIDs := sourceIDSetFromRecords(records)
	existing, err := a.existingImportedChannels(ctx, instance.ID)
	if err != nil {
		return nil, err
	}

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	activeCount := 0
	missingCount := 0
	markedAt := now()
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
	if err := a.updateLocalNewAPISyncToken(ctx, instance.ID, input.AccessToken, input.SaveAccessToken, input.ClearAccessToken); err != nil {
		return nil, err
	}

	if notify {
		a.notify("channels_reconciled", "info", "渠道状态已标记", fmt.Sprintf("%s：%d 条保持活跃，%d 条标记为源端已移除。", instance.Name, activeCount, missingCount), "local_newapi_instance", instance.ID)
	}
	return map[string]interface{}{
		"instanceId":   instance.ID,
		"sourceCount":  len(seenSourceIDs),
		"activeCount":  activeCount,
		"missingCount": missingCount,
		"markedAt":     markedAt,
	}, nil
}

func normalizeLocalNewAPISyncSourceInput(input *localNewAPISyncSourceInput) {
	if strings.TrimSpace(input.UserID) == "" {
		input.UserID = "1"
	}
	input.PageSize = clampInt(input.PageSize, 10, 100, 100)
}

func (a *App) sourceChannelRecordsForLocalNewAPI(ctx context.Context, instance LocalNewAPIInstance, input localNewAPISyncSourceInput) (string, []map[string]interface{}, error) {
	if strings.TrimSpace(instance.DatabasePath) != "" {
		_, records, err := readSQLiteChannelRecords(ctx, instance.DatabasePath)
		return "sqlite", records, err
	}
	if isHTTPURL(instance.BaseURL) {
		accessToken, err := a.resolveLocalNewAPISyncToken(ctx, instance, input.AccessToken)
		if err != nil {
			return "", nil, err
		}
		if strings.TrimSpace(accessToken) == "" {
			return "", nil, fmt.Errorf("该实例需要填写系统访问令牌后才能读取后台 API 渠道。")
		}
		records, err := a.fetchAllAdminAPIChannelRecords(ctx, instance.BaseURL, accessToken, input.UserID, input.PageSize)
		return "admin_api", records, err
	}
	return "", nil, fmt.Errorf("该实例没有可用的 SQLite 路径或后台 API 地址，无法读取渠道。")
}

func sourceIDSetFromRecords(records []map[string]interface{}) map[string]bool {
	seen := map[string]bool{}
	for index, record := range records {
		sourceID := stringValue(record, "id")
		if sourceID == "" {
			sourceID = fmt.Sprintf("row-%d", index+1)
		}
		seen[sourceID] = true
	}
	return seen
}

func (a *App) fetchAllAdminAPIChannelRecords(ctx context.Context, baseURL string, accessToken string, userID string, pageSize int) ([]map[string]interface{}, error) {
	records := []map[string]interface{}{}
	for page := 0; page < 200; page++ {
		items, err := a.fetchAdminAPIChannels(ctx, baseURL, accessToken, userID, page, pageSize)
		if err != nil {
			return nil, err
		}
		if len(items) == 0 {
			break
		}
		records = append(records, items...)
		if len(items) < pageSize {
			break
		}
	}
	return records, nil
}

func readSQLiteChannelRecords(ctx context.Context, dbPath string) (string, []map[string]interface{}, error) {
	cleanPath, err := filepath.Abs(dbPath)
	if err != nil {
		return "", nil, err
	}
	source, err := sql.Open("sqlite", "file:"+filepath.ToSlash(cleanPath)+"?mode=ro")
	if err != nil {
		return "", nil, err
	}
	defer source.Close()

	var tableName string
	if err := source.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name='channels'`).Scan(&tableName); err != nil {
		return "", nil, fmt.Errorf("未找到 channels 表")
	}

	rows, err := source.QueryContext(ctx, `SELECT * FROM channels`)
	if err != nil {
		return "", nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return "", nil, err
	}

	records := []map[string]interface{}{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		dest := make([]interface{}, len(columns))
		for i := range values {
			dest[i] = &values[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return "", nil, err
		}
		record := map[string]interface{}{}
		for i, col := range columns {
			record[col] = normalizeDBValue(values[i])
		}
		records = append(records, record)
	}
	return cleanPath, records, rows.Err()
}

func (a *App) buildLocalNewAPISyncPreview(ctx context.Context, instance LocalNewAPIInstance, source string, records []map[string]interface{}) (LocalNewAPISyncPreview, error) {
	existing, err := a.existingImportedChannels(ctx, instance.ID)
	if err != nil {
		return LocalNewAPISyncPreview{}, err
	}

	preview := LocalNewAPISyncPreview{
		InstanceID:   instance.ID,
		InstanceName: instance.Name,
		Source:       source,
		Items:        []SyncPreviewItem{},
		GeneratedAt:  now(),
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

func (a *App) existingImportedChannels(ctx context.Context, instanceID string) (map[string]existingImportedChannel, error) {
	rows, err := a.db.QueryContext(ctx, `
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

func prepareSyncRecord(record map[string]interface{}, importSource string, index int) preparedSyncRecord {
	sourceID := stringValue(record, "id")
	if sourceID == "" {
		sourceID = fmt.Sprintf("row-%d", index+1)
	}
	name := stringValue(record, "name")
	if name == "" {
		name = "渠道 " + sourceID
	}
	baseURL := extractImportedBaseURL(record)
	keyMasked := ""
	if keyValue := stringValue(record, "key"); keyValue != "" {
		keyMasked = maskSecret(keyValue)
	}
	rawJSON, _ := marshalImportedRecord(record, importSource)
	return preparedSyncRecord{
		SourceID:  sourceID,
		Name:      name,
		BaseURL:   baseURL,
		Status:    stringValue(record, "status"),
		Kind:      inferImportedKind(record, baseURL),
		RawJSON:   rawJSON,
		KeyMasked: keyMasked,
	}
}

func compareImportedChannelFields(current existingImportedChannel, next preparedSyncRecord) []string {
	fields := []string{}
	if current.Name != next.Name {
		fields = append(fields, "名称")
	}
	if strings.TrimRight(current.BaseURL, "/") != strings.TrimRight(next.BaseURL, "/") {
		fields = append(fields, "API 地址")
	}
	if current.Status != next.Status {
		fields = append(fields, "状态")
	}
	if current.Kind != next.Kind {
		fields = append(fields, "识别类型")
	}
	if next.KeyMasked != "" && current.KeyMasked != "" && current.KeyMasked != next.KeyMasked {
		fields = append(fields, "渠道 Key")
	}
	if current.RawJSON != "" && next.RawJSON != "" && current.RawJSON != next.RawJSON {
		fields = append(fields, "原始配置")
	}
	return fields
}

func marshalImportedRecord(record map[string]interface{}, importSource string) (string, error) {
	safeRecord := map[string]interface{}{}
	for key, value := range record {
		safeRecord[key] = maskSensitiveImportedValue(key, value)
	}
	if importSource != "" {
		safeRecord["import_source"] = importSource
	}
	rawJSON, err := json.Marshal(safeRecord)
	if err != nil {
		return "", err
	}
	return string(rawJSON), nil
}

func maskSensitiveImportedValue(key string, value interface{}) interface{} {
	if value == nil {
		return nil
	}
	if isSensitiveImportedField(key) {
		return maskSecret(fmt.Sprint(value))
	}
	text, ok := value.(string)
	if !ok {
		return value
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || !(strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")) {
		return value
	}
	var parsed interface{}
	if json.Unmarshal([]byte(trimmed), &parsed) != nil {
		return value
	}
	masked := maskSensitiveJSONValue(parsed)
	next, err := json.Marshal(masked)
	if err != nil {
		return value
	}
	return string(next)
}

func maskSensitiveJSONValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		next := map[string]interface{}{}
		for key, child := range typed {
			if isSensitiveImportedField(key) {
				next[key] = maskSecret(fmt.Sprint(child))
			} else {
				next[key] = maskSensitiveJSONValue(child)
			}
		}
		return next
	case []interface{}:
		next := make([]interface{}, 0, len(typed))
		for _, child := range typed {
			next = append(next, maskSensitiveJSONValue(child))
		}
		return next
	default:
		return value
	}
}

func isSensitiveImportedField(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(key), "-", "_"))
	switch normalized {
	case "key", "api_key", "apikey", "access_key", "secret", "password", "cookie", "authorization", "bearer", "token", "access_token", "refresh_token":
		return true
	default:
		return false
	}
}
