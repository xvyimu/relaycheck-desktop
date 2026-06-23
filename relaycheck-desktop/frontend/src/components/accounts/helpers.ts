import { PROBLEM_CHECKIN_STATUSES, PROBLEM_LOGIN_STATUSES } from "@/lib/constants";
import { channelInitials } from "@/lib/format";
import type { Account } from "@/types";

export function accountDomainLabel(account: Account): string {
  const raw = account.upstreamSiteBaseUrl || account.upstreamSiteName || account.displayName || "";
  try {
    const url = raw.startsWith("http://") || raw.startsWith("https://") ? new URL(raw) : new URL(`https://${raw}`);
    return url.hostname.replace(/^www\./, "") || raw;
  } catch {
    return raw.replace(/^https?:\/\//, "").replace(/^www\./, "").split("/")[0] || "unknown";
  }
}

export function accountAvatarLabel(account: Account): string {
  const host = accountDomainLabel(account);
  const parts = host.split(".").filter(Boolean);
  const primary = parts.length >= 2 ? parts[parts.length - 2] : parts[0] || host;
  return channelInitials(primary || account.upstreamSiteName || account.displayName || "AC");
}

export function accountBackendShort(kind: string): string {
  const labels: Record<string, string> = {
    newapi: "NEW",
    oneapi: "ONE",
    sub2api: "SUB",
    modified_relay: "MOD",
    openai_compatible: "API",
    official_provider: "OFF",
    unknown: "UNK",
  };
  return labels[kind] || kind.slice(0, 3).toUpperCase() || "UNK";
}

export function defaultLoginUrl(baseUrl: string): string {
  const value = baseUrl.trim().replace(/\/+$/, "");
  return value ? `${value}/login` : "";
}

export function isLocalURL(value: string): boolean {
  return /^(http:\/\/)?(localhost|127\.0\.0\.1|\[::1\])(?::|\/|$)/i.test(value);
}

export function isProblemAccount(account: Account): boolean {
  return PROBLEM_LOGIN_STATUSES.has(account.loginStatus) || PROBLEM_CHECKIN_STATUSES.has(account.lastCheckinStatus || "");
}

export function compareAccounts(left: Account, right: Account, sortKey: string): number {
  const byID = left.id.localeCompare(right.id);
  const compareNumber = (leftValue: number | undefined, rightValue: number | undefined, direction: "asc" | "desc") => {
    const leftScore = Number.isFinite(leftValue) ? Number(leftValue) : (direction === "asc" ? Number.POSITIVE_INFINITY : Number.NEGATIVE_INFINITY);
    const rightScore = Number.isFinite(rightValue) ? Number(rightValue) : (direction === "asc" ? Number.POSITIVE_INFINITY : Number.NEGATIVE_INFINITY);
    return direction === "asc" ? leftScore - rightScore : rightScore - leftScore;
  };
  const compareTime = (leftValue: string | undefined, rightValue: string | undefined, direction: "asc" | "desc") => {
    const parse = (value: string | undefined) => {
      const timestamp = value ? new Date(value).getTime() : NaN;
      return Number.isFinite(timestamp) ? timestamp : (direction === "asc" ? Number.POSITIVE_INFINITY : Number.NEGATIVE_INFINITY);
    };
    const diff = direction === "asc" ? parse(leftValue) - parse(rightValue) : parse(rightValue) - parse(leftValue);
    return diff || byID;
  };

  switch (sortKey) {
    case "balance_asc":
      return compareNumber(left.balance, right.balance, "asc") || byID;
    case "balance_desc":
      return compareNumber(left.balance, right.balance, "desc") || byID;
    case "latency_asc":
      return compareNumber(left.apiKeyLatencyMs, right.apiKeyLatencyMs, "asc") || byID;
    case "latency_desc":
      return compareNumber(left.apiKeyLatencyMs, right.apiKeyLatencyMs, "desc") || byID;
    case "id_asc":
      return byID;
    case "id_desc":
      return -byID;
    case "last_checkin_asc":
      return compareTime(left.lastCheckinAt, right.lastCheckinAt, "asc");
    case "last_checkin_desc":
    default:
      return compareTime(left.lastCheckinAt, right.lastCheckinAt, "desc");
  }
}