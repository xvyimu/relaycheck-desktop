package core

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

type outboundURLPolicy struct {
	AllowLocal bool
}

func validateOutboundHTTPURL(ctx context.Context, raw string, policy outboundURLPolicy) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, errors.New("URL 必须包含 http/https 协议和主机名")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, errors.New("URL 只支持 http 或 https 协议")
	}
	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return nil, errors.New("URL 缺少主机名")
	}
	if policy.AllowLocal && isLoopbackHost(host) {
		return parsed, nil
	}
	if isBlockedHostname(host) {
		return nil, fmt.Errorf("URL 主机 %s 属于本地或保留地址，已拒绝", host)
	}
	ip := net.ParseIP(host)
	if ip != nil {
		if isBlockedOutboundIP(ip, policy.AllowLocal) {
			return nil, fmt.Errorf("URL 主机 %s 属于本地或内网地址，已拒绝", host)
		}
		return parsed, nil
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("无法解析 URL 主机 %s：%w", host, err)
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("无法解析 URL 主机 %s", host)
	}
	for _, address := range addresses {
		if isBlockedOutboundIP(address.IP, policy.AllowLocal) {
			return nil, fmt.Errorf("URL 主机 %s 解析到本地或内网地址 %s，已拒绝", host, address.IP.String())
		}
	}
	return parsed, nil
}

func safeNormalizeBaseURL(ctx context.Context, raw string, policy outboundURLPolicy) (string, error) {
	baseURL := normalizeBaseURL(raw)
	parsed, err := validateOutboundHTTPURL(ctx, baseURL, policy)
	if err != nil {
		return "", err
	}
	parsed.Path = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func (a *App) externalURLPolicy() outboundURLPolicy {
	if a == nil {
		return outboundURLPolicy{}
	}
	return outboundURLPolicy{AllowLocal: a.allowLocalOutbound}
}

// ValidateOutboundURL is the exported adapter for the notifications package's
// NotificationHTTPPort interface. It applies the app's outbound URL policy
// (SSRF defences, controlled by allowLocalOutbound) via validateOutboundHTTPURL.
func (a *App) ValidateOutboundURL(ctx context.Context, raw string) (*url.URL, error) {
	return validateOutboundHTTPURL(ctx, raw, a.externalURLPolicy())
}

// ValidateOutboundURLStrict is the exported adapter for the versioncheck
// package's Infra interface. It applies a strict outbound URL policy that
// always rejects local/loopback/internal addresses, regardless of the app's
// allowLocalOutbound setting, since version manifests must come from a remote
// host.
func (a *App) ValidateOutboundURLStrict(ctx context.Context, raw string) (*url.URL, error) {
	return validateOutboundHTTPURL(ctx, raw, outboundURLPolicy{AllowLocal: false})
}

func isBlockedHostname(host string) bool {
	normalized := strings.Trim(strings.ToLower(host), "[]")
	return normalized == "localhost" ||
		strings.HasSuffix(normalized, ".localhost") ||
		normalized == "metadata.google.internal"
}

func isLoopbackHost(host string) bool {
	normalized := strings.Trim(strings.ToLower(host), "[]")
	if normalized == "localhost" || strings.HasSuffix(normalized, ".localhost") {
		return true
	}
	ip := net.ParseIP(normalized)
	return ip != nil && ip.IsLoopback()
}

func isBlockedOutboundIP(ip net.IP, allowLocal bool) bool {
	if ip == nil {
		return true
	}
	if allowLocal && ip.IsLoopback() {
		return false
	}
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified() ||
		isMetadataIP(ip)
}

func isMetadataIP(ip net.IP) bool {
	if ip.Equal(net.ParseIP("169.254.169.254")) {
		return true
	}
	return false
}
