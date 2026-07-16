import { formatCompactNumber, formatTime } from "@/lib/format";
import { diagnosticLevelLabel, schedulerStatusLabel } from "@/lib/labels";
import { actionItemNavigationIntent } from "@/lib/navigation";
import { LoadingSkeleton } from "../loading-skeleton";
import type { SchedulerPreviewState } from "@/hooks/useSchedulerPreview";
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
import { Button } from "@/components/ui/button";

export interface HubRadarProps {
  status: StatusPayload;
  diagnostics: SystemDiagnostics | null;
  actionCenter: ActionCenter | null;
  modelOverview: ModelOverview | null;
  pricingOverview: ModelPricingOverview | null;
  usageOverview: UsageOverview | null;
  schedulerPreview: Pick<
    SchedulerPreviewState,
    "calendarItems" | "calendarGroups" | "calendarLoading" | "refreshCalendar"
  >;
  onNavigate: (tab: TabKey, intent?: Omit<NavigationIntent, "target">) => void;
  onRefresh: () => void;
}

export function HubRadar({
  status,
  diagnostics,
  actionCenter,
  modelOverview,
  pricingOverview,
  usageOverview,
  schedulerPreview,
  onNavigate,
  onRefresh,
}: HubRadarProps) {
  const { calendarItems, calendarGroups, calendarLoading: calendarBusy, refreshCalendar } = schedulerPreview;

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
        <Button variant="ghost" type="button" onClick={onRefresh}>
          刷新雷达
        </Button>
      </div>
      <div className="hub-radar-grid">
        {radarLoading ? <LoadingSkeleton variant="chart" title="正在生成模型、价格和用量雷达" /> : null}
        <article className="hub-radar-card asset-card">
          <div className="radar-card-top">
            <span>资产底座</span>
            <strong>{status.summary.accountCount}</strong>
          </div>
          <p>
            {status.summary.importedChannelCount} 渠道 · {status.summary.localNewApiCount} 本地 NewAPI
          </p>
          <div className="radar-metrics">
            <span>已识别 {status.summary.identifiedChannelCount}</span>
            <span>通知 {status.summary.unreadNotifications}</span>
          </div>
          <div className="radar-actions">
            <button type="button" onClick={() => onNavigate("channels")}>
              渠道
            </button>
            <Button variant="ghost" type="button" onClick={() => onNavigate("scan")}>
              同步
            </Button>
          </div>
        </article>

        <article className="hub-radar-card key-card">
          <div className="radar-card-top">
            <span>Key / 模型</span>
            <strong>{knownModels ? formatCompactNumber(knownModels) : "-"}</strong>
          </div>
          <p>
            {validKeys} 有效 Key · {usableKeys} 个可调用模型账号
          </p>
          <div className="radar-metrics">
            <span>{modelOverview?.fastestLatencyMs ? `最快 ${modelOverview.fastestLatencyMs}ms` : "待测速"}</span>
            <span>{modelOverview?.sites?.length ?? 0} 站点</span>
          </div>
          <div className="radar-actions">
            <button type="button" onClick={() => onNavigate("sites", { accountsView: "all", accountStatus: "all" })}>
              Key 库
            </button>
            <Button
              variant="ghost"
              type="button"
              onClick={() => onNavigate("sites", { accountsView: "all", query: "unchecked" })}
            >
              待检测
            </Button>
          </div>
        </article>

        <article className={`hub-radar-card usage-card ${lowBalance || declining ? "is-warning" : ""}`}>
          <div className="radar-card-top">
            <span>成本 / 用量</span>
            <strong>{lowBalance}</strong>
          </div>
          <p>
            {priceRows} 价格来源 · {priceModels} 模型价格
          </p>
          <div className="radar-metrics">
            <span>下降 {declining}</span>
            <span>{estimatedDailyUseText}</span>
          </div>
          <div className="radar-actions">
            <button type="button" onClick={() => onNavigate("sites", { accountsView: "all", query: "余额" })}>
              余额用量
            </button>
            <Button variant="ghost" type="button" onClick={() => onNavigate("sites", { accountsView: "all" })}>
              价格雷达
            </Button>
          </div>
        </article>

        <article className={`hub-radar-card ops-card ${issueItems.length ? "is-warning" : ""}`}>
          <div className="radar-card-top">
            <span>{topIssue?.category ? `运营 / ${topIssue.category}` : "自动化 / 健康"}</span>
            <strong>{issueItems.length}</strong>
          </div>
          <p>
            {topIssue ? `${topIssue.title}：${topIssue.impact || topIssue.description}` : `系统状态 ${healthLabel}`}
          </p>
          <div className="radar-metrics">
            <span>
              签到{" "}
              {checkinJob?.nextRunAt
                ? formatTime(checkinJob.nextRunAt)
                : schedulerStatusLabel(checkinJob?.status || "idle")}
            </span>
            <span>
              同步{" "}
              {syncJob?.nextRunAt ? formatTime(syncJob.nextRunAt) : schedulerStatusLabel(syncJob?.status || "idle")}
            </span>
          </div>
          <div className="radar-actions">
            <button
              type="button"
              onClick={() => {
                if (!topIssue) {
                  onNavigate("dashboard");
                  return;
                }
                const intent = actionItemNavigationIntent(topIssue);
                const { target, ...nextIntent } = intent;
                onNavigate(target, nextIntent);
              }}
            >
              {topIssue ? "处理问题" : "查看自检"}
            </button>
            <Button variant="ghost" type="button" onClick={() => onNavigate("settings")}>
              调度
            </Button>
          </div>
        </article>

        <article className="hub-radar-card schedule-card">
          <div className="radar-card-top">
            <span>排程日预览</span>
            <strong>{calendarItems.length}</strong>
          </div>
          {calendarBusy ? (
            <p className="text-sm text-muted-foreground">加载中…</p>
          ) : calendarItems.length === 0 ? (
            <p className="text-sm text-muted-foreground">暂无排程或获取失败</p>
          ) : (
            <div className="schedule-day-group">
              {Object.entries(calendarGroups).map(([date, items]) => (
                <div key={date} className="schedule-day">
                  <div className="schedule-day-label">{date.split("-").slice(1).join("-")}</div>
                  {items.map((item) => (
                    <div key={`${item.siteId}-${item.time}-${item.jobType}`} className="schedule-item">
                      <span className="schedule-item-time">{item.time.slice(0, 5)}</span>
                      <span className="schedule-item-name">{item.siteName || "未命名"}</span>
                      <span
                        className={`schedule-item-badge ${item.jobType === "sync" ? "badge-sync" : "badge-checkin"}`}
                      >
                        {item.jobType === "sync" ? "同步" : "签到"}
                      </span>
                    </div>
                  ))}
                </div>
              ))}
            </div>
          )}
          <div className="radar-actions">
            <button type="button" onClick={() => onNavigate("settings")}>
              排程设置
            </button>
            <Button variant="ghost" type="button" onClick={() => void refreshCalendar()}>
              刷新
            </Button>
          </div>
        </article>
      </div>
    </section>
  );
}
