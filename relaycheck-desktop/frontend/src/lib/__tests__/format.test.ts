import { describe, it, expect } from "vitest";
import {
  formatConfidence,
  channelInitials,
  formatTime,
  formatBuildTime,
  compactJSONPreview,
  formatDuration,
  formatDurationShort,
  formatBytes,
  formatCompactNumber,
  formatUSD,
  formatDecimal,
  trimDecimal,
  formatBalanceValue,
  formatBalanceMeta,
  formatBalanceGroup,
  formatPricingSource,
  formatPriceComparisonMeta,
  formatPriceComparisonBadge,
} from "../format";

describe("formatConfidence", () => {
  it("formats 0.5 as 50%", () => {
    expect(formatConfidence(0.5)).toBe("50%");
  });

  it("formats 1 as 100%", () => {
    expect(formatConfidence(1)).toBe("100%");
  });

  it("formats 0 as 0%", () => {
    expect(formatConfidence(0)).toBe("0%");
  });

  it("formats 0.856 as 86% (rounds, not truncates)", () => {
    expect(formatConfidence(0.856)).toBe("86%");
  });

  it("formats 0.995 as 100% (rounds up)", () => {
    expect(formatConfidence(0.995)).toBe("100%");
  });

  it("returns dash for undefined", () => {
    expect(formatConfidence(undefined)).toBe("-");
  });

  it("returns dash for null", () => {
    expect(formatConfidence(null as unknown as undefined)).toBe("-");
  });
});

describe("channelInitials", () => {
  it("extracts first two English letters in uppercase", () => {
    expect(channelInitials("My Channel")).toBe("MY");
  });

  it("extracts first two Chinese characters", () => {
    expect(channelInitials("中文渠道")).toBe("中文");
  });

  it("extracts single Chinese character when only one present", () => {
    expect(channelInitials("中")).toBe("中");
  });

  it("returns ? for empty string", () => {
    expect(channelInitials("")).toBe("?");
  });

  it("returns ? for string with only special characters", () => {
    expect(channelInitials("@#$%")).toBe("?");
  });

  it("extracts from mixed Chinese and English", () => {
    expect(channelInitials("GPT-4")).toBe("GP");
  });

  it("extracts from numbers", () => {
    expect(channelInitials("1abc")).toBe("1A");
  });

  it("handles pure numeric string", () => {
    expect(channelInitials("12345")).toBe("12");
  });
});

describe("formatTime", () => {
  it("returns dash for empty string", () => {
    expect(formatTime("")).toBe("-");
  });

  it("returns original value for invalid date string", () => {
    expect(formatTime("not-a-date")).toBe("not-a-date");
  });

  it("formats a valid ISO date string", () => {
    const result = formatTime("2026-07-01T12:30:00Z");
    // zh-CN locale format — should contain year/month/day/hour/minute/second parts
    expect(result).not.toBe("-");
    expect(result).not.toBe("2026-07-01T12:30:00Z");
  });
});

describe("formatBuildTime", () => {
  it("returns 本地构建 for empty string", () => {
    expect(formatBuildTime("")).toBe("本地构建");
  });

  it("returns 本地构建 for 'local build'", () => {
    expect(formatBuildTime("local build")).toBe("本地构建");
  });

  it("delegates to formatTime for normal values", () => {
    const result = formatBuildTime("2026-07-01T12:30:00Z");
    expect(result).not.toBe("本地构建");
  });
});

describe("compactJSONPreview", () => {
  it("returns placeholder for empty object", () => {
    expect(compactJSONPreview({})).toBe("暂无可展示的识别原始数据");
  });

  it("returns placeholder for null", () => {
    expect(compactJSONPreview(null)).toBe("暂无可展示的识别原始数据");
  });

  it("returns placeholder for undefined", () => {
    expect(compactJSONPreview(undefined)).toBe("暂无可展示的识别原始数据");
  });

  it("returns placeholder for empty string", () => {
    expect(compactJSONPreview("")).toBe("暂无可展示的识别原始数据");
  });

  it("formats a simple object as JSON", () => {
    const result = compactJSONPreview({ key: "value" });
    expect(result).toContain("key");
    expect(result).toContain("value");
  });

  it("truncates long JSON output", () => {
    const big = { data: "x".repeat(3000) };
    const result = compactJSONPreview(big);
    expect(result).toContain("已截断");
  });
});

