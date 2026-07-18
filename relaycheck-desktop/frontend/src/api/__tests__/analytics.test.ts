import { afterEach, describe, expect, it, vi } from "vitest";

import { analyticsApi } from "../analytics";

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

describe("analyticsApi", () => {
  it("getAnalytics 使用 days query", async () => {
    const fetchMock = mockOk({ days: 7 });
    await analyticsApi.getAnalytics(7);
    expect(fetchMock).toHaveBeenCalledWith("/api/analytics?days=7", {
      credentials: "same-origin",
      headers: undefined,
    });
  });

  it("下钻路径为固定 GET", async () => {
    const fetchMock = mockOk([]);
    await analyticsApi.listBalanceSnapshots();
    await analyticsApi.listCheckinLogs();
    expect(fetchMock).toHaveBeenCalledWith("/api/balances/snapshots", {
      credentials: "same-origin",
      headers: undefined,
    });
    expect(fetchMock).toHaveBeenCalledWith("/api/checkins/logs", {
      credentials: "same-origin",
      headers: undefined,
    });
  });
});
