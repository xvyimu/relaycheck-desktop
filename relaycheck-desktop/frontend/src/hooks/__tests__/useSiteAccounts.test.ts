import { describe, expect, it } from "vitest";

import { accountsListUrl, SITE_ACCOUNTS_PAGE_LIMIT } from "../useSiteAccounts";

describe("accountsListUrl", () => {
  it("returns unfiltered page URL for empty/all site ids", () => {
    expect(accountsListUrl()).toBe(`/api/accounts/page?limit=${SITE_ACCOUNTS_PAGE_LIMIT}`);
    expect(accountsListUrl(null)).toBe(`/api/accounts/page?limit=${SITE_ACCOUNTS_PAGE_LIMIT}`);
    expect(accountsListUrl("")).toBe(`/api/accounts/page?limit=${SITE_ACCOUNTS_PAGE_LIMIT}`);
    expect(accountsListUrl("   ")).toBe(`/api/accounts/page?limit=${SITE_ACCOUNTS_PAGE_LIMIT}`);
    expect(accountsListUrl("all")).toBe(`/api/accounts/page?limit=${SITE_ACCOUNTS_PAGE_LIMIT}`);
  });

  it("encodes upstreamSiteId for site-scoped page queries", () => {
    expect(accountsListUrl("site-a")).toBe(
      `/api/accounts/page?limit=${SITE_ACCOUNTS_PAGE_LIMIT}&upstreamSiteId=site-a`,
    );
    expect(accountsListUrl(" site-b ")).toBe(
      `/api/accounts/page?limit=${SITE_ACCOUNTS_PAGE_LIMIT}&upstreamSiteId=site-b`,
    );
    expect(accountsListUrl("a b/c")).toBe(
      `/api/accounts/page?limit=${SITE_ACCOUNTS_PAGE_LIMIT}&upstreamSiteId=a+b%2Fc`,
    );
  });
});
