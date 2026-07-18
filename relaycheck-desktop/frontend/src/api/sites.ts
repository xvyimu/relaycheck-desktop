import { api } from "@/api/client";
import type { UpstreamSite } from "@/types";

/** 列出上游站点；只读 GET，供排程面板初始化表单。 */
function list(): Promise<UpstreamSite[]> {
  return api<UpstreamSite[]>("/api/upstream-sites");
}

export const sitesApi = {
  list,
} as const;
