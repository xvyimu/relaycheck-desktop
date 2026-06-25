package core

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
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

// RCZIP format constants.
const (
	rczipMagic      = "RCZIP2" // v2: PBKDF2 key derivation
	rczipSaltLen    = 32
	rczipPBKDF2Iter = 200000
	// maxDecompressedSize limits total uncompressed zip content to 256 MB
	// to prevent zip-bomb denial-of-service.
	maxDecompressedSize = 256 * 1024 * 1024
	// maxSingleEntry limits any single zip entry to 200 MB.
	maxSingleEntry = 200 * 1024 * 1024
)

// handleEncryptedExport creates an AES-GCM encrypted zip archive containing
// the SQLite database and all system settings.
//
// POST /api/system/export
// Body: {"password": "..."}
func (a *App) handleEncryptedExport(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}

	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败")
		return
	}
	if len(body.Password) < 6 {
		writeError(w, http.StatusBadRequest, "密码至少 6 个字符")
		return
	}

	result, err := a.createEncryptedExport(r.Context(), body.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// handleEncryptedImport restores database and settings from an encrypted zip.
//
// POST /api/system/import
// Body: {"password": "...", "fileName": "..."}
func (a *App) handleEncryptedImport(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}

	var body struct {
		Password string `json:"password"`
		FileName string `json:"fileName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败")
		return
	}
	if len(body.Password) < 6 {
		writeError(w, http.StatusBadRequest, "密码至少 6 个字符")
		return
	}
	cleanName := filepath.Base(strings.TrimSpace(body.FileName))
	if cleanName == "" || cleanName != strings.TrimSpace(body.FileName) || !strings.HasSuffix(strings.ToLower(cleanName), ".rczip") {
		writeError(w, http.StatusBadRequest, "无效的文件名")
		return
	}

	filePath := filepath.Join(a.backupsDir(), cleanName)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		writeError(w, http.StatusNotFound, "导出文件不存在")
		return
	}

	manifest, err := a.restoreEncryptedExport(r.Context(), filePath, body.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"manifest": manifest,
	})
}

// handleListExports lists available .rczip export files.
func (a *App) handleListExports(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	exports, err := a.listExports()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, exports)
}

func (a *App) listExports() ([]ExportResult, error) {
	if err := os.MkdirAll(a.backupsDir(), 0o700); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(a.backupsDir())
	if err != nil {
		return nil, err
	}
	var results []ExportResult
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".rczip") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		results = append(results, ExportResult{
			FileName:  entry.Name(),
			SizeBytes: info.Size(),
		})
	}
	return results, nil
}

func (a *App) createEncryptedExport(ctx context.Context, password string) (*ExportResult, error) {
	dbPath := a.databasePath()

	// Read the database file
	dbData, err := os.ReadFile(dbPath)
	if err != nil {
		return nil, fmt.Errorf("读取数据库失败: %w", err)
	}

	// Read all settings
	settings, err := a.listAllSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("读取设置失败: %w", err)
	}
	settingsJSON, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("序列化设置失败: %w", err)
	}

	// Build manifest
	manifest := ExportManifest{
		Version:        "2",
		ExportedAt:     time.Now().UTC().Format(time.RFC3339),
		ProductVersion: productVersion,
		DatabaseSize:   int64(len(dbData)),
		SettingCount:   len(settings),
	}
	manifest.Includes.Database = true
	manifest.Includes.Settings = true
	manifestJSON, _ := json.MarshalIndent(manifest, "", "  ")

	// Create zip in memory
	var zipBuf bytes.Buffer
	zipWriter := zip.NewWriter(&zipBuf)

	// Add manifest
	manifestWriter, err := zipWriter.Create("manifest.json")
	if err != nil {
		return nil, err
	}
	if _, err := manifestWriter.Write(manifestJSON); err != nil {
		return nil, err
	}

	// Add database
	dbWriter, err := zipWriter.Create("relaycheck.db")
	if err != nil {
		return nil, err
	}
	if _, err := dbWriter.Write(dbData); err != nil {
		return nil, err
	}

	// Add settings
	settingsWriter, err := zipWriter.Create("settings.json")
	if err != nil {
		return nil, err
	}
	if _, err := settingsWriter.Write(settingsJSON); err != nil {
		return nil, err
	}

	if err := zipWriter.Close(); err != nil {
		return nil, err
	}

	// Encrypt the zip with AES-GCM using PBKDF2-derived key
	encrypted, err := encryptWithPassword(zipBuf.Bytes(), password)
	if err != nil {
		return nil, fmt.Errorf("加密失败: %w", err)
	}

	// Write to file
	fileName := fmt.Sprintf("export-%s.rczip", time.Now().Format("20060102-150405"))
	filePath := filepath.Join(a.backupsDir(), fileName)
	if err := os.MkdirAll(a.backupsDir(), 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filePath, encrypted, 0o600); err != nil {
		return nil, fmt.Errorf("写入文件失败: %w", err)
	}

	return &ExportResult{
		FileName:  fileName,
		SizeBytes: int64(len(encrypted)),
		Manifest:  manifest,
	}, nil
}

func (a *App) restoreEncryptedExport(ctx context.Context, filePath, password string) (*ExportManifest, error) {
	// Read encrypted file
	encrypted, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取导出文件失败: %w", err)
	}

	// Decrypt
	zipData, err := decryptWithPassword(encrypted, password)
	if err != nil {
		return nil, fmt.Errorf("解密失败（密码错误或文件损坏）: %w", err)
	}

	// Unzip with zip-bomb protection
	zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, fmt.Errorf("解压失败: %w", err)
	}

	var manifest ExportManifest
	var dbData []byte
	var settingsData []byte
	var totalDecompressed int64

	for _, file := range zipReader.File {
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		// Use LimitReader to cap each entry at maxSingleEntry
		limitedReader := &io.LimitedReader{R: rc, N: maxSingleEntry + 1}
		data, err := io.ReadAll(limitedReader)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("读取条目 %s 失败: %w", file.Name, err)
		}
		if limitedReader.N == 0 {
			return nil, fmt.Errorf("条目 %s 超过最大允许大小 %d MB", file.Name, maxSingleEntry/(1024*1024))
		}
		totalDecompressed += int64(len(data))
		if totalDecompressed > maxDecompressedSize {
			return nil, fmt.Errorf("解压内容超过最大允许总大小 %d MB", maxDecompressedSize/(1024*1024))
		}
		switch file.Name {
		case "manifest.json":
			if err := json.Unmarshal(data, &manifest); err != nil {
				return nil, fmt.Errorf("解析清单失败: %w", err)
			}
		case "relaycheck.db":
			dbData = data
		case "settings.json":
			settingsData = data
		}
	}

	if len(dbData) == 0 {
		return nil, fmt.Errorf("导出文件中缺少数据库")
	}

	// Backup current database
	currentDBPath := a.databasePath()
	backupPath := currentDBPath + ".pre-import-bak"
	if err := os.Rename(currentDBPath, backupPath); err != nil {
		return nil, fmt.Errorf("备份当前数据库失败: %w", err)
	}

	// Write imported database
	if err := os.WriteFile(currentDBPath, dbData, 0o600); err != nil {
		// Rollback
		_ = os.Rename(backupPath, currentDBPath)
		return nil, fmt.Errorf("写入数据库失败: %w", err)
	}

	// Reopen database
	if err := a.reopenDatabase(); err != nil {
		// Rollback
		_ = os.Rename(backupPath, currentDBPath)
		_ = a.reopenDatabase()
		return nil, fmt.Errorf("重新打开数据库失败: %w", err)
	}

	// Restore settings if present
	var settingsFailed int
	if len(settingsData) > 0 {
		var settings []SystemSetting
		if err := json.Unmarshal(settingsData, &settings); err == nil {
			for _, s := range settings {
				_, err := a.db.ExecContext(ctx,
					`INSERT OR REPLACE INTO system_settings (key, value_json, updated_at) VALUES (?, ?, ?)`,
					s.Key, s.ValueJSON, now())
				if err != nil {
					settingsFailed++
					log.Printf("[import] 设置 %s 写入失败: %v", s.Key, err)
				}
			}
		}
	}
	if settingsFailed > 0 {
		log.Printf("[import] %d 项设置写入失败", settingsFailed)
	}

	// Clean up backup
	_ = os.Remove(backupPath)

	// Reload notification config
	_ = a.reloadNotificationConfig(ctx)

	return &manifest, nil
}

func (a *App) listAllSettings(ctx context.Context) ([]SystemSetting, error) {
	rows, err := a.db.QueryContext(ctx, `SELECT key, value_json, updated_at FROM system_settings ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var settings []SystemSetting
	for rows.Next() {
		var s SystemSetting
		if err := rows.Scan(&s.Key, &s.ValueJSON, &s.UpdatedAt); err != nil {
			return nil, err
		}
		settings = append(settings, s)
	}
	return settings, nil
}

// encryptWithPassword encrypts data using AES-256-GCM with a key derived from
// the password via PBKDF2-SHA256 (200,000 iterations) with a random 32-byte salt.
//
// File format (binary, no base64):
//
//	[6 bytes magic "RCZIP2"] [32 bytes salt] [12 bytes GCM nonce] [ciphertext+tag]
func encryptWithPassword(data []byte, password string) ([]byte, error) {
	// Generate random salt
	salt := make([]byte, rczipSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("生成盐失败: %w", err)
	}

	// Derive key using PBKDF2-SHA256
	key := pbkdf2SHA256([]byte(password), salt, rczipPBKDF2Iter, 32)

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
	result := make([]byte, 0, len(rczipMagic)+len(salt)+len(nonce)+len(ciphertext))
	result = append(result, []byte(rczipMagic)...)
	result = append(result, salt...)
	result = append(result, nonce...)
	result = append(result, ciphertext...)
	return result, nil
}

// decryptWithPassword decrypts data encrypted by encryptWithPassword.
// Supports both RCZIP2 (PBKDF2) and legacy RCZIP1 (raw SHA-256) for backward compatibility.
func decryptWithPassword(data []byte, password string) ([]byte, error) {
	if len(data) < len(rczipMagic) {
		return nil, fmt.Errorf("文件过短，无效的导出文件")
	}
	magic := string(data[:len(rczipMagic)])

	switch magic {
	case "RCZIP2":
		return decryptRCZIP2(data, password)
	case "RCZIP1":
		// Legacy format: magic(6) + nonce + ciphertext, raw SHA-256 key
		return decryptRCZIP1Legacy(data, password)
	default:
		return nil, fmt.Errorf("无效的导出文件格式")
	}
}

func decryptRCZIP2(data []byte, password string) ([]byte, error) {
	headerLen := len(rczipMagic) + rczipSaltLen
	if len(data) < headerLen {
		return nil, fmt.Errorf("文件损坏：缺少盐")
	}
	salt := data[len(rczipMagic):headerLen]
	rest := data[headerLen:]

	key := pbkdf2SHA256([]byte(password), salt, rczipPBKDF2Iter, 32)

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

// pbkdf2SHA256 implements PBKDF2 (RFC 2898) using HMAC-SHA256 as the
// pseudo-random function. This avoids the golang.org/x/crypto/pbkdf2
// dependency which is not available in the vendor directory.
func pbkdf2SHA256(password, salt []byte, iterations, keyLen int) []byte {
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

// decryptRCZIP1Legacy handles the old RCZIP1 format (raw SHA-256 key, no salt)
// for backward compatibility with exports created before the PBKDF2 migration.
func decryptRCZIP1Legacy(data []byte, password string) ([]byte, error) {
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
