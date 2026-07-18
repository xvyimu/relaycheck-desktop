import { isValidElement, type ReactElement, type ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { Account, KeyExportPreview, ModelOverview, ModelPricingOverview } from "@/types";
import type { UnsupportedCheckinCleanupPreview } from "@/api/account-cleanup";

type TestElementProps = {
  children?: ReactNode;
  disabled?: boolean;
  onClick?: () => void | Promise<void>;
  type?: string;
  "aria-expanded"?: boolean;
};

const stateSlots: unknown[] = [];
let stateIndex = 0;
const effectFns: Array<() => void | (() => void)> = [];

const modelsApi = {
  overview: vi.fn(),
  pricing: vi.fn(),
  sync: vi.fn(),
  syncPricing: vi.fn(),
};

const keysApi = {
  exportPreview: vi.fn(),
};

const accountCleanupApi = {
  preview: vi.fn(),
  confirm: vi.fn(),
};

const taskProgress = {
  loading: false,
  error: "",
  progress: null,
  startTask: vi.fn(),
  cancelTask: vi.fn(),
  reset: vi.fn(),
};

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
  };
});

vi.mock("@/api/models", () => ({ modelsApi }));
vi.mock("@/api/keys", () => ({ keysApi }));
vi.mock("@/api/account-cleanup", () => ({ accountCleanupApi }));
vi.mock("@/hooks/useTaskProgress", () => ({
  useTaskProgress: () => taskProgress,
}));
vi.mock("@/components/ui/TaskProgressView", () => ({
  TaskProgressView: () => null,
}));
vi.mock("@/components/ui/button", () => ({
  Button: (props: TestElementProps) => props as unknown as ReactElement,
}));
vi.mock("@/components/ui/empty-state", () => ({
  EmptyState: () => null,
}));
// 组件以函数方式直接渲染时不会执行子组件函数；用 props 形状定位清理面板。
vi.mock("@/components/accounts/UnsupportedCheckinCleanupPanel", () => ({
  UnsupportedCheckinCleanupPanel: function MockUnsupportedCheckinCleanupPanel() {
    return null;
  },
}));

afterEach(() => {
  stateSlots.length = 0;
  stateIndex = 0;
  effectFns.length = 0;
  vi.clearAllMocks();
});

/** 构造当前页账号夹具；默认带指纹，便于展开洞察后触发模型加载。 */
function makeAccount(overrides: Partial<Account> = {}): Account {
  return {
    id: "account-1",
    upstreamSiteId: "site-1",
    upstreamSiteName: "Relay One",
    upstreamSiteBaseUrl: "https://relay.example",
    upstreamSiteKind: "newapi",
    displayName: "账号一",
    authType: "api_key",
    loginStatus: "valid",
    lastCheckinStatus: "success",
    apiKeyFingerprint: "fp-1",
    apiKeyStatus: "valid",
    apiKeyLatencyMs: 42,
    apiKeyModelCount: 2,
    apiKeyModelUsable: true,
    apiKeySampleModels: ["gpt-4o-mini"],
    apiKeyTestModel: "gpt-4o-mini",
    ...overrides,
  } as Account;
}

/** 递归收集元素，定位工具栏按钮与清理面板。 */
function collectElements(node: ReactNode): ReactElement[] {
  if (Array.isArray(node)) return node.flatMap(collectElements);
  if (!isValidElement(node)) return [];
  const props = node.props as TestElementProps;
  return [node, ...collectElements(props.children)];
}

/** 提取可见文本以定位按钮，避免绑定内部 class 结构。 */
function textContent(node: ReactNode): string {
  if (typeof node === "string" || typeof node === "number") return String(node);
  if (Array.isArray(node)) return node.map(textContent).join("");
  if (!isValidElement(node)) return "";
  return textContent((node.props as TestElementProps).children);
}

/** 按完整按钮文案查找控件。 */
function findControl(elements: ReactElement[], label: string): TestElementProps {
  const element = elements.find((item) => textContent((item.props as TestElementProps).children) === label);
  expect(element, `missing control ${label}`).toBeTruthy();
  return element!.props as TestElementProps;
}

/** 渲染 AccountInsights，并在每次渲染前重置 useState 游标。 */
async function renderInsights(accounts: Account[], onDone = vi.fn()) {
  stateIndex = 0;
  effectFns.length = 0;
  const { AccountInsights } = await import("../AccountInsights");
  const tree = AccountInsights({ accounts, onDone });
  for (const effect of effectFns.splice(0)) {
    effect();
  }
  return { tree, onDone, AccountInsights };
}

