import { isValidElement, type ReactElement, type ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { ChannelModelOverview } from "@/types";
import type { ImportFromAdminResult } from "@/api/local-newapi";

type TestElementProps = {
  children?: ReactNode;
  disabled?: boolean;
  onClick?: () => void | Promise<void>;
  onChange?: (event: {
    target: { value?: string; checked?: boolean };
    currentTarget: { value?: string; checked?: boolean };
  }) => void;
  onSubmit?: (event: { preventDefault: () => void }) => void;
  type?: string;
  value?: string;
  checked?: boolean;
  open?: boolean;
};

const stateSlots: unknown[] = [];
let stateIndex = 0;
const effectFns: Array<() => void | (() => void)> = [];

const localNewapiApi = {
  importFromAdmin: vi.fn(),
};

const channelsApi = {
  modelsOverview: vi.fn(),
  syncModels: vi.fn(),
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
    useRef: (initial: unknown) => ({ current: initial }),
  };
});

vi.mock("@/api/local-newapi", () => ({ localNewapiApi }));
vi.mock("@/api/channels", () => ({ channelsApi }));
vi.mock("@/components/ui/dialog-shell", () => ({
  DialogShell: (props: TestElementProps) => props as unknown as ReactElement,
}));
vi.mock("@/components/ui/button", () => ({
  Button: (props: TestElementProps) => props as unknown as ReactElement,
}));
vi.mock("@/components/ui/line-icon", () => ({
  LineIcon: () => null,
}));

afterEach(() => {
  stateSlots.length = 0;
  stateIndex = 0;
  effectFns.length = 0;
  vi.clearAllMocks();
  vi.unstubAllGlobals();
});

/** 提供最小 window/localStorage，避免 Node 环境触发引导副作用时报错。 */
function stubBrowserGlobals() {
  const store = new Map<string, string>();
  vi.stubGlobal("window", {
    localStorage: {
      getItem: (key: string) => store.get(key) ?? null,
      setItem: (key: string, value: string) => {
        store.set(key, value);
      },
      removeItem: (key: string) => {
        store.delete(key);
      },
    },
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    requestAnimationFrame: (callback: FrameRequestCallback) => {
      callback(0);
      return 1;
    },
    cancelAnimationFrame: vi.fn(),
    dispatchEvent: vi.fn(),
  });
}

/** 递归收集元素，按按钮文案与表单控件定位交互入口。 */
function collectElements(node: ReactNode): ReactElement[] {
  if (Array.isArray(node)) return node.flatMap(collectElements);
  if (!isValidElement(node)) return [];
  const props = node.props as TestElementProps;
  return [node, ...collectElements(props.children)];
}

/** 提取可见文本。 */
function textContent(node: ReactNode): string {
  if (typeof node === "string" || typeof node === "number") return String(node);
  if (Array.isArray(node)) return node.map(textContent).join("");
  if (!isValidElement(node)) return "";
  return textContent((node.props as TestElementProps).children);
}

/** 按完整文案查找控件。 */
function findControl(elements: ReactElement[], label: string): TestElementProps {
  const element = elements.find((item) => textContent((item.props as TestElementProps).children) === label);
  expect(element, `missing control ${label}`).toBeTruthy();
  return element!.props as TestElementProps;
}

/**
 * 渲染 OnboardingWizard。
 * useState 顺序：open, stepIndex, busy, message, error, baseUrl, accessToken, saveToken。
 */
async function renderWizard(onNavigate = vi.fn()) {
  stubBrowserGlobals();
  stateIndex = 0;
  effectFns.length = 0;
  const { OnboardingWizard } = await import("../OnboardingWizard");
  const tree = OnboardingWizard({ onNavigate });
  for (const effect of effectFns.splice(0)) {
    effect();
  }
  return { tree, onNavigate };
}

/** 写入已打开状态与表单字段，便于直接触发当前步骤动作。 */
function seedOpenStep(options: {
  stepIndex: number;
  baseUrl?: string;
  accessToken?: string;
  saveToken?: boolean;
  message?: string;
  error?: string;
}) {
  stateSlots[0] = true;
  stateSlots[1] = options.stepIndex;
  stateSlots[2] = false;
  stateSlots[3] = options.message ?? "";
  stateSlots[4] = options.error ?? "";
  stateSlots[5] = options.baseUrl ?? "";
  stateSlots[6] = options.accessToken ?? "";
  stateSlots[7] = options.saveToken ?? true;
}

