import { afterEach, expect, it, vi } from "vitest";

afterEach(() => {
  vi.resetModules();
  vi.restoreAllMocks();
  vi.doUnmock("react");
  vi.doUnmock("@/api/client");
});

function mockReactHooks() {
  const states: Array<{ calls: unknown[]; initial: unknown }> = [];

  vi.doMock("react", () => ({
    useCallback: (callback: unknown) => callback,
    useEffect: () => undefined,
    useRef: (initial: unknown) => ({ current: initial }),
    useState: (initial: unknown) => {
      const state = { calls: [] as unknown[], initial };
      states.push(state);
      return [initial, (value: unknown) => state.calls.push(value)];
    },
  }));

  return { states };
}

it("keeps the last successful data when a later refresh fails", async () => {
  const { states } = mockReactHooks();
  const api = vi.fn()
    .mockResolvedValueOnce(["loaded"])
    .mockRejectedValueOnce(new Error("offline"));

  vi.doMock("@/api/client", () => ({ api }));

  const { useApi } = await import("@/hooks/useApi");
  const state = useApi<string[]>("/api/items", []);

  await state.refresh();
  await state.refresh();

  expect(states[0].calls).toEqual([["loaded"]]);
  expect(states[2].calls).toContain("offline");
});
