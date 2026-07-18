import { isValidElement, type ReactElement, type ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { ImportedChannel } from "@/types";
import type { ChannelFiltersResult } from "@/hooks/useChannelFilters";

type TestElementProps = {
  children?: ReactNode;
  disabled?: boolean;
  onClick?: () => void | Promise<void>;
  className?: string;
  type?: string;
};

const detect = vi.fn();

vi.mock("@/api/channels", () => ({
  channelsApi: {
    detect: (...args: unknown[]) => detect(...args),
  },
}));

vi.mock("@/components/ui/button", () => ({
  Button: (props: TestElementProps) => props as unknown as ReactElement,
}));
vi.mock("@/components/ui/progress", () => ({
  Progress: () => null,
}));
vi.mock("@/components/ui/tooltip", () => ({
  Tooltip: (props: { children?: ReactNode }) => props.children as ReactElement,
}));
vi.mock("../../loading-skeleton", () => ({
  LoadingSkeleton: () => null,
}));
vi.mock("@/lib/format", () => ({
  channelInitials: () => "RC",
  formatTime: (value: string) => value,
}));
vi.mock("@/lib/labels", () => ({
  channelModelStatusLabel: (value: string) => value,
  channelSourceLabel: (value: string) => value,
  channelSourceSyncLabel: (value: string) => value,
  upstreamKindLabel: (value: string) => value,
}));
vi.mock("@/lib/constants", () => ({
  CHANNELS_VISIBLE_INCREMENT: 24,
}));

afterEach(() => {
  vi.clearAllMocks();
});

function channel(overrides: Partial<ImportedChannel> = {}): ImportedChannel {
  return {
    id: "channel-1",
    name: "Alpha",
    sourceChannelId: "source-1",
    upstreamKind: "newapi",
    sourceSyncStatus: "active",
    baseUrl: "https://alpha.test",
    supportsCheckin: true,
    modelCount: 3,
    ...overrides,
  } as ImportedChannel;
}

function collectElements(node: ReactNode): ReactElement[] {
  if (Array.isArray(node)) return node.flatMap(collectElements);
  if (!isValidElement(node)) return [];
  const props = node.props as TestElementProps;
  return [node, ...collectElements(props.children)];
}

function textContent(node: ReactNode): string {
  if (typeof node === "string" || typeof node === "number") return String(node);
  if (Array.isArray(node)) return node.map(textContent).join("");
  if (!isValidElement(node)) return "";
  return textContent((node.props as TestElementProps).children);
}

function findControl(elements: ReactElement[], label: string): TestElementProps {
  const element = elements.find((item) => textContent((item.props as TestElementProps).children) === label);
  expect(element, `missing control ${label}`).toBeTruthy();
  return element!.props as TestElementProps;
}

function makeFilters(overrides: Partial<ChannelFiltersResult> = {}): ChannelFiltersResult {
  const channels = overrides.displayedChannels || [channel()];
  return {
    query: "",
    setQuery: vi.fn(),
    setQueryComposing: vi.fn(),
    accountSearchLoading: false,
    accountSearchTruncated: false,
    accountSearchError: "",
    sourceStatusFilter: "not_archived",
    setSourceStatusFilter: vi.fn(),
    kindFilter: "target_relay",
    setKindFilter: vi.fn(),
    healthFilter: "all",
    setHealthFilter: vi.fn(),
    kindOptions: [],
    visibleChannels: channels,
    displayedChannels: channels,
    hasMoreChannels: false,
    visibleLimit: 50,
    setVisibleLimit: vi.fn(),
    identifiedCount: channels.length,
    checkinCount: 0,
    targetRelayCount: 0,
    missingBaseUrlCount: 0,
    sourceMissingCount: 0,
    sourceArchivedCount: 0,
    healthRiskCount: 0,
    clearFilters: vi.fn(),
    ...overrides,
  } as ChannelFiltersResult;
}

async function renderTable(
  options: {
    filters?: ChannelFiltersResult;
    channels?: ImportedChannel[];
    onSetDrawer?: ReturnType<typeof vi.fn>;
    onSetMessage?: ReturnType<typeof vi.fn>;
    onRefresh?: ReturnType<typeof vi.fn>;
    onUpdateSourceStatus?: ReturnType<typeof vi.fn>;
  } = {},
) {
  const { ChannelTable } = await import("../ChannelTable");
  const channels = options.channels || [channel()];
  const props = {
    channels,
    loaded: true,
    message: "",
    onSetDrawer: options.onSetDrawer || vi.fn(),
    onSetMessage: options.onSetMessage || vi.fn(),
    onRefresh: options.onRefresh || vi.fn().mockResolvedValue(undefined),
    onUpdateSourceStatus: options.onUpdateSourceStatus || vi.fn(),
    filters: options.filters || makeFilters({ displayedChannels: channels, visibleChannels: channels }),
  };
  const tree = ChannelTable(props as never);
  return { tree, props };
}

describe("ChannelTable behavior", () => {
  it("无 baseUrl 时禁用识别按钮", async () => {
    const item = channel({ baseUrl: "" });
    const { tree } = await renderTable({ channels: [item] });
    const detectBtn = findControl(collectElements(tree), "识别并生成站点");
    expect(detectBtn.disabled).toBe(true);
  });

  it("识别成功只调 channelsApi.detect 并刷新", async () => {
    detect.mockResolvedValue({});
    const onSetMessage = vi.fn();
    const onRefresh = vi.fn().mockResolvedValue(undefined);
    const { tree } = await renderTable({ onSetMessage, onRefresh });
    await findControl(collectElements(tree), "识别并生成站点").onClick?.();

    await vi.waitFor(() => {
      expect(detect).toHaveBeenCalledWith("channel-1");
      expect(onSetMessage).toHaveBeenCalledWith("Alpha 已识别并同步到上游站点");
      expect(onRefresh).toHaveBeenCalledTimes(1);
    });
  });

  it("识别失败写入错误文案且不抛未处理拒绝", async () => {
    detect.mockRejectedValue(new Error("上游拒绝"));
    const onSetMessage = vi.fn();
    const onRefresh = vi.fn();
    const { tree } = await renderTable({ onSetMessage, onRefresh });
    await findControl(collectElements(tree), "识别并生成站点").onClick?.();

    await vi.waitFor(() => {
      expect(onSetMessage).toHaveBeenCalledWith("识别失败：上游拒绝");
      expect(onRefresh).not.toHaveBeenCalled();
    });
  });

  it("详情打开 drawer；missing 状态走源状态回调", async () => {
    const onSetDrawer = vi.fn();
    const onUpdateSourceStatus = vi.fn();
    const item = channel({ sourceSyncStatus: "missing" });
    const { tree } = await renderTable({
      channels: [item],
      onSetDrawer,
      onUpdateSourceStatus,
    });
    const elements = collectElements(tree);
    await findControl(elements, "详情").onClick?.();
    expect(onSetDrawer).toHaveBeenCalledWith({ kind: "channel", channel: item });

    await findControl(elements, "恢复活跃").onClick?.();
    expect(onUpdateSourceStatus).toHaveBeenCalledWith(item, "restore-source-status");

    await findControl(elements, "归档保留").onClick?.();
    expect(onUpdateSourceStatus).toHaveBeenCalledWith(item, "archive-source-status");
  });

  it("加载更多按 CHANNELS_VISIBLE_INCREMENT 递增", async () => {
    const setVisibleLimit = vi.fn();
    const filters = makeFilters({
      hasMoreChannels: true,
      displayedChannels: [channel()],
      visibleChannels: [channel(), channel()],
      setVisibleLimit,
    });
    const { tree } = await renderTable({ filters });
    const loadMore = collectElements(tree).find((item) => {
      const props = item.props as TestElementProps;
      return typeof props.onClick === "function" && textContent(props.children).includes("加载更多渠道");
    });
    expect(loadMore, "missing load more").toBeTruthy();
    await (loadMore!.props as TestElementProps).onClick?.();
    expect(setVisibleLimit).toHaveBeenCalledOnce();
    const updater = setVisibleLimit.mock.calls[0][0] as (current: number) => number;
    expect(updater(50)).toBe(74);
  });
});
