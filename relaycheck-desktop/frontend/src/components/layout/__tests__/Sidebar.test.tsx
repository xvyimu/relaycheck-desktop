import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { NAV_GROUPS, Sidebar, TABS } from "../Sidebar";

describe("Sidebar nav groups α-D / S5", () => {
  it("renders three section labels and all flat TABS keys", () => {
    const html = renderToStaticMarkup(
      <Sidebar activeTab="dashboard" onTabChange={() => undefined} />,
    );

    expect(html).toContain('class="sidebar"');
    expect(html).toContain("sidebar-section-label");
    expect(html).toContain("sidebar-nav-group");
    expect(html).toContain("运营");
    expect(html).toContain("资产");
    expect(html).toContain("工具");

    for (const item of TABS) {
      expect(html).toContain(item.label);
    }

    const groupKeys = NAV_GROUPS.flatMap((g) => g.items.map((i) => i.key));
    expect(new Set(groupKeys).size).toBe(TABS.length);
    for (const tab of TABS) {
      expect(groupKeys).toContain(tab.key);
    }
  });

  it("marks the active tab with aria-current", () => {
    const html = renderToStaticMarkup(
      <Sidebar activeTab="accounts" onTabChange={() => undefined} />,
    );
    expect(html).toContain('aria-current="page"');
    expect(html).toContain("账号");
  });

  it("uses live sidebar class (S6 bridge target, not sidebar-v4)", () => {
    const html = renderToStaticMarkup(
      <Sidebar activeTab="dashboard" onTabChange={() => undefined} />,
    );
    expect(html).toContain('class="sidebar"');
    expect(html).not.toContain("sidebar-v4");
  });
});
