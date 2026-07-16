import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

import { BulkReloginWizard, bulkReloginCandidates } from "@/components/accounts/BulkReloginWizard";
import type { Account } from "@/types";

vi.mock("@/api/client", () => ({
  api: vi.fn(),
}));

function makeAccount(partial: Partial<Account> & Pick<Account, "id" | "displayName">): Account {
  return {
    id: partial.id,
    displayName: partial.displayName,
    authType: partial.authType || "password",
    loginStatus: partial.loginStatus || "valid",
    upstreamSiteId: partial.upstreamSiteId || "site-1",
    upstreamSiteName: partial.upstreamSiteName || "Site",
    lastCheckinStatus: partial.lastCheckinStatus,
    email: partial.email,
    username: partial.username,
  } as Account;
}

describe("BulkReloginWizard", () => {
  it("filters problem accounts only", () => {
    const accounts = [
      makeAccount({ id: "a1", displayName: "ok", loginStatus: "valid" }),
      makeAccount({ id: "a2", displayName: "bad", loginStatus: "expired" }),
    ];
    expect(bulkReloginCandidates(accounts).map((a) => a.id)).toEqual(["a2"]);
  });

  it("renders nothing when no problem accounts", () => {
    const html = renderToStaticMarkup(
      <BulkReloginWizard accounts={[makeAccount({ id: "a1", displayName: "ok" })]} onDone={async () => {}} />,
    );
    expect(html).toBe("");
  });

  it("renders wizard shell for problem accounts", () => {
    const html = renderToStaticMarkup(
      <BulkReloginWizard
        accounts={[makeAccount({ id: "a2", displayName: "bad", loginStatus: "expired" })]}
        onDone={async () => {}}
      />,
    );
    expect(html).toContain("bulk-relogin-wizard");
    expect(html).toContain("批量会话重登");
    expect(html).not.toContain("关闭 2FA");
    expect(html).not.toContain("自动登录");
  });
});
