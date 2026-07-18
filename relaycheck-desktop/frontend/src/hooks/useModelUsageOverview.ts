import { useCallback } from "react";

import { dashboardApi } from "@/api/dashboard";
import { useApi } from "@/hooks/useApi";
import type { DashboardModelUsageOverview } from "@/types";

export function useModelUsageOverview(options: { enabled?: boolean } = {}) {
  const enabled = options.enabled ?? true;
  const overview = useApi<DashboardModelUsageOverview | null>(dashboardApi.modelUsagePath, null, { enabled });
  const { refresh: refreshOverview } = overview;

  const refresh = useCallback(async () => {
    await refreshOverview();
  }, [refreshOverview]);

  return {
    modelOverview: overview.data?.model ?? null,
    pricingOverview: overview.data?.pricing ?? null,
    usageOverview: overview.data?.usage ?? null,
    loading: overview.loading,
    loaded: overview.loaded,
    error: overview.error,
    refresh,
  };
}

export type ModelUsageOverviewState = ReturnType<typeof useModelUsageOverview>;
