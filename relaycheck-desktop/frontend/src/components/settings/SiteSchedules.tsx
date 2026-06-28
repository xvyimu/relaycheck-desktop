import { useEffect, useState } from "react";
import { api } from "@/api/client";
import { formatTime } from "@/lib/format";
import type { ChannelSchedule, UpstreamSite } from "@/types";

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

  // Local editing state: map siteId -> form
  const [forms, setForms] = useState<Record<string, SiteScheduleForm>>({});

  async function refresh() {
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
  }

  async function saveSchedule(siteId: string) {
    const form = forms[siteId];
    if (!form) return;

    setBusy("saving");
    setMessage("");
    try {
      const result = await api<{ ok: boolean }>("/api/scheduler/channel-schedules", {
        method: "PUT",
        body: JSON.stringify(form),
      });
      if (result.ok) {
        setMessage("已保存 " + (sites.find((s) => s.id === siteId)?.name || siteId) + " 的签到排程。");
        await refresh();
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

  // Determine scheduled real sites (exclude the synthetic global schedule row).
  const visibleSiteIds = new Set(sites.map((site) => site.id));
  const scheduledSiteIds = new Set(
    schedules
      .filter((schedule) => schedule.enabled && visibleSiteIds.has(schedule.upstreamSiteId))
      .map((schedule) => schedule.upstreamSiteId),
  );

  useEffect(() => {
    void refresh();
  }, []);

  return (
    <>
      <article className="card site-schedules-card">
        <div className="section-heading">
          <div>
            <strong>站点独立签到排程</strong>
            <span>
              已启用 {scheduledSiteIds.size} / {sites.length} 个站点 · 取消勾选"启用"即恢复全局调度
            </span>
          </div>
          <button className="ghost" disabled={busy !== ""} onClick={() => void refresh()}>刷新</button>
        </div>

        {sites.length === 0 ? (
          <div className="detail-hint problem-hint" style={{ padding: "16px 0" }}>
            暂无站点。请先在"站点"标签页导入或扫描添加上游站点。
          </div>
        ) : (
          <div className="site-schedule-list">
            {sites.map((site) => {
              const form = forms[site.id];
              const schedule = schedules.find((s) => s.upstreamSiteId === site.id);
              const isEnabled = form?.enabled ?? false;

              return (
                <article
                  className={`site-schedule-row ${isEnabled ? "is-active" : "is-idle"}`}
                  key={site.id}
                >
                  <div className="site-schedule-header">
                    <div className="site-schedule-info">
                      <strong>{site.name}</strong>
                      <span className="site-schedule-meta">
                        {site.accountCount} 个账号 · {site.supportsCheckin ? "支持签到" : "不支持签到"}
                      </span>
                    </div>
                    <label className="check">
                      <input
                        type="checkbox"
                        checked={isEnabled}
                        onChange={(e) => updateForm(site.id, { enabled: e.target.checked })}
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
                          onChange={(e) => updateForm(site.id, { checkinTime: e.target.value })}
                        />
                      </label>
                      <label className="field compact-field">
                        <span>延迟范围（分钟）</span>
                        <div className="delay-range">
                          <input
                            type="number"
                            min={0}
                            max={120}
                            value={form?.randomDelayMin ?? 0}
                            onChange={(e) => updateForm(site.id, {
                              randomDelayMin: Math.max(0, Number(e.target.value) || 0),
                            })}
                            placeholder="最小"
                          />
                          <span>~</span>
                          <input
                            type="number"
                            min={0}
                            max={120}
                            value={form?.randomDelayMax ?? 30}
                            onChange={(e) => updateForm(site.id, {
                              randomDelayMax: Math.max(0, Number(e.target.value) || 30),
                            })}
                            placeholder="最大"
                          />
                          <span>分</span>
                        </div>
                      </label>

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

        <div className="problem-hint detail-hint">
          为每个站点设置独立的签到时间后，该站点将按自己的排程运行，不受全局"自动签到"时间影响。
          取消启用即恢复为全局调度。
        </div>
      </article>
      {message ? <div className="note">{message}</div> : null}
    </>
  );
}
