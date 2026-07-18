import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

type Effect = () => void | (() => void);

class FakeElement {
  isConnected = true;
  offsetParent: object | null = {};
  tabIndex = 0;
  focused = false;
  private readonly attributes = new Set<string>();
  private nodes: FakeElement[] = [];

  constructor(readonly id: string) {}

  focus() {
    this.focused = true;
    fakeDocument.activeElement = this;
  }

  hasAttribute(name: string) {
    return this.attributes.has(name);
  }

  querySelectorAll() {
    return this.nodes;
  }

  querySelector(selector: string) {
    return this.nodes.find((node) => `#${node.id}` === selector) ?? null;
  }

  setFocusable(...nodes: FakeElement[]) {
    this.nodes = nodes;
  }
}

const fakeDocument = {
  activeElement: null as FakeElement | null,
  body: { style: { overflow: "auto" } },
};

let effects: Effect[] = [];
let keyHandler: ((event: { key: string; shiftKey?: boolean; preventDefault: () => void }) => void) | undefined;
const raf = vi.fn((callback: FrameRequestCallback) => {
  callback(0);
  return 1;
});

async function renderDialog(
  overrides: Partial<{ onClose: () => void; closeOnBackdrop: boolean; initialFocusSelector: string }> = {},
) {
  effects = [];
  keyHandler = undefined;
  vi.resetModules();
  vi.doMock("react", () => ({
    useEffect: (effect: Effect) => effects.push(effect),
    useId: () => "generated-title",
    useRef: <T>(initial: T) => ({ current: initial }),
  }));
  const { DialogShell } = await import("@/components/ui/dialog-shell");
  const onClose = overrides.onClose ?? vi.fn();
  const tree = DialogShell({ open: true, onClose, children: null, ...overrides });
  const panel = new FakeElement("panel");
  const root = tree as unknown as {
    props: {
      children: { props: { ref: { current: FakeElement | null } } };
      onMouseDown: (event: unknown) => void;
    };
  };
  root.props.children.props.ref.current = panel;
  const activate = () => effects.map((effect) => effect()).filter((cleanup): cleanup is () => void => Boolean(cleanup));
  return { activate, onClose, panel, root };
}

beforeEach(() => {
  vi.stubGlobal("document", fakeDocument);
  vi.stubGlobal("HTMLElement", FakeElement);
  vi.stubGlobal("window", {
    requestAnimationFrame: raf,
    cancelAnimationFrame: vi.fn(),
    addEventListener: vi.fn((type: string, listener: typeof keyHandler) => {
      if (type === "keydown") keyHandler = listener;
    }),
    removeEventListener: vi.fn(),
  });
  fakeDocument.activeElement = null;
  fakeDocument.body.style.overflow = "auto";
  raf.mockClear();
});

afterEach(() => {
  vi.doUnmock("react");
  vi.unstubAllGlobals();
});

describe("DialogShell interactions", () => {
  it("locks scrolling, focuses the requested control, and restores the prior focus on close", async () => {
    const opener = new FakeElement("opener");
    fakeDocument.activeElement = opener;
    const { activate, panel } = await renderDialog({ initialFocusSelector: "#save" });
    const save = new FakeElement("save");
    panel.setFocusable(save);
    const cleanups = activate();
    expect(fakeDocument.body.style.overflow).toBe("hidden");
    expect(save.focused).toBe(true);
    expect(raf).toHaveBeenCalled();
    cleanups.forEach((cleanup) => cleanup());
    expect(fakeDocument.body.style.overflow).toBe("auto");
    expect(opener.focused).toBe(true);
  });

  it("closes on Escape and only when the backdrop itself is pressed", async () => {
    const { activate, onClose, root } = await renderDialog();
    activate();
    const prevented = vi.fn();
    keyHandler?.({ key: "Escape", preventDefault: prevented });
    expect(prevented).toHaveBeenCalledOnce();
    expect(onClose).toHaveBeenCalledOnce();

    const backdrop = {};
    root.props.onMouseDown({ target: backdrop, currentTarget: backdrop });
    root.props.onMouseDown({ target: {}, currentTarget: backdrop });
    expect(onClose).toHaveBeenCalledTimes(2);
  });

  it("cycles Tab from the last focusable control to the first", async () => {
    const { activate, panel } = await renderDialog();
    const first = new FakeElement("first");
    const last = new FakeElement("last");
    panel.setFocusable(first, last);
    activate();
    last.focus();
    const prevented = vi.fn();

    keyHandler?.({ key: "Tab", preventDefault: prevented });
    expect(prevented).toHaveBeenCalledOnce();
    expect(first.focused).toBe(true);
  });
});
