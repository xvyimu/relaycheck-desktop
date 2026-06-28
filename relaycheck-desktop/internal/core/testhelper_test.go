package core

import (
	"os"
	"runtime"
	"testing"
	"time"
)

// newTestApp creates a *App backed by an OS temp directory and registers cleanup
// with retry on Windows to avoid the well-known unlinkat race after SQLite.Close().
func newTestApp(t testing.TB) *App {
	t.Helper()
	dir, err := os.MkdirTemp("", "rc-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	app, err := NewApp(dir)
	if err != nil {
		os.RemoveAll(dir)
		t.Fatalf("NewApp: %v", err)
	}
	setupTestCleanup(t, app, dir)
	return app
}

// newTestAppWithDir creates a *App using a caller-supplied temp dir (needed when
// tests also create files in the same dir before NewApp). Registers cleanup
// with retry on Windows.
func newTestAppWithDir(t testing.TB, dir string) *App {
	t.Helper()
	app, err := NewApp(dir)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	setupTestCleanup(t, app, dir)
	return app
}

func setupTestCleanup(t testing.TB, app *App, dir string) {
	t.Cleanup(func() {
		app.Close()
		// On Windows, SQLite may release file handles asynchronously.
		// Retry os.RemoveAll with backoff so TempDir cleanup never flakes.
		if runtime.GOOS == "windows" {
			for i := 0; i < 5; i++ {
				if err := os.RemoveAll(dir); err == nil {
					return
				}
				time.Sleep(time.Duration(25*(i+1)) * time.Millisecond)
			}
		}
	})
}
