package core

import (
	"context"
	"testing"
)

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
			got := detectSiteKindFromHeaders(tt.headers)
			if got != tt.want {
				t.Errorf("detectSiteKindFromHeaders() = %q, want %q", got, tt.want)
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
			got := detectSiteKindFromHTML(tt.html)
			if got != tt.want {
				t.Errorf("detectSiteKindFromHTML() = %q, want %q", got, tt.want)
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
			got := detectSiteKindFromAPIResponse(tt.apiPath, tt.response)
			if got != tt.want {
				t.Errorf("detectSiteKindFromAPIResponse() = %q, want %q", got, tt.want)
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
		got := siteKindConfidence(tt.kind, tt.sources)
		if got < tt.minScore {
			t.Errorf("siteKindConfidence(%q, %d) = %f, expected >= %f", tt.kind, tt.sources, got, tt.minScore)
		}
	}
}

func TestDetectionWithRealApp(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	ctx := context.Background()

	// Create a site with detection data
	_, err = app.db.ExecContext(ctx, `
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, supports_checkin, supports_balance, supports_models, supports_pricing, detection_confidence, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 1, 1, 1, 1, 0.9, ?, ?)
	`, "detect-test-site", "检测测试", "https://detect.example.com", "newapi", "healthy", now(), now())
	if err != nil {
		t.Fatalf("create site: %v", err)
	}

	// Verify the site was created with correct detection
	var kind string
	var confidence float64
	err = app.db.QueryRowContext(ctx, `SELECT kind, detection_confidence FROM upstream_sites WHERE id = ?`, "detect-test-site").Scan(&kind, &confidence)
	if err != nil {
		t.Fatalf("query site: %v", err)
	}
	if kind != "newapi" {
		t.Errorf("expected kind 'newapi', got %q", kind)
	}
	if confidence < 0.9 {
		t.Errorf("expected confidence >= 0.9, got %f", confidence)
	}
}
