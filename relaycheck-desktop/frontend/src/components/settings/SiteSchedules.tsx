import { useEffect, useState } from "react";
import { api } from "@/api/client";
import { formatTime } from "@/lib/format";
import type { ChannelSchedule, ScheduleCalendarItem, NextRunItem, UpstreamSite } from "@/types";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const MAX_RANDOM_DELAY_MINUTES = 120;

function clampRandomDelay(raw: string): number {
  const parsed = Number(raw);
  if (!Number.isFinite(parsed)) return 0;
  return Math.min(MAX_RANDOM_DELAY_MINUTES, Math.max(0, Math.trunc(parsed)));
}

function formatDate(dateStr: string): string {
  const [y, m, d] = dateStr.split("-");
  return `${y}/${m}/${d}`;
}

type SiteScheduleForm = {
  upstreamSiteId: string;
  enabled: boolean;
  checkinTime: string;
  cronExpr: string;
  skipDates: string[];
  randomDelayMin: number;
  randomDelayMax: number;
};

export function SiteSchedules() {
  const [schedules, setSchedules] = useState<ChannelSchedule[]>([]);
  const [sites, setSites] = useState<UpstreamSite[]>([]);
  const [busy, setBusy] = useState<"saving" | "">("");
  const [message, setMessage] = useState("");

  // Calendar / next-runs data
  const [calendarItems, setCalendarItems] = useState<ScheduleCalendarItem[]>([]);
  const [nextRuns, setNextRuns] = useState<NextRunItem[]>([]);
  const [previewError, setPreviewError] = useState("");

  // Local editing state: map siteId -> form
  const [forms, setForms] = useState<Record<string, SiteScheduleForm>>({});

  // Per-site skip-date temp input
  const [skipInputs, setSkipInputs] = useState<Record<string, string>>({});

  async function refresh() {
    try {
      const [nextSchedules, nextSites] = await Promise.all([
        api<ChannelSchedule[]>("/api/scheduler/channel-schedules"),
        api<UpstreamSite[]>("/api/upstream-sites"),
      ]);
      setSchedules(nextSchedules);
      setSites(nextSites);

      // Initialize forms from fetched data
      const nextForms: Record<string, SiteScheduleForm> = {};
      for (const s of nextSchedules) {
        nextForms[s.upstreamSiteId] = {
          upstreamSiteId: s.upstreamSiteId,
          enabled: s.enabled,
          checkinTime: s.checkinTime,
          cronExpr: s.cronExpr || "",
          skipDates: s.skipDates || [],
          randomDelayMin: s.randomDelayMin,
          randomDelayMax: s.randomDelayMax,
        };
      }
      // Add default form for sites without a schedule
      for (const site of nextSites) {
        if (!nextForms[site.id]) {
          nextForms[site.id] = {
            upstreamSiteId: site.id,
            enabled: false,
            checkinTime: "08:00",
            cronExpr: "",
            skipDates: [],
            randomDelayMin: 0,
            randomDelayMax: 30,
          };
        }
      }
      setForms(nextForms);
    } catch (err) {
      // refresh() is called via `void` on mount and from saveSchedule's
      // try block; without this catch a network failure here would either
      // surface as an unhandled rejection (mount) or be misreported as
      // "保存失败" by saveSchedule's catch even though the save succeeded.
      setMessage(err instanceof Error ? `加载排程失败：${err.message}` : "加载排程失败");
    }
  }

  async function refreshCalendar() {
    try {
      const [cal, runs] = await Promise.all([
        api<{ generatedAt: string; items: ScheduleCalendarItem[] }>(
          "/api/scheduler/calendar?days=7",
        ),
        api<{ generatedAt: string; items: NextRunItem[] }>(
          "/api/scheduler/next-runs",
        ),
      ]);
      setCalendarItems(cal.items);
      setNextRuns(runs.items);
      setPreviewError("");
    } catch (error) {
      setPreviewError(
        error instanceof Error
          ? `排程预览加载失败：${error.message}`
          : "排程预览加载失败，可刷新重试",
      );
    }
  }

  async function saveSchedule(siteId: string) {
    const form = forms[siteId];
    if (!form) return;

    setBusy("saving");
    setMessage("");
    try {
      const result = await api<{ ok: boolean }>(
        "/api/scheduler/channel-schedules",
        {
          method: "PUT",
          body: JSON.stringify(form),
        },
      );
      if (result.ok) {
        setMessage(
          "已保存 " +
            (sites.find((s) => s.id === siteId)?.name || siteId) +
            " 的签到排程。",
        );
        await refresh();
        await refreshCalendar();
      }
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "保存失败");
    } finally {
      setBusy("");
    }
  }

  function updateForm(siteId: string, patch: Partial<SiteScheduleForm>) {
    setForms((prev) => ({
      ...prev,
      [siteId]: { ...prev[siteId], ...patch },
    }));
  }

  function updateRandomDelayMin(siteId: string, raw: string) {
    const nextMin = clampRandomDelay(raw);
    const currentMax = forms[siteId]?.randomDelayMax ?? 30;
    updateForm(siteId, {
      randomDelayMin: nextMin,
      randomDelayMax: Math.max(nextMin, currentMax),
    });
  }

  function updateRandomDelayMax(siteId: string, raw: string) {
    const nextMax = clampRandomDelay(raw);
    const currentMin = forms[siteId]?.randomDelayMin ?? 0;
    updateForm(siteId, {
      randomDelayMin: Math.min(currentMin, nextMax),
      randomDelayMax: nextMax,
    });
  }

  // --- skip-date helpers ---

  function addSkipDate(siteId: string) {
    const val = skipInputs[siteId];
    if (!val) return;
    const existing = forms[siteId]?.skipDates || [];
    if (existing.includes(val)) return;
    updateForm(siteId, { skipDates: [...existing, val] });
    setSkipInputs((prev) => ({ ...prev, [siteId]: "" }));
  }

  function removeSkipDate(siteId: string, date: string) {
    const existing = forms[siteId]?.skipDates || [];
    updateForm(
      siteId,
      { skipDates: existing.filter((d) => d !== date) },
    );
  }

  // Determine scheduled real sites (exclude the global schedule compatibility row).
  const visibleSiteIds = new Set(sites.map((site) => site.id));
  const scheduledSiteIds = new Set(
    schedules
      .filter(
        (schedule) =>
          schedule.enabled &&
          visibleSiteIds.has(schedule.upstreamSiteId),
      )
      .map((schedule) => schedule.upstreamSiteId),
  );

  // Filter next-runs to show only per-site items (prefixed with "channel.")
  const siteNextRuns = nextRuns.filter(
    (r) => r.siteId && visibleSiteIds.has(r.siteId),
  );

  useEffect(() => {
    void refresh();
    void refreshCalendar();
  }, []);

  return (
    <>
      {/* -- Per-site scheduling cards -- */}
      <article className="card site-schedules-card">
        <div className="section-heading">
          <div>
            <strong>站点独立签到排程</strong>
            <span>
              已启用 {scheduledSiteIds.size} / {sites.length} 个站点 · 取消勾选"启用"即恢复全局调度
            </span>
          </div>
          <button className="ghost" disabled={busy !== ""} onClick={() => { void refresh(); void refreshCalendar(); }}>
            刷新
          </button>
        </div>

        {sites.length === 0 ? (
          <div className="detail-hint problem-hint" style={{ padding: "16px 0" }}>
            暂无站点。请先在"站点"标签页导入或扫描添加上游站点。
          </div>
        ) : (
          <div className="site-schedule-list">
            {sites.map((site) => {
              const form = forms[site.id];
              const schedule = schedules.find(
                (s) => s.upstreamSiteId === site.id,
              );
              const isEnabled = form?.enabled ?? false;
              const skipDates = form?.skipDates || [];

              return (
                <article
                  className={`site-schedule-row ${isEnabled ? "is-active" : "is-idle"}`}
                  key={site.id}
                >
                  <div className="site-schedule-header">
                    <div className="site-schedule-info">
                      <strong>{site.name}</strong>
                      <span className="site-schedule-meta">
                        {site.accountCount} 个账号 ·{" "}
                        {site.supportsCheckin ? "支持签到" : "不支持签到"}
                      </span>
                    </div>
                    <label className="check">
                      <input
                        type="checkbox"
                        checked={isEnabled}
                        onChange={(e) =>
                          updateForm(site.id, {
                            enabled: e.target.checked,
                          })
                        }
                      />
                      启用独立排程
                    </label>
                  </div>

                  {isEnabled ? (
                    <div className="site-schedule-fields">
                      <label className="field compact-field">
                        <span>签到时间</span>
                        <input
                          type="time"
                          value={form?.checkinTime || "08:00"}
                          onChange={(e) =>
                            updateForm(site.id, {
                              checkinTime: e.target.value,
                            })
                          }
                        />
                      </label>

                      <label className="field compact-field">
                        <span>Cron 表达式（可选）</span>
                        <input
                          type="text"
                          placeholder="例: 0 8 * * *"
                          value={form?.cronExpr || ""}
                          onChange={(e) =>
                            updateForm(site.id, {
                              cronExpr: e.target.value,
                            })
                          }
                          className="cron-input"
                        />
                      </label>

                      <label className="field compact-field">
                        <span>延迟范围（分钟）</span>
                        <div className="delay-range">
                          <input
                            type="number"
                            min={0}
                            max={MAX_RANDOM_DELAY_MINUTES}
                            step={1}
                            value={form?.randomDelayMin ?? 0}
                            onChange={(e) => updateRandomDelayMin(site.id, e.target.value)}
                            placeholder="最小"
                          />
                          <span>~</span>
                          <input
                            type="number"
                            min={0}
                            max={MAX_RANDOM_DELAY_MINUTES}
                            step={1}
                            value={form?.randomDelayMax ?? 30}
                            onChange={(e) => updateRandomDelayMax(site.id, e.target.value)}
                            placeholder="最大"
                          />
                          <span>分</span>
                        </div>
                      </label>

                      {/* -- Skip dates -- */}
                      <div className="field compact-field skip-dates-field">
                        <span>跳过日期</span>
                        <div className="skip-dates-input-row">
                          <input
                            type="date"
                            value={skipInputs[site.id] || ""}
                            onChange={(e) =>
                              setSkipInputs((prev) => ({
                                ...prev,
                                [site.id]: e.target.value,
                              }))
                            }
                          />
                          <button
                            className="ghost compact"
                            onClick={() => addSkipDate(site.id)}
                            disabled={!skipInputs[site.id]}
                          >
                            + 添加
                          </button>
                        </div>
                        {skipDates.length > 0 ? (
                          <div className="skip-dates-list">
                            {skipDates.map((d) => (
                              <span key={d} className="skip-date-chip">
                                {formatDate(d)}
                                <button
                                  className="ghost chip-remove"
                                  onClick={() =>
                                    removeSkipDate(site.id, d)
                                  }
                                >
                                  ×
                                </button>
                              </span>
                            ))}
                          </div>
                        ) : (
                          <span className="detail-hint" style={{ fontSize: 11 }}>
                            无跳过日期
                          </span>
                        )}
                      </div>

                      {schedule ? (
                        <div className="site-schedule-times">
                          {schedule.lastRunAt ? (
                            <span className="detail-hint">
                              上次签到：{formatTime(schedule.lastRunAt)}
                            </span>
                          ) : null}
                          {schedule.nextRunAt ? (
                            <span className="detail-hint">
                              下次签到：{formatTime(schedule.nextRunAt)}
                            </span>
                          ) : null}
                        </div>
                      ) : null}

                      <button
                        className="ghost"
                        disabled={busy !== ""}
                        onClick={() => void saveSchedule(site.id)}
                      >
                        {busy === "saving" ? "保存中…" : "保存排程"}
                      </button>
                    </div>
                  ) : form && schedule ? (
                    <div className="site-schedule-times">
                      {schedule.lastRunAt ? (
                        <span className="detail-hint">
                          上次签到：{formatTime(schedule.lastRunAt)} · 排程已暂停
                        </span>
                      ) : null}
                      <button
                        className="ghost"
                        disabled={busy !== ""}
                        onClick={() => void saveSchedule(site.id)}
                      >
                        保存更改（暂停状态）
                      </button>
                    </div>
                  ) : null}
                </article>
              );
            })}
          </div>
        )}

        <div className="problem-hint detail-hint" style={{ marginTop: 12 }}>
          为每个站点设置独立的签到时间后，该站点将按自己的排程运行，不受全局"自动签到"时间影响。
          取消启用即恢复为全局调度。
        </div>
      </article>

      {/* -- Schedule calendar preview -- */}
      {calendarItems.length > 0 && (
        <article className="card">
          <div className="section-heading">
            <div>
              <strong>未来 7 天排程</strong>
              <span>源自各站独立排程和全局调度</span>
            </div>
          </div>
          <div className="calendar-preview-list">
            {calendarItems.map((item, i) => {
              const isCheckin = item.jobType === "checkin";
              return (
                <div
                  className={`calendar-preview-row ${item.enabled ? "" : "dimmed"}`}
                  key={`${item.date}-${item.time}-${item.siteId}-${i}`}
                >
                  <span className="calendar-preview-date">
                    {formatDate(item.date)}
                  </span>
                  <span className="calendar-preview-time">{item.time}</span>
                  <span className="calendar-preview-site">{item.siteName}</span>
                  <span
                    className={`calendar-preview-type ${isCheckin ? "type-checkin" : "type-sync"}`}
                  >
                    {isCheckin ? "签到" : "同步"}
                  </span>
                  {!item.enabled && (
                    <span className="calendar-preview-paused">已暂停</span>
                  )}
                </div>
              );
            })}
          </div>
        </article>
      )}

      {/* -- Next runs summary -- */}
      {siteNextRuns.length > 0 && (
        <article className="card">
          <div className="section-heading">
            <div>
              <strong>下次签到一览</strong>
              <span>各站独立排程的下次执行倒计时</span>
            </div>
          </div>
          <div className="next-runs-list">
            {siteNextRuns.map((run) => (
              <div className="next-run-row" key={run.jobKey}>
                <span className="next-run-label">{run.siteName || run.label}</span>
                <span className="next-run-time">
                  {run.nextRunAt
                    ? formatTime(run.nextRunAt)
                    : "—"}
                </span>
                {run.nextRunInSeconds >= 0 && (
                  <span className="next-run-countdown">
                    {run.nextRunInSeconds < 60
                      ? "< 1 分钟"
                      : run.nextRunInSeconds < 3600
                        ? `${Math.floor(run.nextRunInSeconds / 60)} 分钟后`
                        : `${Math.floor(run.nextRunInSeconds / 3600)} 小时后`}
                  </span>
                )}
                <span
                  className={`next-run-status ${run.status === "scheduled" ? "status-ok" : "status-idle"}`}
                >
                  {run.status === "scheduled" ? "已排程" : run.status}
                </span>
              </div>
            ))}
          </div>
        </article>
      )}

      {previewError ? (
        <div className="problem-hint detail-hint" role="status">
          {previewError}
        </div>
      ) : null}

      {message ? <div className="note">{message}</div> : null}
    </>
  );
}
