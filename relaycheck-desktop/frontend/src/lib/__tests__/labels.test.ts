import { describe, it, expect } from "vitest";
import {
  diagnosticLevelLabel,
  channelSourceLabel,
  channelSourceSyncLabel,
  upstreamKindLabel,
  channelModelStatusLabel,
  auditActionLabel,
  auditLevelLabel,
  schedulerStatusLabel,
  statusLabel,
  loginStatusLabel,
  apiKeyStatusLabel,
  priceLevelLabel,
  priceLevelShort,
  pricingCacheStatusLabel,
  pricingSourceBadge,
} from "../labels";

describe("diagnosticLevelLabel", () => {
  it("returns correct labels for known levels", () => {
    expect(diagnosticLevelLabel("success")).toBe("正常");
    expect(diagnosticLevelLabel("info")).toBe("提示");
    expect(diagnosticLevelLabel("warning")).toBe("注意");
    expect(diagnosticLevelLabel("danger")).toBe("需处理");
  });

  it("returns the level string for unknown values", () => {
    expect(diagnosticLevelLabel("custom")).toBe("custom");
  });

  it("returns 未知 for empty string", () => {
    expect(diagnosticLevelLabel("")).toBe("未知");
  });
});

describe("channelSourceLabel", () => {
  it("returns correct labels for known source types", () => {
    expect(channelSourceLabel("manual")).toBe("手动添加");
    expect(channelSourceLabel("sqlite")).toBe("SQLite 导入");
    expect(channelSourceLabel("admin_api")).toBe("后台 API 导入");
    expect(channelSourceLabel("legacy")).toBe("旧配置导入");
    expect(channelSourceLabel("unknown")).toBe("未知来源");
  });

  it("returns the source type for unknown values", () => {
    expect(channelSourceLabel("custom_type")).toBe("custom_type");
  });

  it("returns 未知来源 for empty string", () => {
    expect(channelSourceLabel("")).toBe("未知来源");
  });
});

describe("channelSourceSyncLabel", () => {
  it("returns correct labels for known sync statuses", () => {
    expect(channelSourceSyncLabel("active")).toBe("源端存在");
    expect(channelSourceSyncLabel("missing")).toBe("源端已移除");
    expect(channelSourceSyncLabel("archived")).toBe("已归档");
  });

  it("defaults to 源端存在 for unknown value", () => {
    expect(channelSourceSyncLabel("")).toBe("源端存在");
  });
});

describe("upstreamKindLabel", () => {
  it("returns correct labels for known kinds", () => {
    expect(upstreamKindLabel("newapi")).toBe("NewAPI");
    expect(upstreamKindLabel("oneapi")).toBe("OneAPI");
    expect(upstreamKindLabel("sub2api")).toBe("Sub2API");
    expect(upstreamKindLabel("openai_compatible")).toBe("OpenAI 兼容");
    expect(upstreamKindLabel("official_provider")).toBe("官方供应商");
    expect(upstreamKindLabel("modified_relay")).toBe("魔改中转");
    expect(upstreamKindLabel("unknown")).toBe("待识别");
  });

  it("defaults to 待识别 for unknown kind", () => {
    expect(upstreamKindLabel("")).toBe("待识别");
  });
});

describe("channelModelStatusLabel", () => {
  it("returns correct labels for known statuses", () => {
    expect(channelModelStatusLabel("live_key")).toBe("实时模型");
    expect(channelModelStatusLabel("raw_only")).toBe("配置模型");
    expect(channelModelStatusLabel("key_invalid")).toBe("Key 异常");
    expect(channelModelStatusLabel("failed")).toBe("同步失败");
    expect(channelModelStatusLabel("empty")).toBe("无模型");
    expect(channelModelStatusLabel("unchecked")).toBe("未同步");
  });

  it("defaults to 未同步 for unknown status", () => {
    expect(channelModelStatusLabel("")).toBe("未同步");
  });
});

describe("auditActionLabel", () => {
  it("returns correct labels for known actions", () => {
    expect(auditActionLabel("auth.login")).toBe("登录成功");
    expect(auditActionLabel("auth.login_failed")).toBe("登录失败");
    expect(auditActionLabel("auth.logout")).toBe("退出登录");
    expect(auditActionLabel("settings.updated")).toBe("设置变更");
    expect(auditActionLabel("backup.created")).toBe("创建备份");
    expect(auditActionLabel("backup.deleted")).toBe("删除备份");
    expect(auditActionLabel("backup.restored")).toBe("恢复备份");
    expect(auditActionLabel("account.created")).toBe("新增账号");
    expect(auditActionLabel("account.updated")).toBe("更新账号");
    expect(auditActionLabel("account.deleted")).toBe("删除账号");
    expect(auditActionLabel("upstream_site.deleted")).toBe("删除站点");
  });

  it("defaults to 系统事件 for unknown action", () => {
    expect(auditActionLabel("")).toBe("系统事件");
  });
});

describe("auditLevelLabel", () => {
  it("returns correct labels for known levels", () => {
    expect(auditLevelLabel("info")).toBe("信息");
    expect(auditLevelLabel("warning")).toBe("需留意");
    expect(auditLevelLabel("error")).toBe("错误");
  });

  it("defaults to 信息 for unknown level", () => {
    expect(auditLevelLabel("")).toBe("信息");
  });
});

