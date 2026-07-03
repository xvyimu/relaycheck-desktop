import { describe, expect, it } from "vitest";

import {
  loginDiscoverySourceLabel,
  normalizeLoginDiscovery,
  parseLoginDiscovery,
} from "../loginDiscovery";

describe("parseLoginDiscovery", () => {
  it("returns null for invalid JSON", () => {
    expect(parseLoginDiscovery("{bad json")).toBeNull();
  });

  it("ignores non-array candidates without throwing", () => {
    expect(parseLoginDiscovery('{"url":"/login","source":"html_link","confidence":0.7,"candidates":"abc"}')).toEqual({
      url: "/login",
      source: "html_link",
      confidence: 0.7,
    });
  });

  it("keeps only non-empty string candidates", () => {
    expect(
      parseLoginDiscovery(
        '{"url":"/login","source":"path_probe","confidence":0.8,"candidates":["/login",42,"","/signin",false]}',
      ),
    ).toEqual({
      url: "/login",
      source: "path_probe",
      confidence: 0.8,
      candidates: ["/login", "/signin"],
    });
  });
});

describe("normalizeLoginDiscovery", () => {
  it("returns null for non-object values", () => {
    expect(normalizeLoginDiscovery("not an object")).toBeNull();
    expect(normalizeLoginDiscovery(null)).toBeNull();
  });

  it("fills safe defaults for partial objects", () => {
    expect(normalizeLoginDiscovery({ url: "/login" })).toEqual({
      url: "/login",
      source: "",
      confidence: 0,
    });
  });
});

describe("loginDiscoverySourceLabel", () => {
  it("maps known discovery sources to stable labels", () => {
    expect(loginDiscoverySourceLabel("manual")).toBe("手动指定");
    expect(loginDiscoverySourceLabel("html_form")).toBe("登录表单");
    expect(loginDiscoverySourceLabel("html_link")).toBe("页面链接");
    expect(loginDiscoverySourceLabel("path_probe")).toBe("路径探测");
    expect(loginDiscoverySourceLabel("spa_fallback")).toBe("SPA 回退");
    expect(loginDiscoverySourceLabel("fallback")).toBe("默认回退");
  });

  it("falls back for unknown or empty source values", () => {
    expect(loginDiscoverySourceLabel("custom")).toBe("custom");
    expect(loginDiscoverySourceLabel()).toBe("未知");
  });
});
