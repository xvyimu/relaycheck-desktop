package core

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"sync"
)

func (a *App) handleUpstreamSites(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.listUpstreamSites(w, r)
	case http.MethodPost:
		a.createUpstreamSite(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (a *App) listUpstreamSites(w http.ResponseWriter, r *http.Request) {
	items, err := cachedRead(a, "upstream-sites-list", shortReadCacheTTL, func() ([]UpstreamSite, error) {
		rows, err := a.db.QueryContext(r.Context(), `
		SELECT s.id, COALESCE(s.channel_id,''), s.name, COALESCE(s.homepage_url,''), s.base_url,
		       COALESCE(s.login_url,''), s.kind, s.detection_confidence, s.health_status,
		       s.supports_checkin, s.supports_balance, s.supports_models, s.supports_pricing,
		       COALESCE(s.detection_json,''), COALESCE(s.last_health_check_at,''), s.created_at, s.updated_at,
		       (SELECT COUNT(*) FROM channel_accounts a WHERE a.upstream_site_id = s.id)
		FROM upstream_sites s
		ORDER BY s.updated_at DESC
	`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		items := []UpstreamSite{}
		for rows.Next() {
			var item UpstreamSite
			var checkin, balance, models, pricing int
			if err := rows.Scan(&item.ID, &item.ChannelID, &item.Name, &item.HomepageURL, &item.BaseURL, &item.LoginURL, &item.Kind, &item.DetectionConfidence, &item.HealthStatus, &checkin, &balance, &models, &pricing, &item.DetectionJSON, &item.LastHealthCheckAt, &item.CreatedAt, &item.UpdatedAt, &item.AccountCount); err != nil {
				return nil, err
			}
			item.SupportsCheckin = checkin == 1
			item.SupportsBalance = balance == 1
			item.SupportsModels = models == 1
			item.SupportsPricing = pricing == 1
			normalizeOfficialProviderSite(&item)
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

func normalizeOfficialProviderSite(item *UpstreamSite) {
	if !isOfficialProviderBaseURL(item.BaseURL) {
		return
	}
	item.Kind = "official_provider"
	item.SupportsCheckin = false
	if item.HealthStatus == "" || item.HealthStatus == "unknown" {
		item.HealthStatus = "healthy"
	}
}

func (a *App) createUpstreamSite(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name     string `json:"name"`
		BaseURL  string `json:"baseUrl"`
		LoginURL string `json:"loginUrl"`
		Kind     string `json:"kind"`
	}
	if err := decodeJSON(r, &input); err != nil || input.Name == "" || input.BaseURL == "" {
		writeError(w, http.StatusBadRequest, "上游站点参数不完整。")
		return
	}
	if isExcludedRelaySite(input.Name, input.BaseURL) {
		writeError(w, http.StatusBadRequest, "9router 已被排除，不再作为中转站导入。")
		return
	}
	detection := a.detectUpstream(r.Context(), input.BaseURL)
	if input.Kind != "" && input.Kind != "unknown" {
		detection.Kind = input.Kind
	}
	if !isManagedRelayKind(detection.Kind) {
		writeError(w, http.StatusBadRequest, "该地址未识别为 NewAPI/OneAPI/Sub2API/魔改中转面板型中转站，已跳过。")
		return
	}
	if strings.TrimSpace(input.LoginURL) != "" {
		detection.LoginURL = strings.TrimSpace(input.LoginURL)
	}

	channelID := newID()
	siteID := newID()
	detectionJSON := marshalDetection(&detection)
	_, err := a.db.ExecContext(r.Context(), `
		INSERT INTO imported_channels (id, source_channel_id, name, base_url, status, upstream_kind, supports_checkin, supports_balance, supports_models, supports_pricing, raw_json, detection_json, last_detected_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, 'manual', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, channelID, "manual-"+channelID, input.Name, detection.BaseURL, detection.Kind, boolInt(detection.SupportsCheckin), boolInt(detection.SupportsBalance), boolInt(detection.SupportsModels), boolInt(detection.SupportsPricing), `{"source":"manual"}`, detectionJSON, now(), now(), now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	_, err = a.db.ExecContext(r.Context(), `
		INSERT INTO upstream_sites (id, channel_id, name, homepage_url, base_url, login_url, kind, detection_confidence, health_status, supports_checkin, supports_balance, supports_models, supports_pricing, detection_json, last_health_check_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, siteID, channelID, input.Name, detection.HomepageURL, detection.BaseURL, detection.LoginURL, detection.Kind, detection.DetectionConfidence, detection.HealthStatus, boolInt(detection.SupportsCheckin), boolInt(detection.SupportsBalance), boolInt(detection.SupportsModels), boolInt(detection.SupportsPricing), detectionJSON, now(), now(), now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.notify("upstream_site_created", "success", "上游站点已添加", input.Name+" 已加入站点列表。", "upstream_site", siteID)
	writeJSON(w, http.StatusOK, map[string]string{"id": siteID})
}

func (a *App) handleUpstreamSiteByID(w http.ResponseWriter, r *http.Request) {
	tail := pathTail(r.URL.Path, "/api/upstream-sites/")
	if strings.HasSuffix(tail, "/detect") {
		id := strings.TrimSuffix(tail, "/detect")
		a.detectUpstreamSite(w, r, id)
		return
	}
	id := tail
	if r.Method == http.MethodGet {
		a.getUpstreamSiteDetail(w, r, id)
		return
	}
	if r.Method == http.MethodDelete {
		a.deleteUpstreamSite(w, r, id)
		return
	}
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func (a *App) getUpstreamSiteDetail(w http.ResponseWriter, r *http.Request, id string) {
	detail, err := a.loadSiteDetail(r.Context(), id)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "上游站点不存在。")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (a *App) detectUpstreamSite(w http.ResponseWriter, r *http.Request, id string) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var baseURL string
	err := a.db.QueryRowContext(r.Context(), `SELECT base_url FROM upstream_sites WHERE id = ?`, id).Scan(&baseURL)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "上游站点不存在。")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	detection := a.detectUpstream(r.Context(), baseURL)
	_, err = a.db.ExecContext(r.Context(), `
		UPDATE upstream_sites
		SET homepage_url=?, base_url=?, kind=?, detection_confidence=?, health_status=?, supports_checkin=?, supports_balance=?, supports_models=?, supports_pricing=?, detection_json=?, last_health_check_at=?, updated_at=?
		WHERE id=?
	`, detection.HomepageURL, detection.BaseURL, detection.Kind, detection.DetectionConfidence, detection.HealthStatus, boolInt(detection.SupportsCheckin), boolInt(detection.SupportsBalance), boolInt(detection.SupportsModels), boolInt(detection.SupportsPricing), marshalDetection(&detection), now(), now(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, detection)
}

type bulkDetectSiteResult struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	BaseURL         string `json:"baseUrl"`
	Kind            string `json:"kind"`
	HealthStatus    string `json:"healthStatus"`
	SupportsCheckin bool   `json:"supportsCheckin"`
	Error           string `json:"error,omitempty"`
}

func (a *App) handleBulkDetectUpstreamSites(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var input struct {
		Limit               int  `json:"limit"`
		OnlyUnknownOrOpenAI bool `json:"onlyUnknownOrOpenAI"`
	}
	_ = decodeJSON(r, &input)
	input.Limit = clampBatchLimit(input.Limit, 10)
	query := `
		SELECT id, name, base_url
		FROM upstream_sites
		WHERE COALESCE(base_url,'') <> ''
		  AND lower(name) NOT LIKE '%9router%'
		  AND lower(base_url) <> 'http://localhost:20128'
	`
	args := []interface{}{}
	if input.OnlyUnknownOrOpenAI {
		query += ` AND kind IN ('unknown','openai_compatible')`
	}
	query += ` ORDER BY updated_at DESC LIMIT ?`
	args = append(args, input.Limit)

	rows, err := a.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	type siteJob struct {
		ID      string
		Name    string
		BaseURL string
	}
	jobs := []siteJob{}
	for rows.Next() {
		var job siteJob
		if err := rows.Scan(&job.ID, &job.Name, &job.BaseURL); err == nil {
			jobs = append(jobs, job)
		}
	}
	_ = rows.Close()

	results := make([]bulkDetectSiteResult, len(jobs))
	worker := make(chan struct{}, clampBatchLimit(len(jobs), 5))
	var wg sync.WaitGroup
	for index, job := range jobs {
		wg.Add(1)
		worker <- struct{}{}
		go func(i int, site siteJob) {
			defer wg.Done()
			defer func() { <-worker }()
			results[i] = a.detectAndSaveSite(r.Context(), site.ID, site.Name, site.BaseURL)
		}(index, job)
	}
	wg.Wait()

	identified := 0
	for _, result := range results {
		if isManagedRelayKind(result.Kind) {
			identified++
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"processed":  len(results),
		"identified": identified,
		"results":    results,
	})
}

func (a *App) detectAndSaveSite(ctx context.Context, id string, name string, baseURL string) bulkDetectSiteResult {
	result := bulkDetectSiteResult{ID: id, Name: name, BaseURL: baseURL}
	detection := a.detectUpstream(ctx, baseURL)
	_, err := a.db.ExecContext(ctx, `
		UPDATE upstream_sites
		SET homepage_url=?, base_url=?, kind=?, detection_confidence=?, health_status=?, supports_checkin=?, supports_balance=?, supports_models=?, supports_pricing=?, detection_json=?, last_health_check_at=?, updated_at=?
		WHERE id=?
	`, detection.HomepageURL, detection.BaseURL, detection.Kind, detection.DetectionConfidence, detection.HealthStatus, boolInt(detection.SupportsCheckin), boolInt(detection.SupportsBalance), boolInt(detection.SupportsModels), boolInt(detection.SupportsPricing), marshalDetection(&detection), now(), now(), id)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.BaseURL = detection.BaseURL
	result.Kind = detection.Kind
	result.HealthStatus = detection.HealthStatus
	result.SupportsCheckin = detection.SupportsCheckin
	return result
}

func (a *App) deleteUpstreamSite(w http.ResponseWriter, r *http.Request, id string) {
	_, err := a.db.ExecContext(r.Context(), `DELETE FROM upstream_sites WHERE id = ?`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.audit("upstream_site.deleted", "warning", "", "upstream_site", id, "上游站点已删除", nil)
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
