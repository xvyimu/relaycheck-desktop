package core

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type failingReader struct{}

func (failingReader) Read(_ []byte) (int, error) {
	return 0, errors.New("random source unavailable")
}

func TestNewIDFallsBackWhenRandomSourceFails(t *testing.T) {
	first := newIDFromReader(failingReader{})
	second := newIDFromReader(failingReader{})

	if len(first) != 32 || len(second) != 32 {
		t.Fatalf("expected 32-character IDs, got %q and %q", first, second)
	}
	if first == second {
		t.Fatalf("expected fallback IDs to be unique, got %q twice", first)
	}
}

func TestNewIDUsesProvidedRandomBytes(t *testing.T) {
	id := newIDFromReader(io.LimitReader(zeroReader{}, 16))

	if id != "00000000000000000000000000000000" {
		t.Fatalf("unexpected deterministic ID: %s", id)
	}
}

func TestBootstrapAdminPasswordUsesEnvironment(t *testing.T) {
	t.Setenv("RELAYCHECK_BOOTSTRAP_PASSWORD", "local-secret")
	dir := t.TempDir()
	app := &App{dataDir: dir}

	password, err := app.bootstrapAdminPassword()
	if err != nil {
		t.Fatal(err)
	}
	if password != "local-secret" {
		t.Fatalf("expected environment password, got %q", password)
	}
	if _, err := os.Stat(filepath.Join(dir, "bootstrap-admin-password.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected no generated bootstrap file, got err=%v", err)
	}
}

func TestBootstrapAdminPasswordPersistsGeneratedPassword(t *testing.T) {
	t.Setenv("RELAYCHECK_BOOTSTRAP_PASSWORD", "")
	dir := t.TempDir()
	app := &App{dataDir: dir}

	first, err := app.bootstrapAdminPassword()
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 64 {
		t.Fatalf("expected 64-character bootstrap password, got %q", first)
	}

	path := filepath.Join(dir, "bootstrap-admin-password.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != first {
		t.Fatalf("expected generated password in bootstrap file")
	}

	second, err := app.bootstrapAdminPassword()
	if err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Fatalf("expected persisted bootstrap password, got %q then %q", first, second)
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}
