import { api } from "@/api/client";
import type { SiteDetail, UpstreamSite } from "@/types";

/** 渠道/站点 ID 编码，避免路径注入与空格问题。 */
function sitePath(id: string, suffix = ""): string {
  const base = `/api/upstream-sites/${encodeURIComponent(id)}`;
  return suffix ? `${base}/${suffix}` : base;
}

/** 列出上游站点；只读 GET。 */
function list(): Promise<UpstreamSite[]> {
  return api<UpstreamSite[]>("/api/upstream-sites");
}

/** 读取单站详情（站点 + 探测 + 关联账号摘要）。 */
function get(id: string): Promise<SiteDetail> {
  return api<SiteDetail>(sitePath(id));
}

/** 对单站执行探测/识别；POST 无 body。 */
function detect(id: string): Promise<unknown> {
  return api(sitePath(id, "detect"), { method: "POST" });
}

/**
 * 批量探测入口（若 UI 直接调用）。
 * 当前主路径多用 task runner；保留 typed 契约防止裸 URL 回流。
 */
function bulkDetect(body: Record<string, unknown> = {}): Promise<unknown> {
  return api("/api/upstream-sites/bulk-detect", {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export const sitesApi = {
  list,
  get,
  detect,
  bulkDetect,
} as const;
