import type { Account } from "@/types";
import { apiKeyStatusLabel } from "@/lib/labels";

const API_KEY_STALE_MS = 24 * 60 * 60 * 1000;

export function isStaleAPIKeyCheck(account: Account) {
  if (!account.apiKeyFingerprint) return false;
  if (!account.apiKeyLastCheckedAt) return true;
  const checkedAt = new Date(account.apiKeyLastCheckedAt).getTime();
  if (!Number.isFinite(checkedAt)) return true;
  return Date.now() - checkedAt > API_KEY_STALE_MS;
}

export function uniqueAccounts(accounts: Account[]) {
  const seen = new Set<string>();
  return accounts.filter((account) => {
    if (seen.has(account.id)) return false;
    seen.add(account.id);
    return true;
  });
}

export function buildModelCoverage(accounts: Account[]) {
  const grouped = new Map<string, { model: string; accountIds: Set<string>; siteSamples: Set<string> }>();
  for (const account of accounts) {
    const models = new Set(
      [...(account.apiKeySampleModels || []), account.apiKeyTestModel || ""]
        .map((model) => model.trim())
        .filter(Boolean),
    );
    for (const model of models) {
      const current = grouped.get(model) || { model, accountIds: new Set<string>(), siteSamples: new Set<string>() };
      current.accountIds.add(account.id);
      if (account.upstreamSiteName) current.siteSamples.add(account.upstreamSiteName);
      grouped.set(model, current);
    }
  }
  return Array.from(grouped.values())
    .map((item) => ({
      model: item.model,
      accountCount: item.accountIds.size,
      siteSamples: Array.from(item.siteSamples).slice(0, 3),
    }))
    .sort((left, right) => right.accountCount - left.accountCount || left.model.localeCompare(right.model));
}

export function cleanupReasonLabel(reason: string) {
  switch (reason) {
    case "site_not_support_checkin":
      return "站点不支持签到";
    case "last_checkin_unsupported":
      return "上次签到不支持";
    default:
      return reason || "不支持签到";
  }
}

export function keyIssueLabel(account: Account) {
  if (account.apiKeyStatus && !["valid", "unchecked"].includes(account.apiKeyStatus)) {
    return apiKeyStatusLabel(account.apiKeyStatus);
  }
  if (!account.apiKeyLastCheckedAt || account.apiKeyStatus === "unchecked") return "未检测";
  if (isStaleAPIKeyCheck(account)) return "超过 24 小时未重测";
  return apiKeyStatusLabel(account.apiKeyStatus || "unchecked");
}

export function downloadJSON(fileName: string, body: string) {
  const blob = new Blob([body], { type: "application/json;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = fileName;
  anchor.click();
  URL.revokeObjectURL(url);
}
