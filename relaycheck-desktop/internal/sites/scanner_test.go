package sites

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// testInfra is a minimal Infra implementation for detection tests: it routes
// HTTP through an httptest server client and permissively validates URLs so
// loopback test servers are reachable. DB/lifecycle methods are no-ops since
// DetectUpstream only needs HTTP + URL validation.
type testInfra struct {
	client *http.Client
}

func (t *testInfra) DB() *sql.DB { return nil }

func (t *testInfra) DoHTTP(req *http.Request) (*http.Response, error) {
	return t.client.Do(req)
}

func (t *testInfra) ValidateOutboundURL(ctx context.Context, raw string) (*url.URL, error) {
	return url.Parse(raw)
}

func (t *testInfra) ValidateLocalURL(ctx context.Context, raw string) (*url.URL, error) {
	return url.Parse(raw)
}

func (t *testInfra) AllowLocalOutbound() bool { return true }

func (t *testInfra) Notify(kind, level, title, content, relatedType, relatedID string) {}

func (t *testInfra) Audit(action, level, userID, entityType, entityID, detail string, metadata map[string]interface{}) {
}

func (t *testInfra) Now() string { return "now" }

func (t *testInfra) NewID() string { return "id" }

func newTestService(server *httptest.Server) *Service {
	return NewService(&testInfra{client: server.Client()})
}

func TestDetectUpstreamRecognizesNewAPIPanelSignals(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			_, _ = w.Write([]byte(`<html><title>New API</title><body>用户登录 令牌 渠道 额度</body></html>`))
		case "/api/user/self", "/api/channel/", "/api/token/", "/api/status":
			http.Error(w, `{"message":"unauthorized"}`, http.StatusUnauthorized)
		case "/v1/models":
			_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"gpt-4o-mini"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	svc := newTestService(server)
	detection := svc.DetectUpstream(context.Background(), server.URL)

	if detection.Kind != "newapi" {
		t.Fatalf("expected newapi, got %s with signals %v", detection.Kind, detection.MatchedSignals)
	}
	if detection.HealthStatus != "auth_required" {
		t.Fatalf("expected auth_required, got %s", detection.HealthStatus)
	}
	if detection.DetectionConfidence < 0.7 {
		t.Fatalf("expected high confidence, got %.2f", detection.DetectionConfidence)
	}
}

func TestDetectUpstreamRecognizesChineseNewAPIPanelSignals(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			_, _ = w.Write([]byte(`<html><title>中转后台</title><body>用户登录 令牌管理 渠道管理 额度 模型倍率</body></html>`))
		case "/api/user/self", "/api/channel/", "/api/token/", "/api/status":
			http.Error(w, `{"message":"unauthorized"}`, http.StatusUnauthorized)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	svc := newTestService(server)
	detection := svc.DetectUpstream(context.Background(), server.URL)

	if detection.Kind != "newapi" && detection.Kind != "modified_relay" {
		t.Fatalf("expected managed NewAPI-style panel, got %s with signals %v", detection.Kind, detection.MatchedSignals)
	}
	if !containsString(detection.MatchedSignals, "newapi-login") {
		t.Fatalf("expected newapi-login signal from Chinese login page, got %v", detection.MatchedSignals)
	}
	if !containsString(detection.MatchedSignals, "panel-login") {
		t.Fatalf("expected panel-login signal from Chinese admin text, got %v", detection.MatchedSignals)
	}
}

func TestDetectUpstreamRecognizesNewAPICheckinStatusJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/about":
			_, _ = w.Write([]byte(`{"success":true,"data":{"system_name":"New API","version":"0.9.0"}}`))
		case "/api/user/checkin":
			_, _ = w.Write([]byte(`{"success":true,"data":{"enabled":true,"min_quota":10,"max_quota":20,"stats":{"checked_in_today":false}}}`))
		case "/api/user/self":
			http.Error(w, `{"success":false,"message":"未登录"}`, http.StatusUnauthorized)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	svc := newTestService(server)
	detection := svc.DetectUpstream(context.Background(), server.URL)

	if detection.Kind != "newapi" {
		t.Fatalf("expected newapi, got %s with signals %v", detection.Kind, detection.MatchedSignals)
	}
	if !detection.SupportsCheckin {
		t.Fatalf("expected check-in support, got signals %v", detection.MatchedSignals)
	}
}

func TestDetectUpstreamDoesNotSupportDisabledCheckin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/about":
			_, _ = w.Write([]byte(`{"success":true,"data":{"system_name":"New API","version":"0.9.0"}}`))
		case "/api/user/checkin":
			_, _ = w.Write([]byte(`{"success":false,"message":"签到功能未启用"}`))
		case "/api/user/self":
			http.Error(w, `{"success":false,"message":"未登录"}`, http.StatusUnauthorized)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	svc := newTestService(server)
	detection := svc.DetectUpstream(context.Background(), server.URL)

	if detection.Kind != "newapi" {
		t.Fatalf("expected newapi, got %s with signals %v", detection.Kind, detection.MatchedSignals)
	}
	if detection.SupportsCheckin {
		t.Fatalf("expected disabled check-in to be false, got signals %v", detection.MatchedSignals)
	}
	if !containsString(detection.MatchedSignals, "checkin-disabled") {
		t.Fatalf("expected checkin-disabled signal, got %v", detection.MatchedSignals)
	}
}

