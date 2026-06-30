package core

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	"relaycheck-desktop/internal/accounts"
)

// localNewAPISyncRunInput is kept in core because the scheduler
// (runScheduledLocalNewAPISync) constructs it directly. The accounts package
// has its own mirror (accounts.SyncRunInput); the *App forwarder converts.
type localNewAPISyncRunInput struct {
	AccessToken       string `json:"accessToken"`
	SaveAccessToken   bool   `json:"saveAccessToken"`
	ClearAccessToken  bool   `json:"clearAccessToken"`
	UserID            string `json:"userId"`
	ImportKeys        bool   `json:"importKeys"`
	SkipCreateSites   bool   `json:"skipCreateSites"`
	DetectAfterImport bool   `json:"detectAfterImport"`
	PageSize          int    `json:"pageSize"`
}

// localNewAPISyncSourceInput is kept in core because the scheduler constructs
// it directly. The accounts package has its own mirror
// (accounts.SyncSourceInput); the *App forwarder converts.
type localNewAPISyncSourceInput struct {
	AccessToken      string `json:"accessToken"`
	SaveAccessToken  bool   `json:"saveAccessToken"`
	ClearAccessToken bool   `json:"clearAccessToken"`
	UserID           string `json:"userId"`
	PageSize         int    `json:"pageSize"`
}

func (a *App) handleLocalNewAPIInstances(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.listLocalNewAPIInstances(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (a *App) listLocalNewAPIInstances(w http.ResponseWriter, r *http.Request) {
	mirror, err := a.accountsService.ListLocalNewAPIInstances(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	items := make([]LocalNewAPIInstance, 0, len(mirror))
	for _, m := range mirror {
		items = append(items, localNewAPIInstanceFromMirror(m))
	}
	writeJSON(w, http.StatusOK, items)
}

func (a *App) handleLocalNewAPIInstanceByID(w http.ResponseWriter, r *http.Request) {
	tail := pathTail(r.URL.Path, "/api/local-newapi/")
	if strings.HasSuffix(tail, "/sync-preview") {
		id := strings.TrimSuffix(tail, "/sync-preview")
		a.previewLocalNewAPIInstanceSync(w, r, id)
		return
	}
	if strings.HasSuffix(tail, "/mark-missing") {
		id := strings.TrimSuffix(tail, "/mark-missing")
		a.markMissingLocalNewAPIInstance(w, r, id)
		return
	}
	if strings.HasSuffix(tail, "/sync") {
		id := strings.TrimSuffix(tail, "/sync")
		a.syncLocalNewAPIInstance(w, r, id)
		return
	}
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func (a *App) syncLocalNewAPIInstance(w http.ResponseWriter, r *http.Request, id string) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var input localNewAPISyncRunInput
	_ = decodeJSON(r, &input)
	result, err := a.syncLocalNewAPIInstanceData(r.Context(), id, input, true)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// syncLocalNewAPIInstanceData is the *App forwarder for
// accounts.Service.SyncLocalNewAPIInstanceData. Converts the core input
// type to the accounts mirror. Used by the sync handler and the scheduler.
func (a *App) syncLocalNewAPIInstanceData(ctx context.Context, id string, input localNewAPISyncRunInput, notify bool) (map[string]interface{}, error) {
	return a.accountsService.SyncLocalNewAPIInstanceData(ctx, id, accounts.SyncRunInput{
		AccessToken:       input.AccessToken,
		SaveAccessToken:   input.SaveAccessToken,
		ClearAccessToken:  input.ClearAccessToken,
		UserID:            input.UserID,
		ImportKeys:        input.ImportKeys,
		SkipCreateSites:   input.SkipCreateSites,
		DetectAfterImport: input.DetectAfterImport,
		PageSize:          input.PageSize,
	}, notify)
}

// getLocalNewAPIInstance is the *App forwarder for
// accounts.Service.GetLocalNewAPIInstance. Converts the accounts mirror back
// to core.LocalNewAPIInstance so existing callers (sync-preview handlers,
// scheduler) are unchanged.
func (a *App) getLocalNewAPIInstance(ctx context.Context, id string) (LocalNewAPIInstance, error) {
	mirror, err := a.accountsService.GetLocalNewAPIInstance(ctx, id)
	if err != nil {
		return LocalNewAPIInstance{}, err
	}
	return localNewAPIInstanceFromMirror(mirror), nil
}

// updateLocalNewAPISyncToken is the *App forwarder for
// accounts.Service.UpdateLocalNewAPISyncToken. Used by the admin-API import
// handler and the sync flow.
func (a *App) updateLocalNewAPISyncToken(ctx context.Context, instanceID string, token string, save bool, clear bool) error {
	return a.accountsService.UpdateLocalNewAPISyncToken(ctx, instanceID, token, save, clear)
}

// localNewAPIInstanceFromMirror converts an accounts.LocalNewAPIInstance back
// to core.LocalNewAPIInstance so handlers and the scheduler keep using the
// core type.
func localNewAPIInstanceFromMirror(m accounts.LocalNewAPIInstance) LocalNewAPIInstance {
	return LocalNewAPIInstance{
		ID:                 m.ID,
		Name:               m.Name,
		BaseURL:            m.BaseURL,
		DetectedFrom:       m.DetectedFrom,
		Status:             m.Status,
		Version:            m.Version,
		DatabasePath:       m.DatabasePath,
		ChannelCount:       m.ChannelCount,
		HasSyncToken:       m.HasSyncToken,
		SyncTokenMasked:    m.SyncTokenMasked,
		LastScannedAt:      m.LastScannedAt,
		CreatedAt:          m.CreatedAt,
		UpdatedAt:          m.UpdatedAt,
		SyncCapability:     m.SyncCapability,
		SyncTokenEncrypted: m.SyncTokenEncrypted,
	}
}

// syncCapability returns the sync capability label for an instance. Kept in
// core because the scheduler reads it directly.
func syncCapability(item LocalNewAPIInstance) string {
	if strings.TrimSpace(item.DatabasePath) != "" {
		return "sqlite"
	}
	if isHTTPURL(item.BaseURL) {
		if item.HasSyncToken {
			return "admin_api_saved_token"
		}
		return "admin_api_token_required"
	}
	return "unsupported"
}

// isHTTPURL reports whether value starts with http:// or https://. Kept in
// core because the scheduler uses it directly.
func isHTTPURL(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(normalized, "http://") || strings.HasPrefix(normalized, "https://")
}

// _ keeps database/sql imported for sql.ErrNoRows comparisons in future
// forwarders.
var _ = sql.ErrNoRows
