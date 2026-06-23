package core

import (
	"net/http"
	"net/url"
	"testing"
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
