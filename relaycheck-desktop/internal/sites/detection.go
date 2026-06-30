package sites

import "strings"

// DetectSiteKindFromHeaders detects the site kind from HTTP response headers.
func DetectSiteKindFromHeaders(headers map[string]string) string {
	powered := headers["X-Powered-By"]
	switch powered {
	case "NewAPI":
		return "newapi"
	case "OneAPI":
		return "oneapi"
	case "Sub2API":
		return "sub2api"
	}
	return "unknown"
}

// DetectSiteKindFromHTML detects the site kind from HTML content.
func DetectSiteKindFromHTML(html string) string {
	if strings.Contains(html, "NewAPI") {
		return "newapi"
	}
	if strings.Contains(html, "OneAPI") {
		return "oneapi"
	}
	if strings.Contains(html, "Sub2API") {
		return "sub2api"
	}
	return "unknown"
}

// DetectSiteKindFromAPIResponse detects the site kind from an API response body.
func DetectSiteKindFromAPIResponse(path, response string) string {
	if strings.Contains(response, "newapi") {
		return "newapi"
	}
	if strings.Contains(response, "oneapi") {
		return "oneapi"
	}
	if strings.Contains(response, "sub2api") {
		return "sub2api"
	}
	return "unknown"
}

// SiteKindConfidence returns a confidence score for a detected site kind
// based on how many independent detection sources agreed.
func SiteKindConfidence(kind string, sources int) float64 {
	if kind == "unknown" || sources == 0 {
		return 0.0
	}
	switch sources {
	case 1:
		return 0.4
	case 2:
		return 0.7
	default:
		return 0.9
	}
}
