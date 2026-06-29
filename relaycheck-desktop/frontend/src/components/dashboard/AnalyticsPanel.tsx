import { useCallback, useEffect, useMemo, useState } from "react";
import { api } from "@/api/client";
import type { BalanceSnapshot, CheckinLog } from "@/types";

type BalanceTrendPoint = { date: string; balance?: number };
type CheckinDistributionItem = { status: string; label: string; count: number; color: string };
type ResponseTimePoint = { accountName: string; siteName: string; latencyMs: number; status: string };
type SiteReliability = {
  siteId: string;
  siteName: string;
  totalCheckins: number;
  successRate: number;
  avgLatencyMs: number;
  lastCheckinAt: string;
};
type BalanceDeltaPoint = { date: string; delta: number; cumulative?: number };

type AnalyticsData = {
  generatedAt: string;
  days?: number;
  balanceTrend?: BalanceTrendPoint[] | null;
  checkinDistribution?: CheckinDistributionItem[] | null;
  responseTimes?: ResponseTimePoint[] | null;
  siteReliability?: SiteReliability[];
  balanceDeltas?: BalanceDeltaPoint[];
};

type RangeOption = 7 | 30 | 90;

const RANGE_OPTIONS: Array<{ value: RangeOption; label: string }> = [
  { value: 7, label: "7 天" },
  { value: 30, label: "30 天" },
  { value: 90, label: "90 天" },
];

function formatTimeShort(iso: string): string {
  if (!iso) return "—";
  try {
    return new Date(iso).toLocaleString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" });
  } catch {
    return iso;
  }
}

function formatPercent(value: number): string {
  if (!Number.isFinite(value)) return "—";
  return `${(value * 100).toFixed(1)}%`;
}

function formatDelta(value: number): string {
  if (!Number.isFinite(value)) return "—";
  const sign = value > 0 ? "+" : "";
  return `${sign}${value.toFixed(2)}`;
}

function BalanceTrendChart({
  data,
  selectedDate,
  onSelectDate,
}: {
  data: BalanceTrendPoint[] | null;
  selectedDate?: string;
  onSelectDate?: (date: string) => void;
}) {
  if (!data || data.length < 2) {
    return <div className="chart-empty">余额数据不足，需要至少 2 天的记录</div>;
  }

  const width = 320;
  const height = 120;
  const padding = { top: 10, right: 10, bottom: 20, left: 40 };
  const chartW = width - padding.left - padding.right;
  const chartH = height - padding.top - padding.bottom;

  const values = data.map((d) => d.balance ?? 0);
  const minVal = Math.min(...values, 0);
  const maxVal = Math.max(...values, 1);
  const range = maxVal - minVal || 1;

  const points = data.map((d, i) => {
    const x = padding.left + (i / (data.length - 1)) * chartW;
    const y = padding.top + chartH - ((d.balance ?? 0) - minVal) / range * chartH;
    return { x, y, date: d.date, balance: d.balance };
  });

  const pathD = points.map((p, i) => `${i === 0 ? "M" : "L"} ${p.x.toFixed(1)} ${p.y.toFixed(1)}`).join(" ");
  const areaD = `${pathD} L ${points[points.length - 1].x.toFixed(1)} ${padding.top + chartH} L ${points[0].x.toFixed(1)} ${padding.top + chartH} Z`;

  return (
    <svg viewBox={`0 0 ${width} ${height}`} className="analytics-chart" role="img" aria-label="余额趋势图">
      <defs>
        <linearGradient id="balanceGrad" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="var(--v4-blue)" stopOpacity="0.2" />
          <stop offset="100%" stopColor="var(--v4-blue)" stopOpacity="0" />
        </linearGradient>
      </defs>
      {/* Grid lines */}
      {[0, 0.25, 0.5, 0.75, 1].map((t) => (
        <line key={t} x1={padding.left} y1={padding.top + t * chartH} x2={padding.left + chartW} y2={padding.top + t * chartH} stroke="var(--v4-border)" strokeWidth="0.5" />
      ))}
      {/* Area */}
      <path d={areaD} fill="url(#balanceGrad)" />
      {/* Line */}
      <path d={pathD} fill="none" stroke="var(--v4-blue)" strokeWidth="2" strokeLinejoin="round" strokeLinecap="round" />
      {/* Points */}
      {points.map((p, i) => {
        const isSelected = selectedDate === p.date;
        return (
          <circle
            key={p.date}
            cx={p.x}
            cy={p.y}
            r={isSelected ? 4 : 2.5}
            fill={isSelected ? "var(--v4-amber)" : "var(--v4-blue)"}
            stroke={isSelected ? "var(--v4-amber)" : "none"}
            strokeWidth={isSelected ? 1 : 0}
            className="chart-point"
            onClick={() => onSelectDate?.(p.date)}
            onKeyDown={(event) => { if (event.key === "Enter" || event.key === " ") { event.preventDefault(); onSelectDate?.(p.date); } }}
            role="button"
            tabIndex={0}
          >
            <title>{`${p.date}: $${p.balance?.toFixed(2) ?? "—"}（点击查看当日详情）`}</title>
          </circle>
        );
      })}
      {/* Y axis labels */}
      <text x={padding.left - 4} y={padding.top + 4} textAnchor="end" fontSize="9" fill="var(--v4-muted)">${maxVal.toFixed(0)}</text>
      <text x={padding.left - 4} y={padding.top + chartH} textAnchor="end" fontSize="9" fill="var(--v4-muted)">${minVal.toFixed(0)}</text>
      {/* X axis labels */}
      <text x={padding.left} y={height - 4} fontSize="9" fill="var(--v4-muted)">{data[0]?.date.slice(5)}</text>
      <text x={padding.left + chartW} y={height - 4} textAnchor="end" fontSize="9" fill="var(--v4-muted)">{data[data.length - 1]?.date.slice(5)}</text>
    </svg>
  );
}