describe("OnboardingWizard behavior", () => {
  it("连接步骤校验空表单，不调用 localNewapiApi", async () => {
    seedOpenStep({ stepIndex: 0, baseUrl: "", accessToken: "" });
    const { tree } = await renderWizard();
    await findControl(collectElements(tree), "执行").onClick?.();

    expect(localNewapiApi.importFromAdmin).not.toHaveBeenCalled();
    await vi.waitFor(() => {
      expect(String(stateSlots[4])).toContain("请填写 NewAPI 后台地址和访问令牌");
    });
  });

  it("连接步骤通过 localNewapiApi 导入，并展示后端计数文案", async () => {
    localNewapiApi.importFromAdmin.mockResolvedValue({
      instanceId: "instance-1",
      importedCount: 4,
      sitesCreated: 2,
      sitesMerged: 1,
      detectedCount: 0,
      syncTokenSaved: true,
    } satisfies ImportFromAdminResult);

    seedOpenStep({
      stepIndex: 0,
      baseUrl: "https://newapi.example",
      accessToken: "token-1",
      saveToken: true,
    });
    const { tree } = await renderWizard();
    await findControl(collectElements(tree), "执行").onClick?.();

    await vi.waitFor(() => {
      expect(localNewapiApi.importFromAdmin).toHaveBeenCalledWith({
        baseUrl: "https://newapi.example",
        accessToken: "token-1",
        saveAccessToken: true,
      });
      expect(String(stateSlots[3])).toContain("已导入 4 个渠道");
      expect(String(stateSlots[3])).toContain("新建站点 2 个");
      expect(String(stateSlots[3])).toContain("合并站点 1 个");
      expect(stateSlots[4]).toBe("");
    });
  });

  it("渠道步骤调用 channelsApi.syncModels(limit=10)，并使用真实 schema 字段", async () => {
    channelsApi.syncModels.mockResolvedValue({
      generatedAt: "2026-07-18T03:00:00Z",
      syncedChannels: 3,
      channelCount: 8,
      modelCount: 20,
      liveKeyCount: 2,
      rawOnlyCount: 0,
      failedCount: 1,
      uncheckedCount: 4,
      items: [],
      models: [],
    } satisfies ChannelModelOverview);

    seedOpenStep({ stepIndex: 1 });
    const { tree } = await renderWizard();
    await findControl(collectElements(tree), "执行").onClick?.();

    await vi.waitFor(() => {
      expect(channelsApi.syncModels).toHaveBeenCalledWith({ limit: 10 });
      expect(String(stateSlots[3])).toContain("共 8 个");
      expect(String(stateSlots[3])).toContain("成功 3 个");
      expect(String(stateSlots[3])).toContain("失败 1 个");
      expect(String(stateSlots[3])).not.toContain("共 undefined");
    });
  });

  it("第 4 步只导航 checkinPreview，绝不启动任务", async () => {
    seedOpenStep({ stepIndex: 3 });
    const onNavigate = vi.fn();
    const { tree } = await renderWizard(onNavigate);
    await findControl(collectElements(tree), "前往安全预览").onClick?.();

    await vi.waitFor(() => {
      expect(onNavigate).toHaveBeenCalledWith("checkins", { checkinPreview: "open" });
      expect(localNewapiApi.importFromAdmin).not.toHaveBeenCalled();
      expect(channelsApi.syncModels).not.toHaveBeenCalled();
      // open 关闭并标记完成
      expect(stateSlots[0]).toBe(false);
    });
  });

  it("API 失败以 role=alert 语义写入 error，不调用任务 start", async () => {
    localNewapiApi.importFromAdmin.mockRejectedValue(new Error("NewAPI 认证失败"));
    seedOpenStep({
      stepIndex: 0,
      baseUrl: "https://newapi.example",
      accessToken: "bad-token",
    });
    const { tree } = await renderWizard();
    await findControl(collectElements(tree), "执行").onClick?.();

    await vi.waitFor(() => {
      expect(String(stateSlots[4])).toContain("NewAPI 认证失败");
      expect(stateSlots[3]).toBe("");
    });
  });
});
