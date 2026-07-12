package accounts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveAllowedSQLiteImportPathRejectsWindowsSystem(t *testing.T) {
	// C:\Windows is outside NewAPI roots / home / cwd in normal setups.
	_, err := resolveAllowedSQLiteImportPath(`C:\Windows\System32\config\SAM`)
	if err == nil {
		t.Fatal("expected Windows system path to be rejected")
	}
	if !strings.Contains(err.Error(), "允许") && !strings.Contains(err.Error(), "拒绝") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveAllowedSQLiteImportPathAcceptsCWD(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	candidate := filepath.Join(cwd, "one-api.db")
	got, err := resolveAllowedSQLiteImportPath(candidate)
	if err != nil {
		t.Fatalf("cwd path should be allowed: %v", err)
	}
	if filepath.Base(got) != "one-api.db" {
		t.Fatalf("unexpected path %q", got)
	}
}

func TestPathUnderAllowedSQLiteRoots(t *testing.T) {
	if pathUnderAllowedSQLiteRoots(`C:\Windows\explorer.exe`) {
		t.Fatal("Windows path should not be under allowed roots")
	}
}
