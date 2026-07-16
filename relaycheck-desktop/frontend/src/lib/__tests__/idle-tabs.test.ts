import { describe, expect, it } from "vitest";
import { hasEvictableTabs, IDLE_TAB_TTL_MS, pruneIdleTabs } from "@/lib/idle-tabs";

const PINNED = new Set(["dashboard"]);

describe("idle tab keep-alive policy", () => {
  it("keeps dashboard pinned and active tab, drops expired others", () => {
    const now = 1_000_000;
    const last = new Map<string, number>([
      ["dashboard", now - IDLE_TAB_TTL_MS * 2],
      ["settings", now - IDLE_TAB_TTL_MS * 2],
      ["channels", now - 1000],
    ]);
    const visited = new Set(["dashboard", "settings", "channels"]);
    const next = pruneIdleTabs(visited, "channels", last, now, { pinned: PINNED });
    expect(next).not.toBeNull();
    expect(next!.has("dashboard")).toBe(true);
    expect(next!.has("channels")).toBe(true);
    expect(next!.has("settings")).toBe(false);
  });

  it("returns null when nothing expired", () => {
    const now = 1_000_000;
    const last = new Map([
      ["dashboard", now],
      ["channels", now],
    ]);
    const visited = new Set(["dashboard", "channels"]);
    expect(pruneIdleTabs(visited, "channels", last, now, { pinned: PINNED })).toBeNull();
  });

  it("detects evictable tabs", () => {
    expect(hasEvictableTabs(new Set(["dashboard"]), PINNED)).toBe(false);
    expect(hasEvictableTabs(new Set(["dashboard", "settings"]), PINNED)).toBe(true);
  });

  it("uses 5 minute TTL", () => {
    expect(IDLE_TAB_TTL_MS).toBe(300_000);
  });
});
