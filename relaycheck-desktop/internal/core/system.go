package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"relaycheck-desktop/internal/notifications"
)

func (a *App) handleSystemSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.handleGetSystemSettings(w, r)
	case http.MethodPut:
		a.handleUpdateSystemSettings(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (a *App) handleGetSystemSettings(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.QueryContext(r.Context(), `
		SELECT key, value_json, updated_at
		FROM system_settings
		ORDER BY key ASC
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	items := []SystemSetting{}
	for rows.Next() {
		var item SystemSetting
		if err := rows.Scan(&item.Key, &item.ValueJSON, &item.UpdatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (a *App) handleUpdateSystemSettings(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Settings []SystemSetting `json:"settings"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "设置参数不完整。")
		return
	}
	updatedAt := now()
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback()

	for _, setting := range input.Settings {
		key := strings.TrimSpace(setting.Key)
		valueJSON := strings.TrimSpace(setting.ValueJSON)
		if key == "" || valueJSON == "" {
			writeError(w, http.StatusBadRequest, "设置 Key 和 JSON 内容不能为空。")
			return
		}
		if !json.Valid([]byte(valueJSON)) {
			writeError(w, http.StatusBadRequest, "设置 "+key+" 不是有效 JSON。")
			return
		}
		if key == "network.proxy" {
			config, err := parseNetworkProxyConfig(valueJSON)
			if err != nil {
				writeError(w, http.StatusBadRequest, "代理设置无效："+err.Error())
				return
			}
			normalized, _ := json.Marshal(config)
			valueJSON = string(normalized)
		} else if key == "channel.health.schedule" {
			var config channelHealthScheduleConfig
			if err := json.Unmarshal([]byte(valueJSON), &config); err != nil {
				writeError(w, http.StatusBadRequest, "渠道健康探测计划无效："+err.Error())
				return
			}
			normalized, _ := json.Marshal(normalizeChannelHealthScheduleConfig(config))
			valueJSON = string(normalized)
		} else if key == "notification.channels" {
			config, warnings := notifications.ParseChannelsConfig(valueJSON)
			for i := range config.Channels {
				if err := a.encryptChannelEntrySecrets(&config.Channels[i]); err != nil {
					writeError(w, http.StatusInternalServerError, "加密通知渠道密钥失败："+err.Error())
					return
				}
			}
			normalized, _ := json.Marshal(config)
			valueJSON = string(normalized)
			if len(warnings) > 0 {
				log.Printf("[notification] 渠道配置验证告警: %v", warnings)
			}
		}
		if _, err := tx.ExecContext(r.Context(), `
			INSERT INTO system_settings (id, key, value_json, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(key) DO UPDATE SET value_json=excluded.value_json, updated_at=excluded.updated_at
		`, newID(), key, valueJSON, updatedAt, updatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var reloadWarnings []string
	if err := a.reloadNetworkProxyConfig(r.Context()); err != nil {
		log.Printf("[settings] reload network proxy failed: %v", err)
		reloadWarnings = append(reloadWarnings, "代理配置已保存但运行时未刷新： "+err.Error())
	}
	if err := a.reloadNotificationConfig(r.Context()); err != nil {
		log.Printf("[settings] reload notification channels failed: %v", err)
		reloadWarnings = append(reloadWarnings, "通知渠道配置已保存但运行时未刷新： "+err.Error())
	}
	a.audit("settings.updated", "info", "", "system_settings", "", fmt.Sprintf("已保存 %d 项系统设置。", len(input.Settings)), map[string]interface{}{"count": len(input.Settings)})
	response := map[string]interface{}{"updated": len(input.Settings)}
	if len(reloadWarnings) > 0 {
		response["warnings"] = reloadWarnings
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *App) handleSystemBackups(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	backups, err := a.listBackups()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, backups)
}

func (a *App) handleSystemBackup(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	backup, err := a.createBackup("manual")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.notify("system_backup", "success", "数据库备份完成", "已创建本地备份："+backup.FileName, "backup", backup.FileName)
	a.audit("backup.created", "info", "", "backup", backup.FileName, "数据库备份完成："+backup.FileName, map[string]interface{}{"sizeBytes": backup.SizeBytes})
	writeJSON(w, http.StatusOK, backup)
}

func (a *App) handleSystemDeleteBackups(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var input struct {
		FileNames []string `json:"fileNames"`
	}
	if err := decodeJSON(r, &input); err != nil || len(input.FileNames) == 0 {
		writeError(w, http.StatusBadRequest, "请选择要删除的备份文件。")
		return
	}
	deleted := 0
	skipped := []string{}
	for _, fileName := range input.FileNames {
		backupPath, err := a.backupPath(fileName)
		if err != nil {
			skipped = append(skipped, filepath.Base(fileName))
			continue
		}
		if err := os.Remove(backupPath); err != nil {
			skipped = append(skipped, filepath.Base(fileName))
			continue
		}
		deleted++
	}
	if deleted > 0 {
		a.notify("system_backup_deleted", "info", "备份已清理", fmt.Sprintf("已删除 %d 个本地备份。", deleted), "backup", "")
		a.audit("backup.deleted", "warning", "", "backup", "", fmt.Sprintf("已删除 %d 个本地备份。", deleted), map[string]interface{}{"deleted": deleted, "skipped": len(skipped)})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"deleted": deleted,
		"skipped": skipped,
	})
}

func (a *App) handleSystemRestore(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var input struct {
		FileName string `json:"fileName"`
	}
	if err := decodeJSON(r, &input); err != nil || strings.TrimSpace(input.FileName) == "" {
		writeError(w, http.StatusBadRequest, "请选择要恢复的备份文件。")
		return
	}

	restorePath, err := a.backupPath(input.FileName)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := os.Stat(restorePath); err != nil {
		writeError(w, http.StatusNotFound, "备份文件不存在。")
		return
	}

	beforeBackup, err := a.createBackup("before-restore")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "恢复前自动备份失败："+err.Error())
		return
	}
	if err := a.restoreFromBackup(restorePath); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.notify("system_restore", "warning", "数据库已恢复", "已从备份恢复："+filepath.Base(restorePath)+"；恢复前快照："+beforeBackup.FileName, "backup", filepath.Base(restorePath))
	a.audit("backup.restored", "warning", "", "backup", filepath.Base(restorePath), "数据库已从备份恢复："+filepath.Base(restorePath), map[string]interface{}{"beforeBackup": beforeBackup.FileName})
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"restored":     true,
		"fileName":     filepath.Base(restorePath),
		"beforeBackup": beforeBackup,
	})
}

