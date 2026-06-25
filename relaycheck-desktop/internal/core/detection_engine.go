package core

import "strings"

// detectSiteKindFromHeaders detects the site kind from HTTP response headers.
func detectSiteKindFromHeaders(headers map[string]string) string {
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

// detectSiteKindFromHTML detects the site kind from HTML content.
func detectSiteKindFromHTML(html string) string {
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

// detectSiteKindFromAPIResponse detects the site kind from an API response body.
func detectSiteKindFromAPIResponse(path, response string) string {
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

// siteKindConfidence returns a confidence score for a detected site kind
// based on how many independent detection sources agreed.
func siteKindConfidence(kind string, sources int) float64 {
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
