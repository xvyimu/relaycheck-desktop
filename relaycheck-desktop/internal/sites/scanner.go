package sites

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

var defaultScanTargets = []string{
	"http://127.0.0.1:3000",
	"http://localhost:3000",
	"http://127.0.0.1:3001",
	"http://localhost:3001",
	"http://127.0.0.1:8080",
	"http://localhost:8080",
	"http://127.0.0.1:9999",
	"http://localhost:9999",
	"http://127.0.0.1:3010",
	"http://localhost:3010",
}

var localProbePaths = []string{
	"/", "/login", "/api/status", "/api/user/self", "/api/user/token", "/api/user/models",
	"/api/channel/", "/api/channel/models", "/api/token/",
}
var upstreamProbePaths = []string{
	"/", "/login", "/v1/models", "/v1/usage", "/v1beta/models",
	"/api/status", "/api/about", "/api/home_page_content",
	"/api/user/self", "/api/user/self/groups", "/api/user/token", "/api/user/models",
	"/api/user/available_models", "/api/user/dashboard", "/api/user/quota", "/api/user/topup/info",
	"/api/user/login", "/api/auth/login", "/api/login", "/api/user/register",
	"/api/user/checkin", "/api/checkin", "/api/user/check_in",
	"/api/pricing", "/api/option", "/api/group", "/api/redemption",
	"/api/channel/", "/api/channel/models", "/api/token/", "/api/log/token",
	"/api/subscription/self",
	"/api/v1/status", "/api/v1/settings/public", "/api/v1/auth/me", "/api/v1/user", "/api/v1/user/profile",
	"/api/v1/keys", "/api/v1/tokens", "/api/v1/accounts", "/api/v1/groups/available",
	"/api/v1/channels/available", "/api/v1/subscriptions/active", "/api/v1/user/platform-quotas",
}

// checkinCandidate mirrors the host's apiCandidate for probe-path assembly.
// It is duplicated here so the sites package does not import core.
type checkinCandidate struct {
	Method string
	Path   string
}

var checkinCandidates = []checkinCandidate{
	{http.MethodPost, "/api/user/checkin"},
	{http.MethodGet, "/api/user/checkin"},
	{http.MethodPost, "/api/checkin"},
	{http.MethodGet, "/api/checkin"},
	{http.MethodPost, "/api/user/check_in"},
	{http.MethodGet, "/api/user/check_in"},
	{http.MethodPost, "/api/user/signin"},
	{http.MethodGet, "/api/user/signin"},
	{http.MethodPost, "/api/user/sign_in"},
	{http.MethodGet, "/api/user/sign_in"},
	{http.MethodPost, "/api/user/sign-in"},
	{http.MethodGet, "/api/user/sign-in"},
	{http.MethodPost, "/api/signin"},
	{http.MethodGet, "/api/signin"},
	{http.MethodPost, "/api/sign_in"},
	{http.MethodGet, "/api/sign_in"},
	{http.MethodPost, "/api/sign-in"},
	{http.MethodGet, "/api/sign-in"},
	{http.MethodPost, "/api/daily_checkin"},
	{http.MethodGet, "/api/daily_checkin"},
	{http.MethodPost, "/api/daily-checkin"},
	{http.MethodGet, "/api/daily-checkin"},
}

type probeSpec struct {
	Method    string
	Path      string
	StatusKey string
	Checkin   bool
}

type probeFetchResult struct {
	Spec   probeSpec
	Status int
	Body   string
}

// DefaultScanTargets returns the default local NewAPI scan targets so the
// host handler can call ScanTargets without re-declaring the list.
func DefaultScanTargets() []string {
	return defaultScanTargets
}

// ScanTargets probes each target concurrently and returns the per-target
// ProbeResult slice (index-aligned with targets).
func (s *Service) ScanTargets(ctx context.Context, targets []string) []ProbeResult {
	results := make([]ProbeResult, len(targets))
	var wg sync.WaitGroup
	for index, target := range targets {
		wg.Add(1)
		go func(i int, raw string) {
			defer wg.Done()
			results[i] = s.ProbeLocal(ctx, raw)
		}(index, target)
	}
	wg.Wait()
	return results
}

