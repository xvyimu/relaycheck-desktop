import type { LoginDiscovery } from "@/types";

const sourceLabels: Record<string, string> = {
  manual: "手动指定",
  html_form: "登录表单",
  html_link: "页面链接",
  path_probe: "路径探测",
  spa_fallback: "SPA 回退",
  fallback: "默认回退",
};

export function loginDiscoverySourceLabel(source?: string) {
  const normalized = (source || "").toLowerCase();
  return sourceLabels[normalized] || source || "未知";
}

export function normalizeLoginDiscovery(value: unknown): LoginDiscovery | null {
  if (!value || typeof value !== "object") return null;

  const record = value as Record<string, unknown>;
  if (typeof record.url !== "string") return null;
  const url = record.url.trim();
  if (!url) return null;

  const candidates = Array.isArray(record.candidates)
    ? record.candidates
        .filter((candidate): candidate is string => typeof candidate === "string" && candidate.trim().length > 0)
        .map((candidate) => candidate.trim())
    : [];

  return {
    url,
    source: typeof record.source === "string" ? record.source : "",
    confidence: typeof record.confidence === "number" && Number.isFinite(record.confidence) ? record.confidence : 0,
    ...(candidates.length ? { candidates } : {}),
  };
}

export function parseLoginDiscovery(raw?: string): LoginDiscovery | null {
  if (!raw) return null;
  try {
    return normalizeLoginDiscovery(JSON.parse(raw));
  } catch {
    return null;
  }
}
