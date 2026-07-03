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

function renderAccountForm() {
  return renderToStaticMarkup(<AccountForm sites={[site]} onDone={() => undefined} />);
}

describe("AccountForm accessibility", () => {
  it("exposes busy state and selected mode semantics", () => {
    const html = renderAccountForm();

    expect(html).toContain('aria-busy="false"');
    expect(html).toContain('aria-pressed="true"');
    expect(html).toContain('aria-pressed="false"');
  });

  it("uses semantic input attributes for credentials", () => {
    const html = renderAccountForm();

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
