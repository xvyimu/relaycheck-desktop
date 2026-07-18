import { afterEach, expect, it, vi } from "vitest";

afterEach(() => {
  vi.resetModules();
  vi.restoreAllMocks();
  vi.doUnmock("react");
  vi.doUnmock("@/api/client");
  vi.doUnmock("@/hooks/useDebouncedValue");
});

function mockHookRuntime() {
  const states: Array<{ initial: unknown; calls: unknown[] }> = [];
  const cleanups: Array<() => void> = [];

  vi.doMock("react", () => ({
    useCallback: (callback: unknown) => callback,
    useEffect: (effect: () => void | (() => void)) => {
      const cleanup = effect();
      if (cleanup) cleanups.push(cleanup);
    },
    useRef: (initial: unknown) => ({ current: initial }),
    useState: (initial: unknown | (() => unknown)) => {
      const value = typeof initial === "function" ? (initial as () => unknown)() : initial;
      const state = { initial: value, calls: [] as unknown[] };
      states.push(state);
      return [value, (next: unknown) => state.calls.push(next)];
    },
  }));
  vi.doMock("@/hooks/useDebouncedValue", () => ({
    useDebouncedValue: (value: unknown) => value,
  }));

  return { states, cleanups };
}

it("loads matching sites and publishes the normalized searched query", async () => {
  const { states, cleanups } = mockHookRuntime();
  const result = {
    items: [{ upstreamSiteId: "site-1", upstreamSiteName: "Alpha", upstreamSiteBaseUrl: "https://alpha.test" }],
    truncated: true,
  };
  const api = vi.fn().mockResolvedValue(result);
  vi.doMock("@/api/client", () => ({ api }));

  const { useAccountSiteSearch } = await import("../useAccountSiteSearch");
  useAccountSiteSearch("  ALPHA  ", true, 25);

  await vi.waitFor(() => expect(api).toHaveBeenCalledTimes(1));
  expect(api).toHaveBeenCalledWith(
    "/api/accounts/search-sites?query=ALPHA&limit=25",
    expect.objectContaining({ signal: expect.any(AbortSignal) }),
  );
  await vi.waitFor(() => expect(states[0].calls).toContain(result));
  expect(states[1].calls).toContain("alpha");
  expect(states[2].calls).toEqual([true, false]);
  expect(states[3].calls).toContain("");

  cleanups.forEach((cleanup) => cleanup());
});

it("stays idle for an empty or disabled query", async () => {
  const { states } = mockHookRuntime();
  const api = vi.fn();
  vi.doMock("@/api/client", () => ({ api }));

  const { useAccountSiteSearch } = await import("../useAccountSiteSearch");
  useAccountSiteSearch("   ", false);

  expect(api).not.toHaveBeenCalled();
  expect(states[0].calls).toEqual([{ items: [], truncated: false }]);
  expect(states[1].calls).toEqual([""]);
  expect(states[2].calls).toEqual([false]);
  expect(states[3].calls).toEqual([""]);
});

it("returns a stable public error when site search fails", async () => {
  const { states } = mockHookRuntime();
  const api = vi.fn().mockRejectedValue("offline");
  vi.doMock("@/api/client", () => ({ api }));

  const { useAccountSiteSearch } = await import("../useAccountSiteSearch");
  useAccountSiteSearch("Beta");

  await vi.waitFor(() => expect(states[3].calls).toContain("搜索账号关联站点失败"));
  expect(states[0].calls).toContainEqual({ items: [], truncated: false });
  expect(states[1].calls).toContain("beta");
  expect(states[2].calls).toEqual([true, false]);
});
