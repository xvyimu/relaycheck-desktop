import { isValidElement, type ReactElement, type ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { UnsupportedCheckinCleanupPreview } from "@/api/account-cleanup";
import {
  UnsupportedCheckinCleanupPanel,
  type UnsupportedCheckinCleanupPanelProps,
} from "../UnsupportedCheckinCleanupPanel";

type TestChangeEvent = {
  currentTarget: {
    checked: boolean;
  };
};

type TestElementProps = {
  children?: ReactNode;
  checked?: boolean;
  disabled?: boolean;
  onChange?: (event: TestChangeEvent) => void;
  onClick?: () => void | Promise<void>;
  type?: string;
};

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

/** 递归收集 JSX 元素，便于在纯 Node 测试环境触发真实组件回调。 */
function collectElements(node: ReactNode): ReactElement[] {
  if (Array.isArray(node)) return node.flatMap(collectElements);
  if (!isValidElement(node)) return [];
  const props = node.props as TestElementProps;
  return [node, ...collectElements(props.children)];
}

/** 提取 JSX 子树中的可见文本，用稳定按钮文案定位交互入口。 */
function textContent(node: ReactNode): string {
  if (typeof node === "string" || typeof node === "number") return String(node);
  if (Array.isArray(node)) return node.map(textContent).join("");
  if (!isValidElement(node)) return "";
  return textContent((node.props as TestElementProps).children);
}

/** 按完整文案查找控件，避免测试依赖组件内部 DOM 层级。 */
function findControl(elements: ReactElement[], label: string): TestElementProps {
  const element = elements.find((item) => textContent((item.props as TestElementProps).children) === label);
  expect(element, `missing control ${label}`).toBeTruthy();
  return element!.props as TestElementProps;
}

/** 创建带一次性 previewId 的候选集合，供确认与失效交互复用。 */
function makeCleanupPreview(): UnsupportedCheckinCleanupPreview {
  return {
    previewId: "cleanup-preview-1",
    expiresAt: "2026-07-18T02:05:00Z",
    matched: 1,
    deleted: 0,
    limit: 10,
    hasMore: false,
    includeLastUnsupported: true,
    items: [
      {
        accountId: "account-1",
        accountName: "待清理账号",
        upstreamSiteId: "site-1",
        upstreamSiteName: "Relay One",
        upstreamSiteKind: "oneapi",
        lastCheckinStatus: "unsupported",
        reason: "last_checkin_unsupported",
      },
    ],
  };
}

/** 构造受控面板 props，让每个测试只覆盖一个所有权边界。 */
function panelProps(overrides: Partial<UnsupportedCheckinCleanupPanelProps> = {}): UnsupportedCheckinCleanupPanelProps {
  return {
    preview: null,
    busy: false,
    includeLastUnsupported: true,
    onPreview: vi.fn(),
    onConfirm: vi.fn(),
    onIncludeLastUnsupportedChange: vi.fn(),
    onClearPreview: vi.fn(),
    ...overrides,
  };
}

describe("UnsupportedCheckinCleanupPanel", () => {
  it("预览前禁用删除，但保留预览入口", async () => {
    const props = panelProps();
    const elements = collectElements(UnsupportedCheckinCleanupPanel(props));
    const preview = findControl(elements, "预览清理");
    const remove = findControl(elements, "删除本批");

    expect(remove.disabled).toBe(true);
    expect(preview.disabled).toBe(false);
    await preview.onClick?.();
    expect(props.onPreview).toHaveBeenCalledOnce();
  });

  it("用户取消二次确认时不提交确认请求", async () => {
    const confirm = vi.fn().mockReturnValue(false);
    vi.stubGlobal("window", { confirm });
    const props = panelProps({ preview: makeCleanupPreview() });
    const remove = findControl(collectElements(UnsupportedCheckinCleanupPanel(props)), "删除本批");

    await remove.onClick?.();

    expect(confirm).toHaveBeenCalledOnce();
    expect(props.onConfirm).not.toHaveBeenCalled();
  });

  it("用户确认后只传递当前预览的同一个 previewId", async () => {
    vi.stubGlobal("window", { confirm: vi.fn().mockReturnValue(true) });
    const props = panelProps({ preview: makeCleanupPreview() });
    const remove = findControl(collectElements(UnsupportedCheckinCleanupPanel(props)), "删除本批");

    await remove.onClick?.();

    expect(props.onConfirm).toHaveBeenCalledOnce();
    expect(props.onConfirm).toHaveBeenCalledWith("cleanup-preview-1");
  });

  it("切换 include 范围时通知 owner 并清空旧预览", () => {
    const props = panelProps({ preview: makeCleanupPreview() });
    const elements = collectElements(UnsupportedCheckinCleanupPanel(props));
    const checkbox = elements.find(
      (element) => element.type === "input" && (element.props as TestElementProps).type === "checkbox",
    );
    expect(checkbox).toBeTruthy();

    (checkbox!.props as TestElementProps).onChange?.({ currentTarget: { checked: false } });

    expect(props.onIncludeLastUnsupportedChange).toHaveBeenCalledWith(false);
    expect(props.onClearPreview).toHaveBeenCalledOnce();
  });

  it("零候选结果保持删除禁用且不要求 previewId", () => {
    const props = panelProps({
      preview: {
        matched: 0,
        deleted: 0,
        limit: 10,
        hasMore: false,
        includeLastUnsupported: true,
        items: [],
      },
    });
    const elements = collectElements(UnsupportedCheckinCleanupPanel(props));

    expect(findControl(elements, "删除本批").disabled).toBe(true);
    expect(textContent(UnsupportedCheckinCleanupPanel(props))).toContain("当前没有匹配的不支持签到账号");
  });
});
