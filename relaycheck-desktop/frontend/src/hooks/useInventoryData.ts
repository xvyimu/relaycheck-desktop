import { useCallback } from "react";

import { useApi } from "@/hooks/useApi";
import type { Account, ImportedChannel, UpstreamSite } from "@/types";

export function useInventoryData() {
  const channels = useApi<ImportedChannel[]>("/api/channels", []);
  const sites = useApi<UpstreamSite[]>("/api/upstream-sites", []);
  const accounts = useApi<Account[]>("/api/accounts", []);

  const refresh = useCallback(async () => {
    await Promise.all([channels.refresh(), sites.refresh(), accounts.refresh()]);
  }, [accounts.refresh, channels.refresh, sites.refresh]);

  return {
    loading: channels.loading || sites.loading || accounts.loading,
    loaded: channels.loaded && sites.loaded && accounts.loaded,
    error: channels.error || sites.error || accounts.error,
    channels: channels.data,
    sites: sites.data,
    accounts: accounts.data,
    refresh,
  };
}

export type InventoryDataState = ReturnType<typeof useInventoryData>;