describe("AccountInsights behavior", () => {
  it("展开洞察后通过 modelsApi 拉取模型与价格概览，不直接拼 URL", async () => {
    modelsApi.overview.mockResolvedValue({
      generatedAt: "2026-07-18T02:00:00Z",
      modelCount: 1,
      accountCount: 1,
      validKeyCount: 1,
      usableModelCount: 1,
      models: [],
      sites: [],
      priceHints: [],
    } satisfies ModelOverview);
    modelsApi.pricing.mockResolvedValue({
      generatedAt: "2026-07-18T02:00:00Z",
      sourceCount: 1,
      modelCount: 1,
      exactCount: 1,
      ratioCount: 0,
      sources: [],
    } satisfies ModelPricingOverview);

    const first = await renderInsights([makeAccount()]);
    const expand = findControl(collectElements(first.tree), "展开洞察");
    await expand.onClick?.();

    const second = await renderInsights([makeAccount()]);
    await vi.waitFor(() => {
      expect(modelsApi.overview).toHaveBeenCalledOnce();
      expect(modelsApi.pricing).toHaveBeenCalledOnce();
    });
    expect(textContent(second.tree)).toContain("收起洞察");
  });

  it("同步模型/密钥只调用 modelsApi.sync 与 pricing，默认 limit 由 adapter 持有", async () => {
    modelsApi.sync.mockResolvedValue({
      generatedAt: "2026-07-18T02:00:00Z",
      syncedAccounts: 1,
      modelCount: 2,
      accountCount: 1,
      validKeyCount: 1,
      usableModelCount: 1,
      models: [],
      sites: [],
      priceHints: [],
    } satisfies ModelOverview);
    modelsApi.pricing.mockResolvedValue({
      generatedAt: "2026-07-18T02:00:00Z",
      sourceCount: 2,
      modelCount: 2,
      exactCount: 1,
      ratioCount: 1,
      sources: [],
    } satisfies ModelPricingOverview);

    const first = await renderInsights([makeAccount()]);
    await findControl(collectElements(first.tree), "展开洞察").onClick?.();
    const expanded = await renderInsights([makeAccount()]);
    await findControl(collectElements(expanded.tree), "同步模型/密钥").onClick?.();

    await vi.waitFor(() => {
      expect(modelsApi.sync).toHaveBeenCalledWith({ limit: 50 });
      expect(modelsApi.pricing).toHaveBeenCalled();
    });
  });

  it("Key 导出预览只走 keysApi，不直接请求 /api/keys/export-preview 字符串", async () => {
    keysApi.exportPreview.mockResolvedValue({
      generatedAt: "2026-07-18T02:00:00Z",
      total: 1,
      valid: 1,
      usable: 1,
      items: [],
      notice: "脱敏",
    } satisfies KeyExportPreview);

    const first = await renderInsights([makeAccount()]);
    await findControl(collectElements(first.tree), "展开洞察").onClick?.();
    const expanded = await renderInsights([makeAccount()]);
    await findControl(collectElements(expanded.tree), "预览").onClick?.();

    await vi.waitFor(() => {
      expect(keysApi.exportPreview).toHaveBeenCalledOnce();
    });
  });

  it("确认清理失败会清空 preview 状态并提示重新预览", async () => {
    const error = Object.assign(new Error("候选已变化"), { status: 409 });
    accountCleanupApi.confirm.mockRejectedValue(error);

    // 直接写入 cleanupPreview 状态槽（message, expanded, showDetails, keyTestBusyId,
    // modelOverview, pricingOverview, modelSyncBusy, pricingSyncBusy, keyExportPreview,
    // keyExportBusy, cleanupPreview, cleanupBusy, cleanupIncludeLastUnsupported）。
    stateSlots[0] = "";
    stateSlots[1] = true;
    stateSlots[2] = false;
    stateSlots[3] = "";
    stateSlots[4] = null;
    stateSlots[5] = null;
    stateSlots[6] = false;
    stateSlots[7] = false;
    stateSlots[8] = null;
    stateSlots[9] = false;
    stateSlots[10] = {
      previewId: "cleanup-preview-1",
      expiresAt: "2026-07-18T02:05:00Z",
      matched: 1,
      deleted: 0,
      limit: 10,
      hasMore: false,
      includeLastUnsupported: true,
      items: [],
    } satisfies UnsupportedCheckinCleanupPreview;
    stateSlots[11] = false;
    stateSlots[12] = true;

    const { tree } = await renderInsights([makeAccount({ lastCheckinStatus: "unsupported" })]);
    type CleanupPanelProps = {
      onConfirm?: (previewId: string) => void | Promise<void>;
      onPreview?: () => void | Promise<void>;
      preview?: UnsupportedCheckinCleanupPreview | null;
    };
    const cleanup = collectElements(tree).find((item) => {
      const props = item.props as CleanupPanelProps;
      return typeof props.onConfirm === "function" && typeof props.onPreview === "function";
    });
    expect(cleanup).toBeTruthy();
    const props = cleanup!.props as CleanupPanelProps;
    expect(props.preview?.previewId).toBe("cleanup-preview-1");
    await props.onConfirm?.(props.preview!.previewId);

    await vi.waitFor(() => {
      expect(accountCleanupApi.confirm).toHaveBeenCalledWith("cleanup-preview-1");
      expect(stateSlots[10]).toBeNull();
      expect(String(stateSlots[0])).toContain("请重新预览");
    });
  });
});
