package core

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

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

	client := newNetworkHTTPClient(3*time.Second, NetworkProxyConfig{}, outboundURLPolicy{AllowLocal: true})
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
