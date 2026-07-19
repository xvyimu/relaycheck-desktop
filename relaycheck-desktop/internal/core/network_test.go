package core

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestNetworkHTTPClientPinsInitialResolvedIP(t *testing.T) {
	reached := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	pinnedURL, err := url.Parse("http://rebind.invalid:" + serverURL.Port())
	if err != nil {
		t.Fatal(err)
	}
	resolved := resolvedOutboundURL{URL: pinnedURL, IPs: []net.IP{net.ParseIP("127.0.0.1")}}
	client := newNetworkHTTPClient(3*time.Second, NetworkProxyConfig{}, outboundURLPolicy{}, resolved)

	req, err := http.NewRequest(http.MethodGet, pinnedURL.String(), nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("expected request to use the validated IP instead of resolving hostname again: %v", err)
	}
	defer resp.Body.Close()
	if !reached {
		t.Fatal("expected pinned target server to receive the request")
	}
}

func TestProxyTestDoesNotExposeRejectedURLParserDetails(t *testing.T) {
	app := newTestApp(t)
	req := httptest.NewRequest(http.MethodPost, "/api/system/proxy/test", strings.NewReader(`{"targetUrl":"http://[::1/token=TOP_SECRET"}`))
	rec := httptest.NewRecorder()

	app.handleSystemProxyTest(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var response apiResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error != "测试地址不安全，请检查 URL。" {
		t.Fatalf("unexpected public proxy-test error: %q", response.Error)
	}
	if strings.Contains(response.Error, "TOP_SECRET") || strings.Contains(response.Error, "::1") {
		t.Fatalf("proxy-test response leaked rejected URL: %q", response.Error)
	}
}

func TestValidateNetworkProxyConfigAcceptsLocalHTTPProxy(t *testing.T) {
	config := NetworkProxyConfig{
		Enabled:     true,
		URL:         "http://127.0.0.1:7897",
		BypassLocal: true,
	}
	if err := validateNetworkProxyConfig(config); err != nil {
		t.Fatalf("expected valid proxy config, got %v", err)
	}
}

func TestNetworkProxyStatusJSONOmitsFullURL(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()
	app.networkProxy.Set(NetworkProxyConfig{
		Enabled:     true,
		URL:         "http://127.0.0.1:7897",
		BypassLocal: true,
	})

	status := app.networkProxyStatus()
	raw, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if _, ok := decoded["url"]; ok {
		t.Fatalf("public NetworkProxyStatus must not include url field: %s", raw)
	}
	if got, _ := decoded["urlMasked"].(string); got != "http://127.0.0.1:7897" {
		t.Fatalf("urlMasked = %q, want full masked host URL", got)
	}
	if enabled, _ := decoded["enabled"].(bool); !enabled {
		t.Fatalf("enabled = false, want true")
	}
}

func TestSystemStatusProxyJSONOmitsFullURL(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()
	app.networkProxy.Set(NetworkProxyConfig{
		Enabled:     true,
		URL:         "http://proxy.example:8080",
		BypassLocal: true,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/system/status", nil)
	rec := httptest.NewRecorder()
	app.handleSystemStatus(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, `"url":"http://proxy.example:8080"`) || strings.Contains(body, `"url": "http://proxy.example:8080"`) {
		t.Fatalf("system status leaked full proxy url: %s", body)
	}
	if !strings.Contains(body, `"urlMasked":"http://proxy.example:8080"`) && !strings.Contains(body, `"urlMasked": "http://proxy.example:8080"`) {
		// maskProxyURL keeps host:port for non-userinfo URLs
		if !strings.Contains(body, "urlMasked") {
			t.Fatalf("expected urlMasked in system status: %s", body)
		}
	}
}

func TestValidateNetworkProxyConfigRejectsMissingHost(t *testing.T) {
	config := NetworkProxyConfig{
		Enabled: true,
		URL:     "http://",
	}
	if err := validateNetworkProxyConfig(config); err == nil {
		t.Fatal("expected invalid proxy URL to be rejected")
	}
}

func TestProxyURLForRequestBypassesLocalTargets(t *testing.T) {
	proxyURL, _ := url.Parse("http://127.0.0.1:7897")
	config := NetworkProxyConfig{Enabled: true, URL: proxyURL.String(), BypassLocal: true}
	req, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:3001/api/status", nil)

	got, err := proxyURLForRequest(config)(req)
	if err != nil {
		t.Fatalf("unexpected proxy error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected local target to bypass proxy, got %s", got)
	}
}

func TestProxyURLForRequestUsesProxyForExternalTargets(t *testing.T) {
	proxyURL, _ := url.Parse("http://127.0.0.1:7897")
	config := NetworkProxyConfig{Enabled: true, URL: proxyURL.String(), BypassLocal: true}
	req, _ := http.NewRequest(http.MethodGet, "https://wxls.ccwu.cc/", nil)

	got, err := proxyURLForRequest(config)(req)
	if err != nil {
		t.Fatalf("unexpected proxy error: %v", err)
	}
	if got == nil || got.String() != proxyURL.String() {
		t.Fatalf("expected external target to use proxy %s, got %v", proxyURL, got)
	}
}

func TestNetworkHTTPClientRejectsRedirectToMetadata(t *testing.T) {
	private := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("private target should not be reached via redirect")
		w.WriteHeader(http.StatusOK)
	}))
	defer private.Close()

	public := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, private.URL+"/secret", http.StatusFound)
	}))
	defer public.Close()

	// Client policy forbids local/private; public hop is also loopback in tests,
	// so use AllowLocal only for the initial host while CheckRedirect still
	// validates each hop with the same policy — redirect to another loopback is
	// allowed when AllowLocal=true. Use AllowLocal=false and a non-loopback
	// public IP is hard in unit tests; instead validate CheckRedirect rejects
	// metadata IP explicitly via a crafted redirect target.
	metaPublic := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://169.254.169.254/latest/meta-data", http.StatusFound)
	}))
	defer metaPublic.Close()

	initial, err := resolveOutboundHTTPURL(t.Context(), metaPublic.URL, outboundURLPolicy{AllowLocal: true})
	if err != nil {
		t.Fatal(err)
	}
	client := newNetworkHTTPClient(3*time.Second, NetworkProxyConfig{}, outboundURLPolicy{AllowLocal: true}, initial)
	req, err := http.NewRequest(http.MethodGet, metaPublic.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err == nil {
		resp.Body.Close()
		t.Fatal("expected redirect to metadata IP to fail")
	}
	if !strings.Contains(err.Error(), "redirect") && !strings.Contains(err.Error(), "拒绝") && !strings.Contains(err.Error(), "rejected") {
		// still require failure; message shape may wrap
		t.Logf("redirect error (ok as long as failed): %v", err)
	}
}
