import { api } from "@/api/client";

/** 全部标记已读；无 body。 */
function markAllRead(): Promise<unknown> {
  return api("/api/notifications/mark-all-read", { method: "POST" });
}

/** 清除已读通知；无 body。 */
function clearRead(): Promise<unknown> {
  return api("/api/notifications/clear-read", { method: "POST" });
}

/**
 * 收纳并修剪通知。
 * 默认 keep=10；query 集中在 adapter，组件不得手拼 trim URL。
 */
function trim(keep = 10): Promise<unknown> {
  const safeKeep = Number.isFinite(keep) && keep > 0 ? Math.floor(keep) : 10;
  return api(`/api/notifications/trim?keep=${safeKeep}`, { method: "POST" });
}

export const notificationsApi = {
  markAllRead,
  clearRead,
  trim,
} as const;
