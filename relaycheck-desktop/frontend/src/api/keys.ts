import { api } from "@/api/client";
import type { KeyExportPreview } from "@/types";

/** 获取脱敏 Key 导出预览；只读 GET，响应不得包含真实密钥明文。 */
function exportPreview(): Promise<KeyExportPreview> {
  return api<KeyExportPreview>("/api/keys/export-preview");
}

export const keysApi = {
  exportPreview,
} as const;
