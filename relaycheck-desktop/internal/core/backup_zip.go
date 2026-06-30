package core

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"relaycheck-desktop/internal/backup"
)

// Compile-time assertion that *App satisfies the backup package's Infra
// interface (DB + DatabasePath + BackupsDir + ReopenDatabase +
// ReloadNotificationConfig + ProductVersion). The adapter methods live in
// system.go and notification.go.
var _ backup.Infra = (*App)(nil)

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

	result, err := a.backupService.CreateEncryptedExport(r.Context(), body.Password)
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

	manifest, err := a.backupService.RestoreEncryptedExport(r.Context(), filePath, body.Password)
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
	exports, err := a.backupService.ListExports()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, exports)
}
