import { api } from "@/api/client";

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

/** 构造账号 action URL；ID 始终 encode。 */
export function accountActionUrl(id: string, action: AccountAction): string {
  return `${accountBase(id)}/${action}`;
}

/** 构造游标分页 URL；status/upstreamSiteId 为 all 时不传参。 */
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

/** GET 账号游标页。 */
function page<T>(options: AccountsPageQuery = {}, init?: RequestInit): Promise<T> {
  return api<T>(buildAccountsPageUrl(options), init);
}

/** POST 单项账号 action。 */
function postAction<T>(id: string, action: AccountAction, body?: unknown): Promise<T> {
  return api<T>(accountActionUrl(id, action), {
    method: "POST",
    body: body === undefined ? undefined : JSON.stringify(body),
  });
}

/** DELETE 单项账号。 */
function remove(id: string): Promise<unknown> {
  return api(accountBase(id), { method: "DELETE" });
}

/** POST 批量账号 action。 */
function postBulk<T>(action: AccountBulkAction, body?: unknown): Promise<T> {
  return api<T>(`/api/accounts/${action}`, {
    method: "POST",
    body: body === undefined ? undefined : JSON.stringify(body),
  });
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
  page,
  postAction,
  remove,
  postBulk,
} as const;
