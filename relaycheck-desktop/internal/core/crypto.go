package core

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func loadOrCreateKey(path string) ([]byte, error) {
	if content, err := os.ReadFile(path); err == nil {
		if err := secureInstanceKeyFile(path); err != nil {
			return nil, fmt.Errorf("secure instance key file: %w", err)
		}
		if err := verifyInstanceKeyFileSecurity(path); err != nil {
			return nil, fmt.Errorf("verify instance key file security: %w", err)
		}
		return base64.StdEncoding.DecodeString(strings.TrimSpace(string(content)))
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, []byte(base64.StdEncoding.EncodeToString(key)), 0o600); err != nil {
		return nil, err
	}
	if err := secureInstanceKeyFile(path); err != nil {
		_ = os.Remove(path)
		return nil, fmt.Errorf("secure instance key file: %w", err)
	}
	if err := verifyInstanceKeyFileSecurity(path); err != nil {
		_ = os.Remove(path)
		return nil, fmt.Errorf("verify instance key file security: %w", err)
	}
	return key, nil
}

// encryptText delegates to a.crypto.Encrypt. Kept as an *App method so the
// 70+ existing call sites need no changes; new code should use a.crypto.Encrypt
// directly or inject *CryptoService.
func (a *App) encryptText(value string) (string, error) {
	return a.crypto.Encrypt(value)
}

// decryptText delegates to a.crypto.Decrypt. See encryptText for rationale.
func (a *App) decryptText(value string) (string, error) {
	return a.crypto.Decrypt(value)
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
