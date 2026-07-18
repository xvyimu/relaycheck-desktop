package core

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// BenchmarkLargeDatasetHandlers is intentionally a benchmark (not a timing
// assertion) so CI can track trend on a representative local database without
// making developer machines flaky.
func BenchmarkLargeDatasetHandlers(b *testing.B) {
	app := newTestApp(b)
	defer app.Close()
	seedLargeDatasetForBenchmark(b, app, 5000)

	b.Run("accounts-page-search", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			app.invalidateReadCache()
			req := httptest.NewRequest(http.MethodGet, "/api/accounts/page?query=benchmark&limit=80", nil)
			rec := httptest.NewRecorder()
			app.handleAccountsPage(rec, req)
			if rec.Code != http.StatusOK {
				b.Fatalf("accounts page status = %d", rec.Code)
			}
		}
	})

	b.Run("usage-overview", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			app.invalidateReadCache()
			req := httptest.NewRequest(http.MethodGet, "/api/usage/overview?limit=80", nil)
			rec := httptest.NewRecorder()
			app.handleUsageOverview(rec, req)
			if rec.Code != http.StatusOK {
				b.Fatalf("usage overview status = %d", rec.Code)
			}
		}
	})

	b.Run("action-center", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			app.invalidateReadCache()
			req := httptest.NewRequest(http.MethodGet, "/api/system/action-center", nil)
			rec := httptest.NewRecorder()
			app.handleActionCenter(rec, req)
			if rec.Code != http.StatusOK {
				b.Fatalf("action center status = %d", rec.Code)
			}
		}
	})
}

func seedLargeDatasetForBenchmark(b *testing.B, app *App, accountCount int) {
	b.Helper()
	siteID := "benchmark-site"
	nowText := now()
	if _, err := app.db.Exec(`INSERT INTO upstream_sites (id, name, base_url, kind, health_status, supports_balance, created_at, updated_at) VALUES (?, 'Benchmark Site', 'https://benchmark.example', 'newapi', 'healthy', 1, ?, ?)`, siteID, nowText, nowText); err != nil {
		b.Fatal(err)
	}
	tx, err := app.db.Begin()
	if err != nil {
		b.Fatal(err)
	}
	accountStmt, err := tx.Prepare(`INSERT INTO channel_accounts (id, upstream_site_id, display_name, email, auth_type, login_status, balance, balance_unit, created_at, updated_at) VALUES (?, ?, ?, ?, 'api_key', 'valid', ?, 'usd', ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		b.Fatal(err)
	}
	logStmt, err := tx.Prepare(`INSERT INTO checkin_logs (id, account_id, upstream_site_id, status, started_at, finished_at) VALUES (?, ?, ?, 'success', ?, ?)`)
	if err != nil {
		_ = accountStmt.Close()
		_ = tx.Rollback()
		b.Fatal(err)
	}
	snapshotStmt, err := tx.Prepare(`INSERT INTO balance_snapshots (id, account_id, upstream_site_id, balance, unit, created_at) VALUES (?, ?, ?, ?, 'usd', ?)`)
	if err != nil {
		_ = accountStmt.Close()
		_ = logStmt.Close()
		_ = tx.Rollback()
		b.Fatal(err)
	}
	for i := 0; i < accountCount; i++ {
		accountID := fmt.Sprintf("benchmark-account-%05d", i)
		if _, err := accountStmt.Exec(accountID, siteID, "Benchmark Account "+accountID, accountID+"@example.com", float64(i%100), nowText, nowText); err != nil {
			b.Fatal(err)
		}
		if _, err := logStmt.Exec("benchmark-log-"+accountID, accountID, siteID, nowText, nowText); err != nil {
			b.Fatal(err)
		}
		for snapshot := 0; snapshot < 3; snapshot++ {
			if _, err := snapshotStmt.Exec(fmt.Sprintf("benchmark-snapshot-%s-%d", accountID, snapshot), accountID, siteID, float64(100-snapshot), nowText); err != nil {
				b.Fatal(err)
			}
		}
	}
	_ = accountStmt.Close()
	_ = logStmt.Close()
	_ = snapshotStmt.Close()
	if err := tx.Commit(); err != nil {
		b.Fatal(err)
	}
}
