import { describe, expect, it, vi } from "vitest";

import type { ChannelHealthSite, ImportedChannel } from "@/types";

import {
  healthToneClass,
  refreshChannelPanelData,
  shouldAutoloadChannelModels,
  syncChannelModelsAndHealth,
  topHealthRisks,
} from "../ChannelsPanel";

/** 构造健康站点夹具，仅覆盖 risk 筛选所需字段。 */
function healthSite(overrides: Partial<ChannelHealthSite> = {}): ChannelHealthSite {
  return {
    siteId: "site-1",
    siteName: "站点一",
    baseUrl: "https://relay.example",
    kind: "newapi",
    level: "success",
    healthStatus: "ok",
    accountCount: 1,
    validKeyCount: 1,
    invalidKeyCount: 0,
    uncheckedKeyCount: 0,
    modelChannelCount: 1,
    liveModelChannelCount: 1,
    failedModelChannelCount: 0,
    uncheckedModelChannelCount: 0,
    modelCount: 1,
    recommendedAction: "保持观察",
    ...overrides,
  };
}

describe("ChannelsPanel refresh ownership", () => {
  it("refreshes models, health and inventory exactly once in parallel", async () => {
    const refreshModels = vi.fn().mockResolvedValue(undefined);
    const refreshHealth = vi.fn().mockResolvedValue(undefined);
    const refreshInventory = vi.fn().mockResolvedValue(undefined);

    const message = await refreshChannelPanelData(refreshModels, refreshHealth, refreshInventory);

    expect(message).toBe("");
    expect(refreshModels).toHaveBeenCalledTimes(1);
    expect(refreshHealth).toHaveBeenCalledTimes(1);
    expect(refreshInventory).toHaveBeenCalledTimes(1);
  });

  it("reports partial failure without discarding successful refreshes", async () => {
    const refreshModels = vi.fn().mockResolvedValue(undefined);
    const refreshHealth = vi.fn().mockRejectedValue(new Error("health unavailable"));
    const refreshInventory = vi.fn().mockResolvedValue(undefined);

    const message = await refreshChannelPanelData(refreshModels, refreshHealth, refreshInventory);

    expect(message).toContain("部分刷新失败（1/3）");
    expect(refreshModels).toHaveBeenCalledTimes(1);
    expect(refreshInventory).toHaveBeenCalledTimes(1);
  });

  it("仍会发起全部三路刷新，即使前两路已失败", async () => {
    const refreshModels = vi.fn().mockRejectedValue(new Error("models down"));
    const refreshHealth = vi.fn().mockRejectedValue(new Error("health down"));
    const refreshInventory = vi.fn().mockResolvedValue(undefined);

    const message = await refreshChannelPanelData(refreshModels, refreshHealth, refreshInventory);

    expect(message).toContain("部分刷新失败（2/3）");
    expect(refreshModels).toHaveBeenCalledTimes(1);
    expect(refreshHealth).toHaveBeenCalledTimes(1);
    expect(refreshInventory).toHaveBeenCalledTimes(1);
  });

  it("三路全失败时返回 3/3 文案，而不是抛出到调用方", async () => {
    const message = await refreshChannelPanelData(
      vi.fn().mockRejectedValue(new Error("a")),
      vi.fn().mockRejectedValue(new Error("b")),
      vi.fn().mockRejectedValue(new Error("c")),
    );
    expect(message).toBe("部分刷新失败（3/3），已保留成功更新的数据。");
  });
});

describe("ChannelsPanel sync and inventory ownership", () => {
  it("同步模型后才刷新健康概览，顺序不可颠倒", async () => {
    const order: string[] = [];
    const syncModels = vi.fn(async () => {
      order.push("models");
    });
    const refreshHealth = vi.fn(async () => {
      order.push("health");
    });

    await syncChannelModelsAndHealth(syncModels, refreshHealth);

    expect(order).toEqual(["models", "health"]);
    expect(syncModels).toHaveBeenCalledTimes(1);
    expect(refreshHealth).toHaveBeenCalledTimes(1);
  });

  it("模型同步失败时不刷新健康，避免用不一致数据覆盖", async () => {
    const refreshHealth = vi.fn().mockResolvedValue(undefined);
    await expect(
      syncChannelModelsAndHealth(vi.fn().mockRejectedValue(new Error("sync failed")), refreshHealth),
    ).rejects.toThrow("sync failed");
    expect(refreshHealth).not.toHaveBeenCalled();
  });

  it("inventory 已注入时不 autoload 模型；未注入且 active 时才加载", () => {
    const seeded: ImportedChannel[] = [
      { id: "c1", name: "A", sourceChannelId: "s1", upstreamKind: "newapi" } as ImportedChannel,
    ];
    expect(shouldAutoloadChannelModels(true, seeded)).toBe(false);
    expect(shouldAutoloadChannelModels(true, undefined)).toBe(true);
    expect(shouldAutoloadChannelModels(false, undefined)).toBe(false);
    expect(shouldAutoloadChannelModels(false, seeded)).toBe(false);
  });
});

describe("ChannelsPanel health presentation", () => {
  it("映射 health tone class，未知等级安全回落 success", () => {
    expect(healthToneClass("danger")).toBe("level-danger");
    expect(healthToneClass("warning")).toBe("level-warning");
    expect(healthToneClass("success")).toBe("level-success");
    expect(healthToneClass("info")).toBe("level-success");
    expect(healthToneClass("")).toBe("level-success");
  });

  it("只取 danger/warning 风险站点且最多 4 条", () => {
    const sites = [
      healthSite({ siteId: "ok", level: "success" }),
      healthSite({ siteId: "d1", level: "danger", siteName: "危险一" }),
      healthSite({ siteId: "w1", level: "warning", siteName: "警告一" }),
      healthSite({ siteId: "d2", level: "danger" }),
      healthSite({ siteId: "w2", level: "warning" }),
      healthSite({ siteId: "d3", level: "danger" }),
      healthSite({ siteId: "info", level: "info" }),
    ];
    const risks = topHealthRisks(sites);
    expect(risks.map((item) => item.siteId)).toEqual(["d1", "w1", "d2", "w2"]);
    expect(risks).toHaveLength(4);
    expect(risks.every((item) => item.level === "danger" || item.level === "warning")).toBe(true);
  });
});
