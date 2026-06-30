package core

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"strings"

	"relaycheck-desktop/internal/sites"
)

type ProbeResult struct {
	BaseURL        string   `json:"baseUrl"`
	Status         string   `json:"status"`
	Reachable      bool     `json:"reachable"`
	MatchedSignals []string `json:"matchedSignals"`
	Score          int      `json:"score"`
}

type UpstreamDetection struct {
	BaseURL             string   `json:"baseUrl"`
	HomepageURL         string   `json:"homepageUrl"`
	LoginURL            string   `json:"loginUrl"`
	Kind                string   `json:"kind"`
	HealthStatus        string   `json:"healthStatus"`
	DetectionConfidence float64  `json:"detectionConfidence"`
	SupportsCheckin     bool     `json:"supportsCheckin"`
	SupportsBalance     bool     `json:"supportsBalance"`
	SupportsModels      bool     `json:"supportsModels"`
	SupportsPricing     bool     `json:"supportsPricing"`
	MatchedSignals      []string `json:"matchedSignals"`
}

func (a *App) handleScanLocalNewAPI(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	results := a.scanTargets(r.Context(), sites.DefaultScanTargets())
	found := []ProbeResult{}
	for _, result := range results {
		if result.Reachable && result.Score > 0 {
			found = append(found, result)
			if _, execErr := a.db.ExecContext(r.Context(), `
				INSERT INTO local_newapi_instances (id, name, base_url, detected_from, status, last_scanned_at, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT(base_url) DO UPDATE SET status=excluded.status, last_scanned_at=excluded.last_scanned_at, updated_at=excluded.updated_at
			`, newID(), "Local "+hostLabel(result.BaseURL), result.BaseURL, "port_scan", result.Status, now(), now(), now()); execErr != nil {
				log.Printf("[scanner] local newapi instance upsert failed for %s: %v", result.BaseURL, execErr)
			}
		}
	}
	if len(found) > 0 {
		a.notify("local_newapi_discovered", "success", "本地 NewAPI 扫描完成", "发现可识别实例。", "", "")
	}
	writeJSON(w, http.StatusOK, found)
}

// scanTargets is the *App forwarder for sites.Service.ScanTargets. It probes
// each target concurrently and returns the per-target ProbeResult slice
// (index-aligned with targets). Callers (handleScanLocalNewAPI) are unaware
// of the sites service.
func (a *App) scanTargets(ctx context.Context, targets []string) []ProbeResult {
	siteResults := a.sitesService.ScanTargets(ctx, targets)
	results := make([]ProbeResult, len(siteResults))
	for i, r := range siteResults {
		results[i] = ProbeResult{
			BaseURL:        r.BaseURL,
			Status:         r.Status,
			Reachable:      r.Reachable,
			MatchedSignals: r.MatchedSignals,
			Score:          r.Score,
		}
	}
	return results
}

// probeLocal is the *App forwarder for sites.Service.ProbeLocal. It probes a
// single local NewAPI instance and returns the ProbeResult. Callers
// (scanTargets) are unaware of the sites service.
func (a *App) probeLocal(ctx context.Context, raw string) ProbeResult {
	r := a.sitesService.ProbeLocal(ctx, raw)
	return ProbeResult{
		BaseURL:        r.BaseURL,
		Status:         r.Status,
		Reachable:      r.Reachable,
		MatchedSignals: r.MatchedSignals,
		Score:          r.Score,
	}
}

// detectUpstream is the *App forwarder for sites.Service.DetectUpstream. It
// probes a remote base URL and returns the aggregated UpstreamDetection.
// Callers (channels.go, accounts.go, import_sqlite.go, import_admin_api.go,
// sites.go, detection_detail.go) are unaware of the sites service.
func (a *App) detectUpstream(ctx context.Context, raw string) UpstreamDetection {
	d := a.sitesService.DetectUpstream(ctx, raw)
	return UpstreamDetection{
		BaseURL:             d.BaseURL,
		HomepageURL:         d.HomepageURL,
		LoginURL:            d.LoginURL,
		Kind:                d.Kind,
		HealthStatus:        d.HealthStatus,
		DetectionConfidence: d.DetectionConfidence,
		SupportsCheckin:     d.SupportsCheckin,
		SupportsBalance:     d.SupportsBalance,
		SupportsModels:      d.SupportsModels,
		SupportsPricing:     d.SupportsPricing,
		MatchedSignals:      d.MatchedSignals,
	}
}

