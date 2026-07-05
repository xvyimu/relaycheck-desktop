import { describe, expect, it } from "vitest";

import {
  emptyNextRuns,
  emptyScheduleCalendar,
  groupScheduleCalendarItems,
  nextRunItems,
  scheduleCalendarItems,
  schedulerCalendarPath,
  schedulerNextRunsPath,
} from "@/lib/schedulerPreview";

describe("schedulerPreview", () => {
  it("builds stable scheduler endpoint paths", () => {
    expect(schedulerNextRunsPath).toBe("/api/scheduler/next-runs");
    expect(schedulerCalendarPath(2)).toBe("/api/scheduler/calendar?days=2");
    expect(schedulerCalendarPath(0)).toBe("/api/scheduler/calendar?days=1");
    expect(schedulerCalendarPath(99)).toBe("/api/scheduler/calendar?days=31");
  });

  it("returns stable fallback item arrays", () => {
    expect(nextRunItems(null)).toBe(emptyNextRuns.items);
    expect(scheduleCalendarItems(undefined)).toBe(emptyScheduleCalendar.items);
  });

  it("groups calendar items by date without reordering each date", () => {
    const items = [
      { date: "2026-07-04", time: "08:00", siteName: "A", siteId: "a", jobType: "checkin" as const, enabled: true },
      { date: "2026-07-05", time: "09:00", siteName: "B", siteId: "b", jobType: "sync" as const, enabled: true },
      { date: "2026-07-04", time: "10:00", siteName: "C", siteId: "c", jobType: "checkin" as const, enabled: true },
    ];

    const groups = groupScheduleCalendarItems(items);

    expect(Object.keys(groups)).toEqual(["2026-07-04", "2026-07-05"]);
    expect(groups["2026-07-04"].map((item) => item.siteName)).toEqual(["A", "C"]);
    expect(groups["2026-07-05"].map((item) => item.siteName)).toEqual(["B"]);
  });
});
