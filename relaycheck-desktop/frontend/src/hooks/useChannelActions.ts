import { useCallback, useEffect, useState } from "react";
import { channelsApi } from "@/api/channels";
import type { ChannelModelOverview, DetailDrawerState, ImportedChannel } from "@/types";

export interface ChannelActionsResult {
  channels: ImportedChannel[];
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
  /** Inventory owns channels; mutations invalidate it through this callback. */
  onInventoryRefresh?: () => Promise<void>;
};

export function useChannelActions(options: UseChannelActionsOptions = {}): ChannelActionsResult {
  const { active = true, initialChannels, onInventoryRefresh } = options;
  const channels = initialChannels ?? [];
  const [modelOverview, setModelOverview] = useState<ChannelModelOverview | null>(null);
  const [modelSyncing, setModelSyncing] = useState(false);
  const [message, setMessage] = useState("");
  const seeded = Boolean(initialChannels);
  const [loaded, setLoaded] = useState(seeded);
  const [drawer, setDrawer] = useState<DetailDrawerState | null>(null);

  const refresh = useCallback(async () => {
    try {
      // 模型概览契约归 channelsApi，避免 hook 直接拼 URL。
      const nextModels = await channelsApi.modelsOverview();
      setModelOverview(nextModels);
      setLoaded(true);
    } catch (err) {
      setMessage(err instanceof Error ? `加载失败：${err.message}` : "加载渠道数据失败");
      setLoaded(true);
    }
  }, []);

  // Inventory owns channels. This hook loads only channel-specific model state.
  useEffect(() => {
    if (!active) return;
    if (!seeded) return;
    let cancelled = false;
    void (async () => {
      try {
        const overview = await channelsApi.modelsOverview();
        if (!cancelled) {
          setModelOverview(overview);
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
      // 默认 limit 由 channelsApi 持有，与 Onboarding 的 limit=10 覆盖路径共用契约。
      const overview = await channelsApi.syncModels();
      setModelOverview(overview);
      setMessage(`已同步 ${overview.syncedChannels || 0} 个渠道，识别 ${overview.modelCount} 个模型`);
      await onInventoryRefresh?.();
    } catch (err) {
      setMessage(err instanceof Error ? `同步失败：${err.message}` : "同步渠道模型失败");
    } finally {
      setModelSyncing(false);
    }
  }, [onInventoryRefresh]);

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
        // 源状态写路径归 channelsApi；归档仅改本地状态，不删业务数据。
        if (action === "restore-source-status") {
          await channelsApi.restoreSourceStatus(channel.id);
        } else {
          await channelsApi.archiveSourceStatus(channel.id);
        }
        setMessage(`${channel.name} 已${nextLabel}`);
        await onInventoryRefresh?.();
      } catch (err) {
        setMessage(err instanceof Error ? `${nextLabel}失败：${err.message}` : `${nextLabel}失败`);
      }
    },
    [onInventoryRefresh],
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
        const result = await channelsApi.bulkSourceStatus({ fromStatus, toStatus });
        setMessage(`已批量${actionLabel} ${result.affected} 条渠道`);
        await onInventoryRefresh?.();
      } catch (err) {
        setMessage(err instanceof Error ? `批量${actionLabel}失败：${err.message}` : `批量${actionLabel}失败`);
      }
    },
    [onInventoryRefresh],
  );

  return {
    channels,
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
