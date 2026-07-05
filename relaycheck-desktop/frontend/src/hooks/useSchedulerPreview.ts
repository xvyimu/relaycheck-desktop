import { useMemo } from "react";

import { useApi } from "@/hooks/useApi";
import {
  emptyNextRuns,
  emptyScheduleCalendar,
  groupScheduleCalendarItems,
  nextRunItems,
  scheduleCalendarItems,
  schedulerCalendarPath,
  schedulerNextRunsPath,
  type NextRunResponse,
  type ScheduleCalendarResponse,
} from "@/lib/schedulerPreview";

export function useSchedulerPreview(calendarDays = 2) {
  const calendarURL = useMemo(() => schedulerCalendarPath(calendarDays), [calendarDays]);
  const calendar = useApi<ScheduleCalendarResponse>(calendarURL, emptyScheduleCalendar);
  const nextRuns = useApi<NextRunResponse>(schedulerNextRunsPath, emptyNextRuns);

  const calendarItems = scheduleCalendarItems(calendar.data);
  const calendarGroups = useMemo(() => groupScheduleCalendarItems(calendarItems), [calendarItems]);

  return {
    calendarItems,
    calendarGroups,
    calendarLoading: calendar.loading,
    calendarError: calendar.error,
    refreshCalendar: calendar.refresh,
    nextRuns: nextRunItems(nextRuns.data),
    nextRunsLoading: nextRuns.loading,
    nextRunsError: nextRuns.error,
    refreshNextRuns: nextRuns.refresh,
  };
}

export type SchedulerPreviewState = ReturnType<typeof useSchedulerPreview>;
