package core

import (
	"context"
	"net/http"

	"relaycheck-desktop/internal/accounts"
)

// autoDetectResult mirrors accounts.AutoDetectResult. Kept in core for the
// handleAutoDetectAndImport handler's response shape so the API contract is
// unchanged.
type autoDetectResult struct {
	DBPath        string `json:"dbPath"`
	BaseURL       string `json:"baseUrl"`
	ImportedCount int    `json:"importedCount"`
	SitesCreated  int    `json:"sitesCreated"`
	SitesMerged   int    `json:"sitesMerged"`
	Error         string `json:"error,omitempty"`
}

// baseURLForAutoDetectedDB is the *App forwarder for
// accounts.Service.BaseURLForAutoDetectedDB. Used by the auto-detect handler
// and the auto_detect_test.
func (a *App) baseURLForAutoDetectedDB(ctx context.Context, dbPath string) string {
	return a.accountsService.BaseURLForAutoDetectedDB(ctx, dbPath)
}

func (a *App) handleAutoDetectAndImport(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}

	result, err := a.accountsService.AutoDetectAndImport(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	found, _ := result["found"].(bool)
	message, _ := result["message"].(string)
	rawResults, _ := result["results"].([]accounts.AutoDetectResult)
	results := make([]autoDetectResult, 0, len(rawResults))
	for _, r := range rawResults {
		results = append(results, autoDetectResult{
			DBPath:        r.DBPath,
			BaseURL:       r.BaseURL,
			ImportedCount: r.ImportedCount,
			SitesCreated:  r.SitesCreated,
			SitesMerged:   r.SitesMerged,
			Error:         r.Error,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"found":   found,
		"message": message,
		"results": results,
	})
}
