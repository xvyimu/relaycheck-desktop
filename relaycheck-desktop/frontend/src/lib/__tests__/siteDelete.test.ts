import { describe, expect, it } from "vitest";

import { formatSiteDeleteResult, siteDeleteConfirmMessage } from "../siteDelete";

describe("siteDelete helpers", () => {
  it("siteDeleteConfirmMessage 明示级联与备份", () => {
    const message = siteDeleteConfirmMessage({ name: "Relay-A", accountCount: 3 });
    expect(message).toContain("Relay-A");
    expect(message).toContain("关联账号：3 个");
    expect(message).toContain("签到日志");
    expect(message).toContain("备份");
  });

  it("formatSiteDeleteResult 汇总级联计数", () => {
    const text = formatSiteDeleteResult("Relay-A", {
      siteId: "s1",
      deleted: true,
      accounts: 2,
      checkinLogs: 4,
      balanceSnapshots: 1,
      schedules: 0,
      pricingCache: 1,
    });
    expect(text).toContain("Relay-A");
    expect(text).toContain("账号 2");
    expect(text).toContain("签到日志 4");
    expect(text).toContain("价格缓存 1");
  });
});
