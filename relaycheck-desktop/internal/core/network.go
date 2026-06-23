package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultHTTPTimeout = 3500 * time.Millisecond

type NetworkProxyConfig struct {
	Enabled     bool   `json:"enabled"`
	URL         string `json:"url"`
	BypassLocal bool   `json:"bypassLocal"`
}

type NetworkProxyStatus struct {
	Enabled     bool   `json:"enabled"`
	URL         string `json:"url"`
	URLMasked   string `json:"urlMasked"`
	BypassLocal bool   `json:"bypassLocal"`
}

func defaultNetworkProxyConfig() NetworkProxyConfig {
	return NetworkProxyConfig{
		Enabled:     false,
		URL:         "http://127.0.0.1:7897",
		BypassLocal: true,
	}
}

func parseNetworkProxyConfig(valueJSON string) (NetworkProxyConfig, error) {
	config := defaultNetworkProxyConfig()
	if strings.TrimSpace(valueJSON) == "" {
		return config, nil
	}
	if err := json.Unmarshal([]byte(valueJSON), &config); err != nil {
		return config, err
	}
	config.URL = strings.TrimSpace(config.URL)
	if config.URL == "" {
		config.URL = defaultNetworkProxyConfig().URL
	}
	return config, validateNetworkProxyConfig(config)
}

func validateNetworkProxyConfig(config NetworkProxyConfig) error {
	if !config.Enabled {
		return nil
	}
	parsed, err := url.Parse(strings.TrimSpace(config.URL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return errors.New("代理地址必须是完整 URL，例如 http://127.0.0.1:7897")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" && scheme != "socks5" {
		return errors.New("代理协议只支持 http、https 或 socks5")
	}
	if parsed.User != nil {
		return errors.New("代理地址暂不支持用户名密码，避免凭据出现在进程参数或诊断信息中")
	}
	if parsed.Hostname() == "" {
		return errors.New("代理地址缺少主机名")
	}
	return nil
}

func (a *App) reloadNetworkProxyConfig(ctx context.Context) error {
	if a.db == nil {
		return nil
	}
	config, err := a.loadNetworkProxyConfig(ctx)
	if err != nil {
		config = defaultNetworkProxyConfig()
		config.Enabled = false
	}
	a.mu.Lock()
	a.networkProxy = config
	a.mu.Unlock()
	return nil
}

func (a *App) loadNetworkProxyConfig(ctx context.Context) (NetworkProxyConfig, error) {
	if a.db == nil {
		return defaultNetworkProxyConfig(), nil
	}
	var valueJSON string
	err := a.db.QueryRowContext(ctx, `SELECT value_json FROM system_settings WHERE key='network.proxy'`).Scan(&valueJSON)
	if err == sql.ErrNoRows {
		return defaultNetworkProxyConfig(), nil
	}
	if err != nil {
		return defaultNetworkProxyConfig(), err
	}
	return parseNetworkProxyConfig(valueJSON)
}

func (a *App) currentNetworkProxyConfig() NetworkProxyConfig {
	if a == nil {
		return defaultNetworkProxyConfig()
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.networkProxy.URL == "" {
		return defaultNetworkProxyConfig()
	}
	return a.networkProxy
}

func (a *App) networkProxyStatus() NetworkProxyStatus {
	config := a.currentNetworkProxyConfig()
	return NetworkProxyStatus{
		Enabled:     config.Enabled,
		URL:         config.URL,
		URLMasked:   maskProxyURL(config.URL),
		BypassLocal: config.BypassLocal,
	}
}

func (a *App) doHTTP(req *http.Request) (*http.Response, error) {
	return a.doHTTPWithTimeout(req, defaultHTTPTimeout)
}

func (a *App) doHTTPWithTimeout(req *http.Request, timeout time.Duration) (*http.Response, error) {
	if timeout <= 0 {
		timeout = defaultHTTPTimeout
	}
	if a != nil && a.db == nil && a.client != nil {
		return a.client.Do(req)
	}
	client := newNetworkHTTPClient(timeout, a.currentNetworkProxyConfig())
	return client.Do(req)
}

func newNetworkHTTPClient(timeout time.Duration, config NetworkProxyConfig) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = proxyURLForRequest(config)
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}

func proxyURLForRequest(config NetworkProxyConfig) func(*http.Request) (*url.URL, error) {
	if !config.Enabled {
		return nil
	}
	proxy, err := url.Parse(strings.TrimSpace(config.URL))
	return func(req *http.Request) (*url.URL, error) {
		if err != nil {
			return nil, err
		}
		if config.BypassLocal && isLocalTarget(req.URL.Hostname()) {
			return nil, nil
		}
		return proxy, nil
	}
}

func isLocalTarget(host string) bool {
	host = strings.Trim(strings.ToLower(host), "[]")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func maskProxyURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" {
		return raw
	}
	parsed.User = nil
	return parsed.String()
}

func (a *App) chromeProxyArgs() []string {
	config := a.currentNetworkProxyConfig()
	if !config.Enabled || strings.TrimSpace(config.URL) == "" {
		return nil
	}
	if err := validateNetworkProxyConfig(config); err != nil {
		return nil
	}
	args := []string{"--proxy-server=" + config.URL}
	if config.BypassLocal {
		args = append(args, "--proxy-bypass-list=<-loopback>")
	}
	return args
}

func (a *App) handleSystemProxyTest(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var input struct {
		TargetURL string `json:"targetUrl"`
	}
	_ = decodeJSON(r, &input)
	targetURL := strings.TrimSpace(input.TargetURL)
	if targetURL == "" {
		targetURL = "https://www.gstatic.com/generate_204"
	}
	parsed, err := validateOutboundHTTPURL(r.Context(), targetURL, outboundURLPolicy{})
	if err != nil {
		writeError(w, http.StatusBadRequest, "测试地址不安全："+err.Error())
		return
	}
	targetURL = parsed.String()

	started := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Header.Set("user-agent", "RelayCheck-Desktop/0.1")
	resp, err := a.doHTTPWithTimeout(req, 8*time.Second)
	latencyMs := time.Since(started).Milliseconds()
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":        false,
			"message":   err.Error(),
			"latencyMs": latencyMs,
			"proxy":     a.networkProxyStatus(),
			"targetUrl": targetURL,
		})
		return
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":         resp.StatusCode > 0 && resp.StatusCode < 500,
		"httpStatus": resp.StatusCode,
		"latencyMs":  latencyMs,
		"proxy":      a.networkProxyStatus(),
		"targetUrl":  targetURL,
		"message":    fmt.Sprintf("HTTP %d，耗时 %dms。", resp.StatusCode, latencyMs),
	})
}
