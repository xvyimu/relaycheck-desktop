import { afterEach, describe, expect, it, vi } from "vitest";

import type { ChannelHealthOverview, ChannelModelOverview } from "@/types";

import { CHANNEL_HEALTH_OVERVIEW_PATH, channelsApi } from "../channels";

afterEach(() => {
  vi.restoreAllMocks();
});

/** 构造渠道模型概览夹具，字段与后端 ChannelModelOverview 对齐。 */
function makeOverview(overrides: Partial<ChannelModelOverview> = {}): ChannelModelOverview {
  return {
    generatedAt: "2026-07-18T03:00:00Z",
    syncedChannels: 2,
    channelCount: 5,
    modelCount: 12,
    liveKeyCount: 2,
    rawOnlyCount: 1,
    failedCount: 1,
    uncheckedCount: 2,
    items: [],
    models: [],
    ...overrides,
  };
}

/** 构造健康概览夹具。 */
function makeHealth(overrides: Partial<ChannelHealthOverview> = {}): ChannelHealthOverview {
  return {
    generatedAt: "2026-07-18T03:00:00Z",
    overall: "success",
    siteCount: 1,
    healthySiteCount: 1,
    unreachableSiteCount: 0,
    channelCount: 2,
    liveModelChannelCount: 1,
    failedModelChannelCount: 0,
    uncheckedModelChannelCount: 1,
    validKeyCount: 1,
    invalidKeyCount: 0,
    uncheckedKeyCount: 0,
    sites: [],
    ...overrides,
  };
}

describe("channelsApi", () => {
  it("modelsOverview 使用只读 GET，不携带 body", async () => {
    const overview = makeOverview();
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ ok: true, data: overview }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );

    await expect(channelsApi.modelsOverview()).resolves.toEqual(overview);
    expect(fetchMock).toHaveBeenCalledWith("/api/channels/models/overview", {
      credentials: "same-origin",
      headers: undefined,
    });
  });

  it("syncModels 默认 limit=100，并返回稳定 ChannelModelOverview schema", async () => {
    const overview = makeOverview({ syncedChannels: 3 });
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ ok: true, data: overview }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );

    await expect(channelsApi.syncModels()).resolves.toEqual(overview);
    expect(fetchMock).toHaveBeenCalledWith("/api/channels/models/sync", {
      method: "POST",
      body: JSON.stringify({ limit: 100 }),
      credentials: "same-origin",
      headers: { "content-type": "application/json" },
    });
  });

  it("允许 Onboarding 覆盖较小的 limit", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ ok: true, data: makeOverview() }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );

    await channelsApi.syncModels({ limit: 10 });
    expect(fetchMock).toHaveBeenCalledWith("/api/channels/models/sync", {
      method: "POST",
      body: JSON.stringify({ limit: 10 }),
      credentials: "same-origin",
      headers: { "content-type": "application/json" },
    });
  });

  it("healthOverview 只读 GET，且 path 常量与请求一致", async () => {
    const health = makeHealth({ overall: "warning" });
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ ok: true, data: health }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );

    expect(channelsApi.healthOverviewPath).toBe(CHANNEL_HEALTH_OVERVIEW_PATH);
    await expect(channelsApi.healthOverview()).resolves.toEqual(health);
    expect(fetchMock).toHaveBeenCalledWith("/api/channels/health/overview", {
      credentials: "same-origin",
      headers: undefined,
    });
  });

  it("detect/restore/archive 对 ID 编码且只发 POST 无 body", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockImplementation(() =>
      Promise.resolve(
        new Response(JSON.stringify({ ok: true, data: {} }), {
          status: 200,
          headers: { "content-type": "application/json" },
        }),
      ),
    );

    await channelsApi.detect("channel/a b");
    await channelsApi.restoreSourceStatus("channel-1");
    await channelsApi.archiveSourceStatus("channel-2");

    expect(fetchMock).toHaveBeenNthCalledWith(1, "/api/channels/channel%2Fa%20b/detect", {
      method: "POST",
      credentials: "same-origin",
      headers: undefined,
    });
    expect(fetchMock).toHaveBeenNthCalledWith(2, "/api/channels/channel-1/restore-source-status", {
      method: "POST",
      credentials: "same-origin",
      headers: undefined,
    });
    expect(fetchMock).toHaveBeenNthCalledWith(3, "/api/channels/channel-2/archive-source-status", {
      method: "POST",
      credentials: "same-origin",
      headers: undefined,
    });
  });

  it("bulkSourceStatus 仅发送 fromStatus/toStatus", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ ok: true, data: { affected: 3 } }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );

    await expect(channelsApi.bulkSourceStatus({ fromStatus: "missing", toStatus: "archived" })).resolves.toEqual({
      affected: 3,
    });
    expect(fetchMock).toHaveBeenCalledWith("/api/channels/bulk-source-status", {
      method: "POST",
      body: JSON.stringify({ fromStatus: "missing", toStatus: "archived" }),
      credentials: "same-origin",
      headers: { "content-type": "application/json" },
    });
  });
});
