package backup

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Infra is the subset of the host application that the backup domain depends
// on. Extracting it breaks the reverse reference from the backup service back
// to the host god object. The host (e.g. *core.App) satisfies this interface
// by providing database access, filesystem layout, and lifecycle hooks.
//
// All methods are exported so that types defined in other packages (the host
// application) can satisfy the interface cross-package.
type Infra interface {
	// DB returns the application's SQLite database handle.
	DB() *sql.DB
	// DatabasePath returns the absolute path to the SQLite database file.
	DatabasePath() string
	// BackupsDir returns the directory where .rczip export files are stored.
	BackupsDir() string
	// ReopenDatabase closes the current database handle and opens a fresh
	// one pointed at DatabasePath(). Used after restoring an exported
	// database image.
	ReopenDatabase() error
	// ReloadNotificationConfig re-reads notification channel configuration
	// from the database after an import so channel secrets and digest
	// goroutines reflect the imported state.
	ReloadNotificationConfig(ctx context.Context) error
	// ProductVersion returns the host product version string embedded in
	// export manifests.
	ProductVersion() string
}

// Service implements the encrypted-export / encrypted-import domain. It owns
// the RCZIP file format and the settings round-tripping logic, while relying
// on Infra for database access and lifecycle hooks. The host application
// delegates its *App handler methods to this Service.
type Service struct {
	infra Infra
}

// NewService constructs a backup Service backed by the given Infra.
func NewService(infra Infra) *Service {
	return &Service{infra: infra}
}

// ListExports lists available .rczip export files in the backups directory.
func (s *Service) ListExports() ([]ExportResult, error) {
	backupsDir := s.infra.BackupsDir()
	if err := os.MkdirAll(backupsDir, 0o700); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(backupsDir)
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

// CreateEncryptedExport builds an AES-GCM encrypted zip archive containing
// the SQLite database and all system settings, writes it to the backups
// directory as export-<timestamp>.rczip, and returns the result metadata.
func (s *Service) CreateEncryptedExport(ctx context.Context, password string) (*ExportResult, error) {
	dbPath := s.infra.DatabasePath()

	// Read the database file
	dbData, err := os.ReadFile(dbPath)
	if err != nil {
		return nil, fmt.Errorf("读取数据库失败: %w", err)
	}

	// Read all settings
	settings, err := s.listAllSettings(ctx)
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
		ProductVersion: s.infra.ProductVersion(),
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
	encrypted, err := EncryptWithPassword(zipBuf.Bytes(), password)
	if err != nil {
		return nil, fmt.Errorf("加密失败: %w", err)
	}

	// Write to file
	fileName := fmt.Sprintf("export-%s.rczip", time.Now().Format("20060102-150405"))
	backupsDir := s.infra.BackupsDir()
	filePath := filepath.Join(backupsDir, fileName)
	if err := os.MkdirAll(backupsDir, 0o700); err != nil {
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

// RestoreEncryptedExport decrypts an .rczip file, replaces the running
// database image with the imported one, reopens the database, restores
// settings, and reloads notification configuration. The previous database
// is renamed to <dbpath>.pre-import-bak during the swap and removed on
// success.
func (s *Service) RestoreEncryptedExport(ctx context.Context, filePath, password string) (*ExportManifest, error) {
	// Read encrypted file
	encrypted, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取导出文件失败: %w", err)
	}

	// Decrypt
	zipData, err := DecryptWithPassword(encrypted, password)
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
	currentDBPath := s.infra.DatabasePath()
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
	if err := s.infra.ReopenDatabase(); err != nil {
		// Rollback
		_ = os.Rename(backupPath, currentDBPath)
		_ = s.infra.ReopenDatabase()
		return nil, fmt.Errorf("重新打开数据库失败: %w", err)
	}

	// Restore settings if present
	var settingsFailed int
	if len(settingsData) > 0 {
		var settings []Setting
		if err := json.Unmarshal(settingsData, &settings); err == nil {
			now := time.Now().UTC().Format(time.RFC3339Nano)
			for _, setting := range settings {
				_, err := s.infra.DB().ExecContext(ctx,
					`INSERT OR REPLACE INTO system_settings (key, value_json, updated_at) VALUES (?, ?, ?)`,
					setting.Key, setting.ValueJSON, now)
				if err != nil {
					settingsFailed++
					log.Printf("[import] 设置 %s 写入失败: %v", setting.Key, err)
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
	_ = s.infra.ReloadNotificationConfig(ctx)

	return &manifest, nil
}

// listAllSettings reads all rows from system_settings ordered by key. It is
// the inlined equivalent of the host's listAllSettings *App method, kept
// private so the host delegates to CreateEncryptedExport instead.
func (s *Service) listAllSettings(ctx context.Context) ([]Setting, error) {
	rows, err := s.infra.DB().QueryContext(ctx, `SELECT key, value_json, updated_at FROM system_settings ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var settings []Setting
	for rows.Next() {
		var setting Setting
		if err := rows.Scan(&setting.Key, &setting.ValueJSON, &setting.UpdatedAt); err != nil {
			return nil, err
		}
		settings = append(settings, setting)
	}
	return settings, nil
}
