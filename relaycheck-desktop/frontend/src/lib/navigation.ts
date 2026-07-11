import type { ActionItem, ActionSample, NavigationIntent, TabKey } from "@/types";

type NavigableAction = Pick<ActionItem, "target" | "filter">;

export function actionItemNavigationIntent(item: NavigableAction): NavigationIntent {
  switch (item.target) {
    case "accounts":
      return { target: "accounts", accountStatus: item.filter === "problem" ? "problem" : "all" };
    case "checkins":
      return { target: "checkins", checkinStatus: item.filter === "problem" ? "problem" : "all" };
    case "channels":
      if (item.filter === "health") return { target: "channels", siteHealth: "risk" };
      if (item.filter === "missing") return { target: "channels", sourceStatus: "missing" };
      if (item.filter === "unknown") return { target: "channels", channelKind: "unknown", sourceStatus: "not_archived" };
      return { target: "channels" };
    case "balances":
      // balances tab is not implemented; land on accounts (usage/balance surface).
      return { target: "accounts", query: "余额" };
    case "sites":
      return { target: "sites", siteHealth: item.filter === "unreachable" ? "unreachable" : "all" };
    case "notifications":
      return { target: "notifications", unreadOnly: item.filter === "unread" };
    case "scan":
      return { target: "scan" };
    case "settings":
      return { target: "settings" };
    case "dashboard":
      return { target: "dashboard" };
    default:
      return { target: item.target satisfies TabKey };
  }
}

/**
 * Jump into Sites master-detail with a site preselected (S7.3 / β IA-1).
 * Prefer sites tab over accounts tab so left/right panes open on the site.
 * Accounts tab still accepts upstreamSiteId for legacy intents.
 */
export function siteAccountsNavigationIntent(
  upstreamSiteId: string,
): Omit<NavigationIntent, "target"> & { target: "sites" } {
  return {
    target: "sites",
    upstreamSiteId,
  };
}

/**
 * Deep-link from an Action Center sample row.
 * Site samples open Sites master-detail with that site selected (and keep list filter when known).
 * Other samples fall back to the parent action item intent.
 */
export function actionSampleNavigationIntent(
  item: NavigableAction,
  sample: Pick<ActionSample, "entityType" | "entityId">,
): NavigationIntent {
  const entityType = (sample.entityType || "").toLowerCase();
  const entityId = (sample.entityId || "").trim();
  if (entityType === "site" && entityId) {
    const base = actionItemNavigationIntent(item);
    if (base.target === "sites") {
      return { ...base, upstreamSiteId: entityId };
    }
    // channel-health and similar land on channels filter, but sample is a site id —
    // prefer sites master-detail so the operator can act on the site immediately.
    if (item.filter === "unreachable") {
      return { target: "sites", upstreamSiteId: entityId, siteHealth: "unreachable" };
    }
    return { target: "sites", upstreamSiteId: entityId };
  }
  return actionItemNavigationIntent(item);
}
