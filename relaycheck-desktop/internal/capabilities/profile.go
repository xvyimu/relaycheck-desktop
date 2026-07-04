package capabilities

import (
	"net/http"
	"strings"
)

// APICandidate describes one HTTP endpoint candidate for a site capability.
type APICandidate struct {
	Method string
	Path   string
}

// SiteProfile centralizes endpoint conventions for a known relay family.
type SiteProfile struct {
	Kind              string
	PreferredCheckins []APICandidate
	UserIDHeader      string
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

var loginProbePaths = []string{
	"/login",
	"/console/login",
	"/panel/login",
	"/admin/login",
	"/user/login",
	"/auth/login",
	"/signin",
	"/sign-in",
}

var loginAPIPaths = []string{
	"/api/user/login",
	"/api/login",
	"/api/auth/login",
}

var checkinCandidates = []APICandidate{
	{Method: http.MethodPost, Path: "/api/user/checkin"},
	{Method: http.MethodGet, Path: "/api/user/checkin"},
	{Method: http.MethodPost, Path: "/api/checkin"},
	{Method: http.MethodGet, Path: "/api/checkin"},
	{Method: http.MethodPost, Path: "/api/user/check_in"},
	{Method: http.MethodGet, Path: "/api/user/check_in"},
	{Method: http.MethodPost, Path: "/api/user/signin"},
	{Method: http.MethodGet, Path: "/api/user/signin"},
	{Method: http.MethodPost, Path: "/api/user/sign_in"},
	{Method: http.MethodGet, Path: "/api/user/sign_in"},
	{Method: http.MethodPost, Path: "/api/user/sign-in"},
	{Method: http.MethodGet, Path: "/api/user/sign-in"},
	{Method: http.MethodPost, Path: "/api/signin"},
	{Method: http.MethodGet, Path: "/api/signin"},
	{Method: http.MethodPost, Path: "/api/sign_in"},
	{Method: http.MethodGet, Path: "/api/sign_in"},
	{Method: http.MethodPost, Path: "/api/sign-in"},
	{Method: http.MethodGet, Path: "/api/sign-in"},
	{Method: http.MethodPost, Path: "/api/daily_checkin"},
	{Method: http.MethodGet, Path: "/api/daily_checkin"},
	{Method: http.MethodPost, Path: "/api/daily-checkin"},
	{Method: http.MethodGet, Path: "/api/daily-checkin"},
}

var profiles = map[string]SiteProfile{
	"newapi": {
		Kind:              "newapi",
		PreferredCheckins: []APICandidate{{Method: http.MethodPost, Path: "/api/user/checkin"}},
		UserIDHeader:      "New-Api-User",
	},
	"oneapi": {
		Kind:              "oneapi",
		PreferredCheckins: []APICandidate{{Method: http.MethodPost, Path: "/api/user/checkin"}},
		UserIDHeader:      "New-Api-User",
	},
	"modified_relay": {
		Kind: "modified_relay",
		PreferredCheckins: []APICandidate{
			{Method: http.MethodPost, Path: "/api/user/checkin"},
			{Method: http.MethodPost, Path: "/api/user/check_in"},
		},
		UserIDHeader: "New-Api-User",
	},
}

// ProfileForKind returns the normalized profile for a relay kind, if known.
func ProfileForKind(kind string) (SiteProfile, bool) {
	profile, ok := profiles[normalizeKind(kind)]
	if !ok {
		return SiteProfile{}, false
	}
	profile.PreferredCheckins = cloneCandidates(profile.PreferredCheckins)
	return profile, true
}

// UpstreamProbePaths returns the shared remote probe path list.
func UpstreamProbePaths() []string {
	return cloneStrings(upstreamProbePaths)
}

// LoginProbePaths returns browser/login page probe path candidates.
func LoginProbePaths() []string {
	return cloneStrings(loginProbePaths)
}

// IsLoginProbePath reports whether path is part of the login-page probe set.
func IsLoginProbePath(path string) bool {
	for _, candidate := range loginProbePaths {
		if path == candidate {
			return true
		}
	}
	return false
}

// LoginAPIPaths returns password-login API candidates. A custom path is only
// used when it already points at an API route; page URLs stay browser-only.
func LoginAPIPaths(customPath string) []string {
	paths := []string{}
	if strings.Contains(customPath, "/api/") {
		paths = append(paths, customPath)
	}
	paths = append(paths, loginAPIPaths...)
	return dedupeStrings(paths)
}

// CheckinCandidates returns global check-in endpoint fallbacks.
func CheckinCandidates() []APICandidate {
	return cloneCandidates(checkinCandidates)
}

// CheckinCandidatesForKind orders custom rules, known site profile rules, then
// global fallbacks, while keeping the first occurrence of each method/path.
func CheckinCandidatesForKind(kind string, customRules []APICandidate) []APICandidate {
	candidates := make([]APICandidate, 0, len(customRules)+len(checkinCandidates)+2)
	candidates = append(candidates, customRules...)
	if profile, ok := ProfileForKind(kind); ok {
		candidates = append(candidates, profile.PreferredCheckins...)
	}
	candidates = append(candidates, checkinCandidates...)
	return dedupeCandidates(candidates)
}

// UserIDHeaderForKind returns the custom user-id header for known site kinds.
func UserIDHeaderForKind(kind string) (string, bool) {
	if profile, ok := ProfileForKind(kind); ok && profile.UserIDHeader != "" {
		return profile.UserIDHeader, true
	}
	return "", false
}

func normalizeKind(kind string) string {
	return strings.ToLower(strings.TrimSpace(kind))
}

func cloneStrings(values []string) []string {
	out := make([]string, len(values))
	copy(out, values)
	return out
}

func cloneCandidates(values []APICandidate) []APICandidate {
	out := make([]APICandidate, len(values))
	copy(out, values)
	return out
}

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func dedupeCandidates(candidates []APICandidate) []APICandidate {
	seen := map[string]bool{}
	result := []APICandidate{}
	for _, candidate := range candidates {
		key := candidate.Method + " " + candidate.Path
		if candidate.Method == "" || candidate.Path == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, candidate)
	}
	return result
}
