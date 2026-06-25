import { Empty } from "@/components/ui/empty";
import { HubRadar } from "@/components/dashboard/HubRadar";
import { AnalyticsPanel } from "@/components/dashboard/AnalyticsPanel";
import { UpdateBanner } from "@/components/ui/UpdateBanner";
import { formatTime } from "@/lib/format";
import type {
  Account,
  ActionCenter,
  CheckinStatus,
  ImportedChannel,
  ModelOverview,
  ModelPricingOverview,
  NotificationItem,
  StatusPayload,
  SystemDiagnostics,
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
  onNavigate: (tab: TabKey) => void;
  onRefresh: () => Promise<void>;
}

function numberValue(value: number | undefined) {
  return typeof value === "number" ? value.toLocaleString() : "0";
}

function statusTone(value?: string) {
  const normalized = (value || "unknown").toLowerCase();
  if (["success", "ok", "healthy", "active", "valid", "enabled"].includes(normalized)) return "good";
  if (["failed", "error", "danger", "invalid", "expired", "unreachable"].includes(normalized)) return "bad";
  if (["warning", "missing", "archived", "unknown", "unchecked"].includes(normalized)) return "warn";
  return "neutral";
}

function Badge({ value }: { value?: string }) {
  const label = value || "unknown";
  return <span className={`badge ${statusTone(label)}`}>{label}</span>;
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

export function Dashboard({
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
          onNavigate={(nextTab) => onNavigate(nextTab)}
          onRefresh={() => void onRefresh()}
        />
      ) : null}
      <section className="metric-grid">
        <Metric title="本地 NewAPI" value={status?.summary.localNewApiCount} />
        <Metric title="渠道" value={status?.summary.importedChannelCount ?? channels.length} />
        <Metric title="已识别" value={status?.summary.identifiedChannelCount} />
        <Metric title="账号" value={status?.summary.accountCount ?? accounts.length} />
        <Metric title="未读" value={status?.summary.unreadNotifications ?? unread} />
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
          {schedulerJobs.length ? (
            <div className="stack">
              {schedulerJobs.slice(0, 4).map((job) => (
                <div className="list-row" key={job.key}>
                  <div>
                    <strong>{job.label}</strong>
                    <span>{job.nextRunAt ? `下次：${formatTime(job.nextRunAt)}` : job.lastError || "暂无下次运行"}</span>
                  </div>
                  <Badge value={job.status} />
                </div>
              ))}
            </div>
          ) : (
            <Empty message="暂无调度数据。" />
          )}
        </Card>
      </section>
      <AnalyticsPanel />
    </>
  );
}
