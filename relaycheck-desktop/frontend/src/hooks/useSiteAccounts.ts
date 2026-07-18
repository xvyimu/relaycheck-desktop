import { useCallback, useEffect, useRef, useState } from "react";

import { buildAccountsPageUrl } from "@/api/accounts";
import { api } from "@/api/client";
import type { Account, AccountPage } from "@/types";

/** Default page size for site master-detail (server page API). */
export const SITE_ACCOUNTS_PAGE_LIMIT = 200;

/** Site-scoped accounts URL via /api/accounts/page (no startup full list). */
export function accountsListUrl(upstreamSiteId?: string | null, limit = SITE_ACCOUNTS_PAGE_LIMIT): string {
  const id = (upstreamSiteId || "").trim();
  if (!id || id === "all") {
    return buildAccountsPageUrl({ limit });
  }
  return buildAccountsPageUrl({ limit, upstreamSiteId: id });
}

/**
 * Site-scoped accounts fetch (α S3 / FE-4).
 * When siteFilter is "all", enabled=false and data stays null.
 * When a site is selected, fetches that site's page without loading global inventory accounts.
 */
export function useSiteAccounts(upstreamSiteId: string) {
  const siteId = (upstreamSiteId || "").trim();
  const enabled = siteId !== "" && siteId !== "all";
  const url = enabled ? accountsListUrl(siteId) : null;

  const [data, setData] = useState<Account[] | null>(null);
  const [total, setTotal] = useState(0);
  const [truncated, setTruncated] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [loaded, setLoaded] = useState(false);
  const abortRef = useRef<AbortController | null>(null);
  const requestIdRef = useRef(0);

  const refresh = useCallback(async () => {
    if (!url) {
      setData(null);
      setTotal(0);
      setTruncated(false);
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
      const result = await api<AccountPage>(url, { signal: controller.signal });
      if (controller.signal.aborted || requestId !== requestIdRef.current) return;
      const items = result.items || [];
      setData(items);
      setTotal(result.total ?? items.length);
      setTruncated(Boolean(result.nextCursor) || (result.total ?? 0) > items.length);
      setLoaded(true);
    } catch (err) {
      if (controller.signal.aborted || requestId !== requestIdRef.current) return;
      setError(err instanceof Error ? err.message : "加载失败");
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

  useEffect(() => {
    if (!enabled) {
      setData(null);
      setTotal(0);
      setTruncated(false);
      setLoaded(false);
      setError("");
      setLoading(false);
    }
  }, [enabled]);

  return { data, total, truncated, loading, loaded, error, enabled, refresh, url };
}

export type SiteAccountsState = ReturnType<typeof useSiteAccounts>;
