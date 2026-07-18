import { expect, it, vi } from "vitest";

import { TwoFactorGuide } from "../TwoFactorGuide";

it("renders inline two-factor instructions with an inferred login URL", () => {
  const element = TwoFactorGuide({
    siteName: "测试站点",
    baseUrl: "https://relay.example/",
    onOpenBrowserLogin: vi.fn(),
    onClose: vi.fn(),
    footer: "请完成验证后返回。",
  });

  expect(element).toBeTruthy();
});

it("renders the modal guide with a custom login URL", () => {
  const element = TwoFactorGuide({
    siteName: "测试站点",
    loginUrl: "https://relay.example/login",
    variant: "dialog",
    defaultExpanded: false,
    onClose: vi.fn(),
  });

  expect(element).toBeTruthy();
});
