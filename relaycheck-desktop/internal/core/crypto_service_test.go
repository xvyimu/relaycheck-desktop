package core

import (
	"strings"
	"testing"
)

// TestCryptoService_RoundTrip verifies that Encrypt followed by Decrypt
// recovers the original plaintext for non-empty input.
func TestCryptoService_RoundTrip(t *testing.T) {
	svc := NewCryptoService(make([]byte, 32)) // 32-byte AES-256 key
	cases := []string{
		"hello",
		"sk-1234567890abcdef",
		"session=abc; path=/; HttpOnly",
		strings.Repeat("x", 4096),
		"中文凭证",
	}
	for _, plain := range cases {
		encrypted, err := svc.Encrypt(plain)
		if err != nil {
			t.Fatalf("Encrypt(%q) error: %v", plain, err)
		}
		if encrypted == plain {
			t.Fatalf("Encrypt(%q) returned plaintext unchanged", plain)
		}
		decrypted, err := svc.Decrypt(encrypted)
		if err != nil {
			t.Fatalf("Decrypt error: %v", err)
		}
		if decrypted != plain {
			t.Fatalf("Round-trip mismatch: got %q want %q", decrypted, plain)
		}
	}
}

// TestCryptoService_EmptyInput verifies that empty/whitespace input is
// treated as no-op (returns empty string, no error, no encryption).
func TestCryptoService_EmptyInput(t *testing.T) {
	svc := NewCryptoService(make([]byte, 32))
	for _, input := range []string{"", "   ", "\t\n"} {
		encrypted, err := svc.Encrypt(input)
		if err != nil {
			t.Fatalf("Encrypt(%q) error: %v", input, err)
		}
		if encrypted != "" {
			t.Fatalf("Encrypt(%q) = %q, want empty", input, encrypted)
		}
		decrypted, err := svc.Decrypt("")
		if err != nil {
			t.Fatalf("Decrypt(\"\") error: %v", err)
		}
		if decrypted != "" {
			t.Fatalf("Decrypt(\"\") = %q, want empty", decrypted)
		}
	}
}

// TestCryptoService_NonV1Format verifies that strings not matching the
// "v1.<nonce>.<ciphertext>" format return empty string + nil error
// (matching the original decryptText behavior for legacy data).
func TestCryptoService_NonV1Format(t *testing.T) {
	svc := NewCryptoService(make([]byte, 32))
	cases := []string{
		"plaintext",
		"v2.something.else",
		"v1.onlyonepart",
		"v1.",
		"v1..",
		"notbase64!@#$.notbase64",
	}
	for _, input := range cases {
		decrypted, err := svc.Decrypt(input)
		if err != nil {
			t.Fatalf("Decrypt(%q) error: %v (want nil)", input, err)
		}
		if decrypted != "" {
			t.Fatalf("Decrypt(%q) = %q, want empty", input, decrypted)
		}
	}
}

// TestCryptoService_CorruptedCiphertext verifies that tampered ciphertext
// returns an error (not silent empty string).
func TestCryptoService_CorruptedCiphertext(t *testing.T) {
	svc := NewCryptoService(make([]byte, 32))
	encrypted, err := svc.Encrypt("secret")
	if err != nil {
		t.Fatal(err)
	}
	// Tamper with the ciphertext portion (last base64 char flipped)
	tampered := encrypted[:len(encrypted)-1]
	if encrypted[len(encrypted)-1] == 'A' {
		tampered += "B"
	} else {
		tampered += "A"
	}
	if _, err := svc.Decrypt(tampered); err == nil {
		t.Fatal("Decrypt(tampered) returned nil error, want decryption failure")
	}
}

// TestCryptoService_WrongKey verifies that decrypting with a different key
// returns an error.
func TestCryptoService_WrongKey(t *testing.T) {
	key1 := make([]byte, 32)
	key1[0] = 1
	key2 := make([]byte, 32)
	key2[0] = 2

	svc1 := NewCryptoService(key1)
	svc2 := NewCryptoService(key2)

	encrypted, err := svc1.Encrypt("secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc2.Decrypt(encrypted); err == nil {
		t.Fatal("Decrypt with wrong key returned nil error, want failure")
	}
}

// TestCryptoService_InvalidKeyLength verifies that AES rejects keys that
// are not 16, 24, or 32 bytes. (The project uses 32-byte AES-256 keys, but
// the underlying cipher accepts all three sizes; only other lengths fail.)
func TestCryptoService_InvalidKeyLength(t *testing.T) {
	cases := [][]byte{
		make([]byte, 0),
		make([]byte, 1),
		make([]byte, 15),
		make([]byte, 33),
	}
	for _, key := range cases {
		svc := NewCryptoService(key)
		if _, err := svc.Encrypt("test"); err == nil {
			t.Fatalf("Encrypt with %d-byte key returned nil error, want failure", len(key))
		}
	}
}

// TestCryptoService_AcceptedKeyLengths verifies that 16/24/32-byte keys
// all work (AES-128/192/256). The project uses 32-byte keys in production.
func TestCryptoService_AcceptedKeyLengths(t *testing.T) {
	for _, n := range []int{16, 24, 32} {
		svc := NewCryptoService(make([]byte, n))
		encrypted, err := svc.Encrypt("test")
		if err != nil {
			t.Fatalf("Encrypt with %d-byte key error: %v", n, err)
		}
		if _, err := svc.Decrypt(encrypted); err != nil {
			t.Fatalf("Decrypt with %d-byte key error: %v", n, err)
		}
	}
}

// TestCryptoService_EachEncryptionUnique verifies that encrypting the same
// plaintext twice produces different ciphertext (random nonce).
func TestCryptoService_EachEncryptionUnique(t *testing.T) {
	svc := NewCryptoService(make([]byte, 32))
	plain := "same-secret"
	a, err := svc.Encrypt(plain)
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.Encrypt(plain)
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("Two encryptions of same plaintext produced identical ciphertext; nonce may not be random")
	}
}
