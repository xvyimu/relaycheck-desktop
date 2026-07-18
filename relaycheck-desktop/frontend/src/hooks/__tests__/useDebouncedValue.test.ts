import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useDebouncedValue } from "../useDebouncedValue";

type StateSetter = (value: unknown | ((previous: unknown) => unknown)) => void;

const stateSlots: unknown[] = [];
let stateIndex = 0;

vi.mock("react", () => ({
  useEffect: (effect: () => void | (() => void)) => effect(),
  useState: (initial: unknown) => {
    const index = stateIndex++;
    if (!(index in stateSlots)) stateSlots[index] = initial;
    const setState: StateSetter = (value) => {
      stateSlots[index] = typeof value === "function" ? value(stateSlots[index]) : value;
    };
    return [stateSlots[index], setState];
  },
}));

beforeEach(() => {
  vi.useFakeTimers();
  stateSlots.length = 0;
  stateIndex = 0;
});

afterEach(() => {
  vi.useRealTimers();
});

function HookHarness({ value, enabled = true }: { value: string; enabled?: boolean }) {
  return useDebouncedValue(value, 250, enabled);
}

function renderHookValue(value: string, enabled = true) {
  stateIndex = 0;
  return HookHarness({ value, enabled });
}

describe("useDebouncedValue", () => {
  it("publishes the latest value only after 250ms", async () => {
    expect(renderHookValue("a")).toBe("a");
    expect(renderHookValue("alpha")).toBe("a");

    await vi.advanceTimersByTimeAsync(249);
    expect(renderHookValue("alpha")).toBe("a");
    await vi.advanceTimersByTimeAsync(1);
    expect(renderHookValue("alpha")).toBe("alpha");
  });

  it("pauses updates while IME composition is active", async () => {
    expect(renderHookValue("初")).toBe("初");
    expect(renderHookValue("初始", false)).toBe("初");
    await vi.advanceTimersByTimeAsync(500);
    expect(renderHookValue("初始", false)).toBe("初");

    expect(renderHookValue("初始", true)).toBe("初");
    await vi.advanceTimersByTimeAsync(250);
    expect(renderHookValue("初始", true)).toBe("初始");
  });
});
