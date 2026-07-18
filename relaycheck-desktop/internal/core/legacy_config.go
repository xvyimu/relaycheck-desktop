package core

import (
	"net/http"
	"strings"
)

func (a *App) handleLegacyConfigImport(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var input struct {
		ConfigContent string `json:"configContent"`
		FileName      string `json:"fileName"`
	}
	if err := decodeJSON(r, &input); err != nil || strings.TrimSpace(input.ConfigContent) == "" {
		writeError(w, http.StatusBadRequest, "旧配置内容不能为空。")
		return
	}
	result, err := a.accountsService.ImportLegacyConfig(r.Context(), input.ConfigContent, input.FileName)
	if err != nil {
		writeImportFailure(w, err, importFailureMessages{
			InvalidFormat: "旧配置格式或结构无效，请检查 JSON 内容。",
		})
		return
	}
	a.audit("import.legacy_config", "info", "", "upstream_site", stringFromResult(result, "siteId"), "旧配置导入完成。", map[string]interface{}{
		"siteCreated":     boolFromResult(result, "siteCreated"),
		"accountImported": boolFromResult(result, "accountImported"),
		"hasCheckinRule":  boolFromResult(result, "hasCheckinRule"),
		"hasBalanceRule":  boolFromResult(result, "hasBalanceRule"),
	})
	writeJSON(w, http.StatusOK, result)
}