function BalanceDeltaChart({ data }: { data: BalanceDeltaPoint[] }) {
  if (!data || data.length === 0) {
    return <div className="chart-empty">暂无余额增量数据</div>;
  }

  const width = 640;
  const height = 160;
  const padding = { top: 14, right: 14, bottom: 24, left: 48 };
  const chartW = width - padding.left - padding.right;
  const chartH = height - padding.top - padding.bottom;

  const deltas = data.map((d) => d.delta);
  const maxAbs = Math.max(...deltas.map((v) => Math.abs(v)), 1);
  const zeroY = padding.top + chartH / 2;
  const barW = Math.max(2, chartW / data.length - 1);

  return (
    <svg viewBox={`0 0 ${width} ${height}`} className="analytics-chart balance-delta-chart" role="img" aria-label="余额增量图" preserveAspectRatio="xMidYMid meet">
      {/* Zero line */}
      <line x1={padding.left} y1={zeroY} x2={padding.left + chartW} y2={zeroY} stroke="var(--v4-border)" strokeWidth="0.6" strokeDasharray="3 3" />
      {data.map((d, i) => {
        const x = padding.left + (i / data.length) * chartW;
        const barH = (Math.abs(d.delta) / maxAbs) * (chartH / 2);
        const positive = d.delta >= 0;
        const y = positive ? zeroY - barH : zeroY;
        return (
          <rect
            key={d.date}
            x={x}
            y={y}
            width={barW}
            height={Math.max(1, barH)}
            fill={positive ? "var(--v4-green)" : "var(--v4-red)"}
            opacity={0.8}
            className="chart-point"
          >
            <title>{`${d.date}: ${formatDelta(d.delta)}（累计 $${d.cumulative?.toFixed(2) ?? "—"}）`}</title>
          </rect>
        );
      })}
      <text x={padding.left - 6} y={padding.top + 6} textAnchor="end" fontSize="11" fill="var(--v4-muted)">+{maxAbs.toFixed(1)}</text>
      <text x={padding.left - 6} y={zeroY + 4} textAnchor="end" fontSize="11" fill="var(--v4-muted)">0</text>
      <text x={padding.left - 6} y={padding.top + chartH} textAnchor="end" fontSize="11" fill="var(--v4-muted)">-{maxAbs.toFixed(1)}</text>
      <text x={padding.left} y={height - 6} fontSize="11" fill="var(--v4-muted)">{data[0]?.date.slice(5)}</text>
      <text x={padding.left + chartW} y={height - 6} textAnchor="end" fontSize="11" fill="var(--v4-muted)">{data[data.length - 1]?.date.slice(5)}</text>
    </svg>
  );
}

