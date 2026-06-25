package core

import (
	"errors"
	"io"
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

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}
