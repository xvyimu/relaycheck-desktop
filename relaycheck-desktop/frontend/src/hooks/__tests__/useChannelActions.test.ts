import { afterEach, expect, it, vi } from "vitest";

afterEach(() => {
  vi.resetModules();
  vi.restoreAllMocks();
  vi.doUnmock("react");
  vi.doUnmock("@/api/channels");
  vi.unstubAllGlobals();
});

function mockReactHooks() {
  vi.doMock("react", () => ({
    useCallback: (callback: unknown) => callback,
    useEffect: (effect: () => void | (() => void)) => {
      effect();
    },
    useState: (initial: unknown | (() => unknown)) => {
      const value = typeof initial === "function" ? initial() : initial;
      return [value, vi.fn()];
    },
  }));
}

/** 构造完整 channelsApi mock，避免 hook 访问 undefined 方法。 */
function mockChannelsApi(overrides: Record<string, unknown> = {}) {
  const channelsApi = {
    modelsOverview: vi.fn().mockResolvedValue({ items: [], models: [] }),
    syncModels: vi.fn().mockResolvedValue({ syncedChannels: 0, modelCount: 0, items: [], models: [] }),
    healthOverview: vi.fn(),
    healthOverviewPath: "/api/channels/health/overview",
    detect: vi.fn().mockResolvedValue({}),
    restoreSourceStatus: vi.fn().mockResolvedValue({}),
    archiveSourceStatus: vi.fn().mockResolvedValue({}),
    bulkSourceStatus: vi.fn().mockResolvedValue({ affected: 0 }),
    ...overrides,
  };
  vi.doMock("@/api/channels", () => ({ channelsApi }));
  return channelsApi;
}

it("refreshes only channel-owned model data via channelsApi", async () => {
  mockReactHooks();
  const channelsApi = mockChannelsApi();
  const { useChannelActions } = await import("../useChannelActions");
  const actions = useChannelActions({ active: false });
  await actions.refresh();

  expect(channelsApi.modelsOverview).toHaveBeenCalledTimes(1);
});

it("loads only models when active inventory data seeds channels", async () => {
  mockReactHooks();
  const channelsApi = mockChannelsApi();
  const { useChannelActions } = await import("../useChannelActions");
  useChannelActions({ active: true, initialChannels: [] });
  await vi.waitFor(() => expect(channelsApi.modelsOverview).toHaveBeenCalledTimes(1));
});

it("invalidates inventory exactly once after restore via channelsApi", async () => {
  mockReactHooks();
  const channelsApi = mockChannelsApi();
  const onInventoryRefresh = vi.fn().mockResolvedValue(undefined);
  const { useChannelActions } = await import("../useChannelActions");
  const actions = useChannelActions({ active: false, initialChannels: [], onInventoryRefresh });
  await actions.updateChannelSourceStatus(
    { id: "channel-1", name: "Channel", sourceChannelId: "source", upstreamKind: "newapi" } as never,
    "restore-source-status",
  );

  expect(channelsApi.restoreSourceStatus).toHaveBeenCalledWith("channel-1");
  expect(onInventoryRefresh).toHaveBeenCalledTimes(1);
});

it("archive requires confirm; cancel sends zero requests", async () => {
  mockReactHooks();
  const channelsApi = mockChannelsApi();
  vi.stubGlobal("window", { confirm: vi.fn().mockReturnValue(false) });
  const onInventoryRefresh = vi.fn();
  const { useChannelActions } = await import("../useChannelActions");
  const actions = useChannelActions({ active: false, initialChannels: [], onInventoryRefresh });
  await actions.updateChannelSourceStatus(
    { id: "channel-2", name: "Archivable", sourceChannelId: "source", upstreamKind: "newapi" } as never,
    "archive-source-status",
  );

  expect(channelsApi.archiveSourceStatus).not.toHaveBeenCalled();
  expect(onInventoryRefresh).not.toHaveBeenCalled();
});

it("bulkSourceStatus only sends declared fields after confirm", async () => {
  mockReactHooks();
  const channelsApi = mockChannelsApi({
    bulkSourceStatus: vi.fn().mockResolvedValue({ affected: 4 }),
  });
  vi.stubGlobal("window", { confirm: vi.fn().mockReturnValue(true) });
  const onInventoryRefresh = vi.fn().mockResolvedValue(undefined);
  const { useChannelActions } = await import("../useChannelActions");
  const actions = useChannelActions({ active: false, initialChannels: [], onInventoryRefresh });
  await actions.bulkUpdateSourceStatus("missing", "archived");

  expect(channelsApi.bulkSourceStatus).toHaveBeenCalledWith({ fromStatus: "missing", toStatus: "archived" });
  expect(onInventoryRefresh).toHaveBeenCalledTimes(1);
});

it("syncChannelModels uses channelsApi default limit and real schema fields", async () => {
  mockReactHooks();
  const channelsApi = mockChannelsApi({
    syncModels: vi.fn().mockResolvedValue({
      syncedChannels: 2,
      modelCount: 9,
      items: [],
      models: [],
    }),
  });
  const onInventoryRefresh = vi.fn().mockResolvedValue(undefined);
  const { useChannelActions } = await import("../useChannelActions");
  const actions = useChannelActions({ active: false, initialChannels: [], onInventoryRefresh });
  await actions.syncChannelModels();

  expect(channelsApi.syncModels).toHaveBeenCalledWith();
  expect(onInventoryRefresh).toHaveBeenCalledTimes(1);
});
