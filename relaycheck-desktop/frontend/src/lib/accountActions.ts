import type { BrowserLoginOpenResponse, BrowserLoginSaveResponse, LoginStatusTestResponse } from "@/types";

function httpStatusText(httpStatus?: number) {
  return httpStatus ? `（HTTP ${httpStatus}）` : "";
}

export function accountActionButtonLabel(label: string, busy: string, runningLabel?: string): string {
  if (busy !== label) return label;
  return runningLabel || `${label}中…`;
}

export function formatBrowserLoginOpenMessage(result: BrowserLoginOpenResponse): string {
  const prefix = result.status === "already_open" ? "网页登录窗口已在运行" : "网页登录窗口已打开";
  const target = result.url ? `：${result.url}` : "";
  return `${prefix}${target}。完成登录后点击“保存授权”。`;
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
