package core

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// TestLargeDatasetPerformance verifies that dashboard queries complete
// within acceptable time limits even with 500+ accounts.
func TestLargeDatasetPerformance(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()
	var err error

	ctx := context.Background()

	// Create a test site
	siteID := "perf-test-site"
	_, err = app.db.ExecContext(ctx, `
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, supports_checkin, supports_balance, supports_models, supports_pricing, created_at, updated_at)
		VALUES (?, '性能测试站点', 'https://perf.example.com', 'newapi', 'healthy', 1, 1, 1, 1, ?, ?)
	`, siteID, now(), now())
	if err != nil {
		t.Fatalf("create site: %v", err)
	}

	// Insert 500 accounts
	const accountCount = 500
	for i := 0; i < accountCount; i++ {
		id := fmt.Sprintf("perf-account-%d", i)
		name := fmt.Sprintf("测试账号 %d", i)
		_, err := app.db.ExecContext(ctx, `
			INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, login_status, api_key_status, balance, balance_unit, created_at, updated_at)
			VALUES (?, ?, ?, 'cookie', 'valid', 'untested', ?, 'USD', ?, ?)
		`, id, siteID, name, float64(i)*0.5, now(), now())
		if err != nil {
			t.Fatalf("insert account %d: %v", i, err)
		}
	}

	// Insert 500 checkin logs
	for i := 0; i < accountCount; i++ {
		id := fmt.Sprintf("perf-log-%d", i)
		accountID := fmt.Sprintf("perf-account-%d", i)
		status := "success"
		if i%5 == 0 {
			status = "failed"
		}
		_, err := app.db.ExecContext(ctx, `
			INSERT INTO checkin_logs (id, account_id, upstream_site_id, status, message, started_at, finished_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, id, accountID, siteID, status, "测试", now(), now())
		if err != nil {
			t.Fatalf("insert log %d: %v", i, err)
		}
	}

	// Measure dashboard summary query
	start := time.Now()
	var accountCount2 int
	err = app.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM channel_accounts`).Scan(&accountCount2)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("count accounts: %v", err)
	}
	if accountCount2 != accountCount {
		t.Errorf("expected %d accounts, got %d", accountCount, accountCount2)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("count query took %v, expected < 500ms", elapsed)
	}
	t.Logf("count accounts with %d rows: %v", accountCount, elapsed)

	// Measure analytics query
	start = time.Now()
	rows, err := app.db.QueryContext(ctx, `
		SELECT status, COUNT(*) FROM checkin_logs WHERE started_at >= datetime('now','-7 days') GROUP BY status
	`)
	elapsed = time.Since(start)
	if err != nil {
		t.Fatalf("analytics query: %v", err)
	}
	rows.Close()
	if elapsed > 200*time.Millisecond {
		t.Errorf("analytics query took %v, expected < 200ms", elapsed)
	}
	t.Logf("analytics query with %d logs: %v", accountCount, elapsed)
}
