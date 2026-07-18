package core

import (
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

func (a *App) handleModelOverview(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	overview, err := cachedRead(a, "models-overview", overviewReadCacheTTL, func() (modelOverview, error) {
		records, err := a.loadAccountModelRecords(r)
		if err != nil {
			return modelOverview{}, err
		}
		return buildModelOverview(records), nil
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, overview)
}

func (a *App) handleModelSync(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var input struct {
		Limit int `json:"limit"`
	}
	if r.ContentLength != 0 {
		if err := decodeJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, "请求参数无效。")
			return
		}
	}
	input.Limit = clampBatchLimit(input.Limit, 10)
	rows, err := a.db.QueryContext(r.Context(), `
		SELECT id FROM channel_accounts
		WHERE COALESCE(api_key_encrypted,'') <> ''
		ORDER BY COALESCE(api_key_last_checked_at,''), updated_at DESC
		LIMIT ?
	`, input.Limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			log.Printf("[model-sync] scan id failed: %v", err)
			continue
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = rows.Close()
	auths, _ := a.loadAccountAuths(r.Context(), ids)
	successCount := 0
	failedCount := 0
	for _, id := range ids {
		var auth *accountAuthContext
		if loaded, ok := auths[id]; ok {
			auth = &loaded
		}
		result := a.testAPIKeyForAccount(r.Context(), id, auth)
		if result.Status == "failed" {
			log.Printf("[model-sync] test API key failed for account %s: %s", id, result.Message)
			failedCount++
		} else {
			successCount++
		}
	}
	if len(ids) > 0 {
		level := "success"
		msg := fmt.Sprintf("已测试 %d 个账号的 API Key。", len(ids))
		if failedCount > 0 {
			level = "warning"
			msg = fmt.Sprintf("已测试 %d 个 API Key，成功 %d 个，失败 %d 个。", len(ids), successCount, failedCount)
		}
		a.notify("model_sync_completed", level, "模型同步完成", msg, "model", "")
	}
	records, err := a.loadAccountModelRecords(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	overview := buildModelOverview(records)
	overview.SyncedAccounts = len(ids)
	if len(ids) > 0 {
		a.notify("model_sync_completed", "success", "模型同步完成", "已检测并同步 "+strconv.Itoa(len(ids))+" 个 Key 的模型状态。", "model", "")
	}
	writeJSON(w, http.StatusOK, overview)
}

func (a *App) handleKeyExportPreview(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	records, err := a.loadAccountModelRecords(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	preview := keyExportPreview{
		GeneratedAt: now(),
		Items:       []keyExportPreviewItem{},
		Notice:      "安全模式只导出账号、站点、Key 指纹、模型和测速状态，不导出真实 API Key。如需真实密钥，请在原平台重新生成或手动复制。",
	}
	for _, record := range records {
		if record.Fingerprint == "" {
			continue
		}
		preview.Total++
		if record.Status == "valid" {
			preview.Valid++
		}
		if record.ModelUsable {
			preview.Usable++
		}
		preview.Items = append(preview.Items, keyExportPreviewItem{
			AccountID:       record.AccountID,
			AccountName:     record.AccountName,
			SiteName:        record.SiteName,
			BaseURL:         record.BaseURL,
			Fingerprint:     record.Fingerprint,
			Status:          firstNonEmpty(record.Status, "unchecked"),
			ModelCount:      record.ModelCount,
			SampleModels:    limitStrings(record.SampleModels, 6),
			TestModel:       record.TestModel,
			ModelUsable:     record.ModelUsable,
			LatencyMs:       record.LatencyMs,
			LastCheckedAt:   record.LastCheckedAt,
			MaskedExportRef: record.SiteName + " · " + record.Fingerprint,
		})
	}
	sort.SliceStable(preview.Items, func(i, j int) bool {
		left := preview.Items[i]
		right := preview.Items[j]
		if left.Status != right.Status {
			return left.Status == "valid"
		}
		if left.ModelUsable != right.ModelUsable {
			return left.ModelUsable
		}
		return left.SiteName < right.SiteName
	})
	a.audit("keys.export_preview", "info", "", "api_key", "", fmt.Sprintf("Key 脱敏导出预览：%d 个指纹。", preview.Total), map[string]interface{}{"total": preview.Total, "valid": preview.Valid, "usable": preview.Usable})
	writeJSON(w, http.StatusOK, preview)
}

func (a *App) handleModelPricing(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	overview, err := cachedRead(a, "models-pricing", overviewReadCacheTTL, func() (modelPricingOverview, error) {
		rawSources, err := a.loadRawChannelPricingSources(r.Context())
		if err != nil {
			return modelPricingOverview{}, err
		}
		cacheSources, cacheItems, err := a.loadSitePricingCache(r.Context())
		if err != nil {
			return modelPricingOverview{}, err
		}
		accountRecords, err := a.loadAccountModelRecords(r)
		if err != nil {
			return modelPricingOverview{}, err
		}
		return buildPricingOverview(append(rawSources, cacheSources...), cacheItems, accountRecords), nil
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, overview)
}

func (a *App) handleModelPricingSync(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var input struct {
		Limit int `json:"limit"`
	}
	if r.ContentLength != 0 {
		if err := decodeJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, "请求参数无效。")
			return
		}
	}
	input.Limit = clampBatchLimit(input.Limit, 10)
	records, err := a.loadPricingSiteRecords(r.Context(), input.Limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	failedSites := []string{}
	for _, record := range records {
		item := a.syncSitePricing(r.Context(), record)
		if item.Status != "" && item.Status != "success" {
			log.Printf("[pricing] sync site %s (%s) returned status=%s: %s", record.SiteID, record.SiteName, item.Status, item.Message)
			failedSites = append(failedSites, record.SiteName)
		}
	}
	rawSources, err := a.loadRawChannelPricingSources(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	cacheSources, cacheItems, err := a.loadSitePricingCache(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	accountRecords, err := a.loadAccountModelRecords(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	overview := buildPricingOverview(append(rawSources, cacheSources...), cacheItems, accountRecords)
	if len(records) > 0 {
		msg := fmt.Sprintf("已探测 %d 个上游站点的 /api/pricing。", len(records))
		level := "success"
		if len(failedSites) > 0 {
			level = "warning"
			msg += fmt.Sprintf(" %d 个站点同步失败：%s。", len(failedSites), strings.Join(failedSites, "、"))
		}
		a.notify("pricing_sync_completed", level, "价格同步完成", msg, "pricing", "")
	}
	writeJSON(w, http.StatusOK, overview)
}
