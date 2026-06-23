package core

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

func loadOrCreateKey(path string) ([]byte, error) {
	if content, err := os.ReadFile(path); err == nil {
		return base64.StdEncoding.DecodeString(strings.TrimSpace(string(content)))
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	return key, os.WriteFile(path, []byte(base64.StdEncoding.EncodeToString(key)), 0o600)
}

func (a *App) encryptText(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	block, err := aes.NewCipher(a.key)
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

func (a *App) decryptText(value string) (string, error) {
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
	block, err := aes.NewCipher(a.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	plainText, err := gcm.Open(nil, nonce, cipherText, nil)
	if err != nil {
		return "", err
	}
	return string(plainText), nil
}

func maskSecret(value string) string {
	if value == "" {
		return ""
	}
	if len(value) <= 4 {
		return strings.Repeat("*", len(value))
	}
	return strings.Repeat("*", max(4, len(value)-4)) + value[len(value)-4:]
}

func secretFingerprint(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(trimmed))
	return "key_" + hex.EncodeToString(sum[:])[:12]
}
