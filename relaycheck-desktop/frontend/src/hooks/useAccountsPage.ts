import { useCallback, useEffect, useRef, useState } from "react";

import { accountApi } from "@/api/accounts";
import type { AccountPage } from "@/types";

export { buildAccountsPageUrl } from "@/api/accounts";
export type { AccountsPageQuery } from "@/api/accounts";

export interface UseAccountsPageOptions {
  limit?: number;
  query?: string;
  status?: string;
  upstreamSiteId?: string;
  enabled?: boolean;
}

export interface UseAccountsPageResult {
  page: AccountPage;
  loading: boolean;
  loaded: boolean;
  error: string;
  goNext: () => void;
  goPrev: () => void;
  hasNext: boolean;
  hasPrev: boolean;
  refresh: () => Promise<void>;
  reset: () => void;
}

const emptyPage = (): AccountPage => ({ items: [], total: 0, accountTotal: 0, problemTotal: 0 });

/**
 * Cursor-based account pagination hook.
 *
 * Maintains a cursor stack for back-navigation. The stack records the cursor
 * used to enter the current page, so "previous" replays the prior query.
 */
export function useAccountsPage(options: UseAccountsPageOptions = {}): UseAccountsPageResult {
  const { limit = 50, query = "", status = "all", upstreamSiteId = "", enabled = true } = options;

  const [page, setPage] = useState<AccountPage>(emptyPage);
  const [loading, setLoading] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState("");
  const [cursor, setCursor] = useState<string | undefined>(undefined);

  // Stack of cursors used to enter each page (for back-navigation).
  const [cursorStack, setCursorStack] = useState<string[]>([]);
  const stackRef = useRef<string[]>([]);

  const abortRef = useRef<AbortController | null>(null);
  const requestIdRef = useRef(0);
  const filtersRef = useRef({ limit, query, status, upstreamSiteId });

  const fetchPage = useCallback(
    async (nextCursor: string | undefined) => {
      abortRef.current?.abort();
      const controller = new AbortController();
      abortRef.current = controller;
      const requestId = ++requestIdRef.current;

      setLoading(true);
      setError("");
      try {
        const result = await accountApi.page<AccountPage>(
          { limit, query, status, upstreamSiteId, cursor: nextCursor },
          { signal: controller.signal },
        );
        if (controller.signal.aborted || requestId !== requestIdRef.current) return;
        setPage(result);
        setLoaded(true);
      } catch (err) {
        if (controller.signal.aborted || requestId !== requestIdRef.current) return;
        setError(err instanceof Error ? err.message : "加载账号分页失败");
      } finally {
        if (!controller.signal.aborted && requestId === requestIdRef.current) {
          setLoading(false);
        }
      }
    },
    [limit, query, status, upstreamSiteId],
  );

  const refresh = useCallback(async () => {
    await fetchPage(cursor);
  }, [fetchPage, cursor]);

  const goNext = useCallback(() => {
    if (!page.nextCursor) return;
    stackRef.current.push(cursor ?? "");
    setCursorStack([...stackRef.current]);
    setCursor(page.nextCursor);
  }, [page.nextCursor, cursor]);

  const goPrev = useCallback(() => {
    if (stackRef.current.length === 0) return;
    const prevCursor = stackRef.current.pop();
    setCursorStack([...stackRef.current]);
    setCursor(prevCursor || undefined);
  }, []);

  const reset = useCallback(() => {
    stackRef.current = [];
    setCursorStack([]);
    setCursor(undefined);
    setPage(emptyPage());
    setLoaded(false);
    setError("");
  }, []);

  useEffect(() => {
    const prev = filtersRef.current;
    const filtersChanged =
      prev.limit !== limit || prev.query !== query || prev.status !== status || prev.upstreamSiteId !== upstreamSiteId;
    filtersRef.current = { limit, query, status, upstreamSiteId };

    if (filtersChanged) {
      stackRef.current = [];
      setCursorStack([]);
      if (cursor !== undefined) {
        // Drop to first page; the next effect pass will fetch with cursor=undefined.
        setCursor(undefined);
        return;
      }
    }

    if (!enabled) return;
    void fetchPage(cursor);
    return () => {
      abortRef.current?.abort();
      abortRef.current = null;
    };
  }, [fetchPage, cursor, enabled, limit, query, status, upstreamSiteId]);

  return {
    page,
    loading,
    loaded,
    error,
    goNext,
    goPrev,
    hasNext: Boolean(page.nextCursor),
    hasPrev: cursorStack.length > 0,
    refresh,
    reset,
  };
}
