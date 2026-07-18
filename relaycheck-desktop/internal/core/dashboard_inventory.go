package core

import (
	"net/http"
	"sync"
)

func (a *App) handleDashboardInventory(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	overview, err := a.buildDashboardInventory(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "加载 Dashboard 资产数据失败。")
		return
	}
	writeJSON(w, http.StatusOK, overview)
}

func (a *App) buildDashboardInventory(r *http.Request) (DashboardInventoryOverview, error) {
	var overview DashboardInventoryOverview
	var errorsByPart [3]error
	var wait sync.WaitGroup
	wait.Add(3)

	go func() {
		defer wait.Done()
		overview.Channels, errorsByPart[0] = a.loadChannels(r.Context())
	}()
	go func() {
		defer wait.Done()
		overview.Sites, errorsByPart[1] = a.loadUpstreamSites(r.Context())
	}()
	go func() {
		defer wait.Done()
		overview.AccountSummary, errorsByPart[2] = a.loadAccountSummary(r.Context())
	}()

	wait.Wait()
	for _, err := range errorsByPart {
		if err != nil {
			return DashboardInventoryOverview{}, err
		}
	}
	return overview, nil
}
