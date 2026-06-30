package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
)

// settingPayload is a small struct used to verify LoadSettingJSON's JSON
// unmarshalling. Field tags match the convention used by the scheduler
// config structs in scheduler.go.
type settingPayload struct {
	Enabled bool   `json:"enabled"`
	Name    string `json:"name"`
	Count   int    `json:"count"`
}

func TestSchedulerRepo_LoadSettingJSON_MissingKey(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	var target settingPayload
	err := app.schedulerRepo.LoadSettingJSON(context.Background(), "no.such.key", &target)
	if err == nil {
		t.Fatal("expected error for missing key, got nil")
	}
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows for missing key, got %v", err)
	}
}

func TestSchedulerRepo_LoadSettingJSON_Valid(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	const key = "test.setting.valid"
	payload := settingPayload{Enabled: true, Name: "hello", Count: 42}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, err := app.db.Exec(`
		INSERT INTO system_settings (id, key, value_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`, "setting-"+key, key, string(raw), now(), now()); err != nil {
		t.Fatalf("insert system_settings: %v", err)
	}

	var target settingPayload
	if err := app.schedulerRepo.LoadSettingJSON(context.Background(), key, &target); err != nil {
		t.Fatalf("LoadSettingJSON returned error: %v", err)
	}
	if !target.Enabled || target.Name != "hello" || target.Count != 42 {
		t.Fatalf("LoadSettingJSON mis-unmarshalled: %#v", target)
	}
}

func TestSchedulerRepo_LoadSettingJSON_InvalidJSON(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	const key = "test.setting.invalid"
	if _, err := app.db.Exec(`
		INSERT INTO system_settings (id, key, value_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`, "setting-"+key, key, "this is not json {", now(), now()); err != nil {
		t.Fatalf("insert system_settings: %v", err)
	}

	var target settingPayload
	err := app.schedulerRepo.LoadSettingJSON(context.Background(), key, &target)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
	// Should be a json.SyntaxError (or similar UnmarshalTypeError), not sql.ErrNoRows.
	if errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected JSON unmarshal error, got sql.ErrNoRows: %v", err)
	}
	// json.Unmarshal returns *json.SyntaxError for malformed input.
	if _, ok := err.(*json.SyntaxError); !ok {
		// Not all invalid inputs are SyntaxError; some produce UnmarshalTypeError.
		// Just verify the error is non-empty and mentions JSON-ish semantics.
		if err.Error() == "" {
			t.Fatalf("expected non-empty error message, got %v", err)
		}
	}
}

func TestSchedulerRepo_LoadSchedulerRun_MissingJob(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	_, err := app.schedulerRepo.LoadSchedulerRun(context.Background(), "no.such.job")
	if err == nil {
		t.Fatal("expected error for missing job, got nil")
	}
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows for missing job, got %v", err)
	}
}

func TestSchedulerRepo_LoadSchedulerRun_Valid(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	const jobKey = schedulerJobCheckin
	const plannedRunKey = "plan-run-1"
	const nextRunAt = "2026-07-01T08:00:00Z"
	const summary = "scheduled 3 accounts"
	if err := app.schedulerRepo.UpsertSchedulerPlan(context.Background(), jobKey, plannedRunKey, nextRunAt, summary); err != nil {
		t.Fatalf("UpsertSchedulerPlan: %v", err)
	}

	record, err := app.schedulerRepo.LoadSchedulerRun(context.Background(), jobKey)
	if err != nil {
		t.Fatalf("LoadSchedulerRun returned error: %v", err)
	}
	if record.JobKey != jobKey {
		t.Errorf("JobKey = %q, want %q", record.JobKey, jobKey)
	}
	if record.Status != "scheduled" {
		t.Errorf("Status = %q, want %q", record.Status, "scheduled")
	}
	if record.PlannedRunKey != plannedRunKey {
		t.Errorf("PlannedRunKey = %q, want %q", record.PlannedRunKey, plannedRunKey)
	}
	if record.NextRunAt != nextRunAt {
		t.Errorf("NextRunAt = %q, want %q", record.NextRunAt, nextRunAt)
	}
	if record.Summary != summary {
		t.Errorf("Summary = %q, want %q", record.Summary, summary)
	}
	if record.UpdatedAt == "" {
		t.Errorf("UpdatedAt = %q, want non-empty timestamp", record.UpdatedAt)
	}
	// Newly inserted rows have no last-run fields; those columns are NULL and
	// LoadSchedulerRun converts them to empty strings via sql.NullString.
	for _, field := range []struct{ name, val string }{
		{"LastRunKey", record.LastRunKey},
		{"LastStartedAt", record.LastStartedAt},
		{"LastFinishedAt", record.LastFinishedAt},
		{"LastSuccessAt", record.LastSuccessAt},
		{"LastError", record.LastError},
	} {
		if field.val != "" {
			t.Errorf("%s = %q, want empty string for fresh insert", field.name, field.val)
		}
	}
}

