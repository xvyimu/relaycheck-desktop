import { describe, expect, it, vi } from "vitest";

import { appIsInitialLoading, refreshAppData } from "@/lib/appData";

describe("app data orchestration", () => {
  it("shows shell after system load even if inventory still loading", () => {
    expect(
      appIsInitialLoading(
        { loaded: true, loading: false },
        { loaded: false, loading: true },
        { loaded: false, loading: true },
      ),
    ).toBe(false);
  });

  it("keeps startup screen until system is loaded", () => {
    expect(
      appIsInitialLoading(
        { loaded: false, loading: true },
        { loaded: true, loading: false },
        { loaded: true, loading: false },
      ),
    ).toBe(true);
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
