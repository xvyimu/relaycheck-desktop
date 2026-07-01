package backup

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// stubBackupInfra implements the backup.Infra interface for tests.
type stubBackupInfra struct {
	db            *sql.DB
	dbPath        string
	backupsDir    string
	productVers   string
	reopenCalled  bool
	reloadCalled  bool
	reopenErr     error
	reloadErr     error
	reopenDBFn    func() error // optional: if set, ReopenDatabase calls this
}

func (s *stubBackupInfra) DB() *sql.DB                        { return s.db }
func (s *stubBackupInfra) DatabasePath() string                { return s.dbPath }
func (s *stubBackupInfra) BackupsDir() string                   { return s.backupsDir }
func (s *stubBackupInfra) ProductVersion() string               { return s.productVers }
func (s *stubBackupInfra) ReopenDatabase() error {
	s.reopenCalled = true
	if s.reopenDBFn != nil {
		return s.reopenDBFn()
	}
	return s.reopenErr
}
func (s *stubBackupInfra) ReloadNotificationConfig(ctx context.Context) error {
	s.reloadCalled = true
	return s.reloadErr
}

// setupBackupTestDB creates a temp SQLite database with the system_settings
// table and a few sample rows.
func setupBackupTestDB(t *testing.T, dbPath string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS system_settings (
		id TEXT PRIMARY KEY,
		key TEXT NOT NULL UNIQUE,
		value_json TEXT NOT NULL,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`)
	if err != nil {
		db.Close()
		t.Fatalf("create table: %v", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for i, key := range []string{"app.version_check_url", "app.theme", "proxy.enabled"} {
		val, _ := json.Marshal(fmt.Sprintf("value-%d", i))
		_, err = db.Exec(`INSERT INTO system_settings (id, key, value_json, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?)`,
			fmt.Sprintf("id-%d", i), key, string(val), now, now)
		if err != nil {
			db.Close()
			t.Fatalf("insert setting: %v", err)
		}
	}
	return db
}

func TestListExports(t *testing.T) {
	tests := []struct {
		name       string
		setupFiles func(dir string) // create files in the backups dir
		wantCount  int
		wantNames  []string
	}{
		{
			name:       "empty_directory",
			setupFiles: func(dir string) {},
			wantCount:  0,
		},
		{
			name: "single_rczip_file",
			setupFiles: func(dir string) {
				os.WriteFile(filepath.Join(dir, "export-20260101-120000.rczip"), []byte("data"), 0o600)
			},
			wantCount: 1,
			wantNames: []string{"export-20260101-120000.rczip"},
		},
		{
			name: "multiple_rczip_files",
			setupFiles: func(dir string) {
				os.WriteFile(filepath.Join(dir, "export-20260101-120000.rczip"), []byte("a"), 0o600)
				os.WriteFile(filepath.Join(dir, "export-20260102-120000.rczip"), []byte("bb"), 0o600)
			},
			wantCount: 2,
		},
		{
			name: "ignores_non_rczip_files",
			setupFiles: func(dir string) {
				os.WriteFile(filepath.Join(dir, "export-20260101-120000.rczip"), []byte("data"), 0o600)
				os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("ignore"), 0o600)
				os.WriteFile(filepath.Join(dir, "data.json"), []byte("{}"), 0o600)
			},
			wantCount: 1,
		},
		{
			name: "case_insensitive_extension",
			setupFiles: func(dir string) {
				os.WriteFile(filepath.Join(dir, "export-20260101-120000.RCZIP"), []byte("data"), 0o600)
			},
			wantCount: 1,
		},
		{
			name: "ignores_directories",
			setupFiles: func(dir string) {
				os.MkdirAll(filepath.Join(dir, "subdir.rczip"), 0o700)
				os.WriteFile(filepath.Join(dir, "export-20260101-120000.rczip"), []byte("data"), 0o600)
			},
			wantCount: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			backupsDir := t.TempDir()
			tc.setupFiles(backupsDir)

			infra := &stubBackupInfra{backupsDir: backupsDir}
			svc := NewService(infra)

			results, err := svc.ListExports()
			if err != nil {
				t.Fatalf("ListExports error: %v", err)
			}
			if len(results) != tc.wantCount {
				t.Errorf("got %d results, want %d", len(results), tc.wantCount)
			}
			if tc.wantNames != nil && len(results) > 0 {
				found := false
				for _, r := range results {
					if r.FileName == tc.wantNames[0] {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected file %q in results, not found", tc.wantNames[0])
				}
			}
		})
	}
}

func TestListExports_ReportsSize(t *testing.T) {
	backupsDir := t.TempDir()
	content := make([]byte, 1024)
	for i := range content {
		content[i] = byte(i % 256)
	}
	os.WriteFile(filepath.Join(backupsDir, "export-20260101-120000.rczip"), content, 0o600)

	infra := &stubBackupInfra{backupsDir: backupsDir}
	svc := NewService(infra)

	results, err := svc.ListExports()
	if err != nil {
		t.Fatalf("ListExports error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].SizeBytes != 1024 {
		t.Errorf("SizeBytes = %d, want 1024", results[0].SizeBytes)
	}
}

func TestListExports_CreatesMissingDir(t *testing.T) {
	parentDir := t.TempDir()
	missingDir := filepath.Join(parentDir, "nested", "backups")

	infra := &stubBackupInfra{backupsDir: missingDir}
	svc := NewService(infra)

	results, err := svc.ListExports()
	if err != nil {
		t.Fatalf("ListExports error: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results for empty dir, got %v", results)
	}
	if _, err := os.Stat(missingDir); os.IsNotExist(err) {
		t.Error("ListExports should create the backups directory if missing")
	}
}

func TestCreateEncryptedExport(t *testing.T) {
	backupsDir := t.TempDir()
	dbDir := t.TempDir()
	dbPath := filepath.Join(dbDir, "relaycheck.db")

	db := setupBackupTestDB(t, dbPath)
	defer db.Close()

	infra := &stubBackupInfra{
		db:          db,
		dbPath:      dbPath,
		backupsDir:  backupsDir,
		productVers: "1.0.0-test",
	}
	svc := NewService(infra)

	result, err := svc.CreateEncryptedExport(context.Background(), "test-password")
	if err != nil {
		t.Fatalf("CreateEncryptedExport error: %v", err)
	}

	// Verify result metadata.
	if result.FileName == "" {
		t.Error("FileName should not be empty")
	}
	if !endsWith(result.FileName, ".rczip") {
		t.Errorf("FileName should end with .rczip, got %q", result.FileName)
	}
	if result.SizeBytes <= 0 {
		t.Errorf("SizeBytes should be positive, got %d", result.SizeBytes)
	}
	if result.Manifest.Version != "2" {
		t.Errorf("Manifest.Version = %q, want %q", result.Manifest.Version, "2")
	}
	if result.Manifest.ProductVersion != "1.0.0-test" {
		t.Errorf("Manifest.ProductVersion = %q, want %q", result.Manifest.ProductVersion, "1.0.0-test")
	}
	if !result.Manifest.Includes.Database {
		t.Error("Manifest.Includes.Database should be true")
	}
	if !result.Manifest.Includes.Settings {
		t.Error("Manifest.Includes.Settings should be true")
	}
	if result.Manifest.SettingCount != 3 {
		t.Errorf("Manifest.SettingCount = %d, want 3", result.Manifest.SettingCount)
	}
	if result.Manifest.DatabaseSize <= 0 {
		t.Errorf("Manifest.DatabaseSize should be positive, got %d", result.Manifest.DatabaseSize)
	}

	// Verify the file was written.
	filePath := filepath.Join(backupsDir, result.FileName)
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("export file not found: %v", err)
	}
	if info.Size() != result.SizeBytes {
		t.Errorf("file size on disk %d != result SizeBytes %d", info.Size(), result.SizeBytes)
	}

	// Verify the file starts with RCZIP2 magic.
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read export file: %v", err)
	}
	if string(data[:len(RCZIPMagic)]) != RCZIPMagic {
		t.Errorf("file does not start with RCZIP2 magic, got %q", data[:len(RCZIPMagic)])
	}

	// Verify it can be decrypted with the same password.
	decrypted, err := DecryptWithPassword(data, "test-password")
	if err != nil {
		t.Fatalf("decrypt exported file: %v", err)
	}
	_ = decrypted // just verifying the round-trip encryption works
}

func TestCreateEncryptedExport_WrongPasswordCantDecrypt(t *testing.T) {
	backupsDir := t.TempDir()
	dbDir := t.TempDir()
	dbPath := filepath.Join(dbDir, "relaycheck.db")

	db := setupBackupTestDB(t, dbPath)
	defer db.Close()

	infra := &stubBackupInfra{
		db:         db,
		dbPath:     dbPath,
		backupsDir: backupsDir,
	}
	svc := NewService(infra)

	result, err := svc.CreateEncryptedExport(context.Background(), "correct-password")
	if err != nil {
		t.Fatalf("CreateEncryptedExport error: %v", err)
	}

	filePath := filepath.Join(backupsDir, result.FileName)
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read export file: %v", err)
	}

	if _, err := DecryptWithPassword(data, "wrong-password"); err == nil {
		t.Error("expected decrypt to fail with wrong password, got nil error")
	}
}

func TestCreateEncryptedExport_DBFileMissing(t *testing.T) {
	backupsDir := t.TempDir()

	infra := &stubBackupInfra{
		db:         nil,
		dbPath:     filepath.Join(backupsDir, "nonexistent.db"),
		backupsDir: backupsDir,
	}
	svc := NewService(infra)

	_, err := svc.CreateEncryptedExport(context.Background(), "password")
	if err == nil {
		t.Error("expected error when database file does not exist, got nil")
	}
}

func TestRestoreEncryptedExport_RoundTrip(t *testing.T) {
	backupsDir := t.TempDir()
	dbDir := t.TempDir()
	dbPath := filepath.Join(dbDir, "relaycheck.db")

	db := setupBackupTestDB(t, dbPath)

	infra := &stubBackupInfra{
		db:         db,
		dbPath:     dbPath,
		backupsDir: backupsDir,
	}
	// On Windows, the SQLite file is locked by the open *sql.DB handle.
	// ReopenDatabase must close the old handle and open a new one so that
	// the restore process can os.Rename the .pre-import-bak file back.
	infra.reopenDBFn = func() error {
		if infra.db != nil {
			infra.db.Close()
		}
		newDB, err := sql.Open("sqlite", infra.dbPath)
		if err != nil {
			return err
		}
		infra.db = newDB
		return nil
	}
	// Close whatever handle is active when the test finishes.
	t.Cleanup(func() {
		if infra.db != nil {
			infra.db.Close()
		}
	})

	svc := NewService(infra)

	// Create an export.
	result, err := svc.CreateEncryptedExport(context.Background(), "round-trip-pass")
	if err != nil {
		t.Fatalf("CreateEncryptedExport error: %v", err)
	}

	// Read original database file content before import.
	originalDB, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read original db: %v", err)
	}

	// Close the DB handle before restoring. RestoreEncryptedExport needs to
	// os.Rename the file (Windows lock), and it only calls ReopenDatabase
	// *after* the rename — so the lock must already be released.
	infra.db.Close()
	infra.db = nil

	// Import (restore) it back. ReopenDatabase is called by the service and
	// will open a fresh handle via reopenDBFn.
	exportPath := filepath.Join(backupsDir, result.FileName)
	manifest, err := svc.RestoreEncryptedExport(context.Background(), exportPath, "round-trip-pass")
	if err != nil {
		t.Fatalf("RestoreEncryptedExport error: %v", err)
	}

	// Verify manifest fields.
	if manifest.Version != "2" {
		t.Errorf("manifest version = %q, want %q", manifest.Version, "2")
	}
	if !manifest.Includes.Database {
		t.Error("manifest should indicate database included")
	}
	if !manifest.Includes.Settings {
		t.Error("manifest should indicate settings included")
	}

	// Verify ReopenDatabase was called.
	if !infra.reopenCalled {
		t.Error("ReopenDatabase should have been called during import")
	}
	// Verify ReloadNotificationConfig was called.
	if !infra.reloadCalled {
		t.Error("ReloadNotificationConfig should have been called during import")
	}

	// Verify the database file was written back with identical content.
	restoredDB, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read restored db: %v", err)
	}
	if string(restoredDB) != string(originalDB) {
		t.Error("restored database content does not match original")
	}
}

func TestRestoreEncryptedExport_WrongPassword(t *testing.T) {
	backupsDir := t.TempDir()
	dbDir := t.TempDir()
	dbPath := filepath.Join(dbDir, "relaycheck.db")

	db := setupBackupTestDB(t, dbPath)
	defer db.Close()

	infra := &stubBackupInfra{
		db:         db,
		dbPath:     dbPath,
		backupsDir: backupsDir,
	}
	svc := NewService(infra)

	result, err := svc.CreateEncryptedExport(context.Background(), "correct-pass")
	if err != nil {
		t.Fatalf("CreateEncryptedExport error: %v", err)
	}

	exportPath := filepath.Join(backupsDir, result.FileName)
	_, err = svc.RestoreEncryptedExport(context.Background(), exportPath, "wrong-pass")
	if err == nil {
		t.Error("expected error for wrong password during import, got nil")
	}
}

func TestRestoreEncryptedExport_FileNotFound(t *testing.T) {
	infra := &stubBackupInfra{
		dbPath: filepath.Join(t.TempDir(), "nonexistent.db"),
	}
	svc := NewService(infra)

	_, err := svc.RestoreEncryptedExport(context.Background(), "/nonexistent/path.rczip", "password")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestRestoreEncryptedExport_InvalidFormat(t *testing.T) {
	tmpDir := t.TempDir()
	badFile := filepath.Join(tmpDir, "bad.rczip")
	os.WriteFile(badFile, []byte("not-a-valid-rczip-file"), 0o600)

	infra := &stubBackupInfra{
		dbPath: filepath.Join(tmpDir, "relaycheck.db"),
	}
	svc := NewService(infra)

	_, err := svc.RestoreEncryptedExport(context.Background(), badFile, "password")
	if err == nil {
		t.Error("expected error for invalid file format, got nil")
	}
}

func TestRestoreEncryptedExport_ReopenFails_RollsBack(t *testing.T) {
	backupsDir := t.TempDir()
	dbDir := t.TempDir()
	dbPath := filepath.Join(dbDir, "relaycheck.db")

	db := setupBackupTestDB(t, dbPath)

	infra := &stubBackupInfra{
		db:         db,
		dbPath:     dbPath,
		backupsDir: backupsDir,
	}
	// Use a reopen function that actually closes the DB (releasing the Windows
	// file lock) but then returns the simulated error. This lets os.Rename
	// succeed while still testing the error-handling path.
	infra.reopenDBFn = func() error {
		if infra.db != nil {
			infra.db.Close()
			infra.db = nil
		}
		return fmt.Errorf("simulated reopen failure")
	}
	t.Cleanup(func() {
		if infra.db != nil {
			infra.db.Close()
		}
	})

	svc := NewService(infra)

	// Create a valid export.
	result, err := svc.CreateEncryptedExport(context.Background(), "password")
	if err != nil {
		t.Fatalf("CreateEncryptedExport error: %v", err)
	}

	// Read original database before we break ReopenDatabase.
	originalDB, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read original db: %v", err)
	}

	// Close the DB handle before restoring so os.Rename doesn't hit a
	// Windows file lock. ReopenDatabase is called *after* the rename,
	// so the lock must already be released.
	infra.db.Close()
	infra.db = nil

	exportPath := filepath.Join(backupsDir, result.FileName)
	_, err = svc.RestoreEncryptedExport(context.Background(), exportPath, "password")
	if err == nil {
		t.Error("expected error when ReopenDatabase fails, got nil")
	}

	// Verify that the rollback restored the original database content.
	rolledBackDB, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read db after rollback: %v", err)
	}
	if string(rolledBackDB) != string(originalDB) {
		t.Error("database should have been rolled back to original content after ReopenDatabase failure")
	}
}

// endsWith checks whether s ends with suffix (simple replacement for
// strings.HasSuffix to avoid importing strings in test for just one call).
func endsWith(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}
