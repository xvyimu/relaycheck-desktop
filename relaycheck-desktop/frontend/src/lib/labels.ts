import type { APIKeyTestResult, ModelPricingSource } from "@/types";

export function errorClassLabel(errorClass: string): string {
  const labels: Record<string, string> = {
    validation_error: "参数错误",
    auth_error: "登录失效",
    permission_error: "权限不足",
    not_found: "资源不存在",
    method_not_allowed: "方法不支持",
    conflict: "状态冲突",
    rate_limited: "请求过快",
    server_error: "服务异常",
    request_error: "请求异常",
    bad_response: "响应异常",
  };
  return labels[errorClass] || errorClass;
}

export function diagnosticLevelLabel(level: string): string {
  const labels: Record<string, string> = {
    success: "正常",
    info: "提示",
    warning: "注意",
    danger: "需处理",
  };
  return labels[level] || level || "未知";
}

export function channelSourceLabel(sourceType: string): string {
  const labels: Record<string, string> = {
    manual: "手动添加",
    sqlite: "SQLite 导入",
    admin_api: "后台 API 导入",
    legacy: "旧配置导入",
    unknown: "未知来源",
  };
  return labels[sourceType] || sourceType || "未知来源";
}

export function channelSourceSyncLabel(value: string): string {
  const labels: Record<string, string> = {
    active: "源端存在",
    missing: "源端已移除",
    archived: "已归档",
  };
  return labels[value] || value || "源端存在";
}

export function localNewAPISourceLabel(source: string): string {
  const labels: Record<string, string> = {
    sqlite_import: "SQLite",
    admin_api_import: "后台 API",
    port_scan: "端口扫描",
  };
  return labels[source] || source || "未知来源";
}

export function syncCapabilityLabel(value: string): string {
  const labels: Record<string, string> = {
    sqlite: "可直接同步",
    admin_api_saved_token: "已保存令牌",
    admin_api_token_required: "需访问令牌",
    unsupported: "暂不可同步",
  };
  return labels[value] || value || "暂不可同步";
}

export function syncSourceLabel(value: string): string {
  const labels: Record<string, string> = {
    sqlite: "SQLite",
    admin_api: "后台 API",
  };
  return labels[value] || value || "未知来源";
}

export function syncActionLabel(value: string): string {
  const labels: Record<string, string> = {
    new: "新增",
    changed: "变更",
    unchanged: "不变",
    skipped: "跳过",
    removed: "移除",
  };
  return labels[value] || value || "未知";
}

export function syncSummaryScopeLabel(value: string): string {
  const labels: Record<string, string> = {
    single: "单实例",
    all: "全部实例",
    "mark-missing": "源端对齐",
  };
  return labels[value] || value || "同步结果";
}

export function upstreamKindLabel(kind: string): string {
  const labels: Record<string, string> = {
    newapi: "NewAPI",
    oneapi: "OneAPI",
    sub2api: "Sub2API",
    openai_compatible: "OpenAI 兼容",
    official_provider: "官方供应商",
    modified_relay: "魔改中转",
    unknown: "待识别",
  };
  return labels[kind] || kind || "待识别";
}

export function channelStatusLabel(status?: string): string {
  const labels: Record<string, string> = {
    enabled: "启用",
    disabled: "停用",
    healthy: "正常",
    degraded: "异常",
    auth_required: "需授权",
    unreachable: "不可达",
    unknown: "未知",
  };
  return labels[status || ""] || status || "未知";
}

export function channelModelStatusLabel(status: string): string {
  const labels: Record<string, string> = {
    live_key: "实时模型",
    raw_only: "配置模型",
    key_invalid: "Key 异常",
    failed: "同步失败",
    empty: "无模型",
    unchecked: "未同步",
  };
  return labels[status] || status || "未同步";
}

