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
  it("shows the browser-login loop before check-in and balance actions", () => {
    const html = renderToStaticMarkup(
      <AccountCard account={account} onDone={async () => undefined} onOpenDetail={() => undefined} />,
    );

    const open = html.indexOf("网页登录");
    const save = html.indexOf("保存授权");
    const test = html.indexOf("测试登录态");
    const checkin = html.indexOf("执行签到");
    const balance = html.indexOf("刷新 Primary Account 的余额");

    expect(open).toBeGreaterThan(-1);
    expect(save).toBeGreaterThan(open);
    expect(test).toBeGreaterThan(save);
    expect(checkin).toBeGreaterThan(test);
    expect(balance).toBeGreaterThan(checkin);
  });
});
