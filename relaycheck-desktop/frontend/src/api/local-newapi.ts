import { api } from "@/api/client";
import type { ImportCounters } from "@/lib/syncFeedback";
import type { ExcludedRelaySiteRule, LocalNewAPIInstance } from "@/types";

export type ImportFromAdminOptions = {
  baseUrl: string;
  accessToken: string;
  saveAccessToken: boolean;
  importKeys?: boolean;
  skipCreateSites?: boolean;
  detectAfterImport?: boolean;
};

/** 与后端 import-from-admin-api 成功响应对齐的公共字段。 */
export type ImportFromAdminResult = {
  instanceId?: string;
  importedCount?: number;
  sitesCreated?: number;
  sitesMerged?: number;
  detectedCount?: number;
  syncTokenSaved?: boolean;
  fetchedCount?: number;
  skippedExcluded?: number;
  skippedNoBaseURL?: number;
};

export type AutoDetectResultItem = {
  dbPath: string;
  baseUrl: string;
  importedCount: number;
  sitesCreated: number;
  sitesMerged: number;
  error?: string;
};

export type AutoDetectResponse = {
  found: boolean;
  message: string;
  results: AutoDetectResultItem[];
};

export type ExcludeRulesResponse = {
  rules?: ExcludedRelaySiteRule[];
  note?: string;
};

export type LocalNewAPISyncOptions = {
  importKeys?: boolean;
  detectAfterImport?: boolean;
  pageSize?: number;
  accessToken?: string;
  saveAccessToken?: boolean;
};

/** 从 NewAPI Admin API 导入渠道结构；仅发送声明字段，并 trim 地址与令牌。 */
function importFromAdmin(options: ImportFromAdminOptions): Promise<ImportFromAdminResult> {
  return api<ImportFromAdminResult>("/api/local-newapi/import-from-admin-api", {
    method: "POST",
    body: JSON.stringify({
      baseUrl: options.baseUrl.trim(),
      accessToken: options.accessToken.trim(),
      saveAccessToken: options.saveAccessToken,
      // Onboarding 默认只导入渠道结构，不拉密钥、不立即探测。
      importKeys: options.importKeys ?? false,
      skipCreateSites: options.skipCreateSites ?? false,
      detectAfterImport: options.detectAfterImport ?? false,
    }),
  });
}

/** 列出本机已登记的 NewAPI 实例；只读 GET。 */
function listInstances(): Promise<LocalNewAPIInstance[]> {
  return api<LocalNewAPIInstance[]>("/api/local-newapi");
}

/** 读取只读排除规则与说明。 */
function excludeRules(): Promise<ExcludeRulesResponse> {
  return api<ExcludeRulesResponse>("/api/local-newapi/exclude-rules");
}

/** 自动检测本机 SQLite 并导入渠道；无 body。 */
function autoDetectImport(): Promise<AutoDetectResponse> {
  return api<AutoDetectResponse>("/api/local-newapi/auto-detect-import", { method: "POST" });
}

/**
 * 同步指定实例渠道。
 * 默认 importKeys/detectAfterImport=false、pageSize=100；
 * 仅当 accessToken 非空时才附带令牌字段，避免空串覆盖已保存令牌。
 */
function sync(
  instanceId: string,
  options: LocalNewAPISyncOptions = {},
): Promise<ImportCounters & { instanceId?: string }> {
  const body: Record<string, unknown> = {
    importKeys: options.importKeys ?? false,
    detectAfterImport: options.detectAfterImport ?? false,
    pageSize: options.pageSize ?? 100,
  };
  const token = (options.accessToken || "").trim();
  if (token) {
    body.accessToken = token;
    body.saveAccessToken = options.saveAccessToken ?? true;
  }
  return api<ImportCounters & { instanceId?: string }>(`/api/local-newapi/${encodeURIComponent(instanceId)}/sync`, {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export const localNewapiApi = {
  importFromAdmin,
  listInstances,
  excludeRules,
  autoDetectImport,
  sync,
} as const;
