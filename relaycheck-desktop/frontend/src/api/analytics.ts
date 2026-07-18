import { api } from "@/api/client";
import type { BalanceSnapshot, CheckinLog } from "@/types";

/** AnalyticsPanel 本地类型在组件内；adapter 用宽松结构避免双份 schema 漂移。 */
export type AnalyticsOverview = {
  generatedAt?: string;
  days?: number;
  [key: string]: unknown;
};

/** 读取分析面板聚合数据；days 走 query。 */
function getAnalytics(days: number): Promise<AnalyticsOverview> {
  const safe = Number.isFinite(days) && days > 0 ? Math.floor(days) : 30;
  return api<AnalyticsOverview>(`/api/analytics?days=${safe}`);
}

/** 余额快照列表（下钻用）。 */
function listBalanceSnapshots(): Promise<BalanceSnapshot[]> {
  return api<BalanceSnapshot[]>("/api/balances/snapshots");
}

/** 签到日志列表（下钻用）。 */
function listCheckinLogs(): Promise<CheckinLog[]> {
  return api<CheckinLog[]>("/api/checkins/logs");
}

export const analyticsApi = {
  getAnalytics,
  listBalanceSnapshots,
  listCheckinLogs,
} as const;
