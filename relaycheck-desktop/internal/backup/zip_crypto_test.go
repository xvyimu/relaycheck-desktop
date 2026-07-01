package backup

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"strings"
	"testing"
)

// rczip1LegacyEncrypt builds an RCZIP1 payload (raw SHA-256 key, no salt) so
// tests can verify DecryptRCZIP1Legacy without an on-disk fixture. This mirrors
// the pre-PBKDF2 encryption path that was removed from production code.
func rczip1LegacyEncrypt(data []byte, password string) ([]byte, error) {
	hash := sha256.Sum256([]byte(password))
	block, err := aes.NewCipher(hash[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	for i := range nonce { // deterministic nonce for reproducible legacy tests
		nonce[i] = byte(i)
	}
	ciphertext := gcm.Seal(nil, nonce, data, nil)
	result := make([]byte, 0, len("RCZIP1")+len(nonce)+len(ciphertext))
	result = append(result, []byte("RCZIP1")...)
	result = append(result, nonce...)
	result = append(result, ciphertext...)
	return result, nil
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	cases := []struct {
		name     string
		data     []byte
		password string
	}{
		{"empty", []byte{}, "secret"},
		{"small", []byte("hello world"), "p@ssw0rd"},
		{"large", make([]byte, 4096), "long-password-123"},
		{"unicode-pwd", []byte("data"), "密码密码"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			encrypted, err := EncryptWithPassword(tc.data, tc.password)
			if err != nil {
				t.Fatalf("encrypt failed: %v", err)
			}
			if len(encrypted) < len(RCZIPMagic)+rczipSaltLen+12 {
				t.Fatalf("encrypted payload too short: %d bytes", len(encrypted))
			}
			if string(encrypted[:len(RCZIPMagic)]) != RCZIPMagic {
				t.Fatalf("missing RCZIP2 magic header, got %q", encrypted[:len(RCZIPMagic)])
			}
			decrypted, err := DecryptWithPassword(encrypted, tc.password)
			if err != nil {
				t.Fatalf("decrypt failed: %v", err)
			}
			if string(decrypted) != string(tc.data) {
				t.Fatalf("round-trip mismatch: got %q, want %q", decrypted, tc.data)
			}
		})
	}
}

func TestDecryptWrongPasswordFails(t *testing.T) {
	encrypted, err := EncryptWithPassword([]byte("secret data"), "correct-password")
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}
	if _, err := DecryptWithPassword(encrypted, "wrong-password"); err == nil {
		t.Fatal("expected GCM auth failure for wrong password, got nil")
	}
}

func TestDecryptTamperedCiphertextFails(t *testing.T) {
	encrypted, err := EncryptWithPassword([]byte("secret data"), "password")
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}
	// Flip a byte near the end (ciphertext region).
	tampered := make([]byte, len(encrypted))
	copy(tampered, encrypted)
	tampered[len(tampered)-1] ^= 0xFF
	if _, err := DecryptWithPassword(tampered, "password"); err == nil {
		t.Fatal("expected GCM auth failure for tampered ciphertext, got nil")
	}
}

func TestDecryptTamperedSaltFails(t *testing.T) {
	encrypted, err := EncryptWithPassword([]byte("secret data"), "password")
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}
	// Flip a byte in the salt region (right after magic).
	tampered := make([]byte, len(encrypted))
	copy(tampered, encrypted)
	tampered[len(RCZIPMagic)] ^= 0xFF
	if _, err := DecryptWithPassword(tampered, "password"); err == nil {
		t.Fatal("expected failure for tampered salt, got nil")
	}
}

func TestDecryptWithPassword_InvalidInputs(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"too-short", []byte("RCZ")},
		{"bad-magic", []byte("XXXXXX" + strings.Repeat("x", 60))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := DecryptWithPassword(tc.data, "password"); err == nil {
				t.Fatal("expected error for invalid input, got nil")
			}
		})
	}
}

func TestDecryptRCZIP2_MalformedPayload(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"missing-salt", []byte("RCZIP2")},
		{"missing-nonce", append([]byte("RCZIP2"), make([]byte, rczipSaltLen)...),},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := DecryptRCZIP2(tc.data, "password"); err == nil {
				t.Fatal("expected error for malformed RCZIP2 payload, got nil")
			}
		})
	}
}

func TestRCZIP1LegacyRoundTrip(t *testing.T) {
	data := []byte("legacy payload")
	encrypted, err := rczip1LegacyEncrypt(data, "legacy-password")
	if err != nil {
		t.Fatalf("legacy encrypt failed: %v", err)
	}
	if string(encrypted[:len("RCZIP1")]) != "RCZIP1" {
		t.Fatalf("missing RCZIP1 magic header")
	}
	// DecryptWithPassword dispatches RCZIP1 to DecryptRCZIP1Legacy.
	decrypted, err := DecryptWithPassword(encrypted, "legacy-password")
	if err != nil {
		t.Fatalf("legacy decrypt failed: %v", err)
	}
	if string(decrypted) != string(data) {
		t.Fatalf("legacy round-trip mismatch: got %q, want %q", decrypted, data)
	}
}

func TestRCZIP1LegacyWrongPasswordFails(t *testing.T) {
	encrypted, err := rczip1LegacyEncrypt([]byte("legacy payload"), "correct")
	if err != nil {
		t.Fatalf("legacy encrypt failed: %v", err)
	}
	if _, err := DecryptRCZIP1Legacy(encrypted, "wrong"); err == nil {
		t.Fatal("expected GCM auth failure for wrong legacy password, got nil")
	}
}

func TestPBKDF2SHA256(t *testing.T) {
	salt := []byte("fixed-salt-for-determinism")
	// Determinism: same inputs produce same output.
	key1 := PBKDF2SHA256([]byte("password"), salt, 1000, 32)
	key2 := PBKDF2SHA256([]byte("password"), salt, 1000, 32)
	if string(key1) != string(key2) {
		t.Fatal("PBKDF2 should be deterministic for identical inputs")
	}
	// Length honoured.
	if len(key1) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(key1))
	}
	// Different password → different key.
	key3 := PBKDF2SHA256([]byte("different"), salt, 1000, 32)
	if string(key1) == string(key3) {
		t.Fatal("different passwords should produce different keys")
	}
	// Different salt → different key.
	key4 := PBKDF2SHA256([]byte("password"), []byte("other-salt"), 1000, 32)
	if string(key1) == string(key4) {
		t.Fatal("different salts should produce different keys")
	}
	// Custom key length.
	shortKey := PBKDF2SHA256([]byte("password"), salt, 1000, 16)
	if len(shortKey) != 16 {
		t.Fatalf("expected 16-byte key, got %d", len(shortKey))
	}
	longKey := PBKDF2SHA256([]byte("password"), salt, 1000, 64)
	if len(longKey) != 64 {
		t.Fatalf("expected 64-byte key, got %d", len(longKey))
	}
}

func TestEncryptProducesDifferentCiphertexts(t *testing.T) {
	// Random salt + nonce means two encryptions of the same data must differ.
	data := []byte("identical payload")
	enc1, _ := EncryptWithPassword(data, "password")
	enc2, _ := EncryptWithPassword(data, "password")
	if string(enc1) == string(enc2) {
		t.Fatal("two encryptions of identical data must differ (random salt+nonce)")
	}
	// Both must decrypt back to the original.
	dec1, _ := DecryptWithPassword(enc1, "password")
	dec2, _ := DecryptWithPassword(enc2, "password")
	if string(dec1) != string(data) || string(dec2) != string(data) {
		t.Fatal("both ciphertexts must round-trip to the original data")
	}
}
