package versioncheck

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Infra is the subset of the host application that the version-check domain
// depends on. The host (e.g. *core.App) satisfies this interface by providing
// database access (for the configured manifest URL), the shared HTTP client,
// the product version, and strict outbound-URL validation (no local targets).
//
// All methods are exported so that types defined in other packages (the host
// application) can satisfy the interface cross-package.
type Infra interface {
	// DB returns the application's SQLite database handle, used to read the
	// "app.version_check_url" setting.
	DB() *sql.DB
	// HTTPClient returns the shared HTTP client (with proxy and timeout
	// configured) used to fetch the remote version manifest.
	HTTPClient() *http.Client
	// ProductVersion returns the host product version string compared
	// against the remote manifest.
	ProductVersion() string
	// ValidateOutboundURLStrict validates an outbound HTTP(S) URL with a
	// strict policy that rejects local/loopback/addresses. Used to harden
	// the version-manifest fetch against SSRF.
	ValidateOutboundURLStrict(ctx context.Context, raw string) (*url.URL, error)
}

// Service implements the version-check domain. It owns the manifest fetch,
// version comparison, and result assembly logic, relying on Infra for
// database access, HTTP transport, and URL validation. The host application
// delegates its *App handler methods to this Service.
type Service struct {
	infra Infra
}

// NewService constructs a version-check Service backed by the given Infra.
func NewService(infra Infra) *Service {
	return &Service{infra: infra}
}

// VersionCheckResult is the response payload for /api/system/version-check.
type VersionCheckResult struct {
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion,omitempty"`
	UpdateAvailable bool   `json:"updateAvailable"`
	ReleaseURL      string `json:"releaseUrl,omitempty"`
	ReleaseNotes    string `json:"releaseNotes,omitempty"`
	CheckedAt       string `json:"checkedAt"`
	Error           string `json:"error,omitempty"`
}

// versionManifest is the expected JSON shape from the remote manifest URL.
type versionManifest struct {
	Version      string `json:"version"`
	ReleaseURL   string `json:"releaseUrl"`
	ReleaseNotes string `json:"releaseNotes"`
}

// CheckVersion performs a version check against the configured manifest URL
// and returns the assembled result. Soft failures (missing setting, bad URL,
// transport errors, parse errors) are recorded in VersionCheckResult.Error
// and a non-nil result is always returned, mirroring the original handler
// behaviour where every path responds HTTP 200.
func (s *Service) CheckVersion(ctx context.Context) *VersionCheckResult {
	current := s.infra.ProductVersion()
	result := &VersionCheckResult{
		CurrentVersion: current,
		CheckedAt:      time.Now().UTC().Format(time.RFC3339Nano),
	}

	manifestURL := s.getSettingString(ctx, "app.version_check_url")
	if manifestURL == "" {
		result.Error = "未配置版本检查 URL，请在设置中填写版本清单地址。"
		return result
	}

	manifestURL = strings.TrimSpace(manifestURL)
	parsed, err := s.infra.ValidateOutboundURLStrict(ctx, manifestURL)
	if err != nil {
		result.Error = fmt.Sprintf("版本检查 URL 校验失败: %v", err)
		return result
	}

	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		result.Error = fmt.Sprintf("构造请求失败: %v", err)
		return result
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "RelayCheck-Desktop/"+current)

	resp, err := s.infra.HTTPClient().Do(req)
	if err != nil {
		result.Error = fmt.Sprintf("请求版本清单失败: %v", err)
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		result.Error = fmt.Sprintf("版本清单服务返回 HTTP %d", resp.StatusCode)
		return result
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MiB max
	if err != nil {
		result.Error = fmt.Sprintf("读取版本清单失败: %v", err)
		return result
	}

	var manifest versionManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		result.Error = fmt.Sprintf("解析版本清单失败: %v", err)
		return result
	}

	result.LatestVersion = manifest.Version
	result.ReleaseURL = manifest.ReleaseURL
	result.ReleaseNotes = manifest.ReleaseNotes
	result.UpdateAvailable = CompareVersions(current, manifest.Version) < 0

	return result
}

// getSettingString reads a setting value stored as a JSON-encoded string.
// Returns empty string if the setting does not exist or is empty.
func (s *Service) getSettingString(ctx context.Context, key string) string {
	var valueJSON string
	if err := s.infra.DB().QueryRowContext(ctx, `SELECT value_json FROM system_settings WHERE key=?`, key).Scan(&valueJSON); err != nil {
		return ""
	}
	var str string
	if err := json.Unmarshal([]byte(valueJSON), &str); err != nil {
		// Not a JSON string, try raw value
		return strings.Trim(valueJSON, `"`)
	}
	return str
}

// CompareVersions returns -1 if a < b, 0 if a == b, 1 if a > b.
// Handles "v1.0", "1.0.0", "v2.1.3" etc.
func CompareVersions(a, b string) int {
	a = strings.TrimPrefix(strings.TrimSpace(a), "v")
	b = strings.TrimPrefix(strings.TrimSpace(b), "v")

	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")

	maxLen := len(partsA)
	if len(partsB) > maxLen {
		maxLen = len(partsB)
	}

	for i := 0; i < maxLen; i++ {
		va, vb := 0, 0
		if i < len(partsA) {
			fmt.Sscanf(partsA[i], "%d", &va)
		}
		if i < len(partsB) {
			fmt.Sscanf(partsB[i], "%d", &vb)
		}
		if va < vb {
			return -1
		}
		if va > vb {
			return 1
		}
	}
	return 0
}
