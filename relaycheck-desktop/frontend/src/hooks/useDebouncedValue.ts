import { useEffect, useState } from "react";

export function useDebouncedValue<T>(value: T, delayMs: number, enabled = true): T {
  const [debounced, setDebounced] = useState(value);

  useEffect(() => {
    if (!enabled) return;
    const timer = globalThis.setTimeout(() => setDebounced(value), Math.max(0, delayMs));
    return () => globalThis.clearTimeout(timer);
  }, [delayMs, enabled, value]);

  return debounced;
}
