package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestCheckinExecutorRunRetriesTemporaryFailures(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/checkin" {
			http.NotFound(w, r)
			return
		}
		call := calls.Add(1)
		if call <= 2 {
			http.Error(w, `{"message":"temporary upstream error"}`, http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"checked in"}`))
	}))
	defer server.Close()

	app := newTestApp(t)
	defer app.Close()
	app.client = server.Client()
	app.allowLocalOutbound = true

	siteID := newID()
	accountID := newID()
	cookieEncrypted, err := app.encryptText("session=valid")
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, supports_checkin, checkin_config_json, created_at, updated_at)
		VALUES (?, 'Service Retry Relay', ?, 'newapi', 'healthy', 1, '{"method":"POST","path":"/checkin"}', ?, ?)
	`, siteID, server.URL, now(), now())
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.db.Exec(`
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, cookie_encrypted, login_status, created_at, updated_at)
		VALUES (?, ?, 'Service Retry Account', 'cookie', ?, 'valid', ?, ?)
	`, accountID, siteID, cookieEncrypted, now(), now())
	if err != nil {
		t.Fatal(err)
	}

	result, err := app.checkinExecutor.Run(context.Background(), accountID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "success" {
		t.Fatalf("expected success after retries, got %+v", result)
	}
	if result.RetryCount != 2 {
		t.Fatalf("expected two retries, got %+v", result)
	}
	if calls.Load() != 3 {
		t.Fatalf("expected three checkin attempts, got %d", calls.Load())
	}
}

func TestBalanceRefresherRunSavesBalance(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/dashboard/billing/subscription" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"balance":12.5,"used_quota":3,"total_quota":20}`))
	}))
	defer server.Close()

	app := newTestApp(t)
	defer app.Close()
	app.client = server.Client()
	app.allowLocalOutbound = true

	siteID := newID()
	accountID := newID()
	cookieEncrypted, err := app.encryptText("session=valid")
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, supports_balance, created_at, updated_at)
		VALUES (?, 'Service Balance Relay', ?, 'newapi', 'healthy', 1, ?, ?)
	`, siteID, server.URL, now(), now())
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.db.Exec(`
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, cookie_encrypted, login_status, created_at, updated_at)
		VALUES (?, ?, 'Service Balance Account', 'cookie', ?, 'valid', ?, ?)
	`, accountID, siteID, cookieEncrypted, now(), now())
	if err != nil {
		t.Fatal(err)
	}

	result, err := app.balanceRefresher.Run(context.Background(), accountID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Balance == nil || *result.Balance != 12.5 {
		t.Fatalf("expected balance 12.5, got %+v", result)
	}
	if result.Path != "/v1/dashboard/billing/subscription" {
		t.Fatalf("expected first balance path, got %q", result.Path)
	}

	var balance float64
	var status string
	if err := app.db.QueryRow(`SELECT balance, login_status FROM channel_accounts WHERE id=?`, accountID).Scan(&balance, &status); err != nil {
		t.Fatal(err)
	}
	if balance != 12.5 {
		t.Fatalf("expected saved balance 12.5, got %f", balance)
	}
	if status != "valid" {
		t.Fatalf("expected login_status valid, got %q", status)
	}
}

func TestCheckinBatchOrchestratorRunProcessesDueAccounts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/checkin" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"batch checked in"}`))
	}))
	defer server.Close()

	app := newTestApp(t)
	defer app.Close()
	app.client = server.Client()
	app.allowLocalOutbound = true

	siteID := newID()
	accountID := newID()
	cookieEncrypted, err := app.encryptText("session=valid")
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, supports_checkin, checkin_config_json, created_at, updated_at)
		VALUES (?, 'Batch Relay', ?, 'newapi', 'healthy', 1, '{"method":"POST","path":"/checkin"}', ?, ?)
	`, siteID, server.URL, now(), now())
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.db.Exec(`
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, cookie_encrypted, login_status, created_at, updated_at)
		VALUES (?, ?, 'Batch Account', 'cookie', ?, 'valid', ?, ?)
	`, accountID, siteID, cookieEncrypted, now(), now())
	if err != nil {
		t.Fatal(err)
	}

	results, err := app.checkinBatch.Run(context.Background(), "manual")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one batch result, got %d", len(results))
	}
	if results[0]["status"] != "success" {
		t.Fatalf("expected success result, got %#v", results[0])
	}

	snapshot := app.checkinRun.Snapshot()
	if snapshot.SuccessCount != 1 || snapshot.ProcessedAccounts != 1 {
		t.Fatalf("unexpected run snapshot: %#v", snapshot)
	}

	var storedStatus string
	if err := app.db.QueryRow(`SELECT status FROM checkin_logs WHERE account_id=?`, accountID).Scan(&storedStatus); err != nil {
		t.Fatal(err)
	}
	if storedStatus != "success" {
		t.Fatalf("expected persisted success, got %q", storedStatus)
	}
}
