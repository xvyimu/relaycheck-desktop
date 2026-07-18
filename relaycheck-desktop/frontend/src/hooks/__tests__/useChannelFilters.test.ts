import { afterEach, expect, it, vi } from "vitest";

import type { ImportedChannel, NavigationIntent } from "@/types";

afterEach(() => {
  vi.resetModules();
  vi.restoreAllMocks();
  vi.doUnmock("react");
  vi.doUnmock("@/hooks/useAccountSiteSearch");
});

function channel(overrides: Partial<ImportedChannel> = {}): ImportedChannel {
  return {
    id: "channel-1",
    name: "Alpha Relay",
    sourceChannelId: "source-1",
    upstreamKind: "newapi",
    sourceSyncStatus: "active",
    baseUrl: "https://alpha.test",
    supportsCheckin: true,
    modelsStatus: "ok",
    ...overrides,
  } as ImportedChannel;
}

function mockFiltersRuntime(initialValues: unknown[] = []) {
  const setters: Array<ReturnType<typeof vi.fn>> = [];
  let stateIndex = 0;

  vi.doMock("react", () => ({
    useEffect: (effect: () => void | (() => void)) => effect(),
    useMemo: (factory: () => unknown) => factory(),
    useState: (initial: unknown | (() => unknown)) => {
      const fallback = typeof initial === "function" ? (initial as () => unknown)() : initial;
      const value = stateIndex < initialValues.length ? initialValues[stateIndex] : fallback;
      const setter = vi.fn();
      setters.push(setter);
      stateIndex += 1;
      return [value, setter];
    },
  }));
  vi.doMock("@/hooks/useAccountSiteSearch", () => ({
    useAccountSiteSearch: vi.fn().mockReturnValue({
      data: { items: [], truncated: false },
      searchedQuery: "",
      loading: false,
      error: "",
    }),
  }));

  return { setters };
}

it("computes channel totals and applies default relay/archive filters", async () => {
  mockFiltersRuntime();
  const channels = [
    channel(),
    channel({ id: "channel-2", name: "Unknown", upstreamKind: "unknown", supportsCheckin: false }),
    channel({ id: "channel-3", name: "Archived", sourceSyncStatus: "archived" }),
    channel({ id: "channel-4", name: "Risk", modelsStatus: "failed", baseUrl: "" }),
  ];

  const { useChannelFilters } = await import("../useChannelFilters");
  const result = useChannelFilters(channels);

  expect(result.identifiedCount).toBe(3);
  expect(result.checkinCount).toBe(3);
  expect(result.targetRelayCount).toBe(3);
  expect(result.missingBaseUrlCount).toBe(1);
  expect(result.sourceArchivedCount).toBe(1);
  expect(result.healthRiskCount).toBe(1);
  expect(result.visibleChannels.map((item) => item.id)).toEqual(["channel-1", "channel-4"]);
  expect(result.kindOptions).toEqual(["newapi", "unknown"]);
});

it("matches channel raw metadata and exposes pagination state", async () => {
  mockFiltersRuntime(["needle", false, "all", "all", "all", 1]);
  const channels = [
    channel({ id: "one", rawJson: JSON.stringify({ config: { note: "needle-user" } }) }),
    channel({ id: "two", name: "Other Relay" }),
  ];

  const { useChannelFilters } = await import("../useChannelFilters");
  const result = useChannelFilters(channels);

  expect(result.visibleChannels.map((item) => item.id)).toEqual(["one"]);
  expect(result.displayedChannels).toHaveLength(1);
  expect(result.hasMoreChannels).toBe(false);
});

it("applies risk navigation intent and resets filters", async () => {
  const { setters } = mockFiltersRuntime();
  const intent: NavigationIntent = { target: "channels", siteHealth: "risk" } as NavigationIntent;

  const { useChannelFilters } = await import("../useChannelFilters");
  const result = useChannelFilters([channel()], intent);

  expect(setters[2]).toHaveBeenCalledWith("not_archived");
  expect(setters[3]).toHaveBeenCalledWith("target_relay");
  expect(setters[4]).toHaveBeenCalledWith("risk");
  expect(setters[0]).toHaveBeenCalledWith("");

  result.clearFilters();
  expect(setters[0]).toHaveBeenLastCalledWith("");
  expect(setters[2]).toHaveBeenLastCalledWith("not_archived");
  expect(setters[3]).toHaveBeenLastCalledWith("target_relay");
  expect(setters[4]).toHaveBeenLastCalledWith("all");
});
