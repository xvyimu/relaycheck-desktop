import { useCallback, useEffect, useRef, useState } from "react";

import { api } from "@/api/client";

export function useApi<T>(url: string, fallback: T) {
  const [data, setData] = useState<T>(fallback);
  const [loading, setLoading] = useState(false);
  const fallbackRef = useRef(fallback);

  useEffect(() => {
    fallbackRef.current = fallback;
  }, [fallback]);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      setData(await api<T>(url));
    } catch {
      setData(fallbackRef.current);
    } finally {
      setLoading(false);
    }
  }, [url]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  return { data, loading, refresh };
}
