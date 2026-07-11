/** Helpers for NewAPI channel-sync (#8.3) feedback — no secrets in copy. */

export type ImportCounters = {
  fetchedCount?: number;
  importedCount?: number;
  skippedExcluded?: number;
  skippedNoBaseURL?: number;
  sitesCreated?: number;
  sitesMerged?: number;
  detectedCount?: number;
  syncTokenSaved?: boolean;
  lastSyncAt?: string;
  lastSyncSummary?: string;
  skippedExcludedSamples?: Array<{
    sourceChannelId?: string;
    name?: string;
    matchedToken?: string;
  }>;
  skippedExcludedTruncated?: boolean;
  message?: string;
};

export type SyncCapabilityKind = "sqlite" | "admin_api" | "none" | "unknown";

export function normalizeSyncCapability(value?: string): SyncCapabilityKind {
  const raw = (value || "").trim().toLowerCase();
  if (!raw) return "unknown";
  if (raw.includes("sqlite") || raw.includes("database") || raw.includes("db")) return "sqlite";
  if (raw.includes("admin") || raw.includes("api") || raw.includes("token") || raw.includes("http")) {
    return "admin_api";
  }
  if (raw.includes("none") || raw.includes("missing") || raw.includes("unavailable")) return "none";
  return "unknown";
}

export function syncCapabilityLabel(value?: string): string {
  switch (normalizeSyncCapability(value)) {
    case "sqlite":
      return "本机数据库";
    case "admin_api":
      return "后台 Admin API";
    case "none":
      return "不可同步";
    default:
      return value?.trim() || "未知";
  }
}

export function syncTokenStatusLabel(hasSyncToken: boolean, capability?: string): string {
  if (hasSyncToken) return "已保存访问令牌";
  const kind = normalizeSyncCapability(capability);
  if (kind === "sqlite") return "使用本机数据库路径";
  if (kind === "admin_api" || kind === "none" || kind === "unknown") {
    return "需要 NewAPI 系统访问令牌或本机数据库路径";
  }
  return "需要 NewAPI 系统访问令牌或本机数据库路径";
}

/**
 * Human-readable import outcome. Distinguishes empty source, full exclude, and needs token.
 * Never mentions "关闭 2FA".
 */
export function formatImportCountersMessage(counters: ImportCounters, opts?: { error?: string }): string {
  if (opts?.error) {
    const err = opts.error.trim();
    if (/令牌|token|授权|access/i.test(err) && !/2fa|二步|两步/i.test(err)) {
      return err.includes("系统访问令牌") || err.includes("数据库")
        ? err
        : `${err.replace(/。?$/, "")}。需要 NewAPI 系统访问令牌或本机数据库路径。`;
    }
    return err;
  }

  const fetched = counters.fetchedCount ?? 0;
  const imported = counters.importedCount ?? 0;
  const skippedExcluded = counters.skippedExcluded ?? 0;
  const skippedNoBase = counters.skippedNoBaseURL ?? 0;
  const sitesCreated = counters.sitesCreated ?? 0;
  const sitesMerged = counters.sitesMerged ?? 0;
  const detected = counters.detectedCount ?? 0;

  if (fetched === 0 && imported === 0 && skippedExcluded === 0) {
    return "源端无渠道（fetched=0）。可在 NewAPI 后台确认渠道列表，或检查访问令牌权限。";
  }

  if (fetched > 0 && imported === 0 && skippedExcluded >= fetched) {
    return `源端返回 ${fetched} 条，全部因排除规则跳过（非失败）。可在诊断中核对排除关键字。`;
  }

  if (fetched > 0 && imported === 0 && skippedExcluded > 0) {
    return `拉取 ${fetched} 条，导入 0；排除 ${skippedExcluded}，无 BaseURL ${skippedNoBase}。`;
  }

  const parts = [
    `拉取 ${fetched}`,
    `导入 ${imported}`,
    skippedExcluded ? `排除 ${skippedExcluded}` : "",
    skippedNoBase ? `无 BaseURL ${skippedNoBase}` : "",
    sitesCreated ? `新建站点 ${sitesCreated}` : "",
    sitesMerged ? `合并站点 ${sitesMerged}` : "",
    detected ? `探测 ${detected}` : "",
    counters.syncTokenSaved ? "已保存同步令牌" : "",
  ].filter(Boolean);

  return `${parts.join(" · ")}。`;
}

export function instanceNeedsCredential(instance: {
  hasSyncToken: boolean;
  syncCapability?: string;
  databasePath?: string;
}): boolean {
  if (instance.databasePath?.trim()) return false;
  if (instance.hasSyncToken) return false;
  const kind = normalizeSyncCapability(instance.syncCapability);
  return kind !== "sqlite";
}

/** Short audit line for excluded samples (no secrets). */
export function formatExcludedSamplesHint(
  samples?: ImportCounters["skippedExcludedSamples"],
  truncated?: boolean,
): string {
  if (!samples?.length) return "";
  const preview = samples
    .slice(0, 5)
    .map((item) => {
      const name = (item.name || item.sourceChannelId || "?").trim();
      const token = (item.matchedToken || "").trim();
      return token ? `${name}→${token}` : name;
    })
    .join("；");
  const more = truncated || samples.length > 5 ? "（列表已截断）" : "";
  return `排除样例：${preview}${more}`;
}
