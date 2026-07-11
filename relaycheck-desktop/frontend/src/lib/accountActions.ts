import type { BrowserLoginOpenResponse, BrowserLoginSaveResponse, LoginStatusTestResponse } from "@/types";
import { PROBLEM_CHECKIN_STATUSES, PROBLEM_LOGIN_STATUSES } from "@/lib/constants";

/** Local UI phase for the manual re-login loop (not persisted). */
export type ReloginUiPhase = "idle" | "browser_open" | "auth_saved";

export type PrimaryActionKey = "open" | "save" | "test" | "checkin" | "detail";

export const RELOGIN_STEPS = ["打开网页登录", "保存授权", "测试登录态", "签到/余额"] as const;

function httpStatusText(httpStatus?: number) {
  return httpStatus ? `（HTTP ${httpStatus}）` : "";
}

function formatLoginEntryMeta(result: BrowserLoginOpenResponse) {
  const parts: string[] = [];
  if (result.loginUrlSource) {
    parts.push(`入口来源：${result.loginUrlSource}`);
  }
  if (typeof result.loginUrlConfidence === "number" && Number.isFinite(result.loginUrlConfidence)) {
    parts.push(`置信度：${Math.round(result.loginUrlConfidence * 100)}%`);
  }
  if (result.loginUrlReason) {
    parts.push(result.loginUrlReason);
  }
  return parts.length ? `（${parts.join("，")}）` : "";
}

export function accountActionButtonLabel(label: string, busy: string, runningLabel?: string): string {
  if (busy !== label) return label;
  return runningLabel || `${label}中…`;
}

export function formatBrowserLoginOpenMessage(result: BrowserLoginOpenResponse): string {
  const prefix = result.status === "already_open" ? "网页登录窗口已在运行" : "网页登录窗口已打开";
  const target = result.url ? `：${result.url}` : "";
  return `${prefix}${target}${formatLoginEntryMeta(result)}。完成登录后点击“保存授权”。`;
}

export function formatBrowserLoginSaveMessage(result: BrowserLoginSaveResponse): string {
  const cookieText = result.cookieCount ? `（${result.cookieCount} 个 Cookie）` : "";
  return `授权已保存${cookieText}。下一步可测试登录态或直接签到。`;
}

export function formatLoginStatusTestMessage(result: LoginStatusTestResponse): string {
  const status = (result.status || "unknown").toLowerCase();
  const suffix = httpStatusText(result.httpStatus);
  if (status === "valid") {
    return `登录态有效${suffix}。可以执行签到或刷新余额。`;
  }
  if (status === "expired") {
    return `登录态已失效${suffix}。请重新网页登录并保存授权。`;
  }
  return `登录态检测结果：${result.status || "unknown"}${suffix}。`;
}

/** Primary CTA set for the re-login state machine (detail always available). */
export function primaryActionsForRelogin(phase: ReloginUiPhase): PrimaryActionKey[] {
  if (phase === "browser_open") return ["open", "save", "detail"];
  if (phase === "auth_saved") return ["test", "checkin", "detail"];
  return ["open", "checkin", "detail"];
}

export function shouldShowReloginSteps(
  loginStatus: string,
  lastCheckinStatus: string | undefined,
  phase: ReloginUiPhase,
): boolean {
  if (phase !== "idle") return true;
  if (PROBLEM_LOGIN_STATUSES.has(loginStatus)) return true;
  if (lastCheckinStatus && PROBLEM_CHECKIN_STATUSES.has(lastCheckinStatus)) {
    return lastCheckinStatus === "auth_expired" || lastCheckinStatus === "manual_required";
  }
  return false;
}

/** 0=open … 3=ops-ready emphasis */
export function reloginStepIndex(phase: ReloginUiPhase, loginStatus: string): number {
  if (phase === "browser_open") return 1;
  if (phase === "auth_saved") return 2;
  if (loginStatus === "valid") return 3;
  return 0;
}

export function isBrowserLoginOpenSuccess(status: string | undefined): boolean {
  const value = (status || "").toLowerCase();
  return value === "opened" || value === "already_open";
}

export function isBrowserLoginSaveSuccess(status: string | undefined): boolean {
  return (status || "").toLowerCase() === "saved";
}

export function isLoginStatusValid(status: string | undefined): boolean {
  return (status || "").toLowerCase() === "valid";
}

export function isLoginStatusExpired(status: string | undefined): boolean {
  return (status || "").toLowerCase() === "expired";
}

/** Heuristic for auth-ish failures from check-in / balance API error strings. */
export function isLikelyAuthFailureMessage(message: string): boolean {
  const text = message.trim();
  if (!text) return false;
  return /登录|授权|Cookie|会话|401|403|过期|失效|未登录|auth_expired|unauthorized|manual_required|two_factor|验证码|captcha/i.test(
    text,
  );
}

export function appendReloginHint(message: string): string {
  if (/网页登录/.test(message)) return message;
  return `${message.replace(/。?$/, "")}。可点击“网页登录”重新授权，完成后再“保存授权”。`;
}
