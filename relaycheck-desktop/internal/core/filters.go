package core

import "strings"

var excludedRelaySiteTokens = []string{
	"9router",
	"freemodel",
	"free model",
	"tokenrouter",
	"token router",
}

func isExcludedRelaySite(name string, baseURL string) bool {
	_, matched := excludedRelaySiteMatch(name, baseURL)
	return matched
}

func excludedRelaySiteMatch(name string, baseURL string) (string, bool) {
	combined := strings.ToLower(strings.TrimSpace(name) + " " + strings.TrimSpace(baseURL))
	for _, token := range excludedRelaySiteTokens {
		if strings.Contains(combined, token) {
			return token, true
		}
	}
	return "", false
}

func isManagedRelayKind(kind string) bool {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "newapi", "oneapi", "sub2api", "modified_relay":
		return true
	default:
		return false
	}
}
