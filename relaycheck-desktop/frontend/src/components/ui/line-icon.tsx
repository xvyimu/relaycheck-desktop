import type { LineIconName } from "@/types";

export function LineIcon({ name, className = "" }: { name: LineIconName; className?: string }) {
  const common = { fill: "none", stroke: "currentColor", strokeLinecap: "round" as const, strokeLinejoin: "round" as const, strokeWidth: 2 };
  return (
    <svg className={`line-icon ${className}`.trim()} viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      {name === "dashboard" ? <><rect x="4" y="4" width="6" height="7" rx="1.5" {...common} /><rect x="14" y="4" width="6" height="4" rx="1.5" {...common} /><rect x="4" y="15" width="6" height="5" rx="1.5" {...common} /><rect x="14" y="12" width="6" height="8" rx="1.5" {...common} /></> : null}
      {name === "channels" ? <><path d="M5 7h14" {...common} /><path d="M5 12h14" {...common} /><path d="M5 17h14" {...common} /><circle cx="8" cy="7" r="1" fill="currentColor" /><circle cx="16" cy="12" r="1" fill="currentColor" /><circle cx="11" cy="17" r="1" fill="currentColor" /></> : null}
      {name === "sites" ? <><circle cx="12" cy="12" r="8" {...common} /><path d="M4 12h16M12 4a12 12 0 0 1 0 16M12 4a12 12 0 0 0 0 16" {...common} /></> : null}
      {name === "accounts" ? <><circle cx="9" cy="8" r="3" {...common} /><path d="M4 19a5 5 0 0 1 10 0" {...common} /><path d="M16 11h4M18 9v4" {...common} /></> : null}
      {name === "checkins" ? <><path d="M5 13l4 4L19 7" {...common} /><path d="M4 5h8" {...common} /><path d="M4 19h16" {...common} /></> : null}
      {name === "balances" ? <><circle cx="12" cy="12" r="8" {...common} /><path d="M12 7v10M9 9.5c0-1.2 1.1-2 3-2 1.2 0 2 .35 2.5.8M15 14.5c0 1.2-1.1 2-3 2-1.2 0-2-.35-2.5-.8" {...common} /></> : null}
      {name === "notifications" ? <><path d="M6 10a6 6 0 1 1 12 0c0 4 2 5 2 5H4s2-1 2-5" {...common} /><path d="M10 19a2 2 0 0 0 4 0" {...common} /></> : null}
      {name === "scan" ? <><path d="M4 8V5a1 1 0 0 1 1-1h3M16 4h3a1 1 0 0 1 1 1v3M20 16v3a1 1 0 0 1-1 1h-3M8 20H5a1 1 0 0 1-1-1v-3" {...common} /><path d="M8 12h8" {...common} /></> : null}
      {name === "settings" ? <><circle cx="12" cy="12" r="3" {...common} /><path d="M12 3v3M12 18v3M4.8 4.8l2.1 2.1M17.1 17.1l2.1 2.1M3 12h3M18 12h3M4.8 19.2l2.1-2.1M17.1 6.9l2.1-2.1" {...common} /></> : null}
      {name === "success" ? <path d="M5 12l4 4L19 6" {...common} /> : null}
      {name === "warning" ? <><path d="M12 4l9 16H3L12 4z" {...common} /><path d="M12 9v4M12 17h.01" {...common} /></> : null}
      {name === "danger" ? <><circle cx="12" cy="12" r="8" {...common} /><path d="M15 9l-6 6M9 9l6 6" {...common} /></> : null}
      {name === "info" ? <><circle cx="12" cy="12" r="8" {...common} /><path d="M12 11v5M12 8h.01" {...common} /></> : null}
    </svg>
  );
}