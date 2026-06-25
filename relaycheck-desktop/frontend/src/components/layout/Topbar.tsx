import { ThemeToggle } from "@/components/ui/ThemeToggle";
import type { Tab } from "./Sidebar";
import { TABS } from "./Sidebar";

interface TopbarProps {
  activeTab: Tab;
  onRefresh: () => void;
}

export function Topbar({ activeTab, onRefresh }: TopbarProps) {
  const label = TABS.find((item) => item.key === activeTab)?.label ?? "";
  return (
    <header className="topbar">
      <div>
        <p className="eyebrow">本地控制台</p>
        <h1>{label}</h1>
      </div>
      <div className="topbar-actions">
        <ThemeToggle />
        <button type="button" onClick={onRefresh}>刷新</button>
      </div>
    </header>
  );
}
