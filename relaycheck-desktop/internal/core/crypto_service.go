package core

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"strings"
)

// CryptoService encapsulates AES-GCM encryption/decryption for sensitive
// credential fields. It is a pure service: it holds only the instance key and
// has no dependency on *App state, so it can be unit-tested in isolation and
// reused by future domain packages without depending on the god object.
//
// The wire format is "v1.<base64-nonce>.<base64-ciphertext>" and is preserved
// exactly from the original *App.encryptText / *App.decryptText methods.
type CryptoService struct {
	key []byte
}

// NewCryptoService creates a CryptoService using the given 32-byte key.
func NewCryptoService(key []byte) *CryptoService {
	return &CryptoService{key: key}
}

// Encrypt encrypts value using AES-256-GCM. Empty input returns empty string
// with no error. The output format is "v1.<nonce>.<ciphertext>" (base64).
func (c *CryptoService) Encrypt(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	cipherText := gcm.Seal(nil, nonce, []byte(value), nil)
	return "v1." + base64.StdEncoding.EncodeToString(nonce) + "." + base64.StdEncoding.EncodeToString(cipherText), nil
}

// Decrypt decrypts a "v1.<nonce>.<ciphertext>" string. Empty input returns
// empty string. Strings that do not match the v1 format return empty string
// and nil error (same behavior as the original decryptText).
func (c *CryptoService) Decrypt(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	parts := strings.Split(value, ".")
	if len(parts) != 3 || parts[0] != "v1" {
		return "", nil
	}
	nonce, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", err
	}
	cipherText, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	// Guard against malformed input that would panic in gcm.Open: the nonce
	// must be exactly gcm.NonceSize() bytes, and empty ciphertext is invalid.
	if len(nonce) != gcm.NonceSize() || len(cipherText) < gcm.Overhead() {
		return "", nil
	}
	plainText, err := gcm.Open(nil, nonce, cipherText, nil)
	if err != nil {
		return "", err
	}
	return string(plainText), nil
}
