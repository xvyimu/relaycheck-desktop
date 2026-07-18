import { afterEach, expect, it, vi } from "vitest";

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
    useState: (initial: unknown) => [initial, vi.fn()],
  }));
}

it("loads a selected site's account page without requesting the legacy full account list", async () => {
  mockReactHooks();
  const api = vi.fn().mockResolvedValue({ items: [], total: 0, nextCursor: "" });
  vi.doMock("@/api/client", () => ({ api }));

  const { useSiteAccounts } = await import("../useSiteAccounts");
  const state = useSiteAccounts("site-a");

  expect(state.enabled).toBe(true);
  expect(api).toHaveBeenCalledWith(
    "/api/accounts/page?limit=200&upstreamSiteId=site-a",
    expect.objectContaining({ signal: expect.any(AbortSignal) }),
  );
  expect(api).not.toHaveBeenCalledWith("/api/accounts");
});

it("keeps the all-sites master-detail state idle", async () => {
  mockReactHooks();
  const api = vi.fn();
  vi.doMock("@/api/client", () => ({ api }));

  const { useSiteAccounts } = await import("../useSiteAccounts");
  const state = useSiteAccounts("all");

  expect(state.enabled).toBe(false);
  expect(state.url).toBeNull();
  expect(api).not.toHaveBeenCalled();
});
