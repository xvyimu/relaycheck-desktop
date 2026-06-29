import { memo, useMemo } from "react";
import { Empty } from "@/components/ui/empty";
import { HubRadar } from "@/components/dashboard/HubRadar";
import { AnalyticsPanel } from "@/components/dashboard/AnalyticsPanel";
import { UpdateBanner } from "@/components/ui/UpdateBanner";
import { Badge as UiBadge } from "@/components/ui/badge";
import { useNextRuns } from "@/hooks/useNextRuns";
import { formatDuration, formatTime } from "@/lib/format";
import { actionItemNavigationIntent } from "@/lib/navigation";
import { statusTone, toneBadgeVariant } from "@/lib/tone";
import type {
  Account,
  ActionCenter,
  ActionItem,
  CheckinStatus,
  ImportedChannel,
  ModelOverview,
  ModelPricingOverview,
  NotificationItem,
  StatusPayload,
  SystemDiagnostics,
  NavigationIntent,
  TabKey,
  UpstreamSite,
  UsageOverview,
} from "@/types";

export interface DashboardProps {
  status: StatusPayload | null;
  channels: ImportedChannel[];
  sites: UpstreamSite[];
  accounts: Account[];
  checkins: CheckinStatus | null;
  notifications: NotificationItem[];
  diagnostics: SystemDiagnostics | null;
  actionCenter: ActionCenter | null;
  modelOverview: ModelOverview | null;
  pricingOverview: ModelPricingOverview | null;
  usageOverview: UsageOverview | null;
  onNavigate: (tab: TabKey, intent?: Omit<NavigationIntent, "target">) => void;
  onRefresh: () => Promise<void>;
}

function numberValue(value: number | undefined) {
  return typeof value === "number" ? value.toLocaleString() : "0";
}

function StatusBadge({ value }: { value?: string }) {
  const label = value || "unknown";
  return <UiBadge variant={toneBadgeVariant(statusTone(label))}>{label}</UiBadge>;
}

function Metric({ title, value }: { title: string; value?: number }) {
  return (
    <div className="metric-card">
      <span>{title}</span>
      <strong>{numberValue(value)}</strong>
    </div>
  );
}

function Card({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="card">
      <h2>{title}</h2>
      {children}
    </section>
  );
}

function Row({ label, value }: { label: string; value: number | string }) {
  return (
    <div className="kv-row">
      <span>{label}</span>
      <strong>{typeof value === "number" ? value.toLocaleString() : value}</strong>
    </div>
  );
}

function actionCategoryLabel(category?: string) {
  const labels: Record<string, string> = {
    auth: "授权",
    key: "Key",
    checkin: "签到",
    balance: "余额",
    channel: "渠道",
    site: "站点",
    notification: "通知",
  };
  return labels[category || ""] || "运营";
}

function navigateAction(onNavigate: DashboardProps["onNavigate"], item: ActionItem) {
  const intent = actionItemNavigationIntent(item);
  const { target, ...nextIntent } = intent;
  onNavigate(target, nextIntent);
}

