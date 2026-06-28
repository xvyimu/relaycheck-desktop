import { useApi } from "@/hooks/useApi";
import type { NextRunItem } from "@/types";

type NextRunResponse = {
  generatedAt: string;
  items: NextRunItem[];
};

const emptyNextRuns: NextRunResponse = {
  generatedAt: "",
  items: [],
};

export function useNextRuns() {
  const { data, loading, refresh } = useApi<NextRunResponse>("/api/scheduler/next-runs", emptyNextRuns);

  return { nextRuns: data.items || [], loading, refresh };
}
