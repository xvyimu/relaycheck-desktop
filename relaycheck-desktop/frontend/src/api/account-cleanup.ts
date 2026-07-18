import { accountApi } from "@/api/accounts";
import { api } from "@/api/client";
import type {
  UnsupportedCheckinAccountItem,
  UnsupportedCheckinCleanupResult as LegacyUnsupportedCheckinCleanupResult,
} from "@/types";

export type UnsupportedCheckinCleanupOptions = {
  limit: number;
  includeLastUnsupported: boolean;
};

export type UnsupportedCheckinCleanupResult = Omit<LegacyUnsupportedCheckinCleanupResult, "dryRun" | "items"> & {
  previewId?: string;
  expiresAt?: string;
  items: UnsupportedCheckinAccountItem[];
};

export type UnsupportedCheckinCleanupPreview = UnsupportedCheckinCleanupResult & {
  previewId: string;
  expiresAt: string;
};

/** 请求只读候选预览，并由后端签发短期、一次性的 previewId。 */
async function previewUnsupportedCheckinAccounts(
  options: UnsupportedCheckinCleanupOptions,
): Promise<UnsupportedCheckinCleanupResult> {
  // 明确发送 dryRun=true，避免调用方通过默认布尔值进入破坏性路径。
  return api<UnsupportedCheckinCleanupResult>(accountApi.command("delete-unsupported-checkins"), {
    method: "POST",
    body: JSON.stringify({
      limit: options.limit,
      dryRun: true,
      includeLastUnsupported: options.includeLastUnsupported,
    }),
  });
}

/** 消费原预览 ID；确认请求不得重新提交筛选条件或账号集合。 */
async function confirmUnsupportedCheckinAccounts(previewId: string): Promise<UnsupportedCheckinCleanupResult> {
  return api<UnsupportedCheckinCleanupResult>(accountApi.command("delete-unsupported-checkins"), {
    method: "POST",
    body: JSON.stringify({ previewId }),
  });
}

export const accountCleanupApi = {
  preview: previewUnsupportedCheckinAccounts,
  confirm: confirmUnsupportedCheckinAccounts,
} as const;
