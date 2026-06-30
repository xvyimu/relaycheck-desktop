package core

import (
	"context"
	"net/http"
	"strings"
)

func (a *App) handleImportFromAdminAPI(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var input struct {
		BaseURL           string `json:"baseUrl"`
		AccessToken       string `json:"accessToken"`
		SaveAccessToken   bool   `json:"saveAccessToken"`
		UserID            string `json:"userId"`
		InstanceName      string `json:"instanceName"`
		ImportKeys        bool   `json:"importKeys"`
		SkipCreateSites   bool   `json:"skipCreateSites"`
		DetectAfterImport bool   `json:"detectAfterImport"`
		PageSize          int    `json:"pageSize"`
	}
	if err := decodeJSON(r, &input); err != nil || strings.TrimSpace(input.BaseURL) == "" || strings.TrimSpace(input.AccessToken) == "" {
		writeError(w, http.StatusBadRequest, "NewAPI 地址和访问令牌不能为空。")
		return
	}
	if strings.TrimSpace(input.UserID) == "" {
		input.UserID = "1"
	}
	input.PageSize = clampInt(input.PageSize, 10, 100, 100)
	result, err := a.importChannelsFromAdminAPI(r.Context(), input.BaseURL, input.AccessToken, input.UserID, input.InstanceName, input.ImportKeys, !input.SkipCreateSites, input.DetectAfterImport, input.PageSize)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if input.SaveAccessToken {
		if instanceID, ok := result["instanceId"].(string); ok {
			if err := a.updateLocalNewAPISyncToken(r.Context(), instanceID, input.AccessToken, true, false); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			result["syncTokenSaved"] = true
		}
	}
	a.audit("import.admin_api", "info", "", "local_newapi_instance", stringFromResult(result, "instanceId"), "NewAPI 后台导入完成。", map[string]interface{}{
		"importedCount":  intFromResult(result, "importedCount"),
		"sitesCreated":   intFromResult(result, "sitesCreated"),
		"sitesMerged":    intFromResult(result, "sitesMerged"),
		"detectedCount":  intFromResult(result, "detectedCount"),
		"importKeys":     input.ImportKeys,
		"syncTokenSaved": input.SaveAccessToken,
	})
	writeJSON(w, http.StatusOK, result)
}

// importChannelsFromAdminAPI is the *App forwarder for
// accounts.Service.ImportChannelsFromAdminAPI.
func (a *App) importChannelsFromAdminAPI(ctx context.Context, rawBaseURL string, accessToken string, userID string, instanceName string, importKeys bool, createSites bool, detectAfterImport bool, pageSize int) (map[string]interface{}, error) {
	return a.accountsService.ImportChannelsFromAdminAPI(ctx, rawBaseURL, accessToken, userID, instanceName, importKeys, createSites, detectAfterImport, pageSize)
}

// importChannelsFromAdminAPIWithOptions is the *App forwarder for
// accounts.Service.ImportChannelsFromAdminAPIWithOptions. Used by the
// local-NewAPI sync flow.
func (a *App) importChannelsFromAdminAPIWithOptions(ctx context.Context, rawBaseURL string, accessToken string, userID string, instanceName string, importKeys bool, createSites bool, detectAfterImport bool, pageSize int, notify bool) (map[string]interface{}, error) {
	return a.accountsService.ImportChannelsFromAdminAPIWithOptions(ctx, rawBaseURL, accessToken, userID, instanceName, importKeys, createSites, detectAfterImport, pageSize, notify)
}
