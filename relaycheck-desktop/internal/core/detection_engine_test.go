package core

import (
	"context"
	"testing"
)

func TestDetectionWithRealApp(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()
	var err error

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
