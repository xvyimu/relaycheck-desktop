import { describe, expect, it } from "vitest";

import {
  formatBrowserLoginOpenMessage,
  formatBrowserLoginSaveMessage,
  formatLoginStatusTestMessage,
} from "../accountActions";

describe("formatBrowserLoginOpenMessage", () => {
  it("includes the opened URL and next save step", () => {
    expect(
      formatBrowserLoginOpenMessage({
        accountId: "acc-1",
        status: "opened",
        url: "https://relay.example/login",
      }),
    ).toBe("网页登录窗口已打开：https://relay.example/login。完成登录后点击“保存授权”。");
  });

  it("handles already-open browser sessions", () => {
    expect(formatBrowserLoginOpenMessage({ accountId: "acc-1", status: "already_open" })).toBe(
      "网页登录窗口已在运行。完成登录后点击“保存授权”。",
    );
  });
});

describe("formatBrowserLoginSaveMessage", () => {
  it("shows cookie count and next validation step", () => {
    expect(formatBrowserLoginSaveMessage({ accountId: "acc-1", status: "saved", cookieCount: 3 })).toBe(
      "授权已保存（3 个 Cookie）。下一步可测试登录态或直接签到。",
    );
  });

  it("falls back when cookie count is missing", () => {
    expect(formatBrowserLoginSaveMessage({ accountId: "acc-1", status: "saved" })).toBe(
      "授权已保存。下一步可测试登录态或直接签到。",
    );
  });
});

describe("formatLoginStatusTestMessage", () => {
  it("guides the user when the login status is valid", () => {
    expect(formatLoginStatusTestMessage({ status: "valid", httpStatus: 200 })).toBe(
      "登录态有效（HTTP 200）。可以执行签到或刷新余额。",
    );
  });

  it("guides the user when the login status is expired", () => {
    expect(formatLoginStatusTestMessage({ status: "expired", httpStatus: 401 })).toBe(
      "登录态已失效（HTTP 401）。请重新网页登录并保存授权。",
    );
  });
});