// ProbeLocal probes a single local NewAPI instance and returns the result.
func (s *Service) ProbeLocal(ctx context.Context, raw string) ProbeResult {
	baseURL, err := s.safeBaseURL(ctx, raw, true)
	if err != nil {
		return ProbeResult{BaseURL: normalizeBaseURL(raw), Status: "unreachable", Reachable: false, Score: 0}
	}
	signals := map[string]bool{}
	reachable := false
	specs := make([]probeSpec, 0, len(localProbePaths))
	for _, path := range localProbePaths {
		specs = append(specs, probeSpec{Method: http.MethodGet, Path: path, StatusKey: path})
	}
	for _, result := range s.fetchProbeBatch(ctx, baseURL, specs, 6) {
		if result.Status > 0 {
			reachable = true
		}
		inspectSignals(result.Spec.Path, result.Status, result.Body, signals)
	}
	score := len(signals)
	status := "unknown"
	if !reachable {
		status = "unreachable"
	} else if score >= 2 {
		status = "healthy"
	} else if score > 0 {
		status = "degraded"
	}
	return ProbeResult{BaseURL: baseURL, Status: status, Reachable: reachable, Score: score, MatchedSignals: signalList(signals)}
}

// DetectUpstream probes a remote base URL and returns the aggregated
// Detection describing kind, health, and feature support.
func (s *Service) DetectUpstream(ctx context.Context, raw string) Detection {
	baseURL, err := s.safeBaseURL(ctx, raw, s.infra.AllowLocalOutbound())
	if err != nil {
		return Detection{
			BaseURL:      normalizeBaseURL(raw),
			HomepageURL:  normalizeBaseURL(raw),
			LoginURL:     strings.TrimRight(normalizeBaseURL(raw), "/") + "/login",
			Kind:         "unknown",
			HealthStatus: "blocked",
			MatchedSignals: []string{
				"blocked-unsafe-url",
			},
		}
	}
	signals := map[string]bool{}
	reachable := false
	statusByPath := map[string]int{}
	specs := make([]probeSpec, 0, len(upstreamProbePaths)+len(checkinCandidates))
	for _, path := range upstreamProbePaths {
		specs = append(specs, probeSpec{Method: http.MethodGet, Path: path, StatusKey: path})
	}
	specs = append(specs,
		probeSpec{Method: http.MethodPost, Path: "/api/v1/auth/login", StatusKey: "POST /api/v1/auth/login"},
		probeSpec{Method: http.MethodPost, Path: "/chat/completions", StatusKey: "POST /chat/completions"},
		probeSpec{Method: http.MethodPost, Path: "/responses", StatusKey: "POST /responses"},
	)
	for _, candidate := range checkinCandidates {
		specs = append(specs, probeSpec{Method: candidate.Method, Path: candidate.Path, StatusKey: candidate.Method + " " + candidate.Path, Checkin: true})
	}
	checkin := false
	for _, result := range s.fetchProbeBatch(ctx, baseURL, specs, 8) {
		statusByPath[result.Spec.StatusKey] = result.Status
		if result.Status > 0 {
			reachable = true
		}
		if result.Spec.Checkin && isPotentialCheckinEndpoint(result.Status, result.Body) {
			checkin = true
		}
		inspectSignals(result.Spec.Path, result.Status, result.Body, signals)
	}

	kind := "unknown"
	officialProvider := isOfficialProviderBaseURL(baseURL)
	panelSignals := countSignals(signals,
		"api-user-self", "api-user-token", "api-user-models", "api-user-quota",
		"api-user-available-models", "api-user-dashboard", "api-user-login", "api-auth-login",
		"api-login", "api-user-register", "api-user-checkin", "api-about", "api-home-page-content",
		"api-token", "api-channel", "api-log-token", "api-pricing", "api-option",
		"api-group", "api-redemption", "api-subscription", "newapi-login", "oneapi-login", "newapi-api",
		"panel-login", "panel-json", "json-version", "api-status",
	)
	sub2apiSignals := countSignals(signals,
		"sub2api-text", "sub2api-api", "sub2api-ui", "sub2api-gateway", "api-v1-panel",
		"sub2api-auth", "sub2api-settings", "sub2api-user", "sub2api-v1beta-models", "sub2api-openai-route",
	)
	switch {
	case officialProvider:
		kind = "official_provider"
	case sub2apiSignals >= 2 || signals["sub2api-text"] || signals["sub2api-api"]:
		kind = "sub2api"
	case signals["oneapi-text"] || signals["oneapi-login"]:
		kind = "oneapi"
	case looksLikeModifiedNewAPI(signals, panelSignals):
		kind = "modified_relay"
	case signals["newapi-api"] || signals["newapi-text"] || signals["newapi-login"] || (panelSignals >= 2 && signals["api-channel"]):
		kind = "newapi"
	case statusByPath["/v1/models"] > 0 && statusByPath["/v1/models"] != http.StatusNotFound:
		kind = "openai_compatible"
	}
	health := "unknown"
	if !reachable {
		health = "unreachable"
	} else if statusByPath["/api/user/self"] == http.StatusUnauthorized || statusByPath["/api/user/self"] == http.StatusForbidden {
		health = "auth_required"
	} else if len(signals) >= 2 || kind != "unknown" {
		health = "healthy"
	} else {
		health = "degraded"
	}

	if signals["checkin-disabled"] {
		checkin = false
	}
	if officialProvider || kind == "openai_compatible" || kind == "sub2api" || kind == "oneapi" {
		checkin = false
	}
	models := endpointLooksPresent(statusByPath["/v1/models"]) || endpointLooksPresent(statusByPath["/v1beta/models"]) || endpointLooksPresent(statusByPath["/api/channel/models"]) || endpointLooksPresent(statusByPath["/api/user/models"]) || endpointLooksPresent(statusByPath["/api/user/available_models"])
	pricing := endpointLooksPresent(statusByPath["/api/pricing"]) || signals["pricing-json"]
	balance := endpointLooksPresent(statusByPath["/v1/usage"]) || endpointLooksPresent(statusByPath["/api/user/self"]) || endpointLooksPresent(statusByPath["/api/user/quota"]) || endpointLooksPresent(statusByPath["/api/v1/subscriptions/active"]) || endpointLooksPresent(statusByPath["/api/v1/user/platform-quotas"])
	confidence := detectionConfidence(kind, signals, panelSignals, sub2apiSignals)

	return Detection{
		BaseURL: baseURL, HomepageURL: baseURL, LoginURL: baseURL + "/login",
		Kind: kind, HealthStatus: health, DetectionConfidence: confidence,
		SupportsCheckin: checkin, SupportsBalance: balance, SupportsModels: models, SupportsPricing: pricing,
		MatchedSignals: signalList(signals),
	}
}

