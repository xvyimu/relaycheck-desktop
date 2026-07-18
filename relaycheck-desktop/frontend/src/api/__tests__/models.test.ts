import { afterEach, describe, expect, it, vi } from "vitest";

import type { ModelOverview, ModelPricingOverview } from "@/types";

import { modelsApi } from "../models";

afterEach(() => {
  vi.restoreAllMocks();
});

/** 构造模型覆盖概览夹具，避免测试依赖真实上游探测。 */
function makeModelOverview(overrides: Partial<ModelOverview> = {}): ModelOverview {
  return {
    generatedAt: "2026-07-18T02:00:00Z",
    syncedAccounts: 2,
    modelCount: 3,
    accountCount: 2,
    validKeyCount: 2,
    usableModelCount: 1,
    models: [],
    sites: [],
    priceHints: [],
    ...overrides,
  };
}

/** 构造价格概览夹具，覆盖同步前后的公共字段。 */
function makePricingOverview(overrides: Partial<ModelPricingOverview> = {}): ModelPricingOverview {
  return {
    generatedAt: "2026-07-18T02:00:00Z",
    sourceCount: 4,
    modelCount: 3,
    exactCount: 2,
    ratioCount: 1,
    liveCacheCount: 1,
    failedCacheCount: 0,
    sources: [],
    ...overrides,
  };
}

describe("modelsApi", () => {
  it("读取本地模型与价格概览时只发 GET，不携带 body", async () => {
    const overview = makeModelOverview();
    const pricing = makePricingOverview();
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ ok: true, data: overview }), {
          status: 200,
          headers: { "content-type": "application/json" },
        }),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ ok: true, data: pricing }), {
          status: 200,
          headers: { "content-type": "application/json" },
        }),
      );

    await expect(modelsApi.overview()).resolves.toEqual(overview);
    await expect(modelsApi.pricing()).resolves.toEqual(pricing);

    expect(fetchMock).toHaveBeenNthCalledWith(1, "/api/models/overview", {
      credentials: "same-origin",
      headers: undefined,
    });
    expect(fetchMock).toHaveBeenNthCalledWith(2, "/api/models/pricing", {
      credentials: "same-origin",
      headers: undefined,
    });
  });

  it("同步模型与价格时默认 limit=50，并集中持有 POST 契约", async () => {
    const overview = makeModelOverview({ syncedAccounts: 5 });
    const pricing = makePricingOverview({ liveCacheCount: 2 });
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ ok: true, data: overview }), {
          status: 200,
          headers: { "content-type": "application/json" },
        }),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ ok: true, data: pricing }), {
          status: 200,
          headers: { "content-type": "application/json" },
        }),
      );

    await expect(modelsApi.sync()).resolves.toEqual(overview);
    await expect(modelsApi.syncPricing()).resolves.toEqual(pricing);

    expect(fetchMock).toHaveBeenNthCalledWith(1, "/api/models/sync", {
      method: "POST",
      body: JSON.stringify({ limit: 50 }),
      credentials: "same-origin",
      headers: { "content-type": "application/json" },
    });
    expect(fetchMock).toHaveBeenNthCalledWith(2, "/api/models/pricing/sync", {
      method: "POST",
      body: JSON.stringify({ limit: 50 }),
      credentials: "same-origin",
      headers: { "content-type": "application/json" },
    });
  });

  it("允许调用方覆盖同步 limit，但不接受未知字段", async () => {
    const overview = makeModelOverview();
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ ok: true, data: overview }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );

    await modelsApi.sync({ limit: 10 });
    expect(fetchMock).toHaveBeenCalledWith("/api/models/sync", {
      method: "POST",
      body: JSON.stringify({ limit: 10 }),
      credentials: "same-origin",
      headers: { "content-type": "application/json" },
    });
  });
});
