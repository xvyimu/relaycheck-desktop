export type Theme = "light" | "dark" | "system";

const STORAGE_KEY = "relaycheck-theme";
const DEFAULT_THEME: Theme = "system";

function isDarkScheme(): boolean {
  if (typeof window === "undefined") return false;
  return window.matchMedia("(prefers-color-scheme: dark)").matches;
}

function resolveEffective(theme: Theme): "light" | "dark" {
  if (theme === "system") return isDarkScheme() ? "dark" : "light";
  return theme;
}

export function applyTheme(theme: Theme): void {
  if (typeof document === "undefined") return;
  const effective = resolveEffective(theme);
  const root = document.documentElement;
  if (effective === "dark") {
    root.classList.add("dark");
  } else {
    root.classList.remove("dark");
  }
}

export function getTheme(): Theme {
  if (typeof window === "undefined") return DEFAULT_THEME;
  try {
    const stored = window.localStorage.getItem(STORAGE_KEY);
    if (stored === "light" || stored === "dark" || stored === "system") {
      return stored;
    }
  } catch {
    // localStorage may be unavailable (private mode / disabled)
  }
  return DEFAULT_THEME;
}

export function setTheme(theme: Theme): void {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(STORAGE_KEY, theme);
  } catch {
    // ignore persistence failures
  }
  applyTheme(theme);
}

export function initTheme(): () => void {
  const current = getTheme();
  applyTheme(current);

  if (typeof window === "undefined" || !window.matchMedia) {
    return () => {};
  }

  const mql = window.matchMedia("(prefers-color-scheme: dark)");
  const handler = () => {
    if (getTheme() === "system") {
      applyTheme("system");
    }
  };

  if (typeof mql.addEventListener === "function") {
    mql.addEventListener("change", handler);
    return () => mql.removeEventListener("change", handler);
  }
  // Safari < 14 fallback
  mql.addListener(handler);
  return () => mql.removeListener(handler);
}
