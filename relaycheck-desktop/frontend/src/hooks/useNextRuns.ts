import { useApi } from "@/hooks/useApi";
import { emptyNextRuns, nextRunItems, schedulerNextRunsPath, type NextRunResponse } from "@/lib/schedulerPreview";

export function useNextRuns() {
  const { data, loading, refresh } = useApi<NextRunResponse>(schedulerNextRunsPath, emptyNextRuns);

  return { nextRuns: nextRunItems(data), loading, refresh };
}
