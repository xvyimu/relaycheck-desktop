import { formatCompactNumber, formatTime } from "@/lib/format";
import { diagnosticLevelLabel, schedulerStatusLabel } from "@/lib/labels";
import { LoadingSkeleton } from "../loading-skeleton";
import type {
  ActionCenter,
  StatusPayload,
  SystemDiagnostics,
  ModelOverview,
  ModelPricingOverview,
  UsageOverview,
  TabKey,
  NavigationIntent,
} from "@/types";

export interface HubRadarProps {
  status: StatusPayload;
  diagnostics: SystemDiagnostics | null;
  actionCenter: ActionCenter | null;
  modelOverview: ModelOverview | null;
  pricingOverview: ModelPricingOverview | null;
  usageOverview: UsageOverview | null;
  onNavigate: (tab: TabKey, intent?: Omit<NavigationIntent, "target">) => void;
  onRefresh: () => void;
}

function actionNavigationIntent(
  item: { target: TabKey; filter?: string }
): NavigationIntent {
  switch (item.target) {
    case "accounts":
      return { target: "accounts", accountStatus: item.filter === "problem" ? "problem" : "all" };
    case "checkins":
      return { target: "checkins", checkinStatus: item.filter === "problem" ? "problem" : "all" };
    case "channels":
      if (item.filter === "missing") return { target: "channels", sourceStatus: "missing" };
      if (item.filter === "unknown")
        return { target: "channels", channelKind: "unknown", sourceStatus: "not_archived" };
      return { target: "channels" };
    case "balances":
      return { target: "balances" };
    case "sites":
      return { target: "sites", siteHealth: item.filter === "unreachable" ? "unreachable" : "all" };
    case "notifications":
      return { target: "notifications", unreadOnly: item.filter === "unread" };
    case "scan":
      return { target: "scan" };
    case "settings":
      return { target: "settings" };
    default:
      return { target: "dashboard" };
  }
}

export function HubRadar({
  status,
  diagnostics,
  actionCenter,
  modelOverview,
  pricingOverview,
  usageOverview,
  onNavigate,
  onRefresh,
}: HubRadarProps) {
  const issueItems = (actionCenter?.items || []).filter((item) => item.level === "danger" || item.level === "warning");
  const topIssue = issueItems[0];
  const schedulerJobs = status.scheduler?.jobs || [];
  const checkinJob = schedulerJobs.find((job) => job.key === "checkin.daily");
  const syncJob = schedulerJobs.find((job) => job.key === "sync.local_newapi");
  const knownModels = modelOverview?.modelCount ?? 0;
  const usableKeys = modelOverview?.usableModelCount ?? 0;
  const validKeys = modelOverview?.validKeyCount ?? 0;
  const priceRows = pricingOverview?.sourceCount ?? 0;
  const priceModels = pricingOverview?.modelCount ?? 0;
  const lowBalance = usageOverview?.lowBalanceCount ?? 0;
  const declining = usageOverview?.decliningCount ?? 0;
  const healthLabel = diagnostics ? diagnosticLevelLabel(diagnostics.overall) : "读取中";
  const radarLoading = !modelOverview && !pricingOverview && !usageOverview;

  const estimatedDailyUseText = usageOverview
    ? Object.entries(usageOverview.estimatedDailyUse)
        .map(([k, v]) => `${k}:${v}`)
        .join(" ")
    : "快照待刷新";

  return (
    <section className="hub-radar" aria-label="AI API Hub 雷达">
      <div className="hub-radar-head">
        <div>
          <span>AI API Hub Radar</span>
          <strong>资产、Key、成本和自动化</strong>
        </div>
        <button type="button" className="ghost" onClick={onRefresh}>刷新雷达</button>
      </div>
      <div className="hub-radar-grid">
        {radarLoading ? <LoadingSkeleton variant="chart" title="正在生成模型、价格和用量雷达" /> : null}
        <article className="hub-radar-card asset-card">
          <div className="radar-card-top">
            <span>资产底座</span>
            <strong>{status.summary.accountCount}</strong>
          </div>
          <p>{status.summary.importedChannelCount} 渠道 · {status.summary.localNewApiCount} 本地 NewAPI</p>
          <div className="radar-metrics">
            <span>已识别 {status.summary.identifiedChannelCount}</span>
            <span>通知 {status.summary.unreadNotifications}</span>
          </div>
          <div className="radar-actions">
            <button type="button" onClick={() => onNavigate("channels")}>渠道</button>
            <button type="button" className="ghost" onClick={() => onNavigate("scan")}>同步</button>
          </div>
        </article>

        <article className="hub-radar-card key-card">
          <div className="radar-card-top">
            <span>Key / 模型</span>
            <strong>{knownModels ? formatCompactNumber(knownModels) : "-"}</strong>
          </div>
          <p>{validKeys} 有效 Key · {usableKeys} 个可调用模型账号</p>
          <div className="radar-metrics">
            <span>{modelOverview?.fastestLatencyMs ? `最快 ${modelOverview.fastestLatencyMs}ms` : "待测速"}</span>
            <span>{modelOverview?.sites.length ?? 0} 站点</span>
          </div>
          <div className="radar-actions">
            <button type="button" onClick={() => onNavigate("accounts", { accountStatus: "all" })}>Key 库</button>
            <button type="button" className="ghost" onClick={() => onNavigate("accounts", { query: "unchecked" })}>待检测</button>
          </div>
        </article>

        <article className={`hub-radar-card usage-card ${lowBalance || declining ? "is-warning" : ""}`}>
          <div className="radar-card-top">
            <span>成本 / 用量</span>
            <strong>{lowBalance}</strong>
          </div>
          <p>{priceRows} 价格来源 · {priceModels} 模型价格</p>
          <div className="radar-metrics">
            <span>下降 {declining}</span>
            <span>{estimatedDailyUseText}</span>
          </div>
          <div className="radar-actions">
            <button type="button" onClick={() => onNavigate("balances")}>余额用量</button>
            <button type="button" className="ghost" onClick={() => onNavigate("accounts")}>价格雷达</button>
          </div>
        </article>

        <article className={`hub-radar-card ops-card ${issueItems.length ? "is-warning" : ""}`}>
          <div className="radar-card-top">
            <span>自动化 / 健康</span>
            <strong>{issueItems.length}</strong>
          </div>
          <p>{topIssue ? topIssue.title : `系统状态 ${healthLabel}`}</p>
          <div className="radar-metrics">
            <span>签到 {checkinJob?.nextRunAt ? formatTime(checkinJob.nextRunAt) : schedulerStatusLabel(checkinJob?.status || "idle")}</span>
            <span>同步 {syncJob?.nextRunAt ? formatTime(syncJob.nextRunAt) : schedulerStatusLabel(syncJob?.status || "idle")}</span>
          </div>
          <div className="radar-actions">
            <button
              type="button"
              onClick={() =>
                topIssue
                  ? (() => {
                      const intent = actionNavigationIntent(topIssue);
                      const { target, ...nextIntent } = intent;
                      onNavigate(target, nextIntent);
                    })()
                  : onNavigate("dashboard")
              }
            >
              {topIssue ? "处理问题" : "查看自检"}
            </button>
            <button type="button" className="ghost" onClick={() => onNavigate("settings")}>调度</button>
          </div>
        </article>
      </div>
    </section>
  );
}
