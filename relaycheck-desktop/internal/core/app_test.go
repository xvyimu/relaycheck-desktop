package core

import (
	"bytes"
	"errors"
	"io"
	"log"
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

func TestAppCloseIsIdempotent(t *testing.T) {
	app := newTestApp(t)
	var logs bytes.Buffer
	previousWriter := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previousWriter) })

	if err := app.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := app.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if strings.Contains(logs.String(), "wal checkpoint failed") {
		t.Fatalf("second Close attempted another checkpoint: %s", logs.String())
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}
