import { useCallback } from "react";

import { useApi } from "@/hooks/useApi";
import type { ActionCenter, CheckinStatus, NotificationPage, SystemDiagnostics } from "@/types";

const emptyNotificationPage: NotificationPage = {
  items: [],
  total: 0,
  unreadTotal: 0,
  importantTotal: 0,
  nextOffset: null,
};

export function useOpsHealth() {
  const checkins = useApi<CheckinStatus | null>("/api/checkins/status", null);
  const notifications = useApi<NotificationPage>("/api/notifications/page", emptyNotificationPage);
  const diagnostics = useApi<SystemDiagnostics | null>("/api/system/diagnostics", null);
  const actionCenter = useApi<ActionCenter | null>("/api/system/action-center", null);
  const { refresh: refreshCheckins } = checkins;
  const { refresh: refreshNotifications } = notifications;
  const { refresh: refreshDiagnostics } = diagnostics;
  const { refresh: refreshActionCenter } = actionCenter;

  const refresh = useCallback(async () => {
    await Promise.all([refreshCheckins(), refreshNotifications(), refreshDiagnostics(), refreshActionCenter()]);
  }, [refreshActionCenter, refreshCheckins, refreshDiagnostics, refreshNotifications]);

  return {
    loading: checkins.loading || notifications.loading || diagnostics.loading || actionCenter.loading,
    loaded: checkins.loaded && notifications.loaded && diagnostics.loaded && actionCenter.loaded,
    error: checkins.error || notifications.error || diagnostics.error || actionCenter.error,
    checkins: checkins.data,
    notifications: notifications.data.items,
    notificationPage: notifications.data,
    diagnostics: diagnostics.data,
    actionCenter: actionCenter.data,
    refresh,
  };
}

export type OpsHealthState = ReturnType<typeof useOpsHealth>;
