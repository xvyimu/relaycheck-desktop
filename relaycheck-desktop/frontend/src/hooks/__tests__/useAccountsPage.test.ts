import { describe, expect, it } from "vitest";

import { buildAccountsPageUrl } from "../useAccountsPage";

describe("buildAccountsPageUrl", () => {
  it("always includes limit and omits empty filters", () => {
    expect(buildAccountsPageUrl()).toBe("/api/accounts/page?limit=50");
    expect(buildAccountsPageUrl({ limit: 20, query: "  ", status: "all", upstreamSiteId: "all" })).toBe(
      "/api/accounts/page?limit=20",
    );
  });

  it("serializes query/status/site/cursor for server-side pagination", () => {
    expect(
      buildAccountsPageUrl({
        limit: 2,
        query: "alpha user",
        status: "problem",
        upstreamSiteId: "site-a",
        cursor: "cur-1",
      }),
    ).toBe("/api/accounts/page?limit=2&query=alpha+user&status=problem&upstreamSiteId=site-a&cursor=cur-1");
  });

  it("trims query and drops blank cursor", () => {
    expect(buildAccountsPageUrl({ query: "  beta  ", cursor: undefined })).toBe(
      "/api/accounts/page?limit=50&query=beta",
    );
  });

  it("keeps custom limit when filters empty", () => {
    expect(buildAccountsPageUrl({ limit: 200 })).toBe("/api/accounts/page?limit=200");
  });
});
