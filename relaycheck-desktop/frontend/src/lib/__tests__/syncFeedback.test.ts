import { describe, expect, it } from "vitest";

import {
  formatExcludedSamplesHint,
  formatImportCountersMessage,
  instanceNeedsCredential,
  normalizeSyncCapability,
  syncCapabilityLabel,
  syncTokenStatusLabel,
} from "@/lib/syncFeedback";

describe("syncFeedback", () => {
  it("labels capabilities without inventing auth paths", () => {
    expect(normalizeSyncCapability("sqlite")).toBe("sqlite");
    expect(normalizeSyncCapability("admin_api_saved_token")).toBe("admin_api");
    expect(syncCapabilityLabel("sqlite")).toBe("本机数据库");
    expect(syncCapabilityLabel("admin_api_token_required")).toBe("后台 Admin API");
  });

  it("distinguishes empty source, excluded, and token errors", () => {
    expect(formatImportCountersMessage({ fetchedCount: 0, importedCount: 0 })).toMatch(/源端无渠道/);
    expect(
      formatImportCountersMessage({ fetchedCount: 3, importedCount: 0, skippedExcluded: 3 }),
    ).toMatch(/排除/);
    const err = formatImportCountersMessage({}, { error: "token invalid" });
    expect(err).toMatch(/系统访问令牌|数据库路径/);
    expect(err).not.toMatch(/关闭 2FA/);
  });

  it("needs credential only when no db path and no token", () => {
    expect(
      instanceNeedsCredential({ hasSyncToken: false, syncCapability: "admin_api", databasePath: "" }),
    ).toBe(true);
    expect(
      instanceNeedsCredential({ hasSyncToken: true, syncCapability: "admin_api" }),
    ).toBe(false);
    expect(
      instanceNeedsCredential({ hasSyncToken: false, syncCapability: "sqlite", databasePath: "C:/x.db" }),
    ).toBe(false);
  });

  it("token status label never says close 2FA", () => {
    const label = syncTokenStatusLabel(false, "admin_api");
    expect(label).toMatch(/系统访问令牌|数据库路径/);
    expect(label).not.toMatch(/关闭 2FA/);
  });

  it("formats excluded sample hints without secrets", () => {
    const hint = formatExcludedSamplesHint(
      [
        { sourceChannelId: "2", name: "9router free", matchedToken: "9router" },
        { sourceChannelId: "3", name: "tokenrouter", matchedToken: "tokenrouter" },
      ],
      false,
    );
    expect(hint).toMatch(/排除样例/);
    expect(hint).toMatch(/9router/);
    expect(hint).not.toMatch(/关闭 2FA/);
  });
});
