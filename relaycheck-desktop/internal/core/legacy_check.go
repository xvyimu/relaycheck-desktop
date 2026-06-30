package core

import (
	"net/http"

	"relaycheck-desktop/internal/legacycheck"
)

// Compile-time assertion that *App satisfies the legacycheck package's Infra
// interface (DataDir). The adapter method lives in app.go.
var _ legacycheck.Infra = (*App)(nil)

// handleLegacyPythonCheck forwards to the legacycheck service.
//
// GET /api/system/legacy-check
func (a *App) handleLegacyPythonCheck(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	result := a.legacyCheckService.Check(r.Context())
	writeJSON(w, http.StatusOK, result)
}
