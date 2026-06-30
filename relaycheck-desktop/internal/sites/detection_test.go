package sites

import "testing"

func TestDetectSiteKindFromHeaders(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		want    string
	}{
		{
			name:    "NewAPI powered header",
			headers: map[string]string{"X-Powered-By": "NewAPI"},
			want:    "newapi",
		},
		{
			name:    "OneAPI powered header",
			headers: map[string]string{"X-Powered-By": "OneAPI"},
			want:    "oneapi",
		},
		{
			name:    "Sub2API header",
			headers: map[string]string{"X-Powered-By": "Sub2API"},
			want:    "sub2api",
		},
		{
			name:    "Unknown header",
			headers: map[string]string{"X-Custom": "custom"},
			want:    "unknown",
		},
		{
			name:    "Empty headers",
			headers: map[string]string{},
			want:    "unknown",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectSiteKindFromHeaders(tt.headers)
			if got != tt.want {
				t.Errorf("DetectSiteKindFromHeaders() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDetectSiteKindFromHTML(t *testing.T) {
	tests := []struct {
		name string
		html string
		want string
	}{
		{
			name: "NewAPI in title",
			html: `<html><head><title>NewAPI - 管理后台</title></head></html>`,
			want: "newapi",
		},
		{
			name: "OneAPI in title",
			html: `<html><head><title>OneAPI - 接口管理</title></head></html>`,
			want: "oneapi",
		},
		{
			name: "Sub2API in meta",
			html: `<html><head><meta name="generator" content="Sub2API"></head></html>`,
			want: "sub2api",
		},
		{
			name: "Unknown HTML",
			html: `<html><head><title>Some Other App</title></head></html>`,
			want: "unknown",
		},
		{
			name: "Empty HTML",
			html: "",
			want: "unknown",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectSiteKindFromHTML(tt.html)
			if got != tt.want {
				t.Errorf("DetectSiteKindFromHTML() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDetectSiteKindFromAPIResponse(t *testing.T) {
	tests := []struct {
		name     string
		apiPath  string
		response string
		want     string
	}{
		{
			name:     "NewAPI status endpoint",
			apiPath:  "/api/status",
			response: `{"version":"v0.7.4","type":"newapi"}`,
			want:     "newapi",
		},
		{
			name:     "OneAPI status endpoint",
			apiPath:  "/api/status",
			response: `{"type":"oneapi"}`,
			want:     "oneapi",
		},
		{
			name:     "Sub2API in response",
			apiPath:  "/api/status",
			response: `{"type":"sub2api"}`,
			want:     "sub2api",
		},
		{
			name:     "Unknown API response",
			apiPath:  "/api/status",
			response: `{"ok":true}`,
			want:     "unknown",
		},
		{
			name:     "Empty response",
			apiPath:  "/api/status",
			response: "",
			want:     "unknown",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectSiteKindFromAPIResponse(tt.apiPath, tt.response)
			if got != tt.want {
				t.Errorf("DetectSiteKindFromAPIResponse() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSiteKindConfidence(t *testing.T) {
	tests := []struct {
		kind     string
		sources  int
		minScore float64
	}{
		{"newapi", 3, 0.9},
		{"newapi", 2, 0.7},
		{"newapi", 1, 0.4},
		{"unknown", 0, 0.0},
	}
	for _, tt := range tests {
		got := SiteKindConfidence(tt.kind, tt.sources)
		if got < tt.minScore {
			t.Errorf("SiteKindConfidence(%q, %d) = %f, expected >= %f", tt.kind, tt.sources, got, tt.minScore)
		}
	}
}
