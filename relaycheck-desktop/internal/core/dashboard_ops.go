package core

import (
	"net/http"
	"sync"
)

func (a *App) handleDashboardOps(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	overview, err := a.buildDashboardOps(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "加载 Dashboard 运营数据失败。")
		return
	}
	writeJSON(w, http.StatusOK, overview)
}

func (a *App) buildDashboardOps(r *http.Request) (DashboardOpsOverview, error) {
	var overview DashboardOpsOverview
	var errorsByPart [4]error
	var wait sync.WaitGroup
	wait.Add(4)

	go func() {
		defer wait.Done()
		overview.Checkins, errorsByPart[0] = a.loadCheckinStatus(r)
	}()
	go func() {
		defer wait.Done()
		overview.Notifications, errorsByPart[1] = a.listNotificationsPage(r.Context(), notificationOptions(r))
	}()
	go func() {
		defer wait.Done()
		overview.Diagnostics, errorsByPart[2] = a.loadSystemDiagnostics(r)
	}()
	go func() {
		defer wait.Done()
		overview.ActionCenter, errorsByPart[3] = a.loadActionCenter(r)
	}()

	wait.Wait()
	for _, err := range errorsByPart {
		if err != nil {
			return DashboardOpsOverview{}, err
		}
	}
	return overview, nil
}