function CheckinDonutChart({
  data,
  selectedStatus,
  onSelectStatus,
}: {
  data: CheckinDistributionItem[];
  selectedStatus?: string;
  onSelectStatus?: (status: string) => void;
}) {
  const total = data.reduce((sum, d) => sum + d.count, 0);
  if (total === 0) {
    return <div className="chart-empty">近 7 天无签到记录</div>;
  }

  const radius = 50;
  const stroke = 16;
  const circumference = 2 * Math.PI * radius;
  let offset = 0;

  return (
    <div className="donut-chart-wrap">
      <svg viewBox="0 0 140 140" className="analytics-donut" role="img" aria-label="签到状态分布">
        <circle cx="70" cy="70" r={radius} fill="none" stroke="var(--v4-border)" strokeWidth={stroke} />
        {data.map((item) => {
          const dash = (item.count / total) * circumference;
          const isSelected = selectedStatus === item.status;
          const circle = (
            <circle
              key={item.status}
              cx="70"
              cy="70"
              r={radius}
              fill="none"
              stroke={item.color}
              strokeWidth={isSelected ? stroke + 4 : stroke}
              strokeDasharray={`${dash} ${circumference - dash}`}
              strokeDashoffset={-offset}
              transform="rotate(-90 70 70)"
              className="donut-segment"
              opacity={selectedStatus && !isSelected ? 0.4 : 1}
              onClick={() => onSelectStatus?.(item.status)}
              onKeyDown={(event) => { if (event.key === "Enter" || event.key === " ") { event.preventDefault(); onSelectStatus?.(item.status); } }}
              role="button"
              tabIndex={0}
            >
              <title>{`${item.label}: ${item.count} (${((item.count / total) * 100).toFixed(1)}%) — 点击筛选`}</title>
            </circle>
          );
          offset += dash;
          return circle;
        })}
        <text x="70" y="66" textAnchor="middle" fontSize="20" fontWeight="700" fill="var(--v4-text)">{total}</text>
        <text x="70" y="80" textAnchor="middle" fontSize="9" fill="var(--v4-muted)">总签到</text>
      </svg>
      <div className="donut-legend">
        {data.map((item) => (
          <button
            key={item.status}
            type="button"
            className={`donut-legend-item ${selectedStatus === item.status ? "active" : ""}`}
            onClick={() => onSelectStatus?.(item.status)}
          >
            <span className="legend-dot" style={{ background: item.color }} />
            <span className="legend-label">{item.label}</span>
            <span className="legend-count">{item.count}</span>
          </button>
        ))}
      </div>
    </div>
  );
}

function ResponseTimeChart({ data }: { data: ResponseTimePoint[] | null }) {
  if (!data || data.length === 0) {
    return <div className="chart-empty">暂无 API Key 延迟数据</div>;
  }

  const maxLatency = Math.max(...data.map((d) => d.latencyMs), 1);
  const barHeight = 18;
  const gap = 4;
  const labelWidth = 100;
  const chartWidth = 200;

  return (
    <div className="response-time-chart">
      {data.slice(0, 10).map((item, i) => {
        const width = (item.latencyMs / maxLatency) * chartWidth;
        const color = item.status === "valid" ? "var(--v4-green)" : item.status === "rate_limited" ? "var(--v4-amber)" : "var(--v4-red)";
        return (
          <div key={`${item.accountName}-${item.siteName}`} className="response-time-row" style={{ top: i * (barHeight + gap) }}>
            <div className="response-time-label" style={{ width: labelWidth }}>
              <span className="response-time-name">{item.accountName}</span>
              <span className="response-time-site">{item.siteName}</span>
            </div>
            <div className="response-time-bar-track" style={{ width: chartWidth }}>
              <div className="response-time-bar" style={{ width: `${width}px`, background: color, height: barHeight - 4 }} />
              <span className="response-time-value">{item.latencyMs}ms</span>
            </div>
          </div>
        );
      })}
    </div>
  );
}

