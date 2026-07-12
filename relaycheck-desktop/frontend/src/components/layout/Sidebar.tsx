export type Tab =
  | "dashboard"
  | "channels"
  | "sites"
  | "checkins"
  | "scan"
  | "notifications"
  | "settings";

export const TABS: Array<{ key: Tab; label: string }> = [
  { key: "dashboard", label: "仪表盘" },
  { key: "channels", label: "渠道" },
  { key: "sites", label: "站点与账号" },
  { key: "checkins", label: "签到" },
  { key: "scan", label: "本机扫描" },
  { key: "notifications", label: "通知" },
  { key: "settings", label: "设置" },
];

/** α-D: task-clustered sidebar groups (no new tabs). */
export const NAV_GROUPS: Array<{
  id: string;
  label: string;
  items: Array<{ key: Tab; label: string }>;
}> = [
  {
    id: "ops",
    label: "运营",
    items: [
      { key: "dashboard", label: "仪表盘" },
      { key: "checkins", label: "签到" },
      { key: "notifications", label: "通知" },
    ],
  },
  {
    id: "assets",
    label: "资产",
    items: [
      { key: "channels", label: "渠道" },
      { key: "sites", label: "站点与账号" },
    ],
  },
  {
    id: "tools",
    label: "工具",
    items: [
      { key: "scan", label: "本机扫描" },
      { key: "settings", label: "设置" },
    ],
  },
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
      <nav aria-label="主导航">
        {NAV_GROUPS.map((group) => (
          <div className="sidebar-nav-group" key={group.id}>
            <div className="sidebar-section-label" id={`nav-group-${group.id}`}>
              {group.label}
            </div>
            <div className="sidebar-nav-items" role="group" aria-labelledby={`nav-group-${group.id}`}>
              {group.items.map((item) => (
                <button
                  key={item.key}
                  type="button"
                  aria-current={activeTab === item.key ? "page" : undefined}
                  className={activeTab === item.key ? "active" : ""}
                  onClick={() => onTabChange(item.key)}
                >
                  {item.label}
                </button>
              ))}
            </div>
          </div>
        ))}
      </nav>
    </aside>
  );
}
