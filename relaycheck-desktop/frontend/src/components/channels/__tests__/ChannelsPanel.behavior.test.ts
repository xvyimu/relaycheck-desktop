import { isValidElement, type ReactElement, type ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { ChannelHealthOverview, ImportedChannel } from "@/types";

type TestElementProps = {
  children?: ReactNode;
  disabled?: boolean;
  onClick?: () => void | Promise<void>;
  type?: string;
};

const stateSlots: unknown[] = [];
let stateIndex = 0;
const effectFns: Array<() => void | (() => void)> = [];

const refreshActions = vi.fn().mockResolvedValue(undefined);
const setDrawer = vi.fn();
const setMessage = vi.fn();
const syncChannelModels = vi.fn().mockResolvedValue(undefined);
const updateChannelSourceStatus = vi.fn();

const filters = {
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
  kindOptions: [] as string[],
  visibleChannels: [] as ImportedChannel[],
  displayedChannels: [] as ImportedChannel[],
  hasMoreChannels: false,
  visibleLimit: 50,
  setVisibleLimit: vi.fn(),
  identifiedCount: 0,
  checkinCount: 0,
  targetRelayCount: 0,
  missingBaseUrlCount: 0,
  sourceMissingCount: 0,
  sourceArchivedCount: 0,
  healthRiskCount: 0,
  clearFilters: vi.fn(),
};

const refreshHealth = vi.fn().mockResolvedValue(undefined);
const healthData: ChannelHealthOverview = {
  generatedAt: "2026-07-18T04:00:00Z",
  overall: "warning",
  siteCount: 2,
  healthySiteCount: 1,
  unreachableSiteCount: 1,
  channelCount: 3,
  liveModelChannelCount: 1,
  failedModelChannelCount: 1,
  uncheckedModelChannelCount: 1,
  validKeyCount: 1,
  invalidKeyCount: 1,
  uncheckedKeyCount: 0,
  sites: [
    {
      siteId: "risk-1",
      siteName: "风险站点",
      baseUrl: "https://risk.example",
      kind: "newapi",
      level: "danger",
      healthStatus: "unreachable",
      accountCount: 2,
      validKeyCount: 0,
      invalidKeyCount: 1,
      uncheckedKeyCount: 0,
      modelChannelCount: 1,
      liveModelChannelCount: 0,
      failedModelChannelCount: 1,
      uncheckedModelChannelCount: 0,
      modelCount: 0,
      recommendedAction: "检查上游连通性",
    },
  ],
};

const startTask = vi.fn().mockResolvedValue(true);
const cancelTask = vi.fn();
const resetTask = vi.fn();

vi.mock("react", async () => {
  const actual = await vi.importActual<typeof import("react")>("react");
  return {
    ...actual,
    useState: (initial: unknown) => {
      const index = stateIndex++;
      if (!(index in stateSlots)) {
        stateSlots[index] = typeof initial === "function" ? (initial as () => unknown)() : initial;
      }
      const setState = (value: unknown) => {
        stateSlots[index] =
          typeof value === "function" ? (value as (previous: unknown) => unknown)(stateSlots[index]) : value;
      };
      return [stateSlots[index], setState];
    },
    useEffect: (effect: () => void | (() => void)) => {
      effectFns.push(effect);
    },
    useMemo: (factory: () => unknown) => factory(),
    useCallback: (callback: unknown) => callback,
    memo: (component: unknown) => component,
  };
});

vi.mock("@/hooks/useChannelActions", () => ({
  useChannelActions: () => ({
    channels: [] as ImportedChannel[],
    modelOverview: null,
    modelSyncing: false,
    message: "",
    loaded: true,
    drawer: null,
    setDrawer,
    setMessage,
    refresh: refreshActions,
    syncChannelModels,
    updateChannelSourceStatus,
    bulkUpdateSourceStatus: vi.fn(),
  }),
}));

vi.mock("@/hooks/useChannelFilters", () => ({
  useChannelFilters: () => filters,
}));

vi.mock("@/hooks/useApi", () => ({
  useApi: (url: string) => {
    // 行为测试断言面板必须通过 channelsApi 路径消费健康概览。
    if (url !== "/api/channels/health/overview") {
      throw new Error(`unexpected health url: ${url}`);
    }
    return {
      data: healthData,
      loading: false,
      error: "",
      refresh: refreshHealth,
    };
  },
}));

vi.mock("@/hooks/useTaskProgress", () => ({
  useTaskProgress: () => ({
    loading: false,
    error: "",
    progress: null,
    startTask,
    cancelTask,
    reset: resetTask,
  }),
}));

vi.mock("@/components/channels/ChannelTable", () => ({
  ChannelTable: () => null,
}));
vi.mock("@/components/ui/dialog-shell", () => ({
  DialogShell: (props: TestElementProps) => props as unknown as ReactElement,
}));
vi.mock("@/components/ui/button", () => ({
  Button: (props: TestElementProps) => props as unknown as ReactElement,
}));
vi.mock("@/components/ui/TaskProgressView", () => ({
  TaskProgressView: () => null,
}));

afterEach(() => {
  stateSlots.length = 0;
  stateIndex = 0;
  effectFns.length = 0;
  vi.clearAllMocks();
});

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

/** 渲染渠道面板并执行挂载 effect。 */
async function renderPanel(
  props: {
    onRefresh?: () => Promise<void>;
    active?: boolean;
    inventoryChannels?: ImportedChannel[];
  } = {},
) {
  stateIndex = 0;
  effectFns.length = 0;
  const { ChannelsPanel } = await import("../ChannelsPanel");
  // memo 包装后的组件在 mock 后仍可直接以函数调用做静态渲染。
  const Component = ChannelsPanel as unknown as (p: {
    onRefresh: () => Promise<void>;
    active?: boolean;
    inventoryChannels?: ImportedChannel[];
  }) => ReactElement;
  const onRefresh = props.onRefresh ?? vi.fn().mockResolvedValue(undefined);
  const tree = Component({
    onRefresh,
    active: props.active ?? true,
    inventoryChannels: props.inventoryChannels,
  });
  for (const effect of effectFns.splice(0)) {
    effect();
  }
  return { tree, onRefresh };
}

describe("ChannelsPanel behavior", () => {
  it("inventory 注入时不 autoload 模型；未注入时会 refreshActions 一次", async () => {
    await renderPanel({ inventoryChannels: [] });
    expect(refreshActions).not.toHaveBeenCalled();

    refreshActions.mockClear();
    await renderPanel({ inventoryChannels: undefined });
    expect(refreshActions).toHaveBeenCalledTimes(1);
  });

  it("inactive 时不 autoload，避免 keep-alive 后台请求", async () => {
    await renderPanel({ active: false, inventoryChannels: undefined });
    expect(refreshActions).not.toHaveBeenCalled();
  });

  it("健康卡片展示风险站点，且不请求全量 /api/accounts", async () => {
    const { tree } = await renderPanel({ inventoryChannels: [] });
    const text = textContent(tree);
    expect(text).toContain("风险站点");
    expect(text).toContain("检查上游连通性");
    expect(text).toContain("不可达");
    expect(text).not.toContain("/api/accounts");
  });

  it("点击刷新会并行刷新 models/health/inventory", async () => {
    const onRefresh = vi.fn().mockResolvedValue(undefined);
    const { tree } = await renderPanel({ onRefresh, inventoryChannels: [] });
    await findControl(collectElements(tree), "刷新").onClick?.();

    await vi.waitFor(() => {
      expect(refreshActions).toHaveBeenCalled();
      expect(refreshHealth).toHaveBeenCalled();
      expect(onRefresh).toHaveBeenCalled();
    });
  });

  it("健康区同步模型会先 sync 再 refreshHealth", async () => {
    const order: string[] = [];
    syncChannelModels.mockImplementation(async () => {
      order.push("models");
    });
    refreshHealth.mockImplementation(async () => {
      order.push("health");
    });

    const { tree } = await renderPanel({ inventoryChannels: [] });
    // 健康区与工具栏各有一个“同步模型”，取第一个（健康卡片工具栏）。
    const syncButtons = collectElements(tree).filter(
      (item) => textContent((item.props as TestElementProps).children) === "同步模型",
    );
    expect(syncButtons.length).toBeGreaterThanOrEqual(1);
    await (syncButtons[0].props as TestElementProps).onClick?.();

    await vi.waitFor(() => {
      expect(order).toEqual(["models", "health"]);
    });
  });

  it("探测健康启动 task 且参数固定 limit=20", async () => {
    const { tree } = await renderPanel({ inventoryChannels: [] });
    await findControl(collectElements(tree), "探测健康").onClick?.();

    await vi.waitFor(() => {
      expect(startTask).toHaveBeenCalledWith("channel_health_probe", { limit: 20, onlyRisky: false });
      expect(String(stateSlots[0])).toContain("健康探测任务已启动");
    });
  });
});
