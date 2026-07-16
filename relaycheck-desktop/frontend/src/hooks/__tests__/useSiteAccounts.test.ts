import { describe, expect, it } from "vitest";

import { accountsListUrl } from "../useSiteAccounts";

describe("accountsListUrl", () => {
  it("returns unfiltered page URL for empty/all site ids", () => {
    expect(accountsListUrl()).toBe("/api/accounts/page?limit=200");
    expect(accountsListUrl(null)).toBe("/api/accounts/page?limit=200");
    expect(accountsListUrl("")).toBe("/api/accounts/page?limit=200");
    expect(accountsListUrl("   ")).toBe("/api/accounts/page?limit=200");
    expect(accountsListUrl("all")).toBe("/api/accounts/page?limit=200");
  });

  it("encodes upstreamSiteId for site-scoped page queries", () => {
    expect(accountsListUrl("site-a")).toBe("/api/accounts/page?limit=200&upstreamSiteId=site-a");
    expect(accountsListUrl(" site-b ")).toBe("/api/accounts/page?limit=200&upstreamSiteId=site-b");
    expect(accountsListUrl("a b/c")).toBe("/api/accounts/page?limit=200&upstreamSiteId=a+b%2Fc");
  });
});
