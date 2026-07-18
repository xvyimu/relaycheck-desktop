import { useCallback, useEffect, useRef, useState } from "react";

import { accountApi } from "@/api/accounts";
import { api } from "@/api/client";
import { useDebouncedValue } from "@/hooks/useDebouncedValue";
import type { AccountSiteSearchResult } from "@/types";

const emptyResult = (): AccountSiteSearchResult => ({ items: [], truncated: false });

export function useAccountSiteSearch(query: string, enabled = true, limit = 200) {
  const debouncedQuery = useDebouncedValue(query.trim(), 250, enabled);
  const [data, setData] = useState<AccountSiteSearchResult>(emptyResult);
  const [searchedQuery, setSearchedQuery] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const abortRef = useRef<AbortController | null>(null);

  const refresh = useCallback(async () => {
    if (!enabled || !debouncedQuery) {
      abortRef.current?.abort();
      setData(emptyResult());
      setSearchedQuery("");
      setLoading(false);
      setError("");
      return;
    }
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;
    setLoading(true);
    setError("");
    try {
      const result = await api<AccountSiteSearchResult>(accountApi.searchSites(debouncedQuery, limit), {
        signal: controller.signal,
      });
      if (controller.signal.aborted) return;
      setData(result);
      setSearchedQuery(debouncedQuery.toLowerCase());
    } catch (err) {
      if (controller.signal.aborted) return;
      setError(err instanceof Error ? err.message : "搜索账号关联站点失败");
      setData(emptyResult());
      setSearchedQuery(debouncedQuery.toLowerCase());
    } finally {
      if (!controller.signal.aborted) setLoading(false);
    }
  }, [debouncedQuery, enabled, limit]);

  useEffect(() => {
    void refresh();
    return () => abortRef.current?.abort();
  }, [refresh]);

  return { data, searchedQuery, loading, error, refresh };
}
