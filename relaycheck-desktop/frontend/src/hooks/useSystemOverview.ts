import { useCallback, useEffect, useState } from "react";

import { systemApi } from "@/api/system";
import { useApi } from "@/hooks/useApi";
import type { StatusPayload } from "@/types";

export function useSystemOverview() {
  const [startupVersion, setStartupVersion] = useState("");
  const [healthLoading, setHealthLoading] = useState(true);
  const [healthLoaded, setHealthLoaded] = useState(false);
  // 状态 path 常量与 systemApi.status 同一 owner（api/system.ts）。
  const status = useApi<StatusPayload | null>(systemApi.statusPath, null);
  const { refresh: refreshStatus } = status;

  const refreshHealth = useCallback(async () => {
    setHealthLoading(true);
    try {
      const health = await systemApi.health().catch(() => null);
      if (health?.status) {
        setStartupVersion(health.status);
      }
    } finally {
      setHealthLoaded(true);
      setHealthLoading(false);
    }
  }, []);

  const refresh = useCallback(async () => {
    await Promise.all([refreshHealth(), refreshStatus()]);
  }, [refreshHealth, refreshStatus]);

  useEffect(() => {
    void refreshHealth();
  }, [refreshHealth]);

  return {
    loading: healthLoading || status.loading,
    loaded: healthLoaded && status.loaded,
    error: status.error,
    startupVersion,
    status: status.data,
    refresh,
  };
}

export type SystemOverviewState = ReturnType<typeof useSystemOverview>;
