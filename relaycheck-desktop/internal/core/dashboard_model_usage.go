package core

import (
	"net/http"
	"sync"
)

type DashboardModelUsageOverview struct {
	Model   modelOverview        `json:"model"`
	Pricing modelPricingOverview `json:"pricing"`
	Usage   usageOverview        `json:"usage"`
}

func (a *App) handleDashboardModelUsage(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	overview, err := cachedRead(a, "dashboard-model-usage", overviewReadCacheTTL, func() (DashboardModelUsageOverview, error) {
		return a.buildDashboardModelUsage(r)
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "加载 Dashboard 模型与用量数据失败。")
		return
	}
	writeJSON(w, http.StatusOK, overview)
}

func (a *App) buildDashboardModelUsage(r *http.Request) (DashboardModelUsageOverview, error) {
	var records []accountModelRecord
	var rawSources []modelPricingSource
	var cacheSources []modelPricingSource
	var cacheItems []sitePricingCacheItem
	var usage usageOverview
	var errorsByPart [4]error
	var wait sync.WaitGroup
	wait.Add(4)

	go func() {
		defer wait.Done()
		records, errorsByPart[0] = a.loadAccountModelRecords(r)
	}()
	go func() {
		defer wait.Done()
		rawSources, errorsByPart[1] = a.loadRawChannelPricingSources(r.Context())
	}()
	go func() {
		defer wait.Done()
		cacheSources, cacheItems, errorsByPart[2] = a.loadSitePricingCache(r.Context())
	}()
	go func() {
		defer wait.Done()
		usage, errorsByPart[3] = a.buildUsageOverview(r.Context())
	}()

	wait.Wait()
	for _, err := range errorsByPart {
		if err != nil {
			return DashboardModelUsageOverview{}, err
		}
	}
	return DashboardModelUsageOverview{
		Model:   buildModelOverview(records),
		Pricing: buildPricingOverview(append(rawSources, cacheSources...), cacheItems, records),
		Usage:   usage,
	}, nil
}
