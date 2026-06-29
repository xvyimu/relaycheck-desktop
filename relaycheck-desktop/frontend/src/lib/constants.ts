import type { NavItem } from "@/types";

export const NAV_ITEMS = [
  { key: "dashboard", label: "总览", icon: "dashboard", description: "关键指标与运行状态" },
  { key: "channels", label: "渠道", icon: "channels", description: "NewAPI 导入渠道" },
  { key: "sites", label: "上游站点", icon: "sites", description: "识别面板型中转站" },
  { key: "accounts", label: "账号", icon: "accounts", description: "授权、密钥与会话" },
  { key: "checkins", label: "签到", icon: "checkins", description: "手动/批量签到日志" },
  { key: "balances", label: "余额", icon: "balances", description: "额度快照与用量" },
  { key: "notifications", label: "通知", icon: "notifications", description: "站内提醒中心" },
  { key: "scan", label: "本机扫描", icon: "scan", description: "发现与导入 NewAPI" },
  { key: "settings", label: "设置", icon: "settings", description: "备份、恢复与本地配置" },
] as const satisfies readonly NavItem[];

export const STATUS_ICON_SUCCESS_LEVELS: ReadonlySet<string> = new Set(["success", "valid", "active", "live_key", "scheduled", "enabled", "ok"]);
export const STATUS_ICON_WARNING_LEVELS: ReadonlySet<string> = new Set(["warning", "missing", "archived", "manual_required", "captcha_required", "two_factor_required", "unchecked", "idle", "partial"]);
export const STATUS_ICON_DANGER_LEVELS: ReadonlySet<string> = new Set(["danger", "error", "failed", "invalid", "expired", "auth_expired", "key_invalid", "unreachable", "disabled"]);

export const TARGET_RELAY_KINDS: ReadonlySet<string> = new Set(["newapi", "oneapi", "sub2api", "modified_relay"]);
export const PROBLEM_LOGIN_STATUSES: ReadonlySet<string> = new Set(["expired", "manual_required", "captcha_required", "two_factor_required"]);
export const PROBLEM_CHECKIN_STATUSES: ReadonlySet<string> = new Set(["auth_expired", "manual_required", "failed"]);
export const SUCCESSFUL_CHECKIN_STATUSES: ReadonlySet<string> = new Set(["success", "already_checked"]);

export const CHANNEL_RAW_SEARCH_KEYS: ReadonlySet<string> = new Set([
  "name",
  "type",
  "platform",
  "provider",
  "group",
  "groups",
  "tag",
  "tags",
  "note",
  "notes",
  "remark",
  "remarks",
  "description",
  "desc",
]);

export const CHANNELS_INITIAL_VISIBLE_LIMIT = 24;
export const CHANNELS_VISIBLE_INCREMENT = 24;
