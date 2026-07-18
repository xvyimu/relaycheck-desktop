package core

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

type checkinPlanFixture struct {
	id              string
	name            string
	supportsCheckin int
	checkinConfig   string
	cookie          string
	accessToken     string
	apiKey          string
	loginName       string
	password        string
	lastStatus      string
	lastAt          string
	updatedAt       string
}

func insertCheckinPlanFixture(t *testing.T, app *App, fixture checkinPlanFixture) {
	t.Helper()
	crypt := func(value string) string {
		if value == "" {
			return ""
		}
		encrypted, err := app.encryptText(value)
		if err != nil {
			t.Fatalf("encrypt fixture value: %v", err)
		}
		return encrypted
	}
	siteID := "site-" + fixture.id
	if _, err := app.db.Exec(`
		INSERT INTO upstream_sites (
			id, name, base_url, kind, health_status, supports_checkin,
			checkin_config_json, created_at, updated_at
		) VALUES (?, ?, ?, 'newapi', 'healthy', ?, ?, ?, ?)
	`, siteID, "Site "+fixture.name, "https://"+fixture.id+".example", fixture.supportsCheckin, fixture.checkinConfig, fixture.updatedAt, fixture.updatedAt); err != nil {
		t.Fatalf("insert fixture site %s: %v", fixture.id, err)
	}
	if _, err := app.db.Exec(`
		INSERT INTO channel_accounts (
			id, upstream_site_id, display_name, auth_type, login_status, email,
			password_encrypted, cookie_encrypted, access_token_encrypted, api_key_encrypted,
			last_checkin_status, last_checkin_at, created_at, updated_at
		) VALUES (?, ?, ?, 'cookie', 'unknown', ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, fixture.id, siteID, fixture.name, fixture.loginName, crypt(fixture.password), crypt(fixture.cookie), crypt(fixture.accessToken), crypt(fixture.apiKey), fixture.lastStatus, fixture.lastAt, fixture.updatedAt, fixture.updatedAt); err != nil {
		t.Fatalf("insert fixture account %s: %v", fixture.id, err)
	}
}

func TestCheckinPlanBuildsAllDueAndFreezesWillRunOrder(t *testing.T) {
	app := newTestApp(t)

	fixtures := []checkinPlanFixture{
		{id: "excluded-today", name: "Already done", supportsCheckin: 1, apiKey: "key", lastStatus: "success", lastAt: todayCST() + "T00:00:00Z", updatedAt: "2026-07-18T06:00:00Z"},
		{id: "cookie-ready", name: "Cookie ready", supportsCheckin: 1, cookie: "session=ready", updatedAt: "2026-07-18T05:00:00Z"},
		{id: "unsupported", name: "Unsupported", supportsCheckin: 0, cookie: "session=ready", updatedAt: "2026-07-18T04:00:00Z"},
		{id: "missing", name: "Missing credentials", supportsCheckin: 1, updatedAt: "2026-07-18T03:00:00Z"},
		{id: "password-ready", name: "Password ready", supportsCheckin: 1, loginName: "operator@example.com", password: "saved-password", updatedAt: "2026-07-18T02:00:00Z"},
		{id: "custom-ready", name: "Custom rule", supportsCheckin: 0, checkinConfig: `{"method":"POST","path":"/custom-checkin"}`, accessToken: "access-token", updatedAt: "2026-07-18T01:00:00Z"},
	}
	for _, fixture := range fixtures {
		insertCheckinPlanFixture(t, app, fixture)
	}

	preview, err := app.checkinPlans.BuildAllDue(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if preview.MaxAccounts != maxDryRunAccounts || preview.TotalAccounts != 5 || preview.WillRun != 3 || preview.Skipped != 2 {
		t.Fatalf("unexpected preview counts: %#v", preview)
	}
	if preview.PreviewID == "" || preview.ExpiresAt == "" {
		t.Fatalf("runnable preview must include one-time metadata: %#v", preview)
	}

	actions := map[string]string{}
	previewRunIDs := []string{}
	for _, item := range preview.Items {
		actions[item.AccountID] = item.Action
		if item.Action == "will_run" {
			previewRunIDs = append(previewRunIDs, item.AccountID)
		}
	}
	if actions["unsupported"] != "skip_unsupported" || actions["missing"] != "skip_missing_credentials" {
		t.Fatalf("unexpected skipped classification: %#v", actions)
	}
	if actions["cookie-ready"] != "will_run" || actions["password-ready"] != "will_run" || actions["custom-ready"] != "will_run" {
		t.Fatalf("unexpected runnable classification: %#v", actions)
	}

	plan, err := app.checkinPlans.Claim(preview.PreviewID)
	if err != nil {
		t.Fatal(err)
	}
	actualRunIDs := checkinRunAccountIDs(plan.RunAccounts)
	if fmt.Sprint(actualRunIDs) != fmt.Sprint(previewRunIDs) {
		t.Fatalf("T must equal ordered preview R: task=%v preview=%v", actualRunIDs, previewRunIDs)
	}
	if _, err := app.checkinPlans.Claim(preview.PreviewID); !errors.Is(err, errCheckinPreviewUnavailable) {
		t.Fatalf("second claim must fail closed, got %v", err)
	}
}

func TestCheckinPlanWithNoRunnableAccountsDoesNotIssuePreviewID(t *testing.T) {
	app := newTestApp(t)
	insertCheckinPlanFixture(t, app, checkinPlanFixture{
		id: "no-credentials", name: "No credentials", supportsCheckin: 1, updatedAt: "2026-07-18T01:00:00Z",
	})

	preview, err := app.checkinPlans.BuildAllDue(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if preview.WillRun != 0 || preview.PreviewID != "" || preview.ExpiresAt != "" {
		t.Fatalf("zero-runnable preview must not issue token: %#v", preview)
	}
	if app.checkinPlanStore.count() != 0 {
		t.Fatalf("zero-runnable preview must not be stored")
	}
}

func TestCheckinPlanStoreRejectsExpiredReplayedAndOverCapacityPlans(t *testing.T) {
	clock := time.Date(2026, 7, 18, 1, 0, 0, 0, time.UTC)
	store := NewCheckinPlanStore()
	store.now = func() time.Time { return clock }

	plan := CheckinExecutionPlan{ID: "expires", CreatedAt: clock, ExpiresAt: clock.Add(checkinPreviewTTL)}
	if err := store.Put(plan); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(checkinPreviewTTL)
	if _, err := store.Claim(plan.ID); !errors.Is(err, errCheckinPreviewUnavailable) {
		t.Fatalf("expired claim must fail closed, got %v", err)
	}

	for i := 0; i < maxPendingCheckinPreviews; i++ {
		id := fmt.Sprintf("pending-%d", i)
		if err := store.Put(CheckinExecutionPlan{ID: id, CreatedAt: clock, ExpiresAt: clock.Add(time.Minute)}); err != nil {
			t.Fatalf("put %s: %v", id, err)
		}
	}
	if err := store.Put(CheckinExecutionPlan{ID: "overflow", CreatedAt: clock, ExpiresAt: clock.Add(time.Minute)}); !errors.Is(err, errCheckinPreviewCapacity) {
		t.Fatalf("capacity overflow must be rejected, got %v", err)
	}
}

func TestCheckinPlanRejectsThe201stDueAccountWithoutStoringPartialPlan(t *testing.T) {
	app := newTestApp(t)
	siteID := "site-over-limit"
	stamp := "2026-07-18T01:00:00Z"
	if _, err := app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, supports_checkin, created_at, updated_at)
		VALUES (?, 'Over limit', 'https://over-limit.example', 'newapi', 'healthy', 1, ?, ?)
	`, siteID, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	tx, err := app.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i <= maxDryRunAccounts; i++ {
		id := fmt.Sprintf("over-limit-%03d", i)
		if _, err := tx.Exec(`
			INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, login_status, created_at, updated_at)
			VALUES (?, ?, ?, 'api_key', 'valid', ?, ?)
		`, id, siteID, id, stamp, stamp); err != nil {
			_ = tx.Rollback()
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	if _, err := app.checkinPlans.BuildAllDue(context.Background()); !errors.Is(err, errCheckinPreviewLimit) {
		t.Fatalf("201st due account must fail closed, got %v", err)
	}
	if app.checkinPlanStore.count() != 0 {
		t.Fatalf("over-limit preview must not store a partial plan")
	}
}
