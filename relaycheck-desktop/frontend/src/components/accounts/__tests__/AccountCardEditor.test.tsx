import { isValidElement, type ReactElement, type ReactNode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

import { AccountCardEditor, type AccountCardEditorProps } from "../AccountCardEditor";

type TestEvent = {
  target: {
    checked: boolean;
    value: string;
  };
};

type TestElementProps = {
  children?: ReactNode;
  onChange?: (event: TestEvent) => void;
  onClick?: () => void;
  placeholder?: string;
  type?: string;
};

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

function editorProps(overrides: Partial<AccountCardEditorProps> = {}): AccountCardEditorProps {
  return {
    siteName: "Relay Hub",
    setSiteName: vi.fn(),
    kind: "newapi",
    setKind: vi.fn(),
    baseUrl: "https://relay.example",
    setBaseUrl: vi.fn(),
    loginUrl: "https://relay.example/login",
    setLoginUrl: vi.fn(),
    siteUpdateScope: "current",
    setSiteUpdateScope: vi.fn(),
    displayName: "Primary",
    setDisplayName: vi.fn(),
    email: "user@example.com",
    setEmail: vi.fn(),
    username: "user",
    setUsername: vi.fn(),
    authType: "email_password",
    setAuthType: vi.fn(),
    password: "",
    setPassword: vi.fn(),
    apiKey: "",
    setApiKey: vi.fn(),
    hasAPIKey: true,
    clearApiKey: false,
    setClearApiKey: vi.fn(),
    busy: "",
    isBusy: false,
    onSave: vi.fn().mockResolvedValue(undefined),
    onCancel: vi.fn(),
    ...overrides,
  };
}

function propsOf(element: ReactElement): TestElementProps {
  return element.props as TestElementProps;
}

describe("AccountCardEditor", () => {
  it("renders sensitive-field and busy-state semantics", () => {
    const current = renderToStaticMarkup(<AccountCardEditor {...editorProps({ busy: "保存账号", isBusy: true })} />);
    expect(current).toContain('type="password"');
    expect(current).toContain('type="checkbox"');
    expect(current).toContain("清空当前 API Key");
    expect(current).toContain("保存中");
    expect(current).toContain("只修正这张账号卡");

    const shared = renderToStaticMarkup(
      <AccountCardEditor {...editorProps({ hasAPIKey: false, siteUpdateScope: "shared" })} />,
    );
    expect(shared).not.toContain("清空当前 API Key");
    expect(shared).toContain("影响绑定在同一站点下的账号");
  });

  it("routes every edit and command to its owner callback", () => {
    const props = editorProps();
    const elements = collectElements(AccountCardEditor(props));
    const byPlaceholder = (placeholder: string) => {
      const element = elements.find((item) => propsOf(item).placeholder === placeholder);
      expect(element, `missing input ${placeholder}`).toBeTruthy();
      return propsOf(element!);
    };
    const byText = (text: string) => {
      const element = elements.find((item) => textContent(propsOf(item).children) === text);
      expect(element, `missing control ${text}`).toBeTruthy();
      return propsOf(element!);
    };
    const changeValue = (placeholder: string, value: string) =>
      byPlaceholder(placeholder).onChange?.({ target: { checked: false, value } });

    changeValue("站点名称", "Next Hub");
    changeValue("https://example.com", "https://next.example");
    changeValue("默认使用 /login", "https://next.example/login");
    changeValue("显示名称", "Next Account");
    changeValue("邮箱账号", "next@example.com");
    changeValue("非邮箱账号", "next-user");
    changeValue("留空不覆盖旧密码", "new-password");
    changeValue("留空不覆盖旧密钥", "new-key");

    const selects = elements.filter((item) => item.type === "select").map(propsOf);
    selects[0]?.onChange?.({ target: { checked: false, value: "oneapi" } });
    selects[1]?.onChange?.({ target: { checked: false, value: "api_key" } });
    byText("只改当前账号").onClick?.();
    byText("同步同站点全部账号").onClick?.();

    const checkbox = elements.find((item) => item.type === "input" && propsOf(item).type === "checkbox");
    propsOf(checkbox!).onChange?.({ target: { checked: true, value: "" } });
    byText("保存账号").onClick?.();
    byText("取消").onClick?.();

    expect(props.setSiteName).toHaveBeenCalledWith("Next Hub");
    expect(props.setKind).toHaveBeenCalledWith("oneapi");
    expect(props.setBaseUrl).toHaveBeenCalledWith("https://next.example");
    expect(props.setLoginUrl).toHaveBeenCalledWith("https://next.example/login");
    expect(props.setSiteUpdateScope).toHaveBeenNthCalledWith(1, "current");
    expect(props.setSiteUpdateScope).toHaveBeenNthCalledWith(2, "shared");
    expect(props.setDisplayName).toHaveBeenCalledWith("Next Account");
    expect(props.setEmail).toHaveBeenCalledWith("next@example.com");
    expect(props.setUsername).toHaveBeenCalledWith("next-user");
    expect(props.setAuthType).toHaveBeenCalledWith("api_key");
    expect(props.setPassword).toHaveBeenCalledWith("new-password");
    expect(props.setApiKey).toHaveBeenCalledWith("new-key");
    expect(props.setClearApiKey).toHaveBeenCalledWith(true);
    expect(props.onSave).toHaveBeenCalledOnce();
    expect(props.onCancel).toHaveBeenCalledOnce();
  });
});
