import { api } from "@/api/client";
import type { ChannelHealthOverview, ChannelModelOverview } from "@/types";

export type ChannelModelSyncOptions = {
  limit?: number;
};

export type BulkChannelSourceStatusOptions = {
  fromStatus: "missing" | "archived";
  toStatus: "active" | "archived";
};

export type BulkChannelSourceStatusResult = {
  affected: number;
};

/** useApi 与契约测试共用的健康概览路径，避免面板手写 URL。 */
export const CHANNEL_HEALTH_OVERVIEW_PATH = "/api/channels/health/overview";

/** 渠道单项写操作的 URL 前缀；ID 始终 encode。 */
function channelItemPath(id: string, action: string): string {
  return `/api/channels/${encodeURIComponent(id)}/${action}`;
}

/** 读取渠道模型覆盖概览；只读 GET，不触发上游探测。 */
function modelsOverview(): Promise<ChannelModelOverview> {
  return api<ChannelModelOverview>("/api/channels/models/overview");
}

/**
 * 同步渠道模型并返回稳定 ChannelModelOverview。
 * 默认 limit=100 与渠道面板一致；Onboarding 可覆盖为 10。
 */
function syncModels(options: ChannelModelSyncOptions = {}): Promise<ChannelModelOverview> {
  return api<ChannelModelOverview>("/api/channels/models/sync", {
    method: "POST",
    body: JSON.stringify({ limit: options.limit ?? 100 }),
  });
}

/** 读取渠道健康概览；只读 GET，不启动探测任务。 */
function healthOverview(): Promise<ChannelHealthOverview> {
  return api<ChannelHealthOverview>(CHANNEL_HEALTH_OVERVIEW_PATH);
}

/** 识别渠道并同步到上游站点；无 body。 */
function detect(id: string): Promise<unknown> {
  return api(channelItemPath(id, "detect"), { method: "POST" });
}

/** 将缺失/归档渠道恢复为活跃；无 body。 */
function restoreSourceStatus(id: string): Promise<unknown> {
  return api(channelItemPath(id, "restore-source-status"), { method: "POST" });
}

/** 归档保留渠道（不删除账号/日志）；无 body。 */
function archiveSourceStatus(id: string): Promise<unknown> {
  return api(channelItemPath(id, "archive-source-status"), { method: "POST" });
}

/** 批量修改源端同步状态；仅发送声明字段。 */
function bulkSourceStatus(options: BulkChannelSourceStatusOptions): Promise<BulkChannelSourceStatusResult> {
  return api<BulkChannelSourceStatusResult>("/api/channels/bulk-source-status", {
    method: "POST",
    body: JSON.stringify({
      fromStatus: options.fromStatus,
      toStatus: options.toStatus,
    }),
  });
}

export const channelsApi = {
  modelsOverview,
  syncModels,
  healthOverview,
  healthOverviewPath: CHANNEL_HEALTH_OVERVIEW_PATH,
  detect,
  restoreSourceStatus,
  archiveSourceStatus,
  bulkSourceStatus,
} as const;
