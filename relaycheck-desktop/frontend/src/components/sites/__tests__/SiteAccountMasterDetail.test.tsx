import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

import { SiteAccountMasterDetail } from "@/components/sites/SiteAccountMasterDetail";
import type { UpstreamSite } from "@/types";

vi.mock("@/hooks/useSiteAccounts", () => ({
  useSiteAccounts: () => ({
    data: [],
    loading: false,
    error: "",
    enabled: false,
    refresh: vi.fn(),
  }),
}));

vi.mock("@/components/accounts/AccountCard", () => ({
  AccountCard: () => null,
}));

vi.mock("@/components/accounts/AccountDetailContent", () => ({
  AccountDetailContent: () => null,
}));

const sites: UpstreamSite[] = [
  {
    id: "site-1",
    name: "Alpha",
    kind: "newapi",
    baseUrl: "https://example.test",
    healthStatus: "healthy",
    accountCount: 2,
    supportsCheckin: true,
    supportsBalance: true,
    supportsModels: false,
    supportsPricing: false,
  } as UpstreamSite,
];

describe("SiteAccountMasterDetail", () => {
  it("renders dual-pane shell with site list", () => {
    const html = renderToStaticMarkup(<SiteAccountMasterDetail sites={sites} onRefresh={async () => {}} />);
    expect(html).toContain("site-account-master-detail");
    expect(html).toContain("Alpha");
    expect(html).toContain("master-detail-left");
    expect(html).toContain("master-detail-right");
  });

  it("accepts navigation intent site id", () => {
    const html = renderToStaticMarkup(
      <SiteAccountMasterDetail
        sites={sites}
        onRefresh={async () => {}}
        intent={{ target: "sites", upstreamSiteId: "site-1" } as never}
      />,
    );
    expect(html).toContain("当前站点");
    expect(html).toContain("Alpha");
  });

  it("renders delete site control when a site is selected", () => {
    const html = renderToStaticMarkup(
      <SiteAccountMasterDetail
        sites={sites}
        onRefresh={async () => {}}
        intent={{ target: "sites", upstreamSiteId: "site-1" } as never}
      />,
    );
    expect(html).toContain("删除站点");
  });
});
