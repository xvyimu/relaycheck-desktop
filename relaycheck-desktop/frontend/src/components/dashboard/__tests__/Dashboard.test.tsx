import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

vi.mock("@/hooks/useSchedulerPreview", () => ({
  useSchedulerPreview: () => ({
    calendarItems: [],
    calendarGroups: [],
    calendarLoading: false,
    calendarError: "",
    refreshCalendar: async () => undefined,
    nextRuns: [],
    nextRunsLoading: false,
    nextRunsError: "",
    refreshNextRuns: async () => undefined,
  }),
}));

vi.mock("@/components/dashboard/HubRadar", () => ({
  HubRadar: () => <div data-testid="hub-radar">HubRadar</div>,
}));

vi.mock("@/components/dashboard/AnalyticsPanel", () => ({
  AnalyticsPanel: () => <div data-testid="analytics-panel">AnalyticsBody</div>,
}));

vi.mock("@/components/ui/UpdateBanner", () => ({
  UpdateBanner: () => null,
}));

import { Dashboard } from "../Dashboard";
import type { InventoryDataState } from "@/hooks/useInventoryData";
import type { ModelUsageOverviewState } from "@/hooks/useModelUsageOverview";
import type { OpsHealthState } from "@/hooks/useOpsHealth";
import type { SystemOverviewState } from "@/hooks/useSystemOverview";
import type { StatusPayload } from "@/types";

const status: StatusPayload = {
  productName: "RelayCheck Desktop",
  productVersion: "1.1.0",
  buildTime: "",
  architecture: "test",
  bindAddress: "127.0.0.1",
  databasePath: "",
  backupDir: "",
  port: 17890,
  summary: {
    localNewApiCount: 1,
    importedChannelCount: 2,
    identifiedChannelCount: 2,
    accountCount: 3,
    unreadNotifications: 0,
  },
};

const system: SystemOverviewState = {
  loading: false,
  loaded: true,
  error: "",
  startupVersion: "",
  status,
  refresh: async () => undefined,
};

const inventory: InventoryDataState = {
  loading: false,
  loaded: true,
  error: "",
  accounts: [],
  channels: [],
  sites: [],
  refresh: async () => undefined,
};

const ops: OpsHealthState = {
  loading: false,
  loaded: true,
  error: "",
  actionCenter: {
    generatedAt: "",
    overall: "ok",
    items: [
      {
        id: "act-1",
        priority: 1,
        level: "warning",
        category: "auth",
        title: "需要重新授权",
        description: "部分账号登录态失效",
        count: 2,
        target: "accounts",
        filter: "problem",
        action: "处理",
      },
    ],
  },
  checkins: null,
  diagnostics: null,
  notifications: [],
  refresh: async () => undefined,
};

const modelUsage: ModelUsageOverviewState = {
  loading: false,
  loaded: true,
  error: "",
  modelOverview: null,
  pricingOverview: null,
  usageOverview: null,
  refresh: async () => undefined,
};

describe("Dashboard layout α S4", () => {
  it("puts 运营待办 before compact metrics and collapses secondary blocks by default", () => {
    const html = renderToStaticMarkup(
      <Dashboard
        system={system}
        inventory={inventory}
        ops={ops}
        modelUsage={modelUsage}
        onNavigate={() => undefined}
        onRefresh={async () => undefined}
      />,
    );

    const priorityAt = html.indexOf("dashboard-priority-card");
    const metricsAt = html.indexOf("metric-grid-compact");
    const secondaryAt = html.indexOf("dashboard-secondary-grid");
    const analyticsAt = html.indexOf("dashboard-analytics-shell");

    expect(priorityAt).toBeGreaterThan(-1);
    expect(metricsAt).toBeGreaterThan(priorityAt);
    expect(secondaryAt).toBeGreaterThan(metricsAt);
    expect(analyticsAt).toBeGreaterThan(secondaryAt);

    expect(html).toContain("运营待办");
    expect(html).toContain("需要重新授权");
    expect(html).toContain("metric-grid-compact");
    expect(html).toContain('aria-expanded="false"');
    expect(html).toContain("展开分析");
    expect(html).not.toContain("AnalyticsBody");
    expect(html).not.toContain("产品</dt>");
  });
});
