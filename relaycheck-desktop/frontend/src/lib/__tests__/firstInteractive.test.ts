import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  markFirstInteractive,
  readFirstInteractive,
  resetFirstInteractiveForTest,
} from "../firstInteractive";

// vitest node 环境无 window/localStorage；提供最小 stub 验证存取闭环。
function installWindowStub() {
  const store = new Map<string, string>();
  const localStorage = {
    getItem: (k: string) => (store.has(k) ? store.get(k)! : null),
    setItem: (k: string, v: string) => void store.set(k, v),
    removeItem: (k: string) => void store.delete(k),
    clear: () => store.clear(),
  };
  vi.stubGlobal("window", { localStorage } as unknown as Window & typeof globalThis);
  return store;
}

describe("firstInteractive", () => {
  beforeEach(() => {
    resetFirstInteractiveForTest();
    vi.spyOn(console, "info").mockImplementation(() => {});
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("只记一次并写入 localStorage", () => {
    installWindowStub();
    const first = markFirstInteractive();
    expect(first).not.toBeNull();
    expect(markFirstInteractive()).toBeNull();

    const stored = readFirstInteractive();
    expect(stored).not.toBeNull();
    expect(stored?.ms).toBe(first);
    expect(stored?.at).toBeTruthy();
  });

  it("无 window 环境安全降级", () => {
    // 不装 stub：mark 仍返回数值（performance 存在），read 返回 null。
    const ms = markFirstInteractive();
    expect(typeof ms).toBe("number");
    expect(readFirstInteractive()).toBeNull();
  });
});
