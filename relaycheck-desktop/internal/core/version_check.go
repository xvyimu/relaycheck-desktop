package core

import (
	"net/http"

	"relaycheck-desktop/internal/versioncheck"
)

// Compile-time assertion that *App satisfies the versioncheck package's Infra
// interface (DB + HTTPClient + ProductVersion + ValidateOutboundURLStrict).
// The adapter methods live in infra.go, system.go, and url_safety.go.
var _ versioncheck.Infra = (*App)(nil)

// handleVersionCheck forwards to the versioncheck service.
//
// GET /api/system/version-check
func (a *App) handleVersionCheck(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	result := a.versionCheckService.CheckVersion(r.Context())
	writeJSON(w, http.StatusOK, result)
}
