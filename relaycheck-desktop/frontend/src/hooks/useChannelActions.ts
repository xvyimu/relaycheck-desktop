import { useCallback, useState } from "react";
import { api } from "@/api/client";
import type {
  Account,
  ChannelModelOverview,
  DetailDrawerState,
  ImportedChannel,
} from "@/types";

export interface ChannelActionsResult {
  channels: ImportedChannel[];
  accounts: Account[];
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
  bulkUpdateSourceStatus: (
    fromStatus: "missing" | "archived",
    toStatus: "active" | "archived",
  ) => Promise<void>;
}

export function useChannelActions(): ChannelActionsResult {
  const [channels, setChannels] = useState<ImportedChannel[]>([]);
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [modelOverview, setModelOverview] = useState<ChannelModelOverview | null>(null);
  const [modelSyncing, setModelSyncing] = useState(false);
  const [message, setMessage] = useState("");
  const [loaded, setLoaded] = useState(false);
  const [drawer, setDrawer] = useState<DetailDrawerState | null>(null);

  const refresh = useCallback(async () => {
    const [nextChannels, nextModels, nextAccounts] = await Promise.all([
      api<ImportedChannel[]>("/api/channels"),
      api<ChannelModelOverview>("/api/channels/models/overview"),
      api<Account[]>("/api/accounts"),
    ]);
    setChannels(nextChannels);
    setModelOverview(nextModels);
    setAccounts(nextAccounts);
    setLoaded(true);
  }, []);

  const syncChannelModels = useCallback(async () => {
    setModelSyncing(true);
    setMessage("正在同步渠道模型...");
    try {
      const overview = await api<ChannelModelOverview>("/api/channels/models/sync", {
        method: "POST",
        body: JSON.stringify({ limit: 100 }),
      });
      setModelOverview(overview);
      setMessage(`已同步 ${overview.syncedChannels || 0} 个渠道，识别 ${overview.modelCount} 个模型`);
      setChannels(await api<ImportedChannel[]>("/api/channels"));
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
      setMessage(`${channel.name} 正在${nextLabel}...`);
      await api(`/api/channels/${channel.id}/${action}`, { method: "POST" });
      setMessage(`${channel.name} 已${nextLabel}`);
      await refresh();
    },
    [refresh],
  );

  const bulkUpdateSourceStatus = useCallback(
    async (fromStatus: "missing" | "archived", toStatus: "active" | "archived") => {
      const isArchiving = toStatus === "archived";
      const actionLabel = isArchiving ? "归档" : "恢复";
      const statusLabel = fromStatus === "missing" ? "源端已移除" : "已归档";
      const confirmed = window.confirm(`确认${actionLabel}全部"${statusLabel}"渠道？这只会修改本地状态，不会删除任何账号、余额或日志。`);
      if (!confirmed) return;
      setMessage(`正在批量${actionLabel} ${statusLabel} 渠道...`);
      const result = await api<{ affected: number }>("/api/channels/bulk-source-status", {
        method: "POST",
        body: JSON.stringify({ fromStatus, toStatus }),
      });
      setMessage(`已批量${actionLabel} ${result.affected} 条渠道`);
      await refresh();
    },
    [refresh],
  );

  return {
    channels,
    accounts,
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