import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

import { HubRadar } from "../HubRadar";
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
    localNewApiCount: 0,
    importedChannelCount: 0,
    identifiedChannelCount: 0,
    accountCount: 0,
    unreadNotifications: 0,
  },
};

const schedulerPreview = {
  calendarItems: [],
  calendarGroups: {},
  calendarLoading: false,
  refreshCalendar: async () => undefined,
};

describe("HubRadar S5 dead-link fix", () => {
  it("does not navigate to unimplemented balances tab", () => {
    const html = renderToStaticMarkup(
      <HubRadar
        status={status}
        diagnostics={null}
        actionCenter={null}
        modelOverview={null}
        pricingOverview={null}
        usageOverview={null}
        schedulerPreview={schedulerPreview}
        onNavigate={vi.fn()}
        onRefresh={vi.fn()}
      />,
    );

    expect(html).toContain("余额用量");
    expect(html).not.toContain("balances");
  });
});
