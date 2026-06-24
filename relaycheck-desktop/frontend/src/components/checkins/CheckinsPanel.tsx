import { useMemo, useState } from "react";

import { api } from "@/api/client";
import { formatTime } from "@/lib/format";
import type { CheckinStatus } from "@/types";

type CheckinsPanelProps = {
  checkins: CheckinStatus | null;
  onRefresh: () => Promise<void>;
};

function formatCountdown(seconds?: number) {
  if (!Number.isFinite(seconds) || !seconds || seconds <= 0) return "now";
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  if (days > 0) return `${days}d ${hours}h`;
  if (hours > 0) return `${hours}h ${minutes}m`;
  return `${Math.max(1, minutes)}m`;
}

function MetricTile({ label, value }: { label: string; value: number | string }) {
  return (
    <div>
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

export function CheckinsPanel({ checkins, onRefresh }: CheckinsPanelProps) {
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");

  const progress = useMemo(() => {
    const total = Math.max(checkins?.totalAccounts || 0, checkins?.processedAccounts || 0, 1);
    const processed = Math.min(checkins?.processedAccounts || 0, total);
    return {
      total,
      processed,
      percent: Math.round((processed / total) * 100),
    };
  }, [checkins?.processedAccounts, checkins?.totalAccounts]);

  async function runAll() {
    setBusy(true);
    setMessage("");
    try {
      await api("/api/checkins/run-all", { method: "POST" });
      await onRefresh();
      setMessage("Check-in run started.");
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Failed to start check-ins.");
    } finally {
      setBusy(false);
    }
  }

  const running = Boolean(checkins?.running);
  const today = checkins?.today;
  const schedule = checkins?.schedule;

  return (
    <section className="checkin-panel">
      <div className="channel-summary checkin-summary compact-summary">
        <div>
          <span>Mode</span>
          <strong>{checkins?.mode || "idle"}</strong>
        </div>
        <div>
          <span>Processed</span>
          <strong>
            {checkins?.processedAccounts || 0}/{checkins?.totalAccounts || 0}
          </strong>
        </div>
        <div>
          <span>Due today</span>
          <strong>{today?.dueAccounts || 0}</strong>
        </div>
        <div>
          <span>Next run</span>
          <strong>{formatCountdown(schedule?.nextRunInSeconds)}</strong>
        </div>
      </div>

      {message ? <div className="problem-hint">{message}</div> : null}

      <div className="checkin-grid">
        <article className="checkin-card checkin-run-card">
          <div className="section-heading">
            <div>
              <strong>Run state</strong>
              <span>Current batch progress and active account.</span>
            </div>
            <span className={`status-pill ${running ? "success" : "neutral"}`}>
              {running ? "running" : "idle"}
            </span>
          </div>

          <div className="checkin-progress" aria-label="Check-in progress">
            <div>
              <span>{progress.percent}%</span>
              <strong>
                {progress.processed}/{progress.total}
              </strong>
            </div>
            <div
              aria-valuemax={progress.total}
              aria-valuemin={0}
              aria-valuenow={progress.processed}
              className="checkin-progress-track"
              role="progressbar"
            >
              <span style={{ width: `${progress.percent}%` }} />
            </div>
          </div>

          <dl className="kv checkin-kv">
            <dt>Current account</dt>
            <dd>{checkins?.currentAccount || "-"}</dd>
            <dt>Current site</dt>
            <dd>{checkins?.currentSite || "-"}</dd>
            <dt>Pending</dt>
            <dd>{checkins?.pendingAccounts ?? 0}</dd>
            <dt>Started</dt>
            <dd>{formatTime(checkins?.startedAt || "")}</dd>
            <dt>Updated</dt>
            <dd>{formatTime(checkins?.updatedAt || "")}</dd>
          </dl>

          {checkins?.currentMessage || checkins?.lastRunMessage ? (
            <div className="problem-hint detail-hint">
              {checkins.currentMessage || checkins.lastRunMessage}
            </div>
          ) : null}

          <button
            className="wide"
            disabled={busy || running}
            onClick={() => void runAll()}
            type="button"
          >
            {busy || running ? "Running..." : "Run all check-ins"}
          </button>
        </article>

        <article className="checkin-card">
          <div className="section-heading">
            <div>
              <strong>Today</strong>
              <span>Result distribution for today's check-ins.</span>
            </div>
          </div>
          <div className="checkin-metrics">
            <MetricTile label="Success" value={today?.successCount || 0} />
            <MetricTile label="Already" value={today?.alreadyCount || 0} />
            <MetricTile label="Failed" value={today?.failedCount || 0} />
            <MetricTile label="Unsupported" value={today?.unsupportedCount || 0} />
            <MetricTile label="Auth expired" value={today?.authExpiredCount || 0} />
            <MetricTile label="Logs" value={today?.totalLogs || 0} />
          </div>
        </article>

        <article className="checkin-card">
          <div className="section-heading">
            <div>
              <strong>Schedule</strong>
              <span>Automation window and next execution time.</span>
            </div>
            <span className={`status-pill ${schedule?.enabled ? "success" : "neutral"}`}>
              {schedule?.enabled ? "enabled" : "disabled"}
            </span>
          </div>
          <dl className="kv checkin-kv">
            <dt>Time</dt>
            <dd>{schedule?.time || "-"}</dd>
            <dt>Random delay</dt>
            <dd>
              {schedule ? `${schedule.randomDelayMin}-${schedule.randomDelayMax} min` : "-"}
            </dd>
            <dt>Window start</dt>
            <dd>{formatTime(schedule?.nextWindowStartAt || "")}</dd>
            <dt>Window end</dt>
            <dd>{formatTime(schedule?.nextWindowEndAt || "")}</dd>
            <dt>Next run</dt>
            <dd>{formatTime(schedule?.nextRunAt || "")}</dd>
            <dt>Countdown</dt>
            <dd>{formatCountdown(schedule?.nextRunInSeconds)}</dd>
          </dl>
          {schedule?.message ? <div className="note">{schedule.message}</div> : null}
        </article>
      </div>
    </section>
  );
}