// ensureUpstreamSiteForChannel is the *App forwarder for
// sites.Service.EnsureUpstreamSiteForChannel. It upserts an upstream_sites
// row for the given channel, converting the core.UpstreamDetection to a
// sites.Detection so callers (channels.go, import_sqlite.go,
// import_admin_api.go) are unaware of the sites service.
func (a *App) ensureUpstreamSiteForChannel(ctx context.Context, channelID string, name string, rawBaseURL string, kind string, detection *UpstreamDetection) (string, bool, error) {
	var siteDetection *sites.Detection
	if detection != nil {
		siteDetection = &sites.Detection{
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
		}
	}
	return a.sitesService.EnsureUpstreamSiteForChannel(ctx, sites.EnsureSiteInput{
		ChannelID:  channelID,
		Name:       name,
		RawBaseURL: rawBaseURL,
		Kind:       kind,
		Detection:  siteDetection,
	})
}

// normalizeBaseURL is shared by accounts.go, channels.go, import_admin_api.go,
// legacy_config.go, url_safety.go, and checkin_balance.go. It is duplicated
// in the sites package so that package can build fallback base URLs without
// importing core; the two copies must stay in sync.
func normalizeBaseURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return strings.TrimRight(raw, "/")
	}
	parsed.Path = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/")
}

// hostLabel extracts the host portion of raw for display. Used by
// handleScanLocalNewAPI, accounts.go, import_admin_api.go, legacy_config.go.
// Duplicated in the sites package as HostLabel.
func hostLabel(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	return parsed.Host
}

// isOfficialProviderBaseURL reports whether raw points at a known official
// provider. Used by channels.go and sites.go (normalizeOfficialProviderSite).
// Duplicated in the sites package so that package can classify detections
// without importing core; the two copies must stay in sync.
func isOfficialProviderBaseURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return false
	}
	officialDomains := []string{
		"api.openai.com",
		"api.anthropic.com",
		"api.mistral.ai",
		"generativelanguage.googleapis.com",
		"aiplatform.googleapis.com",
		"dashscope.aliyuncs.com",
		"open.bigmodel.cn",
		"api.moonshot.cn",
		"api.deepseek.com",
		"api.siliconflow.cn",
		"api.minimax.chat",
		"ark.cn-beijing.volces.com",
		"maas-api.ml-platform-cn-beijing.volces.com",
		"token.sensenova.cn",
	}
	for _, domain := range officialDomains {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	officialSuffixes := []string{
		".sensenova.cn",
		".sensecore.cn",
	}
	for _, suffix := range officialSuffixes {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}
	return false
}

// isCheckinDisabledText reports whether body contains text indicating that
// the check-in feature is disabled. Used by checkin_balance.go and (via the
// sites package's own copy) the sites detection engine.
func isCheckinDisabledText(body string) bool {
	text := strings.ToLower(body)
	if strings.Contains(body, "签到功能未启用") ||
		strings.Contains(body, "未开启签到") ||
		strings.Contains(body, "未启用签到") ||
		strings.Contains(body, "不支持签到") {
		return true
	}
	return strings.Contains(body, "签到功能未启用") ||
		strings.Contains(body, "未开启签到") ||
		strings.Contains(body, "未启用签到") ||
		strings.Contains(body, "不支持签到") ||
		strings.Contains(text, "checkin disabled") ||
		strings.Contains(text, "check-in disabled") ||
		strings.Contains(text, "signin disabled") ||
		strings.Contains(text, "sign-in disabled") ||
		strings.Contains(text, "checkin not enabled") ||
		strings.Contains(text, "sign in not enabled") ||
		strings.Contains(text, "signin not enabled")
}
