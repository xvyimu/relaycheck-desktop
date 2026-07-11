import { describe, expect, it } from "vitest";

import {
  accountActionButtonLabel,
  appendReloginHint,
  formatBrowserLoginOpenMessage,
  formatBrowserLoginSaveMessage,
  formatLoginStatusTestMessage,
  isBrowserLoginOpenSuccess,
  isBrowserLoginSaveSuccess,
  isLikelyAuthFailureMessage,
  isLoginStatusValid,
  primaryActionsForRelogin,
  reloginStepIndex,
  shouldShowReloginSteps,
} from "../accountActions";

describe("accountActionButtonLabel", () => {
  it("uses the idle label when another action is running", () => {
    expect(accountActionButtonLabel("网页登录", "刷新余额")).toBe("网页登录");
  });

  it("uses a running label for the active action", () => {
    expect(accountActionButtonLabel("网页登录", "网页登录")).toBe("网页登录中…");
  });

  it("allows action-specific running copy", () => {
    expect(accountActionButtonLabel("测试登录态", "测试登录态", "检测中…")).toBe("检测中…");
  });
});

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
  it("includes resolver source, confidence, and reason when present", () => {
    const message = formatBrowserLoginOpenMessage({
      accountId: "acc-1",
      status: "opened",
      url: "https://relay.example/panel/login",
      loginUrlSource: "path_probe",
      loginUrlConfidence: 0.45,
      loginUrlReason: "Low confidence login candidate; verify manually",
    });

    expect(message).toContain("path_probe");
    expect(message).toContain("45%");
    expect(message).toContain("Low confidence login candidate");
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

describe("relogin state machine helpers", () => {
  it("elevates save while browser is open", () => {
    expect(primaryActionsForRelogin("browser_open")).toEqual(["open", "save", "detail"]);
  });

  it("elevates test after auth is saved", () => {
    expect(primaryActionsForRelogin("auth_saved")).toEqual(["test", "checkin", "detail"]);
  });

  it("keeps open + checkin as idle primary set", () => {
    expect(primaryActionsForRelogin("idle")).toEqual(["open", "checkin", "detail"]);
  });

  it("shows steps for problem login or active phase", () => {
    expect(shouldShowReloginSteps("manual_required", undefined, "idle")).toBe(true);
    expect(shouldShowReloginSteps("valid", "auth_expired", "idle")).toBe(true);
    expect(shouldShowReloginSteps("valid", "success", "browser_open")).toBe(true);
    expect(shouldShowReloginSteps("valid", "success", "idle")).toBe(false);
  });

  it("maps phase to step index", () => {
    expect(reloginStepIndex("idle", "expired")).toBe(0);
    expect(reloginStepIndex("browser_open", "manual_required")).toBe(1);
    expect(reloginStepIndex("auth_saved", "valid")).toBe(2);
    expect(reloginStepIndex("idle", "valid")).toBe(3);
  });

  it("detects open/save/valid statuses", () => {
    expect(isBrowserLoginOpenSuccess("opened")).toBe(true);
    expect(isBrowserLoginOpenSuccess("already_open")).toBe(true);
    expect(isBrowserLoginOpenSuccess("failed")).toBe(false);
    expect(isBrowserLoginSaveSuccess("saved")).toBe(true);
    expect(isLoginStatusValid("valid")).toBe(true);
  });

  it("detects auth-ish failures and appends relogin hint once", () => {
    expect(isLikelyAuthFailureMessage("登录态已失效")).toBe(true);
    expect(isLikelyAuthFailureMessage("网络超时")).toBe(false);
    expect(appendReloginHint("签到失败：401")).toContain("网页登录");
    expect(appendReloginHint("请重新网页登录")).toBe("请重新网页登录");
  });
});
