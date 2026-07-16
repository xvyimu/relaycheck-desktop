import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

import { AccountsPanel } from "../AccountsPanel";
import type { UpstreamSite } from "@/types";

// Mock the pagination hook since SSR can't do real HTTP
vi.mock("@/hooks/useAccountsPage", () => ({
  useAccountsPage: () => ({
    page: { items: [], total: 0, accountTotal: 0, problemTotal: 0 },
    loading: false,
    loaded: true,
    error: "",
    goNext: () => undefined,
    goPrev: () => undefined,
    hasNext: false,
    hasPrev: false,
    refresh: async () => undefined,
    reset: () => undefined,
  }),
}));

const sites: UpstreamSite[] = [
  {
    id: "site-a",
    name: "Alpha Site",
    baseUrl: "https://alpha.example",
    kind: "newapi",
    healthStatus: "healthy",
    supportsCheckin: true,
    supportsBalance: true,
    supportsModels: true,
    accountCount: 2,
  },
  {
    id: "site-b",
    name: "Beta Site",
    baseUrl: "https://beta.example",
    kind: "newapi",
    healthStatus: "healthy",
    supportsCheckin: true,
    supportsBalance: true,
    supportsModels: true,
    accountCount: 1,
  },
];

describe("AccountsPanel site filter shell", () => {
  it("renders site select and collapses create form by default", () => {
    const html = renderToStaticMarkup(<AccountsPanel sites={sites} onRefresh={async () => undefined} />);

    expect(html).toContain("按上游站点筛选账号");
    expect(html).toContain("全部站点");
    expect(html).toContain("Alpha Site");
    expect(html).toContain("Beta Site");
    expect(html).toContain("+ 添加账号");
    expect(html).toContain("展开洞察");
    expect(html).toContain("批量测试 Key");
    expect(html).toContain("account-filter-grid");
  });

  it("applies upstreamSiteId from navigation intent", () => {
    const html = renderToStaticMarkup(
      <AccountsPanel
        sites={sites}
        onRefresh={async () => undefined}
        intent={{ target: "sites", upstreamSiteId: "site-b", accountsView: "all" }}
      />,
    );

    // Selected option for site-b is present; filter banner names the site
    expect(html).toContain('value="site-b"');
    expect(html).toContain("站点：Beta Site");
    expect(html).toContain("服务端分页查询");
  });
});