describe("formatDuration", () => {
  it("returns 现在 for 0", () => {
    expect(formatDuration(0)).toBe("现在");
  });

  it("returns 现在 for negative", () => {
    expect(formatDuration(-5)).toBe("现在");
  });

  it("returns 现在 for NaN", () => {
    expect(formatDuration(NaN)).toBe("现在");
  });

  it("returns 现在 for Infinity", () => {
    expect(formatDuration(Infinity)).toBe("现在");
  });

  it("formats minutes", () => {
    expect(formatDuration(120)).toBe("2 分钟");
  });

  it("formats hours and minutes", () => {
    expect(formatDuration(3660)).toBe("1 小时 1 分钟");
  });

  it("formats days and hours", () => {
    expect(formatDuration(86400 + 3600)).toBe("1 天 1 小时");
  });

  it("clamps minimum to 1 minute", () => {
    expect(formatDuration(30)).toBe("1 分钟");
  });
});

describe("formatDurationShort", () => {
  it("returns 现在 for 0", () => {
    expect(formatDurationShort(0)).toBe("现在");
  });

  it("formats minutes only", () => {
    expect(formatDurationShort(120)).toBe("2m");
  });

  it("formats hours and minutes", () => {
    expect(formatDurationShort(3660)).toBe("1h 1m");
  });
});

describe("formatBytes", () => {
  it("returns 0 B for 0", () => {
    expect(formatBytes(0)).toBe("0 B");
  });

  it("returns 0 B for negative", () => {
    expect(formatBytes(-1)).toBe("0 B");
  });

  it("formats bytes", () => {
    expect(formatBytes(500)).toBe("500 B");
  });

  it("formats KB", () => {
    expect(formatBytes(1024)).toBe("1.0 KB");
  });

  it("formats MB with one decimal", () => {
    expect(formatBytes(1536)).toBe("1.5 KB");
  });

  it("formats GB", () => {
    expect(formatBytes(1024 * 1024 * 1024)).toBe("1.0 GB");
  });

  it("returns 0 B for NaN", () => {
    expect(formatBytes(NaN)).toBe("0 B");
  });
});

describe("formatCompactNumber", () => {
  it("formats numbers under 10000 directly", () => {
    const result = formatCompactNumber(999);
    expect(result).toContain("999");
  });

  it("formats 万 for 10000+", () => {
    const result = formatCompactNumber(15000);
    expect(result).toContain("万");
  });

  it("formats 亿 for 100000000+", () => {
    const result = formatCompactNumber(150000000);
    expect(result).toContain("亿");
  });
});

describe("formatUSD", () => {
  it("formats normal values", () => {
    expect(formatUSD(1.5)).toBe("$1.5");
  });

  it("shows < $0.01 for tiny positive values", () => {
    expect(formatUSD(0.005)).toBe("< $0.01");
  });

  it("formats zero", () => {
    expect(formatUSD(0)).toBe("$0");
  });
});

describe("formatDecimal", () => {
  it("formats with default 2 fraction digits", () => {
    expect(formatDecimal(1.234)).toBe("1.23");
  });

  it("respects custom fraction digits", () => {
    expect(formatDecimal(1.2345, 3)).toContain("1.23");
  });
});

describe("trimDecimal", () => {
  it("trims trailing zeros", () => {
    expect(trimDecimal(1.5, 2)).toBe("1.5");
  });

  it("trims trailing dot when all decimals are zero", () => {
    expect(trimDecimal(1.0, 2)).toBe("1");
  });
});