describe("schedulerStatusLabel", () => {
  it("returns correct labels for known statuses", () => {
    expect(schedulerStatusLabel("scheduled")).toBe("已计划");
    expect(schedulerStatusLabel("running")).toBe("运行中");
    expect(schedulerStatusLabel("success")).toBe("成功");
    expect(schedulerStatusLabel("warning")).toBe("部分异常");
    expect(schedulerStatusLabel("failed")).toBe("失败");
    expect(schedulerStatusLabel("skipped")).toBe("已跳过");
    expect(schedulerStatusLabel("idle")).toBe("待机");
  });

  it("defaults to 待机 for unknown status", () => {
    expect(schedulerStatusLabel("")).toBe("待机");
  });
});

describe("statusLabel", () => {
  it("returns correct labels for known statuses", () => {
    expect(statusLabel("success")).toBe("成功");
    expect(statusLabel("already_checked")).toBe("今日已签");
    expect(statusLabel("unsupported")).toBe("不支持/未开启");
    expect(statusLabel("auth_expired")).toBe("需授权");
    expect(statusLabel("failed")).toBe("失败");
  });

  it("defaults to 未签到 for unknown", () => {
    expect(statusLabel("")).toBe("未签到");
  });
});

describe("loginStatusLabel", () => {
  it("returns correct labels for known statuses", () => {
    expect(loginStatusLabel("valid")).toBe("登录有效");
    expect(loginStatusLabel("expired")).toBe("登录失效");
    expect(loginStatusLabel("manual_required")).toBe("需手动登录");
    expect(loginStatusLabel("captcha_required")).toBe("需验证码");
    expect(loginStatusLabel("two_factor_required")).toBe("需二次验证");
    expect(loginStatusLabel("disabled")).toBe("已禁用");
    expect(loginStatusLabel("unknown")).toBe("未知登录态");
  });

  it("defaults to 未知登录态 for unknown", () => {
    expect(loginStatusLabel("")).toBe("未知登录态");
  });
});

describe("apiKeyStatusLabel", () => {
  it("returns correct labels for known statuses", () => {
    expect(apiKeyStatusLabel("valid")).toBe("密钥有效");
    expect(apiKeyStatusLabel("expired")).toBe("密钥失效");
    expect(apiKeyStatusLabel("unknown")).toBe("密钥未知");
    expect(apiKeyStatusLabel("unchecked")).toBe("密钥未测");
    expect(apiKeyStatusLabel("missing")).toBe("无密钥");
  });

  it("defaults to 密钥未测 for unknown", () => {
    expect(apiKeyStatusLabel("")).toBe("密钥未测");
  });
});

describe("priceLevelLabel", () => {
  it("returns correct labels for known levels", () => {
    expect(priceLevelLabel("cheap")).toBe("低价/轻量");
    expect(priceLevelLabel("low")).toBe("偏低");
    expect(priceLevelLabel("standard")).toBe("标准");
    expect(priceLevelLabel("high")).toBe("高价/旗舰");
  });

  it("returns 未知 for unknown level", () => {
    expect(priceLevelLabel("premium")).toBe("未知");
  });
});

describe("priceLevelShort", () => {
  it("returns short labels for known levels", () => {
    expect(priceLevelShort("cheap")).toBe("低");
    expect(priceLevelShort("low")).toBe("偏低");
    expect(priceLevelShort("standard")).toBe("标准");
    expect(priceLevelShort("high")).toBe("高");
  });

  it("returns ? for unknown level", () => {
    expect(priceLevelShort("premium")).toBe("?");
  });
});

describe("pricingCacheStatusLabel", () => {
  it("returns correct labels for known statuses", () => {
    expect(pricingCacheStatusLabel("success")).toBe("在线价格");
    expect(pricingCacheStatusLabel("empty")).toBe("未识别价格");
    expect(pricingCacheStatusLabel("failed")).toBe("探测失败");
    expect(pricingCacheStatusLabel("unknown")).toBe("未探测");
  });

  it("defaults to 未探测 for unknown", () => {
    expect(pricingCacheStatusLabel("")).toBe("未探测");
  });
});

describe("pricingSourceBadge", () => {
  const base = { channelId: "1", channelName: "test", kind: "newapi", model: "gpt-4", source: "admin", fieldPath: "/path", confidence: "high" as const };

  it("returns 价格 when price is present", () => {
    expect(pricingSourceBadge({ ...base, price: 0.03 })).toBe("价格");
  });

  it("returns 倍率 when promptRatio is present", () => {
    expect(pricingSourceBadge({ ...base, promptRatio: 2.0 })).toBe("倍率");
  });

  it("returns 映射 when upstreamModel is present", () => {
    expect(pricingSourceBadge({ ...base, upstreamModel: "gpt-4-32k" })).toBe("映射");
  });

  it("returns 高 for high confidence with no other info", () => {
    expect(pricingSourceBadge(base)).toBe("高");
  });

  it("returns 来源 for low confidence with no other info", () => {
    expect(pricingSourceBadge({ ...base, confidence: "low" })).toBe("来源");
  });
});
