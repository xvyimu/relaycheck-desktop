import { afterEach, describe, expect, it, vi } from "vitest";

import { accountActionUrl, accountApi, buildAccountsPageUrl } from "../accounts";

afterEach(() => {
  vi.restoreAllMocks();
});

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

  it("page/postAction/remove/create/update/postBulk 走中央 request helper", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockImplementation(() =>
      Promise.resolve(
        new Response(JSON.stringify({ ok: true, data: { items: [] } }), {
          status: 200,
          headers: { "content-type": "application/json" },
        }),
      ),
    );
    await accountApi.page({ limit: 10, status: "all" });
    await accountApi.postAction("id/1", "checkin", {});
    await accountApi.remove("id/1");
    await accountApi.create({ displayName: "n" });
    await accountApi.update("id/1", { displayName: "n2" });
    await accountApi.postBulk("bulk-open-browser-login", { limit: 5 });
    expect(fetchMock).toHaveBeenCalledWith("/api/accounts/page?limit=10", expect.objectContaining({}));
    expect(fetchMock).toHaveBeenCalledWith("/api/accounts/id%2F1/checkin", expect.objectContaining({ method: "POST" }));
    expect(fetchMock).toHaveBeenCalledWith("/api/accounts/id%2F1", expect.objectContaining({ method: "DELETE" }));
    expect(fetchMock).toHaveBeenCalledWith("/api/accounts", expect.objectContaining({ method: "POST" }));
    expect(fetchMock).toHaveBeenCalledWith("/api/accounts/id%2F1", expect.objectContaining({ method: "PUT" }));
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/accounts/bulk-open-browser-login",
      expect.objectContaining({ method: "POST" }),
    );
  });
});
