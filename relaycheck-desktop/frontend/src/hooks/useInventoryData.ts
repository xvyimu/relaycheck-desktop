import { useCallback } from "react";

import { useApi } from "@/hooks/useApi";
import type { AccountSummary as AccountSummaryType, ImportedChannel, UpstreamSite } from "@/types";

export function useInventoryData() {
  const channels = useApi<ImportedChannel[]>("/api/channels", []);
  const sites = useApi<UpstreamSite[]>("/api/upstream-sites", []);
  const summary = useApi<AccountSummaryType | null>("/api/accounts/summary", null);
  const { refresh: refreshChannels } = channels;
  const { refresh: refreshSites } = sites;
  const { refresh: refreshSummary } = summary;

  const refresh = useCallback(async () => {
    await Promise.all([refreshChannels(), refreshSites(), refreshSummary()]);
  }, [refreshChannels, refreshSites, refreshSummary]);

  return {
    loading: channels.loading || sites.loading || summary.loading,
    loaded: channels.loaded && sites.loaded && summary.loaded,
    error: channels.error || sites.error || summary.error,
    channels: channels.data,
    sites: sites.data,
    accountTotal: summary.data?.accountTotal ?? 0,
    problemTotal: summary.data?.problemTotal ?? 0,
    refresh,
  };
}

export type InventoryDataState = ReturnType<typeof useInventoryData>;
