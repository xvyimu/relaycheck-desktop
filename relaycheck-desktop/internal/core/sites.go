package core

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"sync"

	"relaycheck-desktop/internal/sites"
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
		siteItems, err := a.sitesService.ListUpstreamSites(r.Context())
		if err != nil {
			return nil, err
		}
		items := make([]UpstreamSite, len(siteItems))
		for i, s := range siteItems {
			items[i] = UpstreamSite{
				ID:                  s.ID,
				ChannelID:           s.ChannelID,
				Name:                s.Name,
				HomepageURL:         s.HomepageURL,
				BaseURL:             s.BaseURL,
				LoginURL:            s.LoginURL,
				Kind:                s.Kind,
				DetectionConfidence: s.DetectionConfidence,
				HealthStatus:        s.HealthStatus,
				SupportsCheckin:     s.SupportsCheckin,
				SupportsBalance:     s.SupportsBalance,
				SupportsModels:      s.SupportsModels,
				SupportsPricing:     s.SupportsPricing,
				AccountCount:        s.AccountCount,
				DetectionJSON:       s.DetectionJSON,
				LastHealthCheckAt:   s.LastHealthCheckAt,
				CreatedAt:           s.CreatedAt,
				UpdatedAt:           s.UpdatedAt,
			}
		}
		return items, nil
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// normalizeOfficialProviderSite forces official-provider sites to a canonical
// kind/health. Kept in core because detection_detail.go's loadSiteDetail uses
// it on core.UpstreamSite values. The sites package has its own copy for
// sites.Site values.
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

	siteID, err := a.sitesService.CreateUpstreamSite(r.Context(), sites.CreateSiteInput{
		Name:     input.Name,
		BaseURL:  input.BaseURL,
		LoginURL: input.LoginURL,
		Kind:     input.Kind,
	}, sites.Detection{
		BaseURL:             detection.BaseURL,
		HomepageURL:         detection.HomepageURL,
		LoginURL:            detection.LoginURL,
		Kind:                detection.Kind,
		HealthStatus:        detection.HealthStatus,
		DetectionConfidence: detection.DetectionConfidence,
		SupportsCheckin:     detection.SupportsCheckin,
		SupportsBalance:     detection.SupportsBalance,
		SupportsModels:      detection.SupportsModels,
		SupportsPricing:     detection.SupportsPricing,
		MatchedSignals:      detection.MatchedSignals,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
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
	detection, err := a.sitesService.DetectUpstreamSite(r.Context(), id)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "上游站点不存在。")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, UpstreamDetection{
		BaseURL:             detection.BaseURL,
		HomepageURL:         detection.HomepageURL,
		LoginURL:            detection.LoginURL,
		Kind:                detection.Kind,
		HealthStatus:        detection.HealthStatus,
		DetectionConfidence: detection.DetectionConfidence,
		SupportsCheckin:     detection.SupportsCheckin,
		SupportsBalance:     detection.SupportsBalance,
		SupportsModels:      detection.SupportsModels,
		SupportsPricing:     detection.SupportsPricing,
		MatchedSignals:      detection.MatchedSignals,
	})
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
		  AND id <> ?
		  AND lower(name) NOT LIKE '%9router%'
		  AND lower(base_url) <> 'http://localhost:20128'
	`
	args := []interface{}{globalScheduleSiteID}
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
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
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

// detectAndSaveSite is the *App forwarder for sites.Service.DetectAndSaveSite.
// It re-probes a site, persists the detection, and returns the per-site
// result used by the bulk-detect endpoint. Callers (handleBulkDetectUpstreamSites)
// are unaware of the sites service.
func (a *App) detectAndSaveSite(ctx context.Context, id string, name string, baseURL string) bulkDetectSiteResult {
	r := a.sitesService.DetectAndSaveSite(ctx, id, name, baseURL)
	return bulkDetectSiteResult{
		ID:              r.ID,
		Name:            r.Name,
		BaseURL:         r.BaseURL,
		Kind:            r.Kind,
		HealthStatus:    r.HealthStatus,
		SupportsCheckin: r.SupportsCheckin,
		Error:           r.Error,
	}
}

func (a *App) deleteUpstreamSite(w http.ResponseWriter, r *http.Request, id string) {
	if err := a.sitesService.DeleteUpstreamSite(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