func (a *App) createBackup(reason string) (SystemBackup, error) {
	if err := os.MkdirAll(a.backupsDir(), 0o700); err != nil {
		return SystemBackup{}, err
	}
	timestamp := time.Now().Format("20060102-150405")
	safeReason := strings.NewReplacer(" ", "-", "_", "-", "/", "-", "\\", "-").Replace(strings.TrimSpace(reason))
	if safeReason == "" {
		safeReason = "manual"
	}
	fileName := fmt.Sprintf("relaycheck-%s-%s.db", timestamp, safeReason)
	targetPath := filepath.Join(a.backupsDir(), fileName)

	a.mu.Lock()
	defer a.mu.Unlock()

	if _, err := a.db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		return SystemBackup{}, err
	}
	sourcePath := a.databasePath()
	if err := copyFile(sourcePath, targetPath); err != nil {
		return SystemBackup{}, err
	}
	return backupInfo(targetPath)
}

func (a *App) restoreFromBackup(backupPath string) error {
	if err := validateSQLiteFile(backupPath); err != nil {
		return err
	}
	tempPath := filepath.Join(a.dataDir, ".restore-"+newID()+".db")
	if err := copyFile(backupPath, tempPath); err != nil {
		return err
	}
	defer os.Remove(tempPath)

	a.mu.Lock()
	defer a.mu.Unlock()

	if err := a.db.Close(); err != nil {
		return err
	}

	dbPath := a.databasePath()
	_ = os.Remove(dbPath + "-wal")
	_ = os.Remove(dbPath + "-shm")
	currentPath := filepath.Join(a.dataDir, ".restore-current-"+newID()+".db")
	currentMoved := false
	if _, err := os.Stat(dbPath); err == nil {
		if err := os.Rename(dbPath, currentPath); err != nil {
			if reopenErr := a.reopenDatabase(); reopenErr != nil {
				log.Printf("[restore] rename current db failed and reopen also failed: rename=%v reopen=%v", err, reopenErr)
			}
			return err
		}
		currentMoved = true
		defer os.Remove(currentPath)
	}
	if err := os.Rename(tempPath, dbPath); err != nil {
		if currentMoved {
			_ = os.Rename(currentPath, dbPath)
		}
		if reopenErr := a.reopenDatabase(); reopenErr != nil {
			return fmt.Errorf("恢复失败：%v；重新打开数据库也失败：%w", err, reopenErr)
		}
		return err
	}

	if err := a.reopenDatabase(); err != nil {
		return a.rollbackRestore(dbPath, currentPath, currentMoved, err)
	}
	ctx := context.Background()
	if err := a.migrate(ctx); err != nil {
		return a.rollbackRestore(dbPath, currentPath, currentMoved, err)
	}
	if err := a.ensureDefaultSettings(ctx); err != nil {
		return a.rollbackRestore(dbPath, currentPath, currentMoved, err)
	}
	return nil
}

