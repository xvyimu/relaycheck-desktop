package core

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckinTaskServiceStartCheckinPublishesProgress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/checkin" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"task checked in"}`))
	}))
	defer server.Close()

	app := newTestApp(t)
	app.client = server.Client()
	app.allowLocalOutbound = true

	siteID := newID()
	accountID := newID()
	cookieEncrypted, err := app.encryptText("session=valid")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, supports_checkin, checkin_config_json, created_at, updated_at)
		VALUES (?, 'Task Relay', ?, 'newapi', 'healthy', 1, '{"method":"POST","path":"/checkin"}', ?, ?)
	`, siteID, server.URL, now(), now()); err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.Exec(`
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, cookie_encrypted, login_status, created_at, updated_at)
		VALUES (?, ?, 'Task Account', 'cookie', ?, 'valid', ?, ?)
	`, accountID, siteID, cookieEncrypted, now(), now()); err != nil {
		t.Fatal(err)
	}

	app.checkinTasks.StartCheckin("task-checkin-service")
	progress := waitForTaskDone(t, app, "task-checkin-service")

	if progress.Type != TaskCheckin {
		t.Fatalf("task type = %q, want %q", progress.Type, TaskCheckin)
	}
	if progress.Status != TaskStatusDone {
		t.Fatalf("task status = %q, want done: %#v", progress.Status, progress)
	}
	if progress.Total != 1 || progress.Current != 1 {
		t.Fatalf("progress = %d/%d, want 1/1", progress.Current, progress.Total)
	}
	if len(progress.Results) != 1 {
		t.Fatalf("result count = %d, want 1", len(progress.Results))
	}
	result := progress.Results[0]
	if result.ID != accountID || result.Name != "Task Account" || result.Status != "success" {
		t.Fatalf("unexpected task result: %#v", result)
	}

	var storedStatus string
	if err := app.db.QueryRow(`SELECT status FROM checkin_logs WHERE account_id=?`, accountID).Scan(&storedStatus); err != nil {
		t.Fatal(err)
	}
	if storedStatus != "success" {
		t.Fatalf("stored status = %q, want success", storedStatus)
	}
}

func TestCheckinTaskServiceStartRefreshBalancesPublishesProgress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/dashboard/billing/subscription" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"balance":42.5,"used_quota":3,"total_quota":50}`))
	}))
	defer server.Close()

	app := newTestApp(t)
	app.client = server.Client()
	app.allowLocalOutbound = true

	siteID := newID()
	accountID := newID()
	cookieEncrypted, err := app.encryptText("session=valid")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, supports_balance, created_at, updated_at)
		VALUES (?, 'Task Balance Relay', ?, 'newapi', 'healthy', 1, ?, ?)
	`, siteID, server.URL, now(), now()); err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.Exec(`
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, cookie_encrypted, login_status, created_at, updated_at)
		VALUES (?, ?, 'Task Balance Account', 'cookie', ?, 'valid', ?, ?)
	`, accountID, siteID, cookieEncrypted, now(), now()); err != nil {
		t.Fatal(err)
	}

	app.checkinTasks.StartRefreshBalances("task-refresh-balance-service", map[string]interface{}{
		"limit":       float64(5),
		"missingOnly": true,
	})
	progress := waitForTaskDone(t, app, "task-refresh-balance-service")

	if progress.Type != TaskRefreshBalances {
		t.Fatalf("task type = %q, want %q", progress.Type, TaskRefreshBalances)
	}
	if progress.Status != TaskStatusDone {
		t.Fatalf("task status = %q, want done: %#v", progress.Status, progress)
	}
	if progress.Total != 1 || progress.Current != 1 {
		t.Fatalf("progress = %d/%d, want 1/1", progress.Current, progress.Total)
	}
	if len(progress.Results) != 1 {
		t.Fatalf("result count = %d, want 1", len(progress.Results))
	}
	result := progress.Results[0]
	if result.ID != accountID || result.Name != "Task Balance Account" || result.Status != "success" {
		t.Fatalf("unexpected task result: %#v", result)
	}

	var balance float64
	if err := app.db.QueryRow(`SELECT balance FROM channel_accounts WHERE id=?`, accountID).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if balance != 42.5 {
		t.Fatalf("stored balance = %f, want 42.5", balance)
	}
}
