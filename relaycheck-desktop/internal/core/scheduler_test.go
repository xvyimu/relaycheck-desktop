package core

import (
	"context"
	"testing"
	"time"
)

func TestBuildSchedulerStatusIncludesKnownJobs(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	status := app.buildSchedulerStatus(context.Background())
	if len(status.Jobs) != 2 {
		t.Fatalf("expected two scheduler jobs, got %d", len(status.Jobs))
	}
	if status.Jobs[0].Key != schedulerJobCheckin || status.Jobs[1].Key != schedulerJobSync {
		t.Fatalf("unexpected scheduler jobs: %#v", status.Jobs)
	}
}

func TestSyncSchedulerWaitsForDefaultIntervalOnStartup(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	startedAt := time.Date(2026, 6, 19, 8, 0, 0, 0, time.Local)
	app.schedulerStartedAt = startedAt
	app.tickSyncScheduler(context.Background(), startedAt.Add(10*time.Minute))

	record, err := app.loadSchedulerRun(context.Background(), schedulerJobSync)
	if err != nil {
		t.Fatal(err)
	}
	if record.Status != "scheduled" {
		t.Fatalf("expected scheduled status, got %s", record.Status)
	}
	if record.LastFinishedAt != "" {
		t.Fatalf("expected no immediate startup sync, got finished at %s", record.LastFinishedAt)
	}
	nextRunAt, err := time.Parse(time.RFC3339Nano, record.NextRunAt)
	if err != nil {
		t.Fatal(err)
	}
	if nextRunAt.Sub(startedAt) != 30*time.Minute {
		t.Fatalf("expected next run 30 minutes after startup, got %s", nextRunAt.Sub(startedAt))
	}
}