func (a *App) rollbackRestore(dbPath string, currentPath string, currentMoved bool, cause error) error {
	if err := a.db.Close(); err != nil {
		log.Printf("[restore] close broken db during rollback failed: %v", err)
	}
	if currentMoved {
		if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
			log.Printf("[restore] remove broken db during rollback failed: %v", err)
		}
		if err := os.Rename(currentPath, dbPath); err != nil {
			log.Printf("[restore] restore previous db during rollback failed: %v", err)
		}
		if reopenErr := a.reopenDatabase(); reopenErr != nil {
			log.Printf("[restore] reopen previous db during rollback failed: %v", reopenErr)
		}
	}
	return cause
}

func (a *App) reopenDatabase() error {
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(a.databasePath())+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)")
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(1)
	a.db = db
	return nil
}

func (a *App) listBackups() ([]SystemBackup, error) {
	if err := os.MkdirAll(a.backupsDir(), 0o700); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(a.backupsDir())
	if err != nil {
		return nil, err
	}
	backups := []SystemBackup{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".db") {
			continue
		}
		item, err := backupInfo(filepath.Join(a.backupsDir(), entry.Name()))
		if err != nil {
			continue
		}
		backups = append(backups, item)
	}
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreatedAt > backups[j].CreatedAt
	})
	return backups, nil
}

func (a *App) backupPath(fileName string) (string, error) {
	cleanName := filepath.Base(strings.TrimSpace(fileName))
	if cleanName == "." || cleanName == "" || cleanName != strings.TrimSpace(fileName) || !strings.HasSuffix(strings.ToLower(cleanName), ".db") {
		return "", fmt.Errorf("只能恢复备份目录中的 .db 文件。")
	}
	return filepath.Join(a.backupsDir(), cleanName), nil
}

func (a *App) databasePath() string {
	return filepath.Join(a.dataDir, "relaycheck.db")
}

func (a *App) backupsDir() string {
	return filepath.Join(a.dataDir, "backups")
}

// The following exported adapters expose the host infrastructure that the
// internal/backup package depends on. They are thin wrappers around the
// unexported methods so that the rest of core can keep using the lowercase
// call sites while *App still satisfies backup.Infra.
func (a *App) DatabasePath() string   { return a.databasePath() }
func (a *App) BackupsDir() string     { return a.backupsDir() }
func (a *App) ReopenDatabase() error  { return a.reopenDatabase() }
func (a *App) ProductVersion() string { return productVersion }

func copyFile(sourcePath, targetPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer target.Close()

	if _, err := io.Copy(target, source); err != nil {
		return err
	}
	return target.Sync()
}

func backupInfo(path string) (SystemBackup, error) {
	info, err := os.Stat(path)
	if err != nil {
		return SystemBackup{}, err
	}
	return SystemBackup{
		FileName:  filepath.Base(path),
		Path:      path,
		SizeBytes: info.Size(),
		CreatedAt: info.ModTime().UTC().Format(time.RFC3339Nano),
	}, nil
}

func validateSQLiteFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	header := make([]byte, 16)
	if _, err := io.ReadFull(file, header); err != nil {
		return err
	}
	if string(header) != "SQLite format 3\x00" {
		return fmt.Errorf("备份文件不是有效的 SQLite 数据库。")
	}
	return nil
}

type PortCheckResult struct {
	Port       int    `json:"port"`
	Available  bool   `json:"available"`
	InUse      bool   `json:"inUse"`
	InUseByPID int    `json:"inUseByPid,omitempty"`
	Error      string `json:"error,omitempty"`
}

func (a *App) handleSystemPortCheck(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	portStr := r.URL.Query().Get("port")
	port := 0
	if portStr != "" {
		var err error
		port, err = strconv.Atoi(portStr)
		if err != nil || port < 1 || port > 65535 {
			writeError(w, http.StatusBadRequest, "端口号无效，需为 1-65535。")
			return
		}
	} else {
		a.mu.RLock()
		port = a.port
		a.mu.RUnlock()
	}

	result := PortCheckResult{Port: port}
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		result.InUse = true
		result.Available = false
		result.Error = err.Error()
	} else {
		_ = listener.Close()
		result.Available = true
		result.InUse = false
	}
	writeJSON(w, http.StatusOK, result)
}