// safeBaseURL validates raw against the host's outbound URL policy and
// returns the normalized base URL. When allowLocal is true the loopback
// addresses are permitted (used by ProbeLocal).
func (s *Service) safeBaseURL(ctx context.Context, raw string, allowLocal bool) (string, error) {
	baseURL := normalizeBaseURL(raw)
	var parsed *url.URL
	var err error
	if allowLocal {
		parsed, err = s.infra.ValidateLocalURL(ctx, baseURL)
	} else {
		parsed, err = s.infra.ValidateOutboundURL(ctx, baseURL)
	}
	if err != nil {
		return "", err
	}
	parsed.Path = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func (s *Service) fetchProbeBatch(ctx context.Context, baseURL string, specs []probeSpec, concurrency int) []probeFetchResult {
	if concurrency <= 0 {
		concurrency = 4
	}
	results := make([]probeFetchResult, len(specs))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for index, spec := range specs {
		wg.Add(1)
		go func(i int, item probeSpec) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			status, body := s.fetchTextWithMethod(ctx, item.Method, baseURL+item.Path)
			results[i] = probeFetchResult{Spec: item, Status: status, Body: body}
		}(index, spec)
	}
	wg.Wait()
	return results
}

func (s *Service) fetchTextWithMethod(ctx context.Context, method string, target string) (int, string) {
	req, err := http.NewRequestWithContext(ctx, method, target, nil)
	if err != nil {
		return 0, ""
	}
	req.Header.Set("user-agent", "RelayCheck-Desktop/0.1")
	resp, err := s.infra.DoHTTP(req)
	if err != nil {
		return 0, ""
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	return resp.StatusCode, string(body)
}

func inspectSignals(path string, status int, body string, signals map[string]bool) {
	text := strings.ToLower(body)
	if strings.Contains(text, "new api") || strings.Contains(text, "newapi") || strings.Contains(text, "new-api") || strings.Contains(text, "new_api") {
		signals["newapi-text"] = true
	}
	if strings.Contains(text, "one api") || strings.Contains(text, "oneapi") || strings.Contains(text, "one-api") || strings.Contains(text, "one_api") {
		signals["oneapi-text"] = true
	}
	if strings.Contains(text, "sub2api") || strings.Contains(text, "sub2 api") || strings.Contains(text, "sub-to-api") || strings.Contains(text, "sub_to_api") {
		signals["sub2api-text"] = true
	}
	if strings.Contains(body, "用户登录") ||
		(strings.Contains(body, "登录") && containsAny(body, "令牌", "额度", "余额", "渠道", "模型")) ||
		strings.Contains(text, "new-api") {
		signals["newapi-login"] = true
	}
	if strings.Contains(text, "one api") && (strings.Contains(text, "login") || strings.Contains(body, "登录")) {
		signals["oneapi-login"] = true
	}
	if containsAny(body, "渠道管理", "令牌管理", "用户管理", "模型倍率", "模型价格", "分组倍率") ||
		(strings.Contains(body, "充值") && containsAny(body, "额度", "余额")) ||
		containsAny(text, "token quota", "model ratio", "group ratio", "channel management", "token management", "user management") {
		signals["panel-login"] = true
	}
	if strings.Contains(text, "用户登录") ||
		(strings.Contains(text, "登录") && (strings.Contains(text, "令牌") || strings.Contains(text, "额度") || strings.Contains(text, "渠道"))) ||
		strings.Contains(text, "new-api") {
		signals["newapi-login"] = true
	}
	if strings.Contains(text, "one api") && (strings.Contains(text, "login") || strings.Contains(text, "登录")) {
		signals["oneapi-login"] = true
	}
	if strings.Contains(text, "api key") && (strings.Contains(text, "quota") || strings.Contains(text, "subscription") || strings.Contains(text, "billing")) {
		signals["sub2api-gateway"] = true
	}
	if strings.Contains(text, "subscription") && strings.Contains(text, "api gateway") {
		signals["sub2api-ui"] = true
	}
	if strings.Contains(text, "用户登录") ||
		(strings.Contains(text, "登录") && (strings.Contains(text, "令牌") || strings.Contains(text, "额度") || strings.Contains(text, "渠道"))) {
		signals["newapi-login"] = true
	}
	if strings.Contains(text, "one api") && strings.Contains(text, "登录") {
		signals["oneapi-login"] = true
	}
	if strings.Contains(text, "渠道管理") || strings.Contains(text, "令牌管理") || strings.Contains(text, "用户管理") ||
		strings.Contains(text, "模型倍率") || (strings.Contains(text, "充值") && strings.Contains(text, "额度")) {
		signals["panel-login"] = true
	}
	if strings.Contains(text, "渠道管理") || strings.Contains(text, "令牌管理") || strings.Contains(text, "用户管理") ||
		strings.Contains(text, "模型倍率") || strings.Contains(text, "充值") && strings.Contains(text, "额度") ||
		strings.Contains(text, "token quota") || strings.Contains(text, "model ratio") {
		signals["panel-login"] = true
	}
	if isCheckinDisabledText(body) {
		signals["checkin-disabled"] = true
	}
	if status == http.StatusOK || status == http.StatusUnauthorized || status == http.StatusForbidden {
		switch path {
		case "/api/about":
			signals["api-about"] = true
		case "/api/home_page_content":
			signals["api-home-page-content"] = true
		case "/api/user/self":
			signals["api-user-self"] = true
		case "/api/user/self/groups":
			signals["api-user-groups"] = true
		case "/api/user/token":
			signals["api-user-token"] = true
		case "/api/user/models":
			signals["api-user-models"] = true
		case "/api/user/available_models":
			signals["api-user-available-models"] = true
		case "/api/user/dashboard":
			signals["api-user-dashboard"] = true
		case "/api/user/quota":
			signals["api-user-quota"] = true
		case "/api/subscription/self":
			signals["api-subscription"] = true
		case "/api/channel/", "/api/channel/models":
			signals["api-channel"] = true
		case "/api/token/":
			signals["api-token"] = true
		case "/api/status":
			signals["api-status"] = true
		case "/v1/models":
			signals["openai-models"] = true
		case "/api/pricing":
			signals["api-pricing"] = true
		case "/api/option":
			signals["api-option"] = true
		case "/api/group":
			signals["api-group"] = true
		case "/api/redemption":
			signals["api-redemption"] = true
		case "/api/log/token":
			signals["api-log-token"] = true
		case "/api/v1/status", "/api/v1/user", "/api/v1/user/profile", "/api/v1/tokens", "/api/v1/accounts", "/api/v1/keys", "/api/v1/groups/available", "/api/v1/channels/available", "/api/v1/subscriptions/active", "/api/v1/user/platform-quotas":
			signals["api-v1-panel"] = true
			signals["sub2api-user"] = true
		case "/api/v1/settings/public":
			signals["api-v1-panel"] = true
			signals["sub2api-settings"] = true
		case "/api/v1/auth/me":
			signals["api-v1-panel"] = true
			signals["sub2api-auth"] = true
		case "/v1beta/models":
			signals["sub2api-v1beta-models"] = true
		}
	}
	if endpointLooksLikePanelAPI(path, status) {
		switch path {
		case "/api/user/login":
			signals["api-user-login"] = true
		case "/api/auth/login":
			signals["api-auth-login"] = true
		case "/api/login":
			signals["api-login"] = true
		case "/api/user/register":
			signals["api-user-register"] = true
		case "/api/user/checkin", "/api/checkin", "/api/user/check_in":
			signals["api-user-checkin"] = true
		case "/api/v1/auth/login":
			signals["api-v1-panel"] = true
			signals["sub2api-auth"] = true
		case "/chat/completions", "/responses":
			signals["sub2api-openai-route"] = true
		}
	}
	var js map[string]interface{}
	if json.Unmarshal([]byte(body), &js) == nil {
		if _, ok := js["version"]; ok {
			signals["json-version"] = true
		}
		value := strings.ToLower(fmt.Sprint(js))
		if path == "/api/about" && (strings.Contains(value, "new api") || strings.Contains(value, "new-api") || strings.Contains(value, "newapi")) {
			signals["newapi-api"] = true
		}
		if strings.Contains(value, "one api") || strings.Contains(value, "one-api") || strings.Contains(value, "oneapi") {
			signals["oneapi-text"] = true
		}
		if strings.Contains(value, "sub2api") || strings.Contains(value, "sub2 api") {
			signals["sub2api-api"] = true
		}
		if isCheckinPath(path) && hasAnyNestedJSONKey(js, "checked_in_today", "quota_awarded", "checkin_date", "min_quota", "max_quota", "total_checkins") {
			signals["api-user-checkin"] = true
			signals["checkin-json"] = true
		}
		if hasAnyJSONKey(js, "success", "message", "data") && hasAnyNestedJSONKey(js, "quota", "used_quota", "request_count", "token_name", "channel_count", "group_ratio", "model_ratio") {
			signals["panel-json"] = true
		}
		if hasAnyNestedJSONKey(js, "price", "pricing", "model_ratio", "group_ratio") {
			signals["pricing-json"] = true
		}
		if hasAnyNestedJSONKey(js, "subscription", "upstream", "provider", "account_id") && hasAnyNestedJSONKey(js, "api_key", "apikey", "quota", "rate_limit") {
			signals["sub2api-api"] = true
		}
	}
}

func endpointLooksLikePanelAPI(path string, status int) bool {
	if status == 0 || status == http.StatusNotFound {
		return false
	}
	switch path {
	case "/api/v1/auth/login", "/chat/completions", "/responses":
		return status == http.StatusOK ||
			status == http.StatusBadRequest ||
			status == http.StatusUnauthorized ||
			status == http.StatusForbidden ||
			status == http.StatusMethodNotAllowed ||
			status == http.StatusUnsupportedMediaType
	}
	switch path {
	case "/api/user/login", "/api/auth/login", "/api/login", "/api/user/register", "/api/user/checkin", "/api/checkin", "/api/user/check_in":
		return status == http.StatusOK ||
			status == http.StatusBadRequest ||
			status == http.StatusUnauthorized ||
			status == http.StatusForbidden ||
			status == http.StatusMethodNotAllowed
	default:
		return false
	}
}

func endpointLooksPresent(status int) bool {
	return status > 0 && status != http.StatusNotFound && status != http.StatusMethodNotAllowed
}

func detectionConfidence(kind string, signals map[string]bool, panelSignals int, sub2apiSignals int) float64 {
	if kind == "unknown" {
		return float64(min(4, len(signals))) / 10
	}
	if kind == "official_provider" {
		return 0.96
	}
	if kind == "openai_compatible" {
		if len(signals) <= 1 {
			return 0.55
		}
		return 0.7
	}
	score := 0.36
	score += float64(min(5, panelSignals)) * 0.09
	score += float64(min(3, sub2apiSignals)) * 0.08
	if signals["newapi-text"] || signals["oneapi-text"] || signals["sub2api-text"] {
		score += 0.16
	}
	if signals["api-channel"] {
		score += 0.08
	}
	if signals["api-user-self"] {
		score += 0.06
	}
	if score > 0.98 {
		return 0.98
	}
	return score
}

func hasAnyJSONKey(payload map[string]interface{}, keys ...string) bool {
	for _, wanted := range keys {
		for key := range payload {
			if strings.EqualFold(key, wanted) {
				return true
			}
		}
	}
	return false
}

func hasAnyNestedJSONKey(value interface{}, keys ...string) bool {
	switch typed := value.(type) {
	case map[string]interface{}:
		for _, wanted := range keys {
			for key, child := range typed {
				if strings.EqualFold(key, wanted) {
					return true
				}
				if hasAnyNestedJSONKey(child, wanted) {
					return true
				}
			}
		}
	case []interface{}:
		for _, child := range typed {
			if hasAnyNestedJSONKey(child, keys...) {
				return true
			}
		}
	}
	return false
}

func isCheckinPath(path string) bool {
	return strings.Contains(path, "checkin") ||
		strings.Contains(path, "check_in") ||
		strings.Contains(path, "signin") ||
		strings.Contains(path, "sign_in") ||
		strings.Contains(path, "sign-in")
}

func isPotentialCheckinEndpoint(status int, body string) bool {
	if status == 0 || status == http.StatusNotFound || status == http.StatusMethodNotAllowed {
		return false
	}
	if isCheckinDisabledText(body) {
		return false
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return true
	}
	var js map[string]interface{}
	if json.Unmarshal([]byte(body), &js) == nil {
		if hasAnyNestedJSONKey(js, "checked_in_today", "quota_awarded", "checkin_date", "min_quota", "max_quota", "total_checkins") {
			return status >= 200 && status < 500
		}
	}
	text := strings.ToLower(body)
	if strings.Contains(text, "<html") &&
		!strings.Contains(text, "checkin") &&
		!strings.Contains(text, "sign") &&
		!strings.Contains(body, "签到") {
		return false
	}
	return status >= 200 && status < 500
}

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

func looksLikeModifiedNewAPI(signals map[string]bool, panelSignals int) bool {
	if panelSignals < 2 {
		return false
	}
	hasLoginAPI := signals["api-user-login"] || signals["api-auth-login"] || signals["api-login"]
	hasNewAPIStyleAPI := signals["api-user-self"] || signals["api-user-token"] || signals["api-user-models"] || signals["api-token"] || signals["panel-json"]
	return hasLoginAPI && hasNewAPIStyleAPI
}

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

func countSignals(signals map[string]bool, names ...string) int {
	count := 0
	for _, name := range names {
		if signals[name] {
			count++
		}
	}
	return count
}

func containsAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}

