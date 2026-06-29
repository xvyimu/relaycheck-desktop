import { useState, useCallback, useRef, useEffect } from "react";

export interface ItemResult {
  id: string;
  name: string;
  status: string;
  message: string;
}

export interface TaskProgress {
  id: string;
  type: string;
  status: "running" | "done" | "cancelled";
  current: number;
  total: number;
  results: ItemResult[];
  startedAt: string;
  updatedAt: string;
  error?: string;
}

export type TaskType =
  | "checkin"
  | "test_keys"
  | "refresh_balances"
  | "detect_sites"
  | "channel_health_probe";

interface UseTaskProgressState {
  progress: TaskProgress | null;
  loading: boolean;
  error: string;
}

/**
 * Hook to start a batch task and stream progress via SSE.
 */
export function useTaskProgress() {
  const [state, setState] = useState<UseTaskProgressState>({
    progress: null,
    loading: false,
    error: "",
  });
  const eventSourceRef = useRef<EventSource | null>(null);

  const cleanup = useCallback(() => {
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }
  }, []);

  useEffect(() => cleanup, [cleanup]);

  const startTask = useCallback(
    async (type: TaskType, params?: Record<string, unknown>) => {
      setState({ progress: null, loading: true, error: "" });

      try {
        const response = await fetch("/api/tasks/start", {
          method: "POST",
          headers: { "content-type": "application/json" },
          credentials: "same-origin",
          body: JSON.stringify({ type, params: params || {} }),
        });
        const payload = await response.json();
        if (!payload.ok) {
          throw new Error(payload.error || "启动任务失败");
        }
        const taskId = payload.data.taskId as string;

        // Close any previously opened EventSource before opening a new one.
        // Without this, repeated startTask calls leak SSE connections and the
        // old onmessage/onerror handlers keep firing into stale state.
        cleanup();
        const es = new EventSource(`/api/tasks/${taskId}/stream`);
        eventSourceRef.current = es;

        es.onmessage = (event) => {
          try {
            const progress: TaskProgress = JSON.parse(event.data);
            setState({ progress, loading: false, error: "" });
            if (progress.status !== "running") {
              es.close();
              eventSourceRef.current = null;
            }
          } catch {
            // Ignore parse errors
          }
        };

        es.onerror = () => {
          es.close();
          eventSourceRef.current = null;
          setState((prev) => ({
            progress: prev.progress,
            loading: false,
            error: prev.progress ? "" : "连接中断，请重试。",
          }));
        };
      } catch (err) {
        setState({
          progress: null,
          loading: false,
          error: err instanceof Error ? err.message : "启动任务失败",
        });
      }
    },
    [cleanup],
  );

  const cancelTask = useCallback(async () => {
    const taskId = state.progress?.id;
    if (!taskId) return;
    try {
      await fetch(`/api/tasks/${taskId}/cancel`, {
        method: "POST",
        credentials: "same-origin",
      });
    } catch {
      // Ignore cancel errors
    }
    cleanup();
  }, [state.progress?.id, cleanup]);

  const reset = useCallback(() => {
    cleanup();
    setState({ progress: null, loading: false, error: "" });
  }, [cleanup]);

  return {
    progress: state.progress,
    loading: state.loading,
    error: state.error,
    startTask,
    cancelTask,
    reset,
  };
}
