import type { DeleteSiteResult } from "@/api/sites";
import type { UpstreamSite } from "@/types";

/** 站点删除二次确认文案：明示级联范围与备份建议。 */
export function siteDeleteConfirmMessage(site: Pick<UpstreamSite, "name" | "accountCount">): string {
  const accounts = site.accountCount || 0;
  return [
    `确认删除站点「${site.name}」？`,
    "",
    "将在单事务内级联删除：",
    `- 关联账号：${accounts} 个`,
    "- 签到日志 / 余额快照 / 排程 / 价格缓存",
    "",
    "此操作不可撤销。删除前请先备份（设置 → 立即备份，或复制 data\\relaycheck.db*）。",
  ].join("\n");
}

/** 将后端级联计数格式化为用户可读结果。 */
export function formatSiteDeleteResult(siteName: string, result: DeleteSiteResult): string {
  return [
    `已删除站点「${siteName}」。`,
    `级联：账号 ${result.accounts}、签到日志 ${result.checkinLogs}、余额 ${result.balanceSnapshots}、排程 ${result.schedules}、价格缓存 ${result.pricingCache}。`,
  ].join(" ");
}