// normalizeBaseURL is duplicated from core so the sites package can build
// fallback base URLs without importing core. Kept in sync with the host.
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

// HostLabel extracts the host portion of raw for display. Exported so the
// host handler can reuse it without re-implementing.
func HostLabel(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	return parsed.Host
}

func signalList(signals map[string]bool) []string {
	items := make([]string, 0, len(signals))
	for key := range signals {
		items = append(items, key)
	}
	return items
}

var excludedRelaySiteTokens = []string{
	"9router",
	"freemodel",
	"free model",
	"tokenrouter",
	"token router",
}

// IsManagedRelayKind reports whether kind is one of the relay panel kinds
// the host treats as manageable. Duplicated from core.filters so the sites
// package is self-contained.
func IsManagedRelayKind(kind string) bool {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "newapi", "oneapi", "sub2api", "modified_relay":
		return true
	default:
		return false
	}
}

// IsExcludedRelaySite reports whether a site name/baseURL pair is on the
// exclusion list (e.g. 9router). Duplicated from core.filters.
func IsExcludedRelaySite(name string, baseURL string) bool {
	combined := strings.ToLower(strings.TrimSpace(name) + " " + strings.TrimSpace(baseURL))
	for _, token := range excludedRelaySiteTokens {
		if strings.Contains(combined, token) {
			return true
		}
	}
	return false
}
