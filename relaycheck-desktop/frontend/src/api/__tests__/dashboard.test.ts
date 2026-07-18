import { afterEach, describe, expect, it, vi } from "vitest";

import { dashboardApi } from "../dashboard";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("dashboardApi", () => {
  it("path 常量与 useApi 消费者共享 owner", () => {
    expect(dashboardApi.opsPath).toBe("/api/dashboard/ops");
    expect(dashboardApi.inventoryPath).toBe("/api/dashboard/inventory");
    expect(dashboardApi.modelUsagePath).toBe("/api/dashboard/model-usage");
  });

  it("helper 只读 GET 对应路径", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockImplementation(() =>
      Promise.resolve(
        new Response(JSON.stringify({ ok: true, data: {} }), {
          status: 200,
          headers: { "content-type": "application/json" },
        }),
      ),
    );
    await dashboardApi.ops();
    await dashboardApi.inventory();
    await dashboardApi.modelUsage();
    expect(fetchMock).toHaveBeenCalledWith("/api/dashboard/ops", expect.objectContaining({}));
    expect(fetchMock).toHaveBeenCalledWith("/api/dashboard/inventory", expect.objectContaining({}));
    expect(fetchMock).toHaveBeenCalledWith("/api/dashboard/model-usage", expect.objectContaining({}));
  });
});
