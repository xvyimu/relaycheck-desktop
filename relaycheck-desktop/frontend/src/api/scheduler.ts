import { api } from "@/api/client";
import type { ChannelSchedule, NextRunItem, ScheduleCalendarItem } from "@/types";

export type ChannelScheduleWrite = {
  upstreamSiteId: string;
  enabled: boolean;
  checkinTime: string;
  cronExpr: string;
  skipDates: string[];
  randomDelayMin: number;
  randomDelayMax: number;
};

/** 读取按站点签到排程列表。 */
function listChannelSchedules(): Promise<ChannelSchedule[]> {
  return api<ChannelSchedule[]>("/api/scheduler/channel-schedules");
}

/** 保存单个站点排程；body 为声明字段表单。 */
function saveChannelSchedule(form: ChannelScheduleWrite): Promise<{ ok: boolean }> {
  return api<{ ok: boolean }>("/api/scheduler/channel-schedules", {
    method: "PUT",
    body: JSON.stringify(form),
  });
}

/** 读取未来 days 天的排程日历；默认 7。 */
function calendar(days = 7): Promise<{ generatedAt: string; items: ScheduleCalendarItem[] }> {
  const safeDays = Number.isFinite(days) && days > 0 ? Math.floor(days) : 7;
  return api<{ generatedAt: string; items: ScheduleCalendarItem[] }>(`/api/scheduler/calendar?days=${safeDays}`);
}

/** 读取下次运行预览。 */
function nextRuns(): Promise<{ generatedAt: string; items: NextRunItem[] }> {
  return api<{ generatedAt: string; items: NextRunItem[] }>("/api/scheduler/next-runs");
}

export const schedulerApi = {
  listChannelSchedules,
  saveChannelSchedule,
  calendar,
  nextRuns,
} as const;
