import { useCallback, useEffect, useRef, useState } from "react";

import { api } from "@/api/client";

export function useApi<T>(url: string, fallback: T) {
  const [data, setData] = useState<T>(fallback);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const fallbackRef = useRef(fallback);

  useEffect(() => {
    fallbackRef.current = fallback;
  }, [fallback]);

  const refresh = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      setData(await api<T>(url));
    } catch (err) {
      // Preserve the error message so callers can surface it to the user
      // instead of silently swapping in the fallback value.
      setError(err instanceof Error ? err.message : "加载失败");
      setData(fallbackRef.current);
    } finally {
      setLoading(false);
    }
  }, [url]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  return { data, loading, error, refresh };
}
