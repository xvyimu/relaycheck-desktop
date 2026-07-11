import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { AccountsPanel } from "../AccountsPanel";
import type { Account, UpstreamSite } from "@/types";

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

const accounts: Account[] = [
  {
    id: "acc-1",
    upstreamSiteId: "site-a",
    upstreamSiteName: "Alpha Site",
    displayName: "Alpha One",
    authType: "browser_profile",
    loginStatus: "valid",
  },
  {
    id: "acc-2",
    upstreamSiteId: "site-a",
    upstreamSiteName: "Alpha Site",
    displayName: "Alpha Two",
    authType: "browser_profile",
    loginStatus: "valid",
  },
  {
    id: "acc-3",
    upstreamSiteId: "site-b",
    upstreamSiteName: "Beta Site",
    displayName: "Beta One",
    authType: "browser_profile",
    loginStatus: "valid",
  },
];

describe("AccountsPanel site filter shell", () => {
  it("renders site select and collapses create form by default", () => {
    const html = renderToStaticMarkup(
      <AccountsPanel accounts={accounts} sites={sites} onRefresh={async () => undefined} />,
    );

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
        accounts={accounts}
        sites={sites}
        onRefresh={async () => undefined}
        intent={{ target: "accounts", upstreamSiteId: "site-b" }}
      />,
    );

    // Selected option for site-b is present; filter banner names the site
    expect(html).toContain('value="site-b"');
    expect(html).toContain("站点：Beta Site");
    // S3: server-side filter copy when site is selected (hook enabled; SSR has no fetch)
    expect(html).toContain("服务端按上游站点过滤账号列表");
  });
});
