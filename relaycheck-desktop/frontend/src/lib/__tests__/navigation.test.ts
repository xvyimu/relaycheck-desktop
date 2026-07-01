import { describe, it, expect } from "vitest";
import { actionItemNavigationIntent } from "../navigation";

describe("actionItemNavigationIntent", () => {
  it("routes accounts with problem filter", () => {
    const result = actionItemNavigationIntent({ target: "accounts", filter: "problem" });
    expect(result).toEqual({ target: "accounts", accountStatus: "problem" });
  });

  it("routes accounts without problem filter", () => {
    const result = actionItemNavigationIntent({ target: "accounts" });
    expect(result).toEqual({ target: "accounts", accountStatus: "all" });
  });

  it("routes accounts with non-problem filter to all", () => {
    const result = actionItemNavigationIntent({ target: "accounts", filter: "other" });
    expect(result).toEqual({ target: "accounts", accountStatus: "all" });
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

  it("routes balances", () => {
    const result = actionItemNavigationIntent({ target: "balances" });
    expect(result).toEqual({ target: "balances" });
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
