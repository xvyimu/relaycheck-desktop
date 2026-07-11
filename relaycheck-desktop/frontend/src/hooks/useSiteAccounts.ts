import { useCallback, useEffect, useRef, useState } from "react";

import { api } from "@/api/client";
import type { Account } from "@/types";

/** Build GET /api/accounts URL; empty/"all" → unfiltered list. */
export function accountsListUrl(upstreamSiteId?: string | null): string {
  const id = (upstreamSiteId || "").trim();
  if (!id || id === "all") return "/api/accounts";
  return `/api/accounts?upstreamSiteId=${encodeURIComponent(id)}`;
}

/**
 * Site-scoped accounts fetch (α S3).
 * When siteFilter is "all", enabled=false and data stays null — callers use inventory.
 * When a site is selected, fetches only that site's accounts without touching channels/sites.
 */
export function useSiteAccounts(upstreamSiteId: string) {
  const siteId = (upstreamSiteId || "").trim();
  const enabled = siteId !== "" && siteId !== "all";
  const url = enabled ? accountsListUrl(siteId) : null;

  const [data, setData] = useState<Account[] | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [loaded, setLoaded] = useState(false);
  const abortRef = useRef<AbortController | null>(null);
  const requestIdRef = useRef(0);

  const refresh = useCallback(async () => {
    if (!url) {
      setData(null);
      setLoading(false);
      setError("");
      setLoaded(false);
      return;
    }

    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;
    const requestId = ++requestIdRef.current;

    setLoading(true);
    setError("");
    try {
      const result = await api<Account[]>(url, { signal: controller.signal });
      if (controller.signal.aborted || requestId !== requestIdRef.current) return;
      setData(result);
      setLoaded(true);
    } catch (err) {
      if (controller.signal.aborted || requestId !== requestIdRef.current) return;
      setError(err instanceof Error ? err.message : "加载失败");
      // Keep previous data if any; do not clear so UI can fall back gracefully.
    } finally {
      if (!controller.signal.aborted && requestId === requestIdRef.current) {
        setLoading(false);
      }
    }
  }, [url]);

  useEffect(() => {
    void refresh();
    return () => {
      abortRef.current?.abort();
      abortRef.current = null;
    };
  }, [refresh]);

  // Clear scoped data when returning to "all" so we don't leak a previous site list.
  useEffect(() => {
    if (!enabled) {
      setData(null);
      setLoaded(false);
      setError("");
      setLoading(false);
    }
  }, [enabled]);

  return { data, loading, loaded, error, enabled, refresh, url };
}

export type SiteAccountsState = ReturnType<typeof useSiteAccounts>;
