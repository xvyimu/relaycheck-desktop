import { afterEach, describe, expect, it, vi } from "vitest";

import { sitesApi } from "../sites";

afterEach(() => {
  vi.restoreAllMocks();
});

function mockOk(data: unknown = {}) {
  return vi.spyOn(globalThis, "fetch").mockImplementation(() =>
    Promise.resolve(
      new Response(JSON.stringify({ ok: true, data }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    ),
  );
}

describe("sitesApi", () => {
  it("list 使用只读 GET /api/upstream-sites", async () => {
    const fetchMock = mockOk([]);
    await expect(sitesApi.list()).resolves.toEqual([]);
    expect(fetchMock).toHaveBeenCalledWith("/api/upstream-sites", {
      credentials: "same-origin",
      headers: undefined,
    });
  });

  it("get/detect 对 ID 编码", async () => {
    const fetchMock = mockOk({ site: { id: "s" }, detection: {}, accounts: [], balanceSnapshots: [], checkinLogs: [], suggestions: [] });
    await sitesApi.get("site/a b");
    await sitesApi.detect("site/a b");
    expect(fetchMock).toHaveBeenCalledWith("/api/upstream-sites/site%2Fa%20b", {
      credentials: "same-origin",
      headers: undefined,
    });
    expect(fetchMock).toHaveBeenCalledWith("/api/upstream-sites/site%2Fa%20b/detect", {
      method: "POST",
      credentials: "same-origin",
      headers: undefined,
    });
  });

  it("bulkDetect 仅 POST 声明 JSON", async () => {
    const fetchMock = mockOk({});
    await sitesApi.bulkDetect({ limit: 10 });
    expect(fetchMock).toHaveBeenCalledWith("/api/upstream-sites/bulk-detect", {
      method: "POST",
      body: JSON.stringify({ limit: 10 }),
      credentials: "same-origin",
      headers: { "content-type": "application/json" },
    });
  });

  it("remove 使用 DELETE 且编码 ID", async () => {
    const fetchMock = mockOk({
      siteId: "site/a",
      deleted: true,
      accounts: 1,
      checkinLogs: 0,
      balanceSnapshots: 0,
      schedules: 0,
      pricingCache: 0,
    });
    await expect(sitesApi.remove("site/a b")).resolves.toMatchObject({ deleted: true, accounts: 1 });
    expect(fetchMock).toHaveBeenCalledWith("/api/upstream-sites/site%2Fa%20b", {
      method: "DELETE",
      credentials: "same-origin",
      headers: undefined,
    });
  });
});