package core

import (
	"context"
	"path/filepath"
	"testing"
)

func TestBaseURLForAutoDetectedDBPrefersExistingInstance(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	dbPath := filepath.Join(t.TempDir(), "one-api.db")
	_, err := app.db.ExecContext(context.Background(), `
		INSERT INTO local_newapi_instances (id, name, base_url, detected_from, status, database_path, created_at, updated_at)
		VALUES (?, 'Local NewAPI', ?, 'scan', 'healthy', ?, ?, ?)
	`, "local-newapi-existing", "http://127.0.0.1:3010", dbPath, now(), now())
	if err != nil {
		t.Fatalf("insert local instance: %v", err)
	}

	got := app.baseURLForAutoDetectedDB(context.Background(), dbPath)
	if got != "http://127.0.0.1:3010" {
		t.Fatalf("expected existing instance base URL, got %q", got)
	}
}
