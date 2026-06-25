export type Tab = "dashboard" | "channels" | "sites" | "accounts" | "checkins" | "notifications" | "settings";

export const TABS: Array<{ key: Tab; label: string }> = [
  { key: "dashboard", label: "仪表盘" },
  { key: "channels", label: "渠道" },
  { key: "sites", label: "站点" },
  { key: "accounts", label: "账号" },
  { key: "checkins", label: "签到" },
  { key: "notifications", label: "通知" },
  { key: "settings", label: "设置" },
];

interface SidebarProps {
  activeTab: Tab;
  onTabChange: (tab: Tab) => void;
}

export function Sidebar({ activeTab, onTabChange }: SidebarProps) {
  return (
    <aside className="sidebar">
      <div className="brand">
        <span className="brand-mark">R</span>
        <div>
          <strong>RelayCheck</strong>
          <small>恢复控制台</small>
        </div>
      </div>
      <nav>
        {TABS.map((item) => (
          <button
            key={item.key}
            className={activeTab === item.key ? "active" : ""}
            onClick={() => onTabChange(item.key)}
          >
            {item.label}
          </button>
        ))}
      </nav>
    </aside>
  );
}
