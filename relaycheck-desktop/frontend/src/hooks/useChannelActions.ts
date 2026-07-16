import { useCallback, useEffect, useState } from "react";
import { api } from "@/api/client";
import type { AccountSearchIndexItem, ChannelModelOverview, DetailDrawerState, ImportedChannel } from "@/types";

export interface ChannelActionsResult {
  channels: ImportedChannel[];
  searchIndex: AccountSearchIndexItem[];
  modelOverview: ChannelModelOverview | null;
  modelSyncing: boolean;
  message: string;
  loaded: boolean;
  drawer: DetailDrawerState | null;
  setDrawer: (state: DetailDrawerState | null) => void;
  setMessage: (msg: string) => void;
  refresh: () => Promise<void>;
  syncChannelModels: () => Promise<void>;
  updateChannelSourceStatus: (
    channel: ImportedChannel,
    action: "restore-source-status" | "archive-source-status",
  ) => Promise<void>;
  bulkUpdateSourceStatus: (fromStatus: "missing" | "archived", toStatus: "active" | "archived") => Promise<void>;
}

export type UseChannelActionsOptions = {
  /** When false, skip auto-fetch and treat panel as inactive (keep-alive). */
  active?: boolean;
  /** Prefer inventory channels to avoid dual-fetch on mount. */
  initialChannels?: ImportedChannel[];
  /** Prefer inventory search index to avoid dual-fetch on mount. */
  initialSearchIndex?: AccountSearchIndexItem[];
};

export function useChannelActions(options: UseChannelActionsOptions = {}): ChannelActionsResult {
  const { active = true, initialChannels, initialSearchIndex } = options;
  const [channels, setChannels] = useState<ImportedChannel[]>(() => initialChannels ?? []);
  const [searchIndex, setSearchIndex] = useState<AccountSearchIndexItem[]>(() => initialSearchIndex ?? []);
  const [modelOverview, setModelOverview] = useState<ChannelModelOverview | null>(null);
  const [modelSyncing, setModelSyncing] = useState(false);
  const [message, setMessage] = useState("");
  const seeded = Boolean(initialChannels);
  const [loaded, setLoaded] = useState(seeded);
  const [drawer, setDrawer] = useState<DetailDrawerState | null>(null);

  // Keep local state aligned when parent inventory refresh lands.
  useEffect(() => {
    if (initialChannels) setChannels(initialChannels);
  }, [initialChannels]);
  useEffect(() => {
    if (initialSearchIndex) setSearchIndex(initialSearchIndex);
  }, [initialSearchIndex]);

  const refresh = useCallback(async () => {
    try {
      const [nextChannels, nextModels, nextSearchIndex] = await Promise.all([
        api<ImportedChannel[]>("/api/channels"),
        api<ChannelModelOverview>("/api/channels/models/overview"),
        api<AccountSearchIndexItem[]>("/api/accounts/search-index"),
      ]);
      setChannels(nextChannels);
      setModelOverview(nextModels);
      setSearchIndex(nextSearchIndex);
      setLoaded(true);
    } catch (err) {
      setMessage(err instanceof Error ? `加载失败：${err.message}` : "加载渠道数据失败");
      setLoaded(true);
    }
  }, []);

  // Inventory only seeds channels. Models + compact search-index still load once while active.
  useEffect(() => {
    if (!active) return;
    if (!seeded) return;
    let cancelled = false;
    void (async () => {
      try {
        const [overview, nextSearchIndex] = await Promise.all([
          api<ChannelModelOverview>("/api/channels/models/overview"),
          api<AccountSearchIndexItem[]>("/api/accounts/search-index"),
        ]);
        if (!cancelled) {
          setModelOverview(overview);
          setSearchIndex(nextSearchIndex);
          setLoaded(true);
        }
      } catch {
        if (!cancelled) setLoaded(true);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [active, seeded]);

  const syncChannelModels = useCallback(async () => {
    setModelSyncing(true);
    setMessage("正在同步渠道模型…");
    try {
      const overview = await api<ChannelModelOverview>("/api/channels/models/sync", {
        method: "POST",
        body: JSON.stringify({ limit: 100 }),
      });
      setModelOverview(overview);
      setMessage(`已同步 ${overview.syncedChannels || 0} 个渠道，识别 ${overview.modelCount} 个模型`);
      setChannels(await api<ImportedChannel[]>("/api/channels"));
    } catch (err) {
      setMessage(err instanceof Error ? `同步失败：${err.message}` : "同步渠道模型失败");
    } finally {
      setModelSyncing(false);
    }
  }, []);

  const updateChannelSourceStatus = useCallback(
    async (channel: ImportedChannel, action: "restore-source-status" | "archive-source-status") => {
      const nextLabel = action === "restore-source-status" ? "恢复为活跃" : "归档";
      if (action === "archive-source-status") {
        const confirmed = window.confirm(
          `确认归档渠道"${channel.name}"？这不会删除账号、余额或签到日志，但该渠道会从日常视图中隐藏。`,
        );
        if (!confirmed) return;
      }
      setMessage(`${channel.name} 正在${nextLabel}…`);
      try {
        await api(`/api/channels/${channel.id}/${action}`, { method: "POST" });
        setMessage(`${channel.name} 已${nextLabel}`);
        await refresh();
      } catch (err) {
        setMessage(err instanceof Error ? `${nextLabel}失败：${err.message}` : `${nextLabel}失败`);
      }
    },
    [refresh],
  );

  const bulkUpdateSourceStatus = useCallback(
    async (fromStatus: "missing" | "archived", toStatus: "active" | "archived") => {
      const isArchiving = toStatus === "archived";
      const actionLabel = isArchiving ? "归档" : "恢复";
      const statusLabel = fromStatus === "missing" ? "源端已移除" : "已归档";
      const confirmed = window.confirm(
        `确认${actionLabel}全部"${statusLabel}"渠道？这只会修改本地状态，不会删除任何账号、余额或日志。`,
      );
      if (!confirmed) return;
      setMessage(`正在批量${actionLabel} ${statusLabel} 渠道…`);
      try {
        const result = await api<{ affected: number }>("/api/channels/bulk-source-status", {
          method: "POST",
          body: JSON.stringify({ fromStatus, toStatus }),
        });
        setMessage(`已批量${actionLabel} ${result.affected} 条渠道`);
        await refresh();
      } catch (err) {
        setMessage(err instanceof Error ? `批量${actionLabel}失败：${err.message}` : `批量${actionLabel}失败`);
      }
    },
    [refresh],
  );

  return {
    channels,
    searchIndex,
    modelOverview,
    modelSyncing,
    message,
    loaded,
    drawer,
    setDrawer,
    setMessage,
    refresh,
    syncChannelModels,
    updateChannelSourceStatus,
    bulkUpdateSourceStatus,
  };
}
