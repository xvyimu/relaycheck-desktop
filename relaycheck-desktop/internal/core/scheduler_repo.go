package core

import (
	"context"
	"database/sql"
	"encoding/json"
)

// SchedulerRepo encapsulates database access for scheduler-related tables
// (scheduler_runs, system_settings). It is a pure repository: holds only
// db, no dependency on *App state.
type SchedulerRepo struct {
	db *sql.DB
}

// NewSchedulerRepo constructs a SchedulerRepo backed by the given db.
func NewSchedulerRepo(db *sql.DB) *SchedulerRepo {
	return &SchedulerRepo{db: db}
}

// LoadSettingJSON reads a JSON-encoded setting from system_settings by key
// and unmarshals it into target. The underlying error (including
// sql.ErrNoRows when the key is absent) is returned to the caller.
func (r *SchedulerRepo) LoadSettingJSON(ctx context.Context, key string, target interface{}) error {
	var valueJSON string
	if err := r.db.QueryRowContext(ctx, `SELECT value_json FROM system_settings WHERE key=?`, key).Scan(&valueJSON); err != nil {
		return err
	}
	return json.Unmarshal([]byte(valueJSON), target)
}

// LoadSchedulerRun reads a scheduler_runs row by job_key. The returned
// record is zero-valued when the row does not exist; callers should inspect
// the error (sql.ErrNoRows) to detect absence.
func (r *SchedulerRepo) LoadSchedulerRun(ctx context.Context, jobKey string) (schedulerRunRecord, error) {
	var record schedulerRunRecord
	var planned, nextRun, lastRun, started, finished, success, errText, summary sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT job_key, status, planned_run_key, next_run_at, last_run_key,
		       last_started_at, last_finished_at, last_success_at, last_error, summary, updated_at
		FROM scheduler_runs
		WHERE job_key=?
	`, jobKey).Scan(&record.JobKey, &record.Status, &planned, &nextRun, &lastRun, &started, &finished, &success, &errText, &summary, &record.UpdatedAt)
	if err != nil {
		return record, err
	}
	record.PlannedRunKey = planned.String
	record.NextRunAt = nextRun.String
	record.LastRunKey = lastRun.String
	record.LastStartedAt = started.String
	record.LastFinishedAt = finished.String
	record.LastSuccessAt = success.String
	record.LastError = errText.String
	record.Summary = summary.String
	return record, nil
}

// UpsertSchedulerPlan inserts a scheduler_runs row with status 'scheduled'
// for the given planned run, or updates the existing row's plan while
// preserving a 'running' status (and its summary) if a run is in flight.
func (r *SchedulerRepo) UpsertSchedulerPlan(ctx context.Context, jobKey string, plannedRunKey string, nextRunAt string, summary string) error {
	timestamp := now()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO scheduler_runs (job_key, status, planned_run_key, next_run_at, summary, updated_at)
		VALUES (?, 'scheduled', ?, ?, ?, ?)
		ON CONFLICT(job_key) DO UPDATE SET
			status=CASE WHEN scheduler_runs.status='running' THEN scheduler_runs.status ELSE excluded.status END,
			planned_run_key=excluded.planned_run_key,
			next_run_at=excluded.next_run_at,
			summary=CASE WHEN scheduler_runs.status='running' THEN scheduler_runs.summary ELSE excluded.summary END,
			updated_at=excluded.updated_at
	`, jobKey, plannedRunKey, nextRunAt, summary, timestamp)
	return err
}
