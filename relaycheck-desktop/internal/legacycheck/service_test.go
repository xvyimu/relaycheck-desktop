package legacycheck

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// fakeInfra implements Infra for testing, returning a fixed dataDir.
type fakeInfra struct {
	dataDir string
}

func (f fakeInfra) DataDir() string { return f.dataDir }

func TestCheck_NoLegacyDir(t *testing.T) {
	tmp := t.TempDir()
	svc := NewService(fakeInfra{dataDir: tmp})
	result := svc.Check(context.Background())
	if result.LegacyDirExists {
		t.Fatal("LegacyDirExists should be false when no legacy dir found")
	}
	if result.APIRoutesCount != 0 || result.DBInitTables != 0 {
		t.Fatal("counts should be zero when no legacy dir found")
	}
	if len(result.Notes) == 0 {
		t.Fatal("should include a 'not found' note")
	}
}

func TestCheck_WithLegacyDir(t *testing.T) {
	tmp := t.TempDir()
	// The Check method searches <dataDir>/../legacy/newapi_signin.
	legacyDir := filepath.Join(tmp, "..", "legacy", "newapi_signin")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	// api.py with 3 @app.route decorators.
	apiContent := `from flask import Flask
app = Flask(__name__)

@app.route("/api/status")
def status(): pass

@app.route("/api/checkin")
def checkin(): pass

@app.route("/api/balance")
def balance(): pass
`
	if err := os.WriteFile(filepath.Join(legacyDir, "api.py"), []byte(apiContent), 0o644); err != nil {
		t.Fatalf("write api.py failed: %v", err)
	}

	// database.py with 2 CREATE TABLE (idempotent).
	dbContent := `CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY);
CREATE TABLE IF NOT EXISTS channels (id INTEGER PRIMARY KEY);
`
	if err := os.WriteFile(filepath.Join(legacyDir, "database.py"), []byte(dbContent), 0o644); err != nil {
		t.Fatalf("write database.py failed: %v", err)
	}

	svc := NewService(fakeInfra{dataDir: tmp})
	result := svc.Check(context.Background())
	if !result.LegacyDirExists {
		t.Fatal("LegacyDirExists should be true")
	}
	if result.APIRoutesCount != 3 {
		t.Fatalf("expected 3 API routes, got %d", result.APIRoutesCount)
	}
	if result.DBInitTables != 2 {
		t.Fatalf("expected 2 DB tables, got %d", result.DBInitTables)
	}
	if !result.DBInitIdempotent {
		t.Fatal("DBInitIdempotent should be true when IF NOT EXISTS present")
	}
}

func TestCheck_LegacyDirNonIdempotent(t *testing.T) {
	tmp := t.TempDir()
	legacyDir := filepath.Join(tmp, "..", "legacy", "newapi_signin")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	// api.py minimal.
	if err := os.WriteFile(filepath.Join(legacyDir, "api.py"), []byte(`@app.route("/")`+"\n"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	// database.py without IF NOT EXISTS (non-idempotent).
	if err := os.WriteFile(filepath.Join(legacyDir, "database.py"), []byte("CREATE TABLE foo;\n"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	svc := NewService(fakeInfra{dataDir: tmp})
	result := svc.Check(context.Background())
	if result.DBInitIdempotent {
		t.Fatal("DBInitIdempotent should be false without IF NOT EXISTS")
	}
	if result.APIRoutesCount != 1 {
		t.Fatalf("expected 1 route, got %d", result.APIRoutesCount)
	}
}

func TestCheck_MissingAPIDB(t *testing.T) {
	tmp := t.TempDir()
	legacyDir := filepath.Join(tmp, "..", "legacy", "newapi_signin")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	// Only api.py, no database.py.
	if err := os.WriteFile(filepath.Join(legacyDir, "api.py"), []byte(`@app.route("/x")`+"\n"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	svc := NewService(fakeInfra{dataDir: tmp})
	result := svc.Check(context.Background())
	if !result.LegacyDirExists {
		t.Fatal("LegacyDirExists should be true (api.py exists)")
	}
	if result.APIRoutesCount != 1 {
		t.Fatalf("expected 1 route, got %d", result.APIRoutesCount)
	}
	if result.DBInitTables != 0 {
		t.Fatalf("expected 0 tables (no database.py), got %d", result.DBInitTables)
	}
	// Should include a note about missing database.py.
	found := false
	for _, note := range result.Notes {
		if note != "" && contains(note, "database.py") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected a note about missing database.py, got %v", result.Notes)
	}
}

func TestCheck_CheckedAtPopulated(t *testing.T) {
	tmp := t.TempDir()
	svc := NewService(fakeInfra{dataDir: tmp})
	result := svc.Check(context.Background())
	if result.CheckedAt == "" {
		t.Fatal("CheckedAt should be populated")
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
