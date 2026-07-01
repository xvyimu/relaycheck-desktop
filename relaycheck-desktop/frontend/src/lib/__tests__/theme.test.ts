import { describe, it, expect, vi, beforeEach } from "vitest";
import { applyTheme, getTheme, setTheme, initTheme, type Theme } from "../theme";

function createLocalStorageMock() {
  const store: Record<string, string> = {};
  return {
    getItem: vi.fn((key: string) => store[key] ?? null),
    setItem: vi.fn((key: string, value: string) => {
      store[key] = value;
    }),
    removeItem: vi.fn((key: string) => {
      delete store[key];
    }),
    clear: vi.fn(() => {
      for (const key of Object.keys(store)) delete store[key];
    }),
    get length() {
      return Object.keys(store).length;
    },
    key: vi.fn((_: number) => null),
  };
}

function createDocumentMock() {
  const classList = new Set<string>();
  return {
    documentElement: {
      classList: {
        add: vi.fn((cls: string) => classList.add(cls)),
        remove: vi.fn((cls: string) => classList.delete(cls)),
        contains: (cls: string) => classList.has(cls),
      },
    },
    _classList: classList,
  };
}

describe("applyTheme", () => {
  let docMock: ReturnType<typeof createDocumentMock>;

  beforeEach(() => {
    docMock = createDocumentMock();
    vi.stubGlobal("document", docMock as unknown as Document);
  });

  it("adds dark class for dark theme", () => {
    applyTheme("dark");
    expect(docMock.documentElement.classList.add).toHaveBeenCalledWith("dark");
    expect(docMock._classList.has("dark")).toBe(true);
  });

  it("removes dark class for light theme", () => {
    applyTheme("light");
    expect(docMock.documentElement.classList.remove).toHaveBeenCalledWith("dark");
    expect(docMock._classList.has("dark")).toBe(false);
  });

  it("adds dark class for system theme when system prefers dark", () => {
    vi.stubGlobal("window", {
      matchMedia: vi.fn().mockReturnValue({ matches: true }),
    });
    applyTheme("system");
    expect(docMock.documentElement.classList.add).toHaveBeenCalledWith("dark");
  });

  it("removes dark class for system theme when system prefers light", () => {
    vi.stubGlobal("window", {
      matchMedia: vi.fn().mockReturnValue({ matches: false }),
    });
    applyTheme("system");
    expect(docMock.documentElement.classList.remove).toHaveBeenCalledWith("dark");
  });
});

describe("getTheme", () => {
  let storageMock: ReturnType<typeof createLocalStorageMock>;

  beforeEach(() => {
    storageMock = createLocalStorageMock();
    vi.stubGlobal("window", { localStorage: storageMock });
  });

  it("returns stored light theme", () => {
    storageMock.setItem("relaycheck-theme", "light");
    expect(getTheme()).toBe("light");
  });

  it("returns stored dark theme", () => {
    storageMock.setItem("relaycheck-theme", "dark");
    expect(getTheme()).toBe("dark");
  });

  it("returns stored system theme", () => {
    storageMock.setItem("relaycheck-theme", "system");
    expect(getTheme()).toBe("system");
  });

  it("returns default system theme when nothing stored", () => {
    expect(getTheme()).toBe("system");
  });

  it("returns default when invalid value stored", () => {
    storageMock.setItem("relaycheck-theme", "invalid");
    expect(getTheme()).toBe("system");
  });
});

describe("setTheme", () => {
  let storageMock: ReturnType<typeof createLocalStorageMock>;
  let docMock: ReturnType<typeof createDocumentMock>;

  beforeEach(() => {
    storageMock = createLocalStorageMock();
    docMock = createDocumentMock();
    vi.stubGlobal("window", { localStorage: storageMock, matchMedia: vi.fn().mockReturnValue({ matches: false }) });
    vi.stubGlobal("document", docMock as unknown as Document);
  });

  it("persists theme to localStorage", () => {
    setTheme("dark");
    expect(storageMock.setItem).toHaveBeenCalledWith("relaycheck-theme", "dark");
  });

  it("applies the theme immediately", () => {
    setTheme("dark");
    expect(docMock.documentElement.classList.add).toHaveBeenCalledWith("dark");
  });

  it("applies light theme correctly", () => {
    setTheme("light");
    expect(docMock.documentElement.classList.remove).toHaveBeenCalledWith("dark");
  });
});

describe("initTheme", () => {
  let storageMock: ReturnType<typeof createLocalStorageMock>;
  let docMock: ReturnType<typeof createDocumentMock>;

  beforeEach(() => {
    storageMock = createLocalStorageMock();
    docMock = createDocumentMock();
    const listeners: Array<() => void> = [];
    vi.stubGlobal("window", {
      localStorage: storageMock,
      matchMedia: vi.fn().mockReturnValue({
        matches: false,
        addEventListener: vi.fn((_: string, handler: () => void) => listeners.push(handler)),
        removeEventListener: vi.fn((_: string, handler: () => void) => {
          const idx = listeners.indexOf(handler);
          if (idx >= 0) listeners.splice(idx, 1);
        }),
      }),
    });
    vi.stubGlobal("document", docMock as unknown as Document);
  });

  it("applies the stored theme on init", () => {
    storageMock.setItem("relaycheck-theme", "dark");
    initTheme();
    expect(docMock.documentElement.classList.add).toHaveBeenCalledWith("dark");
  });

  it("returns a cleanup function", () => {
    const cleanup = initTheme();
    expect(typeof cleanup).toBe("function");
    cleanup(); // should not throw
  });
});
