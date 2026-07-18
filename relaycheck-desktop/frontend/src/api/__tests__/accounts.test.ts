import { describe, expect, it } from "vitest";

import { accountActionUrl, accountApi, buildAccountsPageUrl } from "../accounts";

describe("account API contract", () => {
  it("builds collection, summary, search and page routes centrally", () => {
    expect(accountApi.collection).toBe("/api/accounts");
    expect(accountApi.summary).toBe("/api/accounts/summary");
    expect(accountApi.searchIndex).toBe("/api/accounts/search-index");
    expect(accountApi.searchSites("needle", 20)).toBe("/api/accounts/search-sites?query=needle&limit=20");
    expect(buildAccountsPageUrl({ limit: 20, query: " alpha ", status: "problem" })).toBe(
      "/api/accounts/page?limit=20&query=alpha&status=problem",
    );
  });

  it("encodes account IDs and only accepts declared actions", () => {
    expect(accountActionUrl("account/a b", "test-login")).toBe("/api/accounts/account%2Fa%20b/test-login");
    expect(accountActionUrl("account-1", "refresh-balance")).toBe("/api/accounts/account-1/refresh-balance");
  });

  it("builds item and bulk routes", () => {
    expect(accountApi.item("account/a")).toBe("/api/accounts/account%2Fa");
    expect(accountApi.bulk("bulk-password-login")).toBe("/api/accounts/bulk-password-login");
  });
});
