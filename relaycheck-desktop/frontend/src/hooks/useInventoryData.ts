import { useCallback } from "react";

import { useApi } from "@/hooks/useApi";
import type { DashboardInventoryOverview } from "@/types";

export function useInventoryData() {
  const overview = useApi<DashboardInventoryOverview | null>("/api/dashboard/inventory", null);
  const { refresh: refreshOverview } = overview;

  const refresh = useCallback(async () => {
    await refreshOverview();
  }, [refreshOverview]);

  return {
    loading: overview.loading,
    loaded: overview.loaded,
    error: overview.error,
    channels: overview.data?.channels ?? [],
    sites: overview.data?.sites ?? [],
    accountTotal: overview.data?.accountSummary?.accountTotal ?? 0,
    problemTotal: overview.data?.accountSummary?.problemTotal ?? 0,
    refresh,
  };
}

export type InventoryDataState = ReturnType<typeof useInventoryData>;
