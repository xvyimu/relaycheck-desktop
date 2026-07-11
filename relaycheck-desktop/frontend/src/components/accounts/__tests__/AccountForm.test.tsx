import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { AccountForm } from "../AccountForm";
import type { UpstreamSite } from "@/types";

const site: UpstreamSite = {
  id: "site-1",
  name: "Relay Hub",
  baseUrl: "https://relay.example",
  kind: "newapi",
  healthStatus: "healthy",
  supportsCheckin: true,
  supportsBalance: true,
  supportsModels: true,
  accountCount: 1,
};

function renderAccountForm(defaultExpanded = false) {
  return renderToStaticMarkup(
    <AccountForm sites={[site]} onDone={() => undefined} defaultExpanded={defaultExpanded} />,
  );
}

describe("AccountForm accessibility", () => {
  it("collapses create form by default (S1)", () => {
    const html = renderAccountForm();

    expect(html).toContain("account-create-collapsed");
    expect(html).toContain('aria-expanded="false"');
    expect(html).toContain("+ 添加账号");
    expect(html).not.toContain('aria-busy=');
  });

  it("exposes busy state and selected mode semantics when expanded", () => {
    const html = renderAccountForm(true);

    expect(html).toContain('aria-busy="false"');
    expect(html).toContain('aria-pressed="true"');
    expect(html).toContain('aria-pressed="false"');
  });

  it("uses semantic input attributes for credentials when expanded", () => {
    const html = renderAccountForm(true);

    expect(html).toContain('type="email"');
    expect(html).toContain('inputMode="email"');
    expect(html).toContain('autoComplete="email"');
    expect(html).toContain('autoComplete="username"');
    expect(html).toContain('autoComplete="current-password"');
    expect(html).toContain('autoComplete="off"');
    expect(html).toContain('autoCapitalize="none"');
    expect(html).toContain('spellCheck="false"');
  });
});
