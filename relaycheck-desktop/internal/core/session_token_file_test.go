package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteSessionTokenFilePersistsAndSecuresToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session-token.txt")
	const token = "test-session-token"
	if err := WriteSessionTokenFile(path, token); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != token {
		t.Fatalf("token body = %q, want %q", body, token)
	}
	if err := verifySessionTokenFileSecurity(path); err != nil {
		t.Fatalf("session token file security verification failed: %v", err)
	}
}

func TestWriteSessionTokenFileRejectsEmptyToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session-token.txt")
	if err := WriteSessionTokenFile(path, ""); err == nil {
		t.Fatal("expected empty token to be rejected")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("empty token must not create a file, stat err=%v", err)
	}
}
