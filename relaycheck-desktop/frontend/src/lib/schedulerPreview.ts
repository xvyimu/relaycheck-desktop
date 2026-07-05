import type { NextRunItem, ScheduleCalendarItem } from "@/types";

export type ScheduleCalendarResponse = {
  generatedAt: string;
  items: ScheduleCalendarItem[];
};

export type NextRunResponse = {
  generatedAt: string;
  items: NextRunItem[];
};

export const emptyScheduleCalendar: ScheduleCalendarResponse = {
  generatedAt: "",
  items: [],
};

export const emptyNextRuns: NextRunResponse = {
  generatedAt: "",
  items: [],
};

export const schedulerNextRunsPath = "/api/scheduler/next-runs";

export function schedulerCalendarPath(days: number) {
  const normalized = normalizeSchedulerPreviewDays(days);
  return `/api/scheduler/calendar?days=${normalized}`;
}

export function normalizeSchedulerPreviewDays(days: number) {
  if (!Number.isFinite(days)) return 7;
  const whole = Math.trunc(days);
  if (whole <= 0) return 1;
  if (whole > 31) return 31;
  return whole;
}

export function nextRunItems(response: NextRunResponse | null | undefined) {
  return response?.items ?? emptyNextRuns.items;
}

export function scheduleCalendarItems(response: ScheduleCalendarResponse | null | undefined) {
  return response?.items ?? emptyScheduleCalendar.items;
}

export function groupScheduleCalendarItems(items: ScheduleCalendarItem[]) {
  const groups: Record<string, ScheduleCalendarItem[]> = {};
  for (const item of items) {
    if (!groups[item.date]) groups[item.date] = [];
    groups[item.date].push(item);
  }
  return groups;
}