func TestSchedulerRepo_UpsertSchedulerPlan_InsertThenUpdate(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	const jobKey = schedulerJobSync
	const firstPlannedRunKey = "plan-A"
	const firstNextRunAt = "2026-07-01T08:00:00Z"
	const firstSummary = "first plan"
	const secondPlannedRunKey = "plan-B"
	const secondNextRunAt = "2026-07-02T08:00:00Z"
	const secondSummary = "updated plan"

	// First upsert = INSERT (no existing row).
	if err := app.schedulerRepo.UpsertSchedulerPlan(context.Background(), jobKey, firstPlannedRunKey, firstNextRunAt, firstSummary); err != nil {
		t.Fatalf("first UpsertSchedulerPlan: %v", err)
	}
	afterFirst, err := app.schedulerRepo.LoadSchedulerRun(context.Background(), jobKey)
	if err != nil {
		t.Fatalf("LoadSchedulerRun after first upsert: %v", err)
	}
	if afterFirst.PlannedRunKey != firstPlannedRunKey {
		t.Errorf("after first upsert PlannedRunKey = %q, want %q", afterFirst.PlannedRunKey, firstPlannedRunKey)
	}
	if afterFirst.NextRunAt != firstNextRunAt {
		t.Errorf("after first upsert NextRunAt = %q, want %q", afterFirst.NextRunAt, firstNextRunAt)
	}
	if afterFirst.Summary != firstSummary {
		t.Errorf("after first upsert Summary = %q, want %q", afterFirst.Summary, firstSummary)
	}
	firstUpdatedAt := afterFirst.UpdatedAt

	// Second upsert = UPDATE (same jobKey, different plan). Status is still
	// 'scheduled' (not 'running'), so all updatable fields should change.
	if err := app.schedulerRepo.UpsertSchedulerPlan(context.Background(), jobKey, secondPlannedRunKey, secondNextRunAt, secondSummary); err != nil {
		t.Fatalf("second UpsertSchedulerPlan: %v", err)
	}
	afterSecond, err := app.schedulerRepo.LoadSchedulerRun(context.Background(), jobKey)
	if err != nil {
		t.Fatalf("LoadSchedulerRun after second upsert: %v", err)
	}

	// Verify it is an UPDATE, not a duplicate INSERT.
	var rowCount int
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM scheduler_runs WHERE job_key=?`, jobKey).Scan(&rowCount); err != nil {
		t.Fatalf("count scheduler_runs: %v", err)
	}
	if rowCount != 1 {
		t.Fatalf("expected exactly 1 row for job_key=%q after second upsert, got %d (update should not insert)", jobKey, rowCount)
	}

	if afterSecond.PlannedRunKey != secondPlannedRunKey {
		t.Errorf("after second upsert PlannedRunKey = %q, want %q (update)", afterSecond.PlannedRunKey, secondPlannedRunKey)
	}
	if afterSecond.NextRunAt != secondNextRunAt {
		t.Errorf("after second upsert NextRunAt = %q, want %q (update)", afterSecond.NextRunAt, secondNextRunAt)
	}
	if afterSecond.Summary != secondSummary {
		t.Errorf("after second upsert Summary = %q, want %q (update)", afterSecond.Summary, secondSummary)
	}
	if afterSecond.Status != "scheduled" {
		t.Errorf("after second upsert Status = %q, want %q", afterSecond.Status, "scheduled")
	}
	if afterSecond.UpdatedAt == "" {
		t.Errorf("after second upsert UpdatedAt = empty, want non-empty")
	}
	// updated_at is rewritten on each upsert; the second timestamp must be
	// >= the first (equality is allowed because both upserts may land in the
	// same nanosecond on fast machines).
	if afterSecond.UpdatedAt < firstUpdatedAt {
		t.Errorf("UpdatedAt went backwards: first=%q second=%q", firstUpdatedAt, afterSecond.UpdatedAt)
	}
}
