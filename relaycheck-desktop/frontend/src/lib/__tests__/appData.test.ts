import { describe, expect, it, vi } from "vitest";

import { appIsInitialLoading, refreshAppData } from "@/lib/appData";

describe("app data orchestration", () => {
  it("does not show the startup loading screen after the initial data load", () => {
    expect(
      appIsInitialLoading(
        { loaded: true, loading: true },
        { loaded: true, loading: true },
        { loaded: true, loading: false },
      ),
    ).toBe(false);
  });

  it("includes model usage data in the global refresh", async () => {
    const system = { refresh: vi.fn().mockResolvedValue(undefined) };
    const inventory = { refresh: vi.fn().mockResolvedValue(undefined) };
    const ops = { refresh: vi.fn().mockResolvedValue(undefined) };
    const modelUsage = { refresh: vi.fn().mockResolvedValue(undefined) };

    await refreshAppData(system, inventory, ops, modelUsage);

    expect(system.refresh).toHaveBeenCalledTimes(1);
    expect(inventory.refresh).toHaveBeenCalledTimes(1);
    expect(ops.refresh).toHaveBeenCalledTimes(1);
    expect(modelUsage.refresh).toHaveBeenCalledTimes(1);
  });
});
