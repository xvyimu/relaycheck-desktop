import { afterEach, expect, it, vi } from "vitest";

import type { Account } from "@/types";

type TestElement = {
  props?: {
    children?: unknown;
    onClick?: () => void;
  };
};

function findElementByText(node: unknown, text: string): TestElement | null {
  if (!node || typeof node !== "object") return null;
  const element = node as TestElement;
  const children = element.props?.children;
  if (children === text) return element;
  const items = Array.isArray(children) ? children : [children];
  for (const child of items) {
    const found = findElementByText(child, text);
    if (found) return found;
  }
  return null;
}

afterEach(() => {
  vi.resetModules();
  vi.doUnmock("react");
  vi.doUnmock("@/api/client");
});

it("renders relogin guidance and account diagnostics without requiring a browser", async () => {
  vi.doMock("react", () => ({
    useState: (initial: unknown) => [initial, vi.fn()],
  }));
  vi.doMock("@/api/client", () => ({ api: vi.fn() }));

  const { AccountDetailContent } = await import("../AccountDetailContent");
  const element = AccountDetailContent({
    account: {
      id: "account-1",
      displayName: "测试账号",
      upstreamSiteName: "测试站点",
      upstreamSiteBaseUrl: "https://relay.example",
      upstreamSiteLoginUrl: "https://relay.example/login",
      email: "user@example.test",
      authType: "cookie",
      loginStatus: "two_factor_required",
      lastCheckinStatus: "failed",
      balance: 12.5,
      balanceUnit: "USD",
      lastCheckinAt: "2026-07-17T00:00:00Z",
      lastValidatedAt: "2026-07-17T00:00:00Z",
      apiKeyFingerprint: "abcd1234",
      apiKeyStatus: "invalid",
      apiKeyTestModel: "gpt-4o-mini",
      apiKeyLatencyMs: 42,
      apiKeySampleModels: ["gpt-4o-mini", "qwen-turbo"],
      apiKeyTestMessage: "上游拒绝请求",
    } as Account,
    onClose: vi.fn(),
  });

  expect(element).toBeTruthy();
});

it("renders the healthy account path with no missing-data guidance", async () => {
  vi.doMock("react", () => ({
    useState: (initial: unknown) => [initial, vi.fn()],
  }));
  vi.doMock("@/api/client", () => ({ api: vi.fn() }));

  const { AccountDetailContent } = await import("../AccountDetailContent");
  const element = AccountDetailContent({
    account: {
      id: "account-2",
      displayName: "健康账号",
      authType: "api_key",
      loginStatus: "valid",
      lastCheckinStatus: "success",
      balance: 0,
      balanceUnit: "USD",
      apiKeyStatus: "valid",
    } as Account,
    onClose: vi.fn(),
  });

  expect(element).toBeTruthy();
});

it("posts login-status checks to the backend account test-login action", async () => {
  vi.doMock("react", () => ({
    useState: (initial: unknown) => [initial, vi.fn()],
  }));
  const api = vi.fn().mockResolvedValue({ status: "valid", httpStatus: 200 });
  vi.doMock("@/api/client", () => ({ api }));

  const { AccountDetailContent } = await import("../AccountDetailContent");
  const element = AccountDetailContent({
    account: {
      id: "account-route-contract",
      displayName: "路由测试账号",
      authType: "cookie",
      loginStatus: "expired",
      lastCheckinStatus: "failed",
    } as Account,
    onClose: vi.fn(),
  });
  const button = findElementByText(element, "测试登录态");
  expect(button?.props?.onClick).toBeTypeOf("function");

  button?.props?.onClick?.();

  await vi.waitFor(() =>
    expect(api).toHaveBeenCalledWith("/api/accounts/account-route-contract/test-login", {
      method: "POST",
      body: JSON.stringify({}),
    }),
  );
});
