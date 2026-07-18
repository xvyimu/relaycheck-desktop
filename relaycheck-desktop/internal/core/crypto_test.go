package core

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOrCreateKeyPersistsAndSecuresInstanceKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys", "instance.key")

	created, err := loadOrCreateKey(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 32 {
		t.Fatalf("created key length = %d, want 32", len(created))
	}
	if err := verifyInstanceKeyFileSecurity(path); err != nil {
		t.Fatalf("instance key security verification failed: %v", err)
	}

	loaded, err := loadOrCreateKey(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(loaded, created) {
		t.Fatal("loadOrCreateKey changed an existing instance key")
	}
	if info, err := os.Stat(path); err != nil || info.Size() == 0 {
		t.Fatalf("expected a non-empty key file, info=%v err=%v", info, err)
	}
}
