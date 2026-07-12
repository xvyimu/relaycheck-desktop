import { useCallback, useEffect, useRef, useState } from "react";

import { api } from "@/api/client";

export type UseApiOptions = {
  /** When false, skip auto-fetch and abort in-flight (keep-alive pause). */
  enabled?: boolean;
};

export function useApi<T>(url: string, fallback: T, options: UseApiOptions = {}) {
  const enabled = options.enabled ?? true;
  const [data, setData] = useState<T>(fallback);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [loaded, setLoaded] = useState(false);
  const fallbackRef = useRef(fallback);
  const hasSuccessfulDataRef = useRef(false);
  // Tracks the in-flight request's AbortController so that:
  // 1. A new refresh() aborts the previous in-flight request (race safety).
  // 2. Component unmount aborts the in-flight request (no setState after unmount).
  const abortRef = useRef<AbortController | null>(null);

  useEffect(() => {
    fallbackRef.current = fallback;
  }, [fallback]);

  const refresh = useCallback(async () => {
    // Abort any previous in-flight request to avoid late-resolving responses
    // overwriting newer data.
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;
    setLoading(true);
    setError("");
    try {
      const result = await api<T>(url, { signal: controller.signal });
      // Drop the response if a newer request or unmount aborted this one.
      if (controller.signal.aborted) return;
      setData(result);
      hasSuccessfulDataRef.current = true;
    } catch (err) {
      if (controller.signal.aborted) return;
      // Preserve the error message so callers can surface it to the user
      // instead of silently swapping in the fallback value.
      setError(err instanceof Error ? err.message : "加载失败");
      if (!hasSuccessfulDataRef.current) {
        setData(fallbackRef.current);
      }
    } finally {
      if (!controller.signal.aborted) {
        setLoaded(true);
        setLoading(false);
      }
    }
  }, [url]);

  useEffect(() => {
    if (!enabled) {
      abortRef.current?.abort();
      abortRef.current = null;
      return;
    }
    void refresh();
    // Abort the in-flight request on unmount or when url/enabled changes.
    return () => {
      abortRef.current?.abort();
      abortRef.current = null;
    };
  }, [refresh, enabled]);

  return { data, loading, loaded, error, refresh };
}