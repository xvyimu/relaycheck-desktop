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
	"sync"
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
	a.networkProxy.Set(config)
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
	config := a.networkProxy.Get()
	if config.URL == "" {
		return defaultNetworkProxyConfig()
	}
	return config
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
	policy := outboundURLPolicy{}
	if a != nil {
		policy = a.externalURLPolicy()
	}
	var cfg NetworkProxyConfig
	if a != nil {
		cfg = a.currentNetworkProxyConfig()
	}
	// Preflight SSRF validation (hostname/IP policy).
	if req != nil && req.URL != nil && strings.TrimSpace(req.URL.Host) != "" {
		if _, err := resolveOutboundHTTPURL(req.Context(), req.URL.String(), policy); err != nil {
			return nil, err
		}
	}
	client := newNetworkHTTPClient(timeout, cfg, policy)
	return client.Do(req)
}

// DoHTTPWithTimeout is the exported adapter for the notifications package's
// NotificationHTTPPort interface. It delegates to doHTTPWithTimeout so the
// internal call sites are unchanged.
func (a *App) DoHTTPWithTimeout(req *http.Request, timeout time.Duration) (*http.Response, error) {
	return a.doHTTPWithTimeout(req, timeout)
}

func newNetworkHTTPClient(timeout time.Duration, config NetworkProxyConfig, policy outboundURLPolicy) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = proxyURLForRequest(config)
	var pinMu sync.Mutex
	pinned := map[string][]net.IP{}
	baseDialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	baseDial := transport.DialContext
	if baseDial == nil {
		baseDial = baseDialer.DialContext
	}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		pinMu.Lock()
		ips := append([]net.IP(nil), pinned[strings.ToLower(host)]...)
		pinMu.Unlock()
		if len(ips) > 0 {
			var lastErr error
			for _, ip := range ips {
				conn, err := baseDial(ctx, network, net.JoinHostPort(ip.String(), port))
				if err == nil {
					return conn, nil
				}
				lastErr = err
			}
			return nil, lastErr
		}
		return baseDial(ctx, network, address)
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		// Re-validate every redirect hop and pin non-local resolved IPs so a
		// public 302 cannot rebind to loopback/metadata after the first check.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("stopped after 5 redirects")
			}
			if req.URL == nil {
				return errors.New("redirect missing URL")
			}
			resolved, err := resolveOutboundHTTPURL(req.Context(), req.URL.String(), policy)
			if err != nil {
				return fmt.Errorf("redirect target rejected: %w", err)
			}
			host := strings.ToLower(strings.TrimSpace(resolved.URL.Hostname()))
			// Only pin when policy forbids local targets; allow-local loopback
			// hosts keep default dial behavior for NewAPI-on-localhost.
			if !policy.AllowLocal && len(resolved.IPs) > 0 {
				pinMu.Lock()
				pinned[host] = append([]net.IP(nil), resolved.IPs...)
				pinMu.Unlock()
			}
			return nil
		},
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
