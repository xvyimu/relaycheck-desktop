package core

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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

	preview, err := app.checkinPlans.BuildAllDue(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	plan, err := app.checkinPlans.Claim(preview.PreviewID)
	if err != nil {
		t.Fatal(err)
	}
	lateAccountID := newID()
	if _, err := app.db.Exec(`
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, cookie_encrypted, login_status, created_at, updated_at)
		VALUES (?, ?, 'Added After Preview', 'cookie', ?, 'valid', ?, ?)
	`, lateAccountID, siteID, cookieEncrypted, now(), now()); err != nil {
		t.Fatal(err)
	}

	if err := app.checkinTasks.StartCheckin("task-checkin-service", plan.RunAccounts); err != nil {
		t.Fatal(err)
	}
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
	if result.ID == lateAccountID {
		t.Fatal("task expanded to an account added after preview")
	}
	run := app.checkinRun.Snapshot()
	if run.Running || run.ProcessedAccounts != 1 || run.SuccessCount != 1 {
		t.Fatalf("checkin run state diverged from task progress: %#v", run)
	}

	var storedStatus string
	if err := app.db.QueryRow(`SELECT status FROM checkin_logs WHERE account_id=?`, accountID).Scan(&storedStatus); err != nil {
		t.Fatal(err)
	}
	if storedStatus != "success" {
		t.Fatalf("stored status = %q, want success", storedStatus)
	}
}

func TestCheckinTaskServiceDoesNotReplaceAPlannedAccountDeletedAfterPreview(t *testing.T) {
	app := newTestApp(t)
	insertCheckinPlanFixture(t, app, checkinPlanFixture{
		id: "deleted-after-preview", name: "Deleted after preview", supportsCheckin: 1,
		apiKey: "saved-key", updatedAt: "2026-07-18T01:00:00Z",
	})
	preview, err := app.checkinPlans.BuildAllDue(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	plan, err := app.checkinPlans.Claim(preview.PreviewID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.Exec(`DELETE FROM channel_accounts WHERE id=?`, "deleted-after-preview"); err != nil {
		t.Fatal(err)
	}

	if err := app.checkinTasks.StartCheckin("task-deleted-plan", plan.RunAccounts); err != nil {
		t.Fatal(err)
	}
	progress := waitForTaskDone(t, app, "task-deleted-plan")
	if progress.Total != 1 || progress.Current != 1 || len(progress.Results) != 1 {
		t.Fatalf("deleted plan member must still produce one result: %#v", progress)
	}
	result := progress.Results[0]
	if result.ID != "deleted-after-preview" || result.Status != "failed" {
		t.Fatalf("deleted member was replaced or not failed: %#v", result)
	}
	for _, forbidden := range []string{"relaycheck.db", "SELECT ", "saved-key"} {
		if strings.Contains(result.Message, forbidden) {
			t.Fatalf("deleted account failure exposed %q: %q", forbidden, result.Message)
		}
	}
}

func TestCheckinTaskServiceCancellationStopsRemainingPlanAndFinishesRunState(t *testing.T) {
	started := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case started <- struct{}{}:
		default:
		}
		select {
		case <-r.Context().Done():
		case <-time.After(500 * time.Millisecond):
			http.Error(w, "bounded test timeout", http.StatusGatewayTimeout)
		}
	}))
	defer server.Close()

	app := newTestApp(t)
	app.client = server.Client()
	app.allowLocalOutbound = true
	for i, id := range []string{"cancel-first", "cancel-second"} {
		insertCheckinPlanFixture(t, app, checkinPlanFixture{
			id: id, name: id, supportsCheckin: 1, cookie: "session=ready",
			checkinConfig: `{"method":"POST","path":"/checkin"}`,
			updatedAt:     time.Date(2026, 7, 18, 2-i, 0, 0, 0, time.UTC).Format(time.RFC3339),
		})
		if _, err := app.db.Exec(`UPDATE upstream_sites SET base_url=? WHERE id=?`, server.URL, "site-"+id); err != nil {
			t.Fatal(err)
		}
	}
	preview, err := app.checkinPlans.BuildAllDue(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	plan, err := app.checkinPlans.Claim(preview.PreviewID)
	if err != nil {
		t.Fatal(err)
	}

	const taskID = "task-cancel-plan"
	if err := app.checkinTasks.StartCheckin(taskID, plan.RunAccounts); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("first planned account did not start")
	}
	if !app.taskRunner.cancelTask(taskID) {
		t.Fatal("registered checkin task could not be cancelled")
	}
	progress := waitForTaskDone(t, app, taskID)
	if progress.Status != TaskStatusCancelled || progress.Current >= progress.Total {
		t.Fatalf("cancellation did not stop remaining plan: %#v", progress)
	}
	if snapshot := app.checkinRun.Snapshot(); snapshot.Running {
		t.Fatalf("cancelled task left global run busy: %#v", snapshot)
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
