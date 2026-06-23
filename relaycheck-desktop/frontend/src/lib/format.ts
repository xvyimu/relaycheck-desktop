import type { BalanceSnapshot, ModelPriceComparison, ModelPricingSource } from "@/types";

export function formatConfidence(value?: number) {
  if (value === undefined || value === null) return "-";
  return `${Math.round(value * 100)}%`;
}

/** 从名称中提取 1-2 个字符作为 avatar 占位符。优先取英文首字母（大写），否则取前 1-2 个中文字符。 */
export function channelInitials(name: string): string {
  const stripped = name.replace(/[^a-zA-Z0-9一-鿿]/g, "");
  if (!stripped) return "?";
  const chars = Array.from(stripped);
  if (/[一-鿿]/.test(chars[0])) {
    return chars.slice(0, 2).join("");
  }
  return chars.slice(0, 2).join("").toUpperCase();
}

export function formatTime(value: string) {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
}

export function formatBuildTime(value: string) {
  if (!value || value === "local build") return "本地构建";
  return formatTime(value);
}

export function compactJSONPreview(value: unknown) {
  if (!value || (typeof value === "object" && Object.keys(value as Record<string, unknown>).length === 0)) {
    return "暂无可展示的识别原始数据";
  }
  const text = JSON.stringify(value, null, 2);
  return text.length > 2200 ? `${text.slice(0, 2200)}\n... 已截断` : text;
}

export function formatDuration(seconds: number) {
  if (!Number.isFinite(seconds) || seconds <= 0) return "现在";
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  if (days > 0) return `${days} 天 ${hours} 小时`;
  if (hours > 0) return `${hours} 小时 ${minutes} 分钟`;
  return `${Math.max(1, minutes)} 分钟`;
}

export function formatDurationShort(seconds: number) {
  if (!Number.isFinite(seconds) || seconds <= 0) return "现在";
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  if (hours > 0) return `${hours}h ${minutes}m`;
  return `${Math.max(1, minutes)}m`;
}

export function formatBytes(value: number) {
  if (!Number.isFinite(value) || value <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB"];
  let size = value;
  let unitIndex = 0;
  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024;
    unitIndex += 1;
  }
  return `${size >= 10 || unitIndex === 0 ? size.toFixed(0) : size.toFixed(1)} ${units[unitIndex]}`;
}

export function formatPricingSource(source: ModelPricingSource) {
  const parts: string[] = [];
  if (typeof source.promptRatio === "number") parts.push(`输入倍率 ${formatCompactNumber(source.promptRatio)}`);
  if (typeof source.completionRatio === "number") parts.push(`输出倍率 ${formatCompactNumber(source.completionRatio)}`);
  if (typeof source.price === "number") parts.push(`价格 ${formatCompactNumber(source.price)}${source.currency || source.unit || ""}`);
  if (source.upstreamModel) parts.push(`映射 ${source.upstreamModel}`);
  if (!parts.length) parts.push(source.source || "配置来源");
  return parts.join(" · ");
}

export function formatPriceComparisonMeta(item: ModelPriceComparison) {
  const parts: string[] = [];
  if (typeof item.lowestPromptRatio === "number") parts.push(`最低输入 ${formatCompactNumber(item.lowestPromptRatio)}`);
  if (typeof item.lowestCompletionRatio === "number") parts.push(`输出 ${formatCompactNumber(item.lowestCompletionRatio)}`);
  if (typeof item.lowestPrice === "number") parts.push(`价格 ${formatCompactNumber(item.lowestPrice)}`);
  if (item.usableAccountCount) parts.push(`${item.usableAccountCount} 个 Key 可用`);
  if (item.fastestLatencyMs) parts.push(`${item.fastestLatencyMs}ms`);
  if (!parts.length) parts.push(`${item.sourceCount} 条来源 · ${item.siteCount} 站点`);
  return parts.join(" · ");
}

export function formatPriceComparisonBadge(item: ModelPriceComparison) {
  if (typeof item.lowestPromptRatio === "number") return `x${formatCompactNumber(item.lowestPromptRatio)}`;
  if (typeof item.lowestPrice === "number") return formatCompactNumber(item.lowestPrice);
  if (item.usableAccountCount) return `${item.usableAccountCount} 可用`;
  return `${item.sourceCount} 源`;
}

export function formatBalanceValue(value: number | undefined | null, unit: string) {
  if (value === undefined || value === null) return "-";
  const normalizedUnit = (unit || "unknown").toLowerCase();
  if (normalizedUnit === "quota") {
    return `${formatCompactNumber(value)} quota（约 ${formatUSD(value / 500000)}）`;
  }
  if (normalizedUnit === "token") {
    return `${formatCompactNumber(value)} token`;
  }
  if (normalizedUnit === "usd") {
    return formatUSD(value);
  }
  if (normalizedUnit === "cny") {
    return `¥${formatDecimal(value)}`;
  }
  return `${formatCompactNumber(value)}${unit && unit !== "unknown" ? ` ${unit}` : ""}`;
}

export function formatBalanceMeta(snapshot: BalanceSnapshot) {
  const parts = [];
  if (snapshot.usedQuota !== undefined && snapshot.usedQuota !== null) {
    parts.push(`已用 ${formatBalanceValue(snapshot.usedQuota, snapshot.unit)}`);
  }
  if (snapshot.totalQuota !== undefined && snapshot.totalQuota !== null) {
    parts.push(`总量 ${formatBalanceValue(snapshot.totalQuota, snapshot.unit)}`);
  }
  return parts.length ? parts.join(" · ") : "暂无用量详情";
}

export function formatBalanceGroup(units: Record<string, number>) {
  const entries = Object.entries(units).filter(([, value]) => Number.isFinite(value));
  if (!entries.length) return "-";
  return entries
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([unit, value]) => formatBalanceValue(value, unit))
    .join(" / ");
}

export function formatCompactNumber(value: number) {
  const abs = Math.abs(value);
  if (abs >= 100000000) return `${trimDecimal(value / 100000000, 2)} 亿`;
  if (abs >= 10000) return `${trimDecimal(value / 10000, 2)} 万`;
  return formatDecimal(value);
}

export function formatUSD(value: number) {
  if (value > 0 && value < 0.01) return "< $0.01";
  return `$${formatDecimal(value, 2)}`;
}

export function formatDecimal(value: number, maximumFractionDigits = 2) {
  return new Intl.NumberFormat("zh-CN", {
    maximumFractionDigits,
  }).format(value);
}

export function trimDecimal(value: number, digits: number) {
  return value.toFixed(digits).replace(/\.?0+$/, "");
}
