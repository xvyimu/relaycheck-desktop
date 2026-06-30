package core

import (
	"context"
	"net/http"
	"strings"
)

func (a *App) handleImportFromSQLite(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}

	var input struct {
		DatabasePath      string `json:"databasePath"`
		ImportKeys        bool   `json:"importKeys"`
		InstanceName      string `json:"instanceName"`
		BaseURL           string `json:"baseUrl"`
		SkipCreateSites   bool   `json:"skipCreateSites"`
		DetectAfterImport bool   `json:"detectAfterImport"`
	}
	if err := decodeJSON(r, &input); err != nil || strings.TrimSpace(input.DatabasePath) == "" {
		writeError(w, http.StatusBadRequest, "SQLite 数据库路径不能为空。")
		return
	}

	result, err := a.importChannelsFromSQLite(r.Context(), input.DatabasePath, input.ImportKeys, input.InstanceName, input.BaseURL, !input.SkipCreateSites, input.DetectAfterImport)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	a.audit("import.sqlite", "info", "", "local_newapi_instance", stringFromResult(result, "instanceId"), "SQLite 渠道导入完成。", map[string]interface{}{
		"importedCount": intFromResult(result, "importedCount"),
		"sitesCreated":  intFromResult(result, "sitesCreated"),
		"sitesMerged":   intFromResult(result, "sitesMerged"),
		"detectedCount": intFromResult(result, "detectedCount"),
		"importKeys":    input.ImportKeys,
	})
	writeJSON(w, http.StatusOK, result)
}

// importChannelsFromSQLite is the *App forwarder for
// accounts.Service.ImportChannelsFromSQLite.
func (a *App) importChannelsFromSQLite(ctx context.Context, dbPath string, importKeys bool, instanceName string, baseURL string, createSites bool, detectAfterImport bool) (map[string]interface{}, error) {
	return a.accountsService.ImportChannelsFromSQLite(ctx, dbPath, importKeys, instanceName, baseURL, createSites, detectAfterImport)
}

// importChannelsFromSQLiteWithOptions is the *App forwarder for
// accounts.Service.ImportChannelsFromSQLiteWithOptions. Used by the
// local-NewAPI sync flow and the auto-detect flow.
func (a *App) importChannelsFromSQLiteWithOptions(ctx context.Context, dbPath string, importKeys bool, instanceName string, baseURL string, createSites bool, detectAfterImport bool, notify bool) (map[string]interface{}, error) {
	return a.accountsService.ImportChannelsFromSQLiteWithOptions(ctx, dbPath, importKeys, instanceName, baseURL, createSites, detectAfterImport, notify)
}
