package core

import (
	"net/http"
	"strings"
)

func (a *App) handleChromePasswordImportPreview(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var input struct {
		CSVContent string `json:"csvContent"`
	}
	if err := decodeJSON(r, &input); err != nil || strings.TrimSpace(input.CSVContent) == "" {
		writeError(w, http.StatusBadRequest, "请先选择 Chrome 手动导出的密码 CSV 文件。")
		return
	}
	result, err := a.accountsService.PreviewChromePasswordImport(r.Context(), input.CSVContent)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *App) handleChromePasswordImport(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var input struct {
		CSVContent string `json:"csvContent"`
	}
	if err := decodeJSON(r, &input); err != nil || strings.TrimSpace(input.CSVContent) == "" {
		writeError(w, http.StatusBadRequest, "请先选择 Chrome 手动导出的密码 CSV 文件。")
		return
	}
	result, err := a.accountsService.ImportChromePasswords(r.Context(), input.CSVContent)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	a.audit("import.chrome_passwords", "warning", "", "account", "", "Chrome 密码 CSV 导入完成。", map[string]interface{}{
		"totalRows":       intFromResult(result, "totalRows"),
		"matchedRows":     intFromResult(result, "matchedRows"),
		"uniqueSiteCount": intFromResult(result, "uniqueSiteCount"),
		"importedCount":   intFromResult(result, "importedCount"),
		"skippedExisting": intFromResult(result, "skippedExisting"),
	})
	writeJSON(w, http.StatusOK, result)
}
