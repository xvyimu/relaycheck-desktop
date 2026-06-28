package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBackupPathRejectsPathTraversal(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	if _, err := app.backupPath(`..\relaycheck.db`); err == nil {
		t.Fatal("expected traversal path to be rejected")
	}
	if _, err := app.backupPath(`relaycheck-test.txt`); err == nil {
		t.Fatal("expected non-db file to be rejected")
	}
}

func TestCreateAndDeleteBackupFile(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	backup, err := app.createBackup("test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(backup.Path); err != nil {
		t.Fatalf("expected backup to exist: %v", err)
	}
	backupPath, err := app.backupPath(backup.FileName)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(backupPath) != app.backupsDir() {
		t.Fatalf("expected backup to stay inside backups dir, got %s", backupPath)
	}
	if err := os.Remove(backupPath); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Fatalf("expected backup to be deleted, got %v", err)
	}
}