describe("formatBalanceValue", () => {
  it("returns dash for undefined", () => {
    expect(formatBalanceValue(undefined, "usd")).toBe("-");
  });

  it("returns dash for null", () => {
    expect(formatBalanceValue(null, "usd")).toBe("-");
  });

  it("formats quota unit", () => {
    const result = formatBalanceValue(500000, "quota");
    expect(result).toContain("quota");
  });

  it("formats token unit", () => {
    const result = formatBalanceValue(1000, "token");
    expect(result).toContain("token");
  });

  it("formats usd unit", () => {
    const result = formatBalanceValue(10, "usd");
    expect(result).toContain("$");
  });

  it("formats cny unit", () => {
    const result = formatBalanceValue(10, "cny");
    expect(result).toContain("¥");
  });

  it("formats unknown unit with fallback", () => {
    const result = formatBalanceValue(10, "custom");
    expect(result).toContain("10");
    expect(result).toContain("custom");
  });
});

describe("formatBalanceMeta", () => {
  it("returns placeholder when no data", () => {
    expect(formatBalanceMeta({ id: "1", accountName: "a", upstreamSiteName: "s", unit: "usd", createdAt: "" })).toBe("暂无用量详情");
  });

  it("formats used + total quotas", () => {
    const result = formatBalanceMeta({
      id: "1",
      accountName: "a",
      upstreamSiteName: "s",
      unit: "usd",
      createdAt: "",
      usedQuota: 100,
      totalQuota: 500,
    });
    expect(result).toContain("已用");
    expect(result).toContain("总量");
  });
});

describe("formatBalanceGroup", () => {
  it("returns dash for empty group", () => {
    expect(formatBalanceGroup({})).toBe("-");
  });

  it("formats a single unit", () => {
    const result = formatBalanceGroup({ usd: 10 });
    expect(result).toContain("$");
  });

  it("formats multiple units separated by /", () => {
    const result = formatBalanceGroup({ cny: 100, usd: 10 });
    expect(result).toContain("/");
  });
});

describe("formatPricingSource", () => {
  const base = { channelId: "1", channelName: "test", kind: "newapi", model: "gpt-4", source: "test", fieldPath: "/path", confidence: "high" };

  it("shows prompt ratio when present", () => {
    const result = formatPricingSource({ ...base, promptRatio: 2.0 });
    expect(result).toContain("输入倍率");
  });

  it("shows price when present", () => {
    const result = formatPricingSource({ ...base, price: 0.03 });
    expect(result).toContain("价格");
  });

  it("shows mapping when upstreamModel present", () => {
    const result = formatPricingSource({ ...base, upstreamModel: "gpt-4-32k" });
    expect(result).toContain("映射");
  });

  it("falls back to source label", () => {
    const result = formatPricingSource(base);
    expect(result).toBe("test");
  });
});

describe("formatPriceComparisonMeta", () => {
  const base = { model: "gpt-4", sourceCount: 2, siteCount: 1, usableAccountCount: 0 };

  it("shows lowest prompt ratio when present", () => {
    const result = formatPriceComparisonMeta({ ...base, lowestPromptRatio: 1.5 });
    expect(result).toContain("最低输入");
  });

  it("falls back to source and site counts", () => {
    const result = formatPriceComparisonMeta(base);
    expect(result).toContain("条来源");
    expect(result).toContain("站点");
  });
});

describe("formatPriceComparisonBadge", () => {
  const base = { model: "gpt-4", sourceCount: 2, siteCount: 1, usableAccountCount: 0 };

  it("shows prompt ratio badge", () => {
    const result = formatPriceComparisonBadge({ ...base, lowestPromptRatio: 3.0 });
    expect(result).toContain("x3");
  });

  it("shows usable count when present", () => {
    const result = formatPriceComparisonBadge({ ...base, usableAccountCount: 5 });
    expect(result).toContain("可用");
  });

  it("falls back to source count", () => {
    const result = formatPriceComparisonBadge(base);
    expect(result).toContain("源");
  });
});
