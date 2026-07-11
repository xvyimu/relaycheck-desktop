import { describe, expect, it } from "vitest";

import { accountsListUrl } from "../useSiteAccounts";

describe("accountsListUrl (S3)", () => {
  it("returns unfiltered path for empty or all", () => {
    expect(accountsListUrl()).toBe("/api/accounts");
    expect(accountsListUrl(null)).toBe("/api/accounts");
    expect(accountsListUrl("")).toBe("/api/accounts");
    expect(accountsListUrl("   ")).toBe("/api/accounts");
    expect(accountsListUrl("all")).toBe("/api/accounts");
  });

  it("appends encoded upstreamSiteId", () => {
    expect(accountsListUrl("site-a")).toBe("/api/accounts?upstreamSiteId=site-a");
    expect(accountsListUrl(" site-b ")).toBe("/api/accounts?upstreamSiteId=site-b");
    expect(accountsListUrl("a b/c")).toBe("/api/accounts?upstreamSiteId=a%20b%2Fc");
  });
});
