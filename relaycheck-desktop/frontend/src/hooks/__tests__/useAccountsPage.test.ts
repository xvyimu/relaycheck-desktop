import { afterEach, describe, expect, it, vi } from "vitest";

import { buildAccountsPageUrl } from "../useAccountsPage";

afterEach(() => {
  vi.resetModules();
  vi.restoreAllMocks();
  vi.doUnmock("react");
  vi.doUnmock("@/api/client");
});

function mockReactHooks() {
  vi.doMock("react", () => ({
    useCallback: (callback: unknown) => callback,
    useEffect: (effect: () => void | (() => void)) => {
      effect();
    },
    useRef: (initial: unknown) => ({ current: initial }),
    useState: (initial: unknown | (() => unknown)) => {
      const value = typeof initial === "function" ? initial() : initial;
      return [value, vi.fn()];
    },
  }));
}

describe("buildAccountsPageUrl", () => {
  it("always includes limit and omits empty filters", () => {
    expect(buildAccountsPageUrl()).toBe("/api/accounts/page?limit=50");
    expect(buildAccountsPageUrl({ limit: 20, query: "  ", status: "all", upstreamSiteId: "all" })).toBe(
      "/api/accounts/page?limit=20",
    );
  });

  it("serializes query/status/site/cursor for server-side pagination", () => {
    expect(
      buildAccountsPageUrl({
        limit: 2,
        query: "alpha user",
        status: "problem",
        upstreamSiteId: "site-a",
        cursor: "cur-1",
      }),
    ).toBe("/api/accounts/page?limit=2&query=alpha+user&status=problem&upstreamSiteId=site-a&cursor=cur-1");
  });

  it("trims query and drops blank cursor", () => {
    expect(buildAccountsPageUrl({ query: "  beta  ", cursor: undefined })).toBe(
      "/api/accounts/page?limit=50&query=beta",
    );
  });

  it("keeps custom limit when filters empty", () => {
    expect(buildAccountsPageUrl({ limit: 200 })).toBe("/api/accounts/page?limit=200");
  });
});

describe("useAccountsPage", () => {
  it("loads the first cursor page on mount", async () => {
    mockReactHooks();
    const api = vi.fn().mockResolvedValue({ items: [], total: 0, accountTotal: 0, problemTotal: 0 });
    vi.doMock("@/api/client", () => ({ api }));

    const { useAccountsPage } = await import("../useAccountsPage");
    useAccountsPage({ limit: 20, query: "alpha", status: "problem", upstreamSiteId: "site-a" });

    expect(api).toHaveBeenCalledWith(
      "/api/accounts/page?limit=20&query=alpha&status=problem&upstreamSiteId=site-a",
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    );
  });

  it("does not request a page while disabled", async () => {
    mockReactHooks();
    const api = vi.fn();
    vi.doMock("@/api/client", () => ({ api }));

    const { useAccountsPage } = await import("../useAccountsPage");
    useAccountsPage({ enabled: false });

    expect(api).not.toHaveBeenCalled();
  });
});
