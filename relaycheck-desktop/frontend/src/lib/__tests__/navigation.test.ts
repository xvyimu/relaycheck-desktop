import { describe, it, expect } from "vitest";
import { actionItemNavigationIntent, actionSampleNavigationIntent, siteAccountsNavigationIntent } from "../navigation";

describe("actionItemNavigationIntent", () => {
  it("routes accounts (problem) into the merged sites 全部账号 subview", () => {
    const result = actionItemNavigationIntent({ target: "accounts", filter: "problem" });
    expect(result).toEqual({ target: "sites", accountsView: "all", accountStatus: "problem" });
  });

  it("routes accounts without problem filter into the merged 全部账号 subview", () => {
    const result = actionItemNavigationIntent({ target: "accounts" });
    expect(result).toEqual({ target: "sites", accountsView: "all", accountStatus: "all" });
  });

  it("routes accounts with non-problem filter to the merged 全部账号 subview", () => {
    const result = actionItemNavigationIntent({ target: "accounts", filter: "other" });
    expect(result).toEqual({ target: "sites", accountsView: "all", accountStatus: "all" });
  });

  it("routes checkins with problem filter", () => {
    const result = actionItemNavigationIntent({ target: "checkins", filter: "problem" });
    expect(result).toEqual({ target: "checkins", checkinStatus: "problem" });
  });

  it("routes checkins without problem filter", () => {
    const result = actionItemNavigationIntent({ target: "checkins" });
    expect(result).toEqual({ target: "checkins", checkinStatus: "all" });
  });

  it("routes channels with health filter", () => {
    const result = actionItemNavigationIntent({ target: "channels", filter: "health" });
    expect(result).toEqual({ target: "channels", siteHealth: "risk" });
  });

  it("routes channels with missing filter", () => {
    const result = actionItemNavigationIntent({ target: "channels", filter: "missing" });
    expect(result).toEqual({ target: "channels", sourceStatus: "missing" });
  });

  it("routes channels with unknown filter", () => {
    const result = actionItemNavigationIntent({ target: "channels", filter: "unknown" });
    expect(result).toEqual({ target: "channels", channelKind: "unknown", sourceStatus: "not_archived" });
  });

  it("routes channels with no filter", () => {
    const result = actionItemNavigationIntent({ target: "channels" });
    expect(result).toEqual({ target: "channels" });
  });

  it("routes channels with unrecognized filter (no extra params)", () => {
    const result = actionItemNavigationIntent({ target: "channels", filter: "other" });
    expect(result).toEqual({ target: "channels" });
  });

  it("routes balances to the merged 全部账号 subview (no dead balances tab)", () => {
    const result = actionItemNavigationIntent({ target: "balances" });
    expect(result).toEqual({ target: "sites", accountsView: "all", query: "余额" });
  });

  it("routes sites with unreachable filter", () => {
    const result = actionItemNavigationIntent({ target: "sites", filter: "unreachable" });
    expect(result).toEqual({ target: "sites", siteHealth: "unreachable" });
  });

  it("routes sites without unreachable filter to all", () => {
    const result = actionItemNavigationIntent({ target: "sites" });
    expect(result).toEqual({ target: "sites", siteHealth: "all" });
  });

  it("routes sites with other filter to all", () => {
    const result = actionItemNavigationIntent({ target: "sites", filter: "healthy" });
    expect(result).toEqual({ target: "sites", siteHealth: "all" });
  });

  it("routes notifications with unread filter", () => {
    const result = actionItemNavigationIntent({ target: "notifications", filter: "unread" });
    expect(result).toEqual({ target: "notifications", unreadOnly: true });
  });

  it("routes notifications without unread filter", () => {
    const result = actionItemNavigationIntent({ target: "notifications" });
    expect(result).toEqual({ target: "notifications", unreadOnly: false });
  });

  it("routes scan", () => {
    const result = actionItemNavigationIntent({ target: "scan" });
    expect(result).toEqual({ target: "scan" });
  });

  it("routes settings", () => {
    const result = actionItemNavigationIntent({ target: "settings" });
    expect(result).toEqual({ target: "settings" });
  });

  it("routes dashboard", () => {
    const result = actionItemNavigationIntent({ target: "dashboard" });
    expect(result).toEqual({ target: "dashboard" });
  });
});

describe("siteAccountsNavigationIntent", () => {
  it("targets sites master-detail with the given upstream site id (S7.3)", () => {
    const result = siteAccountsNavigationIntent("site-abc");
    expect(result).toEqual({ target: "sites", upstreamSiteId: "site-abc" });
  });

  it("preserves empty string site id (caller may normalize)", () => {
    const result = siteAccountsNavigationIntent("");
    expect(result).toEqual({ target: "sites", upstreamSiteId: "" });
  });
});

describe("actionSampleNavigationIntent", () => {
  it("deep-links site samples under unreachable-sites to sites master-detail", () => {
    const result = actionSampleNavigationIntent(
      { target: "sites", filter: "unreachable" },
      { entityType: "site", entityId: "site-down" },
    );
    expect(result).toEqual({
      target: "sites",
      siteHealth: "unreachable",
      upstreamSiteId: "site-down",
    });
  });

  it("deep-links channel-health site samples to sites without forcing unreachable filter", () => {
    const result = actionSampleNavigationIntent(
      { target: "channels", filter: "health" },
      { entityType: "site", entityId: "site-health-risk" },
    );
    expect(result).toEqual({ target: "sites", upstreamSiteId: "site-health-risk" });
  });

  it("falls back to parent intent when sample has no entity", () => {
    const result = actionSampleNavigationIntent(
      { target: "accounts", filter: "problem" },
      { entityType: "", entityId: "" },
    );
    expect(result).toEqual({ target: "sites", accountsView: "all", accountStatus: "problem" });
  });

  it("drops accountsView when a site sample deep-links from an accounts item", () => {
    const result = actionSampleNavigationIntent(
      { target: "accounts", filter: "problem" },
      { entityType: "site", entityId: "site-x" },
    );
    // Site sample lands on the site-centric master-detail, not the 全部账号 subview.
    expect(result).toEqual({ target: "sites", accountStatus: "problem", upstreamSiteId: "site-x" });
  });
});
