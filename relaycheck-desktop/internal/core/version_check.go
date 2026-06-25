package core

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

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

func (a *App) handleVersionCheck(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}

	result := VersionCheckResult{
		CurrentVersion: productVersion,
		CheckedAt:      now(),
	}

	manifestURL := a.getSettingString(r.Context(), "app.version_check_url")
	if manifestURL == "" {
		result.Error = "未配置版本检查 URL，请在设置中填写版本清单地址。"
		writeJSON(w, http.StatusOK, result)
		return
	}

	manifestURL = strings.TrimSpace(manifestURL)
	parsed, err := validateOutboundHTTPURL(r.Context(), manifestURL, outboundURLPolicy{AllowLocal: false})
	if err != nil {
		result.Error = fmt.Sprintf("版本检查 URL 校验失败: %v", err)
		writeJSON(w, http.StatusOK, result)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		result.Error = fmt.Sprintf("构造请求失败: %v", err)
		writeJSON(w, http.StatusOK, result)
		return
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "RelayCheck-Desktop/"+productVersion)

	resp, err := a.client.Do(req)
	if err != nil {
		result.Error = fmt.Sprintf("请求版本清单失败: %v", err)
		writeJSON(w, http.StatusOK, result)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		result.Error = fmt.Sprintf("版本清单服务返回 HTTP %d", resp.StatusCode)
		writeJSON(w, http.StatusOK, result)
		return
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MiB max
	if err != nil {
		result.Error = fmt.Sprintf("读取版本清单失败: %v", err)
		writeJSON(w, http.StatusOK, result)
		return
	}

	var manifest versionManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		result.Error = fmt.Sprintf("解析版本清单失败: %v", err)
		writeJSON(w, http.StatusOK, result)
		return
	}

	result.LatestVersion = manifest.Version
	result.ReleaseURL = manifest.ReleaseURL
	result.ReleaseNotes = manifest.ReleaseNotes
	result.UpdateAvailable = compareVersions(productVersion, manifest.Version) < 0

	writeJSON(w, http.StatusOK, result)
}

// getSettingString reads a setting value stored as a JSON-encoded string.
// Returns empty string if the setting does not exist or is empty.
func (a *App) getSettingString(ctx context.Context, key string) string {
	var valueJSON string
	if err := a.db.QueryRowContext(ctx, `SELECT value_json FROM system_settings WHERE key=?`, key).Scan(&valueJSON); err != nil {
		return ""
	}
	var s string
	if err := json.Unmarshal([]byte(valueJSON), &s); err != nil {
		// Not a JSON string, try raw value
		return strings.Trim(valueJSON, `"`)
	}
	return s
}

// compareVersions returns -1 if a < b, 0 if a == b, 1 if a > b.
// Handles "v1.0", "1.0.0", "v2.1.3" etc.
func compareVersions(a, b string) int {
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
