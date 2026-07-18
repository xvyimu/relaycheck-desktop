export type LineIconName =
  | "dashboard"
  | "channels"
  | "sites"
  | "accounts"
  | "checkins"
  | "balances"
  | "notifications"
  | "scan"
  | "settings"
  | "success"
  | "warning"
  | "danger"
  | "info";

export type TabKey =
  | "dashboard"
  | "channels"
  | "sites"
  | "accounts"
  | "checkins"
  | "balances"
  | "notifications"
  | "scan"
  | "settings";

export type NavItem = {
  key: TabKey;
  label: string;
  icon: LineIconName;
  description: string;
};

export type NavigationIntent = {
  target: TabKey;
  sourceStatus?: string;
  channelKind?: string;
  accountStatus?: string;
  checkinStatus?: string;
  siteHealth?: string;
  siteKind?: string;
  upstreamSiteId?: string;
  unreadOnly?: boolean;
  query?: string;
  accountsView?: "all";
  checkinPreview?: "open";
};
