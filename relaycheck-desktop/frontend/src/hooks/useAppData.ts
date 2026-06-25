import { useCallback, useEffect, useState } from "react";
import { api } from "@/api/client";
import type {
  Account,
  ActionCenter,
  CheckinStatus,
  ImportedChannel,
  ModelOverview,
  ModelPricingOverview,
  NotificationItem,
  StatusPayload,
  SystemDiagnostics,
  UpstreamSite,
  UsageOverview,
} from "@/types";

export interface AppData {
  loading: boolean;
  error: string;
  startupVersion: string;
  status: StatusPayload | null;
  channels: ImportedChannel[];
  sites: UpstreamSite[];
  accounts: Account[];
  checkins: CheckinStatus | null;
  notifications: NotificationItem[];
  diagnostics: SystemDiagnostics | null;
  actionCenter: ActionCenter | null;
  modelOverview: ModelOverview | null;
  pricingOverview: ModelPricingOverview | null;
  usageOverview: UsageOverview | null;
}

export function useAppData(): AppData & { reload: () => Promise<void> } {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [startupVersion, setStartupVersion] = useState("");
  const [status, setStatus] = useState<StatusPayload | null>(null);
  const [channels, setChannels] = useState<ImportedChannel[]>([]);
  const [sites, setSites] = useState<UpstreamSite[]>([]);
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [checkins, setCheckins] = useState<CheckinStatus | null>(null);
  const [notifications, setNotifications] = useState<NotificationItem[]>([]);
  const [diagnostics, setDiagnostics] = useState<SystemDiagnostics | null>(null);
  const [actionCenter, setActionCenter] = useState<ActionCenter | null>(null);
  const [modelOverview, setModelOverview] = useState<ModelOverview | null>(null);
  const [pricingOverview, setPricingOverview] = useState<ModelPricingOverview | null>(null);
  const [usageOverview, setUsageOverview] = useState<UsageOverview | null>(null);

  const loadData = useCallback(async () => {
    const [
      nextStatus,
      nextChannels,
      nextSites,
      nextAccounts,
      nextCheckins,
      nextNotifications,
      nextDiagnostics,
      nextActionCenter,
      nextModelOverview,
      nextPricingOverview,
      nextUsageOverview,
    ] = await Promise.all([
      api<StatusPayload>("/api/system/status"),
      api<ImportedChannel[]>("/api/channels"),
      api<UpstreamSite[]>("/api/upstream-sites"),
      api<Account[]>("/api/accounts"),
      api<CheckinStatus>("/api/checkins/status"),
      api<NotificationItem[]>("/api/notifications"),
      api<SystemDiagnostics>("/api/system/diagnostics"),
      api<ActionCenter>("/api/system/action-center"),
      api<ModelOverview>("/api/models/overview"),
      api<ModelPricingOverview>("/api/models/pricing"),
      api<UsageOverview>("/api/usage/overview"),
    ]);
    setStatus(nextStatus);
    setChannels(nextChannels);
    setSites(nextSites);
    setAccounts(nextAccounts);
    setCheckins(nextCheckins);
    setNotifications(nextNotifications);
    setDiagnostics(nextDiagnostics);
    setActionCenter(nextActionCenter);
    setModelOverview(nextModelOverview);
    setPricingOverview(nextPricingOverview);
    setUsageOverview(nextUsageOverview);
  }, []);

  const reload = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const health = await api<{ status?: string }>("/api/health").catch(() => null);
      if (health?.status) {
        setStartupVersion(health.status);
      }
      await loadData();
    } catch (err) {
      setError(err instanceof Error ? err.message : "加载数据失败");
    } finally {
      setLoading(false);
    }
  }, [loadData]);

  useEffect(() => {
    void reload();
  }, [reload]);

  return {
    loading,
    error,
    startupVersion,
    status,
    channels,
    sites,
    accounts,
    checkins,
    notifications,
    diagnostics,
    actionCenter,
    modelOverview,
    pricingOverview,
    usageOverview,
    reload,
  };
}