export function auditActionLabel(action: string): string {
  const labels: Record<string, string> = {
    "auth.login": "登录成功",
    "auth.login_failed": "登录失败",
    "auth.logout": "退出登录",
    "settings.updated": "设置变更",
    "backup.created": "创建备份",
    "backup.deleted": "删除备份",
    "backup.restored": "恢复备份",
    "account.created": "新增账号",
    "account.updated": "更新账号",
    "account.deleted": "删除账号",
    "upstream_site.deleted": "删除站点",
  };
  return labels[action] || action || "系统事件";
}

export function auditLevelLabel(level: string): string {
  const labels: Record<string, string> = {
    info: "信息",
    warning: "需留意",
    error: "错误",
  };
  return labels[level] || level || "信息";
}

export function schedulerStatusLabel(status: string): string {
  const labels: Record<string, string> = {
    scheduled: "已计划",
    running: "运行中",
    success: "成功",
    warning: "部分异常",
    failed: "失败",
    skipped: "已跳过",
    idle: "待机",
  };
  return labels[status] || status || "待机";
}

export function statusLabel(status: string): string {
  const labels: Record<string, string> = {
    success: "成功",
    already_checked: "今日已签",
    unsupported: "不支持/未开启",
    auth_expired: "需授权",
    failed: "失败",
  };
  return labels[status] || status || "未知";
}

export function loginStatusLabel(status: string): string {
  const labels: Record<string, string> = {
    valid: "登录有效",
    expired: "登录失效",
    manual_required: "需手动登录",
    captcha_required: "需验证码",
    two_factor_required: "需二次验证",
    disabled: "已禁用",
    unknown: "未知登录态",
  };
  return labels[status] || status || "未知登录态";
}

export function apiKeyStatusLabel(status: string): string {
  const labels: Record<string, string> = {
    valid: "密钥有效",
    expired: "密钥失效",
    unknown: "密钥未知",
    unchecked: "密钥未测",
    missing: "无密钥",
  };
  return labels[status] || status || "密钥未测";
}

export function usageTrendLabel(trend: string): string {
  const labels: Record<string, string> = {
    down: "余额下降",
    up: "余额回升",
    flat: "基本持平",
  };
  return labels[trend] || trend || "未知趋势";
}

export function priceLevelLabel(level: string): string {
  switch (level) {
    case "cheap":
      return "低价/轻量";
    case "low":
      return "偏低";
    case "standard":
      return "标准";
    case "high":
      return "高价/旗舰";
    default:
      return "未知";
  }
}

export function priceLevelShort(level: string): string {
  switch (level) {
    case "cheap":
      return "低";
    case "low":
      return "偏低";
    case "standard":
      return "标准";
    case "high":
      return "高";
    default:
      return "?";
  }
}

export function pricingCacheStatusLabel(status: string): string {
  const labels: Record<string, string> = {
    success: "在线价格",
    empty: "未识别价格",
    failed: "探测失败",
    unknown: "未探测",
  };
  return labels[status] || status || "未探测";
}

export function pricingSourceBadge(source: ModelPricingSource): string {
  if (typeof source.price === "number") return "价格";
  if (typeof source.promptRatio === "number" || typeof source.completionRatio === "number") return "倍率";
  if (source.upstreamModel) return "映射";
  return source.confidence === "high" ? "高" : "来源";
}

export function formatAPIKeyTestMessage(result: APIKeyTestResult): string {
  const parts = [`${apiKeyStatusLabel(result.status)}`];
  if (result.modelCount !== undefined) parts.push(`模型 ${result.modelCount} 个`);
  if (result.testedModel) parts.push(`测试 ${result.testedModel}`);
  if (result.modelTestLatencyMs !== undefined && result.modelTestLatencyMs > 0) parts.push(`${result.modelTestLatencyMs}ms`);
  if (result.testedModel) parts.push(result.modelUsable ? "模型可用" : "模型不可用");
  if (result.sampleModels?.length) parts.push(`样例：${result.sampleModels.slice(0, 4).join("、")}`);
  if (result.message) parts.push(result.message);
  if (result.modelTestMessage && !result.message?.includes(result.modelTestMessage)) parts.push(result.modelTestMessage);
  return parts.join(" · ");
}
