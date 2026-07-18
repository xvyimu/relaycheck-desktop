import type { TabKey } from "@/types/navigation";

export type ActionCenter = {
  generatedAt: string;
  overall: string;
  items: ActionItem[];
};

export type ActionItem = {
  id: string;
  priority: number;
  level: "success" | "info" | "warning" | "danger" | string;
  category: string;
  title: string;
  description: string;
  impact?: string;
  count: number;
  target: TabKey;
  filter?: string;
  action: string;
  recommendedAction?: string;
  samples?: ActionSample[];
};

export type ActionSample = {
  label: string;
  entityType?: "site" | "account" | "channel" | string;
  entityId?: string;
};

export type UsageAccountItem = {
  accountId: string;
  accountName: string;
  siteId: string;
  siteName: string;
  balance?: number;
  previousBalance?: number;
  balanceDelta?: number;
  unit: string;
  estimatedDailyUse?: number;
  lowBalance: boolean;
  trend: string;
  lastSnapshotAt?: string;
  previousSnapshotAt?: string;
};

export type UsageSiteItem = {
  siteId: string;
  siteName: string;
  accountCount: number;
  lowBalanceCount: number;
  decliningCount: number;
  balanceByUnit: Record<string, number>;
  estimatedDailyUse: Record<string, number>;
};

export type UsageOverview = {
  generatedAt: string;
  accountCount: number;
  siteCount: number;
  lowBalanceCount: number;
  decliningCount: number;
  truncated: boolean;
  estimatedDailyUse: Record<string, number>;
  sites: UsageSiteItem[];
  accounts: UsageAccountItem[];
};
