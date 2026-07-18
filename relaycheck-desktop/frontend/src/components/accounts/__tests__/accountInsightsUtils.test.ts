import { afterEach, describe, expect, it, vi } from "vitest";

import {
  buildModelCoverage,
  cleanupReasonLabel,
  isStaleAPIKeyCheck,
  keyIssueLabel,
  uniqueAccounts,
} from "@/components/accounts/accountInsightsUtils";
import type { Account } from "@/types";

function account(overrides: Partial<Account>): Account {
  return {
    id: "account-1",
    upstreamSiteId: "site-1",
    upstreamSiteName: "Site One",
    upstreamSiteBaseUrl: "https://site.example",
    upstreamSiteKind: "newapi",
    displayName: "Account One",
    authType: "api_key",
    loginStatus: "valid",
    ...overrides,
  } as Account;
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe("accountInsightsUtils", () => {
  it("classifies stale key checks without treating accounts without keys as stale", () => {
    vi.spyOn(Date, "now").mockReturnValue(new Date("2026-07-17T00:00:00Z").getTime());
    expect(isStaleAPIKeyCheck(account({ apiKeyFingerprint: "" }))).toBe(false);
    expect(isStaleAPIKeyCheck(account({ apiKeyFingerprint: "fp", apiKeyLastCheckedAt: "" }))).toBe(true);
    expect(isStaleAPIKeyCheck(account({ apiKeyFingerprint: "fp", apiKeyLastCheckedAt: "invalid" }))).toBe(true);
    expect(isStaleAPIKeyCheck(account({ apiKeyFingerprint: "fp", apiKeyLastCheckedAt: "2026-07-15T00:00:00Z" }))).toBe(
      true,
    );
    expect(isStaleAPIKeyCheck(account({ apiKeyFingerprint: "fp", apiKeyLastCheckedAt: "2026-07-16T12:00:00Z" }))).toBe(
      false,
    );
  });

  it("deduplicates accounts and aggregates model coverage", () => {
    const first = account({
      id: "account-1",
      apiKeySampleModels: ["gpt-4o", "claude-3"],
      apiKeyTestModel: "gpt-4o",
    });
    const duplicate = account({ id: "account-1", displayName: "Duplicate" });
    const second = account({
      id: "account-2",
      upstreamSiteName: "Site Two",
      apiKeySampleModels: ["gpt-4o"],
    });

    expect(uniqueAccounts([first, duplicate, second]).map((item) => item.id)).toEqual(["account-1", "account-2"]);
    expect(buildModelCoverage([first, second])).toEqual([
      { model: "gpt-4o", accountCount: 2, siteSamples: ["Site One", "Site Two"] },
      { model: "claude-3", accountCount: 1, siteSamples: ["Site One"] },
    ]);
  });

  it("formats cleanup and key issue labels", () => {
    expect(cleanupReasonLabel("site_not_support_checkin")).toBe("站点不支持签到");
    expect(cleanupReasonLabel("last_checkin_unsupported")).toBe("上次签到不支持");
    expect(cleanupReasonLabel("custom")).toBe("custom");
    expect(keyIssueLabel(account({ apiKeyStatus: "invalid" }))).toBe("密钥无效");
    expect(keyIssueLabel(account({ apiKeyStatus: "unchecked", apiKeyLastCheckedAt: "" }))).toBe("未检测");
  });
});
