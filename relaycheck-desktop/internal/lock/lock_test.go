package lock_test

import (
	"os"
	"path/filepath"
	"testing"

	"relaycheck-desktop/internal/lock"
)

func TestAcquire_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".lock")

	f, err := lock.Acquire(path)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	f.Close()
}

func TestAcquire_RejectsSecond(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".lock")

	f1, err := lock.Acquire(path)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	defer f1.Close()

	_, err = lock.Acquire(path)
	if err != lock.ErrAlreadyLocked {
		t.Fatalf("expected ErrAlreadyLocked, got: %v", err)
	}
}

func TestAcquire_ReleasesOnClose(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".lock")

	f1, err := lock.Acquire(path)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	f1.Close() // release

	f2, err := lock.Acquire(path)
	if err != nil {
		t.Fatalf("second acquire after close: %v", err)
	}
	f2.Close()
}

func TestAcquire_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".lock")

	f, err := lock.Acquire(path)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	f.Close()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("lockfile was not created")
	}
}
