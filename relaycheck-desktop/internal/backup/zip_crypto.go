package backup

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
)

// ExportManifest describes the contents of an encrypted export file.
type ExportManifest struct {
	Version        string `json:"version"`
	ExportedAt    string `json:"exportedAt"`
	ProductVersion string `json:"productVersion"`
	Includes      struct {
		Database  bool `json:"database"`
		Settings bool `json:"settings"`
	} `json:"includes"`
	DatabaseSize int64 `json:"databaseSize"`
	SettingCount int   `json:"settingCount"`
}

// ExportResult is the response for /api/system/export.
// Path is intentionally omitted to avoid leaking server filesystem paths.
type ExportResult struct {
	FileName  string         `json:"fileName"`
	SizeBytes int64          `json:"sizeBytes"`
	Manifest  ExportManifest `json:"manifest"`
}

// Setting is a key-value pair stored in the system_settings table. It mirrors
// the host application's setting row shape so that exported JSON remains
// stable across versions and the backup package does not need to import the
// host's model types.
type Setting struct {
	Key       string `json:"key"`
	ValueJSON string `json:"valueJson"`
	UpdatedAt string `json:"updatedAt"`
}

// RCZIP format constants.
const (
	// RCZIPMagic is the v2 magic header (PBKDF2 key derivation).
	RCZIPMagic      = "RCZIP2"
	rczipSaltLen    = 32
	rczipPBKDF2Iter = 200000
	// maxDecompressedSize limits total uncompressed zip content to 256 MB
	// to prevent zip-bomb denial-of-service.
	maxDecompressedSize = 256 * 1024 * 1024
	// maxSingleEntry limits any single zip entry to 200 MB.
	maxSingleEntry = 200 * 1024 * 1024
)

// EncryptWithPassword encrypts data using AES-256-GCM with a key derived from
// the password via PBKDF2-SHA256 (200,000 iterations) with a random 32-byte salt.
//
// File format (binary, no base64):
//
//	[6 bytes magic "RCZIP2"] [32 bytes salt] [12 bytes GCM nonce] [ciphertext+tag]
func EncryptWithPassword(data []byte, password string) ([]byte, error) {
	// Generate random salt
	salt := make([]byte, rczipSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("生成盐失败: %w", err)
	}

	// Derive key using PBKDF2-SHA256
	key := PBKDF2SHA256([]byte(password), salt, rczipPBKDF2Iter, 32)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	ciphertext := gcm.Seal(nil, nonce, data, nil)

	// Format: magic + salt + nonce + ciphertext
	result := make([]byte, 0, len(RCZIPMagic)+len(salt)+len(nonce)+len(ciphertext))
	result = append(result, []byte(RCZIPMagic)...)
	result = append(result, salt...)
	result = append(result, nonce...)
	result = append(result, ciphertext...)
	return result, nil
}

// DecryptWithPassword decrypts data encrypted by EncryptWithPassword.
// Supports both RCZIP2 (PBKDF2) and legacy RCZIP1 (raw SHA-256) for backward compatibility.
func DecryptWithPassword(data []byte, password string) ([]byte, error) {
	if len(data) < len(RCZIPMagic) {
		return nil, fmt.Errorf("文件过短，无效的导出文件")
	}
	magic := string(data[:len(RCZIPMagic)])

	switch magic {
	case "RCZIP2":
		return DecryptRCZIP2(data, password)
	case "RCZIP1":
		// Legacy format: magic(6) + nonce + ciphertext, raw SHA-256 key
		return DecryptRCZIP1Legacy(data, password)
	default:
		return nil, fmt.Errorf("无效的导出文件格式")
	}
}

// DecryptRCZIP2 decrypts a v2 RCZIP payload (PBKDF2-derived key).
func DecryptRCZIP2(data []byte, password string) ([]byte, error) {
	headerLen := len(RCZIPMagic) + rczipSaltLen
	if len(data) < headerLen {
		return nil, fmt.Errorf("文件损坏：缺少盐")
	}
	salt := data[len(RCZIPMagic):headerLen]
	rest := data[headerLen:]

	key := PBKDF2SHA256([]byte(password), salt, rczipPBKDF2Iter, 32)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(rest) < nonceSize {
		return nil, fmt.Errorf("文件损坏：缺少 nonce")
	}
	nonce := rest[:nonceSize]
	ciphertext := rest[nonceSize:]

	return gcm.Open(nil, nonce, ciphertext, nil)
}

// PBKDF2SHA256 implements PBKDF2 (RFC 2898) using HMAC-SHA256 as the
// pseudo-random function. This avoids the golang.org/x/crypto/pbkdf2
// dependency which is not available in the vendor directory.
func PBKDF2SHA256(password, salt []byte, iterations, keyLen int) []byte {
	hashLen := sha256.Size
	numBlocks := (keyLen + hashLen - 1) / hashLen
	result := make([]byte, 0, numBlocks*hashLen)

	for block := 1; block <= numBlocks; block++ {
		// U1 = PRF(password, salt || INT_32_BE(block))
		mac := hmac.New(sha256.New, password)
		mac.Write(salt)
		var buf [4]byte
		binary.BigEndian.PutUint32(buf[:], uint32(block))
		mac.Write(buf[:])
		u := mac.Sum(nil)

		t := make([]byte, len(u))
		copy(t, u)

		// U2..Uc = PRF(password, U_prev)
		for i := 1; i < iterations; i++ {
			mac := hmac.New(sha256.New, password)
			mac.Write(t)
			t = mac.Sum(nil)
			for j := range u {
				u[j] ^= t[j]
			}
		}
		result = append(result, u...)
	}
	return result[:keyLen]
}

// DecryptRCZIP1Legacy handles the old RCZIP1 format (raw SHA-256 key, no salt)
// for backward compatibility with exports created before the PBKDF2 migration.
func DecryptRCZIP1Legacy(data []byte, password string) ([]byte, error) {
	data = data[len("RCZIP1"):]

	hash := sha256.Sum256([]byte(password))
	key := hash[:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("文件损坏")
	}
	nonce := data[:nonceSize]
	ciphertext := data[nonceSize:]

	return gcm.Open(nil, nonce, ciphertext, nil)
}
