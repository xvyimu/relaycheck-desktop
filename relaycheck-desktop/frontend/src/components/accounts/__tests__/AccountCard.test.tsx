import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { AccountCard } from "../AccountCard";
import type { Account } from "@/types";

const account: Account = {
  id: "account-1",
  upstreamSiteId: "site-1",
  upstreamSiteName: "Relay Hub",
  upstreamSiteBaseUrl: "https://relay.example",
  upstreamSiteLoginUrl: "https://relay.example/login",
  upstreamSiteKind: "newapi",
  displayName: "Primary Account",
  email: "user@example.com",
  authType: "browser_profile",
  loginStatus: "manual_required",
};

describe("AccountCard action chain", () => {
  it("keeps only browser login, check-in, and detail as primary actions", () => {
    const html = renderToStaticMarkup(
      <AccountCard account={account} onDone={async () => undefined} onOpenDetail={() => undefined} />,
    );

    const primaryStart = html.indexOf('account-action-group primary');
    const morePanel = html.indexOf("account-more-panel");
    const primaryHtml = morePanel === -1 ? html.slice(primaryStart) : html.slice(primaryStart, morePanel);

    expect(primaryHtml.indexOf("网页登录")).toBeGreaterThan(-1);
    expect(primaryHtml.indexOf("签到")).toBeGreaterThan(-1);
    expect(primaryHtml.indexOf(">详情<")).toBeGreaterThan(-1);
    expect(primaryHtml.indexOf("保存授权")).toBe(-1);
    expect(primaryHtml.indexOf("测试登录态")).toBe(-1);
    expect(primaryHtml.indexOf("刷新余额")).toBe(-1);
    expect(html.indexOf("更多")).toBeGreaterThan(-1);
  });
});
