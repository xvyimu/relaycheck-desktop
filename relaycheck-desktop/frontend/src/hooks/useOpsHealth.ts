import { useCallback } from "react";

import { dashboardApi } from "@/api/dashboard";
import { useApi } from "@/hooks/useApi";
import type { DashboardOpsOverview, NotificationPage } from "@/types";

const emptyNotificationPage: NotificationPage = {
  items: [],
  total: 0,
  unreadTotal: 0,
  importantTotal: 0,
  nextOffset: null,
};

export function useOpsHealth() {
  const overview = useApi<DashboardOpsOverview | null>(dashboardApi.opsPath, null);
  const { refresh: refreshOverview } = overview;

  const refresh = useCallback(async () => {
    await refreshOverview();
  }, [refreshOverview]);

  return {
    loading: overview.loading,
    loaded: overview.loaded,
    error: overview.error,
    checkins: overview.data?.checkins ?? null,
    notifications: overview.data?.notifications?.items ?? [],
    notificationPage: overview.data?.notifications ?? emptyNotificationPage,
    diagnostics: overview.data?.diagnostics ?? null,
    actionCenter: overview.data?.actionCenter ?? null,
    refresh,
  };
}

export type OpsHealthState = ReturnType<typeof useOpsHealth>;