function SiteReliabilityTable({ data }: { data: SiteReliability[] }) {
  if (!data || data.length === 0) {
    return <div className="chart-empty">所选范围内无签到记录</div>;
  }
  return (
    <div className="analytics-table-wrap">
      <table className="analytics-table">
        <thead>
          <tr>
            <th>站点</th>
            <th>签到数</th>
            <th>成功率</th>
            <th>平均耗时</th>
            <th>最近签到</th>
          </tr>
        </thead>
        <tbody>
          {data.map((item) => (
            <tr key={item.siteId}>
              <td className="cell-name">{item.siteName}</td>
              <td>{item.totalCheckins}</td>
              <td>
                <span className={`rate-badge ${item.successRate >= 0.9 ? "good" : item.successRate >= 0.5 ? "warn" : "bad"}`}>
                  {formatPercent(item.successRate)}
                </span>
              </td>
              <td>{item.avgLatencyMs > 0 ? `${item.avgLatencyMs}ms` : "—"}</td>
              <td className="cell-muted">{formatTimeShort(item.lastCheckinAt)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function DrillDownPanel({
  title,
  onClose,
  children,
}: {
  title: string;
  onClose: () => void;
  children: React.ReactNode;
}) {
  return (
    <div className="analytics-drilldown">
      <div className="analytics-drilldown-header">
        <strong>{title}</strong>
        <button type="button" className="drilldown-close" onClick={onClose} aria-label="关闭详情">×</button>
      </div>
      <div className="analytics-drilldown-body">{children}</div>
    </div>
  );
}

function BalanceDayDetail({ date, snapshots }: { date: string; snapshots: BalanceSnapshot[] }) {
  const dayItems = snapshots.filter((s) => s.createdAt.startsWith(date));
  if (dayItems.length === 0) {
    return <div className="chart-empty">{date} 无余额快照记录</div>;
  }
  return (
    <div className="analytics-drilldown-list">
      {dayItems.slice(0, 20).map((s) => (
        <div key={s.id} className="drilldown-row">
          <span className="drilldown-name">{s.accountName}</span>
          <span className="drilldown-site">{s.upstreamSiteName}</span>
          <span className="drilldown-value">
            {s.balance !== undefined && s.balance !== null ? `$${s.balance.toFixed(2)}` : "—"}
            {s.unit && s.unit !== "unknown" ? ` ${s.unit}` : ""}
          </span>
          <span className="drilldown-time">{formatTimeShort(s.createdAt)}</span>
        </div>
      ))}
      {dayItems.length > 20 ? <div className="drilldown-more">还有 {dayItems.length - 20} 条记录…</div> : null}
    </div>
  );
}

function CheckinStatusDetail({ status, label, logs }: { status: string; label: string; logs: CheckinLog[] }) {
  const filtered = logs.filter((l) => l.status === status);
  if (filtered.length === 0) {
    return <div className="chart-empty">该状态暂无签到记录</div>;
  }
  return (
    <div className="analytics-drilldown-list">
      <div className="drilldown-summary">共 {filtered.length} 条「{label}」记录</div>
      {filtered.slice(0, 20).map((l) => (
        <div key={l.id} className="drilldown-row">
          <span className="drilldown-name">{l.accountName}</span>
          <span className="drilldown-site">{l.upstreamSiteName}</span>
          <span className="drilldown-value">{l.reward ? `奖励 ${l.reward}` : l.message || "—"}</span>
          <span className="drilldown-time">{formatTimeShort(l.startedAt)}</span>
        </div>
      ))}
      {filtered.length > 20 ? <div className="drilldown-more">还有 {filtered.length - 20} 条记录…</div> : null}
    </div>
  );
}

export function AnalyticsPanel() {
  const [data, setData] = useState<AnalyticsData | null>(null);
  const [loading, setLoading] = useState(true);
  const [range, setRange] = useState<RangeOption>(30);
  const [selectedDate, setSelectedDate] = useState<string | undefined>(undefined);
  const [selectedStatus, setSelectedStatus] = useState<string | undefined>(undefined);
  const [balanceSnapshots, setBalanceSnapshots] = useState<BalanceSnapshot[]>([]);
  const [checkinLogs, setCheckinLogs] = useState<CheckinLog[]>([]);
  const [drillLoading, setDrillLoading] = useState(false);

  const loadAnalytics = useCallback(async (days: RangeOption) => {
    setLoading(true);
    try {
      const result = await api<AnalyticsData>(`/api/analytics?days=${days}`);
      setData(result);
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadAnalytics(range);
    const timer = setInterval(() => void loadAnalytics(range), 60000);
    return () => clearInterval(timer);
  }, [range, loadAnalytics]);

  // Load supporting data for drill-down on demand.
  useEffect(() => {
    if (!selectedDate && !selectedStatus) return;
    let cancelled = false;
    async function loadDrill() {
      setDrillLoading(true);
      try {
        const [snapshots, logs] = await Promise.all([
          selectedDate ? api<BalanceSnapshot[]>("/api/balances/snapshots") : Promise.resolve(balanceSnapshots),
          selectedStatus ? api<CheckinLog[]>("/api/checkins/logs") : Promise.resolve(checkinLogs),
        ]);
        if (!cancelled) {
          if (selectedDate) setBalanceSnapshots(snapshots);
          if (selectedStatus) setCheckinLogs(logs);
        }
      } catch {
        // ignore
      } finally {
        if (!cancelled) setDrillLoading(false);
      }
    }
    void loadDrill();
    return () => { cancelled = true; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedDate, selectedStatus]);

  const selectedDistItem = useMemo(
    () => (data?.checkinDistribution ?? []).find((d) => d.status === selectedStatus),
    [data?.checkinDistribution, selectedStatus],
  );

  if (loading && !data) {
    return (
      <div className="analytics-panel">
        <div className="analytics-grid">
          <div className="card analytics-card skeleton" style={{ height: 180 }} />
          <div className="card analytics-card skeleton" style={{ height: 180 }} />
          <div className="card analytics-card skeleton" style={{ height: 180 }} />
        </div>
      </div>
    );
  }

  if (!data) return null;

  return (
    <div className="analytics-panel">
      <div className="analytics-header">
        <strong>数据分析</strong>
        <div className="analytics-header-right">
          <div className="range-selector" role="group" aria-label="时间范围">
            {RANGE_OPTIONS.map((opt) => (
              <button
                key={opt.value}
                type="button"
                className={`range-btn ${range === opt.value ? "active" : ""}`}
                onClick={() => setRange(opt.value)}
              >
                {opt.label}
              </button>
            ))}
          </div>
          <span className="analytics-updated">更新于 {formatTimeShort(data.generatedAt)}</span>
        </div>
      </div>
      <div className="analytics-grid">
        <div className="card analytics-card">
          <div className="analytics-card-title">
            余额趋势（{data.days ?? range} 天）
            {selectedDate ? <span className="drilldown-hint"> · 已选 {selectedDate}</span> : <span className="drilldown-hint"> · 点击数据点查看详情</span>}
          </div>
          <BalanceTrendChart
            data={data.balanceTrend ?? []}
            selectedDate={selectedDate}
            onSelectDate={(date) => setSelectedDate((prev) => (prev === date ? undefined : date))}
          />
          {selectedDate ? (
            <DrillDownPanel title={`${selectedDate} 余额详情`} onClose={() => setSelectedDate(undefined)}>
              {drillLoading ? (
                <div className="chart-empty">加载中…</div>
              ) : (
                <BalanceDayDetail date={selectedDate} snapshots={balanceSnapshots} />
              )}
            </DrillDownPanel>
          ) : null}
        </div>
        <div className="card analytics-card">
          <div className="analytics-card-title">
            签到状态分布（7 天）
            {selectedStatus && selectedDistItem ? <span className="drilldown-hint"> · 已选「{selectedDistItem.label}」</span> : <span className="drilldown-hint"> · 点击筛选</span>}
          </div>
          <CheckinDonutChart
            data={data.checkinDistribution ?? []}
            selectedStatus={selectedStatus}
            onSelectStatus={(status) => setSelectedStatus((prev) => (prev === status ? undefined : status))}
          />
          {selectedStatus ? (
            <DrillDownPanel
              title={`「${selectedDistItem?.label ?? selectedStatus}」签到记录`}
              onClose={() => setSelectedStatus(undefined)}
            >
              {drillLoading ? (
                <div className="chart-empty">加载中…</div>
              ) : (
                <CheckinStatusDetail
                  status={selectedStatus}
                  label={selectedDistItem?.label ?? selectedStatus}
                  logs={checkinLogs}
                />
              )}
            </DrillDownPanel>
          ) : null}
        </div>
        <div className="card analytics-card">
          <div className="analytics-card-title">API Key 响应时间</div>
          <ResponseTimeChart data={data.responseTimes ?? []} />
        </div>
      </div>
      <div className="analytics-grid analytics-grid-secondary">
        <div className="card analytics-card">
          <div className="analytics-card-title">站点可靠性（{data.days ?? range} 天）</div>
          <SiteReliabilityTable data={data.siteReliability ?? []} />
        </div>
        <div className="card analytics-card">
          <div className="analytics-card-title">余额增量（{data.days ?? range} 天）</div>
          <BalanceDeltaChart data={data.balanceDeltas ?? []} />
        </div>
      </div>
    </div>
  );
}