function DashboardBase({
  status,
  channels,
  sites,
  accounts,
  checkins,
  notifications,
  diagnostics,
  actionCenter,
  modelOverview,
  pricingOverview,
  usageOverview,
  onNavigate,
  onRefresh,
}: DashboardProps) {
  const problemChannels = channels.filter((item) => item.sourceSyncStatus === "missing" || item.upstreamKind === "unknown").length;
  const problemAccounts = accounts.filter((item) => ["expired", "invalid", "failed"].includes((item.loginStatus || "").toLowerCase())).length;
  const unread = notifications.filter((item) => !item.read).length;
  const schedulerJobs = status?.scheduler?.jobs || [];
  const actionItems = actionCenter?.items || [];
  const priorityActions = actionItems;

  const { nextRuns, loading: nextRunsBusy } = useNextRuns();

  const schedulerContent = useMemo<React.ReactNode>(() => {
    if (nextRunsBusy) {
      return <Empty message="加载中…" />;
    }
    if (nextRuns.length) {
      return (
        <div className="stack">
          {nextRuns.slice(0, 8).map((item) => (
            <div className="list-row" key={item.jobKey}>
              <div>
                <strong>{item.label}</strong>
                <span>
                  {item.siteName ? `${item.siteName} · ` : ""}
                  {formatDuration(item.nextRunInSeconds)}
                </span>
              </div>
              {item.nextRunAt ? (
                <span className="text-xs text-muted-foreground">{formatTime(item.nextRunAt)}</span>
              ) : (
                <StatusBadge value={item.status} />
              )}
            </div>
          ))}
        </div>
      );
    }
    if (schedulerJobs.length) {
      return (
        <div className="stack">
          {schedulerJobs.slice(0, 4).map((job) => (
            <div className="list-row" key={job.key}>
              <div>
                <strong>{job.label}</strong>
                <span>{job.nextRunAt ? `下次：${formatTime(job.nextRunAt)}` : job.lastError || "暂无下次运行"}</span>
              </div>
              <StatusBadge value={job.status} />
            </div>
          ))}
        </div>
      );
    }
    return <Empty message="暂无调度数据。" />;
  }, [nextRuns, nextRunsBusy, schedulerJobs]);

  return (
    <>
      <UpdateBanner />
      {status ? (
        <HubRadar
          status={status}
          diagnostics={diagnostics}
          actionCenter={actionCenter}
          modelOverview={modelOverview}
          pricingOverview={pricingOverview}
          usageOverview={usageOverview}
          onNavigate={onNavigate}
          onRefresh={onRefresh}
        />
      ) : null}
      <section className="metric-grid">
        <Metric title="本地 NewAPI" value={status?.summary.localNewApiCount} />
        <Metric title="渠道" value={status?.summary.importedChannelCount ?? channels.length} />
        <Metric title="已识别" value={status?.summary.identifiedChannelCount} />
        <Metric title="账号" value={status?.summary.accountCount ?? accounts.length} />
        <Metric title="未读" value={status?.summary.unreadNotifications ?? unread} />
      </section>
      <section className="card dashboard-priority-card">
        <div className="section-heading">
          <div>
            <h2>运营待办</h2>
            <span>{priorityActions.length ? `按风险优先处理 ${priorityActions.length} 项` : "当前没有需要立即处理的运营事项"}</span>
          </div>
          <button type="button" className="ghost" onClick={() => void onRefresh()}>刷新待办</button>
        </div>
        {priorityActions.length ? (
          <div className="dashboard-priority-list">
            {priorityActions.map((item) => (
              <article className={`dashboard-priority-item level-${item.level}`} key={item.id}>
                <div>
                  <div className="dashboard-priority-head">
                    <span className="action-category">{actionCategoryLabel(item.category)}</span>
                    <b>{item.count}</b>
                  </div>
                  <strong>{item.title}</strong>
                  <span>{item.impact || item.description}</span>
                </div>
                {item.samples?.length ? (
                  <div className="task-samples">
                    {item.samples.slice(0, 3).map((sample) => (
                      <span key={sample}>{sample}</span>
                    ))}
                  </div>
                ) : null}
                <em>{item.recommendedAction || item.action}</em>
                <div className="dashboard-priority-actions">
                  <button type="button" onClick={() => navigateAction(onNavigate, item)}>处理</button>
                  <button type="button" className="ghost" onClick={() => navigateAction(onNavigate, item)}>查看列表</button>
                </div>
              </article>
            ))}
          </div>
        ) : (
          <Empty message="运营状态清爽，暂无待办。" />
        )}
      </section>
      <section className="card-grid">
        <Card title="系统">
          <dl className="kv">
            <dt>产品</dt>
            <dd>{status?.productName || "RelayCheck Desktop"}</dd>
            <dt>版本</dt>
            <dd>{status?.productVersion || "未知"}</dd>
            <dt>运行时</dt>
            <dd>{status ? `${status.bindAddress}:${status.port}` : "未知"}</dd>
            <dt>自检</dt>
            <dd>{status?.lastDiagnostics?.overall || "未知"}</dd>
          </dl>
        </Card>
        <Card title="运营">
          <div className="stack">
            <Row label="待复核渠道" value={problemChannels} />
            <Row label="待复核账号" value={problemAccounts} />
            <Row label="今日待签到" value={checkins?.today.dueAccounts ?? 0} />
            <Row label="今日签到失败" value={checkins?.today.failedCount ?? 0} />
          </div>
        </Card>
        <Card title="调度器">
          {schedulerContent}
        </Card>
      </section>
      <AnalyticsPanel />
    </>
  );
}

export const Dashboard = memo(DashboardBase);
