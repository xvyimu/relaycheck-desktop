import { isValidElement, type ReactElement, type ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { AutoDetectResponse } from "@/api/local-newapi";

type TestElementProps = {
  children?: ReactNode;
  disabled?: boolean;
  onClick?: () => void | Promise<void>;
  type?: string;
  "aria-label"?: string;
};

const stateSlots: unknown[] = [];
let stateIndex = 0;

const autoDetectImport = vi.fn();

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
    memo: (component: unknown) => component,
  };
});

vi.mock("@/api/local-newapi", () => ({
  localNewapiApi: {
    autoDetectImport: (...args: unknown[]) => autoDetectImport(...args),
  },
}));

vi.mock("@/components/scan/LocalNewAPISyncPanel", () => ({
  LocalNewAPISyncPanel: () => null,
}));
vi.mock("@/components/ui/button", () => ({
  Button: (props: TestElementProps) => props as unknown as ReactElement,
}));
vi.mock("@/components/ui/badge", () => ({
  Badge: (props: TestElementProps) => props as unknown as ReactElement,
}));
vi.mock("@/components/ui/card", () => ({
  Card: (props: TestElementProps) => props as unknown as ReactElement,
  CardContent: (props: TestElementProps) => props as unknown as ReactElement,
  CardHeader: (props: TestElementProps) => props as unknown as ReactElement,
  CardTitle: (props: TestElementProps) => props as unknown as ReactElement,
}));
vi.mock("@/components/ui/line-icon", () => ({
  LineIcon: () => null,
}));

afterEach(() => {
  stateSlots.length = 0;
  stateIndex = 0;
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

function findByAriaLabel(elements: ReactElement[], label: string): TestElementProps {
  const element = elements.find((item) => (item.props as TestElementProps)["aria-label"] === label);
  expect(element, `missing aria-label ${label}`).toBeTruthy();
  return element!.props as TestElementProps;
}

async function renderScan(onRefresh = vi.fn(), onNavigate = vi.fn()) {
  stateIndex = 0;
  const mod = await import("../ScanPanel");
  const Component = mod.ScanPanel as unknown as (p: {
    onRefresh: () => Promise<void>;
    onNavigate: (tab: string, intent?: unknown) => void;
  }) => ReactElement;
  const tree = Component({ onRefresh, onNavigate });
  return { tree, onRefresh, onNavigate };
}

describe("ScanPanel behavior", () => {
  it("扫描成功 found=true 时调用 onRefresh，并展示下一步导航", async () => {
    const payload: AutoDetectResponse = {
      found: true,
      message: "导入完成",
      results: [{ dbPath: "one.db", baseUrl: "http://127.0.0.1", importedCount: 2, sitesCreated: 1, sitesMerged: 0 }],
    };
    autoDetectImport.mockResolvedValue(payload);
    const first = await renderScan();
    await findByAriaLabel(collectElements(first.tree), "检测并导入本机 NewAPI 数据库").onClick?.();

    await vi.waitFor(() => {
      expect(autoDetectImport).toHaveBeenCalledOnce();
      expect(first.onRefresh).toHaveBeenCalledOnce();
    });

    // 重新渲染以读取更新后的 result 状态
    const second = await renderScan(first.onRefresh, first.onNavigate);
    const text = textContent(second.tree);
    expect(text).toContain("查看渠道");
    expect(text).toContain("前往站点与账号");
  });

  it("扫描失败写入稳定文案且不抛未处理拒绝", async () => {
    autoDetectImport.mockRejectedValue(new Error("network"));
    const first = await renderScan();
    await findByAriaLabel(collectElements(first.tree), "检测并导入本机 NewAPI 数据库").onClick?.();

    await vi.waitFor(() => expect(autoDetectImport).toHaveBeenCalled());
    const second = await renderScan();
    expect(textContent(second.tree)).toContain("扫描请求失败，请检查服务状态。");
    expect(textContent(second.tree)).not.toContain("查看渠道");
  });
});