func TestDetectUpstreamRecognizesSub2APISignals(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(`<html><body>Sub2API subscription API Gateway quota dashboard</body></html>`))
		case "/api/v1/status":
			_, _ = w.Write([]byte(`{"data":{"api_key":"sk-test","quota":100,"subscription":"active"}}`))
		case "/v1/models":
			_, _ = w.Write([]byte(`{"data":[{"id":"deepseek-chat"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	svc := newTestService(server)
	detection := svc.DetectUpstream(context.Background(), server.URL)

	if detection.Kind != "sub2api" {
		t.Fatalf("expected sub2api, got %s with signals %v", detection.Kind, detection.MatchedSignals)
	}
	if !detection.SupportsModels {
		t.Fatal("expected models support")
	}
}

func TestDetectUpstreamRecognizesSub2APIGatewayRoutesWithoutBrandText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/login":
			http.Error(w, `{"message":"missing credentials"}`, http.StatusBadRequest)
		case "/api/v1/settings/public":
			_, _ = w.Write([]byte(`{"data":{"site_name":"Relay Gateway","payment_enabled":false}}`))
		case "/api/v1/user/profile":
			http.Error(w, `{"message":"unauthorized"}`, http.StatusUnauthorized)
		case "/v1/models", "/v1beta/models":
			http.Error(w, `{"error":{"message":"missing api key"}}`, http.StatusUnauthorized)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	svc := newTestService(server)
	detection := svc.DetectUpstream(context.Background(), server.URL)

	if detection.Kind != "sub2api" {
		t.Fatalf("expected sub2api, got %s with signals %v", detection.Kind, detection.MatchedSignals)
	}
	if detection.SupportsCheckin {
		t.Fatalf("sub2api gateway routes should not imply check-in support: %v", detection.MatchedSignals)
	}
	if !detection.SupportsModels {
		t.Fatalf("expected model gateway support, got %#v", detection)
	}
}

func TestDetectUpstreamRecognizesModifiedNewAPIByLoginAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/user/login":
			http.Error(w, `{"success":false,"message":"missing username or password"}`, http.StatusBadRequest)
		case "/api/user/self":
			http.Error(w, `{"success":false,"message":"unauthorized"}`, http.StatusUnauthorized)
		case "/v1/models":
			_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"claude-opus-4-6"},{"id":"gpt-5.5"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	svc := newTestService(server)
	detection := svc.DetectUpstream(context.Background(), server.URL)

	if detection.Kind != "modified_relay" {
		t.Fatalf("expected modified_relay, got %s with signals %v", detection.Kind, detection.MatchedSignals)
	}
	if detection.HealthStatus != "auth_required" {
		t.Fatalf("expected auth_required, got %s", detection.HealthStatus)
	}
	if !containsString(detection.MatchedSignals, "api-user-login") {
		t.Fatalf("expected api-user-login signal, got %v", detection.MatchedSignals)
	}
}

func TestDetectUpstreamDiscoversHomepageLoginLink(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(`<html><body><a href="/console/login">登录</a></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	svc := newTestService(server)
	detection := svc.DetectUpstream(context.Background(), server.URL)

	want := server.URL + "/console/login"
	if detection.LoginURL != want {
		t.Fatalf("expected login URL %q, got %q", want, detection.LoginURL)
	}
	assertLoginDiscovery(t, detection.LoginDiscovery, want, "html_link", 0.85)
}

func TestDetectUpstreamDiscoversLoginLinkByText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(`<html><body><a href="/console">登录</a></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	svc := newTestService(server)
	detection := svc.DetectUpstream(context.Background(), server.URL)

	want := server.URL + "/console"
	if detection.LoginURL != want {
		t.Fatalf("expected login URL %q, got %q", want, detection.LoginURL)
	}
	assertLoginDiscovery(t, detection.LoginDiscovery, want, "html_link", 0.85)
}

func TestDetectUpstreamPreservesSameOriginLoginFragment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(`<html><body><a href="/#/login">Login</a></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	svc := newTestService(server)
	detection := svc.DetectUpstream(context.Background(), server.URL)

	want := server.URL + "/#/login"
	if detection.LoginURL != want {
		t.Fatalf("expected login URL %q, got %q", want, detection.LoginURL)
	}
	assertLoginDiscovery(t, detection.LoginDiscovery, want, "html_link", 0.85)
}

func TestDetectUpstreamDiscoversPasswordFormOnCandidatePage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/console/login":
			_, _ = w.Write([]byte(`<html><body><form action="/api/user/login"><input name="password" type="password"></form></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	svc := newTestService(server)
	detection := svc.DetectUpstream(context.Background(), server.URL)

	want := server.URL + "/console/login"
	if detection.LoginURL != want {
		t.Fatalf("expected login URL %q, got %q", want, detection.LoginURL)
	}
	assertLoginDiscovery(t, detection.LoginDiscovery, want, "html_form", 0.95)
}

func TestDetectUpstreamDiscoversFixedLoginCandidate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/panel/login":
			_, _ = w.Write([]byte(`<html><title>Sign in</title><body>用户登录 令牌 渠道 额度</body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	svc := newTestService(server)
	detection := svc.DetectUpstream(context.Background(), server.URL)

	want := server.URL + "/panel/login"
	if detection.LoginURL != want {
		t.Fatalf("expected login URL %q, got %q", want, detection.LoginURL)
	}
	assertLoginDiscovery(t, detection.LoginDiscovery, want, "path_probe", 0.75)
}

func TestDetectUpstreamUsesSPAFallbackForLoginShell(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/", "/login":
			_, _ = w.Write([]byte(`<html><body><div id="root"></div><script src="/assets/app.js"></script><script>window.route="login-panel"</script></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	svc := newTestService(server)
	detection := svc.DetectUpstream(context.Background(), server.URL)

	want := server.URL + "/login"
	if detection.LoginURL != want {
		t.Fatalf("expected login URL %q, got %q", want, detection.LoginURL)
	}
	assertLoginDiscovery(t, detection.LoginDiscovery, want, "spa_fallback", 0.60)
}

func TestDetectUpstreamFallsBackToLoginPathWhenNoCandidateMatches(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	svc := newTestService(server)
	detection := svc.DetectUpstream(context.Background(), server.URL)

	want := server.URL + "/login"
	if detection.LoginURL != want {
		t.Fatalf("expected fallback login URL %q, got %q", want, detection.LoginURL)
	}
	assertLoginDiscovery(t, detection.LoginDiscovery, want, "fallback", 0.40)
}

func TestDetectUpstreamKeepsLoginCandidatesSameOriginAndDeduplicated(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(`<html><body>
				<a href="/login">Login</a>
				<a href="/login">登录</a>
				<a href="https://evil.example/login">Login</a>
			</body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	svc := newTestService(server)
	detection := svc.DetectUpstream(context.Background(), server.URL)

	want := server.URL + "/login"
	assertLoginDiscovery(t, detection.LoginDiscovery, want, "html_link", 0.85)
	if got := countString(detection.LoginDiscovery.Candidates, want); got != 1 {
		t.Fatalf("expected %q once in candidates, got %d in %v", want, got, detection.LoginDiscovery.Candidates)
	}
	if containsString(detection.LoginDiscovery.Candidates, "https://evil.example/login") {
		t.Fatalf("expected cross-origin candidate to be ignored, got %v", detection.LoginDiscovery.Candidates)
	}
}

func TestHostLabel(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"https://relay.example.com/path?x=1", "relay.example.com"},
		{"https://relay.example.com:8443/path", "relay.example.com:8443"},
		{"://bad-url", "://bad-url"},
	}

	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			if got := HostLabel(tc.raw); got != tc.want {
				t.Fatalf("HostLabel(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestIsManagedRelayKind(t *testing.T) {
	cases := []struct {
		kind string
		want bool
	}{
		{"newapi", true},
		{" OneAPI ", true},
		{"sub2api", true},
		{"modified_relay", true},
		{"official_provider", false},
		{"openai_compatible", false},
		{"", false},
	}

	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			if got := IsManagedRelayKind(tc.kind); got != tc.want {
				t.Fatalf("IsManagedRelayKind(%q) = %v, want %v", tc.kind, got, tc.want)
			}
		})
	}
}

func TestIsExcludedRelaySite(t *testing.T) {
	cases := []struct {
		name    string
		baseURL string
		want    bool
	}{
		{"9Router mirror", "https://example.com", true},
		{"Relay", "https://freemodel.example.com", true},
		{"Token Router", "https://example.com", true},
		{"Normal Relay", "https://relay.example.com", false},
		{"", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name+"/"+tc.baseURL, func(t *testing.T) {
			if got := IsExcludedRelaySite(tc.name, tc.baseURL); got != tc.want {
				t.Fatalf("IsExcludedRelaySite(%q, %q) = %v, want %v", tc.name, tc.baseURL, got, tc.want)
			}
		})
	}
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func countString(values []string, wanted string) int {
	count := 0
	for _, value := range values {
		if value == wanted {
			count++
		}
	}
	return count
}

func assertLoginDiscovery(t *testing.T, discovery *LoginDiscovery, url string, source string, confidence float64) {
	t.Helper()
	if discovery == nil {
		t.Fatal("expected login discovery to be present")
	}
	if discovery.URL != url {
		t.Fatalf("expected discovery URL %q, got %q", url, discovery.URL)
	}
	if discovery.Source != source {
		t.Fatalf("expected discovery source %q, got %q", source, discovery.Source)
	}
	if discovery.Confidence != confidence {
		t.Fatalf("expected discovery confidence %.2f, got %.2f", confidence, discovery.Confidence)
	}
	if !containsString(discovery.Candidates, url) {
		t.Fatalf("expected candidates to include %q, got %v", url, discovery.Candidates)
	}
}
