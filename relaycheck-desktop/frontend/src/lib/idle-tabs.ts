/** Idle tab keep-alive policy — pure helpers for App + tests. */

export const IDLE_TAB_TTL_MS = 5 * 60 * 1000;

/**
 * Drop non-active, non-pinned tabs whose last visit is older than TTL.
 * Returns null when nothing would change (caller keeps previous Set).
 */
export function pruneIdleTabs<T extends string>(
  visited: ReadonlySet<T>,
  active: T,
  lastVisitedAt: ReadonlyMap<T, number>,
  now: number,
  options: { ttlMs?: number; pinned: ReadonlySet<T> },
): Set<T> | null {
  const ttlMs = options.ttlMs ?? IDLE_TAB_TTL_MS;
  const next = new Set<T>();
  let changed = false;
  for (const key of visited) {
    if (key === active || options.pinned.has(key)) {
      next.add(key);
      continue;
    }
    const last = lastVisitedAt.get(key) ?? now;
    if (now - last < ttlMs) {
      next.add(key);
    } else {
      changed = true;
    }
  }
  return changed ? next : null;
}

/** True when at least one non-pinned visited tab exists (interval worth running). */
export function hasEvictableTabs<T extends string>(
  visited: ReadonlySet<T>,
  pinned: ReadonlySet<T>,
): boolean {
  for (const key of visited) {
    if (!pinned.has(key)) return true;
  }
  return false;
}
