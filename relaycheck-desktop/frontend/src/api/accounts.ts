export type AccountsPageQuery = {
  limit?: number;
  query?: string;
  status?: string;
  upstreamSiteId?: string;
  cursor?: string;
};

export type AccountAction =
  | "open-browser-login"
  | "finish-browser-login"
  | "test-login"
  | "test-api-key"
  | "checkin"
  | "refresh-balance"
  | "clear-session";

export type AccountBulkAction =
  | "bulk-open-browser-login"
  | "bulk-finish-browser-login"
  | "bulk-password-login"
  | "bulk-test-api-keys"
  | "bulk-refresh-balances";

export type AccountCommand = "delete-unsupported-checkins";

function accountBase(id: string): string {
  return `/api/accounts/${encodeURIComponent(id)}`;
}

export function accountActionUrl(id: string, action: AccountAction): string {
  return `${accountBase(id)}/${action}`;
}

export function buildAccountsPageUrl(options: AccountsPageQuery = {}): string {
  const params = new URLSearchParams();
  params.set("limit", String(options.limit ?? 50));
  const query = (options.query || "").trim();
  const status = (options.status || "").trim();
  const upstreamSiteId = (options.upstreamSiteId || "").trim();
  if (query) params.set("query", query);
  if (status && status !== "all") params.set("status", status);
  if (upstreamSiteId && upstreamSiteId !== "all") params.set("upstreamSiteId", upstreamSiteId);
  if (options.cursor) params.set("cursor", options.cursor);
  return `/api/accounts/page?${params.toString()}`;
}

export const accountApi = {
  collection: "/api/accounts",
  summary: "/api/accounts/summary",
  searchIndex: "/api/accounts/search-index",
  searchSites: (query: string, limit = 200) => {
    const params = new URLSearchParams({ query: query.trim(), limit: String(limit) });
    return `/api/accounts/search-sites?${params.toString()}`;
  },
  item: accountBase,
  action: accountActionUrl,
  bulk: (action: AccountBulkAction) => `/api/accounts/${action}`,
  command: (action: AccountCommand) => `/api/accounts/${action}`,
} as const;
