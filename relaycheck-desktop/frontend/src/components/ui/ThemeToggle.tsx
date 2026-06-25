import { useEffect, useState } from "react";
import { getTheme, setTheme, type Theme } from "@/lib/theme";

const ORDER: Theme[] = ["system", "light", "dark"];

const LABELS: Record<Theme, string> = {
  system: "跟随系统",
  light: "浅色",
  dark: "深色",
};

function SystemIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <rect x="2" y="3" width="20" height="14" rx="2" />
      <path d="M8 21h8M12 17v4" />
    </svg>
  );
}

function SunIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="12" r="4" />
      <path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41" />
    </svg>
  );
}

function MoonIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
    </svg>
  );
}

function ThemeIcon({ theme }: { theme: Theme }) {
  if (theme === "light") return <SunIcon />;
  if (theme === "dark") return <MoonIcon />;
  return <SystemIcon />;
}

export function ThemeToggle() {
  const [theme, setThemeState] = useState<Theme>(() => getTheme());

  useEffect(() => {
    setThemeState(getTheme());
  }, []);

  function cycle() {
    const currentIndex = ORDER.indexOf(theme);
    const next = ORDER[(currentIndex + 1) % ORDER.length];
    setTheme(next);
    setThemeState(next);
  }

  return (
    <button
      type="button"
      className="theme-toggle"
      onClick={cycle}
      aria-label={`主题：${LABELS[theme]}，点击切换`}
      title={`主题：${LABELS[theme]}`}
    >
      <span className="theme-toggle-icon" aria-hidden="true">
        <ThemeIcon theme={theme} />
      </span>
    </button>
  );
}
