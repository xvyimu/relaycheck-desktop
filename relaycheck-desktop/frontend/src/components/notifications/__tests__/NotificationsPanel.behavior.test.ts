import { isValidElement, type ReactElement, type ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { NotificationItem } from "@/types";

type TestElementProps = {
  children?: ReactNode;
  disabled?: boolean;
  onClick?: () => void | Promise<void>;
  type?: string;
};

const stateSlots: unknown[] = [];
let stateIndex = 0;
const effectFns: Array<() => void | (() => void)> = [];

const markAllRead = vi.fn().mockResolvedValue({});
const clearRead = vi.fn().mockResolvedValue({});
const trim = vi.fn().mockResolvedValue({});

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
    memo: (component: unknown) => component,
  };
});

vi.mock("@/api/notifications", () => ({
  notificationsApi: {
    markAllRead: (...args: unknown[]) => markAllRead(...args),
    clearRead: (...args: unknown[]) => clearRead(...args),
    trim: (...args: unknown[]) => trim(...args),
  },
}));

vi.mock("@/components/ui/button", () => ({
  Button: (props: TestElementProps) => props as unknown as ReactElement,
}));
vi.mock("@/lib/format", () => ({
  formatTime: (value: string) => value,
}));
vi.mock("@/lib/tone", () => ({
  statusTone: () => "info",
}));

afterEach(() => {
  stateSlots.length = 0;
  stateIndex = 0;
  effectFns.length = 0;
  vi.clearAllMocks();
  vi.unstubAllGlobals();
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

function item(overrides: Partial<NotificationItem> = {}): NotificationItem {
  return {
    id: "n1",
    type: "info",
    level: "info",
    title: "通知",
    content: "内容",
    read: false,
    createdAt: "2026-07-18T00:00:00Z",
    ...overrides,
  };
}

async function renderPanel(
  options: {
    unreadTotal?: number;
    total?: number;
    items?: NotificationItem[];
    onRefresh?: ReturnType<typeof vi.fn>;
  } = {},
) {
  stateIndex = 0;
  effectFns.length = 0;
  const mod = await import("../NotificationsPanel");
  const Component = mod.NotificationsPanel as unknown as (p: {
    items: NotificationItem[];
    total: number;
    unreadTotal: number;
    importantTotal: number;
    onRefresh: () => Promise<void>;
  }) => ReactElement;
  const onRefresh = options.onRefresh || vi.fn().mockResolvedValue(undefined);
  const tree = Component({
    items: options.items || [item()],
    total: options.total ?? 2,
    unreadTotal: options.unreadTotal ?? 1,
    importantTotal: 0,
    onRefresh: onRefresh as () => Promise<void>,
  });
  for (const effect of effectFns.splice(0)) effect();
  return { tree, onRefresh };
}

describe("NotificationsPanel behavior", () => {
  it("未读为 0 时禁用全部标记已读", async () => {
    const { tree } = await renderPanel({ unreadTotal: 0, total: 1, items: [item({ read: true })] });
    const btn = findControl(collectElements(tree), "全部标记已读");
    expect(btn.disabled).toBe(true);
  });

  it("清除已读取消 confirm 时零请求", async () => {
    vi.stubGlobal("window", { confirm: vi.fn().mockReturnValue(false) });
    const { tree, onRefresh } = await renderPanel({ unreadTotal: 0, total: 2, items: [item({ read: true })] });
    await findControl(collectElements(tree), "清除已读").onClick?.();
    expect(clearRead).not.toHaveBeenCalled();
    expect(onRefresh).not.toHaveBeenCalled();
  });

  it("清除已读确认后只调 notificationsApi.clearRead", async () => {
    vi.stubGlobal("window", { confirm: vi.fn().mockReturnValue(true) });
    const { tree, onRefresh } = await renderPanel({
      unreadTotal: 0,
      total: 2,
      items: [item({ read: true }), item({ id: "n2", read: true })],
    });
    await findControl(collectElements(tree), "清除已读").onClick?.();
    await vi.waitFor(() => {
      expect(clearRead).toHaveBeenCalledOnce();
      expect(onRefresh).toHaveBeenCalledOnce();
    });
  });

  it("收纳已读调用 trim(10) 并隐藏已读", async () => {
    const { tree, onRefresh } = await renderPanel({
      unreadTotal: 0,
      total: 1,
      items: [item({ read: true })],
    });
    await findControl(collectElements(tree), "收纳已读").onClick?.();
    await vi.waitFor(() => {
      expect(trim).toHaveBeenCalledWith(10);
      expect(onRefresh).toHaveBeenCalledOnce();
      // showRead 状态槽（busy, message, showRead）
      expect(stateSlots[2]).toBe(false);
    });
  });

  it("全部标记已读走 notificationsApi", async () => {
    const { tree, onRefresh } = await renderPanel({ unreadTotal: 2 });
    await findControl(collectElements(tree), "全部标记已读").onClick?.();
    await vi.waitFor(() => {
      expect(markAllRead).toHaveBeenCalledOnce();
      expect(onRefresh).toHaveBeenCalledOnce();
    });
  });
});
